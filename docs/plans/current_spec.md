---
post_title: "Current Spec: Stabilize Flaky SMTP STARTTLS+AUTH Unit Test"
categories:
  - actions
  - testing
  - backend
  - reliability
tags:
  - go
  - smtp
  - starttls
  - unit-tests
  - ci-stability
summary: "Implementation plan to remove CI flakiness in MailService STARTTLS+AUTH tests by hardening mock SMTP server lifecycle and connection handling in test helpers only."
post_date: 2026-02-22
---

## Active Plan: Stabilize Flaky SMTP STARTTLS+AUTH Unit Test

Date: 2026-02-22
Status: Active and authoritative
Scope Type: Test reliability hardening (backend unit tests only)

## 1) Introduction

This plan addresses flakiness in backend unit test
`TestMailService_TestConnection_StartTLSSuccessWithAuth` by improving mock SMTP
test helpers used by `backend/internal/services/mail_service_test.go`.

Root-cause hypothesis to validate and fix:

- Existing mock SMTP server helpers accept only one connection, then exit.
- STARTTLS + AUTH flows in `net/smtp` are negotiation-heavy and can involve
  additional command/connection behavior under CI timing variance.
- Single-accept test server behavior creates race-prone shutdown windows.

Goal:

- Make test helper servers robust and deterministic by accepting connections in
  a loop until cleanup, handling each connection in its own goroutine, and
  enforcing deterministic shutdown that waits for accept-loop exit and active
  handler goroutines (explicit per-connection synchronization via waitgroup).

Non-goal:

- No production mail service behavior changes.

## 2) Research Findings

### 2.1 Current flaky target and helper topology

Primary target test:

- `backend/internal/services/mail_service_test.go`
  - `TestMailService_TestConnection_StartTLSSuccessWithAuth`

Helper functions in same file (current behavior):

- `startMockSMTPServer(...)`
  - Single `Accept()` call, single connection handler, then goroutine exits.
- `startMockSSLSMTPServer(...)`
  - Single `Accept()` call, single connection handler, then goroutine exits.
- `handleSMTPConn(...)`
  - Handles SMTP protocol conversation for each accepted connection.

### 2.2 Flakiness vector summary

- Current single-accept model is fragile for tests where client behavior may
  include additional negotiation/timing paths.
- Cleanup currently closes listener and waits on a done signal, but done is
  tied to a single accept goroutine rather than a full server lifecycle.
- Under CI contention, this can surface nondeterministic failures in tests
  expecting reliable STARTTLS + AUTH handshake behavior.

### 2.3 Repo config review requested by user

Reviewed files:

- `.gitignore`
- `codecov.yml`
- `.dockerignore`
- `Dockerfile`

Conclusion for this scope:

- No changes required for this test-only helper stabilization.
- Existing ignore/coverage/docker config does not block this plan.

## 3) Requirements (EARS)

- R1: WHEN mock SMTP test helpers are started, THE SYSTEM SHALL accept
  connections in a loop until explicit cleanup closes the listener.
- R2: WHEN a connection is accepted, THE SYSTEM SHALL handle it in its own
  goroutine so one slow session cannot block new accepts.
- R3: WHEN cleanup is invoked, THE SYSTEM SHALL close the listener and await
  deterministic server-loop termination plus completion of active connection
  handlers (done channel + waitgroup synchronization).
- R4: IF cleanup wait exceeds timeout, THEN THE SYSTEM SHALL report a failure
  signal and return without hanging test execution.
- R5: WHEN this fix is implemented, THE SYSTEM SHALL keep production code
  untouched and restrict changes to test helper scope.
- R6: WHEN targeted backend tests run, THE SYSTEM SHALL pass reliably in local
  and CI-like conditions.

## 4) Technical Specification

### 4.1 Exact target files and functions (minimal diff scope)

Primary file to edit:

- `backend/internal/services/mail_service_test.go`

Functions in scope:

- `startMockSMTPServer(t *testing.T, tlsConf *tls.Config, supportStartTLS bool, requireAuth bool) (string, func())`
- `startMockSSLSMTPServer(t *testing.T, tlsConf *tls.Config, requireAuth bool) (string, func())`

Related function (read-only unless strictly necessary):

- `handleSMTPConn(conn net.Conn, tlsConf *tls.Config, supportStartTLS bool, requireAuth bool)`

Out of scope:

- `backend/internal/services/mail_service.go`
- Any non-mail-service test files.

### 4.2 Planned helper behavior changes

For both helper server starters:

1. Replace single `Accept()` with accept loop:
   - Continue accepting until listener close returns an error.
   - Treat listener-close accept errors as normal shutdown path.
2. Spawn per-connection handler goroutine:
   - Each accepted connection handled independently.
   - Connection closed within goroutine after handling.
3. Deterministic lifecycle signaling:
  - Keep `done` channel signaling tied to server accept-loop exit.
  - Ensure exactly one close of `done` from server goroutine.
  - Track active connection-handler goroutines with explicit waitgroup sync.
4. Cleanup contract:
   - `cleanup()` closes listener first.
  - Wait for accept-loop exit and active handler completion with bounded timeout (`2s` currently acceptable).
  - Never block indefinitely.
  - Timeout path is a test failure signal (non-hanging), not a silent pass.

### 4.3 Concurrency and race safety notes

- Accept-loop ownership:
  - One goroutine owns listener accept loop and is the only goroutine that may
    close `done`.
- Connection handler isolation:
  - One goroutine per accepted connection; no shared mutable state required for
    protocol behavior in this helper.
- Per-connection synchronization:
  - A waitgroup must increment before each handler goroutine starts and must
    decrement on handler exit so cleanup can deterministically wait for active
    handlers to finish.
- Listener close semantics:
  - `listener.Close()` from cleanup is expected to break `Accept()`.
  - Exit condition should avoid noisy test failures on intentional close.
- Cleanup timeout behavior:
  - Timeout remains defensive to prevent suite hangs under pathological CI
    resource starvation.
  - Timeout branch must report failure (e.g., `t.Errorf`/`t.Fatalf` policy) and
    return; no panic and no silent success.

### 4.4 Error handling policy in helpers

- Treat only expected shutdown accept errors (listener close path) as normal.
- Surface unexpected `Accept()` failures as test failure signals.
- Keep helper logic simple and deterministic; avoid over-engineered retry logic.

## 5) Implementation Plan

### Phase 1: Testing protocol sequencing note

- Policy remains E2E-first globally.
- Explicit exception rationale for this change: scope is backend test-helper-only
  (`mail_service_test.go`) with no production/frontend/runtime behavior delta,
  so targeted backend test-first verification is authorized for this plan.
- Mandatory preflight before unit/coverage steps:
  - `bash scripts/local-patch-report.sh`
  - Artifacts: `test-results/local-patch-report.md` and `test-results/local-patch-report.json`

### Phase 2: Baseline confirmation and failure reproduction context

- Capture current helper behavior and flaky test target:
  - `go test ./backend/internal/services -run TestMailService_TestConnection_StartTLSSuccessWithAuth -count=1 -v`

### Phase 3: Helper lifecycle hardening

- Update `startMockSMTPServer` and `startMockSSLSMTPServer` to loop accept.
- Add per-connection goroutine handling.
- Preserve/strengthen deterministic cleanup using listener close + done wait +
  per-connection waitgroup completion.

### Phase 4: Targeted validation

- Re-run target test repeatedly to validate stability:
  - `go test ./backend/internal/services -run TestMailService_TestConnection_StartTLSSuccessWithAuth -count=20`
- Run mail service test subset:
  - `go test ./backend/internal/services -run TestMailService_ -count=1`
- Run race-focused targeted validation:
  - `go test -race ./backend/internal/services -run 'TestMailService_(TestConnection|Send)' -count=1`

### Phase 5: Backend coverage validation

- Run backend coverage task/script required by repo workflow:
  - Preferred script: `scripts/go-test-coverage.sh`
  - VS Code equivalent: backend coverage task if available.

## 6) Validation Matrix

| Validation Item | Command / Task | Scope | Pass Criteria |
|---|---|---|---|
| Targeted flaky test | `go test ./backend/internal/services -run TestMailService_TestConnection_StartTLSSuccessWithAuth -count=20` | Direct flaky test | No failures across repeated runs |
| Mail service subset | `go test ./backend/internal/services -run TestMailService_ -count=1` | Nearby regression safety | All selected tests pass |
| Race-focused targeted tests | `go test -race ./backend/internal/services -run 'TestMailService_(TestConnection|Send)' -count=1` | Concurrency/race safety | No race reports; tests pass |
| Package sanity | `go test ./backend/internal/services -count=1` | Service package confidence | Package tests pass |
| Backend coverage gate | `scripts/go-test-coverage.sh` | Repo-required backend coverage check | Meets configured minimum threshold (85% default) |

Notes:

- E2E-first protocol remains project-wide policy. This plan uses the explicit
  backend-only targeted-test exception because scope is confined to test helper
  internals with no production/UI behavior changes.
- Local patch report preflight is required before unit/coverage gates.

## 7) Risk Assessment

Risk level: Low.

- Change type: test-only.
- Production code: untouched.
- Runtime behavior: unchanged for shipped binary.
- Primary risk: helper lifecycle bug causing test hangs.
- Mitigation: bounded cleanup timeout, accept-loop exit on listener close,
  focused repeated-run validation.

## 8) PR Slicing Strategy

Decision: Single PR.

Rationale:

- Extremely small scope (one test file, two helper functions).
- No cross-domain dependencies.
- Easier review and rollback.

### PR-1 (single slice)

- Scope:
  - `backend/internal/services/mail_service_test.go` helper lifecycle updates.
- Dependencies:
  - None.
- Validation gates:
  - Validation matrix in Section 6.
- Rollback contingency:
  - Revert single PR if instability increases.

## 9) Config File Review Outcome

Reviewed for this request:

- `.gitignore`
- `codecov.yml`
- `.dockerignore`
- `Dockerfile`

Suggested updates:

- None required for this scope.
- Revisit only if implementation introduces new generated artifacts or test
  output paths not currently handled (not expected here).

## 10) Acceptance Criteria

- AC1: `startMockSMTPServer` accepts in loop until cleanup and no longer exits
  after a single connection.
- AC2: `startMockSSLSMTPServer` accepts in loop until cleanup and no longer
  exits after a single connection.
- AC3: Each accepted connection is handled in its own goroutine.
- AC4: Cleanup closes listener and uses done-channel plus per-connection
  waitgroup synchronization with bounded timeout.
- AC5: Unexpected `Accept()` failures are surfaced as test failure signals;
  expected listener-close shutdown errors are treated as normal.
- AC6: `TestMailService_TestConnection_StartTLSSuccessWithAuth` passes reliably
  under repeated runs.
- AC7: Targeted race validation for mail-service tests passes with `go test -race`.
- AC8: Cleanup timeout path reports failure and returns (non-hanging), never
  silent pass.
- AC9: Backend coverage script/task completes successfully at configured
  threshold.
- AC10: No production file changes are included in the implementation PR.

## 11) Definition of Done

- Planned helper changes are implemented exactly in scoped functions.
- Cleanup deterministically waits for accept-loop exit and active handlers via
  done + waitgroup synchronization.
- Only expected listener-close shutdown accept errors are non-fatal; unexpected
  accept errors fail tests.
- Cleanup timeout is reported as failure signal and cannot pass silently.
- Validation matrix passes.
- Diff is limited to test helper scope in `mail_service_test.go`.
- No updates to `.gitignore`, `codecov.yml`, `.dockerignore`, or `Dockerfile`
  are required or included.
