# Investigation Report: CrowdSec Enrollment & Live Log Viewer Issues

**Date:** December 15, 2025
**Investigator:** GitHub Copilot
**Status:** ✅ Issue A RESOLVED - Issue B Analysis Pending

---

## Executive Summary (Updated December 16, 2025)

This document covers TWO issues:

1. **CrowdSec Enrollment** ✅ **FIXED**: Shows success locally but engine doesn't appear in CrowdSec.net dashboard
   - **Root Cause**: Code incorrectly set status to `enrolled` after `cscli console enroll` succeeded, but CrowdSec's help explicitly states users must "validate the enrollment in the webapp"
   - **Fix Applied**: Changed status to `pending_acceptance` and updated frontend to inform users they must accept on app.crowdsec.net

2. **Live Log Viewer**: Shows "Disconnected" status (Analysis pending implementation)

---

## ✅ RESOLVED Issue A: CrowdSec Console Enrollment Not Working

### Symptoms
- User submits enrollment with valid key
- Charon shows "Enrollment submitted" success message
- No engine appears in CrowdSec.net dashboard
- User reports: "The CrowdSec enrollment request NEVER reached crowdsec.net"

### Root Cause (CONFIRMED)

**The Bug**: After a **successful** `cscli console enroll <key>` command (exit code 0), CrowdSec's help explicitly states:
> "After running this command you will need to validate the enrollment in the webapp."

Exit code 0 = enrollment REQUEST sent, NOT enrollment COMPLETE.

The code incorrectly set `status = enrolled` when it should have been `status = pending_acceptance`.

### Fixes Applied (December 16, 2025)

#### Fix A1: Backend Status Semantics
**File**: `backend/internal/crowdsec/console_enroll.go`
- Added `consoleStatusPendingAcceptance = "pending_acceptance"` constant
- Changed success status from `enrolled` to `pending_acceptance`
- Fixed idempotency check to also skip re-enrollment when status is `pending_acceptance`
- Fixed config path check to look in `config/config.yaml` subdirectory first
- Updated log message to say "pending acceptance on crowdsec.net"

#### Fix A2: Frontend User Guidance
**File**: `frontend/src/pages/CrowdSecConfig.tsx`
- Updated success toast to say "Accept the enrollment on app.crowdsec.net to complete registration"
- Added `isConsolePendingAcceptance` variable
- Updated `canRotateKey` to include `pending_acceptance` status
- Added info box with link to app.crowdsec.net when status is `pending_acceptance`

#### Fix A3: Test Updates
**Files**: `backend/internal/crowdsec/console_enroll_test.go`, `backend/internal/api/handlers/crowdsec_handler_test.go`
- Updated all tests expecting `enrolled` to expect `pending_acceptance`
- Updated test for idempotency to verify second call is blocked for `pending_acceptance`
- Changed `EnrolledAt` assertion to `LastAttemptAt` (enrollment is not complete yet)

### Verification
All backend tests pass:
- `TestConsoleEnrollSuccess` ✅
- `TestConsoleEnrollIdempotentWhenAlreadyEnrolled` ✅
- `TestConsoleEnrollNormalizesFullCommand` ✅
- `TestConsoleEnrollDoesNotPassTenant` ✅
- `TestConsoleEnrollmentStatus/returns_pending_acceptance_status_after_enrollment` ✅
- `TestConsoleStatusAfterEnroll` ✅

Frontend type-check passes ✅

---

## NEW Issue B: Live Log Viewer Shows "Disconnected"

### Symptoms
- Live Log Viewer component shows "Disconnected" status badge
- No logs appear (even when there should be logs)
- WebSocket connection may not be establishing

### Root Cause Analysis

**Primary Finding: WebSocket Connection Works But Logs Are Sparse**

The WebSocket implementation is correct. The issue is likely:

1. **No logs being generated** - If CrowdSec/Caddy aren't actively processing requests, there are no logs
2. **Initial connection timing** - The `isConnected` state depends on `onOpen` callback

**Verified Working Components:**

1. **Backend WebSocket Handler**: `backend/internal/api/handlers/logs_ws.go`
   - Properly upgrades HTTP to WebSocket
   - Subscribes to `BroadcastHook` for log entries
   - Sends ping messages every 30 seconds

2. **Frontend Connection Logic**: `frontend/src/api/logs.ts`
   - `connectLiveLogs()` correctly builds WebSocket URL
   - Properly handles `onOpen`, `onClose`, `onError` callbacks

3. **Frontend Component**: `frontend/src/components/LiveLogViewer.tsx`
   - `isConnected` state is set in `handleOpen` callback
   - Connection effect runs on mount and mode changes

### Potential Issues Found

#### Issue B1: WebSocket Route May Be Protected

**Location**: `backend/internal/api/routes/routes.go` Line 158

The WebSocket endpoint is under the `protected` route group, meaning it requires authentication:

```go
protected.GET("/logs/live", handlers.LogsWebSocketHandler)
```

**Problem**: WebSocket connections may fail silently if auth token isn't being passed. The browser's native WebSocket API doesn't automatically include HTTP-only cookies or Authorization headers.

**Verification Steps:**
1. Check browser DevTools Network tab for WebSocket connection
2. Look for 401/403 responses
3. Check if `token` query parameter is being sent

#### Issue B2: No Error Display to User

**Location**: `frontend/src/components/LiveLogViewer.tsx` Lines 170-172

```tsx
const handleError = (error: Event) => {
  console.error('WebSocket error:', error);
  setIsConnected(false);
};
```

**Problem**: Errors are only logged to console, not displayed to user. User sees "Disconnected" without knowing why.

### Required Fixes for Issue B

#### Fix B1: Add Error State Display

**File**: `frontend/src/components/LiveLogViewer.tsx`

Add error state tracking:

```tsx
const [connectionError, setConnectionError] = useState<string | null>(null);

const handleError = (error: Event) => {
  console.error('WebSocket error:', error);
  setIsConnected(false);
  setConnectionError('Failed to connect to log stream. Check authentication.');
};

const handleOpen = () => {
  console.log(`${currentMode} log viewer connected`);
  setIsConnected(true);
  setConnectionError(null); // Clear any previous errors
};
```

Display error in UI:

```tsx
{connectionError && (
  <div className="text-red-400 text-xs p-2">{connectionError}</div>
)}
```

#### Fix B2: Add Authentication to WebSocket URL

**File**: `frontend/src/api/logs.ts`

The WebSocket needs to pass auth token as query parameter since WebSocket API doesn't support custom headers:

```typescript
export const connectLiveLogs = (
  filters: LiveLogFilter,
  onMessage: (log: LiveLogEntry) => void,
  onOpen?: () => void,
  onError?: (error: Event) => void,
  onClose?: () => void
): (() => void) => {
  const params = new URLSearchParams();
  if (filters.level) params.append('level', filters.level);
  if (filters.source) params.append('source', filters.source);

  // Add auth token from localStorage if available
  const token = localStorage.getItem('token');
  if (token) {
    params.append('token', token);
  }

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/api/v1/logs/live?${params.toString()}`;
  // ...
};
```

**Backend Auth Check** (verify this exists):
The backend auth middleware must check for `token` query parameter in addition to headers/cookies for WebSocket connections.

#### Fix B3: Add Reconnection Logic

**File**: `frontend/src/components/LiveLogViewer.tsx`

Add automatic reconnection with exponential backoff:

```tsx
const [reconnectAttempts, setReconnectAttempts] = useState(0);
const maxReconnectAttempts = 5;

const handleClose = () => {
  console.log(`${currentMode} log viewer disconnected`);
  setIsConnected(false);

  // Auto-reconnect logic
  if (reconnectAttempts < maxReconnectAttempts) {
    const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 30000);
    setTimeout(() => {
      setReconnectAttempts(prev => prev + 1);
      // Trigger reconnection by updating a dependency
    }, delay);
  }
};
```

---

## Summary of All Fixes

### Issue A: CrowdSec Enrollment

| File | Change |
|------|--------|
| `frontend/src/pages/CrowdSecConfig.tsx` | Update success toast to mention acceptance step |
| `frontend/src/pages/CrowdSecConfig.tsx` | Add info box with link to crowdsec.net |
| `backend/internal/crowdsec/console_enroll.go` | Add `pending_acceptance` status constant |
| `docs/cerberus.md` | Add documentation about acceptance requirement |

### Issue B: Live Log Viewer

| File | Change |
|------|--------|
| `frontend/src/components/LiveLogViewer.tsx` | Add error state display |
| `frontend/src/api/logs.ts` | Pass auth token in WebSocket URL |
| `frontend/src/components/LiveLogViewer.tsx` | Add reconnection logic with backoff |

---

## Testing Checklist

### Enrollment Testing
- [ ] Submit enrollment with valid key
- [ ] Verify success message mentions acceptance step
- [ ] Verify UI shows guidance to accept on crowdsec.net
- [ ] Accept enrollment on crowdsec.net
- [ ] Verify engine appears in dashboard

### Live Logs Testing
- [ ] Open Live Log Viewer page
- [ ] Verify WebSocket connects (check Network tab)
- [ ] Verify "Connected" badge shows
- [ ] Generate some logs (make HTTP request to proxy)
- [ ] Verify logs appear in viewer
- [ ] Test disconnect/reconnect behavior

---

## References

- [CrowdSec Console Documentation](https://docs.crowdsec.net/docs/console/)
- [WEBSOCKET_FIX_SUMMARY.md](../../WEBSOCKET_FIX_SUMMARY.md)
- [cerberus.md - Console Enrollment](../../docs/cerberus.md)

---
---

# PREVIOUS ANALYSIS (Resolved Issues - Kept for Reference)

---

## Issue 1: CrowdSec Card Toggle Broken on Cerberus Dashboard

### Symptoms
- CrowdSec card shows "Active" but toggle doesn't work properly
- Shows "on and active" but CrowdSec is NOT actually on

### Root Cause Analysis

**Files Involved:**
- [frontend/src/pages/Security.tsx](frontend/src/pages/Security.tsx#L69-L110) - `crowdsecPowerMutation`
- [frontend/src/api/crowdsec.ts](frontend/src/api/crowdsec.ts#L5-L18) - `startCrowdsec`, `stopCrowdsec`, `statusCrowdsec`
- [backend/internal/api/handlers/security_handler.go](backend/internal/api/handlers/security_handler.go#L61-L137) - `GetStatus()`
- [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go#L140-L206) - `Start()`, `Stop()`, `Status()`

**The Problem:**

1. **Dual-Source State Conflict**: The `GetStatus()` endpoint in [security_handler.go#L61-L137](backend/internal/api/handlers/security_handler.go#L61-L137) combines state from TWO sources:
   - `settings` table: `security.crowdsec.enabled` and `security.crowdsec.mode`
   - `security_configs` table: `CrowdSecMode` field

2. **Toggle Updates Wrong Store**: When the user toggles CrowdSec via `crowdsecPowerMutation`:
   - It calls `updateSetting('security.crowdsec.enabled', ...)` which updates the `settings` table
   - It calls `startCrowdsec()` / `stopCrowdsec()` which updates `security_configs.CrowdSecMode`

3. **State Priority Mismatch**: In [security_handler.go#L100-L108](backend/internal/api/handlers/security_handler.go#L100-L108):
   ```go
   // CrowdSec enabled override (from settings table)
   if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.enabled").Scan(&setting).Error; err == nil && setting.Value != "" {
       if strings.EqualFold(setting.Value, "true") {
           crowdSecMode = "local"
       } else {
           crowdSecMode = "disabled"
       }
   }
   ```
   The `settings` table overrides `security_configs`, but the `Start()` handler updates `security_configs`.

4. **Process State Not Verified**: The frontend shows "Active" based on `status.crowdsec.enabled` from the API, but this is computed from DB settings, NOT from actual process status. The `crowdsecStatus` state (line 43-44) fetches real process status but this is a **separate query** displayed below the card.

### The Fix

**Backend ([security_handler.go](backend/internal/api/handlers/security_handler.go)):**
- `GetStatus()` should check actual CrowdSec process status via the `CrowdsecExecutor.Status()` call, not just DB state

**Frontend ([Security.tsx](frontend/src/pages/Security.tsx)):**
- The toggle's `checked` state should use `crowdsecStatus?.running` (actual process state) instead of `status.crowdsec.enabled` (DB setting)
- Or sync both states properly after toggle

---

## Issue 2: Live Log Viewer Shows "Disconnected" But Logs Appear

### Symptoms
- Shows "Disconnected" status badge but logs ARE appearing
- Navigating away and back causes logs to disappear

### Root Cause Analysis

**Files Involved:**
- [frontend/src/components/LiveLogViewer.tsx](frontend/src/components/LiveLogViewer.tsx#L146-L240)
- [frontend/src/api/logs.ts](frontend/src/api/logs.ts#L95-L174) - `connectLiveLogs`, `connectSecurityLogs`

**The Problem:**

1. **Connection State Race Condition**: In [LiveLogViewer.tsx#L165-L240](frontend/src/components/LiveLogViewer.tsx#L165-L240):
   ```tsx
   useEffect(() => {
     // Close existing connection
     if (closeConnectionRef.current) {
       closeConnectionRef.current();
       closeConnectionRef.current = null;
     }
     // ... setup handlers ...
     return () => {
       if (closeConnectionRef.current) {
         closeConnectionRef.current();
         closeConnectionRef.current = null;
       }
       setIsConnected(false);  // <-- Issue: cleanup runs AFTER effect re-runs
     };
   }, [currentMode, filters, securityFilters, isPaused, maxLogs, showBlockedOnly]);
   ```

2. **Dependency Array Includes `isPaused`**: When `isPaused` changes, the entire effect re-runs, creating a new WebSocket. But the cleanup of the old connection sets `isConnected(false)` AFTER the new connection's `onOpen` sets `isConnected(true)`, causing a flash of "Disconnected".

3. **Logs Disappear on Navigation**: The `logs` state is stored locally in the component via `useState<DisplayLogEntry[]>([])`. When the component unmounts (navigation) and remounts, state resets to empty array. There's no persistence or caching.

### The Fix

**[LiveLogViewer.tsx](frontend/src/components/LiveLogViewer.tsx):**

1. **Fix State Race**: Use a ref to track connection state transitions:
   ```tsx
   const connectionIdRef = useRef(0);
   // In effect: increment connectionId, check it in callbacks
   ```

2. **Remove `isPaused` from Dependencies**: Pausing should NOT close/reopen the WebSocket. Instead, just skip adding messages when paused:
   ```tsx
   // Current (wrong): connection is in dependency array
   // Fixed: only filter/process messages based on isPaused flag
   ```

3. **Persist Logs Across Navigation**: Either:
   - Store logs in React Query cache
   - Use a global store (zustand/context)
   - Accept the limitation with a "Logs cleared on navigation" note

---

## Issue 3: DEPRECATED CrowdSec Mode Toggle Still in UI

### Symptoms
- CrowdSec config page shows "Disabled/Local/External" mode toggle
- This is confusing because CrowdSec should run based SOLELY on the Feature Flag in System Settings

### Root Cause Analysis

**Files Involved:**
- [frontend/src/pages/CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx#L68-L100) - Mode toggle UI
- [frontend/src/pages/SystemSettings.tsx](frontend/src/pages/SystemSettings.tsx#L89-L107) - Feature flag toggle
- [backend/internal/models/security_config.go](backend/internal/models/security_config.go#L15) - `CrowdSecMode` field

**The Problem:**

1. **Redundant Control Surfaces**: There are THREE ways to control CrowdSec:
   - Feature Flag: `feature.cerberus.enabled` in Settings (System Settings page)
   - Per-Service Toggle: `security.crowdsec.enabled` in Settings (Security Dashboard)
   - Mode Toggle: `CrowdSecMode` in SecurityConfig (CrowdSec Config page)

2. **Deprecated UI Still Present**: In [CrowdSecConfig.tsx#L68-L100](frontend/src/pages/CrowdSecConfig.tsx#L68-L100):
   ```tsx
   <Card>
     <div className="flex items-center justify-between gap-4 flex-wrap">
       <div className="space-y-1">
         <h2 className="text-lg font-semibold">CrowdSec Mode</h2>
         <p className="text-sm text-gray-400">
           {isLocalMode ? 'CrowdSec runs locally...' : 'CrowdSec decisions are paused...'}
         </p>
       </div>
       <div className="flex items-center gap-3">
         <span className="text-sm text-gray-400">Disabled</span>
         <Switch
           checked={isLocalMode}
           onChange={(e) => handleModeToggle(e.target.checked)}
           ...
         />
         <span className="text-sm text-gray-200">Local</span>
       </div>
     </div>
   </Card>
   ```

3. **`isLocalMode` Derived from Wrong Source**: Line 28:
   ```tsx
   const isLocalMode = !!status && status.crowdsec?.mode !== 'disabled'
   ```
   This checks `mode` from `security_configs.CrowdSecMode`, not the feature flag.

4. **`handleModeToggle` Updates Wrong Setting**: Lines 72-77:
   ```tsx
   const handleModeToggle = (nextEnabled: boolean) => {
     const mode = nextEnabled ? 'local' : 'disabled'
     updateModeMutation.mutate(mode)  // Updates security.crowdsec.mode in settings
   }
   ```

### The Fix

**[CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx):**
1. **Remove the Mode Toggle Card entirely** (lines 68-100)
2. **Add a notice**: "CrowdSec is controlled via the toggle on the Security Dashboard or System Settings"

**Backend Cleanup (optional future work):**
- Remove `CrowdSecMode` field from SecurityConfig model
- Migrate all state to use only `security.crowdsec.enabled` setting

---

## Issue 4: Enrollment Shows "CrowdSec is not running"

### Symptoms
- CrowdSec enrollment shows error even when enabled
- Red warning box: "CrowdSec is not running"

### Root Cause Analysis

**Files Involved:**
- [frontend/src/pages/CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx#L30-L45) - `lapiStatusQuery`
- [frontend/src/pages/CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx#L172-L196) - Warning display logic
- [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go#L252-L275) - `Status()`

**The Problem:**

1. **LAPI Status Query Uses Wrong Condition**: In [CrowdSecConfig.tsx#L30-L40](frontend/src/pages/CrowdSecConfig.tsx#L30-L40):
   ```tsx
   const lapiStatusQuery = useQuery<CrowdSecStatus>({
     queryKey: ['crowdsec-lapi-status'],
     queryFn: statusCrowdsec,
     enabled: consoleEnrollmentEnabled && initialCheckComplete,
     refetchInterval: 5000,
     retry: false,
   })
   ```
   The query is `enabled` only when `consoleEnrollmentEnabled` (feature flag for console enrollment).

2. **Warning Shows When Process Not Running**: In [CrowdSecConfig.tsx#L172-L196](frontend/src/pages/CrowdSecConfig.tsx#L172-L196):
   ```tsx
   {lapiStatusQuery.data && !lapiStatusQuery.data.running && initialCheckComplete && (
     <div className="..." data-testid="lapi-not-running-warning">
       <p>CrowdSec is not running</p>
       ...
     </div>
   )}
   ```
   This shows when `lapiStatusQuery.data.running === false`.

3. **Status Check May Return Stale Data**: The `Status()` backend handler checks:
   - PID file existence
   - Process status via `kill -0`
   - LAPI health via `cscli lapi status`

   But if CrowdSec was just enabled, there may be a race condition where the settings say "enabled" but the process hasn't started yet.

4. **Startup Reconciliation Timing**: `ReconcileCrowdSecOnStartup()` in [crowdsec_startup.go](backend/internal/services/crowdsec_startup.go) runs at container start, but if the user enables CrowdSec AFTER startup, the process won't auto-start.

### The Fix

**[CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx):**

1. **Improve Warning Message**: The "not running" warning should include:
   - A "Start CrowdSec" button that calls `startCrowdsec()` API
   - Or a link to the Security Dashboard where the toggle is

2. **Check Both States**: Show the warning only when:
   - User has enabled CrowdSec (via either toggle)
   - AND the process is not running

3. **Add Auto-Retry**: After enabling CrowdSec, poll status more aggressively for 30 seconds

---

## Implementation Plan

### Phase 1: Backend Fixes (Priority: High)

#### 1.1 Unify State Source
**File**: [backend/internal/api/handlers/security_handler.go](backend/internal/api/handlers/security_handler.go)

**Change**: Modify `GetStatus()` to include actual process status:
```go
// Add after line 137:
// Check actual CrowdSec process status
if h.crowdsecExecutor != nil {
    ctx := c.Request.Context()
    running, pid, _ := h.crowdsecExecutor.Status(ctx, h.dataDir)
    // Override enabled state based on actual process
    crowdsecProcessRunning = running
}
```

Add `crowdsecExecutor` field to `SecurityHandler` struct and inject it during initialization.

#### 1.2 Consistent Mode Updates
**File**: [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go)

**Change**: In `Start()` and `Stop()`, also update the `settings` table:
```go
// In Start(), after updating SecurityConfig (line ~165):
if h.DB != nil {
    setting := models.Setting{Key: "security.crowdsec.enabled", Value: "true", Category: "security", Type: "bool"}
    h.DB.Where(models.Setting{Key: "security.crowdsec.enabled"}).Assign(setting).FirstOrCreate(&setting)
}

// In Stop(), after updating SecurityConfig (line ~228):
if h.DB != nil {
    setting := models.Setting{Key: "security.crowdsec.enabled", Value: "false", Category: "security", Type: "bool"}
    h.DB.Where(models.Setting{Key: "security.crowdsec.enabled"}).Assign(setting).FirstOrCreate(&setting)
}
```

### Phase 2: Frontend Fixes (Priority: High)

#### 2.1 Fix CrowdSec Toggle State
**File**: [frontend/src/pages/Security.tsx](frontend/src/pages/Security.tsx)

**Change 1**: Use actual process status for toggle (around line 203):
```tsx
// Replace: checked={status.crowdsec.enabled}
// With:
checked={crowdsecStatus?.running ?? status.crowdsec.enabled}
```

**Change 2**: After successful toggle, refetch both status and process status

#### 2.2 Fix LiveLogViewer Connection State
**File**: [frontend/src/components/LiveLogViewer.tsx](frontend/src/components/LiveLogViewer.tsx)

**Change 1**: Remove `isPaused` from useEffect dependencies (line 237):
```tsx
// Change from:
}, [currentMode, filters, securityFilters, isPaused, maxLogs, showBlockedOnly]);
// To:
}, [currentMode, filters, securityFilters, maxLogs, showBlockedOnly]);
```

**Change 2**: Handle pause inside message handler (line 192):
```tsx
const handleMessage = (entry: SecurityLogEntry) => {
  // isPaused check stays here, not in effect
  if (isPausedRef.current) return;  // Use ref instead of state
  // ... rest of handler
};
```

**Change 3**: Add ref for isPaused:
```tsx
const isPausedRef = useRef(isPaused);
useEffect(() => { isPausedRef.current = isPaused; }, [isPaused]);
```

#### 2.3 Remove Deprecated Mode Toggle
**File**: [frontend/src/pages/CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx)

**Change**: Remove the entire "CrowdSec Mode" Card (lines 291-311 in current render):
```tsx
// DELETE: The entire <Card> block containing "CrowdSec Mode"
```

Add informational banner instead:
```tsx
{/* Replace mode toggle with info banner */}
<div className="bg-blue-900/20 border border-blue-700 rounded-lg p-4">
  <p className="text-sm text-blue-200">
    <strong>Note:</strong> CrowdSec is controlled via the toggle on the{' '}
    <Link to="/security" className="underline">Security Dashboard</Link>.
    Enable/disable CrowdSec there, then configure presets and files here.
  </p>
</div>
```

#### 2.4 Fix Enrollment Warning
**File**: [frontend/src/pages/CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx)

**Change**: Add "Start CrowdSec" button to the warning (around line 185):
```tsx
<Button
  variant="primary"
  size="sm"
  onClick={async () => {
    try {
      await startCrowdsec();
      toast.info('Starting CrowdSec...');
      lapiStatusQuery.refetch();
    } catch (err) {
      toast.error('Failed to start CrowdSec');
    }
  }}
>
  Start CrowdSec
</Button>
```

### Phase 3: Remove Deprecated Mode (Priority: Medium)

#### 3.1 Backend Model Cleanup (Future)
**File**: [backend/internal/models/security_config.go](backend/internal/models/security_config.go)

Mark `CrowdSecMode` as deprecated with migration path.

#### 3.2 Settings Migration
Create migration to ensure all users have `security.crowdsec.enabled` setting derived from `CrowdSecMode`.

---

## Files to Modify Summary

### Backend
| File | Changes |
|------|---------|
| `backend/internal/api/handlers/security_handler.go` | Add process status check to `GetStatus()` |
| `backend/internal/api/handlers/crowdsec_handler.go` | Sync `settings` table in `Start()`/`Stop()` |

### Frontend
| File | Changes |
|------|---------|
| `frontend/src/pages/Security.tsx` | Use `crowdsecStatus?.running` for toggle state |
| `frontend/src/components/LiveLogViewer.tsx` | Fix `isPaused` dependency, use ref |
| `frontend/src/pages/CrowdSecConfig.tsx` | Remove mode toggle, add info banner, add "Start CrowdSec" button |

---

## Testing Checklist

- [ ] Toggle CrowdSec on Security Dashboard → verify process starts
- [ ] Toggle CrowdSec off → verify process stops
- [ ] Refresh page → verify toggle state matches process state
- [ ] Open LiveLogViewer → verify "Connected" status
- [ ] Pause logs → verify connection remains open
- [ ] Navigate away and back → logs are cleared (expected) but connection re-establishes
- [ ] CrowdSec Config page → no mode toggle, info banner present
- [ ] Enrollment section → shows "Start CrowdSec" button when process not running
