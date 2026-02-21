---
post_title: "Current Spec: Discord Notify-Only Dispatch (No Legacy Fallback)"
categories:
  - actions
  - testing
  - security
tags:
  - discord
  - notifications
  - notify-rollout
  - migration
  - e2e
summary: "Authoritative implementation plan for Discord Notify-only dispatch with explicit retirement of legacy fallback ambiguity, deterministic migration behavior, and proof-oriented testing."
post_date: 2026-02-21
---

## Active Plan: Discord Notify-Only Dispatch (No Legacy Fallback)

Date: 2026-02-21
Status: Active and authoritative
Scope Type: Product implementation plan (frontend + backend + migration + verification)

## 1) Introduction

This plan defines the implementation to satisfy the updated requirement direction:

- Do not rely on legacy fallback engine for Discord dispatch.
- Discord payloads saved in DB must be sent by Notify engine directly.
- Remove ambiguity where delivery success could still come through legacy path.
- Keep rollout scope focused on Discord-only cutover without unrelated refactors.

This plan is intentionally scoped to Discord-only dispatch/runtime behavior, fallback retirement, and auditable proof that no hidden legacy path remains.

## 2) Research Findings

### 2.1 Frontend provider-type control points (exact files)

- `frontend/src/pages/Notifications.tsx`
  - Provider type is normalized to Discord (`DISCORD_PROVIDER_TYPE`) before submit/test/preview.
  - Provider dropdown currently renders Discord-only option, and request payloads are forced to Discord.
- `frontend/src/api/notifications.ts`
  - `withDiscordType(...)` normalizes outbound payload type to Discord.
  - Interface remains `type: string`, requiring backend to remain authoritative for no-fallback invariants.
- `frontend/src/pages/__tests__/Notifications.test.tsx`
  - Must verify Discord-only create/edit/test behavior and avoid implying fallback rescue semantics.
- `tests/settings/notifications.spec.ts`
  - Includes coverage for Discord behavior and deprecated non-Discord rendering semantics.

### 2.2 Backend/provider type validation/default/control points (exact files)

- `backend/internal/api/handlers/notification_provider_handler.go`
  - Enforces Discord-only on create/update and blocks enabling deprecated non-Discord providers.
- `backend/internal/services/notification_service.go`
  - Enforces Discord-only for provider lifecycle/test and skips non-Discord dispatch.
  - Contains explicit legacy-fallback-disabled sentinel/error path that must stay fail-closed.
  - Includes `EnsureNotifyOnlyProviderMigration(...)` for deterministic cutover state.
- `backend/internal/services/enhanced_security_notification_service.go`
  - `SendViaProviders(...)` dispatches only to Discord providers in current rollout stage.
  - `dispatchToProvider(...)` still has non-Discord branches and is a key ambiguity-removal target.
- `backend/internal/models/notification_provider.go`
  - `Type` comment still lists non-Discord types.
  - `Engine` and URL comments still include retired legacy-notifier URL semantics.
- `backend/internal/notifications/engine.go`
  - Defines a retired legacy notification engine constant.
- `backend/internal/notifications/router.go`
  - Legacy engine branch remains.
- `backend/internal/notifications/feature_flags.go`
  - Retired fallback flag key remains in feature-flag wiring.
- `backend/internal/api/handlers/feature_flags_handler.go`
  - Still exposes a retired legacy fallback key in defaults and update path guards.
- `backend/internal/api/routes/routes.go`
  - Notification services are wired via both `NotificationService` and
    `EnhancedSecurityNotificationService`; migration path still references transitional behavior.

### 2.3 Exact backend/frontend files impacted by fallback removal

Backend files:

- `backend/internal/notifications/engine.go`
- `backend/internal/notifications/router.go`
- `backend/internal/notifications/feature_flags.go`
- `backend/internal/api/handlers/feature_flags_handler.go`
- `backend/internal/services/notification_service.go`
- `backend/internal/services/enhanced_security_notification_service.go`
- `backend/internal/api/handlers/notification_provider_handler.go`
- `backend/internal/api/routes/routes.go`
- `backend/internal/models/notification_provider.go`

Frontend files:

- `frontend/src/pages/Notifications.tsx`
- `frontend/src/api/notifications.ts`

### 2.4 Dependency/runtime current state (legacy notifier)

- `backend/go.mod`: no active legacy notifier dependency.
- `backend/go.sum`: no active legacy notifier entry found.
- Direct import/call in backend source currently not present.
- Legacy strings/constants/tests remain in runtime code paths and tests.
- Historical artifacts and cached module content may contain legacy notifier references but are not
  production dependencies.

### 2.5 Firefox failing specs context

- Artifact `playwright-output/manual-dns-targeted/.last-run.json` contains 16 failed test IDs only; suite attribution must come from Playwright report/traces, not this file alone.
- Separate summary `FIREFOX_E2E_FIXES_SUMMARY.md` references prior Firefox remediation work.
- User-reported “5 failing Firefox specs” is treated as active triage input and must be
  classified at execution time into in-scope vs out-of-scope suites (defined below).

## 3) Requirements (EARS)

- R1: WHEN opening notification provider create/edit UI, THE SYSTEM SHALL offer only `discord`
  in provider type selection for this rollout.
- R2: WHEN create/update/test notification provider API endpoints receive a non-Discord type,
  THE SYSTEM SHALL reject with deterministic validation error.
- R3: WHILE legacy non-Discord providers exist in DB, THE SYSTEM SHALL handle them per explicit
  compatibility policy without enabling dispatch.
- R4: WHEN verifying runtime and dependency state, THE SYSTEM SHALL produce auditable evidence
  that the retired legacy notifier is not installed, linked, or used by active runtime code paths.
- R5: IF additional services are introduced in future, THEN THE SYSTEM SHALL enable them
  behind explicit validation gates and staged rollout controls.
- R6: WHEN Discord notifications are delivered successfully, THE SYSTEM SHALL provide evidence that Notify path handled dispatch and not a legacy fallback path.
- R7: IF rollback is required, THEN THE SYSTEM SHALL use explicit rollback controls without reintroducing hidden legacy fallback behavior.

## 4) Technical Specification

### 4.1 Discord-only enforcement strategy (defense in depth)

#### Layer A: UI/input restriction

- `frontend/src/pages/Notifications.tsx`
  - Replace provider type dropdown options with a single option: `discord`.
  - Keep visible label semantics intact for accessibility.
  - Prevent stale-form values from preserving non-Discord types in edit mode.

#### Layer B: Frontend request contract hardening

- `frontend/src/api/notifications.ts`
  - Frontend request contract SHALL submit `type` as `discord` only; client must reject or normalize any stale non-discord value before submit.

#### Layer C: API handler validation (authoritative gate)

- `backend/internal/api/handlers/notification_provider_handler.go`
  - Enforce `req.Type == "discord"` for all create/update paths (not only security-event cases).
  - Return stable error code/message for non-Discord type attempts.

#### Layer D: Service-layer invariant

- `backend/internal/services/notification_service.go`
  - Add service-level type guard for create/update/test/send paths to reject non-Discord.
  - Keep this guard even if handler validation exists (defense in depth).

#### Layer E: Dispatch/runtime invariant

- `backend/internal/services/enhanced_security_notification_service.go`
  - Maintain dispatch as Discord-only.
  - Remove non-Discord dispatch branches from active runtime logic for this phase.

### 4.2 Feature-flag policy change for legacy fallback

Policy for `feature.notifications.legacy.fallback_enabled`:

- Deprecate now.
- Force false in reads and writes.
- Immutable false: never true at runtime.
- Reject `true` update attempts with deterministic API error.
- Keep compatibility aliases read-only and forced false during transition.
- Remove `feature.notifications.legacy.fallback_enabled` from all public/default flag responses in PR-3 of this plan, and remove all retired env aliases in the same PR. No compatibility-window extension is allowed without a new approved spec revision.

Migration implications:

- Existing settings row for legacy fallback is normalized to false if present.
- Transition period allows key visibility as false-only for compatibility.
- Final state keeps this key out of runtime dispatch decisions; there is no fallback runtime path.

Legacy fallback control surface policy: any API write attempting to set legacy fallback true SHALL return `400` with code `LEGACY_FALLBACK_REMOVED`; any legacy fallback read endpoint/field retained temporarily SHALL return hard-false only and SHALL be removed in PR-3. No hidden route or config path may alter dispatch away from Notify, and Notify failures SHALL NOT reroute to a fallback engine.

### 4.3 Runtime dispatch contract (Discord-only Notify path)

Contract:

- Discord dispatch source of truth is Notify engine path (`notify_v1`) only.
- `NotificationService.SendExternal(...)` and `EnhancedSecurityNotificationService.SendViaProviders(...)` must fail closed for non-Discord delivery attempts.
- No implicit retry/reroute to legacy engine on Notify failure.
- Any successful Discord delivery during rollout must be attributable to Notify path execution.

#### Layer F: Optional DB invariant (recommended)

- Add migration with DB-level check constraint for notification provider type to `discord`
  for newly created/updated rows in this rollout.
  - If DB constraint is deferred, service/API guards remain mandatory.

### 4.4 Data migration and compatibility policy for existing non-Discord providers

Policy decision for this rollout: **Deprecate + read-only visibility + non-dispatch**.

- Existing non-Discord rows are preserved for audit/history but cannot be newly created
  or converted back to active dispatch in this phase.
- Compatibility behavior:
  - `enabled` forced false for non-Discord providers during migration reconciliation.
  - `migration_state` remains failed/deprecated for non-supported providers.
  - UI list behavior: render non-Discord rows as deprecated read-only rows with a clear non-dispatch badge.
- API contract: non-Discord existing rows are always non-dispatch, non-enable, deletable, and return deterministic validation error on enable/type mutation attempts.
- Explicitly block enable and type mutation attempts for non-Discord rows via API with deterministic validation errors.
- Allow deletion of deprecated rows.

Migration safety requirements:

- Migration SHALL be idempotent.
- Migration SHALL execute within an explicit transaction boundary.
- Migration SHALL require a pre-migration backup before mutating provider rows.
- Migration SHALL define and document a rollback procedure.
- Migration SHALL persist audit log fields for every mutated provider row (including provider identifier, previous values, new values, mutation timestamp, and operation identifier).

Migration implementation candidates:

- `backend/internal/services/notification_service.go` (`EnsureNotifyOnlyProviderMigration`)
- `backend/internal/services/enhanced_security_notification_service.go`
- new migration file under backend migration path if schema/data migration is introduced.

### 4.5 Legacy notifier retirement/removal scope

Targets to remove from active runtime/config surface:

- `backend/internal/notifications/engine.go` legacy engine constant
- `backend/internal/notifications/router.go` legacy engine branch
- `backend/internal/notifications/feature_flags.go` legacy flag constant
- `backend/internal/api/handlers/feature_flags_handler.go` retired legacy flag exposure
- `backend/internal/models/notification_provider.go` retired legacy notifier comments/engine semantics
- `backend/internal/services/notification_service.go` legacy naming/comments/hooks
- `backend/internal/services/enhanced_security_notification_service.go` non-Discord dispatch branches in `dispatchToProvider`
- `frontend/src/pages/Notifications.tsx` and `frontend/src/api/notifications.ts` to keep Discord-only UX/request contract aligned with backend fallback retirement

Out of scope for blocking completion:

- Archived docs/history artifacts under `docs/plans/archive/**` and similar archive/report folders.

## 5) Dependency and Runtime Audit Plan (verify Discord-only runtime)

Run and record all commands in implementation evidence.

Task-aligned proof gates (mandatory):

- Run task `Security: Verify SBOM` and capture output evidence.
- Run task `Security: Scan Docker Image (Local)` and capture output evidence.
- Store reproducibility evidence under `test-results/notify-rollout-audit/` (task outputs, command transcripts, and generated reports).

### 5.1 Dependency manifests

- `cd /projects/Charon/backend && go mod graph | grep -i "legacy_shoutrrr|feature.notifications.legacy.fallback_enabled|EngineLegacy|legacySendFunc|shoutrrr" || true`
- `grep -i "legacy_shoutrrr|feature.notifications.legacy.fallback_enabled|EngineLegacy|legacySendFunc|shoutrrr" /projects/Charon/backend/go.mod /projects/Charon/backend/go.sum || true`
- `grep -i "legacy_shoutrrr|feature.notifications.legacy.fallback_enabled|EngineLegacy|legacySendFunc|shoutrrr" /projects/Charon/package-lock.json /projects/Charon/frontend/package-lock.json 2>/dev/null || true`

### 5.2 Source/runtime references (non-archive)

- `grep -RIn --exclude-dir=.git --exclude-dir=.cache --exclude-dir=docs --exclude-dir=playwright-output --exclude-dir=test-results "legacy_shoutrrr|feature.notifications.legacy.fallback_enabled|EngineLegacy|legacySendFunc|shoutrrr" /projects/Charon/backend /projects/Charon/frontend || true`

### 5.3 Built artifact/container runtime checks

- Build image: `docker build -t charon:discord-only-audit /projects/Charon`
- Module metadata in binary:
  - `docker run --rm charon:discord-only-audit sh -lc 'go version -m /app/charon 2>/dev/null | grep -i "legacy_shoutrrr|feature.notifications.legacy.fallback_enabled|EngineLegacy|legacySendFunc|shoutrrr" || true'`
- Runtime filesystem/strings scan:
  - `docker run --rm charon:discord-only-audit sh -lc 'find /app /usr/local/bin -maxdepth 4 -type f | xargs grep -Iin "legacy_shoutrrr|feature.notifications.legacy.fallback_enabled|EngineLegacy|legacySendFunc|shoutrrr" 2>/dev/null || true'`

### 5.4 Documentation references (active docs only)

- `grep -RIn "legacy_shoutrrr|feature.notifications.legacy.fallback_enabled|EngineLegacy|legacySendFunc|shoutrrr" /projects/Charon/README.md /projects/Charon/ARCHITECTURE.md /projects/Charon/docs/features /projects/Charon/docs/security* || true`

Audit pass criteria:

- No active dependency references.
- No active runtime/path references.
- No active user-facing docs claiming retired multi-service notifier behavior for this rollout.

## 6) Implementation Plan (phased)

### Phase 1: Docker E2E rebuild decision and environment prep

- Apply repo testing-rule decision for E2E container rebuild based on changed paths.
- Rebuild/start E2E environment when required before any test execution.

### Phase 2: E2E first and failing-spec triage baseline

- Capture exact Firefox failures for current branch:
  - `cd /projects/Charon && npx playwright test --project=firefox`
- Classify failing specs:
  - **In-scope**: notification provider tests (`tests/settings/notifications.spec.ts` and related).
  - **Out-of-scope**: manual DNS provider suites unless directly regressed by this change.
- If user-reported 5 failing specs are not notifications, create/attach separate tracking item and
  do not block Discord-only implementation on unrelated suite failures.

### Phase 3: Local Patch Report preflight (mandatory before unit/coverage)

- Run local patch coverage preflight and generate required artifacts:
  - `test-results/local-patch-report.md`
  - `test-results/local-patch-report.json`

### Phase 4: Backend hard gate (authoritative)

- Implement Discord-only validation in API handlers and service layer.
- Remove/retire active legacy engine and flag exposure from runtime path.
- Apply migration policy for existing non-Discord rows.

### Phase 5: Conditional GORM security check gate

- Run GORM security scanner in check mode when backend model/database trigger paths are modified.
- Gate completion on zero CRITICAL/HIGH findings for triggered changes.

### Phase 6: Frontend Discord-only UX

- Restrict dropdown/options to Discord only.
- Align frontend type contract and submission behavior.
- Ensure edit/create forms cannot persist stale non-Discord values.

### Phase 7: Legacy notifier retirement cleanup + audit evidence

- Remove active runtime legacy references listed in Section 4.5.
- Execute dependency/runtime/doc audit commands in Section 5 and save evidence.

### Phase 8: Validation and release readiness

- Run validation sequence in required order (Section 7).
- Confirm rollout note: additional providers remain disabled pending per-provider validation PRs.

## 7) Targeted Testing Strategy

Mandatory repo QA/test order for this plan:

1. Docker E2E rebuild decision and required rebuild/start.
2. E2E first (Firefox baseline + targeted notification suite).
3. Local Patch Report preflight artifact generation.
4. Conditional GORM security check gate (`--check` semantics when trigger paths change).
5. CodeQL CI-aligned scans (Go and JS).
6. Coverage gates (backend/frontend coverage thresholds and patch coverage review).
7. Frontend type-check and backend/frontend build verification.

### 7.1 Backend tests (in scope)

- `backend/internal/api/handlers/notification_provider_blocker3_test.go`
  - Expand from security-event-only gating to global Discord-only gating.
- `backend/internal/services/notification_service_test.go`
  - Ensure non-Discord create/update/test/send rejection.
- `backend/internal/notifications/router_test.go`
  - Prove legacy fallback decision remains disabled and cannot be toggled into active use.
- `backend/internal/services/notification_service_discord_only_test.go`
  - Prove Discord-only dispatch behavior and no successful fallback path.
- `backend/internal/services/security_notification_service_test.go`
  - Prove security notifications route through Discord-only provider dispatch in rollout mode.
- Add/adjust tests for migration policy behavior on existing non-Discord rows.

### 7.1.1 Notify-vs-fallback evidence points (required)

- Unit proof SHALL assert legacy dispatch hook call-count is exactly 0 for all Discord-success and Discord-failure scenarios.
- Unit/integration proof SHALL assert Notify dispatch hook/request path is called for Discord deliveries (including `Content-Type: application/json` and deterministic provider-type guard behavior).
- Negative proof SHALL assert non-Discord create/update/test attempts fail with stable error codes and cannot produce a successful delivery event.
- CI gate SHALL fail if any test demonstrates delivery success through a legacy fallback path.

### 7.2 Frontend unit tests (in scope)

- `frontend/src/pages/__tests__/Notifications.test.tsx`
  - Assert only Discord option exists in provider-type select.
  - Remove/adjust non-Discord create assumptions.

### 7.3 E2E tests (in scope)

- `tests/settings/notifications.spec.ts`
  - Keep Discord create/edit/test flows.
  - Replace or retire non-Discord CRUD expectations for this phase.

### 7.4 Firefox failing specs handling (required context)

- If the current failing 5 specs are notification-related, they are in scope and must be fixed
  within this PR.
- If they are unrelated (for example manual-dns-provider failures), track separately and avoid
  scope creep; do not mark as fixed under this provider-type change.

### 7.5 Suggested verification command set

- `cd /projects/Charon && npx playwright test --project=firefox tests/settings/notifications.spec.ts`
- `cd /projects/Charon && npx playwright test --project=firefox --grep "Notification Providers"`
- `cd /projects/Charon && bash scripts/local-patch-report.sh`
- `cd /projects/Charon && ./scripts/scan-gorm-security.sh --check` (conditional on trigger paths)
- `cd /projects/Charon && pre-commit run codeql-go-scan --all-files`
- `cd /projects/Charon && pre-commit run codeql-js-scan --all-files`
- `cd /projects/Charon && go test ./backend/internal/api/handlers -run Blocker3 -v`
- `cd /projects/Charon && go test ./backend/internal/services -run Notification -v`
- `cd /projects/Charon && scripts/go-test-coverage.sh`
- `cd /projects/Charon/frontend && npm test -- Notifications`
- `cd /projects/Charon && scripts/frontend-test-coverage.sh`
- `cd /projects/Charon/frontend && npm run type-check && npm run build`
- `cd /projects/Charon/backend && go build ./...`

## 8) Review of `.gitignore`, `.dockerignore`, `codecov.yml`, `Dockerfile`

### Findings and required updates decision

- `.gitignore`
  - No mandatory change required for feature behavior.
  - Optional: ensure any new audit evidence artifacts (if added) are either committed intentionally
    or ignored consistently.
- `.dockerignore`
  - No mandatory change required for Discord-only behavior.
  - If adding migration/audit helper scripts needed at build time, ensure they are not excluded.
- `codecov.yml`
  - No mandatory change required.
  - If new test files are added, verify they are not accidentally ignored by broad patterns.
- `Dockerfile`
  - No direct package-level retired notifier dependency is declared.
  - No mandatory Dockerfile change for Discord-only behavior unless runtime audit shows residual
    retired notifier-linked binaries or references.

## 9) PR Slicing Strategy

Decision: **Multiple PRs (3 slices)** for safer rollout and easier rollback.

### PR-1: Backend guardrails + migration policy

- Scope:
  - API/service Discord-only validation.
  - Existing non-Discord compatibility policy implementation.
  - Legacy flag/engine runtime retirement in backend.
- Primary files:
  - `backend/internal/api/handlers/notification_provider_handler.go`
  - `backend/internal/services/notification_service.go`
  - `backend/internal/services/enhanced_security_notification_service.go`
  - `backend/internal/notifications/engine.go`
  - `backend/internal/notifications/router.go`
  - `backend/internal/notifications/feature_flags.go`
  - `backend/internal/api/handlers/feature_flags_handler.go`
  - `backend/internal/models/notification_provider.go`
- Gate:
  - Backend tests pass, API rejects non-Discord, migration behavior verified.
- Rollback:
  - Revert PR-1 if existing-provider compatibility regresses.
  - Do not reintroduce fallback by setting legacy fallback flag true or restoring legacy dispatch routing as a hotfix.

### PR-2: Frontend Discord-only UX and tests

- Scope:
  - Dropdown/UI restriction to Discord only.
  - Frontend test updates for new provider-type policy.
- Primary files:
  - `frontend/src/pages/Notifications.tsx`
  - `frontend/src/api/notifications.ts`
  - `frontend/src/pages/__tests__/Notifications.test.tsx`
  - `tests/settings/notifications.spec.ts`
- Gate:
  - Dependency: PR-2 entry is blocked until PR-1 is merged.
  - Firefox notifications E2E targeted suite passes.
  - Exit: frontend request contract submits `type=discord` only and backend rejects non-Discord mutations.
- Rollback:
  - Revert PR-2 without reverting backend safeguards.
  - No rollback action may re-enable hidden fallback behavior.

### PR-3: Legacy notifier retirement evidence + docs alignment

- Scope:
  - Execute audit commands, capture evidence, update active docs if needed.
- Primary files:
  - `README.md`, `ARCHITECTURE.md`, relevant `docs/features/**` and `docs/security*` files (only if active references remain).
- Gate:
  - Dependency: PR-3 entry is blocked until PR-2 is merged.
  - Entry blocker: runtime/dependency audit must pass before PR-3 can proceed.
  - Dependency/runtime/doc audit pass criteria met.
  - Exit: docs must reflect final backend/frontend state after PR-1 and PR-2 behavior.
- Rollback:
  - Revert docs-only evidence changes without impacting runtime behavior.
  - Preserve explicit no-fallback runtime guarantees from earlier slices.

## 10) Rollback Strategy (No Hidden Fallback Ambiguity)

Allowed rollback mechanisms:

1. Revert entire rollout slice(s) to previous known-good commit/tag.
2. Pause notifications by disabling delivery globally while keeping legacy fallback permanently disabled (`feature.notifications.legacy.fallback_enabled=false`, immutable). Rollback SHALL revert to a prior release tag/DB snapshot only; it SHALL NOT re-enable or simulate legacy fallback.
3. Restore DB snapshot if migration state must be reverted.

Disallowed rollback mechanisms:

- Re-enable `feature.notifications.legacy.fallback_enabled` for runtime dispatch.
- Reintroduce runtime `EngineLegacy` or implicit fallback retry paths.
- Apply emergency patches that create dual-path ambiguity for Discord success attribution.

## 11) Risks and Mitigations

- Risk: Existing non-Discord providers silently break user expectations.
  - Mitigation: explicit deprecated/read-only policy and clear API errors.
- Risk: Hidden runtime legacy paths remain while UI appears Discord-only.
  - Mitigation: backend-first enforcement and runtime audit commands.
- Risk: Scope creep from unrelated Firefox failures.
  - Mitigation: strict in-scope/out-of-scope classification in Phase 1.
- Risk: Future provider rollout pressure bypasses validation discipline.
  - Mitigation: staged enablement policy with per-provider validation gate.

## 12) Acceptance Criteria

1. Provider type dropdown in notifications UI exposes only Discord.
2. API create/update/test rejects any non-Discord provider type with deterministic error.
3. Service/runtime dispatch is Discord-only for this rollout and attributable to Notify path.
4. Existing non-Discord DB rows follow explicit compatibility policy (deprecated read-only and non-dispatch).
5. Retired notifier references are absent from active dependency manifests (`go.mod`, `go.sum`, package lock files).
6. Runtime audit confirms no active retired-notifier install/link/reference in executable/runtime paths.
7. Active docs no longer claim multi-service notifier behavior for the current rollout.
8. Firefox failing-spec triage clearly distinguishes in-scope notification failures from separately tracked suites.
9. `.gitignore`, `.dockerignore`, `codecov.yml`, and `Dockerfile` are reviewed and updated only if required by actual implementation deltas.
10. Additional provider services remain disabled pending one-by-one validated rollout PRs.
11. Legacy fallback feature-flag behavior is force-false and non-revivable through standard API operations.
12. Rollback procedures preserve no-fallback ambiguity constraints.

## 13) Handoff to Supervisor

This spec is ready for Supervisor review and implementation assignment.

Review focus:

- Verify defense-in-depth layering is complete (UI + API + service + runtime).
- Confirm migration policy is explicit and reversible.
- Confirm audit command set is sufficient to prove legacy notifier retirement.
- Confirm PR slicing minimizes risk and isolates rollback paths.
