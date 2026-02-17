# CI Sequencing Audit

Date: 2026-02-08

## Scope

Audit target: .github/workflows/ci-pipeline.yml

Focus areas:
- YAML syntax validity
- Job `if` condition patterns for `e2e`, `coverage-*`, and `security-*`
- Job dependency sequencing (Lint -> Build -> Integration -> Gate -> E2E/Rest)
- Fork behavior (integration skipped, E2E still runs)

## Results

### YAML syntax

- Visual inspection indicates valid YAML structure and indentation.
- No duplicate keys or malformed mappings detected.

### `if` condition pattern review

The following jobs implement `always()` and use a `success || skipped` guard on the integration gate:

- `e2e`: `always()` plus `needs.integration-gate.result == 'success' || ... == 'skipped'`, and `needs.build-image.result == 'success'`.
- `e2e-gate`: `always()` plus `needs.integration-gate.result == 'success' || ... == 'skipped'`.
- `coverage-backend`: `always()` plus `needs.integration-gate.result == 'success' || ... == 'skipped'`.
- `coverage-frontend`: `always()` plus `needs.integration-gate.result == 'success' || ... == 'skipped'`.
- `coverage-gate`: `always()` plus `needs.integration-gate.result == 'success' || ... == 'skipped'`.
- `codecov-upload`: `always()` plus `needs.integration-gate.result == 'success' || ... == 'skipped'`.
- `codecov-gate`: `always()` plus `needs.integration-gate.result == 'success' || ... == 'skipped'` and `needs.codecov-upload.result != 'skipped'`.
- `security-codeql`: `always()` plus `needs.integration-gate.result == 'success' || ... == 'skipped'`.
- `security-trivy`: `always()` plus `needs.integration-gate.result == 'success' || ... == 'skipped'`, and `needs.build-image.result == 'success'`.
- `security-supply-chain`: `always()` plus `needs.integration-gate.result == 'success' || ... == 'skipped'`, and `needs.build-image.result == 'success'`.
- `security-gate`: `always()` plus `needs.integration-gate.result == 'success' || ... == 'skipped'`.

### Sequencing (Lint -> Build -> Integration -> Gate -> E2E/Rest)

- `build-image` depends on `lint`, establishing Lint -> Build.
- Integration jobs depend on `build-image`.
- `integration-gate` depends on `build-image` and all integration jobs.
- `e2e` depends on `build-image` and `integration-gate`.
- Coverage and security jobs depend on `integration-gate` (but not directly on `build-image`).
- `pipeline-gate` depends on all gates.

### Fork logic (Integration Skip -> E2E Run)

- Fork PRs set `push_image=false`, which makes `run_integration=false`.
- Integration jobs and `integration-gate` are skipped.
- `e2e` still runs because it allows `integration-gate` to be `skipped` and only requires `build-image` to succeed.

## Findings

### IMPORTANT: Coverage and security jobs can run after a skipped integration gate caused by failed build

If `lint` or `build-image` fail, `integration-gate` is skipped. The coverage and security jobs only check `(needs.integration-gate.result == 'success' || ... == 'skipped')`, so they can run even when the build failed. This weakens the strict sequence guarantee (Lint -> Build -> Integration -> Gate -> E2E/Rest) for these jobs.

Suggested fix:
- Add `needs.build-image.result == 'success'` to `coverage-*`, `coverage-gate`, `codecov-*`, and `security-codeql` conditions, or require `needs.build-image.result == 'success'` at the `integration-gate` level and check for `success` (not `skipped`) where strict sequencing is required.

## Conclusion

- YAML syntax appears valid on inspection.
- `always() && (success || skipped)` pattern is applied consistently for the targeted jobs.
- Fork logic correctly skips integration and still runs E2E.
- Sequencing is mostly correct, with the exception noted for coverage and security jobs when the integration gate is skipped due to an upstream failure.
