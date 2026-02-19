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
