# E2E Test Failure Remediation Plan - Emergency Token ACL Bypass

**Status**: ACTIVE - READY FOR IMPLEMENTATION
**Priority**: 🔴 CRITICAL - CI Blocking
**Created**: 2026-01-28
**CI Run**: [#53 (e892669)](https://github.com/Wikid82/Charon/actions/runs/21456401944)
**Branch**: feature/beta-release
**PR**: #574 (Merge pull request from renovate/feature/beta-release-we...)

---

## Executive Summary

| Metric | Value |
|--------|-------|
| **Total Tests** | 362 (across 4 shards) |
| **Passed** | 139 per shard (~556 total runs) |
| **Failed** | 1 unique test (runs in each shard) |
| **Skipped** | 22 per shard |
| **Total Duration** | 8m 55s |

### Failure Summary

| Test File | Test Name | Error | Category |
|-----------|-----------|-------|----------|
| `emergency-token.spec.ts:44` | Test 1: Emergency token bypasses ACL | Expected 403, Received 200 | 🔴 CRITICAL |

### Root Cause Identified

**The test fails because the `beforeAll` hook enables ACL but does NOT re-enable Cerberus (the security master switch).**

The emergency security reset (run in `global-setup.ts`) disables ALL security modules including `feature.cerberus.enabled`. When the test's `beforeAll` only enables `security.acl.enabled = true`, the Cerberus middleware short-circuits at line 162-165 because `IsEnabled()` returns `false`.

```go
// cerberus.go:162-165
if !c.IsEnabled() {
    ctx.Next()  // ← Request passes through without ACL check
    return
}
```

**Impact**:
- 🔴 **CI pipeline blocked** - Cannot merge to feature/beta-release
- 🔴 **1 E2E test fails** - emergency-token.spec.ts Test 1
- 🟢 **No production code issue** - Cerberus ACL logic is correct
- 🟢 **Test issue only** - beforeAll hook missing Cerberus enable step

**The Fix**: Update the test's `beforeAll` hook to enable `feature.cerberus.enabled = true` BEFORE enabling `security.acl.enabled = true`.

**Complexity**: LOW - Single test file fix, ~15 minutes

---

## Phase 1: Critical Fix (🔴 BLOCKING)

### Failure Analysis

#### Test: `emergency-token.spec.ts:44:3` - "Test 1: Emergency token bypasses ACL"

**File:** [tests/security-enforcement/emergency-token.spec.ts](../../../tests/security-enforcement/emergency-token.spec.ts#L44)

**Error Message:**
```
Error: expect(received).toBe(expected) // Object.is equality
Expected: 403
Received: 200

52 | const blockedResponse = await unauthenticatedRequest.get('/api/v1/security/status');
53 | await unauthenticatedRequest.dispose();
54 | expect(blockedResponse.status()).toBe(403);
```

**Expected Behavior:**
- When ACL is enabled, unauthenticated requests to `/api/v1/security/status` should return 403

**Actual Behavior:**
- Request returns 200 (ACL check is bypassed)

**Root Cause Chain:**

1. `global-setup.ts` calls `/api/v1/emergency/security-reset`
2. Emergency reset disables: `feature.cerberus.enabled`, `security.acl.enabled`, `security.waf.enabled`, `security.rate_limit.enabled`, `security.crowdsec.enabled`
3. Test's `beforeAll` only enables: `security.acl.enabled = true`
4. Cerberus middleware checks `IsEnabled()` which reads `feature.cerberus.enabled` (still false)
5. Cerberus returns early without checking ACL → request passes through

**Issue Type:** Test Issue (incomplete setup)

---

## EARS Requirements

### REQ-001: Cerberus Master Switch Precondition

**Priority:** 🔴 CRITICAL

**EARS Notation:**
> WHEN security test suite `beforeAll` hook enables any individual security module (ACL, WAF, rate limiting),
> THE SYSTEM SHALL first ensure `feature.cerberus.enabled` is set to `true` before enabling the specific module.

**Acceptance Criteria:**
- [ ] Test's `beforeAll` enables `feature.cerberus.enabled = true` BEFORE `security.acl.enabled`
- [ ] Wait for security propagation between setting changes
- [ ] Verify Cerberus is active by checking `/api/v1/security/status` response

---

### REQ-002: Security Module Dependency Validation

**Priority:** 🟡 MEDIUM

**EARS Notation:**
> WHILE the Cerberus master switch (`feature.cerberus.enabled`) is disabled,
> THE SYSTEM SHALL ignore individual security module settings (ACL, WAF, rate limiting) and allow all requests through.

**Documentation Note:** This is DOCUMENTED behavior, but tests must respect this precondition.

---

### REQ-003: ACL Blocking Verification

**Priority:** 🔴 CRITICAL

**EARS Notation:**
> WHEN ACL is enabled AND Cerberus is enabled AND there are no active access lists AND the request is NOT from an authenticated admin,
> THE SYSTEM SHALL return HTTP 403 with error message containing "Blocked by access control".

**Verification:**
```go
// cerberus.go:233-238
if activeCount == 0 {
    if isAdmin && !adminWhitelistConfigured {
        ctx.Next()
        return
    }
    ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Blocked by access control list"})
    return
}
```

---

## Implementation Plan

### Task 1: Fix Test `beforeAll` Hook (🔴 CRITICAL)

**File:** [tests/security-enforcement/emergency-token.spec.ts](../../../tests/security-enforcement/emergency-token.spec.ts#L18-L40)

**Current Code (Lines 18-40):**
```typescript
test.beforeAll(async ({ request }) => {
  console.log('🔧 Setting up test suite: Ensuring ACL is enabled...');

  const emergencyToken = process.env.CHARON_EMERGENCY_TOKEN;
  if (!emergencyToken) {
    throw new Error('CHARON_EMERGENCY_TOKEN not set - cannot configure test environment');
  }

  // Use emergency token to enable ACL (bypasses any existing security)
  const enableResponse = await request.patch('/api/v1/settings', {
    data: { key: 'security.acl.enabled', value: 'true' },
    headers: {
      'X-Emergency-Token': emergencyToken,
    },
  });

  if (!enableResponse.ok()) {
    throw new Error(`Failed to enable ACL for test suite: ${enableResponse.status()}`);
  }

  // Wait for security propagation
  await new Promise(resolve => setTimeout(resolve, 2000));
  console.log('✅ ACL enabled for test suite');
});
```

**Fixed Code:**
```typescript
test.beforeAll(async ({ request }) => {
  console.log('🔧 Setting up test suite: Ensuring Cerberus and ACL are enabled...');

  const emergencyToken = process.env.CHARON_EMERGENCY_TOKEN;
  if (!emergencyToken) {
    throw new Error('CHARON_EMERGENCY_TOKEN not set - cannot configure test environment');
  }

  // CRITICAL: Must enable Cerberus master switch FIRST
  // The emergency reset disables feature.cerberus.enabled, which makes
  // all other security modules ineffective (IsEnabled() returns false).
  const cerberusResponse = await request.patch('/api/v1/settings', {
    data: { key: 'feature.cerberus.enabled', value: 'true' },
    headers: { 'X-Emergency-Token': emergencyToken },
  });
  if (!cerberusResponse.ok()) {
    throw new Error(`Failed to enable Cerberus: ${cerberusResponse.status()}`);
  }
  console.log('  ✓ Cerberus master switch enabled');

  // Wait for Cerberus to activate
  await new Promise(resolve => setTimeout(resolve, 1000));

  // Now enable ACL (will be effective since Cerberus is active)
  const aclResponse = await request.patch('/api/v1/settings', {
    data: { key: 'security.acl.enabled', value: 'true' },
    headers: { 'X-Emergency-Token': emergencyToken },
  });
  if (!aclResponse.ok()) {
    throw new Error(`Failed to enable ACL: ${aclResponse.status()}`);
  }
  console.log('  ✓ ACL enabled');

  // Wait for security propagation (settings cache TTL is 60s, but changes are immediate)
  await new Promise(resolve => setTimeout(resolve, 2000));

  // VALIDATION: Verify security is actually blocking before proceeding
  console.log('  🔍 Verifying ACL is active...');
  const statusResponse = await request.get('/api/v1/security/status');
  if (statusResponse.ok()) {
    const status = await statusResponse.json();
    if (!status.acl?.enabled) {
      throw new Error('ACL verification failed: ACL not reported as enabled in status');
    }
    console.log('  ✓ ACL verified as enabled');
  }

  console.log('✅ Cerberus and ACL enabled for test suite');
});
```

**Estimated Time:** 15 minutes

---

### Task 2: Add `afterAll` Cleanup Hook

**File:** [tests/security-enforcement/emergency-token.spec.ts](../../../tests/security-enforcement/emergency-token.spec.ts)

**New Code (add after `beforeAll`):**
```typescript
test.afterAll(async ({ request }) => {
  console.log('🧹 Cleaning up test suite: Resetting security state...');

  const emergencyToken = process.env.CHARON_EMERGENCY_TOKEN;
  if (!emergencyToken) {
    console.warn('⚠️ No emergency token for cleanup');
    return;
  }

  // Reset to safe state for other tests
  const response = await request.post('/api/v1/emergency/security-reset', {
    headers: { 'X-Emergency-Token': emergencyToken },
  });

  if (response.ok()) {
    console.log('✅ Security state reset for next test suite');
  } else {
    console.warn(`⚠️ Security reset returned: ${response.status()}`);
  }
});
```

**Estimated Time:** 5 minutes

---

## Dependency Diagram

```
┌───────────────────────────────────────────────────────────────────┐
│                        Global Setup                                │
│  (global-setup.ts → emergency security reset → ALL modules OFF)    │
└───────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌───────────────────────────────────────────────────────────────────┐
│                        Auth Setup                                  │
│  (auth.setup.ts → authenticates test user)                        │
└───────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌───────────────────────────────────────────────────────────────────┐
│                  Test Suite: beforeAll                             │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │ STEP 1: enable feature.cerberus.enabled = true              │  │
│  │         (Cerberus master switch - REQUIRED FIRST!)          │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                              │                                     │
│                              ▼                                     │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │ STEP 2: enable security.acl.enabled = true                  │  │
│  │         (Now effective because Cerberus is ON)              │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                              │                                     │
│                              ▼                                     │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │ STEP 3: Verify /api/v1/security/status shows ACL enabled    │  │
│  └─────────────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌───────────────────────────────────────────────────────────────────┐
│                     Test 1 Execution                               │
│  • Unauthenticated request → should get 403                       │
│  • Emergency token request → should get 200                       │
└───────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌───────────────────────────────────────────────────────────────────┐
│                  Test Suite: afterAll                              │
│  • Call emergency security reset                                   │
│  • Restore clean state for next suite                              │
└───────────────────────────────────────────────────────────────────┘
```

---

## Success Criteria

### Phase 1 Complete When:

- [ ] `emergency-token.spec.ts` passes locally with `npx playwright test tests/security-enforcement/emergency-token.spec.ts`
- [ ] All 4 CI shards pass the emergency token test
- [ ] No regressions in other security enforcement tests
- [ ] Test properly cleans up in `afterAll`

### Verification Commands:

```bash
# Local verification
npx playwright test tests/security-enforcement/emergency-token.spec.ts --project=chromium

# Full security test suite
npx playwright test tests/security-enforcement/ --project=chromium

# View report after run
npx playwright show-report
```

---

## Estimated Total Remediation Time

| Task | Time |
|------|------|
| Task 1: Fix beforeAll hook | 15 min |
| Task 2: Add afterAll cleanup | 5 min |
| Local testing & verification | 15 min |
| **Total** | **35 min** |

---

## Related Files

| File | Purpose |
|------|---------|
| [tests/security-enforcement/emergency-token.spec.ts](../../../tests/security-enforcement/emergency-token.spec.ts) | Failing test (FIX HERE) |
| [tests/global-setup.ts](../../../tests/global-setup.ts) | Global setup with emergency reset |
| [tests/fixtures/security.ts](../../../tests/fixtures/security.ts) | Security test helpers |
| [backend/internal/cerberus/cerberus.go](../../../backend/internal/cerberus/cerberus.go) | Cerberus middleware |

---

## Appendix: Other Observations

### Skipped Test Analysis

**File:** `emergency-reset.spec.ts:69` - "should rate limit after 5 attempts"

This test is marked `.skip` in the source. The skip is intentional and documented:

```typescript
// Rate limiting is covered in emergency-token.spec.ts (Test 2)
test.skip('should rate limit after 5 attempts', ...)
```

**No action required** - this is expected behavior.

### CI Workflow Observations

1. **4-shard parallel execution** - Each shard runs the same failing test independently
2. **139 passing tests per shard** - Good overall health
3. **22 skipped tests** - Expected (tagged tests, conditional skips)
4. **Merge Test Reports failed** - Cascading failure from E2E test failure

---

## Remediation Status

| Phase | Status | Assignee | ETA |
|-------|--------|----------|-----|
| Phase 1: Critical Fix | 🟡 Ready for Implementation | - | ~35 min |

---

*Generated by Planning Agent on 2026-01-28*
