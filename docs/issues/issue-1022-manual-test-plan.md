---
title: "Manual Test Plan - Issue #1022: Apostrophe in Workflow Dispatch Reason Crashes Weekly Promotion"
status: Open
priority: Medium
labels: testing, ci
---

# Test Objective

Confirm that the `weekly-nightly-promotion` workflow completes successfully when the manual trigger `reason` field contains an apostrophe (e.g., `it's`), and that the PR body reflects the reason correctly.

# What Was Fixed

The `Create Promotion PR` step previously injected `${{ inputs.reason }}` directly into a bash heredoc. An apostrophe in the reason value (e.g., `it's an ad-hoc run`) broke the shell quoting and caused the step to fail with a syntax error.

The fix moves `inputs.reason` into a dedicated `TRIGGER_REASON` environment variable, which is then read inside the `actions/github-script` step via `process.env.TRIGGER_REASON`. This isolates the user-supplied string from the shell entirely.

Manual testing is required because this workflow runs against the live GitHub Actions environment and cannot be fully covered by unit or integration tests.

# Prerequisites

- Write access to the repository (to trigger `workflow_dispatch`).
- The `nightly` branch must exist and have at least one commit ahead of `main`, **or** `skip_workflow_check` must be set to `true` to bypass the health gate.
- A GitHub account with permission to view Actions run logs.

# Manual Scenarios

## 1) Apostrophe in reason — primary regression check

- [ ] Navigate to **Actions → Weekly Nightly to Main Promotion → Run workflow**.
- [ ] Set **Why are you running this manually?** to: `it's a manual test run`
- [ ] Set **Skip nightly workflow status check?** to `true` (avoids a failing nightly blocking the test).
- [ ] Click **Run workflow**.
- [ ] Wait for the run to complete.
- [ ] **Expected**: The run completes without a syntax error. All jobs show green (or the run skips the PR-creation job with "No changes to promote" if `nightly` is already up-to-date with `main` — both are acceptable outcomes).
- [ ] Open the run logs for the **Create Promotion PR** step.
- [ ] **Expected**: No `syntax error` or `unexpected token` entries in the log.
- [ ] If a PR was created, open it and confirm the **Trigger** line in the PR body reads: `it's a manual test run`.

## 2) Double apostrophe / multiple special characters

- [ ] Trigger the workflow again with reason: `can't stop, won't stop`
- [ ] Set **Skip nightly workflow status check?** to `true`.
- [ ] **Expected**: Run completes. PR body (if created) or log output contains the reason verbatim.

## 3) Plain-text reason — regression check

- [ ] Trigger the workflow with reason: `Ad-hoc promotion request` (the default value, no special characters).
- [ ] **Expected**: Run behaves identically to before the fix. No regressions introduced.

## 4) Scheduled trigger (no `inputs.reason`)

- [ ] Review a recent scheduled Monday run in the Actions history (or re-run one).
- [ ] **Expected**: The `TRIGGER_REASON` env var falls back to `'Scheduled weekly promotion'` and the PR body reflects that. No errors introduced by the env-var change.

# Expected Results

| Scenario | Expected outcome |
|---|---|
| Reason with apostrophe | Workflow completes; no shell syntax error |
| Reason with multiple special chars | Workflow completes; reason appears verbatim in PR body |
| Plain-text reason | No regression; workflow behaves as before |
| Scheduled run | Fallback reason used; no errors |

# Pass / Fail Criteria

**PASS** — All four scenarios complete without a `syntax error` or `unexpected token` in the logs, and user-supplied reason text appears correctly in the PR body or step output.

**FAIL** — Any scenario produces a shell syntax error, an unexpected token error, or the reason field is garbled/missing in the PR body.

# Related

- Issue #1022 — Apostrophe in `workflow_dispatch` reason causes syntax error in weekly promotion workflow
