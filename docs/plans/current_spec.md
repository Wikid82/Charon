## 1. Introduction

### Overview
Compatibility rollout for Caddy `2.11.1` is already reflected in the build
default (`Dockerfile` currently sets `ARG CADDY_VERSION=2.11.1`).

This plan is now focused on rollout verification and regression-proofing, not
changing the default ARG.

### Objective
Establish deterministic, evidence-backed gates that prove published images and
security artifacts are fresh, digest-bound, and aligned across registries for
the Caddy `2.11.1` rollout.

## 2. Current State (Verified)

1. `Dockerfile` default is already `CADDY_VERSION=2.11.1`.
2. `ARCHITECTURE.md` now reports Caddy `2.11.1`.
3. Existing scan artifacts can become stale if not explicitly tied to pushed
   digests.

## 3. Technical Specification (EARS)

1. WHEN image builds run without an explicit `CADDY_VERSION` override, THE
   SYSTEM SHALL continue producing Caddy `2.11.1`.
2. WHEN an image tag is pushed, THE SYSTEM SHALL validate index digest parity
   between GHCR and Docker Hub for that same tag.
3. WHEN multi-arch images are published, THE SYSTEM SHALL validate per-arch
   digest parity across GHCR and Docker Hub for each platform present.
4. WHEN vulnerability and SBOM scans execute, THE SYSTEM SHALL scan
   `image@sha256:<index-digest>` instead of mutable tags.
5. WHEN scan artifacts are generated, THE SYSTEM SHALL prove artifacts were
   produced after the push event in the same validation run.
6. IF a verification gate fails, THEN THE SYSTEM SHALL block rollout sign-off
   until all gates pass.

## 4. Scope and Planned Edits

### In scope
1. `docs/plans/current_spec.md` (this plan refresh).
2. `ARCHITECTURE.md` version sync is already complete (`2.11.1`); no pending
   update is required in this plan.
3. Verification workflow/checklist updates needed to enforce deterministic gates.

### Out of scope
1. No functional Caddy build logic changes unless a verification failure proves
   they are required.
2. No plugin list or patch-scenario refactors.

## 5. Deterministic Acceptance Gates

### Gate 1: Digest Freshness (pre/post push)
1. Capture pre-push index digest for target tag on GHCR and Docker Hub.
2. Push image.
3. Capture post-push index digest on GHCR and Docker Hub.
4. Pass criteria:
   - Post-push index digest changed as expected from pre-push (or matches
     intended new digest when creating new tag).
   - GHCR and Docker Hub index digests are identical for the tag.
   - Per-arch digests are identical across registries for each published
     platform.

### Gate 2: Digest-Bound Rescan
1. Resolve the post-push index digest.
2. Run all security scans against immutable ref:
   - `ghcr.io/<owner>/<repo>@sha256:<index-digest>`
   - Optional mirror check against Docker Hub digest ref.
3. Pass criteria:
   - No scan uses mutable tags as the primary target.
   - Artifact metadata and logs show digest reference.

### Gate 3: Artifact Freshness
1. Record push timestamp and digest capture timestamp.
2. Generate SBOM and vuln artifacts after push in the same run.
3. Pass criteria:
   - Artifact generation timestamps are greater than push timestamp.
   - Artifacts are newly created/overwritten in this run.
   - Evidence ties each artifact to the scanned digest.

### Gate 4: Evidence Block (mandatory)
Every validation run must include a structured evidence block with:
1. Tag name.
2. Index digest.
3. Per-arch digests.
4. Scan tool versions.
5. Push and scan timestamps.
6. Artifact file names produced in this run.

## 6. Implementation Plan

### Phase 1: Baseline Capture
1. Confirm current `Dockerfile` default remains `2.11.1`.
2. Capture pre-push digest state for target tag across both registries.

### Phase 2: Docs Sync
1. Confirm `ARCHITECTURE.md` remains synced at Caddy `2.11.1`.

### Phase 3: Push and Verification
1. Push validation tag.
2. Execute Gate 1 (digest freshness and parity).
3. Execute Gate 2 (digest-bound rescan).
4. Execute Gate 3 (artifact freshness).
5. Produce Gate 4 evidence block.

### Phase 4: Sign-off
1. Mark rollout verified only when all gates pass.
2. If any gate fails, open follow-up remediation task before merge.

## 7. Acceptance Criteria

1. Plan and execution no longer assume Dockerfile default is beta.
2. Objective is rollout verification/regression-proofing for Caddy `2.11.1`.
3. `ARCHITECTURE.md` version metadata is included in required docs sync.
4. Digest freshness gate passes:
   - Pre/post push validation completed.
   - GHCR and Docker Hub index digest parity confirmed.
   - Per-arch digest parity confirmed.
5. Digest-bound rescan gate passes with `image@sha256` scan targets.
6. Artifact freshness gate passes with artifacts produced after push in the same
   run.
7. Evidence block is present and complete with:
   - Tag
   - Index digest
   - Per-arch digests
   - Scan tool versions
   - Timestamps
   - Artifact names

## 8. PR Slicing Strategy

### Decision
Single PR.

### Trigger Reasons
1. Scope is narrow and cross-cutting risk is low.
2. Verification logic and docs sync are tightly coupled.
3. Review size remains small and rollback is straightforward.

### PR-1
1. Scope:
   - Refresh `docs/plans/current_spec.md` to verification-focused plan.
   - Sync `ARCHITECTURE.md` Caddy version metadata.
   - Add/adjust verification checklist content needed for gates.
2. Dependencies:
   - Existing publish/scanning pipeline availability.
3. Validation gates:
   - Gate 1 through Gate 4 all required.

## 9. Rollback and Contingency

1. If verification updates are incorrect or incomplete, revert PR-1.
2. If rollout evidence fails, hold release sign-off and keep last known-good
   digest as active reference.
3. Re-run verification with corrected commands/artifacts before reattempting
   sign-off.
