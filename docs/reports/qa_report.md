# QA Report: Admin Whitelist and Access List Matching

- Branch: `fix/admin-exclusion-matchers` (9 commits on `development`, `5e7fa0bc7`..`cd75c2e87`)
- Scope: deny routes for blocked addresses, IP block-lists, country lists and local-network lists are built as one
  matcher set that excludes the admin whitelist; country and local-network lists honour the admin whitelist (also
  while no GeoIP database is available); the access list tester reports the proxy's decision; GeoIP reload errors
  return a stable `error_code` and a fixed message; documentation for the admin whitelist.
- Image under test: `charon:local` from `docker-rebuild-e2e` on the branch head.
- Date: 2026-11-10

## Verdict

All gates pass. Local patch coverage is 100% (391 of 391 changed lines) after the coverage fixes made in response to the first QA pass.

## Static checks and build

| Check | Result |
| --- | --- |
| `make lint-backend` (golangci-lint, full config) | 0 issues |
| `make lint-fast` (backend + agent) | 0 issues |
| `go build ./...` | pass |
| `go list -deps ./cmd/api \| grep -c caddymatchtest` | `0` (test helper not linked into the binary) |
| `lefthook run pre-commit` on the branch files (forced) | all hooks pass, including semgrep; `shellcheck` exits with "No files specified" because the branch changes no `.sh` files (artefact of forcing the hook, not a finding) |
| Frontend | untouched (`git diff origin/development..HEAD -- frontend/` is empty) |
| Models | untouched; GORM scan not required |
| `npm run lint:e2e` | 0 errors, 124 warnings (limit 124) |
| `npx tsc --noEmit -p tests/tsconfig.json` | pass |

## Backend tests

| Run | Result |
| --- | --- |
| `go test -race` scoped to access list, GeoIP, evaluate and reload tests in `services` and `handlers` | pass |
| `go test -race` for `caddy`, `caddy/caddymatchtest`, `api/routes`, `cerberus` | pass |
| Full backend with `-race` via `scripts/go-test-coverage.sh` (30 min per-package timeout) | pass, no failures |

A plain `go test -race` of `services` and `handlers` with Go's default 10 minute timeout stops on the timeout
(the slowest packages take 10-11 minutes under `-race`); the coverage script sets a 30 minute timeout for this reason.
Not related to this branch.

## Coverage

| Measure | Value |
| --- | --- |
| Backend statement coverage (full run) | 93.4% |
| Backend line coverage gate | 92.2% (minimum 87%) |
| Local patch coverage (`scripts/local-patch-report.sh`, strict) | 100% (391 of 391 changed lines) |

The branch initially left 27 changed lines without a test (an unreachable parse branch, a merge branch no caller uses, an
unknown-list-type branch and guard branches in the test helper). They were covered or removed, and the test helper
package is excluded from coverage like the existing `hubcachetest` helper.


## E2E (Playwright, `--project=security-tests`, after the image rebuild)

`tests/security-enforcement/acl-enforcement.spec.ts`, `tests/security-enforcement/security-headers-enforcement.spec.ts`,
`tests/security/self-proxy-requests.spec.ts`, run twice back-to-back against the same container:

| Run | Result |
| --- | --- |
| 1 | 24 passed (24.9 s) |
| 2 | 24 passed (22.2 s) |

No flaky or skipped tests.

## Container gates

Throwaway container from the rebuilt image, a control client on a private bridge network, and client containers
on separate bridge networks whose subnets are public ranges, so the proxy sees public addresses with known
countries (bundled GeoLite2 Country database):

| Client | Address | Country | Admin whitelist |
| --- | --- | --- | --- |
| usa | 216.160.83.56 | US | yes (`/32`) |
| usb | 50.114.0.10 | US | no |
| gbc | 81.2.69.142 | GB | no |
| sed | 89.160.20.112 | SE | no (added for the database-missing case) |

Admin whitelist: the control client and `216.160.83.56/32`. Cerberus and ACL on. Hosts forward to the app, so a pass
is 200 and a refusal is 403.

| Host | List |
| --- | --- |
| plain.qa | none |
| block.qa | IP block-list `50.114.0.0/24` |
| geoallow.qa | country allow-list `GB` |
| geoblock.qa | country block-list `US` |
| local.qa | local network only |

### Database available

| Host | usa | usb | gbc |
| --- | --- | --- | --- |
| plain.qa | 200 | 200 | 200 |
| block.qa | 200 | 403 `Access denied: IP blacklisted` | 200 |
| geoallow.qa | 200 | 403 `Access denied: Geographic restriction` | 200 |
| geoblock.qa | 200 | 403 `Access denied: Geographic restriction` | 200 |
| local.qa | 200 | 403 `Access denied: Not a local network IP` | 403 |

- G-A0 IP block-list: only the listed subnet's client is refused; the other clients pass.
- G-A1 country allow-list (GB): the whitelisted US client passes, the non-whitelisted US client is refused, GB passes.
  Country block-list (US): the same pattern.
- G-A1 local network only: the whitelisted public client passes; the other public clients are refused.
- The live Caddy config shows one matcher set per deny route, with the admin ranges as the first entry of `not`.

### Security decision

A manual `block` decision for `81.2.69.142`, after the next config apply:

| Host | usa | usb | gbc |
| --- | --- | --- | --- |
| plain.qa | 200 | 200 | 403 `Access denied: Blocked by security decision` |
| every listed host | as above | as above | 403 |

Only the decided address is refused, on every host; the other clients keep their list outcomes. A `block` decision
for the admin-whitelisted `216.160.83.56` has no effect (documented behaviour).

### Database removed

`/app/geoip/GeoLite2-Country.mmdb` removed (only the `.placeholder` left, `/app/data/geoip` empty), container
restarted. `GET /security/geoip/status` reports `available: false`; `GET /security/status` reports
`acl.reason: geoip_unavailable`.

| Host | usa | usb | gbc (decided) | sed |
| --- | --- | --- | --- | --- |
| geoallow.qa | 200 | 403 | 403 | 403 |
| geoblock.qa | 200 | 403 | 403 | 403 |
| block.qa | 200 | 403 | 403 | 200 |
| local.qa | 200 | 403 | 403 | 403 |

The whitelisted client passes the country lists; non-whitelisted clients are refused by them, including the SE
client that the US block-list admits while the database is present.

### Tester parity (G-A2)

`POST /access-lists/:id/test` for each client address and list, with and without the database:

- Every answer equals the proxy's outcome for the access list. Reasons seen: "Address is in the admin whitelist",
  "Blocked by geo whitelist: country is not in the allowed countries", "Not a private network IP (RFC1918)",
  "GeoIP database not available - country lists refuse every address except admin addresses".
- The tester evaluates the access list only. For the decided address `81.2.69.142` it reports the list's answer
  (for example allowed on `block.qa`) while the proxy refuses it because of the decision.
- A zoned address (`fe80::1%eth0`) returns 400 `invalid IP address`; an IPv4-mapped IPv6 address is evaluated as
  the IPv4 address.

### Reload errors (G-A3)

| Case | Response |
| --- | --- |
| Database file removed | 500 `{"error":"Failed to reload the GeoIP database","error_code":"geoip_reload_failed"}` |
| Corrupt file at `/app/data/geoip/GeoLite2-Country.mmdb` | same body |
| Second container with `CHARON_GEOIP_DB_PATH` naming a missing file | same body |
| Unauthenticated | 401 |

No response body contains a file path. The path and OS error appear only in the server log (`detail` field).

### Cerberus API middleware

Unauthenticated `GET /api/v1/setup` from the public clients:

| Enabled lists | usa | usb | gbc / sed |
| --- | --- | --- | --- |
| All four | 403 `Blocked by access control list` | 403 | 403 |
| Country lists only (database present) | 200 | 200 | 200 |
| Country lists only (database removed) | 200 | 200 | 200 |

Country lists remain "allowed" on `/api`; the local-network list refuses public unauthenticated callers whether or
not they are admin-whitelisted. These are the outcomes the branch keeps unchanged.

## Security review

- No Gotify tokens in the diff, test helpers, logs or report.
- Reload error bodies no longer carry OS error text; errors are logged through `util.SanitizeForLog`.
- The admin exclusion uses the connection address (`remote_ip`), so forwarded headers cannot claim it.
- Admin whitelist entries are validated on save (`invalid admin whitelist CIDR`), so the tester's admin range parse
  does not see malformed input from the API.
- Local CodeQL/Trivy not run (fix-scoped PR; CI runs both).

## Cleanup

All throwaway containers, networks and the data volume were removed. `charon-e2e` was rebuilt by the skill and left
running.
