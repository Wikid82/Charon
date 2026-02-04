# Manual Test Plan: Phase 2 E2E Test Optimizations

**Status**: Pending Manual Testing
**Created**: 2026-02-02
**Priority**: P1 (Performance Validation)
**Estimated Time**: 30-45 minutes

## Overview

Validate Phase 2 E2E test optimizations in real-world scenarios to ensure performance improvements don't introduce regressions or unexpected behavior.

## Objective

Confirm that feature flag polling optimizations, cross-browser label helpers, and conditional verification logic work correctly across different browsers and test execution patterns.

## Prerequisites

- [ ] E2E environment running (`docker-rebuild-e2e` completed)
- [ ] All browsers installed (Chromium, Firefox, WebKit)
- [ ] Clean test environment (no orphaned test data)
- [ ] Baseline metrics captured (pre-Phase 2)

---

## Test Cases

### TC-1: Feature Flag Polling Optimization

**Goal**: Verify feature flag changes propagate correctly without beforeEach polling

**Steps**:
1. Run system settings tests in isolation:
   ```bash
   npx playwright test tests/settings/system-settings.spec.ts --project=chromium
   ```
2. Monitor console output for feature flag API calls
3. Compare API call count to baseline (should be ~90% fewer)

**Expected Results**:
- ✅ All tests pass
- ✅ Feature flag toggles work correctly
- ✅ API calls reduced from ~31 to 3-5 per test file
- ✅ No inter-test dependencies (tests pass in any order)

**Actual Results**:
- [ ] Pass / [ ] Fail
- Notes: _______________________

---

### TC-2: Test Isolation with afterEach Cleanup

**Goal**: Verify test cleanup restores default state without side effects

**Steps**:
1. Run tests with random execution order:
   ```bash
   npx playwright test tests/settings/system-settings.spec.ts \
     --repeat-each=3 \
     --workers=1 \
     --project=chromium
   ```
2. Check for flakiness or state leakage between tests
3. Verify cleanup logs in console output

**Expected Results**:
- ✅ Tests pass consistently across all 3 runs
- ✅ No test failures due to unexpected initial state
- ✅ Cleanup logs show state restoration

**Actual Results**:
- [ ] Pass / [ ] Fail
- Notes: _______________________

---

### TC-3: Cross-Browser Label Locator (Chromium)

**Goal**: Verify label helper works in Chromium

**Steps**:
1. Run DNS provider tests in Chromium:
   ```bash
   npx playwright test tests/dns-provider-types.spec.ts --project=chromium --headed
   ```
2. Watch for "Script Path" field locator behavior
3. Verify no locator timeout errors

**Expected Results**:
- ✅ All DNS provider form tests pass
- ✅ Script path field located successfully
- ✅ No "strict mode violation" errors

**Actual Results**:
- [ ] Pass / [ ] Fail
- Notes: _______________________

---

### TC-4: Cross-Browser Label Locator (Firefox)

**Goal**: Verify label helper works in Firefox (previously failing)

**Steps**:
1. Run DNS provider tests in Firefox:
   ```bash
   npx playwright test tests/dns-provider-types.spec.ts --project=firefox --headed
   ```
2. Watch for "Script Path" field locator behavior
3. Verify fallback chain activates if primary locator fails

**Expected Results**:
- ✅ All DNS provider form tests pass
- ✅ Script path field located successfully (primary or fallback)
- ✅ No browser-specific workarounds needed

**Actual Results**:
- [ ] Pass / [ ] Fail
- Notes: _______________________

---

### TC-5: Cross-Browser Label Locator (WebKit)

**Goal**: Verify label helper works in WebKit (previously failing)

**Steps**:
1. Run DNS provider tests in WebKit:
   ```bash
   npx playwright test tests/dns-provider-types.spec.ts --project=webkit --headed
   ```
2. Watch for "Script Path" field locator behavior
3. Verify fallback chain activates if primary locator fails

**Expected Results**:
- ✅ All DNS provider form tests pass
- ✅ Script path field located successfully (primary or fallback)
- ✅ No browser-specific workarounds needed

**Actual Results**:
- [ ] Pass / [ ] Fail
- Notes: _______________________

---

### TC-6: Conditional Feature Flag Verification

**Goal**: Verify conditional skip optimization reduces polling iterations

**Steps**:
1. Enable debug logging in `wait-helpers.ts` (if available)
2. Run a test that verifies flags but doesn't toggle them:
   ```bash
   npx playwright test tests/security/security-dashboard.spec.ts --project=chromium
   ```
3. Check console logs for "[POLL] Feature flags already in expected state" messages

**Expected Results**:
- ✅ Tests pass
- ✅ Conditional skip activates when flags already match
- ✅ ~50% fewer polling iterations observed

**Actual Results**:
- [ ] Pass / [ ] Fail
- Notes: _______________________

---

### TC-7: Full Suite Performance (All Browsers)

**Goal**: Verify overall test suite performance improved

**Steps**:
1. Run full E2E suite across all browsers:
   ```bash
   npx playwright test --project=chromium --project=firefox --project=webkit
   ```
2. Record total execution time
3. Compare to baseline metrics (pre-Phase 2)

**Expected Results**:
- ✅ All tests pass (except known skips)
- ✅ Execution time reduced by 20-30%
- ✅ No new flaky tests introduced
- ✅ No timeout errors observed

**Actual Results**:
- [ ] Pass / [ ] Fail
- Total time: _______ (Baseline: _______)
- Notes: _______________________

---

### TC-8: Parallel Execution Stress Test

**Goal**: Verify optimizations handle parallel execution gracefully

**Steps**:
1. Run tests with maximum workers:
   ```bash
   npx playwright test tests/settings/system-settings.spec.ts --workers=4
   ```
2. Monitor for race conditions or resource contention
3. Check for worker-isolated cache behavior

**Expected Results**:
- ✅ Tests pass consistently
- ✅ No race conditions observed
- ✅ Worker isolation functions correctly
- ✅ Request coalescing reduces duplicate API calls

**Actual Results**:
- [ ] Pass / [ ] Fail
- Notes: _______________________

---

## Regression Checks

### RC-1: Existing Test Behavior

**Goal**: Verify Phase 2 changes don't break existing tests

**Steps**:
1. Run tests that don't use new helpers:
   ```bash
   npx playwright test tests/proxy-hosts/ --project=chromium
   ```
2. Verify backward compatibility

**Expected Results**:
- ✅ All tests pass
- ✅ No unexpected failures in unrelated tests

**Actual Results**:
- [ ] Pass / [ ] Fail
- Notes: _______________________

---

### RC-2: CI/CD Pipeline Simulation

**Goal**: Verify changes work in CI environment

**Steps**:
1. Run tests with CI environment variables:
   ```bash
   CI=true npx playwright test --workers=1 --retries=2
   ```
2. Verify CI-specific behavior (retries, reporting)

**Expected Results**:
- ✅ Tests pass in CI mode
- ✅ Retry logic works correctly
- ✅ Reports generated successfully

**Actual Results**:
- [ ] Pass / [ ] Fail
- Notes: _______________________

---

## Known Issues

### Issue 1: E2E Test Interruptions (Non-Blocking)
- **Location**: `tests/core/access-lists-crud.spec.ts:766, 794`
- **Impact**: 2 tests interrupted during login
- **Action**: Tracked separately, not caused by Phase 2 changes

### Issue 2: Frontend Security Page Test Failures (Non-Blocking)
- **Location**: `src/pages/__tests__/Security.loading.test.tsx`
- **Impact**: 15 test failures, WebSocket mock issues
- **Action**: Testing infrastructure issue, not E2E changes

---

## Success Criteria

**PASS Conditions**:
- [ ] All manual test cases pass (TC-1 through TC-8)
- [ ] No new regressions introduced (RC-1, RC-2)
- [ ] Performance improvements validated (20-30% faster)
- [ ] Cross-browser compatibility confirmed

**FAIL Conditions**:
- [ ] Any CRITICAL test failures in Phase 2 changes
- [ ] New flaky tests introduced by optimizations
- [ ] Performance degradation observed
- [ ] Cross-browser compatibility broken

---

## Sign-Off

| Role | Name | Date | Status |
|------|------|------|--------|
| QA Engineer | __________ | _______ | [ ] Pass / [ ] Fail |
| Tech Lead | __________ | _______ | [ ] Approved / [ ] Rejected |

**Notes**: _____________________________________________

---

## Next Actions

**If PASS**:
- [ ] Mark issue as complete
- [ ] Merge PR #609
- [ ] Monitor production metrics

**If FAIL**:
- [ ] Document failures in detail
- [ ] Create remediation tickets
- [ ] Re-run tests after fixes

**Follow-Up Items** (Regardless):
- [ ] Fix login flow timeouts (Issue tracked separately)
- [ ] Restore frontend coverage measurement
- [ ] Update baseline metrics documentation
