---
title: "CI Sequencing and Parallelization"
status: "draft"
scope: "ci/pipeline, ci/integration, ci/coverage, ci/security"
---

## 1. Introduction

This plan reorders CI job dependencies so lint remains first, integration runs after image build, E2E depends on integration-gate, and coverage/security jobs run in parallel with E2E once integration has passed.

Objectives:

- Keep `lint` as the earliest gate before `build-image`.
- Ensure integration tests run after `build-image` and before `e2e`.
- Make `e2e` depend on `integration-gate`.
- Start coverage and security jobs after `integration-gate` so they complete while E2E runs.
- Preserve strict gating: if a required stage is skipped or fails, downstream gates fail.

## 2. Research Findings

### 2.1 Current CI Ordering

- `lint` runs first and gates `build-image`.
- Integration jobs depend on `build-image`, then `integration-gate` aggregates results.
- `e2e` depends only on `build-image`.
- Coverage and security jobs depend on `build-image` (CodeQL has no dependencies).
- `codecov-upload` depends on `coverage-gate` and `e2e`.
- `pipeline-gate` evaluates gate jobs based on input flags, not on integration status.

### 2.2 Impact of Current Ordering

- E2E can run without integration tests completing.
- Coverage and security jobs start before or during integration, competing for runners.
- `security-codeql` can start immediately, cluttering early logs.

## 3. Technical Specifications

### 3.1 Target Dependency Graph

```
setup -> lint -> build-image
build-image -> integration-* -> integration-gate
integration-gate -> e2e -> e2e-gate
integration-gate -> coverage-* -> coverage-gate
integration-gate -> security-* -> security-gate
coverage-gate + e2e -> codecov-upload -> codecov-gate
pipeline-gate depends on lint, build-image, integration-gate, e2e-gate, coverage-gate, codecov-gate, security-gate
```

### 3.2 Job Dependency Updates

Update `.github/workflows/ci-pipeline.yml`:

- `e2e`:
  - `needs`: `build-image`, `integration-gate`.
  - `if`: require `needs.integration-gate.result == 'success'` and `needs.build-image.result == 'success'` and the existing `run_e2e` input guard.
  - Keep `build-image` in `needs` for `image_ref` outputs.

- `e2e-gate`:
  - Keep `needs: e2e`.
  - Update `if` guard to require integration to have run (see Section 3.4) so skips are treated consistently.

- `coverage-backend` and `coverage-frontend`:
  - `needs`: `integration-gate`.
  - `if`: existing `run_coverage` input guard AND `needs.integration-gate.result == 'success'`.
  - No `build-image` dependency is required for coverage tests.

- `security-codeql`:
  - Add `needs: integration-gate`.
  - `if`: existing `run_security_scans` guard AND `needs.integration-gate.result == 'success'` AND the existing fork guard.

- `security-trivy` and `security-supply-chain`:
  - `needs`: `build-image`, `integration-gate`.
  - `if`: existing guards AND `needs.integration-gate.result == 'success'`.

- `security-gate`:
  - Keep `needs` on all security jobs.
  - `if`: same as today, plus integration-enabled guard (Section 3.4).

### 3.3 Parallelization Strategy

- Once `integration-gate` succeeds, start `e2e`, `coverage-*`, and `security-*` concurrently.
- This ensures non-E2E work completes during the longer E2E runtime window.

### 3.4 Enablement and Strict Gating Logic

Adjust enablement expressions so skip behavior is intentional and strict:

- Define an integration-enabled expression for reuse:
  - `integration_enabled = needs.build-image.outputs.run_integration == 'true'`
- Define an integration-gate pass-or-skip expression for reuse:
  - `integration_gate_ok = needs.integration-gate.result == 'success' || needs.integration-gate.result == 'skipped'`

Update gate conditions to include `integration_enabled`:

- `e2e` and `e2e-gate`:
  - Only enabled if `run_e2e` is not false.
  - Add guard: `always() && integration_gate_ok` so fork PRs (integration skipped) still run.

- `coverage-*`, `coverage-gate`, `codecov-upload`, `codecov-gate`:
  - Only enabled if `run_coverage` is not false.
  - Add guard: `always() && integration_gate_ok` so fork PRs (integration skipped) still run.

- `security-*`, `security-gate`:
  - Only enabled if `run_security_scans` is not false (and fork guard for CodeQL/Trivy/SBOM).
  - Add guard: `always() && integration_gate_ok` so fork PRs (integration skipped) still run.

- `pipeline-gate`:
  - Update enabled checks (`e2e_enabled`, `coverage_enabled`, `security_enabled`) to use `integration_gate_ok` (not `integration_enabled`).
  - Keep strict gating: when enabled, skipped results remain failures.

### 3.5 Artifact and Output Dependencies

- `codecov-upload` continues to depend on `e2e` for E2E coverage artifacts and on `coverage-gate` for unit coverage.
- `security-trivy` and `security-supply-chain` continue using `needs.build-image.outputs.image_ref_dockerhub`.

### 3.6 Data Flow and Runners

- Integration is isolated to its phase, reducing early runner contention.
- The post-integration phase allows `e2e`, `coverage-*`, and `security-*` to run in parallel.

## 4. Implementation Plan

### Phase 1: Playwright Tests (Behavior Baseline)

- No UI behavior changes are expected; treat as baseline verification only.

### Phase 2: Dependency Rewire

- Update `e2e` to require `integration-gate` and `build-image`.
- Add `integration-gate` to `coverage-*` and `security-*` `needs`.
- Move `security-codeql` behind `integration-gate`.

### Phase 3: Strict Gating Alignment

- Update job `if` conditions to include `integration_enabled`.
- Update `pipeline-gate` enablement logic to match new gating rules.

### Phase 4: Validation

- Verify that `lint -> build-image -> integration -> e2e` is enforced.
- Confirm coverage and security jobs start only after integration succeeds.
- Confirm `codecov-upload` and `codecov-gate` still run after coverage and E2E are complete.

## 5. Acceptance Criteria (EARS)

- WHEN a pipeline starts, THE SYSTEM SHALL run `lint` before `build-image`.
- WHEN `build-image` succeeds and integration is enabled, THE SYSTEM SHALL run all integration tests and aggregate them in `integration-gate`.
- WHEN `integration-gate` succeeds or is skipped and E2E is enabled, THE SYSTEM SHALL start `e2e` and require it to pass before `e2e-gate` succeeds.
- WHEN `integration-gate` succeeds or is skipped and coverage is enabled, THE SYSTEM SHALL start `coverage-backend` and `coverage-frontend` in parallel with E2E.
- WHEN `integration-gate` succeeds or is skipped and security scans are enabled, THE SYSTEM SHALL start `security-codeql`, `security-trivy`, and `security-supply-chain` in parallel with E2E.
- WHEN integration is skipped for fork PRs (`run_integration=false`), THE SYSTEM SHALL still run `e2e`, `coverage-*`, and `security-*` if their respective enablement flags are true.
- IF `integration-gate` is not successful while integration is enabled, THEN THE SYSTEM SHALL skip `e2e`, `coverage-*`, and `security-*` and fail the appropriate gates.
- WHEN coverage and E2E complete successfully, THE SYSTEM SHALL run `codecov-upload` and `codecov-gate`.

## 6. Risks and Mitigations

- Risk: Coupling coverage/security to integration could reduce flexibility for ad-hoc runs.
  Mitigation: Keep `run_integration` default true; document that disabling integration disables downstream stages.

- Risk: CodeQL no longer starts early, increasing total elapsed time for that job.
  Mitigation: CodeQL runs in parallel with E2E to keep total pipeline time stable.

- Risk: Misaligned gate logic could mark expected skips as failures.
  Mitigation: Centralize enablement logic (`integration_enabled`) and apply consistently in job `if` conditions and `pipeline-gate`.

## 7. Confidence Score

Confidence: 88 percent

Rationale: The sequencing changes are localized to job `needs` and `if` expressions. The main uncertainty is ensuring gate logic stays strict while respecting the new integration-first requirement.
