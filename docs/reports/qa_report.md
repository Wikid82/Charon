# QA Report: GeoIP Database Card and Unenforced Country List Alert

- Branch: `feature/geoip-database-card` (13 commits on `development`, up to `fae16ffec`, plus the uncommitted QA
  changes listed below)
- Spec: `docs/plans/geoip_database_card_spec.md` (feature PR, commit C14)
- Scope: notification bell alerts (`GET /notifications/alerts`), country lists left unenforced (with an alert) while
  no GeoIP database is usable, GeoIP database status with source, date and size, replacing and removing the
  data-folder copy (`POST`/`DELETE /security/geoip/database`) behind `RequireSameOriginForBrowserAuth`, the access
  list self-check (`POST /access-lists/self-check`), the Security page card, the access list form note, 28 locales,
  and docs.
- Image under test: `charon:local` from `docker-rebuild-e2e` on the working tree described above.
- Date: 2026-11-10

## Verdict

**Pass.** Every Definition of Done gate passes. Local patch coverage is 100% (983 of 983 changed lines). The QA pass
added tests, two small test seams, and one small hardening of build-date handling; nothing else in the product was
changed. No CRITICAL or HIGH findings in the code of this branch.

## Changes made during QA

| Area | Change |
| --- | --- |
| `services/geoip_database_upload.go`, `geoip_database_store.go` | File operations of an upload or removal (create temp file, rename, folder sync) go through a `geoipFileOps` value on the store, defaulting to the OS calls. Behaviour is unchanged; tests replace single operations to reach the write, sync, close, rename and folder-sync failure branches. |
| `services/geoip_database_upload_fileops_test.go` (new) | Failure-branch tests for the seam above: each failure reports a write failure, leaves no temp file, restores the previous copy (or removes the new one), and applies nothing. |
| `geoip/inspect.go` | New `BuildTime(epoch)` that clamps a build epoch to the end of year 9999 before converting it to a time. Used by the build-date check and both date formatters. See the finding below. |
| `api/routes/geoip_marker.go`, `routes.go` | The startup sweep of the data folder moved into `sweepGeoIPDataFolder`, so its failure log is testable. Same behaviour. |
| Tests | Lookup handler answers and admin checks (`security_geoip_lookup_test.go`), `EnvPathInEffect`, fixture encoder refusals, `BuildTime`, the sweep helper, the GeoIP API client functions, and the locale key helper. |
| `pages/__tests__/Login.overlay.audit.test.tsx` | Made deterministic: the mocked login request now stays pending until the overlay has been asserted. It failed once under CPU load because a 100 ms timer finished the request before the click returned. Not related to this branch; fixed per the DoD. |
| Lint | `make lint-backend` reported 45 gosec findings, all in files of this branch (44 in tests). Test fixtures now use `0o750` folders or carry the repo's usual justified `nolint` comments; the remaining production one (data folder `MkdirAll` at `0o755`) carries a justified `nolint`. |

## Static checks and build

| Check | Result |
| --- | --- |
| `make lint-backend` (golangci-lint v2.15.0, full config) | 0 issues (after the fixes above) |
| `lefthook run pre-commit --all-files` | all 20 hooks pass, including semgrep, golangci-lint-fast, frontend lint/type-check, locale lint, e2e lint (0 errors, 124 warnings at the limit) |
| `cd frontend && npm run lint` | pass, 0 warnings |
| `node node_modules/typescript/bin/tsc6 --noEmit` | pass |
| `cd frontend && npm run build` | pass |
| `cd backend && go build ./...`, `go vet` on touched packages | pass |
| `./scripts/scan-gorm-security.sh --check` | 0 CRITICAL, 0 HIGH, 0 MEDIUM, 2 INFO (pre-existing index suggestions) |

## Coverage

| Run | Result |
| --- | --- |
| `scripts/go-test-coverage.sh` (full) | all packages pass; statements 93.6%, lines 92.6% (gate 87%) |
| `scripts/frontend-test-coverage.sh` (full) | 344 files, 4466 tests pass (2 todo); statements 91.76%, lines 92.65% |
| `bash scripts/local-patch-report.sh` | overall 100.0% (983/983), backend 100.0% (794/794), frontend 100.0% (189/189) |

The patch report compares committed `HEAD` with `origin/development`. The uncommitted QA additions were checked
separately: the backend coverage profile has no uncovered block in any Go file they touch.

## E2E (Playwright, `--project=security-tests`, after the image rebuild)

`npx playwright test tests/security/geoip-unenforced-alert.spec.ts tests/security/geoip-database-card.spec.ts
tests/security-enforcement/acl-enforcement.spec.ts tests/security-enforcement/security-headers-enforcement.spec.ts
tests/security/self-proxy-requests.spec.ts --project=security-tests`: **33 passed**, run twice (before and after the
QA code changes, each after a fresh `docker-rebuild-e2e`).

## Security scans

| Scan | Result |
| --- | --- |
| CodeQL Go + JS (`lefthook run codeql`, CLI 2.27.2) | 0 blocking. Go: 4 results, all pre-existing entries in `codeql-suppressions.yml`, none in files of this branch. JS: 0 results. Parity check passes. |
| Trivy filesystem (`skill-runner.sh security-scan-trivy`) | No findings in the branch's dependency manifests. The skill exits non-zero on items outside the tracked tree: stale untracked `.claude/worktrees/*` copies of other branches, and an ignored, untracked CA key generated by an earlier test run under `backend/internal/api/routes/keys/` (not in git, not in the image). |
| Trivy image (`charon:local`, HIGH/CRITICAL) | Alpine base and the Charon binary: 0. One HIGH each in the bundled Caddy and CrowdSec binaries; both CVEs are already accepted in `.trivyignore` and `.grype.yaml`. The branch does not change the Dockerfile. |

## Security review

| Area | Result |
| --- | --- |
| Upload route authorization | `POST`/`DELETE /security/geoip/database` sit in the admin-only security group and check the role again in the handler. A non-admin gets 403. |
| `RequireSameOriginForBrowserAuth` | Applied only to the two new routes. Requests with an `Authorization` header pass (a foreign page cannot add it without a preflight, which is not granted); other unsafe requests go through the standard library's cross-origin check. Spot check: a cookie upload with a foreign `Origin` and `Sec-Fetch-Site: cross-site` got 403 `cross_origin_refused`; same-origin and non-browser requests succeed. |
| Upload handling | Body capped by `http.MaxBytesReader` (32 MiB plus framing); exactly one multipart part named `file`; client file name and content type ignored; temp file named by `os.CreateTemp` in the data folder; symlinked targets refused; validation (MaxMind format, country edition, record decode, build date) runs before the rename; failures restore the previous copy; one change at a time (busy answers 409 at once). Installed file is `0600`, owned by the app user. |
| Build-date check | Hardened during QA (see finding). |
| Alerts endpoint | Management access (any role except pass-through). It returns only the alert key, severity, up to five access list names, a count, and whether a database is available. Access list names are already readable by the same roles. |
| Self-check endpoint | Admin only; evaluates a draft against the caller's own address and the admin whitelist; no data is stored. |
| Error bodies | Fixed messages plus `error_code`; no paths or OS text in any response (verified in handler tests and the spot check). |
| Logs | Failure detail, which can include the data-folder path, is written to the server log only and sanitized, as the spec allows. Audit rows carry type, build date, size and digest, never a path or client file name. |
| Backups | Backups are built from an allow-list (database and `caddy/`), so the GeoIP folder, its previous copy and upload temp files are never included. Pinned by `TestBackupService_ExcludesGeoIPFolder`. |
| Gotify tokens | None in the diff, test artifacts or logs reviewed. |

### Finding (fixed in QA): out-of-range build dates

Severity: low (admin-only upload). A database whose build epoch is outside the range Go's time conversion handles
could pass the "not in the future" check by wrapping into the past, and would then always win over later bundled
updates. `geoip.BuildTime` now clamps such an epoch to the end of year 9999, so the upload is refused as dated in the
future. Covered by `TestBuildTime_ClampsEpochsPastTheTimeRange` and
`TestGeoIPDatabaseStoreCheckBuildEpoch_RefusesEpochsPastTheTimeRange`.

## Spot check (throwaway container)

A separate `charon:local` container on a loopback-only port with a fresh data volume, removed afterwards. The dev
containers were not touched.

| Step | Result |
| --- | --- |
| Remove the bundled copy, enable Cerberus and ACL, reload | 500 `geoip_reload_failed`, fixed message, no path |
| Add a country allow-list on an enabled host | `GET /notifications/alerts` returns one `geoip_unavailable` error alert naming the list, `database_available: false`; status reports `unenforced_lists_total: 1` |
| Cross-origin cookie upload | 403 `cross_origin_refused` |
| Upload a file that is not a database | 400 `geoip_not_mmdb` |
| Upload a valid country database | 200, source `data`, `unenforced_lists_total: 0`; alerts list empty |
| Remove the data-folder copy (no bundled copy) | 200, source `none`; the alert returns |

## Acceptance criteria (spec section 12, feature items)

| Criterion | Evidence |
| --- | --- |
| No usable database: country-list hosts reachable, no country 403 | `acl-enforcement.spec.ts` "Country lists", `geoip-unenforced-alert.spec.ts` |
| Bell shows the alert first, with links, no dismiss; stays during and after a failed apply | `NotificationCenter.test.tsx`, `geoip-unenforced-alert.spec.ts`, `manager_last_applied_test.go` |
| A successful apply with the database clears the alert | E2E "Restored database"; spot check |
| A failed reload still applies; proxy, status and alert agree | `security_geoip_reload_apply_test.go`; spot check |
| `acl.reason = geoip_unavailable` and tester say "not enforced" | `security_status_enforcement_test.go`, `access_list_evaluate_test.go` |
| Card, replace/remove, lookup and form note; hidden for non-admins | `GeoIPDatabaseCard.test.tsx`, `AccessListGeoNote.test.tsx`, `Security.geoip.test.tsx`, `geoip-database-card.spec.ts` |
| Cross-origin cookie requests refused; UI and header-authenticated clients succeed | `browser_origin_test.go`, `geoip_database_routes_test.go`, E2E cross-origin rows; spot check |
| No toasts or banners; strings in 28 locales; parity tests | diff review (results are inline); `geoipLocales.test.ts`, `notificationCenterLocales.test.ts` |
| Troubleshooting page exists, linked, in the nav | `docs/troubleshooting/geoip-database.md`, `security-module-applied.md`, `docs-site/zensical.toml` |
| Full DoD | this report |

## Deviations from the spec

- Two test seams were added that the spec does not describe: `geoipFileOps` on the store and the
  `sweepGeoIPDataFolder` helper in `routes`. Neither changes behaviour.
- `geoip.BuildTime` was added to harden build-date handling (see the finding).
- The strict docs build (`zensical build --strict`) was not run locally because Zensical is not installed in this
  environment; the docs workflow runs it in CI.
- The spec's DoD floor for patch coverage is 85%; this branch reaches 100%.

## Follow-ups (not blocking)

- Low: `LookupGeoIP` (moved unchanged from the older handler) returns the lookup library's error text in its 500
  body (`"GeoIP lookup failed: ..."`). The text carries no path, but a fixed message plus `error_code` would match
  the other GeoIP endpoints.
- Low, informational: when the database file disappears under a running instance and a reload fails, the service
  keeps the database it already had open, so the status reports the previous source and build date with
  `available: false` and a null size. The card and the alert use `available`, so users see the right state; the
  extra fields may confuse someone reading the raw API.
- Housekeeping: the local Trivy filesystem skill scans untracked `.claude/worktrees/` copies and git-ignored test
  keys. Excluding those paths in the skill would make its exit code reflect the tracked tree only.
