---
post_title: "Phase 2 Test Interruption Analysis"
author1: "Charon Team"
post_slug: "phase-2-test-interruption-analysis"
microsoft_alias: "charon-team"
featured_image: "https://wikid82.github.io/charon/assets/images/featured/charon.png"
categories: ["testing"]
tags: ["playwright", "e2e", "phase-2", "diagnostics"]
ai_note: "true"
summary: "Analysis of Phase 2 Playwright interruptions, with root cause
  hypotheses and recommended diagnostics to stabilize execution."
post_date: "2026-02-09"
---

## 1. Introduction

This analysis investigates Phase 2 Playwright test interruptions that surface
as exit code 130 and errors such as "Target page, context or browser has been
closed" or "Test ended." The focus is on the setup/teardown flow, storage
state usage, and security-related dependencies that can affect browser
lifecycles.

## 2. Research Findings

### 2.1 Setup and Auth Storage State Flow

- Phase 1 authentication is performed in [tests/auth.setup.ts](tests/auth.setup.ts)
  using the Playwright request context, with storage state persisted to the
  path defined by [tests/constants.ts](tests/constants.ts).
- The auth setup validates cookie domain alignment with `baseURL` and writes
  the storage state to `playwright/.auth/user.json`, which Phase 2 uses.
- Global setup in [tests/global-setup.ts](tests/global-setup.ts) performs
  preflight health checks, optional emergency security reset, and cleanup.
  It does not explicitly close browsers, but it can terminate the process if
  the emergency token is invalid.

### 2.2 Playwright Project Dependencies and Scheduling

- [playwright.config.js](playwright.config.js) declares projects for `setup`,
  `security-tests`, `security-teardown`, and the browser projects.
- When `PLAYWRIGHT_SKIP_SECURITY_DEPS` is set (default), browser projects depend
  only on `setup`, but `security-teardown` remains a standalone project and can
  still run during the same invocation.
- Fully parallel execution is enabled, so independent projects may run in
  parallel unless constrained.

### 2.3 Security Teardown Behavior

- [tests/security-teardown.setup.ts](tests/security-teardown.setup.ts) uses an
  authenticated API request context and enforces a specific security state:
  Cerberus enabled, modules enabled, and admin whitelist configured.
- This teardown is documented as a post-security-test action, but it is not
  guarded from running alongside non-security browser tests.

### 2.4 Test-Level Browser/Context Closure

- No Phase 2 core/settings/tasks tests explicitly close the shared browser.
- A direct `context.close()` exists in
  [tests/monitoring/real-time-logs.spec.ts](tests/monitoring/real-time-logs.spec.ts),
  but it is scoped to a context created in `beforeAll` and should not affect
  other tests.

## 3. Root Cause Hypotheses (Ranked)

### H1: Security Teardown Runs Concurrently and Alters State Mid-Run

Because `security-teardown` is configured as a standalone project, it can run
in parallel when security dependencies are skipped. Its API calls modify
security settings and admin whitelist configuration. If this overlaps with
Phase 2 navigation or API-driven setup, it can lead to transient 401/403
responses, blocked routes, or reload events. The resulting timeouts can cause
Playwright to end tests early, which produces "Test ended" and "Target page,
context or browser has been closed" errors.

### H2: Storage State Domain Mismatch or Invalid State

`auth.setup.ts` writes storage state for the `baseURL` domain. If Phase 2 runs
with a different `PLAYWRIGHT_BASE_URL` host (for example, `localhost` vs
`127.0.0.1`), cookies may not be sent, leading to authentication failures.
Tests that assume an authenticated session can then hang in navigation or UI
assertions, eventually triggering test end and context closure errors.

### H3: Global Setup Emergency Reset Leaves Partial State

Global setup performs an emergency reset and a post-auth reset if storage state
exists. If the emergency server or token validation is inconsistent, the reset
may partially apply, leaving security modules enabled without the expected
whitelist. This can block subsequent API calls and surface as navigation
timeouts, especially early in Phase 2.

### H4: Environment Instability or Resource Pressure

If the Docker container becomes unresponsive, navigation requests can fail or
hang, which can cascade into test termination. No direct evidence is present in
this review, but the symptoms are consistent with sudden page closures after a
few tests.

## 4. Recommendations for Next Steps

- Confirm whether `security-teardown` is executing during Phase 2 runs and
  whether it overlaps with browser projects. If it does, isolate it to only run
  when `security-tests` are executed or after them.
- Validate storage state consistency by confirming Phase 2 uses the same
  `PLAYWRIGHT_BASE_URL` host as Phase 1 authentication. Align to a single host
  (`127.0.0.1` or `localhost`) and keep it consistent across setup and tests.
- Capture Playwright project scheduling output and test order to confirm whether
  any teardown is running concurrently with Phase 2 suites.
- Add a lightweight health check in Phase 2 suites (or in the base fixture) to
  detect server unresponsiveness early and surface actionable failures instead
  of page-closed errors.

## 5. Acceptance Criteria

- Phase 2 suites (core, settings, tasks) run to completion without premature
  browser/context closure errors.
- No occurrences of "page.goto: Target page, context or browser has been closed"
  or "page.goto: Test ended" in Phase 2 runs.
- Project scheduling confirms `security-teardown` does not run concurrently with
  non-security browser projects when security dependencies are skipped.
