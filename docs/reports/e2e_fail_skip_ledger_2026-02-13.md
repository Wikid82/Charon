# E2E Fail/Skip Ledger — 2026-02-13

**Phase:** 6 (Fail & Skip Census)
**Date:** 2026-02-13
**Source command:** `npx playwright test --project=firefox --project=chromium --project=webkit`
**Latest full-suite totals:** **1500 passed**, **62 failed**, **50 skipped**
**Supporting evidence sampled:** `/tmp/playwright-full-run.txt` (failure signatures and representative failures), `tests/**/*.spec.ts` (skip sources), `playwright.config.js` (project-level execution behavior)

---

## Failure Clusters

| Browser(s) | Test file | Representative failing tests | Failure signature | Suspected root cause | Owner | Priority | Repro command |
|---|---|---|---|---|---|---|---|
| firefox, chromium | `tests/settings/user-lifecycle.spec.ts` | `Complete user lifecycle: creation to resource access`; `Deleted user cannot login`; `Session isolation after logout and re-login` | `TimeoutError: page.waitForSelector('[data-testid="dashboard-container"], [role="main"]')` | Login/session readiness race before dashboard main region is stable | Playwright Dev | P0 | `npx playwright test tests/settings/user-lifecycle.spec.ts --project=chromium --project=firefox` |
| firefox, chromium | `tests/core/multi-component-workflows.spec.ts` | `WAF enforcement applies to newly created proxy`; `Security enforced even on previously created resources` | `TimeoutError: page.waitForSelector('[role="main"]')` | Security toggle + config propagation timing not synchronized with assertions | Playwright Dev + Backend Dev | P0 | `npx playwright test tests/core/multi-component-workflows.spec.ts --project=chromium --project=firefox` |
| firefox, chromium | `tests/core/data-consistency.spec.ts` | `Data created via UI is properly stored and readable via API`; `Pagination and sorting produce consistent results`; `Client-side and server-side validation consistent` | Repeated long timeout failures during API↔UI consistency checks | Eventual consistency and reload synchronization gaps in tests | Playwright Dev | P0 | `npx playwright test tests/core/data-consistency.spec.ts --project=chromium --project=firefox` |
| firefox, chromium | `tests/tasks/long-running-operations.spec.ts` | `Backup creation does not block other operations`; `Long-running task completion can be verified` | `TimeoutError: page.waitForSelector('[role="main"]')` in `beforeEach` | Setup/readiness gate too strict under background-task load | Playwright Dev | P1 | `npx playwright test tests/tasks/long-running-operations.spec.ts --project=chromium --project=firefox` |
| firefox, chromium | `tests/core/admin-onboarding.spec.ts` | `Logout clears session`; `Re-login after logout successful` | Session/onboarding flow intermittency; conditional skip present in file | Session reset and auth state handoff not deterministic | Playwright Dev | P1 | `npx playwright test tests/core/admin-onboarding.spec.ts --project=chromium --project=firefox` |
| firefox, chromium | `tests/core/auth-long-session.spec.ts` | `should maintain valid session for 60 minutes with token refresh`; `session should be isolated and not leak to other contexts` | Long-session / refresh assertions fail under timing variance | Token refresh and context isolation are timing-sensitive and cross-context brittle | Backend Dev + Playwright Dev | P1 | `npx playwright test tests/core/auth-long-session.spec.ts --project=chromium --project=firefox` |
| firefox, chromium | `tests/core/domain-dns-management.spec.ts` | `Add domain to system`; `Renew SSL certificate for domain`; `Export domains configuration as JSON` | `TimeoutError` on dashboard/main selector in `beforeEach` | Shared setup readiness issue amplified in domain/DNS suite | Playwright Dev | P1 | `npx playwright test tests/core/domain-dns-management.spec.ts --project=chromium --project=firefox` |
| firefox, chromium | `tests/modal-dropdown-triage.spec.ts` | `D. Uptime - CreateMonitorModal Type Dropdown` | `Test timeout ... keyboard.press: Target page/context/browser has been closed` | Modal close path and locator strictness under race conditions | Frontend Dev + Playwright Dev | P1 | `npx playwright test tests/modal-dropdown-triage.spec.ts --project=chromium --project=firefox` |
| firefox, chromium | `tests/settings/user-management.spec.ts` | `should copy invite link` | `expect(locator).toBeVisible() ... element(s) not found` for Copy control | Copy button locator not resilient across render states | Frontend Dev | P2 | `npx playwright test tests/settings/user-management.spec.ts --project=chromium --project=firefox --grep "copy invite link"` |
| firefox, chromium | `tests/dns-provider-types.spec.ts` | `should show script path field when Script type is selected` | `expect(locator).toBeVisible() ... element(s) not found` for script path field | Type-dependent field render timing and selector fallback mismatch | Frontend Dev | P2 | `npx playwright test tests/dns-provider-types.spec.ts --project=chromium --project=firefox --grep "Script type"` |
| firefox, chromium | `tests/core/auth-api-enforcement.spec.ts`, `tests/core/authorization-rbac.spec.ts` | Bearer token / RBAC enforcement examples from full-run failed set | Authentication/authorization assertions intermittently fail with suite instability | Upstream auth/session readiness and shared state interference | Backend Dev + Playwright Dev | P1 | `npx playwright test tests/core/auth-api-enforcement.spec.ts tests/core/authorization-rbac.spec.ts --project=chromium --project=firefox` |
| webkit (to confirm exact list next run) | Cross-cutting impacted suites | Engine-specific flakiness noted in Phase 6 planning track | Browser-engine-specific instability (pending exact test IDs) | WebKit-specific timing/render behavior and potential detached-element races | Playwright Dev | P1 | `npx playwright test --project=webkit --reporter=list` |

---

## Skip Tracking

**Current skipped total (full suite):** **50**

### Known skip sources

1. **Explicit `test.skip` / `describe.skip` in test code**
   - `tests/manual-dns-provider.spec.ts` contains multiple `test.describe.skip(...)` blocks and individual `test.skip(...)`.
   - `tests/core/admin-onboarding.spec.ts` contains conditional `test.skip(true, ...)` for Cerberus-dependent UI path.

2. **Conditional runtime skips**
   - Browser/env dependent test behavior appears in multiple suites (auth/session/security flow gating).

3. **Project-level non-execution behavior**
   - `playwright.config.js` uses dependency/ignore patterns (`skipSecurityDeps`, project `testIgnore` for security suites on browser projects).
   - Full-run artifacts can include `did not run` counts in addition to explicit skips.

### Actions to enumerate exact skip list on next run

- Run with machine-readable reporter and archive artifact:
  - `npx playwright test --project=firefox --project=chromium --project=webkit --reporter=json > /tmp/e2e-full-2026-02-13.json`
- Extract exact skipped tests with reason and browser:
  - `jq -r '.. | objects | select(.status? == "skipped") | [.projectName,.location.file,.title,.annotations] | @tsv' /tmp/e2e-full-2026-02-13.json`
- Produce canonical skip registry from the JSON output:
  - `docs/reports/e2e_skip_registry_2026-02-13.md`
- Add owner + expiration date for each non-contractual skip before Phase 8 re-enable work.

---

## Top-15 Remediation Queue (Release impact × fixability)

| Rank | Test / Scope | Browser(s) | Impact | Fixability | Owner | Priority | Immediate next action |
|---:|---|---|---|---|---|---|---|
| 1 | `tests/settings/user-lifecycle.spec.ts` — `Complete user lifecycle: creation to resource access` | chromium, firefox | Critical auth/user-flow gate | High | Playwright Dev | P0 | Add deterministic dashboard-ready wait helper and apply to suite `beforeEach` |
| 2 | `tests/settings/user-lifecycle.spec.ts` — `Deleted user cannot login` | chromium, firefox | Security correctness | High | Playwright Dev | P0 | Wait on delete response + auth state settle before login assertion |
| 3 | `tests/settings/user-lifecycle.spec.ts` — `Session isolation after logout and re-login` | chromium, firefox | Session integrity | Medium | Playwright Dev | P0 | Explicitly clear and verify storage/session before re-login step |
| 4 | `tests/core/multi-component-workflows.spec.ts` — `WAF enforcement applies...` | chromium, firefox | Security enforcement contract | Medium | Backend Dev + Playwright Dev | P0 | Gate assertions on config-reload completion signal |
| 5 | `tests/core/multi-component-workflows.spec.ts` — `Security enforced even on previously created resources` | chromium, firefox | Security regression risk | Medium | Backend Dev + Playwright Dev | P0 | Add module-enabled verification helper before traffic checks |
| 6 | `tests/core/data-consistency.spec.ts` — `Data created via UI ... readable via API` | chromium, firefox | Core CRUD integrity | Medium | Playwright Dev | P0 | Introduce API-response synchronization checkpoints |
| 7 | `tests/core/data-consistency.spec.ts` — `Data deleted via UI is removed from API` | chromium, firefox | Data correctness | Medium | Playwright Dev | P0 | Verify deletion response then poll API until terminal state |
| 8 | `tests/core/data-consistency.spec.ts` — `Pagination and sorting produce consistent results` | chromium, firefox | User trust in data views | High | Playwright Dev | P0 | Stabilize table wait + deterministic sort verification |
| 9 | `tests/tasks/long-running-operations.spec.ts` — `Backup creation does not block other operations` | chromium, firefox | Background task reliability | Medium | Playwright Dev | P1 | Replace fixed waits with condition-based readiness checks |
| 10 | `tests/tasks/long-running-operations.spec.ts` — `Long-running task completion can be verified` | chromium, firefox | Operational correctness | Medium | Playwright Dev | P1 | Wait for terminal task-state API response before UI assert |
| 11 | `tests/core/admin-onboarding.spec.ts` — `Logout clears session` | chromium, firefox | Login/session contract | High | Playwright Dev | P1 | Ensure logout request completion + redirect settle criteria |
| 12 | `tests/core/auth-long-session.spec.ts` — `maintain valid session for 60 minutes` | chromium, firefox | Auth platform stability | Low-Medium | Backend Dev + Playwright Dev | P1 | Isolate token-refresh assertions and instrument refresh timeline |
| 13 | `tests/modal-dropdown-triage.spec.ts` — `CreateMonitorModal Type Dropdown` | chromium, firefox | Key form interaction | High | Frontend Dev | P1 | Harden locator strategy and modal-close sequencing |
| 14 | `tests/settings/user-management.spec.ts` — `should copy invite link` | chromium, firefox | Invitation UX | High | Frontend Dev | P2 | Provide stable copy-control locator and await render completion |
| 15 | `tests/dns-provider-types.spec.ts` — `script path field when Script type selected` | chromium, firefox | Provider config UX | High | Frontend Dev | P2 | Align field visibility assertion with selected provider type state |

---

## Operational Notes

- This ledger is Phase 6 tracking output and should be updated after each full-suite rerun.
- Next checkpoint: attach exact fail + skip lists from JSON reporter output and reconcile against this queue.
- Phase handoff dependency: Queue approval unlocks Phase 7 cluster remediation execution.
