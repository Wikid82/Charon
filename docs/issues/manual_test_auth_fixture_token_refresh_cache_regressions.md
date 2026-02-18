---
title: Manual Test Plan - Auth Fixture Token Refresh/Cache Regressions
status: Open
priority: High
assignee: QA
labels: testing, auth, regression
---

## Objective

Validate that recent auth fixture token refresh/cache updates do not introduce login instability, stale session behavior, or parallel test flakiness.

## Preconditions

- Charon test environment is running and reachable.
- A valid test user account is available.
- Browser context can be reset between scenarios (clear cookies and site data).
- Test runner can execute targeted auth fixture scenarios.

## Scenarios

### 1) Baseline Login and Session Reuse

- Step: Sign in once with valid credentials.
- Step: Run an action that requires authentication.
- Step: Run a second authenticated action without re-authenticating.
- Expected outcome:
  - First action succeeds.
  - Second action succeeds without unexpected login prompts.
  - No session-expired message appears.

### 2) Token Refresh Near Expiry

- Step: Start with a session near refresh threshold.
- Step: Trigger an authenticated action that forces token refresh path.
- Step: Continue with another authenticated action.
- Expected outcome:
  - Refresh occurs without visible interruption.
  - Follow-up authenticated action succeeds.
  - No unauthorized or redirect loop behavior occurs.

### 3) Concurrent Authenticated Actions

- Step: Trigger multiple authenticated actions at the same time.
- Step: Observe completion and authentication state.
- Expected outcome:
  - Actions complete without random auth failures.
  - No intermittent unauthorized responses.
  - Session remains valid after all actions complete.

### 4) Cache Reuse Across Test Steps

- Step: Complete one authenticated test step.
- Step: Move to the next step in the same run.
- Step: Verify auth state continuity.
- Expected outcome:
  - Auth state is reused when still valid.
  - No unnecessary re-login is required.
  - No stale-token error appears.

### 5) Clean-State Reset Behavior

- Step: Clear session data for a clean run.
- Step: Trigger an authenticated action.
- Step: Sign in again when prompted.
- Expected outcome:
  - User is correctly prompted to authenticate.
  - New session works normally after sign-in.
  - No residual state from previous run affects behavior.

## Bug Capture Template

Use this template for each defect found.

- Title:
- Date/Time (UTC):
- Tester:
- Environment (branch/commit, browser, OS):
- Scenario ID:
- Preconditions used:
- Steps to reproduce:
  1.
  2.
  3.
- Expected result:
- Actual result:
- Frequency (always/intermittent/once):
- Severity (critical/high/medium/low):
- Evidence:
  - Screenshot path:
  - Video path:
  - Relevant log snippet:
- Notes:
