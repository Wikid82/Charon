# Uptime Monitoring Regression Investigation (Scheduled vs Manual)

Date: 2026-03-01
Owner: Planning Agent
Status: Investigation Complete, Fix Plan Proposed
Severity: High (false DOWN states on automated monitoring)

## 1. Executive Summary

Two services (Wizarr and Charon) can flip to `DOWN` during scheduled cycles while manual checks immediately return `UP` because scheduled checks use a host-level TCP gate that can short-circuit monitor-level HTTP checks.

The scheduled path is:
- `ticker -> CheckAll -> checkAllHosts -> (host status down) -> markHostMonitorsDown`

The manual path is:
- `POST /api/v1/uptime/monitors/:id/check -> CheckMonitor -> checkMonitor`

Only the scheduled path runs host precheck gating. If host precheck fails (TCP to upstream host/port), `CheckAll` skips HTTP checks and forcibly writes monitor status to `down` with heartbeat message `Host unreachable`.

This is a backend state mutation problem (not only UI rendering).

## 1.1 Monitoring Policy (Authoritative Behavior)

Charon uptime monitoring SHALL follow URL-truth semantics for HTTP/HTTPS monitors,
matching third-party external monitor behavior (Uptime Kuma style) without requiring
any additional service.

Policy:
- HTTP/HTTPS monitors are URL-truth based. The monitor result is authoritative based
  on the configured URL check outcome (status code/timeout/TLS/connectivity from URL
  perspective).
- Internal TCP reachability precheck (`ForwardHost:ForwardPort`) is
  non-authoritative for HTTP/HTTPS monitor status.
- TCP monitors remain endpoint-socket checks and may rely on direct socket
  reachability semantics.
- Host precheck may still be used for optimization, grouping telemetry, and operator
  diagnostics, but SHALL NOT force HTTP/HTTPS monitors to DOWN.

## 2. Research Findings

### 2.1 Execution Path Comparison (Required)

### Scheduled path behavior
- Entry: `backend/internal/api/routes/routes.go` (background ticker, calls `uptimeService.CheckAll()`)
- `CheckAll()` calls `checkAllHosts()` first.
  - File: `backend/internal/services/uptime_service.go:354`
- `checkAllHosts()` updates each `UptimeHost.Status` via TCP checks in `checkHost()`.
  - File: `backend/internal/services/uptime_service.go:395`
- `checkHost()` dials `UptimeHost.Host` + monitor port (prefer `ProxyHost.ForwardPort`, fallback to URL port).
  - File: `backend/internal/services/uptime_service.go:437`
- Back in `CheckAll()`, monitors are grouped by `UptimeHostID`.
  - File: `backend/internal/services/uptime_service.go:367`
- If `UptimeHost.Status == "down"`, `markHostMonitorsDown()` is called and individual monitor checks are skipped.
  - File: `backend/internal/services/uptime_service.go:381`
  - File: `backend/internal/services/uptime_service.go:593`

### Manual path behavior
- Entry: `POST /api/v1/uptime/monitors/:id/check`.
  - Handler: `backend/internal/api/handlers/uptime_handler.go:107`
- Calls `service.CheckMonitor(*monitor)` asynchronously.
  - File: `backend/internal/services/uptime_service.go:707`
- `checkMonitor()` performs direct HTTP/TCP monitor check and updates monitor + heartbeat.
  - File: `backend/internal/services/uptime_service.go:711`

### Key divergence
- Scheduled: host-gated (precheck can override monitor)
- Manual: direct monitor check (no host gate)

## 3. Root Cause With Evidence

## 3.1 Primary Root Cause: Host Precheck Overrides HTTP Success in Scheduled Cycles

When `UptimeHost` is marked `down`, scheduled checks do not run `checkMonitor()` for that host's monitors. Instead they call `markHostMonitorsDown()` which:
- sets each monitor `Status = "down"`
- writes `UptimeHeartbeat{Status: "down", Message: "Host unreachable"}`
- maxes failure count (`FailureCount = MaxRetries`)

Evidence:
- Short-circuit: `backend/internal/services/uptime_service.go:381`
- Forced down write: `backend/internal/services/uptime_service.go:610`
- Forced heartbeat message: `backend/internal/services/uptime_service.go:624`

This exactly matches symptom pattern:
1. Manual refresh sets monitor `UP` via direct HTTP check.
2. Next scheduler cycle can force it back to `DOWN` from host precheck path.

## 3.2 Hypothesis Check: TCP precheck can fail while public URL HTTP check succeeds

Confirmed as plausible by design:
- `checkHost()` tests upstream reachability (`ForwardHost:ForwardPort`) from Charon runtime.
- `checkMonitor()` tests monitor URL (public domain URL, often via Caddy/public routing).

A service can be publicly reachable by monitor URL while upstream TCP precheck fails due to network namespace/routing/DNS/hairpin differences.

This is especially likely for:
- self-referential routes (Charon monitoring Charon via public hostname)
- host/container networking asymmetry
- services reachable through proxy path but not directly on upstream socket from current runtime context

## 3.3 Recent Change Correlation (Required)

### `SyncAndCheckForHost` (regression amplifier)
- Introduced in commit `2cd19d89` and called from proxy host create path.
- Files:
  - `backend/internal/services/uptime_service.go:1195`
  - `backend/internal/api/handlers/proxy_host_handler.go:418`
- Behavior: creates/syncs monitor and immediately runs `checkMonitor()`.

Impact: makes monitors quickly show `UP` after create/manual, then scheduler can flip to `DOWN` if host precheck fails. This increased visibility of scheduled/manual inconsistency.

### `CleanupStaleFailureCounts`
- Introduced in `2cd19d89`, refined in `7a12ab79`.
- File: `backend/internal/services/uptime_service.go:1277`
- It runs at startup and resets stale monitor states only; not per-cycle override logic.
- Not root cause of recurring per-cycle flip.

### Frontend effective status changes
- Latest commit `0241de69` refactors `effectiveStatus` handling.
- File: `frontend/src/pages/Uptime.tsx`.
- Backend evidence proves this is not visual-only: scheduler writes `down` heartbeats/messages directly in DB.

## 3.4 Grouping Logic Analysis (`UptimeHost`/`UpstreamHost`)

Monitors are grouped by `UptimeHostID` in `CheckAll()`. `UptimeHost` is derived from `ProxyHost.ForwardHost` in sync flows.

Relevant code:
- group map by `UptimeHostID`: `backend/internal/services/uptime_service.go:367`
- host linkage in sync: `backend/internal/services/uptime_service.go:189`, `backend/internal/services/uptime_service.go:226`
- sync single-host update path: `backend/internal/services/uptime_service.go:1023`

Risk: one host precheck failure can mark all grouped monitors down without URL-level validation.

## 4. Technical Specification (Fix Plan)

## 4.1 Minimal Proper Fix (First)

Goal: eliminate false DOWN while preserving existing behavior as much as possible.

Change `CheckAll()` host-down branch to avoid hard override for HTTP/HTTPS monitors.

Mandatory hotfix rule:
- WHEN a host precheck is `down`, THE SYSTEM SHALL partition host monitors by type inside `CheckAll()`.
- `markHostMonitorsDown` MUST be invoked only for `tcp` monitors.
- `http`/`https` monitors MUST still run through `checkMonitor()` and MUST NOT be force-written `down` by the host precheck path.
- Host precheck outcomes MAY be recorded for optimization/telemetry/grouping, but MUST NOT be treated as final status for `http`/`https` monitors.

Proposed rule:
1. If host is down:
  - For `http`/`https` monitors: still run `checkMonitor()` (do not force down).
  - For `tcp` monitors: keep current host-down fast-path (`markHostMonitorsDown`) or direct tcp check.
2. If host is not down:
   - Keep existing behavior (run `checkMonitor()` for all monitors).

Rationale:
- Aligns scheduled behavior with manual for URL-based monitors.
- Preserves reverse proxy product semantics where public URL availability is the source of truth.
- Minimal code delta in `CheckAll()` decision branch.
- Preserves optimization for true TCP-only monitors.

### Exact file/function targets
- `backend/internal/services/uptime_service.go`
  - `CheckAll()`
  - add small helper (optional): `partitionMonitorsByType(...)`

## 4.2 Long-Term Robust Fix (Deferred)

Introduce host precheck as advisory signal, not authoritative override.

Design:
1. Add `HostReachability` result to run context (not persisted as forced monitor status).
2. Always execute per-monitor checks, but use host precheck to:
   - tune retries/backoff
   - annotate failure reason
   - optimize notification batching
3. Optionally add feature flag:
   - `feature.uptime.strict_host_precheck` (default `false`)
   - allows legacy strict gating in environments that want it.

Benefits:
- Removes false DOWN caused by precheck mismatch.
- Keeps performance and batching controls.
- More explicit semantics for operators.

## 5. API/Schema Impact

No API contract change required for minimal fix.
No database migration required for minimal fix.

Long-term fix may add one feature flag setting only.

## 6. EARS Requirements

### Ubiquitous
- THE SYSTEM SHALL evaluate HTTP/HTTPS monitor availability using URL-level checks as the authoritative signal.

### Event-driven
- WHEN the scheduled uptime cycle runs, THE SYSTEM SHALL execute HTTP/HTTPS monitor checks regardless of internal host precheck state.
- WHEN the scheduled uptime cycle runs and host precheck is down, THE SYSTEM SHALL apply host-level forced-down logic only to TCP monitors.

### State-driven
- WHILE a monitor type is `http` or `https`, THE SYSTEM SHALL NOT force monitor status to `down` solely from internal host precheck failure.
- WHILE a monitor type is `tcp`, THE SYSTEM SHALL evaluate status using endpoint socket reachability semantics.

### Unwanted behavior
- IF internal host precheck is unreachable AND URL-level HTTP/HTTPS check returns success, THEN THE SYSTEM SHALL set monitor status to `up`.
- IF internal host precheck is reachable AND URL-level HTTP/HTTPS check fails, THEN THE SYSTEM SHALL set monitor status to `down`.

### Optional
- WHERE host precheck telemetry is enabled, THE SYSTEM SHALL record host-level reachability for diagnostics and grouping without overriding HTTP/HTTPS monitor final state.

## 7. Implementation Plan

### Phase 1: Reproduction Lock-In (Tests First)
- Add backend service test proving current regression:
  - host precheck fails
  - monitor URL check would succeed
  - scheduled `CheckAll()` currently writes down (existing behavior)
- File: `backend/internal/services/uptime_service_test.go` (new test block)

### Phase 2: Minimal Backend Fix
- Update `CheckAll()` branch logic to run HTTP/HTTPS monitors even when host is down.
- Make monitor partitioning explicit and mandatory in `CheckAll()` host-down branch.
- Add an implementation guard before partitioning: normalize monitor type using
  `strings.TrimSpace` + `strings.ToLower` to prevent `HTTP`/`HTTPS` case
  regressions and whitespace-related misclassification.
- Ensure `markHostMonitorsDown` is called only for TCP monitor partitions.
- File: `backend/internal/services/uptime_service.go`

### Phase 3: Backend Validation
- Add/adjust tests:
  - scheduled path no longer forces down when HTTP succeeds
  - manual and scheduled reach same final state for HTTP monitors
  - internal host unreachable + public URL HTTP 200 => monitor is `UP`
  - internal host reachable + public URL failure => monitor is `DOWN`
  - TCP monitor behavior unchanged under host-down conditions
- Files:
  - `backend/internal/services/uptime_service_test.go`
  - `backend/internal/services/uptime_service_race_test.go` (if needed for concurrency side-effects)

### Phase 4: Integration/E2E Coverage
- Add targeted API-level integration test for scheduler vs manual parity.
- Add Playwright scenario for:
  - monitor set UP by manual check
  - remains UP after scheduled cycle when URL is reachable
- Add parity scenario for:
  - internal TCP precheck unreachable + URL returns 200 => `UP`
  - internal TCP precheck reachable + URL failure => `DOWN`
- Files:
  - `backend/internal/api/routes/routes_test.go` (or uptime handler integration suite)
  - `tests/monitoring/uptime-monitoring.spec.ts` (or equivalent uptime spec file)

Scope note:
- This hotfix plan is intentionally limited to backend behavior correction and
  regression tests (unit/integration/E2E).
- Dedicated documentation-phase work is deferred and out of scope for this
  hotfix PR.

## 8. Test Plan (Unit / Integration / E2E)

Duplicate notification definition (hotfix acceptance/testing):
- A duplicate notification means the same `(monitor_id, status,
  scheduler_tick_id)` is emitted more than once within a single scheduler run.

## Unit Tests
1. `CheckAll_HostDown_DoesNotForceDown_HTTPMonitor_WhenHTTPCheckSucceeds`
2. `CheckAll_HostDown_StillHandles_TCPMonitor_Conservatively`
3. `CheckAll_ManualAndScheduledParity_HTTPMonitor`
4. `CheckAll_InternalHostUnreachable_PublicURL200_HTTPMonitorEndsUp` (blocking)
5. `CheckAll_InternalHostReachable_PublicURLFail_HTTPMonitorEndsDown` (blocking)

## Integration Tests
1. Scheduler endpoint (`/api/v1/system/uptime/check`) parity with monitor check endpoint.
2. Verify DB heartbeat message is real HTTP result (not `Host unreachable`) for HTTP monitors where URL is reachable.
3. Verify when host precheck is down, HTTP monitor heartbeat/notification output is derived from `checkMonitor()` (not synthetic host-path `Host unreachable`).
4. Verify no duplicate notifications are emitted from host+monitor paths for the same scheduler run, where duplicate is defined as repeated `(monitor_id, status, scheduler_tick_id)`.
5. Verify internal host precheck unreachable + public URL 200 still resolves monitor `UP`.
6. Verify internal host precheck reachable + public URL failure resolves monitor `DOWN`.

## E2E Tests
1. Create/sync monitor scenario where manual refresh returns `UP`.
2. Wait one scheduler interval.
3. Assert monitor remains `UP` and latest heartbeat is not forced `Host unreachable` for reachable URL.
4. Assert scenario: internal host precheck unreachable + public URL 200 => monitor remains `UP`.
5. Assert scenario: internal host precheck reachable + public URL failure => monitor is `DOWN`.

## Regression Guardrails
- Add a test explicitly asserting that host precheck must not unconditionally override HTTP monitor checks.
- Add explicit assertions that HTTP monitors under host-down precheck emit
  check-derived heartbeat messages and do not produce duplicate notifications
  under the `(monitor_id, status, scheduler_tick_id)` rule within a single
  scheduler run.

## 9. Risks and Rollback

## Risks
1. More HTTP checks under true host outage may increase check volume.
2. Notification patterns may shift from single host-level event to monitor-level batched events.
3. Edge cases for mixed-type monitor groups (HTTP + TCP) need deterministic behavior.

## Mitigations
1. Preserve batching (`queueDownNotification`) and existing retry thresholds.
2. Keep TCP strict path unchanged in minimal fix.
3. Add explicit log fields and targeted tests for mixed groups.

## Rollback Plan
1. Revert the `CheckAll()` branch change only (single-file rollback).
2. Keep added tests; mark expected behavior as legacy if temporary rollback needed.
3. If necessary, introduce temporary feature toggle to switch between strict and tolerant host gating.

## 10. PR Slicing Strategy

Decision: Single focused PR (hotfix + tests)

Trigger reasons:
- High-severity runtime behavior fix requiring minimal blast radius
- Fast review/rollback with behavior-only delta plus regression coverage
- Avoid scope creep into optional hardening/feature-flag work

### PR-1 (Hotfix + Tests)
Scope:
- `CheckAll()` host-down branch adjustment for HTTP/HTTPS
- Unit/integration/E2E regression tests for URL-truth semantics

Files:
- `backend/internal/services/uptime_service.go`
- `backend/internal/services/uptime_service_test.go`
- `backend/internal/api/routes/routes_test.go` (or equivalent)
- `tests/monitoring/uptime-monitoring.spec.ts` (or equivalent)

Validation gates:
- backend unit tests pass
- targeted uptime integration tests pass
- targeted uptime E2E tests pass
- no behavior regression in existing `CheckAll` tests

Rollback:
- single revert of PR-1 commit

## 11. Acceptance Criteria (DoD)

1. Scheduled and manual checks produce consistent status for HTTP/HTTPS monitors.
2. A reachable monitor URL is not forced to `DOWN` solely by host precheck failure.
3. New regression tests fail before fix and pass after fix.
4. No break in TCP monitor behavior expectations.
5. No new critical/high security findings in touched paths.
6. Blocking parity case passes: internal host precheck unreachable + public URL 200 => scheduled result is `UP`.
7. Blocking parity case passes: internal host precheck reachable + public URL failure => scheduled result is `DOWN`.
8. Under host-down precheck, HTTP monitors produce check-derived heartbeat messages (not synthetic `Host unreachable` from host path).
9. No duplicate notifications are produced by host+monitor paths within a
  single scheduler run, where duplicate is defined as repeated
  `(monitor_id, status, scheduler_tick_id)`.

## 12. Implementation Risks

1. Increased scheduler workload during host-precheck failures because HTTP/HTTPS checks continue to run.
2. Notification cadence may change due to check-derived monitor outcomes replacing host-forced synthetic downs.
3. Mixed monitor groups (TCP + HTTP/HTTPS) require strict ordering/partitioning to avoid regression.

Mitigations:
- Keep change localized to `CheckAll()` host-down branch decisioning.
- Add explicit regression tests for both parity directions and mixed monitor types.
- Keep rollback path as single-commit revert.
