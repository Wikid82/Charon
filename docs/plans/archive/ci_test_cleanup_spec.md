# CI/CD Test Remix & Stabilization Plan

**Status**: Draft
**Owner**: DevOps / QA
**Context**: Fixing flaky E2E tests in `proxy-hosts.spec.ts` identified in CI Remediation Report.

## 1. Problem Analysis

### Symptoms
1. **"Add Proxy Host" Modal Failure**: Test clicks "Add Proxy Host" but dialog doesn't appear.
2. **Empty State Detection Failure**: Test asserts "Empty State OR Table" visible, but fails (neither visible).
3. **Spinner Timeouts**: Loading state tests are flaky.

### Root Cause
**Mismatched Loading Indicators**:
- The test helper `waitForLoadingComplete` waits for `.animate-spin` (loading spinner).
- The `ProxyHosts` page uses `SkeletonTable` (pulse animation) for its initial loading state.
- **Result**: `waitForLoadingComplete` returns immediately because no spinner is found. The test proceeds while the Skeleton is still visible.
- **Impact**:
    - **Empty State Test**: Fails because checking for EmptyState/Table happens while Skeleton is still rendered.
    - **Add Host Test**: The click might verify, but the page is currently rendering/hydrating/transitioning, causing flaky behavior or race conditions.

## 2. Remediation Specification

### Objective
Make `proxy-hosts.spec.ts` robust by accurately detecting the page's "ready" state and using precise selectors.

### Tasks

#### Phase 1: Selector Hardening
- **Target specific "Add" button**: Use `data-testid` or precise hierarchy to distinguish the Header button from the Empty State button (though logic allows either, precision helps debugging).
- **Consolidate Button Interaction**: Ensure we are waiting for the button to be interactive.

#### Phase 2: Loading State Logic Update
- **Detect Skeleton**: Add logic to wait for `SkeletonTable` (or `.animate-pulse`, `.skeleton`) to disappear.
- **Update Test Flow**:
    - `beforeEach`: Wait for Table OR Empty State to be visible (implies Skeleton is gone).
    - `should show loading skeleton`: Update to assert presence of `role="status"` or `.animate-pulse` selector instead of `.animate-spin`.

#### Phase 3: Empty State Verification
- **Explicit Assertion**: Instead of `catch(() => false)`, use `expect(locator).toBeVisible()` inside a `test.step` that handles the conditional logic gracefully (e.g., using `Promise.race` or checking count before assertion).
- **Wait for transition**: Ensure test waits for the transition from `loading=true` to `loading=false`.

## 3. Implementation Steps

### Step 1: Update `tests/utils/wait-helpers.ts` (Optional)
*Consider adding `waitForSkeletonComplete` if this pattern is common.*
*For now, local handling in `proxy-hosts.spec.ts` is sufficient.*

### Step 2: Rewrite `tests/core/proxy-hosts.spec.ts`
Modify `beforeEach` and specific tests:

```typescript
// Proposed Change for beforeEach
test.beforeEach(async ({ page, adminUser }) => {
  await loginUser(page, adminUser);
  await page.goto('/proxy-hosts');

  // Wait for REAL content availability, bypassing Skeleton
  const table = page.getByRole('table');
  const emptyState = page.getByRole('heading', { name: 'No proxy hosts' });
  const addHostBtn = page.getByRole('button', { name: 'Add Proxy Host' }).first();

  // Wait for either table OR empty state to be visible
  await expect(async () => {
      const tableVisible = await table.isVisible();
      const emptyVisible = await emptyState.isVisible();
      expect(tableVisible || emptyVisible).toBeTruthy();
  }).toPass({ timeout: 10000 });

  await expect(addHostBtn).toBeVisible();
});
```

### Step 3: Fix "Loading Skeleton" Test
Target the actual Skeleton element:
```typescript
test('should show loading skeleton while fetching data', async ({ page }) => {
    await page.reload();
    // Verify Skeleton exists
    const skeleton = page.locator('.animate-pulse'); // or specific skeleton selector
    await expect(skeleton.first()).toBeVisible();

    // Then verify it disappears
    await expect(skeleton.first()).not.toBeVisible();
});
```

## 4. Verification
1. Run `npx playwright test tests/core/proxy-hosts.spec.ts --project=chromium`
2. Ensure 0% flake rate.
