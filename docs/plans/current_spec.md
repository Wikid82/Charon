## Frontend Coverage Fast-Recovery Plan (Minimum Threshold First)

Date: 2026-02-16
Owner: Planning Agent
Scope: Raise frontend unit-test coverage to project minimum quickly, without destabilizing ongoing flaky E2E CI validation.

## 1) Objective

Recover frontend coverage to the minimum required gate with the fewest
iterations by targeting the biggest low-coverage modules first, starting
with high-yield API files and then selected large UI files.

Primary gate:
- Frontend lines coverage >= 85% (Codecov project `frontend` + local Vitest gate)

Hard constraints:
- Do not modify production behavior unless a testability blocker is proven.
- Keep E2E stabilization work isolated (final flaky E2E already in CI validation).

## 2) Research Findings (Current Snapshot)

Baseline sources discovered:
- `frontend/coverage.log` (recent Vitest coverage table with uncovered ranges)
- `frontend/vitest.config.ts` (default gate from `CHARON_MIN_COVERAGE`/`CPM_MIN_COVERAGE`, fallback `85.0`)
- `codecov.yml` (frontend project target `85%`, patch target `100%`)

Observed recent baseline in `frontend/coverage.log`:
- All files lines: `86.91%`
- Note: this run used a stricter environment gate (`88%`) and failed that stricter gate.

### Ranked High-Yield Candidates (size + low current line coverage)

Estimates below use current file length as approximation and prioritize
modules where added tests can cover many currently uncovered lines quickly.

| Rank | Module | File lines | Current line coverage | Approx uncovered lines | Existing test target to extend | Expected project lines impact |
|---|---|---:|---:|---:|---|---|
| 1 | `src/api/securityHeaders.ts` | 188 | 10.00% | ~169 | `frontend/src/api/__tests__/securityHeaders.test.ts` | +2.0% to +3.2% |
| 2 | `src/api/import.ts` | 137 | 31.57% | ~94 | `frontend/src/api/__tests__/import.test.ts` | +1.0% to +1.9% |
| 3 | `src/pages/UsersPage.tsx` | 775 | 75.67% | ~189 | `frontend/src/pages/__tests__/UsersPage.test.tsx` | +0.5% to +1.3% |
| 4 | `src/pages/Security.tsx` | 643 | 72.22% | ~179 | `frontend/src/pages/__tests__/Security.test.tsx` | +0.4% to +1.1% |
| 5 | `src/pages/Uptime.tsx` | 591 | 74.77% | ~149 | `frontend/src/pages/__tests__/Uptime.test.tsx` | +0.4% to +1.0% |
| 6 | `src/pages/SecurityHeaders.tsx` | 340 | 69.35% | ~104 | `frontend/src/pages/__tests__/SecurityHeaders.test.tsx` | +0.3% to +0.9% |
| 7 | `src/pages/Plugins.tsx` | 391 | 62.26% | ~148 | `frontend/src/pages/__tests__/Plugins.test.tsx` | +0.3% to +0.9% |
| 8 | `src/components/SecurityHeaderProfileForm.tsx` | 467 | 58.97% | ~192 | `frontend/src/components/__tests__/SecurityHeaderProfileForm.test.tsx` | +0.4% to +1.0% |
| 9 | `src/components/CredentialManager.tsx` | 609 | 75.75% | ~148 | `frontend/src/components/__tests__/CredentialManager.test.tsx` | +0.3% to +0.8% |
| 10 | `src/api/client.ts` | 71 | 34.78% | ~46 | `frontend/src/api/__tests__/client.test.ts` | +0.2% to +0.6% |

Planning decision:
- Start with API modules (`securityHeaders.ts`, `import.ts`, optional `client.ts`) for fastest coverage-per-test effort.
- Move to large pages only if the threshold is still not satisfied.

## 3) Requirements in EARS Notation
- WHEN frontend lines coverage is below the minimum threshold, THE SYSTEM SHALL prioritize modules by uncovered size and low coverage first.
- WHEN selecting coverage targets, THE SYSTEM SHALL use existing test files before creating new test files.
- WHEN collecting baseline and post-change metrics, THE SYSTEM SHALL use project-approved tasks/scripts only.
- WHEN E2E is already being validated in CI, THE SYSTEM SHALL avoid introducing E2E test/config changes in this coverage effort.
- IF frontend threshold is still not met after minimal path execution, THEN THE SYSTEM SHALL execute fallback targets in priority order until threshold is met.
- WHEN coverage work is complete, THE SYSTEM SHALL pass frontend coverage gate, type-check, and manual pre-commit checks required by project testing instructions.


## 4) Technical Specification (Coverage-Only)

### In Scope
- Frontend unit tests (Vitest) only.
- Extending existing tests under:
  - `frontend/src/api/__tests__/`
  - `frontend/src/pages/__tests__/`
  - `frontend/src/components/__tests__/`

### Out of Scope
- Backend changes.
- Playwright test logic changes.
- CI workflow redesign.
- Product behavior changes unrelated to testability.

### No Schema/API Contract Changes
- No backend API contract changes are required for this plan.
- No database changes are required.

## 5) Phased Execution Plan

### Phase 0 — Baseline and Target Lock (single pass)

Goal: establish current truth and exact gap to 85%.

1. Run approved frontend coverage task:
   - Preferred: VS Code task `Test: Frontend Coverage (Vitest)`
   - Equivalent script path: `.github/skills/scripts/skill-runner.sh test-frontend-coverage`
2. Capture baseline artifacts:
   - `frontend/coverage/coverage-summary.json`
   - `frontend/coverage/lcov.info`
3. Record baseline:
   - total lines pct
   - delta to 85%
   - top 10 uncovered modules (from `coverage.log`/summary)
4. Early-exit gate (immediately after baseline capture):
    - Read active threshold from the same gate source used by the coverage task
       (`CHARON_MIN_COVERAGE`/`CPM_MIN_COVERAGE`, fallback `85.0`).
    - IF baseline frontend lines pct is already >= active threshold,
       THEN stop further test additions for this plan cycle.

Exit criteria:
- Baseline numbers captured once and frozen for this cycle.
- Either baseline is below threshold and Phase 1 proceeds, or execution exits
   early because baseline already meets/exceeds threshold.

### Phase 1 — Minimal Path (fewest requests/iterations)

Goal: cross 85% quickly with smallest change set.

Target set A (execute in order):
1. `frontend/src/api/__tests__/securityHeaders.test.ts`
2. `frontend/src/api/__tests__/import.test.ts`
3. `frontend/src/api/__tests__/client.test.ts` (only if needed after #1-#2)

Test focus inside these files:
- error mapping branches
- non-2xx response handling
- optional parameter/query serialization branches
- retry/timeout/cancel edge paths where already implemented

Validation after each target (or pair):
- run frontend coverage task
- read updated total lines pct
- stop as soon as >= 85%

Expected result:
- Most likely to reach threshold within 2-3 targeted API test updates.

### Phase 2 — Secondary Path (only if still below threshold)

Goal: add one large UI target at a time, highest projected return first.

Target set B (execute in order, stop once >= 85%):
1. `frontend/src/pages/__tests__/UsersPage.test.tsx`
2. `frontend/src/pages/__tests__/Uptime.test.tsx`
3. `frontend/src/pages/__tests__/SecurityHeaders.test.tsx`
4. `frontend/src/pages/__tests__/Plugins.test.tsx`
5. `frontend/src/components/__tests__/SecurityHeaderProfileForm.test.tsx`
6. `frontend/src/pages/__tests__/Security.test.tsx` (de-prioritized because
   it is currently fully skipped; only consider if skip state is removed)

Test focus:
- critical uncovered conditional render branches
- form validation and submit error paths
- loading/empty/error states not currently asserted

### Phase 3 — Final Verification and Gates

1. E2E verification-first check (run-first policy):
   - Run `Test: E2E Playwright (Skill)` first.
   - Use `Test: E2E Playwright (Targeted Suite)` only when scoped execution is
     sufficient for the impacted area.
   - No E2E code/config changes are allowed in this plan.
2. Frontend coverage:
   - VS Code task `Test: Frontend Coverage (Vitest)`
3. Frontend type-check:
   - VS Code task `Lint: TypeScript Check`
4. Manual pre-commit checks:
   - VS Code task `Lint: Pre-commit (All Files)`
5. Confirm Codecov expectations:
   - project frontend target >= 85%
   - patch coverage target = 100% for modified lines

## 6) Baseline vs Post-Change Collection Protocol

Use this exact protocol for both baseline and post-change snapshots:

1. Execute `Test: Frontend Coverage (Vitest)`.
2. Save metrics from `frontend/coverage/coverage-summary.json`:
   - lines, statements, functions, branches.
3. Keep `frontend/coverage/lcov.info` as Codecov upload source.
4. Compare baseline vs post:
   - absolute lines pct delta
   - per-target module line pct deltas

Reporting format:
- `Baseline lines: X%`
- `Post lines: Y%`
- `Net gain: (Y - X)%`
- `Threshold status: PASS/FAIL`

## 6.1) Codecov Patch Triage (Explicit, Required)

Patch triage must capture exact missing/partial patch ranges from Codecov Patch
view and map each range to concrete tests.

Required workflow:
1. Open PR Patch view in Codecov.
2. Copy exact missing/partial ranges into the table below.
3. Map each range to a specific test file and test case additions.
4. Re-run coverage and update status until all listed ranges are covered.

Required triage table template:

| File | Patch status | Exact missing/partial patch range(s) | Uncovered patch lines | Mapped test file | Planned test case(s) for exact ranges | Status |
|---|---|---|---:|---|---|---|
| `frontend/src/...` | Missing or Partial | `Lxx-Lyy`; `Laa-Lbb` | 0 | `frontend/src/.../__tests__/...test.ts[x]` | `it('...')` cases covering each listed range | Open / In Progress / Done |

Rules:
- Ranges must be copied exactly from Codecov Patch view (not paraphrased).
- Non-contiguous ranges must be listed explicitly.
- Do not mark triage complete until every listed range is covered by passing tests.

## 7) Risk Controls (Protect Ongoing Flaky E2E Validation)
- No edits to Playwright specs, fixtures, config, sharding, retries, or setup.
- No edits to `.docker/compose/docker-compose.playwright-*.yml`.
- No edits to global app runtime behavior unless a testability blocker is proven.
- Keep all changes inside frontend unit tests unless absolutely required.
- Run E2E verification-first as an execution gate using approved task labels,
  but make no E2E test/config changes; keep flaky E2E stabilization scope in CI.


## 8) Minimal Path and Fallback Path

### Minimal Path (default)
- Baseline once.
- Apply the early-exit gate immediately after baseline capture.
- Update tests for:
  1) `securityHeaders.ts`
  2) `import.ts`
- Re-run coverage.
- If >= 85%, stop and verify final gates.

### Fallback Path (if still below threshold)
- Add `client.ts` API tests.
- If still below, add one UI target at a time in this order:
   `UsersPage` -> `Uptime` -> `SecurityHeaders` -> `Plugins` -> `SecurityHeaderProfileForm` -> `Security (de-prioritized while fully skipped)`.
- Re-run coverage after each addition; stop immediately when threshold is reached.

## 9) Config/Ignore File Recommendations (Only If Needed)

Current assessment for this effort:
- `.gitignore`: already excludes frontend coverage artifacts.
- `codecov.yml`: already enforces frontend 85% and patch 100% with suitable ignores.
- `.dockerignore`: already excludes frontend coverage/test artifacts from image context.
- `Dockerfile`: no changes required for unit coverage-only work.

Decision:
- No config-file modifications are required for this coverage recovery task.

## 10) Acceptance Checklist
- [ ] Baseline coverage collected with approved frontend coverage task.
- [ ] Target ranking confirmed from largest-lowest-coverage modules.
- [ ] Minimal path executed first (API targets before UI targets).
- [ ] Early-exit gate applied after baseline capture.
- [ ] Frontend lines coverage >= 85%.
- [ ] TypeScript check passes.
- [ ] Manual pre-commit run passes.
- [ ] E2E verification-first gate executed with approved task labels (no E2E code/config changes).
- [ ] No Playwright/E2E infra changes introduced.
- [ ] Post-change coverage summary recorded.
- [ ] Codecov patch triage table completed with exact missing/partial ranges and mapped tests.


## 11) Definition of Done (Coverage Task Specific)

Done is achieved only when all are true:
1. Frontend lines coverage is >= 85% using project-approved coverage task output.
2. Coverage gains come from targeted high-yield modules listed in this plan.
3. Type-check and manual pre-commit checks pass.
4. No changes were made that destabilize or alter flaky E2E CI validation scope.
5. Codecov patch coverage expectations remain satisfiable (100% for modified lines).
6. Baseline/post-change metrics and final threshold status are documented in the task handoff.
