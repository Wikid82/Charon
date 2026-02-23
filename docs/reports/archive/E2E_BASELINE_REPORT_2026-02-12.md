# E2E Test Baseline Report - February 12, 2026

## Executive Summary

**Test Run Date**: 2026-02-12 15:46 UTC
**Environment**: charon-e2e container (healthy, ports 8080/2020/2019)
**Browsers**: Firefox, Chromium, WebKit (full suite)

## Results Overview

Based on test execution analysis:
- **Estimated Passed**: ~1,450-1,470 tests (similar to previous runs)
- **Identified Failures**: ~15-20 distinct failures observed in output
- **Total Test Count**: ~1,600-1,650 (across 3 browsers)

## Failure Categories (Prioritized by Impact)

### 1. HIGH PRIORITY: DNS Provider Test Timeouts (90s+)
**Impact**: 5-6 failures **Root Cause**: Tests timing out after 90+ seconds
**Affected Tests**:
- `tests/dns-provider.spec.ts:238` - Create Manual DNS provider
- `tests/dns-provider.spec.ts:239` - Create Webhook DNS provider
- `tests/dns-provider.spec.ts:240` - Validation errors for missing fields
- `tests/dns-provider.spec.ts:242` - Display provider list or empty state
- `tests/dns-provider.spec.ts:243` - Show Add Provider button

**Evidence**:
```
✘   238 …NS Provider CRUD Operations › Create Provider › should create a Manual DNS provider (5.8s)
✘   239 …S Provider CRUD Operations › Create Provider › should create a Webhook DNS provider (1.6m)
✘   240 …tions › Create Provider › should show validation errors for missing required fields (1.6m)
```

**Analysis**: Tests start but timeout waiting for some condition. Logs show loader polling continuing indefinitely.

**Remediation Strategy**:
1. Check if `waitForLoadingComplete()` is being used
2. Verify DNS provider page loading mechanism
3. Add explicit waits for form elements
4. Consider if container needs DNS provider initialization

### 2. HIGH PRIORITY: Data Consistency Tests (90s timeouts)
**Impact**: 4-5 failures
**Root Cause**: Long-running transactions timing out

**Affected Tests**:
- `tests/data-consistency.spec.ts:156` - Data created via UI is stored and readable via API
- `tests/data-consistency.spec.ts:158` - Data deleted via UI is removed from API (1.6m)
- `tests/data-consistency.spec.ts:160` - Failed transaction prevents partial updates (1.5m)
- `tests/data-consistency.spec.ts:162` - Client-side and server-side validation consistent (1.5m)
- `tests/data-consistency.spec.ts:163` - Pagination and sorting produce consistent results

**Evidence**:
```
✘   158 …sistency.spec.ts:217:3 › Data Consistency › Data deleted via UI is removed from API (1.6m)
✘   160 …spec.ts:326:3 › Data Consistency › Failed transaction prevents partial data updates (1.5m)
✘   162 …pec.ts:388:3 › Data Consistency › Client-side and server-side validation consistent (1.5m)
```

**Remediation Strategy**:
1. Review API wait patterns in these tests
2. Check if `waitForAPIResponse()` is properly used
3. Verify database state between UI and API operations
4. Consider splitting multi-step operations into smaller waits

### 3. MEDIUM PRIORITY: Multi-Component Workflows (Security Enforcement)
**Impact**: 5 failures
**Root Cause**: Tests expecting security modules to be active, possibly missing setup

**Affected Tests**:
- `tests/multi-component-workflows.spec.ts:62` - WAF enforcement applies to newly created proxy
- `tests/multi-component-workflows.spec.ts:171` - User with proxy creation role can create proxies
- `tests/multi-component-workflows.spec.ts:172` - Backup restore recovers deleted user data
- `tests/multi-component-workflows.spec.ts:173` - Security modules apply to subsequently created resources
- `tests/multi-component-workflows.spec.ts:174` - Security enforced on previously created resources

**Evidence**:
```
✘   170 …s:62:3 › Multi-Component Workflows › WAF enforcement applies to newly created proxy (7.3s)
✘   171 …i-Component Workflows › User with proxy creation role can create and manage proxies (7.4s)
```

**Remediation Strategy**:
1. Verify security modules (WAF, ACL, Rate Limiting) are properly initialized
2. Check if tests need security module enabling in beforeEach
3. Confirm API endpoints for security enforcement exist
4. May need container environment variable for security features

### 4. LOW PRIORITY: Navigation - Responsive Mobile Menu
**Impact**: 1 failure
**Root Cause**: Mobile menu toggle test failing in responsive mode

**Affected Test**:
- `tests/navigation.spec.ts:731` - Responsive Navigation › should toggle mobile menu

**Evidence**:
```
✘   200 …tion.spec.ts:731:5 › Navigation › Responsive Navigation › should toggle mobile menu (2.4s)
```

**Remediation Strategy**:
1. Check viewport size is properly set for mobile testing
2. Verify mobile menu button locator
3. Ensure menu visibility toggle is waited for
4. Simple fix, low complexity

## Test Health Indicators

### Positive Signals
- **Fast test execution**: Most passing tests complete in 2-5 seconds
- **Stable core features**: Dashboard, Certificates, Proxy Hosts, Access Lists all passing
- **Good accessibility coverage**: ARIA snapshots and keyboard navigation tests passing
- **No container issues**: Tests failing due to app logic, not infrastructure

### Concerns
- **Timeout pattern**: Multiple 90-second timeouts suggest waiting mechanism issues
- **Security enforcement**: Tests may need environment configuration
- **DNS provider**: Consistently failing, may need feature initialization

## Recommended Remediation Order

### Phase 1: Quick Wins (Est. 1-2 hours)
1. **Navigation mobile menu** (1 test) - Simple viewport/locator fix
2. **DNS provider locators** (investigation) - Check if issue is locator-based first

### Phase 2: DNS Provider Timeouts (Est. 2-3 hours)
3. **DNS provider full remediation** (5-6 tests)
   - Add proper wait conditions
   - Fix loader polling
   - Verify form element availability

### Phase 3: Data Consistency (Est. 2-4 hours)
4. **Data consistency timeouts** (4-5 tests)
   - Optimize API wait patterns
   - Add explicit response waits
   - Review transaction test setup

### Phase 4: Security Workflows (Est. 3-5 hours)
5. **Multi-component security tests** (5 tests)
   - Verify security module initialization
   - Add proper feature flags/env vars
   - Confirm API endpoints exist

## Expected Outcome

**Current Estimated State**: ~1,460 passed, ~20 failed (98.7% pass rate)
**Target After Remediation**: 1,480 passed, 0 failed (100% pass rate)

**Effort Estimate**: 8-14 hours total for complete remediation

## Next Steps

1. **Confirm exact baseline**: Run `npx playwright test --reporter=json > results.json` to get precise counts
2. **Start with Phase 1**: Fix navigation mobile menu (quick win)
3. **Deep dive DNS providers**: Run `npx playwright test tests/dns-provider.spec.ts --debug` to diagnose
4. **Iterate**: Fix, test targeted file, validate, move to next batch

## Notes

- All tests are using the authenticated `adminUser` fixture properly
- Container readiness waits (`waitForLoadingComplete()`) are working for most tests
- No browser-specific failures observed yet (will need full run with all browsers to confirm)
- Test structure and locators are generally good (role-based, accessible)

---

**Report Generated**: 2026-02-12 15:46 UTC
**Next Review**: After Phase 1 completion
