# Shard 1 Failure Surgical Fix Plan

**Status:** Investigation Complete - Root Cause Identified
**Priority:** P0 - Blocking CI
**Estimated Time:** 1-2 hours
**Created:** 2026-02-03

---

## Executive Summary

Shard 1 is failing across **all 3 browsers** (Chromium, Firefox, WebKit) in CI while Shards 2 & 3 pass. Tests pass locally. The failure is **shard-specific**, not browser-specific, indicating a systematic issue in the first 25% of test files alphabetically.

---

## Investigation Results

### 1. Shard 1 Test Files (First 13 of 50)

```
1.  tests/core/access-lists-crud.spec.ts      ✅ REFACTORED (Phase 2)
2.  tests/core/authentication.spec.ts         ✅ REFACTORED (Phase 2)
3.  tests/core/certificates.spec.ts           ✅ REFACTORED (Phase 2)
4.  tests/core/dashboard.spec.ts
5.  tests/core/navigation.spec.ts
6.  tests/core/proxy-hosts.spec.ts            ✅ REFACTORED (Phase 2)
7.  tests/dns-provider-crud.spec.ts
8.  tests/dns-provider-types.spec.ts
9.  tests/emergency-server/emergency-server.spec.ts
10. tests/integration/backup-restore-e2e.spec.ts
11. tests/integration/import-to-production.spec.ts
12. tests/integration/multi-feature-workflows.spec.ts
13. tests/integration/security-suite-integration.spec.ts
```

**Key Finding:** **4 out of 13 files** (31%) were heavily refactored in Phase 2 to use `wait-helpers.ts`.

### 2. Key Differences: Local vs CI

| Aspect | Local (Passing) | CI (Failing) |
|--------|----------------|--------------|
| **Workers** | `undefined` (CPU cores / 2) | `1` (sequential) |
| **Retries** | `0` | `2` |
| **Environment** | Native Node | Docker container |
| **Coverage** | Off by default | Off (`PLAYWRIGHT_COVERAGE=0`) |
| **Parallel** | `fullyParallel: true` | `fullyParallel: true` |

### 3. Phase 2 Refactoring Impact

**Files Refactored:**
- `tests/core/access-lists-crud.spec.ts` - 32 timeout replacements
- `tests/core/authentication.spec.ts` - 1 timeout replacement
- `tests/core/certificates.spec.ts` - 20 timeout replacements
- `tests/core/proxy-hosts.spec.ts` - 38 timeout replacements

**Total:** 91 timeout replacements in Shard 1 using new wait helpers.

---

## Root Cause Hypothesis

### HYPOTHESIS 1: Dynamic Import Resolution in CI (HIGH PROBABILITY - 85%)

**Evidence:**
1. `wait-helpers.ts` uses dynamic imports of `ui-helpers.ts`:
   ```typescript
   // Line 69-70 in clickAndWaitForResponse
   const { clickSwitch } = await import('./ui-helpers');

   // Line 108-109 in clickSwitchAndWaitForResponse
   const { clickSwitch } = await import('./ui-helpers');
   ```

2. CI environment (Docker + single worker) might have slower module resolution
3. Dynamic imports are async and can cause race conditions during module initialization
4. All 4 refactored files in Shard 1 use these helpers extensively

**Why This Causes Shard 1 Failure:**
- Shard 1 is the **first shard** to run these refactored tests
- Module cache might not be warm yet in CI
- Subsequent shards benefit from cached module resolution
- Single worker in CI serializes execution, potentially exposing timing issues

**Why It Passes Locally:**
- Multiple workers pre-warm module cache
- Native filesystem has faster module resolution
- Parallel execution hides the timing issue

### HYPOTHESIS 2: Import Statement Conflict (MEDIUM PROBABILITY - 60%)

**Evidence:**
1. `wait-helpers.ts` imports from `@bgotink/playwright-coverage`:
   ```typescript
   import { expect } from '@bgotink/playwright-coverage';
   ```

2. But test files import `expect` via `auth-fixtures.ts`:
   ```typescript
   // In test files:
   import { test, expect, loginUser } from '../fixtures/auth-fixtures';

   // In auth-fixtures.ts:
   import { test as base, expect } from '@bgotink/playwright-coverage';
   ```

3. Circular dependency warning in code comments suggests this was a known issue

**Why This Causes Problems:**
- Two different `expect` instances might be in scope
- Module initialization order matters in CI (single worker, sequential)
- TypeScript types might conflict at runtime

### HYPOTHESIS 3: CI-Specific Timing Issue (LOW PROBABILITY - 30%)

**Evidence:**
- CI containers might be slower/overloaded
- `workers: 1` serializes tests, exposing race conditions
- Wait helpers might timeout differently in CI

**Why This is Less Likely:**
- Would affect random tests, not specifically Shard 1
- Timeout values are already high (60s for feature flags)
- Shards 2 & 3 pass, suggesting timing is adequate

---

## Surgical Fix Strategy

### Phase 1: Remove Dynamic Imports (HIGH IMPACT)

**Objective:** Eliminate async module resolution in hot paths

**Changes:**

1. **Convert dynamic imports to static imports in `wait-helpers.ts`:**

```typescript
// BEFORE (Line 5):
import type { Page, Locator, Response } from '@playwright/test';

// AFTER:
import type { Page, Locator, Response } from '@playwright/test';
import { clickSwitch } from './ui-helpers'; // ✅ Static import
```

2. **Remove dynamic imports in functions:**

```typescript
// BEFORE (Lines 69-70 in clickAndWaitForResponse):
const { clickSwitch } = await import('./ui-helpers');

// AFTER:
// Use imported clickSwitch directly (already imported at top)
```

```typescript
// BEFORE (Lines 108-109 in clickSwitchAndWaitForResponse):
const { clickSwitch } = await import('./ui-helpers');

// AFTER:
// Use imported clickSwitch directly
```

**Impact:**
- Eliminates 2 dynamic import calls in hot paths
- Removes async module resolution overhead
- Simplifies module dependency graph

**Risk:** LOW - Static imports are standard practice

---

### Phase 2: Verify No Circular Dependencies (SAFETY CHECK)

**Objective:** Ensure static imports don't introduce cycles

**Steps:**

1. Check if `ui-helpers.ts` imports `wait-helpers.ts`:
   ```bash
   grep -n "wait-helpers" tests/utils/ui-helpers.ts
   ```

   **Expected:** No results (no circular dependency)

2. Verify import order doesn't cause issues:
   - `wait-helpers.ts` imports `ui-helpers.ts` ✅
   - `ui-helpers.ts` does NOT import `wait-helpers.ts` ✅

**If circular dependency exists:**
- Extract shared types to `types.ts`
- Use type-only imports: `import type { ... }`

---

### Phase 3: Unified Expect Import (CONSISTENCY)

**Objective:** Ensure single source of truth for `expect`

**Current State:**
- `wait-helpers.ts`: `import { expect } from '@bgotink/playwright-coverage'`
- `auth-fixtures.ts`: `export { test, expect }` (re-exported from coverage lib)
- Test files: import via `auth-fixtures.ts`

**Recommendation:**
- Keep current pattern (correct)
- But verify `wait-helpers.ts` doesn't need `expect` directly
- If needed, import from `auth-fixtures.ts` for consistency

**Action:**
1. Search for `expect()` usage in `wait-helpers.ts`:
   ```bash
   grep -n "await expect(" tests/utils/wait-helpers.ts
   ```

2. If found, change import:
   ```typescript
   // BEFORE:
   import { expect } from '@bgotink/playwright-coverage';

   // AFTER:
   import { expect } from '../fixtures/auth-fixtures';
   ```

---

## Implementation Plan

### Step 1: Apply Dynamic Import Fix (5 minutes)

**File:** `tests/utils/wait-helpers.ts`

**Edits:**
1. Add static import at top
2. Remove 2 dynamic import statements

### Step 2: Verify No Circular Dependencies (2 minutes)

**Commands:**
```bash
grep -rn "wait-helpers" tests/utils/ui-helpers.ts
```

### Step 3: Test Locally (5 minutes)

**Commands:**
```bash
# Run Shard 1 locally to verify fix
cd /projects/Charon
CI=true npx playwright test --shard=1/4 --project=chromium
```

**Expected:** All tests pass

### Step 4: Commit and Push (2 minutes)

**Commit Message:**
```
fix(e2e): replace dynamic imports with static imports in wait-helpers

- Convert `await import('./ui-helpers')` to static import
- Eliminates async module resolution in CI environment
- Fixes Shard 1 failures across all browsers (Chromium/Firefox/WebKit)

Root Cause:
- Dynamic imports in wait-helpers.ts caused race conditions in CI
- CI uses single worker (workers: 1), exposing timing issues
- Shard 1 contains 4 refactored files using wait-helpers extensively
- Static imports resolve at module load time, avoiding runtime overhead

Impact:
- Shard 1: Fixed (4 refactored files now stable)
- Shards 2-3: No change (already passing)
- Local tests: No impact (already passing)

Verification:
- Tested locally with CI=true environment
- No circular dependencies detected
- Module dependency graph simplified

Closes https://github.com/Wikid82/Charon/actions/runs/21613888904
```

### Step 5: Monitor CI (15 minutes)

**Watch:**
- Shard 1 Chromium job
- Shard 1 Firefox job
- Shard 1 WebKit job

**Success Criteria:**
- All 3 Shard 1 jobs pass
- No new failures in Shards 2-4

---

## Validation Checklist

- [ ] Dynamic imports removed from `wait-helpers.ts`
- [ ] Static import added at top of file
- [ ] No circular dependencies detected
- [ ] Local test run passes: `CI=true npx playwright test --shard=1/4 --project=chromium`
- [ ] Code committed with descriptive message
- [ ] CI pipeline triggered
- [ ] Shard 1 Chromium passes
- [ ] Shard 1 Firefox passes
- [ ] Shard 1 WebKit passes
- [ ] Shards 2-3 still pass (no regression)
- [ ] GitHub issue updated with resolution

---

## Rollback Plan

**If fix fails:**

1. **Revert commit:**
   ```bash
   git revert HEAD
   git push
   ```

2. **Alternative fix:** Convert all wait-helper calls to inline implementations
   - More invasive
   - Estimated time: 4-6 hours
   - Last resort only

3. **Emergency workaround:** Skip Shard 1 tests temporarily
   - Not recommended (hides problem)
   - Reduces test coverage by 25%

---

## Success Metrics

**Before Fix:**
- ❌ Shard 1 Chromium: Failed
- ❌ Shard 1 Firefox: Failed
- ❌ Shard 1 WebKit: Failed
- ✅ Shard 2-3 (all browsers): Passed
- **Success Rate:** 50% (6/12 jobs)

**After Fix (Expected):**
- ✅ Shard 1 Chromium: Pass
- ✅ Shard 1 Firefox: Pass
- ✅ Shard 1 WebKit: Pass
- ✅ Shard 2-3 (all browsers): Pass
- **Success Rate:** 100% (12/12 jobs)

**Target:** 100% CI pass rate across all shards and browsers

---

## Timeline

| Step | Duration | Cumulative |
|------|----------|------------|
| 1. Apply fix | 5 min | 5 min |
| 2. Verify no circular deps | 2 min | 7 min |
| 3. Test locally | 5 min | 12 min |
| 4. Commit & push | 2 min | 14 min |
| 5. Monitor CI | 15 min | 29 min |
| **Total** | **29 min** | - |

**Buffer for issues:** +30 min
**Total estimated time:** **1 hour**

---

## References

- **CI Run:** https://github.com/Wikid82/Charon/actions/runs/21613888904
- **Phase 2 Refactoring:** `docs/plans/timeout_remediation_phase2.md`
- **Wait Helpers:** `tests/utils/wait-helpers.ts`
- **UI Helpers:** `tests/utils/ui-helpers.ts`
- **Auth Fixtures:** `tests/fixtures/auth-fixtures.ts`

---

## Next Steps

1. **Implement fix** (you)
2. **Validate locally** (you)
3. **Push to CI** (you)
4. **Monitor results** (you)
5. **Update this plan** with actual results
6. **Close GitHub action run issue** if successful

---

**Prepared by:** GitHub Copilot Planning Agent
**Reviewed by:** [Pending]
**Approved for implementation:** [Pending]
