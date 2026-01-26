# E2E Test Failure Diagnosis - Skip Security Tests

**Issue**: E2E tests failing across all shards in CI. Need to isolate whether security features (ACL, rate limiting) are the root cause.
**Status**: 🔴 ACTIVE - Planning Phase
**Priority**: 🔴 CRITICAL - Blocking all CI
**Created**: 2026-01-26

---

## 🔍 Problem Analysis

### Current Test Architecture
The Playwright configuration has a strict dependency chain:

```
setup (auth) → security-tests → security-teardown → browser tests (chromium/firefox/webkit)
```

**Key Components:**
1. **setup**: Creates authenticated user and stores session
2. **security-tests**: Sequential tests that enable ACL, WAF, CrowdSec, rate limiting - verifies they block correctly
3. **security-teardown**: Disables all security modules via API or emergency endpoint
4. **browser tests**: Main test suites that depend on security being disabled

### Observed Failures
- **Shard 3**: `account-settings.spec.ts:289` - "should validate certificate email format"
- **Shard 4**: `user-management.spec.ts:948` - "should resend invite for pending user"
- **Pattern**: Tests that create/modify resources are failing

### Hypothesis
Two possible root causes:
1. **Security tests are failing/hanging** - blocking browser tests from running
2. **Security teardown is failing** - leaving ACL/rate limiting enabled, which blocks subsequent API calls in browser tests

---

## 🛠️ Remediation Strategy

### Approach: Temporary Security Test Bypass

**Goal**: Skip the entire security-tests project and its teardown to determine if security features are causing the failures.

**Implementation**: Modify `playwright.config.js` to:
1. Comment out the `security-tests` project
2. Comment out the `security-teardown` project
3. Remove `'security-tests'` from the dependencies of browser projects
4. Keep the `setup` project active (authentication still needed)

### Changes Required

**File**: `playwright.config.js`

- Comment out lines 151-169 (security-tests project)
- Comment out lines 171-174 (security-teardown project)
- Remove `'security-tests'` from dependencies arrays on lines 182, 193, 203

---

## ✅ Expected Outcomes

### If Tests Pass
- **Confirms**: Security features (ACL/rate limiting) are the root cause
- **Next Step**: Investigate why security-teardown is failing or incomplete
- **Triage**: Focus on security-teardown.setup.ts and emergency reset endpoint

### If Tests Still Fail
- **Confirms**: Issue is NOT related to security features
- **Next Step**: Investigate Docker environment, database state, or test data isolation
- **Triage**: Focus on test-data-manager.ts, database persistence, or environment setup

---

## 🚦 Rollback Strategy

Once diagnosis is complete, restore the full test suite:

```bash
# Revert playwright.config.js changes
git checkout playwright.config.js

# Run full test suite including security
npx playwright test
```

---

## 📋 Implementation Checklist

- [ ] Modify playwright.config.js to comment out security projects
- [ ] Remove security-tests dependency from browser projects
- [ ] Commit with clear diagnostic message
- [ ] Trigger CI run
- [ ] Analyze results and document findings
- [ ] Restore security tests once diagnosis complete
