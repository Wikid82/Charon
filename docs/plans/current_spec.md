---
title: "CI Docker Build and Scanning Blocker (PR #666)"
status: "draft"
scope: "ci/docker-build-scan"
---

## 1. Introduction

This plan addresses the CI failure that blocks Docker build and scanning
for PR #666. The goal is to restore a clean, deterministic pipeline
where the image builds once, scans consistently, and security artifacts
align across workflows. The approach is minimal and evidence-driven:
collect logs, map the path, isolate the blocker, and apply the smallest
effective fix.

Objectives:

- Identify the exact failing step in the build/scan chain.
- Trace the failure to a reproducible root cause.
- Propose minimal workflow/Dockerfile changes to restore green CI.
- Ensure all scan workflows resolve the same PR image.
- Review .gitignore, codecov.yml, .dockerignore, and Dockerfile if
  needed for artifact hygiene.

## 2. Research Findings

CI workflow and build context (already reviewed):

- Docker build orchestration: .github/workflows/docker-build.yml
- Security scan for PR artifacts: .github/workflows/security-pr.yml
- Supply-chain verification for PRs: .github/workflows/supply-chain-pr.yml
- SBOM verification for non-PR builds: .github/workflows/supply-chain-verify.yml
- Dockerfile linting: .github/workflows/docker-lint.yml and
  .hadolint.yaml
- Weekly rebuild and scan: .github/workflows/security-weekly-rebuild.yml
- Quality checks (non-Docker): .github/workflows/quality-checks.yml
- Build context filters: .dockerignore
- Runtime Docker build instructions: Dockerfile
- Ignored artifacts: .gitignore
- Coverage configuration: codecov.yml

Observed from the public workflow summary (PR #666):

- Job build-and-push failed in the Docker Build, Publish & Test workflow.
- Logs require GitHub authentication; obtained via gh CLI after auth.
- Evidence status: confirmed via gh CLI logs (see Results).

Root cause captured from CI logs (authenticated gh CLI):

- npm ci failed with ERESOLVE due to eslint@10 conflicting with the
  @typescript-eslint peer dependency range.

Secondary/unconfirmed mismatch to verify only if remediation fails:

- PR tags are generated as pr-{number}-{short-sha} in docker-build.yml.
- Several steps reference pr-{number} (no short SHA) and use
  --pull=never.
- This can cause image-not-found errors after Buildx pushes without
  --load.

## 3. Technical Specifications

### 3.1 CI Flow Map (Build -> Scan -> Verify)

```mermaid
flowchart LR
  A[PR Push] --> B[docker-build.yml: build-and-push]
  B --> C[docker-build.yml: scan-pr-image]
  B --> D[security-pr.yml: Trivy binary scan]
  B --> E[supply-chain-pr.yml: SBOM + Grype]
  B --> F[supply-chain-verify.yml: SBOM verify (non-PR)]
```

### 3.2 Primary Failure Hypotheses (Ordered)

1) Eslint peer dependency conflict (confirmed root cause)

- npm ci failed with ERESOLVE due to eslint@10 conflicting with the
  @typescript-eslint peer dependency range.

2) Tag mismatch between build output and verification steps

- Build tags for PRs are pr-{number}-{short-sha} (metadata action).
- Verification steps reference pr-{number} (no SHA) and do not pull.
- This is consistent with image-not-found errors.
  Status: unconfirmed secondary hypothesis.

3) Buildx push without local image for verification steps

- Build uses docker buildx build --push without --load.
- Verification steps use docker run --pull=never with local tags.
- Buildx does not allow --load with multi-arch builds; --load only
  produces a single-platform image. For multi-arch, prioritize pull by
  digest or publish a single-platform build output for local checks.
- If the tag is not local and not pulled, verification fails.

4) Dockerfile stage failure during network-heavy steps

- gosu-builder: git clone and Go build
- frontend-builder: npm ci / npm run build
- backend-builder: go mod download / xx-go build
- caddy-builder: xcaddy build and Go dependency patching
- crowdsec-builder: git clone + go get + sed patch
- GeoLite2 download and checksum verification

Any of these can fail with network timeouts or dependency resolution
errors in CI. The eslint peer dependency conflict is confirmed; other
hypotheses remain unconfirmed.

### 3.3 Evidence Required (Single-Request Capture)

Evidence capture completed in a single session. The following items
were captured:

- Full logs for the failing docker-build.yml build-and-push job
- Failing step name and exit code
- Buildx command line as executed
- Metadata tags produced by docker/metadata-action
- Dockerfile stage that failed (if build failure)

If accessible, also capture downstream scan job logs to confirm the image
reference used.

### 3.4 Specific Files and Components to Investigate

Docker build and tagging:

- .github/workflows/docker-build.yml
  - Generate Docker metadata (tag formatting)
  - Build and push Docker image (with retry)
  - Verify Caddy Security Patches
  - Verify CrowdSec Security Patches
  - Job: scan-pr-image

Security scanning:

- .github/workflows/security-pr.yml
  - Extract PR number from workflow_run
  - Extract charon binary from container (image reference)
  - Trivy scans (fs, SARIF, blocking table)

Supply-chain verification:

- .github/workflows/supply-chain-pr.yml
  - Check for PR image artifact
  - Load Docker image (artifact)
  - Build Docker image (Local)

Dockerfile stages and critical components:

- gosu-builder: /tmp/gosu build
- frontend-builder: /app/frontend build
- backend-builder: /app/backend build
- caddy-builder: xcaddy build and Go dependency patching
- crowdsec-builder: Go build and sed patch in
  pkg/exprhelpers/debugger.go
- Final runtime stage: GeoLite2 download and checksum

### 3.5 Tag and Digest Source-of-Truth Propagation

Source of truth for the PR image reference is the output of the metadata
and build steps in docker-build.yml. Downstream workflows must consume a
single canonical reference, defined as:

- primary: digest from buildx outputs (immutable)
- secondary: pr-{number}-{short-sha} tag (human-friendly)

Propagation rules:

- docker-build.yml SHALL publish the digest and tag as job outputs.
- docker-build.yml SHALL write digest and tag to a small artifact
  (e.g., pr-image-ref.txt) for downstream workflow_run consumers.
- security-pr.yml and supply-chain-pr.yml SHALL prefer the digest from
  outputs or artifact, and only fall back to tag if digest is absent.
- Any step that runs a local container SHALL ensure the referenced image
  is available by either --load (local) or explicit pull by digest.

### 3.6 Required Outcome (EARS Requirements)

- WHEN a pull request triggers docker-build.yml, THE SYSTEM SHALL build a
  PR image tagged as pr-{number}-{short-sha} and emit its digest.
- WHEN verification steps run in docker-build.yml, THE SYSTEM SHALL
  reference the same digest or tag emitted by the build step.
- WHEN security-pr.yml runs for a workflow_run, THE SYSTEM SHALL resolve
  the PR image using the digest or the emitted tag.
- WHEN supply-chain-pr.yml runs, THE SYSTEM SHALL load the exact PR image
  by digest or by the emitted tag without ambiguity.
- IF the image reference cannot be resolved, THEN THE SYSTEM SHALL fail
  fast with a clear message that includes the expected digest and tag.

### 3.7 Config Hygiene Review (Requested Files)

.gitignore:

- Ensure CI scan artifacts are ignored locally, including any new names
  introduced by fixes (e.g., trivy-pr-results.sarif,
  trivy-binary-results.sarif, grype-results.json,
  sbom.cyclonedx.json).

codecov.yml:

- Confirm CI-generated security artifacts are excluded from coverage.
- Add any new artifact names if introduced by fixes.

.dockerignore:

- Verify required frontend/backend sources and manifests are included.

Dockerfile:

- Review GeoLite2 download behavior for CI reliability.
- Confirm CADDY_IMAGE build-arg naming consistency across workflows.

## 4. Implementation Plan (Minimal-Request Phases)

### Phase 0: Evidence Capture (Single Request)

Status: completed. Evidence captured and root cause confirmed.

- Retrieve full logs for the failing docker-build.yml build-and-push job.
- Capture the exact failing step, error output, and emitted tags/digest.
- Record the buildx command output as executed.
- Capture downstream scan logs if accessible to confirm image reference.

### Phase 1: Reproducibility Pass (Single Local Build)

- Run a local docker buildx build using the same arguments as
  docker-build.yml.
- Capture any stage failures and map them to Dockerfile stages.
- Confirm whether Buildx produces local images or only remote tags.

### Phase 2: Root Cause Isolation

Status: completed. Root cause identified as the eslint peer dependency
conflict in the frontend build stage.

- If failure is tag mismatch, trace tag references across docker-build.yml,
  security-pr.yml, and supply-chain-pr.yml.
- If failure is a Dockerfile stage, isolate to specific step (gosu,
  frontend, backend, caddy, crowdsec, GeoLite2).
- If failure is network-related, document retries/timeout behavior and
  any missing mirrors.

### Phase 3: Targeted Remediation Plan

Focus on validating the eslint remediation. Revisit secondary
hypotheses only if the remediation does not resolve CI.

Conditional options (fallbacks, unconfirmed):

Option A (Tag alignment):

- Update verification steps to use pr-{number}-{short-sha} tag.
- Or add a secondary tag pr-{number} for compatibility.

Option B (Local image availability):

- Add --load for PR builds so verification can run locally.
- Or explicitly pull by digest/tag before verification and remove
  --pull=never.

Option C (Workflow scan alignment):

- Update security-pr.yml and supply-chain-pr.yml to consume the digest
  or emitted tag from docker-build.yml outputs/artifact.
- Add fallback order: digest artifact -> emitted tag -> local build.

## 5. Results (Evidence)

Evidence status: confirmed via gh CLI logs after authentication.

Root cause (confirmed):

- Align eslint with the @typescript-eslint peer range to resolve npm ci
  ERESOLVE in the frontend build stage.

### Phase 4: Validation (Minimal Jobs)

- Rerun docker-build.yml for the PR (or workflow_dispatch).
- Confirm build-and-push succeeds and verification steps resolve the
  exact digest or tag.
- Confirm security-pr.yml and supply-chain-pr.yml resolve the same
  digest or tag and complete scans.
- Deterministic check: use docker buildx imagetools inspect on the
  emitted tag and compare the reported digest to the recorded build
  digest, or pull by digest and verify the digest of the local image
  matches the build output.

### Phase 5: Documentation and Hygiene

- Document the final tag/digest propagation in this plan.
- Update .gitignore / .dockerignore / codecov.yml if new artifacts are
  produced.

## 6. Acceptance Criteria

- docker-build.yml build-and-push succeeds for PR #666.
- Verification steps resolve the same digest or tag emitted by build.
- security-pr.yml and supply-chain-pr.yml consume the same digest or tag
  published by docker-build.yml.
- A validation check confirms tag-to-digest alignment across workflows
  (digest matches tag for the PR image), using buildx imagetools inspect
  or an equivalent digest comparison.
- No new CI artifacts are committed to the repository.
- Root cause is documented with logs and mapped to specific steps.

## 7. Risks and Mitigations

- Risk: CI logs are inaccessible without login, delaying diagnosis.
  - Mitigation: request logs or export them once, then reproduce locally.

- Risk: Multiple workflows use divergent tag formats.
  - Mitigation: define a single source of truth for PR tags and digest
    propagation.

- Risk: Buildx produces only remote tags, breaking local verification.
  - Mitigation: add --load for PR builds or pull by digest before
    verification.

## 8. Confidence Score

Confidence: 88 percent

Rationale: The eslint peer dependency conflict is confirmed as the
frontend build failure. Secondary tag mismatch hypotheses remain
unconfirmed and are now conditional fallbacks only.
