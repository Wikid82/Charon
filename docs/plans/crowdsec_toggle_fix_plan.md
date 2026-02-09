# CrowdSec Toggle Integration Fix Plan

**Date**: December 15, 2025
**Issue**: CrowdSec toggle stuck ON, reconciliation silently exits, process not starting
**Root Cause**: Database disconnect between frontend (Settings table) and reconciliation (SecurityConfig table)

---

## Executive Summary

The CrowdSec toggle shows "ON" but the process is NOT running. The reconciliation function silently exits without starting CrowdSec because:

1. **Frontend writes to Settings table** (`security.crowdsec.enabled`)
2. **Backend reconciliation reads from SecurityConfig table** (`crowdsec_mode = "local"`)
3. **No synchronization** between the two tables
4. **Auto-initialization code EXISTS** (lines 46-71 in crowdsec_startup.go) but creates config with `crowdsec_mode = "disabled"`
5. **Reconciliation sees "disabled"** and exits silently with no logs

---

## Root Cause Analysis (DETAILED)

### Evidence Trail

**Container Logs Show Silent Exit**:

```
{"bin_path":"crowdsec","data_dir":"/app/data/crowdsec","level":"info","msg":"CrowdSec reconciliation: starting startup check","time":"2025-12-14T23:32:33-05:00"}
[NO FURTHER LOGS - Function exited here]
```

**Database State on Fresh Start**:

```
SELECT * FROM security_configs → record not found
{"level":"info","msg":"CrowdSec reconciliation: no SecurityConfig found, creating default config"}
```

**Process Check**:

```bash
$ docker exec charon ps aux | grep -i crowdsec
[NO RESULTS - Process not running]
```

### Why Reconciliation Exits Silently

**FILE**: `backend/internal/services/crowdsec_startup.go`

**Execution Flow**:

```
1. User clicks toggle ON in Security.tsx
2. Frontend calls updateSetting('security.crowdsec.enabled', 'true')
3. Settings table updated → security.crowdsec.enabled = "true"
4. Frontend calls startCrowdsec() → Handler updates SecurityConfig
5. CrowdSec starts successfully, toggle shows ON
6. Container restarts (docker restart or reboot)
7. ReconcileCrowdSecOnStartup() executes at line 26:

   Line 44: db.First(&cfg) → returns gorm.ErrRecordNotFound

   Lines 46-71: Auto-initialization block executes:
     - Creates SecurityConfig with crowdsec_mode = "disabled"
     - Logs "default SecurityConfig created successfully"
     - Returns early (line 70) WITHOUT checking Settings table
     - CrowdSec is NEVER started

   Result: Toggle shows "ON" (Settings table), but process is "OFF" (not running)
```

**THE BUG (Lines 46-71)**:

```go
if err == gorm.ErrRecordNotFound {
    // AUTO-INITIALIZE: Create default SecurityConfig on first startup
    logger.Log().Info("CrowdSec reconciliation: no SecurityConfig found, creating default config")

    defaultCfg := models.SecurityConfig{
        UUID:             "default",
        Name:             "Default Security Config",
        Enabled:          false,
        CrowdSecMode:     "disabled",  // ← PROBLEM: Ignores Settings table state
        WAFMode:          "disabled",
        WAFParanoiaLevel: 1,
        RateLimitMode:    "disabled",
        RateLimitBurst:   10,
        RateLimitRequests: 100,
        RateLimitWindowSec: 60,
    }

    if err := db.Create(&defaultCfg).Error; err != nil {
        logger.Log().WithError(err).Error("CrowdSec reconciliation: failed to create default SecurityConfig")
        return
    }

    logger.Log().Info("CrowdSec reconciliation: default SecurityConfig created successfully")
    // Don't start CrowdSec on fresh install - user must enable via UI
    return  // ← EXITS WITHOUT checking Settings table or starting process
}
```

**Why This Causes the Issue**:

1. **First Container Start**: User enables CrowdSec via toggle
   - Settings: `security.crowdsec.enabled = "true"` ✅
   - SecurityConfig: `crowdsec_mode = "local"` ✅ (via Start handler)
   - Process: Running ✅

2. **Container Restart**: Database persists but SecurityConfig table may be empty (migration issue or corruption)
   - Reconciliation runs
   - SecurityConfig table: **EMPTY** (record lost or never migrated)
   - Auto-init creates SecurityConfig with `crowdsec_mode = "disabled"`
   - Returns early without checking Settings table
   - Settings: Still shows `"true"` (UI says ON)
   - SecurityConfig: Says `"disabled"` (reconciliation source)
   - Process: NOT started ❌

3. **Result**: **State Mismatch**
   - Frontend toggle: **ON** (reads Settings table)
   - Backend reconciliation: **OFF** (reads SecurityConfig table)
   - Process: **NOT RUNNING** (reconciliation didn't start it)

---

## Current Code Analysis

### 1. Reconciliation Function (crowdsec_startup.go)

**Location**: `backend/internal/services/crowdsec_startup.go`

**Lines 44-71 (Auto-initialization - THE BUG)**:

```go
var cfg models.SecurityConfig
if err := db.First(&cfg).Error; err != nil {
    if err == gorm.ErrRecordNotFound {
        // AUTO-INITIALIZE: Create default SecurityConfig on first startup
        logger.Log().Info("CrowdSec reconciliation: no SecurityConfig found, creating default config")

        defaultCfg := models.SecurityConfig{
            UUID:             "default",
            Name:             "Default Security Config",
            Enabled:          false,
            CrowdSecMode:     "disabled",  // ← IGNORES Settings table
            WAFMode:          "disabled",
            WAFParanoiaLevel: 1,
            RateLimitMode:    "disabled",
            RateLimitBurst:   10,
            RateLimitRequests: 100,
            RateLimitWindowSec: 60,
        }

        if err := db.Create(&defaultCfg).Error; err != nil {
            logger.Log().WithError(err).Error("CrowdSec reconciliation: failed to create default SecurityConfig")
            return
        }

        logger.Log().Info("CrowdSec reconciliation: default SecurityConfig created successfully")
        // Don't start CrowdSec on fresh install - user must enable via UI
        return  // ← EARLY EXIT - Never checks Settings table
    }
    logger.Log().WithError(err).Warn("CrowdSec reconciliation: failed to read SecurityConfig")
    return
}
```

**Lines 74-90 (Runtime Setting Override - UNREACHABLE after auto-init)**:

```go
// Also check for runtime setting override in settings table
var settingOverride struct{ Value string }
crowdSecEnabled := false
if err := db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.enabled").Scan(&settingOverride).Error; err == nil && settingOverride.Value != "" {
    crowdSecEnabled = strings.EqualFold(settingOverride.Value, "true")
    logger.Log().WithFields(map[string]interface{}{
        "setting_value":    settingOverride.Value,
        "crowdsec_enabled": crowdSecEnabled,
    }).Debug("CrowdSec reconciliation: found runtime setting override")
}
```

**This code is NEVER REACHED** when SecurityConfig doesn't exist because line 70 returns early!

**Lines 91-98 (Decision Logic)**:

```go
// Only auto-start if CrowdSecMode is "local" OR runtime setting is enabled
if cfg.CrowdSecMode != "local" && !crowdSecEnabled {
    logger.Log().WithFields(map[string]interface{}{
        "db_mode":         cfg.CrowdSecMode,
        "setting_enabled": crowdSecEnabled,
    }).Debug("CrowdSec reconciliation skipped: mode is not 'local' and setting not enabled")
    return
}
```

**Also UNREACHABLE** during auto-init scenario!

### 2. Start Handler (crowdsec_handler.go)

**Location**: `backend/internal/api/handlers/crowdsec_handler.go`

**Lines 167-192 - CORRECT IMPLEMENTATION**:

```go
func (h *CrowdsecHandler) Start(c *gin.Context) {
    ctx := c.Request.Context()

    // UPDATE SecurityConfig to persist user's intent
    var cfg models.SecurityConfig
    if err := h.DB.First(&cfg).Error; err != nil {
        if err == gorm.ErrRecordNotFound {
            // Create default config with CrowdSec enabled
            cfg = models.SecurityConfig{
                UUID:         "default",
                Name:         "Default Security Config",
                Enabled:      true,
                CrowdSecMode: "local",  // ← CORRECT: Sets mode to "local"
            }
            if err := h.DB.Create(&cfg).Error; err != nil {
                logger.Log().WithError(err).Error("Failed to create SecurityConfig")
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to persist configuration"})
                return
            }
        } else {
            logger.Log().WithError(err).Error("Failed to read SecurityConfig")
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read configuration"})
            return
        }
    } else {
        // Update existing config
        cfg.CrowdSecMode = "local"
        cfg.Enabled = true
        if err := h.DB.Save(&cfg).Error; err != nil {
            logger.Log().WithError(err).Error("Failed to update SecurityConfig")
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to persist configuration"})
            return
        }
    }

    // Start the process...
}
```

**Analysis**: This is CORRECT. The Start handler properly updates SecurityConfig when user clicks "Start" from the CrowdSec config page (/security/crowdsec).

### 3. Frontend Toggle (Security.tsx)

**Location**: `frontend/src/pages/Security.tsx`

**Lines 64-120 - THE DISCONNECT**:

```tsx
const crowdsecPowerMutation = useMutation({
  mutationFn: async (enabled: boolean) => {
    // Step 1: Update Settings table
    await updateSetting('security.crowdsec.enabled', enabled ? 'true' : 'false', 'security', 'bool')

    if (enabled) {
      // Step 2: Call Start() which updates SecurityConfig
      const result = await startCrowdsec()

      // Step 3: Verify running
      const status = await statusCrowdsec()
      if (!status.running) {
        await updateSetting('security.crowdsec.enabled', 'false', 'security', 'bool')
        throw new Error('CrowdSec process failed to start')
      }

      return result
    } else {
      // Step 2: Call Stop() which DOES NOT update SecurityConfig!
      await stopCrowdsec()

      // Step 3: Verify stopped
      await new Promise(resolve => setTimeout(resolve, 500))
      const status = await statusCrowdsec()
      if (status.running) {
        throw new Error('CrowdSec process still running')
      }

      return { enabled: false }
    }
  },
})
```

**Analysis**:

- **Enable Path**: Updates Settings → Calls Start() → Start() updates SecurityConfig → ✅ Both tables synced
- **Disable Path**: Updates Settings → Calls Stop() → Stop() **does NOT always update SecurityConfig** → ❌ Tables out of sync

Looking at the Stop handler:

```go
func (h *CrowdsecHandler) Stop(c *gin.Context) {
    ctx := c.Request.Context()
    if err := h.Executor.Stop(ctx, h.DataDir); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // UPDATE SecurityConfig to persist user's intent
    var cfg models.SecurityConfig
    if err := h.DB.First(&cfg).Error; err == nil {
        cfg.CrowdSecMode = "disabled"
        cfg.Enabled = false
        if err := h.DB.Save(&cfg).Error; err != nil {
            logger.Log().WithError(err).Warn("Failed to update SecurityConfig after stopping CrowdSec")
        }
    }

    c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}
```

**This IS CORRECT** - Stop() handler updates SecurityConfig when it can find it. BUT:

**Scenario Where It Fails**:

1. SecurityConfig table gets corrupted/cleared/migrated incorrectly
2. User clicks toggle OFF
3. Stop() tries to update SecurityConfig → record not found → skips update
4. Settings table still updated to "false"
5. Container restarts → auto-init creates SecurityConfig with "disabled"
6. Both tables say "disabled" but UI might show stale state

---

## Comprehensive Fix Strategy

### Phase 1: Fix Auto-Initialization (CRITICAL - IMMEDIATE)

**FILE**: `backend/internal/services/crowdsec_startup.go`

**CHANGE**: Lines 46-71 (auto-initialization block)

**AFTER** (with Settings table check):

```go
if err == gorm.ErrRecordNotFound {
    // AUTO-INITIALIZE: Create default SecurityConfig by checking Settings table
    logger.Log().Info("CrowdSec reconciliation: no SecurityConfig found, checking Settings table for user preference")

    // Check if user has already enabled CrowdSec via Settings table (from toggle or legacy config)
    var settingOverride struct{ Value string }
    crowdSecEnabledInSettings := false
    if err := db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.enabled").Scan(&settingOverride).Error; err == nil && settingOverride.Value != "" {
        crowdSecEnabledInSettings = strings.EqualFold(settingOverride.Value, "true")
        logger.Log().WithFields(map[string]interface{}{
            "setting_value": settingOverride.Value,
            "enabled":       crowdSecEnabledInSettings,
        }).Info("CrowdSec reconciliation: found existing Settings table preference")
    }

    // Create SecurityConfig that matches Settings table state
    crowdSecMode := "disabled"
    if crowdSecEnabledInSettings {
        crowdSecMode = "local"
    }

    defaultCfg := models.SecurityConfig{
        UUID:               "default",
        Name:               "Default Security Config",
        Enabled:            crowdSecEnabledInSettings,
        CrowdSecMode:       crowdSecMode,  // ← NOW RESPECTS Settings table
        WAFMode:            "disabled",
        WAFParanoiaLevel:   1,
        RateLimitMode:      "disabled",
        RateLimitBurst:     10,
        RateLimitRequests:  100,
        RateLimitWindowSec: 60,
    }

    if err := db.Create(&defaultCfg).Error; err != nil {
        logger.Log().WithError(err).Error("CrowdSec reconciliation: failed to create default SecurityConfig")
        return
    }

    logger.Log().WithFields(map[string]interface{}{
        "crowdsec_mode": defaultCfg.CrowdSecMode,
        "enabled":       defaultCfg.Enabled,
        "source":        "settings_table",
    }).Info("CrowdSec reconciliation: default SecurityConfig created from Settings preference")

    // Continue to process the config (DON'T return early)
    cfg = defaultCfg
}
```

**KEY CHANGES**:

1. **Check Settings table** during auto-initialization
2. **Create SecurityConfig matching Settings state** (not hardcoded "disabled")
3. **Don't return early** - let the rest of the function process the config
4. **Assign to cfg variable** so flow continues to line 74+

### Phase 2: Enhance Logging (IMMEDIATE)

**FILE**: `backend/internal/services/crowdsec_startup.go`

**CHANGE**: Lines 91-98 (decision logic - better logging)

**AFTER**:

```go
// Start when EITHER SecurityConfig has mode="local" OR Settings table has enabled=true
// Exit only when BOTH are disabled
if cfg.CrowdSecMode != "local" && !crowdSecEnabled {
    logger.Log().WithFields(map[string]interface{}{
        "db_mode":         cfg.CrowdSecMode,
        "setting_enabled": crowdSecEnabled,
    }).Info("CrowdSec reconciliation skipped: both SecurityConfig and Settings indicate disabled")
    return
}

// Log which source triggered the start
if cfg.CrowdSecMode == "local" {
    logger.Log().WithField("mode", cfg.CrowdSecMode).Info("CrowdSec reconciliation: starting based on SecurityConfig mode='local'")
} else if crowdSecEnabled {
    logger.Log().WithField("setting", "true").Info("CrowdSec reconciliation: starting based on Settings table override")
}
```

**KEY CHANGES**:

1. **Change log level** from Debug to Info (so we see it in logs)
2. **Add source attribution** (which table triggered the start)
3. **Clarify condition** (exit only when BOTH are disabled)

### Phase 3: Add Unified Toggle Endpoint (OPTIONAL BUT RECOMMENDED)

**WHY**: Currently the toggle updates Settings, then calls Start/Stop which updates SecurityConfig. This creates potential race conditions. A unified endpoint is safer.

**FILE**: `backend/internal/api/handlers/crowdsec_handler.go`

**ADD**: New method (after Stop(), around line 260)

```go
// ToggleCrowdSec enables or disables CrowdSec, synchronizing Settings and SecurityConfig atomically
func (h *CrowdsecHandler) ToggleCrowdSec(c *gin.Context) {
    var payload struct {
        Enabled bool `json:"enabled"`
    }
    if err := c.ShouldBindJSON(&payload); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
        return
    }

    logger.Log().WithField("enabled", payload.Enabled).Info("CrowdSec toggle: received request")

    // Use a transaction to ensure Settings and SecurityConfig stay in sync
    tx := h.DB.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
        }
    }()

    // STEP 1: Update Settings table
    settingKey := "security.crowdsec.enabled"
    settingValue := "false"
    if payload.Enabled {
        settingValue = "true"
    }

    var settingModel models.Setting
    if err := tx.Where("key = ?", settingKey).FirstOrCreate(&settingModel, models.Setting{
        Key:      settingKey,
        Value:    settingValue,
        Type:     "bool",
        Category: "security",
    }).Error; err != nil {
        tx.Rollback()
        logger.Log().WithError(err).Error("CrowdSec toggle: failed to update Settings table")
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update settings"})
        return
    }
    settingModel.Value = settingValue
    if err := tx.Save(&settingModel).Error; err != nil {
        tx.Rollback()
        logger.Log().WithError(err).Error("CrowdSec toggle: failed to save Settings table")
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update settings"})
        return
    }

    // STEP 2: Update SecurityConfig table
    var cfg models.SecurityConfig
    if err := tx.First(&cfg).Error; err != nil {
        if err == gorm.ErrRecordNotFound {
            // Create config matching toggle state
            crowdSecMode := "disabled"
            if payload.Enabled {
                crowdSecMode = "local"
            }

            cfg = models.SecurityConfig{
                UUID:               "default",
                Name:               "Default Security Config",
                Enabled:            payload.Enabled,
                CrowdSecMode:       crowdSecMode,
                WAFMode:            "disabled",
                WAFParanoiaLevel:   1,
                RateLimitMode:      "disabled",
                RateLimitBurst:     10,
                RateLimitRequests:  100,
                RateLimitWindowSec: 60,
            }
            if err := tx.Create(&cfg).Error; err != nil {
                tx.Rollback()
                logger.Log().WithError(err).Error("CrowdSec toggle: failed to create SecurityConfig")
                c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist configuration"})
                return
            }
        } else {
            tx.Rollback()
            logger.Log().WithError(err).Error("CrowdSec toggle: failed to read SecurityConfig")
            c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read configuration"})
            return
        }
    } else {
        // Update existing config
        if payload.Enabled {
            cfg.CrowdSecMode = "local"
            cfg.Enabled = true
        } else {
            cfg.CrowdSecMode = "disabled"
            cfg.Enabled = false
        }
        if err := tx.Save(&cfg).Error; err != nil {
            tx.Rollback()
            logger.Log().WithError(err).Error("CrowdSec toggle: failed to update SecurityConfig")
            c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist configuration"})
            return
        }
    }

    // Commit the transaction before starting/stopping process
    if err := tx.Commit().Error; err != nil {
        logger.Log().WithError(err).Error("CrowdSec toggle: transaction commit failed")
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit changes"})
        return
    }

    logger.Log().WithFields(map[string]interface{}{
        "enabled":       cfg.Enabled,
        "crowdsec_mode": cfg.CrowdSecMode,
    }).Info("CrowdSec toggle: synchronized Settings and SecurityConfig successfully")

    // STEP 3: Start or stop the process
    ctx := c.Request.Context()
    if payload.Enabled {
        // Start CrowdSec
        pid, err := h.Executor.Start(ctx, h.BinPath, h.DataDir)
        if err != nil {
            logger.Log().WithError(err).Error("CrowdSec toggle: failed to start process, reverting DB changes")

            // Revert both tables (in new transaction)
            revertTx := h.DB.Begin()
            cfg.CrowdSecMode = "disabled"
            cfg.Enabled = false
            revertTx.Save(&cfg)
            settingModel.Value = "false"
            revertTx.Save(&settingModel)
            revertTx.Commit()

            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }

        // Wait for LAPI readiness
        lapiReady := false
        maxWait := 30 * time.Second
        pollInterval := 500 * time.Millisecond
        deadline := time.Now().Add(maxWait)

        for time.Now().Before(deadline) {
            args := []string{"lapi", "status"}
            if _, err := os.Stat(filepath.Join(h.DataDir, "config.yaml")); err == nil {
                args = append([]string{"-c", filepath.Join(h.DataDir, "config.yaml")}, args...)
            }

            checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
            _, err := h.CmdExec.Execute(checkCtx, "cscli", args...)
            cancel()

            if err == nil {
                lapiReady = true
                break
            }

            time.Sleep(pollInterval)
        }

        logger.Log().WithFields(map[string]interface{}{
            "pid":        pid,
            "lapi_ready": lapiReady,
        }).Info("CrowdSec toggle: started successfully")

        c.JSON(http.StatusOK, gin.H{
            "enabled":    true,
            "pid":        pid,
            "lapi_ready": lapiReady,
        })
        return
    } else {
        // Stop CrowdSec
        if err := h.Executor.Stop(ctx, h.DataDir); err != nil {
            logger.Log().WithError(err).Error("CrowdSec toggle: failed to stop process")
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }

        logger.Log().Info("CrowdSec toggle: stopped successfully")
        c.JSON(http.StatusOK, gin.H{"enabled": false})
        return
    }
}
```

**Register Route**:

```go
// In RegisterRoutes() method
rg.POST("/admin/crowdsec/toggle", h.ToggleCrowdSec)
```

**Frontend API Client** (`frontend/src/api/crowdsec.ts`):

```typescript
export async function toggleCrowdsec(enabled: boolean): Promise<{ enabled: boolean; pid?: number; lapi_ready?: boolean }> {
  const response = await client.post('/admin/crowdsec/toggle', { enabled })
  return response.data
}
```

**Frontend Toggle Update** (`frontend/src/pages/Security.tsx`):

```tsx
const crowdsecPowerMutation = useMutation({
  mutationFn: async (enabled: boolean) => {
    if (enabled) {
      toast.info('Starting CrowdSec... This may take up to 30 seconds')
    }

    // Use unified toggle endpoint (handles Settings + SecurityConfig + Process)
    const result = await toggleCrowdsec(enabled)

    // Backend already verified state, just do final status check
    const status = await statusCrowdsec()
    if (enabled && !status.running) {
      throw new Error('CrowdSec process failed to start. Check server logs for details.')
    }
    if (!enabled && status.running) {
      throw new Error('CrowdSec process still running. Check server logs for details.')
    }

    return result
  },
  // ... rest remains the same
})
```

---

## Testing Plan

### Test 1: Fresh Install

**Scenario**: Brand new Charon installation

1. Start container: `docker compose up -d`
2. Navigate to Security page
3. Verify CrowdSec toggle shows OFF
4. Check status: `curl http://localhost:8080/api/v1/admin/crowdsec/status`
   - Expected: `{"running": false}`
5. Check logs: `docker logs charon 2>&1 | grep "reconciliation"`
   - Expected: "no SecurityConfig found, checking Settings table"
   - Expected: "default SecurityConfig created from Settings preference"
   - Expected: "crowdsec_mode: disabled"

### Test 2: Toggle ON → Container Restart

**Scenario**: User enables CrowdSec, then restarts container

1. Enable toggle in UI (click ON)
2. Verify CrowdSec starts
3. Check status: `{"running": true, "pid": xxx}`
4. Restart: `docker restart charon`
5. Wait 10 seconds
6. Check status again: `{"running": true, "pid": xxx}` (NEW PID)
7. Check logs:
   - Expected: "starting based on SecurityConfig mode='local'"

### Test 3: Legacy Migration (Settings Table Only)

**Scenario**: Existing install with Settings table but no SecurityConfig

1. Manually set: `INSERT INTO settings (key, value, type, category) VALUES ('security.crowdsec.enabled', 'true', 'bool', 'security');`
2. Delete SecurityConfig: `DELETE FROM security_configs;`
3. Restart container
4. Check logs:
   - Expected: "found existing Settings table preference"
   - Expected: "default SecurityConfig created from Settings preference"
   - Expected: "crowdsec_mode: local"
5. Check status: `{"running": true}`

### Test 4: Toggle OFF → Container Restart

**Scenario**: User disables CrowdSec, then restarts container

1. Start with CrowdSec enabled and running
2. Click toggle OFF in UI
3. Verify process stops
4. Restart: `docker restart charon`
5. Wait 10 seconds
6. Check status: `{"running": false}`
7. Verify toggle still shows OFF

### Test 5: Corrupted SecurityConfig Recovery

**Scenario**: SecurityConfig gets deleted but Settings exists

1. Enable CrowdSec via UI
2. Manually delete SecurityConfig: `DELETE FROM security_configs;`
3. Restart container
4. Verify auto-init recreates SecurityConfig matching Settings table
5. Verify CrowdSec auto-starts

---

## Verification Checklist

### Phase 1 (Auto-Initialization Fix)

- [ ] Modified `crowdsec_startup.go` lines 46-71
- [ ] Auto-init checks Settings table for existing preference
- [ ] Auto-init creates SecurityConfig matching Settings state
- [ ] Auto-init does NOT return early (continues to line 74+)
- [ ] Test 1 (Fresh Install) passes
- [ ] Test 3 (Legacy Migration) passes

### Phase 2 (Logging Enhancement)

- [ ] Modified `crowdsec_startup.go` lines 91-98
- [ ] Changed log level from Debug to Info
- [ ] Added source attribution logging
- [ ] Test 2 (Toggle ON → Restart) shows correct log
- [ ] Test 4 (Toggle OFF → Restart) shows correct log

### Phase 3 (Unified Toggle - Optional)

- [ ] Added `ToggleCrowdSec()` method to `crowdsec_handler.go`
- [ ] Registered `/admin/crowdsec/toggle` route
- [ ] Added `toggleCrowdsec()` to `crowdsec.ts`
- [ ] Updated `crowdsecPowerMutation` in `Security.tsx`
- [ ] Test 4 (Toggle synchronization) passes
- [ ] Test 5 (Corrupted recovery) passes

### Pre-Deployment

- [ ] Pre-commit linters pass: `pre-commit run --all-files`
- [ ] Backend tests pass: `cd backend && go test ./...`
- [ ] Frontend tests pass: `cd frontend && npm run test`
- [ ] Docker build succeeds: `docker build -t charon:local .`
- [ ] Integration test passes: `scripts/crowdsec_integration.sh`

---

## Success Criteria

✅ **Fix is complete when**:

1. Toggle shows correct state (ON = running, OFF = stopped)
2. Toggle persists across container restarts
3. Reconciliation logs clearly show decision reason
4. Auto-initialization respects Settings table preference
5. No "stuck toggle" scenarios
6. All 5 test cases pass
7. Pre-commit checks pass
8. No regressions in existing CrowdSec functionality

---

## Risk Assessment

| Change | Risk Level | Mitigation |
|--------|------------|------------|
| Phase 1 (Auto-init) | **Low** | Only affects fresh installs or corrupted state recovery |
| Phase 2 (Logging) | **Very Low** | Only changes log output, no logic changes |
| Phase 3 (Unified toggle) | **Medium** | New endpoint, requires thorough testing, but backward compatible |

---

## Rollback Plan

If issues arise:

1. **Immediate Revert**: `git revert <commit-hash>` (no DB changes needed)
2. **Manual Fix** (if toggle stuck):

   ```sql
   -- Reset SecurityConfig
   UPDATE security_configs
   SET crowdsec_mode = 'disabled', enabled = 0
   WHERE uuid = 'default';

   -- Reset Settings
   UPDATE settings
   SET value = 'false'
   WHERE key = 'security.crowdsec.enabled';
   ```

3. **Force Stop CrowdSec**: `docker exec charon pkill -SIGTERM crowdsec`

---

## Dependency Impact Analysis

### Phase 1: Auto-Initialization Changes (crowdsec_startup.go)

#### Files Directly Modified

- `backend/internal/services/crowdsec_startup.go` (lines 46-71)

#### Dependencies and Required Updates

**1. Unit Tests - MUST BE UPDATED**

- **File**: `backend/internal/services/crowdsec_startup_test.go`
- **Impact**: Test `TestReconcileCrowdSecOnStartup_NoSecurityConfig` expects the function to skip/return early when no SecurityConfig exists
- **Required Change**: Update test to:
  - Create a Settings table entry with `security.crowdsec.enabled = 'true'`
  - Verify that SecurityConfig is auto-created with `crowdsec_mode = "local"`
  - Verify that CrowdSec process is started (not skipped)
- **Additional Tests Needed**:
  - `TestReconcileCrowdSecOnStartup_NoSecurityConfig_SettingsDisabled` - Settings='false' → creates config with mode="disabled", does NOT start
  - `TestReconcileCrowdSecOnStartup_NoSecurityConfig_SettingsEnabled` - Settings='true' → creates config with mode="local", DOES start
  - `TestReconcileCrowdSecOnStartup_NoSecurityConfig_NoSettingsEntry` - No Settings entry → creates config with mode="disabled", does NOT start

**2. Integration Tests - VERIFICATION NEEDED**

- **Files**:
  - `scripts/crowdsec_integration.sh`
  - `scripts/crowdsec_startup_test.sh`
  - `scripts/crowdsec_decision_integration.sh`
- **Impact**: These scripts may assume specific startup behavior
- **Verification Required**:
  - Do any scripts pre-populate Settings table?
  - Do any scripts expect reconciliation to skip on fresh DB?
  - Do any scripts verify log output from reconciliation?
- **Action**: Review scripts for assumptions about auto-initialization behavior

**3. Migration/Upgrade Path - DATABASE CONCERN**

- **Scenario**: Existing installations with Settings='true' but missing SecurityConfig
- **Impact**: After upgrade, reconciliation will auto-create SecurityConfig from Settings (POSITIVE)
- **Risk**: Low - this is the intended fix
- **Documentation**: Should document this as expected behavior in migration guide

**4. Models - NO CHANGES REQUIRED**

- **File**: `backend/internal/models/security_config.go`
- **Analysis**: SecurityConfig model structure unchanged
- **File**: `backend/internal/models/setting.go`
- **Analysis**: Setting model structure unchanged

**5. Route Registration - NO CHANGES REQUIRED**

- **File**: `backend/internal/api/routes/routes.go` (line 360)
- **Analysis**: Already calls `ReconcileCrowdSecOnStartup`, no signature changes

**6. Handler Dependencies - NO CHANGES REQUIRED**

- **File**: `backend/internal/api/handlers/crowdsec_handler.go`
- **Analysis**: Start/Stop handlers operate independently, no coupling to reconciliation logic

### Phase 2: Logging Enhancement Changes (crowdsec_startup.go)

#### Files Directly Modified

- `backend/internal/services/crowdsec_startup.go` (lines 91-98)

#### Dependencies and Required Updates

**1. Log Aggregation/Parsing - DOCUMENTATION UPDATE**

- **Concern**: Changing log level from Debug → Info increases log volume
- **Impact**:
  - Logs will now appear in production (Info is default minimum level)
  - Log aggregation tools may need filter updates if they parse specific messages
- **Required**: Update any log parsing scripts or documentation about expected log output

**2. Integration Tests - POTENTIAL GREP PATTERNS**

- **Files**: `scripts/crowdsec_*.sh`
- **Impact**: If scripts `grep` for specific log messages, they may need updates
- **Action**: Search for log message expectations in scripts

**3. Documentation - UPDATE REQUIRED**

- **File**: `docs/features.md`
- **Section**: CrowdSec Integration (line 167+)
- **Required Change**: Add note about reconciliation behavior:

  ```markdown
  #### Startup Behavior

  CrowdSec automatically starts on container restart if:
  - SecurityConfig has `crowdsec_mode = "local"` OR
  - Settings table has `security.crowdsec.enabled = "true"`

  Check container logs for reconciliation decisions:
  - "CrowdSec reconciliation: starting based on SecurityConfig mode='local'"
  - "CrowdSec reconciliation: starting based on Settings table override"
  - "CrowdSec reconciliation skipped: both SecurityConfig and Settings indicate disabled"
  ```

**4. Troubleshooting Guide - UPDATE RECOMMENDED**

- **File**: `docs/troubleshooting/` (if exists) or `docs/security.md`
- **Required Change**: Add section on "CrowdSec Not Starting After Restart"
  - Explain reconciliation logic
  - Show how to check Settings and SecurityConfig tables
  - Show example log output

### Phase 3: Unified Toggle Endpoint (OPTIONAL)

#### Files Directly Modified

- `backend/internal/api/handlers/crowdsec_handler.go` (new method)
- `backend/internal/api/handlers/crowdsec_handler.go` (RegisterRoutes)
- `frontend/src/api/crowdsec.ts` (new function)
- `frontend/src/pages/Security.tsx` (mutation update)

#### Dependencies and Required Updates

**1. Handler Tests - NEW TESTS REQUIRED**

- **File**: `backend/internal/api/handlers/crowdsec_handler_test.go`
- **Required Tests**:
  - `TestCrowdsecHandler_Toggle_EnableSuccess`
  - `TestCrowdsecHandler_Toggle_DisableSuccess`
  - `TestCrowdsecHandler_Toggle_TransactionRollback` (if Start fails)
  - `TestCrowdsecHandler_Toggle_VerifyBothTablesUpdated`

**2. Existing Handlers - DEPRECATION CONSIDERATION**

- **Files**:
  - Start handler (line ~167 in crowdsec_handler.go)
  - Stop handler (line ~260 in crowdsec_handler.go)
- **Impact**: New toggle endpoint duplicates Start/Stop functionality
- **Decision Required**:
  - **Option A**: Keep both for backward compatibility (RECOMMENDED)
  - **Option B**: Deprecate Start/Stop, add deprecation warnings
  - **Option C**: Remove Start/Stop entirely (BREAKING CHANGE - NOT RECOMMENDED)
- **Recommendation**: Keep Start/Stop handlers unchanged, document toggle as "preferred method"

**3. Frontend API Layer - MIGRATION PATH**

- **File**: `frontend/src/api/crowdsec.ts`
- **Current Exports**: `startCrowdsec`, `stopCrowdsec`, `statusCrowdsec`
- **After Change**: Add `toggleCrowdsec` to exports (line 75)
- **Backward Compatibility**: Keep existing functions, don't remove them

**4. Frontend Component - LIMITED SCOPE**

- **File**: `frontend/src/pages/Security.tsx`
- **Impact**: Only `crowdsecPowerMutation` needs updating (lines 86-125)
- **Other Components**: No other components import these functions (verified)
- **Risk**: Low - isolated change

**5. API Documentation - NEW ENDPOINT**

- **File**: `docs/api.md` (if exists)
- **Required Addition**: Document `/admin/crowdsec/toggle` endpoint

**6. Integration Tests - NEW TEST CASE**

- **Files**: `scripts/crowdsec_integration.sh`
- **Required Addition**: Test toggle endpoint directly

**7. Backward Compatibility - ANALYSIS**

- **Frontend**: Existing `/admin/crowdsec/start` and `/admin/crowdsec/stop` endpoints remain functional
- **API Consumers**: External tools using Start/Stop continue to work
- **Risk**: None - purely additive change

### Cross-Cutting Concerns

#### Database Migration

- **No schema changes required** - both Settings and SecurityConfig tables already exist
- **Data migration**: None needed - changes are behavioral only

#### Configuration Files

- **No changes required** - no new environment variables or config files

#### Docker/Deployment

- **No Dockerfile changes** - all changes are code-level
- **No docker-compose changes** - no new services or volumes

#### Security Implications

- **Phase 1**: Improves security by respecting user's intent across restarts
- **Phase 2**: No security impact (logging only)
- **Phase 3**: Transaction safety prevents partial updates (improvement)

#### Performance Considerations

- **Phase 1**: Adds one SQL query during auto-initialization (one-time, on startup)
- **Phase 2**: Minimal - only adds log statements
- **Phase 3**: Minimal - wraps existing logic in transaction

#### Rollback Safety

- **All phases**: No database schema changes, can be rolled back via git revert
- **Data safety**: No data loss risk - only affects process startup behavior

### Summary of Required File Updates

| Phase | Files to Modify | Files to Create | Tests to Add | Docs to Update |
|-------|----------------|-----------------|--------------|----------------|
| **Phase 1** | `crowdsec_startup.go` | None | 3 new unit tests | None (covered in Phase 2) |
| **Phase 2** | `crowdsec_startup.go` | None | None | `features.md`, troubleshooting docs |
| **Phase 3** | `crowdsec_handler.go`, `crowdsec.ts`, `Security.tsx` | None | 4 new handler tests | `api.md` (if exists) |

### Testing Matrix

| Scenario | Phase 1 | Phase 2 | Phase 3 |
|----------|---------|---------|---------|
| Fresh install → toggle ON → restart | ✅ Fixes | ✅ Better logs | ✅ Cleaner code |
| Existing install with Settings='true', missing SecurityConfig | ✅ Fixes | ✅ Better logs | N/A |
| Toggle ON → restart → verify logs | ✅ Works | ✅ MUST verify new messages | ✅ Works |
| Toggle OFF → restart → verify logs | ✅ Works | ✅ MUST verify new messages | ✅ Works |
| Start/Stop handlers (backward compat) | N/A | N/A | ✅ MUST verify still work |

### Missing from Original Plan

The original plan DID NOT explicitly mention:

1. **Unit test updates required** - Critical for Phase 1 (`TestReconcileCrowdSecOnStartup_NoSecurityConfig` needs major refactoring)
2. **Integration script verification** - May break if they expect specific behavior
3. **Documentation updates** - Features and troubleshooting guides need new reconciliation behavior documented
4. **Backward compatibility analysis for Phase 3** - Need explicit decision on Start/Stop handler fate
5. **API documentation** - New endpoint needs docs
6. **Testing matrix for all three phases together** - Need to verify they work in combination

---

**END OF SPECIFICATION**
