---
title: "CI Workflow Reliability Fixes"
status: "docs_complete"
scope: "ci/workflows"
notes: Finalize E2E split workflow execution and aggregation logic, prevent security tests from leaking into non-security shards, align Docker build and Codecov defaults, and ensure workflows run only on pull_request or workflow_dispatch.
---

## 1. Introduction

This plan finalizes CI workflow specifications based on Supervisor feedback and deeper analysis. The scope covers three workflows:

- E2E split workflow reliability and accurate results aggregation.
- Docker build workflow trigger logic for PRs and manual dispatches.
- Codecov upload workflow default behavior on non-dispatch events.

Objectives:

- Ensure Playwright jobs fail deterministically when tests fail and do not get masked by aggregation.
- Ensure results aggregation reflects expected runs based on `inputs.browser` and `inputs.test_category`.
- Ensure non-security jobs do not run security test suites.
- Preserve intended Docker build scan/test job triggers for pull_request and workflow_dispatch only.
- Preserve Codecov default behavior on non-dispatch events.
- Enforce policy that no push events trigger CI run jobs.

## 2. Research Findings

### 2.1 E2E Split Workflow Execution and Aggregation

[.github/workflows/e2e-tests-split.yml](../../.github/workflows/e2e-tests-split.yml) currently:

- Executes Playwright in multiple jobs without a consistent exit-code capture pattern.
- Aggregates results in `e2e-results` by converting any `skipped` job into `success` regardless of whether it should have run.

This can hide failures and unexpected skips when inputs or secrets are misconfigured.

### 2.2 Security Test Leakage into Non-Security Shards

Non-security jobs call Playwright with explicit paths including `tests/integration` and other folders. Security-only jobs explicitly run:

- `tests/security-enforcement/`
- `tests/security/`

If `tests/integration` contains security-sensitive tests, those could execute in non-security shards. This violates the isolation goal of the split workflow.

### 2.3 Docker Build Workflow Trigger Logic

[.github/workflows/docker-build.yml](../../.github/workflows/docker-build.yml) is intended to:

- Run `scan-pr-image` for pull requests.
- Run `test-image` for pull_request and workflow_dispatch.

The workflow also currently triggers on `push`. The user requirement is that no push events trigger CI run jobs, so all push triggers must be removed and job logic updated accordingly.

### 2.4 Codecov Default Behavior on Non-Dispatch Events

[.github/workflows/codecov-upload.yml](../../.github/workflows/codecov-upload.yml) uses `inputs.run_backend` and `inputs.run_frontend`. On `pull_request`, `inputs` is undefined, leading to unintended skips. The intended behavior is to run by default on non-dispatch events and honor inputs only on `workflow_dispatch`. The workflow must not be triggered by `push`.

## 3. Technical Specifications

### 3.1 E2E Workflow Execution Hardening

Workflow: [.github/workflows/e2e-tests-split.yml](../../.github/workflows/e2e-tests-split.yml)

Requirements:

- Apply a robust execution pattern in every Playwright run step (security and non-security jobs) to capture exit codes and fail the job consistently.
- Keep timing logs but do not allow Playwright failures to be hidden by a non-zero exit path.

Required run-step pattern:

```bash
set -euo pipefail
STATUS=0
npx playwright test ... || STATUS=$?
echo "PLAYWRIGHT_STATUS=$STATUS" >> "$GITHUB_ENV"
exit "$STATUS"
```

### 3.2 E2E Results Aggregation with Expected-Run Logic

Workflow: [.github/workflows/e2e-tests-split.yml](../../.github/workflows/e2e-tests-split.yml)

Requirements:

- Replicate the same input filtering logic used in the job `if` clauses to determine if each job should have run.
- `e2e-results` MUST declare `needs: [shard-jobs...]` covering every shard job so the aggregation evaluates actual results.
- Treat `skipped`, `failure`, or `cancelled` as failure outcomes when a job should have run.
- Ignore `skipped` when a job should not have run.

Expected-run logic (derived from inputs and defaults):

- `effective_browser = inputs.browser || 'all'`
- `effective_category = inputs.test_category || 'all'`

For each job:

- Security jobs should run when `effective_browser` matches the browser (or `all`) AND `effective_category` is `security` or `all`.
- Non-security jobs should run when `effective_browser` matches the browser (or `all`) AND `effective_category` is `non-security` or `all`.

Aggregation behavior:

- If a job should run and result is `skipped`, `failure`, or `cancelled`, the aggregation fails.
- If a job should not run and result is `skipped`, ignore it.
- Only return success when all expected jobs are successful.

### 3.3 Security Isolation for Non-Security Shards

Workflow: [.github/workflows/e2e-tests-split.yml](../../.github/workflows/e2e-tests-split.yml)

Requirements:

- Ensure non-security jobs do not execute any tests from `tests/security-enforcement/` or `tests/security/`.
- Validate whether `tests/integration` includes security tests. If it does, split those tests into the security suites or exclude them explicitly from non-security runs.
- Maintain explicit path lists for non-security jobs to avoid accidental inclusion via globbing.

### 3.4 Docker Build Workflow Conditions (Preserve Intended Behavior)

Workflow: [.github/workflows/docker-build.yml](../../.github/workflows/docker-build.yml)

Requirements:

- Remove `push` triggers from the workflow `on:` block.
- Keep `scan-pr-image` for `pull_request` only.
- Keep `test-image` for `pull_request` and `workflow_dispatch` only.
- Preserve `skip_build` gating.
- Remove any job-level checks for `github.event_name == 'push'`.

Expected `on:` block:

```yaml
on:
	pull_request:
		branches:
			- main
			- development
	workflow_dispatch:
```

Expected `if` conditions:

- `scan-pr-image`: `needs.build-and-push.outputs.skip_build != 'true' && needs.build-and-push.result == 'success' && github.event_name == 'pull_request'`
- `test-image`: `needs.build-and-push.outputs.skip_build != 'true' && needs.build-and-push.result == 'success' && (github.event_name == 'pull_request' || github.event_name == 'workflow_dispatch')`

### 3.5 Codecov Default Behavior on Non-Dispatch Events

Workflow: [.github/workflows/codecov-upload.yml](../../.github/workflows/codecov-upload.yml)

Requirements:

- Remove `push` triggers from the workflow `on:` block.
- Keep default coverage uploads on non-dispatch events.
- Only honor `inputs.run_backend` and `inputs.run_frontend` for `workflow_dispatch`.
- Remove any job-level checks for `github.event_name == 'push'`.

Expected `on:` block:

```yaml
on:
	pull_request:
		branches:
			- main
			- development
	workflow_dispatch:
```

Expected job conditions:

- `backend-codecov`: `${{ github.event_name != 'workflow_dispatch' || inputs.run_backend != 'false' }}`
- `frontend-codecov`: `${{ github.event_name != 'workflow_dispatch' || inputs.run_frontend != 'false' }}`

### 3.6 Trigger Policy: No Push CI Runs

Workflows: [.github/workflows/e2e-tests-split.yml](../../.github/workflows/e2e-tests-split.yml),
[.github/workflows/docker-build.yml](../../.github/workflows/docker-build.yml),
[.github/workflows/codecov-upload.yml](../../.github/workflows/codecov-upload.yml)

Requirements:

- Remove `push` from the `on:` block in each workflow.
- Ensure only `pull_request` and `workflow_dispatch` events trigger runs.
- Remove any job-level `if` clauses that include `github.event_name == 'push'`.

Expected `on:` blocks:

```yaml
on:
	pull_request:
		branches:
			- main
			- development
	workflow_dispatch:
```

## 4. Implementation Plan

### Phase 1: Playwright Tests (Behavior Definition)

- No new Playwright tests are required; this change focuses on CI execution and aggregation.
- Confirm current Playwright suites already cover security enforcement and non-security coverage across browsers.

### Phase 2: Backend Implementation

- Not applicable. No backend code changes are required.

### Phase 3: Frontend Implementation

- Not applicable. No frontend code changes are required.

### Phase 4: Integration and Testing

- Update all Playwright run steps in the split workflow to use the robust execution pattern.
- Update `e2e-results` to compute expected-run logic and fail on unexpected skips.
- Audit non-security test path lists to ensure security tests are not executed outside security jobs.
- Update Docker workflow `on:` triggers and job conditions to remove `push` and align with `pull_request` and `workflow_dispatch` only.
- Update Codecov workflow `on:` triggers and job conditions to default on non-dispatch events (no push triggers).
- Update E2E split workflow `on:` triggers to remove `push` and align with `pull_request` and `workflow_dispatch` only.

### Phase 5: Documentation and Deployment

- Update this plan with final specs and ensure it matches workflow behavior.
- No deployment or release changes are required.

## 5. Acceptance Criteria (EARS)

- WHEN a Playwright job runs, THE SYSTEM SHALL record the Playwright exit status and exit the step with that status.
- WHEN a job should have run based on `inputs.browser` and `inputs.test_category`, THE SYSTEM SHALL fail aggregation if the job result is `skipped`, `failure`, or `cancelled`.
- WHEN a job should not have run based on `inputs.browser` and `inputs.test_category`, THE SYSTEM SHALL ignore its `skipped` result.
- WHEN non-security shards run, THE SYSTEM SHALL NOT execute tests from `tests/security-enforcement/` or `tests/security/`.
- WHEN the Docker build workflow runs on a pull request, THE SYSTEM SHALL execute `scan-pr-image`.
- WHEN the Docker build workflow runs on a workflow dispatch, THE SYSTEM SHALL execute `test-image`.
- WHEN the Docker build workflow runs on a pull request, THE SYSTEM SHALL execute `test-image`.
- WHEN the Codecov workflow runs on a non-dispatch event, THE SYSTEM SHALL execute backend and frontend coverage jobs by default.
- WHEN the Codecov workflow is manually dispatched and inputs disable a job, THE SYSTEM SHALL skip the disabled job.
- WHEN any of the target workflows are triggered, THE SYSTEM SHALL only allow `pull_request` and `workflow_dispatch` events and SHALL NOT run for `push` events.

## 6. Risks and Mitigations

- Risk: Aggregation logic becomes too strict for selective runs. Mitigation: derive expected-run logic from inputs with the same defaults used in the workflow.
- Risk: Security tests remain in `tests/integration` and leak into non-security shards. Mitigation: audit `tests/integration` and relocate or exclude security tests explicitly.
- Risk: Removing push triggers reduces CI coverage for direct branch pushes. Mitigation: enforce pull_request-based checks and require manual dispatch for emergency validations.

## 7. Confidence Score

Confidence: 86 percent

Rationale: Required changes are localized to workflow triggers, job conditions, and test path selection. The main remaining uncertainty is whether any security tests reside in `tests/integration`, which must be verified before finalizing non-security path lists.
