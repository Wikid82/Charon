---
title: "Revert CI Pipeline Consolidation"
status: "draft"
scope: "ci/workflows, integration, e2e, security"
notes: Restore per-workflow pull_request triggers, retire ci-pipeline.yml, and reestablish self-contained image builds.
---

## 1. Introduction

This plan dismantles the consolidated CI pipeline and restores individual
pull_request triggers for component workflows. The goal is to return to a
simple, independent workflow model where each integration or test workflow
runs on PRs without relying on a central pipeline or shared image artifacts.

Objectives:

- Identify workflows that had pull_request triggers removed or were merged
  into ci-pipeline.yml.
- Restore per-workflow pull_request triggers for integration, E2E, and
  build workflows.
- Delete ci-pipeline.yml as the required path to retire the consolidated
  pipeline.
- Ensure each workflow is self-contained for image availability.

## 2. Research Findings

### 2.1 Current Consolidated Pipeline

- [.github/workflows/ci-pipeline.yml](.github/workflows/ci-pipeline.yml)
  runs on pull_request and bundles lint, image build, integration tests,
  E2E, coverage, CodeQL, Trivy, supply-chain scans, and gates.
- The pipeline builds and uploads an image artifact for integration and
  uses e2e-tests-split.yml via workflow_call.

### 2.2 Integration Workflows (Current State)

- [.github/workflows/cerberus-integration.yml](.github/workflows/cerberus-integration.yml): workflow_dispatch only.
- [.github/workflows/crowdsec-integration.yml](.github/workflows/crowdsec-integration.yml): workflow_dispatch only.
- [.github/workflows/waf-integration.yml](.github/workflows/waf-integration.yml): workflow_dispatch only.
- [.github/workflows/rate-limit-integration.yml](.github/workflows/rate-limit-integration.yml): workflow_dispatch only.
- Each workflow currently pulls a registry image and tags it as
  charon:local. There is no pull_request trigger and no local build step.

### 2.3 E2E Workflows (Current State)

- [.github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml): workflow_call +
  workflow_dispatch only. No pull_request trigger.
- The build job can build an image locally when invoked directly, but
  the file is currently only invoked by ci-pipeline.yml.

### 2.4 Build Workflow (Current State)

- [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml): workflow_dispatch only.
- This workflow is designed to be the main build pipeline but is not
  currently triggered by pull_request.

### 2.5 Security Workflows (Current State)

- [.github/workflows/security-pr.yml](.github/workflows/security-pr.yml): workflow_dispatch only.
- [.github/workflows/supply-chain-pr.yml](.github/workflows/supply-chain-pr.yml): workflow_dispatch only.
- [.github/workflows/codeql.yml](.github/workflows/codeql.yml): schedule + workflow_dispatch only.
- These workflows include logic for push and pull_request contexts but
  their triggers do not include pull_request.

### 2.6 Historical Reference

- [.github/workflows/e2e-tests.yml.backup](.github/workflows/e2e-tests.yml.backup) and
  [.github/workflows/e2e-tests-split.yml.backup](.github/workflows/e2e-tests-split.yml.backup) show prior pull_request
  trigger patterns and path filters that can be restored.

## 3. Technical Specifications

### 3.1 Workflow Inventory and Trigger Restoration

Target workflows to restore pull_request triggers:

- [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml)
- [.github/workflows/cerberus-integration.yml](.github/workflows/cerberus-integration.yml)
- [.github/workflows/crowdsec-integration.yml](.github/workflows/crowdsec-integration.yml)
- [.github/workflows/waf-integration.yml](.github/workflows/waf-integration.yml)
- [.github/workflows/rate-limit-integration.yml](.github/workflows/rate-limit-integration.yml)
- [.github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml)
- [.github/workflows/security-pr.yml](.github/workflows/security-pr.yml)
- [.github/workflows/supply-chain-pr.yml](.github/workflows/supply-chain-pr.yml)
- [.github/workflows/codeql.yml](.github/workflows/codeql.yml) (decision point)

Notes:

- e2e-tests-split.yml should run directly on pull_request with the
  internal build job enabled, not only via workflow_call.
- security-pr.yml and supply-chain-pr.yml must include pull_request
  triggers so security coverage is not lost.
- codeql.yml needs a decision: re-enable pull_request in codeql.yml or
  leave CodeQL in a separate PR workflow. The consolidated pipeline is
  currently the only PR CodeQL path.

### 3.2 ci-pipeline.yml Decommission Strategy

Decision:

- Option A (required): delete ci-pipeline.yml to fully end the
  consolidated pipeline and avoid duplicate PR checks.

### 3.3 Image Availability Strategy (Critical Challenge)

Independent PR workflows cannot rely on a shared image from another
workflow unless using artifacts or a registry. The user wants to avoid
pipeline complexity.

Required behavior for each integration workflow:

- Restore the "Build Docker image (Local)" step in each integration
  workflow, reverting any artifact handover dependency.
- Build a local Docker image within the workflow before tests run.
- Tag the image as charon:local for consistency with existing scripts.
- Avoid external registry dependency for PR builds.

Impacted workflows:

- cerberus-integration.yml
- crowdsec-integration.yml
- waf-integration.yml
- rate-limit-integration.yml

E2E workflows:

- e2e-tests-split.yml already supports building an image locally when
  invoked directly. Ensure pull_request triggers route through this path
  (not workflow_call).

### 3.4 Pull Request Trigger Scope and Path Filters

- Use branch filters consistent with prior backups and docker-build.yml
  usage: main, development, feature/**, hotfix/**.
- Apply path filters for E2E to avoid unnecessary runs:
  frontend/**, backend/**, tests/**, playwright.config.js,
  .github/workflows/e2e-tests-split.yml.
- Integration workflows typically run on any backend/frontend changes.
  Consider adding path filters if desired, but default to full PR runs
  for parity with previous behavior.

### 3.5 Dependency and Concurrency Rules

- Remove workflow_run coupling to docker-build.yml for integration and
  E2E workflows. Each workflow should be independently triggered by
  pull_request.
- Keep job-level concurrency where it prevents duplicate runs on the
  same PR, but avoid cross-workflow dependencies.

## 4. Implementation Plan

### Phase 1: Baseline Verification (Tests)

- Confirm current CI behavior for PRs: identify which checks are now
  only running via ci-pipeline.yml.
- Capture baseline PR check set from GitHub Actions UI for comparison
  after restoration.

### Phase 2: Restore PR Triggers (Core Workflows)

- Add pull_request triggers to docker-build.yml with branches including
  main and development.
- Add pull_request triggers to cerberus-integration.yml,
  crowdsec-integration.yml, waf-integration.yml, and
  rate-limit-integration.yml.
- Add pull_request triggers to e2e-tests-split.yml, using the backup
  trigger block as the source of truth.

### Phase 3: Make Integration Workflows Self-Contained

- Restore the "Build Docker image (Local)" step in each integration
  workflow and remove dependency on ci-pipeline.yml artifacts.
- Remove registry pull steps or make them optional for manual runs.
- Ensure test scripts continue to reference charon:local.

### Phase 4: Security Workflow Triggers

- Add pull_request triggers to security-pr.yml and supply-chain-pr.yml
  as a mandatory requirement to preserve PR security coverage.
- Decide on CodeQL: either add pull_request to codeql.yml or create a
  dedicated PR CodeQL workflow. If the pipeline is deleted, CodeQL must
  have an alternative PR trigger.

### Phase 5: Decommission ci-pipeline.yml

- Delete ci-pipeline.yml.

### Phase 6: Validation and Audit

- Verify that PRs show the restored individual checks instead of a
  single pipeline job.
- Confirm each integration workflow completes without relying on
  registry or artifact inputs and includes the restored local build step.
- Validate E2E workflow runs directly on pull_request with build job
  executed locally.
- Confirm security workflows run on pull_request.

## 5. Acceptance Criteria (EARS)

- WHEN a pull_request is opened or updated, THE SYSTEM SHALL trigger
  docker-build.yml directly on pull_request for main and development.
- WHEN a pull_request is opened or updated, THE SYSTEM SHALL trigger
  cerberus-integration.yml, crowdsec-integration.yml, waf-integration.yml,
  and rate-limit-integration.yml on pull_request.
- WHEN an integration workflow runs on pull_request, THE SYSTEM SHALL
  restore and run the "Build Docker image (Local)" step, build a local
  Docker image, and tag it as charon:local before tests.
- WHEN a pull_request is opened or updated, THE SYSTEM SHALL trigger
  e2e-tests-split.yml directly on pull_request without relying on
  ci-pipeline.yml.
- WHEN the consolidated pipeline is retired, THE SYSTEM SHALL NOT run
  ci-pipeline.yml on pull_request.
- WHEN a pull_request is opened or updated, THE SYSTEM SHALL run
  security-pr.yml and supply-chain-pr.yml on pull_request.
- WHEN CodeQL is required for pull_request, THE SYSTEM SHALL run a
  CodeQL workflow on pull_request independent of ci-pipeline.yml.

## 6. Risks and Mitigations

- Risk: PR checks increase in parallel count and runtime.
  Mitigation: use path filters for E2E and consider optional filters
  for integration workflows.
- Risk: Image build duplication increases CI cost.
  Mitigation: keep builds scoped to workflows that need the image, and
  avoid registry pushes for PR builds.
- Risk: Security scans or CodeQL no longer run on PR if triggers are
  not restored.
  Mitigation: explicitly re-enable PR triggers in security workflows
  or add a dedicated PR security workflow.

## 7. Confidence Score

Confidence: 82 percent

Rationale: The workflow inventory and trigger gaps are clear. The main
uncertainty is selecting the final CodeQL and security trigger model
once ci-pipeline.yml is removed.
