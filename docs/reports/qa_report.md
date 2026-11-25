# QA Report: Name Checks for Proxy Upstream Connections

- Branch: `fix/upstream-name-filter` (9 commits on `origin/development`, head `cd416997e`)
- Plan: `docs/plans/pr34b_dns_stub_plan.md` (rev 2), sections 4.7 (edge cases), 6 (commit gates) and 7 (test plan,
  container gates, Definition of Done)
- Scope: forward target names are looked up through a small name server on loopback each time the proxy opens a
  connection, and addresses that forward targets may not use are left out of the answer; names on ports Charon keeps
  for itself also leave out this machine; the config check accepts reverse proxy handlers from advanced configuration;
  advanced configuration that sends a named upstream or health-check target through a transport that cannot use the
  name checks is refused; the OrbStack host name is treated like the other container host names; cached host
  addresses refresh in the background; the Security page shows one muted line when the name checks are not running.
- Image under test: the E2E image rebuilt from the head above (`charon:local`, tagged `charon:pr34b` for the gates).
  Control image: release `0.51.6`. All gate containers, networks and volumes were throwaway (`pr34b-*`, published
  on `127.0.0.1` high ports, rootless daemon) and were removed afterwards; `docker ps`, network and volume lists
  match the state before the gates. The live containers `charon-dev` and `orthrus-dev` were not touched.
- Toolchain: Go 1.27.4, Caddy v2.11.9, CodeQL CLI 2.27.5, CoreDNS 1.12.1 (pinned by digest), busybox 1.37 (pinned
  by digest).
- Date: 2026-11-25.

## Verdict

**Approved.** No blocking items. Every test suite, build, lint, scan and container gate that was run passes. The
OrbStack and Podman gates were not run (neither runtime is installed); the docs-site strict build is left to CI
(Zensical is not installed locally).

## Definition of Done

| Gate | Result |
| --- | --- |
| Playwright, firefox only: `tests/core/proxy-host-upstream-names.spec.ts`, `tests/core/proxy-hosts.spec.ts`, `tests/core/proxy-host-docker-source.spec.ts` | 52 passed |
| GORM security scan | Not triggered: no model, query or migration change (the only new `db.Create` is a test fixture) |
| Local patch report (`scripts/local-patch-report.sh`) | 92.2% overall, backend 92.2% (643 changed lines, 593 covered); both artifacts written |
| Backend coverage (`scripts/go-test-coverage.sh`) | All tests pass; 94.2% statements, 93.2% lines (gate 87%) |
| Frontend coverage (`scripts/frontend-test-coverage.sh`) | 380 files, 4734 tests pass; 92.91% lines (gate 87%) |
| `go test -race` | Full packages: `network/upstreamdns`, `caddy`, `cmd/api`, `api/routes` pass. `services` and `api/handlers` with `-run 'Upstream\|Resolver\|Rollback\|Apply\|Load\|Status\|Listener\|Forward\|Probe\|Gateway\|HostSide\|Cache\|Preflight\|Advanced\|Dial\|Health\|Reserved'` pass (the full race run of these two packages is left to CI because of its length) |
| CodeQL (`lefthook run codeql --all-files`) | Go: 1 result, outside the changed files (see below); JS: 0. `codeql-check-findings` passes |
| `lefthook run pre-commit --all-files` | All hooks pass, including semgrep, golangci-lint-fast, markdownlint and locale lint |
| `go build ./...`, `npm run type-check`, `npm run build` | Pass |
| `make lint-backend` (full config, gosec on test files), run last | First run: 1 gosec G115 on a test counter conversion in `backend/internal/network/upstreamdns/server_test.go:107`; QA added `// #nosec G115 -- small test counter` (the repo convention) and re-ran: 0 issues |
| `make lint-agent` | Not needed: no change under `agent/` |
| Trivy | Left to CI: no new feature and no dependency change |

### Patch coverage

Uncovered changed lines, per file (all files are above the 85% floor):

| File | Patch coverage | Uncovered changed lines |
| --- | ---: | --- |
| `backend/internal/caddy/manager.go` | 80.0% | 937-941, 958-959, 964-965, 971-972 (encode and decode error returns in the load path; the client fallback for test doubles) |
| `backend/internal/services/upstream_names.go` | 87.7% | 76-77, 79-81, 84-85 (bodies of the log callbacks for lookup errors, read errors and recovered panics) |
| `backend/internal/network/upstreamdns/server.go` | 88.7% | 94-97, 111-115, 181, 227-228, 238-239, 270-271 (second-listener bind failure cleanup, the IPv6 loopback fallback, rare error branches) |
| `backend/internal/network/upstreamdns/answer.go` | 89.7% | 139-140, 144-145, 149-150, 153-154, 158-159 (packet builder error returns) |
| `backend/internal/caddy/dial_walk.go` | 90.5% | 118, 150-151, 182-184 |

The frontend change shows 0 changed lines in the patch report: the coverage output has no line entries for the new
JSX lines in `Security.tsx`. The new Vitest test (`Security.upstreamNameChecks.test.tsx`) covers the line being
shown when `available` is `false` and hidden otherwise.

### CodeQL

The one Go result is `go/cookie-secure-not-set` at `backend/internal/api/handlers/auth_handler.go:181`. That file is
not changed on this branch, the call carries a `codeql[go/cookie-secure-not-set]` suppression comment with its
reasoning, and `codeql-check-findings` passes. No result touches the new name server, the load path or the
Security page.

## Container gates

| Gate | Steps | Result |
| --- | --- | --- |
| G0 | `caddy version` in the image | v2.11.9 |
| G1 | User network, CoreDNS records changed after save | Name to a LAN address: 200. Same name changed to a link-local address: 502, Caddy logs a lookup failure (`no such host`), no connection; the answerer logs one warning without the name or address. Name with one refused and one LAN address: the LAN address is served (3 of 3). Running config: every `reverse_proxy` has `transport.resolver` on `udp/127.0.0.1:<port>`; no Charon snapshot in `/app/data/caddy` contains a resolver |
| G1 control | Same steps on release `0.51.6` | Link-local: Caddy logs `dial tcp …:8080: i/o timeout` after 3 s (a connection was attempted). `0.0.0.0`: 200 (the request reached a service on the container's own loopback) |
| Reserved-port listener | Name on port 8085 and name on port 8080, both changed to `127.0.0.1` after save | 8085: 502, warning tagged `listener: reserved`, handler pointed at the reserved listener. 8080: served (allowed by design) |
| G6 | Name changed to `0.0.0.0` | 502, no connection |
| Reload and rollback | Update a host (reload); then save a host whose advanced config Caddy refuses | Reload: resolver present, served. Refused load: API answers 500 with the Caddy error, Charon rolls back, the reinstated snapshot is loaded with the resolver on both listeners, sites served, refused addresses still filtered, the refused host is not stored |
| G2 | Default bridge, CoreDNS as the container's name server (no Docker DNS) | LAN: 200; link-local: 502 |
| G3 | Host-networking emulation: shared network namespace, loopback stub resolver on `127.0.0.53` | LAN: 200; link-local: 502; `0.0.0.0`: 502; `127.0.0.1:8080`: served |
| G4 | IPv6 user network | AAAA-only ULA name: 200. `fe80::1`: 502. `::ffff:` mapped link-local: 502. AAAA query for an A-only name: answer without addresses |
| G5 | `--add-host host.containers.internal:<LAN>` | Served from the hosts file; CoreDNS received no query for the name. Real Podman: not run |
| G7 | `--dns-search a.test --dns-search pr34b.test --dns-option ndots:2`, forward host `app` | Served when the first candidate answers; when the first candidate does not exist; and when every address of the first candidate is refused (the client moves on to the second candidate) |
| G8 | CoreDNS stopped | Name hosts: 502 after about 3 s; literal host: 200; health: 200; no answerer log line names a host or address |
| Answerer start failure | Caddy and Charon in two containers sharing one network namespace; UDP sockets refused only in Charon's container | Error logged once ("Upstream name checks are not available …"); config loaded without a resolver; literal site 200; `GET /api/v1/security/status` gives `upstream_name_checks: {available: false}`; the Security page shows the muted line in Firefox, and a normal container does not |
| G9 | Volume first used by `0.51.6`, then this build, then a restart | Served on both; resolver present; the answerer port changes on restart; the snapshots written by `0.51.6` are byte-identical afterwards and no snapshot contains a resolver |
| G10 | 500 sequential `Connection: close` requests, alternating with `0.51.6` | 500/500 OK on both; median 7.13 ms against 6.52 ms (difference 0.61 ms, bound 2 ms); p95 11.04 ms against 9.86 ms |
| G11 | Requests every 100 ms during `docker stop` | 200 until "Server shutdown complete"; failures (connection refused) start only after Caddy exits 5 ms later; no 502 |
| G-OrbStack, real Podman | | Not run (not installed) |

Notes on the harness: a first attempt at the start-failure gate refused UDP for the whole container, which also
stops Caddy's HTTP/3 listener, and a one-port ephemeral range also starved Caddy's own lookups; both were discarded
and the split-container setup above was used.

## Commit and text checks

- Exactly one commit uses the `(security)` scope; every commit body is empty; no advisory or scanner ids and no
  session links in messages or added lines.
- Added lines contain none of the reserved terms from the plan header.
- Added docs describe current behaviour only; the branch removes one existing "now" from `docs/features/orthrus.md`.

## Observations (in scope, not blocking)

1. The Caddy error log line for a refused name reads `lookup <name> on 127.0.0.11:53: no such host`: Go names the
   server from `resolv.conf` even though the query went to the loopback answerer. Readers may think Docker DNS
   failed. A short note in the troubleshooting docs would help.
2. When IPv4 loopback ports run out, the second listener binds on `[::1]` and the start succeeds with one IPv4
   and one IPv6 listener (seen in a discarded harness run). Lookups through the `[::1]` listener were not tested.
3. The patch report does not count JSX-only lines in `.tsx` files, so frontend patch coverage reads 100% for this
   change. This comes from the tooling, not from the branch.
