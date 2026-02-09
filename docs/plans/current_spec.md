---
title: "Fix Noisy Frontend Tests and Clean Coverage Output"
status: "planning"
scope: "frontend/tests"
notes: Address console noise in AuditLogs tests, act() warnings in UsersPage tests, and confirm coverage outputs stay clean without introducing new artifacts.
---

## 1. Introduction

This plan targets two noisy frontend test suites and their downstream coverage output. The immediate goals are to:

- Silence expected console errors in AuditLogs tests without hiding real failures.
- Eliminate act() warnings in UsersPage tests by synchronizing timer-driven updates.
- Keep coverage output clean and stable across local runs and CI.

## 2. Research Findings

### 2.1 AuditLogs test noise

The AuditLogs page logs export failures with `console.error` in the export handler, which is triggered in the export error test. This produces expected console output in [frontend/src/pages/AuditLogs.tsx](frontend/src/pages/AuditLogs.tsx) and [frontend/src/pages/__tests__/AuditLogs.test.tsx](frontend/src/pages/__tests__/AuditLogs.test.tsx).

### 2.2 UsersPage act() warnings

UsersPage uses a debounced effect for invite URL preview (500ms timeout). Tests in [frontend/src/pages/__tests__/UsersPage.test.tsx](frontend/src/pages/__tests__/UsersPage.test.tsx) trigger this effect but do not consistently advance timers or await the debounce path, which can surface act() warnings in the Vitest environment configured in [frontend/vitest.config.ts](frontend/vitest.config.ts) and [frontend/src/test/setup.ts](frontend/src/test/setup.ts).

### 2.3 Test utilities and configuration

- Test setup already sets `globalThis.IS_REACT_ACT_ENVIRONMENT = true` and filters specific console errors in [frontend/src/test/setup.ts](frontend/src/test/setup.ts).
- Query client test helper is centralized in [frontend/src/test-utils/renderWithQueryClient.tsx](frontend/src/test-utils/renderWithQueryClient.tsx).
- Coverage reporting is configured in [frontend/vitest.config.ts](frontend/vitest.config.ts) with output under `frontend/coverage/` and global ignore rules in [codecov.yml](codecov.yml).

### 2.4 Ignore and packaging rules review

- `.gitignore` already ignores frontend coverage, test results, and Playwright artifacts; no new artifact paths are currently introduced by the target tests. See [.gitignore](.gitignore).
- `.dockerignore` excludes frontend test and coverage artifacts, and the entire tests directory, so no changes are needed for this task. See [.dockerignore](.dockerignore).
- `codecov.yml` already ignores frontend test utilities and setup files, and enforces 100% patch coverage. See [codecov.yml](codecov.yml).
- `Dockerfile` is unrelated to test runtime and coverage artifacts for this scope. See [Dockerfile](Dockerfile).

## 3. Technical Specifications

### 3.1 AuditLogs console noise control

Objective: Prevent expected export errors from leaking into console output while preserving real console errors.

Plan:

- In [frontend/src/pages/__tests__/AuditLogs.test.tsx](frontend/src/pages/__tests__/AuditLogs.test.tsx), use `vi.spyOn(console, 'error').mockImplementation(() => {})` inside the specific export error test to prevent noise.
- Assert with resilient matchers (e.g., `expect(consoleErrorSpy).toHaveBeenCalled()` and `toHaveBeenCalledWith(expect.stringContaining(...), expect.anything())` as appropriate to the test).
- Always restore the spy in the same test scope (e.g., `consoleErrorSpy.mockRestore()` in `finally` or after assertions) so other console errors remain visible.

### 3.2 UsersPage act() warning elimination

Objective: Make timer-driven state updates explicit in tests that trigger the invite URL preview debounce.

Plan:

- In [frontend/src/pages/__tests__/UsersPage.test.tsx](frontend/src/pages/__tests__/UsersPage.test.tsx), call `vi.useFakeTimers()` before `userEvent.setup({ advanceTimers: vi.advanceTimersByTime })` in tests that cover the invite URL preview debounce.
- Advance timers by 500ms inside `act(() => { vi.advanceTimersByTime(500); })`, then await `waitFor` assertions.
- Ensure each test restores real timers to avoid cross-test contamination.

### 3.3 Coverage output cleanliness

Objective: Keep coverage output and Codecov patch status clean after test stabilization.

Plan:

- Confirm coverage output remains under `frontend/coverage/` with no new artifacts.
- Ensure no new files or paths are added that require ignore updates in [.gitignore](.gitignore) or [codecov.yml](codecov.yml).
- If the test updates add new helper files, ensure they are in existing ignored paths (e.g., `frontend/src/test-utils/**`), otherwise update ignore lists.

## 4. Implementation Plan

### Phase 1: Playwright Tests (Behavior Definition)

- No Playwright changes required; the issues are limited to unit tests and their console/timer behavior.
- Confirm no E2E coverage changes are needed for this task.

### Phase 2: Backend Implementation

- Not applicable.

### Phase 3: Frontend Implementation

1. Update AuditLogs test to spy on and assert `console.error` for the export error path.
2. Update UsersPage invite URL preview tests to use fake timers and act-wrapped timer advancement.
3. Ensure any test-specific helpers remain in existing test utility paths.

### Phase 4: Integration and Testing

1. Run Playwright E2E tests first (before unit tests), per the mandatory E2E-first policy.
2. Run targeted Vitest suites for:
   - [frontend/src/pages/__tests__/AuditLogs.test.tsx](frontend/src/pages/__tests__/AuditLogs.test.tsx)
   - [frontend/src/pages/__tests__/UsersPage.test.tsx](frontend/src/pages/__tests__/UsersPage.test.tsx)
3. Run frontend coverage locally using the existing script in `scripts/frontend-test-coverage.sh` (no changes planned).
4. Verify coverage artifacts remain in ignored paths and Codecov patch coverage stays at 100%.

### Phase 5: Documentation and Deployment

- Update this plan to reflect any deviations discovered during implementation.
- No deployment changes required.

## 5. Acceptance Criteria (EARS)

- WHEN AuditLogs export fails during tests, THE SYSTEM SHALL capture the error via a test-local `console.error` spy and SHALL NOT emit unhandled console output.
- WHEN UsersPage invite preview debounce logic runs in tests, THE SYSTEM SHALL advance timers within `act()` and SHALL NOT emit act() warnings.
- WHEN frontend coverage is generated, THE SYSTEM SHALL write outputs only under existing ignored paths and SHALL NOT introduce new unignored artifacts.
- WHEN Codecov evaluates patch coverage, THE SYSTEM SHALL report 100% coverage for modified lines.

## 6. Risks and Mitigations

- Risk: Over-suppression of console errors hides real regressions. Mitigation: limit console spying to the single export error test and assert expected payloads.
- Risk: Fake timers leak across tests. Mitigation: ensure timers are restored in `afterEach` or per-test cleanup in UsersPage tests.
- Risk: Timer advancement out of sync with user-event. Mitigation: use `userEvent.setup({ advanceTimers: vi.advanceTimersByTime })` while fake timers are enabled.

## 7. Dependencies and Impacted Files

- Tests: [frontend/src/pages/__tests__/AuditLogs.test.tsx](frontend/src/pages/__tests__/AuditLogs.test.tsx), [frontend/src/pages/__tests__/UsersPage.test.tsx](frontend/src/pages/__tests__/UsersPage.test.tsx)
- Test setup: [frontend/src/test/setup.ts](frontend/src/test/setup.ts)
- Coverage config: [frontend/vitest.config.ts](frontend/vitest.config.ts), [codecov.yml](codecov.yml)
- Ignore files review: [.gitignore](.gitignore), [.dockerignore](.dockerignore)

## 8. Confidence Score

Confidence: 84 percent

Rationale: The issues are scoped to two test files with known sources (console error logging and debounced timers). Minor uncertainty remains around timer behavior in user-event sequences across different Vitest versions.
