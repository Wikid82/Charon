# RCA + Fix Plan: E2E Firefox (Shard 4/4) CI Failures on PR #1166

**Type**: CI failure root-cause analysis + fix plan (single PR — additional commit(s) on the existing branch)
**Branch**: `feature/orthrus` (current working branch; no worktree, no new branch, per `CLAUDE.md`)
**Status**: DRAFT — pending Supervisor review
**Author**: `planning` agent
**Investigated run**: `https://github.com/Wikid82/Charon/actions/runs/30186328462/job/89789834178?pr=1166` (job "E2E Firefox (Shard 4/4)", commit `64bc3b8`, branch head `5d5fe3e8`)
**Supersedes**: previous contents of this file (the GH #1160/#1161 muzzle-parity spec plus its follow-ups — that work is already implemented on this branch; see `git log -- docs/plans/current_spec.md` for its history if it's needed for reference again)

---

## 1. Introduction

### 1.1 Objective

Determine why 3 unrelated E2E tests (`user-lifecycle.spec.ts`, `user-management.spec.ts`, `long-running-operations.spec.ts`) fail with `page.goto: Test timeout of 60000ms exceeded` in the Firefox Shard 4/4 CI job on PR #1166, and produce a fix plan. A prior investigation pass already ruled out disk pressure, an image-digest false alarm, and container/health-check startup problems. This plan's job was to test the working hypothesis handed off from that pass — that new Orthrus write-mode audit logging / rate limiting / locking got wired into **global** middleware and is blocking unrelated `/login`, `/users`, `/proxy-hosts` traffic — and, per `CLAUDE.md`'s mandatory Root Cause Analysis Protocol, to keep tracing upstream rather than stopping at the first plausible-looking cause.

### 1.2 Headline finding

**The working hypothesis is REFUTED by direct evidence.** This is not a backend regression introduced by PR #1166's Orthrus code, and — per a Supervisor-requested follow-up pass documented in §2.3 — it is not a SQLite-level lock/contention issue either. The three timeouts are real, but the failure point is client-side (inside the Firefox tab under test), not server-side.

**Update after a second evidence pass (this revision)**: the trace/video/screenshot artifacts from the investigated run (not pulled in the first pass) were downloaded and inspected directly, per a Supervisor review request. For 2 of the 3 failures (`user-lifecycle.spec.ts:314` and `user-management.spec.ts:1210`), this produces a **confirmed root cause with file/line precision**, superseding the earlier "leading hypothesis" framing entirely: the hangs are caused by the E2E test helpers themselves issuing a `page.goto()` call while a very recent prior navigation (to the same or a related URL) is still settling, which Firefox does not reliably surface to Playwright as a distinct, trackable navigation — so `page.goto()`'s promise never resolves, even though the application is healthy and the destination page is (or was about to be) fully and correctly rendered the entire time. This is a **test-authoring / Playwright-Firefox interaction defect in this repo's own E2E helpers**, not a defect in Charon's Go backend or React frontend application code, and — importantly — the evidence no longer supports the originally-suspected `react-router-dom` → `react-router` migration as the mechanism (see §2.5's revised conclusion). The third failure (`long-running-operations.spec.ts:75`) is very likely the same class of issue but the pulled trace only captured its successful retry, not the original hang, so it is recorded as **strongly suspected, not yet trace-confirmed** — see §2.6 and Phase 0b.

Per the RCA protocol's instruction not to guess and not to patch a symptom: the fix below is scoped only to what the evidence in §2.4-§2.6 directly supports, and the one remaining gap (failure #3's own trace) is called out as still open rather than papered over.

---

## 2. Research Findings

### 2.1 What changed in PR #1166 (`feature/orthrus` vs. merge-base `9a17b49b`)

34 commits, primarily under `backend/internal/orthrus/`, `agent/`, and Orthrus-specific frontend components. Relevant excerpt of the full diff stat:

```
backend/internal/api/handlers/orthrus_handler.go        |  60 +-
backend/internal/api/routes/routes.go                   |   8 +-
backend/internal/models/orthrus_agent.go                |  12 +
backend/internal/orthrus/muzzle.go                      | 195 +-
backend/internal/orthrus/server.go                      |  32 +-
backend/internal/orthrus/session.go                     |  52 +-
backend/internal/services/orthrus_service.go            |  15 +-
frontend/src/components/hecate/AgentWriteModeDialog.tsx | 189 ++
frontend/src/pages/AuditLogs.tsx                        |  31 +-
tests/orthrus-write-mode.spec.ts                        | 279 +++
... (agent/, scripts/ci/, docs/)
```

None of the three failing spec files were touched by this PR, and none exercise any Orthrus route. This shard's 179 tests do not include `tests/orthrus-write-mode.spec.ts`.

### 2.2 Where the PR's new locking / rate-limiting / audit-logging actually live

Traced every new `sync.Mutex`, `rate.Limiter`, and audit-log call this PR adds:

| Addition | Location | Scope |
|---|---|---|
| `writeLimiter *rate.Limiter` (token bucket, 0.5 req/s, burst 5) | `backend/internal/orthrus/session.go`, field on `AgentSession` | Per-connection. Constructed only `if writeEnabled` inside `NewAgentSession`; only reachable from `HandleWebSocket` when a real Orthrus agent dials in over WebSocket. |
| `auditLogger AuditLogger` | `backend/internal/orthrus/session.go`, `server.go` | Per-`AgentSession` / per-`Muzzle` field. Invoked only from within `Muzzle.ServeHTTP` (the Orthrus **external TCP proxy** handler, bound to its own listener/port — not the `:8080` Gin router) and from `AgentSession` write-path methods. |
| `orthrusServer.SetAuditLogger(securityService)` | `backend/internal/api/routes/routes.go:559-568` (8-line diff) | Called once at startup. Passes a dependency into the Orthrus package; does **not** register any `router.Use(...)` or `api.Use(...)` middleware. The existing global middleware chain (`EmergencyBypass`, `gzip.Gzip`, `SecurityHeaders` at router level; `OptionalAuth`, `cerb.RateLimitMiddleware`, `cerb.Middleware` at the `api` group level) is untouched by this PR. |
| Handler-level audit log on `PATCH /api/v1/orthrus/agents/:uuid` (`write_enabled` toggle) | `backend/internal/api/handlers/orthrus_handler.go` `Patch()` | Fires only when an operator PATCHes an Orthrus agent record with a `write_enabled` field present — one specific admin action, not a per-request hook. |

**Conclusion**: every new lock, rate limiter, and audit-log call this PR introduces requires an actual connected Orthrus agent (WebSocket session or external-proxy TCP listener) or an explicit `PATCH .../orthrus/agents/:uuid` admin call to execute at all. Shard 4 of this run does not touch Orthrus in any way. None of this PR's new synchronization primitives were even instantiated during the failing run, let alone contended.

### 2.3 Direct evidence from the `docker-logs-firefox-shard-4` container-log artifact — including the SQLite-lock hypothesis, closed

Downloaded via `gh api repos/Wikid82/Charon/actions/artifacts/8631695831/zip` (the artifact from the exact failing run, `created_at: 2026-07-26T12:00:33Z`, matching the job's own timestamps) and cross-referenced line-by-line against the three failure windows from the Playwright report.

**Failure #1 window** (`user-lifecycle.spec.ts:314`, STEP 4, first attempt): the backend log shows a normal, fast request burst ending at `11:44:44Z` (last browser-originated request: `GET /api/v1/themes` → 401, expected post-logout). Then:

```
{"client":"::1", ... "path":"/api/v1/health","status":200,"time":"...11:44:48Z"}
{"client":"::1", ... "path":"/api/v1/health","status":200,"time":"...11:44:53Z"}
{"client":"::1", ... "path":"/api/v1/health","status":200,"time":"...11:44:58Z"}
{"client":"::1", ... "path":"/api/v1/health","status":200,"time":"...11:45:03Z"}
... (continues every 5s, sub-millisecond latency, uninterrupted) ...
{"client":"::1", ... "path":"/api/v1/health","status":200,"time":"...11:45:38Z"}
{"client":"172.18.0.1", "method":"DELETE", "path":"/api/v1/users/15", ..., "time":"...11:45:41Z"}   ← next test attempt begins
```

`"client":"::1"` is Docker's own `HEALTHCHECK` (`wget --spider http://localhost:8080/health`, per `ARCHITECTURE.md`'s Docker Compose example), issued from inside the container itself, entirely independent of anything the Firefox browser does. It kept succeeding every 5 seconds, at sub-millisecond latency, for the entire ~57-second span in which the test's `page.goto('/login')` was hanging.

**Supervisor review flagged a real gap here**: `GET /health` (`backend/internal/api/handlers/health_handler.go`) touches no database/GORM call at all — it proves the Gin engine, goroutine scheduler, and global middleware chain were alive, but says nothing about a SQLite-level lock (a busy writer, WAL contention, or a lock scoped to a specific route/middleware rather than truly global). This needed closing with either the DB-touching healthcheck (`GET /api/v1/health/db`, registered at `routes.go:243`) or another concrete source, not left as an assumption. Both were pursued:

1. **`GET /api/v1/health/db` timing, checked against the same artifact.** This endpoint is real (`backend/internal/api/handlers/db_health_handler.go`) and genuinely DB-touching — it runs `database.CheckIntegrityDedicated(h.dbPath)`, a `PRAGMA quick_check`-class integrity scan, deliberately on a **dedicated connection**, per the handler's own doc comment, specifically *because* an earlier version ran it on the shared pool and could block the whole app for the duration of a multi-minute scan on a large database (a real, code-documented precedent for exactly the kind of DB contention risk this hypothesis was chasing). It turned out to be client-driven (`"client":"172.18.0.1"`, not `::1` — i.e., invoked by the frontend, likely a periodic status widget, not Docker's healthcheck), and a full-text scan of the artifact shows it is **not called at all until `11:52:32Z`** — after failure #1's entire window (`11:44:44Z`-`11:45:41Z`) and after failure #2's window (ending `~11:52:12Z`), and only twice (`12:00:21Z`, `12:00:22Z`) near failure #3, both **after** that failure's hang had already ended (`11:58:48Z`-`~12:00:04Z`). **Stated plainly, per the review's own instructions**: `/health/db` was not being polled during any of the 3 actual hang windows, so its timing cannot be used as direct empirical proof for those specific windows — the honest state is that this specific endpoint doesn't settle the question either way for this run, not that it confirms the backend was fine.
2. **A stronger, direct alternative: reading the exact code path each hung navigation executes, to check whether it can touch SQLite at all.** All 3 hangs are Playwright `page.goto()` calls to `/login`, `/users`, and `/proxy-hosts` — none are `/api/*` routes. `backend/internal/server/server.go`'s `router.NoRoute(...)` handler (the catch-all Gin serves these through) is:
   ```go
   router.NoRoute(func(c *gin.Context) {
       path := c.Request.URL.Path
       if path == "/api" || strings.HasPrefix(path, "/api/") {
           c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
           return
       }
       c.File(frontendDir + "/index.html")
   })
   ```
   `c.File(...)` is a static-file serve — zero GORM, zero SQLite, for any of the three specific routes that hung. The only global (`router.Use()`) middleware ahead of it that takes a `db` parameter is `EmergencyBypass(managementCIDRs, db)` (`backend/internal/api/middleware/emergency.go:32`) — read in full: `db` is accepted as a parameter but **never referenced anywhere in the function body** (confirmed by reading the entire file; every branch is a header/IP/token check, no `db.` call exists). `gzip.Gzip` and `SecurityHeaders` (the other two global middlewares) are pure response-transform/header-setting code with no DB access either (confirmed by reading `backend/internal/api/middleware/security.go` in full). **So the exact code path executed for all 3 hung routes never acquires a database connection at any layer** — this is a direct, code-level proof (not an inference from a proxy endpoint's timing) that a SQLite lock, however it might arise elsewhere in the app, cannot be the cause of these specific hangs, regardless of what `/health/db` was or wasn't doing during the window.
3. **Bracketing evidence, for completeness**: genuinely DB-touching requests (`PUT /api/v1/users/16`, `POST /api/v1/auth/login` at `11:44:43Z`; `DELETE /api/v1/users/15` at `11:45:41Z`, `POST /api/v1/users` at `11:45:43Z`) succeeded at normal (sub-100ms) latency both immediately before and immediately after every hang window in the artifact. A SQLite writer-lock or WAL-busy condition severe enough to matter does not spontaneously clear itself in the exact instant each hang ends and then recur in the exact instant the next one begins, three separate times, while never once producing a slow or errored DB-touching request anywhere else in the log. Also worth noting for completeness (found while reading `db_health_handler.go`): the shared GORM pool is configured `SetMaxOpenConns(1)` — a single-connection pool is a real, legitimate contention risk *in general* (any handler holding that one connection blocks every other DB-touching request), which is exactly why `Check` was deliberately moved onto a dedicated connection. This is a correct thing to know about this codebase, but it is irrelevant to explaining these 3 hangs specifically, since point 2 already shows none of the hung routes ever request that pool's one connection in the first place.

**Conclusion, now closed on both the Gin-engine-liveness axis and the SQLite-lock axis**: the Go HTTP server was never blocked (health-check evidence, §2.3 original), and the specific routes that hung cannot have been blocked by a SQLite lock even in principle, because their entire execution path never touches the database (code-level proof, points 2-3 above). The failure point is not the backend.

Separately: a full-text search of the container log for any request from the browser's client IP (`172.18.0.1`) during each ~57-60s hang window returns **zero rows** — not a slow request, not a partial/truncated one, nothing. The same gap pattern was confirmed for the other two failures. A backend regression (SQLite-related or otherwise) cannot produce that symptom by definition — there is nothing for the backend to be slow *at* if the request never arrives. §2.4-§2.6 below establish, via the run's trace/video artifacts, exactly why it never arrives.

**Corroborating detail**: `ps-aux.txt` / `free-m.txt` / `df-h.txt` from the run's own diagnostics artifact (`e2e-diagnostics-firefox-shard-4`, id `8631695485`) show a healthy runner throughout (15.6 GB RAM, 10.2 GB free at snapshot time; 88 GB free disk; load average 1.19) — no resource-exhaustion signal at the OS level either.

### 2.4 Pattern shared by all 3 failing assertions

| Test | Failing call | What immediately preceded it |
|---|---|---|
| `user-lifecycle.spec.ts:205` (`navigateToLogin`, called from STEP 4) | `page.goto('/login', {waitUntil:'domcontentloaded'})` | STEP 3 just completed `logoutButton.click()` + `page.waitForURL(/login/)` (succeeded). STEP 4 then checks `emailInput.isVisible()`; per `loginWithCredentials`, if not visible, it calls `navigateToLogin()` again — the first `page.goto('/login')` inside that call (line 186) had *just* completed (~84ms earlier, per the trace — §2.5), and this second, redundant `page.goto('/login')` (line 205) is the one that hangs. |
| `user-management.spec.ts:1227` | `page.goto('/users', {waitUntil:'domcontentloaded'})` | Immediately preceded by `logoutUser(page)` then `loginUser(page, regularUser)` (a fresh login), inside the same test step that then does a fresh top-level navigation immediately afterward — racing the app's own client-side post-login redirect (§2.5). |
| `long-running-operations.spec.ts:75` (`afterEach`) | `page.goto('/proxy-hosts', {waitUntil:'domcontentloaded', timeout:10000})` | The test body itself just finished a `Login during backup completed in 1179ms` step (another full login cycle) immediately before `afterEach` fires. |

Every failure is a Playwright `page.goto()` — a genuine top-level browser navigation — and every one immediately follows an auth-state transition (logout, login, or both), fired within roughly 100ms-1s of a preceding navigation to the same page or a page the app is about to redirect away from. No failure is an in-app client-side route transition (`<Link>`/`navigate()`), and no failure is a plain API call. Every API call logged in the same windows completed in tens of milliseconds.

### 2.5 Confirmed mechanism (trace evidence): a raced `page.goto()` never gets a trackable navigation event, while the app stays fully healthy

The originally-suspected `react-router-dom` → `react-router` migration (merged into this branch from `development`, not authored by this PR — see the superseded analysis this replaces, below) was a reasonable hypothesis with no trace evidence available. On Supervisor's request, the trace-containing artifacts from the same investigated run — `playwright-report-firefox-shard-4` (id `8631694705`) and `playwright-output-firefox-shard-4` (id `8631695221`), not pulled in the first pass — were downloaded (`gh api repos/Wikid82/Charon/actions/artifacts/<id>/zip`) and inspected directly: Playwright trace files (`*.trace`, JSONL action/network logs), `error-context.md` DOM snapshots, and `screencast-frame` JPEGs. This is now a **confirmed** mechanism for 2 of the 3 failures, not a hypothesis.

**Failure #1 (`user-lifecycle.spec.ts:314`), trace `94104822bf03be11ddb7c39dec177132f8097e36.zip`**:

- `2-trace.trace` shows two `goto` calls to `/login`: `call@77` (`startTime:112004`, this is line 186's `page.goto('/login')`) and `call@89` (`startTime:112249.66`, this is line 205's fallback `page.goto('/login')`) — **only 245ms apart**.
- `call@77` completed normally (`"after"` event at `endTime:112165.858`). Its `after@call@77` DOM snapshot shows the document mid-hydration: `<link rel="modulepreload" href=".../Login-*.js">` still present, and the visible content is a full-screen `"Loading..."` spinner overlay, **not** the rendered Login form yet.
- `call@89` fires 84ms after `call@77` completes — while that first navigation's JS module graph (`Login-*.js`, `Card-*.js`, `Input-*.js`) is still in flight. `call@89` has a `"before"` event, a `frame-snapshot`, and a `"log"` line in the trace, but **no `"after"` event anywhere in the file** — it never resolved, matching the reported 60s timeout.
- **Screenshot evidence pinpoints exactly what was on screen throughout the entire hang**: the frame immediately after `call@89` fires (`page@...-1785066346500.jpeg`) shows the same `"Loading..."` spinner. The frame captured near the very end of the trace, ~55.8 seconds later (`page@...-1785066402251.jpeg`), shows the **fully and correctly rendered Login form** (email field, password field, Sign In button) — meaning the application had already finished loading and rendering the correct destination page well within the hang window. Playwright's `page.goto()` promise for `call@89` never resolved despite this.
- **Cross-referenced against the backend log**: only **one** `GET /login` request appears in the container log for this entire attempt (`11:44:43Z`, 304µs). If `call@89` had produced a genuine second top-level navigation, a second `GET /login` should appear in the log ~245ms later. It does not. The browser never even sent the second request over the wire.

**Failure #2 (`user-management.spec.ts:1210`), trace `2140c230918f91489ecb3aa202b5f4898a64c3c7.zip`**:

- The hung call is `call@103` (`goto("/users", waitUntil:"domcontentloaded")`, `startTime:380539.515`, wall-clock `~11:50:14.78Z`) — confirmed to never receive an `"after"` event (versus the immediately-preceding `call@82`/`call@48`, both of which completed normally).
- The `frame-snapshot` taken immediately before `call@103` fires shows `frameUrl: "http://127.0.0.1:8080/login"` — i.e., from Playwright's own tracking, the frame's last known URL was still `/login` at the exact moment the test asked it to navigate to `/users`, even though the preceding `logoutUser`/`loginUser` step (which includes a login-success client-side redirect to `/`) had just run. This is the `/users`-and-Dashboard analogue of failure #1's same-URL race: the new top-level `goto()` was issued while the app's own very-recent client-side navigation (post-login redirect) was still settling.
- Screenshot at the end of the hang (`page@...-1785066669174.jpeg`, ~55s into the wait) shows the **Dashboard fully rendered** — sidebar, all four stat tiles, "Statistics" section, and critically **"Stats Service Health: Live — Connected via WebSocket"**, meaning the app's own real-time WebSocket connection was actively alive and updating on screen the entire time. The app was never stuck, blank, or erroring — it just never left the page Playwright's `goto()` was trying to navigate away from.
- Cross-referenced against the backend log at the trace's precise wall-clock window (`11:50:14.78Z` to at least `11:51:09Z`): **zero** `GET /users` requests in that span (the nearest ones are `11:50:12Z`, 2 seconds *before* the hang started, and `11:52:16Z`, roughly 2 minutes after — both from different attempts/steps). Same signature as failure #1: the browser never dispatched the request.

**Unifying conclusion**: in both trace-confirmed cases, the specific `page.goto()` call that hangs is issued within roughly 100ms-2s of a just-completed or still-settling prior navigation (a same-URL reload in failure #1; a fresh-URL `goto()` racing the app's own client-side post-login redirect in failure #2). In neither case does the browser ever dispatch the corresponding HTTP request, and in both cases the application itself finishes rendering correctly, fully, and without error well inside the 60-second window — the "Stats Service Health: Live" widget in failure #2 is direct, on-screen proof the app was actively functioning, not frozen. This is consistent with a known category of Playwright/Firefox limitation: firing a new top-level navigation while a very recent navigation to the same or a related location is still resolving can fail to produce a distinct, Playwright-trackable navigation-commit event, leaving `page.goto()`'s internal wait with nothing to resolve against until the test timeout fires. **This is a defect in this repo's own E2E test helpers' navigation pattern (calling `page.goto()` redundantly, back-to-back, without waiting for the app to fully settle first) interacting with a Firefox/Playwright navigation-tracking edge case — not a defect in Charon's backend or frontend application code, and not something introduced by PR #1166.**

**Revised conclusion on the router migration (superseding §2.5 as originally written)**: the `react-router-dom` → `react-router` migration (commits `839f791c`, `4ac65862`, merged in from `development`) remains on the table only as a *possible, unproven* contributor to timing sensitivity (e.g., if it changed how quickly the Login page hydrates relative to when the test's `isVisible()` check runs, that could affect how often the race in failure #1 is lost) — but the trace evidence does not implicate it directly, and the confirmed mechanism above (test helpers issuing back-to-back `goto()` calls) does not require the migration to explain any of the observed behavior. It would be overclaiming to keep asserting the migration as "the leading hypothesis" now that direct trace evidence points somewhere more specific and better-supported. It is recorded here for traceability, not as the fix target.

Also checked and still ruled out as contributing: `frontend/src/api/client.ts`'s axios 401 interceptor already has explicit redirect-loop prevention and a 30-second request timeout — neither is implicated by (nor explains) a client-side navigation-event race.

### 2.6 Failure #3 (`long-running-operations.spec.ts:75`) — same symptom class, not yet trace-confirmed

The only trace artifact captured for this test (`a291410bf84e39b879070c80ba5eef2d351e9c5d.zip`) corresponds to its **successful retry**, not the original failing attempt — all 5 `goto` calls in that trace (`/`, `/settings/backup`, `/settings/backup`, `/proxy-hosts`, `/users`) completed normally with `"after"` events, matching the fast (144ms-1179ms) navigation times already seen in the container log for the retry. Playwright's trace-capture setting for this run did not retain a trace for the first, failing attempt, so this failure cannot be given the same file/line-precision confirmation as failures #1 and #2 from this artifact set alone.

What *is* known, from the original container-log analysis: the error is "Test timeout of 60000ms exceeded **while running `afterEach` hook**" — Playwright's overall 60s test-level budget (which covers the test body plus all hooks) was exhausted while `afterEach`'s `page.goto('/proxy-hosts', {timeout:10000})` (line 77) was in flight, not that specific call's own 10-second timeout. This leaves two live possibilities, not yet distinguished: (a) the same navigation-event-race mechanism as failures #1/#2, triggered by `afterEach` firing immediately after the test body's own `Login during backup completed in 1179ms` step (another recent auth transition, fitting the pattern); or (b) the test body itself (concurrent backup creation + multiple navigation/API assertions) simply consumed most of the 60s budget under this run's CI load, leaving `afterEach` too little time regardless of any navigation-tracking issue. Phase 0b (§4) exists specifically to close this remaining gap before this failure's own fix is finalized — it should not be assumed to be the same bug as #1/#2 without its own trace.

---

## 3. Diagnosis Summary

| Hypothesis (from task background) | Verdict | Evidence |
|---|---|---|
| Audit-logging/rate-limiting wired into global Gin middleware, blocking all routes | **Refuted** | `routes.go` diff is 8 lines, no new `router.Use()`/`api.Use()`; all new locks/logging live inside Orthrus-only code paths never invoked by this shard (§2.2) |
| Backup operation holds a DB-level or in-memory mutex blocking unrelated request paths | **Refuted** | No new mutex touches backup/task execution in this PR's diff; independently, the backend answered Docker's own healthcheck every 5s throughout every hang window — nothing was blocked backend-side (§2.3) |
| SQLite `database is locked` contention from synchronous audit-log writes | **Refuted, closed with code-level proof (not just healthcheck timing)** | The non-DB healthcheck alone doesn't prove this (Supervisor's flag, correctly raised); closed instead by reading the exact code path all 3 hung routes execute (Gin's `NoRoute` static-file handler, plus the global `EmergencyBypass`/`gzip`/`SecurityHeaders` middleware ahead of it) and confirming none of it ever acquires a DB connection, plus bracketing evidence that genuinely DB-touching requests succeeded at normal latency immediately before and after every hang window (§2.3) |
| Rate limiter using a blocking/synchronous store | **Refuted** | The only new rate limiter (`session.go`'s `writeLimiter`) is a per-`AgentSession`, in-process `golang.org/x/time/rate.Limiter` (non-blocking semantics, no shared store), never constructed unless a real write-enabled Orthrus agent connects |
| `CHARON_EMERGENCY_SERVER_ENABLED` interacting with this PR's changes | **Refuted** | Emergency-reset calls complete in single-digit milliseconds throughout; no Orthrus code touches the emergency server package |
| Frontend client-side navigation stall correlated with the react-router-dom→react-router migration | **Superseded** | Trace evidence (§2.5) points to a more specific, confirmed mechanism that does not require the migration to explain it; kept here only for traceability |
| **Test-helper `page.goto()` race: a navigation fired within ~100ms-2s of a still-settling prior navigation never produces a Playwright-trackable event, while the app itself renders correctly throughout** | **Confirmed for failures #1 and #2 (file/line precision, trace evidence); strongly suspected but not yet trace-confirmed for failure #3** | §2.5 (trace walkthrough for #1/#2); §2.6 (why #3 is not yet at the same confidence level, and what closes that gap) |

**This is not an ordinary environmental/flaky-infra dead end** — 2 of the 3 failures now have a confirmed, evidence-backed, file/line-precise mechanism, not a hypothesis. The honest remaining gap is failure #3, which is very likely the same class of issue but lacks its own trace (§2.6); Phase 0b below exists specifically to close that, and the plan does not assume the answer in advance.

---

## 4. Implementation Plan

### Phase 0 — Reproduction spike: COMPLETE for failures #1 and #2 (this revision)

The reproduction spike originally planned here was carried out against the artifacts from the investigated run itself, per Supervisor's request, rather than a fresh local run — `playwright-report-firefox-shard-4` (id `8631694705`) and `playwright-output-firefox-shard-4` (id `8631695221`) were downloaded (`gh api repos/Wikid82/Charon/actions/artifacts/<id>/zip`) and their `error-context.md` DOM snapshots, Playwright `*.trace` JSONL action/network logs, and `screencast-frame` JPEGs inspected directly. This produced the confirmed mechanism in §2.5 for failures #1 and #2 — no further reproduction is needed for those two before writing the fix.

### Phase 0b — Reproduction spike still required for failure #3

**Goal**: distinguish, with the same rigor now applied to #1/#2, whether `long-running-operations.spec.ts`'s `afterEach` hang (§2.6) is the same navigation-race mechanism or simply the test body consuming most of its 60s budget before `afterEach` starts.

1. Re-run just this spec with tracing forced on for **every** attempt (not just retries, since the existing artifact only captured the successful retry):
   ```
   npx playwright test tests/tasks/long-running-operations.spec.ts --project=firefox --trace=on --retries=0
   ```
2. If it fails on the first attempt under this config, inspect the trace exactly as done for #1/#2 in §2.5: check whether `afterEach`'s `page.goto('/proxy-hosts')` (line 77) call ever produces an `"after"` event, whether a matching `GET /proxy-hosts` appears in the container log at the corresponding wall-clock time, and what the screencast frames show on-screen throughout.
3. If it does *not* fail locally (timing-sensitive), instead measure how much of the 60s budget the test *body* alone consumes in a representative run — if it's already routinely >45-50s before `afterEach` even starts, mechanism (b) from §2.6 (budget exhaustion, not a navigation race) is the more likely explanation, and the fix is a test-timeout/scope adjustment (e.g. split the test, or extend its own `test.setTimeout()`), not the same navigation-pattern fix as #1/#2.

### Phase 1 — Fix (confirmed scope for failures #1 and #2; failure #3 per Phase 0b's outcome)

This is a **test-code fix, not an application-code fix** — §2.5's trace evidence shows the application (both backend and frontend) functioning correctly throughout every hang window; the defect is in how the E2E helpers themselves call `page.goto()`.

- **`tests/settings/user-lifecycle.spec.ts`, `navigateToLogin()` (lines 184-209, hang at line 205)**: the fallback path currently does `clearCookies()` + `localStorage.clear()`/`sessionStorage.clear()` then an immediate second `page.goto('/login', {waitUntil:'domcontentloaded'})` when `emailInput.isVisible()` returns false moments after the *first* `goto('/login')` (line 186) completed. Per §2.5, that first navigation's JS modules were still loading when the check ran (a hydration-timing race, not an app bug), and the resulting second `goto()` to the same URL never produces a trackable navigation event. Fix: replace the redundant same-URL `page.goto()` with `page.reload({waitUntil:'domcontentloaded'})` (a `reload()` is guaranteed by Firefox/Playwright to produce a fresh, distinct navigation-commit event, unlike a same-URL `goto()`), and/or give the first navigation's hydration more time before evaluating `isVisible()` (e.g. an explicit short wait or a hydration marker) so the fallback path is taken less often in the first place.
- **`tests/settings/user-management.spec.ts:1218-1228` ("Navigate to users page directly" step)**: `page.goto('/users', ...)` is fired immediately after `loginUser(page, regularUser)` returns, racing the app's own client-side post-login redirect (still showing `frameUrl: /login` per the trace at the moment `goto()` fires). Fix: after `loginUser`/`waitForLoadingComplete` returns, wait for the URL to actually settle away from `/login` (e.g. `await page.waitForURL((url) => !url.pathname.includes('/login'))`, or reuse `waitForLoadingComplete`'s own settling semantics if it already exposes one) before firing the next top-level `goto()`.
- **`tests/tasks/long-running-operations.spec.ts` `afterEach` (lines 75-99)**: pending Phase 0b's outcome — if the same race, apply the equivalent "wait for the preceding navigation/redirect to settle before the next `goto()`" fix; if budget exhaustion, adjust the test's own timeout/scope instead.
- **No `frontend/src/**` changes are in scope for this fix.** The originally-anticipated targets (`Login.tsx`, `AuthContext.tsx`) are not implicated by the trace evidence — the application rendered correctly and (in failure #2's case) was actively serving a live WebSocket connection on screen throughout the hang. Reopening application code here would not be tracing to the actual root cause.

### Phase 2 — Regression tests

- Per Supervisor's review: the original regression-test scope only codified the `/login` double-goto shape (failure #1). Generalized here to cover **both** confirmed failure shapes, not just the login one:
  - A Playwright regression test (its own commit, not folded into the fix commit) exercising "log out, then request `/login` again via `page.goto` before the SPA has fully hydrated" — codifying the `navigateToLogin` double-goto pattern (failure #1's shape).
  - A second Playwright regression test exercising "log in, then immediately `page.goto()` to a different top-level route before the app's own post-login redirect has settled" (failure #2's shape) — using a protected, non-login route (e.g. `/users` or `/proxy-hosts`) so this is a genuinely distinct case from the first, not a restatement of it.
- If Phase 0b confirms failure #3 is the same race, add a third case for "navigate immediately after a login inside a test's own body/hook boundary" (the `afterEach`-adjacent shape) rather than assuming the first two cases already cover it.
- Explicitly **not** in scope: any new backend/Orthrus test to "cover" this bug — §2.2 already demonstrates the Orthrus audit-logger/rate-limiter/mutex code this PR added is correctly scoped (per-connection, never global), so there is no backend regression here to cover. As a cheap defense-in-depth addition (see Commit 4 below), a small Go unit test asserting `orthrusServer.SetAuditLogger` registers nothing on the shared `*gin.Engine` keeps that invariant regression-tested rather than merely argued in this doc.

### Phase 3 — Verification

- Re-run the full Definition of Done from `CLAUDE.md` (Playwright E2E full suite across all browsers, not just the 3 previously-failing specs, since the fix touches shared test-helper navigation patterns reused across many spec files).
- Specifically re-run Firefox shard 4/4 in isolation at least twice to build confidence against the ~100%-reproducible failure pattern.
- Confirm `npm run type-check` and `go build ./...` both remain clean (the fix is test-code-only; no application code changes are anticipated).

---

## 5. Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Phase 0b (failure #3) still needs a fresh reproduction run, unlike #1/#2 which are already trace-confirmed | Timebox Phase 0b; if inconclusive, treat failure #3 independently per its own risk row below rather than blocking the now-confirmed #1/#2 fix |
| The `page.reload()`/settle-wait fix changes test timing/behavior broadly, since these helpers are reused across many spec files | Run the full cross-browser E2E suite (Phase 3), not just the 3 originally-failing specs, before merge |
| Fix scope creeps into unrelated router-migration or other test-infrastructure cleanup | Phase 1 is explicitly scoped to the exact mechanism confirmed in §2.5; do not open a general "audit all E2E navigation helpers" workstream under this PR — file a separate follow-up issue if broader patterns are noticed, per `CLAUDE.md`'s One-Feature-One-PR rule |
| Failure #3 turns out to be budget exhaustion (test body too slow), not the same navigation race | Fix its test timeout/scope independently rather than forcing the #1/#2 fix pattern onto it; note this explicitly in the PR description so reviewers don't expect one uniform fix across all 3 |
| Failure #3's root cause remains inconclusive even after Phase 0b | Quarantine that one test with `test.fixme` + a tracked follow-up issue, and merge the now-confirmed #1/#2 fix rather than blocking the whole PR on the least-understood of the three |

---

## 6. Commit Slicing Strategy

**Decision**: Single PR (#1166, `feature/orthrus`), additional ordered commits on top of the existing 34. This is a CI-unblocking bug fix for the branch this feature already lives on — not a new feature and not a new PR, per `CLAUDE.md`'s "One Feature = One PR" / "Slice Commits, Not PRs" rules.

| Commit | Scope | Files | Depends on | Validation gate |
|---|---|---|---|---|
| **1. `test(e2e): reproduce and confirm long-running-operations afterEach hang (Phase 0b)`** | Targeted re-run/trace-capture for failure #3 only, per §4 Phase 0b; this commit's own output (trace + a short written conclusion in the commit message or PR description) is the deliverable, not a code fix yet | `tests/tasks/long-running-operations.spec.ts` (temporary trace-config override if needed), no production test-helper changes yet | none | Must not change existing test outcomes; produces a concrete answer to "same race as #1/#2, or budget exhaustion?" before commit 2 |
| **2. `fix(e2e): stop racing page.goto() against still-settling prior navigations`** | The confirmed Phase 1 fix: `navigateToLogin()` in `user-lifecycle.spec.ts` (use `page.reload()` instead of a same-URL `page.goto()`, and/or wait longer before the `isVisible()` check); the "Navigate to users page directly" step in `user-management.spec.ts` (wait for the post-login redirect to settle before the next `goto()`); `long-running-operations.spec.ts`'s `afterEach` per commit 1's finding | `tests/settings/user-lifecycle.spec.ts`, `tests/settings/user-management.spec.ts`, `tests/tasks/long-running-operations.spec.ts` | Commit 1 (for the #3 portion; #1/#2 portions can land even if commit 1 is still pending, since they're already trace-confirmed) | The 3 previously-failing specs pass locally, Firefox project, `--retries=0` |
| **3. `test(e2e): regression coverage for goto/navigation-settle races`** | The two (or three, per Phase 0b) generalized regression cases from §4 Phase 2 — covering both the login-reload shape and a non-login protected-route shape, not just the login one | New or extended spec file(s) under `tests/` | Commit 2 | `npx playwright test --project=firefox` targeted at the new cases passes |
| **4. `test(orthrus): assert SetAuditLogger registers no global gin middleware` (defense-in-depth, independent)** | One small Go unit test documenting/enforcing the §2.2 invariant this investigation relied on | `backend/internal/orthrus/server_test.go` (or similar) | none | `go test ./...`; `make lint-fast` |
| **5. `docs: record Shard-4 reload-hang RCA in qa_report.md`** | Summarize this investigation (including the closed SQLite-lock question and the confirmed trace mechanism) for future reference (mirrors this branch's existing pattern of recording muzzle-parity QA audits in `docs/reports/qa_report.md`) | `docs/reports/qa_report.md` | Commits 1-4 | Docs-only; markdown lint |

**Rollback / contingency for the PR as a whole**: failures #1 and #2 are now confirmed and fixable independently of failure #3's outcome — do not let Phase 0b's timeline block landing commits 2/3 for the two already-understood cases. If Phase 0b is inconclusive after a reasonable, timeboxed effort, quarantine `long-running-operations.spec.ts`'s affected test with `test.fixme` + a tracked follow-up issue referencing this doc, and merge the rest. If `qa-security`/`supervisor` judge any residual risk here to be user-facing rather than test-only, that's a call for `supervisor` to make, not this plan, since it changes the PR's risk posture rather than its CI mechanics.

---

## 7. Acceptance Criteria (Definition of Done)

- [ ] Failure #3's mechanism determined via Phase 0b (same race as #1/#2, or budget exhaustion) before its own fix commit is written.
- [ ] `tests/settings/user-lifecycle.spec.ts`, `tests/settings/user-management.spec.ts`, `tests/tasks/long-running-operations.spec.ts` pass locally against the fix, Firefox project, `--retries=0`.
- [ ] Full `npx playwright test --project=firefox` (and, before final merge, the full cross-browser matrix per existing CI) passes, including at least one clean re-run of Shard 4/4 specifically.
- [ ] `cd frontend && npm run type-check` clean (no application-code changes anticipated, but the gate still runs per standard process).
- [ ] `cd backend && go build ./...` and `go test ./...` clean (only if commit 4 is included).
- [ ] `scripts/frontend-test-coverage.sh` ≥ 85% maintained.
- [ ] `lefthook run pre-commit` clean (staticcheck/CodeQL/muzzle-parity-checker all still pass — none of this fix touches the muzzle allowlist files, so the parity checker is expected to be a no-op gate here).
- [ ] This investigation's findings recorded permanently in `docs/reports/qa_report.md` (commit 5), consistent with this branch's existing practice for the GH #1160/#1161 investigation.
- [ ] `supervisor` sign-off obtained before merge, per standard pipeline.
- [ ] `qa-security` sign-off obtained before merge — even though the confirmed fix (§4 Phase 1) turned out to be test-code-only rather than touching `AuthContext.tsx`'s token-handling lifecycle (the original concern that motivated this checkbox), the investigation and fix are still squarely in the login/auth-transition flow, and an independent security-focused review of the final diff (confirming no application-code auth path was quietly touched, and that the new/changed test assertions don't weaken any existing security-relevant coverage) is warranted before merge.
