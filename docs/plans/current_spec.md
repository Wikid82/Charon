## QA Test Failure Remediation Addendum (2026-02-19)

### Scope

- In scope: test instability fixes and UI expectation alignment for the currently failing QA gates only.
- Out of scope: feature additions, backend behavior changes unrelated to failing assertions, design refactors, and unrelated test cleanup.

### Failure Inventory (Source: `docs/reports/qa_report.md`)

1. `tests/settings/notifications.spec.ts`
  - Failing case A: provider enable/disable checkbox interaction fails (click interception/out-of-viewport).
  - Failing case B: invalid template preview assertion matches hidden text instead of visible error feedback.
2. `tests/security-enforcement/zzz-security-ui/system-security-settings.spec.ts`
  - Failing case: save success toast not visible in `should save general settings successfully`.
3. `frontend/src/pages/__tests__/Security.functional.test.tsx`
  - Failing case: expects Notifications button disabled when Cerberus is disabled, but current UI behavior keeps header action enabled.
4. Local patch preflight warning
  - `scripts/local-patch-report.sh` exits warning mode when `frontend/coverage/lcov.info` is missing, while still producing required artifacts.

### Root-Cause Hypotheses (Per Failing Test)

#### 1) `tests/settings/notifications.spec.ts`

- **Case A (enable/disable provider):**
  - Locator strategy is fragile (`checkbox` with sibling-label heuristic and fallback `first()`), so interaction can target a non-actionable/offscreen element after modal/card layout shifts.
  - `setChecked` can still fail when the selected checkbox is occluded by sticky UI layers or not in viewport; no explicit scroll/visibility stabilization is performed before toggling.
- **Case B (invalid template preview error):**
  - Assertion `getByText(/error|failed|invalid/i).first()` is broad and may resolve to hidden/static text (for example option labels or non-toast content) rather than user-visible error feedback.
  - Test does not scope to live regions, toast container, or form-level validation block, causing false target selection.

#### 2) `tests/security-enforcement/zzz-security-ui/system-security-settings.spec.ts`

- Save operation likely succeeds (API 200 observed), but toast visibility assertion is timing- and selector-sensitive.
- Current assertion mixes generic role/status and data-testid checks without first anchoring to a deterministic post-save signal (response + stable UI state), so ephemeral toast rendering window is missed in some runs.

#### 3) `frontend/src/pages/__tests__/Security.functional.test.tsx`

- Test expectation drift: spec assumes Notifications header button must be disabled when Cerberus is off.
- Current UI behavior appears to allow opening Notification settings regardless of Cerberus state (feature warning applies to enforcement controls, not necessarily header actions), making test assertion stale.

#### 4) `scripts/local-patch-report.sh` / coverage preflight

- Script behavior is by design: in missing-input mode it writes `test-results/local-patch-report.md` and `test-results/local-patch-report.json` then exits non-zero.
- QA gate interpretation is misaligned: artifact generation succeeds, but warning mode is treated as failure rather than preflight warning requiring upstream coverage generation.

### Ordered Minimal Fix Steps (Test + UI Expectation Alignment Only)

1. **Stabilize Notifications checkbox interaction**
  **Target:** `tests/settings/notifications.spec.ts`
  - Replace fallback checkbox locator with deterministic test-id or form-scoped role query tied to Enabled control only.
  - Add pre-action stabilization: ensure element is attached, visible, enabled, and scrolled into view before toggle.
  - Prefer click on associated label/control pair in modal context if native checkbox is visually wrapped.

2. **Narrow invalid-template error assertion to visible feedback channel**
  **Target:** `tests/settings/notifications.spec.ts`
  - Replace generic text matcher with scoped locator order: explicit error toast (`data-testid`/role alert), then visible form validation container.
  - Assert visibility on that scoped locator and avoid `.first()` on broad text searches.

3. **Stabilize system settings save-success verification**
  **Target:** `tests/security-enforcement/zzz-security-ui/system-security-settings.spec.ts`
  - Keep API response wait, then assert one deterministic success channel (shared toast helper or single canonical toast locator) with bounded timeout.
  - Avoid mixed fallback selectors that may match stale/inactive nodes.

4. **Align unit test expectation with intended UI contract**
  **Target:** `frontend/src/pages/__tests__/Security.functional.test.tsx` (+ `frontend/src/pages/Security.tsx` only if behavior contract requires it)
  - Confirm intended contract from current security page behavior and copy text semantics.
  - Minimal change path: update assertion to expected enabled state if product behavior intentionally permits Notifications action while Cerberus is disabled.
  - Alternate (only if contract requires disabled action): adjust button disabled logic in `Security.tsx` and keep existing unit expectation.
  - Choose exactly one path; do not change unrelated security-card logic.

5. **Clarify patch-preflight gate interpretation and sequencing**
  **Targets:** `docs/reports/qa_report.md` (evidence wording), optional test/runbook docs if needed
  - Record that preflight artifacts were produced in warning mode due to missing frontend coverage input.
  - Keep remediation action in validation sequence: generate frontend coverage before local-patch-report when full green gate is required.

### Validation Sequence (Return Full Gate to Green)

1. Run targeted Playwright notifications suite:
  `tests/settings/notifications.spec.ts` (Firefox project as currently gated).
2. Run targeted security UI suite:
  `tests/security-enforcement/zzz-security-ui/system-security-settings.spec.ts` (`security-tests` project).
3. Run focused frontend unit test file:
  `frontend/src/pages/__tests__/Security.functional.test.tsx`.
4. Generate frontend coverage output (`frontend/coverage/lcov.info`) via frontend coverage task/script.
5. Re-run local patch preflight and verify both artifacts exist and warning is cleared.
6. Re-run full required QA gate order for this branch:
  - Playwright targeted suites
  - Local patch preflight
  - Backend/frontend coverage gates
  - Type check
  - Pre-commit all files
  - Security scans (Trivy, Docker image scan, CodeQL Go/JS + findings gate)

### Acceptance Criteria (Addendum-Specific)

- All three failing test files pass in their targeted runs.
- No assertion relies on broad hidden-text matching for error/success feedback.
- Notifications unit assertion reflects the actual intended UI contract (test and implementation consistent).
- `test-results/local-patch-report.md` and `test-results/local-patch-report.json` are present, and preflight no longer reports missing `frontend/coverage/lcov.info`.
- No unrelated feature or architectural change is introduced.

## Notify-only PR1 Remediation Addendum (Supervisor Unblock)

Date: 2026-02-19
Status: Active addendum for PR1 unblock.
Precedence: This addendum overrides conflicting sections below for PR1 execution.

### Scope Lock

- In scope: Notify-only cutover for external notification provider dispatch, legacy provider migration completeness, and minimal QA unblock evidence.
- Out of scope: certificate flaky test work and any unrelated certificate/runtime refactors.

### Dispatch Source of Truth (Decision)

- **Selected approach: direct hard-cutover runtime logic** (not router+flags dispatch).
- Runtime authority is in:
  - `backend/internal/services/notification_service.go`
   - `supportsJSONTemplates(...)`
   - `(*NotificationService).SendExternal(...)`
   - `(*NotificationService).TestProvider(...)`
   - `legacyFallbackInvocationError(...)`
  - `backend/internal/api/routes/routes.go`
   - `notificationService := services.NewNotificationService(db)`
- `backend/internal/notifications/router.go` is non-authoritative for PR1 runtime dispatch and must not be introduced into runtime flow for this unblock.

### Legacy Provider Migration Completeness (Notify-only Runtime)

1. Add a single reconciliation pass in `backend/internal/services/notification_service.go`:
  - target function: `EnsureNotifyOnlyProviderMigration(ctx context.Context) error`
  - invoked once from route boot path after `NewNotificationService(db)` wiring.
2. Reconcile each `notification_providers` row to terminal state:
  - Supported types (`webhook`, `discord`, `slack`, `gotify`, `generic`):
    - `engine="notify_v1"`, `migration_state="migrated"`, `migration_error=""`, `last_migrated_at=now`
  - Unsupported types under notify-only (for example `telegram`):
    - `migration_state="failed"`, `migration_error="unsupported provider type in notify-only runtime"`, `enabled=false`, `last_migrated_at=now`
  - Preserve prior origin URL in `legacy_url` when mutating ambiguous/legacy rows.
3. Keep fail-closed runtime semantics:
  - `SendExternal`/`TestProvider` reject unsupported providers without legacy fallback.
4. Migration completeness gate (must be zero):
  - enabled providers where `migration_state` is empty, `pending`, or `failed`.

### Ordered Remediation Tasks (Minimal, QA-first)

1. Update plan scope and precedence in `docs/plans/current_spec.md` (this addendum).
2. Lock runtime dispatch authority to direct notify-only logic in `backend/internal/services/notification_service.go`.
3. Add migration reconciliation pass in `backend/internal/services/notification_service.go` and call it from `backend/internal/api/routes/routes.go`.
4. Preserve server-managed migration fields behavior in `backend/internal/api/handlers/notification_provider_handler.go`.
5. Capture unblock evidence in `docs/reports/qa_report.md`.

### Test List (Execution Order)

1. `go test ./backend/internal/services -run 'TestNotificationService_SendExternal_NotifyOnlyBlocksLegacyFallback|TestSendExternal_UnsupportedProviderFailsClosed|TestTestProvider_NotifyOnlyRejectsUnsupportedProvider|TestTestProvider_DiscordUsesNotifyPathInPR1'`
2. `go test ./backend/internal/api/handlers -run 'TestNotificationProviderHandler_CreateIgnoresServerManagedMigrationFields|TestNotificationProviderHandler_UpdatePreservesServerManagedMigrationFields'`
3. Add/run migration completeness test:
  - `go test ./backend/internal/services -run 'TestNotificationService_EnsureNotifyOnlyProviderMigration'`
4. `npx playwright test tests/settings/notifications.spec.ts --project=firefox`
5. `bash scripts/local-patch-report.sh` (must emit `test-results/local-patch-report.md` and `test-results/local-patch-report.json`)

### Acceptance Criteria (PR1 Unblock)

1. `current_spec.md` clearly reflects Notify-only PR1 scope and excludes certificate flaky scope.
2. Exactly one backend dispatch source-of-truth is documented and used: direct hard-cutover in `NotificationService`.
3. Legacy provider migration reconciliation exists and is covered by targeted tests.
4. No enabled provider remains in non-terminal migration state under Notify-only runtime.
5. QA evidence is updated for Supervisor re-review.

## Fix Flaky Go Test: `TestCertificateHandler_List_WithCertificates`

### 1) Introduction

This specification defines a focused, low-risk plan to eliminate intermittent CI failures for:

- `backend/internal/api/handlers.TestCertificateHandler_List_WithCertificates`

while preserving production behavior in:

- `backend/internal/services/certificate_service.go`
- `backend/internal/api/handlers/certificate_handler.go`
- `backend/internal/api/routes/routes.go`

Primary objective:

- Remove nondeterministic startup concurrency in certificate service initialization that intermittently races with handler list calls under CI parallelism/race detection.

### Decision Record - 2026-02-18

**Decision:** For this targeted flaky-fix, `docs/plans/current_spec.md` is the temporary single source of truth instead of creating separate `requirements.md`, `design.md`, and `tasks.md` artifacts.

**Context:** The scope is intentionally narrow (single flaky test root-cause + deterministic validation loop), and splitting artifacts now would add process overhead without reducing delivery risk.

**Traceability mapping:**

- Requirements content is captured in Section 3 (EARS requirements).
- Design content is captured in Section 4 (technical specifications).
- Task breakdown and validation gates are captured in Section 5 (implementation plan).

**Review trigger:** If scope expands beyond the certificate flaky-fix boundary, restore full artifact split into `requirements.md`, `design.md`, and `tasks.md`.

---

### 2) Research Findings

#### 2.1 CI failure evidence

Observed in `.github/logs/ci_failure.log`:

- `WARNING: DATA RACE`
- failing test: `TestCertificateHandler_List_WithCertificates`
- conflicting access path:
  - **Write** from `(*CertificateService).SyncFromDisk` (`certificate_service.go`)
  - **Read** from `(*CertificateService).ListCertificates` (`certificate_service.go`), triggered via handler list path

#### 2.2 Existing architecture and hotspot map

**Service layer**

- File: `backend/internal/services/certificate_service.go`
- Key symbols:
  - `NewCertificateService(caddyDataDir string, db *gorm.DB) *CertificateService`
  - `SyncFromDisk() error`
  - `ListCertificates() ([]models.SSLCertificate, error)`
  - `InvalidateCache()`
  - `refreshCacheFromDB() error`

**Handler layer**

- File: `backend/internal/api/handlers/certificate_handler.go`
- Key symbols:
  - `func (h *CertificateHandler) List(c *gin.Context)`
  - `func (h *CertificateHandler) Upload(c *gin.Context)`
  - `func (h *CertificateHandler) Delete(c *gin.Context)`

**Route wiring**

- File: `backend/internal/api/routes/routes.go`
- Key symbol usage:
  - `services.NewCertificateService(caddyDataDir, db)`

**Tests and setup patterns**

- Files:
  - `backend/internal/api/handlers/certificate_handler_coverage_test.go`
  - `backend/internal/api/handlers/certificate_handler_test.go`
  - `backend/internal/api/handlers/certificate_handler_security_test.go`
  - `backend/internal/services/certificate_service_test.go`
  - `backend/internal/api/handlers/testdb.go`
- Findings:
  - many handler tests instantiate the real constructor directly.
  - one test already includes a timing workaround (`time.Sleep`) due to startup race behavior.
  - service tests already use a helper pattern that avoids constructor-side async startup, which validates the architectural direction for deterministic initialization in tests.

#### 2.3 Root cause summary

Two explicit root-cause branches are tracked for this flaky area:

- **Branch A — constructor startup race:** `NewCertificateService` starts async state mutation (`SyncFromDisk`) immediately, while handler-driven reads (`ListCertificates`) may execute concurrently. This matches CI race output in `.github/logs/ci_failure.log` for `TestCertificateHandler_List_WithCertificates`.
- **Branch B — DB schema/setup ordering drift in tests:** some test paths reach service queries before deterministic schema setup is complete, producing intermittent setup-order failures such as `no such table: ssl_certificates` and `no such table: proxy_hosts`.

Plan intent: address both branches together in one tight PR by making constructor behavior deterministic and enforcing migration/setup ordering in certificate handler test setup.

---

### 3) EARS Requirements (Source of Truth)

#### 3.1 Ubiquitous requirements

- THE SYSTEM SHALL construct `CertificateService` in a deterministic initialization state before first handler read in tests.
- THE SYSTEM SHALL provide race-free access to certificate cache state during list operations.

#### 3.2 Event-driven requirements

- WHEN `NewCertificateService` returns, THE SYSTEM SHALL guarantee that any required first-read synchronization state is consistent for immediate `ListCertificates` calls.
- WHEN `CertificateHandler.List` is called, THE SYSTEM SHALL return data or a deterministic error without concurrency side effects caused by service startup.
- WHEN certificate handler tests initialize database state, THE SYSTEM SHALL complete required schema setup for certificate-related tables before service/handler execution.

#### 3.3 Unwanted-behavior requirements

- IF CI executes tests with race detector or parallel scheduling, THEN THE SYSTEM SHALL NOT produce data races between service initialization and list reads.
- IF startup synchronization fails, THEN THE SYSTEM SHALL surface explicit errors and preserve handler-level HTTP error behavior.
- IF test setup ordering is incorrect, THEN THE SYSTEM SHALL fail fast in setup with explicit diagnostics instead of deferred `no such table` runtime query failures.

#### 3.4 Optional requirements

- WHERE test-only construction paths are needed, THE SYSTEM SHALL use explicit test helpers/options instead of sleeps or timing assumptions.

---

### 4) Technical Specifications

#### 4.1 Service design changes

Primary design target (preferred):

1. Update `NewCertificateService` to remove nondeterministic startup behavior at construction boundary.
2. Ensure `SyncFromDisk` / cache state transition is serialized relative to early `ListCertificates` usage.
3. Keep existing mutex discipline explicit and auditable (single writer, safe readers).
4. Enforce deterministic DB setup ordering in certificate handler tests through shared setup helper behavior (migrate/setup before constructor and request execution).

Design constraints:

- No public API break for callers in `routes.go`.
- No broad refactor of certificate business logic.
- Keep behavior compatible with current handler response contracts.

#### 4.2 Handler and route impact

- `certificate_handler.go` should require no contract changes.
- `routes.go` keeps existing construction call shape unless a minimally invasive constructor option path is required.

#### 4.3 Data flow (target state)

1. Route wiring constructs certificate service.
2. Service enters stable initialized state before first list read path.
3. Handler `List` requests certificate data.
4. Service reads synchronized cache/DB state.
5. Handler responds `200` with deterministic payload or explicit `500` error.

#### 4.4 Error handling matrix

| Scenario | Expected behavior | Validation |
|---|---|---|
| Startup sync succeeds | List path returns stable data | handler list tests pass repeatedly |
| Startup sync fails | Error bubbles predictably to handler | HTTP 500 tests remain deterministic |
| Empty store | 200 + empty list (or existing contract shape) | existing empty-list tests pass |
| Concurrent test execution | No race detector findings in certificate tests | `go test -race` targeted suite |
| Test DB setup ordering incomplete | setup fails immediately with explicit setup error; no deferred query-time table errors | dedicated setup-ordering test + repeated run threshold |

#### 4.5 API and schema impact

- API endpoints: **no external contract changes**.
- Request/response schema: **unchanged** for certificate list/upload/delete.
- Database schema: **no migrations required**.

---

### 5) Implementation Plan (Phased)

## Phase 1: Playwright / UI-UX Baseline (Gate)

Although this fix is backend-test focused, follow test protocol gate:

- Reuse healthy E2E environment when possible; rebuild only if runtime image inputs changed.
- Use exact task/suite gate:
  - task ID `shell: Test: E2E Playwright (FireFox) - Core: Certificates`
  - suite path executed by the task: `tests/core/certificates.spec.ts`
- If rebuild is required by test protocol, run task ID `shell: Docker: Rebuild E2E Environment` before Playwright.

Deliverables:

- passing targeted Playwright suite for certificate list interactions
- archived output in standard test artifacts

## Phase 2: Backend Service Stabilization

Scope:

- `backend/internal/services/certificate_service.go`

Tasks:

1. Make constructor initialization deterministic at first-use boundary.
2. Remove startup timing dependence that allows early `ListCertificates` to overlap with constructor-initiated sync mutation.
3. Preserve current cache invalidation and DB refresh semantics.
4. Keep lock lifecycle simple and reviewable.

Complexity: **Medium** (single component, concurrency-sensitive)

## Phase 3: Backend Test Hardening

Scope:

- `backend/internal/api/handlers/certificate_handler_coverage_test.go`
- `backend/internal/api/handlers/certificate_handler_test.go`
- `backend/internal/api/handlers/certificate_handler_security_test.go`
- optional shared test helpers:
  - `backend/internal/api/handlers/testdb.go`

Tasks:

1. Remove sleep-based or timing-dependent assumptions.
2. Standardize deterministic service setup helper for handler tests.
3. Ensure flaky case (`TestCertificateHandler_List_WithCertificates`) is fully deterministic.
4. Preserve assertion intent and API-level behavior checks.
5. Add explicit setup-ordering validation in tests to guarantee required schema migration/setup completes before handler/service invocation.

Complexity: **Medium** (many callsites, low logic risk)

## Phase 4: Integration & Validation

Tasks:

1. Run reproducible stability stress loop for the known flaky case via task ID `shell: Test: Backend Flaky - Certificate List Stability Loop`.
  - **Task command payload:**
    - `mkdir -p test-results/flaky && cd /projects/Charon && go test ./backend/internal/api/handlers -run '^TestCertificateHandler_List_WithCertificates$' -count=100 -shuffle=on -parallel=8 -json 2>&1 | tee test-results/flaky/cert-list-stability.jsonl`
  - **Artifactized logging requirement:** persist `test-results/flaky/cert-list-stability.jsonl` for reproducibility and CI comparison.
  - **Pass threshold:** `100/100` successful runs, zero failures.
2. Run race-mode stress for the same path via task ID `shell: Test: Backend Flaky - Certificate List Race Loop`.
  - **Task command payload:**
    - `mkdir -p test-results/flaky && cd /projects/Charon && go test -race ./backend/internal/api/handlers -run '^TestCertificateHandler_List_WithCertificates$' -count=30 -shuffle=on -parallel=8 -json 2>&1 | tee test-results/flaky/cert-list-race.jsonl`
  - **Artifactized logging requirement:** persist `test-results/flaky/cert-list-race.jsonl`.
  - **Pass threshold:** exit code `0` and zero `WARNING: DATA RACE` occurrences.
3. Run setup-ordering validation loop via task ID `shell: Test: Backend Flaky - Certificate DB Setup Ordering Loop`.
  - **Task command payload:**
    - `mkdir -p test-results/flaky && cd /projects/Charon && go test ./backend/internal/api/handlers -run '^TestCertificateHandler_DBSetupOrdering' -count=50 -shuffle=on -parallel=8 -json 2>&1 | tee test-results/flaky/cert-db-setup-ordering.jsonl`
  - **Pass threshold:** `50/50` successful runs and zero `no such table: ssl_certificates|proxy_hosts` messages from positive-path setup runs.
4. Run focused certificate handler regression suite via task ID `shell: Test: Backend Flaky - Certificate Handler Focused Regression`.
  - **Task command payload:**
    - `mkdir -p test-results/flaky && cd /projects/Charon && go test ./backend/internal/api/handlers -run '^TestCertificateHandler_' -count=1 -json 2>&1 | tee test-results/flaky/cert-handler-regression.jsonl`
  - **Pass threshold:** all selected tests pass in one clean run.
5. Execute patch-coverage preflight via existing project task ID `shell: Test: Local Patch Report`.
  - **Task command payload:** `bash scripts/local-patch-report.sh`
  - **Pass threshold:** both artifacts exist: `test-results/local-patch-report.md` and `test-results/local-patch-report.json`.

Task definition status for Phase 4 gates:

- Existing task ID in `.vscode/tasks.json`:
  - `shell: Test: Local Patch Report`
- New task IDs to add in `.vscode/tasks.json` with the exact command payloads above:
  - `shell: Test: Backend Flaky - Certificate List Stability Loop`
  - `shell: Test: Backend Flaky - Certificate List Race Loop`
  - `shell: Test: Backend Flaky - Certificate DB Setup Ordering Loop`
  - `shell: Test: Backend Flaky - Certificate Handler Focused Regression`

Complexity: **Low-Medium**

## Phase 5: Documentation & Operational Hygiene

Tasks:

1. Update `docs/plans/current_spec.md` (this file) if implementation details evolve.
2. Record final decision outcomes and any deferred cleanup tasks.
3. Defer unrelated repository config hygiene (`.gitignore`, `codecov.yml`, `.dockerignore`, `Dockerfile`) unless a direct causal link to this flaky test is proven during implementation.

Complexity: **Low**

---

### 6) PR Slicing Strategy

### Decision

**Primary decision: single PR** for this fix.

Why:

- Scope is tightly coupled (constructor semantics + related tests).
- Minimizes context switching and user review requests.
- Reduces risk of partially landed concurrency behavior.

### Trigger reasons to split into multiple PRs

Split only if one or more occur:

- Constructor change causes wider runtime behavior concerns requiring staged rollout.
- Test hardening touches many unrelated test modules beyond certificate handlers.
- Validation reveals additional root-cause branches outside certificate service/test setup ordering.

### Ordered fallback slices (if split is required)

#### PR-1: Service Determinism Core

Scope:

- `backend/internal/services/certificate_service.go`

Dependencies:

- none

Acceptance criteria:

- service constructor/list path no longer exhibits startup race in targeted race tests
- no public API break at route wiring callsite

Rollback/contingency:

- revert constructor change only; preserve tests unchanged for quick recovery

#### PR-2: Handler Test Determinism

Scope:

- `backend/internal/api/handlers/certificate_handler_coverage_test.go`
- `backend/internal/api/handlers/certificate_handler_test.go`
- `backend/internal/api/handlers/certificate_handler_security_test.go`
- optional helper consolidation in `backend/internal/api/handlers/testdb.go`

Dependencies:

- PR-1 merged (preferred)

Acceptance criteria:

- `TestCertificateHandler_List_WithCertificates` stable across repeated CI-style runs
- no `time.Sleep` timing guard required for constructor race avoidance

Rollback/contingency:

- revert only test refactor while keeping stable service change

---

### 7) Acceptance Criteria (Definition of Done)

1. Stability gate: task ID `shell: Test: Backend Flaky - Certificate List Stability Loop` passes `100/100`.
2. Race gate: task ID `shell: Test: Backend Flaky - Certificate List Race Loop` passes with zero race reports.
3. Setup-ordering gate: task ID `shell: Test: Backend Flaky - Certificate DB Setup Ordering Loop` passes `50/50`, with no `no such table: ssl_certificates|proxy_hosts` in positive-path setup runs.
4. Regression gate: task ID `shell: Test: Backend Flaky - Certificate Handler Focused Regression` passes.
5. Playwright baseline gate is completed via task ID `shell: Test: E2E Playwright (FireFox) - Core: Certificates` (suite: `tests/core/certificates.spec.ts`), with task ID `shell: Docker: Rebuild E2E Environment` run first only when rebuild criteria are met.
6. Patch coverage preflight task ID `shell: Test: Local Patch Report` generates `test-results/local-patch-report.md` and `test-results/local-patch-report.json`.
7. Scope gate: no changes to `.gitignore`, `codecov.yml`, `.dockerignore`, or `Dockerfile` unless directly required to fix this flaky test.
8. Reproducibility gate: stress-loop outputs are artifactized under `test-results/flaky/` and retained in PR evidence (`cert-list-stability.jsonl`, `cert-list-race.jsonl`, `cert-db-setup-ordering.jsonl`, `cert-handler-regression.jsonl`).
9. If any validation fails, failure evidence and explicit follow-up tasks are recorded before completion.

---

### 8) Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Constructor behavior change impacts startup timing | medium | keep API stable; run targeted route/handler regression tests |
| Over-fixing spreads beyond certificate scope | medium | constrain edits to service + certificate tests only |
| DB setup-ordering fixes accidentally mask true migration problems | medium | fail fast during setup with explicit diagnostics and dedicated setup-ordering tests |
| Hidden race persists in adjacent tests | medium | run repeated/race-targeted suite; expand only if evidence requires |
| Out-of-scope config churn creates review noise | low | explicitly defer unrelated config hygiene from this PR |

---

### 9) Handoff

After this plan is approved:

1. Delegate execution to Supervisor/implementation agent with this spec as the source of truth.
2. Execute phases in order with validation gates between phases.
3. Keep PR scope narrow and deterministic; prefer single PR unless split triggers are hit.
  - Migration failures emit structured logs and increment migration-failure counters.
  - Legacy path execution attempts increment explicit counters and write warning/error logs.

## 10) Critical Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Legacy provider records fail conversion under Notify-only runtime | Notification send failures for affected providers | Transactional migration, explicit `migration_state`/`migration_error`, actionable errors, focused migration tests |
| Hidden fallback assumptions still exist in tests/code | False confidence and production drift | Sentinel-based tests that fail on fallback invocation; remove fallback wiring |
| Rollout outage without fallback safety net | Increased short-term operational risk | Smaller PR slices, strict preflight E2E/tests/scans, fast revert procedure |
| UI/API contract mismatch during canonicalization | Broken settings UX | Canonical endpoint tests across backend/frontend/E2E |
| Silent regression in legacy-path blocking instrumentation | Fallback attempts occur without operational visibility, delaying incident response | Add explicit legacy-attempt counters/logs to DoD, dashboard alert thresholds, and validation step that asserts non-zero logging on injected fallback-attempt test path |

## 11) Handoff

After approval, delegate execution to the `Supervisor` agent with this file as the single active plan baseline.
  - Migration failures emit structured logs and increment migration-failure counters.
  - Legacy path execution attempts increment explicit counters and write warning/error logs.

## 10) Critical Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Legacy provider records fail conversion under Notify-only runtime | Notification send failures for affected providers | Transactional migration, explicit `migration_state`/`migration_error`, actionable errors, focused migration tests |
| Hidden fallback assumptions still exist in tests/code | False confidence and production drift | Sentinel-based tests that fail on fallback invocation; remove fallback wiring |
| Rollout outage without fallback safety net | Increased short-term operational risk | Smaller PR slices, strict preflight E2E/tests/scans, fast revert procedure |
| UI/API contract mismatch during canonicalization | Broken settings UX | Canonical endpoint tests across backend/frontend/E2E |
| Silent regression in legacy-path blocking instrumentation | Fallback attempts occur without operational visibility, delaying incident response | Add explicit legacy-attempt counters/logs to DoD, dashboard alert thresholds, and validation step that asserts non-zero logging on injected fallback-attempt test path |

## 11) Handoff

After approval, delegate execution to the `Supervisor` agent with this file as the single active plan baseline.
