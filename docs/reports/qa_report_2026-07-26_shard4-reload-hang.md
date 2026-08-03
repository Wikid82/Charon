# QA Report — E2E Firefox (Shard 4/4) Reload-Hang RCA + Fix (PR #1166)

**Date:** 2026-07-26
**Branch:** `feature/orthrus` (PR #1166)
**Investigated run:** `https://github.com/Wikid82/Charon/actions/runs/30186328462/job/89789834178?pr=1166` (job "E2E Firefox (Shard 4/4)")
**Scope of this report:** RCA and fix for 3 E2E test failures in that run — `tests/settings/user-lifecycle.spec.ts`, `tests/settings/user-management.spec.ts`, `tests/tasks/long-running-operations.spec.ts` — all failing with `page.goto: Test timeout of 60000ms exceeded`.
**Full investigation writeup:** `docs/plans/current_spec.md` ("RCA + Fix Plan: E2E Firefox (Shard 4/4) CI Failures on PR #1166")

---

## Headline finding

**Not an application bug.** The originally-suspected mechanism — PR #1166's new Orthrus write-mode audit logging/rate limiting/locking leaking into global middleware and blocking unrelated `/login`, `/users`, `/proxy-hosts` traffic — was refuted with direct code and artifact evidence (see `docs/plans/current_spec.md` §2.2-§2.3): none of this PR's new locks/rate-limiter/audit-logger code paths are reachable without a connected Orthrus agent session or an explicit `PATCH .../orthrus/agents/:uuid` call, neither of which occurs in this shard; the three hung routes are served entirely by Gin's static-file `NoRoute` handler, which never touches the database.

**Confirmed root cause (2 of 3 failures, trace-confirmed with file/line precision):** a test-authoring defect in this repo's own Playwright E2E helpers. Firing a `page.goto()` within ~100ms-2s of a just-completed or still-settling prior navigation (a same-URL reload, or a fresh-URL navigation racing the app's own client-side post-login redirect) can fail to produce a Playwright-trackable navigation-commit event in Firefox — the destination page renders correctly (confirmed via screenshots, including a live WebSocket widget still updating on screen throughout one hang), but `page.goto()`'s promise never resolves, hanging until the 60s test timeout. See `docs/plans/current_spec.md` §2.5 for the full trace walkthrough.

**Failure #3** (`long-running-operations.spec.ts`'s `afterEach` hook) is very likely the same class of issue but could not be trace-confirmed — the only captured artifact from the investigated run was of its successful retry, not the original hang.

---

## Fix applied

All changes are test-code only; no `backend/` or `frontend/` application code was modified.

| File | Change |
|---|---|
| `tests/settings/user-lifecycle.spec.ts` | `navigateToLogin()`'s fallback path replaced a redundant same-URL `page.goto('/login')` with `page.reload({waitUntil:'domcontentloaded'})` — a reload is guaranteed to produce a fresh, distinct navigation-commit event, unlike a same-URL `goto()` fired while the prior navigation is still hydrating. |
| `tests/settings/user-management.spec.ts` | The "Navigate to users page directly" step now waits for the URL to actually leave `/login` (`page.waitForURL((url) => !url.pathname.includes('/login'))`) after `loginUser`/`waitForLoadingComplete` return, before firing the next top-level `page.goto('/users')`. This closes the race the trace evidence showed: `loginUser`'s fast paths resolve on network-idle, not on a URL assertion, so its own client-side post-login redirect can still be in flight. |
| `tests/settings/navigation-settle-regression.spec.ts` (new) | Two regression cases generalizing both confirmed failure shapes: a same-URL reload-after-fresh-navigation case (failure #1's shape), and a fresh-route-navigation-racing-post-login-redirect case using a non-login protected route (failure #2's shape, kept distinct from the first so it isn't a restatement). |
| `tests/tasks/long-running-operations.spec.ts` | The affected test quarantined with `test.fixme` + an inline comment pointing to this report and to `docs/plans/current_spec.md`'s Phase 0b, since its mechanism (same navigation race vs. simple test-body budget exhaustion before `afterEach` starts) was not resolved in this session — see "Deferred: Phase 0b" below. |
| `backend/internal/orthrus/server_test.go` | Added `TestOrthrusServer_SetAuditLogger_RegistersNoGlobalMiddleware` (defense-in-depth, independent of the E2E fix): a reflection-based structural check that `OrthrusServer` holds no `*gin.Engine`/`gin.IRouter`/`gin.HandlerFunc` field, plus a behavioral check that calling `SetAuditLogger` leaves an unrelated `gin.Engine`'s registered routes untouched. Documents/enforces the invariant this investigation relied on when ruling out the Orthrus-middleware hypothesis. |

---

## Deferred: Phase 0b (failure #3's mechanism)

`docs/plans/current_spec.md`'s Phase 0b calls for re-running `long-running-operations.spec.ts` with tracing forced on for every attempt, against a live Charon instance in the project's Docker Compose E2E environment, to determine whether the `afterEach` hang is the same navigation race or simple budget exhaustion.

**This was not carried out in this session.** The host this session is running on is the user's live home-server Docker host — it has a `charon:local` container that has been running for 12+ hours alongside the user's other production homelab services (media/management stack: `mealie`, `tautulli`, `prowlarr`, `seerr`, `wizarr`, etc.), not a disposable CI runner. Building and starting a fresh E2E Compose stack here would be a resource-intensive action on shared infrastructure that wasn't confirmed with the user, so it was deliberately not attempted. (Confirmed safe in the process: the live `charon` container's port 8080 is mapped to host port 8787, not 8080, so nothing this session did could have reached it; a stray `npx playwright test` invocation used to sanity-check the quarantine, which timed out with nothing listening on `127.0.0.1:8080`, was confirmed to have started no containers, processes, or webServer and left `docker ps` unchanged.)

Per `docs/plans/current_spec.md`'s own contingency for exactly this situation ("If Phase 0b is inconclusive after a reasonable, timeboxed effort, quarantine that one test with `test.fixme` + a tracked follow-up issue, and merge the rest"), the affected test was quarantined rather than guessed at. **Follow-up needed:** run Phase 0b (§4 of the plan) in the actual CI Docker environment or a disposable sandbox before re-enabling this test.

---

## Verification performed in this session

| Check | Result |
|---|---|
| `cd backend && go build ./...` | Clean |
| `go build ./internal/orthrus/...` | Clean |
| `go test ./internal/orthrus/...` | `ok` — all tests pass, including the new `TestOrthrusServer_SetAuditLogger_RegistersNoGlobalMiddleware` |
| `go vet ./internal/orthrus/...` | Clean |
| `golangci-lint run --config ../.golangci-fast.yml ./internal/orthrus/...` | `0 issues` |
| `npx playwright test --list` across the 4 modified/new spec files | All 50 tests parse correctly (no syntax/type errors); the quarantined test still lists as expected (Playwright lists `fixme` tests but skips their execution) |

**Not performed in this session, and why:** a full live Playwright execution of the fixed/new specs against a running Charon instance (the actual reproduction of the original hang, and confirmation the fix resolves it). This requires the project's Docker Compose E2E environment, which was deliberately not started here for the shared-infrastructure reason above. **This must be run before merge** — per `docs/plans/current_spec.md` §7's acceptance criteria: the 3 originally-failing specs passing locally (Firefox, `--retries=0`), and a full cross-browser `npx playwright test` run including at least one clean re-run of Shard 4/4 — as part of normal CI on push, or in an isolated sandbox/CI runner, not this shared host.

---

## Recommendation

**Not ready to merge without a CI run.** The RCA is sound and independently evidenced (trace artifacts, code-path tracing, container logs — see `docs/plans/current_spec.md`), and the fix for failures #1/#2 is scoped exactly to the confirmed mechanism with no application-code changes. But per this repo's Definition of Done, the actual Playwright execution proving the fix works — and Phase 0b's reproduction for failure #3 — still need to run in CI (or a disposable environment) before this is mergeable. `supervisor` and `qa-security` sign-off (per `docs/plans/current_spec.md` §7) are also still outstanding, since the Management-agent pipeline that was mid-review of this plan could not proceed further in-session; this fix was implemented directly against the already-Supervisor-approved plan.
