# Phase 2: TestDataManager Authentication Fix Implementation Plan

**Goal**: Fix TestDataManager to use authenticated API context, enabling 8 skipped tests in user management.

**Target**: Enable all user management E2E tests that are currently skipped due to TestDataManager using unauthenticated API calls.

**Estimated Effort**: 2-3 hours

---

## Executive Summary

The `testData` fixture in Playwright tests creates a `TestDataManager` instance using an **unauthenticated API context**. This causes all API calls made by `TestDataManager` (like `createUser()`, `deleteUser()`) to fail with "Admin access required" errors. This plan details how to fix the fixture to use authenticated API requests by leveraging the stored authentication state from `playwright/.auth/user.json`.

---

## Table of Contents

1. [Root Cause Analysis](#1-root-cause-analysis)
2. [Proposed Solution](#2-proposed-solution)
3. [Implementation Details](#3-implementation-details)
4. [Tests to Re-Enable](#4-tests-to-re-enable)
5. [Verification Steps](#5-verification-steps)
6. [Dependencies & Prerequisites](#6-dependencies--prerequisites)
7. [Risks & Mitigations](#7-risks--mitigations)
8. [Implementation Checklist](#8-implementation-checklist)

---

## Supervisor Review: APPROVED ✅

**Reviewed by**: Supervisor Agent (Senior Advisor)
**Date**: 2026-01-22
**Verdict**: Plan is ready for implementation

### Review Summary

| Criterion | Status | Notes |
|-----------|--------|-------|
| Completeness | ✅ Pass | All changes documented |
| Technical Accuracy | ✅ Pass | Correct Playwright pattern |
| Risk Assessment | ✅ Pass | Adequate with fallbacks |
| Dependencies | ✅ Pass | All verified |
| Verification | ✅ Pass | Comprehensive |
| Edge Cases | 🔸 Minor | Added defensive file existence check |

### Incorporated Recommendations

1. **Defensive `existsSync()` check**: Added to implementation to verify storage state file exists before use
2. **Import verification**: Clarified import strategy for `playwrightRequest`
3. **Dependent fixture verification**: Added to checklist to verify `adminUser`/`regularUser`/`guestUser` fixtures

---

## 1. Root Cause Analysis

### Current Problem

The `testData` fixture in `auth-fixtures.ts` creates a `TestDataManager` instance using the Playwright `request` fixture:

```typescript
// tests/fixtures/auth-fixtures.ts (lines 69-75)
testData: async ({ request }, use, testInfo) => {
  const manager = new TestDataManager(request, testInfo.title);
  await use(manager);
  await manager.cleanup();
},
```

The issue: **Playwright's `request` fixture creates an unauthenticated API context**. It does NOT use the `storageState` from `playwright.config.js` that the browser context uses.

When `TestDataManager.createUser()` is called:
1. It posts to `/api/v1/users` using the unauthenticated `request` context
2. The backend requires admin authentication for user creation
3. The request fails with 401/403 "Admin access required"

### Evidence

From [user-management.spec.ts](../../tests/settings/user-management.spec.ts#L535-L538):
```typescript
// SKIP: testData.createUser() uses unauthenticated API calls
// The TestDataManager's request context doesn't inherit auth from the browser session
// This causes user creation and cleanup to fail with "Admin access required"
// TODO: Fix by making TestDataManager use authenticated API requests
```

---

## 2. Proposed Solution

### Approach: Create Authenticated APIRequestContext from Storage State

Modify the `testData` fixture to create an authenticated `APIRequestContext` using the stored authentication state from `playwright/.auth/user.json`.

### Key Changes

| File | Change Type | Description |
|------|-------------|-------------|
| [tests/fixtures/auth-fixtures.ts](../../tests/fixtures/auth-fixtures.ts) | Modify | Update `testData` fixture to use authenticated API context |
| [tests/auth.setup.ts](../../tests/auth.setup.ts) | Reference only | Storage state path already exported |

---

## 3. Implementation Details

### 3.1 File: `tests/fixtures/auth-fixtures.ts`

#### Current Implementation (Lines 69-75)

```typescript
/**
 * TestDataManager fixture with automatic cleanup
 * Creates a unique namespace per test and cleans up all resources after
 */
testData: async ({ request }, use, testInfo) => {
  const manager = new TestDataManager(request, testInfo.title);
  await use(manager);
  await manager.cleanup();
},
```

#### Proposed Implementation

```typescript
// Import playwrightRequest directly from @playwright/test (not from coverage wrapper)
// @bgotink/playwright-coverage doesn't re-export request.newContext()
import { request as playwrightRequest } from '@playwright/test';
import { existsSync } from 'fs';
import { STORAGE_STATE } from '../auth.setup';

// ... existing code ...

/**
 * TestDataManager fixture with automatic cleanup
 *
 * FIXED: Now creates an authenticated API context using stored auth state.
 * This ensures API calls (like createUser, deleteUser) inherit the admin
 * session established by auth.setup.ts.
 *
 * Previous Issue: The base `request` fixture was unauthenticated, causing
 * "Admin access required" errors on protected endpoints.
 */
testData: async ({ baseURL }, use, testInfo) => {
  // Defensive check: Verify auth state file exists (created by auth.setup.ts)
  if (!existsSync(STORAGE_STATE)) {
    throw new Error(
      `Auth state file not found at ${STORAGE_STATE}. ` +
      'Ensure auth.setup has run first. Check that dependencies: ["setup"] is configured.'
    );
  }

  // Create an authenticated API request context using stored auth state
  // This inherits the admin session from auth.setup.ts
  const authenticatedContext = await playwrightRequest.newContext({
    baseURL,
    storageState: STORAGE_STATE,
    extraHTTPHeaders: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
    },
  });

  const manager = new TestDataManager(authenticatedContext, testInfo.title);

  try {
    await use(manager);
  } finally {
    // Ensure cleanup runs even if test fails
    await manager.cleanup();
    // Dispose the API context to release resources
    await authenticatedContext.dispose();
  }
},
```

### 3.2 Full Updated Import Section

Add these imports at the top of `auth-fixtures.ts`:

```typescript
import { test as base, expect } from '@bgotink/playwright-coverage';
import { request as playwrightRequest } from '@playwright/test';
import { existsSync } from 'fs';
import { TestDataManager } from '../utils/TestDataManager';
import { STORAGE_STATE } from '../auth.setup';
```

**Note**: `playwrightRequest` is imported from `@playwright/test` directly because `@bgotink/playwright-coverage` does not re-export the `request` module needed for `request.newContext()`.

### 3.3 Verify STORAGE_STATE Export in auth.setup.ts

The current `auth.setup.ts` already exports `STORAGE_STATE`:

```typescript
// tests/auth.setup.ts (lines 20-22)
const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
export const STORAGE_STATE = join(__dirname, '../playwright/.auth/user.json');
```

**No changes needed to this file.**

---

## 4. Tests to Re-Enable

After implementing this fix, the following 8 tests in [tests/settings/user-management.spec.ts](../../tests/settings/user-management.spec.ts) should be re-enabled by removing `test.skip()`:

| # | Test Name | File:Line | Current Skip Reason |
|---|-----------|-----------|---------------------|
| 1 | should open permissions modal | [user-management.spec.ts:496](../../tests/settings/user-management.spec.ts#L496) | Permissions button + testData auth |
| 2 | should update permission mode | [user-management.spec.ts:534](../../tests/settings/user-management.spec.ts#L534) | `testData.createUser()` uses unauthenticated API calls |
| 3 | should add permitted hosts | [user-management.spec.ts:609](../../tests/settings/user-management.spec.ts#L609) | Depends on settings button + testData auth |
| 4 | should remove permitted hosts | [user-management.spec.ts:665](../../tests/settings/user-management.spec.ts#L665) | Same as above |
| 5 | should save permission changes | [user-management.spec.ts:722](../../tests/settings/user-management.spec.ts#L722) | Same as above |
| 6 | should enable/disable user | [user-management.spec.ts:773](../../tests/settings/user-management.spec.ts#L773) | TestDataManager auth + test data pollution |
| 7 | should delete user with confirmation | [user-management.spec.ts:839](../../tests/settings/user-management.spec.ts#L839) | Delete button + testData auth |
| 8 | should change user role | [user-management.spec.ts:899](../../tests/settings/user-management.spec.ts#L899) | Role badge selector + testData auth |

### Note on Mixed Skip Reasons

Some tests have **dual skip reasons** (e.g., "UI not implemented" + "testData auth issue"). After the auth fix, these tests should be evaluated:

- **If UI is now implemented**: Remove skip entirely
- **If UI is still missing**: Keep skip with updated reason (remove auth-related reason)

---

## 5. Verification Steps

### 5.1 Pre-Implementation Verification

1. **Confirm auth state file exists**:
   ```bash
   ls -la playwright/.auth/user.json
   ```

2. **Verify auth.setup.ts runs before tests**:
   ```bash
   npx playwright test --project=setup --reporter=list
   ```

### 5.2 Implementation Verification

1. **Run a single testData-dependent test**:
   ```bash
   npx playwright test tests/settings/user-management.spec.ts \
     --grep "should update permission mode" \
     --project=chromium \
     --reporter=list
   ```

2. **Verify API calls are authenticated** by adding debug logging:
   ```typescript
   // Temporary debug in TestDataManager.createUser()
   console.log('Creating user with context:', await this.request.storageState());
   ```

3. **Run all user-management tests**:
   ```bash
   npx playwright test tests/settings/user-management.spec.ts \
     --project=chromium \
     --reporter=list
   ```

### 5.3 Post-Implementation Verification

1. **Verify skip count reduction**:
   ```bash
   grep -c "test.skip" tests/settings/user-management.spec.ts
   # Before: ~22 skips
   # After: ~14 skips (8 removed for auth fix)
   ```

2. **Run full E2E suite to check for regressions**:
   ```bash
   npx playwright test --project=chromium
   ```

3. **Verify cleanup works** (no orphaned test users):
   - Check users list in UI after running tests
   - All `test-*` prefixed users should be cleaned up

---

## 6. Dependencies & Prerequisites

### Dependencies

| Dependency | Status | Notes |
|------------|--------|-------|
| `auth.setup.ts` runs first | ✅ Configured | Via `dependencies: ['setup']` in playwright.config.js |
| `STORAGE_STATE` exported | ✅ Already exported | From auth.setup.ts |
| Storage state file created | ✅ Auto-created | By auth.setup.ts on first run |

### Prerequisites

1. **E2E environment running**: Docker containers must be up
2. **Auth setup successful**: `playwright/.auth/user.json` must exist and be valid
3. **Admin user exists**: The setup user must have admin role

---

## 7. Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Storage state file missing/stale | Low | Test failures | Defensive `existsSync()` check added; re-run setup if needed |
| Auth cookie expired mid-test | Low | API 401 errors | Tests are short; setup runs before each run |
| Circular dependency with auth fixtures | Low | Import errors | testData only imports STORAGE_STATE, not auth fixtures |
| Context disposal race condition | Low | Resource leak | Use try/finally pattern (already in proposal) |
| Parallel test isolation | Medium | Test pollution | All tests share admin session; document that `testData` is not parallelism-safe if multiple workers create conflicting resources |

### Fallback Plan

If the storage state approach doesn't work, alternative options:

1. **API Token Approach**: Generate a long-lived API token during setup, pass to TestDataManager
2. **Direct Login in Fixture**: Have testData fixture call login API directly before each test
3. **Shared Admin Session**: Use a dedicated admin user just for TestDataManager operations

---

## 8. Implementation Checklist

### Core Implementation
- [ ] Update `tests/fixtures/auth-fixtures.ts` with authenticated context
- [ ] Add `existsSync` defensive check for storage state file
- [ ] Import `playwrightRequest` from `@playwright/test` (not coverage wrapper)
- [ ] Verify `STORAGE_STATE` import works

### Verification (Supervisor Recommendations)
- [ ] Run single test to verify fix
- [ ] Verify `adminUser`/`regularUser`/`guestUser` dependent fixtures still work
- [ ] Confirm API requests include authentication cookies

### Test Re-enablement
- [ ] Remove `test.skip()` from 8 identified tests
- [ ] Update skip comments in tests that remain skipped (remove auth-related reasons)

### Regression & Documentation
- [ ] Run full user-management test suite
- [ ] Verify no regressions in other test files
- [ ] Update `docs/plans/skipped-tests-remediation.md` with Phase 2 completion status
- [ ] Document any remaining issues

---

## 9. Phase 2 Completion Status

### Implementation Completed ✅

| Item | Status | Notes |
|------|--------|-------|
| Update `auth-fixtures.ts` | ✅ Complete | Authenticated context implemented |
| Add `existsSync` check | ✅ Complete | Defensive file check added |
| Import verification | ✅ Complete | `playwrightRequest` from `@playwright/test` |
| Test re-enablement | 🔸 Partial | 2 tests re-enabled then re-skipped (see blocker) |

### Blocker Discovered: Cookie Domain Mismatch

**Root Cause**: Environment configuration inconsistency prevents authenticated context from working:

1. `playwright.config.js` defaults `baseURL` to `http://localhost:8080`
2. Auth setup creates session cookies for `localhost` domain
3. Tests run against Tailscale IP `http://100.98.12.109:8080`
4. **Cookies aren't sent cross-domain** → API calls remain unauthenticated

**Evidence**:
- Tests pass when run against `localhost:8080`
- Tests fail when run against Tailscale IP due to missing auth cookies

**Fix Required** (separate task):
```bash
# Option 1: Set environment variable consistently
export PLAYWRIGHT_BASE_URL="http://localhost:8080"

# Option 2: Update global-setup.ts default to match playwright.config.js
# tests/global-setup.ts line 8:
-const baseUrl = process.env.PLAYWRIGHT_BASE_URL || 'http://100.98.12.109:8080';
+const baseUrl = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080';
```

### Tests Re-Skipped with Updated Comments

The 2 tests were re-skipped with environment documentation:
- `tests/settings/user-management.spec.ts` - "should update permission mode"
- `tests/settings/user-management.spec.ts` - "should enable/disable user"

---

## 10. Future Improvements

After Phase 2 completion, consider:

1. **Environment configuration fix**: Align `global-setup.ts` and `playwright.config.js` default URLs to prevent cookie domain mismatch.

2. **Per-test authentication contexts**: Currently all tests share the same admin session. For true isolation, each test could create its own authenticated context.

3. **Role-based TestDataManager**: Allow TestDataManager to operate as different roles (admin, user, guest) to test permission boundaries.

4. **Parallel-safe user creation**: The current fix uses a single shared auth session. For highly parallel execution, consider per-worker authentication.

---

## Change Log

| Date | Author | Change |
|------|--------|--------|
| 2026-01-22 | Planning Agent | Initial Phase 2 implementation plan |
| 2026-01-22 | Supervisor Agent | Approved with recommendations: defensive checks, import verification, fixture testing |
| 2026-01-22 | Frontend_Dev | Implemented authenticated context in auth-fixtures.ts |
| 2026-01-22 | QA_Security | Discovered cookie domain mismatch blocker |
| 2026-01-22 | Management | Re-skipped tests, documented blocker for future resolution |
