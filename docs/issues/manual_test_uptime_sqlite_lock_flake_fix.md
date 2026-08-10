---
title: Manual Test Plan - Uptime Monitor SQLite Lock Flake Fix
status: Open
priority: Medium
assignee: QA
labels: testing, backend, reliability
---

# Test Objective
Confirm the fix for the flaky `TestProxyHostCreate_TriggersAsyncUptimeSyncWhenServiceConfigured`
backend test (a pre-existing SQLite lock-contention bug in test setup, not the
recent OpenTelemetry dependency bump that was originally suspected) resolves
the flake, and confirm the accompanying production-hardening retry logic for
uptime monitor creation does not change any user-visible behavior when adding
a proxy host.

# Scope
- In scope: backend test reliability for proxy-host creation and its
  triggered async uptime-monitor sync; confirming uptime monitors still get
  created correctly (from a user's point of view) when a proxy host with a
  configured service is added.
- Out of scope: production SQLite configuration in general (unchanged),
  the separately-filed check-then-act race in `ensureUptimeHost` for proxy
  hosts sharing a `forward_host` (tracked at
  https://github.com/Wikid82/Charon/issues/1221 — not part of this fix).

# Prerequisites
- Charon repository is up to date on `development` (commits `3c81849d`,
  `96ee480a`, `f1bcb3a4`).
- Backend test environment is available (`cd backend && go test ./...`).
- Ability to run backend tests repeatedly, including with `-race` and
  `-count`.
- A running Charon instance (Docker or `go run ./cmd/api` + frontend dev
  server) for the manual UI check in Scenario 4.

# Manual Scenarios

## 1) Target flaky test repeated run
- [ ] Run `go test ./internal/api/handlers/... -run TestProxyHostCreate_TriggersAsyncUptimeSyncWhenServiceConfigured -count=20 -race -v` from `backend/`.
- [ ] Confirm 20/20 pass, zero data races, and no `database table is locked` log lines.

## 2) New concurrent-load regression test
- [ ] Run `go test ./internal/api/handlers/... -run TestProxyHostCreate_TriggersAsyncUptimeSyncWhenServiceConfigured_ConcurrentLoad -count=20 -race -v` from `backend/`.
- [ ] Confirm all runs pass — this test fires 8 concurrent proxy-host-creation requests to deliberately reproduce the lock-contention window and verifies every one gets its uptime monitor created.

## 3) Full package regression run
- [ ] Run `go test ./internal/api/handlers/... -count=5` (repeat the full package 5 times) from `backend/`.
- [ ] Run `go test ./internal/services/... -count=1`.
- [ ] Confirm 0 failures across both packages, no lock-contention errors in the output.

## 4) User-facing sanity check — uptime monitor still gets created
- [ ] In the Charon UI, add a new Proxy Host with a valid forward service and uptime monitoring enabled.
- [ ] Confirm the host saves successfully and an uptime monitor appears for it (Uptime page) within a few seconds, with a status check having run.
- [ ] Repeat by adding 3-4 proxy hosts back-to-back in quick succession.
- [ ] Confirm every host gets its uptime monitor created — none silently missing.

## 5) Retry-on-lock hardening (defense-in-depth) — race-forced verification
- [ ] Run `go test ./internal/services/... -run TestSyncAndCheckForHost_RetriesOnTransientLockError -count=10 -race -v` from `backend/`.
- [ ] Confirm 10/10 pass with zero data races.

# Expected Results
- The originally-flaky test and its new concurrent-load sibling pass
  reliably under repeated and `-race` runs.
- No `database table is locked: uptime_monitors` errors appear in any test
  output.
- Full backend package runs for `internal/api/handlers` and
  `internal/services` show zero failures.
- End users see no difference in behavior: adding a proxy host with uptime
  monitoring enabled still creates its monitor promptly and reliably, even
  when several hosts are added in quick succession.

# Regression Checks (No Production Impact)
- [ ] Confirm this is a test-infrastructure fix plus an internal retry loop
      only — no database schema changes, no API contract changes, no
      frontend changes.
- [ ] Confirm production SQLite configuration (`journal_mode=WAL`,
      `busy_timeout=5000`, single-connection pool) is unchanged.
- [ ] Confirm the standard backend test workflow (`go test ./...`) still
      completes successfully after this fix.
- [ ] Confirm the separately-tracked `ensureUptimeHost` race
      (GitHub #1221) is out of scope and was not silently folded into this
      fix.
