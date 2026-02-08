## Requirements - Frontend Test Iteration

Source: [docs/plans/current_spec.md](docs/plans/current_spec.md)

## EARS Requirements

1. WHEN the frontend test iteration begins, THE SYSTEM SHALL rebuild the E2E environment only when application or Docker build inputs changed, and SHALL skip rebuild for test-only changes if the container is already healthy.
2. WHEN Playwright tests are executed, THE SYSTEM SHALL run the setup project before browser projects and preserve storage state at tests/playwright/.auth/user.json.
3. WHEN Playwright failures occur, THE SYSTEM SHALL capture the failing test file, failing step, and related helper or fixture in tests/utils or tests/fixtures.
4. WHEN Vitest unit tests are executed, THE SYSTEM SHALL apply frontend/src/test/setup.ts and honor coverage thresholds from CHARON_MIN_COVERAGE or CPM_MIN_COVERAGE.
5. WHEN coverage is enforced, THE SYSTEM SHALL meet 100 percent patch coverage and at least the configured frontend minimum coverage threshold.
6. WHEN a failure indicates backend behavior (HTTP 4xx/5xx or missing API contract), THE SYSTEM SHALL open a backend triage path before modifying frontend tests.
7. WHEN Phase 3, 4, or 5 test runs are executed, THE SYSTEM SHALL use the VS Code task labels defined in the plan and avoid ad hoc commands.
8. WHEN the targeted Playwright rerun task is required, THE SYSTEM SHALL create the task in Phase 0 if it does not already exist.
9. WHEN Phase 5 validation runs are executed, THE SYSTEM SHALL run Lint: TypeScript Check and record a zero-error result before completion.
10. WHEN PoC/MVP success criteria are evaluated, THE SYSTEM SHALL require the top 3 failing suites to pass twice (baseline plus one rerun), allow one rerun per suite, and record that no new failures were introduced.
11. WHEN a developer runs the Navigation Shard task, THE SYSTEM SHALL execute only tests/core/navigation.spec.ts using the Playwright Firefox project (firefox).
12. WHEN the Navigation Shard task executes, THE SYSTEM SHALL apply --shard=1/1 to preserve CI-style shard semantics.
13. WHEN the Navigation Shard task runs, THE SYSTEM SHALL keep Cerberus dependencies disabled by setting PLAYWRIGHT_SKIP_SECURITY_DEPS=1.
14. WHEN the Navigation Shard task completes, THE SYSTEM SHALL produce standard Playwright outputs in playwright-report/ and test-results/.
