# QA Report: Client Address, Proxy Trust and Management Access

- Branch: `fix/cerberus-client-address` (14 commits on `origin/development`, head `5d8f55729`)
- Scope: one client address for every proxy check, an operator-chosen trusted proxy list for the proxy layer, access
  lists that protect Charon's own interface (with upgrade seeding and lock-out guards), country lists on that
  interface, forwarded host and scheme honoured only from trusted proxies, live stream limits, the emergency reset
  reply under a slow apply, and the matching UI and docs.
- Images under test: a throwaway image built from the working tree (`pr25qa-charon:qa`; every build step was cached, so
  it has the same inputs as the current E2E image) and the previous branch image (`pr24-charon:local`) for the upgrade
  check. All containers, networks and volumes were throwaway (`pr25qa-*`) and were removed afterwards.
- Toolchain: Go 1.27.4, CodeQL CLI 2.27.2, Caddy 2.11.9 (in the image), golangci-lint 2.15.0.
- Date: 2026-11-15

## Verdict

**Approved once the QA fixes below are committed.** The branch is correct on every container check. QA found four
things that would have failed CI (two test fixtures, one stale test and two code scanning results) and one flaky E2E
check, and fixed them in the working tree, uncommitted. With those fixes every Definition of Done gate passes.

## QA fixes (uncommitted)

| File | Change | Why |
| --- | --- | --- |
| `backend/internal/api/handlers/security_geoip_database_test.go` | The fixture migrates the settings table; the "busy" test fails within 10 s if the first change never reaches its apply | Every GeoIP database change now reads the management access choice first. The fixture had no settings table, so two tests failed and the busy test blocked until the 30 min package timeout, which hid the rest of the package |
| `backend/internal/api/handlers/management_guard.go` | The GeoIP lock-out check is skipped for a handler built without a database; the check's error log is sanitised | A handler without a database (used by existing tests) panicked on reload. With no database there is no management choice to protect. The log line carried request-derived text (new code scanning result) |
| `backend/internal/api/handlers/security_toggles_test.go` | `TestACLForbiddenIfIPNotWhitelisted` rewritten as `TestACLEnabledWithoutManagementListsOutsideAdminAllowList` | It asserted the earlier rule for switching access control on. That decision now belongs to the management lists (covered in `acl_enable_guard_test.go`) |
| `backend/internal/api/handlers/management_guard_test.go` | Two tests: an allow-list whose stored rules cannot be used is answered 400 with the list named; an unusable hop peer keeps the hop's client address | Patch coverage on the guard's error branches |
| `.github/codeql/codeql-suppressions.yml` | The cookie entry's line range follows the sink, which moved when the forwarded-header helper left `auth_handler.go`; dated note added | The findings gate reported the entry as stale and blocking |
| `tests/security/live-stream-limit.spec.ts` | Each "stream is live" poll makes one API request | A stream sends a frame only when the server logs a line. Without traffic the check depended on unrelated activity and failed on a quiet container (2 of 2 runs); after the fix 6 of 6 passed |

## Gates run (all foreground or block-waited)

| Gate | Result |
| --- | --- |
| E2E image | Throwaway image built from the working tree (fully cached) and run as its own container; Playwright pointed at it with `PLAYWRIGHT_BASE_URL`. The live E2E container was not touched |
| Playwright `--project=security-tests`: `trusted-proxies`, `management-access`, `live-stream-limit`, `self-proxy-requests` | 21 passed, 1 failed (stream check, fixed above). After the fix `live-stream-limit` passed 6/6 with `--repeat-each=2` |
| Playwright `--project=security-tests`: `acl-enforcement`, `authorization-rbac`, `zzz-security-ui/access-lists-crud` | 178 passed |
| Playwright `--project=firefox`: `core/proxy-hosts`, `core/proxy-host-docker-source`, `proxy-host-drag-drop` | 47 passed, 13 skipped (existing environment skips: no Docker socket in the throwaway container) |
| `./scripts/scan-gorm-security.sh --check` | Pass: 0 critical, 0 high, 0 medium, 2 informational |
| `bash scripts/local-patch-report.sh` | Pass. Overall 92.2% (1046/1134), backend 91.4% (905/990), frontend 97.9% (141/144). Remaining uncovered lines are error branches (storage failures, logging) and test helpers |
| `scripts/go-test-coverage.sh` (`CHARON_TEST_TIMEOUT=60m`) | First run: `handlers` timed out (see QA fixes). After the fixes: pass, 93.8% statements / 92.7% lines, 0 failures. `TestPerf_GetStatus_Parallel_AssertThreshold` passed in the full run |
| `scripts/frontend-test-coverage.sh` | Pass. 4672 tests, statements 92.01%, branches 86.76%, functions 89.55%, lines 92.86% |
| `npm run type-check` / `npm run lint` / `npm run build` | Pass / pass (0 warnings) / pass |
| `go build ./...` | Pass |
| `lefthook run pre-commit --all-files` | Pass, all 20 steps (including semgrep, markdownlint, shellcheck, golangci-lint-fast) |
| CodeQL Go | First scan: 2 blocking (new log-injection result in the guard, stale cookie entry). After the fixes: 0 blocking, 4 existing results suppressed with documented reasons |
| CodeQL JS | 0 results |
| Trivy (throwaway image, HIGH/CRITICAL) | 2 HIGH, both existing upstream findings without a fix and already documented (one in the CrowdSec binaries, one in the Caddy binary). This branch changes no dependency. No new finding |
| Integration scripts (copies with `pr25qa-*` names, loopback-only ports, the throwaway image) | `rate_limit_integration` pass, `cerberus_integration` pass, `coraza_integration` pass (block and monitor), `crowdsec_decision_integration` pass after correcting the copy's readiness probe (see out-of-scope findings) |
| `make lint-backend` (run last) | 0 issues. `agent/` is unchanged, so `make lint-agent` was not needed |

## Container checks

Set-up: Charon, an nginx proxy that appends to `X-Forwarded-For`, a header-echo upstream and client containers with
fixed addresses on one private network. Proxy hosts covered an allow-list, a block-list, a local-network list, a
country allow-list (the bundled database), a plain host and a host that targets Charon itself.

| Check | Result |
| --- | --- |
| Generated proxy config uses only real placeholders | Pass. Tokens: `http.vars.client_ip`, `http.request.remote.host`, `http.request.header.X-Forwarded-For`, `http.request.host`, `http.request.scheme`, `http.request.local.port`, `geoip2.country_code`. The client address is used for the rate-limit key, `X-Real-IP`, the self-proxy client header and the `X-Forwarded-For` set right before `geoip2`. The connection address appears only in the self-proxy peer header. Every address matcher is `client_ip`; the server has strict mode and `X-Forwarded-For` as its header |
| Untrusted peer sends a forwarded address | Pass. Allow-list, block-list, local-network list, country list, rate-limit bucket, WAF `REMOTE_ADDR` rule and CrowdSec ban all judge the connection address. Rotating forged addresses share one rate-limit bucket. The upstream gets the peer as `X-Real-IP` and `X-Forwarded-For` |
| Trusted proxy (nginx listed) | Pass. The rightmost untrusted entry decides every check; a forged leftmost entry sent through nginx is ignored; two visitors behind nginx get separate rate-limit buckets; a CrowdSec ban on a visitor applies through nginx and not to another visitor who forges it |
| Country lookup | Pass. With a public visitor reported through two trusted hops, the country list follows the rightmost untrusted entry. The country-list host's upstream gets `X-Forwarded-For: <visitor>, <proxy>`; other hosts keep the full chain |
| Logs | Pass. The access log carries `client_ip` (visitor) and `remote_ip` (peer). CrowdSec's access-log acquisition parses every line. The log viewer search matches either address |
| Environment fallback | Pass. With the CI value of `CHARON_TRUSTED_PROXIES`, the proxy layer uses it minus the entry that is too broad (`fc00::/7`), reports `source: environment` and the dropped entry, and logs one start warning. The live proxy config matches |
| Trust list API | Pass. Invalid, too broad (`/7`, `/0`, IPv6 `/23`) and more than 64 entries are refused with their codes and the entry named; malformed bodies are 400; bare and IPv4-mapped addresses are canonicalised, loopback is dropped, a whole local range is accepted with a warning. Both settings keys are hidden from `GET /settings` and refused on `POST /settings` (also with odd case and spaces) and `PATCH /config`. A refused apply restores the previous value on PUT and DELETE (500). Standard users see only `has_proxies`, `source` and the loopback entries and get 403 on writes; anonymous callers get 401 |
| Upgrade seeding | Pass. A database from the previous image with access control on and six lists: the enabled allow-list, block-list and local-network list are chosen; the country list with the local-network option is named in a warning and not chosen; the plain country list and the disabled list are not chosen; one warning for the local-network list. The interface stays reachable. A second start does not seed again |
| Lock-out guards | Pass, all answer `management_access_would_lock_out`: saving the choice; switching access control on through `POST /settings` (also `" TRUE "`), `PATCH /config`, `POST /security/acl/enable` and `PATCH /security/acl`; editing, retyping or re-enabling a chosen list; removing the caller from the admin allow-list through `POST /security/config`, `POST /settings` and `PATCH /config`; dropping or deleting the trust list from behind the trusted proxy through the self-proxy hop; GeoIP upload and reload (409, database unchanged). An address in the admin allow-list is exempt; disabling a chosen list is allowed; a list used by the interface cannot be deleted |
| Unusable stored rules | Pass. An allow-type list refuses the caller with one warning; a block-type list is not applied with a warning; a guard names the list (400 `access_list_invalid`) |
| Exemptions and other routes | Pass. The agent connect route is not refused; with no list chosen nothing is restricted while proxy-host lists exist; login, health and setup status work for allowed callers; both emergency reset routes work for a caller the lists refuse and switch access control off |
| Same-host tunnel (sidecar sharing Charon's network namespace) | Pass. A local-network list refuses a public visitor and admits a local one, on a proxy host and on `/api/v1` through the self-proxy host. The emergency reset through the tunnel is judged by the visitor; the direct loopback reset works. The seed warning for local-network lists is logged (upgrade check) |
| Country lists without a database | Pass. Not enforced on the interface; the bell alert carries a separate entry with the management scope |
| Live view origin behind a Host-rewriting proxy | Pass on the logs, Cerberus logs and stats streams: 403 without an Application URL or trusted proxy; 101 with either; a cross-origin request and a forged `X-Forwarded-Host` from an untrusted peer stay 403 |
| Live stream limits | Pass. The ninth stream for one user closes with 1013 `stream_limit_reached` on the logs, Cerberus logs and stats streams; slots are released; eight users fill 64 streams and the next one closes with 1013. The Hecate stream uses the same quota (wired in routes, unit-tested; no tunnel exists in a test container) |
| Emergency reset under a slow apply | Pass. With the proxy paused, the Tier 2 reset answers 200 after the 30 s apply window; other Tier 2 routes keep answering. A non-loopback bind logs one start warning |
| Self-proxy hop headers | Pass. Forged hop headers sent to the management port or the proxy port change nothing |

## Code review notes

- The strict walk used to predict the caller address under a new trust list matches Caddy 2.11.9 (unparsable entries
  skipped, ports and zones removed, all trusted falls back to the peer).
- The removed interface check, the deprecated log handler and the unused public URL helpers have no remaining
  callers. The parity test pins the new decision function to the recorded outcome of the removed check for every list
  type, local-network option, address class and admin allow-list.

## Findings in scope (non-blocking)

| Severity | Finding | Suggested change |
| --- | --- | --- |
| Low | The upgrade seed's log line lists the chosen lists by UUID, while the release note tells operators to read the startup log; on a fresh install the line still says the lists were chosen "from the previous settings" | Log list names, and skip or reword the line when nothing was chosen |
| Info | The trust list keeps an address that a saved range already covers (for example `/32` inside a `/24`) | Optional: drop covered entries when normalising |

## Out-of-scope findings (not filed)

| Severity | Finding | Where | Suggested approach |
| --- | --- | --- | --- |
| Low, **security-sensitive (flagged)** | After an apply times out because the proxy stopped answering, the proxy can still process the abandoned load when it resumes. The stored value is restored, but the live proxy config differs until the next apply. Shared by every writer that uses the apply and restore path; not introduced here | `handlers/proxy_apply.go`, `caddy` manager apply and rollback | Fold into the planned shared apply-or-restore helper: re-apply the restored state, or check the live config after a timed-out apply |
| Low | `scripts/crowdsec_decision_integration.sh` waits for `GET /api/v1/` with `curl -f`; that path answers 404, so the script can never pass. CI only runs shellcheck on it | `scripts/crowdsec_decision_integration.sh:188` | Probe `/api/v1/health` |
| Low | Local Trivy reports the Caddy go-jose finding that `.grype.yaml` already accepts; `.trivyignore` has no matching entry | `.trivyignore` | Add the documented entry with the same review date |
