# QA Report: Handling of Sensitive Requests

- Branch: `fix/sensitive-request-handling` at `a2a3d00de` (13 commits on `development` `c3383e14c`).
- Scope: spec `docs/plans/cerberus_hardening_batch6_spec.md` section 5 (PR 23b). Items: 455 (query token kept out of the connection list, logs and streamed log entries), 430 (live log socket limits), 898 (key-containing export formats always ask for the password), 589 (backup before a certificate delete follows the encryption setting), 805 (management networks), 870 (emergency reset attempt limits).
- This is a re-verification after the fixes for the four items that blocked the first review (patch coverage, lint, E2E attempt budget, log socket test race).
- Image under test: `charon:local`, rebuilt from `a2a3d00de` with `docker-rebuild-e2e`.
- Date: 2026-11-10

## Verdict

**NOT APPROVED (one test-only blocker).** All four earlier blockers are resolved: patch coverage is 100%, `make lint-backend` reports 0 issues, the E2E specs stay well inside the failed-attempt budget, and the earlier race in `TestConfigureLogSocket_PongRefreshesReadDeadline` no longer appears. Every behaviour check passes. One new test, `TestWriteLogSocketPing_SetsWriteDeadline`, is flaky under `-race` (B5). It does not affect application behaviour, but CI runs the backend suite with `-race`, so it must be fixed before merge.

## Earlier Blockers

| ID | Item | Status | Evidence |
| --- | --- | --- | --- |
| B1 | Patch coverage 91.0% | RESOLVED | `scripts/local-patch-report.sh` (strict, baseline `origin/development`): 281/281 changed backend lines covered, 100.0%. Frontend and agent: no changed executable lines |
| B2 | `make lint-backend` reported 2 issues | RESOLVED | `make lint-backend`: 0 issues (8 linters, exit 0) |
| B3 | E2E reset tests had no spare room in the failed-attempt budget | RESOLVED | See "E2E Attempt Budget" below: 1 charged attempt per listener per CI run, at most 3 with `retries: 2`, against a budget of 5 |
| B4 | Data race in `log_socket_deadlines_test.go` | RESOLVED for the reported race; see B5 | The pong-refresh test no longer leaks its ping goroutine. In three `go test -race -count=15` loops over the log socket tests, the only race report came together with a B5 timeout |

## Blocking Item

| ID | Item | Evidence | Remediation |
| --- | --- | --- | --- |
| B5 | `TestWriteLogSocketPing_SetsWriteDeadline` is flaky under `-race` | `go test -race -count=15 -run 'LogSocket\|ConfigureLogSocket\|WriteLogSocket' ./internal/api/handlers/` failed in 2 of 3 loops. Each failure was one run of this test at 30.0 s ("ping writes never hit the write deadline", `log_socket_deadlines_test.go:227`); the other 14 runs took 7-9 s. One of the two failures also reported "race detected during execution of test". The test relies on 2,000,000 empty pings filling the socket buffers of a client that never reads. Loopback buffers grow automatically (receive maximum on the test host: 32 MiB), so whether and when the writes block depends on the kernel, not on the code under test. When the test times out, its handler goroutine keeps calling `writeLogSocketPing`, which reads `logSocketWriteWait` after the test's cleanup has restored it, so the race is a consequence of the timeout | Make the buffer size deterministic: in the handler, set a small send buffer on the underlying connection (`conn.NetConn().(*net.TCPConn).SetWriteBuffer(4096)`), and dial with a `websocket.Dialer` whose `NetDialContext` sets a small receive buffer on the client side. Then loop until the first error instead of a fixed count. Make the test wait for the handler goroutine before cleanup (for example close the client and receive from `writeErr` in a `t.Cleanup` registered after `withLogSocketTimings`). Apply the same buffer setup to `TestWriteLogSocketJSON_SetsWriteDeadline`, which passes today only because its 64 KiB payloads fill the buffers quickly |

## Gate Results

| Gate | Result | Evidence |
| --- | --- | --- |
| `make lint-backend` | PASS | 0 issues |
| `go build ./...` (backend) | PASS | Clean |
| `go test -race` on `internal/ratelimit`, `internal/api/middleware`, `internal/server` | PASS | All ok |
| `go test -race -count=15` on the log socket tests | FAIL (B5) | 3 loops: 1 pass, 2 failures, all in `TestWriteLogSocketPing_SetsWriteDeadline` |
| Backend coverage (`scripts/go-test-coverage.sh`, full run with `-race`) | PASS | Statement 93.4%, line 92.3% (gate 87%), zero test failures |
| Local patch coverage (`scripts/local-patch-report.sh`, strict) | PASS | 100.0% (281/281) |
| Playwright `--project=security-tests --retries=0`, two runs back-to-back on one freshly rebuilt container: `emergency-token`, `emergency-reset`, `emergency-server/tier2-validation`, `zzzz-break-glass-recovery`, `acl-enforcement`, `security/websocket-connection-token` | PASS | Run 1: 33 passed (22.4 s). Run 2, started 25 s later: 33 passed (21.8 s) |
| Frontend gates (coverage, type check, build) | PASS (carried over) | No frontend files changed since the first review; results from that review stand (lines 92.52%) |
| GORM scan | PASS (carried over) | No model, query or migration changes since the first review |
| CodeQL and Trivy | Deferred to CI | The fix commits add no new feature surface; the first review's local scans were clean for this branch |

## E2E Attempt Budget

Only a 401 from the reset handler (wrong or missing emergency token) spends the per-address budget: 5 attempts, refilling one per minute, separately for the main listener and the Tier 2 listener. Requests refused earlier (Basic Auth on the Tier 2 listener, the management network check, the ACL, or a route the reset endpoint does not serve) spend nothing.

Every spec that sends a reset request was audited (`grep -rn security-reset tests/`):

| Spec | Requests that are refused | Charged on main | Charged on Tier 2 |
| --- | --- | --- | --- |
| `security-enforcement/emergency-token.spec.ts` | One wrong token in "Emergency reset requires a valid token and never throttles successful resets" | 1 | 0 |
| `security-enforcement/emergency-server/tier2-validation.spec.ts` | One wrong token ("should reject invalid emergency token on Tier 2"); one request without Basic Auth (refused before the throttle) | 0 | 1 |
| `security-enforcement/emergency-server/emergency-server.spec.ts` | One request without Basic Auth (refused before the throttle) | 0 | 0 |
| `security/self-proxy-requests.spec.ts` | One request without Basic Auth (refused before the throttle) | 0 | 0 |
| `security-enforcement/acl-enforcement.spec.ts` | A GET to the reset path (the throttle guards only POST) and a POST to `/emergency/security-reset` on the main listener (not a reset route there) | 0 | 0 |
| Setup files and all other specs (`auth.setup.ts`, `security-shard.setup.ts`, `fixtures/security.ts`, `utils/security-settings-api.ts`, `crowdsec-lifecycle`, `zzz-admin-whitelist-blocking`, `user-lifecycle`, `multi-component-*`, `security-dashboard`) | Correct token only. Contexts with `httpCredentials` first receive a Basic Auth challenge (401 before the throttle) | 0 | 0 |
| **Total per CI run** | | **1** | **1** |
| **Worst case with `retries: 2`** (both tests fail on every attempt) | | **3** | **1 + 2 = 3** |

Each CI security job starts its own container, so budgets are not shared across jobs. The container logs from the two local runs match the table: one handler 401 on the main listener per run; on the Tier 2 listener, the Basic Auth challenges plus one handler 401 per run.

## Container Checks

Throwaway containers from `charon:local` with their own volume and Docker network; clients ran in separate containers at fixed addresses. Everything was removed afterwards.

### Query token in logs (PASS)

Setup: Cerberus and CrowdSec on, a proxy host whose upstream is Charon itself, and a Cerberus live-log socket open. Through the proxy host: two requests with a session token in the query string (`?token=…` and `?a=1&token=…&b=2`), one unknown path with the token, a live-log socket upgrade with the token, and five normal requests.

- **Container output (`docker logs`):** no line contains the token (checked for the full value and its first 40 characters). Normal proxy runtime messages still do (server start, config load, admin API, TLS and CrowdSec module messages: 46 lines). The application's own request lines show the path only.
- **Access log file** (`/var/log/caddy/access.log`): 20 lines, none with the token. URIs were written as `/api/v1/websocket/stats`, `/api/v1/websocket/stats?a=1&b=2`, `/api/v1/logs/live` and `/api/v1/does-not-exist`; other query parameters are kept (`/api/v1/health?n=1`). All lines are valid JSON.
- **Live-log stream:** 10 entries for the proxy host, none with the token.
- **CrowdSec:** `cscli metrics show acquisition` shows `file:/var/log/caddy/access.log` with 20 lines read and 20 parsed.
- **Data files:** a search of `/app/data` and `/var/log` found no file containing the token.

### Reset attempt limits and management networks (PASS)

Management networks set to one client address (A); a second client (B) is outside it.

| Client | Requests | Results |
| --- | --- | --- |
| B, main listener | 8 resets with the correct token, then 7 with a wrong token, then 1 correct | All 16 returned 403; none returned 429, so refused requests spend no attempts |
| B, Tier 2 listener (with Basic Auth) | 8 correct, then 7 wrong | All 403 |
| B, Tier 2 listener (no Basic Auth) | 7 | All 401 from Basic Auth |
| A, main listener | 1 correct | 200 |
| A, main listener | 7 wrong | 401, 401, 401, 401, 401, 429, 429 |
| A, main listener | 1 correct, right after | 429 with `Retry-After: 60` (as documented) |
| A, Tier 2 listener | 7 wrong | 401 five times, then 429 twice: a separate budget from the main listener |

## Findings

| ID | Severity | Finding | Evidence | Suggested follow-up |
| --- | --- | --- | --- | --- |
| F1 | Medium | Flaky log socket deadline test | B5 | Fix in this PR (B5) |
| F2 | Low | A management list that names only remote addresses also blocks resets from the Charon machine, including on the Tier 2 listener | First review, management network table | Document that `127.0.0.1` must be listed to keep resets from the Charon machine working, or always allow loopback on the Tier 2 listener |
| F3 | Info | Trivy HIGH findings in bundled third-party binaries with no upstream fix | First review | Already accepted in `.grype.yaml`; no action in this PR |

Resolved since the first review: the E2E attempt budget (now 1 per listener per run), non-401 refusals spending the budget (only 401 counts now, confirmed above), and the outdated limit comment in `tier2-validation.spec.ts`.

## Notes

- The shared `charon-e2e` container was recreated by `docker-rebuild-e2e` before the E2E runs. No other running containers were touched.
