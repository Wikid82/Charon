# PR #666 Patch Coverage Recovery Spec (Approval-Ready)

Date: 2026-02-16
Owner: Planning Agent
Status: Draft for Supervisor approval (single coherent plan)

## 1) Scope Decision (Unified)

### In Scope
- Backend unit-test additions only, targeting changed patch lines in backend handlers/services/utils.
- Minimum-risk posture: prioritize test-only additions in files already touched by PR #666 before opening any new test surface.
- Coverage validation using current backend coverage task/script.

### Out of Scope
- E2E/Playwright, integration, frontend, Docker, and security scan remediation.

### E2E Decision for this task
- E2E is explicitly **out-of-scope** for this patch-coverage remediation.
- Rationale: target metric is Codecov patch lines on backend changes; E2E adds runtime risk/cycle time without direct line-level patch closure.

### Scope Reconciliation with Current Implementation (PR #666)
Confirmed already touched backend test files:
- `backend/internal/api/handlers/import_handler_test.go`
- `backend/internal/api/handlers/settings_handler_helpers_test.go`
- `backend/internal/api/handlers/emergency_handler_test.go`
- `backend/internal/services/backup_service_test.go`
- `backend/internal/api/handlers/backup_handler_test.go`
- `backend/internal/api/handlers/system_permissions_handler_test.go`
- `backend/internal/api/handlers/notification_provider_handler_validation_test.go`

Optional/deferred (not yet touched in current remediation pass):
- `backend/internal/util/permissions_test.go`
- `backend/internal/services/notification_service_json_test.go`
- `backend/internal/services/backup_service_rehydrate_test.go`
- `backend/internal/api/handlers/security_handler_coverage_test.go`

## 2) Single Source of Truth for Success

Authoritative success metric:
- **Codecov PR patch status (`lines`)** is the source of truth for this task.

Relationship to `codecov.yml`:
- `coverage.status.patch.default.target: 100%` and `required: false` means patch status is advisory in CI.
- For this plan, we set an internal quality gate: **patch lines >= 85%** (minimum), preferred **>= 87%** buffer.
- Local script output (`go tool cover`) remains diagnostic; pass/fail is decided by Codecov patch `lines` after upload.

## 3) Feasibility Math and Coverage-Line Budget

Given baseline:
- Patch coverage = `60.84011%`
- Missing patch lines = `578`

Derived totals:
- Let total patch lines = `T`
- `578 = T * (1 - 0.6084011)` => `T ≈ 1476`
- Currently covered lines `C0 = 1476 - 578 = 898`

Required for >=85%:
- `C85 = ceil(0.85 * 1476) = 1255`
- Additional covered lines required: `1255 - 898 = 357`

Budget by phase (line-coverage gain target):

| Phase | Target line gain | Cumulative gain target | Stop/Go threshold |
|---|---:|---:|---|
| Phase 1 | +220 | +220 | Stop if <+170; re-scope before Phase 2 |
| Phase 2 | +100 | +320 | Stop if <+70; activate residual plan |
| Phase 3 (residual closure) | +45 | +365 | Must reach >=+357 total |

Notes:
- Planned total gain `+365` gives `+8` lines safety over minimum `+357`.
- If patch denominator changes due to rebase/new touched lines, recompute budget before continuing.

## 4) Target Files/Functions (Concise, Specific)

Primary hotspots (Phase 1 focus, aligned to touched tests first):
- `backend/internal/api/handlers/system_permissions_handler.go`
  - `RepairPermissions`, `repairPath`, `normalizePath`, `pathHasSymlink`, `isWithinAllowlist`, `mapRepairErrorCode`
- `backend/internal/services/backup_service.go`
  - `RestoreBackup`, `extractDatabaseFromBackup`, `unzipWithSkip`, `RehydrateLiveDatabase`, `GetAvailableSpace`
- `backend/internal/api/handlers/settings_handler.go`
  - `UpdateSetting`, `PatchConfig`, `validateAdminWhitelist`, `syncAdminWhitelistWithDB`
- `backend/internal/api/handlers/import_handler.go`
  - `GetStatus`, `Upload`, `Commit`, `Cancel`, `safeJoin`
- `backend/internal/api/handlers/backup_handler.go`
  - `Restore`, `isSQLiteTransientRehydrateError`
- `backend/internal/api/handlers/emergency_handler.go`
  - `SecurityReset`, `disableAllSecurityModules`, `upsertSettingWithRetry`
- `backend/internal/api/handlers/notification_provider_handler.go`
  - `isProviderValidationError`, provider validation branches

Secondary hotspots (Phase 2 focus, optional/deferred expansion):
- `backend/internal/api/handlers/security_handler.go` (`GetStatus`, `latestConfigApplyState`)
- `backend/internal/util/permissions.go` (`CheckPathPermissions`, `MapSaveErrorCode`, `MapDiagnosticErrorCode`)
- `backend/internal/services/notification_service.go` (`sendJSONPayload`, `TestProvider`, `RenderTemplate`)

## 5) Execution Phases with Strict Stop/Go and De-Scoping Rules

### Phase 0 - Baseline Lock
Actions:
- Run `Test: Backend with Coverage` task (`.github/skills/scripts/skill-runner.sh test-backend-coverage`).
- Record baseline patch lines from Codecov PR view and local artifact `backend/coverage.txt`.

Go gate:
- Baseline captured and denominator confirmed.

Stop gate:
- If patch denominator changed by >5% from 1476, pause and recompute budgets before coding.

### Phase 1 - High-yield branch closure
Actions:
- Extend existing tests only in:
  - `backend/internal/api/handlers/system_permissions_handler_test.go`
  - `backend/internal/services/backup_service_test.go`
  - `backend/internal/api/handlers/backup_handler_test.go`
  - `backend/internal/api/handlers/emergency_handler_test.go`
  - `backend/internal/api/handlers/settings_handler_helpers_test.go`
  - `backend/internal/api/handlers/import_handler_test.go`
  - `backend/internal/api/handlers/notification_provider_handler_validation_test.go`

Go gate:
- Achieve >= `+170` covered patch lines and no failing backend tests.

Stop gate:
- If < `+170`, do not proceed; re-scope to only highest delta-per-test functions.

### Phase 2 - Secondary branch fill
Actions:
- Extend tests in:
  - `backend/internal/api/handlers/security_handler_coverage_test.go`
  - `backend/internal/util/permissions_test.go`
  - `backend/internal/services/backup_service_rehydrate_test.go`
  - `backend/internal/services/notification_service_json_test.go`

Go gate:
- Additional >= `+70` covered patch lines in this phase.

Stop gate:
- If < `+70`, skip low-yield areas and move directly to residual-line closure.

### Phase 3 - Residual-line closure (minimum-risk)
Actions:
- Work only uncovered/partial lines still shown in Codecov patch details.
- Add narrow table-driven tests to existing files; no new harness/framework.

Go gate:
- Reach total >= `+357` covered lines and patch >=85%.

Stop gate:
- If a residual branch requires production refactor, de-scope it and log as follow-up.

### Global de-scope rules (all phases)
- No production code changes unless a test proves a correctness bug.
- No new test framework, no integration/E2E expansion, no unrelated cleanup.
- No edits outside targeted backend test and directly related helper files.

## 6) Current Tasks/Scripts (Deprecated references removed)

Use these current commands/tasks only:
- Backend coverage (preferred): `Test: Backend with Coverage`
  - command: `.github/skills/scripts/skill-runner.sh test-backend-coverage`
- Equivalent direct script: `bash scripts/go-test-coverage.sh`
- Optional backend unit quick check: `Test: Backend Unit Tests`
  - command: `.github/skills/scripts/skill-runner.sh test-backend-unit`

Deprecated tasks are explicitly out-of-plan (for this work):
- `Security: CodeQL Go Scan (DEPRECATED)`
- `Security: CodeQL JS Scan (DEPRECATED)`

## 7) Residual Uncovered Lines Handling (Beyond hotspot table)

After each phase, run a residual triage loop:
1. Export remaining uncovered/partial patch lines from Codecov patch detail.
2. Classify each residual line into one of:
   - `validation/error mapping`
   - `permission/role guard`
   - `fallback/retry`
   - `low-value defensive log/telemetry`
3. Apply closure rule:
   - First three classes: add targeted tests in existing suite.
   - Last class: close only if deterministic and cheap; otherwise de-scope with rationale.
4. Maintain a residual ledger in the PR description:
   - line(s), owning function, planned test, status (`closed`/`de-scoped`), reason.

Exit condition:
- No unclassified residual lines remain.
- Any de-scoped residual lines have explicit follow-up items.

## 8) Acceptance Criteria (Unified)

1. One coherent plan only (this document), no conflicting statuses.
2. E2E explicitly out-of-scope for this patch-coverage task.
3. Success is measured by Codecov patch `lines`; local statement output is diagnostic only.
4. Feasibility math and phase budgets remain explicit and tracked against actual deltas.
5. All phase stop/go gates enforced; de-scope rules followed.
6. Only current tasks/scripts are referenced.
7. Residual uncovered lines are either closed with tests or formally de-scoped with follow-up.
8. Scope remains reconciled with touched files first; deferred files are only pulled in if phase gates require expansion.
