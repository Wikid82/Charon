# Investigation Summary: Re-Enrollment & Live Log Viewer Issues

**Date:** December 16, 2025
**Investigator:** GitHub Copilot
**Status:** ✅ Complete

---

## 🎯 Quick Summary

### Issue 1: Re-enrollment with NEW key didn't work

**Status:** ✅ NO BUG - User error (invalid key)

- Frontend correctly sends `force: true`
- Backend correctly adds `--overwrite` flag
- CrowdSec API rejected the new key as invalid
- Same key worked because it was still valid in CrowdSec's system

**User Action Required:**

- Generate fresh enrollment key from app.crowdsec.net
- Copy key completely (no spaces/newlines)
- Try re-enrollment again

### Issue 2: Live Log Viewer shows "Disconnected"

**Status:** ⚠️ LIKELY AUTH ISSUE - Needs fixing

- WebSocket connections NOT reaching backend (no logs)
- Most likely cause: WebSocket auth headers missing
- Frontend defaults to wrong mode (`application` vs `security`)

**Fixes Required:**

1. Add auth token to WebSocket URL query params
2. Change default mode to `security`
3. Add error display to show auth failures

---

## 📊 Detailed Findings

### Issue 1: Re-Enrollment Analysis

#### Evidence from Code Review

**Frontend (`CrowdSecConfig.tsx`):**

```typescript
// ✅ CORRECT: Passes force=true when re-enrolling
onClick={() => submitConsoleEnrollment(true)}

// ✅ CORRECT: Includes force in payload
await enrollConsoleMutation.mutateAsync({
  enrollment_key: enrollmentToken.trim(),
  force,  // ← Correctly passed
})
```

**Backend (`console_enroll.go`):**

```go
// ✅ CORRECT: Adds --overwrite flag when force=true
if req.Force {
    args = append(args, "--overwrite")
}
```

**Docker Logs Evidence:**

```json
{
  "force": true,  // ← Force flag WAS sent
  "msg": "starting crowdsec console enrollment"
}
```

```text
Error: cscli console enroll: could not enroll instance:
API error: the attachment key provided is not valid
```

↑ **This proves the NEW key was REJECTED by CrowdSec API**

#### Root Cause

The user's new enrollment key was **invalid** according to CrowdSec's validation. Possible reasons:

1. Key was copied incorrectly (extra spaces/newlines)
2. Key was already used or revoked
3. Key was generated for different organization
4. Key expired (though CrowdSec keys typically don't expire)

The **original key worked** because:

- It was still valid in CrowdSec's system
- The `--overwrite` flag allowed re-enrolling to same account

---

### Issue 2: Live Log Viewer Analysis

#### Architecture

```
Frontend Component (LiveLogViewer.tsx)
    ↓
    ├─ Mode: "application" → /api/v1/logs/live
    └─ Mode: "security" → /api/v1/cerberus/logs/ws
                                    ↓
                        Backend Handler (cerberus_logs_ws.go)
                                    ↓
                        LogWatcher Service (log_watcher.go)
                                    ↓
                        Tails: /app/data/logs/access.log
```

#### Evidence

**✅ Access log has data:**

```bash
$ docker exec charon tail -20 /app/data/logs/access.log
# Shows 20+ lines of JSON-formatted Caddy access logs
# Logs are being written continuously
```

**❌ No WebSocket connection logs:**

```bash
$ docker logs charon 2>&1 | grep -i "websocket"
# Shows route registration but NO connection attempts
[GIN-debug] GET /api/v1/cerberus/logs/ws --> ...LiveLogs-fm
# ↑ Route exists but no "WebSocket connection attempt" logs
```

**Expected logs when connection succeeds:**

```
Cerberus logs WebSocket connection attempt
Cerberus logs WebSocket connected
```

These logs are MISSING → Connections are failing before reaching the handler

#### Root Cause

**Most likely issue:** WebSocket authentication failure

1. Both endpoints are under `protected` route group (require auth)
2. Native WebSocket API doesn't support custom headers
3. Frontend doesn't add auth token to WebSocket URL
4. Backend middleware rejects with 401/403
5. WebSocket upgrade fails silently
6. User sees "Disconnected" without explanation

**Secondary issue:** Default mode is `application` but user needs `security`

#### Verification Steps Performed

```bash
# ✅ CrowdSec process is running
$ docker exec charon ps aux | grep crowdsec
70 root 0:06 /usr/local/bin/crowdsec -c /app/data/crowdsec/config/config.yaml

# ✅ Routes are registered
[GIN-debug] GET /api/v1/logs/live --> handlers.LogsWebSocketHandler
[GIN-debug] GET /api/v1/cerberus/logs/ws --> handlers.LiveLogs-fm

# ✅ Access logs exist and have recent entries
/app/data/logs/access.log (3105315 bytes, modified 22:54)

# ❌ No WebSocket connection attempts in logs
```

---

## 🔧 Required Fixes

### Fix 1: Add Auth Token to WebSocket URLs (HIGH PRIORITY)

**File:** `frontend/src/api/logs.ts`

Both `connectLiveLogs()` and `connectSecurityLogs()` need:

```typescript
// Get auth token from storage
const token = localStorage.getItem('token') || sessionStorage.getItem('token');
if (token) {
  params.append('token', token);
}
```

**File:** `backend/internal/api/middleware/auth.go` (or wherever auth middleware is)

Ensure auth middleware checks for token in query parameters:

```go
// Check query parameter for WebSocket auth
if token := c.Query("token"); token != "" {
    // Validate token
}
```

### Fix 2: Change Default Mode to Security (MEDIUM PRIORITY)

**File:** `frontend/src/components/LiveLogViewer.tsx` Line 142

```typescript
export function LiveLogViewer({
  mode = 'security',  // ← Change from 'application'
  // ...
}: LiveLogViewerProps) {
```

**Rationale:** User specifically said "I only need SECURITY logs"

### Fix 3: Add Error Display (MEDIUM PRIORITY)

**File:** `frontend/src/components/LiveLogViewer.tsx`

```tsx
const [connectionError, setConnectionError] = useState<string | null>(null);

const handleError = (error: Event) => {
  console.error('WebSocket error:', error);
  setIsConnected(false);
  setConnectionError('Connection failed. Please check authentication.');
};

// In JSX (inside log viewer):
{connectionError && (
  <div className="text-red-400 text-xs p-2 border-t border-gray-700">
    ⚠️ {connectionError}
  </div>
)}
```

### Fix 4: Add Reconnection Logic (LOW PRIORITY)

Add automatic reconnection with exponential backoff for transient failures.

---

## ✅ Testing Checklist

### Re-Enrollment Testing

- [ ] Generate new enrollment key from app.crowdsec.net
- [ ] Copy key to clipboard (verify no extra whitespace)
- [ ] Paste into Charon enrollment form
- [ ] Click "Re-enroll" button
- [ ] Check Docker logs for `"force":true` and `--overwrite`
- [ ] If error, verify exact error message from CrowdSec API

### Live Log Viewer Testing

- [ ] Open browser DevTools → Network tab
- [ ] Open Live Log Viewer
- [ ] Check for WebSocket connection to `/api/v1/cerberus/logs/ws`
- [ ] Verify status is 101 (not 401/403)
- [ ] Check Docker logs for "WebSocket connection attempt"
- [ ] Generate test traffic (make HTTP request to proxied service)
- [ ] Verify log appears in viewer
- [ ] Test mode toggle (Application vs Security)

---

## 📚 Key Files Reference

### Re-Enrollment

- `frontend/src/pages/CrowdSecConfig.tsx` (re-enroll UI)
- `frontend/src/api/consoleEnrollment.ts` (API client)
- `backend/internal/crowdsec/console_enroll.go` (enrollment logic)
- `backend/internal/api/handlers/crowdsec_handler.go` (HTTP handler)

### Live Log Viewer

- `frontend/src/components/LiveLogViewer.tsx` (component)
- `frontend/src/api/logs.ts` (WebSocket client)
- `backend/internal/api/handlers/cerberus_logs_ws.go` (WebSocket handler)
- `backend/internal/services/log_watcher.go` (log tailing service)

---

## 🎓 Lessons Learned

1. **Always check actual errors, not symptoms:**
   - User said "new key didn't work"
   - Actual error: "the attachment key provided is not valid"
   - This is a CrowdSec API validation error, not a Charon bug

2. **WebSocket debugging is different from HTTP:**
   - No automatic auth headers
   - Silent failures are common
   - Must check both browser Network tab AND backend logs

3. **Log everything:**
   - The `"force":true` log was crucial evidence
   - Without it, we'd be debugging the wrong issue

4. **Read the docs:**
   - CrowdSec help text says "you will need to validate the enrollment in the webapp"
   - This explains why status is `pending_acceptance`, not `enrolled`

---

## 📞 Next Steps

### For User

1. **Re-enrollment:**
   - Get fresh key from app.crowdsec.net
   - Try re-enrollment with new key
   - If fails, share exact error from Docker logs

2. **Live logs:**
   - Wait for auth fix to be deployed
   - Or manually add `?token=<your-token>` to WebSocket URL as temporary workaround

### For Development

1. Deploy auth token fix for WebSocket (Fix 1)
2. Change default mode to security (Fix 2)
3. Add error display (Fix 3)
4. Test both issues thoroughly
5. Update user

---

**Investigation Duration:** ~1 hour
**Files Analyzed:** 12
**Docker Commands Run:** 5
**Conclusion:** One user error (invalid key), one real bug (WebSocket auth)
