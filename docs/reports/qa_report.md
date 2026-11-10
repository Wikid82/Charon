# QA Report: Access-List Enforcement

- Branch: `fix/access-list-enforcement` (HEAD `16c402eec`)
- Scope: country lists in the proxy (GeoIP handler and database state), allow-lists without usable rules, the ACL switch check on every settings path, security headers on refused requests, the shared local network ranges, the image build step for the GeoIP database, the related UI messages, locale keys, E2E specs and docs (spec `docs/plans/cerberus_hardening_batch6_spec.md` section 4). This pass re-verifies the follow-up commits: country codes stored in one case, list edits applied to the proxy, GeoIP reload reaching the proxy (also when the database was missing at startup), and the image keeping a downloaded database whose checksum is out of date.
- Image under test: `charon:local` rebuilt after the last application commit.
- Date: 2026-11-09

## Verdict

**APPROVED.** All earlier blocking items are resolved, patch coverage is back above the target (99.6%), and every behaviour check passes in throwaway containers. Three smaller findings (F2, F3, and the coverage gaps) were fixed after the re-check. One high finding in existing code (F1) is tracked separately as issue #1891 and ships as its own change.

## Blocking Items

| ID | Item | Evidence | Remediation |
| --- | --- | --- | --- |
| B1 (resolved) | Patch coverage was 97.7% (260/266 backend lines) | `scripts/local-patch-report.sh` (strict). Uncovered: `backend/internal/services/access_list_service.go:160-161` (count error in `InUse`), `backend/internal/api/handlers/access_list_handler.go:44-45` (`InUse` error in `applyIfInUse`), `backend/internal/api/routes/routes.go:951` (database file present but not loadable at startup), `backend/internal/caddy/config.go:1008` (country list with no codes) | Add unit tests: `InUse` with a closed DB; `Update` on a handler whose `InUse` fails (expect 500, nothing applied); route registration with an unreadable `.mmdb` at `CHARON_GEOIP_DB_PATH` (service exists, not loaded); `buildACLHandler` for a `geo_` list with empty `country_codes` (pins the generated expression) |

## Earlier Items: Status

| ID | Earlier item | Status | Evidence |
| --- | --- | --- | --- |
| Lint | `make lint-backend` reported 1 issue | Resolved | 0 issues |
| Reload | A replaced database did not reach the proxy | Resolved | Container check (b) below |
| Docs | Docs pointed to a reload control that does not exist | Resolved | Both pages now say: restart Charon or send `POST /api/v1/security/geoip/reload`; the reload works when the database was missing at startup (check (b)) |
| Case | Country codes saved in lowercase were not matched | Resolved | Container check (a) below |
| Edits | Editing an attached list did not reach the proxy | Resolved | Container check (c) below |
| R1 | Images could ship without a database after an upstream update | Resolved | A checksum mismatch now keeps the download and prints a build warning; only a failed or empty download drops the file and writes the marker. The local build verified the checksum and bundled the database |
| O1 | Startup line said lists "stay refused" after a database loaded | Resolved | The message now depends on whether the database loaded |
| O2 | GeoIP status message said "available" while unavailable | Resolved | Unavailable state reports `message: "GeoIP database unavailable"` with `reason: geoip_unavailable` |

## Gate Results

| Gate | Result | Evidence |
| --- | --- | --- |
| `make lint-backend` | PASS | 0 issues |
| `go build ./...` | PASS | Clean |
| Scoped `go test` (caddy, services, models, handlers, routes) | PASS | All packages ok |
| Backend coverage (`scripts/go-test-coverage.sh`) | PASS | Statement 93.3%, line 92.2% (gate 87%); zero failures |
| Frontend coverage (`scripts/frontend-test-coverage.sh`) | PASS | Lines 92.52%, statements 91.62%, branches 86.33%, functions 88.95%; 331 files, 4405 tests passed, 2 todo |
| Local patch coverage (`scripts/local-patch-report.sh`, strict) | PASS | Overall 99.6% (281/282), backend 99.6% (275/276), frontend 100% (6/6) |
| Playwright `acl-enforcement`, `security-headers-enforcement`, `rate-limit-enforcement` (`--project=security-tests --retries=0`) | PASS | 20 passed (11.1 s) on the rebuilt image |
| E2E image rebuild (`docker-rebuild-e2e`) | PASS | Build log: GeoIP downloaded and checksum verified |

CodeQL, Trivy and the GORM scan were run in the previous pass and passed. The follow-up commits are fixes with no new feature surface, so they are left to CI.

## Container Checks

Throwaway containers from `charon:local` with their own volumes (no tmpfs over `/app/data`), the API bound to loopback only, an admin allow-range for the API client, and three internal Docker networks whose subnets cover addresses in the MaxMind data. The bundled database resolved the clients as GB, SE and US. Each client request came from a separate container with a fixed source address. All containers, networks and volumes were removed afterwards.

### (a) Country codes saved in lowercase (PASS)

Lists created through the API with `country_codes: "gb"` (allow-list) and `"se"` (block-list) were stored as `GB` and `SE`, and the proxy matchers used the upper-case codes.

| Client | Allow-list host (GB) | Block-list host (SE) |
| --- | --- | --- |
| GB | 200 | 200 |
| SE | 403 | 403 |
| US | 403 | 200 |

### (b) GeoIP reload reaches the proxy (PASS)

- **Replaced database at the same path:** the file was replaced with a valid country database that holds no addresses. Before the reload the proxy kept the old answers. After `POST /security/geoip/reload` (200) the GB client was refused by the GB allow-list and the SE client passed the SE block-list. Replacing the bundled database again and reloading restored the original answers. The container did not restart.
- **Database missing at startup, then provided:** a container started with `CHARON_GEOIP_DB_PATH` pointing to a missing file refused every client on both country hosts. After copying the database to that path, the proxy still refused until the reload. `POST /security/geoip/reload` returned 200 with `loaded: true`, the status showed `available: true` with no reason, the `geoip2` app appeared in the proxy config, and the hosts answered as in check (a). No restart was needed.

### (c) Editing an attached list applies at once (PASS)

With no other apply in between:

- Changing the attached country allow-list from `gb` to `se` through `PUT /access-lists/:id`: GB 403, SE 200, US 403. Changing it back restored GB 200.
- Changing the attached IP allow-list from the GB subnet to the SE subnet: GB 403, SE 200. Changing it back restored GB 200.
- Editing a list that no host uses returned 200 and did not apply the proxy config.

### (d) Re-run of earlier checks (PASS)

- **Database absent:** every client got 403 with `Access denied: Geographic restriction` and the profile security headers on both country hosts. No `geoip2` app in the proxy config. `/security/status` showed `acl.reason: geoip_unavailable`; `/security/geoip/status` showed `available: false`, the same reason and `message: "GeoIP database unavailable"`. One Warn per apply naming both lists.
- **Allow-list without usable rules:** an attached IP allow-list rewritten in SQLite to `[]`, then to text that cannot be parsed, each followed by a restart. All clients got 403 both times, with one Warn per apply naming the cause (`hosts: 1`).

## Findings

| ID | Severity | Finding | Evidence | Suggested follow-up |
| --- | --- | --- | --- | --- |
| F1 | High (existing code, not caused by this branch) | An IP block-list refuses every visitor, not only the listed addresses, when an admin allow-range is set | `backend/internal/caddy/config.go` around line 1139 (`buildACLHandler`, `blacklist` branch) puts the listed ranges and the admin exclusion in two separate matcher sets of one route. The proxy treats separate matcher sets as alternatives, so any address outside the admin range matches. Container check: block-list for the GB subnet, admin range `172.16.0.0/12`; GB, SE and US clients all got 403 | Put both conditions in one matcher set (`remote_ip` plus `not`), add a test that pins it, and cover it in `acl-enforcement.spec.ts`. Tracked as #1891 and fixed in a separate change |
| F2 (resolved) | Low | A reload with a file that cannot be opened discards the loaded database, while the proxy keeps the old data until the next apply | The reload returned 500 and the status changed to unavailable at once, but country hosts kept answering from the old data. The next unrelated apply then refused every client on those hosts | Open the new file before closing the old one and keep the old database on failure, or apply after a failed reload so the status and the proxy agree |
| F3 (resolved) | Low | A country list with an empty entry (for example a trailing comma) is rejected | `country_codes: " se , "` returned 400 with the message "invalid country code" and an empty code | Normalize before validating, so empty entries are dropped |
| F4 | Info | The `GeoIP service not initialized` branch of the reload handler can no longer be reached in the running app | The service is now always created at startup | Keep it as a guard or remove it |
| F5 | Low (out of scope) | Creating a security header profile with `hsts_enabled`, `xss_protection` or `x_content_type_options` set to `false` stores `true` | The model's `gorm:"default:true"` replaces a zero value on create | File an issue; create with a field selection or pointer fields |
| F6 | Info | The spec and `CLAUDE.md` refer to `make trivy`, which does not exist | No such Makefile target | Add the target or update the references |

## Notes

- The build now uses the checksum as a warning only, so any non-empty download is bundled. A download that is not a valid database is reported at runtime as unavailable, and country lists refuse requests until a valid file is provided.
- The live `charon` containers were not touched; the shared `charon-e2e` container was used only by the image rebuild and the targeted Playwright run.
