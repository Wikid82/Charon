---
post_title: "Current Spec: Docker Socket Local-vs-Remote Regression and Traceability"
categories:
  - actions
  - testing
  - docker
  - backend
  - frontend
tags:
  - playwright
  - docker-socket
  - regression
  - traceability
  - coverage
summary: "Execution-ready, strict-scope plan for docker socket local-vs-remote regression tests and traceability, with resolved test strategy, failure simulation, coverage sequencing, and minimal PR slicing."
post_date: 2026-02-24
---

## Active Plan

Date: 2026-02-24
Status: Execution-ready
Scope: Docker socket local-vs-remote regression tests and traceability only

## Introduction

This plan protects the recent Playwright compose change where the docker socket
mount was already added. The objective is to prevent regressions in local Docker
source behavior, guarantee remote Docker no-regression behavior, and provide
clear requirement-to-test traceability.

Out of scope:
- Gotify/notifications changes
- security hardening outside this regression ask
- backend/frontend feature refactors unrelated to docker source regression tests

## Research Findings

Current-state confirmations:
- Playwright compose already includes docker socket mount (user already added it)
  and this plan assumes that current state as baseline.
- Existing Docker source coverage is present but not sufficient to lock failure
  classes and local-vs-remote recovery behavior.

Known test/code areas for this scope:
- E2E: `tests/core/proxy-hosts.spec.ts`
- Frontend tests: `frontend/src/hooks/__tests__/useDocker.test.tsx`
- Frontend form tests: `frontend/src/components/__tests__/ProxyHostForm-dropdown-changes.test.tsx`
- Backend service tests: `backend/internal/services/docker_service_test.go`
- Backend handler tests: `backend/internal/api/handlers/docker_handler_test.go`

Confidence score: 96%

Rationale:
- Required paths already exist.
- Scope is strictly additive/traceability-focused.
- No unresolved architecture choices remain.

## Requirements (EARS)

- WHEN Docker source is `Local (Docker Socket)` and socket access is available,
  THE SYSTEM SHALL list containers successfully through the real request path.
- WHEN local Docker returns permission denied,
  THE SYSTEM SHALL surface a deterministic docker-unavailable error state.
- WHEN local Docker returns missing socket,
  THE SYSTEM SHALL surface a deterministic docker-unavailable error state.
- WHEN local Docker returns daemon unreachable,
  THE SYSTEM SHALL surface a deterministic docker-unavailable error state.
- WHEN local Docker fails and user switches to remote Docker source,
  THE SYSTEM SHALL allow recovery and load remote containers without reload.
- WHEN remote Docker path is valid,
  THE SYSTEM SHALL continue to work regardless of local failure-class tests.

## Resolved Decisions

1. Test-file strategy: keep all new E2E cases in existing
   `tests/core/proxy-hosts.spec.ts` under one focused Docker regression describe block.
2. Failure simulation strategy: use deterministic interception/mocking for failure
   classes (`permission denied`, `missing socket`, `daemon unreachable`), and use
   one non-intercepted real-path local-success test.
3. Codecov timing: update `codecov.yml` only in PR-2 and only if needed after
   PR-1 test signal review; no unrelated coverage policy churn.

## Explicit Test Strategy

### E2E (Playwright)

1. Real-path local-success test (no interception):
   - Validate local Docker source works when socket is accessible in current
     Playwright compose baseline.
2. Deterministic failure-class tests (interception/mocking):
   - local permission denied
   - local missing socket
   - local daemon unreachable
3. Remote no-regression test:
   - Validate remote Docker path still lists containers and remains unaffected by
     local failure-class scenarios.
4. Local-fail-to-remote-recover test:
   - Validate source switch recovery without page reload.

### Unit tests

- Frontend: hook/form coverage for error surfacing and recovery UX.
- Backend: connectivity classification and handler status mapping for the three
  failure classes plus remote success control case.

## Concrete DoD Order (Testing Protocol Aligned)

1. Run E2E first (mandatory): execute Docker regression scenarios above.
2. Generate local patch report artifacts (mandatory):
   - `test-results/local-patch-report.md`
   - `test-results/local-patch-report.json`
3. Run unit tests and enforce coverage thresholds:
   - backend unit tests with repository minimum coverage threshold
   - frontend unit tests with repository minimum coverage threshold
4. If patch coverage gaps remain for changed lines, add targeted tests until
   regression lines are covered with clear rationale.

## Traceability Matrix

| Requirement | Test name | File | PR slice |
|---|---|---|---|
| Local works with accessible socket | `Docker Source - local socket accessible loads containers (real path)` | `tests/core/proxy-hosts.spec.ts` | PR-1 |
| Local permission denied surfaces deterministic error | `Docker Source - local permission denied shows docker unavailable` | `tests/core/proxy-hosts.spec.ts` | PR-1 |
| Local missing socket surfaces deterministic error | `Docker Source - local missing socket shows docker unavailable` | `tests/core/proxy-hosts.spec.ts` | PR-1 |
| Local daemon unreachable surfaces deterministic error | `Docker Source - local daemon unreachable shows docker unavailable` | `tests/core/proxy-hosts.spec.ts` | PR-1 |
| Remote path remains healthy | `Docker Source - remote server path no regression` | `tests/core/proxy-hosts.spec.ts` | PR-1 |
| Recovery from local failure to remote success | `Docker Source - switch local failure to remote success recovers` | `tests/core/proxy-hosts.spec.ts` | PR-1 |
| Frontend maps failure details correctly | `useDocker - maps docker unavailable details by failure class` | `frontend/src/hooks/__tests__/useDocker.test.tsx` | PR-1 |
| Form keeps UX recoverable after local failure | `ProxyHostForm - allows remote switch after local docker error` | `frontend/src/components/__tests__/ProxyHostForm-dropdown-changes.test.tsx` | PR-1 |
| Backend classifies failure classes | `TestIsDockerConnectivityError_*` | `backend/internal/services/docker_service_test.go` | PR-1 |
| Handler maps unavailable classes and preserves remote success | `TestDockerHandler_ListContainers_*` | `backend/internal/api/handlers/docker_handler_test.go` | PR-1 |
| Coverage traceability policy alignment (if needed) | `Codecov ignore policy update review` | `codecov.yml` | PR-2 |

## Implementation Plan

### Phase 1: Regression tests

- Add E2E Docker regression block in `tests/core/proxy-hosts.spec.ts` with one
  real-path success, three deterministic failure-class tests, one remote
  no-regression test, and one recovery test.
- Extend frontend and backend unit tests for the same failure taxonomy and
  recovery behavior.

Exit criteria:
- All required tests exist and pass.
- Failure classes are deterministic and non-flaky.

### Phase 2: Traceability and coverage policy (conditional)

- Review whether current `codecov.yml` ignore entries reduce traceability for
  docker regression files.
- If needed, apply minimal `codecov.yml` update only for docker-related ignores.

Exit criteria:
- Traceability from requirement to coverage/reporting is clear.
- No unrelated codecov policy changes.

## PR Slicing Strategy

Decision: two minimal PRs.

### PR-1: regression tests + compose profile baseline

Scope:
- docker socket local-vs-remote regression tests (E2E + targeted unit tests)
- preserve and validate current Playwright compose socket-mount baseline

Validation gates:
- E2E first pass for regression matrix
- local patch report artifacts generated
- unit tests and coverage thresholds pass

Rollback contingency:
- revert only newly added regression tests if instability appears

### PR-2: traceability/coverage policy update (if needed)

Scope:
- minimal `codecov.yml` adjustment strictly tied to docker regression
  traceability

Validation gates:
- coverage reporting reflects changed docker regression surfaces
- no unrelated policy drift

Rollback contingency:
- revert only `codecov.yml` delta

## Acceptance Criteria

- Exactly one coherent plan exists in this file with one frontmatter block.
- Scope remains strictly docker socket local-vs-remote regression tests and
  traceability only.
- All key decisions are resolved directly in the plan.
- Current-state assumption is consistent: socket mount already added in
  Playwright compose baseline.
- Test strategy explicitly includes:
  - one non-intercepted real-path local-success test
  - deterministic intercepted/mocked failure-class tests
  - remote no-regression test
- DoD order is concrete and protocol-aligned:
  - E2E first
  - local patch report artifacts
  - unit tests and coverage thresholds
- Traceability matrix maps requirement -> test name -> file -> PR slice.
- PR slicing is minimal and non-contradictory:
  - PR-1 regression tests + compose profile baseline
  - PR-2 traceability/coverage policy update if needed

## Handoff

This plan is clean, internally consistent, and execution-ready for Supervisor
review and delegation.
