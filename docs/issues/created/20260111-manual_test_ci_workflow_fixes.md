# Manual Test Plan: CI Workflow Fixes

**Created:** 2026-01-11
**PR:** #461
**Feature:** CI/CD Workflow Documentation & Supply Chain Fix

## Objective

Manually verify that the CI workflow fixes work correctly in production, focusing on finding potential bugs in the Supply Chain Verification orchestration.

## Background

**What Was Fixed:**

1. Removed `branches` filter from `supply-chain-verify.yml` to enable `workflow_run` triggering on all branches
2. Added documentation to explain the GitHub Security warning (false positive)
3. Updated SECURITY.md with comprehensive security scanning documentation

**Expected Behavior:**

- Supply Chain Verification should now trigger via `workflow_run` after Docker Build completes on ANY branch
- Previous behavior: Only triggered via `pull_request` fallback (branch filter prevented workflow_run)

## Test Scenarios

### Scenario 1: Push to Feature Branch (workflow_run Test)

**Goal:** Verify `workflow_run` trigger works on feature branches after fix

**Steps:**

1. Create a small test commit on `feature/beta-release`
2. Push the commit
3. Monitor GitHub Actions workflow runs

**Expected Results:**

- ✅ Docker Build workflow triggers and completes successfully
- ✅ Supply Chain Verification triggers **via workflow_run event** (not pull_request)
- ✅ Supply Chain completes successfully
- ✅ GitHub Actions logs show event type is `workflow_run`

**How to Verify Event Type:**

```bash
gh run list --workflow="supply-chain-verify.yml" --limit 1 --json event,conclusion
# Should show: "event": "workflow_run", "conclusion": "success"
```

**Potential Bugs to Watch For:**

- ❌ Supply Chain doesn't trigger at all
- ❌ Supply Chain triggers but fails
- ❌ Multiple simultaneous runs (race condition)
- ❌ Timeout or hang in workflow_run chain

---

### Scenario 2: PR Synchronization (Fallback Still Works)

**Goal:** Verify `pull_request` fallback trigger still works correctly

**Steps:**

1. With PR #461 open, push another small commit
2. Monitor GitHub Actions workflow runs

**Expected Results:**

- ✅ Docker Build triggers via `pull_request` event
- ✅ Supply Chain may trigger via BOTH `workflow_run` AND `pull_request` (race condition possible)
- ✅ If both trigger, both should complete successfully without conflict
- ✅ PR should show both workflow checks passing

**Potential Bugs to Watch For:**

- ❌ Duplicate runs causing conflicts
- ❌ Race condition causing failures
- ❌ PR checks showing "pending" indefinitely
- ❌ One workflow cancels the other

---

### Scenario 3: Main Branch Push (Default Branch Behavior)

**Goal:** Verify fix doesn't break main branch behavior

**Steps:**

1. After PR #461 merges to main, monitor the merge commit
2. Check GitHub Actions runs

**Expected Results:**

- ✅ Docker Build runs on main
- ✅ Supply Chain triggers via `workflow_run`
- ✅ Both complete successfully
- ✅ Weekly scheduled runs continue to work

**Potential Bugs to Watch For:**

- ❌ Main branch workflows broken
- ❌ Weekly schedule interferes with workflow_run
- ❌ Permissions issues on main branch

---

### Scenario 4: Failed Docker Build (Error Handling)

**Goal:** Verify Supply Chain doesn't trigger when Docker Build fails

**Steps:**

1. Intentionally break Docker Build (e.g., invalid Dockerfile syntax)
2. Push to a test branch
3. Monitor workflow behavior

**Expected Results:**

- ✅ Docker Build fails as expected
- ✅ Supply Chain **does NOT trigger** (workflow_run only fires on `completed` and `success`)
- ✅ No cascading failures

**Potential Bugs to Watch For:**

- ❌ Supply Chain triggers on failed builds
- ❌ Error handling missing
- ❌ Workflow stuck in pending state

---

### Scenario 5: Manual Workflow Dispatch

**Goal:** Verify manual trigger still works

**Steps:**

1. Go to GitHub Actions → Supply Chain Verification
2. Click "Run workflow"
3. Select `feature/beta-release` branch
4. Click "Run workflow"

**Expected Results:**

- ✅ Workflow starts via `workflow_dispatch` event
- ✅ Completes successfully
- ✅ SBOM and attestations generated

**Potential Bugs to Watch For:**

- ❌ Manual dispatch broken
- ❌ Branch selector doesn't work
- ❌ Workflow fails with "branch not found"

---

### Scenario 6: Weekly Scheduled Run

**Goal:** Verify scheduled trigger still works

**Steps:**

1. Wait for next Monday 00:00 UTC
2. Check GitHub Actions for scheduled run

**Expected Results:**

- ✅ Workflow triggers via `schedule` event
- ✅ Runs on main branch
- ✅ Completes successfully

**Potential Bugs to Watch For:**

- ❌ Schedule doesn't fire
- ❌ Wrong branch selected
- ❌ Interference with other workflows

---

## Edge Cases to Test

### Edge Case 1: Rapid Pushes (Rate Limiting)

**Test:** Push 3-5 commits rapidly to feature branch
**Expected:** All Docker Builds run, Supply Chain may queue or skip redundant runs
**Watch For:** Workflow queue overflow, cancellations, failures

### Edge Case 2: Long-Running Docker Build

**Test:** Create a commit that makes Docker Build take >10 minutes
**Expected:** Supply Chain waits for completion before triggering
**Watch For:** Timeouts, abandoned runs, state corruption

### Edge Case 3: Branch Deletion During Run

**Test:** Delete feature branch while workflows are running
**Expected:** Workflows complete or cancel gracefully
**Watch For:** Orphaned runs, resource leaks, errors

---

## Success Criteria

- [ ] All 6 scenarios pass without critical bugs
- [ ] `workflow_run` event type confirmed in logs
- [ ] No cascading failures
- [ ] PR checks consistently pass
- [ ] Error handling works correctly
- [ ] Manual and scheduled triggers functional

## Bug Severity Guidelines

**CRITICAL** (Block Merge):

- Supply Chain doesn't run at all
- Cascading failures breaking other workflows
- Security vulnerabilities introduced

**HIGH** (Fix Before Release):

- Race conditions causing frequent failures
- Resource leaks or orphaned workflows
- Error handling missing

**MEDIUM** (Fix in Future PR):

- Duplicate runs (but both succeed)
- Inconsistent behavior (works sometimes)
- Minor UX issues

**LOW** (Document as Known Issue):

- Cosmetic issues in logs
- Non-breaking edge cases
- Timing inconsistencies

---

## Notes for Testers

1. **Event Type Verification is Critical:** The core fix was to enable `workflow_run` on feature branches. If logs still show only `pull_request` events, the fix didn't work.

2. **False Positives are OK:** The GitHub Security warning may persist for 4-8 weeks due to tracking lag. This is expected.

3. **Timing Matters:** There may be a 1-2 second delay between Docker Build completion and Supply Chain trigger. This is normal.

4. **Logs are Essential:** Always check the "Event" field in GitHub Actions run details to confirm the trigger type.

---

## Reporting Bugs

If bugs are found during manual testing:

1. Create a new issue in `docs/issues/bug_*.md`
2. Include:
   - Scenario number
   - Exact steps to reproduce
   - Expected vs actual behavior
   - GitHub Actions run ID
   - Event type from logs
   - Severity classification

3. Link to this test plan
4. Assign to appropriate team member
