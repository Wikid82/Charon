---
title: "CI Pipeline Consolidation"
status: "draft"
scope: "ci/pipeline"
notes: This plan replaces the current CI workflow chain with a single pipeline that supports PR triggers while keeping maintenance workflows scheduled.
---

## 1. Introduction

This plan consolidates the existing CI workflows into one pipeline
workflow that can trigger on pull requests across branches (in addition
to manual dispatch). The pipeline will run in a strict order defined by
the user:
lint, build, parallel integration prerequisites, E2E, parallel
coverage, then security scans. All stages will consume the same built
Docker image to ensure consistent test results.

Maintenance workflows remain scheduled (nightly/weekly/Renovate/repo
health) and are explicitly out of scope for trigger changes.

Out of scope: Alpine migration. Any base-image migration work must be
captured in a separate plan/spec.

Objectives:

- Enable the pipeline to run on pull requests across branches in
  addition to manual dispatch.
- Create one pipeline workflow that sequences jobs in the requested
  order with explicit dependencies.
- Ensure all integration, E2E, coverage, and security checks use the
  same image digest produced by the pipeline build job.
- Push the pipeline image to Docker Hub and GHCR, but use Docker Hub as
  the test image source.
- Keep the E2E image tag unchanged from the current convention.
- Align the pipeline with the current Definition of Done (DoD) by
  mapping required checks into pipeline stages.
- Preserve scheduled maintenance workflows and do not convert them to
  manual-only triggers.

## 2. Research Findings

### 2.1 Current Workflow Topology

The CI chain is currently split across multiple workflows linked by
workflow_run triggers. The core files in scope are:

- .github/workflows/docker-build.yml
- .github/workflows/docker-lint.yml
- .github/workflows/e2e-tests-split.yml
- .github/workflows/quality-checks.yml
- .github/workflows/codecov-upload.yml
- .github/workflows/codeql.yml
- .github/workflows/security-pr.yml
- .github/workflows/supply-chain-pr.yml
- .github/workflows/cerberus-integration.yml
- .github/workflows/crowdsec-integration.yml
- .github/workflows/waf-integration.yml
- .github/workflows/rate-limit-integration.yml
- .github/workflows/benchmark.yml
- .github/workflows/supply-chain-verify.yml

Several maintenance workflows also exist (nightly builds, weekly
security rebuilds, repository health, Renovate automation). They are
not part of the requested pipeline order and will remain scheduled
with their existing triggers.

### 2.2 Current Image Tagging and Digest Sources

- docker-build.yml outputs a build digest from the buildx iidfile and
  pushes images to GHCR and Docker Hub.
- Tags currently include:
  - pr-{number}-{short-sha} for PRs
  - {sanitized-branch}-{short-sha} for feature branches
  - latest/dev/nightly for main/development/nightly builds
  - sha-{short-sha} for non-PR builds
  - nightly branch tag (per user request) for nightly branch builds

### 2.3 Definition of Done (DoD) Alignment

The DoD requires E2E tests to run first, then security scans, pre-commit
checks, static analysis, coverage gates, type checks, and build
verification. The requested pipeline order differs by placing E2E after
integration prerequisites and before coverage and security scans.

Decision: the pipeline order is authoritative for CI. The DoD
order remains guidance for local workflows, but CI ordering will follow
the requested pipeline sequence and map required checks into stages.

## 3. Technical Specifications

### 3.1 Workflow Trigger Strategy

The new pipeline workflow will trigger on pull_request across branches
and workflow_dispatch. Existing CI workflows listed in Section 2.1 will
be converted to workflow_dispatch only (no PR triggers). Existing
workflow_run triggers will be removed. Scheduled maintenance workflows
will keep their schedules intact.

### 3.2 New Pipeline Workflow

Create a new workflow file that runs the entire pipeline in one run:

- File: .github/workflows/ci-pipeline.yml
- Trigger: workflow_dispatch and pull_request across branches
- Inputs:
  - image_tag_override (optional)
  - run_coverage (boolean)
  - run_security_scans (boolean)
  - run_integration (boolean)
  - run_e2e (boolean)

### 3.3 Job Order and Dependencies

The pipeline job graph will enforce the requested order.

Job dependency table:

| Job | Purpose | Needs |
| --- | --- | --- |
| lint | Dockerfile lint, Go lint, frontend lint, repo health | none |
| build-image | Build and push Docker image, emit digest | lint |
| integration-cerberus | Cerberus integration tests | build-image |
| integration-crowdsec | CrowdSec integration tests | build-image |
| integration-waf | WAF integration tests | build-image |
| integration-ratelimit | Rate limit integration tests | build-image |
| e2e | Playwright E2E split workflow equivalent | integration-* |
| coverage-backend | Go tests with coverage and Codecov upload | e2e |
| coverage-frontend | Frontend tests with coverage and Codecov upload | e2e |
| coverage-e2e | Optional E2E coverage job | e2e |
| security-codeql | CodeQL Go and JS scans | coverage-* |
| security-trivy | Trivy image scan | coverage-* |
| security-supply-chain | SBOM generation and attestation | coverage-* |

Integration jobs should run in parallel. Coverage and security jobs
should run in parallel within their stages.

### 3.4 Shared Image Strategy

All downstream jobs must use the same image digest produced by the
build-image job. The build-image job will output:

- image_digest: from docker/build-push-action or iidfile
- image_ref: docker.io/wikid82/charon@sha256:...
- image_ref_ghcr: ghcr.io/wikid82/charon@sha256:...
- image_tag: pr-{number}-{short-sha} or sha-{short-sha}

Downstream jobs will pull the image by digest to ensure immutability and
retag it locally as charon:e2e-test for docker compose usage. For test
stages, the image source registry must be Docker Hub even though GHCR is
also pushed. The E2E image tag must remain unchanged from the current
convention.

### 3.5 Required File Updates

Workflow updates to manual-only triggers:

- .github/workflows/docker-build.yml
- .github/workflows/docker-lint.yml
- .github/workflows/e2e-tests-split.yml
- .github/workflows/quality-checks.yml
- .github/workflows/codecov-upload.yml
- .github/workflows/codeql.yml
- .github/workflows/security-pr.yml
- .github/workflows/supply-chain-pr.yml
- .github/workflows/cerberus-integration.yml
- .github/workflows/crowdsec-integration.yml
- .github/workflows/waf-integration.yml
- .github/workflows/rate-limit-integration.yml
- .github/workflows/benchmark.yml
- .github/workflows/supply-chain-verify.yml

Workflow additions (PR + manual triggers):

- .github/workflows/ci-pipeline.yml

Optional configuration updates if required for image reuse:

- .docker/compose/docker-compose.playwright-ci.yml (use image ref or
  tag via environment variable)
- scripts/*.sh or .github/skills/scripts/skill-runner.sh, only if
  necessary to accept image ref overrides

### 3.6 Error Handling and Gates

- Fail fast in lint and build stages.
- Integration, E2E, coverage, and security stages should fail the
  pipeline if any job fails.
- Preserve existing retry behavior for registry pushes and pulls.

### 3.7 Required Checks and Branch Protection

- Add a pipeline summary job (e.g., pipeline-gate) that depends on all
  pipeline jobs and fails if any required job fails.
- Require the pipeline-gate status check in branch protection/rulesets
  for main and release branches.
- Pipeline workflows remain required by enforcing that the pipeline is
  run against the merge commit or branch HEAD before merge.
- Keep admin bypass disabled for protected branches unless explicitly
  approved.

### 3.7 Requirements (EARS Notation)

- WHEN a user manually dispatches the pipeline or opens a pull request,
  THE SYSTEM SHALL run the lint stage before any build or test jobs.
- WHEN the build stage completes, THE SYSTEM SHALL publish a single
  image digest that all later jobs consume.
- WHEN any integration test fails, THE SYSTEM SHALL stop the pipeline
  before E2E execution.
- WHEN E2E completes, THE SYSTEM SHALL run coverage jobs in parallel.
- WHEN coverage completes, THE SYSTEM SHALL run security scans in
  parallel using the same image digest.
- WHEN the pipeline pushes images, THE SYSTEM SHALL push to Docker Hub
  and GHCR but use Docker Hub as the test image source.
- WHEN E2E runs, THE SYSTEM SHALL keep the existing E2E image tag and
  preserve the security shard as a separate shard with the current
  timeout-safe layout.
- IF any required DoD check fails, THEN THE SYSTEM SHALL fail the
  pipeline and report the failing stage.

## 4. Implementation Plan

### Phase 1: Playwright Tests (Behavior Baseline)

- Validate the existing Playwright suites used by e2e-tests-split.yml
  can run under the new pipeline using the shared image digest.
- Confirm the E2E stage still honors security and non-security shards
  and that Cerberus toggle logic is preserved.

### Phase 2: Backend and CI Workflow Refactor

- Add the new pipeline workflow file.
- Modify existing CI workflows in Section 3.5 to use workflow_dispatch
  only (no pull_request triggers).
- Move the docker-build logic into the pipeline build-image job and
  export digest and tag outputs.
- Update integration job steps to consume the digest and retag locally
  as needed for existing scripts.

### Phase 3: Frontend and E2E Workflow Refactor

- Update the E2E steps to pull the Docker Hub digest and retag to
  charon:e2e-test before docker compose starts.
- Ensure environment variables or compose overrides reference the
  shared image and keep the E2E tag unchanged.
- Preserve E2E sharding so the security shard remains separate and the
  shard layout avoids timeouts.

### Phase 4: Coverage and Security Stage Consolidation

- Replace codecov-upload.yml and codeql.yml with pipeline jobs that run
  after E2E completion.
- Ensure Codecov uploads and CodeQL scans run with the same code
  checkout and digest metadata for traceability.

### Phase 5: Documentation and DoD Alignment

- Update docs/plans/current_spec.md with the final pipeline plan.
- Document the DoD ordering impact and confirm whether the DoD should
  be updated to match the new pipeline order or the pipeline should
  adapt to the DoD ordering.

### Phase 6: Branch Protection Updates

- Update branch protection/rulesets to require the pipeline-gate check.
- Document the manual pipeline run requirement for PR validation.

## 5. Acceptance Criteria

- The pipeline workflow triggers via pull_request across branches and
  workflow_dispatch.
- All CI workflows listed in Section 3.5 trigger via
  workflow_dispatch only and no longer use workflow_run or
  pull_request.
- Maintenance workflows (nightly/weekly/Renovate/repo health) retain
  their scheduled triggers and are not changed to PR/manual-only.
- The new pipeline workflow runs lint, build, integration, E2E,
  coverage, and security stages in the requested order.
- Integration, E2E, coverage, and security jobs consume the same image
  digest produced by the build stage.
- The pipeline exposes image_digest and image_ref outputs for audit
  and debugging.
- All DoD-required checks are represented in the pipeline and fail the
  run when they do not pass.
- The pipeline pushes images to Docker Hub and GHCR, and test stages
  pull from Docker Hub.
- E2E sharding keeps the security shard separate and retains the
  timeout-safe shard layout.
- The nightly branch tag remains part of the image tagging scheme.
## 6. Risks and Mitigations

 - Risk: PR-triggered pipeline increases CI load and could cause noisy
   failures on draft or experimental branches.
  - Mitigation: keep legacy workflows manual-only, enforce the
    pipeline-gate required check, and allow maintainers to re-run the
    pipeline as needed.

## 7. Confidence Score

Confidence: 86 percent

Rationale: Manual pipeline consolidation is well scoped, but requires
careful coordination with branch protection and required checks.
