## Playwright E2E Green Plan (Single Objective)

Date: 2026-02-15
Owner: Planning Agent
Scope: Achieve 100% green Playwright E2E quickly through root-cause fixes and performance/stability optimization.

---

## Objective

Deliver a fully green Playwright E2E suite with deterministic results and controlled runtime by fixing root causes first (not symptom retries).

This file contains one objective only for today’s request.

---

## Requirements (EARS)

- WHEN the E2E environment is invalid or stale, THE SYSTEM SHALL stop execution and rebuild before test reproduction.
- WHEN a failure is reproduced, THE SYSTEM SHALL identify root cause in frontend/backend/helpers before mitigation.
- WHEN synchronization is required, THE SYSTEM SHALL use condition-based waits and deterministic locators rather than fixed sleeps.
- WHEN `tests/core/data-consistency.spec.ts` is relevant to the failing path, THE SYSTEM SHALL include it consistently in targeted reproduction, impacted rerun, browser fan-out, and final confirmation.
- WHEN all impacted fixes are complete, THE SYSTEM SHALL pass security matrix and full split confirmation without regressions.
- WHEN production code is modified, THE SYSTEM SHALL complete coverage runs before final sign-off.
- WHEN Codecov Patch view reports missing or partial coverage, THE SYSTEM SHALL require 100% patch coverage for modified lines before go-live.
- WHEN patch coverage is below 100%, THE SYSTEM SHALL capture exact missing/partial line ranges and map each range to targeted tests.
- WHEN stability is being validated, THE SYSTEM SHALL use Playwright-native `--repeat-each` consecutive-pass gates rather than ad hoc loops.
- WHEN classifying possible state contamination, THE SYSTEM SHALL run an early single-worker diagnostic branch (`--workers=1`) before parallel reruns.
- WHEN retries are enabled for instrumentation, THE SYSTEM SHALL enforce `--fail-on-flaky-tests` so flaky-pass results do not qualify as green.

---

## Hard-Gated Execution Order (Stop/Go)

### Gate 1: Environment Validity and Rebuild Decision

Stop:
- `charon-e2e` unhealthy, missing required env, stale runtime after app/runtime changes.

Go:
- Health checks pass and runtime mode matches test scope.

Order:
1. Validate health and required env.
2. Prefer runner-managed setup where feasible:
   - Use Playwright project-dependency setup flow (for example, setup project + dependent browser projects) instead of external-only setup scripts when equivalent.
   - Use `webServer` readiness gating option when applicable so runner controls startup/ready checks.
3. Decide rebuild using testing protocol:
   - Rebuild required for runtime changes (`backend/**`, `frontend/**`, runtime/Docker inputs).
   - Reuse container for test-only changes if healthy.

### Gate 2: Targeted Shard Reproduction

Stop:
- Failure not reproducible in targeted shard/spec.

Go:
- Failure reproduced with clear signal and trace.

Order:
1. Reproduce in smallest shard/spec containing failure.
2. Capture trace/artifacts once failure is reproduced.

### Gate 3: Root-Cause Fix Loop

Stop:
- Change does not map to verified cause.
- Proposed fix is only timeout/retry inflation.

Go:
- Cause mapped to specific file/function/component.

Loop:
1. Classify cause (`SYNC_WAIT`, `LOCATOR_AMBIGUITY`, `STATE_LEAK`, `ENV_ORCHESTRATION`, `PERF_TIMEOUT`).
2. Run taxonomy-first fastest diagnostic move:
   - `SYNC_WAIT`: run failing test with trace + UI mode/timeouts unchanged; verify missing awaited state transition first.
   - `LOCATOR_AMBIGUITY`: run strict locator count/assertion first (`toHaveCount(1)`/role+name narrowing) before selector rewrites.
   - `STATE_LEAK`: run early deterministic branch with `--workers=1` and same shard/spec to confirm isolation issue.
   - `ENV_ORCHESTRATION`: verify setup project execution order and `webServer` ready signal before app-code changes.
   - `PERF_TIMEOUT`: run smallest repro with unchanged expectations and capture slow step timings before raising timeout.
3. Apply root-cause fix.
4. Re-run smallest failing scope.
5. Repeat until deterministic pass.

### Gate 4: Impacted Shard Rerun

Stop:
- Any failure in impacted shard after fix.

Go:
- Impacted shard fully green.

### Gate 5: Browser Fan-Out

Stop:
- Any browser fails on impacted scope.

Go:
- Chromium + Firefox + WebKit pass impacted scope.

Order:
1. Single-browser deterministic validation first (primary browser baseline).
2. Cross-browser fan-out second (Chromium + Firefox + WebKit).

### Gate 6: Security Matrix and Full Split Confirmation

Stop:
- Security suites fail or split topology regresses.

Go:
- Security matrix green and full split pipeline green.

### Gate 7: Mandatory Coverage and Codecov Patch Triage

Stop:
- Coverage runs not completed for modified production code.
- Codecov Patch view not reviewed after coverage upload.
- Patch coverage for modified lines is < 100%.
- Missing/partial patch line ranges are not documented with mapped targeted tests.

Go:
- Coverage runs completed.
- Codecov Patch coverage for modified lines is exactly 100%.
- All missing/partial ranges are triaged and resolved with targeted tests.

Order:
1. Run required coverage suites for modified areas.
2. Open Codecov Patch view.
3. Copy exact missing/partial modified line ranges.
4. Map each range to targeted tests.
5. Add/adjust tests and rerun coverage until patch coverage is 100%.

---

## Performance and Stability Thresholds

### Runtime Thresholds

- Baseline source: most recent known-good split run on same branch/runtime profile.
- Non-security shard runtime regression allowed: <= 10% per shard.
- Total non-security wall-clock regression allowed: <= 10%.
- Security matrix runtime regression allowed: <= 15%.

Runtime fail:
- Any threshold exceeded without explicit documented acceptance as follow-up.

### Stability / Flake Thresholds

- Targeted repaired failure: 3 consecutive passes required.
- Impacted shard: 2 consecutive passes required.
- Browser fan-out: 2 consecutive passes per browser on impacted scope.
- Final full split confirmation: 1 full run with zero retry-required failures.
- Consecutive-pass gates use Playwright-native `--repeat-each`.
- Retry runs are instrumentation only (small CI retry count) and must be paired with `--fail-on-flaky-tests`.

Stability fail:
- Any non-deterministic reappearance inside required consecutive pass window.
- Any flaky-pass classified by Playwright as flaky.

### Auth-State Reuse Nuance

- Shared `storageState` reuse is acceptable only when server-side state mutation conflicts are controlled.
- If tests mutate shared server-side entities (user/session/settings records), isolate state per test/suite or reset deterministically before reuse.

---

## Root-Cause-First Focus Areas

### Orchestration and helpers
- `tests/global-setup.ts` (`waitForContainer`, `emergencySecurityReset`)
- `tests/auth.setup.ts` (`performLoginAndSaveState`, `resetAdminCredentials`)
- `tests/utils/wait-helpers.ts`
- `tests/utils/ui-helpers.ts`
- `tests/utils/TestDataManager.ts`

### Flake-prone suites
- `tests/core/navigation.spec.ts`
- `tests/core/proxy-hosts.spec.ts`
- `tests/core/data-consistency.spec.ts`
- `tests/settings/user-management.spec.ts`
- `tests/settings/smtp-settings.spec.ts`
- `tests/settings/notifications.spec.ts`
- `tests/tasks/*.spec.ts`

### UI/component hotspots
- `frontend/src/components/ProxyHostForm.tsx`
- `frontend/src/pages/ProxyHosts.tsx`
- `frontend/src/pages/UsersPage.tsx`
- `frontend/src/pages/Settings.tsx`
- `frontend/src/pages/Certificates.tsx`

### Backend integrity path
- `backend/internal/api/handlers/proxy_host_handler.go` (`Update`)
- `backend/internal/services/proxyhost_service.go` (`Update`, validation paths)
- `backend/internal/models/proxy_host.go`

---

## Data-Consistency Spec Policy (Aligned)

`tests/core/data-consistency.spec.ts` is consistently in scope for this plan:

- Included in targeted reproduction when active failure signal.
- Included in impacted shard reruns after relevant fixes.
- Included in cross-browser fan-out for impacted scope.
- Included in final full split confirmation.

No phase excludes this spec while claiming full-green readiness.

---

## Phased Task Plan

### Phase 1: Environment and Reproduction
- Complete Gate 1 and Gate 2.
- Produce failure map with taxonomy and ownership.

### Phase 2: Root-Cause Fix Loop
- Complete Gate 3.
- Prioritize helper/contract/product fixes over retries/timeouts.

### Phase 3: Impacted Validation
- Complete Gate 4 and Gate 5.
- Enforce consecutive-pass thresholds.

### Phase 4: Full Confirmation
- Complete Gate 6.
- Complete Gate 7.
- Verify full-green state for split topology and security matrix.

### Phase 5: Patch Coverage Triage Closure
- This phase runs after implementation changes and after coverage is executed/uploaded.
- Capture exact missing/partial line ranges from Codecov Patch view.
- Maintain line-range-to-test mapping until each range is covered.
- Re-run only targeted suites first, then required final confirmation suite.

Status convention for this phase:
- `Pending Execution`: acceptable before implementation and before coverage + Codecov Patch review are run.
- `Closed`: required at completion for every triage entry.

#### Codecov Patch Triage Table (Mandatory)

Note: `Codecov Patch line range` and related placeholders below are execution artifacts. Populate them only in Phase 5 after running coverage and opening the Codecov Patch view.

| Codecov Patch line range | File | Coverage status | Targeted test(s) to add/run | Owner | Status |
| --- | --- | --- | --- | --- | --- |
| `<paste exact range from Codecov>` | `<path>` | Missing/Partial | `<test file + test name>` | `<name>` | Pending Execution |

---

## Critical Path Exclusions

Removed from critical path for this request:
- `.gitignore` audit
- `codecov.yml` audit
- `.dockerignore` audit
- `Dockerfile` audit

These are non-blocking unless directly proven as root cause of active E2E failures.

---

## Non-Blocking Follow-Up (Optional)

- Config hygiene review for `.gitignore`, `codecov.yml`, `.dockerignore`, `Dockerfile`.
- Additional CI/runtime optimization outside current pass criteria.

---

## Definition of Done

1. Hard-gated execution order completed without skipped stop/go checks.
2. Active failures fixed via verified root causes.
3. `tests/core/data-consistency.spec.ts` handled consistently per policy.
4. Impacted shards green with required consecutive passes.
5. Browser fan-out green with required consecutive passes.
6. Security matrix and full split confirmation green.
7. Runtime/stability thresholds satisfied, or explicit follow-up recorded for approved exceptions.
8. Coverage completion is documented for all modified production code.
9. Codecov Patch coverage for modified lines is 100%.
10. Codecov Patch missing/partial line ranges are explicitly captured and each is mapped to targeted tests, with all entries closed.

---

## Policy-Bound Caveat (Coverage)

- Repository policy remains a hard gate: modified production lines require 100% Codecov patch coverage.
- No exceptions are allowed unless repository policy itself is changed.
- Filler tests are not acceptable; each missing/partial patch line must be covered by behavior-linked targeted tests tied to the affected scenario.
