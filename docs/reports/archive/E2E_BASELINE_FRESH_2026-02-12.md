# E2E Test Baseline - Fresh Run After DNS Provider Fixes
**Date:** February 12, 2026, 20:37:05
**Duration:** 21 minutes (20:16 - 20:37)
**Command:** `npx playwright test --project=firefox --project=chromium --project=webkit`

## Executive Summary

**Total Failures: 28 (All Chromium)**
- **Firefox: 0 failures** ✅
- **Webkit: 0 failures** ✅
- **Chromium: 28 failures** ❌

**Estimated Total Tests:** ~540 tests across 3 browsers = ~1620 total executions
- **Estimated Passed:** ~1592 (98.3% pass rate)
- **Estimated Failed:** ~28 (1.7% failure rate)

## Improvement from Previous Baseline

**Previous (Feb 12, E2E_BASELINE_REPORT_2026-02-12.md):**
- ~1461 passed
- ~163 failed
- 90% pass rate

**Current:**
- ~1592 passed (+131 more passing tests)
- ~28 failed (-135 fewer failures)
- 98.3% pass rate (+8.3% improvement)

**Result: 83% reduction in failures! 🎉**

## Failure Breakdown by Category

### 1. **Settings - User Lifecycle (7 failures - HIGHEST IMPACT)**
- `settings-user-lifecycle-Ad-11b34` - Deleted user cannot login
- `settings-user-lifecycle-Ad-26d31` - Session persistence after logout and re-login
- `settings-user-lifecycle-Ad-3b06b` - Users see only their own data
- `settings-user-lifecycle-Ad-47c9f` - User cannot promote self to admin
- `settings-user-lifecycle-Ad-d533c` - Permissions apply immediately on user refresh
- `settings-user-lifecycle-Ad-da1df` - Permissions propagate from creation to resource access
- `settings-user-lifecycle-Ad-f3472` - Audit log records user lifecycle events

### 2. **Core - Multi-Component Workflows (5 failures)**
- `core-multi-component-workf-32590` - WAF enforcement applies to newly created proxy
- `core-multi-component-workf-bab1e` - User with proxy creation role can create and manage proxies
- `core-multi-component-workf-ed6bc` - Backup restore recovers deleted user data
- `core-multi-component-workf-01dc3` - Security modules apply to subsequently created resources
- `core-multi-component-workf-15e40` - Security enforced even on previously created resources

### 3. **Core - Data Consistency (5 failures)**
- `core-data-consistency-Data-70ee2` - Pagination and sorting produce consistent results
- `core-data-consistency-Data-b731b` - Client-side and server-side validation consistent
- `core-data-consistency-Data-31d18` - Data stored via API is readable via UI
- `core-data-consistency-Data-d42f5` - Data deleted via UI is removed from API
- `core-data-consistency-Data-0982b` - Real-time events reflect partial data updates

### 4. **Settings - User Management (2 failures)**
- `settings-user-management-U-203fa` - User should copy invite link
- `settings-user-management-U-ff1cf` - User should remove permitted hosts

### 5. **Modal - Dropdown Triage (2 failures)**
- `modal-dropdown-triage-Moda-73472` - InviteUserModal Role Dropdown
- `modal-dropdown-triage-Moda-dac27` - ProxyHostForm ACL Dropdown

### 6. **Core - Certificates SSL (2 failures)**
- `core-certificates-SSL-Cert-15be2` - Display certificate domain in table
- `core-certificates-SSL-Cert-af82e` - Display certificate issuer

### 7. **Core - Authentication (2 failures)**
- `core-authentication-Authen-c9954` - Redirect with error message and redirect to login page
- `core-authentication-Authen-e89dd` - Force login when session expires

### 8. **Core - Admin Onboarding (2 failures)**
- `core-admin-onboarding-Admi-7d633` - Setup Logout clears session
- `core-admin-onboarding-Admi-e9ee4` - First login after logout successful

### 9. **Core - Navigation (1 failure)**
- `core-navigation-Navigation-5c4df` - Responsive Navigation should toggle mobile menu

## Analysis: Why Only Chromium Failures?

Two possible explanations:

### Theory 1: Browser-Specific Issues (Most Likely)
Chromium has stricter timing or renders differently, causing legitimate failures that don't occur in Firefox/Webkit. Common causes:
- Chromium's faster JavaScript execution triggers race conditions
- Different rendering engine timing for animations/transitions
- Stricter security policies in Chromium
- Different viewport handling for responsive tests

### Theory 2: Test Suite Design
Tests may be more Chromium-focused in their assertions or locators, causing false failures in Chromium while Firefox/Webkit happen to pass by chance.

**Recommendation:** Investigate the highest-impact categories (User Lifecycle, Multi-Component Workflows) to determine if these are genuine Chromium bugs or test design issues.

## Next Steps - Prioritized by Impact

### Priority 1: **Settings - User Lifecycle (7 failures)**
**Why:** Critical security and user management functionality
**Impact:** Core authentication, authorization, and audit features
**Estimated Fix Time:** 2-4 hours

**Actions:**
1. Read `tests/core/settings-user-lifecycle.spec.ts`
2. Run targeted tests: `npx playwright test settings-user-lifecycle --project=chromium --headed`
3. Identify common pattern (likely timing issues or role/permission checks)
4. Apply consistent fix across all 7 tests
5. Verify with: `npx playwright test settings-user-lifecycle --project=chromium`

### Priority 2: **Core - Multi-Component Workflows (5 failures)**
**Why:** Integration testing of security features
**Impact:** WAF, ACL, Backup/Restore features
**Estimated Fix Time:** 2-3 hours

**Actions:**
1. Read `tests/core/coreMulti-component-workflows.spec.ts`
2. Check for timeout issues (previous baseline showed 8.8-8.9s timeouts)
3. Increase test timeouts or optimize test setup
4. Validate security module toggle states before assertions

### Priority 3: **Core - Data Consistency (5 failures)**
**Why:** Core CRUD operations and API/UI sync
**Impact:** Fundamental data integrity
**Estimated Fix Time:** 2-3 hours

**Actions:**
1. Read `tests/core/core-data-consistency.spec.ts`
2. Previous baseline showed 90s timeout on validation test
3. Add explicit waits for data synchronization
4. Verify pagination/sorting with `waitForLoadState('networkidle')`

### Priority 4: **Modal Dropdown Failures (2 failures)**
**Why:** Known issue from dropdown triage effort
**Impact:** User workflows blocked
**Estimated Fix Time:** 1 hour

**Actions:**
1. Read `tests/modal-dropdown-triage.spec.ts`
2. Apply dropdown locator fixes from DNS provider work
3. Use role-based locators: `getByRole('combobox', { name: 'Role' })`

### Priority 5: **Lower-Impact Categories (7 failures)**
Certificates (2), Authentication (2), Admin Onboarding (2), Navigation (1)

**Estimated Fix Time:** 2-3 hours for all

## Success Criteria

**Target for Next Iteration:**
- **Total Failures: < 10** (currently 28)
- **Pass Rate: > 99%** (currently 98.3%)
- **All Chromium failures investigated and fixed or documented**
- **Firefox/Webkit remain at 0 failures**

## Commands for Next Steps

### Run Highest-Impact Tests Only
```bash
# User Lifecycle (7 tests)
npx playwright test settings-user-lifecycle --project=chromium

# Multi-Component Workflows (5 tests)
npx playwright test core-multi-component-workflows --project=chromium

# Data Consistency (5 tests)
npx playwright test core-data-consistency --project=chromium
```

### Debug Individual Failures
```bash
# Headed mode with inspector
npx playwright test settings-user-lifecycle --project=chromium --headed --debug

# Generate trace for later analysis
npx playwright test settings-user-lifecycle --project=chromium --trace on
```

### Validate Full Suite After Fixes
```bash
# Quick validation (Chromium only)
npx playwright test --project=chromium

# Full validation (all browsers)
npx playwright test --project=firefox --project=chromium --project=webkit
```

## Notes

- **DNS Provider fixes were successful** - no DNS-related failures observed
- **Previous timeout issues significantly reduced** - from ~163 failures to 28
- **Firefox/Webkit stability excellent** - 0 failures indicates good cross-browser support
- **Chromium failures are isolated** - does not affect other browsers, suggesting browser-specific issues rather than fundamental test flaws

## Files for Investigation

1. `tests/core/settings-user-lifecycle.spec.ts` (7 failures)
2. `tests/core/core-multi-component-workflows.spec.ts` (5 failures)
3. `tests/core/core-data-consistency.spec.ts` (5 failures)
4. `tests/modal-dropdown-triage.spec.ts` (2 failures)
5. `tests/core/certificates.spec.ts` (2 failures)
6. `tests/core/authentication.spec.ts` (2 failures)
7. ` tests/core/admin-onboarding.spec.ts` (2 failures)
8. `tests/core/navigation.spec.ts` (1 failure)

---

**Generated:** February 12, 2026 20:37:05
**Test Duration:** 21 minutes
**Baseline Status:** ✅ **EXCELLENT** - 83% fewer failures than previous baseline
