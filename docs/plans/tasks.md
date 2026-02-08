## Tasks - Frontend Test Iteration

Source: [docs/plans/current_spec.md](docs/plans/current_spec.md)

## Phase 0 - Spec and Task Readiness

- Record traceability to requirements and design artifacts.
- Update docs/plans/requirements.md with EARS items from the plan.
- Update docs/plans/design.md with orchestration flow and runbook notes.
- Update docs/plans/tasks.md with this phased plan and breakdown.
- Verify existing VS Code tasks for Playwright and TypeScript checks.
- Create VS Code tasks if missing:
  - Test: E2E Playwright (Targeted Suite) with a suite path input.
  - Test: Frontend Unit (Vitest).
  - Test: Frontend Coverage (Vitest).
  - Test: E2E Playwright (FireFox) - Core: Navigation Shard.
- Record task labels in the plan and use them for Phases 3-5.

## Phase 1 - Playwright E2E Baseline

- Run Docker: Rebuild E2E Environment only when application or Docker build inputs changed, or when the container is not running or state is suspect.
- Run Test: E2E Playwright (Skill) to capture baseline failures.
- Run Test: E2E Playwright (FireFox) - Core: Navigation Shard to validate the one-off task.
- Record failing suites and map them to fixtures/helpers in tests/fixtures and tests/utils.
- Confirm security teardown expectations for security-related failures.

## Phase 2 - Backend Triage Gate

- If failures show API errors or contract mismatches, open a backend triage path.
- Confirm backend contract stability before modifying frontend tests.

## Phase 3 - PoC/MVP Targeted Reruns

- Select the top 3 failing suites by failure count; tiebreak by distinct failures, then path.
- Run Docker: Rebuild E2E Environment before each Playwright run only when application or Docker build inputs changed, or when the container is not running or state is suspect.
- Use Test: E2E Playwright (Targeted Suite) with a suite path input.
- Rerun each failing suite once; each suite must pass twice (baseline plus one rerun) with no new failures.
- Record baseline and results before proceeding.

## Phase 4 - Frontend Unit Test Convergence

- Run Test: Frontend Unit (Vitest).
- Fix failing tests using createTestQueryClient and mockData as required.
- Validate page wiring and accessible labels for component tests.

## Phase 5 - Coverage and Regression Lock

- Run Test: Frontend Coverage (Vitest) and confirm thresholds.
- If coverage fails, capture patch line ranges from Codecov Patch view before changes.
- Run Docker: Rebuild E2E Environment only when application or Docker build inputs changed, or when the container is not running or state is suspect.
- Re-run Test: E2E Playwright (Skill) to ensure no regressions.
- Run Lint: TypeScript Check.

## Phase 6 - Documentation and Deployment Readiness

- Update runbooks only if required environment variables or steps changed.
- Re-check .gitignore, codecov.yml, .dockerignore, and Dockerfile for new artifacts.
- Confirm requirements, design, and tasks remain current for this plan.
