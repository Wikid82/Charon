---
title: Manual Test Plan - Security Scan PR Event Gating and Artifact Resolution
status: Open
priority: High
assignee: DevOps
labels: testing, workflows, security, ci/cd
---

## Goal
Validate that `Security Scan (PR)` in `.github/workflows/security-pr.yml` behaves deterministically for trigger gating, PR artifact resolution, and trust-boundary checks.

## Scope
- Event gating for `workflow_run`, `workflow_dispatch`, `pull_request`, and `push`
- PR artifact lookup and image loading path
- Failure behavior for missing/corrupt artifacts
- Permission and trust-boundary protection paths

## Preconditions
- You can run workflows in this repository.
- You can view workflow logs in GitHub Actions.
- At least one recent PR exists with a successful `Docker Build, Publish & Test` run and published `pr-image-<PR_NUMBER>` artifact.
- Use a test branch or draft PR for negative testing.

## Evidence to Capture
- Run URL for each scenario
- Job status (`success`, `failure`, `skipped`)
- Exact failure line when expected
- `reason_category` value when present

## Manual Test Checklist

### 1. `workflow_run` from upstream `pull_request` (happy path)
- [ ] Trigger a PR build by pushing a commit to an open PR.
- [ ] Wait for `Docker Build, Publish & Test` to complete successfully.
- [ ] Confirm `Security Scan (PR)` starts from `workflow_run`.
- [ ] Confirm job `Trivy Binary Scan` runs.
- [ ] Confirm logs show trust-boundary validation success.
- [ ] Confirm artifact `pr-image-<PR_NUMBER>` is found and downloaded.
- [ ] Confirm `Load Docker image` resolves to `charon:artifact`.
- [ ] Confirm binary extraction and Trivy scan steps execute.

Expected outcome:
- Workflow succeeds or fails only on real security findings, not on event/artifact resolution.

Failure signals:
- `reason_category=unsupported_upstream_event` on a PR-triggered upstream run.
- Artifact lookup fails for a known valid PR artifact.
- `Load Docker image` cannot resolve image ref despite valid artifact.

### 2. `workflow_run` from upstream `push` (should not run)
- [ ] Push directly to a branch that triggers `Docker Build, Publish & Test` as `push` (for example, `main` in a controlled test window).
- [ ] Open `Security Scan (PR)` run created by `workflow_run`.
- [ ] Verify `Trivy Binary Scan` is skipped by job-level gating.
- [ ] Verify no artifact lookup/download steps were executed.

Expected outcome:
- `Security Scan (PR)` job does not run for upstream `push`.

Failure signals:
- `Trivy Binary Scan` executes for upstream `push`.
- Any artifact resolution step runs under upstream `push`.

### 3. `workflow_dispatch` with valid `pr_number`
- [ ] Open `Security Scan (PR)` and click `Run workflow`.
- [ ] Provide a numeric `pr_number` that has a successful docker-build artifact.
- [ ] Start run and inspect logs.
- [ ] Confirm PR number validation passes.
- [ ] Confirm run lookup resolves a successful `docker-build.yml` run for that PR.
- [ ] Confirm artifact download, image load, extraction, and Trivy steps run.

Expected outcome:
- Workflow executes artifact-only replay path and proceeds to scan.

Failure signals:
- Dispatch falls back to local image build.
- `reason_category=not_found` for a PR known to have valid artifact.

### 4. `workflow_dispatch` without `pr_number` (input validation)
- [ ] Open `Run workflow` for `Security Scan (PR)`.
- [ ] Attempt run with empty `pr_number` (or non-numeric value if UI blocks empty).
- [ ] Inspect early step logs.

Expected outcome:
- Job fails fast before artifact lookup/load.
- Clear validation message indicates missing/invalid `pr_number`.

Failure signals:
- Workflow continues to artifact lookup with invalid input.
- Error message is ambiguous or missing reason category.

### 5. Artifact missing case
- [ ] Run `workflow_dispatch` with a numeric PR that does not have a successful docker-build artifact.
- [ ] Inspect `Check for PR image artifact` logs.

Expected outcome:
- Hard fail with a clear error.
- Log includes `reason_category=not_found`, run context, and artifact name.

Failure signals:
- Step silently skips or succeeds without artifact.
- Workflow proceeds to download/load steps.

### 6. Artifact corrupt/unreadable case
- [ ] Use a controlled test branch to simulate bad artifact content for `charon-pr-image.tar` (for example, tar missing `manifest.json` and no usable load image ID, or unreadable tar).
- [ ] Trigger path through `workflow_run` or `workflow_dispatch`.
- [ ] Inspect `Load Docker image` logs.

Expected outcome:
- Job fails in `Load Docker image` before extraction when image cannot be resolved.
- Error states artifact is missing/unreadable, or valid image reference cannot be resolved.

Failure signals:
- Job continues to extraction with empty/invalid image ref.
- `docker create` fails later due to unresolved image (late failure indicates missed validation).

### 7. Trust-boundary and permission guard failures
- [ ] Verify `permissions` in run metadata are minimal: `contents: read`, `actions: read`, `security-events: write`.
- [ ] For `workflow_run`, inspect guard step output.
- [ ] Confirm guard fails when any of the following are invalid:
  - Upstream workflow name mismatch
  - Upstream event not `pull_request`
  - Upstream head repository not equal to current repository

Expected outcome:
- Guard fails early with explicit `reason_category`.
- No artifact lookup/load/extract occurs after guard failure.

Failure signals:
- Guard passes with mismatched trust-boundary values.
- Workflow attempts artifact operations after trust-boundary failure.
- Unexpected write permissions are present.

## Regression Watchlist
- Event-gating changes accidentally allow `workflow_run` from `push` to execute scan.
- Manual dispatch path silently accepts non-numeric or empty PR input.
- Artifact resolver relies on a single tag and breaks on alternate load output formats.
- Trust-boundary checks are bypassed due to conditional logic drift.

## Exit Criteria
- All scenarios pass with expected behavior.
- Any failure signal is logged as a bug with run URL and exact failing step.
- No ambiguous skip behavior remains for required hard-fail paths.
