# CI Pipeline Optimization Plan

## 1. Introduction

**Overview:**
This plan optimizes the CI pipeline dependency graph so the `e2e` job starts as early as possible, while preserving quality gates. The primary change is to decouple `lint` from `build-image`, allowing both to run in parallel after `setup` completes.

**Objectives:**
- Start `e2e` as soon as `build-image` finishes.
- Keep `lint` as a required gate via `pipeline-gate`.
- Preserve existing security scan behavior, especially early/parallel execution of `security-codeql`.

## 2. Research Findings

**Existing workflow file:**
- [ci-pipeline.yml](.github/workflows/ci-pipeline.yml)

**Current dependency graph (relevant):**
- `setup` has no needs (fast input normalization).
- `lint` has no needs.
- `build-image` needs `lint` and `setup`.
- `e2e` needs `build-image`.
- `pipeline-gate` needs `lint`, `build-image`, `integration-gate`, `e2e-gate`, `coverage-gate`, `codecov-gate`, `security-gate`.
- `security-codeql` has no needs and runs early/parallel.

**Observation:**
- `build-image` is unnecessarily serialized behind `lint`, delaying downstream jobs (`e2e`, integrations, security image scans).
- `security-codeql` already runs independently and should remain so.

## 3. Technical Specifications

### 3.1 Dependency Graph Changes

**Target behavior:**
- `lint` runs in parallel with `setup` and `build-image`.
- `build-image` depends only on `setup`.
- `e2e` continues to depend on `build-image`.
- `pipeline-gate` continues to enforce `lint` success.
- `security-codeql` remains without `needs`.

**Proposed change:**
- Update `build-image.needs` to only include `setup`.

### 3.2 EARS Requirements

- WHEN the CI pipeline runs, THE SYSTEM SHALL start `build-image` after `setup` completes, without waiting for `lint`.
- WHEN `build-image` completes successfully, THE SYSTEM SHALL start `e2e` as soon as it is scheduled.
- WHEN `lint` fails, THE SYSTEM SHALL block the pipeline via `pipeline-gate` even if `e2e` or `build-image` succeed.
- WHEN security scans are enabled, THE SYSTEM SHALL run `security-codeql` in parallel with other jobs without dependency on `setup`, `lint`, or `build-image`.

### 3.3 Error Handling and Edge Cases

- If `setup` fails, `build-image` and its dependents must not run (existing behavior preserved).
- If `lint` fails but `build-image` and `e2e` succeed, `pipeline-gate` must still fail.
- If `security-codeql` is skipped (e.g., forked PR), `security-gate` must continue to interpret skip correctly (no change).

### 3.4 Risks and Mitigations

| Risk | Impact | Mitigation |
| --- | --- | --- |
| `build-image` could start before `lint` detects issues | Failing lint might occur after expensive build/test work | `pipeline-gate` still enforces `lint` success; cost is acceptable for speed |
| Misconfigured `needs` graph causes unintended skips | Downstream jobs might not run | Only remove `lint` from `build-image.needs`; do not change other gates |

## 4. Implementation Plan

### Phase 1: Playwright Tests (Behavioral Expectations)
- No Playwright changes are required for this CI optimization. Confirm `e2e` workflow reuse remains unchanged.

### Phase 2: Backend Implementation
- Not applicable.

### Phase 3: Frontend Implementation
- Not applicable.

### Phase 4: Integration and Testing
- Validate the dependency graph in `ci-pipeline.yml` locally by reasoning and optional dry-run (no CI execution in this plan).
- Confirm `security-codeql` retains no `needs`.

### Phase 5: Documentation and Deployment
- Update this plan only (no documentation changes elsewhere).

## 5. Acceptance Criteria

- DoD: CI dependency graph reflects `build-image` depending only on `setup`.
- DoD: `lint` remains a required gate in `pipeline-gate`.
- DoD: `security-codeql` continues to run early/parallel (no `needs`).
- DoD: `e2e` still depends on `build-image` only.

## 6. Complexity and Impact

- **Complexity:** Low
- **Impact:** Moderate CI speed-up for E2E and integration jobs
