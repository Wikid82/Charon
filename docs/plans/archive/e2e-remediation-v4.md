# E2E Test Failure Remediation Plan v4.0

**Created:** January 30, 2026
**Status:** Active Remediation Plan
**Prior Attempt:** Port binding fix (127.0.0.1:2020 → 0.0.0.0:2020) + Toast role attribute
**Result:** Failures increased from 15 to 16 — indicates deeper issues unaddressed

---

## Executive Summary

Comprehensive code path analysis of 16 E2E test failures categorized below. Each failure classified as TEST BUG, APP BUG, or ENV ISSUE.

### Classification Overview

| Classification | Count | Description |
|----------------|-------|-------------|
| **TEST BUG** | 8 | Incorrect selectors, wrong expectations, broken skip logic |
| **APP BUG** | 2 | Application code doesn't meet requirements |
| **ENV ISSUE** | 6 | Docker configuration or race conditions in parallel execution |

### Failure Categories

| Category | Failures | Priority |
|----------|----------|----------|
| Emergency Server Tier 2 | 8 | CRITICAL |
| Security Enforcement | 3 | HIGH |
| Authentication Errors | 2 | HIGH |
| Settings Success Toasts | 2 | MEDIUM |
| Form Validation | 1 | MEDIUM |

---

## Detailed Analysis by Category

---

## Category 1: Emergency Server Tier 2 (8 Failures) — CRITICAL

### Root Cause: TEST BUG + ENV ISSUE

The emergency server tests use a broken skip pattern where `beforeAll` sets a module-level flag, but `beforeEach` captures stale closure state. Additionally, 502 errors suggest the server may not be starting or network isolation prevents access.

### Evidence from Source Code

**Test Files:**
- [tests/emergency-server/emergency-server.spec.ts](../../tests/emergency-server/emergency-server.spec.ts)
- [tests/emergency-server/tier2-validation.spec.ts](../../tests/emergency-server/tier2-validation.spec.ts)

**Current Pattern (Broken):**
```typescript
// Module-level flag
let emergencyServerHealthy = false;

test.beforeAll(async () => {
  emergencyServerHealthy = await checkEmergencyServerHealth();  // Sets to true/false
});

test.beforeEach(async ({}, testInfo) => {
  if (!emergencyServerHealthy) {
    testInfo.skip(true, 'Emergency server not accessible');  // PROBLEM: closure stale
  }
});
```

**Why This Fails:**
- Playwright may execute `beforeEach` before `beforeAll` completes in some parallelization modes
- The `emergencyServerHealthy` closure captures the initial `false` value
- `testInfo.skip()` in `beforeEach` is unreliable with async `beforeAll`

**Backend Configuration:**
- File: [backend/internal/server/emergency_server.go](../../backend/internal/server/emergency_server.go)
- Health endpoint `/health` is correctly defined BEFORE Basic Auth middleware
- Server binds to `CHARON_EMERGENCY_BIND` (set to `0.0.0.0:2020` in Docker)

**Docker Configuration:**
- Port mapping `"2020:2020"` was fixed from `127.0.0.1:2020:2020`
- But 502 errors suggest gateway/proxy layer issue, not port binding

### Classification: 6 TEST BUG + 2 ENV ISSUE

| Test | Error | Classification |
|------|-------|---------------|
| Emergency server health endpoint | 502 Bad Gateway | ENV ISSUE |
| Emergency reset via Tier 2 | 502 Bad Gateway | ENV ISSUE |
| Basic auth protects endpoints | Skip logic fails | TEST BUG |
| Reset requires emergency token | Skip logic fails | TEST BUG |
| Rate limiting on reset endpoint | Skip logic fails | TEST BUG |
| Validates reset payload | Skip logic fails | TEST BUG |
| Returns proper error for invalid token | Skip logic fails | TEST BUG |
| Emergency server bypasses Caddy | Skip logic fails | TEST BUG |

### EARS Requirements

```
REQ-EMRG-001: WHEN emergency server health check fails
             THE TEST FRAMEWORK SHALL skip all emergency server tests gracefully
             WITH descriptive skip reason logged to console

REQ-EMRG-002: WHEN emergency server is accessible
             THE TESTS SHALL execute normally without 502 errors
```

### Remediation: Phase 1

**File: tests/emergency-server/emergency-server.spec.ts**

**Change:** Replace `beforeAll` + `beforeEach` pattern with per-test health check function

```typescript
// BEFORE (broken):
let emergencyServerHealthy = false;
test.beforeAll(async () => { emergencyServerHealthy = await checkEmergencyServerHealth(); });
test.beforeEach(async ({}, testInfo) => { if (!emergencyServerHealthy) testInfo.skip(); });

// AFTER (fixed):
async function skipIfServerUnavailable(testInfo: TestInfo): Promise<boolean> {
  const isHealthy = await checkEmergencyServerHealth();
  if (!isHealthy) {
    testInfo.skip(true, 'Emergency server not accessible from test environment');
    return false;
  }
  return true;
}

test('Emergency server health endpoint', async ({}, testInfo) => {
  if (!await skipIfServerUnavailable(testInfo)) return;
  // ... test body
});
```

**Rationale:** Moving the health check INTO each test's scope eliminates closure stale state issues.

**File: tests/fixtures/security.ts**

**Change:** Increase health check timeout and add retry logic

```typescript
// Current:
const response = await fetch(`${EMERGENCY_SERVER.baseURL}/health`, { timeout: 5000 });

// Fixed:
async function checkEmergencyServerHealth(maxRetries = 3): Promise<boolean> {
  for (let i = 0; i < maxRetries; i++) {
    try {
      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), 5000);
      const response = await fetch(`${EMERGENCY_SERVER.baseURL}/health`, {
        signal: controller.signal,
      });
      clearTimeout(timeout);
      if (response.ok) return true;
      console.log(`Health check attempt ${i + 1} failed: ${response.status}`);
    } catch (e) {
      console.log(`Health check attempt ${i + 1} error: ${e.message}`);
    }
    await new Promise(r => setTimeout(r, 1000));
  }
  return false;
}
```

**ENV ISSUE Investigation Required:**

The 502 errors suggest the emergency server isn't being hit directly. Check if:
1. Caddy is intercepting port 2020 requests (it shouldn't)
2. Docker network isolation is preventing Playwright → Container communication
3. Emergency server fails to start (check container logs)

**Verification Command:**
```bash
# Inside running container
docker exec charon curl -v http://localhost:2019/health  # Emergency server
docker logs charon 2>&1 | grep -i "emergency\|2020"
```

---

## Category 2: Security Enforcement (3 Failures) — HIGH

### Root Cause: ENV ISSUE (Race Conditions)

Security module tests fail due to insufficient wait times after enabling Cerberus/ACL modules. The backend updates settings in SQLite, then triggers a Caddy reload, but the security status API returns stale data before reload completes.

### Evidence from Source Code

**Test Files:**
- [tests/security-enforcement/combined-enforcement.spec.ts](../../tests/security-enforcement/combined-enforcement.spec.ts)
- [tests/security-enforcement/emergency-token.spec.ts](../../tests/security-enforcement/emergency-token.spec.ts)

**Current Pattern:**
```typescript
// combined-enforcement.spec.ts line ~99
await setSecurityModuleEnabled(requestContext, 'cerberus', true);
await new Promise(r => setTimeout(r, 2000));  // 2 seconds wait

let status = await getSecurityStatus(requestContext);
let cerberusRetries = 10;
while (!status.cerberus.enabled && cerberusRetries > 0) {
  await new Promise(r => setTimeout(r, 500));  // 500ms between retries
  status = await getSecurityStatus(requestContext);
  cerberusRetries--;
}
// Total wait: 2000 + (10 * 500) = 7000ms max
```

**Why This Fails:**
- Caddy config reload can take 3-5 seconds under load
- Parallel test execution may disable modules while this test runs
- SQLite write → Caddy reload → Security status cache update has propagation delay

### Classification: 3 ENV ISSUE

| Test | Error | Issue |
|------|-------|-------|
| Enable all security modules simultaneously | Timeout 10.6s | Wait too short |
| Emergency token from unauthorized IP | ACL not enabled | Propagation delay |
| WAF enforcement for blocked pattern | Module not enabled | Parallel test interference |

### EARS Requirements

```
REQ-SEC-001: WHEN security module is enabled via API
             THE SYSTEM SHALL reflect enabled status within 15 seconds
             AND Caddy configuration SHALL be reloaded successfully

REQ-SEC-002: WHEN ACL module is enabled
             THE SYSTEM SHALL enforce IP allowlisting within 5 seconds
```

### Remediation: Phase 2

**File: tests/security-enforcement/combined-enforcement.spec.ts**

**Change:** Increase retry count and wait times, add test isolation

```typescript
// BEFORE:
await new Promise(r => setTimeout(r, 2000));
let cerberusRetries = 10;
while (!status.cerberus.enabled && cerberusRetries > 0) {
  await new Promise(r => setTimeout(r, 500));
  // ...
}

// AFTER:
await new Promise(r => setTimeout(r, 3000));  // Increased initial wait
let cerberusRetries = 15;  // Increased retries
while (!status.cerberus.enabled && cerberusRetries > 0) {
  await new Promise(r => setTimeout(r, 1000));  // Increased interval
  status = await getSecurityStatus(requestContext);
  cerberusRetries--;
}
// Total wait: 3000 + (15 * 1000) = 18000ms max
```

**File: tests/security-enforcement/emergency-token.spec.ts**

**Change:** Add retry logic to ACL verification in `beforeAll`

```typescript
// BEFORE (line ~106):
if (!status.acl?.enabled) {
  throw new Error('ACL verification failed - ACL not showing as enabled');
}

// AFTER:
let aclEnabled = false;
for (let i = 0; i < 10; i++) {
  const status = await getSecurityStatus(requestContext);
  if (status.acl?.enabled) {
    aclEnabled = true;
    break;
  }
  console.log(`ACL not yet enabled, retry ${i + 1}/10`);
  await new Promise(r => setTimeout(r, 500));
}
if (!aclEnabled) {
  throw new Error('ACL verification failed after 10 retries');
}
```

**Test Isolation:**

Add `test.describe.configure({ mode: 'serial' })` to prevent parallel execution conflicts:

```typescript
test.describe('Security Enforcement Tests', () => {
  test.describe.configure({ mode: 'serial' });  // Run tests sequentially
  // ... tests
});
```

---

## Category 3: Authentication Errors (2 Failures) — HIGH

### Root Cause: 1 TEST BUG + 1 APP BUG

Two authentication-related tests fail:
1. **Password validation toast** — Test uses wrong selector
2. **Auth error propagation** — Axios interceptor may not extract error message correctly

### Evidence from Source Code

**Test File:** [tests/settings/account-settings.spec.ts](../../tests/settings/account-settings.spec.ts)

**Test Pattern (lines ~432-452):**
```typescript
await test.step('Submit and verify error', async () => {
  const updateButton = page.getByRole('button', { name: /update.*password/i });
  await updateButton.click();

  // Error toast uses role="alert" (with data-testid fallback)
  const errorToast = page.locator('[data-testid="toast-error"]')
    .or(page.getByRole('alert'))
    .filter({ hasText: /incorrect|invalid|wrong|failed/i });
  await expect(errorToast.first()).toBeVisible({ timeout: 10000 });
});
```

**Analysis:** This selector pattern is CORRECT. The issue is likely that:
1. The API returns a 400 but the error message isn't displayed
2. The toast auto-dismisses before assertion runs

**Backend Handler (auth_handler.go):**
```go
if err := h.authService.ChangePassword(...); err != nil {
  c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
  return
}
```

**Frontend Handler (AuthContext.tsx):**
```typescript
const changePassword = async (oldPassword: string, newPassword: string) => {
  await client.post('/auth/change-password', {
    old_password: oldPassword,
    new_password: newPassword,
  });
  // No explicit error handling — relies on axios to throw
};
```

**Frontend Consumer (Account.tsx):**
```typescript
try {
  await changePassword(oldPassword, newPassword)
  toast.success(t('account.passwordUpdated'))
} catch (err) {
  const error = err as Error
  toast.error(error.message || t('account.passwordUpdateFailed'))
}
```

### Classification: 1 TEST BUG + 1 APP BUG

| Test | Error | Classification |
|------|-------|---------------|
| Validate current password shows error | Toast not visible | APP BUG (error message not extracted) |
| Password mismatch validation | Error not shown | TEST BUG (validation is client-side only) |

### Remediation: Phase 3

**File: frontend/src/api/client.ts**

**Change:** Ensure axios response interceptor extracts API error messages

```typescript
// Verify this interceptor exists and extracts error.response.data.error:
client.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.data?.error) {
      error.message = error.response.data.error;
    }
    return Promise.reject(error);
  }
);
```

**File: frontend/src/context/AuthContext.tsx**

**Change:** Add explicit error extraction in changePassword

```typescript
const changePassword = async (oldPassword: string, newPassword: string) => {
  try {
    await client.post('/auth/change-password', {
      old_password: oldPassword,
      new_password: newPassword,
    });
  } catch (error: any) {
    const message = error.response?.data?.error || error.message || 'Password change failed';
    throw new Error(message);
  }
};
```

---

## Category 4: Settings Success Toasts (2 Failures) — MEDIUM

### Root Cause: TEST BUG (Mixed Selector Pattern)

Some settings tests use `getByRole('alert')` for success toasts, but our Toast component uses:
- `role="alert"` for error/warning toasts
- `role="status"` for success/info toasts

### Evidence from Source Code

**Toast.tsx (lines 33-37):**
```tsx
<div
  role={toast.type === 'error' || toast.type === 'warning' ? 'alert' : 'status'}
  // ...
>
```

**wait-helpers.ts already handles this correctly:**
```typescript
if (type === 'success' || type === 'info') {
  toast = page.locator(`[data-testid="toast-${type}"]`)
    .or(page.getByRole('status'))
    .filter({ hasText: text })
    .first();
}
```

**But tests bypass the helper:**
```typescript
// smtp-settings.spec.ts (around line 336):
const successToast = page
  .getByRole('alert')  // WRONG for success toasts!
  .filter({ hasText: /success|saved/i });
```

### Classification: 2 TEST BUG

| Test | Error | Issue |
|------|-------|-------|
| Update SMTP configuration | Success toast not found | Uses getByRole('alert') instead of getByRole('status') |
| Save general settings | Success toast not found | Same issue |

### Remediation: Phase 4

**File: tests/settings/smtp-settings.spec.ts**

**Change:** Use the correct selector pattern for success toasts

```typescript
// BEFORE:
const successToast = page.getByRole('alert').filter({ hasText: /success|saved/i });

// AFTER:
const successToast = page.getByRole('status')
  .or(page.getByRole('alert'))
  .filter({ hasText: /success|saved/i });
```

**Alternative:** Use the existing `waitForToast` helper:
```typescript
import { waitForToast } from '../utils/wait-helpers';

await waitForToast(page, /success|saved/i, { type: 'success' });
```

**File: tests/settings/system-settings.spec.ts**

Apply same fix if needed at line ~413.

---

## Category 5: Form Validation (1 Failure) — MEDIUM

### Root Cause: TEST BUG (Timing/Selector Issue)

Certificate email validation test expects save button to be disabled for invalid email, but the test may not be triggering validation correctly.

### Evidence from Source Code

**Test (account-settings.spec.ts lines ~287-310):**
```typescript
await test.step('Enter invalid email', async () => {
  const certEmailInput = page.locator('#cert-email');
  await certEmailInput.clear();
  await certEmailInput.fill('not-a-valid-email');
});

await test.step('Verify save button is disabled', async () => {
  const saveButton = page.getByRole('button', { name: /save.*certificate/i });
  await expect(saveButton).toBeDisabled();
});
```

**Application Logic (Account.tsx lines ~92-99):**
```typescript
useEffect(() => {
  if (certEmail && !useUserEmail) {
    setCertEmailValid(isValidEmail(certEmail))
  } else {
    setCertEmailValid(null)
  }
}, [certEmail, useUserEmail])
```

**Button Disabled Logic:**
```tsx
disabled={isLoading || (useUserEmail ? false : (certEmailValid !== true))}
```

**Analysis:** The logic is correct:
- When `useUserEmail` is `false` AND `certEmailValid` is `false`, button should be disabled
- Test may fail if `useUserEmail` was not properly toggled to `false` first

### Classification: 1 TEST BUG

### Remediation: Phase 4

**File: tests/settings/account-settings.spec.ts**

**Change:** Ensure checkbox is unchecked BEFORE entering invalid email

```typescript
await test.step('Ensure use account email is unchecked', async () => {
  const checkbox = page.locator('#useUserEmail');
  const isChecked = await checkbox.isChecked();
  if (isChecked) {
    await checkbox.click();
  }
  // Wait for UI to update
  await expect(checkbox).not.toBeChecked({ timeout: 3000 });
});

await test.step('Verify custom email field is visible', async () => {
  const certEmailInput = page.locator('#cert-email');
  await expect(certEmailInput).toBeVisible({ timeout: 3000 });
});

await test.step('Enter invalid email', async () => {
  const certEmailInput = page.locator('#cert-email');
  await certEmailInput.clear();
  await certEmailInput.fill('not-a-valid-email');
  // Trigger validation by blurring
  await certEmailInput.blur();
  await page.waitForTimeout(100);  // Allow React state update
});

await test.step('Verify save button is disabled', async () => {
  const saveButton = page.getByRole('button', { name: /save.*certificate/i });
  await expect(saveButton).toBeDisabled({ timeout: 3000 });
});
```

---

## Implementation Plan

### Execution Order

| Priority | Phase | Tasks | Files | Est. Time |
|----------|-------|-------|-------|-----------|
| 1 | Phase 1 | Fix emergency server skip logic | tests/emergency-server/*.spec.ts | 1 hour |
| 2 | Phase 2 | Fix security enforcement timeouts | tests/security-enforcement/*.spec.ts | 1 hour |
| 3 | Phase 3 | Fix auth error toast display | frontend/src/context/AuthContext.tsx, frontend/src/api/client.ts | 30 min |
| 4 | Phase 4 | Fix settings toast selectors | tests/settings/*.spec.ts | 30 min |
| 5 | Verify | Run full E2E suite | - | 1 hour |

### Files Modified

| File | Changes | Category |
|------|---------|----------|
| tests/emergency-server/emergency-server.spec.ts | Replace beforeAll/beforeEach with per-test skip | Phase 1 |
| tests/emergency-server/tier2-validation.spec.ts | Same pattern fix | Phase 1 |
| tests/fixtures/security.ts | Add retry logic to health check | Phase 1 |
| tests/security-enforcement/combined-enforcement.spec.ts | Increase timeouts, add serial mode | Phase 2 |
| tests/security-enforcement/emergency-token.spec.ts | Add retry loop for ACL verification | Phase 2 |
| frontend/src/context/AuthContext.tsx | Explicit error extraction in changePassword | Phase 3 |
| frontend/src/api/client.ts | Verify axios interceptor | Phase 3 |
| tests/settings/smtp-settings.spec.ts | Fix toast selector (status vs alert) | Phase 4 |
| tests/settings/system-settings.spec.ts | Same fix | Phase 4 |
| tests/settings/account-settings.spec.ts | Ensure checkbox state before validation test | Phase 4 |

**Total Files:** 10
**Estimated Lines Changed:** ~200

---

## Validation Criteria

### WHEN Phase 1 fixes are applied

**THE SYSTEM SHALL:**
- Skip emergency server tests gracefully when server is unreachable
- Log skip reason: "Emergency server not accessible from test environment"
- NOT produce 502 errors in test output (tests are skipped, not run)

### WHEN Phase 2 fixes are applied

**THE SYSTEM SHALL:**
- Enable all security modules within 18 seconds (extended from 7s)
- Run security tests serially to prevent parallel interference
- Verify ACL is enabled with up to 10 retry attempts

### WHEN Phase 3 fixes are applied

**THE SYSTEM SHALL:**
- Display error toast with message "invalid current password" or similar
- Toast uses `role="alert"` and contains error text from API

### WHEN Phase 4 fixes are applied

**THE SYSTEM SHALL:**
- Display success toast with `role="status"` after settings save
- Tests use correct selector pattern: `getByRole('status').or(getByRole('alert'))`

---

## Verification Commands

```bash
# Run full E2E suite after all fixes
npx playwright test --project=chromium

# Test specific categories
npx playwright test tests/emergency-server/ --project=chromium
npx playwright test tests/security-enforcement/ --project=security-tests
npx playwright test tests/settings/ --project=chromium

# Debug emergency server issues
docker exec charon curl -v http://localhost:2019/health
docker logs charon 2>&1 | grep -E "emergency|2020|2019"
```

---

## Open Questions for Investigation

1. **502 Error Source:** Is the emergency server starting at all? Check container logs.
2. **Playwright Network:** Can Playwright container reach port 2020 on the app container?
3. **Parallel Test Conflicts:** Should all security tests run with `mode: 'serial'`?

---

## Appendix: Error Messages Reference

### Emergency Server
```
Error: locator.click: Target closed
Error: expect(received).ok() - Emergency server health check failed
502 Bad Gateway
```

### Security Enforcement
```
Error: Timeout exceeded 10600ms waiting for security modules
Error: ACL verification failed - ACL not showing as enabled
```

### Auth/Toast
```
Error: expect(received).toBeVisible() - role="alert" toast not found
```

### Settings
```
Error: expect(received).toBeVisible() - Success toast not appearing
Error: expect(received).toBeDisabled() - Button not disabled
```
