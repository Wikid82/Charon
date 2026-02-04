# E2E Test Fix Specification

## Executive Summary

Three Playwright E2E tests are failing due to incorrect API endpoint usage, wrong response field access, and hardcoded authentication path. This specification provides comprehensive fixes for all three issues with detailed before/after code snippets, root cause analysis, and verification steps.

**Failing Tests:**
1. Break Glass Recovery - Step 2: Re-enable Cerberus framework (line 97)
2. Emergency Security Reset - should reset security when called with valid token (line 28)
3. Security Teardown - verify-security-state-for-ui-tests (line 34)

---

## Test Execution Order & Dependencies

From `playwright.config.js`, tests execute in this order:
1. **Setup** (`auth.setup.ts`) → Creates authentication state
2. **Security Tests** (sequential) → Enables/validates security modules
3. **Emergency Reset** (`emergency-reset.spec.ts`) → Break glass test (disables modules)
4. **Break Glass Recovery** (`zzzz-break-glass-recovery.spec.ts`) → Restores Cerberus + Universal bypass
5. **Security Teardown** (`security-teardown.setup.ts`) → Verifies state for browser tests
6. **Browser Projects** (chromium, firefox, webkit) → UI/UX tests

**Authentication Storage:**
- Path: `playwright/.auth/user.json` (defined in `tests/constants.ts`)
- Type: Session cookies and localStorage state
- Referenced by: `STORAGE_STATE` constant

---

## Issue 1: Break Glass Recovery - Wrong Endpoint & Field Access

### Location
**File:** `tests/security-enforcement/zzzz-break-glass-recovery.spec.ts`
**Lines:** 92-97 (Step 2: Verify Cerberus is enabled)

### Root Cause Analysis

**Problem 1: Wrong Endpoint**
- Test uses: `GET /api/v1/security/config`
- Correct endpoint: `GET /api/v1/security/status`

**Problem 2: Wrong Response Structure**
- Test expects: `body.enabled` (flat structure)
- Actual response: `body.cerberus.enabled` (nested structure)

**Backend Response Structure:**
```go
// backend/internal/api/handlers/security_handler.go:184-185
c.JSON(http.StatusOK, gin.H{
    "cerberus": gin.H{"enabled": enabled},
    "crowdsec": gin.H{"mode": crowdSecMode, "api_url": crowdSecAPIURL, "enabled": crowdsecEnabled},
    "waf": gin.H{"mode": wafMode, "enabled": wafEnabled},
    "rate_limit": gin.H{"mode": rateLimitMode, "enabled": rateLimitEnabled},
    "acl": gin.H{"mode": aclMode, "enabled": aclEnabled},
})
```

**Why `/api/v1/security/config` is Wrong:**
- Returns `{"config": SecurityConfig}` model structure
- Intended for configuration management (CRUD operations)
- Does not include `enabled` field at root level
- Contains database model fields (Name, AdminWhitelist, WAFExclusions, etc.)

**Why `/api/v1/security/status` is Correct:**
- Returns current runtime status of all security modules
- Aggregates state from 3 sources (settings table > DB config > static config)
- Provides flat-structure access to all module states
- Used by frontend dashboard and monitoring

### Fix Implementation

**Before (BROKEN):**
```typescript
await test.step('Verify Cerberus is enabled', async () => {
  const response = await request.get(`${BASE_URL}/api/v1/security/config`);
  expect(response.ok()).toBeTruthy();

  const body = await response.json();
  expect(body.enabled).toBe(true); // feature.cerberus.enabled = true
  console.log('✅ Cerberus framework status verified: ENABLED');
});
```

**After (FIXED):**
```typescript
await test.step('Verify Cerberus is enabled', async () => {
  const response = await request.get(`${BASE_URL}/api/v1/security/status`);
  expect(response.ok()).toBeTruthy();

  const body = await response.json();
  expect(body.cerberus.enabled).toBe(true); // feature.cerberus.enabled = true
  console.log('✅ Cerberus framework status verified: ENABLED');
});
```

**Changes:**
1. **Line 92:** Change endpoint from `/api/v1/security/config` to `/api/v1/security/status`
2. **Line 96:** Change field access from `body.enabled` to `body.cerberus.enabled`

### Additional Fix Required (Lines 153-175)

**Issue:** Step 4 also has incorrect field access (line 157)

**Before:**
```typescript
await test.step('Verify all security modules are enabled', async () => {
  const response = await request.get(`${BASE_URL}/api/v1/security/status`);
  expect(response.ok()).toBeTruthy();

  const body = await response.json();

  // Cerberus framework
  expect(body.cerberus_enabled).toBe(true); // WRONG: Should be body.cerberus.enabled

  // Security modules
  expect(body.acl?.enabled).toBe(true);
  expect(body.waf?.enabled).toBe(true);
  expect(body.rate_limit?.enabled).toBe(true);
```

**After:**
```typescript
await test.step('Verify all security modules are enabled', async () => {
  const response = await request.get(`${BASE_URL}/api/v1/security/status`);
  expect(response.ok()).toBeTruthy();

  const body = await response.json();

  // Cerberus framework
  expect(body.cerberus.enabled).toBe(true); // FIXED: Correct nested access

  // Security modules
  expect(body.acl?.enabled).toBe(true);
  expect(body.waf?.enabled).toBe(true);
  expect(body.rate_limit?.enabled).toBe(true);
```

**Changes:**
1. **Line 157:** Change `body.cerberus_enabled` to `body.cerberus.enabled`
2. **Line 165:** Change console log from `body.cerberus_enabled` to `body.cerberus.enabled`

### Verification Steps

1. Run test in isolation:
   ```bash
   npx playwright test tests/security-enforcement/zzzz-break-glass-recovery.spec.ts --project=chromium
   ```

2. Verify Step 2 passes:
   - Assert Step 2 completes without "undefined" error
   - Check console output shows "✅ Cerberus framework status verified: ENABLED"

3. Verify Step 4 passes:
   - Assert all module status checks pass
   - Check console output shows correct Cerberus status

---

## Issue 2: Emergency Security Reset - Missing Module in Response

### Location
**File:** `tests/security-enforcement/emergency-reset.spec.ts`
**Line:** 28

### Root Cause Analysis

**Problem:** Test expects `feature.cerberus.enabled` in `disabled_modules` array, but backend intentionally **does not** disable it.

**Backend Implementation:**
```go
// backend/internal/api/handlers/emergency_handler.go:282-297
func (h *EmergencyHandler) disableAllSecurityModules() ([]string, error) {
    disabledModules := []string{}

    // Settings to disable - NOTE: We keep feature.cerberus.enabled = true
    // so E2E tests can validate break glass functionality.
    // Only individual security modules are disabled for clean test state.
    securitySettings := map[string]string{
        // Feature framework stays ENABLED (removed from this map)
        // "feature.cerberus.enabled": "false",  ← BUG FIX: Keep framework enabled
        // "security.cerberus.enabled": "false", ← BUG FIX: Keep framework enabled

        // Individual security modules disabled for clean slate
        "security.acl.enabled":        "false",
        "security.waf.enabled":        "false",
        "security.rate_limit.enabled": "false",
        "security.crowdsec.enabled":   "false",
        "security.crowdsec.mode":      "disabled",
    }
```

**Design Intent (from code comments):**
- Cerberus **framework** remains ENABLED for break glass validation
- Only individual **security modules** (ACL, WAF, Rate Limit, CrowdSec) are disabled
- This allows break-glass-recovery test to re-enable modules and verify framework behavior

**Why This is Correct Behavior:**
1. Break glass disables **enforcement modules**, not the **management framework**
2. Keeping Cerberus enabled allows dashboard to show module states
3. Subsequent tests (break-glass-recovery) can re-enable modules via API
4. Emergency reset is for **lockout recovery**, not **framework removal**

### Fix Implementation

**Before (BROKEN):**
```typescript
test('should reset security when called with valid token', async ({ request }) => {
  const response = await request.post('/api/v1/emergency/security-reset', {
    headers: {
      'X-Emergency-Token': EMERGENCY_TOKEN,
      'Content-Type': 'application/json',
    },
    data: { reason: 'E2E test validation' },
  });

  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expect(body.success).toBe(true);
  expect(body.disabled_modules).toContain('security.acl.enabled');
  expect(body.disabled_modules).toContain('feature.cerberus.enabled'); // ← REMOVE THIS
});
```

**After (FIXED):**
```typescript
test('should reset security when called with valid token', async ({ request }) => {
  const response = await request.post('/api/v1/emergency/security-reset', {
    headers: {
      'X-Emergency-Token': EMERGENCY_TOKEN,
      'Content-Type': 'application/json',
    },
    data: { reason: 'E2E test validation' },
  });

  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  expect(body.success).toBe(true);

  // Verify individual security modules are disabled
  expect(body.disabled_modules).toContain('security.acl.enabled');
  expect(body.disabled_modules).toContain('security.waf.enabled');
  expect(body.disabled_modules).toContain('security.rate_limit.enabled');
  expect(body.disabled_modules).toContain('security.crowdsec.enabled');
  expect(body.disabled_modules).toContain('security.crowdsec.mode');

  // NOTE: feature.cerberus.enabled is NOT disabled by emergency reset
  // The Cerberus framework stays enabled to allow security module management
  // Only enforcement modules (ACL, WAF, Rate Limit, CrowdSec) are disabled
  expect(body.disabled_modules).not.toContain('feature.cerberus.enabled');
});
```

**Changes:**
1. **Line 28:** Remove expectation for `feature.cerberus.enabled`
2. **Add comprehensive module verification** for all disabled modules
3. **Add explanatory comment** documenting the design intent
4. **Add negative assertion** confirming Cerberus framework stays enabled

### Verification Steps

1. Run test in isolation:
   ```bash
   npx playwright test tests/security-enforcement/emergency-reset.spec.ts --project=chromium
   ```

2. Verify response structure:
   - Assert `disabled_modules` contains exactly 5 keys
   - Confirm `feature.cerberus.enabled` is NOT in the array
   - Validate all security module keys are present

3. Verify break glass workflow:
   - Emergency reset disables enforcement modules
   - Break glass recovery can re-enable them (next test verifies this)

---

## Issue 3: Security Teardown - Hardcoded Auth Path

### Location
**File:** `tests/security-teardown.setup.ts`
**Line:** 34

### Root Cause Analysis

**Problem:** Hardcoded authentication path doesn't match the workspace standard

**Hardcoded Path:** `playwright/.auth/admin.json`
**Standard Path:** `playwright/.auth/user.json` (via `STORAGE_STATE` constant)

**Why This is Wrong:**
1. **Inconsistency:** All other tests use `STORAGE_STATE` from `tests/constants.ts`
2. **File doesn't exist:** `admin.json` is never created by `auth.setup.ts`
3. **Auth failure:** Request context has no cookies, leading to 401/403 errors
4. **Playwright config uses `user.json`:** All browser projects reference this file

**Standard Auth Flow:**
```typescript
// tests/constants.ts
export const STORAGE_STATE = join(__dirname, '../playwright/.auth/user.json');

// playwright.config.js (lines 98, 107, 116)
use: {
  storageState: STORAGE_STATE, // Points to user.json
}
```

### Fix Implementation

**Before (BROKEN):**
```typescript
import { test as teardown } from '@bgotink/playwright-coverage';
import { request } from '@playwright/test';

teardown('verify-security-state-for-ui-tests', async () => {
  console.log('\n🔍 Security Teardown: Verifying state for UI tests...');
  console.log('   Expected: Cerberus ON + All modules ON + Universal bypass (0.0.0.0/0)');

  const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';

  // Create authenticated request context with storage state
  const requestContext = await request.newContext({
    baseURL,
    storageState: 'playwright/.auth/admin.json', // ← HARDCODED PATH
  });
```

**After (FIXED):**
```typescript
import { test as teardown } from '@bgotink/playwright-coverage';
import { request } from '@playwright/test';
import { STORAGE_STATE } from './constants'; // ← ADD THIS IMPORT

teardown('verify-security-state-for-ui-tests', async () => {
  console.log('\n🔍 Security Teardown: Verifying state for UI tests...');
  console.log('   Expected: Cerberus ON + All modules ON + Universal bypass (0.0.0.0/0)');

  const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';

  // Create authenticated request context with storage state
  const requestContext = await request.newContext({
    baseURL,
    storageState: STORAGE_STATE, // ← USE CONSTANT
  });
```

**Changes:**
1. **Line 3:** Add import statement `import { STORAGE_STATE } from './constants';`
2. **Line 34:** Replace hardcoded string with `STORAGE_STATE` constant

### Additional Fixes Required (Lines 46-79)

The test also uses wrong API endpoints and field names (similar to Issue 1):

**Before:**
```typescript
// Verify Cerberus framework is enabled
const cerberusResponse = await requestContext.get(`${baseURL}/api/v1/security/config`);
if (cerberusResponse.ok()) {
  const config = await cerberusResponse.json();
  if (config.enabled === true) { // WRONG: Should be config.config.Enabled
    console.log('✅ Cerberus framework: ENABLED');
  }
```

**After:**
```typescript
// Verify Cerberus framework is enabled
const cerberusResponse = await requestContext.get(`${baseURL}/api/v1/security/status`);
if (cerberusResponse.ok()) {
  const status = await cerberusResponse.json();
  if (status.cerberus.enabled === true) { // FIXED: Correct nested access
    console.log('✅ Cerberus framework: ENABLED');
  }
```

**Also fix admin_whitelist check (lines 56-62):**
```typescript
// Before
if (config.admin_whitelist === '0.0.0.0/0') {

// After
if (status.acl?.admin_whitelist === '0.0.0.0/0') {
  // NOTE: admin_whitelist may be in /api/v1/security/config instead
  // Fetch from config endpoint if not in status response
```

**Actually, admin_whitelist belongs to SecurityConfig, not status. Fix:**
```typescript
// Get admin whitelist from config endpoint
const configResponse = await requestContext.get(`${baseURL}/api/v1/security/config`);
if (configResponse.ok()) {
  const configData = await configResponse.json();
  if (configData.config?.admin_whitelist === '0.0.0.0/0') {
    console.log('✅ Admin whitelist: 0.0.0.0/0 (universal bypass)');
  } else {
    console.log(`⚠️  Admin whitelist: ${configData.config?.admin_whitelist || 'none'} (expected: 0.0.0.0/0)`);
    allChecksPass = false;
  }
}
```

### Comprehensive Fix (Lines 40-95)

**Complete refactored teardown logic:**

```typescript
let allChecksPass = true;

try {
  // Verify Cerberus framework is enabled via status endpoint
  const statusResponse = await requestContext.get(`${baseURL}/api/v1/security/status`);
  if (statusResponse.ok()) {
    const status = await statusResponse.json();
    if (status.cerberus.enabled === true) {
      console.log('✅ Cerberus framework: ENABLED');
    } else {
      console.log('⚠️  Cerberus framework: DISABLED (expected: ENABLED)');
      allChecksPass = false;
    }

    // Verify security modules status
    console.log(`   ACL module:         ${status.acl?.enabled ? '✅ ENABLED' : '⚠️  disabled'}`);
    console.log(`   WAF module:         ${status.waf?.enabled ? '✅ ENABLED' : '⚠️  disabled'}`);
    console.log(`   Rate Limit module:  ${status.rate_limit?.enabled ? '✅ ENABLED' : '⚠️  disabled'}`);
    console.log(`   CrowdSec module:    ${status.crowdsec?.running ? '✅ RUNNING' : '⚠️  not available (OK for E2E)'}`);

    // ACL, WAF, and Rate Limit should be enabled
    if (!status.acl?.enabled || !status.waf?.enabled || !status.rate_limit?.enabled) {
      console.log('⚠️  Some security modules are disabled (expected: all enabled)');
      allChecksPass = false;
    }
  } else {
    console.log('⚠️  Could not verify security module status');
    allChecksPass = false;
  }

  // Verify admin whitelist via config endpoint
  const configResponse = await requestContext.get(`${baseURL}/api/v1/security/config`);
  if (configResponse.ok()) {
    const configData = await configResponse.json();
    if (configData.config?.admin_whitelist === '0.0.0.0/0') {
      console.log('✅ Admin whitelist: 0.0.0.0/0 (universal bypass)');
    } else {
      console.log(`⚠️  Admin whitelist: ${configData.config?.admin_whitelist || 'none'} (expected: 0.0.0.0/0)`);
      allChecksPass = false;
    }
  } else {
    console.log('⚠️  Could not verify admin whitelist configuration');
    allChecksPass = false;
  }

  if (allChecksPass) {
    console.log('\n✅ Security Teardown COMPLETE: State verified for UI tests');
    console.log('   Browser tests can now safely test toggles/navigation');
  } else {
    console.log('\n⚠️  Security Teardown: Some checks failed (see warnings above)');
    console.log('   UI tests may encounter issues if configuration is incorrect');
    console.log('   Expected state: Cerberus ON + All modules ON + Universal bypass (0.0.0.0/0)');
  }
} catch (error) {
  console.error('Error verifying security state:', error);
  throw new Error('Security teardown verification failed');
} finally {
  await requestContext.dispose();
}
```

### Verification Steps

1. Check authentication file exists:
   ```bash
   ls -la playwright/.auth/user.json
   ```

2. Run teardown in isolation:
   ```bash
   npx playwright test tests/security-teardown.setup.ts
   ```

3. Verify successful authentication:
   - Assert no ENOENT errors
   - Check API requests return 200 OK
   - Validate all status checks pass

---

## Test Fixes Summary

### Files to Modify

1. **`tests/security-enforcement/zzzz-break-glass-recovery.spec.ts`**
   - Line 3: Add import (if not present): `import { STORAGE_STATE } from '../constants';`
   - Line 92: Change endpoint to `/api/v1/security/status`
   - Line 96: Change field to `body.cerberus.enabled`
   - Line 153: Change endpoint to `/api/v1/security/status` (already correct)
   - Line 157: Change field to `body.cerberus.enabled`
   - Line 165: Change console log field to `body.cerberus.enabled`

2. **`tests/security-enforcement/emergency-reset.spec.ts`**
   - Line 28: Remove `feature.cerberus.enabled` expectation
   - Add comprehensive assertions for all disabled modules
   - Add explanatory comment about design intent

3. **`tests/security-teardown.setup.ts`**
   - Line 3: Add import: `import { STORAGE_STATE } from './constants';`
   - Line 34: Replace hardcoded path with `STORAGE_STATE`
   - Lines 40-95: Refactor to use correct endpoints and field access

### Implementation Order

1. **First:** Fix authentication path (Issue 3)
   - Prevents authentication failures in all subsequent tests
   - Required for API requests to succeed

2. **Second:** Fix emergency reset expectations (Issue 2)
   - Aligns test with actual backend behavior
   - Establishes correct break glass state

3. **Third:** Fix break glass recovery endpoints (Issue 1)
   - Verifies Cerberus restoration works correctly
   - Prepares correct state for browser tests

### Expected Test Results After Fixes

**Pass Criteria:**
- All 3 tests pass without errors
- No "undefined" field access errors
- No ENOENT file not found errors
- No unexpected module state warnings

**Console Output Validation:**
```
Test 1 (Break Glass Recovery):
✅ Admin whitelist set to 0.0.0.0/0 (universal bypass)
✅ Whitelist configuration verified
✅ Cerberus framework re-enabled
✅ Cerberus framework status verified: ENABLED
✅ ACL module enabled
✅ WAF module enabled
✅ Rate Limiting module enabled

Test 2 (Emergency Security Reset):
✅ Success: true
✅ Disabled modules: [5 keys, NOT including feature.cerberus.enabled]

Test 3 (Security Teardown):
✅ Cerberus framework: ENABLED
✅ ACL module: ENABLED
✅ WAF module: ENABLED
✅ Rate Limit module: ENABLED
✅ Admin whitelist: 0.0.0.0/0 (universal bypass)
✅ Security Teardown COMPLETE
```

---

## Backend API Reference

### GET /api/v1/security/status

**Purpose:** Get current runtime status of all security modules

**Response Structure:**
```json
{
  "cerberus": {
    "enabled": true
  },
  "acl": {
    "mode": "enabled",
    "enabled": true
  },
  "waf": {
    "mode": "block",
    "enabled": true
  },
  "rate_limit": {
    "mode": "enabled",
    "enabled": true
  },
  "crowdsec": {
    "mode": "local",
    "api_url": "http://crowdsec:8080",
    "enabled": true
  }
}
```

**Priority Chain:**
1. Settings table (runtime overrides) - HIGHEST
2. SecurityConfig DB record (user configuration)
3. Static config from environment - LOWEST

**Usage:** Status checks, dashboard display, module state verification

### GET /api/v1/security/config

**Purpose:** Get SecurityConfig database record

**Response Structure:**
```json
{
  "config": {
    "id": 1,
    "name": "default",
    "enabled": true,
    "admin_whitelist": "0.0.0.0/0",
    "waf_mode": "block",
    "waf_exclusions": "[]",
    "rate_limit_mode": "enabled",
    "rate_limit_enable": true,
    "crowdsec_mode": "local",
    "crowdsec_api_url": "http://crowdsec:8080",
    "acl_mode": "enabled"
  }
}
```

**Usage:** Configuration management, whitelist verification, WAF exclusions

### POST /api/v1/emergency/security-reset

**Purpose:** Break glass endpoint to disable security modules

**Request Headers:**
- `X-Emergency-Token: <32+ char token>`

**Request Body:**
```json
{
  "reason": "Emergency lockout recovery"
}
```

**Response Structure:**
```json
{
  "success": true,
  "message": "All security modules have been disabled. Please reconfigure security settings.",
  "disabled_modules": [
    "security.acl.enabled",
    "security.waf.enabled",
    "security.rate_limit.enabled",
    "security.crowdsec.enabled",
    "security.crowdsec.mode"
  ]
}
```

**Note:** `feature.cerberus.enabled` is intentionally NOT disabled.

---

## Playwright Best Practices Applied

### 1. Use Constants for Shared Values
```typescript
import { STORAGE_STATE } from './constants';
// Better than: storageState: 'playwright/.auth/user.json'
```

### 2. Descriptive Test Steps
```typescript
await test.step('Verify Cerberus is enabled', async () => {
  // Clear intent for debugging
});
```

### 3. Comprehensive Assertions
```typescript
// Bad
expect(body.disabled_modules).toContain('security.acl.enabled');

// Good
expect(body.disabled_modules).toContain('security.acl.enabled');
expect(body.disabled_modules).toContain('security.waf.enabled');
expect(body.disabled_modules).toContain('security.rate_limit.enabled');
expect(body.disabled_modules).not.toContain('feature.cerberus.enabled');
```

### 4. Explanatory Comments
```typescript
// NOTE: feature.cerberus.enabled is NOT disabled by emergency reset
// The Cerberus framework stays enabled to allow security module management
```

### 5. Correct API Endpoint Usage
- `/api/v1/security/status` → Current runtime state
- `/api/v1/security/config` → Database configuration

### 6. Centralized Request Context
```typescript
const requestContext = await request.newContext({
  baseURL,
  storageState: STORAGE_STATE,
});

try {
  // Use requestContext for all API calls
} finally {
  await requestContext.dispose(); // Always cleanup
}
```

---

## Acceptance Criteria

### Definition of Done

- [ ] All 3 test files modified with correct fixes
- [ ] No hardcoded authentication paths remain
- [ ] All API endpoints use correct routes
- [ ] All response fields use correct nested access
- [ ] Tests pass locally on all 3 browsers (chromium, firefox, webkit)
- [ ] Tests pass in CI environment
- [ ] No regression in other test files
- [ ] Console output shows expected success messages
- [ ] Code follows Playwright best practices
- [ ] Explanatory comments added for design decisions

### Verification Commands

```bash
# 1. Rebuild E2E environment
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e

# 2. Run affected tests
npx playwright test tests/security-enforcement/zzzz-break-glass-recovery.spec.ts --project=chromium
npx playwright test tests/security-enforcement/emergency-reset.spec.ts --project=chromium
npx playwright test tests/security-teardown.setup.ts

# 3. Run full security-tests project
npx playwright test --project=security-tests

# 4. Run all browser projects to verify no regression
npx playwright test --project=chromium --project=firefox --project=webkit
```

### Expected Test Duration

- Break Glass Recovery: ~15 seconds
- Emergency Security Reset: ~8 seconds
- Security Teardown: ~5 seconds
- **Total:** ~28 seconds for all three tests

---

## Implementation Checklist

### Phase 1: Authentication Fix (Issue 3)
- [ ] Add `STORAGE_STATE` import to `security-teardown.setup.ts`
- [ ] Replace hardcoded path with constant
- [ ] Verify `user.json` exists after running `auth.setup.ts`
- [ ] Test standalone execution of teardown

### Phase 2: Emergency Reset Fix (Issue 2)
- [ ] Remove `feature.cerberus.enabled` expectation
- [ ] Add comprehensive module assertions
- [ ] Add explanatory comment
- [ ] Test standalone execution of emergency-reset

### Phase 3: Break Glass Recovery Fix (Issue 1)
- [ ] Change Step 2 endpoint to `/api/v1/security/status`
- [ ] Fix Step 2 field access to `body.cerberus.enabled`
- [ ] Fix Step 4 field access to `body.cerberus.enabled`
- [ ] Fix console log field references
- [ ] Test standalone execution of break-glass-recovery

### Phase 4: Integration Testing
- [ ] Run all three tests sequentially
- [ ] Verify test execution order is correct
- [ ] Check authentication state persists
- [ ] Validate security module state transitions
- [ ] Confirm browser tests can run after teardown

### Phase 5: Validation
- [ ] Run on all browsers (chromium, firefox, webkit)
- [ ] Verify CI pipeline passes
- [ ] Check code review for Playwright best practices
- [ ] Update QA report with test results
- [ ] Close related GitHub issues

---

## Related Documentation

- **Test Execution Order:** `playwright.config.js` (lines 98-168)
- **Authentication Setup:** `tests/auth.setup.ts`
- **Constants:** `tests/constants.ts`
- **Backend Handlers:**
  - `backend/internal/api/handlers/security_handler.go` (GetStatus, GetConfig)
  - `backend/internal/api/handlers/emergency_handler.go` (SecurityReset)
- **Previous Analysis:** `docs/plans/e2e-test-triage-plan.md`

---

## Appendix: Complete File Diffs

### A. `tests/security-enforcement/zzzz-break-glass-recovery.spec.ts`

```diff
--- a/tests/security-enforcement/zzzz-break-glass-recovery.spec.ts
+++ b/tests/security-enforcement/zzzz-break-glass-recovery.spec.ts
@@ -89,11 +89,11 @@ test.describe.serial('Break Glass Recovery - Universal Bypass', () => {

     await test.step('Verify Cerberus is enabled', async () => {
-      const response = await request.get(`${BASE_URL}/api/v1/security/config`);
+      const response = await request.get(`${BASE_URL}/api/v1/security/status`);
       expect(response.ok()).toBeTruthy();

       const body = await response.json();
-      expect(body.enabled).toBe(true); // feature.cerberus.enabled = true
+      expect(body.cerberus.enabled).toBe(true); // feature.cerberus.enabled = true
       console.log('✅ Cerberus framework status verified: ENABLED');
     });
   });
@@ -154,15 +154,15 @@ test.describe.serial('Break Glass Recovery - Universal Bypass', () => {
       const body = await response.json();

       // Cerberus framework
-      expect(body.cerberus_enabled).toBe(true);
+      expect(body.cerberus.enabled).toBe(true);

       // Security modules
       expect(body.acl?.enabled).toBe(true);
       expect(body.waf?.enabled).toBe(true);
       expect(body.rate_limit?.enabled).toBe(true);

       // CrowdSec may or may not be running
-      console.log(`   Cerberus: ${body.cerberus_enabled ? '✅ ENABLED' : '❌ DISABLED'}`);
+      console.log(`   Cerberus: ${body.cerberus.enabled ? '✅ ENABLED' : '❌ DISABLED'}`);
       console.log(`   ACL:      ${body.acl?.enabled ? '✅ ENABLED' : '❌ DISABLED'}`);
       console.log(`   WAF:      ${body.waf?.enabled ? '✅ ENABLED' : '❌ DISABLED'}`);
       console.log(`   Rate Lim: ${body.rate_limit?.enabled ? '✅ ENABLED' : '❌ DISABLED'}`);
```

### B. `tests/security-enforcement/emergency-reset.spec.ts`

```diff
--- a/tests/security-enforcement/emergency-reset.spec.ts
+++ b/tests/security-enforcement/emergency-reset.spec.ts
@@ -25,7 +25,17 @@ test.describe('Emergency Security Reset (Break-Glass)', () => {
     expect(response.ok()).toBeTruthy();
     const body = await response.json();
     expect(body.success).toBe(true);
+
+    // Verify individual security modules are disabled
     expect(body.disabled_modules).toContain('security.acl.enabled');
-    expect(body.disabled_modules).toContain('feature.cerberus.enabled');
+    expect(body.disabled_modules).toContain('security.waf.enabled');
+    expect(body.disabled_modules).toContain('security.rate_limit.enabled');
+    expect(body.disabled_modules).toContain('security.crowdsec.enabled');
+    expect(body.disabled_modules).toContain('security.crowdsec.mode');
+
+    // NOTE: feature.cerberus.enabled is NOT disabled by emergency reset
+    // The Cerberus framework stays enabled to allow security module management
+    // Only enforcement modules (ACL, WAF, Rate Limit, CrowdSec) are disabled
+    expect(body.disabled_modules).not.toContain('feature.cerberus.enabled');
   });
```

### C. `tests/security-teardown.setup.ts`

```diff
--- a/tests/security-teardown.setup.ts
+++ b/tests/security-teardown.setup.ts
@@ -19,6 +19,7 @@
  */

 import { test as teardown } from '@bgotink/playwright-coverage';
 import { request } from '@playwright/test';
+import { STORAGE_STATE } from './constants';

 teardown('verify-security-state-for-ui-tests', async () => {
@@ -31,7 +32,7 @@ teardown('verify-security-state-for-ui-tests', async () => {
   // Create authenticated request context with storage state
   const requestContext = await request.newContext({
     baseURL,
-    storageState: 'playwright/.auth/admin.json',
+    storageState: STORAGE_STATE,
   });

   let allChecksPass = true;

   try {
-    // Verify Cerberus framework is enabled
-    const cerberusResponse = await requestContext.get(`${baseURL}/api/v1/security/config`);
-    if (cerberusResponse.ok()) {
-      const config = await cerberusResponse.json();
-      if (config.enabled === true) {
+    // Verify Cerberus framework is enabled via status endpoint
+    const statusResponse = await requestContext.get(`${baseURL}/api/v1/security/status`);
+    if (statusResponse.ok()) {
+      const status = await statusResponse.json();
+      if (status.cerberus.enabled === true) {
         console.log('✅ Cerberus framework: ENABLED');
       } else {
         console.log('⚠️  Cerberus framework: DISABLED (expected: ENABLED)');
         allChecksPass = false;
       }

-      if (config.admin_whitelist === '0.0.0.0/0') {
-        console.log('✅ Admin whitelist: 0.0.0.0/0 (universal bypass)');
-      } else {
-        console.log(`⚠️  Admin whitelist: ${config.admin_whitelist || 'none'} (expected: 0.0.0.0/0)`);
+      // Verify security modules status
+      console.log(`   ACL module:         ${status.acl?.enabled ? '✅ ENABLED' : '⚠️  disabled'}`);
+      console.log(`   WAF module:         ${status.waf?.enabled ? '✅ ENABLED' : '⚠️  disabled'}`);
+      console.log(`   Rate Limit module:  ${status.rate_limit?.enabled ? '✅ ENABLED' : '⚠️  disabled'}`);
+      console.log(`   CrowdSec module:    ${status.crowdsec?.running ? '✅ RUNNING' : '⚠️  not available (OK for E2E)'}`);
+
+      // ACL, WAF, and Rate Limit should be enabled
+      if (!status.acl?.enabled || !status.waf?.enabled || !status.rate_limit?.enabled) {
+        console.log('⚠️  Some security modules are disabled (expected: all enabled)');
         allChecksPass = false;
       }
     } else {
-      console.log('⚠️  Could not verify Cerberus configuration');
+      console.log('⚠️  Could not verify security module status');
       allChecksPass = false;
     }

-    // Verify security modules status
-    const statusResponse = await requestContext.get(`${baseURL}/api/v1/security/status`);
-    if (statusResponse.ok()) {
-      const status = await statusResponse.json();
-
-      console.log(`   ACL module:         ${status.acl?.enabled ? '✅ ENABLED' : '⚠️  disabled'}`);
-      console.log(`   WAF module:         ${status.waf?.enabled ? '✅ ENABLED' : '⚠️  disabled'}`);
-      console.log(`   Rate Limit module:  ${status.rate_limit?.enabled ? '✅ ENABLED' : '⚠️  disabled'}`);
-      console.log(`   CrowdSec module:    ${status.crowdsec?.running ? '✅ RUNNING' : '⚠️  not available (OK for E2E)'}`);
-
-      // ACL, WAF, and Rate Limit should be enabled
-      if (!status.acl?.enabled || !status.waf?.enabled || !status.rate_limit?.enabled) {
-        console.log('⚠️  Some security modules are disabled (expected: all enabled)');
+    // Verify admin whitelist via config endpoint
+    const configResponse = await requestContext.get(`${baseURL}/api/v1/security/config`);
+    if (configResponse.ok()) {
+      const configData = await configResponse.json();
+      if (configData.config?.admin_whitelist === '0.0.0.0/0') {
+        console.log('✅ Admin whitelist: 0.0.0.0/0 (universal bypass)');
+      } else {
+        console.log(`⚠️  Admin whitelist: ${configData.config?.admin_whitelist || 'none'} (expected: 0.0.0.0/0)`);
         allChecksPass = false;
       }
     } else {
-      console.log('⚠️  Could not verify security module status');
+      console.log('⚠️  Could not verify admin whitelist configuration');
       allChecksPass = false;
     }
```

---

**End of Specification**
