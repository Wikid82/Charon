# Manual Testing Plan: E2E Test Fixes Validation

**Created:** 2026-02-01
**Status:** Pending
**Priority:** P0 - Verify CI Fixes
**Assignee:** QA Team

---

## Overview

Validate E2E test fixes for feature toggle timeouts and clipboard access failures work correctly in CI environment.

**Fixes Applied:**
1. Feature toggle tests: Sequential wait pattern (4 tests)
2. Clipboard test: Browser-specific verification (1 test)

---

## Test Environment

**Prerequisites:**
- Feature branch: `feature/beta-release`
- Docker E2E container rebuilt with latest code
- Database migrations applied
- Admin user credentials available

**Setup:**
```bash
# Rebuild E2E environment
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e

# Verify container is healthy
docker ps | grep charon-e2e
```

---

## Test Cases

### **TC1: Feature Toggle - Cerberus Security**

**File:** `tests/settings/system-settings.spec.ts`
**Test:** "should toggle Cerberus security feature"
**Line:** ~135-162

**Steps:**
1. Navigate to Settings → System Settings
2. Click Cerberus security toggle
3. Verify PUT request completes (<15s)
4. Verify GET request completes (<10s)
5. Confirm toggle state changed

**Expected Results:**
- ✅ Test completes in <15 seconds total
- ✅ No timeout errors
- ✅ Toggle state persists after refresh

**Command:**
```bash
npx playwright test tests/settings/system-settings.spec.ts --project=chromium --grep "Cerberus"
```

---

### **TC2: Feature Toggle - CrowdSec Enrollment**

**File:** `tests/settings/system-settings.spec.ts`
**Test:** "should toggle CrowdSec console enrollment"
**Line:** ~174-201

**Steps:**
1. Navigate to Settings → System Settings
2. Click CrowdSec console enrollment toggle
3. Verify PUT request completes (<15s)
4. Verify GET request completes (<10s)
5. Confirm toggle state changed

**Expected Results:**
- ✅ Test completes in <15 seconds total
- ✅ No timeout errors
- ✅ Toggle state persists after refresh

**Command:**
```bash
npx playwright test tests/settings/system-settings.spec.ts --project=chromium --grep "CrowdSec"
```

---

### **TC3: Feature Toggle - Uptime Monitoring**

**File:** `tests/settings/system-settings.spec.ts`
**Test:** "should toggle uptime monitoring"
**Line:** ~213-240

**Steps:**
1. Navigate to Settings → System Settings
2. Click uptime monitoring toggle
3. Verify PUT request completes (<15s)
4. Verify GET request completes (<10s)
5. Confirm toggle state changed

**Expected Results:**
- ✅ Test completes in <15 seconds total
- ✅ No timeout errors
- ✅ Toggle state persists after refresh

**Command:**
```bash
npx playwright test tests/settings/system-settings.spec.ts --project=chromium --grep "uptime"
```

---

### **TC4: Feature Toggle - Persistence**

**File:** `tests/settings/system-settings.spec.ts`
**Test:** "should persist feature toggle changes"
**Line:** ~252-298

**Steps:**
1. Navigate to Settings → System Settings
2. Toggle feature ON
3. Verify PUT + GET requests complete
4. Refresh page
5. Verify toggle still ON
6. Toggle feature OFF
7. Verify PUT + GET requests complete
8. Refresh page
9. Verify toggle still OFF

**Expected Results:**
- ✅ Both toggle operations complete in <15s each
- ✅ State persists across page reloads
- ✅ No timeout errors

**Command:**
```bash
npx playwright test tests/settings/system-settings.spec.ts --project=chromium --grep "persist"
```

---

### **TC5: Clipboard Copy - Chromium**

**File:** `tests/settings/user-management.spec.ts`
**Test:** "should copy invite link"
**Line:** ~368-442
**Browser:** Chromium

**Steps:**
1. Navigate to Settings → User Management
2. Create invite for test user
3. Click copy button
4. Verify success toast appears
5. Verify clipboard contains invite link

**Expected Results:**
- ✅ Clipboard contains "accept-invite"
- ✅ Clipboard contains "token="
- ✅ No NotAllowedError

**Command:**
```bash
npx playwright test tests/settings/user-management.spec.ts --project=chromium --grep "copy invite"
```

---

### **TC6: Clipboard Copy - Firefox**

**File:** `tests/settings/user-management.spec.ts`
**Test:** "should copy invite link"
**Browser:** Firefox

**Steps:**
1. Navigate to Settings → User Management
2. Create invite for test user
3. Click copy button
4. Verify success toast appears
5. Test skips clipboard read (not supported)

**Expected Results:**
- ✅ Success toast displayed
- ✅ Invite link input visible with correct value
- ✅ No NotAllowedError
- ✅ Test completes without clipboard verification

**Command:**
```bash
npx playwright test tests/settings/user-management.spec.ts --project=firefox --grep "copy invite"
```

---

### **TC7: Clipboard Copy - WebKit**

**File:** `tests/settings/user-management.spec.ts`
**Test:** "should copy invite link"
**Browser:** WebKit

**Steps:**
1. Navigate to Settings → User Management
2. Create invite for test user
3. Click copy button
4. Verify success toast appears
5. Test skips clipboard read (not supported)

**Expected Results:**
- ✅ Success toast displayed
- ✅ Invite link input visible with correct value
- ✅ No NotAllowedError (previously failing)
- ✅ Test completes without clipboard verification

**Command:**
```bash
npx playwright test tests/settings/user-management.spec.ts --project=webkit --grep "copy invite"
```

---

## Cross-Browser Validation

**Full Suite (All 5 affected tests):**
```bash
npx playwright test \
  tests/settings/system-settings.spec.ts \
  tests/settings/user-management.spec.ts \
  --project=chromium \
  --project=firefox \
  --project=webkit \
  --grep "toggle|copy invite"
```

**Expected Results:**
- ✅ 12 tests pass (4 toggles × 3 browsers = 12, clipboard test already browser-filtered)
- ✅ Total execution time: <2 minutes
- ✅ 0 failures, 0 timeouts, 0 errors

---

## CI Validation

**GitHub Actions Run:**
1. Push changes to `feature/beta-release`
2. Wait for CI workflow to complete
3. Check test results at: https://github.com/Wikid82/Charon/actions

**Success Criteria:**
- ✅ All E2E tests pass on all browsers (Chromium, Firefox, WebKit)
- ✅ No timeout errors in workflow logs
- ✅ No NotAllowedError in WebKit results
- ✅ Build time improved (no 30s timeouts)

---

## Regression Testing

**Verify no side effects:**
```bash
# Run full settings test suite
npx playwright test tests/settings/ --project=chromium

# Check for unintended test failures
npx playwright show-report
```

**Areas to Validate:**
- Other settings tests still pass
- System settings page loads correctly
- User management page functions properly
- No new test flakiness introduced

---

## Bug Scenarios

### **Scenario 1: Feature Toggle Still Timing Out**

**Symptoms:**
- Test fails with timeout error
- Error mentions "waitForResponse" or "30000ms"

**Investigation:**
1. Check backend logs for `/feature-flags` endpoint
2. Verify database writes complete
3. Check network latency in CI environment
4. Confirm PUT timeout (15s) and GET timeout (10s) are present in code

**Resolution:**
- If backend is slow: Increase timeouts further (PUT: 20s, GET: 15s)
- If code error: Verify `clickAndWaitForResponse` imported and used correctly

---

### **Scenario 2: Clipboard Test Fails on Chromium**

**Symptoms:**
- Test fails on Chromium (previously passing browser)
- Error: "clipboard.readText() failed"

**Investigation:**
1. Verify permissions granted: `context.grantPermissions(['clipboard-read', 'clipboard-write'])`
2. Check if page context is correct
3. Verify clipboard API available in test environment

**Resolution:**
- Ensure permission grant happens before clipboard test step
- Verify try-catch block is present in implementation

---

### **Scenario 3: Clipboard Test Still Fails on WebKit/Firefox**

**Symptoms:**
- NotAllowedError still thrown on WebKit/Firefox

**Investigation:**
1. Verify browser detection logic: `testInfo.project?.name`
2. Confirm early return present: `if (browserName !== 'chromium') { return; }`
3. Check if clipboard verification skipped correctly

**Resolution:**
- Verify browser name comparison is exact: `'chromium'` (lowercase)
- Ensure return statement executes before clipboard read

---

## Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Feature Toggle Pass Rate | 100% | CI test results |
| Feature Toggle Execution Time | <15s each | Playwright reporter |
| Clipboard Test Pass Rate (All Browsers) | 100% | CI test results |
| CI Build Time Improvement | -5 minutes | GitHub Actions duration |
| Test Flakiness | 0% | 3 consecutive clean CI runs |

---

## Sign-Off

**Test Plan Created By:** GitHub Copilot (Management Agent)
**Date:** 2026-02-01
**Status:** Ready for Execution

**Validation Required By:**
- [ ] QA Engineer (manual execution)
- [ ] CI Pipeline (automated validation)
- [ ] Code Review (PR approval)

---

## References

- **Remediation Plan:** `docs/plans/current_spec.md`
- **QA Report:** `docs/reports/qa_e2e_test_fixes_report.md`
- **Modified Files:**
  - `tests/settings/system-settings.spec.ts`
  - `tests/settings/user-management.spec.ts`
- **CI Run (Original Failure):** https://github.com/Wikid82/Charon/actions/runs/21558579945/job/62119064951?pr=583
