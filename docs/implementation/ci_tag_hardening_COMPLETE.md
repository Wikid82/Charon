---
title: "CI Tag Hardening"
status: "draft"
scope: "ci/tagging"
notes: Harden image tag computation and add debug visibility in CI pipeline.
---

## 1. Introduction

This plan hardens the `Compute image tags` step in the CI pipeline and
adds a debug step to improve visibility into generated tags. The focus
is limited to `.github/workflows/ci-pipeline.yml`.

Objectives:

- Add explicit error checks for `DEFAULT_TAG`, `IMAGE_NAME`, and tag list
  generation.
- Echo computed tags to stdout inside the tag computation step.
- Add a dedicated `Echo generated tags` step before image build/push.

## 2. Research Findings

- The tag computation logic lives in `Compute image tags` under the
  `build-image` job in `.github/workflows/ci-pipeline.yml`.
- The pipeline uses `IMAGE_NAME` from `env` and normalizes it in the
  `Normalize image name` step.
- The `Build and push Docker image` step uses `steps.tags.outputs.tags`.
- There is no explicit guard to prevent empty `IMAGE_NAME` or
  `DEFAULT_TAG`, and the script does not emit the tag list to stdout.

## 3. Technical Specifications

### 3.1 Harden `Normalize image name`

Add a validation to ensure `IMAGE_NAME` is not empty after normalization.
Preferred location: the `Normalize image name` step.

- Validate with a shell check and emit a GitHub Actions error:
  - `if [ -z "$IMAGE_NAME" ]; then echo "::error::IMAGE_NAME is empty!" && exit 1; fi`
- Keep normalization as-is, but fail fast when empty.
- Ensure this validation runs before any tag construction uses
  `IMAGE_NAME`.

### 3.2 Harden `Compute image tags`

Add explicit validation and visibility to the `Compute image tags` step.

Required checks:

- `DEFAULT_TAG` must be non-empty:
  - `if [ -z "$DEFAULT_TAG" ]; then echo "::error::DEFAULT_TAG is empty!" && exit 1; fi`
- `IMAGE_NAME` must be validated before any tag assembly:
  - `if [ -z "$IMAGE_NAME" ]; then echo "::error::IMAGE_NAME is empty!" && exit 1; fi`
- `TAGS` array must contain entries:
  - `if [ ${#TAGS[@]} -eq 0 ]; then echo "::error::No tags generated!" && exit 1; fi`
- `TAGS=()` must be explicitly initialized before any tags are
  appended.
- Each entry in the final `TAGS` array must be non-empty and must not
  contain whitespace. If any entry fails validation, emit a GitHub
  Actions error and exit.

Required output visibility:

- Echo computed tags to stdout inside the script, after the array is
  fully populated and validated.
- Keep output formatting line-based for clarity.

Optional redundancy (if desired):

- Re-check `IMAGE_NAME` inside the `Compute image tags` step to catch any
  unexpected environment issues before tag assembly.

### 3.3 Add Debug Step

Insert a new step named `Echo generated tags` directly before
`Build and push Docker image`.

- Command: `echo "${{ steps.tags.outputs.tags }}"`
- Purpose: Immediate visibility of tags outside the tag computation
  script.

## 4. Implementation Plan

### Phase 1: Playwright Tests (Behavior Baseline)

- No UI behavior changes are expected. Document that E2E scope is
  unchanged and re-run only if CI changes impact downstream stages.

### Phase 2: Harden Normalize Step

- Update `Normalize image name` to validate non-empty `IMAGE_NAME` after
  normalization and exit with a GitHub Actions error message.

### Phase 3: Harden Compute Tags Step

- Add `DEFAULT_TAG` empty check.
- Add `TAGS` array empty check.
- Initialize `TAGS=()` explicitly before appending entries.
- Validate `IMAGE_NAME` before tag assembly in this step.
- Iterate through the final `TAGS` array and fail if any entry is empty
  or contains whitespace.
- Echo computed tags to stdout after validations.
- (Optional) Add a defensive `IMAGE_NAME` empty check here if not already
  done in the normalize step.

### Phase 4: Add Debug Step

- Insert `Echo generated tags` step before `Build and push Docker image`
  and use the `steps.tags.outputs.tags` output.

### Phase 5: Validation

- Verify the pipeline fails fast when `IMAGE_NAME` or `DEFAULT_TAG` is
  empty or when no tags are generated.
- Confirm `Compute image tags` outputs the tag list to stdout.
- Confirm the new debug step prints the computed tag list before the
  Docker build step.

## 5. Acceptance Criteria (EARS)

- WHEN the CI pipeline normalizes `IMAGE_NAME`, THE SYSTEM SHALL fail
  with a GitHub Actions error if `IMAGE_NAME` is empty.
- WHEN `DEFAULT_TAG` is computed, THE SYSTEM SHALL fail with a GitHub
  Actions error if `DEFAULT_TAG` is empty.
- WHEN the tag list is assembled, THE SYSTEM SHALL validate every entry
  and fail if any entry is empty or contains whitespace.
- WHEN the tag list is assembled, THE SYSTEM SHALL fail with a GitHub
  Actions error if no tags are generated.
- WHEN tag computation completes successfully, THE SYSTEM SHALL echo the
  computed tag list to stdout within the script.
- WHEN the pipeline reaches the image build step, THE SYSTEM SHALL echo
  `steps.tags.outputs.tags` in a dedicated `Echo generated tags` step
  immediately before `Build and push Docker image`.

## 6. Risks and Mitigations

- Risk: Additional checks could fail runs that previously continued with
  invalid state.
  Mitigation: The failures are intentional and improve safety; update any
  dependent workflow assumptions if failures are observed.
- Risk: Tags output may include multi-line values and be hard to scan.
  Mitigation: Keep stdout echo line-based and avoid extra formatting.

## 7. Confidence Score

Confidence: 92 percent

Rationale: The changes are localized to a single workflow and involve
straightforward shell validation and logging logic with minimal risk.
