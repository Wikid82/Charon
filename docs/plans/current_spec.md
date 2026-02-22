---
post_title: "Current Spec: Raise PR #729 Patch Coverage (Backend Targeted Tests)"
categories:
  - actions
  - testing
  - ci
  - security
tags:
  - go
  - patch-coverage
  - codecov
  - pr-729
  - unit-tests
summary: "Focused, test-first plan to raise PR #729 patch coverage by adding targeted backend tests for low-coverage changed files, prioritized by impact and implementation risk."
post_date: 2026-02-22
---

## Active Plan: Raise PR #729 Patch Coverage (Backend)

Date: 2026-02-22
Status: Active and authoritative
Scope Type: Backend patch-coverage lift via targeted tests
Authority: This is the only active authoritative plan section in this file.

## 1) Introduction

This plan targets PR #729 patch coverage gaps from
[docs/reports/codecove_patch_report.md](docs/reports/codecove_patch_report.md)
with a minimal, test-first approach.

Goal:

- Raise patch coverage by adding focused backend tests for changed files with
  low patch %, ordered by coverage impact first and implementation risk second.

Non-goals:

- No frontend changes.
- No broad refactors.
- No production code changes unless a branch is untestable without a tiny seam.

## 2) Research Findings

### 2.1 Patch report baseline

Current patch coverage snapshot from
[docs/reports/codecove_patch_report.md](docs/reports/codecove_patch_report.md):

1. `backend/internal/services/enhanced_security_notification_service.go`
   - 13.13% (285 missing + 6 partial)
2. `backend/internal/services/notification_service.go`
   - 75.45% (20 missing + 7 partial)
3. `backend/internal/api/handlers/notification_provider_handler.go`
   - 79.22% (15 missing + 1 partial)
4. `backend/internal/api/handlers/security_notifications.go`
   - 83.11% (10 missing + 3 partial)
5. `backend/internal/api/handlers/feature_flags_handler.go`
   - 71.42% (11 missing + 1 partial)
6. `backend/internal/services/proxyhost_service.go`
   - 33.33% (8 missing + 4 partial)
7. `backend/internal/api/routes/routes.go`
   - 75.00% (2 missing + 2 partial)
8. `backend/internal/cerberus/cerberus.go`
   - 73.33% (2 missing + 2 partial)
9. `backend/internal/notifications/router.go`
   - 88.23% (2 missing)

### 2.2 Existing test surface (reuse-first)

Existing package tests are strong and should be extended instead of introducing
new harness patterns:

- Services:
  - `backend/internal/services/notification_service_test.go`
  - `backend/internal/services/proxyhost_service_test.go`
  - `backend/internal/services/enhanced_security_notification_service_discord_only_test.go`
- Handlers:
  - `backend/internal/api/handlers/notification_provider_handler_test.go`
  - `backend/internal/api/handlers/notification_provider_patch_coverage_test.go`
  - `backend/internal/api/handlers/security_notifications_patch_coverage_test.go`
  - `backend/internal/api/handlers/feature_flags_handler_coverage_test.go`
  - `backend/internal/api/handlers/feature_flags_blocker3_test.go`
- Routes:
  - `backend/internal/api/routes/routes_coverage_test.go`
- Cerberus:
  - `backend/internal/cerberus/cerberus_test.go`
- Notifications router:
  - `backend/internal/notifications/router_test.go`

### 2.3 Constraints

- Test-first and minimal: backend tests only by default.
- Tiny production seam allowed only for truly unreachable branches.
- Keep branch behavior assertions deterministic and table-driven where possible.

## 3) Requirements (EARS)

- R1: WHEN addressing PR #729 coverage gaps, THE SYSTEM SHALL prioritize files
  by missing patch lines first, then by implementation risk.
- R2: WHEN adding coverage, THE SYSTEM SHALL modify or add backend test files
  only, unless a branch is untestable without a tiny seam.
- R3: IF a tiny seam is required, THEN THE SYSTEM SHALL keep it package-local,
  minimal, and solely to unlock deterministic branch testing.
- R4: WHEN tests are implemented, THE SYSTEM SHALL validate with local patch
  report preflight, targeted tests, backend coverage script, and patch report rerun.
- R5: WHEN planning delivery, THE SYSTEM SHALL use a single PR unless scope or
  rollback risk materially increases.
- R6: WHEN plan is finalized, THE SYSTEM SHALL explicitly review
  `.gitignore`, `.dockerignore`, `codecov.yml`, and `Dockerfile` for required
  updates.

## 4) Technical Specification

### 4.1 Priority model

Priority score = Coverage impact (missing+partial lines, weighted high) +
implementation risk (weighted low).

| Priority | File | Coverage Gap | Risk | Strategy |
|---|---|---:|---|---|
| P0 | `backend/internal/services/enhanced_security_notification_service.go` | 285 + 6 | Medium | New focused coverage file + extend existing Discord-only tests |
| P1 | `backend/internal/services/notification_service.go` | 20 + 7 | Medium | Extend existing comprehensive service tests |
| P2 | `backend/internal/api/handlers/notification_provider_handler.go` | 15 + 1 | Low | Extend handler patch tests for admin/error branches |
| P2 | `backend/internal/api/handlers/feature_flags_handler.go` | 11 + 1 | Low | Extend coverage tests for env/db parse and transaction branches |
| P2 | `backend/internal/api/handlers/security_notifications.go` | 10 + 3 | Low | Extend patch tests for source-validation and dispatch-failure branches |
| P3 | `backend/internal/services/proxyhost_service.go` | 8 + 4 | Low | Extend validation + advanced config edge branches |
| P4 | `backend/internal/api/routes/routes.go` | 2 + 2 | Medium | Extend route wiring error/non-fatal paths only |
| P4 | `backend/internal/cerberus/cerberus.go` | 2 + 2 | Low | Extend IsEnabled/cache and fail-closed dispatch edges |
| P5 | `backend/internal/notifications/router.go` | 2 | Low | Add table cases for missing flags/default false |

### 4.2 Exact target test files to modify/create

Primary target files:

1. `backend/internal/services/enhanced_security_notification_service_discord_only_test.go` (modify)
2. `backend/internal/services/enhanced_security_notification_service_coverage_test.go` (create)
3. `backend/internal/services/notification_service_test.go` (modify)
4. `backend/internal/api/handlers/notification_provider_patch_coverage_test.go` (modify)
5. `backend/internal/api/handlers/notification_provider_handler_test.go` (modify, only if needed)
6. `backend/internal/api/handlers/security_notifications_patch_coverage_test.go` (modify)
7. `backend/internal/api/handlers/feature_flags_handler_coverage_test.go` (modify)
8. `backend/internal/services/proxyhost_service_test.go` (modify)
9. `backend/internal/api/routes/routes_coverage_test.go` (modify)
10. `backend/internal/cerberus/cerberus_test.go` (modify)
11. `backend/internal/notifications/router_test.go` (modify)

### 4.3 Branch targets per file

#### P0: enhanced security notification service

- `GetSettings` feature-flag off/on branch behavior.
- Managed provider aggregation permutations (0/1/many managed destinations).
- Gotify incomplete payload rejection path.
- Idempotent `updateManagedProviders` (no save when unchanged).
- Migration marker checksum no-op and disabled-flag read-only migration behavior.
- Feature defaulting matrix (`CHARON_ENV`, `GIN_MODE`, `_test_mode_marker`).
- `sendWebhook` SSRF rejection + non-2xx handling.

#### P1: notification service

- Event filtering for security event variants (`security_waf`, `security_acl`,
  `security_rate_limit`, `security_crowdsec`).
- Discord URL validation error branches and non-discord fail-closed paths.
- `UpdateProvider` mutation guards (deprecated provider type/enable cases).
- Template size/timeouts and payload validation branches where uncovered.

#### P2: handler targets

- Notification provider handler:
  - `Create` and `Update` blocked-code branches (discord-only, immutable legacy,
    cannot enable deprecated).
  - `isProviderValidationError` classification branches.
- Security notifications handler:
  - Invalid source IP parsing, unauthorized source path, malformed payload, and
    accepted path with dispatch error non-fatal behavior.
- Feature flags handler:
  - legacy fallback hard-false resolution branches, invalid env value warning
    path coverage, and unknown key ignore behavior in updates.

#### P3-P5: lower-line but necessary gaps

- Proxy host service: malformed forward host/path stripping and advanced config
  normalization failure branches.
- Routes: migration invocation/error paths and non-fatal startup behavior.
- Cerberus: feature-flag absent/false fail-closed dispatch and cache refresh
  branch coverage.
- Notifications router: explicit default false behavior for missing flags.

### 4.4 Tiny seam policy (only if required)

Allowed only when a specific branch cannot be reached with current test hooks.
Potential seam examples (if needed):

- Package-level function var for ticker/sleep in route boot goroutines.
- Package-level clock helper for time-dependent branch determinism.

If introduced:

- Must be private to package.
- Must not alter runtime behavior.
- Must be documented in test file rationale and removed if later unnecessary.

## 5) Implementation Plan

### Phase 1: Baseline and preflight

1. Run local patch report preflight:
   - `bash scripts/local-patch-report.sh`
2. Confirm artifacts:
   - `test-results/local-patch-report.md`
   - `test-results/local-patch-report.json`
3. Capture baseline patch gaps for the 9 target files.

### Phase 2: Highest-impact tests first (P0-P1)

1. Implement `enhanced_security_notification_service` branch tests.
2. Implement `notification_service` gap tests.
3. Run targeted service tests only:
   - `go test ./backend/internal/services -run 'Test(DiscordOnly|EnhancedSecurity|NotificationService|ProxyHostService)' -count=1`

### Phase 3: Handler and service gap closure (P2-P3)

1. Extend handler coverage tests for notification provider, security
   notifications, and feature flags.
2. Extend proxy host service tests for uncovered validation/normalization
   branches.
3. Run targeted handler/service tests:
   - `go test ./backend/internal/api/handlers -run 'Test(NotificationProvider|SecurityNotification|FeatureFlags)' -count=1`
   - `go test ./backend/internal/services -run 'TestProxyHostService' -count=1`

### Phase 4: Wiring and security/router edges (P4-P5)

1. Extend route, cerberus, and notifications router tests for remaining small
   branch gaps.
2. Run targeted package tests:
   - `go test ./backend/internal/api/routes -run 'TestRegister' -count=1`
   - `go test ./backend/internal/cerberus -run 'TestCerberus' -count=1`
   - `go test ./backend/internal/notifications -run 'TestRouter' -count=1`

### Phase 5: Coverage validation and rerun

1. Run backend coverage script:
   - `scripts/go-test-coverage.sh`
2. Re-run local patch report:
   - `bash scripts/local-patch-report.sh`
3. Verify patch coverage increase and remaining uncovered lines are justified or
   queued.

## 6) Validation Workflow (Required Order)

| Step | Command | Expected Result |
|---|---|---|
| 1 | `npx playwright test --project=firefox` | E2E-first compliance satisfied before unit/coverage work |
| 2 | `bash scripts/local-patch-report.sh` | Local patch preflight artifacts generated (`.md` + `.json`) |
| 3 | `go test` targeted packages/files (phased above) | Targeted unit validation passes and intended branches are exercised |
| 4 | `scripts/go-test-coverage.sh` | Targeted coverage validation passes backend coverage gate |
| 5 | `bash scripts/local-patch-report.sh` | Patch % improves for PR #729 target files |

## 7) PR Slicing Strategy

Decision: **Single PR**.

Rationale:

- All work is backend test-only and tightly coupled to one outcome (patch
  coverage increase for PR #729).
- Review remains tractable because changes are limited to test files.
- Rollback is low risk (test-only diff).

Contingency trigger for multi-PR split (only if needed):

- If tiny production seams become necessary in multiple packages, split into:
  - PR-1: minimal seam(s) + seam tests.
  - PR-2: coverage tests consuming seam(s).

## 8) Risk Guardrail (P0 Blast Radius)

Timebox / fallback trigger:

- If, after completing P0 + P1 and running `bash scripts/local-patch-report.sh`,
  either of the following is true:
  - overall patch coverage is still `< 80%`, or
  - combined uplift from baseline is `< +8 percentage points`,
  THEN stop further broad branch expansion and switch to fallback mode.

Fallback decision path (next-step actions):

1. Freeze any additional new-branch test expansion in `P2+` files.
2. Generate and attach latest artifacts:
   - `test-results/local-patch-report.md`
   - `test-results/local-patch-report.json`
3. Run targeted gap triage on top remaining uncovered changed lines only.
4. Choose one path:
   - Path A (default): add one minimal seam in highest-impact file and finish in
     same PR.
   - Path B: split into follow-up PR for seam + residual coverage if seam scope
     exceeds one package or introduces rollback risk.
5. Re-run `bash scripts/local-patch-report.sh` and proceed only if targets in
   Section 10 are met.

## 9) Config File Review Outcome

Reviewed for this plan:

- `.gitignore`
- `.dockerignore`
- `codecov.yml`
- `Dockerfile`

Planned updates: **None** (test-only backend changes do not require updates).

## 10) Acceptance Criteria (Measurable)

Overall patch-coverage target for this PR uplift effort:

- AC1: Overall PR patch coverage (Codecov patch view) reaches `>= 85%`.

Per-file patch coverage targets (from current 9-file backend target list):

- AC2: `backend/internal/services/enhanced_security_notification_service.go` `>= 60%`
- AC3: `backend/internal/services/notification_service.go` `>= 90%`
- AC4: `backend/internal/api/handlers/notification_provider_handler.go` `>= 92%`
- AC5: `backend/internal/api/handlers/security_notifications.go` `>= 95%`
- AC6: `backend/internal/api/handlers/feature_flags_handler.go` `>= 90%`
- AC7: `backend/internal/services/proxyhost_service.go` `>= 75%`
- AC8: `backend/internal/api/routes/routes.go` `>= 90%`
- AC9: `backend/internal/cerberus/cerberus.go` `>= 90%`
- AC10: `backend/internal/notifications/router.go` `>= 95%`

Artifact-gated pass condition:

- AC11: `bash scripts/local-patch-report.sh` must generate both artifacts:
  - `test-results/local-patch-report.md`
  - `test-results/local-patch-report.json`
- AC12: The generated artifacts must show overall target (AC1) and all per-file
  targets (AC2-AC10) as met for this uplift effort.

Process and scope checks:

- AC13: Plan remains backend test-focused with no unrelated production changes.
- AC14: `.gitignore`, `.dockerignore`, `codecov.yml`, and `Dockerfile` review
  remains recorded with no required updates for this scope.
