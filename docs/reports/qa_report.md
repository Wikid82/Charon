# QA Report: Route Handling in the Proxy Layer

- Branch: `fix/proxy-route-handling` (3 commits over `origin/development` `d7a3af60a`, HEAD `17eef751c`)
- Scope: how the proxy configuration generator builds routes for a proxy host (`backend/internal/caddy/config.go`), the unit tests that pin those routes, one Playwright case in `tests/security-enforcement/acl-enforcement.spec.ts`, and the matching `ARCHITECTURE.md` section.
- Change type: `fix:` (security scope). CodeQL Go was run locally as the spec requires; Trivy is deferred to CI (no dependency or image input changed).
- Date: 2026-11-09

## Verdict

**PASS. No blockers.** Every gate run locally passed.

## Generated Routes (behaviour)

For each enabled proxy host the generator now emits:

1. One route per custom location, matching the location path. Handlers: security pre-handlers, host-level handlers, then the reverse proxy.
2. **Only when the host's main upstream is Charon itself** (same detection the generator already uses for the proxy hop headers; upstreams marked remote never qualify): a recovery route matching exactly `/api/v1/emergency/security-reset` and `/emergency/security-reset`, with no wildcard. Handlers: host-level handlers, then the reverse proxy. The reset endpoint performs its own token and management-network checks.
3. The main route, matching the host with no path matcher. Handlers: security pre-handlers, host-level handlers, then the reverse proxy.

Every other path, including everything else under `/emergency/` and `/api/v1/emergency/` (token endpoints included), is served by the main route or a location route and runs the security pre-handlers first. Hosts whose upstream is not Charon get no recovery route.

Route table observed in the throwaway container (three hosts: allow-listed host, allow-listed host proxying Charon, WAF-only host):

| Host | Path matcher | Handlers |
|---|---|---|
| WAF-only host | none | waf, headers, headers, vars, reverse_proxy |
| Host proxying Charon | `/api/v1/emergency/security-reset`, `/emergency/security-reset` | headers, headers, vars, reverse_proxy |
| Host proxying Charon | none | waf, access list, headers, headers, vars, reverse_proxy |
| Allow-listed host | none | waf, access list, headers, headers, vars, reverse_proxy |

## Gate Results

| Gate | Result | Evidence |
|---|---|---|
| Diff review against the design (spec section 3, D1) | PASS | Recovery route only behind the self-upstream check; exact paths from one `resetRecoveryPaths` list; locations and main route unchanged and still start with the security pre-handlers; remote and agent upstreams excluded by the existing remote flag; redirect and location tests updated, not removed |
| `go build ./...`, `go vet ./...` | PASS | Clean |
| `go test ./internal/caddy/... ./internal/api/... ./internal/cerberus/... -count=1` | PASS | All packages `ok` (see note on one non-reproducible handlers failure below) |
| Backend coverage (`scripts/go-test-coverage.sh`) | PASS | Statement 93.2%, line 92.1% (gate 87%). Zero failures |
| Local patch coverage (`scripts/local-patch-report.sh`, strict) | PASS | Backend 100% (20/20 changed lines), overall 100% |
| `make lint-fast` | PASS | 0 issues (backend and agent) |
| `make lint-backend` (golangci-lint v2.15.0, full) | PASS | 0 issues |
| GORM security scan | Not triggered | No model, query or migration changes |
| CodeQL Go (local, CLI 2.27.2, `go-security-and-quality`) | PASS | 4 results, all matched by non-expired entries in `.github/codeql/codeql-suppressions.yml`; 0 blocking; extraction parity 394/394. None in the diff |
| Container gate (throwaway container built from this branch, own network, loopback-only ports, removed afterwards) | PASS | See below |
| Playwright `acl-enforcement.spec.ts --project=security-tests --retries=0` | PASS | 9 passed (5.2 s) |
| `npm run lint:e2e` | PASS | 124 warnings, 0 errors (budget 124); none in the touched spec |

### Container gate

Requests were sent from a separate container whose address is on no list.

| Check | Request | Result |
|---|---|---|
| (a) Allow-listed host refuses a non-listed client | `/`, `/emergency/x`, `/emergency/..;/admin`, `/api/v1/emergency/token/x`, `POST /api/v1/emergency/security-reset`, `POST /emergency/security-reset` | 403 on every path |
| (b) Host proxying Charon, other paths | `/`, `/emergency/x`, `/emergency/..;/admin`, `/api/v1/emergency/token/x`, `/api/v1/emergency/token/status` | 403 on every path |
| (b) Host proxying Charon, reset path without a token | `POST /api/v1/emergency/security-reset` | 401 from the reset handler |
| (b) Host proxying Charon, reset path with a valid token | `POST /api/v1/emergency/security-reset` | 200; modules disabled as expected; the allow-listed host then stops refusing |
| (b) Host proxying Charon, trailing-slash form | `POST /api/v1/emergency/security-reset/` | 403 (main route) |
| (c) WAF host, test payload under the prefix | `/emergency/x?q=<probe>`, `/api/v1/emergency/<probe>`, `POST /api/v1/emergency/security-reset?q=<probe>` | 403 on every path; `/` without the payload reaches the (absent) upstream: 502 |

On the host proxying Charon, the exact reset path is answered by Charon itself (route 2 above), so it is not subject to the proxy WAF or access list on that one host; the endpoint's own checks apply. This is the approved design (D1).

## CodeQL suppression re-review (D11)

| Rule | Location | Review by | Verdict |
|---|---|---|---|
| `go/cookie-secure-not-set` | `backend/internal/api/handlers/auth_handler.go:201` | 2026-12-05 | Still justified. `Secure` is false only for a plain-HTTP request whose connecting peer is local or private; every other request gets `Secure=true`. Behaviour and doc comment unchanged |
| `go/log-injection` (two taint paths, one sink) | `backend/internal/api/handlers/remote_server_handler.go:148` | 2026-11-27 | Still justified. The logged value is a `uint` ID; `RemoteServer.ID` is `json:"-"`, so the request body cannot set it |
| `go/log-injection` | `backend/internal/services/uptime_service.go:1551` | 2026-11-27 | Still justified. Logs the `uint remoteServerID` argument only |

None has expired. Two entries reach their review date on 2026-11-27 and need renewal (or a fix) before then.

## Notes

- **Non-reproducible test failure:** the first scoped run reported `FAIL` for `internal/api/handlers` (209 s) without the failing test in the captured tail. The package then passed twice (a standalone rerun and the full coverage run). The package has no changes in this branch. Recorded for follow-up if it recurs.
- **Branch base moved during the run:** the branch was rebased onto `origin/development` (`d7a3af60a`). The only tree difference from the earlier HEAD is one CI skill script, so results apply to the current HEAD.
- **Wording:** commit subjects, the commit bodies (empty), code comments, test names and this report describe behaviour only.
