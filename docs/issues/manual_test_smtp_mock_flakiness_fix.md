---
title: Manual Test Plan - SMTP Mock Server Flakiness Fix
status: Open
priority: High
assignee: QA
labels: testing, backend, reliability
---

# Test Objective
Confirm the SMTP mock server flakiness fix improves mail test reliability without changing production mail behavior.

# Scope
- In scope: test reliability for SMTP mock server flows used by backend mail tests.
- Out of scope: production SMTP sending behavior and user-facing mail features.

# Prerequisites
- Charon repository is up to date.
- Backend test environment is available.
- Ability to run backend tests repeatedly.

# Manual Scenarios

## 1) Target flaky test repeated run
- [ ] Run `TestMailService_TestConnection_StartTLSSuccessWithAuth` repeatedly (at least 20 times).
- [ ] Record pass/fail count and any intermittent errors.

## 2) Mail service targeted subset run
- [ ] Run mail service connection and send test subset once.
- [ ] Confirm no new intermittent failures appear in related tests.

## 3) Race-focused verification
- [ ] Run targeted mail service tests with race detection enabled.
- [ ] Confirm no race warnings or hangs occur.

## 4) Cleanup/shutdown stability check
- [ ] Repeat targeted runs and watch for stuck test processes or timeout behavior.
- [ ] Confirm test execution exits cleanly each run.

# Expected Results
- Repeated target test runs complete with zero flaky failures.
- Related mail service test subset remains stable.
- No race detector findings for targeted scenarios.
- No hangs during test cleanup/shutdown.

# Regression Checks (No Production Impact)
- [ ] Confirm only test reliability behavior changed; no production mail behavior changes are required.
- [ ] Confirm no production endpoints, settings, or user-facing mail flows are affected.
- [ ] Confirm standard backend test workflow still completes successfully after this fix.
