# Phase 1: Skipped Playwright Tests Remediation - Implementation Plan

**Date:** January 21, 2026
**Status:** Ready for Implementation
**Priority:** P0 - Quick Wins
**Target:** Enable 40+ skipped tests with minimal effort
**Estimated Effort:** 2-4 hours

---

## Executive Summary

This plan addresses the first phase of the skipped Playwright tests remediation. Phase 1 focuses on "quick wins" that can enable 40+ tests with minimal code changes. The primary fix involves enabling Cerberus in the E2E test environment, which alone will restore 35+ tests.

### Phase 1 Targets

| Fix | Tests Enabled | Effort | Files Modified |
|-----|---------------|--------|----------------|
| Enable Cerberus in E2E environment | 35 | 5 min | 2 |
| Fix checkbox toggle wait in account-settings | 1 | 10 min | 1 |
| Fix language selector test in system-settings | 1 | 10 min | 1 |
| Stabilize keyboard navigation tests | 3 | 30 min | 2 |
| **Total** | **40** | **~1 hour** | **6** |

---

## 1. Enable Cerberus in E2E Environment (+35 tests)

### 1.1 Root Cause Analysis

**Problem:** The Cerberus security module is disabled in E2E test environments via `FEATURE_CERBERUS_ENABLED=false`. Tests check this flag at runtime and skip when false.

**Evidence:**

1. **docker-compose.playwright.yml** (line 54):
   ```yaml
   - FEATURE_CERBERUS_ENABLED=false
   ```

2. **docker-compose.e2e.yml** (line 33):
   ```yaml
   - FEATURE_CERBERUS_ENABLED=false
   ```

3. **Test Skip Pattern** in [tests/monitoring/real-time-logs.spec.ts](../../tests/monitoring/real-time-logs.spec.ts#L222-L238):
   ```typescript
   let cerberusEnabled = false;
   // ...
   const connectionStatus = page.locator('[data-testid="connection-status"]');
   cerberusEnabled = await connectionStatus.isVisible({ timeout: 3000 }).catch(() => false);
   // ...
   test.skip(!cerberusEnabled, 'LiveLogViewer not available - Cerberus security module is disabled');
   ```

**Affected Test Files:**
- [tests/monitoring/real-time-logs.spec.ts](../../tests/monitoring/real-time-logs.spec.ts) - 25 tests
- [tests/security/security-dashboard.spec.ts](../../tests/security/security-dashboard.spec.ts) - 7 tests
- [tests/security/rate-limiting.spec.ts](../../tests/security/rate-limiting.spec.ts) - 2+ tests

### 1.2 Implementation

#### Step 1: Update docker-compose.playwright.yml

**File:** `.docker/compose/docker-compose.playwright.yml`
**Line:** 54
**Change:**

```diff
      # Security features - disabled by default for faster tests
      # Enable via profile: --profile security-tests
-     - FEATURE_CERBERUS_ENABLED=false
+     - FEATURE_CERBERUS_ENABLED=true
      - CHARON_SECURITY_CROWDSEC_MODE=disabled
```

**Rationale:** The Playwright compose file is used for E2E testing. Enabling Cerberus allows all security-related tests to run. CrowdSec mode can remain disabled (it has separate tests with the `security-tests` profile).

#### Step 2: Update docker-compose.e2e.yml

**File:** `.docker/compose/docker-compose.e2e.yml`
**Line:** 33
**Change:**

```diff
      - CHARON_ACME_STAGING=true
-     - FEATURE_CERBERUS_ENABLED=false
+     - FEATURE_CERBERUS_ENABLED=true
    volumes:
```

**Rationale:** This is the legacy E2E compose file. Both files should have consistent configuration.

### 1.3 Verification

After making the changes:

```bash
# Rebuild E2E environment
docker compose -f .docker/compose/docker-compose.playwright.yml down
docker compose -f .docker/compose/docker-compose.playwright.yml up -d --build

# Wait for healthy
sleep 10

# Verify Cerberus is enabled
curl -s http://localhost:8080/api/v1/feature-flags | jq '."feature.cerberus.enabled"'
# Expected output: true

# Run affected tests
npx playwright test tests/monitoring/real-time-logs.spec.ts --project=chromium
npx playwright test tests/security/security-dashboard.spec.ts --project=chromium
```

**Expected Result:** 35+ tests that were previously skipped should now execute.

---

## 2. Fix Checkbox Toggle Wait in Account Settings (+1 test)

### 2.1 Root Cause Analysis

**Problem:** The checkbox toggle behavior in the account settings page is inconsistent. The test clicks the checkbox but the state change is not reliably detected.

**Location:** [tests/settings/account-settings.spec.ts](../../tests/settings/account-settings.spec.ts#L259-L275)

**Current Skip Reason (line 258):**
```typescript
/**
 * Test: Enter custom certificate email
 * Note: Skip - checkbox toggle behavior inconsistent; may need double-click or wait
 */
test.skip('should enter custom certificate email', async ({ page }) => {
```

**Root Issue:** The checkbox is a Radix UI component that uses custom rendering. Direct `.click()` may not reliably toggle the underlying input state. The working test at lines 200-250 uses:
```typescript
const checkbox = page.getByRole('checkbox', { name: /use.*account.*email|same.*email/i });
await checkbox.click({ force: true });
await page.waitForTimeout(100);
```

### 2.2 Implementation

**File:** `tests/settings/account-settings.spec.ts`
**Lines:** 259-275
**Change:**

```diff
    /**
     * Test: Enter custom certificate email
-    * Note: Skip - checkbox toggle behavior inconsistent; may need double-click or wait
     */
-   test.skip('should enter custom certificate email', async ({ page }) => {
+   test('should enter custom certificate email', async ({ page }) => {
      const customEmail = `cert-${Date.now()}@custom.local`;

      await test.step('Uncheck use account email', async () => {
-       const checkbox = page.locator('#useUserEmail');
-       await checkbox.click();
-       await expect(checkbox).not.toBeChecked();
+       // Use getByRole for Radix UI checkbox with force click
+       const checkbox = page.getByRole('checkbox', { name: /use.*account.*email|same.*email/i });
+
+       // First check if already unchecked
+       const isChecked = await checkbox.isChecked();
+       if (isChecked) {
+         await checkbox.click({ force: true });
+         // Wait for state transition
+         await page.waitForTimeout(100);
+       }
+       await expect(checkbox).not.toBeChecked({ timeout: 5000 });
      });

      await test.step('Enter custom email', async () => {
        const certEmailInput = page.locator('#cert-email');
-       await expect(certEmailInput).toBeVisible();
+       await expect(certEmailInput).toBeVisible({ timeout: 5000 });
        await certEmailInput.clear();
        await certEmailInput.fill(customEmail);
        await expect(certEmailInput).toHaveValue(customEmail);
      });
    });
```

### 2.3 Verification

```bash
npx playwright test tests/settings/account-settings.spec.ts \
  --grep "should enter custom certificate email" \
  --project=chromium
```

**Expected Result:** Test passes consistently.

---

## 3. Fix Language Selector Test in System Settings (+1 test)

### 3.1 Root Cause Analysis

**Problem:** The language selector test conditionally skips when it can't find the selector. The selector pattern is too broad and may not match the actual component.

**Location:** [tests/settings/system-settings.spec.ts](../../tests/settings/system-settings.spec.ts#L373-L388)

**Current Code:**
```typescript
await test.step('Find language selector', async () => {
  // Language selector may be a custom component
  const languageSelector = page
    .getByRole('combobox', { name: /language/i })
    .or(page.locator('[id*="language"]'))
    .or(page.getByText(/language/i).locator('..').locator('select, [role="combobox"]'));

  const hasLanguageSelector = await languageSelector.first().isVisible({ timeout: 3000 }).catch(() => false);

  if (hasLanguageSelector) {
    await expect(languageSelector.first()).toBeVisible();
  } else {
    // Skip if no language selector found
    test.skip();
  }
});
```

**Actual Component:** Based on [frontend/src/components/LanguageSelector.tsx](../../frontend/src/components/LanguageSelector.tsx):
```tsx
<select
  value={language}
  onChange={handleChange}
  className="bg-surface-elevated border border-border rounded-md..."
>
```

The component is a native `<select>` element without an ID or data-testid attribute.

### 3.2 Implementation

#### Step 1: Add data-testid to LanguageSelector component

**File:** `frontend/src/components/LanguageSelector.tsx`
**Change:**

```diff
  return (
    <div className="flex items-center gap-3">
      <Globe className="h-5 w-5 text-content-secondary" />
      <select
+       data-testid="language-selector"
        value={language}
        onChange={handleChange}
        className="bg-surface-elevated border border-border rounded-md px-3 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all"
      >
```

#### Step 2: Update the test to use the data-testid

**File:** `tests/settings/system-settings.spec.ts`
**Lines:** 373-388
**Change:**

```diff
    await test.step('Find language selector', async () => {
-     // Language selector may be a custom component
-     const languageSelector = page
-       .getByRole('combobox', { name: /language/i })
-       .or(page.locator('[id*="language"]'))
-       .or(page.getByText(/language/i).locator('..').locator('select, [role="combobox"]'));
-
-     const hasLanguageSelector = await languageSelector.first().isVisible({ timeout: 3000 }).catch(() => false);
-
-     if (hasLanguageSelector) {
-       await expect(languageSelector.first()).toBeVisible();
-     } else {
-       // Skip if no language selector found
-       test.skip();
-     }
+     // Language selector is a native select element
+     const languageSelector = page.getByTestId('language-selector');
+     await expect(languageSelector).toBeVisible({ timeout: 5000 });
    });
```

### 3.3 Verification

```bash
# Rebuild frontend after component change
cd frontend && npm run build && cd ..

# Rebuild Docker image
docker compose -f .docker/compose/docker-compose.playwright.yml up -d --build

# Run the test
npx playwright test tests/settings/system-settings.spec.ts \
  --grep "language" \
  --project=chromium
```

**Expected Result:** Test finds the language selector and passes.

---

## 4. Stabilize Keyboard Navigation Tests (+3 tests)

### 4.1 Root Cause Analysis

**Problem:** Keyboard navigation tests are flaky due to timing issues with tab counts and focus detection.

**Affected Tests:**

1. **[tests/settings/account-settings.spec.ts#L675](../../tests/settings/account-settings.spec.ts#L675)**
   ```typescript
   // Skip: Tab navigation order is browser/layout dependent
   test.skip('should be keyboard navigable', async ({ page }) => {
   ```

2. **[tests/settings/user-management.spec.ts#L1000](../../tests/settings/user-management.spec.ts#L1000)**
   ```typescript
   // Skip: Keyboard navigation test is flaky due to timing issues with tab count
   test.skip('should be keyboard navigable', async ({ page }) => {
   ```

3. **[tests/core/navigation.spec.ts#L597](../../tests/core/navigation.spec.ts#L597)**
   ```typescript
   // TODO: Implement skip-to-content link in the application
   test.skip('should have skip to main content link', async ({ page }) => {
   ```

**Root Issues:**
- Tests loop through tab presses looking for specific elements
- Focus order is layout-dependent and may vary
- No explicit waits between key presses
- The skip-to-content test requires an actual skip link implementation (intentional skip)

### 4.2 Implementation

#### Fix 1: Account Settings Keyboard Navigation

**File:** `tests/settings/account-settings.spec.ts`
**Lines:** 670-720
**Change:**

```diff
  test.describe('Accessibility', () => {
    /**
     * Test: Keyboard navigation through account settings
-    * Note: Skip - Tab navigation order is browser/layout dependent
     */
-   test.skip('should be keyboard navigable', async ({ page }) => {
+   test('should be keyboard navigable', async ({ page }) => {
      await test.step('Tab through profile section', async () => {
        // Start from first focusable element
        await page.keyboard.press('Tab');
+       await page.waitForTimeout(50); // Brief pause for focus to settle

        // Tab to profile name
        const nameInput = page.locator('#profile-name');
        let foundName = false;

-       for (let i = 0; i < 15; i++) {
+       for (let i = 0; i < 20; i++) {
          if (await nameInput.evaluate((el) => el === document.activeElement)) {
            foundName = true;
            break;
          }
          await page.keyboard.press('Tab');
+         await page.waitForTimeout(50); // Allow focus to update
        }

        expect(foundName).toBeTruthy();
      });

      await test.step('Tab through password section', async () => {
        const currentPasswordInput = page.locator('#current-password');
        let foundPassword = false;

-       for (let i = 0; i < 20; i++) {
+       for (let i = 0; i < 25; i++) {
          if (await currentPasswordInput.evaluate((el) => el === document.activeElement)) {
            foundPassword = true;
            break;
          }
          await page.keyboard.press('Tab');
+         await page.waitForTimeout(50);
        }

        expect(foundPassword).toBeTruthy();
      });
```

#### Fix 2: User Management Keyboard Navigation

**File:** `tests/settings/user-management.spec.ts`
**Lines:** 995-1060
**Change:**

```diff
    /**
     * Test: Keyboard navigation
     * Priority: P1
     */
-   // Skip: Keyboard navigation test is flaky due to timing issues with tab count
-   test.skip('should be keyboard navigable', async ({ page }) => {
+   test('should be keyboard navigable', async ({ page }) => {
      await test.step('Tab to invite button', async () => {
        await page.keyboard.press('Tab');
+       await page.waitForTimeout(50);

        let foundInviteButton = false;
-       for (let i = 0; i < 10; i++) {
+       for (let i = 0; i < 15; i++) {
          const focused = page.locator(':focus');
          const text = await focused.textContent().catch(() => '');

          if (text?.toLowerCase().includes('invite')) {
            foundInviteButton = true;
            break;
          }
          await page.keyboard.press('Tab');
+         await page.waitForTimeout(50);
        }

        expect(foundInviteButton).toBeTruthy();
      });

      await test.step('Activate with Enter key', async () => {
        await page.keyboard.press('Enter');
+       await page.waitForTimeout(200); // Wait for modal animation

        // Modal should open
        const modal = page.locator('[class*="fixed"]').filter({
          has: page.getByRole('heading', { name: /invite/i }),
        });
-       await expect(modal).toBeVisible();
+       await expect(modal).toBeVisible({ timeout: 5000 });
      });

      await test.step('Close modal with Escape', async () => {
        await page.keyboard.press('Escape');
+       await page.waitForTimeout(200); // Wait for modal close animation

        // Modal should close (if escape is wired up)
        const closeButton = page.getByRole('button', { name: /close|×|cancel/i });
        if (await closeButton.isVisible()) {
          await closeButton.click();
        }
      });

      await test.step('Tab through table rows', async () => {
        // Focus should be able to reach action buttons in table
        let foundActionButton = false;

-       for (let i = 0; i < 20; i++) {
+       for (let i = 0; i < 30; i++) {
          await page.keyboard.press('Tab');
+         await page.waitForTimeout(50);
          const focused = page.locator(':focus');
          const tagName = await focused.evaluate((el) => el.tagName.toLowerCase()).catch(() => '');

          if (tagName === 'button') {
            const isInTable = await focused.evaluate((el) => {
              return !!el.closest('table');
            }).catch(() => false);

            if (isInTable) {
              foundActionButton = true;
              break;
```

#### Note on Skip-to-Content Test

The test at [tests/core/navigation.spec.ts#L597](../../tests/core/navigation.spec.ts#L597) requires implementing an actual skip-to-content link in the application. This is an **intentional skip** and should remain skipped until the application feature is implemented.

**This is a Phase 2+ task** - requires frontend development to add the skip link component.

### 4.3 Verification

```bash
# Run account settings keyboard test
npx playwright test tests/settings/account-settings.spec.ts \
  --grep "keyboard navigable" \
  --project=chromium

# Run user management keyboard test
npx playwright test tests/settings/user-management.spec.ts \
  --grep "keyboard navigable" \
  --project=chromium
```

**Expected Result:** Both keyboard navigation tests pass consistently.

---

## Implementation Order

Execute changes in this order to avoid build failures:

### Step 1: Frontend Component Change
1. Add `data-testid="language-selector"` to `frontend/src/components/LanguageSelector.tsx`
2. Rebuild frontend: `cd frontend && npm run build`

### Step 2: Docker Configuration Changes
1. Update `.docker/compose/docker-compose.playwright.yml` - set `FEATURE_CERBERUS_ENABLED=true`
2. Update `.docker/compose/docker-compose.e2e.yml` - set `FEATURE_CERBERUS_ENABLED=true`
3. Rebuild: `docker compose -f .docker/compose/docker-compose.playwright.yml up -d --build`

### Step 3: Test File Updates
1. Update `tests/settings/account-settings.spec.ts`:
   - Fix checkbox toggle test (lines 259-275)
   - Fix keyboard navigation test (lines 670-720)
2. Update `tests/settings/system-settings.spec.ts`:
   - Fix language selector test (lines 373-388)
3. Update `tests/settings/user-management.spec.ts`:
   - Fix keyboard navigation test (lines 995-1060)

### Step 4: Verification
```bash
# Run full E2E test suite to verify
npx playwright test --project=chromium

# Or run specific affected files
npx playwright test \
  tests/monitoring/real-time-logs.spec.ts \
  tests/security/security-dashboard.spec.ts \
  tests/security/rate-limiting.spec.ts \
  tests/settings/account-settings.spec.ts \
  tests/settings/system-settings.spec.ts \
  tests/settings/user-management.spec.ts \
  --project=chromium
```

---

## Files to Modify Summary

| File | Type | Changes |
|------|------|---------|
| `.docker/compose/docker-compose.playwright.yml` | Config | Line 54: `FEATURE_CERBERUS_ENABLED=true` |
| `.docker/compose/docker-compose.e2e.yml` | Config | Line 33: `FEATURE_CERBERUS_ENABLED=true` |
| `frontend/src/components/LanguageSelector.tsx` | React | Add `data-testid="language-selector"` |
| `tests/settings/account-settings.spec.ts` | Test | Lines 259-275, 670-720: Fix skipped tests |
| `tests/settings/system-settings.spec.ts` | Test | Lines 373-388: Fix selector pattern |
| `tests/settings/user-management.spec.ts` | Test | Lines 995-1060: Fix keyboard navigation |

---

## Success Metrics

| Metric | Before | After | Target |
|--------|--------|-------|--------|
| Skipped Tests (Total) | 98 | ~58 | <60 |
| Cerberus Tests Running | 0 | 35 | 35 |
| Account Settings Skips | 3 | 1* | 1* |
| System Settings Skips | 4 | 3 | 3 |
| User Management Skips | 22 | 21 | 21 |

*Note: Some skips are intentional (e.g., skip-to-content link not implemented)

---

## Rollback Plan

If issues occur, revert these changes:

```bash
# Revert Docker configs
git checkout .docker/compose/docker-compose.playwright.yml
git checkout .docker/compose/docker-compose.e2e.yml

# Revert frontend component
git checkout frontend/src/components/LanguageSelector.tsx

# Revert test files
git checkout tests/settings/account-settings.spec.ts
git checkout tests/settings/system-settings.spec.ts
git checkout tests/settings/user-management.spec.ts

# Rebuild
docker compose -f .docker/compose/docker-compose.playwright.yml up -d --build
```

---

## Next Phase Preview

After Phase 1 completion, Phase 2 will address:

1. **TestDataManager Authentication Fix** (+8 tests)
   - Refactor to use authenticated API context
   - Update auth-fixtures.ts

2. **SMTP Persistence Backend Fix** (+3 tests)
   - Investigate `/api/v1/settings/smtp` endpoint

3. **Import Route Implementation** (+6 tests)
   - Implement NPM/JSON import handlers

---

## Change Log

| Date | Author | Change |
|------|--------|--------|
| 2026-01-21 | Planning Agent | Initial Phase 1 implementation plan |
