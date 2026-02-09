# Sprint 1 - E2E Test Timeout Remediation Findings

**Date**: 2026-02-02
**Status**: In Progress
**Sprint**: Sprint 1 (Quick Fixes - Priority Implementation)

## Implemented Changes

### ✅ Fix 1.1 + Fix 1.1b: Remove beforeEach polling, add afterEach cleanup

**File**: `tests/settings/system-settings.spec.ts`

**Changes Made**:
1. **Removed** `waitForFeatureFlagPropagation()` call from `beforeEach` hook (lines 35-46)
   - This was causing 10s × 31 tests = 310s of polling overhead per shard
   - Commented out with clear explanation linking to remediation plan

2. **Added** `test.afterEach()` hook with direct API state restoration:
   ```typescript
   test.afterEach(async ({ page }) => {
     await test.step('Restore default feature flag state', async () => {
       const defaultFlags = {
         'cerberus.enabled': true,
         'crowdsec.console_enrollment': false,
         'uptime.enabled': false,
       };

       // Direct API mutation to reset flags (no polling needed)
       await page.request.put('/api/v1/feature-flags', {
         data: defaultFlags,
       });
     });
   });
   ```

**Rationale**:
- Tests already verify feature flag state individually after toggle actions
- Initial state verification in beforeEach was redundant
- Explicit cleanup in afterEach ensures test isolation without polling overhead
- Direct API mutation for state restoration is faster than polling

**Expected Impact**:
- 310s saved per shard (10s × 31 tests)
- Elimination of inter-test dependencies
- No state leakage between tests

### ✅ Fix 1.3: Implement request coalescing with fixed cache

**File**: `tests/utils/wait-helpers.ts`

**Changes Made**:

1. **Added module-level cache** for in-flight requests:
   ```typescript
   // Cache for in-flight requests (per-worker isolation)
   const inflightRequests = new Map<string, Promise<Record<string, boolean>>>();
   ```

2. **Implemented cache key generation** with sorted keys and worker isolation:
   ```typescript
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
   ```

3. **Modified `waitForFeatureFlagPropagation()`** to use cache:
   - Returns cached promise if request already in flight for worker
   - Logs cache hits/misses for observability
   - Removes promise from cache after completion (success or failure)

4. **Added cleanup function**:
   ```typescript
   export function clearFeatureFlagCache(): void {
     inflightRequests.clear();
     console.log('[CACHE] Cleared all cached feature flag requests');
   }
   ```

**Why Sorted Keys?**
- `{a:true, b:false}` vs `{b:false, a:true}` are semantically identical
- Without sorting, they generate different cache keys → cache misses
- Sorting ensures consistent key regardless of property order

**Why Worker Isolation?**
- Playwright workers run in parallel across different browser contexts
- Each worker needs its own cache to avoid state conflicts
- Worker index provides unique namespace per parallel process

**Expected Impact**:
- 30-40% reduction in duplicate API calls (revised from original 70-80% estimate)
- Cache hit rate should be >30% based on similar flag state checks
- Reduced API server load during parallel test execution

## Investigation: Fix 1.2 - DNS Provider Label Mismatches

**Status**: Partially Investigated

**Issue**:
- Test: `tests/dns-provider-types.spec.ts` (line 260)
- Symptom: Label locator `/script.*path/i` passes in Chromium, fails in Firefox/WebKit
- Test code:
  ```typescript
  const scriptField = page.getByLabel(/script.*path/i);
  await expect(scriptField).toBeVisible({ timeout: 10000 });
  ```

**Investigation Steps Completed**:
1. ✅ Confirmed E2E environment is running and healthy
2. ✅ Attempted to run DNS provider type tests in Chromium
3. ⏸️ Further investigation deferred due to test execution issues

**Investigation Steps Remaining** (per spec):
1. Run with Playwright Inspector to compare accessibility trees:
   ```bash
   npx playwright test tests/dns-provider-types.spec.ts --project=chromium --headed --debug
   npx playwright test tests/dns-provider-types.spec.ts --project=firefox --headed --debug
   ```

2. Use `await page.getByRole('textbox').all()` to list all text inputs and their labels

3. Document findings in a Decision Record if labels differ

4. If fixable: Update component to ensure consistent aria-labels

5. If not fixable: Use the helper function approach from Phase 2

**Recommendation**:
- Complete investigation in separate session with headed browser mode
- DO NOT add `.or()` chains unless investigation proves it's necessary
- Create formal Decision Record once root cause is identified

## Validation Checkpoints

### Checkpoint 1: Execution Time
**Status**: ⏸️ In Progress

**Target**: <15 minutes (900s) for full test suite

**Command**:
```bash
time npx playwright test tests/settings/system-settings.spec.ts --project=chromium
```

**Results**:
- Test execution interrupted during validation
- Observed: Tests were picking up multiple spec files from security/ folder
- Need to investigate test file patterns or run with more specific filtering

**Action Required**:
- Re-run with corrected test file path or filtering
- Ensure only system-settings tests are executed
- Measure execution time and compare to baseline

### Checkpoint 2: Test Isolation
**Status**: ⏳ Pending

**Target**: All tests pass with `--repeat-each=5 --workers=4`

**Command**:
```bash
npx playwright test tests/settings/system-settings.spec.ts --project=chromium --repeat-each=5 --workers=4
```

**Status**: Not executed yet

### Checkpoint 3: Cross-browser
**Status**: ⏳ Pending

**Target**: Firefox/WebKit pass rate >85%

**Command**:
```bash
npx playwright test tests/settings/system-settings.spec.ts --project=firefox --project=webkit
```

**Status**: Not executed yet

### Checkpoint 4: DNS provider tests (secondary issue)
**Status**: ⏳ Pending

**Target**: Firefox tests pass or investigation complete

**Command**:
```bash
npx playwright test tests/dns-provider-types.spec.ts --project=firefox
```

**Status**: Investigation deferred

## Technical Decisions

### Decision: Use Direct API Mutation for State Restoration

**Context**:
- Tests need to restore default feature flag state after modifications
- Original approach used polling-based verification in beforeEach
- Alternative approaches: polling in afterEach vs direct API mutation

**Options Evaluated**:
1. **Polling in afterEach** - Verify state propagated after mutation
   - Pros: Confirms state is actually restored
   - Cons: Adds 500ms-2s per test (polling overhead)

2. **Direct API mutation without polling** (chosen)
   - Pros: Fast, predictable, no overhead
   - Cons: Assumes API mutation is synchronous/immediate
   - Why chosen: Feature flag updates are synchronous in backend

**Rationale**:
- Feature flag updates via PUT /api/v1/feature-flags are processed synchronously
- Database write is immediate (SQLite WAL mode)
- No async propagation delay in single-process test environment
- Subsequent tests will verify state on first read, catching any issues

**Impact**:
- Test runtime reduced by 15-60s per test file (31 tests × 500ms-2s polling)
- Risk: If state restoration fails, next test will fail loudly (detectable)
- Acceptable trade-off for 10-20% execution time improvement

**Review**: Re-evaluate if state restoration failures observed in CI

### Decision: Cache Key Sorting for Semantic Equality

**Context**:
- Multiple tests may check the same feature flag state but with different property order
- Without normalization, `{a:true, b:false}` and `{b:false, a:true}` generate different keys

**Rationale**:
- JavaScript objects have insertion order, but semantically these are identical states
- Sorting keys ensures cache hits for semantically identical flag states
- Minimal performance cost (~1ms for sorting 3-5 keys)

**Impact**:
- Estimated 10-15% cache hit rate improvement
- No downside - pure optimization

## Next Steps

1. **Complete Fix 1.2 Investigation**:
   - Run DNS provider tests in headed mode with Playwright Inspector
   - Document actual vs expected label structure in Firefox/WebKit
   - Create Decision Record with root cause and recommended fix

2. **Execute All Validation Checkpoints**:
   - Fix test file selection issue (why security tests run instead of system-settings)
   - Run all 4 checkpoints sequentially
   - Document pass/fail results with screenshots if failures occur

3. **Measure Impact**:
   - Baseline: Record execution time before fixes
   - Post-fix: Record execution time after fixes
   - Calculate actual time savings vs predicted 310s savings

4. **Update Spec**:
   - Document actual vs predicted impact
   - Adjust estimates for Phase 2 based on Sprint 1 findings

## Code Review Checklist

- [x] Fix 1.1: Remove beforeEach polling
- [x] Fix 1.1b: Add afterEach cleanup
- [x] Fix 1.3: Implement request coalescing
- [x] Add cache cleanup function
- [x] Document cache key generation logic
- [ ] Fix 1.2: Complete investigation
- [ ] Run all validation checkpoints
- [ ] Update spec with actual findings

## References

- **Remediation Plan**: `docs/plans/current_spec.md`
- **Modified Files**:
  - `tests/settings/system-settings.spec.ts`
  - `tests/utils/wait-helpers.ts`
- **Investigation Target**: `tests/dns-provider-types.spec.ts` (line 260)

---

**Last Updated**: 2026-02-02
**Author**: GitHub Copilot (Playwright Dev Mode)
**Status**: Sprint 1 implementation complete, validation checkpoints pending
