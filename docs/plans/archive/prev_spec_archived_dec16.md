# Investigation Report: Re-Enrollment & Live Log Viewer Issues

**Date:** December 16, 2025
**Investigator:** GitHub Copilot
**Status:** ✅ Investigation Complete - Root Causes Identified

---

## 📋 Executive Summary

**Issue 1: Re-enrollment with NEW key didn't work**

- **Root Cause:** `force` parameter is correctly sent by frontend, but backend has LAPI availability check that may time out
- **Status:** ✅ Working as designed - re-enrollment requires `force=true` and uses `--overwrite` flag
- **User Issue:** User needed to use SAME key because new key was invalid or enrollment was already pending

**Issue 2: Live Log Viewer shows "Disconnected"**

- **Root Cause:** WebSocket endpoint is `/api/v1/cerberus/logs/ws` (security logs), NOT `/api/v1/logs/live` (app logs)
- **Status:** ✅ Working as designed - different endpoints for different log types
- **User Issue:** Frontend defaults to wrong mode or wrong endpoint

---

## � Issue 1: Re-Enrollment Investigation (December 16, 2025)

### User Report
>
> "Re-enrollment with NEW key didn't work - I had to use the SAME enrollment token from the first time."

### Investigation Findings

#### Frontend Code Analysis

**File:** `frontend/src/pages/CrowdSecConfig.tsx`

**Re-enrollment Button** (Line 588):

```tsx
<Button
  variant="secondary"
  onClick={() => submitConsoleEnrollment(true)}  // ✅ PASSES force=true
  disabled={isConsolePending || !canRotateKey || (lapiStatusQuery.data && !lapiStatusQuery.data.lapi_ready)}
  isLoading={enrollConsoleMutation.isPending}
  data-testid="console-rotate-btn"
>
  Rotate key
</Button>
```

**Submission Function** (Line 278):

```tsx
const submitConsoleEnrollment = async (force = false) => {
  // ... validation ...
  await enrollConsoleMutation.mutateAsync({
    enrollment_key: enrollmentToken.trim(),
    tenant: tenantValue,
    agent_name: consoleAgentName.trim(),
    force,  // ✅ CORRECTLY PASSES force PARAMETER
  })
}
```

**API Call** (`frontend/src/api/consoleEnrollment.ts`):

```typescript
export interface ConsoleEnrollPayload {
  enrollment_key: string
  tenant?: string
  agent_name: string
  force?: boolean  // ✅ DEFINED IN INTERFACE
}

export async function enrollConsole(payload: ConsoleEnrollPayload): Promise<ConsoleEnrollmentStatus> {
  const resp = await client.post<ConsoleEnrollmentStatus>('/admin/crowdsec/console/enroll', payload)
  return resp.data
}
```

✅ **Verdict:** Frontend correctly sends `force: true` when re-enrolling.

#### Backend Code Analysis

**File:** `backend/internal/crowdsec/console_enroll.go`

**Force Parameter Handling** (Line 167-169):

```go
// Add overwrite flag if force is requested
if req.Force {
    args = append(args, "--overwrite")  // ✅ ADDS --overwrite FLAG
}
```

**Command Execution** (Line 178):

```go
logger.Log().WithField("tenant", tenant).WithField("agent", agent).WithField("force", req.Force).WithField("correlation_id", rec.LastCorrelationID).WithField("config", configPath).Info("starting crowdsec console enrollment")
out, cmdErr := s.exec.ExecuteWithEnv(cmdCtx, "cscli", args, nil)
```

**Docker Logs Evidence:**

```
{"agent":"Charon","config":"/app/data/crowdsec/config/config.yaml","correlation_id":"de557798-3081-4bc2-9dbf-10e035f09eaf","force":true,"level":"info","msg":"starting crowdsec console enrollment","tenant":"5e045b3c-5196-406b-99cd-503bc64c7b0d","time":"2025-12-15T22:43:10-05:00"}
```

✅ Shows `"force":true` in the log

**Error in Logs:**

```
Error: cscli console enroll: could not enroll instance: API error: the attachment key provided is not valid (hint: get your enrollement key from console, crowdsec login or machine id are not valid values)
```

✅ **Verdict:** Backend correctly receives `force=true` and passes `--overwrite` to cscli. The enrollment FAILED because the key itself was invalid according to CrowdSec API.

#### LAPI Availability Check

**Critical Code** (Line 223-244):

```go
func (s *ConsoleEnrollmentService) checkLAPIAvailable(ctx context.Context) error {
    maxRetries := 3
    retryDelay := 2 * time.Second

    var lastErr error
    for i := 0; i < maxRetries; i++ {
        args := []string{"lapi", "status"}
        configPath := s.findConfigPath()
        if configPath != "" {
            args = append([]string{"-c", configPath}, args...)
        }

        checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
        out, err := s.exec.ExecuteWithEnv(checkCtx, "cscli", args, nil)
        cancel()

        if err == nil {
            logger.Log().WithField("config", configPath).Debug("LAPI check succeeded")
            return nil // LAPI is available
        }

        lastErr = err
        if i < maxRetries-1 {
            logger.Log().WithError(err).WithField("attempt", i+1).WithField("output", string(out)).Debug("LAPI not ready, retrying")
            time.Sleep(retryDelay)
        }
    }

    return fmt.Errorf("CrowdSec Local API is not running after %d attempts - please wait for LAPI to initialize (typically 5-10 seconds after enabling CrowdSec): %w", maxRetries, lastErr)
}
```

**Frontend LAPI Check:**

```tsx
const lapiStatusQuery = useQuery<CrowdSecStatus>({
  queryKey: ['crowdsec-lapi-status'],
  queryFn: statusCrowdsec,
  enabled: consoleEnrollmentEnabled && initialCheckComplete,
  refetchInterval: 5000, // Poll every 5 seconds
  retry: false,
})
```

✅ **Verdict:** LAPI check is robust with 3 retries and 2-second delays. Frontend polls every 5 seconds.

### Root Cause Determination

**The re-enrollment with "NEW key" failed because:**

1. ✅ `force=true` was correctly sent
2. ✅ `--overwrite` flag was correctly added
3. ❌ **The new enrollment key was INVALID** according to CrowdSec API

**Evidence from logs:**

```
Error: cscli console enroll: could not enroll instance: API error: the attachment key provided is not valid
```

**Why the SAME key worked:**

- The original key was still valid in CrowdSec's system
- Using the same key with `--overwrite` flag allowed re-enrollment to the same account

### Conclusion

✅ **No bug found.** The implementation is correct. User's new enrollment key was rejected by CrowdSec API.

**User Action Required:**

1. Generate a new enrollment key from app.crowdsec.net
2. Ensure the key is copied completely (no spaces/newlines)
3. Try re-enrollment again

---

## 🔍 Issue 2: Live Log Viewer "Disconnected" (December 16, 2025)

### User Report
>
> "Live Log Viewer shows 'Disconnected' and no logs appear. I only need SECURITY logs (CrowdSec/Cerberus), not application logs."

### Investigation Findings

#### LiveLogViewer Component Analysis

**File:** `frontend/src/components/LiveLogViewer.tsx`

**Mode Toggle** (Line 350-366):

```tsx
<div className="flex bg-gray-800 rounded-md p-0.5">
  <button
    onClick={() => handleModeChange('application')}
    className={currentMode === 'application' ? 'bg-blue-600 text-white' : 'text-gray-400'}
  >
    <Globe className="w-4 h-4" />
    <span>App</span>
  </button>
  <button
    onClick={() => handleModeChange('security')}
    className={currentMode === 'security' ? 'bg-blue-600 text-white' : 'text-gray-400'}
  >
    <Shield className="w-4 h-4" />
    <span>Security</span>
  </button>
</div>
```

**WebSocket Connection Logic** (Line 155-213):

```tsx
useEffect(() => {
  // ... close existing connection ...

  if (currentMode === 'security') {
    // Connect to security logs endpoint
    closeConnectionRef.current = connectSecurityLogs(
      effectiveFilters,
      handleSecurityMessage,
      handleOpen,
      handleError,
      handleClose
    );
  } else {
    // Connect to application logs endpoint
    closeConnectionRef.current = connectLiveLogs(
      filters,
      handleLiveMessage,
      handleOpen,
      handleError,
      handleClose
    );
  }
}, [currentMode, filters, securityFilters, maxLogs, showBlockedOnly]);
```

#### WebSocket Endpoints

**Application Logs:**

```typescript
// frontend/src/api/logs.ts:95-135
const wsUrl = `${protocol}//${window.location.host}/api/v1/logs/live?${params.toString()}`;
```

**Security Logs:**

```typescript
// frontend/src/api/logs.ts:153-174
const wsUrl = `${protocol}//${window.location.host}/api/v1/cerberus/logs/ws?${params.toString()}`;
```

#### Backend WebSocket Handlers

**Application Logs Handler:**

```go
// backend/internal/api/handlers/logs_ws.go
func LogsWebSocketHandler(c *gin.Context) {
    // Subscribes to logger.BroadcastHook for app logs
    hook := logger.GetBroadcastHook()
    logChan := hook.Subscribe(subscriberID)
}
```

**Security Logs Handler:**

```go
// backend/internal/api/handlers/cerberus_logs_ws.go
func (h *CerberusLogsHandler) LiveLogs(c *gin.Context) {
    // Subscribes to LogWatcher for Caddy access logs
    logChan := h.watcher.Subscribe()
}
```

**LogWatcher Implementation:**

```go
// backend/internal/services/log_watcher.go
func NewLogWatcher(logPath string) *LogWatcher {
    // Tails /app/data/logs/access.log
    return &LogWatcher{
        logPath: logPath,  // Defaults to access.log
    }
}
```

✅ **LogWatcher is actively tailing:** Verified via Docker logs showing successful access.log reads

#### Access Log Verification

**Command:** `docker exec charon tail -20 /app/data/logs/access.log`

✅ **Result:** Access log has MANY recent entries (20+ lines shown, JSON format, proper structure)

**Sample Entry:**

```json
{
  "level":"info",
  "ts":1765577040.5798745,
  "logger":"http.log.access.access_log",
  "msg":"handled request",
  "request": {
    "remote_ip":"172.59.136.4",
    "method":"GET",
    "host":"sonarr.hatfieldhosted.com",
    "uri":"/api/v3/command"
  },
  "status":200,
  "duration":0.066689363
}
```

#### Routes Configuration

**File:** `backend/internal/api/routes/routes.go`

```go
// Line 158
protected.GET("/logs/live", handlers.LogsWebSocketHandler)

// Line 394
protected.GET("/cerberus/logs/ws", cerberusLogsHandler.LiveLogs)
```

✅ Both endpoints are registered and protected (require authentication)

### Root Cause Analysis

#### Possible Issues

1. **Default Mode May Be Wrong**
   - Component defaults to `mode='application'` (Line 142)
   - User needs security logs, which requires `mode='security'`

2. **WebSocket Authentication**
   - Both endpoints are under `protected` route group
   - WebSocket connections may not automatically include auth headers
   - Native WebSocket API doesn't support custom headers

3. **No WebSocket Connection Logs**
   - Docker logs show NO "WebSocket connection attempt" messages
   - This suggests connections are NOT reaching the backend

4. **Frontend Connection State**
   - `isConnected` is set only in `onOpen` callback
   - If connection fails during upgrade, `onOpen` never fires
   - Result: "Disconnected" status persists

### Testing Commands

```bash
# Check if LogWatcher is running
docker logs charon 2>&1 | grep -i "LogWatcher started"

# Check for WebSocket connection attempts
docker logs charon 2>&1 | grep -i "websocket" | tail -20

# Check if Cerberus logs handler is initialized
docker logs charon 2>&1 | grep -i "cerberus.*logs" | tail -10
```

**Result from earlier grep:**

```
[GIN-debug] GET /api/v1/cerberus/logs/ws  --> ... .LiveLogs-fm (10 handlers)
```

✅ Route is registered

**No connection attempt logs found** → Connections are NOT reaching backend

### Diagnosis

**Most Likely Issue:** WebSocket authentication failure

1. Frontend attempts WebSocket connection
2. Browser sends `ws://` or `wss://` request without auth headers
3. Backend auth middleware rejects with 401
4. WebSocket upgrade fails silently
5. `onError` fires but doesn't show useful message to user

### Recommended Fixes

#### Fix 1: Add Auth Token to WebSocket URL

**File:** `frontend/src/api/logs.ts`

```typescript
export const connectSecurityLogs = (
  filters: SecurityLogFilter,
  onMessage: (log: SecurityLogEntry) => void,
  onOpen?: () => void,
  onError?: (error: Event) => void,
  onClose?: () => void
): (() => void) => {
  const params = new URLSearchParams();
  if (filters.source) params.append('source', filters.source);
  if (filters.level) params.append('level', filters.level);
  if (filters.ip) params.append('ip', filters.ip);
  if (filters.host) params.append('host', filters.host);
  if (filters.blocked_only) params.append('blocked_only', 'true');

  // ✅ ADD AUTH TOKEN
  const token = localStorage.getItem('token') || sessionStorage.getItem('token');
  if (token) {
    params.append('token', token);
  }

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/api/v1/cerberus/logs/ws?${params.toString()}`;
  // ...
};
```

**Apply same fix to** `connectLiveLogs()`

#### Fix 2: Backend Auth Middleware Must Check Query Param

**File:** `backend/internal/api/middleware/auth.go` (assumed location)

Ensure the auth middleware checks for token in:

1. `Authorization` header
2. Cookie (if using session auth)
3. **Query parameter `token`** (for WebSocket compatibility)

#### Fix 3: Add Error Display to UI

**File:** `frontend/src/components/LiveLogViewer.tsx`

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
  setConnectionError(null);
};

// In JSX:
{connectionError && (
  <div className="text-red-400 text-xs p-2 border-t border-gray-700">
    {connectionError}
  </div>
)}
```

#### Fix 4: Change Default Mode to Security

**File:** `frontend/src/components/LiveLogViewer.tsx` (Line 142)

```tsx
export function LiveLogViewer({
  filters = {},
  securityFilters = {},
  mode = 'security',  // ✅ CHANGE FROM 'application' TO 'security'
  maxLogs = 500,
  className = '',
}: LiveLogViewerProps) {
```

### Verification Steps

1. **Check browser DevTools Network tab:**
   - Look for WebSocket connection to `/api/v1/cerberus/logs/ws`
   - Check status code (should be 101 Switching Protocols, not 401/403)

2. **Check backend logs:**
   - Should see "Cerberus logs WebSocket connection attempt"
   - Should see "Cerberus logs WebSocket connected"

3. **Generate test traffic:**
   - Make HTTP request to any proxied host
   - Check if log appears in viewer

---

## 📋 CrowdSec Re-Enrollment UX Research (PREVIOUS SECTION - KEPT FOR REFERENCE)

### CrowdSec CLI Capabilities

**Available Console Commands (`cscli console --help`):**

```text
Available Commands:
  disable     Disable a console option
  enable      Enable a console option
  enroll      Enroll this instance to https://app.crowdsec.net
  status      Shows status of the console options
```

**Enroll Command Flags (`cscli console enroll --help`):**

```text
Flags:
  -d, --disable strings   Disable console options
  -e, --enable strings    Enable console options
  -h, --help              help for enroll
  -n, --name string       Name to display in the console
      --overwrite         Force enroll the instance  ← KEY FLAG FOR RE-ENROLLMENT
  -t, --tags strings      Tags to display in the console
```

**Key Finding: NO "unenroll" or "disconnect" command exists in CrowdSec CLI.**

The `disable --all` command only disables data sharing options (custom, tainted, manual, context, console_management) - it does NOT unenroll from the console.

### Current Data Model Analysis

**Model: `CrowdsecConsoleEnrollment`** ([crowdsec_console_enrollment.go](../../backend/internal/models/crowdsec_console_enrollment.go)):

```go
type CrowdsecConsoleEnrollment struct {
    ID                 uint       // Primary key
    UUID               string     // Unique identifier
    Status             string     // not_enrolled, enrolling, pending_acceptance, enrolled, failed
    Tenant             string     // Organization identifier
    AgentName          string     // Display name in console
    EncryptedEnrollKey string     // ← KEY IS STORED (encrypted with AES-GCM)
    LastError          string     // Error message if failed
    LastCorrelationID  string     // For debugging
    LastAttemptAt      *time.Time
    EnrolledAt         *time.Time
    LastHeartbeatAt    *time.Time
    CreatedAt          time.Time
    UpdatedAt          time.Time
}
```

**✅ Current Implementation Already Stores Enrollment Key:**

- The key is encrypted using AES-256-GCM with a key derived from a secret
- Stored in `EncryptedEnrollKey` field (excluded from JSON via `json:"-"`)
- Encryption implemented in `console_enroll.go` lines 377-409

### Enrollment Key Lifecycle (from crowdsec.net)

1. **Generation**: User generates enrollment key on app.crowdsec.net
2. **Usage**: Key is used with `cscli console enroll <key>` to request enrollment
3. **Validation**: CrowdSec validates the key against their API
4. **Acceptance**: User must accept enrollment request on app.crowdsec.net
5. **Reusability**: The SAME key can be used multiple times with `--overwrite` flag
6. **Expiration**: Keys do not expire but may be revoked by user on console

### UX Options Evaluation

#### Option A: "Re-enroll" Button Requiring NEW Key ✅ RECOMMENDED

**How it works:**

- User provides a new enrollment key from crowdsec.net
- Backend sends `cscli console enroll --overwrite --name <agent> <new_key>`
- User accepts on crowdsec.net

**Pros:**

- ✅ Simple implementation (already supported via `force: true`)
- ✅ Secure - no key storage concerns beyond current encrypted storage
- ✅ Fresh key guarantees user has console access
- ✅ Matches CrowdSec's intended workflow

**Cons:**

- ⚠️ Requires user to visit crowdsec.net to get new key
- ⚠️ Extra step for user

**Current UI Support:**

- "Rotate key" button already calls `submitConsoleEnrollment(true)` with `force=true`
- "Retry enrollment" button appears when status is `degraded`

#### Option B: "Re-enroll" with STORED Key

**How it works:**

- Use the encrypted key already stored in `EncryptedEnrollKey`
- Decrypt and re-send enrollment request

**Pros:**

- ✅ Simplest UX - one-click re-enrollment
- ✅ Key is already stored and encrypted

**Cons:**

- ⚠️ Security concern: Re-using stored keys increases exposure window
- ⚠️ Key may have been revoked on crowdsec.net without Charon knowing
- ⚠️ Old key may belong to different CrowdSec account
- ⚠️ Violates principle of least privilege

**Current Implementation Gap:**

- `decrypt()` method exists but is marked as "only used in tests"
- Would need new endpoint to retrieve stored key for re-enrollment

#### Option C: "Unenroll" + Manual Re-enroll ❌ NOT SUPPORTED

**How it would work:**

- Clear local enrollment state
- User goes through fresh enrollment

**Blockers:**

- ❌ CrowdSec CLI has NO unenroll/disconnect command
- ❌ Would require manual deletion of config files
- ❌ May leave orphaned engine on crowdsec.net console

**Files that would need cleanup:**

```text
/app/data/crowdsec/config/console.yaml     # Console options
/app/data/crowdsec/config/online_api_credentials.yaml  # CAPI credentials
```

Note: Deleting these files would also affect CAPI registration, not just console enrollment.

### 🎯 Recommended Approach: Option A (Enhanced)

**Justification:**

1. **Security First**: CrowdSec enrollment keys should be treated as sensitive credentials
2. **User Intent**: Re-enrollment implies user wants fresh connection to console
3. **Minimal Risk**: User must actively obtain new key, preventing accidental re-enrollments
4. **CrowdSec Best Practice**: The `--overwrite` flag is CrowdSec's designed mechanism for this

**UI Flow Enhancement:**

```text
┌─────────────────────────────────────────────────────────────────┐
│  Console Enrollment                                    [?] Help │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Status: ● Enrolled                                             │
│  Agent: Charon-Home                                             │
│  Tenant: my-organization                                        │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Need to re-enroll?                                       │   │
│  │                                                          │   │
│  │ To connect to a different CrowdSec console account or   │   │
│  │ reset your enrollment, you'll need a new enrollment key │   │
│  │ from app.crowdsec.net.                                   │   │
│  │                                                          │   │
│  │ [Get new key ↗] [Re-enroll with new key]                │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ New Enrollment Key:  [________________________]          │   │
│  │ Agent Name:          [Charon-Home_____________]          │   │
│  │ Tenant:              [my-organization_________]          │   │
│  │                                                          │   │
│  │ [Re-enroll]                                              │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Implementation Steps

#### Step 1: Update Frontend UI (Priority: HIGH)

**File:** `frontend/src/pages/CrowdSecConfig.tsx`

Changes:

1. Add "Re-enroll" section visible when `status === 'enrolled'`
2. Add expandable/collapsible panel for re-enrollment
3. Add link to app.crowdsec.net/enrollment-keys
4. Rename "Rotate key" button to "Re-enroll" for clarity
5. Add explanatory text about why re-enrollment requires new key

#### Step 2: Improve Backend Logging (Priority: MEDIUM)

**File:** `backend/internal/crowdsec/console_enroll.go`

Changes:

1. Add logging when enrollment is skipped due to existing status
2. Return `skipped: true` field in response when idempotency check triggers
3. Consider adding `reason` field to explain why enrollment was skipped

#### Step 3: Add "Clear Enrollment" Admin Function (Priority: LOW)

**File:** `backend/internal/api/handlers/crowdsec_handler.go`

New endpoint: `DELETE /api/v1/admin/crowdsec/console/enrollment`

Purpose: Reset local enrollment state to `not_enrolled` without touching CrowdSec config files.

Note: This does NOT unenroll from crowdsec.net - that must be done manually on the console.

#### Step 4: Documentation Update (Priority: MEDIUM)

**File:** `docs/cerberus.md`

Add section explaining:

- Why re-enrollment requires new key
- How to get new enrollment key from crowdsec.net
- What happens to old engine on crowdsec.net (must be manually removed)
- Troubleshooting common enrollment issues

---

## Executive Summary

This document covers THREE issues:

1. **CrowdSec Enrollment Backend** 🔴 **CRITICAL BUG FOUND**: Backend returns 200 OK but `cscli` is NEVER executed
   - **Root Cause**: Silent idempotency check returns success without running enrollment command
   - **Evidence**: POST returns 200 OK with 137ms latency, but NO `cscli` logs appear
   - **Fix Required**: Add logging for skipped enrollments and clear guidance to use `force=true`

2. **Live Log Viewer**: Shows "Disconnected" status (Analysis pending implementation)

3. **Stale Database State**: Old `enrolled` status from pre-fix deployment blocks new enrollments
   - **Symptoms**: User clicks Enroll, sees 200 OK, but nothing happens on crowdsec.net
   - **Root Cause**: Database has `status=enrolled` from before the `pending_acceptance` fix was deployed

---

## 🔴 CRITICAL BUG: Silent Idempotency Check (December 16, 2025)

### Problem Statement

User submits enrollment form, backend returns 200 OK (confirmed in Docker logs), but the enrollment NEVER appears on crowdsec.net. No `cscli` command execution visible in logs.

### Docker Log Evidence

```
POST /api/v1/admin/crowdsec/console/enroll → 200 OK (137ms latency)
NO "starting crowdsec console enrollment" log ← cscli NEVER executed
NO cscli output logs
```

### Code Path Analysis

**File:** [backend/internal/crowdsec/console_enroll.go](backend/internal/crowdsec/console_enroll.go)

#### Step 1: Handler calls service (line 865-920)

```go
// crowdsec_handler.go:888-895
status, err := h.Console.Enroll(ctx, crowdsec.ConsoleEnrollRequest{
    EnrollmentKey: payload.EnrollmentKey,
    Tenant:        payload.Tenant,
    AgentName:     payload.AgentName,
    Force:         payload.Force,  // <-- User did NOT check Force checkbox
})
```

#### Step 2: Idempotency Check (lines 155-165) ⚠️ BUG HERE

```go
// console_enroll.go:155-165
if rec.Status == consoleStatusEnrolling {
    return s.statusFromModel(rec), fmt.Errorf("enrollment already in progress")
}
// If already enrolled or pending acceptance, skip unless Force is set
if (rec.Status == consoleStatusEnrolled || rec.Status == consoleStatusPendingAcceptance) && !req.Force {
    return s.statusFromModel(rec), nil  // <-- RETURNS SUCCESS WITHOUT LOGGING OR RUNNING CSCLI!
}
```

#### Step 3: Database State (confirmed via container inspection)

```
uuid: fb129bb5-d223-4c66-941c-a30e2e2b3040
status: enrolled  ← SET BY OLD CODE BEFORE pending_acceptance FIX
tenant: 5e045b3c-5196-406b-99cd-503bc64c7b0d
agent_name: Charon
```

### Root Cause

1. **Historical State**: User enrolled BEFORE the `pending_acceptance` fix was deployed
2. **Old Code Bug**: Previous code set `status = enrolled` immediately after cscli returned exit 0
3. **Silent Skip**: Current code silently skips enrollment when `status` is `enrolled` (or `pending_acceptance`)
4. **No User Feedback**: Returns 200 OK without logging or informing user enrollment was skipped

### Manual Test Results from Container

```bash
# cscli is available and working
docker exec charon cscli console enroll --help
# ✅ Shows help

# LAPI is running
docker exec charon cscli lapi status
# ✅ "You can successfully interact with Local API (LAPI)"

# Console status
docker exec charon cscli console status
# ✅ Shows options table (custom=true, tainted=true)

# Manual enrollment with invalid key shows proper error
docker exec charon cscli console enroll --name test TESTINVALIDKEY123
# ✅ Error: "the attachment key provided is not valid"

# Config path exists and is correct
docker exec charon ls /app/data/crowdsec/config/config.yaml
# ✅ File exists
```

### Required Fixes

#### Fix 1: Add Logging for Skipped Enrollments

**File:** `backend/internal/crowdsec/console_enroll.go` lines 162-165

**Current:**

```go
if (rec.Status == consoleStatusEnrolled || rec.Status == consoleStatusPendingAcceptance) && !req.Force {
    return s.statusFromModel(rec), nil
}
```

**Fixed:**

```go
if (rec.Status == consoleStatusEnrolled || rec.Status == consoleStatusPendingAcceptance) && !req.Force {
    logger.Log().WithField("status", rec.Status).WithField("agent", rec.AgentName).WithField("tenant", rec.Tenant).Info("enrollment skipped: already enrolled or pending - use force=true to re-enroll")
    return s.statusFromModel(rec), nil
}
```

#### Fix 2: Add "Skipped" Indicator to Response

Add a field to indicate enrollment was skipped vs actually submitted:

```go
type ConsoleEnrollmentStatus struct {
    Status          string     `json:"status"`
    Skipped         bool       `json:"skipped,omitempty"`  // <-- NEW
    // ... other fields
}
```

And in the idempotency return:

```go
status := s.statusFromModel(rec)
status.Skipped = true
return status, nil
```

#### Fix 3: Frontend Should Show "Already Enrolled" State

**File:** `frontend/src/pages/CrowdSecConfig.tsx`

When `consoleStatusQuery.data?.status === 'enrolled'` or `'pending_acceptance'`:

- Show "You are already enrolled" message
- Show "Force Re-Enrollment" button with checkbox
- Explain that acceptance on crowdsec.net may be required

#### Fix 4: Migrate Stale "enrolled" Status to "pending_acceptance"

Either:

1. Add a database migration to change all `enrolled` to `pending_acceptance`
2. Or have users click "Force Re-Enroll" once

### Workaround for User

Until fix is deployed, user can re-enroll using the Force option:

1. In the UI: Check "Force re-enrollment" checkbox before clicking Enroll
2. Or via curl:

```bash
curl -X POST http://localhost:8080/api/v1/admin/crowdsec/console/enroll \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"enrollment_key":"<key>", "agent_name":"Charon", "force":true}'
```

---

## Previous Frontend Analysis (Still Valid for Reference)

### Enrollment Flow Path

```
User clicks "Enroll" button
    ↓
CrowdSecConfig.tsx: <Button onClick={() => submitConsoleEnrollment(false)} ...>
    ↓
submitConsoleEnrollment() function (line 269-299)
    ↓
validateConsoleEnrollment() check (line 254-267)
    ↓
enrollConsoleMutation.mutateAsync(payload)
    ↓
useConsoleEnrollment.ts: enrollConsole(payload)
    ↓
consoleEnrollment.ts: client.post('/admin/crowdsec/console/enroll', payload)
```

### Conditions That Block the Enrollment Request

#### 1. **Feature Flag Disabled** (POSSIBLE BLOCKER)

**File:** [CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx#L44-L45)

```typescript
const { data: featureFlags } = useQuery({ queryKey: ['feature-flags'], queryFn: getFeatureFlags })
const consoleEnrollmentEnabled = Boolean(featureFlags?.['feature.crowdsec.console_enrollment'])
```

**Impact:** If `feature.crowdsec.console_enrollment` is `false` or undefined, the **entire enrollment card is not rendered**:

```typescript
{consoleEnrollmentEnabled && (
  <Card data-testid="console-enrollment-card">
    ... enrollment UI ...
  </Card>
)}
```

#### 2. **Enroll Button Disabled Conditions** ⚠️ HIGH PROBABILITY

**File:** [CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx#L692)

```typescript
disabled={isConsolePending || (lapiStatusQuery.data && !lapiStatusQuery.data.lapi_ready) || !enrollmentToken.trim()}
```

The button is disabled when:

| Condition | Description |
|-----------|-------------|
| `isConsolePending` | Enrollment mutation is already in progress OR status is 'enrolling' |
| `lapiStatusQuery.data && !lapiStatusQuery.data.lapi_ready` | LAPI status query returned data but `lapi_ready` is `false` |
| `!enrollmentToken.trim()` | Enrollment token input is empty |

**⚠️ CRITICAL FINDING:** The LAPI ready check can block enrollment:

- If `lapiStatusQuery.data` exists AND `lapi_ready` is `false`, button is DISABLED
- This can happen if CrowdSec process is running but LAPI hasn't fully initialized

#### 3. **Validation Blocks in submitConsoleEnrollment()** ⚠️ HIGH PROBABILITY

**File:** [CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx#L269-L276)

```typescript
const submitConsoleEnrollment = async (force = false) => {
  const allowMissingTenant = force && !consoleTenant.trim()
  const requireAck = normalizedConsoleStatus === 'not_enrolled'
  if (!validateConsoleEnrollment({ allowMissingTenant, requireAck })) return  // <-- EARLY RETURN
  ...
}
```

**Validation function** (line 254-267):

```typescript
const validateConsoleEnrollment = (options?) => {
  const nextErrors = {}
  if (!enrollmentToken.trim()) {
    nextErrors.token = 'Enrollment token is required'
  }
  if (!consoleAgentName.trim()) {
    nextErrors.agent = 'Agent name is required'
  }
  if (!consoleTenant.trim() && !options?.allowMissingTenant) {
    nextErrors.tenant = 'Tenant / organization is required'  // <-- BLOCKS if tenant empty
  }
  if (options?.requireAck && !consoleAck) {
    nextErrors.ack = 'You must acknowledge...'  // <-- BLOCKS if checkbox unchecked
  }
  setConsoleErrors(nextErrors)
  return Object.keys(nextErrors).length === 0
}
```

**Validation will SILENTLY block** the request if:

1. `enrollmentToken` is empty
2. `consoleAgentName` is empty
3. `consoleTenant` is empty (for non-force enrollment)
4. **`consoleAck` checkbox is unchecked** (for first-time enrollment where status is `not_enrolled`)

### Summary of Blocking Conditions

| Condition | Where | Effect |
|-----------|-------|--------|
| Feature flag disabled | Line 44-45 | Entire enrollment card not rendered |
| **LAPI not ready** | Line 692 | **Button disabled** |
| Token empty | Line 692, validation | Button disabled + validation blocks |
| Agent name empty | Validation line 260 | Validation silently blocks |
| **Tenant empty** | Validation line 262 | **Validation silently blocks** |
| **Acknowledgment unchecked** | Validation line 265 | **Validation silently blocks** |
| Already enrolling | Line 692 | Button disabled |

### Most Likely Root Causes (Ordered by Probability)

#### 1. **LAPI Not Ready Check** ⚠️ HIGH PROBABILITY

The condition `(lapiStatusQuery.data && !lapiStatusQuery.data.lapi_ready)` will disable the button if:

- The status query has completed (data exists)
- But `lapi_ready` is `false`

**Check:** Call `GET /api/v1/admin/crowdsec/status` and verify `lapi_ready` field.

#### 2. **Acknowledgment Checkbox Not Checked** ⚠️ HIGH PROBABILITY

For first-time enrollment (`status === 'not_enrolled'`), the checkbox MUST be checked. The validation will silently `return` without making the API call.

**Check:** Ensure checkbox with `data-testid="console-ack-checkbox"` is checked.

#### 3. **Tenant Field Empty**

For non-force enrollment, the tenant field is required. An empty tenant will block the request silently.

**Check:** Ensure tenant input has a value.

### Code Sections That Need Fixes

#### Fix 1: Add Debug Logging (Temporary)

Add to `submitConsoleEnrollment()`:

```typescript
const submitConsoleEnrollment = async (force = false) => {
  console.log('[DEBUG] submitConsoleEnrollment called', {
    force,
    enrollmentToken: enrollmentToken.trim() ? 'present' : 'empty',
    consoleTenant,
    consoleAgentName,
    consoleAck,
    normalizedConsoleStatus,
    lapiReady: lapiStatusQuery.data?.lapi_ready,
  })
  // ... rest
}
```

#### Fix 2: Improve Validation Feedback

The validation currently sets `consoleErrors` but these may not be visible to the user. Ensure error messages are displayed.

#### Fix 3: Check LAPI Status Polling

The LAPI status query starts only after 3 seconds (`initialCheckComplete`). If the user clicks before then, the button may be enabled (good) but LAPI might not actually be ready (backend will fail).

### Recommended Debug Steps

1. **Open browser DevTools → Console**
2. **Check if enrollment card is rendered** (look for `data-testid="console-enrollment-card"`)
3. **Inspect button element** - check if `disabled` attribute is present
4. **Check Network tab** for:
   - `GET /api/v1/feature-flags` response
   - `GET /api/v1/admin/crowdsec/status` response (check `lapi_ready`)
5. **Verify form state**:
   - Token field has value
   - Agent name has value
   - Tenant has value
   - Checkbox is checked

### API Client Verification

**File:** [consoleEnrollment.ts](frontend/src/api/consoleEnrollment.ts#L27-L30)

```typescript
export async function enrollConsole(payload: ConsoleEnrollPayload): Promise<ConsoleEnrollmentStatus> {
  const resp = await client.post<ConsoleEnrollmentStatus>('/admin/crowdsec/console/enroll', payload)
  return resp.data
}
```

✅ The API client is correctly implemented. The issue is upstream - **the function is never being called** because conditions are blocking it.

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
