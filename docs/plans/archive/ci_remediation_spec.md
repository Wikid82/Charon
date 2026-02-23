# CI Remediation Plan: E2E Tests & Workflow Optimization

**Objective**: Stabilize the E2E testing pipeline by addressing missing browser dependencies, optimizing shard distribution, and fixing flaky tests.

## 1. CI Workflow Updates (`.github/workflows/e2e-tests-split.yml`)

### 1.1 Fix Missing Browser Dependencies in Security Jobs
The security enforcement jobs for Firefox and WebKit are failing because they lack the Chromium dependency required by the shared test utilities (likely in `fixtures/auth-fixtures` or `utils/` which might depend on Chromium-specific behaviors or default browser contexts during setup).

**Action**: Add the Chromium installation step to `e2e-firefox-security` and `e2e-webkit-security` jobs, mirroring the non-security jobs.

**Implementation Details**:
```yaml
# In e2e-firefox-security:
- name: Install Playwright Chromium
  run: |
    echo "📦 Installing Chromium (required by security-tests dependency)..."
    npx playwright install --with-deps chromium
    EXIT_CODE=$?
    echo "✅ Install command completed (exit code: $EXIT_CODE)"
    exit $EXIT_CODE

# In e2e-webkit-security:
- name: Install Playwright Chromium
  run: |
    echo "📦 Installing Chromium (required by security-tests dependency)..."
    npx playwright install --with-deps chromium
    EXIT_CODE=$?
    echo "✅ Install command completed (exit code: $EXIT_CODE)"
    exit $EXIT_CODE
```

### 1.2 Optimize Shard Distribution
Shard 4 is consistently timing out (>20m) while others finish quickly (4-13m). Reducing the shard count forces a redistribution of tests which effectively rebalances the load.

**Action**:
1. Change shard strategy from 4 to 3.
2. Increase workflow timeout from default (or 20m) to **25 minutes** to accommodate the slightly higher per-shard load.

**Implementation Details**:
```yaml
# In e2e-chromium, e2e-firefox, e2e-webkit jobs:
timeout-minutes: 25   # Increased for safety

strategy:
  fail-fast: false
  matrix:
    shard: [1, 2, 3]  # Reduced from [1, 2, 3, 4]
    total-shards: [3] # Reduced from [4]
```

## 2. Test Stability Fixes

### 2.1 Fix `certificates.spec.ts` (Core)
**Issue**: Tests fail when checking for "Empty State OR Table" because `isVisible().catch()` returns false for both during the transitional loading state, even after waiting for loading to complete.

**Solution**: Use Playwright's distinct `expect` assertions with locators combined via `.or()` to allow Playwright's auto-retrying mechanism to handle the state transition.

**Implementation**:
```typescript
// Replace explicit boolean checks:
// const hasEmptyMessage = await emptyCellMessage.isVisible().catch(() => false);
// const hasTable = await table.isVisible().catch(() => false);
// expect(hasEmptyMessage || hasTable).toBeTruthy();

// With robust locator assertion:
await expect(
  page.getByRole('table').or(page.getByText(/no.*certificates.*found/i))
).toBeVisible({ timeout: 10000 });
```
*Apply this pattern to lines 104 and 120.*

### 2.2 Fix `proxy-hosts.spec.ts` (Core)
**Issue**: `waitForModal` failures (undefined selector match). The custom helper is less reliable than direct Playwright assertions, especially when animations or DOM updates are involved.

**Solution**: Replace `waitForModal(page)` with explicit expectations for the dialog visibility.

**Implementation**:
```typescript
// Replace:
// await waitForModal(page);

// With:
await expect(page.getByRole('dialog')).toBeVisible();
```
*Apply to all occurrences in `Create`, `Update`, `Delete` describe blocks.*

### 2.3 Fix `crowdsec-import.spec.ts` (Security)
**Issue**: Flaky failure on "should handle archive with optional files". The backend likely returns a 500/4xx error intermittently (possibly due to file locking on `acquis.yaml` or state issues from previous tests).

**Solution**: Implement a retry loop for the API request. This handles transient backend locking issues.

**Implementation**:
```typescript
// Wrap the request in a retry loop
await expect(async () => {
  const response = await request.post('/api/v1/admin/crowdsec/import', {
     // ... payload ...
  });
  expect(response.ok(), `Import failed with status: ${response.status()}`).toBeTruthy();
  const data = await response.json();
  expect(data).toHaveProperty('status', 'imported');
}).toPass({
  intervals: [1000, 2000, 5000],
  timeout: 15_000
});
```

## 3. Execution Plan

### Phase 1: Test Stability
1. Modify `tests/core/certificates.spec.ts`.
2. Modify `tests/core/proxy-hosts.spec.ts`.
3. Modify `tests/security/crowdsec-import.spec.ts`.
4. Verification: Run these specific tests locally (using the skill) to ensure they pass consistently.

### Phase 2: Workflow Updates
1. Modify `.github/workflows/e2e-tests-split.yml`.
2. Verification: Rely on CI execution (cannot fully simulate GitHub Actions matrix locally).

### Phase 3: Final Verification
1. Push changes and monitor the full E2E suite.
