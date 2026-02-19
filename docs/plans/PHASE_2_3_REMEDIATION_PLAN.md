# Phase 2.3: Critical Fixes Remediation Plan

**Status:** Planning - Ready for Execution
**Created:** 2026-02-09
**Target Completion:** 2026-02-09 (2-3 hours parallel execution)
**Dependencies:** Phase 2.2 discovery complete, Phase 3 E2E security testing blocked until completion

---

## 1. Executive Summary

### Pre-Execution Validation Checklist

Before proceeding to Phase 2.3a, verify all prerequisites:

- [ ] All developers assigned and available
- [ ] Database in clean state (fresh container)
- [ ] Git workspace clean (no uncommitted changes)
- [ ] Code review owners assigned
- [ ] Approval authority (Tech Lead) available for sign-off
- [ ] Backend Docker build environment ready
- [ ] Frontend test environment ready (Node.js, Playwright)
- [ ] Auth endpoint verified exists (2.3c pre-check)

**If any items unchecked:** Resolve before proceeding to Phase 2.3a

---

### Overview

Phase 2.3 addresses three **critical blocking issues** identified during Phase 2.2 discovery that prevent progression to Phase 3 E2E security testing:

| Issue | Severity | Component | Fix Effort | Blocker? |
|-------|----------|-----------|-----------|----------|
| **CVE-2024-45337** - golang.org/x/crypto/ssh authorization bypass | CRITICAL | Backend Dependencies | 1 hour | YES - Production blocker |
| **InviteUser Email Blocking** - Synchronous SMTP blocks HTTP response | HIGH | Backend (user_handler.go) | 2-3 hours | YES - Test suite blocker |
| **Test Auth Token Refresh** - E2E tests fail with 401 after 30+ min | MEDIUM | Frontend (Playwright fixtures) | 0.5-1 hour | YES - Test execution blocker |

### Critical Path & Timeline

**Sequential Timeline:** 4-5 hours
**Parallel Timeline:** 2-3 hours (recommended)
**Phase 3 Start Eligible:** After ALL three phases complete

**Interdependency Analysis:**
- ✅ **2.3a and 2.3b are independent** (different code areas)
- ✅ **2.3a and 2.3c are independent** (different languages/layers)
- ✅ **2.3b and 2.3c are independent** (can run in parallel)
- ✅ **All three can run simultaneously** with different developers

### Phase 3 Blocking Dependencies

| Phase | Blocker Type | Consequence if Delayed |
|-------|-------------|----------------------|
| **2.3a** | Security compliance | Cannot deploy to production (CVE vulnerability) |
| **2.3b** | Functional requirement | User management test suite fails/timeouts |
| **2.3c** | Test infrastructure | Phase 3 tests will fail with 401 errors after 30 min |

**Decision:** All three MUST complete before Phase 3 approval.

---

## 2. Phase 2.3a: Dependency Security Update (1 hour)

**Priority:** 🔴 CRITICAL
**Owner:** Backend Developer
**Can Run in Parallel:** Yes (with 2.3b and 2.3c)
**Start Time:** Immediately
**Target Completion:** 1 hour

### Objective

Update golang.org/x/crypto and related dependencies to patch CVE-2024-45337 (SSH authorization bypass), then verify with container security scan.

### Root Cause

**CVE Details:**
- **CVE-2024-45337** - golang.org/x/crypto/ssh authorization bypass
- **Affected versions:** Before v0.31.0
- **Risk:** Attackers can bypass authorization checks via SSH protocol manipulation
- **Impact:** If Charon exposes SSH management → complete auth bypass

### Current Status

```bash
# Current go.mod references:
go list -m all | grep -E 'golang.org/x/(crypto|net|oauth2)|github.com/quic-go'
# Expected output: Old versions (v0.27.0, v0.28.x, v0.x.x)
```

### Steps

#### Step 1: Update Dependencies (15 min)

**File:** `backend/go.mod`
**Command:** Execute from `/projects/Charon/`

```bash
cd backend

# Update golang.org/x/crypto to latest
go get -u golang.org/x/crypto

# Update related security packages
go get -u golang.org/x/net
go get -u golang.org/x/oauth2

# Update WebRTC/QUIC dependencies (may depend on crypto)
go get -u github.com/quic-go/quic-go

# Cleanup and verify integrity
go mod tidy
go mod verify
```

**Expected Changes:**
- `golang.org/x/crypto` → v0.31.0 or later
- `golang.org/x/net` → latest (v0.33.0+)
- `golang.org/x/oauth2` → latest
- `github.com/quic-go/quic-go` → latest compatible

**Verification:**
```bash
# Should show updated versions
go list -m all | grep -E 'golang.org/x|(quic-go|crypto)'

# Should complete without errors
go mod verify
```

#### Step 2: Build & Test Backend (15 min)

**Ensure backend compiles with new dependencies:**

```bash
# Test compilation (without running)
go build -v ./...

# Run backend unit tests
go test -short ./...

# Should complete in <5 min with no errors
```

**Expected Result:** Build succeeds, tests pass, no deprecation warnings related to crypto APIs.

#### Step 3: Rebuild Docker Image (15 min)

**File:** `Dockerfile`
**Command:** Execute from `/projects/Charon/`

```bash
# Clean build (no cache) to ensure new go.mod is used
docker build \
  --no-cache \
  -t charon:local \
  -f Dockerfile \
  .

# Expected output:
# ✓ Building backend stage (uses new go.mod)
# ✓ Running `go mod verify`
# ✓ Building binary
# ✓ Final image layers
# Successfully built IMAGE_ID
# Successfully tagged charon:local
```

**Timing:** 5-7 minutes for full build

#### Step 4: Container Security Scan (15 min)

**Tool:** Trivy (vulnerability scanner)
**Command:** Execute from `/projects/Charon/`

```bash
# Scan the local image for vulnerabilities
trivy image \
  --severity CRITICAL,HIGH \
  --exit-code 0 \
  --timeout=30m \
  charon:local

# Save results to file for review
trivy image \
  --format json \
  --severity CRITICAL,HIGH \
  charon:local > /tmp/trivy-charon-local.json
```

**Expected Output:**
```
charon:local (alpine 3.19)
=======================
Total: 0 vulnerabilities (CRITICAL: 0, HIGH: 0)

Scanned at: 2026-02-09T14:30:00Z
Database updated at: 2026-02-09T14:00:00Z
```

**If vulnerabilities remain:**
- ❌ CVE-2024-45337 still present → dependency update failed
- ❌ New vulnerabilities discovered → investigate and update
- → Document in troubleshooting section
- → Retry with `go mod graph | grep crypto` to debug

#### Step 5: Smoke Test Core Functionality (10 min)

**Endpoint:** `POST /api/v1/auth/login`
**Data:** Use default test credentials

```bash
# Start or ensure container is running
docker run -d \
  --name charon-test \
  -p 8080:8080 \
  -e CHARON_DB_PATH=/data/charon.db \
  charon:local

# Wait for health check
sleep 5

# Test login endpoint
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email":"admin@example.com",
    "password":"TestPass123!"
  }' | jq .

# Expected response:
# {
#   "token": "eyJ...",
#   "expires_at": "2026-02-10T14:30:00Z",
#   ...
# }

# Cleanup
docker stop charon-test
docker rm charon-test
```

### Success Criteria

- ✅ **Dependency Update:** All golang.org/x packages updated to latest
- ✅ **Build Success:** Docker image builds without errors
- ✅ **No CVE-2024-45337:** Trivy scan reports 0 CRITICAL vulnerabilities
- ✅ **Smoke Test:** Login endpoint responds with valid token
- ✅ **Trivy Database:** Current (within 1 hour of scan time)

### Failure Handling

**If build fails after dependency update:**
1. Check for incompatible API changes: `go mod why -graph golang.org/x/crypto`
2. Review changelog for breaking changes
3. May need code updates in cryptography-related handlers
4. Escalate to platform owner if APIs changed significantly

**If Trivy still reports CVE-2024-45337:**
1. Verify `golang.org/x/crypto v0.31.0+` installed: `go list -m golang.org/x/crypto`
2. Check Trivy database is current: `trivy image-config --scanners config --list`
3. Rebuild without cache: `docker build --no-cache ...`

### Regression Testing

Run quick smoke tests to ensure nothing broke:
- ✅ Login succeeds
- ✅ Logout succeeds
- ✅ Token validation works
- ✅ Permission checks work (admin endpoint accessible)

**Timing:** 5-10 minutes total

---

## 3. Phase 2.3b: Async Email Refactor (2-3 hours, Parallelizable)

**Priority:** 🟡 HIGH
**Owner:** Backend Developer (may be different from 2.3a, or same with sequential scheduling)
**Can Run in Parallel:** Yes (with 2.3a and 2.3c)
**Start Time:** Immediately (or after 2.3a if same developer)
**Target Completion:** 2-3 hours

### Objective

Convert InviteUser endpoint from synchronous email sending (blocking HTTP request) to async pattern (non-blocking background job). This unblocks the user management test suite and prevents endpoint timeouts in production.

### Root Cause

**Current Code:** `/projects/Charon/backend/internal/api/handlers/user_handler.go` (lines 462-469)

```go
// CURRENT BLOCKING PATTERN
if h.MailService.IsConfigured() {
    baseURL, ok := utils.GetConfiguredPublicURL(h.DB)
    if ok {
        appName := getAppName(h.DB)
        // ❌ THIS BLOCKS THE ENTIRE HTTP REQUEST
        if err := h.MailService.SendInvite(user.Email, inviteToken, appName, baseURL); err == nil {
            emailSent = true
        }
    }
}
return c.JSON(200, user)
```

**CRITICAL BUG - Race Condition:**

The user `Email` field referenced inside a goroutine MUST be captured BEFORE launching the goroutine. If any other goroutine or code modifies the `user` object, the email sending could get stale or corrupted data.

**Danger Pattern (DON'T DO THIS):**
```go
go func() {
    // ❌ RACE CONDITION: user object may be modified before this runs
    if err := h.MailService.SendInvite(user.Email, ...); err != nil { ... }
}()
```

**Why it blocks:**
1. `h.MailService.SendInvite()` calls SMTP synchronously
2. Waits for SMTP server response (can take 1-30 seconds)
3. HTTP request blocked until email completes or errors
4. Test timeout after 60 seconds if SMTP is slow

### Implementation Strategy

**Three options for async pattern:**

#### Option A: Simple Goroutine (Recommended - 30 min)

**Best for:** MVP, fast iteration, sufficient functionality
**Trade-off:** No guaranteed delivery, no retry mechanism
**Code change:**

```go
// AFTER - Non-blocking async pattern
go func() {
    if h.MailService.IsConfigured() {
        baseURL, ok := utils.GetConfiguredPublicURL(h.DB)
        if ok {
            appName := getAppName(h.DB)
            if err := h.MailService.SendInvite(user.Email, inviteToken, appName, baseURL); err != nil {
                // Log error but don't block response
                h.Logger.Error("Failed to send invite email",
                    zap.String("user_email", user.Email),
                    zap.Error(err))
            }
        }
    }
}()

// Response returns immediately (no wait for email)
return c.JSON(http.StatusCreated, user)
```

**Pros:**
- ✅ Minimal code change (5 lines)
- ✅ No external dependencies
- ✅ Immediate response (sub-200ms)
- ✅ Thread-safe with goroutines

**Cons:**
- ❌ No retry mechanism
- ❌ No persistent queue
- ❌ Email may not send if service crashes during goroutine execution

#### Option B: Channel-Based Queue (Recommended for Phase 2.3b - 1.5-2 hours)

**Best for:** Balanced reliability + maintainability
**Trade-off:** More code, but structured queue pattern
**Files to create/modify:**
- Create: `backend/internal/services/email_queue.go`
- Modify: `backend/internal/api/handlers/user_handler.go`
- Modify: `backend/internal/api/server.go` (initialize queue worker)

**Architecture:**
```
InviteUser handler
    ↓
Send job to channel (non-blocking, buffered channel)
    ↓
Return 201 response immediately
    ↓
Background worker goroutine
    ├─ Read job from channel
    ├─ Send email
    ├─ Log result (success/failure)
    └─ Continue processing next job
```

**Implementation sketch:**

```go
// backend/internal/services/email_queue.go
type EmailJob struct {
    Email     string
    Token     string
    AppName   string
    BaseURL   string
    CreatedAt time.Time
}

type EmailQueue struct {
    jobs chan EmailJob
    log  *zap.Logger
}

func NewEmailQueue(size int, log *zap.Logger) *EmailQueue {
    q := &EmailQueue{
        jobs: make(chan EmailJob, size),
        log:  log,
    }
    // Start worker goroutine
    go q.worker()
    return q
}

func (q *EmailQueue) Enqueue(job EmailJob) error {
    select {
    case q.jobs <- job:
        return nil
    default:
        // Queue full - could retry or log warning
        q.log.Warn("Email queue full, discarding job", zap.String("email", job.Email))
        return errors.New("queue full")
    }
}

func (q *EmailQueue) worker() {
    for job := range q.jobs {
        // Process email (retry logic optional)
        if err := q.sendEmail(job); err != nil {
            q.log.Error("Failed to send email",
                zap.String("email", job.Email),
                zap.Error(err))
        }
    }
}
```

**Handler usage:**
```go
// In InviteUser handler (much simpler now)
go func() {
    h.EmailQueue.Enqueue(EmailJob{
        Email:   user.Email,
        Token:   inviteToken,
        AppName: appName,
        BaseURL: baseURL,
    })
}()

return c.JSON(http.StatusCreated, user)
```

**Pros:**
- ✅ Structured queue pattern
- ✅ Buffered channel handles spikes
- ✅ Single worker processes emails in order
- ✅ Easy to monitor (queue length, errors)
- ✅ Extensible (add retry logic later)

**Cons:**
- ⚠️ Email lost if service crashes (not persisted)
- ⚠️ More code than Option A

#### Option C: Database Task Table (Most Robust - 2-3 hours)

**Best for:** Production-grade reliability
**Trade-off:** Most code, database schema change required
**Files:**
- Migrate: Create table `email_tasks`
- Create: `backend/internal/services/email_persistence.go`
- Modify: `backend/internal/api/handlers/user_handler.go`
- Modify: `backend/internal/api/server.go` (initialize worker)

**Architecture:**
```
InviteUser handler
    ↓
Insert email_task row (status='pending')
    ↓
Return 201 response immediately
    ↓
Background worker goroutine
    ├─ Query pending email_task rows
    ├─ Send email
    ├─ Update task (status='sent' or 'failed')
    ├─ Retry on failure (configurable attempts)
    └─ Continue polling
```

**Schema:**
```sql
CREATE TABLE email_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL,
    token TEXT NOT NULL,
    subject TEXT,
    body TEXT,
    status TEXT DEFAULT 'pending', -- pending, sent, failed
    attempts INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 3,
    error_message TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    sent_at DATETIME,
    UNIQUE(email, token) -- Prevent duplicates
);
```

**Pros:**
- ✅ Guaranteed delivery (persisted in database)
- ✅ Automatic retry (configurable)
- ✅ Full audit trail (when sent, errors)
- ✅ Survives service crashes

**Cons:**
- ❌ Schema migration required
- ❌ Additional polling overhead
- ❌ Complexity in retry logic

### Recommended Approach for Phase 2.3b

**Execute Option A (simple goroutine) for Phase 2.3b** (30 min)
- Fast, unblocks tests immediately
- Sufficient for current requirements
- Can refactor to Option B/C later if needed

**Then if time permits, begin Option B refactoring** (additional 1-2 hours)

### Implementation: Option A (30 min)

#### File: `backend/internal/api/handlers/user_handler.go`

**Location:** Method `InviteUser`, around line 462-469

**Current code:**
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

**Updated code (WITH RACE CONDITION FIX):**
```go
// Send invite email asynchronously (non-blocking)
emailSent := false // Placeholder - email will be sent in background
if h.MailService.IsConfigured() {
    // Capture user data BEFORE launching goroutine to avoid race condition
    userEmail := user.Email

    go func() {
        baseURL, ok := utils.GetConfiguredPublicURL(h.DB)
        if ok {
            appName := getAppName(h.DB)
            // Use captured email instead of user.Email to prevent race condition
            if err := h.MailService.SendInvite(userEmail, inviteToken, appName, baseURL); err != nil {
                // Log failure but don't block response
                h.Logger.Error("Failed to send invite email",
                    zap.String("user_email", userEmail),
                    zap.String("error", err.Error()))
            }
        }
    }()
    emailSent = true // Set true immediately since email will be sent in background
}
```

**What changed:**
1. **CAPTURE user.Email before goroutine** (`userEmail := user.Email`)
2. Wrapped email sending in `go func() { ... }()` goroutine
3. Use captured `userEmail` inside goroutine (not `user.Email`)
4. Email sends in background (non-blocking)
5. HTTP response returns immediately
6. Added error logging (via h.Logger which should exist)
7. Set `emailSent = true` immediately since we're sending async

**WHY THIS MATTERS:**
If the `user` object is modified or freed while the goroutine is running, directly accessing `user.Email` could read corrupt/stale data. By capturing `userEmail` first, we guarantee the goroutine always sends to the correct email address.

### Testing Strategy: Phase 2.3b

#### Test 1: Response Time Verification (5 min)

**File:** Add to test if needed, or use curl:

```bash
# Measure response time
time curl -s -X POST http://localhost:8080/api/v1/users/invite \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email":"newuser@example.com"}' | jq .

# Expected output:
# ✅ real    0m0.150s  (should be <200ms, not >5s)
# ✅ JSON response with user details
```

#### Test 2: Database Commit Verification (5 min)

```bash
# Verify user created immediately (before email completes)
curl -s http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq '.items[] | select(.email=="newuser@example.com")'

# Expected:
# ✅ User appears in list immediately
# ✅ Status shows created (not pending)
```

#### Test 3: Email Sending in Background (10 min)

**File:** Unit test in `/projects/Charon/backend/internal/api/handlers/user_handler_test.go`

```go
// Add test case
func TestInviteUserAsync(t *testing.T) {
    // Setup: Create mock mail service
    mockMailService := &MockMailService{
        sendInviteDelay: time.Second * 2, // Simulate slow SMTP
    }

    handler := &UserHandler{
        MailService: mockMailService,
        // ... other fields
    }

    // Record response time
    start := time.Now()
    response := handler.InviteUser(testContext)
    elapsed := time.Since(start)

    // Assert: Response returned quickly (async)
    assert.Less(t, elapsed, 200*time.Millisecond, "Response should be immediate")
    assert.Equal(t, http.StatusCreated, response.Status, "Should return 201")

    // Sleep to allow goroutine to complete
    time.Sleep(time.Second * 3)

    // Assert: Mail service was called
    assert.Equal(t, 1, mockMailService.callCount, "Email should be sent")
}
```

#### Test 4: E2E Test Suite - Test #248 (10 min)

**File:** Run existing E2E tests

```bash
# Run the full user management test suite
npx playwright test \
  --project=firefox \
  tests/user-management.spec.ts::test('should invite user') \
  --timeout=5000  # Reduce timeout to verify fast response

# Expected:
# ✅ Test passes
# ✅ User created
# ✅ Response time <200ms (not timeout)
```

#### Test 5: Other User Management Tests (10 min)

```bash
# Run all related user management tests
npx playwright test \
  --project=firefox \
  tests/user-management.spec.ts

# Expected:
# ✅ Test #248 (invite user)
# ✅ Test #258 (update permissions)
# ✅ Test #260 (remove hosts)
# ✅ Test #262 (toggle user)
# ✅ Test #269 (set role to admin)
# ✅ Test #270 (set role to user)
# All tests should complete without timeout
```

### Success Criteria: Phase 2.3b

- ✅ **Response Time:** InviteUser endpoint returns in <200ms (not >5 seconds)
- ✅ **Immediate Commit:** User created and visible in database immediately after response
- ✅ **Async Email:** Email sent in background (verified via logs or email delivery)
- ✅ **Error Handling:** Email failures logged but don't block endpoint
- ✅ **Test #248 Passes:** E2E test completes without timeout
- ✅ **No Regressions:** All other user management tests pass
- ✅ **Code Change:** Minimal (5-10 lines modified in one handler)

### Failure Handling

**If endpoint still times out after change:**
1. Verify goroutine was added correctly (check code review)
2. Check if there's another blocking operation (database query?)
3. Profile with pprof if needed: `go tool pprof http://localhost:6060/debug/pprof/profile`
4. May need Option B (queue-based) or Option C (database-based) if other bottlenecks found

**If email no longer sends:**
1. Goroutine may be exiting before email completes
2. Add `time.Sleep()` in test (not production) to allow goroutine to finish
3. Consider Option B if guaranteed delivery needed

### Effort Estimate

| Task | Duration | Notes |
|------|----------|-------|
| Code change (Option A) | 10 min | Simple goroutine wrap |
| Unit test addition | 10 min | Add async test case |
| Manual testing (curl) | 10 min | Verify response time |
| E2E test validation | 10 min | Run Playwright tests |
| Code review + fixes | 10 min | Address feedback |
| **Total** | **50 min** | Within 30min-1hr estimate |

**If refactoring to Option B during same phase: +60-90 min**

---

## 4. Phase 2.3c: Test Auth Token Refresh (30 min - 1 hour, Parallelizable)

**Priority:** 🟡 MEDIUM
**Owner:** Frontend Developer (or Backend if no separate Frontend)
**Can Run in Parallel:** Yes (with 2.3a and 2.3b)
**Start Time:** Immediately
**Target Completion:** 30 min - 1 hour

### Objective

Implement automatic auth token refresh in Playwright test fixtures to prevent HTTP 401 errors during long-running test sessions (>30 minutes).

### Pre-Execution Verification

**CRITICAL STEP - Do this FIRST before implementing fixtures:**

Verify the refresh endpoint exists and works. If it's missing, you'll need to implement it first (additional 30 min).

#### Manual Verification Script

Run this before starting Phase 2.3c implementation:

```bash
#!/bin/bash
# Pre-check: Verify auth token refresh endpoint exists

echo "[1/3] Getting fresh auth token..."
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"TestPass123!"}' \
  | jq -r '.token')

if [ -z "$TOKEN" ] || [ "$TOKEN" == "null" ]; then
  echo "❌ FAILED: Could not obtain auth token. Check login endpoint."
  exit 1
fi

echo "✅ Token obtained: ${TOKEN:0:20}..."

echo "[2/3] Checking if refresh endpoint exists (POST /api/v1/auth/refresh)..."
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}')

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | head -1)

if [ "$HTTP_CODE" == "404" ]; then
  echo "❌ FAILED: Refresh endpoint not found (HTTP 404)"
  echo "   You must implement POST /api/v1/auth/refresh first (30 min task)"
  exit 1
elif [ "$HTTP_CODE" == "401" ]; then
  echo "❌ FAILED: Refresh endpoint returned 401 (invalid token)"
  echo "   Check token format and auth logic"
  exit 1
elif [ "$HTTP_CODE" == "200" ]; then
  echo "✅ Refresh endpoint exists and returned 200 OK"
  NEW_TOKEN=$(echo "$BODY" | jq -r '.token' 2>/dev/null)
  if [ -z "$NEW_TOKEN" ] || [ "$NEW_TOKEN" == "null" ]; then
    echo "⚠️  WARNING: Endpoint returned 200 but no new token in response"
    echo "   Response body: $BODY"
  else
    echo "✅ New token received: ${NEW_TOKEN:0:20}..."
  fi
else
  echo "⚠️  Unexpected HTTP code: $HTTP_CODE"
  echo "   Response: $BODY"
  exit 1
fi

echo "[3/3] Verification complete"
echo "✅ READY TO PROCEED with Phase 2.3c implementation"
```

**Expected output:**
```
✅ Token obtained: eyJhbGc...
✅ Refresh endpoint exists and returned 200 OK
✅ New token received: eyJhbGc...
✅ READY TO PROCEED with Phase 2.3c implementation
```

**If failed:** Implement `/api/v1/auth/refresh` endpoint first (separate 30-min task before Phase 2.3c)

### Problem Statement

**Current Symptom:**
- E2E tests run for 30+ minutes
- After ~30 min, all API requests fail with HTTP 401 Unauthorized
- Tests timeout waiting for response
- Root cause: JWT auth token expires after 30 minutes

**Why This Happens:**
- JWT token issued at test start with 30-minute expiration
- Long test suites (Phase 3 E2E suite may be 60+ min)
- Token not refreshed before it expires
- All subsequent API calls rejected

**Affected Tests:**
- Full Phase 2 E2E suite (currently <30 min, but approaching limit)
- Phase 3 E2E security testing (60+ min, definitely exceeds token lifetime)
- Any future smoke tests or integration suites

### Current Architecture

**Auth Flow:**
```
Login (POST /auth/login)
    ↓ Returns JWT token + refresh_token
    ↓ Token stored in Playwright fixtures
    ↓ Used in all subsequent API requests
    ↓ Token expires after 30 min
    ↓ ❌ All requests fail with 401
```

**Token Details:**
- **Issued by:** Backend (location: verify where tokens set in login handler)
- **Expires:** 30 minutes (configurable, likely in config or constants)
- **Refresh endpoint:** Assume exists (POST /auth/refresh or similar)
- **Refresh token:** May be issued with JWT for refresh flow

**Current Fixture:**
```typescript
// tests/fixtures/auth.ts (or similar)
// Likely stores token in memory but doesn't refresh
```

### Solution Options

#### Option A: Automatic Token Refresh in Fixtures (Recommended - 30 min)

**Best for:** Playwright-native solution, no backend changes
**File:** `tests/fixtures/auth.ts` (or wherever auth setup exists)

**Implementation:**
```typescript
// tests/fixtures/auth.ts

import { test as base, expect } from '@playwright/test';

export const test = base.extend<{ authenticatedToken: string }>({
    authenticatedToken: async ({ page }, use) => {
        // Login and get token
        const response = await page.request.post('http://localhost:8080/api/v1/auth/login', {
            data: {
                email: process.env.TEST_EMAIL || 'admin@example.com',
                password: process.env.TEST_PASSWORD || 'TestPass123!'
            }
        });

        const { token, expires_at } = await response.json();

        // Create refresh wrapper
        let currentToken = token;
        let tokenExpiry = new Date(expires_at);

        // Auto-refresh before expiry (85% of lifetime = ~25 min into 30 min token)
        const tokenRefreshInterval = setInterval(async () => {
            const now = new Date();
            const timeUntilExpiry = tokenExpiry.getTime() - now.getTime();

            // Refresh if within 5 minutes of expiry
            if (timeUntilExpiry < 5 * 60 * 1000) {
                try {
                    const refreshResponse = await page.request.post(
                        'http://localhost:8080/api/v1/auth/refresh',
                        {
                            headers: {
                                'Authorization': `Bearer ${currentToken}`
                            }
                        }
                    );

                    if (refreshResponse.ok()) {
                        const refreshData = await refreshResponse.json();
                        currentToken = refreshData.token;
                        tokenExpiry = new Date(refreshData.expires_at);
                        console.log('[AUTH] Token refreshed successfully');
                    } else {
                        console.warn('[AUTH] Token refresh failed', refreshResponse.status());
                    }
                } catch (err) {
                    console.error('[AUTH] Token refresh error:', err);
                }
            }
        }, 60 * 1000); // Check every 1 minute

        // Use token in tests
        await use(currentToken);

        // Cleanup
        clearInterval(tokenRefreshInterval);
    }
});

// In tests, use the authenticatedToken fixture:
// test('example', async ({ page, authenticatedToken }) => {
//     await page.request.get('/api/v1/users', {
//         headers: { 'Authorization': `Bearer ${authenticatedToken}` }
//     });
// });
```

**Pros:**
- ✅ No backend changes needed
- ✅ Automatic & transparent to tests
- ✅ Handles token expiry gracefully
- ✅ Works with existing auth infrastructure

**Cons:**
- ⚠️ Assumes refresh endpoint exists
- ⚠️ Slight overhead (periodic checks)

#### Option B: Longer Token Expiration for Tests (5 min)

**Best for:** Quick fix if refresh endpoint doesn't exist
**File:** Backend config or test environment setup

**Implementation:**
```bash
# Environment variable approach
TEST_JWT_EXPIRATION=1440  # 24 hours instead of 30 min

# Or in backend config
CHARON_JWT_EXPIRATION_MINUTES=1440  # For test environment only
```

**Pros:**
- ✅ Single line change
- ✅ No fixture complexity

**Cons:**
- ❌ Reduces security (longer token lifetime)
- ❌ Only suitable for test environment
- ❌ May not work if backend doesn't respect env var

#### Option C: Cache & Reuse Auth Token (Recommended addition - 15 min)

**Best for:** Combining with Option A for reliability
**File:** `tests/fixtures/auth.ts`

**Implementation:**
```typescript
// Store token on disk between test runs
const tokenCachePath = './test-auth-cache.json';

export const test = base.extend<{ authenticatedToken: string }>({
    authenticatedToken: async ({ page }, use) => {
        let token = null;
        let tokenExpiry = null;

        // Try to load cached token first
        try {
            const cached = JSON.parse(fs.readFileSync(tokenCachePath, 'utf-8'));
            const expiryTime = new Date(cached.expires_at);

            if (expiryTime > new Date()) {
                // Token still valid
                token = cached.token;
                tokenExpiry = expiryTime;
                console.log('[AUTH] Using cached token');
            }
        } catch (err) {
            // Cache doesn't exist or invalid
        }

        // If no valid cached token, login
        if (!token) {
            const response = await page.request.post(
                'http://localhost:8080/api/v1/auth/login',
                {
                    data: {
                        email: process.env.TEST_EMAIL || 'admin@example.com',
                        password: process.env.TEST_PASSWORD || 'TestPass123!'
                    }
                }
            );

            const data = await response.json();
            token = data.token;
            tokenExpiry = new Date(data.expires_at);

            // Cache for next test run
            fs.writeFileSync(tokenCachePath, JSON.stringify({
                token,
                expires_at: tokenExpiry.toISOString()
            }));
        }

        // Refresh if needed (reuse token too)
        const refreshInterval = setInterval(async () => {
            // ... same as Option A
        }, 60 * 1000);

        await use(token);
        clearInterval(refreshInterval);
    }
});
```

**Pros:**
- ✅ Reuses token across test runs
- ✅ Faster startup (skip login on valid cached token)
- ✅ Automatic refresh if cache near expiry

**Cons:**
- ⚠️ Requires gitignore for cache file
- ⚠️ File-based cache less robust

### Recommended Approach for Phase 2.3c

**Execute Option A + Option C** (45 min total)
1. Add automatic token refresh in fixtures (Option A) - 30 min
2. Cache token for reuse across test runs (Option C) - 15 min

### Implementation: Option A + C (45 min)

#### File: `tests/fixtures/auth.ts`

**Assumption:** File exists (standard Playwright fixture pattern)

**Current file likely contains:**
```typescript
import { test as base } from '@playwright/test';

export const test = base.extend({
    // existing fixtures
});
```

**Add auth with refresh:**
```typescript
import { test as base, expect } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

const TOKEN_CACHE_PATH = path.join(__dirname, '../../.auth-token-cache.json');

export const test = base.extend<{
    authenticatedToken: string;
    apiHeaders: (token: string) => Record<string, string>;
}>({
    authenticatedToken: async ({ page, context }, use) => {
        let currentToken = '';
        let tokenExpiry = new Date(0);

        /**
         * Load cached token if still valid
         */
        function loadCachedToken(): string | null {
            try {
                if (fs.existsSync(TOKEN_CACHE_PATH)) {
                    const cached = JSON.parse(fs.readFileSync(TOKEN_CACHE_PATH, 'utf-8'));
                    const expiry = new Date(cached.expires_at);

                    if (expiry > new Date()) {
                        console.log('[AUTH] Using cached token (valid until ' + expiry.toISOString() + ')');
                        tokenExpiry = expiry;
                        return cached.token;
                    }
                }
            } catch (err) {
                console.warn('[AUTH] Failed to load cached token:', err);
            }
            return null;
        }

        /**
         * Save token to cache
         */
        function cacheToken(token: string, expiresAt: string): void {
            try {
                fs.writeFileSync(TOKEN_CACHE_PATH, JSON.stringify(
                    { token, expires_at: expiresAt },
                    null,
                    2
                ));
                console.log('[AUTH] Token cached for future test runs');
            } catch (err) {
                console.warn('[AUTH] Failed to cache token:', err);
            }
        }

        /**
         * Refresh token when near expiry
         */
        async function refreshToken(): Promise<boolean> {
            try {
                const response = await page.request.post(
                    'http://localhost:8080/api/v1/auth/refresh',
                    {
                        headers: {
                            'Authorization': `Bearer ${currentToken}`
                        }
                    }
                );

                if (response.ok()) {
                    const data = await response.json();
                    currentToken = data.token;
                    tokenExpiry = new Date(data.expires_at);
                    cacheToken(currentToken, data.expires_at);
                    console.log('[AUTH] Token refreshed (new expiry: ' + data.expires_at + ')');
                    return true;
                } else {
                    console.warn('[AUTH] Token refresh failed:', response.status());
                    return false;
                }
            } catch (err) {
                console.error('[AUTH] Token refresh error:', err);
                return false;
            }
        }

        /**
         * Get or create fresh token
         */
        async function ensureValidToken(): Promise<string> {
            const now = new Date();
            const timeUntilExpiry = tokenExpiry.getTime() - now.getTime();

            // If token expires in less than 5 minutes, refresh
            if (timeUntilExpiry < 5 * 60 * 1000 && currentToken) {
                await refreshToken();
                return currentToken;
            }

            // If no token, try cache, then login
            if (!currentToken) {
                currentToken = loadCachedToken() || '';
            }

            if (!currentToken) {
                // No cached token, login fresh
                const loginResponse = await page.request.post(
                    'http://localhost:8080/api/v1/auth/login',
                    {
                        data: {
                            email: process.env.TEST_EMAIL || 'admin@example.com',
                            password: process.env.TEST_PASSWORD || 'TestPass123!'
                        }
                    }
                );

                if (!loginResponse.ok()) {
                    throw new Error(`Login failed: ${loginResponse.status()}`);
                }

                const data = await loginResponse.json();
                currentToken = data.token;
                tokenExpiry = new Date(data.expires_at);
                cacheToken(currentToken, data.expires_at);
                console.log('[AUTH] Fresh token obtained (expiry: ' + data.expires_at + ')');
            }

            return currentToken;
        }

        // Setup interval to refresh before expiry
        const refreshCheckInterval = setInterval(async () => {
            const now = new Date();
            const timeUntilExpiry = tokenExpiry.getTime() - now.getTime();

            if (currentToken && timeUntilExpiry < 5 * 60 * 1000) {
                await refreshToken();
            }
        }, 60 * 1000); // Check every minute

        // Ensure token on first use
        await ensureValidToken();

        // Provide token to tests
        await use(currentToken);

        // Cleanup
        clearInterval(refreshCheckInterval);
    },

    /**
     * Helper to generate authenticated API headers
     */
    apiHeaders: async ({ authenticatedToken }, use) => {
        const getHeaders = (token: string) => ({
            'Authorization': `Bearer ${token}`,
            'Content-Type': 'application/json'
        });

        await use(getHeaders);
    }
});

export { expect };
```

**Update `.gitignore`:**
```
# Auth cache (test-only, contains valid JWT)
.auth-token-cache.json
```

#### Concurrency Safety: Cache File Locking

**IMPORTANT:** If Playwright tests run with `--workers=N` (parallel workers), multiple test instances write to `.auth-token-cache.json` simultaneously. This can corrupt the JSON file.

**Add file locking to prevent corruption:**

Install dependency:
```bash
npm install --save-dev async-lock
```

Update `tests/fixtures/auth.ts` with locking:

```typescript
import * as fs from 'fs';
import * as path from 'path';
import AsyncLock from 'async-lock';

const TOKEN_CACHE_PATH = path.join(__dirname, '../../.auth-token-cache.json');
const cacheLock = new AsyncLock();  // Prevent concurrent writes

// Update cacheToken function (found in the extended fixture code above):
function cacheToken(token: string, expiresAt: string): void {
    // Use lock to ensure only one worker writes cache at a time
    cacheLock.acquire('auth-cache', () => {
        try {
            fs.writeFileSync(TOKEN_CACHE_PATH, JSON.stringify(
                { token, expires_at: expiresAt },
                null,
                2
            ));
            console.log('[AUTH] Token cached safely (locked write)');
        } catch (err) {
            console.warn('[AUTH] Failed to cache token:', err);
        }
    });
}
```

**Why this matters:**
- **Without locking:** 2 workers write simultaneously → corrupted JSON file → cache becomes unusable
- **With locking:** Only 1 worker writes at a time → safe JSON file → cache works reliably

**When to use:**
- ✅ Use if running: `npx playwright test --workers=2` or higher
- ❌ Not needed if running with `--workers=1` (sequential)

#### Update test usage:

**Before (using raw token):**
```typescript
test('should list users', async ({ page }) => {
    const response = await page.request.get('http://localhost:8080/api/v1/users', {
        headers: {
            'Authorization': `Bearer ${token}`
        }
    });
});
```

**After (using fixtures):**
```typescript
import { test, expect } from '../fixtures/auth';

test('should list users', async ({ page, apiHeaders, authenticatedToken }) => {
    const response = await page.request.get(
        'http://localhost:8080/api/v1/users',
        {
            headers: apiHeaders(authenticatedToken)
        }
    );

    expect(response.ok()).toBeTruthy();
});
```

### Testing Strategy: Phase 2.3c

#### Test 1: Single Long-Running Test (20 min)

**Objective:** Verify token doesn't expire in 60-minute test session

```bash
# Run a single test that takes 30+ minutes
# This should complete without 401 errors

npx playwright test \
  tests/some-long-test.spec.ts::test('60-minute task') \
  --timeout=3600000  # 60 minutes
```

**Expected Result:**
- ✅ No HTTP 401 errors mid-test
- ✅ Token refreshed at ~25 min mark (verify in console logs)
- ✅ All API calls succeed

#### Test 2: Full Phase 2 E2E Suite (30 min)

```bash
# Run all Phase 2 E2E tests
npx playwright test \
  tests/phase2/ \
  --reporter=html

# Expected:
# ✅ All tests complete
# ✅ No 401 errors
# ✅ Console logs show token refresh events
```

**Verification:**
- Check console for: `[AUTH] Token refreshed`
- Check for cached token: `ls -la .auth-token-cache.json`

#### Test 3: Verify Cache Reuse (5 min)

```bash
# Run suite twice to verify token reuse
npx playwright test tests/phase2/ --workers=1

# Look for:
# First run: "[AUTH] Fresh token obtained"
# Run console log again (or second invocation):
# "[AUTH] Using cached token"
```

#### Test 4: Verify Refresh Endpoint (5 min)

**Manual test:**
```bash
# Get token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"TestPass123!"}' | jq -r '.token')

# Try refresh endpoint
curl -s -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Authorization: Bearer $TOKEN" | jq .

# Expected:
# {
#   "token": "eyJ...",
#   "expires_at": "2026-02-09T15:30:00Z"
# }
```

### Success Criteria: Phase 2.3c

- ✅ **No 401 Errors:** 60+ minute test run completes without HTTP 401
- ✅ **Token Refresh:** Logs show token is refreshed automatically
- ✅ **Cache Reuse:** Second test run uses cached token (not login again)
- ✅ **Endpoint Works:** Refresh endpoint accessible and returns new token
- ✅ **All API Calls Succeed:** No auth-related failures in test output

### Failure Handling

**If still getting 401 errors:**
1. Verify refresh endpoint exists: `curl -X POST /api/v1/auth/refresh`
2. Check token expiry time: `jwt.io` to decode token
3. If refresh endpoint missing, implement it first (30 min task)
4. If token lifetime config found, try Option B (longer lifetime)

**If cache causes issues:**
1. Delete `.auth-token-cache.json` and re-run
2. Disable caching (comment out cache code) to isolate issue
3. Document cache invalidation triggers if needed

### Effort Estimate: Phase 2.3c

| Task | Duration | Notes |
|------|----------|-------|
| Create/update auth fixture | 20 min | Add refresh logic |
| Add token cache | 10 min | File-based cache |
| Update test imports | 5 min | Use new fixtures |
| Manual testing | 10 min | Verify no 401s |
| **Total** | **45 min** | Within 30min-1hr estimate |

---

## 5. Parallelization Strategy

### Execution Model: Concurrent Work Groups

**All three phases can run in parallel with minimal conflicts:**

#### Independence Analysis

| Phase | Phase | Can Run Parallel? | Reason |
|-------|-------|------------------|--------|
| 2.3a | 2.3b | ✅ YES | Different files (go.mod vs user_handler.go) |
| 2.3a | 2.3c | ✅ YES | Different layers (backend deps vs frontend fixtures) |
| 2.3b | 2.3c | ✅ YES | Different languages (Go vs TypeScript) |

**Key:** No shared code modifications or merge conflicts expected.

### Execution Timeline Scenarios

#### **SCENARIO A: Separate Machines or Teams (True Parallel - 1h wall-clock)**

```
Dev A (2.3a): Dependency update (1 hour)
Dev B (2.3b): Async email refactor (1 hour)
Dev C (2.3c): Auth token refresh (45 min)

All three run simultaneously:
09:00 - START all three
09:45 - Dev C complete (Phase 2.3c done)
10:00 - Dev A & B complete (Phases 2.3a & 2.3b done)
10:00-10:15 - Integration testing
10:15 - PHASE 3 READY

Total wall-clock: 1 hour 15 minutes
```

#### **SCENARIO B: Shared Repository with Coordination (1h 50min wall-clock)**

```
09:00 - Dev A starts 2.3a, Dev B waits, Dev C starts 2.3c (parallel)
        └─ A is working on go.mod (no conflicts)
        └─ C is working on test fixtures (no conflicts)
        └─ B waits for A to commit

10:00 - Dev A finishes 2.3a, commits
        └─ Dev B pulls latest (no conflicts)
        └─ Dev B starts 2.3b

09:45 - Dev C finishes 2.3c (started at 09:00)

10:50 - Dev B finishes 2.3b
        └─ All three phases complete

10:50-11:05 - Integration testing
11:05 - PHASE 3 READY

Total wall-clock: 1 hour 50 minutes (sequential backend, parallel frontend)
Why slower than A: Backend 2.3b must wait for 2.3a commit, but frontend 2.3c runs in parallel
```

#### **SCENARIO C: Single Developer (2h 45min wall-clock)**

```
09:00 - Dev starts 2.3a (Dependency Update)
10:00 - Dev completes 2.3a, starts 2.3b (Async Email)
        └─ Commits 2.3a changes first

10:50 - Dev completes 2.3b, starts 2.3c (Auth Token)
        └─ Commits 2.3b changes

11:35 - Dev completes 2.3c
        └─ Commits 2.3c changes

11:35-11:50 - Integration testing
11:50 - PHASE 3 READY

Total wall-clock: 2 hours 45 minutes (pure serial)
```

### Team Assignments & Schedule

| Phase | Owner Role | Duration | Start | Expected Finish | Code Reviewer | Notes |
|-------|------------|----------|-------|-----------------|---------------|-------|
| 2.3a | Backend Dev | 1h | 09:00 | 10:00 | Tech Lead | Dependency security update |
| 2.3b | Backend Dev (same or different) | 1h | 09:00* | 10:00* | Senior Backend | Async email refactor |
| 2.3c | Frontend Dev | 45min | 09:00 | 09:45 | Frontend Lead | Token refresh fixtures |
| Integration Test | QA Lead | 15min | 10:00 | 10:15 | Tech Lead | Smoke test all changes |
| Phase 3 Approval | Tech Lead | 5min | 10:15 | 10:20 | - | Go/no-go decision |

**Notes:**
- *2.3b timing depends on parallelization scenario:
  - Scenario A/B: Dev B can start at 09:00 (different developer) → finish 10:00
  - Scenario B (shared repo): Dev B waits for 2.3a commit → start 10:00 → finish 11:00
  - Scenario C (single dev): Dev A after 2.3a → start 10:00 → finish 10:50

- Actual names and assignments based on team availability
- Dev can be same person (sequential) or different people (parallel)
- Code reviewers assigned in parallel with implementation

#### Role Definitions

| Role | Responsibilities | Example |
|------|------------------|----------|
| Backend Dev | Implement 2.3a & 2.3b code changes | Alice (Go expertise) |
| Frontend Dev | Implement 2.3c fixture changes | Bob (TypeScript/Playwright) |
| Tech Lead | Approve go/no-go for Phase 3 | Charlie (Architecture) |
| QA Lead | Run integration tests | Diana (Test expertise) |
| Senior Backend | Review async email implementation | Dave (async/concurrency expert) |
| Frontend Lead | Review Playwright fixture changes | Eve (test automation) |

### Coordination Points

**Minimal coordination needed:**
- ✅ All phases independent
- ✅ No git conflicts expected (different files)
- ✅ No integration dependencies
- ✅ Can commit independently

**Recommended coordination:**
- [] 09:00: All devs start simultaneously
- [] 09:30: Quick sync (Slack/Teams) - any blockers?
- [] 10:00: Check 2.3a validation complete
- [] 10:15: Final integration test before Phase 3 approval

---

## 6. Risk Assessment & Mitigation

### Risk Matrix

| Risk | Severity | Probability | Impact | Mitigation | Owner |
|------|----------|------------|--------|-----------|-------|
| **Async email sends wrong data** | HIGH | Medium | Invite emails contain wrong token | Add unit test with email content verification | Dev B |
| **Async email never sends silently** | HIGH | Low | Users don't receive invites | Add audit log when job queued, monitor logs | Dev B |
| **Token refresh loop failures** | MEDIUM | Low | 401 errors during long tests | Verify refresh endpoint exists first (manual test) | Dev C |
| **Dependency update breaks auth** | MEDIUM | Very Low | Login broken after crypto update | Build Docker image before committing | Dev A |
| **Cached token invalid between runs** | MEDIUM | Low | Test fails with invalid token | Add cache expiry validation | Dev C |
| **Multiple devs modify user_handler.go** | LOW | Low | Git merge conflicts | Dev A commits 2.3a first, Dev B pulls latest before 2.3b | Dev A, B |
| **Email queue loses jobs on crash** | LOW | Low | Some invites unsent in production | Document Option A limitation, plan Option B migration | Dev B |

### Detailed Risk Mitigation

#### Risk 1: Async Email Data Corruption

**Scenario:** Email sent with previous test's data or corrupted token

**Mitigation:**
- Add unit test with email verification
- Log email content when sending
- Verify test data doesn't leak

**Example test:**
```go
func TestInviteEmailCorrectData(t *testing.T) {
    // Setup: capture email data
    var sentEmail EmailData
    mockService := &MockMailService{
        OnSendInvite: func(email, token, appName, baseURL string) error {
            sentEmail = EmailData{email, token, appName, baseURL}
            return nil
        },
    }

    // Act: invite user
    handler.InviteUser(ctx, "newemail@test.com")

    // Wait for goroutine
    time.Sleep(100 * time.Millisecond)

    // Assert: email data correct
    assert.Equal(t, "newemail@test.com", sentEmail.Email)
    assert.NotEmpty(t, sentEmail.Token)
    assert.NotEmpty(t, sentEmail.AppName)
}
```

#### Risk 2: Silent Email Failures

**Scenario:** Email fails to send, but no one notices

**Mitigation:**
- Add structured logging for all email attempts
- Commit audit log entry when job queued (separate from success)
- Monitor logs post-deployment for "Failed to send" messages

**Example logging:**
```go
go func() {
    auditLog(user.ID, "invite_email_queued", token)

    if err := h.MailService.SendInvite(...); err != nil {
        auditLog(user.ID, "invite_email_failed", err.Error())
        h.Logger.Error("invite email failed",
            zap.String("user_email", user.Email),
            zap.Error(err))
    } else {
        auditLog(user.ID, "invite_email_sent", "")
    }
}()
```

#### Risk 3: Token Refresh Endpoint Missing

**Scenario:** Refresh endpoint doesn't exist, token refresh fails

**Mitigation:**
- Pre-test refresh endpoint before implementing fixture
- If missing, implement it first (additional 30 min)
- Fall back to Option B (longer token lifetime) if needed

**Manual verification (do this first):**
```bash
# Step 1: Get token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"TestPass123!"}' \
  | jq -r '.token')

# Step 2: Try to refresh
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Authorization: Bearer $TOKEN"

# Expected: 200 OK with new token
# If 404: endpoint missing, implement it first
```

#### Risk 4: Dependency Update Breaks Compilation

**Scenario:** Updated crypto library has breaking API changes

**Mitigation:**
- Build Docker image (compiles all code)
- Smoke test login endpoint
- Review changelog for breaking changes

**If build fails:**
```bash
# Check what changed
go mod graph | grep crypto

# Review changelog
# May need code updates in cryptography-related handlers

# Last resort: downgrade to specific working version
go get golang.org/x/crypto@v0.30.0  # (if v0.31.0 breaks)
```

#### Risk 5: Cached Auth Token Causes Test Failures

**Scenario:** Cached token is invalid (user deleted, permissions revoked)

**Mitigation:**
- Add TTL to cache (15 minutes max)
- Verify token with simple API call before reuse
- Re-login if cache not valid

**Enhanced cache validation:**
```typescript
async function validateCachedToken(token: string, page: Page): Promise<boolean> {
    try {
        const response = await page.request.get(
            'http://localhost:8080/api/v1/auth/validate',
            { headers: { 'Authorization': `Bearer ${token}` } }
        );
        return response.ok();
    } catch {
        return false;
    }
}
```

#### Risk 6: Git Merge Conflicts in user_handler.go

**Scenario:** Multiple devs edit same file, merge conflict on commit

**Mitigation:**
- **Commit order:** Dev A (2.3a) → rebase → Dev B (2.3b)
- Dev B pulls latest before starting
- Small, focused edits minimize conflict chance

**Git workflow:**
```bash
# Dev A commits first
git add backend/go.mod backend/go.sum
git commit -m "chore(deps): update golang.org/x/crypto and dependencies"
git push

# Dev B pulls and checks for changes
git pull
git status  # Verify no conflicts

# Dev B makes edit in user_handler.go
git add backend/internal/api/handlers/user_handler.go
git commit -m "fix(api): make InviteUser async to prevent HTTP blocking"
git push
```

#### Risk 7: Email Queue Jobs Lost on Service Crash (Option A Only)

**Scenario:** Service crashes, in-flight goroutines lost, emails don't send

**Mitigation:**
- Document as Phase 2.3b limitation
- Plan migration to Option B (queue-based) for Phase 2.4
- In production, prefer Option C (database-persisted) if critical

**Note:** For MVP (Phase 2.3), Option A acceptable since:
- Email serves optional invite convenience
- User can always resend invite
- Can function without email delivery


---

## 7. Validation & Sign-Off

### Pre-Remediation Checks

**Before starting any phase:**

- [ ] All three phases understood by assigned developers
- [ ] Git repository clean (no uncommitted changes)
- [ ] Latest main branch pulled locally
- [ ] Test environment up and running
- [ ] All tools available (go, docker, npm, trivy, curl)

### Phase 2.3a Validation

#### Automated Checks

```bash
# 1. Dependency versions updated
go list -m golang.org/x/crypto | grep "v0\.3[1-9]"  # ✅ Must show v0.31.0+

# 2. Build succeeds
docker build -t charon:local . 2>&1 | tail -5  # ✅ Must show "Successfully tagged charon:local"

# 3. Container scan passes
trivy image --severity CRITICAL charon:local  # ✅ Must show "Total: 0"

# 4. Smoke test succeeds
curl -s http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer $(curl -s -X POST http://localhost:8080/api/v1/auth/login \
    -H "Content-Type: application/json" \
    -d '{"email":"admin@example.com","password":"TestPass123!"}' | jq -r '.token')" | jq '.items | length'
# ✅ Must return number > 0 (users listed)
```

#### Manual Verification

- [ ] Docker build output contains no warnings
- [ ] Trivy report shows vulnerability from CVE-2024-45337 resolved
- [ ] Login endpoint responds immediately (<200ms)
- [ ] User list endpoint works with valid token

#### Sign-Off Criteria

```markdown
**Phase 2.3a: COMPLETE** ✅

- [x] Dependencies updated to latest
- [x] Docker image builds without errors
- [x] Trivy scan passes (0 CRITICAL)
- [x] Smoke tests pass (login, list users)
- [x] No new test failures introduced

**Commit:** `chore(deps): update golang.org/x/crypto and dependencies`
**PR Ready:** Yes
```

### Phase 2.3b Validation

#### Automated Checks

```bash
# 1. Code compiles
cd backend && go build -v ./...  # ✅ Must show build output

# 2. Unit tests pass
go test ./... -short -v 2>&1 | grep -E "PASS|FAIL"  # ✅ All PASS

# 3. E2E test #248 passes
npx playwright test \
  tests/user-management.spec.ts --grep="invite user" \
  --timeout=5000 2>&1 | tail -20  # ✅ Must show: 1 passed
```

#### Performance Verification

```bash
# Measure endpoint response time
time curl -s -X POST http://localhost:8080/api/v1/users/invite \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email":"measure@test.com"}' > /dev/null

# Expected: real 0m0.150s (NOT > 1s)
```

#### Manual Verification

- [ ] InviteUser returns in <200ms
- [ ] User appears in database immediately after response
- [ ] Test #248 completes without timeout
- [ ] Test #258-270 all pass
- [ ] Email logs show async sending
- [ ] No error messages in test output

#### Sign-Off Criteria

```markdown
**Phase 2.3b: COMPLETE** ✅

- [x] InviteUser refactored to async
- [x] Response time < 200ms (verified with curl)
- [x] Test #248 passes (user created, no timeout)
- [x] All user management tests pass (6 related tests)
- [x] No regressions in other handlers
- [x] Error handling verified (failed email logged, doesn't break endpoint)

**Commit:** `fix(api): make InviteUser async to prevent HTTP blocking`
**PR Ready:** Yes
```

### Phase 2.3c Validation

#### Automated Checks

```bash
# 1. Fixture syntax correct
npx eslint tests/fixtures/auth.ts  # ✅ Must show: 0 errors

# 2. Long test doesn't timeout
npx playwright test \
  tests/health-check.spec.ts \
  --timeout=3600000 \
  --workers=1 2>&1 | grep -E "passed|failed"  # ✅ Must show: 1 passed

# 3. No 401 errors in logs
npx playwright test tests/ 2>&1 | grep -c "401"  # ✅ Must return: 0
```

#### Manual Verification

- [ ] Playwright test runs for 60+ minutes without 401
- [ ] Console logs show: `[AUTH] Token refreshed...`
- [ ] Cache file created: `.auth-token-cache.json` exists
- [ ] Second test run uses cached token
- [ ] Refresh endpoint returns valid token

#### Sign-Off Criteria

```markdown
**Phase 2.3c: COMPLETE** ✅

- [x] Auth fixture created with token refresh logic
- [x] 60-minute test run completes with no 401 errors
- [x] Token automatically refreshed when near expiry
- [x] Token cached for future test runs
- [x] Credential refresh endpoint verified working
- [x] No test behavior changes (all Phase 2 tests still pass)

**Commit:** `test: add automatic token refresh for long test sessions`
**PR Ready:** Yes
```

### Integration Testing (All Phases)

**After all three phases complete:**

```bash
# Full smoke test suite
npx playwright test tests/ --reporter=html

# Container verification
docker run -d --name final-check -p 8080:8080 charon:local
sleep 5

# Test auth flow
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"TestPass123!"}' | jq -r '.token')

# Test user creation
curl -s -X POST http://localhost:8080/api/v1/users/invite \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"email":"final@test.com"}' | jq '.id'

# Verify scan still clean
trivy image charon:local --severity CRITICAL

docker stop final-check
docker rm final-check
```

**Expected Results:**
- ✅ All endpoint responses successful
- ✅ Token valid and properly used
- ✅ User creation fast (<200ms)
- ✅ Container scan still clean

### Final Sign-Off Checklist

```markdown
## Phase 2.3 COMPLETE - Ready for Phase 3

**Date Completed:** [TIMESTAMP]
**Total Time:** [ACTUAL TIME VS ESTIMATE]
**Developers:** [NAMES]

### Phase 2.3a: Dependency Security ✅
- [x] golang.org/x/crypto v0.31.0+
- [x] Trivy scan passes
- [x] Docker image builds
- [x] Smoke tests pass

### Phase 2.3b: Async Email ✅
- [x] InviteUser response < 200ms
- [x] Test #248 passes
- [x] All user management tests pass
- [x] No regressions

### Phase 2.3c: Auth Token Refresh ✅
- [x] 60+ minute test runs without 401
- [x] Token auto-refresh working
- [x] Cache mechanism functional
- [x] Refresh endpoint verified

### Integration Testing ✅
- [x] Full E2E suite passes
- [x] Container scan clean
- [x] All endpoints responding

### Security Approval ✅
- [x] No CRITICAL vulnerabilities
- [x] No new security concerns
- [x] Dependencies verified

### Code Review Status ✅
- [x] All commits reviewed
- [x] Code follows project standards
- [x] Tests passing
- [x] Ready to merge

### Phase 3 Readiness: **APPROVED** ✅

All critical fixes complete. Ready to proceed with Phase 3 E2E security testing.

Authorized by: [TECH LEAD NAME]
Date: [DATE]
```

---

## 8. Time Estimates & Critical Path

### Detailed Task Breakdown

#### Phase 2.3a: Dependency Update

| Task | Effort | Critical Path |
|------|--------|-----------------|
| Update dependencies (go get) | 5 min | YES |
| Run go mod tidy & verify | 5 min | YES |
| Build Docker image | 7 min | YES |
| Container security scan | 5 min | YES |
| Smoke test (login, list users) | 5 min | YES |
| **Subtotal** | **27-30 min** | **Serial** |
| Buffer (10% for troubleshooting) | 3 min | - |
| **Total** | **1 hour** | ✅ Realistic estimate |

#### Phase 2.3b: Async Email Refactor (Option A)

| Task | Effort | Critical Path |
|------|--------|------------------|
| Code change (wrap in goroutine) | 5 min | YES |
| Update user_handler.go | 5 min | YES |
| Add error logging (Logger usage) | 3 min | NO |
| Build & compile test | 2 min | YES |
| Unit test addition (response time test) | 10 min | NO |
| E2E test validation (#248) | 10 min | YES |
| Test suite validation (all user tests) | 10 min | YES |
| Code review & fixes | 5 min | YES |
| **Subtotal** | **50 min** | **Serial** |
| Buffer (10%) | 5 min | - |
| **Total** | **55-60 min ~= 1 hour** | ✅ Within estimate |

#### Phase 2.3c: Auth Token Refresh

| Task | Effort | Critical Path |
|------|--------|------------------|
| Verify refresh endpoint exists (manual test) | 5 min | YES |
| Create/update auth fixture file | 15 min | YES |
| Add token refresh interval logic | 10 min | YES |
| Add token caching (file-based) | 8 min | NO |
| Update test imports/usage | 5 min | YES |
| 60-min test validation | 10 min | YES |
| Cache verification (second run) | 5 min | NO |
| Code review & fixes | 5 min | YES |
| **Subtotal** | **40 min** | **Serial** |
| Buffer (10%) | 4 min | - |
| **Total** | **44-45 min ~= 1 hour** | ✅ Within estimate |

### Timeline Visualization

#### Parallel Execution (Recommended)

```
Timeline (hours)
0h    1h    2h    3h
|-----|-----|-----|
2.3a: [=====] 1h         (Dev A: Dependencies)
2.3b: [================] 1h (Dev B: Async Email)
2.3c: [============] 45m  (Dev C: Auth Token)
                    |
                3b mark (all complete: <1.5h wall-clock)

Wall-clock total: 1 hour (limited by longest task = 2.3b)
```

#### Sequential Execution (If 1 developer)

```
Timeline (hours)
0h    1h    2h    3h
|-----|-----|-----|-----|
2.3a: [=====] 1h
      2.3b: [================] 1h
                    2.3c: [============] 45m
                                      |
                                    2h50m (all complete)

Wall-clock total: 2h50m
```

### Critical Path Analysis

**Critical path = longest task dependency chain**

```
2.3a: 1 hour (completely independent)
2.3b: 1 hour (can start immediately, no deps on 2.3a)
2.3c: 45 min (can start immediately, depends on refresh endpoint existing)
       ↓
       If refresh endpoint missing: +30 min implementation needed
```

**Longest path:** max(1h, 1h, 45min) = **1 hour in parallel**

### Realistic Time Estimates with Buffers

| Scenario | Estimate | Confidence | Notes |
|----------|----------|-----------|-------|
| **Best case (no issues)** | 1 hour | 20% | All changes work first try |
| **Expected (1-2 small issues)** | 1.5 hours | 70% | Typical: need one test retry, one small fix |
| **Worst case (major issue)** | 3 hours | 10% | Unlikely: e.g., refresh endpoint missing |

**Recommended buffer: 1.5 hours total (50% of base estimate)**
**Plan for: 10:00-11:30 (assuming 09:30 start)**

---

## 9. Phase 3 Blocking Dependencies

### Dependency Graph

```
Phase 3 E2E Security Testing
    ├─ Requires: Phase 2.3a ✅ (CRITICAL)
    │   ├─ Reason: No CRITICAL vulnerabilities in production
    │   ├─ Blocker type: Security compliance
    │   └─ Time impact: Fail if not complete
    │
    ├─ Requires: Phase 2.3b ⚠️ (HIGH)
    │   ├─ Reason: User management tests must pass
    │   ├─ Blocker type: Functional requirement
    │   └─ Time impact: User-related Phase 3 tests fail/timeout
    │
    └─ Requires: Phase 2.3c ✅ (CRITICAL)
        ├─ Reason: Long test sessions timeout with 401
        ├─ Blocker type: Test infrastructure
        └─ Time impact: Phase 3 tests fail after 30 min
```

### Phase 3 Readiness Schecklist

Before starting Phase 3, verify:

```markdown
## Phase 3 Readiness Check

**2.3a - Security Compliance ✅ REQUIRED**
- [ ] CVE-2024-45337 NOT present in image
- [ ] All golang.org/x packages updated
- [ ] Trivy scan reports 0 CRITICAL

**2.3b - Functional Requirement ✅ REQUIRED**
- [ ] User invite endpoint responds in <200ms
- [ ] Test #248 (invite user) passes
- [ ] Tests #258-270 (other user ops) pass
- [ ] No timeout errors in user management

**2.3c - Test Infrastructure ✅ REQUIRED**
- [ ] Auth fixtures support token refresh
- [ ] 60-minute test run without 401 errors
- [ ] Token cache functional (optional but helpful)

**Full E2E Suite ✅ REQUIRED (smoke test)**
- [ ] All Phase 2 tests pass
- [ ] >95% pass rate (acceptable for remediation phase)
- [ ] No new vulnerabilities introduced
- [ ] Container builds successfully

**GO** → Phase 3 when ALL checks pass
**NO-GO** → Fix remaining issues before Phase 3
```

---

## 10. Risk Escalation & Decision Gates

### Decision Gates

**Gate 1: Phase 2.3a Complete** (1 hour)
- ✅ Decision: APPROVE to proceed to 2.3b+c
- ❌ Decision: HALT - investigate CVE vulnerability

**Gate 2: Phase 2.3b Complete** (2 hours)
- ✅ Decision: APPROVE Phase 2.3c
- ⚠️ Decision: CONDITIONAL - if user tests still failing, delay Phase 3

**Gate 3: Phase 2.3c Complete** (2.5 hours)
- ✅ Decision: APPROVE Phase 3 start
- ❌ Decision: HALT - auth infrastructure issue

**Gate 4: Integration Testing** (2.5 hours)
- ✅ Decision: APPROVED FOR PHASE 3
- ⚠️ Decision: CONDITIONAL - proceed with caution, monitor Phase 3 closely
- ❌ Decision: REJECT - rework Phase 2 sections before Phase 3

### Escalation Path

**If Phase 2.3a fails (Dependency Update):**
1. **Owner:** Backend Dev + Tech Lead
2. **Action:** Investigate breaking change in crypto API
3. **Options:**
   - Downgrade to specific version if 0.31.0 incompatible
   - Update code for API changes
   - Block Phase 3 until resolved
4. **Timeline:** +30 min investigation + fix

**If Phase 2.3b fails (Async Email - Test #248 still times out):**
1. **Owner:** Backend Dev
2. **Action:** Profile endpoint, identify actual bottleneck
3. **Options:**
   - Async refactor insufficient → use Option B (queue-based)
   - Bottleneck elsewhere (database query?) → investigate separate
   - Email service misconfiguration → check logs
4. **Timeline:** Can proceed to Phase 3 but mark user management as "defer testing"

**If Phase 2.3c fails (Auth Token Refresh):**
1. **Owner:** Frontend Dev + Backend Dev
2. **Action:** Check refresh endpoint exists and works
3. **Options:**
   - Endpoint missing → implement first (30 min)
   - Endpoint broken → fix auth logic
   - Fixture implementation issue → debug Playwright
4. **Timeline:** Must be resolved before Phase 3 starts (blocking)

### Critical Go/No-Go Decision

**APPROVED FOR PHASE 3 if ALL true:**
- ✅ 2.3a: No CRITICAL vulnerabilities in image
- ✅ 2.3b: User management tests pass (at least 4/6, not all timing out)
- ✅ 2.3c: Long test runs (60 min) don't fail with 401

**REVIEW & REWORK if ANY failed:**
- ❌ 2.3a: Vulnerability still present
- ❌ 2.3b: All user tests timing out (async didn't solve)
- ❌ 2.3c: Short test runs failing with 401

---

## Appendix A: Quick Reference Commands

### Phase 2.3a Commands

```bash
# Update dependencies
cd /projects/Charon/backend
go get -u golang.org/x/crypto golang.org/x/net golang.org/x/oauth2 github.com/quic-go/quic-go
go mod tidy && go mod verify

# Build image
docker build -t charon:local .

# Scan for vulnerabilities
trivy image --severity CRITICAL charon:local

# Smoke test
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"TestPass123!"}'
```

### Phase 2.3b Commands

```bash
# Test response time
time curl -s -X POST http://localhost:8080/api/v1/users/invite \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"email":"test@example.com}'

# Run E2E test
npx playwright test tests/user-management.spec.ts --grep="invite" --timeout=5000

# Run all user tests
npx playwright test tests/user-management.spec.ts --reporter=html
```

### Phase 2.3c Commands

```bash
# Verify refresh endpoint
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Authorization: Bearer $TOKEN"

# Run long test
npx playwright test tests/health-check.spec.ts --timeout=3600000

# Check cache file
ls -la .auth-token-cache.json
cat .auth-token-cache.json | jq
```

---

## Appendix B: File Locations Reference

| File/Directory | Purpose | Owner |
|---|---|---|
| `backend/go.mod`, `go.sum` | Dependency management | Phase 2.3a |
| `backend/internal/api/handlers/user_handler.go` (lines 462-469) | InviteUser async refactor | Phase 2.3b |
| `tests/fixtures/auth.ts` | Token refresh fixtures | Phase 2.3c |
| `.auth-token-cache.json` | Cached token (gitignore) | Phase 2.3c |
| `Dockerfile` | Docker image build | Phase 2.3a (validation) |
| `backend/internal/services/mail_service.go` | Email service (reference only) | Phase 2.3b (research) |

---

## Appendix C: Success Metrics Dashboard

**Print this table and track during execution:**

```
Phase 2.3 Remediation - Execution Checklist
==========================================

| Phase | Completed By | Start | End | Status | Blocker | Notes |
|-------|-------------|-------|-----|--------|---------|-------|
| 2.3a  | Dev A       | 09:00 | 10:00 | ✅ | – | Deps updated, scan passed |
| 2.3b  | Dev B       | 09:00 | 10:00 | ✅ | – | Tests pass, <200ms response |
| 2.3c  | Dev C       | 09:00 | 09:45 | ✅ | – | Long tests pass, no 401s |
| **INTEGRATION** | **All** | 10:00 | 10:30 | ✅ | – | Full suite pass, ready |

Total time: 1.5 hours (parallel)
Phase 3 approval: **READY** ✅
```

---

**DOCUMENT COMPLETE**

This Phase 2.3 Remediation Plan is ready for team review and execution. All three critical fixes are defined with specific steps, success criteria, and validation checkpoints. Proceed with parallelized execution targeting 2-3 hour total completion time.

**Next Steps:**
1. Review this plan with team (15 min)
2. Assign developers to phases
3. Start Phase 2.3a, 2.3b, 2.3c in parallel
4. Track progress against checklist
5. Validate completeness before Phase 3 approval
6. Commit with standardized messages per commit section
7. Open PR for code review
8. Merge when all validations pass
9. **Phase 3 E2E Security Testing → APPROVED TO START**
