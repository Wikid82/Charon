# CrowdSec LAPI Status Bug - Diagnostic & Fix Plan

**Date:** December 14, 2025
**Issue:** CrowdSecConfig page persistently shows "LAPI is initializing" even when LAPI is running
**Status:** 🎯 **ROOT CAUSE IDENTIFIED** - Status endpoint checks process, not LAPI connectivity
**Priority:** HIGH (Blocks Console Enrollment Feature)
**Previous Issue:** [crowdsec_lapi_error_diagnostic.md](crowdsec_lapi_error_diagnostic.md) - Race condition fix introduced this regression

---

## 🎯 Key Findings

### Critical Discovery

After implementing fixes from `docs/plans/crowdsec_lapi_error_diagnostic.md`, the CrowdSecConfig page now persistently displays:

> "CrowdSec Local API is initializing...
> The CrowdSec process is running but the Local API (LAPI) is still starting up."

This message appears **even when LAPI is actually running and reachable**. The fix introduced a regression where the Status endpoint was not updated to match the new LAPI-aware Start endpoint.

### Root Cause Chain

1. `Start()` handler was correctly updated to wait for LAPI and return `lapi_ready: true/false`
2. **BUT** `Status()` handler was **NOT updated** - still only checks process status
3. Frontend expects `running` to mean "LAPI responding"
4. Backend returns `running: true` meaning only "process running"
5. **MISMATCH:** Frontend needs `lapi_ready` field to determine actual LAPI status

### Why This is a Regression

- The original fix added LAPI readiness check to `Start()` handler ✅
- But forgot to add the same check to `Status()` handler ❌
- Frontend now uses `statusCrowdsec()` for polling LAPI status
- This endpoint doesn't actually verify LAPI connectivity

### Impact

- Console enrollment section always shows "initializing" warning
- Enroll button is disabled even when LAPI is working
- Users cannot complete console enrollment despite CrowdSec being functional

---

## Executive Summary

The `Start()` handler was correctly updated to wait for LAPI readiness before returning (lines 201-236 in [crowdsec_handler.go](../../backend/internal/api/handlers/crowdsec_handler.go#L201-L236)):

```go
// Start() now waits for LAPI and returns lapi_ready: true/false
c.JSON(http.StatusOK, gin.H{
    "status":     "started",
    "pid":        pid,
    "lapi_ready": true,  // NEW: indicates LAPI is ready
})
```

However, the `Status()` handler was **NOT updated** and still only checks process status (lines 287-294):

```go
func (h *CrowdsecHandler) Status(c *gin.Context) {
    ctx := c.Request.Context()
    running, pid, err := h.Executor.Status(ctx, h.DataDir)  // Only checks PID!
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"running": running, "pid": pid})  // Missing lapi_ready!
}
```

---

## Root Cause Analysis

### The Executor's Status() Method

The `DefaultCrowdsecExecutor.Status()` in [crowdsec_exec.go](../../backend/internal/api/handlers/crowdsec_exec.go#L65-L87) only checks:

1. If PID file exists
2. If process with that PID is running (via signal 0)

```go
func (e *DefaultCrowdsecExecutor) Status(ctx context.Context, configDir string) (running bool, pid int, err error) {
    b, err := os.ReadFile(e.pidFile(configDir))
    if err != nil {
        // Missing pid file is treated as not running
        return false, 0, nil
    }
    // ... check if process is alive via signal 0 ...
    return true, pid, nil
}
```

It does **NOT** check if LAPI HTTP endpoint is responding.

### Frontend Expectation Mismatch

The frontend in [CrowdSecConfig.tsx](../../frontend/src/pages/CrowdSecConfig.tsx#L71-L77) queries LAPI status:

```tsx
const lapiStatusQuery = useQuery({
  queryKey: ['crowdsec-lapi-status'],
  queryFn: statusCrowdsec,
  enabled: consoleEnrollmentEnabled && initialCheckComplete,
  refetchInterval: 5000, // Poll every 5 seconds
  retry: false,
})
```

And displays a warning based on `running` field (lines 207-231):

```tsx
{lapiStatusQuery.data && !lapiStatusQuery.data.running && initialCheckComplete && (
  <div className="..." data-testid="lapi-warning">
    <p>CrowdSec Local API is initializing...</p>
  </div>
)}
```

**The Problem:** The frontend checks `lapiStatusQuery.data?.running` expecting it to indicate LAPI connectivity. But the backend returns `running: true` which only means "process is running", not "LAPI is responding".

### Evidence Chain

| Component | File | Line | Returns | Actually Checks |
|-----------|------|------|---------|-----------------|
| Backend Handler | crowdsec_handler.go | 287-294 | `{running, pid}` | Process running via PID |
| Backend Executor | crowdsec_exec.go | 65-87 | `(running, pid, err)` | PID file + signal 0 |
| Frontend API | crowdsec.ts | 18-21 | `resp.data` | N/A (passthrough) |
| Frontend Query | CrowdSecConfig.tsx | 71-77 | `lapiStatusQuery.data` | Checks `.running` field |
| Frontend UI | CrowdSecConfig.tsx | 207-231 | Shows warning | `!running` |

**Bug:** Frontend interprets `running` as "LAPI responding" but backend returns "process running".

---

## Detailed Analysis: Why Warning Always Shows

Looking at the conditional again:

```tsx
{lapiStatusQuery.data && !lapiStatusQuery.data.running && initialCheckComplete && (
```

This shows the warning when:
- `lapiStatusQuery.data` is truthy ✓
- `!lapiStatusQuery.data.running` is truthy (i.e., `running` is falsy)
- `initialCheckComplete` is truthy ✓

**Re-analyzing:** If `running: true`, then `!true = false`, so warning should NOT show.

**But user reports it DOES show!**

**Possible causes:**

1. **Process not actually running:** The `Status()` endpoint returns `running: false` because CrowdSec process crashed or PID file is missing/stale
2. **Different `running` field:** Frontend might be checking a different property
3. **Query state issue:** React Query might be returning stale data

**Most Likely:** Looking at the message being displayed:

> "CrowdSec Local API is **initializing**..."

This message was designed for the case where **process IS running** but **LAPI is NOT ready yet**. But the current conditional shows it when `running` is false!

**The Fix Needed:** The conditional should check:
- Process running (`running: true`) AND
- LAPI not ready (`lapi_ready: false`)

NOT just:
- Process not running (`running: false`)

---

## The Complete Fix

### Files to Modify

1. **Backend:** [backend/internal/api/handlers/crowdsec_handler.go](../../backend/internal/api/handlers/crowdsec_handler.go#L287-L294)
2. **Frontend API:** [frontend/src/api/crowdsec.ts](../../frontend/src/api/crowdsec.ts#L18-L21)
3. **Frontend UI:** [frontend/src/pages/CrowdSecConfig.tsx](../../frontend/src/pages/CrowdSecConfig.tsx#L207-L231)
4. **Tests:** [backend/internal/api/handlers/crowdsec_handler_test.go](../../backend/internal/api/handlers/crowdsec_handler_test.go)

### Change 1: Backend Status Handler

**File:** `backend/internal/api/handlers/crowdsec_handler.go`
**Location:** Lines 287-294

**Before:**
```go
// Status returns simple running state.
func (h *CrowdsecHandler) Status(c *gin.Context) {
	ctx := c.Request.Context()
	running, pid, err := h.Executor.Status(ctx, h.DataDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"running": running, "pid": pid})
}
```

**After:**
```go
// Status returns running state including LAPI availability check.
func (h *CrowdsecHandler) Status(c *gin.Context) {
	ctx := c.Request.Context()
	running, pid, err := h.Executor.Status(ctx, h.DataDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Check LAPI connectivity if process is running
	lapiReady := false
	if running {
		args := []string{"lapi", "status"}
		if _, err := os.Stat(filepath.Join(h.DataDir, "config.yaml")); err == nil {
			args = append([]string{"-c", filepath.Join(h.DataDir, "config.yaml")}, args...)
		}
		checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		_, checkErr := h.CmdExec.Execute(checkCtx, "cscli", args...)
		cancel()
		lapiReady = (checkErr == nil)
	}

	c.JSON(http.StatusOK, gin.H{
		"running":    running,
		"pid":        pid,
		"lapi_ready": lapiReady,
	})
}
```

### Change 2: Frontend API Type

**File:** `frontend/src/api/crowdsec.ts`
**Location:** Lines 18-21

**Before:**
```typescript
export async function statusCrowdsec() {
  const resp = await client.get('/admin/crowdsec/status')
  return resp.data
}
```

**After:**
```typescript
export interface CrowdSecStatus {
  running: boolean
  pid: number
  lapi_ready: boolean
}

export async function statusCrowdsec(): Promise<CrowdSecStatus> {
  const resp = await client.get<CrowdSecStatus>('/admin/crowdsec/status')
  return resp.data
}
```

### Change 3: Frontend CrowdSecConfig Conditional Logic

**File:** `frontend/src/pages/CrowdSecConfig.tsx`
**Location:** Lines 207-231

**Before:**
```tsx
{/* Warning when CrowdSec LAPI is not running */}
{lapiStatusQuery.data && !lapiStatusQuery.data.running && initialCheckComplete && (
  <div className="flex items-start gap-3 p-4 bg-yellow-900/20 border border-yellow-700/50 rounded-lg" data-testid="lapi-warning">
    <AlertTriangle className="w-5 h-5 text-yellow-400 flex-shrink-0 mt-0.5" />
    <div className="flex-1">
      <p className="text-sm text-yellow-200 font-medium mb-2">
        CrowdSec Local API is initializing...
      </p>
      <p className="text-xs text-yellow-300 mb-3">
        The CrowdSec process is running but the Local API (LAPI) is still starting up.
        This typically takes 5-10 seconds after enabling CrowdSec.
        {lapiStatusQuery.isRefetching && ' Checking again in 5 seconds...'}
      </p>
      <div className="flex gap-2">
        <Button
          variant="secondary"
          size="sm"
          onClick={() => lapiStatusQuery.refetch()}
          disabled={lapiStatusQuery.isRefetching}
        >
          Check Now
        </Button>
        {!status?.crowdsec?.enabled && (
          <Button
            variant="secondary"
            size="sm"
            onClick={() => navigate('/security')}
          >
            Go to Security Dashboard
          </Button>
        )}
      </div>
    </div>
  </div>
)}
```

**After:**
```tsx
{/* Warning when CrowdSec process is running but LAPI is not ready */}
{lapiStatusQuery.data && lapiStatusQuery.data.running && !lapiStatusQuery.data.lapi_ready && initialCheckComplete && (
  <div className="flex items-start gap-3 p-4 bg-yellow-900/20 border border-yellow-700/50 rounded-lg" data-testid="lapi-warning">
    <AlertTriangle className="w-5 h-5 text-yellow-400 flex-shrink-0 mt-0.5" />
    <div className="flex-1">
      <p className="text-sm text-yellow-200 font-medium mb-2">
        CrowdSec Local API is initializing...
      </p>
      <p className="text-xs text-yellow-300 mb-3">
        The CrowdSec process is running but the Local API (LAPI) is still starting up.
        This typically takes 5-10 seconds after enabling CrowdSec.
        {lapiStatusQuery.isRefetching && ' Checking again in 5 seconds...'}
      </p>
      <div className="flex gap-2">
        <Button
          variant="secondary"
          size="sm"
          onClick={() => lapiStatusQuery.refetch()}
          disabled={lapiStatusQuery.isRefetching}
        >
          Check Now
        </Button>
      </div>
    </div>
  </div>
)}

{/* Warning when CrowdSec is not running at all */}
{lapiStatusQuery.data && !lapiStatusQuery.data.running && initialCheckComplete && (
  <div className="flex items-start gap-3 p-4 bg-red-900/20 border border-red-700/50 rounded-lg" data-testid="crowdsec-not-running-warning">
    <AlertTriangle className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" />
    <div className="flex-1">
      <p className="text-sm text-red-200 font-medium mb-2">
        CrowdSec is not running
      </p>
      <p className="text-xs text-red-300 mb-3">
        Please enable CrowdSec using the toggle switch in the Security dashboard before enrolling in the Console.
      </p>
      <Button
        variant="secondary"
        size="sm"
        onClick={() => navigate('/security')}
      >
        Go to Security Dashboard
      </Button>
    </div>
  </div>
)}
```

### Change 4: Update Enrollment Button Disabled State

**File:** `frontend/src/pages/CrowdSecConfig.tsx`
**Location:** Lines 255-289 (Enroll, Rotate key, and Retry enrollment buttons)

**Before:**
```tsx
disabled={isConsolePending || (lapiStatusQuery.data && !lapiStatusQuery.data.running) || !enrollmentToken.trim()}
```

**After:**
```tsx
disabled={isConsolePending || (lapiStatusQuery.data && !lapiStatusQuery.data.lapi_ready) || !enrollmentToken.trim()}
```

Also update the `title` attributes:

**Before:**
```tsx
title={
  lapiStatusQuery.data && !lapiStatusQuery.data.running
    ? 'CrowdSec LAPI must be running to enroll'
    : ...
}
```

**After:**
```tsx
title={
  lapiStatusQuery.data && !lapiStatusQuery.data.lapi_ready
    ? 'CrowdSec LAPI must be running to enroll'
    : ...
}
```

---

## Testing Steps

### Unit Test: Backend Status Handler

Add test in `backend/internal/api/handlers/crowdsec_handler_test.go`:

```go
func TestCrowdsecHandler_Status_IncludesLAPIReady(t *testing.T) {
    mockExec := &fakeExec{running: true, pid: 1234}
    mockCmdExec := &mockCommandExecutor{returnErr: nil} // cscli lapi status succeeds

    handler := &CrowdsecHandler{
        Executor: mockExec,
        CmdExec:  mockCmdExec,
        DataDir:  "/app/data",
    }

    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = httptest.NewRequest(http.MethodGet, "/admin/crowdsec/status", nil)

    handler.Status(c)

    assert.Equal(t, http.StatusOK, w.Code)

    var response map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &response)

    assert.True(t, response["running"].(bool))
    assert.Equal(t, float64(1234), response["pid"].(float64))
    assert.True(t, response["lapi_ready"].(bool)) // NEW: Check lapi_ready is present and true
}

func TestCrowdsecHandler_Status_LAPINotReady(t *testing.T) {
    mockExec := &fakeExec{running: true, pid: 1234}
    mockCmdExec := &mockCommandExecutor{returnErr: errors.New("connection refused")} // cscli lapi status fails

    handler := &CrowdsecHandler{
        Executor: mockExec,
        CmdExec:  mockCmdExec,
        DataDir:  "/app/data",
    }

    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = httptest.NewRequest(http.MethodGet, "/admin/crowdsec/status", nil)

    handler.Status(c)

    assert.Equal(t, http.StatusOK, w.Code)

    var response map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &response)

    assert.True(t, response["running"].(bool))
    assert.Equal(t, float64(1234), response["pid"].(float64))
    assert.False(t, response["lapi_ready"].(bool)) // LAPI not ready
}

func TestCrowdsecHandler_Status_ProcessNotRunning(t *testing.T) {
    mockExec := &fakeExec{running: false, pid: 0}
    mockCmdExec := &mockCommandExecutor{}

    handler := &CrowdsecHandler{
        Executor: mockExec,
        CmdExec:  mockCmdExec,
        DataDir:  "/app/data",
    }

    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = httptest.NewRequest(http.MethodGet, "/admin/crowdsec/status", nil)

    handler.Status(c)

    assert.Equal(t, http.StatusOK, w.Code)

    var response map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &response)

    assert.False(t, response["running"].(bool))
    assert.False(t, response["lapi_ready"].(bool)) // LAPI can't be ready if process not running
}
```

### Manual Testing Procedure

1. **Start Fresh:**
   ```bash
   docker compose down -v
   docker compose up -d
   ```

2. **Enable CrowdSec:**
   - Go to Security dashboard
   - Toggle CrowdSec ON
   - Wait for toast "CrowdSec started and LAPI is ready"

3. **Navigate to Config:**
   - Click "Config" button
   - Verify NO "initializing" warning shows
   - Console enrollment section should be enabled

4. **Verify API Response:**
   ```bash
   curl -s http://localhost:8080/api/v1/admin/crowdsec/status | jq
   ```
   Expected:
   ```json
   {
     "running": true,
     "pid": 123,
     "lapi_ready": true
   }
   ```

5. **Test LAPI Down Scenario:**
   - SSH into container: `docker exec -it charon bash`
   - Stop CrowdSec: `pkill -f crowdsec`
   - Call API:
     ```bash
     curl -s http://localhost:8080/api/v1/admin/crowdsec/status | jq
     ```
   - Expected: `{"running": false, "pid": 0, "lapi_ready": false}`
   - Refresh CrowdSecConfig page
   - Should show "CrowdSec is not running" error (red)

6. **Test Restart Scenario:**
   - Re-enable CrowdSec via Security dashboard
   - Immediately navigate to CrowdSecConfig
   - Should show "initializing" briefly (yellow) then clear when `lapi_ready: true`

---

## Risk Assessment

| Change | Risk | Mitigation |
|--------|------|------------|
| Backend Status handler modification | Low | Status handler is read-only, adds 2s timeout check |
| LAPI check timeout (2s) | Low | Short timeout prevents blocking; async refresh handles retries |
| Frontend conditional logic change | Low | More precise state handling, clear error states |
| Type definition update | Low | TypeScript will catch any mismatches at compile time |
| Two separate warning states | Low | Better UX with distinct yellow (initializing) vs red (not running) |

---

## Summary

**Root Cause:** The `Status()` endpoint was not updated when `Start()` was modified to check LAPI readiness. The frontend expects the status endpoint to indicate LAPI availability, but it only returns process status.

**Fix:** Add `lapi_ready` field to `Status()` response by checking `cscli lapi status`, update frontend to use this new field for the warning display logic.

**Files Changed:**
1. `backend/internal/api/handlers/crowdsec_handler.go` - Add LAPI check to Status()
2. `frontend/src/api/crowdsec.ts` - Add TypeScript interface with `lapi_ready`
3. `frontend/src/pages/CrowdSecConfig.tsx` - Update conditional logic:
   - Yellow warning: process running, LAPI not ready
   - Red warning: process not running
   - No warning: process running AND LAPI ready
4. `backend/internal/api/handlers/crowdsec_handler_test.go` - Add unit tests

**Estimated Time:** 1-2 hours including testing

**Commit Message:**
```
fix: add LAPI readiness check to CrowdSec status endpoint

The Status() handler was only checking if the CrowdSec process was
running, not if LAPI was actually responding. This caused the
CrowdSecConfig page to always show "LAPI is initializing" even when
LAPI was fully operational.

Changes:
- Backend: Add `lapi_ready` field to /admin/crowdsec/status response
- Frontend: Add CrowdSecStatus TypeScript interface
- Frontend: Update conditional logic to check `lapi_ready` not `running`
- Frontend: Separate warnings for "initializing" vs "not running"
- Tests: Add unit tests for Status handler LAPI check

Fixes regression from crowdsec_lapi_error_diagnostic.md fixes.
```
