# Manual Test Plan: E2E Feature Flags Timeout Fix

**Created:** 2026-02-02
**Priority:** P1 - High
**Type:** Manual Testing
**Component:** E2E Tests, Feature Flags API
**Related PR:** #583

---

## Objective

Manually verify the E2E test timeout fix implementation works correctly in a real CI environment after resolving the Playwright infrastructure issue.

## Prerequisites

- [ ] Playwright deduplication issue resolved: `rm -rf node_modules && npm install && npm dedupe`
- [ ] E2E container rebuilt: `.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`
- [ ] Container health check passing: `docker ps` shows `charon-e2e` as healthy

## Test Scenarios

### 1. Feature Flag Toggle Tests (Chromium)

**File:** `tests/settings/system-settings.spec.ts`

**Execute:**
```bash
npx playwright test tests/settings/system-settings.spec.ts --project=chromium --workers=1 --retries=0
```

**Expected Results:**
- [ ] All 7 tests pass (4 refactored + 3 new)
- [ ] Zero timeout errors
- [ ] Test execution time: ≤5s per test
- [ ] Console shows retry attempts (if transient failures occur)

**Tests to Validate:**
1. [ ] `should toggle Cerberus security feature`
2. [ ] `should toggle CrowdSec console enrollment`
3. [ ] `should toggle uptime monitoring`
4. [ ] `should persist feature toggle changes`
5. [ ] `should handle concurrent toggle operations`
6. [ ] `should retry on 500 Internal Server Error`
7. [ ] `should fail gracefully after max retries exceeded`

### 2. Cross-Browser Validation

**Execute:**
```bash
npx playwright test tests/settings/system-settings.spec.ts --project=chromium --project=firefox --project=webkit
```

**Expected Results:**
- [ ] All browsers pass: Chromium, Firefox, WebKit
- [ ] No browser-specific timeout issues
- [ ] Consistent behavior across browsers

### 3. Performance Metrics Extraction

**Execute:**
```bash
docker logs charon-e2e 2>&1 | grep "\[METRICS\]"
```

**Expected Results:**
- [ ] Metrics logged for GET operations: `[METRICS] GET /feature-flags: {latency}ms`
- [ ] Metrics logged for PUT operations: `[METRICS] PUT /feature-flags: {latency}ms`
- [ ] Latency values: <200ms P99 (CI environment)

### 4. Reliability Test (10 Consecutive Runs)

**Execute:**
```bash
for i in {1..10}; do
  echo "Run $i of 10"
  npx playwright test tests/settings/system-settings.spec.ts --project=chromium --workers=1 --retries=0
  if [ $? -ne 0 ]; then
    echo "FAILED on run $i"
    break
  fi
done
```

**Expected Results:**
- [ ] 10/10 runs pass (100% pass rate)
- [ ] Zero timeout errors across all runs
- [ ] Retry attempts: <5% of operations

### 5. UI Verification

**Manual Steps:**
1. [ ] Navigate to `/settings/system` in browser
2. [ ] Toggle Cerberus security feature switch
3. [ ] Verify toggle animation completes
4. [ ] Verify "Saved" notification appears
5. [ ] Refresh page
6. [ ] Verify toggle state persists

**Expected Results:**
- [ ] UI responsive (<1s toggle feedback)
- [ ] State changes reflect immediately
- [ ] No console errors

## Bug Discovery Focus

**Look for potential issues in:**

### Backend Performance
- [ ] Feature flags endpoint latency spikes (>500ms)
- [ ] Database lock timeouts
- [ ] Transaction rollback failures
- [ ] Memory leaks after repeated toggles

### Test Resilience
- [ ] Retry logic not triggering on transient failures
- [ ] Polling timeouts on slow CI runners
- [ ] Race conditions in concurrent toggle test
- [ ] Hard-coded wait remnants causing flakiness

### Edge Cases
- [ ] Concurrent toggles causing data corruption
- [ ] Network failures not handled gracefully
- [ ] Max retries not throwing expected error
- [ ] Initial state mismatch in `beforeEach`

## Success Criteria

- [ ] All 35 checks above pass without issues
- [ ] Zero timeout errors in 10 consecutive runs
- [ ] Performance metrics confirm <200ms P99 latency
- [ ] Cross-browser compatibility verified
- [ ] No new bugs discovered during manual testing

## Failure Handling

**If any test fails:**

1. **Capture Evidence:**
   - Screenshot of failure
   - Full test output (no truncation)
   - `docker logs charon-e2e` output
   - Network/console logs from browser DevTools

2. **Analyze Root Cause:**
   - Is it a code defect or infrastructure issue?
   - Is it reproducible locally?
   - Does it happen in all browsers?

3. **Take Action:**
   - **Code Defect:** Reopen issue, describe failure, assign to developer
   - **Infrastructure:** Document in known issues, create follow-up ticket
   - **Flaky Test:** Investigate retry logic, increase timeouts if justified

## Notes

- Run tests during low CI load times for accurate performance measurement
- Use `--headed` flag for UI verification: `npx playwright test --headed`
- Check Playwright trace if tests fail: `npx playwright show-report`

---

**Assigned To:** QA Team
**Estimated Time:** 2-3 hours
**Due Date:** Within 24 hours of Playwright infrastructure fix
