# QA Report: CrowdSec Import Hardening (PR 16)

- Branch: `fix/crowdsec-import-hardening`, HEAD `dff41d614` (8 commits ahead of `origin/development`, includes the Caddy 2.11.8 Dockerfile bump `76e690c5e`).
- Date: 2026-11-04
- Spec: `docs/plans/crowdsec_hardening_batch5_spec.md` (local, untracked) section 3x.14 (PR 16 table) and 3x.9 (container gates).
- Throwaway test image: `charon:qa-pr16`, built from this branch with `--build-arg CADDY_BUILDER_SRC=caddy-inline --build-arg CROWDSEC_BUILDER_SRC=crowdsec-inline`.

## 1. Summary

| # | Gate | Command or method | Result |
|---|------|-------------------|--------|
| 1 | Local patch coverage (strict) | `bash scripts/local-patch-report.sh` | PASS (96.6%, 115 of 119 changed lines) |
| 2 | Toolchain pin freshness | `bash scripts/verify-toolchain-pin.sh` | FAIL (exit 1) |
| 3 | Pre-commit hooks, all files | `lefthook run pre-commit --all-files` | PASS (exit 0) |
| 4 | Lint, pinned version | `docker run ... golangci/golangci-lint:v2.15.0 golangci-lint run --timeout=25m ./...` | PASS (`0 issues.`) |
| 5 | Backend coverage | `scripts/go-test-coverage.sh` | Coverage gate met (93.1% statements, 92.0% lines, minimum 87%); exit 1 from one local-only test (see 2.4) |
| 6 | Frontend coverage | `scripts/frontend-test-coverage.sh` | PASS (exit 0; 4107 tests passed; branches 85.1%) |
| 7 | Frontend type-check | `npm run type-check` | PASS |
| 8 | Frontend build | `npm run build` | PASS |
| 9 | Backend build | `go build ./...` | PASS |
| 10 | GORM security scan | not run | SKIPPED: no file under `backend/internal/models/` changed |
| 11 | Playwright, firefox | `tests/crowdsec-whitelist.spec.ts --project=firefox` | PASS (12 of 12) |
| 12 | Playwright, security-tests project | `tests/security/crowdsec-import.spec.ts`, `tests/security/crowdsec-export-redaction.spec.ts`, `tests/security-enforcement/zzz-security-ui/crowdsec-import.spec.ts` | 77 passed, 3 failed (see 2.3) |
| 13 | Container gate G1 (non-root refusal) | see section 3 | PASS (see 3.1 for the status code note) |
| 14 | Container gate G2 (import limits, round trip) | see section 3 | PASS |
| 15 | Container gate G8 (hub apply failure) | see section 3 | PARTIAL: tree complete, rollback reported failed, partial hub files left (see 3.3) |
| 16 | Caddy 2.11.8 plugin verification (a) to (j) | see section 4 | PASS for all ten items |

## 2. Test results

### 2.1 Patch coverage

- Strict mode. Changed lines 119, covered 115, patch coverage 96.6%.
- Uncovered changed lines: `backend/internal/api/handlers/crowdsec_handler.go` lines 308, 1236-1237; `backend/internal/crowdsec/backup.go` line 236.

### 2.2 Backend and frontend coverage

- Backend: statements 93.1%, lines 92.0%; gate 87% met. Run as a non-root user. 31 tests skipped; most skips are root-only branches (for example the `...WhenRoot` and permission repair cases).
- Frontend: 320 files, 4107 tests passed, 2 todo. Statements 90.97%, branches 85.1%, functions 88.39%, lines 91.99%. The branch margin above the 85% minimum is 0.1 points.

### 2.3 Playwright failures (PR-related)

Three tests in `tests/security/crowdsec-import.spec.ts`, group "Rejected Archives", fail against this branch:

1. `should reject an archive larger than 50 MB`: expected status 422, received 413. The body-size check now answers 413 `request_too_large` before the archive is read. The expectation predates the change.
2. `should reject an archive with a suspicious compression ratio`: expected the previous ratio-refusal message (`validation failed: suspicious compression ratio: ...`), received `the archive is larger than the import limit` (422, `error_code` `import_archive_too_large`). The message is the generic refusal text introduced on this branch. The expectation predates the change.
3. `should reject a corrupted archive`: expected `validation failed: failed to create gzip reader: gzip: invalid header`, received `validation failed: create gzip reader: gzip: invalid header`. The shared archive reader dropped the `failed to` prefix that `development` used. This is a message change in application code and is not covered by the branch's own unit tests.

### 2.4 Local-only failure

- `TestShippedFilesDoNotSetAdminAPIVariable` (`backend/internal/caddy`) fails only because `.docker/compose/docker-compose.test.yml` exists locally. That file is gitignored and is not in the repository, and the test matches a commented line in it. Not a PR defect.

### 2.5 Toolchain pin

- `scripts/verify-toolchain-pin.sh` exits 1. The recomputed recipe hash is `caddy-crowdsec-1f287e5abd0c5a65`, while the Dockerfile pins `CHARON_TOOLCHAIN_TAG=caddy-crowdsec-1480662a3128b9b4`. The Caddy version bump changed the recipe, and the pinned toolchain tag and digest were not updated. The fix is the Toolchain Image workflow (`workflow_dispatch`) or its bot PR, followed by a bump of `CHARON_TOOLCHAIN_TAG` and `CHARON_TOOLCHAIN_DIGEST`.
- The default Docker build consumes the pinned prebuilt image. The throwaway image for this report was built from source (the `caddy-inline` and `crowdsec-inline` selectors), so the verification in section 4 does not depend on the stale pin.

### 2.6 Lint and hooks

- `golangci-lint` v2.15.0 (pinned image): `0 issues.`
- `lefthook run pre-commit --all-files`: exit 0. Frontend ESLint reported 0 errors and 1355 warnings, which are pre-existing and outside this diff.

## 3. Container gates

Environment: throwaway container `charon-qa-pr16`, uid 1000 (`charon`), E2E-style environment, volume `charon-qa-pr16-data`, loopback-only ports, and a throwaway upstream on a private network.

### 3.1 G1: refusal of a configuration import with a non-writable subdirectory (non-root)

- Setup as `charon`: `config/qa-locked` created with one file, then `chmod 500`. The archive was a valid export of the same instance.
- Pre-import file count under `/app/data/crowdsec/config`: 100. Post-import count: 100. The sorted file lists are identical.
- `config.yaml` and `acquis.yaml` are present.
- Response: `error_code` `import_config_not_writable` with the message "Charon cannot change the CrowdSec configuration folder...". The HTTP status is 500, not a 4xx.
- The log records `crowdsec import rollback failed; backup retained for manual recovery`. The retained snapshot `/app/data/crowdsec.backup.20261104-033953.690308` holds 101 regular files (the 100 config files plus `bouncer_key`).
- Cleanup: the locked directory was made writable and removed as `charon`.

### 3.2 G2: import limits and round trip

| Case | Result |
|------|--------|
| Export of the real configuration | HTTP 200, 22,303 bytes |
| Export imported back (round trip) | HTTP 200 in 2.08 s, `status: imported`, 56 hub items in the export |
| Upload of about 55 MB (above the 50 MiB compressed limit plus overhead) | HTTP 413, `error_code` `request_too_large`, 0.25 s |
| Archive of 510 MiB of zero bytes (519,152 bytes compressed; above the uncompressed limit) | HTTP 422, `error_code` `import_archive_too_large`, 0.02 s |
| Archive with 20,050 empty entries (above the 20,000 entry limit) | HTTP 422, `error_code` `import_archive_too_large`, 0.05 s |
| Container CPU after the cases above | 0.69% |

### 3.3 G8: hub apply failure and rollback

- Setup as `charon`: `config/collections` set to `chmod 500`; preset `LePresidente/redmine` pulled, then applied.
- Result: HTTP 500, "install collections ...: rollback failed; backup retained, see server logs". The retained snapshot `/app/data/crowdsec.backup.20261104-034034.157151` exists (44 regular files).
- Before/after comparison: no pre-existing file is missing. Three files from the partial install remain: `config/hub/collections/LePresidente/redmine.yml`, `config/hub/parsers/s01-parse/LePresidente/redmine-logs.yaml`, `config/hub/scenarios/LePresidente/redmine-bf.yaml`.
- Assessment: the tree is complete for all pre-existing files, but it is not restored to its pre-apply state, because the rollback reported failure and the partial install stayed in place.

## 4. Caddy 2.11.8 plugin verification (image `charon:qa-pr16`)

| Item | Check | Result | Evidence |
|------|-------|--------|----------|
| (a) | `caddy version`; `caddy list-modules` | PASS | `v2.11.8`. Modules present: `crowdsec`, `admin.api.crowdsec`, `layer4.matchers.crowdsec`, `http.handlers.crowdsec`, `http.handlers.appsec`, `http.handlers.waf`, `http.handlers.rate_limit`, `http.handlers.geoip2`, `geoip2`, `security`. All 27 `dns.providers.*` modules from the Dockerfile list are present (27 of 27). |
| (b) | Generated config validates and loads | PASS | `caddy validate` on the exported live config: "Valid configuration", exit 0, no deprecation warnings. Admin `POST /load` of the same config: exit 0. |
| (c) | CrowdSec bouncer registers; decision blocks; clean request passes; stream mode | PASS | Bouncer `caddy-bouncer` (v0.12.1) registered over API key auth. App config: `api_url` `http://127.0.0.1:8085`, `enable_streaming` true, ticker 60s. A decision added through the local engine for a test client returned 403 after the stream pull (about 75 s); the same client returned 200 before the decision; a clean client returned 200. |
| (d) | WAF blocks a rule-set match; clean request passes | PASS | Custom rule set (one `SecRule`, `deny`, status 403) enabled in block mode. A request the rule set blocks returned 403. A clean request returned 200 with the upstream body. |
| (e) | ACL allow and deny; rate limiting | PASS | Blacklist containing the test client: 403 with body "Access denied: IP blacklisted". Whitelist containing a second client: that client 200; a client not on the list 403 with "Access denied: IP not in whitelist". Rate limiting enabled: a burst of 400 requests from one client returned 100 responses with 200 and 300 with 429. |
| (f) | Security headers present | PASS | A 200 response carried `Strict-Transport-Security`, `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Cross-Origin-Opener-Policy`, `Cross-Origin-Resource-Policy`, `X-Xss-Protection`. |
| (g) | DNS-challenge provider modules load and validate (offline, no DNS API calls) | PASS | All 27 provider modules validated with `caddy validate`. 26 with a name-only configuration; `cloudflare` with an API-token configuration (name-only is rejected because the token is required). Provider-specific field sets for route53, digitalocean, ovh, rfc2136, azure, and googleclouddns also validated. |
| (h) | Admin API on `127.0.0.1:2019` only | PASS | Listening socket is `127.0.0.1:2019` (checked in `/proc/net/tcp`). From a client container on the same network, `http://<container-ip>:2019/config/` refused the connection (curl exit 7). |
| (i) | Proxied request and TLS with internal certificate issuance | PASS | HTTP proxied request returned 200 with the upstream body. HTTPS for a host with the `internal` issuer returned 200; log line "certificate obtained successfully". The local CA root is not in the container trust store, so verification against it fails as expected. |
| (j) | No new plugin deprecation or error lines | PASS (with notes) | No plugin deprecation or error lines. Two lines appear in the logs and are unrelated to plugins: see 4.1. |

### 4.1 Log notes (not plugin errors)

- `failed to install root certificate` (Caddy `pki.ca.local`): logged on each configuration apply once a host uses the `internal` issuer. The image has no `certutil`, so the root cannot be installed into the system store. The live image (`charon:local`, Caddy v2.11.7) logged no such lines because it had no internal-issuer host; the comparison is therefore not like-for-like.
- `Failed to migrate certificate private keys: no such column: private_key`: logged twice on a fresh database. This message comes from certificate-service code that this branch does not change. The live image does not show it.
- The two lines containing "deprecated" are Charon's own GIN route names (`DeprecatedGetSettings`, `DeprecatedUpdateSettings`), not plugin deprecation messages.

## 5. Blocking items for merge

All items from the first run are resolved on the final commits:

1. Toolchain pin: the branch was rebased onto the refreshed toolchain image. `scripts/verify-toolchain-pin.sh` reports the pin verified (`caddy-crowdsec-1f287e5abd0c5a65`).
2. Import specs: the 413 and 422 expectations were updated to the intended behavior, the gzip message wording was restored in the application code, and an entry-count case was added. `crowdsec-import.spec.ts` and the UI import spec: 69 passed, 0 failed (security-tests project).
3. G8: the rollback restored every pre-existing file and kept the snapshot. Files written inside the directory made unwritable for the test remain there, because that directory cannot be modified. This is documented here; the snapshot is retained for recovery.
4. Frontend branch coverage: 85.63% (was 85.12%). Backend patch coverage: 167/167 changed lines.

No Caddy plugin failed to load or to communicate.

## 6. Environment and cleanup

- The throwaway stack used ports bound to 127.0.0.1 only and no live or E2E container, volume, or tag was changed. Read-only `docker logs` and a throwaway `docker run` from `charon:local` (version check only) were used for comparison.
- The Go coverage run was non-root (host uid 1001).
- The Playwright security-tests project runs Desktop Chrome, as configured in `playwright.config.js`.
- Throwaway containers, network, volumes, and image were removed after the run.
