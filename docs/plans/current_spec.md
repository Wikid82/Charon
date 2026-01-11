# Current Specification

**Status**: 🔴 ACTIVE - Docs-to-Issues CI Skip Investigation
**Last Updated**: 2026-01-11
**Current Work**: Resolving PR #461 CI check failures caused by docs-to-issues workflow

---

## Active Investigation: Docs-to-Issues CI Skip Problem

**Priority:** 🔴 HIGH - Blocking PR validation
**Reported:** PR #461
**Reference:** https://github.com/Wikid82/Charon/pull/461/changes/8bd0f9433a8455a1e2adb62ea80095488b8746ad

### Problem Statement

The `docs-to-issues.yml` workflow is preventing CI checks from appearing on PRs when it runs, breaking the PR review and merge process.

**Observed Behavior:**
1. When docs-to-issues workflow triggers on a PR, it creates an automated commit
2. The commit message contains `[skip ci]` flag
3. GitHub interprets this as "skip ALL CI workflows for this commit"
4. No required status checks run on the PR
5. PR cannot be properly validated or merged

**Impact:**
- **Blocks PR merges** - Required status checks never run
- **Breaks validation pipeline** - Tests, linting, security scans don't execute
- **Defeats CI/CD purpose** - Changes go unverified

### Root Cause Analysis

**Workflow Location:** `.github/workflows/docs-to-issues.yml`

**Problem Line:** Line 346
```yaml
git commit -m "chore: move processed issue files to created/ [skip ci]"
```

**Why [skip ci] Exists:**
The workflow uses `[skip ci]` to prevent an infinite loop:
1. Workflow runs on `push` to `docs/issues/**/*.md`
2. Creates GitHub issues from markdown files
3. Moves processed files to `docs/issues/created/`
4. Commits the file moves back to the same branch
5. Without `[skip ci]`, this commit would trigger the workflow again → infinite loop

**GitHub's [skip ci] Behavior:**
- **Scope:** `[skip ci]` skips ALL workflows triggered by `push` events for that commit
- **Alternative Syntax:** `[ci skip]`, `[no ci]`, `[skip actions]`, `***NO_CI***`
- **Documentation:** https://docs.github.com/en/actions/managing-workflow-runs/skipping-workflow-runs

**Why This Breaks PRs:**
- GitHub requires status checks to pass on the **latest commit** of a PR
- When the latest commit has `[skip ci]`, the required workflows don't run
- GitHub shows "Waiting for status to be reported" indefinitely
- PR is blocked from merging

### Current Workflow Behavior

**Trigger Conditions:**
```yaml
on:
  push:
    branches:
      - main
      - development
      - feature/**
    paths:
      - 'docs/issues/**/*.md'
      - '!docs/issues/created/**'
```

**Process Flow:**
1. Detects new/modified markdown files in `docs/issues/`
2. Parses frontmatter (title, labels, assignees)
3. Creates GitHub issues via API
4. Moves processed files to `docs/issues/created/YYYYMMDD-filename.md`
5. Commits file moves with `[skip ci]`
6. Pushes commit to same branch

**Protection Mechanism:**
- `if: github.actor != 'github-actions[bot]'` - Prevents bot-triggered loops
- `[skip ci]` - Prevents push-triggered loops

### Investigation Results

**Other Workflows That Commit:**

1. **auto-versioning.yml** (Line 69):
   - Creates and pushes tags: `git push origin "${TAG}"`
   - **No [skip ci]** - but only pushes tags, not commits
   - Tags don't trigger `on: push` workflows

2. **propagate-changes.yml**:
   - Creates PRs instead of committing directly
   - **No [skip ci]** - uses PR mechanism instead
   - Avoids the problem entirely

3. **auto-changelog.yml**:
   - Uses `release-drafter` action (no direct commits)
   - **No [skip ci]** - doesn't create commits

**Pattern Analysis:**
- ✅ **Best Practice:** Create separate PRs for automated changes (propagate-changes)
- ✅ **Alternative:** Use tags instead of commits (auto-versioning)
- ❌ **Anti-pattern:** Direct commits with [skip ci] (docs-to-issues)

### Solution Options

#### Option 1: Remove [skip ci] + Add Path Filter (RECOMMENDED)

**Changes:**
1. Remove `[skip ci]` from commit message
2. Add path exclusion to workflow trigger:
   ```yaml
   on:
     push:
       paths:
         - 'docs/issues/**/*.md'
         - '!docs/issues/created/**'  # Already present
   ```
3. Rely on existing `github.actor != 'github-actions[bot]'` guard

**Pros:**
- ✅ CI checks run normally on PRs
- ✅ Still prevents infinite loops (path filter + actor guard)
- ✅ Minimal code changes
- ✅ No workflow complexity increase

**Cons:**
- ⚠️ Adds one extra workflow run per docs-to-issues execution (the file move commit)
- ⚠️ Slight increase in Actions minutes usage (~30 seconds per run)

**Risk Assessment:** LOW
- Path filter already excludes `docs/issues/created/**`
- Actor guard prevents bot loops
- Double protection against infinite loops

#### Option 2: Create Separate PR for File Moves

**Changes:**
1. Instead of committing directly, create a PR with file moves
2. Use `actions/github-script` to create PR via API
3. Auto-merge PR or leave for manual review

**Pros:**
- ✅ No [skip ci] needed
- ✅ CI runs on all commits
- ✅ Audit trail via PRs
- ✅ Follows propagate-changes pattern

**Cons:**
- ❌ High complexity - requires significant refactoring
- ❌ Creates many auto-PRs (noise)
- ❌ Slower - PR review process adds delay
- ❌ Requires CPMP_TOKEN or similar for auto-merge

**Risk Assessment:** MEDIUM
- More moving parts
- Could create PR spam
- Requires additional token management

#### Option 3: Don't Commit File Moves

**Changes:**
1. Remove the "Move processed files" and "Commit moved files" steps
2. Let files remain in `docs/issues/` after processing
3. Add `.gitignore` for processed files or manual cleanup

**Pros:**
- ✅ No [skip ci] needed
- ✅ Simplest solution
- ✅ CI runs normally

**Cons:**
- ❌ Processed files accumulate in `docs/issues/`
- ❌ No clear separation of pending vs. completed
- ❌ Requires manual cleanup or gitignore maintenance
- ❌ Loses historical record of when issues were created

**Risk Assessment:** MEDIUM
- Defeats organizational purpose of the workflow
- Clutters the docs/issues directory

#### Option 4: Use [skip ci] Only on Main/Development

**Changes:**
1. Add conditional to only skip CI on protected branches:
   ```yaml
   - name: Commit moved files
     run: |
       if [[ "${{ github.ref }}" == "refs/heads/main" || "${{ github.ref }}" == "refs/heads/development" ]]; then
         git commit -m "chore: move processed issue files [skip ci]"
       else
         git commit -m "chore: move processed issue files"
       fi
   ```

**Pros:**
- ✅ CI runs on PRs (feature branches)
- ✅ Skips CI on main/dev (reduces noise)
- ✅ Balanced approach

**Cons:**
- ⚠️ Still causes double runs on feature branches
- ⚠️ Adds branch-specific logic complexity

**Risk Assessment:** LOW
- Good compromise solution
- Preserves optimization on main branches
- Enables validation on PRs

### Recommended Solution: Option 1 (Remove [skip ci])

**Justification:**
1. **Solves the problem completely** - CI checks will run on PRs
2. **Low risk** - Multiple safeguards prevent infinite loops
3. **Minimal changes** - One-line modification
4. **Performance impact is negligible** - ~30 seconds extra per run
5. **Aligns with CI/CD principles** - All changes should be validated

**Implementation Steps:**

1. **Modify `.github/workflows/docs-to-issues.yml` line 346:**
   ```yaml
   # BEFORE:
   git commit -m "chore: move processed issue files to created/ [skip ci]"

   # AFTER:
   git commit -m "chore: move processed issue files to created/"
   ```

2. **Verify safeguards are in place:**
   - ✅ Line 45: `if: github.actor != 'github-actions[bot]'`
   - ✅ Lines 11-12: Path exclusions for `docs/issues/created/**`

3. **Test on feature branch:**
   - Create test markdown file in `docs/issues/test-issue.md`
   - Push to feature branch
   - Verify workflow runs and creates issue
   - Verify file moves to `created/`
   - Verify commit triggers ONE more workflow run
   - Verify second run skips (no files in `docs/issues/`)
   - Verify CI checks appear on PR

4. **Monitor for infinite loops:**
   - Check workflow run history
   - Confirm only 2 runs per docs change (original + file move)
   - No cascading triggers

### Success Criteria

- ✅ CI checks run and appear on PRs when docs-to-issues workflow executes
- ✅ No infinite workflow loops occur
- ✅ File moves still happen correctly
- ✅ Issues are still created properly
- ✅ PR #461 can be validated and merged
- ✅ No increase in workflow failures

### Rollback Plan

If Option 1 causes problems:
1. Revert commit (one-line change)
2. Implement Option 4 (conditional [skip ci]) as fallback
3. Re-test on feature branch

### Documentation Updates

After implementation:
- [ ] Update `.github/workflows/docs-to-issues.yml` with inline comments
- [ ] Document decision in implementation summary
- [ ] Update CHANGELOG.md
- [ ] Add note to workflow documentation (if exists)

---

## Completed Work

### CI/CD Workflow Fixes (2026-01-11) ✅

**Status:** Complete - All documentation finalized

The CI workflow investigation and documentation has been completed. Both issues were determined to be false positives or expected GitHub behavior with no security gaps.

**Final Documentation:**
- **Implementation Summary**: [docs/implementation/CI_WORKFLOW_FIXES_2026-01-11.md](../implementation/CI_WORKFLOW_FIXES_2026-01-11.md)
- **QA Report**: [docs/reports/qa_report.md](../reports/qa_report.md)
- **Archived Plan**: [docs/plans/archive/GITHUB_SECURITY_WARNING_RESOLUTION_PLAN_2026-01-11.md](archive/GITHUB_SECURITY_WARNING_RESOLUTION_PLAN_2026-01-11.md)

**Changes Made:**
- ✅ Workflow files documented with explanatory comments
- ✅ SECURITY.md updated with comprehensive scanning coverage
- ✅ CHANGELOG.md updated with workflow migration entry
- ✅ Implementation summary created
- ✅ All validation tests passed (CodeQL, Trivy, pre-commit)
- ✅ Planning docs archived

**Merge Status:** ✅ SAFE TO MERGE - Zero security gaps, fully documented

---

## Active Projects

*Ready for next task*

---

## Recently Completed

### Workflow Orchestration Fix (2026-01-11)

Successfully fixed workflow orchestration issue where supply-chain-verify was running before docker-build completed, causing verification to skip on PRs.

**Documentation**:

- **Implementation Summary**: [docs/implementation/WORKFLOW_ORCHESTRATION_FIX.md](../implementation/WORKFLOW_ORCHESTRATION_FIX.md)
- **QA Report**: [docs/reports/qa_report_workflow_orchestration.md](../reports/qa_report_workflow_orchestration.md)
- **Archived Plan**: [docs/plans/archive/workflow_orchestration_fix_2026-01-11.md](archive/workflow_orchestration_fix_2026-01-11.md)

**Status**: ✅ Complete - Deployed to production

---

### Grype SBOM Remediation (2026-01-10)

Successfully resolved CI/CD failures in the Supply Chain Verification workflow caused by Grype SBOM format mismatch.

**Documentation**:
- **Implementation Summary**: [docs/implementation/GRYPE_SBOM_REMEDIATION.md](../implementation/GRYPE_SBOM_REMEDIATION.md)
- **QA Report**: [docs/reports/qa_report.md](../reports/qa_report.md)
- **Archived Plan**: [docs/plans/archive/grype_sbom_remediation_2026-01-10.md](archive/grype_sbom_remediation_2026-01-10.md)

**Status**: ✅ Complete - Deployed to production

---

## Guidelines for Creating New Specs

When starting a new project, create a detailed specification in this file following the [Spec-Driven Workflow v1](.github/instructions/spec-driven-workflow-v1.instructions.md) format.

### Required Sections

1. **Problem Statement** - What issue are we solving?
2. **Root Cause Analysis** - Why does the problem exist?
3. **Solution Design** - How will we solve it?
4. **Implementation Plan** - Step-by-step tasks
5. **Testing Strategy** - How will we validate success?
6. **Success Criteria** - What defines "done"?

### Archiving Completed Specs

When a specification is complete:

1. Create implementation summary in `docs/implementation/`
2. Move spec to `docs/plans/archive/` with timestamp
3. Update this file with completion notice

---

## Archive Location

Completed and archived specifications can be found in:
- [docs/plans/archive/](archive/)

---

**Note**: This file should only contain ONE active specification at a time. Archive completed work before starting new projects.
