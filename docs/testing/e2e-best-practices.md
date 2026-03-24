# E2E Testing Best Practices

**Purpose**: Document patterns and anti-patterns discovered during E2E test optimization to prevent future performance regressions and cross-browser failures.

**Target Audience**: Developers writing Playwright E2E tests for Charon.

## Table of Contents

- [Feature Flag Testing](#feature-flag-testing)
- [Cross-Browser Locators](#cross-browser-locators)
- [API Call Optimization](#api-call-optimization)
- [Performance Budget](#performance-budget)
- [Test Isolation](#test-isolation)

---

## Feature Flag Testing

### ❌ AVOID: Polling in beforeEach Hooks

**Anti-Pattern**:

```typescript
test.beforeEach(async ({ page, adminUser }) => {
  await loginUser(page, adminUser);
  await page.goto('/settings/system');

  // ⚠️ PROBLEM: Runs before EVERY test
  await waitForFeatureFlagPropagation(
    page,
    {
      'cerberus.enabled': true,
      'crowdsec.console_enrollment': false,
    },
    { timeout: 10000 } // 10s timeout per test
  );
});
```

**Why This Is Bad**:

- Polls `/api/v1/feature-flags` endpoint **31 times** per test file (once per test)
- With 12 parallel processes (4 shards × 3 browsers), causes API server bottleneck
- Adds 310s minimum execution time per shard (31 tests × 10s timeout)
- Most tests don't modify feature flags, so polling is unnecessary

**Real Impact**: Test shards exceeded 30-minute GitHub Actions timeout limit, blocking CI/CD pipeline.

---

### ✅ PREFER: Per-Test Verification Only When Toggled

**Correct Pattern**:

```typescript
test('should toggle Cerberus feature', async ({ page }) => {
  await test.step('Navigate to system settings', async () => {
    await page.goto('/settings/system');
    await waitForLoadingComplete(page);
  });

  await test.step('Toggle Cerberus feature', async () => {
    const toggle = page.getByRole('switch', { name: /cerberus/i });
    const initialState = await toggle.isChecked();

    await retryAction(async () => {
      const response = await clickSwitchAndWaitForResponse(page, toggle, /\/feature-flags/);
      expect(response.ok()).toBeTruthy();

      // ✅ ONLY verify propagation AFTER toggling
      await waitForFeatureFlagPropagation(page, {
        'cerberus.enabled': !initialState,
      });
    });
  });
});
```

**Why This Is Better**:

- API calls reduced by **90%** (from 31 per shard to 3-5 per shard)
- Only tests that actually toggle flags incur the polling cost
- Faster test execution (shards complete in <15 minutes vs >30 minutes)
- Clearer test intent—verification is tied to the action that requires it

**Rule of Thumb**:

- **No toggle, no propagation check**: If a test reads flag state without changing it, don't poll.
- **Toggle = verify**: Always verify propagation after toggling to ensure state change persisted.

---

## Cross-Browser Locators

### ❌ AVOID: Label-Only Locators

**Anti-Pattern**:

```typescript
await test.step('Verify Script path/command field appears', async () => {
  // ⚠️ PROBLEM: Fails in Firefox/WebKit
  const scriptField = page.getByLabel(/script.*path/i);
  await expect(scriptField).toBeVisible({ timeout: 10000 });
});
```

**Why This Fails**:

- Label locators depend on browser-specific DOM rendering
- Firefox/WebKit may render Label components differently than Chromium
- Regex patterns may not match if label has extra whitespace or is split across nodes
- Results in **70% pass rate** on Firefox/WebKit vs 100% on Chromium

---

### ✅ PREFER: Multi-Strategy Locators with Fallbacks

**Correct Pattern**:

```typescript
import { getFormFieldByLabel } from './utils/ui-helpers';

await test.step('Verify Script path/command field appears', async () => {
  // ✅ Tries multiple strategies until one succeeds
  const scriptField = getFormFieldByLabel(
    page,
    /script.*path/i,
    {
      placeholder: /dns-challenge\.sh/i,
      fieldId: 'field-script_path'
    }
  );
  await expect(scriptField.first()).toBeVisible();
});
```

**Helper Implementation** (`tests/utils/ui-helpers.ts`):

```typescript
/**
 * Get form field with cross-browser label matching
 * Tries multiple strategies: label, placeholder, id, aria-label
 *
 * @param page - Playwright Page object
 * @param labelPattern - Regex or string to match label text
 * @param options - Fallback strategies (placeholder, fieldId)
 * @returns Locator that works across Chromium, Firefox, and WebKit
 */
export function getFormFieldByLabel(
  page: Page,
  labelPattern: string | RegExp,
  options: { placeholder?: string | RegExp; fieldId?: string } = {}
): Locator {
  const baseLocator = page.getByLabel(labelPattern);

  // Build fallback chain
  let locator = baseLocator;

  if (options.placeholder) {
    locator = locator.or(page.getByPlaceholder(options.placeholder));
  }

  if (options.fieldId) {
    locator = locator.or(page.locator(`#${options.fieldId}`));
  }

  // Fallback: role + label text nearby
  if (typeof labelPattern === 'string') {
    locator = locator.or(
      page.getByRole('textbox').filter({
        has: page.locator(`label:has-text("${labelPattern}")`),
      })
    );
  }

  return locator;
}
```

**Why This Is Better**:

- **95%+ pass rate** on Firefox/WebKit (up from 70%)
- Gracefully degrades through fallback strategies
- No browser-specific workarounds needed in test code
- Single helper enforces consistent pattern across all tests

**When to Use**:

- Any test that interacts with form fields
- Tests that must pass on all three browsers (Chromium, Firefox, WebKit)
- Accessibility-critical tests (label locators are user-facing)

---

## API Call Optimization

### ❌ AVOID: Duplicate API Requests

**Anti-Pattern**:

```typescript
// Multiple tests in parallel all polling the same endpoint
test('test 1', async ({ page }) => {
  await waitForFeatureFlagPropagation(page, { flag: true }); // API call
});

test('test 2', async ({ page }) => {
  await waitForFeatureFlagPropagation(page, { flag: true }); // Duplicate API call
});
```

**Why This Is Bad**:

- 12 parallel workers all hit `/api/v1/feature-flags` simultaneously
- No request coalescing or caching
- API server degrades under concurrent load
- Tests timeout due to slow responses

---

### ✅ PREFER: Request Coalescing with Worker Isolation

**Correct Pattern** (`tests/utils/wait-helpers.ts`):

```typescript
// Cache in-flight requests per worker
const inflightRequests = new Map<string, Promise<Record<string, boolean>>>();

function generateCacheKey(
  expectedFlags: Record<string, boolean>,
  workerIndex: number
): string {
  // Sort keys to ensure {a:true, b:false} === {b:false, a:true}
  const sortedFlags = Object.keys(expectedFlags)
    .sort()
    .reduce((acc, key) => {
      acc[key] = expectedFlags[key];
      return acc;
    }, {} as Record<string, boolean>);

  // Include worker index to isolate parallel processes
  return `${workerIndex}:${JSON.stringify(sortedFlags)}`;
}

export async function waitForFeatureFlagPropagation(
  page: Page,
  expectedFlags: Record<string, boolean>,
  options: FeatureFlagPropagationOptions = {}
): Promise<Record<string, boolean>> {
  const workerIndex = test.info().parallelIndex;
  const cacheKey = generateCacheKey(expectedFlags, workerIndex);

  // Return existing promise if already in flight
  if (inflightRequests.has(cacheKey)) {
    console.log(`[CACHE HIT] Worker ${workerIndex}: ${cacheKey}`);
    return inflightRequests.get(cacheKey)!;
  }

  console.log(`[CACHE MISS] Worker ${workerIndex}: ${cacheKey}`);

  // Poll API endpoint (existing logic)...
}
```

**Why This Is Better**:

- **30-40% reduction** in duplicate API calls
- Multiple tests requesting same state share one API call
- Worker isolation prevents cache collisions between parallel processes
- Sorted keys ensure semantic equivalence (`{a:true, b:false}` === `{b:false, a:true}`)

**Cache Behavior**:

- **Hit**: Another test in same worker already polling for same state
- **Miss**: First test in worker to request this state OR different state requested
- **Clear**: Cache cleared after all tests in worker complete (`test.afterAll()`)

---

## Performance Budget

### ❌ PROBLEM: Shards Exceeding Timeout

**Symptom**:

```bash
# GitHub Actions logs
Error: The operation was canceled.
Job duration: 31m 45s (exceeds 30m limit)
```

**Root Causes**:

1. Feature flag polling in beforeEach (31 tests × 10s = 310s minimum)
2. API bottleneck under parallel load
3. Slow browser startup in CI environment
4. Network latency for external resources

---

### ✅ SOLUTION: Enforce 15-Minute Budget Per Shard

**CI Configuration** (`.github/workflows/e2e-tests.yml`):

```yaml
- name: Verify shard performance budget
  if: always()
  run: |
    SHARD_DURATION=$((SHARD_END - SHARD_START))
    MAX_DURATION=900  # 15 minutes = 900 seconds

    if [[ $SHARD_DURATION -gt $MAX_DURATION ]]; then
      echo "::error::Shard exceeded performance budget: ${SHARD_DURATION}s > ${MAX_DURATION}s"
      echo "::error::Investigate slow tests or API bottlenecks"
      exit 1
    fi

    echo "✅ Shard completed within budget: ${SHARD_DURATION}s"
```

**Why This Is Better**:

- **Early detection** of performance regressions in CI
- Forces developers to optimize slow tests before merge
- Prevents accumulation of "death by a thousand cuts" slowdowns
- Clear failure message directs investigation to bottleneck

**How to Debug Timeouts**:

1. **Check metrics**: Review API call counts in test output

   ```bash
   grep "CACHE HIT\|CACHE MISS" test-output.log
   ```

2. **Profile locally**: Instrument slow helpers

   ```typescript
   const startTime = Date.now();
   await waitForLoadingComplete(page);
   console.log(`Loading took ${Date.now() - startTime}ms`);
   ```

3. **Isolate shard**: Run failing shard locally to reproduce

   ```bash
   npx playwright test --shard=2/4 --project=firefox
   ```

---

## Test Isolation

### ❌ AVOID: State Leakage Between Tests

**Anti-Pattern**:

```typescript
test('enable Cerberus', async ({ page }) => {
  await toggleCerberus(page, true);
  // ⚠️ PROBLEM: Doesn't restore state
});

test('ACL settings require Cerberus', async ({ page }) => {
  // Assumes Cerberus is enabled from previous test
  await page.goto('/settings/acl');
  // ❌ FLAKY: Fails if first test didn't run or failed
});
```

**Why This Is Bad**:

- Tests depend on execution order (serial execution works, parallel fails)
- Flakiness when running with `--workers=4` or `--repeat-each=5`
- Hard to debug failures (root cause is in different test file)

---

### ✅ PREFER: Explicit State Restoration

**Correct Pattern**:

```typescript
test.afterEach(async ({ page }) => {
  await test.step('Restore default feature flag state', async () => {
    const defaultFlags = {
      'cerberus.enabled': true,
      'crowdsec.console_enrollment': false,
      'uptime.enabled': false,
    };

    // Direct API call to reset flags (no polling needed)
    for (const [flag, value] of Object.entries(defaultFlags)) {
      await page.evaluate(async ({ flag, value }) => {
        await fetch(`/api/v1/feature-flags/${flag}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ enabled: value }),
        });
      }, { flag, value });
    }
  });
});
```

**Why This Is Better**:

- **Zero inter-test dependencies**: Tests can run in any order
- Passes randomization testing: `--repeat-each=5 --workers=4`
- Explicit cleanup makes state management visible in code
- Fast restoration (no polling required, direct API call)

**Validation Command**:

```bash
# Verify test isolation with randomization
npx playwright test tests/settings/system-settings.spec.ts \
  --repeat-each=5 \
  --workers=4 \
  --project=chromium

# Should pass consistently regardless of execution order
```

---

## Robust Assertions for Dynamic Content

### ❌ AVOID: Boolean Logic on Transient States

**Anti-Pattern**:

```typescript
const hasEmptyMessage = await emptyCellMessage.isVisible().catch(() => false);
const hasTable = await table.isVisible().catch(() => false);
expect(hasEmptyMessage || hasTable).toBeTruthy();
```

**Why This Is Bad**:

- Fails during the split second where neither element is fully visible (loading transitions).
- Playwright's auto-retrying logic is bypassed by the `catch()` block.
- Leads to flaky "false negatives" where both checks return false before content loads.

### ✅ PREFER: Locator Composition with `.or()`

**Correct Pattern**:

```typescript
await expect(
  page.getByRole('table').or(page.getByText(/no.*certificates.*found/i))
).toBeVisible({ timeout: 10000 });
```

**Why This Is Better**:

- Leverages Playwright's built-in **auto-retry** mechanism.
- Waits for *either* condition to become true.
- Handles loading spinners and layout shifts gracefully.
- Reduces boilerplate code.

---

## Resilient Actions

### ❌ AVOID: Fixed Timeouts or Custom Loops

**Anti-Pattern**:

```typescript
// Flaky custom retry loop
for (let i = 0; i < 3; i++) {
  try {
    await action();
    break;
  } catch (e) {
    await page.waitForTimeout(1000);
  }
}
```

### ✅ PREFER: `.toPass()` for Verification Loops

**Correct Pattern**:

```typescript
await expect(async () => {
  const response = await request.post('/endpoint');
  expect(response.ok()).toBeTruthy();
}).toPass({
  intervals: [1000, 2000, 5000],
  timeout: 15_000
});
```

**Why This Is Better**:

- Built-in assertion retry logic.
- Configurable backoff intervals.
- Cleaner syntax for verifying eventual success (e.g. valid API response after background processing).

---

## Summary Checklist

Before writing E2E tests, verify:

- [ ] **Feature flags**: Only poll after toggling, not in beforeEach
- [ ] **Locators**: Use `getFormFieldByLabel()` for form fields
- [ ] **API calls**: Check for cache hit/miss logs, expect >30% hit rate
- [ ] **Performance**: Local execution <5 minutes, CI shard <15 minutes
- [ ] **Isolation**: Add `afterEach` cleanup if test modifies state
- [ ] **Cross-browser**: Test passes on all three browsers (Chromium, Firefox, WebKit)

---

## References

- **Implementation Details**: See `docs/plans/current_spec.md` (Fix 3.3)
- **Helper Library**: `tests/utils/ui-helpers.ts`
- **Playwright Config**: `playwright.config.js`
- **CI Workflow**: `.github/workflows/e2e-tests.yml`

---

**Last Updated**: 2026-02-02
