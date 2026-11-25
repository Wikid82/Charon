# QA Report: Reference Integrity, File Handling, Plugin Loading and Backup Transport

- Branch: `fix/file-and-transport-handling` (15 commits on `origin/development`, head `4c00f44fb`)
- Plan: `docs/plans/pr36_file_transport_fk_plan.md` (rev 2), sections 2-4, 6 and 7 (test plan, container gates
  G1-G6, Definition of Done) and the edge-case table in 4.5
- Scope: every database connection gets the same connection settings and reference checks; schema updates run on
  one pinned connection, repair rows whose referenced item no longer exists, then check what is left; a live
  restore repairs the restored data before it is committed; deleting a DNS provider that hosts use is refused, and
  deleting a provider, a remote backup destination or a backup also removes the rows that belong to it; permission
  repair works through one opened descriptor relative to the allowed folder; plugins load from a verified private
  copy, under one signature list taken from the configuration; WebDAV backup destinations need `https://` unless
  "Allow unencrypted HTTP" is ticked, and existing `http://` destinations are ticked automatically; the Sources
  form says that direct Docker connections are unencrypted.
- Image under test: the E2E image rebuilt with `--clean` from the head above (`charon:local`). The previous image
  was `wikid82/charon:dev` at `eb14d21` (an ancestor of `origin/development`, without this branch). All gate
  containers, volumes and the network were throwaway (`pr36qa-*`, loopback port 18090) and were removed
  afterwards. The live containers `charon`, `charon-dev` and `orthrus-dev` were not touched.
- Toolchain: Go 1.27.4, CodeQL CLI 2.27.5, lefthook 2.2.3, Playwright 1.64.1.
- Date: 2026-11-25.

## Verdict

**Not approved yet: three blocking items.** Every container gate matches the plan, and every test suite, build, lint
and scan passes except these three:

1. **Patch coverage is below the merge gate.** The local patch report gives 81.2% overall and 81.1% for the
   backend (808 changed lines, 655 covered). The CI patch gate (`codecov.yml`) is 87%. The files with the most
   uncovered changed lines are listed under [Patch coverage](#patch-coverage).
2. **CodeQL Go reports one blocking result** in `backend/internal/api/handlers/system_permissions_handler.go`
   (the shared failure-logging helper of the permission repair). `codeql-check-findings` fails on it, so CI will
   fail too. The fix belongs in application code and goes back to backend development.
3. **The full lint (`make lint-backend`, gosec on all files as in CI) reports one application-code finding:**
   G302 at `backend/internal/services/plugin_loader_files.go:109` (`os.Chmod(abs, 0o700)` on the plugin cache
   folder). 0700 is the intended mode for a folder, so a `#nosec G302` with a justification is enough; it goes back
   to backend development.

QA changed no application code and added no tests. The same lint run reported 19 gosec findings in test files
added by this branch (fixture modes and test-controlled paths). QA annotated those lines with the repo's
`#nosec` convention and a justification, in `system_permissions_repair_leaf_test.go`,
`system_permissions_repair_result_test.go`, `routes/plugin_loader_wiring_test.go` and
`services/plugin_loader_cache_test.go`, and re-ran their tests (pass). These edits are uncommitted. Overall project
coverage passes (backend 94.1% statements, frontend 92.9% lines).

## Gate results

| Gate | Command | Result |
| --- | --- | --- |
| E2E image | `.claude/skills/scripts/skill-runner.sh docker-rebuild-e2e --clean` | pass |
| Targeted Playwright | `npx playwright test tests/dns-provider-crud.spec.ts tests/tasks/backups-remote-targets.spec.ts tests/certificate-delete.spec.ts tests/proxy-groups.spec.ts --project=firefox` | pass, 66/66 (2.7 min); `proxy-groups.spec.ts:257` passed first time |
| GORM scan | `./scripts/scan-gorm-security.sh --check` | pass, 0 critical/high/medium, 2 info |
| Patch coverage | `bash scripts/local-patch-report.sh` | **fail**: overall 81.2%, backend 81.1%, frontend 90.9% |
| Backend coverage | `scripts/go-test-coverage.sh` | pass, 94.1% statements, 93.0% lines, no test failures |
| Frontend coverage | `scripts/frontend-test-coverage.sh` | pass, 92.9% lines; 379 files, 4730 tests passed, 2 todo |
| Race detector | `go test -race` on `internal/database`, `internal/config`, `internal/api/routes`, `internal/testutil` (full), `internal/api/handlers` and `internal/services` (permission, plugin, DNS provider, backup, remote target, WebDAV, access list, header profile, host, restore, reference tests), `internal/services/remotestorage` | pass |
| Plugin build | `make plugin-powerdns`, `make plugin-powerdns-smoke` | pass |
| CodeQL Go and JS | `lefthook run codeql` | **fail**: Go 2 results (1 suppressed by an existing entry, 1 blocking); JS 0 |
| Pre-commit | `lefthook run pre-commit --all-files` | pass, all 20 hooks |
| Builds | `go build ./...`; `npm run type-check`; `npm run build` | pass |
| Full lint | `make lint-backend` (run last, after QA's test annotations) | **fail**: 1 gosec finding in application code (19 test-file findings annotated by QA) |
| Commit messages | `git log origin/development..HEAD` | pass (see [Commit messages](#commit-messages)) |

## Patch coverage

Files with uncovered changed lines (from `test-results/local-patch-report.md`, fresh coverage of the head):

| File | Patch coverage | Uncovered changed lines |
| --- | ---: | ---: |
| `backend/internal/services/plugin_loader_files.go` | 68.1% | 45 |
| `backend/internal/database/integrity.go` | 84.6% | 31 |
| `backend/internal/services/host_restore.go` | 74.5% | 14 |
| `backend/internal/services/backup_remote_service.go` | 81.6% | 14 |
| `backend/internal/services/dns_provider_service.go` | 66.7% | 12 |
| `backend/internal/api/handlers/system_permissions_handler.go` | 82.5% | 11 |
| `backend/internal/testutil/dsn.go` | 71.4% | 4 |
| `backend/internal/services/redirection_host_restore.go` | 80.0% | 4 |
| `backend/internal/api/handlers/proxy_host_handler.go` | 81.3% | 3 |
| `backend/internal/database/database.go` | 86.4% | 3 |
| eight more files | 60.0-96.6% | 1-2 each |

Most uncovered lines are error returns (`if err != nil { return ... }`) in the plugin cache code, the repair and
check code, the delete transactions and the WebDAV backfill. Covering about 50 of them reaches 87%.

## Container gates

### G1: upgrade with damaged data

On a volume created by the previous image, QA created through the API: a custom certificate, an access list, a
security header profile, a proxy group, a DNS provider with two credentials, a second DNS provider, three proxy
hosts (one using the certificate, list, profile, group and a location; one using the DNS provider with the DNS
challenge; one with a location), a standard user, an `http://` WebDAV destination (a throwaway WebDAV server on
the private network) and a backup that uploaded to it. With the container stopped, the certificate, list,
profile, group, the used DNS provider and the third host were deleted directly with `sqlite3`, two upload rows
pointing at a missing backup and a missing destination were added, and two unknown keys were added to the
destination's stored configuration. `PRAGMA foreign_key_check` then listed 10 kinds of broken references. The
previous image was started once more on the damaged data to record its Caddy configuration and take a damaged
backup (for G3).

PR image on the same volume:

- Boot completed (healthy after 5 s).
- One Warn "Repaired database references" per rule with changes (table, column, action, count, row ids only):
  upload rows (2), DNS provider credentials (2), locations (1), monitor of the deleted host (1), and the cleared
  certificate, group, header profile, access list and DNS provider of the hosts; the DNS provider line carries
  `dns_challenge_disabled: 1`. Then Info "Database reference check finished" with `repaired: 11, remaining: 0`.
- Info "Marked existing WebDAV backup targets as allowing unencrypted HTTP" with `count: 1`.
- `PRAGMA foreign_key_check` is empty. The heartbeats of the removed monitor are gone. The host that used the DNS
  challenge now has `use_dns_challenge = 0`.
- The destination's stored configuration gained `webdav.allow_insecure_http: true`; both unknown keys, `updated_at`
  and the encrypted secret are byte-for-byte unchanged.
- The standard user saved both repaired hosts (`PUT /api/v1/proxy-hosts/:uuid`): 200 each.
- The effective Caddy configuration (`caddy-admin get /config/`, sorted JSON) is identical to the one the previous
  image served on the same damaged data, including the HTTP challenge for the host that used the DNS challenge.

### G2: second restart

After a restart, the log has no "Repaired database references" line, no Info summary and no backfill line;
`foreign_key_check` stays empty.

### G3: live restore of a damaged backup

A fresh PR-image instance received the damaged backup (`POST /api/v1/backups/upload`) and restored it
(`POST /api/v1/backups/:filename/restore`). The job finished with `live_rehydrate_applied: true`,
`restart_required: false`, `caddy_reloaded: true`; the container did not restart. During the restore the log shows
the same Warn lines and the Info summary (`repaired: 11, remaining: 0`), then the WebDAV backfill (`count: 1`).
Afterwards the restored `http://` destination is listed with `allow_insecure_http: true` (the form initialises its
checkbox from this field) and `foreign_key_check` is empty.

### G4: plugins

The PowerDNS plugin was built in a throwaway `golang:1.27.4-alpine` container with the same source paths as the
image build, then mounted read-only as `/app/plugins` (folder 0755, file 0644). `make plugin-powerdns` and
`make plugin-powerdns-smoke` also pass on the host.

| Case | Result |
| --- | --- |
| File 0644, no signature list | loaded; `/app/data/plugin-cache` is 0700, the copy `<sha256>.so` is 0500, owned by `charon`; the process maps only the cached copy |
| Reload with the same bytes | `count: 1`, no "already loaded" error (see F1) |
| File 0664, restart | refused: "plugin file is group- or world-writable (mode 664): set it to mode 644 or stricter" in the log and the plugin status |
| `CHARON_PLUGIN_SIGNATURES='not json'` | Error "CHARON_PLUGIN_SIGNATURES is not valid; no external plugins are loaded until it is fixed"; nothing loaded at start-up, on reload or on enable ("plugin not in allowlist") |
| Valid list with a wrong hash | refused at start-up, on reload and on enable with "plugin signature does not match allowlist"; nothing mapped |
| `/app/data` as `noexec` tmpfs | refused with "Could not load the plugin from /app/data/plugin-cache. If that folder does not allow running program files (for example a noexec mount), set CHARON_PLUGIN_CACHE_DIR to a folder that does." The original file is never opened as a library |
| Same, with `CHARON_PLUGIN_CACHE_DIR` on an exec tmpfs | loaded from `/plugcache/<sha256>.so` |

### G5: permission repair as root

`/app/charon` ran as uid 0 (the image entrypoint always switches to `charon`, so the binary was started directly).

| Path | Result |
| --- | --- |
| Regular file, owner 1234, mode 0666 | `repaired`, now `0:0` and 0600 |
| Symlink to a file inside the folder | `permissions_symlink_rejected`; the target keeps owner 1234 and mode 0666 |
| Symlink to `/etc/passwd` | `permissions_symlink_rejected`; `/etc/passwd` unchanged (0:0, 0644) |
| File with two links | `permissions_hardlink_rejected`, "files with more than one link are not changed"; both names unchanged |

### G6: WebDAV

| Case | Result |
| --- | --- |
| New `http://` destination without the option | 400 `webdav_https_required`, "Use an https:// address, or turn on "Allow unencrypted HTTP" for this destination." |
| New `ftp://` destination | 400 "WebDAV addresses must start with https:// or http://." |
| New `http://` destination with `allow_insecure_http: true` | 201 |
| Existing `http://` destination from G1 | listed with `allow_insecure_http: true`; connection test succeeds; a new backup uploads to it |

The list note and the form checkbox are covered by Vitest and by the two enabled E2E tests.

## Commit messages

- Exactly two `(security)` commits: `fix(security): harden database integrity checks` and
  `fix(security): harden plugin loading checks`. Both have empty bodies. All 15 commits have empty bodies.
- No subject or body has a tracking or scanner id, a session id or link, `!` or a breaking-change footer.
- Lines added by the branch (`git diff origin/development..HEAD`) contain none of the words excluded for public
  text. The only pattern hit is the existing `session_version` column in a schema test fixture.

## In-scope observations (low)

- **O1.** On a fresh install, a plugin that fails its first load has no row in the plugin list, so its status text
  (for example the `CHARON_PLUGIN_CACHE_DIR` hint for a `noexec` data folder) shows only in the log. Rows are
  created only after a successful load; a later failure updates the row. This behaviour predates the branch.
- **O2.** The plan says to log each host whose DNS challenge is turned off. The implementation logs one Warn per rule
  with a `dns_challenge_disabled` count and the host ids instead. The information is the same, but the text differs
  from the plan.
- **O3.** Every `/app/charon` subcommand (for example `caddy-admin get`) now writes the plugin signature mode
  line, or the Error for an invalid list, to stderr, because the configuration parses the list when it loads.
  Standard output stays clean JSON. With an invalid list the Error appears once per process, so up to three times
  at container start.

## Out-of-scope findings

- **F1 (low).** Reloading plugins while a plugin is already loaded sets that plugin's status to `error` with
  "failed to register plugin: dns provider already registered", although the plugin stays registered and usable.
  `LoadPlugin` registers again without checking `loadedPlugins` first; the same code exists on
  `origin/development`. Suggested approach: skip, or unregister and re-register, providers already loaded from the
  same path. To be filed as a public issue.
