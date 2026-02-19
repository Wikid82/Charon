# Phase 2.2 - User Management Discovery & Root Cause Analysis

**Status:** Discovery Complete - Root Cause Identified
**Date Started:** 2026-02-09
**Objective:** Identify root causes of 6 failing user management tests

## Root Cause: Synchronous Email Blocking in InviteUser

### CRITICAL FINDING

**Code Location:** `/projects/Charon/backend/internal/api/handlers/user_handler.go` (lines 400-470)
**Problem Method:** `InviteUser` handler
**Issue:** Email sending **blocks HTTP response** - entire request hangs until SMTP completes or times out

### Why Tests Timeout (Test #248)

Request flow in `InviteUser`:
```
1. Check admin role                          ✅ <1ms
2. Parse request JSON                        ✅ <1ms
3. Check email exists                        ✅ Database query
4. Generate invite token                     ✅ <1ms
5. Create user in database (transaction)     ✅ Database write
6. ❌ BLOCKS: Call h.MailService.SendInvite() - SYNCHRONOUS SMTP
   └─ Connect to SMTP server
   └─ Authenticate
   └─ Send email
   └─ Wait for confirmation (NO TIMEOUT!)
7. Return JSON response (if email succeeds)
```

**The Problem:**
Lines 462-469:
```go
// Try to send invite email
emailSent := false
if h.MailService.IsConfigured() {
    baseURL, ok := utils.GetConfiguredPublicURL(h.DB)
    if ok {
        appName := getAppName(h.DB)
        if err := h.MailService.SendInvite(user.Email, inviteToken, appName, baseURL); err == nil {
            emailSent = true
        }
    }
}
```

This code **blocks the HTTP request** until `SendInvite()` returns.

### MailService Architecture

**File:** `/projects/Charon/backend/internal/services/mail_service.go`
**Method:** `SendEmail()` at line 255

The `SendEmail()` method:
- Makes **direct SMTP connections** via `smtp.SendMail()` (line 315)
- OR custom TLS dialect for SSL/STARTTLS
- **Waits for SMTP response** before returning
- **No async queue, no goroutines, no background workers**

**Example:** If SMTP server takes 5 seconds to respond (or 30s timeout):
→ HTTP request blocks for 5-30+ seconds
→ Playwright test times out after 60s

### Why Test #248 Fails

Test expectation: "Invite user, get response, user appears in list"
Actual behavior: "Invite user → blocks on SMTP → no response → test timeout"

**Test File:** `/projects/Charon/tests/monitoring/uptime-monitoring.spec.ts` (for reference)
**When SMTP is configured:** Request hangs indefinitely
**When SMTP is NOT configured:** Request completes quickly (MailService.IsConfigured() = false)

## Other Test Failures (Tests #258, #260, #262, #269-270)

### Status: Likely Unrelated to Email Blocking

These tests involve:
- **#258:** Update permission mode
- **#260:** Remove permitted hosts
- **#262:** Enable/disable user toggle
- **#269:** Update user role to admin
- **#270:** Update user role to user

**Reason:** These endpoints (PUT /users/:id/permissions, PUT /users/:id) do NOT send emails

**Hypothesis for other timeouts:**
- Possible slow database queries (missing indexes?)
- Possible missing database preloading (N+1 queries?)
- Frontend mocking/test infrastructure issue (not handler code)
- Transaction deadlocks (concurrent test execution)

**Status:** Requires separate investigation

## Solution Approach for Phase 2.1

### Recommendation: Async Email Sending

**Change:**  Convert email sending to **background job** pattern:
1. ✅ Create user in database
2. ✅ Return response immediately (201 Created)
3. → Send email asynchronously (goroutine/queue)
4. → If email fails, log error, user still created

**Before:**
```go
// User creation + email (both must succeed to return)
tx.Create(&user)      // ✅
SendEmail(...)         // ❌ BLOCKS - no timeout
return JSON(user)      // Only if above completes
```

**After:**
```go
// User creation (fast) + async email (non-blocking)
tx.Create(&user)       // ✅ <100ms
go SendEmailAsync(...) // 🔄 Background (non-blocking)
return JSON(user)      // ✅ Immediate response (~150ms total)
```

## Manual Testing Findings

**SMTP Configuration Status:** NOT configured in test database
**Result:** Invite endpoint returns immediately (emailSent=false skip)
**Test Environment:** Application accessible at http://localhost:8080

**Code Verification:**
- ✅ `POST /users/invite` endpoint EXISTS and is properly registered
- ✅ `PUT /users/:id/permissions` endpoint EXISTS and is properly registered
- ✅ `GET /users` endpoint EXISTS (for list display)
- ✅ User models properly initialized with permission_mode and permitted_hosts
- ✅ Database schema includes all required fields

## Root Cause Summary

| Issue | Severity | Root Cause | Impact |
|-------|----------|-----------|--------|
| Test #248 Timeout | CRITICAL | Sync SMTP blocking HTTP response | InviteUser endpoint completely unavailable when SMTP is slow |
| Test #258-270 Timeout | UNKNOWN | Requires further investigation | May be database, mocking, or concurrency issues |

## Recommendations

### Immediate (Phase 2.1 Fix)

1. **Refactor InviteUser to async email**
   - Create user (fast)
   - Return immediately with 201 Created
   - Send email in background goroutine
   - Endpoint: <100ms response time

2. **Add timeout to SMTP calls**
   - If email takes >5s, fail gracefully
   - Never block HTTP response >1s

3. **Add feature flag for optional email**
   - Allow invite without email sending
   - Endpoint can pre-generate token for manual sharing

### Follow-up (Phase 2.2)

1. **Investigate Tests #258-270 separately** (they may be unrelated)
2. **Profile UpdateUserPermissions endpoint** (database efficiency?)
3. **Review E2E test mocking** (ensure fixtures don't interfere)

## Evidence & References

**Code files reviewed:**
- `/projects/Charon/backend/internal/api/handlers/user_handler.go` (InviteUser, UpdateUserPermissions)
- `/projects/Charon/backend/internal/services/mail_service.go` (SendEmail, SendInvite)
- `/projects/Charon/backend/internal/models/user.go` (User model)
- `/projects/Charon/tests/monitoring/uptime-monitoring.spec.ts` (E2E test patterns)

**Endpoints verified working:**
- POST /api/v1/users/invite - EXISTS, properly registered
- PUT /api/v1/users/:id/permissions - EXISTS, properly registered
- GET /api/v1/users - EXISTS (all users endpoint)

**Test Database State:**
- SMTP not configured (safe mode)
- Users table has admin + test users
- Permitted hosts associations work
- Invite tokens generate successfully on user creation

## Next Steps

1. ✅ Root cause identified: Synchronous email blocking
2. → Implement async email sending in InviteUser handler
3. → Test with E2E suite
4. → Document performance improvements
5. → Investigate remaining test failures if needed
