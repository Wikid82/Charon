---
title: "CI Image Ref Debug and Validation Fix"
status: "draft"
scope: "ci/build-image, ci/integration"
---

## 1. Introduction

This plan addresses integration failures reporting `invalid reference format` by making image output values observable, trimming/normalizing digests and image references, and validating Docker Hub image refs before downstream jobs consume them. The focus is the `Emit image outputs` step and related tag logging in the CI pipeline.

Objectives:
- Remove masking that hides computed image refs in logs.
- Normalize and trim digest and image refs to prevent whitespace/newline errors.
- Validate Docker Hub image references in the build job to surface failures early.
- Use safe `printf` in the tag echo step to avoid formatting artifacts.

## 2. Research Findings

### 2.1 Current CI Flow
- The build job defines image tags in `Compute image tags`, then builds/pushes images and emits outputs in `Emit image outputs` in [ .github/workflows/ci-pipeline.yml ].
- Integration jobs pull `needs.build-image.outputs.image_ref_dockerhub` and run `docker pull` with that value.
- `IS_FORK` is defined at workflow env level, while `PUSH_IMAGE` is computed in `Determine image push policy` and exported via outputs.

### 2.2 Current Risk Points
- `Emit image outputs` uses raw `${{ steps.push.outputs.digest }}` without trimming. Any whitespace or newline in `digest` can produce an invalid reference.
- `IMAGE_REF_DOCKERHUB` is assembled from `DIGEST` or from `TAGS_RAW` (a multi-line string). It is not explicitly trimmed before being written to outputs.
- `Echo generated tags` currently uses `echo`, which can interpret escape sequences or alter formatting.
- `Emit image outputs` masks the computed refs, reducing the ability to troubleshoot malformed references.

## 3. Technical Specifications

### 3.1 Remove Masking in Emit Outputs
- Remove `::add-mask::${IMAGE_REF_DOCKERHUB}` and `::add-mask::${IMAGE_REF_GHCR}` from `Emit image outputs`.
- Log the final `IMAGE_REF_DOCKERHUB` and `IMAGE_REF_GHCR` values in plain text for debugging.

### 3.2 Trim Digest
- Before use, trim `DIGEST` using `xargs` or bash trimming.
- Ensure `DIGEST` is empty or strictly formatted as `sha256:...` before assembling an immutable ref.

### 3.3 Sanitize Image Ref Outputs
- Normalize `IMAGE_REF_DOCKERHUB` and `IMAGE_REF_GHCR` by trimming whitespace and removing CR characters.
- Ensure outputs are written as a single line with no trailing spaces or newlines.

### 3.4 Local Validation in Build Job
- Add a validation command in or immediately after `Emit image outputs`:
  - Preferred: `docker manifest inspect "${IMAGE_REF_DOCKERHUB}"` if manifest is expected in the registry.
  - Fallback: `docker pull "${IMAGE_REF_DOCKERHUB}"`.
- Gate the validation on `PUSH_IMAGE=true` and `PUSH_OUTCOME=success` to avoid failing on non-push builds.
- On failure, emit a clear error that includes the actual `IMAGE_REF_DOCKERHUB` value.

### 3.5 Safe Tag Logging
- Replace `echo` in `Echo generated tags` with `printf '%s\n'` to avoid formatting surprises and preserve newlines.

### 3.6 Data Flow Summary (Image Ref)
- Build tags -> Build/Push -> Emit normalized refs -> Validate ref -> Downstream `docker pull`.

## 4. Implementation Plan

### Phase 1: Playwright Tests (Behavior Baseline)
- No UI changes are expected; note that Playwright coverage is unchanged.

### Phase 2: CI Build Job Debugging Enhancements
- Update `Echo generated tags` to use `printf`.
- In `Emit image outputs`, remove masking and add explicit logging of computed refs.
- Add trim logic for `DIGEST`.
- Trim `IMAGE_REF_DOCKERHUB` and `IMAGE_REF_GHCR` before writing outputs.

### Phase 3: Build Job Validation Gate
- Add Docker manifest/pull validation in `Emit image outputs` (or immediately after).
- Ensure validation only runs for successful push runs.

### Phase 4: Integration Safety
- Ensure downstream integration jobs continue to consume the sanitized `image_ref_dockerhub` output.
- Confirm no behavior change for forked PRs where `PUSH_IMAGE=false`.

### Complexity Estimates
| Component | Complexity | Notes |
| --- | --- | --- |
| Emit image outputs normalization | Low | String trimming and output formatting |
| Tag echo change | Low | Replace `echo` with `printf` |
| Local validation | Medium | Adds network dependency on registry and failure handling |

## 5. Acceptance Criteria (EARS)

- WHEN the build job emits image outputs, THE SYSTEM SHALL log `IMAGE_REF_DOCKERHUB` and `IMAGE_REF_GHCR` without masking.
- WHEN the build job receives a digest, THE SYSTEM SHALL trim whitespace before assembling immutable image references.
- WHEN the build job writes image refs to outputs, THE SYSTEM SHALL ensure they are single-line, whitespace-free strings.
- WHEN the build job completes a successful image push, THE SYSTEM SHALL validate `IMAGE_REF_DOCKERHUB` via `docker manifest inspect` or `docker pull` before downstream jobs run.
- WHEN tags are echoed in the build job, THE SYSTEM SHALL use `printf` for safe, predictable output.

## 6. Risks and Mitigations

- Risk: Registry hiccups cause false negatives during validation.
  Mitigation: Use `docker manifest inspect` first; on failure, retry once or emit a clear message with ref value and context.
- Risk: Removing masking exposes sensitive data.
  Mitigation: Image refs are not secrets; confirm no credentials or tokens are logged.
- Risk: Additional validation adds runtime.
  Mitigation: Only validate on push-enabled runs and keep validation in build job (single check).

## 7. Open Questions

- Should validation use `docker manifest inspect` only, or fallback to `docker pull` for improved diagnostics?
- Should we log both raw and normalized digest values for deeper troubleshooting?

## 8. Confidence Score

Confidence: 86 percent

Rationale: The failure mode is consistent with whitespace or formatting issues in image refs, and the proposed changes are localized to the build job. Validation behavior depends on registry availability but should be manageable with careful gating.
