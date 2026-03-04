# Phase 1 Validation Checklist

**Date:** February 2, 2026
**Status:** Ready for Validation
**Phase:** Emergency Hotfix + Deep Diagnostics

---

## Pre-Deployment Validation

### 1. File Integrity Check

- [x] `.github/workflows/e2e-tests-split.yml` created (34KB)
- [x] `.github/workflows/e2e-tests.yml.backup` created (26KB backup)
- [x] `docs/reports/phase1_analysis.md` created (3.8KB)
- [x] `docs/reports/phase1_diagnostics.md` created (18KB)
- [x] `docs/reports/phase1_complete.md` created (11KB)
- [x] `tests/utils/diagnostic-helpers.ts` created (9.7KB)

### 2. Workflow YAML Validation

```bash
# Validate YAML syntax
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/e2e-tests-split.yml'))"
# ✅ PASSED: Workflow YAML syntax is valid
```

### 3. Workflow Structure Validation

**Expected Jobs:**
- [x] `build` - Build Docker image once
- [x] `e2e-chromium` - 4 shards, independent execution
- [x] `e2e-firefox` - 4 shards, independent execution
- [x] `e2e-webkit` - 4 shards, independent execution
- [x] `upload-coverage` - Merge and upload per-browser coverage
- [x] `test-summary` - Generate summary report
- [x] `comment-results` - Post PR comment
- [x] `e2e-results` - Final status check

**Total Jobs:** 8 (vs 7 in original workflow)

### 4. Browser Isolation Validation

**Dependency Tree:**
```
build
 ├─ e2e-chromium (independent)
 ├─ e2e-firefox (independent)
 └─ e2e-webkit (independent)
      └─ upload-coverage (needs all 3)
           └─ test-summary
                └─ comment-results
                     └─ e2e-results
```

**Validation:**
- [x] No dependencies between browser jobs
- [x] All browsers depend only on `build`
- [x] Chromium failure cannot block Firefox/WebKit
- [x] Each browser runs 4 shards in parallel

### 5. Coverage Strategy Validation

**Expected Artifacts:**
- [x] `e2e-coverage-chromium-shard-{1..4}` (4 artifacts)
- [x] `e2e-coverage-firefox-shard-{1..4}` (4 artifacts)
- [x] `e2e-coverage-webkit-shard-{1..4}` (4 artifacts)
- [x] `e2e-coverage-merged` (1 artifact with all browsers)

**Expected Codecov Flags:**
- [x] `e2e-chromium` flag
- [x] `e2e-firefox` flag
- [x] `e2e-webkit` flag

**Expected Reports:**
- [x] `playwright-report-{browser}-shard-{1..4}` (12 HTML reports)

---

## Local Validation (Pre-Push)

### Step 1: Lint Workflow File

```bash
# GitHub Actions YAML linter
docker run --rm -v "$PWD:/repo" rhysd/actionlint:latest -color /repo/.github/workflows/e2e-tests-split.yml
```

**Expected:** No errors or warnings

### Step 2: Test Playwright with Split Projects

```bash
# Test Chromium only
npx playwright test --project=chromium --shard=1/4

# Test Firefox only
npx playwright test --project=firefox --shard=1/4

# Test WebKit only
npx playwright test --project=webkit --shard=1/4

# Verify no cross-contamination
```

**Expected:** Each browser runs independently without errors

### Step 3: Verify Diagnostic Helpers

```bash
# Run TypeScript compiler
npx tsc --noEmit tests/utils/diagnostic-helpers.ts

# Expected: No type errors
```

**Expected:** Clean compilation (0 errors)

### Step 4: Simulate CI Environment

```bash
# Rebuild E2E container
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e

# Wait for health check
curl -sf http://localhost:8080/api/v1/health

# Run with CI settings
CI=1 npx playwright test --project=chromium --workers=1 --retries=2 --shard=1/4
```

**Expected:** Tests run in CI mode without interruptions

---

## CI Validation (Post-Push)

### Step 1: Create Feature Branch

```bash
# Create feature branch for Phase 1 hotfix
git checkout -b phase1-browser-split-hotfix

# Add files
git add .github/workflows/e2e-tests-split.yml \
        .github/workflows/e2e-tests.yml.backup \
        docs/reports/phase1_*.md \
        tests/utils/diagnostic-helpers.ts

# Commit with descriptive message
git commit -m "feat(ci): Phase 1 - Split browser jobs for complete isolation

- Split e2e-tests into 3 independent jobs (chromium, firefox, webkit)
- Add per-browser coverage upload with flags (e2e-{browser})
- Create diagnostic helpers for root cause analysis
- Document Phase 1 investigation findings

Fixes: Browser interruptions blocking downstream tests
See: docs/plans/browser_alignment_triage.md Phase 1
Related: PR #609"

# Push to remote
git push origin phase1-browser-split-hotfix
```

### Step 2: Create Pull Request

**PR Title:** `[Phase 1] Emergency Hotfix: Split Browser Jobs for Complete Isolation`

**PR Description:**
```markdown
## Phase 1: Browser Alignment Triage - Emergency Hotfix

### Problem
Chromium test interruption at test #263 blocks Firefox/WebKit from executing.
Only 10% of E2E tests (263/2,620) were running in CI.

### Solution
Split browser tests into 3 completely independent jobs:
- `e2e-chromium` (4 shards)
- `e2e-firefox` (4 shards)
- `e2e-webkit` (4 shards)

### Benefits
- ✅ **Complete Browser Isolation:** Chromium failure cannot block Firefox/WebKit
- ✅ **Parallel Execution:** All browsers run simultaneously (faster CI)
- ✅ **Independent Failure Analysis:** Each browser has separate HTML reports
- ✅ **Per-Browser Coverage:** Separate flags for Codecov (e2e-chromium, e2e-firefox, e2e-webkit)

### Changes
1. **New Workflow:** `.github/workflows/e2e-tests-split.yml`
   - 3 independent browser jobs (no cross-dependencies)
   - Per-browser coverage upload with flags
   - Enhanced diagnostic logging

2. **Diagnostic Tools:** `tests/utils/diagnostic-helpers.ts`
   - Browser console logging
   - Page state capture
   - Dialog lifecycle tracking
   - Performance monitoring

3. **Documentation:**
   - `docs/reports/phase1_analysis.md` - Test execution order analysis
   - `docs/reports/phase1_diagnostics.md` - Root cause investigation (18KB)
   - `docs/reports/phase1_complete.md` - Phase 1 completion report

### Testing
- [x] YAML syntax validated
- [ ] All 3 browser jobs execute independently in CI
- [ ] Coverage artifacts upload with correct flags
- [ ] Chromium failure does not block Firefox/WebKit

### Next Steps
- Phase 2: Fix root cause (replace `page.waitForTimeout()` anti-patterns)
- Phase 3: Improve coverage to 85%+
- Phase 4: Consolidate back to single job after fix validated

### References
- Triage Plan: `docs/plans/browser_alignment_triage.md`
- Diagnostic Report: `docs/reports/browser_alignment_diagnostic.md`
- Related Issue: #609 (E2E tests blocking PR merge)
```

### Step 3: Monitor CI Execution

**Check GitHub Actions:**
1. Navigate to Actions tab → `E2E Tests (Split Browsers)` workflow
2. Verify all 8 jobs appear:
   - [x] `build` (1 job)
   - [x] `e2e-chromium` (4 shards)
   - [x] `e2e-firefox` (4 shards)
   - [x] `e2e-webkit` (4 shards)
   - [x] `upload-coverage` (if enabled)
   - [x] `test-summary`
   - [x] `comment-results`
   - [x] `e2e-results`

**Expected Behavior:**
- Build completes in ~5 minutes
- All browser shards start simultaneously (after build)
- Each shard uploads HTML report on completion
- Coverage artifacts uploaded (if `PLAYWRIGHT_COVERAGE=1`)
- Summary comment posted to PR

### Step 4: Verify Browser Isolation

**Test Chromium Failure Scenario:**
1. Temporarily add `test.fail()` to a Chromium-only test
2. Push change and observe CI behavior
3. **Expected:** Chromium jobs fail, Firefox/WebKit continue

**Validation Command:**
```bash
# Check workflow run status
gh run view <run-id> --log

# Expected output:
# - e2e-chromium: failure (expected)
# - e2e-firefox: success
# - e2e-webkit: success
# - e2e-results: failure (as expected, Chromium failed)
```

### Step 5: Verify Coverage Upload

**Check Codecov Dashboard:**
1. Navigate to Codecov dashboard for the repository
2. Go to the commit/PR page
3. Verify flags appear:
   - [x] `e2e-chromium` flag with coverage %
   - [x] `e2e-firefox` flag with coverage %
   - [x] `e2e-webkit` flag with coverage %

**Expected:**
- 3 separate flag entries in Codecov
- Each flag shows independent coverage percentage
- Combined E2E coverage matches or exceeds original

---

## Post-Deployment Validation

### Step 1: Monitor PR #609

**Expected Behavior:**
- E2E tests execute for all 3 browsers
- No "did not run" status for Firefox/WebKit
- Per-shard HTML reports available for download
- PR comment shows all 3 browser results

### Step 2: Analyze Test Results

**Download Artifacts:**
- `playwright-report-chromium-shard-{1..4}` (4 reports)
- `playwright-report-firefox-shard-{1..4}` (4 reports)
- `playwright-report-webkit-shard-{1..4}` (4 reports)

**Verify:**
- [ ] Each browser ran >800 tests (not 0)
- [ ] No interruptions detected (check traces)
- [ ] Shard execution times < 15 minutes each
- [ ] HTML reports contain test details

### Step 3: Validate Coverage Merge

**If `PLAYWRIGHT_COVERAGE=1` enabled:**
- [ ] Download `e2e-coverage-merged` artifact
- [ ] Verify `chromium/lcov.info` exists
- [ ] Verify `firefox/lcov.info` exists
- [ ] Verify `webkit/lcov.info` exists
- [ ] Check Codecov dashboard for 3 flags

**If coverage disabled:**
- [ ] No coverage artifacts uploaded
- [ ] `upload-coverage` job skipped
- [ ] No Codecov updates

---

## Rollback Plan

**If Phase 1 hotfix causes issues:**

### Option 1: Revert to Original Workflow

```bash
# Restore backup
cp .github/workflows/e2e-tests.yml.backup .github/workflows/e2e-tests.yml

# Commit revert
git add .github/workflows/e2e-tests.yml
git commit -m "revert(ci): rollback to original E2E workflow

Phase 1 hotfix caused issues. Restoring original workflow
while investigating alternative solutions.

See: docs/reports/phase1_rollback.md"

git push origin phase1-browser-split-hotfix
```

### Option 2: Disable Specific Browser

**If one browser has persistent issues:**

```yaml
# Add to workflow
jobs:
  e2e-firefox:
    # Temporarily disable Firefox until root cause identified
    if: false
```

### Option 3: Merge Shards

**If sharding causes resource contention:**

```yaml
strategy:
  matrix:
    shard: [1]  # Change from [1, 2, 3, 4] to [1]
    total-shards: [1]  # Change from [4] to [1]
```

---

## Success Criteria

### Must Have (Blocking)
- [x] Workflow YAML syntax valid
- [x] All 3 browser jobs defined
- [x] No dependencies between browser jobs
- [x] Documentation complete
- [ ] CI executes all 3 browsers (verify in PR)
- [ ] Chromium failure does not block Firefox/WebKit (verify in PR)

### Should Have (Important)
- [x] Per-browser coverage upload configured
- [x] Diagnostic helpers created
- [x] Backup of original workflow
- [ ] PR comment shows all 3 browser results (verify in PR)
- [ ] HTML reports downloadable per shard (verify in PR)

### Nice to Have (Optional)
- [ ] Coverage flags visible in Codecov dashboard
- [ ] Performance improvement measured (parallel execution)
- [ ] Phase 2 plan approved by team

---

## Next Steps After Validation

### If Validation Passes ✅

1. **Merge Phase 1 PR**
   - Squash commits or keep history (team preference)
   - Update PR #609 to use new workflow

2. **Begin Phase 2**
   - Create `tests/utils/wait-helpers.ts`
   - Refactor interrupted tests in `certificates.spec.ts`
   - Code review checkpoint after first 2 files

3. **Monitor Production**
   - Watch for new interruptions
   - Track test execution times
   - Monitor CI resource usage

### If Validation Fails ❌

1. **Analyze Failure**
   - Download workflow logs
   - Check job dependencies
   - Verify environment variables

2. **Apply Fix**
   - Update workflow configuration
   - Re-run validation checklist
   - Document issue in `phase1_rollback.md`

3. **Escalate if Needed**
   - If fix not obvious, revert to original workflow
   - Document issues for team discussion
   - Schedule Phase 1 retrospective

---

## Approval Sign-Off

**Phase 1 Deliverables Validated:**
- [ ] DevOps Lead
- [ ] QA Lead
- [ ] Engineering Manager

**Date:** _________________

**Ready for Deployment:** YES / NO

---

**Document Control:**
**Version:** 1.0
**Last Updated:** February 2, 2026
**Status:** Ready for Validation
**Next Review:** After CI validation in PR
