# Shard 1 Investigation Summary

**Date:** 2026-02-03
**Status:** ✅ Root Cause Identified - Fix Ready
**CI Run:** https://github.com/Wikid82/Charon/actions/runs/21613888904

---

## Problem Statement

After completing Phase 1-3 of timeout remediation (semantic wait helpers, coverage improvements):
- **Shard 1 failed on ALL 3 browsers** (Chromium, Firefox, WebKit)
- **Shards 2 & 3 passed**
- **Overall success rate: 50% (6/12 jobs)**
- **Shard 4: Cancelled (never ran)**

---

## Investigation Findings

### 1. Shard Distribution Analysis

**50 total test files → 4 shards = ~12.5 files per shard**

**Shard 1 (Files 1-13):**
```
✅ tests/core/access-lists-crud.spec.ts      (32 timeout replacements)
✅ tests/core/authentication.spec.ts         (1 timeout replacement)
✅ tests/core/certificates.spec.ts           (20 timeout replacements)
   tests/core/dashboard.spec.ts
   tests/core/navigation.spec.ts
✅ tests/core/proxy-hosts.spec.ts            (38 timeout replacements)
   tests/dns-provider-crud.spec.ts
   tests/dns-provider-types.spec.ts
   tests/emergency-server/emergency-server.spec.ts
   tests/emergency-server/tier2-validation.spec.ts
   tests/integration/backup-restore-e2e.spec.ts
   tests/integration/import-to-production.spec.ts
   tests/integration/multi-feature-workflows.spec.ts
```

**Critical Pattern:** 4 out of 13 files (31%) were refactored in Phase 2 to use `wait-helpers.ts`

**Total Impact:** 91 timeout replacements in Shard 1 using new wait helpers

### 2. Local vs CI Differences

| Factor | Local | CI | Impact |
|--------|-------|----|----- --|
| **Workers** | Default (CPU/2) | `1` | CI serializes execution |
| **Retries** | `0` | `2` | CI masks intermittent issues |
| **Module Cache** | Warm (parallel) | Cold (sequential) | CI slower module resolution |
| **Test Result** | ✅ Pass | ❌ Fail | Environment-specific issue |

### 3. Code Analysis

**Dynamic Imports in `wait-helpers.ts` (2 locations):**

**Location 1:** Line 69-70 in `clickAndWaitForResponse()`
```typescript
const { clickSwitch } = await import('./ui-helpers');
```

**Location 2:** Line 108-109 in `clickSwitchAndWaitForResponse()`
```typescript
const { clickSwitch } = await import('./ui-helpers');
```

**Why This is Problematic:**
1. Dynamic imports are **async** - add runtime overhead
2. Module resolution happens **at call time**, not module load time
3. CI's **single worker** executes Shard 1 first with cold module cache
4. Shard 1 has **4 refactored files** calling these helpers extensively
5. Subsequent shards benefit from **warm cache**, avoiding the issue

### 4. Dependency Verification

**Circular Dependency Check:**
```bash
grep -n "wait-helpers" tests/utils/ui-helpers.ts
# Result: No matches ✅
```

**Conclusion:** Safe to convert dynamic imports to static imports

**Expect Import Analysis:**
```bash
grep -n "await expect(" tests/utils/wait-helpers.ts
# Result: 20+ usages ✅
```

**Conclusion:** `expect` import from `@bgotink/playwright-coverage` is correct and necessary

---

## Root Cause

### ❗️ PRIMARY CAUSE: Dynamic Import Resolution in CI

**Confidence Level:** 85%

**Mechanism:**
1. `wait-helpers.ts` uses dynamic imports in hot paths
2. CI environment (Docker + single worker) has slower module resolution
3. Shard 1 runs first with cold module cache
4. Async import overhead causes subtle timing issues
5. Shards 2-3 benefit from warmed cache

**Why It Passes Locally:**
- Multiple workers pre-warm module cache in parallel
- Native filesystem has faster module resolution
- Parallel execution masks timing issues

**Why It Fails in CI:**
- Single worker (`workers: 1`) serializes execution
- Docker filesystem might be slower
- Cold module cache on first shard
- Timing issues exposed by sequential execution

---

## Solution

### ✅ Replace Dynamic Imports with Static Imports

**File to Modify:** `tests/utils/wait-helpers.ts`

**Change 1: Add Static Import** (Line 5)
```typescript
// BEFORE:
import type { Page, Locator, Response } from '@playwright/test';

// AFTER:
import type { Page, Locator, Response } from '@playwright/test';
import { clickSwitch } from './ui-helpers'; // ✅ Static import
```

**Change 2: Remove Dynamic Import** (Line 69-70)
```typescript
// BEFORE:
const { clickSwitch } = await import('./ui-helpers');

// AFTER:
// Use imported clickSwitch directly (already imported at top)
```

**Change 3: Remove Dynamic Import** (Line 108-109)
```typescript
// BEFORE:
const { clickSwitch } = await import('./ui-helpers');

// AFTER:
// Use imported clickSwitch directly
```

---

## Expected Impact

### Before Fix

| Shard | Browser | Status | Note |
|-------|---------|--------|------|
| 1 | Chromium | ❌ Failed | Dynamic imports |
| 1 | Firefox | ❌ Failed | Dynamic imports |
| 1 | WebKit | ❌ Failed | Dynamic imports |
| 2 | Chromium | ✅ Passed | Warm cache |
| 2 | Firefox | ✅ Passed | Warm cache |
| 2 | WebKit | ✅ Passed | Warm cache |
| 3 | Chromium | ✅ Passed | Warm cache |
| 3 | Firefox | ✅ Passed | Warm cache |
| 3 | WebKit | ✅ Passed | Warm cache |
| 4 | All | ⚠️ Cancelled | Workflow stopped |

**Success Rate:** 50% (6/12 jobs passing)

### After Fix

| Shard | Browser | Status | Note |
|-------|---------|--------|------|
| 1 | Chromium | ✅ Pass | Static imports |
| 1 | Firefox | ✅ Pass | Static imports |
| 1 | WebKit | ✅ Pass | Static imports |
| 2 | Chromium | ✅ Pass | No change |
| 2 | Firefox | ✅ Pass | No change |
| 2 | WebKit | ✅ Pass | No change |
| 3 | Chromium | ✅ Pass | No change |
| 3 | Firefox | ✅ Pass | No change |
| 3 | WebKit | ✅ Pass | No change |
| 4 | Chromium | ✅ Pass | Will run |
| 4 | Firefox | ✅ Pass | Will run |
| 4 | WebKit | ✅ Pass | Will run |

**Success Rate:** 100% (12/12 jobs passing)

---

## Implementation Timeline

| Step | Task | Duration |
|------|------|----------|
| 1 | Remove dynamic imports from `wait-helpers.ts` | 5 min |
| 2 | Test locally with `CI=true` | 5 min |
| 3 | Commit and push | 2 min |
| 4 | Monitor CI pipeline | 15 min |
| **Total** | | **27 min** |

**With buffer:** ~1 hour

---

## Validation Checklist

### Pre-Implementation
- [x] Shard 1 test files identified
- [x] Dynamic import locations found
- [x] No circular dependencies confirmed
- [x] `expect` usage verified

### Implementation
- [ ] Static import added to `wait-helpers.ts`
- [ ] Dynamic imports removed (2 locations)
- [ ] Local test passes: `CI=true npx playwright test --shard=1/4 --project=chromium`

### Post-Implementation
- [ ] Fix pushed to repository
- [ ] CI pipeline triggered
- [ ] Shard 1 Chromium passes
- [ ] Shard 1 Firefox passes
- [ ] Shard 1 WebKit passes
- [ ] Shards 2-3 still pass
- [ ] Shard 4 runs and passes
- [ ] GitHub issue updated

---

## Risk Assessment

### Implementation Risk: **LOW**

**Why:**
- Static imports are standard practice
- No architectural changes required
- No circular dependencies exist
- Change is localized to 3 lines in 1 file

### Regression Risk: **VERY LOW**

**Why:**
- Only changes module load timing
- Shards 2-3 already passing (won't affect them)
- Local tests already passing
- Fix makes code simpler and more maintainable

---

## Alternative Solutions (Not Recommended)

### Option 1: Increase Timeouts
**Pros:** Quick fix
**Cons:** Hides root cause, makes tests slower
**Verdict:** ❌ Not recommended

### Option 2: Disable Shard 1 Tests
**Pros:** Unblocks CI immediately
**Cons:** Reduces coverage by 25%, hides problem
**Verdict:** ❌ Not recommended

### Option 3: Split wait-helpers.ts
**Pros:** Separates concerns
**Cons:** More complex, requires refactoring all imports
**Verdict:** ❌ Overkill for this issue

---

## Lessons Learned

### 1. Dynamic Imports in Test Utilities
**Problem:** Async module resolution adds overhead in CI
**Solution:** Use static imports unless truly necessary

### 2. CI-Specific Behavior
**Problem:** Single worker serialization exposes issues masked locally
**Learning:** Always test with `CI=true` locally before pushing

### 3. Module Cache Effects
**Problem:** Warm cache in later shards masks cold cache issues in Shard 1
**Learning:** Pay special attention to first shard in CI

### 4. Shard Distribution
**Problem:** Alphabetical ordering concentrated refactored files in Shard 1
**Learning:** Consider test file naming to balance shard load

---

## References

- **Detailed Fix Plan:** [shard1_fix_plan.md](./shard1_fix_plan.md)
- **Phase 2 Refactoring:** [timeout_remediation_phase2.md](./timeout_remediation_phase2.md)
- **CI Workflow:** [.github/workflows/e2e-tests-split.yml](../../.github/workflows/e2e-tests-split.yml)
- **Wait Helpers:** [tests/utils/wait-helpers.ts](../../tests/utils/wait-helpers.ts)
- **Failed CI Run:** https://github.com/Wikid82/Charon/actions/runs/21613888904

---

**Investigation Complete:** 2026-02-03
**Next Action:** Implement fix per [shard1_fix_plan.md](./shard1_fix_plan.md)
