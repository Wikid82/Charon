# Security Scan (PR) Deterministic Artifact Policy - Supervisor Remediation Plan

## 1. Introduction

### Overview

`Security Scan (PR)` failed because `.github/workflows/security-pr.yml` loaded
an artifact image tag (`pr-718-385081f`) and later attempted extraction with a
different synthesized tag (`pr-718`).

Supervisor conflict resolution in this plan selects Option A:
`workflow_run` artifact handling is restricted to upstream
`pull_request` events only.

### Root-Cause Clarity (Preserved)

The failure was not a Docker load failure. It was a source-of-truth violation in
image selection:

1. Artifact load path succeeded.
2. Extraction path reconstructed an alternate reference.
3. Alternate reference did not exist, causing `docker create ... not found`.

This plan keeps scope strictly on `.github/workflows/security-pr.yml`.

### Objectives

1. Remove all ambiguous behavior for artifact absence on `workflow_run`.
2. Remove `workflow_run` support for upstream `push` events to align with PR
   artifact naming contract (`pr-image-<pr_number>`).
3. Codify one deterministic `workflow_dispatch` policy in SHALL form.
4. Harden image selection so it is not brittle on `RepoTags[0]`.
5. Add CI security hardening requirements for permissions and trust boundary.
6. Expand validation matrix to include `pull_request` and negative paths.

---

## 2. Research Findings

### 2.1 Failure Evidence

Source: `.github/logs/ci_failure.log`

Observed facts:

1. Artifact `pr-image-718` was found and downloaded from run `22164807859`.
2. `docker load` reported: `Loaded image: ghcr.io/wikid82/charon:pr-718-385081f`.
3. Extraction attempted: `docker create ghcr.io/wikid82/charon:pr-718`.
4. Docker reported: `... pr-718: not found`.

### 2.2 Producer Contract

Source: `.github/workflows/docker-build.yml`

Producer emits immutable PR tags with SHA suffix (`pr-<num>-<sha>`). Consumer
must consume artifact metadata/load output, not reconstruct mutable tags.

### 2.3 Current Consumer Gaps

Source: `.github/workflows/security-pr.yml`

Current consumer contains ambiguous policy points:

1. `workflow_run` artifact absence behavior can be interpreted as skip or fail.
2. `workflow_dispatch` policy is not single-path deterministic.
3. Image identification relies on single `RepoTags[0]` assumption.
4. Trust boundary and permission minimization are not explicitly codified as
   requirements.

---

## 3. Technical Specifications

### 3.1 Deterministic EARS Requirements (Blocking)

1. WHEN `security-pr.yml` is triggered by `workflow_run` with
   `conclusion == success` and upstream event `pull_request`, THE SYSTEM SHALL
   require the expected image artifact to exist and SHALL hard fail the job if
   the artifact is missing.

2. WHEN `security-pr.yml` is triggered by `workflow_run` and artifact lookup
   fails, THEN THE SYSTEM SHALL exit non-zero with a diagnostic that includes:
   upstream run id, expected artifact name, and reason category (`not found` or
   `api/error`).

3. WHEN `security-pr.yml` is triggered by `workflow_run` and upstream event is
   not `pull_request`, THEN THE SYSTEM SHALL hard fail immediately with reason
   category `unsupported_upstream_event` and SHALL NOT attempt artifact lookup,
   image load, or extraction.

4. WHEN `security-pr.yml` is triggered by `workflow_dispatch`, THE SYSTEM SHALL
   require `inputs.pr_number` and SHALL hard fail immediately if input is empty.

5. WHEN `security-pr.yml` is triggered by `workflow_dispatch` with valid
   `inputs.pr_number`, THE SYSTEM SHALL resolve artifact `pr-image-<pr_number>`
   from the latest successful `docker-build.yml` run for that PR and SHALL hard
   fail if artifact resolution or download fails.

6. WHEN artifact image is loaded, THE SYSTEM SHALL derive a canonical local
   image alias (`charon:artifact`) from validated load result and SHALL use only
   that alias for `docker create` in artifact-based paths.

7. WHEN artifact metadata parsing is required, THE SYSTEM SHALL NOT depend only
   on `RepoTags[0]`; it SHALL validate all available repo tags and SHALL support
   fallback selection using docker load image ID when tags are absent/corrupt.

8. IF no valid tag and no valid load image ID can be resolved, THEN THE SYSTEM
   SHALL hard fail before extraction.

9. WHEN event is `pull_request` or `push`, THE SYSTEM SHALL build and use
   `charon:local` only and SHALL NOT execute artifact lookup/load logic.

### 3.2 Deterministic Policy Decisions

#### Policy A: `workflow_run` Missing Artifact

Decision: hard fail only.

No skip behavior is allowed for upstream-success `workflow_run`.

#### Policy A1: `workflow_run` Upstream Event Contract

Decision: upstream event MUST be `pull_request`.

If upstream event is `push` or any non-PR event, fail immediately with
`unsupported_upstream_event`; no artifact path execution is allowed.

#### Policy B: `workflow_dispatch`

Decision: artifact-only manual replay.

No local-build fallback is allowed for `workflow_dispatch`. Required input is
`pr_number`; missing input is immediate hard fail.

### 3.3 Image Selection Hardening Contract

For step `Load Docker image` in `.github/workflows/security-pr.yml`:

1. Validate artifact file exists and is readable tar.
2. Parse `manifest.json` and iterate all candidate tags under `RepoTags[]`.
3. Run `docker load` and capture structured output.
4. Resolve source image by deterministic priority:
   - First valid tag from `RepoTags[]` that exists locally after load.
   - Else image ID extracted from `docker load` output (if present).
   - Else fail.
5. Retag resolved source to `charon:artifact`.
6. Emit outputs:
   - `image_ref=charon:artifact`
   - `source_image_ref=<resolved tag or image id>`
   - `source_resolution_mode=manifest_tag|load_image_id`

### 3.4 CI Security Hardening Requirements

For job `security-scan` in `.github/workflows/security-pr.yml`:

1. THE SYSTEM SHALL enforce least-privilege permissions by default:
   - `contents: read`
   - `actions: read`
   - `security-events: write`
   - No additional write scopes unless explicitly required.

2. THE SYSTEM SHALL restrict `pull-requests: write` usage to only steps that
   require PR annotations/comments. If no such step exists, this permission
   SHALL be removed.

3. THE SYSTEM SHALL enforce workflow_run trust boundary guards:
   - Upstream workflow name must match expected producer.
   - Upstream conclusion must be `success`.
   - Upstream event must be `pull_request` only.
   - Upstream head repository must equal `${{ github.repository }}` (same-repo
     trust boundary), otherwise hard fail.

4. THE SYSTEM SHALL NOT use untrusted `workflow_run` payload values to build
   shell commands without validation and quoting.

### 3.5 Step-Level Scope in `security-pr.yml`

Targeted steps:

1. `Extract PR number from workflow_run`
2. `Validate workflow_run upstream event contract`
3. `Check for PR image artifact`
4. `Skip if no artifact` (to be converted to deterministic fail paths for
   `workflow_run` and `workflow_dispatch`)
5. `Load Docker image`
6. `Extract charon binary from container`

### 3.6 Event Data Flow (Deterministic)

```text
pull_request/push
  -> Build Docker image (Local)
  -> image_ref=charon:local
  -> Extract /app/charon
  -> Trivy scan

workflow_run (upstream success only)
   -> Assert upstream event == pull_request (hard fail if false)
  -> Require artifact exists (hard fail if missing)
  -> Load/validate image
  -> image_ref=charon:artifact
  -> Extract /app/charon
  -> Trivy scan

workflow_dispatch
  -> Require pr_number input (hard fail if missing)
  -> Resolve pr-image-<pr_number> artifact (hard fail if missing)
  -> Load/validate image
  -> image_ref=charon:artifact
  -> Extract /app/charon
  -> Trivy scan
```

### 3.7 Error Handling Matrix

| Step | Condition | Required Behavior |
|---|---|---|
| Validate workflow_run upstream event contract | `workflow_run` upstream event is not `pull_request` | Hard fail with `unsupported_upstream_event`; stop before artifact lookup |
| Check for PR image artifact | `workflow_run` upstream success but artifact missing | Hard fail with run id + artifact name |
| Extract PR number from workflow_run | `workflow_dispatch` and empty `inputs.pr_number` | Hard fail with input requirement message |
| Load Docker image | Missing/corrupt `charon-pr-image.tar` | Hard fail before `docker load` |
| Load Docker image | Missing/corrupt `manifest.json` | Attempt load-image-id fallback; fail if unresolved |
| Load Docker image | No valid `RepoTags[]` and no load image id | Hard fail |
| Extract charon binary from container | Empty/invalid `image_ref` | Hard fail before `docker create` |
| Extract charon binary from container | `/app/charon` missing | Hard fail with chosen image reference |

### 3.8 API/DB Changes

No backend API, frontend, or database schema changes.

---

## 4. Implementation Plan

### Phase 1: Playwright Impact Check

1. Mark Playwright scope as N/A because this change is workflow-only.
2. Record N/A rationale in PR description.

### Phase 2: Deterministic Event Policies

File: `.github/workflows/security-pr.yml`

1. Convert ambiguous skip/fail logic to hard-fail policy for
   `workflow_run` missing artifact after upstream success.
2. Enforce deterministic `workflow_dispatch` policy:
   - Required `pr_number` input.
   - Artifact-only replay path.
   - No local fallback.
3. Enforce PR-only `workflow_run` event contract:
   - Upstream event must be `pull_request`.
   - Upstream `push` or any non-PR event hard fails with
     `unsupported_upstream_event`.

### Phase 3: Image Selection Hardening

File: `.github/workflows/security-pr.yml`

1. Harden `Load Docker image` with manifest validation and multi-tag handling.
2. Add fallback resolution via docker load image ID.
3. Emit explicit outputs for traceability (`source_resolution_mode`).
4. Ensure extraction consumes only selected alias (`charon:artifact`).

### Phase 4: CI Security Hardening

File: `.github/workflows/security-pr.yml`

1. Reduce job permissions to least privilege.
2. Remove/conditionalize `pull-requests: write` if not required.
3. Add workflow_run trust-boundary guard conditions and explicit fail messages.

### Phase 5: Validation

1. `pre-commit run actionlint --files .github/workflows/security-pr.yml`
2. Simulate deterministic paths (or equivalent CI replay) for all matrix cases.
3. Verify logs show chosen `source_image_ref` and `source_resolution_mode`.

---

## 5. Validation Matrix

| ID | Trigger Path | Scenario | Expected Result |
|---|---|---|---|
| V1 | `workflow_run` | Upstream success + artifact present | Pass, uses `charon:artifact` |
| V2 | `workflow_run` | Upstream success + artifact missing | Hard fail (non-zero) |
| V3 | `workflow_run` | Upstream success + artifact manifest corrupted | Hard fail after validation/fallback attempt |
| V4 | `workflow_run` | Upstream success + upstream event `push` | Hard fail with `unsupported_upstream_event` |
| V5 | `pull_request` | Direct PR trigger | Pass, uses `charon:local`, no artifact lookup |
| V6 | `push` | Direct push trigger | Pass, uses `charon:local`, no artifact lookup |
| V7 | `workflow_dispatch` | Missing `pr_number` input | Hard fail immediately |
| V8 | `workflow_dispatch` | Valid `pr_number` + artifact exists | Pass, uses `charon:artifact` |
| V9 | `workflow_dispatch` | Valid `pr_number` + artifact missing | Hard fail |
| V10 | `workflow_run` | Upstream from untrusted repository context | Hard fail by trust-boundary guard |

---

## 6. Acceptance Criteria

1. Plan states unambiguous hard-fail behavior for missing artifact on
   `workflow_run` after upstream `pull_request` success.
2. Plan states `workflow_run` event contract is PR-only and that upstream
   `push` is a deterministic hard-fail contract violation.
3. Plan states one deterministic `workflow_dispatch` policy in SHALL terms:
   required `pr_number`, artifact-only path, no local fallback.
4. Plan defines robust image resolution beyond `RepoTags[0]`, including
   load-image-id fallback and deterministic aliasing.
5. Plan includes least-privilege permissions and explicit workflow_run trust
   boundary constraints.
6. Plan includes validation coverage for `pull_request` and direct `push` local
   paths plus negative paths: unsupported upstream event, missing dispatch
   input, missing artifact, corrupted/missing manifest.
7. Root cause remains explicit: image-reference mismatch inside
   `.github/workflows/security-pr.yml` after successful artifact load.

---

## 7. Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Overly strict dispatch policy blocks ad-hoc scans | Medium | Document explicit manual replay contract in workflow description |
| PR-only workflow_run contract fails upstream push-triggered runs | Medium | Intentional contract enforcement; document `unsupported_upstream_event` and route push scans through direct push path |
| Manifest parsing edge cases | Medium | Multi-source resolver with load-image-id fallback |
| Permission tightening breaks optional PR annotations | Low | Make PR-write permission step-scoped only if needed |
| Trust-boundary guards reject valid internal events | Medium | Add clear diagnostics and test cases V1/V10 |

---

## 8. PR Slicing Strategy

### Decision

Single PR.

### Trigger Reasons

1. Change is isolated to one workflow (`security-pr.yml`).
2. Deterministic policy + hardening are tightly coupled and safest together.
3. Split PRs would create temporary policy inconsistency.

### Ordered Slice

#### PR-1: Deterministic Policy and Security Hardening for `security-pr.yml`

Scope:

1. Deterministic missing-artifact handling (`workflow_run` hard fail).
2. Deterministic `workflow_dispatch` artifact-only policy.
3. Hardened image resolution and aliasing.
4. Least-privilege + trust-boundary constraints.
5. Validation matrix execution evidence.

Files:

1. `.github/workflows/security-pr.yml`
2. `docs/plans/current_spec.md`

Dependencies:

1. `.github/workflows/docker-build.yml` artifact naming contract unchanged.

Validation Gates:

1. actionlint passes.
2. Validation matrix V1-V10 results captured.
3. No regression to `ghcr.io/...:pr-<num> not found` pattern.

Rollback / Contingency:

1. Revert PR-1 if trust-boundary guards block legitimate same-repo runs.
2. Keep hard-fail semantics; adjust guard predicate, not policy.

---

## 9. Handoff

After approval, implementation handoff to Supervisor SHALL include:

1. Exact step-level edits required in `.github/workflows/security-pr.yml`.
2. Proof logs for each failed/pass matrix case.
3. Confirmation that no files outside plan scope were required.
3. Require explicit evidence that artifact path no longer performs GHCR PR tag
   reconstruction.
