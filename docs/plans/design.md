## Design - Frontend Test Iteration

Source: [docs/plans/current_spec.md](docs/plans/current_spec.md)

## Architecture Overview

This plan standardizes frontend test execution by enforcing a Playwright-first flow and task-driven execution for Vitest unit tests and coverage checks. The plan relies on existing Playwright harness components in tests/ and the Vitest setup in frontend/ to stabilize environment checks, data seeding, and UI interactions.

## Test Orchestration Flow

1. Rebuild the E2E environment using Docker only when application or Docker build inputs changed; otherwise reuse the running container if healthy.
2. Playwright global setup validates environment readiness, performs emergency reset steps, and cleans orphaned data via TestDataManager.
3. Auth setup provisions or authenticates a user and persists storage state to tests/playwright/.auth/user.json.
4. Browser projects inherit storage state and execute UI flows, using ui-helpers and test-steps wrappers for consistent locators and timing.
5. Vitest runs in jsdom with frontend/src/test/setup.ts, and hook/page tests use createTestQueryClient and mockData for deterministic inputs.

## Data Flow and Dependencies

- Playwright setup reads environment variables, checks health endpoints, then writes storage state for all projects.
- UI tests call helpers in tests/utils to create data through /api/v1 endpoints, then validate UI output.
- Vitest setup normalizes browser APIs and translations, then runs hook and component tests with mock data.

## Runbook Notes

- Playwright runs must be preceded by Docker: Rebuild E2E Environment only when application or Docker build inputs changed, or when the container is not running or state is suspect.
- Phases 3-5 must use VS Code task labels defined in the plan, including the targeted suite rerun task.
- The Navigation Shard task runs tests/core/navigation.spec.ts with PLAYWRIGHT_HTML_OPEN=never, PLAYWRIGHT_SKIP_SECURITY_DEPS=1, and --shard=1/1 using the Playwright firefox project.
- Coverage thresholds are configured in frontend/vitest.config.ts using CHARON_MIN_COVERAGE or CPM_MIN_COVERAGE.

## Error Handling and Edge Cases

- Missing or weak CHARON_EMERGENCY_TOKEN triggers a fail-fast stop in tests/global-setup.ts.
- Security modules left enabled can block API calls; use verifySecurityDisabled and security-helpers to restore expected state.
- Config reload overlays can block UI interactions; ui-helpers clickSwitch waits for overlays to clear.
- Cookie domain mismatch between base URL and storageState can cause 401s; tests/auth.setup.ts logs mismatches and requires baseURL alignment.

## Traceability

- Requirements: [docs/plans/requirements.md](docs/plans/requirements.md)
- Tasks: [docs/plans/tasks.md](docs/plans/tasks.md)
