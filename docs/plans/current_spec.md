---
title: "CI Pipeline Reliability and Docker Tagging"
status: "draft"
scope: "ci/linting, ci/integration, docker/publishing"
notes: Restore Go linting parity, prevent integration-stage cancellation after successful image builds, and correct Docker tag outputs across CI workflows.
---

## 1. Introduction

This plan expands the CI scope to address three related gaps: missing Go
lint enforcement, integration jobs being cancelled after a successful
image build, and incomplete Docker tag outputs on Docker Hub. The
intended outcome is a predictable pipeline where linting blocks early,
integration and E2E gates complete reliably, and registries receive the
full tag set required for traceability and stable consumption.

Objectives:

- Reinstate golangci-lint in the pipeline lint stage.
- Use the fast config that already blocks local commits.
- Ensure golangci-lint config is valid for the version used in CI.
- Remove CI-only leniency so lint failures block merges.
- Prevent integration jobs from being cancelled when image builds have
  already completed successfully.
- Ensure Docker Hub and GHCR receive SHA-only and branch+SHA tags, plus
  latest/dev/nightly tags for main/development/nightly branches.
- Keep CI behavior consistent across pre-commit, Makefile, VS Code
  tasks, and GitHub Actions workflows.

## 2. Research Findings

### 2.1 Current CI State (Linting)

- The main pipeline is [ .github/workflows/ci-pipeline.yml ] and its
  lint job runs repo health, Hadolint, GORM scanner, and frontend lint.
  There is no Go lint step in this pipeline.
- A separate manual workflow, [ .github/workflows/quality-checks.yml ],
  runs golangci-lint with `continue-on-error: true`, which means CI does
  not block on Go lint failures.

### 2.2 Integration Cancellation Symptoms

- [ .github/workflows/ci-pipeline.yml ] defines workflow-level
  concurrency:
  `group: ci-manual-pipeline-${{ github.workflow }}-${{ github.ref_name }}`
  with `cancel-in-progress: true`.
- Integration jobs depend on `build-image` and gate on
  `inputs.run_integration != false` and
  `needs.build-image.outputs.push_image == 'true'`.
- Integration-gate fails if any dependent integration job reports
  `failure` or `cancelled`, and runs with `if: always()`.
- A workflow-level cancellation after the build-image job completes will
  cancel downstream integration jobs even though the build succeeded.

### 2.3 Current Image Tag Outputs

- In [ .github/workflows/ci-pipeline.yml ], the `Compute image tags`
  step emits:
  - `DEFAULT_TAG` (sha-<short> or pr-<number>-<short>)
  - latest/dev/nightly tags based on `github.ref_name`
- In [ .github/workflows/docker-build.yml ], `docker/metadata-action`
  emits tags including:
  - `type=raw,value=pr-${{ env.TRIGGER_PR_NUMBER }}-{{sha}}` for PRs
  - `type=sha,format=short` for non-PRs
  - feature branch tag via `steps.feature-tag.outputs.tag`
  - `latest` only when `is_default_branch` is true
  - `dev` only when `env.TRIGGER_REF == 'refs/heads/development'`
- Docker Hub currently shows only PR and SHA-prefixed tags for some
  builds; SHA-only and branch+SHA tags are not emitted consistently.
- Nightly tagging exists in [ .github/workflows/nightly-build.yml ],
  but the main Docker build workflow does not emit a `nightly` tag based
  on branch detection.

## 3. Technical Specifications

### 3.1 CI Lint Job (Pipeline)

Add a Go lint step to the lint job in
[ .github/workflows/ci-pipeline.yml ]:

- Tooling: `golangci/golangci-lint-action`.
- Working directory: `backend`.
- Config: `backend/.golangci-fast.yml`.
- Timeout: match config intent (2m fast, or 5m if parity with other
  pipeline steps is preferred).
- Failures: do not allow `continue-on-error`.

### 3.2 CI Lint Job (Manual Quality Checks)

Update [ .github/workflows/quality-checks.yml ] to align with local
blocking behavior:

- Remove `continue-on-error: true` from the golangci-lint step.
- Ensure the step points to `backend/.golangci-fast.yml` or runs in
  `backend` so that the config is picked up deterministically.
- Pin golangci-lint version to the same major used in CI pipeline to
  avoid config drift.

### 3.3 Integration Cancellation Root Cause and Fix

Investigate and address workflow-level cancellation affecting
integration jobs after `build-image` completes.

Required investigation steps:

- Inspect recent CI runs for cancellation reasons in the Actions UI
  (workflow-level cancellation vs job-level failure).
- Confirm whether cancellations coincide with the workflow-level
  concurrency group in [ .github/workflows/ci-pipeline.yml ].
- Verify `inputs.run_integration` values are only populated on
  `workflow_dispatch` events and evaluate the behavior on
  `pull_request` events.
- Verify `needs.build-image.outputs.push_image` and
  `needs.build-image.outputs.image_ref_dockerhub` are set for non-fork
  pull requests and branch pushes.

Proposed fix (preferred):

- Remove workflow-level concurrency from
  [ .github/workflows/ci-pipeline.yml ] and instead apply job-level
  concurrency to the build-image job only, keeping cancellation limited
  to redundant builds while allowing downstream integration/E2E/coverage
  jobs to finish.
- Add explicit guards to integration jobs:
  `if: needs.build-image.result == 'success' &&
  needs.build-image.outputs.push_image == 'true' &&
  needs.build-image.outputs.image_ref_dockerhub != '' &&
  (inputs.run_integration != false)`.
- Update the integration-gate logic to treat `skipped` jobs as
  non-fatal and only fail on `failure` or `cancelled` when
  `needs.build-image.result == 'success'` and `push_image == 'true'`.

Alternative fix (not recommended; does not meet primary objective):

- Keep workflow-level concurrency but change to
  `cancel-in-progress: ${{ github.event_name == 'pull_request' }}` so
  branch pushes and manual dispatches complete all downstream jobs.
- This option still cancels PR runs after successful builds, which
  conflicts with the primary objective of allowing integration gates
  to complete reliably.

### 3.4 Image Tag Outputs (CI Pipeline)

Update the `Compute image tags` step in
[ .github/workflows/ci-pipeline.yml ] to emit additional tags.

Required additions:

- SHA-only tag (short SHA, no prefix):
  `${SHORT_SHA}` for both GHCR and Docker Hub.
- Tag normalization rules for `SANITIZED_BRANCH`:
  - Ensure the tag is non-empty after sanitization.
  - Ensure the first character is `[a-z0-9]`; if it would start with
    `-` or `.`, normalize by trimming leading `-` or `.` and recheck.
  - Replace non-alphanumeric characters with `-` and collapse multiple
    `-` characters into one.
  - Limit the tag length to 128 characters after normalization.
  - Fallback: if the sanitized result is empty or still invalid after
    normalization, use `branch` as the fallback prefix.
- Branch+SHA tag for non-PR events using a sanitized branch name derived
  from `github.ref_name` (lowercase, `/` → `-`, non-alnum → `-`,
  trimmed, collapsed). Example:
  `${SANITIZED_BRANCH}-${SHORT_SHA}`.
- Preserve existing `pr-${PR_NUMBER}-${SHORT_SHA}` for PRs.
- Keep `latest`, `dev`, and `nightly` tags based on:
  `github.ref_name == 'main' | 'development' | 'nightly'`.

Decision point: SHA-only tags for PR builds

- Option A (recommended): publish SHA-only tags only for trusted
  branches (main/development/nightly and non-fork pushes). PR builds
  continue to use `pr-${PR_NUMBER}-${SHORT_SHA}` without SHA-only tags.
- Option B: publish SHA-only tags for PR builds when image push is
  enabled for a non-fork authorized run (e.g., same-repo PRs), in
  addition to PR-prefixed tags.
- Assumption (default until decided): follow Option A to avoid
  ambiguous SHA-only tags for untrusted PR contexts.

Required step-level variables and expressions:

- Step: `Compute image tags` (id: `tags`).
- Variables: `SHORT_SHA`, `DEFAULT_TAG`, `PR_NUMBER`, `SANITIZED_BRANCH`.
- Expressions:
  - `${{ github.event_name }}`
  - `${{ github.ref_name }}`
  - `${{ github.event.pull_request.number }}`

### 3.5 Image Tag Outputs (docker-build.yml)

Update [ .github/workflows/docker-build.yml ] `Generate Docker metadata`
tags to match the required outputs.

Required additions:

- Add SHA-only short tag for all events:
  `type=sha,format=short,prefix=,suffix=`.
- Add branch+SHA short tag for non-PR events using a sanitized branch
  name derived from `env.TRIGGER_REF` or `env.TRIGGER_HEAD_BRANCH`.
- Apply the same tag normalization rules as the CI pipeline
  (`SANITIZED_BRANCH` non-empty, leading character normalized, length
  <= 128, fallback to `branch`).
- Add explicit branch tags for main/development/nightly based on
  `env.TRIGGER_REF` (do not rely on `is_default_branch` for
  workflow_run triggers):
  - `type=raw,value=latest,enable=${{ env.TRIGGER_REF == 'refs/heads/main' }}`
  - `type=raw,value=dev,enable=${{ env.TRIGGER_REF == 'refs/heads/development' }}`
  - `type=raw,value=nightly,enable=${{ env.TRIGGER_REF == 'refs/heads/nightly' }}`

Required step names and variables:

- Step: `Compute feature branch tag` (id: `feature-tag`) remains for
  `refs/heads/feature/*`.
- New step: `Compute branch+sha tag` (id: `branch-tag`) for all
  non-PR events using `TRIGGER_REF`.
- Metadata step: `Generate Docker metadata` (id: `meta`).
- Expressions:
  - `${{ env.TRIGGER_EVENT }}`
  - `${{ env.TRIGGER_REF }}`
  - `${{ env.TRIGGER_HEAD_SHA }}`
  - `${{ env.TRIGGER_PR_NUMBER }}`
  - `${{ steps.branch-tag.outputs.tag }}`

### 3.6 Repository Hygiene Review (Requested)

- [ .gitignore ]: No change required for CI updates; no new artifacts
  introduced by the tag changes.
- [ codecov.yml ]: No change required; coverage configuration remains
  correct.
- [ .dockerignore ]: No change required; CI-only YAML edits are already
  excluded from Docker build context.
- [ Dockerfile ]: No change required; tagging logic is CI-only.
- [ Branch tag normalization ]: No new files required; logic should be
  implemented in existing CI steps only.

## 4. Implementation Plan

### Phase 1: Playwright Tests (Behavior Baseline)

- Confirm that no UI behavior is affected by CI-only changes.
- Keep this phase as a verification note: E2E is unchanged and can be
  re-run if CI changes surface unexpected side effects.

### Phase 2: Pipeline Lint Restoration

- Add a Go lint step to the lint job in
  [ .github/workflows/ci-pipeline.yml ].
- Use `backend/.golangci-fast.yml` and ensure the step blocks on
  failure.
- Keep the lint job dependency order intact (repo health → Hadolint →
  GORM scan → Go lint → frontend lint).

### Phase 3: Integration Cancellation Fix

- Remove workflow-level concurrency from
  [ .github/workflows/ci-pipeline.yml ] and add job-level concurrency
  on `build-image` only.
- Add explicit `if` guards to integration jobs based on
  `needs.build-image.result`, `needs.build-image.outputs.push_image`,
  and `needs.build-image.outputs.image_ref_dockerhub`.
- Update `integration-gate` to ignore `skipped` results when integration
  is not expected to run and only fail on `failure` or `cancelled` when
  build-image succeeded and pushed an image.

### Phase 4: Docker Tagging Updates

- Update `Compute image tags` in
  [ .github/workflows/ci-pipeline.yml ] to emit SHA-only and
  branch+SHA tags in addition to the existing PR and branch tags.
- Update `Generate Docker metadata` in
  [ .github/workflows/docker-build.yml ] to emit SHA-only, branch+SHA,
  and explicit latest/dev/nightly tags based on `env.TRIGGER_REF`.
- Add tag normalization logic in both workflows to ensure valid Docker
  tag prefixes (non-empty, valid leading character, <= 128 length,
  fallback when sanitized branch is empty or invalid).

### Phase 5: Validation and Guardrails

- Verify CI logs show the golangci-lint version and config in use.
- Confirm integration jobs are no longer cancelled after successful
  builds when new runs are queued.
- Validate that Docker Hub and GHCR tags include:
  - SHA-only short tags
  - Branch+SHA short tags
  - latest/dev/nightly tags for main/development/nightly branches

## 5. Acceptance Criteria (EARS)

- WHEN a pull request or manual pipeline run executes, THE SYSTEM SHALL
  run golangci-lint in the pipeline lint stage using
  `backend/.golangci-fast.yml`.
- WHEN golangci-lint finds violations, THE SYSTEM SHALL fail the
  pipeline lint stage and block downstream jobs.
- WHEN the manual quality workflow runs, THE SYSTEM SHALL enforce the
  same blocking behavior and fast config as pre-commit.
- WHEN a build-image job completes successfully and image push is
  enabled for a non-fork authorized run, THE SYSTEM SHALL allow
  integration jobs to run to completion without being cancelled by
  workflow-level concurrency.
- WHEN integration jobs are skipped by configuration while image push
  is disabled or not authorized for the run, THE SYSTEM SHALL not mark
  the integration gate as failed.
- WHEN a non-PR build runs on main/development/nightly branches and
  image push is enabled for a non-fork authorized run, THE SYSTEM SHALL
  publish `latest`, `dev`, or `nightly` tags respectively to Docker Hub
  and GHCR.
- WHEN any image is built in CI and image push is enabled for a
  non-fork authorized run, THE SYSTEM SHALL publish SHA-only and
  branch+SHA tags in addition to existing PR or default tags.

## 6. Risks and Mitigations

- Risk: CI runtime increases due to added golangci-lint execution.
  Mitigation: use the fast config and keep timeout tight (2m) with
  caching enabled by the action.
- Risk: Config incompatibility with CI golangci-lint version.
  Mitigation: pin the version and log it in CI; validate config format.
- Risk: Reduced cancellation leads to overlapping integration runs.
  Mitigation: keep job-level concurrency on build-image; monitor queue
  time and adjust if needed.
- Risk: Tag proliferation complicates image selection for users.
  Mitigation: document tag matrix in release notes or README once
  verified in CI.
- Risk: Sanitized branch names may collapse to empty or invalid tags.
  Mitigation: enforce normalization rules with a safe fallback prefix
  to keep tag generation deterministic.

## 7. Confidence Score

Confidence: 84 percent

Rationale: The linting changes are straightforward, but integration
job cancellation behavior depends on workflow-level concurrency and may
require validation in Actions history to select the most appropriate
fix. Tagging changes are predictable once metadata-action inputs are
aligned with branch detection.
