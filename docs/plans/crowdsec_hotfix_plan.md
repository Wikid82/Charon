# CrowdSec Critical Hotfix Remediation Plan

**Date**: December 15, 2025
**Priority**: CRITICAL
**Issue Count**: 4 reported issues after 17 failed commit attempts
**Affected Components**: Backend (handlers, services), Frontend (pages, hooks, components)

---

## Executive Summary

After exhaustive analysis of the CrowdSec functionality across both backend and frontend, I have identified the **root causes** of all four reported issues. The core problem is a **dual-state architecture conflict** where CrowdSec's enabled state is managed by TWO independent systems that don't synchronize properly:

1. **Settings Table** (`security.crowdsec.enabled` and `security.crowdsec.mode`) - Runtime overrides
2. **SecurityConfig Table** (`CrowdSecMode` column) - User configuration

Additionally, the Live Log Viewer has a **WebSocket lifecycle bug** and the deprecated mode UI causes state conflicts.

---

## The 4 Reported Issues

| # | Issue | Root Cause | Severity |
|---|-------|------------|----------|
| 1 | CrowdSec card toggle broken - shows "active" but not actually on | Dual-state conflict: `security.crowdsec.mode` overrides `security.crowdsec.enabled` | CRITICAL |
| 2 | Live logs show "disconnected" but logs appear; navigation clears logs | WebSocket reconnection lifecycle bug + state not persisted | HIGH |
| 3 | Deprecated mode toggle still in UI causing confusion | UI component not removed after deprecation | MEDIUM |
| 4 | Enrollment shows "not running" when LAPI initializing | Race condition between process start and LAPI readiness | HIGH |

---

## Current State Analysis

### Backend Data Flow

#### 1. SecurityConfig Model

**File**: [backend/internal/models/security_config.go](../../backend/internal/models/security_config.go)

```go
type SecurityConfig struct {
    CrowdSecMode   string `json:"crowdsec_mode"` // "disabled" or "local" - DEPRECATED
    Enabled        bool   `json:"enabled"`       // Cerberus master switch
    // ...
}
```

#### 2. GetStatus Handler - THE BUG

**File**: [backend/internal/api/handlers/security_handler.go#L75-175](../../backend/internal/api/handlers/security_handler.go#L75-175)

The `GetStatus` endpoint has a **three-tier priority chain** that causes the bug:

```go
// PRIORITY 1 (highest): Settings table overrides
// Line 135-140: Check security.crowdsec.enabled
if strings.EqualFold(setting.Value, "true") {
    crowdSecMode = "local"
} else {
    crowdSecMode = "disabled"
}

// Line 143-148: THEN check security.crowdsec.mode - THIS OVERRIDES THE ABOVE!
setting = struct{ Value string }{}
if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.mode").Scan(&setting).Error; err == nil && setting.Value != "" {
    crowdSecMode = setting.Value  // <-- BUG: This can override the enabled check!
}
```

**The Bug Flow**:

1. User toggles CrowdSec ON → `security.crowdsec.enabled = "true"` → `crowdSecMode = "local"` ✓
2. BUT if `security.crowdsec.mode = "disabled"` was previously set (by deprecated UI), it OVERRIDES step 1
3. Final result: `crowdSecMode = "disabled"` even though user just toggled it ON

#### 3. CrowdSec Start Handler - INCONSISTENT STATE UPDATE

**File**: [backend/internal/api/handlers/crowdsec_handler.go#L184-240](../../backend/internal/api/handlers/crowdsec_handler.go#L184-240)

```go
func (h *CrowdsecHandler) Start(c *gin.Context) {
    // Updates SecurityConfig table
    cfg.CrowdSecMode = "local"
    cfg.Enabled = true
    h.DB.Save(&cfg)  // Saves to security_configs table

    // BUT: Does NOT update settings table!
    // Missing: h.DB.Create/Update(&models.Setting{Key: "security.crowdsec.enabled", Value: "true"})
}
```

**Problem**: `Start()` updates `SecurityConfig.CrowdSecMode` but the frontend toggle updates `settings.security.crowdsec.enabled`. These are TWO DIFFERENT tables that both affect CrowdSec state.

#### 4. Feature Flags Handler

**File**: [backend/internal/api/handlers/feature_flags_handler.go](../../backend/internal/api/handlers/feature_flags_handler.go)

Only manages THREE flags:

- `feature.cerberus.enabled` (Cerberus master switch)
- `feature.uptime.enabled`
- `feature.crowdsec.console_enrollment`

**Missing**: No `feature.crowdsec.enabled`. CrowdSec uses `security.crowdsec.enabled` in settings table, which is NOT a feature flag.

### Frontend Data Flow

#### 1. Security.tsx (Cerberus Dashboard)

**File**: [frontend/src/pages/Security.tsx#L65-110](../../frontend/src/pages/Security.tsx#L65-110)

```typescript
const crowdsecPowerMutation = useMutation({
    mutationFn: async (enabled: boolean) => {
      // Step 1: Update settings table
      await updateSetting('security.crowdsec.enabled', enabled ? 'true' : 'false', 'security', 'bool')

      if (enabled) {
        // Step 2: Start process (which updates SecurityConfig table)
        const result = await startCrowdsec()
        // ...
      }
    }
})
```

The mutation updates TWO places:

1. `settings` table via `updateSetting()` → sets `security.crowdsec.enabled`
2. `security_configs` table via `startCrowdsec()` backend → sets `CrowdSecMode`

But `GetStatus` reads from BOTH and can get conflicting values.

#### 2. CrowdSecConfig.tsx - DEPRECATED MODE TOGGLE

**File**: [frontend/src/pages/CrowdSecConfig.tsx#L69-90](../../frontend/src/pages/CrowdSecConfig.tsx#L69-90)

```typescript
const updateModeMutation = useMutation({
    mutationFn: async (mode: string) => updateSetting('security.crowdsec.mode', mode, 'security', 'string'),
    // This updates security.crowdsec.mode which OVERRIDES security.crowdsec.enabled!
})
```

**This is the deprecated toggle that should not exist.** It sets `security.crowdsec.mode` which takes precedence over `security.crowdsec.enabled` in `GetStatus`.

#### 3. LiveLogViewer.tsx - WEBSOCKET BUGS

**File**: [frontend/src/components/LiveLogViewer.tsx#L100-150](../../frontend/src/components/LiveLogViewer.tsx#L100-150)

```typescript
useEffect(() => {
    // Close existing connection
    if (closeConnectionRef.current) {
      closeConnectionRef.current();
      closeConnectionRef.current = null;
    }
    // ... reconnect logic
}, [currentMode, filters, securityFilters, isPaused, maxLogs, showBlockedOnly]);
//                                          ^^^^^^^^
// BUG: isPaused in dependencies causes reconnection when user just wants to pause!
```

**Problems**:

1. `isPaused` in deps → toggling pause causes WebSocket disconnect/reconnect
2. Navigation away unmounts component → `logs` state is lost
3. `isConnected` is local state → lost on unmount, starts as `false` on remount
4. No reconnection retry logic

#### 4. Console Enrollment LAPI Check

**File**: [frontend/src/pages/CrowdSecConfig.tsx#L85-120](../../frontend/src/pages/CrowdSecConfig.tsx#L85-120)

```typescript
// Wait 3 seconds before first LAPI check
const timer = setTimeout(() => {
    setInitialCheckComplete(true)
}, 3000)
```

**Problem**: 3 seconds may not be enough. CrowdSec LAPI typically takes 5-10 seconds to initialize. Users see "not running" error during this window.

---

## Identified Problems

### Problem 1: Dual-State Conflict (Toggle Shows Active But Not Working)

**Evidence Chain**:

```
User toggles ON → updateSetting('security.crowdsec.enabled', 'true')
                → startCrowdsec() → sets SecurityConfig.CrowdSecMode = 'local'

User refreshes page → getSecurityStatus()
                    → Reads security.crowdsec.enabled = 'true' → crowdSecMode = 'local'
                    → Reads security.crowdsec.mode (if exists) → OVERRIDES to whatever value

If security.crowdsec.mode = 'disabled' (from deprecated UI) → Final: crowdSecMode = 'disabled'
```

**Locations**:

- Backend: [security_handler.go#L135-148](../../backend/internal/api/handlers/security_handler.go#L135-148)
- Backend: [crowdsec_handler.go#L195-215](../../backend/internal/api/handlers/crowdsec_handler.go#L195-215)
- Frontend: [Security.tsx#L65-110](../../frontend/src/pages/Security.tsx#L65-110)

### Problem 2: Live Log Viewer State Issues

**Evidence**:

- Shows "Disconnected" immediately after page load (initial state = false)
- Logs appear because WebSocket connects quickly, but `isConnected` state update races
- Navigation away loses all log entries (component state)
- Pausing causes reconnection flicker

**Location**: [LiveLogViewer.tsx#L100-150](../../frontend/src/components/LiveLogViewer.tsx#L100-150)

### Problem 3: Deprecated Mode Toggle Still Present

**Evidence**: CrowdSecConfig.tsx still renders:

```tsx
<Card>
  <h2>CrowdSec Mode</h2>
  <Switch checked={isLocalMode} onChange={(e) => handleModeToggle(e.target.checked)} />
  {/* Disabled/Local toggle - DEPRECATED */}
</Card>
```

**Location**: [CrowdSecConfig.tsx#L395-420](../../frontend/src/pages/CrowdSecConfig.tsx#L395-420)

### Problem 4: Enrollment "Not Running" Error

**Evidence**: User enables CrowdSec, immediately tries to enroll, sees error because:

1. Process starts (running=true)
2. LAPI takes 5-10s to initialize (lapi_ready=false)
3. Frontend shows "not running" because it checks lapi_ready

**Locations**:

- Frontend: [CrowdSecConfig.tsx#L85-120](../../frontend/src/pages/CrowdSecConfig.tsx#L85-120)
- Backend: [console_enroll.go#L165-190](../../backend/internal/crowdsec/console_enroll.go#L165-190)

---

## Remediation Plan

### Phase 1: Backend Fixes (CRITICAL)

#### 1.1 Fix GetStatus Priority Chain

**File**: `backend/internal/api/handlers/security_handler.go`
**Lines**: 143-148

**Current Code (BUGGY)**:

```go
// CrowdSec mode override (AFTER enabled check - causes override bug)
setting = struct{ Value string }{}
if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.mode").Scan(&setting).Error; err == nil && setting.Value != "" {
    crowdSecMode = setting.Value
}
```

**Fix**: Remove the mode override OR make enabled take precedence:

```go
// OPTION A: Remove mode override entirely (recommended)
// DELETE lines 143-148

// OPTION B: Make enabled take precedence over mode
setting = struct{ Value string }{}
if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.mode").Scan(&setting).Error; err == nil && setting.Value != "" {
    // Only use mode if enabled wasn't explicitly set
    var enabledSetting struct{ Value string }
    if h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.enabled").Scan(&enabledSetting).Error != nil || enabledSetting.Value == "" {
        crowdSecMode = setting.Value
    }
    // If enabled was set, ignore deprecated mode setting
}
```

#### 1.2 Update Start/Stop to Sync State

**File**: `backend/internal/api/handlers/crowdsec_handler.go`

**In Start() after line 215**:

```go
// Sync settings table (source of truth for UI)
if h.DB != nil {
    settingEnabled := models.Setting{
        Key:      "security.crowdsec.enabled",
        Value:    "true",
        Type:     "bool",
        Category: "security",
    }
    h.DB.Where(models.Setting{Key: "security.crowdsec.enabled"}).Assign(settingEnabled).FirstOrCreate(&settingEnabled)

    // Clear deprecated mode setting to prevent conflicts
    h.DB.Where("key = ?", "security.crowdsec.mode").Delete(&models.Setting{})
}
```

**In Stop() after line 260**:

```go
// Sync settings table
if h.DB != nil {
    settingEnabled := models.Setting{
        Key:      "security.crowdsec.enabled",
        Value:    "false",
        Type:     "bool",
        Category: "security",
    }
    h.DB.Where(models.Setting{Key: "security.crowdsec.enabled"}).Assign(settingEnabled).FirstOrCreate(&settingEnabled)
}
```

#### 1.3 Add Deprecation Warning for Mode Setting

**File**: `backend/internal/api/handlers/settings_handler.go`

Add validation in the update handler:

```go
func (h *SettingsHandler) UpdateSetting(c *gin.Context) {
    // ... existing code ...

    if setting.Key == "security.crowdsec.mode" {
        logger.Log().Warn("DEPRECATED: security.crowdsec.mode is deprecated and will be removed. Use security.crowdsec.enabled instead.")
    }

    // ... rest of existing code ...
}
```

### Phase 2: Frontend Fixes

#### 2.1 Remove Deprecated Mode Toggle

**File**: `frontend/src/pages/CrowdSecConfig.tsx`

**Remove these sections**:

1. **Lines 69-78** - Remove `updateModeMutation`:

```typescript
// DELETE THIS ENTIRE MUTATION
const updateModeMutation = useMutation({
    mutationFn: async (mode: string) => updateSetting('security.crowdsec.mode', mode, 'security', 'string'),
    onSuccess: (_data, mode) => {
      queryClient.invalidateQueries({ queryKey: ['security-status'] })
      toast.success(mode === 'disabled' ? 'CrowdSec disabled' : 'CrowdSec set to Local mode')
    },
    onError: (err: unknown) => {
      const msg = err instanceof Error ? err.message : 'Failed to update mode'
      toast.error(msg)
    },
})
```

1. **Lines ~395-420** - Remove the Mode Card from render:

```tsx
// DELETE THIS ENTIRE CARD
<Card>
  <div className="flex items-center justify-between gap-4 flex-wrap">
    <div className="space-y-1">
      <h2 className="text-lg font-semibold">CrowdSec Mode</h2>
      <p className="text-sm text-gray-400">...</p>
    </div>
    <div className="flex items-center gap-3">
      <span>Disabled</span>
      <Switch checked={isLocalMode} onChange={(e) => handleModeToggle(e.target.checked)} />
      <span>Local</span>
    </div>
  </div>
</Card>
```

1. **Replace with informational banner**:

```tsx
<Card>
  <div className="p-4 bg-blue-900/20 border border-blue-700/50 rounded-lg">
    <p className="text-sm text-blue-200">
      CrowdSec is controlled from the <Link to="/security" className="text-blue-400 underline">Security Dashboard</Link>.
      Use the toggle there to enable or disable CrowdSec protection.
    </p>
  </div>
</Card>
```

#### 2.2 Fix Live Log Viewer

**File**: `frontend/src/components/LiveLogViewer.tsx`

**Fix 1**: Remove `isPaused` from dependencies (line 148):

```typescript
// BEFORE:
}, [currentMode, filters, securityFilters, isPaused, maxLogs, showBlockedOnly]);

// AFTER:
}, [currentMode, filters, securityFilters, maxLogs, showBlockedOnly]);
```

**Fix 2**: Use ref for pause state in message handler:

```typescript
// Add ref near other refs (around line 70):
const isPausedRef = useRef(isPaused);

// Sync ref with state (add useEffect around line 95):
useEffect(() => {
  isPausedRef.current = isPaused;
}, [isPaused]);

// Update message handler (lines 110-120):
const handleSecurityMessage = (entry: SecurityLogEntry) => {
    if (!isPausedRef.current) {  // Use ref instead of state
        const displayEntry = toDisplayFromSecurity(entry);
        setLogs((prev) => {
            const updated = [...prev, displayEntry];
            return updated.length > maxLogs ? updated.slice(-maxLogs) : updated;
        });
    }
};
```

**Fix 3**: Add reconnection retry logic:

```typescript
// Add state for retry (around line 50):
const [retryCount, setRetryCount] = useState(0);
const maxRetries = 5;
const retryDelay = 2000; // 2 seconds base delay

// Update connection effect (around line 100):
useEffect(() => {
    // ... existing close logic ...

    const handleClose = () => {
      console.log(`${currentMode} log viewer disconnected`);
      setIsConnected(false);

      // Schedule retry with exponential backoff
      if (retryCount < maxRetries) {
        const delay = retryDelay * Math.pow(1.5, retryCount);
        setTimeout(() => setRetryCount(r => r + 1), delay);
      }
    };

    // ... rest of effect ...

    return () => {
      if (closeConnectionRef.current) {
        closeConnectionRef.current();
        closeConnectionRef.current = null;
      }
      setIsConnected(false);
      // Reset retry on intentional unmount
    };
}, [currentMode, filters, securityFilters, maxLogs, showBlockedOnly, retryCount]);

// Reset retry count on successful connect:
const handleOpen = () => {
    console.log(`${currentMode} log viewer connected`);
    setIsConnected(true);
    setRetryCount(0);  // Reset retry counter
};
```

#### 2.3 Improve Enrollment LAPI Messaging

**File**: `frontend/src/pages/CrowdSecConfig.tsx`

**Fix 1**: Increase initial delay (line 85):

```typescript
// BEFORE:
}, 3000) // Wait 3 seconds

// AFTER:
}, 5000) // Wait 5 seconds for LAPI to initialize
```

**Fix 2**: Improve warning messages (around lines 200-250):

```tsx
{/* Show LAPI initializing warning when process running but LAPI not ready */}
{lapiStatusQuery.data && lapiStatusQuery.data.running && !lapiStatusQuery.data.lapi_ready && initialCheckComplete && (
  <div className="flex items-start gap-3 p-4 bg-yellow-900/20 border border-yellow-700/50 rounded-lg">
    <AlertTriangle className="w-5 h-5 text-yellow-400 flex-shrink-0 mt-0.5" />
    <div className="flex-1">
      <p className="text-sm text-yellow-200 font-medium mb-2">
        CrowdSec Local API is initializing...
      </p>
      <p className="text-xs text-yellow-300 mb-3">
        The CrowdSec process is running but LAPI takes 5-10 seconds to become ready.
        Console enrollment will be available once LAPI is ready.
        {lapiStatusQuery.isRefetching && ' Checking status...'}
      </p>
      <Button variant="secondary" size="sm" onClick={() => lapiStatusQuery.refetch()} disabled={lapiStatusQuery.isRefetching}>
        Check Again
      </Button>
    </div>
  </div>
)}

{/* Show not running warning when process not running */}
{lapiStatusQuery.data && !lapiStatusQuery.data.running && initialCheckComplete && (
  <div className="flex items-start gap-3 p-4 bg-red-900/20 border border-red-700/50 rounded-lg">
    <AlertTriangle className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" />
    <div className="flex-1">
      <p className="text-sm text-red-200 font-medium mb-2">
        CrowdSec is not running
      </p>
      <p className="text-xs text-red-300 mb-3">
        Enable CrowdSec from the <Link to="/security" className="text-red-400 underline">Security Dashboard</Link> first.
        The process typically takes 5-10 seconds to start and LAPI another 5-10 seconds to initialize.
      </p>
    </div>
  </div>
)}
```

### Phase 3: Cleanup & Testing

#### 3.1 Database Cleanup Migration (Optional)

Create a one-time migration to remove conflicting settings:

```sql
-- Remove deprecated mode setting to prevent conflicts
DELETE FROM settings WHERE key = 'security.crowdsec.mode';
```

#### 3.2 Backend Test Updates

Add test cases for:

1. `GetStatus` returns correct enabled state when only `security.crowdsec.enabled` is set
2. `GetStatus` returns correct state when deprecated `security.crowdsec.mode` exists (should be ignored)
3. `Start()` updates `settings` table
4. `Stop()` updates `settings` table

#### 3.3 Frontend Test Updates

Add test cases for:

1. `LiveLogViewer` doesn't reconnect when pause toggled
2. `LiveLogViewer` retries connection on disconnect
3. `CrowdSecConfig` doesn't render mode toggle

---

## Test Plan

### Manual QA Checklist

- [ ] **Toggle Test**:
  1. Go to Security Dashboard
  2. Toggle CrowdSec ON
  3. Verify card shows "Active"
  4. Verify `docker exec charon ps aux | grep crowdsec` shows process
  5. Toggle CrowdSec OFF
  6. Verify card shows "Disabled"
  7. Verify process stopped

- [ ] **State Persistence Test**:
  1. Toggle CrowdSec ON
  2. Refresh page
  3. Verify toggle still shows ON
  4. Check database: `SELECT * FROM settings WHERE key LIKE '%crowdsec%'`

- [ ] **Live Logs Test**:
  1. Go to Security Dashboard
  2. Verify "Connected" status appears
  3. Generate some traffic
  4. Verify logs appear
  5. Click "Pause" - verify NO flicker/reconnect
  6. Navigate to another page
  7. Navigate back
  8. Verify reconnection happens (status goes from Disconnected → Connected)

- [ ] **Enrollment Test**:
  1. Enable CrowdSec
  2. Go to CrowdSecConfig
  3. Verify warning shows "LAPI initializing" (not "not running")
  4. Wait for LAPI ready
  5. Enter enrollment key
  6. Click Enroll
  7. Verify success

- [ ] **Deprecated UI Removed**:
  1. Go to CrowdSecConfig page
  2. Verify NO "CrowdSec Mode" card with Disabled/Local toggle
  3. Verify informational banner points to Security Dashboard

### Integration Test Commands

```bash
# Test 1: Backend state consistency
# Enable via API
curl -X POST http://localhost:8080/api/v1/admin/crowdsec/start

# Check settings table
sqlite3 data/charon.db "SELECT * FROM settings WHERE key = 'security.crowdsec.enabled'"
# Expected: value = "true"

# Check status endpoint
curl http://localhost:8080/api/v1/security/status | jq '.crowdsec'
# Expected: {"mode":"local","enabled":true,...}

# Test 2: No deprecated mode conflict
sqlite3 data/charon.db "SELECT * FROM settings WHERE key = 'security.crowdsec.mode'"
# Expected: No rows (or deprecated warning logged)

# Test 3: Disable and verify
curl -X POST http://localhost:8080/api/v1/admin/crowdsec/stop

curl http://localhost:8080/api/v1/security/status | jq '.crowdsec'
# Expected: {"mode":"disabled","enabled":false,...}

sqlite3 data/charon.db "SELECT * FROM settings WHERE key = 'security.crowdsec.enabled'"
# Expected: value = "false"
```

---

## Implementation Order

| Order | Phase | Task | Priority | Est. Time |
|-------|-------|------|----------|-----------|
| 1 | 1.1 | Fix GetStatus to ignore deprecated mode | CRITICAL | 15 min |
| 2 | 1.2 | Update Start/Stop to sync settings table | CRITICAL | 20 min |
| 3 | 2.1 | Remove deprecated mode toggle from UI | HIGH | 15 min |
| 4 | 2.2 | Fix LiveLogViewer pause/reconnection | HIGH | 30 min |
| 5 | 2.3 | Improve enrollment LAPI messaging | MEDIUM | 15 min |
| 6 | 1.3 | Add deprecation warning for mode setting | LOW | 10 min |
| 7 | 3.1 | Database cleanup migration | LOW | 10 min |
| 8 | 3.2-3.3 | Update tests | MEDIUM | 30 min |

**Total Estimated Time**: ~2.5 hours

---

## Success Criteria

1. ✅ Toggling CrowdSec ON shows "Active" AND process is actually running
2. ✅ Toggling CrowdSec OFF shows "Disabled" AND process is stopped
3. ✅ State persists across page refresh
4. ✅ No deprecated mode toggle visible on CrowdSecConfig page
5. ✅ Live logs show "Connected" when WebSocket connects
6. ✅ Pausing logs does NOT cause reconnection
7. ✅ Enrollment shows appropriate LAPI status message
8. ✅ All existing tests pass
9. ✅ No errors in browser console related to CrowdSec

---

## Appendix: File Reference

| Issue | Backend Files | Frontend Files |
|-------|---------------|----------------|
| Toggle Bug | `security_handler.go#L135-148`, `crowdsec_handler.go#L184-265` | `Security.tsx#L65-110` |
| Deprecated Mode | `security_handler.go#L143-148` | `CrowdSecConfig.tsx#L69-90, L395-420` |
| Live Logs | `cerberus_logs_ws.go` | `LiveLogViewer.tsx#L100-150`, `logs.ts` |
| Enrollment | `console_enroll.go#L165-190` | `CrowdSecConfig.tsx#L85-120` |
