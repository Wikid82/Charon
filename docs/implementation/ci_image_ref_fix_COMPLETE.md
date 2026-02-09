---
title: "CI Image Ref Resolution for Integration Jobs"
status: "draft"
scope: "ci/build-image, ci/integration"
notes: Ensure integration jobs always receive a valid Docker Hub image ref.
---

## 1. Introduction

This plan addresses a logic failure in the `Emit image outputs` step in
[.github/workflows/ci-pipeline.yml](.github/workflows/ci-pipeline.yml)
where `image_ref_dockerhub` can be emitted as an empty string. The
failure results in `docker pull ""` and aborts integration jobs even
when `run_integration` is true and the image was pushed.

Objectives:

- Diagnose why `image_ref_dockerhub` can be empty.
- Define a robust image ref selection strategy for Docker Hub.
- Update the CI pipeline to emit a valid ref for integration jobs.

## 2. Research Findings

### 2.1 Current `Emit image outputs` logic

Location:
- [.github/workflows/ci-pipeline.yml](.github/workflows/ci-pipeline.yml)

Summary:
- The step tries `steps.push.outputs.digest` first, then falls back to
  `grep` on `steps.tags.outputs.tags` to find a Docker Hub tag.
- It emits `image_ref_dockerhub` and `image_ref_ghcr` regardless of
  whether a match is found.

### 2.2 Likely failure modes

Observed symptom: integration jobs attempt `docker pull ""`, which
means `image_ref_dockerhub` is empty.

Potential causes in the current logic:

1. **Digest output missing or empty**
   - `steps.push.outputs.digest` can be empty if the build did not push
     or the action did not emit a digest for the run.
   - When the digest is empty, the step relies entirely on tag parsing.

2. **Multiline tag output parsing**
   - `steps.tags.outputs.tags` is a multiline output.
   - The current `grep` assumes line starts exactly with
     `docker.io`. If the content is empty, malformed, or contains
     non-visible characters, `grep` returns nothing.

3. **Interpolation edge cases**
   - Workflow expression substitution happens before the shell runs.
   - If the substituted string is empty or contains carriage returns,
     the `grep` command can fail to match and emit an empty ref.

### 2.3 Impacted jobs

- `integration-cerberus`
- `integration-crowdsec`
- `integration-waf`
- `integration-ratelimit`

All of these jobs pull `needs.build-image.outputs.image_ref_dockerhub`
without validating it is non-empty.

## 3. Technical Specifications

### 3.1 Robust image ref selection

The output logic must always resolve to a valid, non-empty Docker Hub
reference when `push_image` is true and `steps.push` succeeds.

Preferred selection order:

1. **Digest-based reference**
   - `docker.io/<image>@<digest>`
   - Most reliable for immutability.

2. **Deterministic tag match via DEFAULT_TAG**
   - Compare tags against the computed `DEFAULT_TAG` and select the tag
     that matches `docker.io/<image>:<DEFAULT_TAG>` when present.
   - This ensures the primary tag is deterministic instead of picking
     the first match in an arbitrary list order.

3. **First Docker Hub tag from the computed tag list**
   - Read the `steps.tags.outputs.tags` multiline output into an array
     and pick the first entry that starts with `docker.io/`.
   - Avoid `grep | head -1` on a single expanded string and use a
     controlled loop that can handle empty lines and carriage returns.

4. **Computed fallback tag from known values**
   - Use `DEFAULT_TAG` from the tag step (or expose it as an output)
     to build `docker.io/<image>:<default_tag>` if no Docker Hub tag
     could be extracted.

5. **Hard failure on empty ref when push succeeded**
   - If `push_image == true` and `steps.push.outcome == 'success'`,
     and the ref is still empty, fail the job to prevent downstream
     integration jobs from pulling `""`.
   - Emit a `::error::` message that explains the failure and includes
     the relevant signals (digest presence, tag count, DEFAULT_TAG).

### 3.2 Docker Hub prefix handling

Rules for Docker Hub references:

- Always emit `docker.io/<image>...` for Docker Hub to keep consistency
  with `docker login` and `docker pull` commands in integration jobs.
- Do not emit `library/` prefix.

### 3.3 Safe parsing and logging requirements

- Parsing MUST use `readarray -t` (bash 4+) or a
  `while IFS= read -r` loop to safely handle multiline values.
- Strip carriage returns (`\r`) from each tag line before evaluation.
- Log decision points with clear, single-line messages that explain
  why a reference was chosen (e.g., "Found digest...",
  "Digest empty, checking tags...", "Selected primary tag...",
  "DEFAULT_TAG match missing, using first docker.io tag...").

### 3.4 Integration job guardrails

Add guardrails to integration jobs to avoid pulling an empty ref:

- `if: needs.build-image.outputs.image_ref_dockerhub != ''`
- If the ref is empty, the integration job should be skipped and
  `integration-gate` should treat skipped as non-fatal.

### 3.5 Output contract

`build-image` must emit:

- `image_ref_dockerhub` (non-empty for pushed images)
- `image_ref_ghcr` (optional but should be non-empty if digest exists)
- `image_tag` (for visibility and debug)

## 4. Implementation Plan

### Phase 1: Playwright Tests (Behavior Baseline)

- No UI behavior changes are expected.
- No Playwright updates required; note this as a no-op phase.

### Phase 2: Update `Emit image outputs` step

- Replace `grep`-based parsing with a loop that:
  - Uses `readarray -t` or `while IFS= read -r` for safe parsing.
  - Trims carriage returns on each line before evaluation.
  - Selects the `DEFAULT_TAG`-matching Docker Hub tag when available.
  - Falls back to the first Docker Hub tag otherwise.
- Emit `DEFAULT_TAG` (or equivalent) from the tags step so the
  outputs step has a deterministic fallback.
- Add a hard error if the ref is empty when push succeeded using
  `::error::` so the failure is highly visible.
- Add debug logging for each decision branch and the final selection
  reason to aid troubleshooting.

### Phase 3: Integration job guardrails

- Add `if:` conditions to integration jobs to skip when
  `image_ref_dockerhub` is empty.
- Update `integration-gate` to ignore `skipped` outcomes when the
  image ref is empty and integration is not expected to run.

### Phase 4: Documentation

- Update any relevant CI documentation if a summary exists for image
  ref behavior (only if such documentation already exists).

## 5. Acceptance Criteria (EARS)

- WHEN the build-image job completes with push enabled, THE SYSTEM
  SHALL emit a non-empty `image_ref_dockerhub` suitable for
  `docker pull`.
- WHEN the build digest is available, THE SYSTEM SHALL prefer
  `docker.io/<image>@<digest>` as the emitted Docker Hub reference.
- WHEN the digest is not available, THE SYSTEM SHALL select the first
  Docker Hub tag from the computed tag list unless a tag matching
  `DEFAULT_TAG` is present, in which case that tag SHALL be selected.
- WHEN no Docker Hub tag can be parsed, THE SYSTEM SHALL construct a
  Docker Hub ref using the default tag computed during tag generation.
- IF the Docker Hub reference is still empty after all fallbacks while
  push succeeded, THEN THE SYSTEM SHALL fail the build-image job and
  emit a `::error::` message to prevent invalid downstream pulls.
- WHEN `image_ref_dockerhub` is empty, THE SYSTEM SHALL skip integration
  jobs and the integration gate SHALL NOT fail solely due to the skip.

## 6. Risks and Mitigations

- Risk: The fallback tag does not exist in Docker Hub if tag generation
  and push diverge.
  Mitigation: Use the same computed tag output from the tag step and
  fail early if no tag can be verified.

- Risk: Tight guardrails skip integration runs unintentionally.
  Mitigation: Limit skipping to the case where `image_ref_dockerhub` is
  empty and push is expected; otherwise keep existing behavior.

## 7. Confidence Score

Confidence: 83 percent

Rationale: The failure mode is clear (empty output) but the exact cause
needs confirmation from CI logs. The proposed logic reduces ambiguity
by preferring deterministic tag selection and enforcing a failure when
an empty ref would otherwise propagate.
