# CrowdSec Preset Apply Failure - Fix Plan

**Date:** December 12, 2025
**Status:** Analysis Complete - Ready for Implementation
**Severity:** High

---

## Issue Summary

User reported error when applying a CrowdSec preset:

```
Apply failed: read archive: open /app/data/crowdsec/hub_cache/crowdsecurity/caddy/bundle.tgz: no such file or directory. Backup created at /app/data/crowdsec.backup.20251211-194408
```

---

## Root Cause Analysis

### The Bug

The `Apply()` function in [hub_sync.go](../../backend/internal/crowdsec/hub_sync.go#L535) has a **fatal ordering bug** that destroys the cache before reading from it.

### Detailed Flow

1. **Pull Phase (Works Correctly)**
   - User pulls preset `crowdsecurity/caddy`
   - `HubCache.Store()` writes to: `/app/data/crowdsec/hub_cache/crowdsecurity/caddy/bundle.tgz`
   - `CachedPreset.ArchivePath` stores this absolute path

2. **Apply Phase (Bug Occurs)**
   ```
   Step 1: loadCacheMeta() → Returns meta.ArchivePath = "/app/data/crowdsec/hub_cache/.../bundle.tgz"
   Step 2: backupExisting() → RENAMES "/app/data/crowdsec" to "/app/data/crowdsec.backup.TIMESTAMP"
           ⚠️ THIS MOVES THE CACHE TOO! hub_cache is INSIDE crowdsec/
   Step 3: cscli fails (not available or preset not in hub)
   Step 4: os.ReadFile(meta.ArchivePath) → FILE NOT FOUND!
           The path still points to "/app/data/crowdsec/..." but that directory was renamed!
   ```

### Visual Representation

**Before Backup:**
```
/app/data/crowdsec/
├── hub_cache/
│   └── crowdsecurity/
│       └── caddy/
│           ├── bundle.tgz      ← meta.ArchivePath points here
│           ├── preview.yaml
│           └── metadata.json
├── config.yaml
└── other_files/
```

**After `backupExisting()` (line 535):**
```
/app/data/crowdsec.backup.20251211-194408/  ← Renamed!
├── hub_cache/
│   └── crowdsecurity/
│       └── caddy/
│           ├── bundle.tgz      ← File is now HERE
│           ├── preview.yaml
│           └── metadata.json
├── config.yaml
└── other_files/

/app/data/crowdsec/                         ← Directory no longer exists!
```

**Result:** `os.ReadFile(meta.ArchivePath)` fails because the path `/app/data/crowdsec/hub_cache/.../bundle.tgz` no longer exists.

---

## Why This Wasn't Caught Earlier

1. **Tests use temp directories** - Each test creates fresh directories, so the race condition doesn't manifest
2. **cscli path succeeds in CI** - When `cscli` is available and works, the code returns early before hitting the bug
3. **Recent changes to backup logic** - The copy-based fallback and backup improvements may have introduced this ordering issue
4. **Cache directory nested inside DataDir** - The architecture decision to put `hub_cache` inside `DataDir` (crowdsec config) creates this coupling

---

## Fix Options

### Option A: Read Archive Before Backup (Recommended)

**Rationale:** Simple, minimal change, maintains existing backup behavior.

**File:** [backend/internal/crowdsec/hub_sync.go](../../backend/internal/crowdsec/hub_sync.go)

**Changes:**

```go
func (s *HubService) Apply(ctx context.Context, slug string) (ApplyResult, error) {
    // ... existing validation code ...

    result := ApplyResult{AppliedPreset: cleanSlug, Status: "failed"}
    meta, metaErr := s.loadCacheMeta(applyCtx, cleanSlug)
    if metaErr == nil {
        result.CacheKey = meta.CacheKey
    }
    hasCS := s.hasCSCLI(applyCtx)

    // === NEW: Read archive BEFORE backup ===
    var archive []byte
    var archiveErr error
    if metaErr == nil {
        archive, archiveErr = os.ReadFile(meta.ArchivePath)
        if archiveErr != nil {
            logger.Log().WithError(archiveErr).WithField("archive_path", meta.ArchivePath).Warn("failed to read cached archive before backup")
        }
    }
    // === END NEW ===

    backupPath := filepath.Clean(s.DataDir) + ".backup." + time.Now().Format("20060102-150405")
    if err := s.backupExisting(backupPath); err != nil {
        return result, fmt.Errorf("backup: %w", err)
    }
    result.BackupPath = backupPath

    // Try cscli first
    if hasCS {
        cscliErr := s.runCSCLI(applyCtx, cleanSlug)
        if cscliErr == nil {
            result.Status = "applied"
            result.ReloadHint = true
            result.UsedCSCLI = true
            return result, nil
        }
        logger.Log().WithField("slug", cleanSlug).WithError(cscliErr).Warn("cscli install failed; attempting cache fallback")
    }

    // === MODIFIED: Use pre-loaded archive or refresh ===
    if metaErr != nil || archiveErr != nil {
        refreshed, refreshErr := s.refreshCache(applyCtx, cleanSlug, metaErr)
        if refreshErr != nil {
            _ = s.rollback(backupPath)
            return result, fmt.Errorf("load cache for %s: %w", cleanSlug, refreshErr)
        }
        meta = refreshed
        result.CacheKey = meta.CacheKey
        // Re-read archive from refreshed cache location
        archive, archiveErr = os.ReadFile(meta.ArchivePath)
        if archiveErr != nil {
            _ = s.rollback(backupPath)
            return result, fmt.Errorf("read archive: %w", archiveErr)
        }
    }

    // Use the pre-loaded archive bytes
    if err := s.extractTarGz(applyCtx, archive, s.DataDir); err != nil {
        _ = s.rollback(backupPath)
        return result, fmt.Errorf("extract: %w", err)
    }
    // === END MODIFIED ===

    result.Status = "applied"
    result.ReloadHint = true
    result.UsedCSCLI = false
    return result, nil
}
```

### Option B: Move Cache Outside DataDir

**Rationale:** Architectural fix - separates transient cache from operational config.

**Files to modify:**
- [backend/internal/api/handlers/crowdsec_handler.go](../../backend/internal/api/handlers/crowdsec_handler.go) - Change cache location
- [backend/internal/crowdsec/hub_sync.go](../../backend/internal/crowdsec/hub_sync.go) - Add cache dir parameter

**Changes:**
```go
// In NewCrowdsecHandler:
// BEFORE:
cacheDir := filepath.Join(dataDir, "hub_cache")

// AFTER:
cacheDir := filepath.Join(filepath.Dir(dataDir), "hub_cache")
// Results in: /app/data/hub_cache (sibling of crowdsec, not child)
```

**Pros:** Clean separation, cache survives config resets
**Cons:** Breaking change for existing installs, requires migration

### Option C: Selective Backup (Exclude Cache)

**Rationale:** Only backup config files, not cache.

**Changes to `backupExisting()`:**
```go
func (s *HubService) backupExisting(backupPath string) error {
    // ... existing checks ...

    // Skip hub_cache during backup - it's transient
    return filepath.WalkDir(s.DataDir, func(path string, d fs.DirEntry, err error) error {
        if strings.Contains(path, "hub_cache") {
            return filepath.SkipDir
        }
        // ... copy logic ...
    })
}
```

**Pros:** Faster backups, cache preserved
**Cons:** More complex, backup is no longer complete snapshot

---

## Recommended Implementation

**Choose Option A** for these reasons:

1. **Minimal code change** - Single function modification
2. **No breaking changes** - Existing cache paths remain valid
3. **No migration needed** - Works immediately
4. **Maintains complete backups** - Backup still captures full state
5. **Easy to test** - Clear before/after behavior

---

## Files to Modify

| File | Change |
|------|--------|
| [backend/internal/crowdsec/hub_sync.go](../../backend/internal/crowdsec/hub_sync.go) | Reorder archive read before backup in `Apply()` |
| [backend/internal/crowdsec/hub_sync_test.go](../../backend/internal/crowdsec/hub_sync_test.go) | Add test for apply with backup scenario |
| [backend/internal/crowdsec/hub_pull_apply_test.go](../../backend/internal/crowdsec/hub_pull_apply_test.go) | Add regression test |

---

## Specific Code Changes

### Change 1: hub_sync.go - Apply() Function

**Location:** Lines 514-580

**Before:**
```go
func (s *HubService) Apply(ctx context.Context, slug string) (ApplyResult, error) {
    cleanSlug := sanitizeSlug(slug)
    // ... validation ...

    result := ApplyResult{AppliedPreset: cleanSlug, Status: "failed"}
    meta, metaErr := s.loadCacheMeta(applyCtx, cleanSlug)
    if metaErr == nil {
        result.CacheKey = meta.CacheKey
    }
    hasCS := s.hasCSCLI(applyCtx)

    backupPath := filepath.Clean(s.DataDir) + ".backup." + time.Now().Format("20060102-150405")
    if err := s.backupExisting(backupPath); err != nil {
        return result, fmt.Errorf("backup: %w", err)
    }
    result.BackupPath = backupPath

    // Try cscli first
    if hasCS {
        // ... cscli logic ...
    }

    if metaErr != nil {
        // ... refresh cache logic ...
    }

    archive, err := os.ReadFile(meta.ArchivePath)  // ❌ FAILS - file moved by backup!
    if err != nil {
        _ = s.rollback(backupPath)
        return result, fmt.Errorf("read archive: %w", err)
    }
    // ...
}
```

**After:**
```go
func (s *HubService) Apply(ctx context.Context, slug string) (ApplyResult, error) {
    cleanSlug := sanitizeSlug(slug)
    // ... validation ...

    result := ApplyResult{AppliedPreset: cleanSlug, Status: "failed"}
    meta, metaErr := s.loadCacheMeta(applyCtx, cleanSlug)
    if metaErr == nil {
        result.CacheKey = meta.CacheKey
    }
    hasCS := s.hasCSCLI(applyCtx)

    // ✅ NEW: Read archive into memory BEFORE backup moves the files
    var archive []byte
    var archiveReadErr error
    if metaErr == nil {
        archive, archiveReadErr = os.ReadFile(meta.ArchivePath)
        if archiveReadErr != nil {
            logger.Log().WithError(archiveReadErr).WithField("archive_path", meta.ArchivePath).
                Warn("failed to read cached archive before backup")
        }
    }

    backupPath := filepath.Clean(s.DataDir) + ".backup." + time.Now().Format("20060102-150405")
    if err := s.backupExisting(backupPath); err != nil {
        return result, fmt.Errorf("backup: %w", err)
    }
    result.BackupPath = backupPath

    // Try cscli first
    if hasCS {
        cscliErr := s.runCSCLI(applyCtx, cleanSlug)
        if cscliErr == nil {
            result.Status = "applied"
            result.ReloadHint = true
            result.UsedCSCLI = true
            return result, nil
        }
        logger.Log().WithField("slug", cleanSlug).WithError(cscliErr).
            Warn("cscli install failed; attempting cache fallback")
    }

    // ✅ MODIFIED: Handle cache miss OR failed archive read
    if metaErr != nil || archiveReadErr != nil {
        // Need to refresh cache (either wasn't cached or file was unreadable)
        originalErr := metaErr
        if originalErr == nil {
            originalErr = archiveReadErr
        }
        refreshed, refreshErr := s.refreshCache(applyCtx, cleanSlug, originalErr)
        if refreshErr != nil {
            _ = s.rollback(backupPath)
            logger.Log().WithError(refreshErr).WithField("slug", cleanSlug).
                WithField("backup_path", backupPath).
                Warn("cache refresh failed; rolled back backup")
            result.ErrorMessage = fmt.Sprintf("load cache for %s: %v", cleanSlug, refreshErr)
            return result, fmt.Errorf("load cache for %s: %w", cleanSlug, refreshErr)
        }
        meta = refreshed
        result.CacheKey = meta.CacheKey

        // Read from the newly refreshed cache
        archive, archiveReadErr = os.ReadFile(meta.ArchivePath)
        if archiveReadErr != nil {
            _ = s.rollback(backupPath)
            return result, fmt.Errorf("read archive after refresh: %w", archiveReadErr)
        }
    }

    // ✅ Use pre-loaded archive bytes (no file read here)
    if err := s.extractTarGz(applyCtx, archive, s.DataDir); err != nil {
        _ = s.rollback(backupPath)
        return result, fmt.Errorf("extract: %w", err)
    }

    result.Status = "applied"
    result.ReloadHint = true
    result.UsedCSCLI = false
    return result, nil
}
```

### Change 2: Add Regression Test

**File:** [backend/internal/crowdsec/hub_pull_apply_test.go](../../backend/internal/crowdsec/hub_pull_apply_test.go)

**New test:**
```go
func TestApplyReadsArchiveBeforeBackup(t *testing.T) {
    // This test verifies the fix for the bug where Apply() would:
    // 1. Load cache metadata (getting archive path)
    // 2. Backup DataDir (moving the cache!)
    // 3. Try to read archive from original path (FAIL!)

    baseDir := t.TempDir()
    dataDir := filepath.Join(baseDir, "crowdsec")
    cacheDir := filepath.Join(dataDir, "hub_cache")

    // Create cache
    cache, err := NewHubCache(cacheDir, time.Hour)
    require.NoError(t, err)

    // Create a mock hub server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if strings.Contains(r.URL.Path, ".tgz") {
            // Return a valid tar.gz
            var buf bytes.Buffer
            gw := gzip.NewWriter(&buf)
            tw := tar.NewWriter(gw)
            content := []byte("test: value\n")
            tw.WriteHeader(&tar.Header{Name: "test.yaml", Size: int64(len(content)), Mode: 0644})
            tw.Write(content)
            tw.Close()
            gw.Close()
            w.Write(buf.Bytes())
            return
        }
        if strings.Contains(r.URL.Path, ".yaml") {
            w.Write([]byte("preview: content"))
            return
        }
        // Index
        w.Write([]byte(`{"items":[{"name":"test/preset","version":"1.0"}]}`))
    }))
    defer server.Close()

    hub := &HubService{
        Cache:         cache,
        DataDir:       dataDir,
        HTTPClient:    server.Client(),
        HubBaseURL:    server.URL,
        MirrorBaseURL: server.URL,
        PullTimeout:   10 * time.Second,
        ApplyTimeout:  10 * time.Second,
    }

    ctx := context.Background()

    // Pull to populate cache
    _, err = hub.Pull(ctx, "test/preset")
    require.NoError(t, err, "pull should succeed")

    // Verify cache exists
    _, err = cache.Load(ctx, "test/preset")
    require.NoError(t, err, "cache should exist after pull")

    // Add some extra files to DataDir to make backup more realistic
    require.NoError(t, os.WriteFile(filepath.Join(dataDir, "config.yaml"), []byte("test: config"), 0644))

    // Apply - this should NOT fail with "read archive: no such file"
    result, err := hub.Apply(ctx, "test/preset")
    require.NoError(t, err, "apply should succeed - archive should be read before backup")
    assert.Equal(t, "applied", result.Status)
    assert.NotEmpty(t, result.BackupPath)

    // Verify backup was created
    _, err = os.Stat(result.BackupPath)
    assert.NoError(t, err, "backup should exist")
}
```

---

## Edge Cases to Consider

| Scenario | Current Behavior | Fixed Behavior |
|----------|-----------------|----------------|
| First-time apply (no cache) | Fails with cache miss | Attempts refresh, same behavior |
| cscli available and works | Returns early, never hits bug | Same - returns early |
| cscli fails, cache exists | **FAILS** - archive moved | Succeeds - archive pre-loaded |
| Archive file corrupted | Fails on read | Same - fails on read, but before backup |
| Network down during refresh | Fails | Same - fails with clear error |
| Large archive (>25MB) | Limited by maxArchiveSize | Same - memory is fine for 25MB |
| Concurrent applies | Potential race | Still potential race (separate issue) |

---

## Testing Plan

1. **Unit Tests**
   - [ ] `TestApplyReadsArchiveBeforeBackup` - New regression test
   - [ ] Existing `TestPullThenApplyFlow` should still pass
   - [ ] `TestApplyWithoutPullFails` should still pass

2. **Integration Tests**
   - [ ] Manual test in Docker container
   - [ ] Pull preset via UI
   - [ ] Apply preset via UI
   - [ ] Verify no "read archive" error

3. **Edge Case Tests**
   - [ ] Apply with expired cache (should refresh)
   - [ ] Apply with network failure (should error gracefully)
   - [ ] Apply with cscli available (should use cscli path)

---

## Rollout Plan

1. **Implement fix** in `hub_sync.go`
2. **Add regression test** in `hub_pull_apply_test.go`
3. **Run full test suite**: `go test ./...`
4. **Run pre-commit**: `pre-commit run --all-files`
5. **Build and test locally**: `docker build -t charon:local .`
6. **Manual verification in container**
7. **Commit with**: `fix: read archive before backup in CrowdSec preset apply`

---

## Related Files Reference

| File | Purpose |
|------|---------|
| [hub_sync.go](../../backend/internal/crowdsec/hub_sync.go) | HubService.Apply() - main fix location |
| [hub_cache.go](../../backend/internal/crowdsec/hub_cache.go) | Cache storage, stores ArchivePath |
| [crowdsec_handler.go](../../backend/internal/api/handlers/crowdsec_handler.go) | HTTP handler, initializes cache |
| [routes.go](../../backend/internal/api/routes/routes.go) | Sets crowdsecDataDir from config |
| [config.go](../../backend/internal/config/config.go) | CrowdSecConfigDir default |

---

## Summary

**Root Cause:** The `Apply()` function backs up the entire DataDir (which includes the cache) before reading the cached archive, resulting in a "file not found" error.

**Fix:** Read the archive into memory before creating the backup.

**Impact:** Low risk - the fix only changes the order of operations and doesn't affect the backup or extraction logic.

**Effort:** ~30 minutes implementation + testing
| 1 | Cerberus shows ON by default on first load (should be OFF) | High |
| 2 | Cerberus dashboard header shows "disabled" even when enabled | Medium |
| 3 | CrowdSec toggle auto-enables when Cerberus is enabled | Medium |
| 4 | CrowdSec toggle unresponsive + Config button grayed out | High |

---

## Root Cause Analysis

### Issue 1: Cerberus Shows ON by Default

**Root Cause:** The `feature_flags_handler.go` has a default value of `true` for all feature flags including `feature.cerberus.enabled`.

**File:** [backend/internal/api/handlers/feature_flags_handler.go#L39-L42](../../backend/internal/api/handlers/feature_flags_handler.go#L39-L42)

```go
// Line 39-42
for _, key := range defaultFlags {
    defaultVal := true  // <-- THIS IS THE BUG
    if v, ok := defaultFlagValues[key]; ok {
        defaultVal = v
    }
```

**Problem:** The code sets `defaultVal := true` for all flags, then only overrides it if the key exists in `defaultFlagValues`. However, `feature.cerberus.enabled` is NOT in `defaultFlagValues`:

```go
// Line 29-31
var defaultFlagValues = map[string]bool{
    "feature.crowdsec.console_enrollment": false,
}
```

**Result:** On first load with an empty database, `feature.cerberus.enabled` defaults to `true` instead of `false`.

**Additional Context:**
- The [backend/internal/config/config.go#L60](../../backend/internal/config/config.go#L60) correctly defaults `CerberusEnabled` to `false`:
  ```go
  CerberusEnabled: getEnvAny("false", "CERBERUS_SECURITY_CERBERUS_ENABLED", ...) == "true"
  ```
- However, the feature flags handler ignores this config and uses its own default.

---

### Issue 2: Dashboard Header Shows "Disabled" Even When Enabled

**Root Cause:** The header banner logic in `Security.tsx` checks `status.cerberus?.enabled` which comes from the security status API, but there's a **data source mismatch**.

**Files:**
- [frontend/src/pages/Security.tsx#L141-L153](../../frontend/src/pages/Security.tsx#L141-L153) - Header banner logic
- [backend/internal/api/handlers/security_handler.go#L35-L49](../../backend/internal/api/handlers/security_handler.go#L35-L49) - Security status API

**Problem Flow:**

1. **Security.tsx** checks `status.cerberus?.enabled` from `/api/v1/security/status`
2. **security_handler.go** reads from config AND settings table:
   ```go
   // Line 36-48
   enabled := h.cfg.CerberusEnabled
   var settingKey = "security.cerberus.enabled"  // <-- WRONG KEY!
   if h.db != nil {
       var setting struct{ Value string }
       if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", settingKey).Scan(&setting).Error; ...
   ```
3. **SystemSettings.tsx** toggles `feature.cerberus.enabled` (via feature flags API)

**The Mismatch:**

| Component | Key Used |
|-----------|----------|
| SystemSettings toggle | `feature.cerberus.enabled` |
| Security status API | `security.cerberus.enabled` |

The toggle writes to `feature.cerberus.enabled` but the security status reads from `security.cerberus.enabled` - **two different keys!**

---

### Issue 3: CrowdSec Auto-Enables When Cerberus is Enabled

**Root Cause:** The `docker-compose.override.yml` and `docker-compose.local.yml` both set `CHARON_SECURITY_CROWDSEC_MODE=local`:

**File:** [docker-compose.override.yml#L21](../../docker-compose.override.yml#L21)
```yaml
- CHARON_SECURITY_CROWDSEC_MODE=local
```

**Problem:** When the container starts:
1. Config loads with `CrowdSecMode: "local"` from env var
2. Security status API returns `crowdsec.enabled: true` because mode is "local"
3. Frontend shows CrowdSec as enabled

**File:** [backend/internal/api/handlers/security_handler.go#L59-L62](../../backend/internal/api/handlers/security_handler.go#L59-L62)
```go
// Allow runtime override for CrowdSec enabled flag via settings table
crowdsecEnabled := mode == "local"  // <-- Auto-true if mode is "local"
```

---

### Issue 4: CrowdSec Toggle Unresponsive + Config Button Grayed Out

**Root Cause:** Multiple issues combine to break the toggle:

**A. Toggle Disabled Logic:**

**File:** [frontend/src/pages/Security.tsx#L127](../../frontend/src/pages/Security.tsx#L127)
```tsx
const crowdsecToggleDisabled = cerberusDisabled || crowdsecPowerMutation.isPending
```

**File:** [frontend/src/pages/Security.tsx#L126](../../frontend/src/pages/Security.tsx#L126)
```tsx
const cerberusDisabled = !status.cerberus?.enabled
```

Since `status.cerberus?.enabled` is `false` due to Issue 2 (wrong settings key), `cerberusDisabled` is `true`, making the toggle disabled.

**B. Config Button Disabled:**

**File:** [frontend/src/pages/Security.tsx#L128](../../frontend/src/pages/Security.tsx#L128)
```tsx
const crowdsecControlsDisabled = cerberusDisabled || crowdsecPowerMutation.isPending
```

Same logic - the controls are disabled because Cerberus appears disabled.

**C. Switch Component Event Handling:**

**File:** [frontend/src/components/ui/Switch.tsx#L17-L20](../../frontend/src/components/ui/Switch.tsx#L17-L20)

The Switch component passes `disabled` to the native checkbox input, which prevents click events. This is correct behavior - the issue is the `disabled` prop is incorrectly `true`.

---

## Recommended Fixes

### Fix 1: Update Feature Flag Defaults

**File:** `backend/internal/api/handlers/feature_flags_handler.go`

```go
// Change defaultFlagValues to include cerberus.enabled as false
var defaultFlagValues = map[string]bool{
    "feature.cerberus.enabled":            false, // ADD THIS
    "feature.crowdsec.console_enrollment": false,
    "feature.uptime.enabled":              true,  // Uptime can default ON
}
```

### Fix 2: Align Settings Keys

**Option A (Recommended):** Update security_handler.go to read from feature flags key

**File:** `backend/internal/api/handlers/security_handler.go`

```go
// Line 37: Change from
var settingKey = "security.cerberus.enabled"
// To
var settingKey = "feature.cerberus.enabled"
```

**Option B:** Create a sync mechanism between feature flags and security settings

### Fix 3: Remove CrowdSec Mode Override from Docker Compose

**Files:**
- `docker-compose.override.yml`
- `docker-compose.local.yml`

```yaml
# Remove or comment out:
# - CHARON_SECURITY_CROWDSEC_MODE=local
# Or change to:
- CHARON_SECURITY_CROWDSEC_MODE=disabled
```

### Fix 4: No Additional Fix Needed

Issue 4 is a symptom of Issues 1-2. Once those are fixed:
- `cerberusDisabled` will be `false` when Cerberus is enabled
- `crowdsecToggleDisabled` will be `false`
- `crowdsecControlsDisabled` will be `false`
- Toggle and Config button will be interactive

---

## Test Scenarios

### Test 1: Fresh Install Default State
```
Given: Clean database, no env vars set
When: User loads the Settings > System page
Then: Cerberus toggle should be OFF
And: /api/v1/feature-flags returns { "feature.cerberus.enabled": false }
```

### Test 2: Cerberus Toggle Sync
```
Given: User is on Settings > System page
When: User enables Cerberus toggle
Then: /api/v1/security/status returns { "cerberus": { "enabled": true } }
And: Security dashboard header banner is NOT displayed
```

### Test 3: CrowdSec Toggle Interaction
```
Given: Cerberus is enabled
And: User is on Security dashboard
When: User clicks CrowdSec toggle
Then: Toggle should respond to click
And: CrowdSec enabled state should change
And: Toast notification should appear
```

### Test 4: CrowdSec Config Button
```
Given: Cerberus is enabled
And: User is on Security dashboard
When: User clicks CrowdSec "Config" button
Then: User should navigate to /security/crowdsec
And: Button should NOT be grayed out
```

### Test 5: Environment Variable Override
```
Given: CERBERUS_SECURITY_CERBERUS_ENABLED=true set
When: User loads Settings > System (fresh DB)
Then: Cerberus toggle should be ON (env override)
```

---

## Implementation Priority

| Priority | Fix | Effort | Impact |
|----------|-----|--------|--------|
| P0 | Fix 2 (Key alignment) | Low | High - Fixes Issues 2, 4 |
| P1 | Fix 1 (Default values) | Low | High - Fixes Issue 1 |
| P2 | Fix 3 (Docker compose) | Low | Medium - Fixes Issue 3 |

---

## Files to Modify

1. **backend/internal/api/handlers/feature_flags_handler.go** - Add default value for cerberus
2. **backend/internal/api/handlers/security_handler.go** - Change settings key to `feature.cerberus.enabled`
3. **docker-compose.override.yml** - Remove or change CrowdSec mode
4. **docker-compose.local.yml** - Remove or change CrowdSec mode

---

## Additional Observations

1. **Dual Control Systems:** There are two overlapping control systems:
   - Feature flags (`feature.cerberus.enabled`) - toggled in SystemSettings.tsx
   - Security config (`SecurityConfig.Enabled` in DB) - used by Enable/Disable endpoints

   Consider consolidating to one source of truth.

2. **Config vs Settings:** The `config.SecurityConfig` struct loaded from env vars is separate from DB-backed `SecurityConfig` model. This creates confusion about which takes precedence.

3. **No Migration:** When updating default values, existing users may need a migration or reset to see the new defaults.

---

## Code Reference Summary

| File | Line | Purpose |
|------|------|---------|
| `feature_flags_handler.go` | L29-31 | Missing cerberus default |
| `feature_flags_handler.go` | L39 | `defaultVal := true` bug |
| `security_handler.go` | L37 | Wrong settings key |
| `Security.tsx` | L126-128 | Disabled state logic |
| `SystemSettings.tsx` | L99-105 | Feature toggle UI |
| `docker-compose.override.yml` | L21 | CrowdSec mode env var |
| `config.go` | L60 | Correct cerberus default |
