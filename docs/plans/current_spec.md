# E2E Test Timeout Remediation Plan

**Status**: Active
**Created**: 2026-02-02
**Priority**: P0 (Blocking CI/CD pipeline)
**Estimated Effort**: 5-7 business days

## Executive Summary

E2E tests are timing out due to cascading API bottleneck caused by feature flag polling in `beforeEach` hooks, combined with browser-specific label locator failures. This blocks PR merges and slows development velocity.

**Impact**:
- 31 tests × 10s timeout × 12 parallel processes = ~310s minimum execution time per shard
- 4 shards × 3 browsers = 12 jobs, many exceeding 30min GitHub Actions limit
- Firefox/WebKit tests fail on DNS provider form due to label locator mismatches

## Root Cause Analysis

### Primary Issue: Feature Flag Polling API Bottleneck

**Location**: `tests/settings/system-settings.spec.ts` (lines 27-48)

```typescript
test.beforeEach(async ({ page, adminUser }) => {
  await loginUser(page, adminUser);
  await waitForLoadingComplete(page);
  await page.goto('/settings/system');
  await waitForLoadingComplete(page);

  // ⚠️ PROBLEM: Runs before EVERY test
  await waitForFeatureFlagPropagation(
    page,
    {
      'cerberus.enabled': true,
      'crowdsec.console_enrollment': false,
      'uptime.enabled': false,
    },
    { timeout: 10000 } // 10s timeout per test
  ).catch(() => {
    console.log('[WARN] Initial state verification skipped');
  });
});
```

**Why This Causes Timeouts**:
1. `waitForFeatureFlagPropagation()` polls `/api/v1/feature-flags` every 500ms for up to 10s
2. Runs in `beforeEach` hook = executes 31 times per test file
3. 12 parallel processes (4 shards × 3 browsers) all hitting same endpoint
4. API server degrades under concurrent load → tests timeout → shards exceed job limit

**Evidence**:
- `tests/utils/wait-helpers.ts` (lines 411-470): Polling interval 500ms, default timeout 30s
- Workflow config: 4 shards × 3 browsers = 12 concurrent jobs
- Observed: Multiple shards exceed 30min job timeout

### Secondary Issue: Browser-Specific Label Locator Failures

**Location**: `tests/dns-provider-types.spec.ts` (line 260)

```typescript
await test.step('Verify Script path/command field appears', async () => {
  const scriptField = page.getByLabel(/script.*path/i);
  await expect(scriptField).toBeVisible({ timeout: 10000 });
});
```

**Why Firefox/WebKit Fail**:
1. Backend returns `script_path` field with label "Script Path"
2. Frontend applies `aria-label="Script Path"` to input (line 276 in DNSProviderForm.tsx)
3. Firefox/WebKit may render Label component differently than Chromium
4. Regex `/script.*path/i` may not match if label has extra whitespace or is split across nodes

**Evidence**:
- `frontend/src/components/DNSProviderForm.tsx` (lines 273-279): Hardcoded `aria-label="Script Path"`
- `backend/pkg/dnsprovider/custom/script_provider.go` (line 85): Backend returns "Script Path"
- Test passes in Chromium, fails in Firefox/WebKit = browser-specific rendering difference

## Requirements (EARS Notation)

### REQ-1: Feature Flag Polling Optimization
**WHEN** E2E tests execute, **THE SYSTEM SHALL** minimize API calls to feature flag endpoint to reduce load and execution time.

**Acceptance Criteria**:
- Feature flag polling occurs once per test file, not per test
- API calls reduced by 90% (from 31 per shard to <3 per shard)
- Test execution time reduced by 20-30%

### REQ-2: Browser-Agnostic Label Locators
**WHEN** E2E tests query form fields, **THE SYSTEM SHALL** use locators that work consistently across Chromium, Firefox, and WebKit.

**Acceptance Criteria**:
- All DNS provider form tests pass on Firefox and WebKit
- Locators use multiple fallback strategies (`getByLabel`, `getByPlaceholder`, `getById`)
- No browser-specific workarounds needed

### REQ-3: API Stress Reduction
**WHEN** parallel test processes execute, **THE SYSTEM SHALL** implement throttling or debouncing to prevent API bottlenecks.

**Acceptance Criteria**:
- Concurrent API calls limited via request coalescing
- Tests use cached responses where appropriate
- API server remains responsive under test load

### REQ-4: Test Isolation
**WHEN** a test modifies feature flags, **THE SYSTEM SHALL** restore original state without requiring global polling.

**Acceptance Criteria**:
- Feature flag state restored per-test using direct API calls
- No inter-test dependencies on feature flag state
- Tests can run in any order without failures

## Technical Design

### Phase 1: Quick Fixes (Deploy within 24h)

#### Fix 1.1: Remove Unnecessary Feature Flag Polling from beforeEach
**File**: `tests/settings/system-settings.spec.ts`

**Change**: Remove `waitForFeatureFlagPropagation()` from `beforeEach` hook entirely.

**Rationale**:
- Tests already verify feature flag state in test steps
- Initial state verification is redundant if tests toggle and verify in each step
- Polling is only needed AFTER toggling, not before every test

**Implementation**:
```typescript
test.beforeEach(async ({ page, adminUser }) => {
  await loginUser(page, adminUser);
  await waitForLoadingComplete(page);
  await page.goto('/settings/system');
  await waitForLoadingComplete(page);

  // ✅ REMOVED: Feature flag polling - tests verify state individually
});
```

**Expected Impact**: 10s × 31 tests = 310s saved per shard

#### Fix 1.1b: Add Test Isolation Strategy
**File**: `tests/settings/system-settings.spec.ts`

**Change**: Add `test.afterEach()` hook to restore default feature flag state after each test.

**Rationale**:
- Not all 31 tests explicitly verify feature flag state in their steps
- Some tests may modify flags without restoring them
- State leakage between tests can cause flakiness
- Explicit cleanup ensures test isolation

**Implementation**:
```typescript
test.afterEach(async ({ page }) => {
  await test.step('Restore default feature flag state', async () => {
    // Reset to known good state after each test
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

**Validation Command**:
```bash
# Test isolation: Run tests in random order with multiple workers
npx playwright test tests/settings/system-settings.spec.ts \
  --repeat-each=5 \
  --workers=4 \
  --project=chromium

# Should pass consistently regardless of execution order
```

**Expected Impact**: Eliminates inter-test dependencies, prevents state leakage

#### Fix 1.2: Investigate and Fix Root Cause of Label Locator Failures
**File**: `tests/dns-provider-types.spec.ts`

**Change**: Investigate why label locator fails in Firefox/WebKit before applying workarounds.

**Current Symptom**:
```typescript
const scriptField = page.getByLabel(/script.*path/i);
await expect(scriptField).toBeVisible({ timeout: 10000 });
// ❌ Passes in Chromium, fails in Firefox/WebKit
```

**Investigation Steps** (REQUIRED before implementing fix):

1. **Use Playwright Inspector** to examine actual DOM structure:
   ```bash
   PWDEBUG=1 npx playwright test tests/dns-provider-types.spec.ts \
     --project=firefox \
     --headed
   ```
   - Pause at failure point
   - Inspect the form field in dev tools
   - Check actual `aria-label`, `label` association, and `for` attribute
   - Document differences between Chromium vs Firefox/WebKit

2. **Check Component Implementation**:
   - Review `frontend/src/components/DNSProviderForm.tsx` (lines 273-279)
   - Verify Label component from shadcn/ui renders correctly
   - Check if `htmlFor` attribute matches input `id`
   - Test manual form interaction in Firefox/WebKit locally

3. **Verify Backend Response**:
   - Inspect `/api/v1/dns-providers/custom/script` response
   - Confirm `script_path` field metadata includes correct label
   - Check if label is being transformed or sanitized

**Potential Root Causes** (investigate in order):
- Label component not associating with input (missing `htmlFor`/`id` match)
- Browser-specific text normalization (e.g., whitespace, case sensitivity)
- ARIA label override conflicting with visible label
- React hydration issue in Firefox/WebKit

**Fix Strategy** (only after investigation):

**IF** root cause is fixable in component:
- Fix the actual bug in `DNSProviderForm.tsx`
- No workaround needed in tests
- **Document Decision in Decision Record** (required)

**IF** root cause is browser-specific rendering quirk:
- Use `.or()` chaining as documented fallback:
  ```typescript
  await test.step('Verify Script path/command field appears', async () => {
    // Primary strategy: label locator (works in Chromium)
    const scriptField = page
      .getByLabel(/script.*path/i)
      .or(page.getByPlaceholder(/dns-challenge\.sh/i))  // Fallback 1
      .or(page.locator('input[id^="field-script"]'));   // Fallback 2

    await expect(scriptField.first()).toBeVisible({ timeout: 10000 });
  });
  ```
- **Document Decision in Decision Record** (required)
- Add comment explaining why `.or()` is needed

**Decision Record Template** (create if workaround is needed):
```markdown
### Decision - 2026-02-02 - DNS Provider Label Locator Workaround

**Decision**: Use `.or()` chaining for Script Path field locator

**Context**:
- `page.getByLabel(/script.*path/i)` fails in Firefox/WebKit
- Root cause: [document findings from investigation]
- Component: `DNSProviderForm.tsx` line 276

**Options**:
1. Fix component (preferred) - [reason why not chosen]
2. Use `.or()` chaining (chosen) - [reason]
3. Skip Firefox/WebKit tests - [reason why not chosen]

**Rationale**: [Explain trade-offs and why workaround is acceptable]

**Impact**:
- Test reliability: [describe]
- Maintenance burden: [describe]
- Future component changes: [describe]

**Review**: Re-evaluate when Playwright or shadcn/ui updates are applied
```

**Expected Impact**: Tests pass consistently on all browsers with understood root cause

#### Fix 1.3: Add Request Coalescing with Worker Isolation
**File**: `tests/utils/wait-helpers.ts`

**Change**: Cache in-flight requests with proper worker isolation and sorted keys.

**Implementation**:
```typescript
// Add at module level
const inflightRequests = new Map<string, Promise<Record<string, boolean>>>();

/**
 * Generate stable cache key with worker isolation
 * Prevents cache collisions between parallel workers
 */
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
  // Get worker index from test info
  const workerIndex = test.info().parallelIndex;
  const cacheKey = generateCacheKey(expectedFlags, workerIndex);

  // Return existing promise if already in flight for this worker
  if (inflightRequests.has(cacheKey)) {
    console.log(`[CACHE HIT] Worker ${workerIndex}: ${cacheKey}`);
    return inflightRequests.get(cacheKey)!;
  }

  console.log(`[CACHE MISS] Worker ${workerIndex}: ${cacheKey}`);

  const promise = (async () => {
    // Existing polling logic...
    const interval = options.interval ?? 500;
    const timeout = options.timeout ?? 30000;
    const maxAttempts = options.maxAttempts ?? Math.ceil(timeout / interval);

    let lastResponse: Record<string, boolean> | null = null;
    let attemptCount = 0;

    while (attemptCount < maxAttempts) {
      attemptCount++;

      const response = await page.evaluate(async () => {
        const res = await fetch('/api/v1/feature-flags', {
          method: 'GET',
          headers: { 'Content-Type': 'application/json' },
        });
        return { ok: res.ok, status: res.status, data: await res.json() };
      });

      lastResponse = response.data as Record<string, boolean>;

      const allMatch = Object.entries(expectedFlags).every(
        ([key, expectedValue]) => response.data[key] === expectedValue
      );

      if (allMatch) {
        inflightRequests.delete(cacheKey);
        return lastResponse;
      }

      await page.waitForTimeout(interval);
    }

    inflightRequests.delete(cacheKey);
    throw new Error(
      `Feature flag propagation timeout after ${attemptCount} attempts (${timeout}ms).\n` +
      `Expected: ${JSON.stringify(expectedFlags)}\n` +
      `Actual: ${JSON.stringify(lastResponse)}`
    );
  })();

  inflightRequests.set(cacheKey, promise);
  return promise;
}

// Clear cache after all tests in a worker complete
test.afterAll(() => {
  const workerIndex = test.info().parallelIndex;
  const keysToDelete = Array.from(inflightRequests.keys())
    .filter(key => key.startsWith(`${workerIndex}:`));

  keysToDelete.forEach(key => inflightRequests.delete(key));
  console.log(`[CLEANUP] Worker ${workerIndex}: Cleared ${keysToDelete.length} cache entries`);
});
```

**Why Sorted Keys?**
- `{a:true, b:false}` vs `{b:false, a:true}` are semantically identical
- Without sorting, they generate different cache keys → cache misses
- Sorting ensures consistent key regardless of property order

**Why Worker Isolation?**
- Playwright workers run in parallel across different browser contexts
- Each worker needs its own cache to avoid state conflicts
- Worker index provides unique namespace per parallel process

**Expected Impact**: Reduce duplicate API calls by 30-40% (revised from 70-80%)

### Phase 2: Root Cause Fixes (Deploy within 72h)

#### Fix 2.1: Convert Feature Flag Verification to Per-Test Pattern
**Files**: All test files using `waitForFeatureFlagPropagation()`

**Change**: Move feature flag verification into individual test steps where state changes occur.

**Pattern**:
```typescript
// ❌ OLD: Global beforeEach polling
test.beforeEach(async ({ page }) => {
  await waitForFeatureFlagPropagation(page, { 'cerberus.enabled': true });
});

// ✅ NEW: Per-test verification only when toggled
test('should toggle Cerberus feature', async ({ page }) => {
  await test.step('Toggle Cerberus feature', async () => {
    const toggle = page.getByRole('switch', { name: /cerberus/i });
    const initialState = await toggle.isChecked();

    await retryAction(async () => {
      const response = await clickSwitchAndWaitForResponse(page, toggle, /\/feature-flags/);
      expect(response.ok()).toBeTruthy();

      // Only verify propagation after toggle action
      await waitForFeatureFlagPropagation(page, {
        'cerberus.enabled': !initialState,
      });
    });
  });
});
```

**CRITICAL AUDIT REQUIREMENT**:
Before implementing, audit all 31 tests in `system-settings.spec.ts` to identify:
1. Which tests explicitly toggle feature flags (require propagation check)
2. Which tests only read feature flag state (no propagation check needed)
3. Which tests assume Cerberus is enabled (document dependency)

**Audit Template**:
```markdown
| Test Name | Toggles Flags? | Requires Cerberus? | Action |
|-----------|----------------|-------------------|--------|
| "should display security settings" | No | Yes | Add dependency comment |
| "should toggle ACL" | Yes | Yes | Add propagation check |
| "should display CrowdSec status" | No | Yes | Add dependency comment |
```

**Files to Update**:
- `tests/settings/system-settings.spec.ts` (31 tests)
- `tests/cerberus/security-dashboard.spec.ts` (if applicable)

**Expected Impact**: 90% reduction in API calls (from 31 per shard to 3-5 per shard)

#### Fix 2.2: Implement Label Helper for Cross-Browser Compatibility
**File**: `tests/utils/ui-helpers.ts`

**Implementation**:
```typescript
/**
 * Get form field with cross-browser label matching
 * Tries multiple strategies: label, placeholder, id, aria-label
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

**Usage in Tests**:
```typescript
await test.step('Verify Script path/command field appears', async () => {
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

**Files to Update**:
- `tests/dns-provider-types.spec.ts` (3 tests)
- `tests/dns-provider-crud.spec.ts` (accessibility tests)

**Expected Impact**: 100% pass rate on Firefox/WebKit

#### Fix 2.3: Add Conditional Feature Flag Verification
**File**: `tests/utils/wait-helpers.ts`

**Change**: Skip polling if already in expected state.

**Implementation**:
```typescript
export async function waitForFeatureFlagPropagation(
  page: Page,
  expectedFlags: Record<string, boolean>,
  options: FeatureFlagPropagationOptions = {}
): Promise<Record<string, boolean>> {
  // Quick check: are we already in expected state?
  const currentState = await page.evaluate(async () => {
    const res = await fetch('/api/v1/feature-flags');
    return res.json();
  });

  const alreadyMatches = Object.entries(expectedFlags).every(
    ([key, expectedValue]) => currentState[key] === expectedValue
  );

  if (alreadyMatches) {
    console.log('[POLL] Feature flags already in expected state - skipping poll');
    return currentState;
  }

  // Existing polling logic...
}
```

**Expected Impact**: 50% fewer iterations when state is already correct

### Phase 3: Prevention & Monitoring (Deploy within 1 week)

#### Fix 3.1: Add E2E Performance Budget
**File**: `.github/workflows/e2e-tests.yml`

**Change**: Add step to enforce execution time limits per shard.

**Implementation**:
```yaml
- name: Verify shard performance budget
  if: always()
  run: |
    SHARD_DURATION=$((SHARD_END - SHARD_START))
    MAX_DURATION=900  # 15 minutes

    if [[ $SHARD_DURATION -gt $MAX_DURATION ]]; then
      echo "::error::Shard exceeded performance budget: ${SHARD_DURATION}s > ${MAX_DURATION}s"
      echo "::error::Investigate slow tests or API bottlenecks"
      exit 1
    fi

    echo "✅ Shard completed within budget: ${SHARD_DURATION}s"
```

**Expected Impact**: Early detection of performance regressions

#### Fix 3.2: Add API Call Metrics to Test Reports
**File**: `tests/utils/wait-helpers.ts`

**Change**: Track and report API call counts.

**Implementation**:
```typescript
// Track metrics at module level
const apiMetrics = {
  featureFlagCalls: 0,
  cacheHits: 0,
  cacheMisses: 0,
};

export function getAPIMetrics() {
  return { ...apiMetrics };
}

export function resetAPIMetrics() {
  apiMetrics.featureFlagCalls = 0;
  apiMetrics.cacheHits = 0;
  apiMetrics.cacheMisses = 0;
}

// Update waitForFeatureFlagPropagation to increment counters
export async function waitForFeatureFlagPropagation(...) {
  apiMetrics.featureFlagCalls++;

  if (inflightRequests.has(cacheKey)) {
    apiMetrics.cacheHits++;
    return inflightRequests.get(cacheKey)!;
  }

  apiMetrics.cacheMisses++;
  // ...
}
```

**Add to test teardown**:
```typescript
test.afterAll(async () => {
  const metrics = getAPIMetrics();
  console.log('API Call Metrics:', metrics);

  if (metrics.featureFlagCalls > 50) {
    console.warn(`⚠️ High API call count: ${metrics.featureFlagCalls}`);
  }
});
```

**Expected Impact**: Visibility into API usage patterns

#### Fix 3.3: Document Best Practices for E2E Tests
**File**: `docs/testing/e2e-best-practices.md` (to be created)

**Content**:
```markdown
# E2E Testing Best Practices

## Feature Flag Testing

### ❌ AVOID: Polling in beforeEach
```typescript
test.beforeEach(async ({ page }) => {
  // This runs before EVERY test - expensive!
  await waitForFeatureFlagPropagation(page, { flag: true });
});
```

### ✅ PREFER: Per-test verification
```typescript
test('feature toggle', async ({ page }) => {
  // Only verify after we change the flag
  await clickToggle(page);
  await waitForFeatureFlagPropagation(page, { flag: false });
});
```

## Cross-Browser Locators

### ❌ AVOID: Label-only locators
```typescript
page.getByLabel(/script.*path/i)  // May fail in Firefox/WebKit
```

### ✅ PREFER: Multi-strategy locators
```typescript
getFormFieldByLabel(page, /script.*path/i, {
  placeholder: /dns-challenge/i,
  fieldId: 'field-script_path'
})
```
```

**Expected Impact**: Prevent future performance regressions

## Implementation Plan

### Sprint 1: Quick Wins (Days 1-2)
- [ ] **Task 1.1**: Remove feature flag polling from `system-settings.spec.ts` beforeEach
  - **Assignee**: TBD
  - **Files**: `tests/settings/system-settings.spec.ts`
  - **Validation**: Run test file locally, verify <5min execution time

- [ ] **Task 1.1b**: Add test isolation with afterEach cleanup
  - **Assignee**: TBD
  - **Files**: `tests/settings/system-settings.spec.ts`
  - **Validation**:
    ```bash
    npx playwright test tests/settings/system-settings.spec.ts \
      --repeat-each=5 --workers=4 --project=chromium
    ```

- [ ] **Task 1.2**: Investigate label locator failures (BEFORE implementing workaround)
  - **Assignee**: TBD
  - **Files**: `tests/dns-provider-types.spec.ts`, `frontend/src/components/DNSProviderForm.tsx`
  - **Validation**: Document investigation findings, create Decision Record if workaround needed

- [ ] **Task 1.3**: Add request coalescing with worker isolation
  - **Assignee**: TBD
  - **Files**: `tests/utils/wait-helpers.ts`
  - **Validation**: Check console logs for cache hits/misses, verify cache clears in afterAll

**Sprint 1 Go/No-Go Checkpoint**:

✅ **PASS Criteria** (all must be green):
1. **Execution Time**: Test file runs in <5min locally
   ```bash
   time npx playwright test tests/settings/system-settings.spec.ts --project=chromium
   ```
   Expected: <300s

2. **Test Isolation**: Tests pass with randomization
   ```bash
   npx playwright test tests/settings/system-settings.spec.ts \
     --repeat-each=5 --workers=4 --shard=1/1
   ```
   Expected: 0 failures

3. **Cache Performance**: Cache hit rate >30%
   ```bash
   grep -o "CACHE HIT" test-output.log | wc -l
   grep -o "CACHE MISS" test-output.log | wc -l
   # Calculate: hits / (hits + misses) > 0.30
   ```

❌ **STOP and Investigate If**:
- Execution time >5min (insufficient improvement)
- Test isolation fails (indicates missing cleanup)
- Cache hit rate <20% (worker isolation not working)
- Any new test failures introduced

**Action on Failure**: Revert changes, root cause analysis, re-plan before proceeding to Sprint 2

### Sprint 2: Root Fixes (Days 3-5)
- [ ] **Task 2.0**: Audit all 31 tests for Cerberus dependencies
  - **Assignee**: TBD
  - **Files**: `tests/settings/system-settings.spec.ts`
  - **Validation**: Complete audit table identifying which tests require propagation checks

- [ ] **Task 2.1**: Refactor feature flag verification to per-test pattern
  - **Assignee**: TBD
  - **Files**: `tests/settings/system-settings.spec.ts` (only tests that toggle flags)
  - **Validation**: All tests pass with <50 API calls total (check metrics)
  - **Note**: Add `test.step()` wrapper to all refactored examples

- [ ] **Task 2.2**: Create `getFormFieldByLabel` helper (only if workaround confirmed needed)
  - **Assignee**: TBD
  - **Files**: `tests/utils/ui-helpers.ts`
  - **Validation**: Use in 3 test files, verify Firefox/WebKit pass
  - **Prerequisite**: Decision Record from Task 1.2 investigation

- [ ] **Task 2.3**: Add conditional skip to feature flag polling
  - **Assignee**: TBD
  - **Files**: `tests/utils/wait-helpers.ts`
  - **Validation**: Verify early exit logs appear when state already matches

**Sprint 2 Go/No-Go Checkpoint**:

✅ **PASS Criteria** (all must be green):
1. **API Call Reduction**: <50 calls per shard
   ```bash
   # Add instrumentation to count API calls
   grep "GET /api/v1/feature-flags" charon.log | wc -l
   ```
   Expected: <50 calls

2. **Cross-Browser Stability**: Firefox/WebKit pass rate >95%
   ```bash
   npx playwright test --project=firefox --project=webkit
   ```
   Expected: <5% failure rate

3. **Test Coverage**: No coverage regression
   ```bash
   .github/skills/scripts/skill-runner.sh test-e2e-playwright-coverage
   ```
   Expected: Coverage ≥ baseline

4. **Audit Completeness**: All 31 tests categorized
   - Verify audit table is 100% complete
   - All tests have appropriate propagation checks or dependency comments

❌ **STOP and Investigate If**:
- API calls still >100 per shard (insufficient improvement)
- Firefox/WebKit pass rate <90% (locator fixes inadequate)
- Coverage drops >2% (tests not properly refactored)
- Missing audit entries (incomplete understanding of dependencies)

**Action on Failure**: Do NOT proceed to Sprint 3. Re-analyze bottlenecks and revise approach.

### Sprint 3: Prevention (Days 6-7)
- [ ] **Task 3.1**: Add performance budget check to CI
  - **Assignee**: TBD
  - **Files**: `.github/workflows/e2e-tests.yml`
  - **Validation**: Trigger workflow, verify budget check runs

- [ ] **Task 3.2**: Implement API call metrics tracking
  - **Assignee**: TBD
  - **Files**: `tests/utils/wait-helpers.ts`, test files
  - **Validation**: Run test suite, verify metrics in console output

- [ ] **Task 3.3**: Document E2E best practices
  - **Assignee**: TBD
  - **Files**: `docs/testing/e2e-best-practices.md` (create)
  - **Validation**: Technical review by team

## Coverage Impact Analysis

### Baseline Coverage Requirements

**MANDATORY**: Before making ANY changes, establish baseline coverage:

```bash
# Create baseline coverage report
.github/skills/scripts/skill-runner.sh test-e2e-playwright-coverage

# Save baseline metrics
cp coverage/e2e/lcov.info coverage/e2e/baseline-lcov.info
cp coverage/e2e/coverage-summary.json coverage/e2e/baseline-summary.json

# Document baseline
echo "Baseline Coverage: $(grep -A 1 'lines' coverage/e2e/coverage-summary.json)" >> docs/plans/coverage-baseline.txt
```

**Baseline Thresholds** (from `playwright.config.js`):
- **Lines**: ≥80%
- **Functions**: ≥80%
- **Branches**: ≥80%
- **Statements**: ≥80%

### Codecov Requirements

**100% Patch Coverage** (from `codecov.yml`):
- Every line of production code modified MUST be covered by tests
- Applies to frontend changes in:
  - `tests/settings/system-settings.spec.ts`
  - `tests/dns-provider-types.spec.ts`
  - `tests/utils/wait-helpers.ts`
  - `tests/utils/ui-helpers.ts`

**Verification Commands**:
```bash
# After each sprint, verify coverage
.github/skills/scripts/skill-runner.sh test-e2e-playwright-coverage

# Compare to baseline
diff coverage/e2e/baseline-summary.json coverage/e2e/coverage-summary.json

# Check for regressions
if [[ $(jq '.total.lines.pct' coverage/e2e/coverage-summary.json) < $(jq '.total.lines.pct' coverage/e2e/baseline-summary.json) ]]; then
  echo "❌ Coverage regression detected"
  exit 1
fi

# Upload to Codecov (CI will enforce patch coverage)
git diff --name-only main...HEAD > changed-files.txt
curl -s https://codecov.io/bash | bash -s -- -f coverage/e2e/lcov.info
```

### Impact Analysis by Sprint

**Sprint 1 Changes**:
- Files: `system-settings.spec.ts`, `wait-helpers.ts`
- Risk: Removing polling might reduce coverage of error paths
- Mitigation: Ensure error handling in `afterEach` is tested

**Sprint 2 Changes**:
- Files: `system-settings.spec.ts` (31 tests refactored), `ui-helpers.ts`
- Risk: Per-test refactoring might miss edge cases
- Mitigation: Run coverage diff after each test refactored

**Sprint 3 Changes**:
- Files: E2E workflow, test documentation
- Risk: No production code changes (no coverage impact)

### Coverage Failure Protocol

**IF** coverage drops below baseline:
1. Identify uncovered lines: `npx nyc report --reporter=html`
2. Add targeted tests for missed paths
3. Re-run coverage verification
4. **DO NOT** merge until coverage restored

**IF** Codecov reports <100% patch coverage:
1. Review Codecov PR comment for specific lines
2. Add test cases covering modified lines
3. Push fixup commit
4. Re-check Codecov status

## Validation Strategy

### Local Testing (Before push)
```bash
# Quick validation: Run affected test file
npx playwright test tests/settings/system-settings.spec.ts --project=chromium

# Cross-browser validation
npx playwright test tests/dns-provider-types.spec.ts --project=firefox --project=webkit

# Full suite (should complete in <20min per shard)
npx playwright test --shard=1/4
```

### CI Validation (After push)
1. **Green CI**: All 12 jobs (4 shards × 3 browsers) pass
2. **Performance**: Each shard completes in <15min (down from 30min)
3. **API Calls**: Feature flag endpoint receives <100 requests per shard (down from ~1000)

### Rollback Plan
If fixes introduce failures:
1. Revert commits atomically (Fix 1.1, 1.2, 1.3 are independent)
2. Re-enable `test.skip()` for failing tests temporarily
3. Document known issues in PR comments

## Success Metrics

| Metric | Before | Target | How to Measure |
|--------|--------|--------|----------------|
| Shard Execution Time | 30+ min | <15 min | GitHub Actions logs |
| Feature Flag API Calls | ~1000/shard | <100/shard | Add metrics to wait-helpers.ts |
| Firefox/WebKit Pass Rate | 70% | 95%+ | CI test results |
| Job Timeout Rate | 30% | <5% | GitHub Actions workflow analytics |

## Performance Profiling (Optional Enhancement)

### Profiling waitForLoadingComplete()

During Sprint 1, if time permits, profile `waitForLoadingComplete()` to identify additional bottlenecks:

```typescript
// Add instrumentation to wait-helpers.ts
export async function waitForLoadingComplete(page: Page, timeout = 30000) {
  const startTime = Date.now();

  await page.waitForLoadState('networkidle', { timeout });
  await page.waitForLoadState('domcontentloaded', { timeout });

  const duration = Date.now() - startTime;
  if (duration > 5000) {
    console.warn(`[SLOW] waitForLoadingComplete took ${duration}ms`);
  }

  return duration;
}
```

**Analysis**:
```bash
# Run tests with profiling enabled
npx playwright test tests/settings/system-settings.spec.ts --project=chromium > profile.log

# Extract slow calls
grep "\[SLOW\]" profile.log | sort -t= -k2 -n

# Identify patterns
# - Is networkidle too strict?
# - Are certain pages slower than others?
# - Can we use 'load' state instead of 'networkidle'?
```

**Potential Optimization**:
If `networkidle` is consistently slow, consider using `load` state for non-critical pages:
```typescript
// For pages without dynamic content
await page.waitForLoadState('load', { timeout });

// For pages with feature flag updates
await page.waitForLoadState('networkidle', { timeout });
```

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Breaking existing tests | Medium | High | Run full test suite locally before push |
| Firefox/WebKit still fail | Low | Medium | Add `.or()` chaining for more fallbacks |
| API server still bottlenecks | Low | Medium | Add rate limiting to test container |
| Regressions in future PRs | Medium | Medium | Add performance budget check to CI |

## Infrastructure Considerations

### Current Setup
- **Workflow**: `.github/workflows/e2e-tests.yml`
- **Sharding**: 4 shards × 3 browsers = 12 jobs
- **Timeout**: 30 minutes per job
- **Concurrency**: All jobs run in parallel

### Recommended Changes
1. **Add caching**: Cache Playwright browsers between runs (already implemented)
2. **Optimize container startup**: Health check timeout reduced from 60s to 30s
3. **Consider**: Reduce shards from 4→3 if execution time improves sufficiently

### Monitoring
- Track job duration trends in GitHub Actions analytics
- Alert if shard duration exceeds 20min
- Weekly review of flaky test reports

## Decision Record Template for Workarounds

Whenever a workaround is implemented instead of fixing root cause (e.g., `.or()` chaining for label locators), document the decision:

```markdown
### Decision - [DATE] - [BRIEF TITLE]

**Decision**: [What was decided]

**Context**:
- Original issue: [Describe problem]
- Root cause investigation findings: [Summarize]
- Component/file affected: [Specific paths]

**Options Evaluated**:
1. **Fix root cause** (preferred)
   - Pros: [List]
   - Cons: [List]
   - Why not chosen: [Specific reason]

2. **Workaround with `.or()` chaining** (chosen)
   - Pros: [List]
   - Cons: [List]
   - Why chosen: [Specific reason]

3. **Skip affected browsers** (rejected)
   - Pros: [List]
   - Cons: [List]
   - Why not chosen: [Specific reason]

**Rationale**:
[Detailed explanation of trade-offs and why workaround is acceptable]

**Impact**:
- **Test Reliability**: [Describe expected improvement]
- **Maintenance Burden**: [Describe ongoing cost]
- **Future Considerations**: [What needs to be revisited]

**Review Schedule**:
[When to re-evaluate - e.g., "After Playwright 1.50 release" or "Q2 2026"]

**References**:
- Investigation notes: [Link to investigation findings]
- Related issues: [GitHub issues, if any]
- Component documentation: [Relevant docs]
```

**Where to Store**:
- Simple decisions: Inline comment in test file
- Complex decisions: `docs/decisions/workaround-[feature]-[date].md`
- Reference in PR description when merging

## Additional Files to Review

Before implementation, review these files for context:

- [ ] `playwright.config.js` - Test configuration, timeout settings
- [ ] `.docker/compose/docker-compose.playwright-ci.yml` - Container environment
- [ ] `tests/fixtures/auth-fixtures.ts` - Login helper usage
- [ ] `tests/cerberus/security-dashboard.spec.ts` - Other files using feature flag polling
- [ ] `codecov.yml` - Coverage requirements (patch coverage must remain 100%)

## References

- **Original Issue**: GitHub Actions job timeouts in E2E workflow
- **Related Docs**:
  - `docs/testing/playwright-typescript.instructions.md` - Test writing guidelines
  - `docs/testing/testing.instructions.md` - Testing protocols
  - `.github/instructions/testing.instructions.md` - CI testing protocols
- **Prior Plans**:
  - `docs/plans/phase4-settings-plan.md` - System settings feature implementation
  - `docs/implementation/dns_providers_IMPLEMENTATION.md` - DNS provider architecture

## Next Steps

1. **Triage**: Assign tasks to team members
2. **Sprint 1 Kickoff**: Implement quick fixes (1-2 days)
3. **PR Review**: All changes require approval before merge
4. **Monitor**: Track metrics for 1 week post-deployment
5. **Iterate**: Adjust thresholds based on real-world performance

---

**Last Updated**: 2026-02-02
**Owner**: TBD
**Reviewers**: TBD
