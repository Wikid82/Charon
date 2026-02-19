# CI Pipeline Integration and Gate Enforcement Fix Plan

## Introduction

This plan addresses two pipeline defects in [ .github/workflows/ci-pipeline.yml ](.github/workflows/ci-pipeline.yml):

- Integration jobs are skipped even when the image build/push is successful.
- Gate jobs report success even when upstream jobs are skipped.

The goal is to make the execution order deterministic and strict: Setup -> Build/Push -> Integration -> Integration Gate -> E2E -> E2E Gate, with gates failing if any required dependency is not successful.

## Research Findings

### Integration jobs are conditionally skipped

The integration jobs (`integration-cerberus`, `integration-crowdsec`, `integration-waf`, `integration-ratelimit`) are gated by the same `if:` expression in [ .github/workflows/ci-pipeline.yml ](.github/workflows/ci-pipeline.yml). That expression requires:

- `needs.build.result == 'success'`
- `needs.build.outputs.image_ref != ''`
- the workflow not being explicitly disabled via `workflow_dispatch` input

This creates two likely skip paths:

1. **Image reference availability is tied to Docker Hub only.** If the build job does not push or resolve a Docker Hub reference, integration jobs skip even if an image exists elsewhere (e.g., GHCR).
2. **Push policy is not part of the integration condition.** The build job exposes `image_pushed`, but integration jobs do not check it. This prevents a predictable decision about whether an image is actually available in a registry the jobs can pull from.

### Gate jobs accept skipped dependencies

The gate jobs (`integration-gate`, `coverage-gate`, `codecov-gate`, `pipeline-gate`) use `if: always()` and only fail on `failure` or `cancelled`. They do not fail on `skipped`, which allows skipped dependencies to be treated as a success.

Examples in [ .github/workflows/ci-pipeline.yml ](.github/workflows/ci-pipeline.yml):

- `integration-gate` exits 0 when integration is skipped due to build state or `run_integration` being false.
- `coverage-gate` and `pipeline-gate` do not enforce a strict success-only check across dependencies.

### Reusable E2E workflow masks skipped jobs

The reusable workflow [ .github/workflows/e2e-tests-split.yml ](.github/workflows/e2e-tests-split.yml) includes a final job that explicitly converts `skipped` to `success`. That behavior is useful for partial `workflow_dispatch` runs, but in CI (where `browser=all` and `test_category=all`) it allows a silent skip to pass.

## Technical Specifications

### Requirements (EARS Notation)

- WHEN the build-and-push stage completes and produces a successful push, THE SYSTEM SHALL start all integration jobs.
- WHEN integration is required, THE SYSTEM SHALL fail the integration gate if any integration job result is not `success`.
- WHEN E2E tests are required, THE SYSTEM SHALL fail the E2E gate if the reusable workflow result is not `success`.
- WHEN coverage jobs are required, THE SYSTEM SHALL fail the coverage gate if any coverage or E2E dependency is not `success`.
- WHEN any required gate fails, THE SYSTEM SHALL fail the pipeline gate.
- WHEN a stage is enabled, THE SYSTEM SHALL treat any `skipped` or `missing` dependency as a gate failure.
- IF a stage is explicitly disabled via `workflow_dispatch` or `workflow_call` input, THEN THE SYSTEM SHALL skip the stage and its gate by using the same stage-enabled condition on the gate job.

### Integration job eligibility and image selection

Define a single computed boolean output that decides whether integration should run. This avoids duplicating conditions across jobs, aligns with the image availability policy, and normalizes input booleans across `workflow_dispatch` and `workflow_call`.

Definitive architecture:

- **Job `setup`** outputs `input_run_integration` (user intent only).
- **Job `build-and-push`** computes final `run_integration`.
- **Computed logic:** `run_integration = (needs.setup.outputs.input_run_integration == 'true') && (steps.push.outcome == 'success')`.
- **Dependent jobs (integration + gate)** use the exact same `if` expression: `${{ needs.build-and-push.outputs.run_integration == 'true' }}`.
- **Gate logic** fails if any `needs` is not `success`.

- `run_integration=true` if and only if:
  - `needs.setup.outputs.input_run_integration` is true, and
  - the push step in `build-and-push` succeeds.
- Integration tests run in a separate job and require the image to be available in a registry. A `pull_request` event alone does not permit integration to run without a pushed image.

Recommended outputs:

- `setup.outputs.input_run_integration`: normalized input boolean derived from `workflow_dispatch` or `workflow_call`
- `build-and-push.outputs.image_ref`: resolved image reference with fallback to GHCR
- `build-and-push.outputs.image_registry`: `dockerhub` or `ghcr`
- `build-and-push.outputs.image_pushed`: `true` only when a registry push occurred
- `build-and-push.outputs.run_integration`: computed eligibility boolean

Integration jobs should use the same `if:` expression based on `needs.build-and-push.outputs.run_integration` and should pull from the resolved `image_ref`.

### Gate enforcement pattern (fail on skipped or failed)

Use a strict pattern that fails on anything other than `success` when a stage is required. This should be reusable across integration, coverage, E2E, and pipeline gates. Gate jobs MUST use the same stage-enabled `if` as the jobs in the stage.

For integration, the gate job `if` condition must be `${{ needs.build-and-push.outputs.run_integration == 'true' }}`.

Gate logic details (explicit YAML/script pattern):

1. Gate job uses the same stage-enabled `if` as the jobs in the stage.
2. Gate job uses a single verification step that inspects `needs` via JSON and fails if any required job is not `success` (including `skipped` or `missing`).
3. Gate job is skipped when the stage is intentionally disabled, since the job-level `if` matches the stage condition.

Reusable pattern (standard block or composite action):

- Inputs:
  - `required_jobs`: JSON array of job ids in scope for that gate.
- Logic:
  - Iterate `required_jobs` and fail on any result not equal to `success`.

Canonical gate step example (for plan reference):

```yaml
steps:
  - name: Evaluate gate
    env:
      NEEDS_JSON: ${{ toJSON(needs) }}
      REQUIRED_JOBS: ${{ inputs.required_jobs }}
    run: |
      set -euo pipefail
      for job in $(echo "$REQUIRED_JOBS" | jq -r '.[]'); do
        result=$(echo "$NEEDS_JSON" | jq -r --arg job "$job" '.[$job].result // "missing"')
        if [[ "$result" != "success" ]]; then
          echo "::error::Gate failed: $job result is $result"
          exit 1
        fi
      done
```

Example `stage_enabled` signals by gate:

- Integration gate: `needs.build-and-push.outputs.run_integration == 'true'`
- E2E gate: `inputs.run_e2e == 'true'` (or the equivalent workflow input)
- Coverage gate: `inputs.run_coverage == 'true'`
- Pipeline gate: always true, but only depends on gates and required security jobs

### E2E strictness

In [ .github/workflows/e2e-tests-split.yml ](.github/workflows/e2e-tests-split.yml), the final `e2e-results` job should only convert `skipped` to `success` when the skip is intentional (for example, the workflow is manually dispatched with `browser` or `test_category` not including that job). For CI runs with `browser=all` and `test_category=all`, any skipped job should be treated as a failure.

### Integration run logic (must match actual build/push)

Integration jobs must depend on the *actual execution* of the build/push step and the explicit input toggle. Use a single source of truth from `setup` and `build-and-push` outputs:

- `setup.outputs.input_run_integration`: normalized input boolean derived from `workflow_dispatch` or `workflow_call`
- `build-and-push.outputs.image_ref`: resolved registry reference from the same push
- `build-and-push.outputs.image_pushed`: `true` only when a registry push occurred
- `build-and-push.outputs.run_integration`: computed boolean that validates input enablement and push availability

Integration job `if:` should be:

```yaml
if: ${{ needs.build-and-push.outputs.run_integration == 'true' }}
```

`run_integration` must be computed using the strict integration requirement:

```yaml
run_integration: ${{ (needs.setup.outputs.input_run_integration == 'true') && (steps.push.outcome == 'success') }}
```

### Boolean/type safety

- Normalize `workflow_dispatch` string inputs using `fromJSON` before comparison.
- Preserve `workflow_call` boolean inputs as-is, and pass them through `inputs.*` without string comparisons.
- Use a setup step to emit normalized boolean outputs (for example, `inputs.run_integration`) so job conditions stay consistent and avoid mixed string/boolean logic.

### Fail-fast strategy (efficiency)

Document and enforce a fail-fast strategy to reduce wasted runtime:

- For matrix jobs (E2E, coverage, or any parallel test suites), set `strategy.fail-fast: true` for CI runs so other matrix jobs stop when one fails.
- Downstream stages must `need` their gate job to prevent unnecessary execution after a failure.
- Use workflow `concurrency` with `cancel-in-progress: true` for CI workflows targeting the same branch to avoid redundant runs.

### Sequence enforcement

Ensure the dependency chain is explicit and strict:

1. `setup`
2. `build-and-push`
3. integration jobs
4. `integration-gate`
5. `e2e` (reusable workflow)
6. `e2e-gate` (new)
7. coverage jobs
8. `coverage-gate`
9. `codecov-gate`
10. security jobs
11. `pipeline-gate`

## Implementation Plan

### Phase 1: CI Workflow Validation Plan

- Add or update workflow validation checks to detect skipped jobs in CI mode.
- Update `e2e-tests-split.yml` so the final `e2e-results` job fails if any job is skipped when `inputs.browser=all` and `inputs.test_category=all`.

### Phase 2: Integration Stage Fix

- Add `input_run_integration` output in `setup`.
- Add a computed `run_integration` output in `build-and-push` using the push step outcome.
- Add a resolved `image_ref` output that can use GHCR as a fallback if Docker Hub is unavailable.
- Update all integration jobs to use the computed `run_integration` output and the resolved `image_ref`.

### Phase 3: Gate Standardization

- Add a new `e2e-gate` job that fails if `needs.e2e.result` is not `success` when E2E is required.
- Implement a reusable gate-check block or composite action that accepts `required_jobs` and `stage_enabled` inputs.
- Update `integration-gate`, `coverage-gate`, `codecov-gate`, and `pipeline-gate` to enforce a strict success-only check for required dependencies.

### Phase 4: Sequence and Dependency Updates

- Wire dependencies so `coverage-backend` and `coverage-frontend` depend on `e2e-gate` rather than `integration-gate` directly.
- Ensure `pipeline-gate` depends on all gates and required security jobs.

### Phase 5: Documentation and Verification

- Update this plan with any final implementation decisions once validated.
- Document the new gating behavior in relevant CI documentation if present.

## Acceptance Criteria

- Integration jobs run whenever `input_run_integration` is true and the build/push step succeeds.
- Integration gate fails if any integration job is `skipped`, `failure`, or `cancelled` while integration is required.
- E2E gate fails if the reusable E2E workflow result is not `success` while E2E is required.
- Coverage gate fails if any coverage or E2E dependency is not `success` while coverage is required.
- Pipeline gate fails if any required gate or security job is not `success`.
- The execution order is enforced as: Build -> Integration -> Integration Gate -> E2E -> E2E Gate -> Coverage -> Coverage Gate -> Codecov Gate -> Security -> Pipeline Gate.
- Fail-fast behavior is documented and applied for matrix jobs in CI runs.
