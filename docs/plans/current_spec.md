# Current Project Specification

## Active Issue: CrowdSec Non-Root Migration Fix - REVISED

**Status**: Implementation Ready - Supervisor Review Complete
**Priority**: CRITICAL
**Last Updated**: 2024-12-22 (Revised after supervisor review)

### Quick Summary

The container migration from root to non-root user broke CrowdSec. Supervisor review identified **7 critical issues** that would cause the original fix to fail. This revised plan addresses all issues.

**Root Cause**: Permission issues, missing symlink creation logic, and incomplete config template population.

### Changes Required

1. **Dockerfile** (Line ~332): Add config template population before final COPY
2. **Entrypoint Script** (Lines 68-73): Replace symlink verification with creation logic
3. **Entrypoint Script** (Line 100): Fix LOG variable to use directory-based path
4. **Entrypoint Script** (Line 51): Add hub_cache directory creation
5. **Entrypoint Script** (Line 99): Keep CFG pointing to `/etc/crowdsec` (resolves via symlink)
6. **Entrypoint Script** (Lines 68-73): Strengthen error handling in migration
7. **Verification Checklist**: Expand from 7 to 11 steps

---

## Detailed Implementation Plan

### Issue 1: Missing Config Template Population (HIGH PRIORITY)

**Location**: `Dockerfile` before line 332 (before final COPY commands)

**Problem**: The Dockerfile doesn't populate `/etc/crowdsec.dist/` with CrowdSec default configs (`config.yaml`, `user.yaml`, etc.). This causes the entrypoint script to have nothing to copy when initializing persistent storage.

**Current Code** (Lines 330-332):
```dockerfile
# Copy CrowdSec configuration templates from source
COPY configs/crowdsec/acquis.yaml /etc/crowdsec.dist/acquis.yaml
COPY configs/crowdsec/install_hub_items.sh /usr/local/bin/install_hub_items.sh
```

**Required Fix** (Add BEFORE line 330):
```dockerfile
# Generate CrowdSec default configs to .dist directory
RUN if command -v cscli >/dev/null; then \
        mkdir -p /etc/crowdsec.dist && \
        cscli config restore /etc/crowdsec.dist/ || \
        cp -r /etc/crowdsec/* /etc/crowdsec.dist/ 2>/dev/null || true; \
    fi
```

**Rationale**: The `cscli config restore` command generates all required default configs (`config.yaml`, `user.yaml`, `local_api_credentials.yaml`, etc.). If that fails, we fall back to copying any existing configs. This ensures the `.dist` directory is always populated for the entrypoint to use.

**Risk**: Low - Command has multiple fallbacks and won't fail the build if CrowdSec is unavailable.

---

### Issue 2: Symlink Not Created (HIGH PRIORITY)

**Location**: `.docker/docker-entrypoint.sh` lines 68-73

**Problem**: The entrypoint only VERIFIES the symlink exists but never CREATES it. This is the root cause of CrowdSec failures.

**Current Code** (Lines 68-73):
```bash
# Link /etc/crowdsec to persistent config for runtime compatibility
# Note: This symlink is created at build time; verify it exists
if [ -L "/etc/crowdsec" ]; then
    echo "CrowdSec config symlink verified: /etc/crowdsec -> $CS_CONFIG_DIR"
else
    echo "Warning: /etc/crowdsec symlink not found. CrowdSec may use volume config directly."
fi
```

**Required Fix** (Replace lines 68-73):
```bash
# Migrate existing directory to persistent storage if needed
if [ -d "/etc/crowdsec" ] && [ ! -L "/etc/crowdsec" ]; then
    echo "Migrating /etc/crowdsec to persistent storage..."
    if [ -n "$(ls -A /etc/crowdsec 2>/dev/null)" ]; then
        cp -rn /etc/crowdsec/* "$CS_CONFIG_DIR/" || {
            echo "ERROR: Failed to migrate configs"
            exit 1
        }
    fi
    rm -rf /etc/crowdsec || {
        echo "ERROR: Failed to remove old directory"
        exit 1
    }
fi

# Create symlink if it doesn't exist
if [ ! -L "/etc/crowdsec" ]; then
    ln -sf "$CS_CONFIG_DIR" /etc/crowdsec || {
        echo "ERROR: Failed to create symlink"
        exit 1
    }
    echo "Created symlink: /etc/crowdsec -> $CS_CONFIG_DIR"
fi
```

**Rationale**: This implements proper migration logic with fail-fast error handling. If `/etc/crowdsec` exists as a directory, we migrate its contents before creating the symlink.

**Risk**: Medium - Changes startup flow. Must test with both fresh and existing volumes.

---

### Issue 3: Wrong LOG Environment Variable

**Location**: `.docker/docker-entrypoint.sh` line 100

**Problem**: The `LOG` variable points directly to a file instead of using the log directory variable, breaking consistency.

**Current Code** (Line 100):
```bash
export LOG=/var/log/crowdsec.log
```

**Required Fix** (Replace line 100):
```bash
export LOG="$CS_LOG_DIR/crowdsec.log"
```

**Required Addition** (Add after line 47 where other CS_* variables are defined):
```bash
CS_LOG_DIR="/var/log/crowdsec"
```

**Rationale**: Ensures all CrowdSec paths are consistently managed through variables, making future changes easier.

**Risk**: Low - Simple variable change with no behavioral impact.

---

### Issue 4: Missing Hub Cache Directory

**Location**: `.docker/docker-entrypoint.sh` after line 51

**Problem**: The hub cache directory `/app/data/crowdsec/hub_cache/` is never explicitly created, causing hub operations to fail.

**Current Code** (Lines 49-51):
```bash
# Ensure persistent directories exist (within writable volume)
mkdir -p "$CS_CONFIG_DIR" 2>/dev/null || echo "Warning: Cannot create $CS_CONFIG_DIR"
mkdir -p "$CS_DATA_DIR" 2>/dev/null || echo "Warning: Cannot create $CS_DATA_DIR"
```

**Required Fix** (Add after line 51):
```bash
mkdir -p "$CS_PERSIST_DIR/hub_cache"
```

**Rationale**: CrowdSec stores hub metadata in a separate cache directory. Without this, `cscli hub update` fails silently.

**Risk**: Low - Simple directory creation with no side effects.

---

### Issue 5: CFG Variable Should Stay /etc/crowdsec

**Location**: `.docker/docker-entrypoint.sh` line 99

**Problem**: The original plan incorrectly suggested changing CFG to `$CS_CONFIG_DIR`, but it should remain `/etc/crowdsec` since it resolves to persistent storage via the symlink.

**Current Code** (Line 99):
```bash
export CFG=/etc/crowdsec
```

**Required Action**: **KEEP AS-IS** - Do NOT change this line.

**Rationale**: The CFG variable should point to `/etc/crowdsec` which resolves to `$CS_CONFIG_DIR` via symlink. This maintains compatibility with CrowdSec's expected paths while still using persistent storage.

**Risk**: None - No change required.

---

### Issue 6: Weak Migration Error Handling

**Location**: `.docker/docker-entrypoint.sh` lines 56-62

**Problem**: Too many `|| true` statements allow silent failures during config migration.

**Current Code** (Lines 56-62):
```bash
# Initialize persistent config if key files are missing
if [ ! -f "$CS_CONFIG_DIR/config.yaml" ]; then
    echo "Initializing persistent CrowdSec configuration..."
    if [ -d "/etc/crowdsec.dist" ]; then
        cp -r /etc/crowdsec.dist/* "$CS_CONFIG_DIR/" 2>/dev/null || echo "Warning: Could not copy dist config"
    elif [ -d "/etc/crowdsec" ] && [ ! -L "/etc/crowdsec" ]; then
        # Fallback if .dist is missing
        cp -r /etc/crowdsec/* "$CS_CONFIG_DIR/" 2>/dev/null || echo "Warning: Could not copy config"
    fi
fi
```

**Required Fix** (Replace lines 56-62):
```bash
# Initialize persistent config if key files are missing
if [ ! -f "$CS_CONFIG_DIR/config.yaml" ]; then
    echo "Initializing persistent CrowdSec configuration..."
    if [ -d "/etc/crowdsec.dist" ] && [ -n "$(ls -A /etc/crowdsec.dist 2>/dev/null)" ]; then
        cp -r /etc/crowdsec.dist/* "$CS_CONFIG_DIR/" || {
            echo "ERROR: Failed to copy config from /etc/crowdsec.dist"
            exit 1
        }
        echo "Successfully initialized config from .dist directory"
    elif [ -d "/etc/crowdsec" ] && [ ! -L "/etc/crowdsec" ] && [ -n "$(ls -A /etc/crowdsec 2>/dev/null)" ]; then
        cp -r /etc/crowdsec/* "$CS_CONFIG_DIR/" || {
            echo "ERROR: Failed to copy config from /etc/crowdsec"
            exit 1
        }
        echo "Successfully initialized config from /etc/crowdsec"
    else
        echo "ERROR: No config source found (neither .dist nor /etc/crowdsec available)"
        exit 1
    fi
fi
```

**Rationale**: Fail-fast approach ensures we detect misconfigurations early. Empty directory checks prevent copying empty directories.

**Risk**: Medium - Strict error handling may reveal edge cases. Must test thoroughly.

---

### Issue 7: Incomplete Verification Checklist

**Problem**: Original checklist had only 7 steps and missed critical tests for volume replacement, permissions, config persistence, and hub updates.

**Original Checklist** (Steps 1-7):
1. Fresh container start with empty volumes
2. Container restart (data persists)
3. CrowdSec enable/disable via UI
4. Log file permissions and rotation
5. LAPI readiness and machine registration
6. Hub updates and parsers
7. Multi-architecture compatibility

**Required Additional Steps** (8-11):
8. **Volume Replacement Test**: Start container with volume → destroy volume → recreate volume. Verify configs regenerate correctly.
9. **Permission Inheritance**: Create new files in persistent storage (e.g., `cscli decisions add`). Verify ownership is correct (1000:1000).
10. **Config Persistence**: Make config changes via `cscli` (e.g., add bouncer, modify settings). Restart container. Verify changes persist.
11. **Hub Update Test**: Run `cscli hub update && cscli hub upgrade`. Verify hub data is stored in persistent volume and survives restarts.

**Rationale**: These tests cover critical failure modes discovered in production: volume loss, permission issues on newly created files, config changes not persisting, and hub data being ephemeral.

**Risk**: None - This is documentation only.

---

## Implementation Order

Follow this sequence to apply changes safely:

### Phase 1: Dockerfile Changes (Low Risk)
1. Add config template population to `Dockerfile` before line 330
2. Build test image: `docker build -t charon:test .`
3. Verify `/etc/crowdsec.dist/` is populated: `docker run --rm charon:test ls -la /etc/crowdsec.dist/`
4. Expected output: `config.yaml`, `user.yaml`, `local_api_credentials.yaml`, `profiles.yaml`

### Phase 2: Entrypoint Script Changes (Medium Risk)
5. Apply all 5 entrypoint script fixes in a single commit (they're interdependent)
6. Rebuild image: `docker build -t charon:test .`
7. Test with fresh volumes (see Phase 3)

### Phase 3: Testing Strategy
Run all 11 verification tests in order:

**Test 1: Fresh Start**
```bash
docker volume create charon_data_test
docker run -d --name charon_test -v charon_data_test:/app/data charon:test
docker logs charon_test | grep -E "(symlink|CrowdSec config)"
```
Expected: "Created symlink: /etc/crowdsec -> /app/data/crowdsec/config"

**Test 2: Container Restart**
```bash
docker restart charon_test
docker logs charon_test | grep "symlink verified"
```
Expected: "CrowdSec config symlink verified: /etc/crowdsec -> /app/data/crowdsec/config"

**Test 3-7**: Follow existing test procedures from original plan

**Test 8: Volume Replacement**
```bash
docker stop charon_test
docker rm charon_test
docker volume rm charon_data_test
docker volume create charon_data_test
docker run -d --name charon_test -v charon_data_test:/app/data charon:test
docker exec charon_test ls -la /app/data/crowdsec/config/
```
Expected: `config.yaml` and other files regenerated

**Test 9: Permission Inheritance**
```bash
docker exec charon_test cscli decisions add -i 1.2.3.4
docker exec charon_test ls -ln /app/data/crowdsec/data/
```
Expected: All files owned by uid 1000, gid 1000

**Test 10: Config Persistence**
```bash
docker exec charon_test cscli config set api.server.log_level=debug
docker restart charon_test
docker exec charon_test cscli config show api.server.log_level
```
Expected: "debug"

**Test 11: Hub Update**
```bash
docker exec charon_test cscli hub update
docker exec charon_test ls -la /app/data/crowdsec/hub_cache/
docker restart charon_test
docker exec charon_test cscli hub list -o json
```
Expected: Hub cache persists, parsers/scenarios remain installed

### Phase 4: Rollback Procedure
If any test fails:
1. Tag working version: `docker tag charon:current charon:rollback`
2. Revert changes to `.docker/docker-entrypoint.sh` and `Dockerfile`
3. Rebuild: `docker build -t charon:current .`
4. Document failure in issue tracker with test logs

---

## Summary of All Changes

| File | Line(s) | Change Type | Priority | Risk |
|------|---------|-------------|----------|------|
| `Dockerfile` | Before 330 | Add config restore RUN | HIGH | Low |
| `.docker/docker-entrypoint.sh` | 47 | Add CS_LOG_DIR variable | HIGH | Low |
| `.docker/docker-entrypoint.sh` | 51 | Add hub_cache mkdir | HIGH | Low |
| `.docker/docker-entrypoint.sh` | 56-62 | Strengthen config init | HIGH | Medium |
| `.docker/docker-entrypoint.sh` | 68-73 | Implement symlink creation | HIGH | Medium |
| `.docker/docker-entrypoint.sh` | 99 | Keep CFG=/etc/crowdsec | NONE | None |
| `.docker/docker-entrypoint.sh` | 100 | Fix LOG variable | HIGH | Low |
| Verification checklist | N/A | Add 4 new tests | HIGH | None |

---

## Risk Assessment

### Low Risk Changes (Can be applied immediately)
- Dockerfile config template population
- LOG variable fix
- Hub cache directory creation
- CFG variable (no change)

### Medium Risk Changes (Require thorough testing)
- Symlink creation logic (fundamental behavior change)
- Error handling strengthening (may expose edge cases)

### High Risk Scenarios to Test
- Existing installations upgrading from old version
- Corrupted/incomplete config directories
- Simultaneous volume and config failures
- Cross-architecture compatibility (arm64 especially)

---

## Acceptance Criteria

All 11 verification tests must pass before merging:
- [ ] Fresh container start
- [ ] Container restart
- [ ] CrowdSec enable/disable
- [ ] Log file permissions
- [ ] LAPI readiness
- [ ] Hub updates
- [ ] Multi-arch compatibility
- [ ] Volume replacement
- [ ] Permission inheritance
- [ ] Config persistence
- [ ] Hub update persistence

---

## References

- Original issue: CrowdSec non-root migration
- Supervisor review: 2024-12-22
- Related files: `Dockerfile`, `.docker/docker-entrypoint.sh`
- Testing environment: Docker 24.x, volumes with uid 1000

---

# Historical Analysis: CrowdSec Reconciliation Failure Diagnostics

## Executive Summary

Investigation of why CrowdSec shows "not started" in the UI when it should **already be enabled**. This is NOT a first-time enable issue—it's a **reconciliation/runtime failure** after container restart or app startup.

---

## Problem Statement

User reports CrowdSec was previously enabled and working, but after container restart:
- UI shows CrowdSec as "not started"
- The setting in database says it should be enabled
- No obvious errors in the UI

---

## 1. Reconciliation Flow Overview

When Charon starts, `ReconcileCrowdSecOnStartup()` runs **asynchronously** (in a goroutine) to restore CrowdSec state.

### Flow Diagram

```
App Startup → go ReconcileCrowdSecOnStartup() → (async goroutine)
                        │
                        ▼
           ┌────────────────────────────────────┐
           │ 1. Validate: db != nil && exec != nil │
           └────────────────────────────────────┘
                        │ (fail → silent return)
                        ▼
           ┌────────────────────────────────────┐
           │ 2. Check: SecurityConfig table exists │
           └────────────────────────────────────┘
                        │ (no table → WARN + return)
                        ▼
           ┌────────────────────────────────────┐
           │ 3. Query: SecurityConfig record       │
           └────────────────────────────────────┘
                        │ (not found → auto-create from Settings)
                        │ (error → return)
                        ▼
           ┌────────────────────────────────────┐
           │ 4. Query: Settings table override     │
           │    key = "security.crowdsec.enabled"  │
           └────────────────────────────────────┘
                        │
                        ▼
           ┌────────────────────────────────────┐
           │ 5. Decide: Start if CrowdSecMode ==   │
           │    "local" OR setting == "true"       │
           └────────────────────────────────────┘
                        │ (both false → INFO skip)
                        ▼
           ┌────────────────────────────────────┐
           │ 6. Validate: Binary exists at path    │
           │    /usr/local/bin/crowdsec            │
           └────────────────────────────────────┘
                        │ (not found → ERROR + return)
                        ▼
           ┌────────────────────────────────────┐
           │ 7. Validate: Config dir exists        │
           │    dataDir/config                     │
           └────────────────────────────────────┘
                        │ (not found → ERROR + return)
                        ▼
           ┌────────────────────────────────────┐
           │ 8. Check: Status (already running?)   │
           └────────────────────────────────────┘
                        │ (running → INFO + done)
                        │ (error → WARN + return!)
                        ▼
           ┌────────────────────────────────────┐
           │ 9. Start: CrowdSec process            │
           └────────────────────────────────────┘
                        │ (error → ERROR + return)
                        ▼
           ┌────────────────────────────────────┐
           │ 10. Verify: Wait 2s + check status    │
           └────────────────────────────────────┘
```

---

## 2. Most Likely Failure Points (Priority Order)

### 2.1 Binary Not Found ⭐ HIGH LIKELIHOOD

**Code:** `backend/internal/services/crowdsec_startup.go:117-120`

```go
if _, err := os.Stat(binPath); os.IsNotExist(err) {
    logger.Log().WithField("path", binPath).Error("CrowdSec reconciliation: binary not found, cannot start")
    return
}
```

**Diagnosis:**
```bash
docker exec <container> ls -la /usr/local/bin/crowdsec
docker exec <container> printenv CHARON_CROWDSEC_BIN
```

---

### 2.2 Config Directory Missing ⭐ HIGH LIKELIHOOD

**Code:** `backend/internal/services/crowdsec_startup.go:122-126`

```go
configPath := filepath.Join(dataDir, "config")
if _, err := os.Stat(configPath); os.IsNotExist(err) {
    logger.Log().WithField("path", configPath).Error("CrowdSec reconciliation: config directory not found")
    return
}
```

**Diagnosis:**
```bash
docker exec <container> ls -la /data/crowdsec/config/
docker exec <container> cat /data/crowdsec/config/config.yaml
```

---

### 2.3 Database State Mismatch ⭐ MEDIUM LIKELIHOOD

Two sources must be checked:
1. `security_configs.crowdsec_mode = "local"`
2. `settings.key = "security.crowdsec.enabled"` with `value = "true"`

If **BOTH** are not "enabled", reconciliation silently skips.

**Diagnosis:**
```bash
docker exec <container> sqlite3 /data/charon.db "SELECT crowdsec_mode, enabled FROM security_configs LIMIT 1;"
docker exec <container> sqlite3 /data/charon.db "SELECT key, value FROM settings WHERE key LIKE '%crowdsec%';"
```

---

### 2.4 Stale PID File (PID Recycled) ⭐ MEDIUM LIKELIHOOD

**Code:** `backend/internal/api/handlers/crowdsec_exec.go:118-147`

Status check reads PID file, checks if process exists, then verifies `/proc/<pid>/cmdline` contains "crowdsec".

**Diagnosis:**
```bash
docker exec <container> cat /data/crowdsec/crowdsec.pid
docker exec <container> pgrep -a crowdsec
```

---

### 2.5 Process Crashes After Start ⭐ MEDIUM LIKELIHOOD

**Code:** `backend/internal/services/crowdsec_startup.go:146-159`

After starting, waits 2 seconds and verifies. If crashed:
```
logger.Log().Error("CrowdSec reconciliation: process started but is no longer running - may have crashed")
```

**Diagnosis:**
```bash
# Try manual start to see errors
docker exec <container> /usr/local/bin/crowdsec -c /data/crowdsec/config/config.yaml

# Check for port conflicts (LAPI uses 8085)
docker exec <container> netstat -tlnp 2>/dev/null | grep 8085
```

---

### 2.6 Status Check Error (Silently Aborts) ⭐ LOW LIKELIHOOD

**Code:** `backend/internal/services/crowdsec_startup.go:129-134`

```go
if err != nil {
    logger.Log().WithError(err).Warn("CrowdSec reconciliation: failed to check status")
    return  // ← Aborts without trying to start!
}
```

---

## 3. Status Handler Analysis

The UI calls `GET /api/v1/admin/crowdsec/status`:

**Code:** `backend/internal/api/handlers/crowdsec_handler.go:313-333`

Returns `running: false` when:
- PID file doesn't exist
- PID doesn't correspond to a running process
- PID is running but `/proc/<pid>/cmdline` doesn't contain "crowdsec"

---

## 4. Diagnostic Commands Summary

```bash
# 1. Check binary
docker exec <container> ls -la /usr/local/bin/crowdsec

# 2. Check config directory
docker exec <container> ls -la /data/crowdsec/config/

# 3. Check database state
docker exec <container> sqlite3 /data/charon.db \
  "SELECT crowdsec_mode, enabled FROM security_configs LIMIT 1;"
docker exec <container> sqlite3 /data/charon.db \
  "SELECT key, value FROM settings WHERE key LIKE '%crowdsec%';"

# 4. Check PID file
docker exec <container> cat /data/crowdsec/crowdsec.pid 2>/dev/null || echo "No PID file"

# 5. Check running processes
docker exec <container> pgrep -a crowdsec || echo "Not running"

# 6. Check logs for reconciliation
docker logs <container> 2>&1 | grep -i "crowdsec reconciliation"

# 7. Try manual start
docker exec <container> /usr/local/bin/crowdsec \
  -c /data/crowdsec/config/config.yaml &

# 8. Check port conflicts
docker exec <container> netstat -tlnp 2>/dev/null | grep -E "8085|8080"
```

---

## 5. Log Messages to Look For

| Priority | Cause | Log Message |
|----------|-------|-------------|
| 1 | Binary missing | `"CrowdSec reconciliation: binary not found"` |
| 2 | Config missing | `"CrowdSec reconciliation: config directory not found"` |
| 3 | DB says disabled | `"CrowdSec reconciliation skipped: both SecurityConfig and Settings indicate disabled"` |
| 4 | Crashed after start | `"process started but is no longer running"` |
| 5 | Start failed | `"CrowdSec reconciliation: FAILED to start CrowdSec"` |
| 6 | Status check failed | `"failed to check status"` |

---

## 6. Key Timeouts

| Operation | Timeout | Location |
|-----------|---------|----------|
| Status check | 5 seconds | crowdsec_startup.go:128 |
| Start timeout | 30 seconds | crowdsec_startup.go:146 |
| Post-start delay | 2 seconds | crowdsec_startup.go:153 |
| Verification check | 5 seconds | crowdsec_startup.go:156 |

---

---

# Original Analysis: First-Time Enable Issues

## Observed Browser Console Errors

```
- 401 Unauthorized on /api/v1/auth/me
- Multiple 400 Bad Request on /api/v1/settings/validate-url
- Auto-logging out due to inactivity
- Various ERR_NETWORK_CHANGED errors
- CrowdSec appears to not be running
```

---

## Relevant Code Files and Flow Analysis

### 2.1 CrowdSec Startup Flow

#### Entry Point: Frontend Toggle

**File:** [frontend/src/pages/Security.tsx](../../../frontend/src/pages/Security.tsx#L147-L183)

```typescript
const crowdsecPowerMutation = useMutation({
  mutationFn: async (enabled: boolean) => {
    await updateSetting('security.crowdsec.enabled', enabled ? 'true' : 'false', 'security', 'bool')
    if (enabled) {
      toast.info('Starting CrowdSec... This may take up to 30 seconds')
      const result = await startCrowdsec()
      const status = await statusCrowdsec()
      if (!status.running) {
        await updateSetting('security.crowdsec.enabled', 'false', 'security', 'bool')
        throw new Error('CrowdSec process failed to start. Check server logs for details.')
      }
      return result
    } else {
      await stopCrowdsec()
      // ...
    }
  },
  // ...
})
```

#### API Client Configuration

**File:** [frontend/src/api/client.ts](../../../frontend/src/api/client.ts#L7-L11)

```typescript
const client = axios.create({
  baseURL: '/api/v1',
  withCredentials: true,
  timeout: 30000, // 30 second timeout
});
```

**Issue Identified:** The frontend has a **30-second timeout**, which aligns with the backend LAPI readiness timeout. However, the startup process involves multiple sequential steps that could exceed this total.

#### Backend Start Handler

**File:** [backend/internal/api/handlers/crowdsec_handler.go](../../../backend/internal/api/handlers/crowdsec_handler.go#L175-L252)

Key timeouts in `Start()`:

- LAPI readiness polling: **30 seconds max** (line 229: `maxWait := 30 * time.Second`)
- Poll interval: **500ms** (line 230: `pollInterval := 500 * time.Millisecond`)
- Individual LAPI check: **2 seconds** (line 237: `context.WithTimeout(ctx, 2*time.Second)`)

```go
// Wait for LAPI to be ready (with timeout)
lapiReady := false
maxWait := 30 * time.Second
pollInterval := 500 * time.Millisecond
deadline := time.Now().Add(maxWait)

for time.Now().Before(deadline) {
    checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
    _, err := h.CmdExec.Execute(checkCtx, "cscli", args...)
    cancel()
    if err == nil {
        lapiReady = true
        break
    }
    time.Sleep(pollInterval)
}
```

#### Backend Executor (Process Management)

**File:** [backend/internal/api/handlers/crowdsec_exec.go](../../../backend/internal/api/handlers/crowdsec_exec.go)

The `DefaultCrowdsecExecutor.Start()` method (lines 39-66):

- Uses `exec.Command` (not `CommandContext`) - process is detached
- Sets `Setpgid: true` to create new process group
- Writes PID file synchronously
- Returns immediately after starting the process

```go
func (e *DefaultCrowdsecExecutor) Start(ctx context.Context, binPath, configDir string) (int, error) {
    configFile := filepath.Join(configDir, "config", "config.yaml")
    cmd := exec.Command(binPath, "-c", configFile)
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setpgid: true, // Create new process group
    }
    // ...
    if err := cmd.Start(); err != nil {
        return 0, err
    }
    // ... writes PID file
    go func() {
        _ = cmd.Wait()
        _ = os.Remove(e.pidFile(configDir))
    }()
    return pid, nil
}
```

#### Background Reconciliation

**File:** [backend/internal/services/crowdsec_startup.go](../../../backend/internal/services/crowdsec_startup.go)

Key timeouts in `ReconcileCrowdSecOnStartup()`:

- Status check timeout: **5 seconds** (line 139)
- Start timeout: **30 seconds** (line 150)
- Verification delay: **2 seconds** (line 159: `time.Sleep(2 * time.Second)`)
- Verification check timeout: **5 seconds** (line 161)

```go
// Start context with 30 second timeout
startCtx, startCancel := context.WithTimeout(context.Background(), 30*time.Second)
defer startCancel()

newPid, err := executor.Start(startCtx, binPath, dataDir)
// ...

// VERIFY: Wait briefly and confirm process is actually running
time.Sleep(2 * time.Second)

verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 5*time.Second)
defer verifyCancel()
```

---

## 3. Identified Potential Root Causes

### 3.1 Timeout Race Condition (HIGH PROBABILITY)

The frontend timeout (30s) and backend LAPI polling timeout (30s) are identical. Combined with:

- Initial process start time
- Settings database update
- SecurityConfig database update
- Network latency

**Total time could easily exceed 30 seconds**, causing the frontend to timeout before the backend responds.

### 3.2 CrowdSec Binary/Config Not Found

In [crowdsec_startup.go](../../../backend/internal/services/crowdsec_startup.go#L124-L135):

```go
// VALIDATE: Ensure binary exists
if _, err := os.Stat(binPath); os.IsNotExist(err) {
    logger.Log().WithField("path", binPath).Error("CrowdSec reconciliation: binary not found")
    return
}

// VALIDATE: Ensure config directory exists
configPath := filepath.Join(dataDir, "config")
if _, err := os.Stat(configPath); os.IsNotExist(err) {
    logger.Log().WithField("path", configPath).Error("CrowdSec reconciliation: config directory not found")
    return
}
```

**Check:** The binary path defaults to `/usr/local/bin/crowdsec` (from `routes.go` line 292) and config dir is `data/crowdsec`. If either is missing, the startup silently fails.

### 3.3 LAPI Never Becomes Ready

The handler waits for `cscli lapi status` to succeed. If CrowdSec starts but LAPI never initializes (e.g., database issues, missing configuration), the handler will timeout.

### 3.4 Authentication Issues (401 on /auth/me)

The 401 errors suggest the user's session is expiring during the long-running operation. This is likely a **symptom, not the cause**:

**File:** [frontend/src/api/client.ts](../../../frontend/src/api/client.ts#L25-L33)

```typescript
client.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      console.warn('Authentication failed:', error.config?.url);
    }
    return Promise.reject(error);
  }
);
```

The session timeout or network interruption during the 30+ second CrowdSec startup could cause parallel requests to `/auth/me` to fail.

### 3.5 ERR_NETWORK_CHANGED

This indicates network connectivity issues on the client side. If the network changes during the long-running request, it will fail. This is external to the application but exacerbated by long timeouts.

---

## 4. Configuration Defaults

| Setting | Default Value | Source |
|---------|---------------|--------|
| CrowdSec Binary | `/usr/local/bin/crowdsec` | `CHARON_CROWDSEC_BIN` env or hardcoded |
| CrowdSec Config Dir | `data/crowdsec` | `CHARON_CROWDSEC_CONFIG_DIR` env |
| CrowdSec Mode | `disabled` | `CERBERUS_SECURITY_CROWDSEC_MODE` env |
| Frontend Timeout | 30 seconds | `client.ts` |
| LAPI Wait Timeout | 30 seconds | `crowdsec_handler.go` |
| Process Start Timeout | 30 seconds | `crowdsec_startup.go` |

---

## 5. Remediation Plan

### Phase 1: Immediate Fixes (Timeout Handling)

#### 5.1.1 Increase Frontend Timeout for CrowdSec Operations

**File:** `frontend/src/api/crowdsec.ts`

Create a dedicated request with extended timeout for CrowdSec start:

```typescript
export async function startCrowdsec(): Promise<{ status: string; pid: number; lapi_ready?: boolean }> {
  const resp = await client.post('/admin/crowdsec/start', {}, {
    timeout: 60000, // 60 second timeout for startup operations
  })
  return resp.data
}
```

#### 5.1.2 Add Progress/Status Feedback

Implement polling-based status check instead of waiting for single long request:

1. Backend: Return immediately after starting process, with status "starting"
2. Frontend: Poll status endpoint until "running" or timeout

#### 5.1.3 Improve Error Messages

**File:** `backend/internal/api/handlers/crowdsec_handler.go`

Add detailed error responses:

```go
if !lapiReady {
    logger.Log().WithField("pid", pid).Warn("CrowdSec started but LAPI not ready within timeout")
    c.JSON(http.StatusOK, gin.H{
        "status":     "started",
        "pid":        pid,
        "lapi_ready": false,
        "warning":    "Process started but LAPI initialization may take additional time",
        "next_step":  "Poll /admin/crowdsec/status until lapi_ready is true",
    })
    return
}
```

### Phase 2: Diagnostic Improvements

#### 5.2.1 Add Health Check Endpoint

Create `/admin/crowdsec/health` that returns:

- Binary path and existence check
- Config directory and existence check
- Process status
- LAPI status
- Last error (if any)

#### 5.2.2 Enhanced Logging

Add structured logging for all CrowdSec operations with correlation IDs.

### Phase 3: Long-term Fixes

#### 5.3.1 Async Startup Pattern

Convert to async pattern:

1. `POST /admin/crowdsec/start` returns immediately with job ID
2. `GET /admin/crowdsec/jobs/{id}` returns job status
3. Frontend polls job status with exponential backoff

#### 5.3.2 WebSocket Status Updates

Use existing WebSocket infrastructure to push status updates during startup.

---

## 6. Diagnostic Commands

To investigate the issue on the running container:

```bash
# Check if CrowdSec binary exists
ls -la /usr/local/bin/crowdsec

# Check CrowdSec config directory
ls -la /app/data/crowdsec/config/

# Check if CrowdSec is running
pgrep -f crowdsec
ps aux | grep crowdsec

# Check CrowdSec logs (if running)
cat /var/log/crowdsec.log

# Test LAPI status
cscli lapi status

# Check PID file
cat /app/data/crowdsec/crowdsec.pid

# Check database for CrowdSec settings
sqlite3 /app/data/charon.db "SELECT * FROM settings WHERE key LIKE '%crowdsec%';"
sqlite3 /app/data/charon.db "SELECT * FROM security_configs;"
```

---

## 7. Summary

| Issue | Probability | Impact | Fix Complexity |
|-------|-------------|--------|----------------|
| Timeout race condition | HIGH | Startup fails | Low |
| Missing binary/config | MEDIUM | Startup fails silently | Low |
| LAPI initialization slow | MEDIUM | Timeout | Medium |
| Session expiry during startup | LOW | User sees 401 | Low |
| Network instability | LOW | Request fails | N/A (external) |

**Recommended Immediate Action:** Increase frontend timeout for CrowdSec start operations to 60 seconds and add polling-based status verification.

---

## 8. Files to Modify

| File | Change |
|------|--------|
| `frontend/src/api/crowdsec.ts` | Extend timeout for start operation |
| `frontend/src/pages/Security.tsx` | Add polling for status after start |
| `backend/internal/api/handlers/crowdsec_handler.go` | Return partial success, add health endpoint |
| `backend/internal/services/crowdsec_startup.go` | Add more diagnostic logging |

---

*Investigation completed: December 22, 2025*
*Author: GitHub Copilot (Research Mode)*
