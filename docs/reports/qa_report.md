# QA and Security Report - GH #1422 automatic database maintenance

Branch `feat/db-maintenance-1422`, HEAD e937a7dc, range `3437ab88..HEAD`. Spec: docs/plans/current_spec.md rev 7.

## Verdict: PASS (no blocking findings)

## Gates

| Gate | Command | Result |
|---|---|---|
| Playwright | `npx playwright test tests/settings/database-maintenance.spec.ts --project=firefox` (after docker-rebuild-e2e) | 14/14 passed |
| GORM scan | `./scripts/scan-gorm-security.sh --check` | 0 CRITICAL, 0 HIGH, 0 MEDIUM |
| Patch coverage | `bash scripts/local-patch-report.sh` | artifacts present; overall 94.8%, backend 94.4%, frontend 100% (uncovered lines are error branches) |
| CodeQL Go+JS | `lefthook run codeql --all-files` (CodeQL 2.26.4) | Go 4 results, all pre-existing and suppressed in codeql-suppressions.yml (none in files of this branch); JS 0; 0 blocking |
| Trivy | `docker build -t charon:local .` + `aquasec/trivy image --severity CRITICAL,HIGH charon:local` | OS 0, app/charon 0, caddy 0; 1 HIGH in each of crowdsec and cscli binaries (CVE-2026-32286, pgproto3, no fixed version), third-party, already tracked in .trivyignore/.grype.yaml/SECURITY.md. No CRITICAL. |
| Pre-commit | `lefthook run pre-commit --all-files` | all 16 hooks passed (incl. semgrep) |
| Lint | `make lint-fast`, `make lint-backend` | 0 issues both |
| Backend coverage | `scripts/go-test-coverage.sh` | statements 92.1%, lines 89.1% (gate 87%) PASS |
| Frontend coverage | `scripts/frontend-test-coverage.sh` | 289 files / 3586 tests passed (4 skipped, 2 todo pre-existing), lines 91.31% (gate 87%) PASS |
| Types / builds | `npm run type-check`, `go build ./...`, `npm run build` | all OK |
| Backend suite | `go test ./...` | all packages ok, 0 failures |
| Race | `go test -race ./internal/dbmaint/... ./cmd/... ./internal/api/routes/... ./internal/services/... ./internal/database/...` | all ok |

Note: the Trivy container was pointed at the rootless docker socket (/run/user/1001/docker.sock) instead of the stale /var/run/docker.sock hard-coded in `make security-scan-full`.

## Synthetic real-data run (scratch DB, removed afterwards)

Live schema via AutoMigrate, 60 monitors, 3,000,000 heartbeats, oldest 1.23M deleted, WAL, auto_vacuum=0, real StartupPlan/Start/Run path:

- Before: 569,270,272 B main, 138,982 pages, 56,645 free (40.8%), reclaimable 232 MB; Decide = Run.
- Advisor (Reclaimer.AfterPrune on a legacy DB): Pending=true, 232 MB.
- GET /api/v1/system/database before: notice `restart_to_optimize` (info), can_request_optimize true.
- Conversion wall time 5.7 s; during it GET /, /api/v1/system/database and /api/v1/health/db returned 503 and /api/v1/maintenance/status returned 200 `{"active":true,"phase":"converting",...}`.
- After: 318,664,704 B (-44%), integrity_check ok, auto_vacuum=2, journal_mode=wal, 1,770,000 rows intact, last_result converted, notice null, `.tmp` empty and mode 0700.
- Steady state: after pruning 770k more rows, Reclaimer drained 25,291 pages in 13 steps (318.7 MB to 215.1 MB), advice no longer pending.

## Security audit

- SQL: every statement in dbmaint, database and the pruner is a constant or parameterised (`?`); the only formatted ones take integer constants/ints (`incremental_vacuum(%d)`, `busy_timeout=%d`); `"PRAGMA "+name` callers pass literals only. No user input reaches DDL/PRAGMA.
- Gate: answers only GET/HEAD on the exact paths /api/v1/health and /api/v1/maintenance/status; /api/v1/health/db, trailing-slash and sub-paths are blocked (503 JSON or HTML) while active and pass through otherwise; the gate sits before all DB-touching middleware; status/health do no DB access. CSP is hash-based (default-src 'none', script/style sha256 computed from the embedded page, verified by test), plus no-store, nosniff, X-Frame-Options DENY. Remote users cannot trigger the gate: phases change only from the boot-time runner.
- Endpoints: GET /system/database and POST/DELETE /system/database/optimize-on-restart are on managementAdmin; tests assert 401 (no token), 403 (role=user). Cookie auth is SameSite Strict/Lax like every other mutating endpoint; no new CSRF surface. POST is idempotent (200 {requested:true}); error bodies are generic or name only the env var.
- Reserved prefix guard (`migration.`, `maintenance.`): normalised (lowercase+trim) prefix check on UpdateSetting and on every flattened PatchConfig key (nested JSON, mixed case, whitespace, empty-segment cases covered); GetSettings and PatchConfig responses filter reserved rows; Category is ignored. Other settings writers use fixed keys.
- Temp dir: `<data>/.tmp` via Lstat (symlink and non-dir refused), owner == euid, forced 0700, filepath.Clean; operator-set SQLITE_TMPDIR honoured untouched; failure falls back with a warning.
- State/file_id: inode-only id, state of another file is ignored/deleted, in-progress marker counts as a failed attempt, 3-attempt back-off, a fresh admin request resets it. The flag cannot force a conversion below the 100 MB floor nor skip the integrity, disk or writer-lock checks. Corrupt state rows are deleted, not trusted.
- Writer exclusion: BEGIN EXCLUSIVE probe with busy_timeout=0 on the pinned single pool connection, retries then skip as database_busy (tested with a real second writer).
- Logs: sizes, counts and reason codes only; no secrets, no paths beyond config.
- DoS: status endpoint is a mutex read plus fmt (no DB, no allocation proportional to input).
- Provenance: no session IDs, claude.ai links, Co-Authored-By or "generated with" in any commit message or diff; all 14 commit subjects use conventional prefixes; none uses a `(security)` scope. Added `nolint` comments are all justified test or gosec-G115 notes.

## Findings (all non-blocking, informational)

1. LOW/INFO - The Makefile target `security-scan-full` mounts /var/run/docker.sock, which on this dev host is a stale root daemon; the scan needs the rootless socket. Pre-existing tooling issue, not part of this PR.
2. INFO - FileID is inode-only; a replaced database file that happens to reuse the old inode would inherit stale attempts/last_result. Consequence is bounded (back-off counter or a notice), and Load clears state on mismatch. No action needed.
3. INFO - /api/v1/health (GET/HEAD) is now answered by the gate and so no longer passes cerberus.RateLimitMiddleware; the handler is DB-free and cheap, so this is acceptable.
4. INFO - Trivy HIGH CVE-2026-32286 in bundled crowdsec/cscli binaries has no fixed version and is already tracked.


---

## Re-run: Tasks > Database page and development merge

Date: 2026-10-01. Branch `feat/db-maintenance-1422`, HEAD `107241f8`. Scope: merge of `development` (d3625e08) plus Addendum A (Database page under Tasks). Heavy commands ran with TMPDIR/GOTMPDIR under /var/tmp.

Result: **PASS** (no blocking findings).

### Gate results

| Gate | Command | Outcome |
|---|---|---|
| E2E (Database page) | `npx playwright test tests/tasks/database-maintenance.spec.ts --project=firefox` (charon-e2e rebuilt first) | 25 passed |
| E2E (tab bar) | `tests/tasks/logs-viewing.spec.ts --project=firefox` | 25 passed |
| E2E (Settings uptime card) | `tests/monitoring/uptime-monitoring-scale.spec.ts --project=firefox` | 9 passed |
| Patch coverage | `bash scripts/local-patch-report.sh` (baseline origin/development...HEAD) | Overall 94.9% (1532/1614), backend 94.4%, frontend 100% (141/141), agent n/a; all pass; artifacts present |
| CodeQL | `lefthook run codeql --all-files` (CLI 2.26.4) | Go: 4 results, all already suppressed in codeql-suppressions.yml (none in files touched by this branch); JS: 0; 0 blocking |
| Trivy | `docker build -t charon:local .` then `aquasec/trivy image --severity CRITICAL,HIGH` via rootless socket | With .trivyignore: 0 findings. Without it: only CVE-2026-32286 (HIGH, pgproto3/v2, no fix) in crowdsec and cscli, the known accepted item |
| Pre-commit | `lefthook run pre-commit --all-files` | all hooks passed (semgrep 0 findings) |
| Lint | `make lint-fast`; `make lint-backend` | 0 issues; 0 issues |
| Frontend coverage | `scripts/frontend-test-coverage.sh` | 291 files / 3633 tests passed; lines 91.35% (gate 87%) |
| Backend coverage | `scripts/go-test-coverage.sh` | pass; line coverage 89.1% (gate 87%), statements 92.2% |
| Type/build | `npm run type-check`; `go build ./... && go vet ./...`; `npm run build` | all clean |
| Go tests | `go test ./...`; `go test -race ./internal/dbmaint/... ./internal/api/... ./cmd/...` | 0 failures; 0 races |

### Security / compliance audit

- Admin gating: `/tasks/database` is wrapped in `RequireRole allowed={['admin']}` (App.tsx:139); Layout nav item (Layout.tsx:178) and Tasks tab (Tasks.tsx:17) are admin-only. E2E confirms non-admin has no nav item, deep link redirects to "/", and no `/system/database` request is made.
- XSS: no `dangerouslySetInnerHTML`/`innerHTML` in the new page, component, hook or API client; all values rendered as React text.
- Accessibility: passive notices use `role="status"`; load error uses an `Alert` with its own accessible name (covered by E2E).
- `grep -rn "systemSettings.database" frontend/src` is empty; no leftover references to removed components.
- Commit hygiene on `3437ab88..HEAD`: no session IDs, claude.ai links, Co-Authored-By or "generated with" lines in messages or diff; all non-merge subjects use conventional prefixes; no `(security)` scope used.
- Docs: links in docs/database-maintenance.md, docs/features.md, ARCHITECTURE.md resolve (the one flagged URL is an external GitHub link); ARCHITECTURE.md and the docs refer to Tasks -> Database. `docs-site/docs/` is git-ignored and untracked (no hand edits).

### Findings

None blocking. Informational: local patch coverage shortfalls are confined to existing dbmaint/database error branches (diskspace.go, probe.go, tmpdir.go, database.go), above the 85% backend threshold overall. GH #1426 /tmp leak not exercised as a failure.

---

## Re-run: #1427 follow-ups

Branch `fix/db-maintenance-followups-1427`, range `a7aa2909^..HEAD` (HEAD d18a541a). Backend + docs only. Heavy commands ran with TMPDIR/GOTMPDIR under /var/tmp (scratch dir removed afterwards).

**Verdict: PASS. No blocking findings.**

### Gates

| Gate | Command | Outcome |
|---|---|---|
| Build/vet/fmt | `cd backend && go build ./... && go vet ./...`; `gofmt -l .` | clean; gofmt lists nothing |
| Full backend suite | `go test -count=1 ./...` | 0 failures |
| Race (2 runs) | `go test -race -count=1 ./internal/dbmaint/... ./cmd/api/... ./internal/database/... ./internal/api/... ./internal/services/...` | run 1 and run 2 both exit 0, no races |
| Slow real-main() test (2 runs, -race, -v) | `./cmd/api -run TestMaintenance_StopDuringConversionIsSafeAndTheNextBootConverts` | PASS twice (SIGTERM ~18s, SIGKILL ~17-19s), no flakiness |
| Patch coverage | `bash scripts/local-patch-report.sh` (re-run after fresh coverage) | artifacts present; backend 51/51 changed lines = 100% (gate 85%); frontend/agent 0 changed lines |
| Backend coverage | `scripts/go-test-coverage.sh` | pass; line coverage 89.1% (gate 87%), statements 92.1% |
| Lefthook | `lefthook run pre-commit --all-files` | all hooks passed (semgrep 0 findings, 367 rules) |
| Lint | `make lint-fast`; `make lint-backend` | 0 issues; 0 issues |
| GORM | `./scripts/scan-gorm-security.sh --check` | PASSED, 0 issues (2 informational suggestions) |
| CodeQL / Trivy | deferred to CI | no `feat:`, no new network surface or endpoint, no new dependency |
| Frontend untouched | `git diff --stat a7aa2909^..HEAD -- frontend tests` | empty; no frontend gates needed |

Working tree was clean after the runs (changelog.json not modified); nothing committed.

### Security / compliance audit

- SQL: `DiscardMarkerIfConverted`, the counters and markers go through the existing `getJSON`/`ClearInProgress`/`ResetAttempts` helpers (parameterised); `PRAGMA journal_size_limit=%d` is built from a compile-time integer constant (64<<20), no user input, no injection path.
- Temp dir: `PrepareTempDir` keeps `filepath.Clean`, `Lstat`, the symlink/non-directory refusal, the owner==euid check and the 0700 check unchanged. The new `ErrTempDirSkipped` path returns early only when euid==0 and the data directory is owned by a non-root uid, and then touches nothing (a symlinked or attacker-owned `.tmp` is simply never reached; the root process does not create or chmod anything). When the process is non-root or root-owned data, the original checks apply. `ApplyTempDir` handles the skip at Debug level only.
- Back-off: the failure limit now also applies when the user's flag is set, and a flag-driven run no longer bypasses it. The flag is only set by `POST /system/database/optimize-on-restart`, registered on `managementAdmin` (routes.go:583, admin-only); that handler also calls `ResetAttempts`. No unauthenticated or non-admin path can force or skip a conversion. Pre-conversion failures count and keep the flag, bounded by the 3-attempt limit (no infinite retry loop, no repeated heavy work).
- Busy classification: `errors.As` on `Code()` matches `code & 0xff == 5` (SQLITE_BUSY and its extended codes only; SQLITE_LOCKED is 6 and is not matched). The text fallback is unchanged.
- Logs: new messages carry only the reason string; no secrets or new paths.
- DoS: retries are capped at 3; WAL cap bounds disk growth; no new unbounded loop or allocation.
- Docs: the chown advice is scoped to `<data>/.tmp`, tells the operator to stop Charon first, and mentions deleting the folder as an alternative.
- Commit hygiene: `git log a7aa2909^..HEAD --format=%B` and the diff contain no session IDs, claude.ai session links, Co-Authored-By or "generated with" lines. All 11 subjects use `fix:`, `refactor:`, `test:` or `docs:` prefixes; no `(security)` scope is used.

### Findings

None blocking. Informational:
- Low (docs/comment nit, drain.go:63-64): the doc comment now wraps unevenly after the `spike_test.go` to `driver_behavior_test.go` rename (one short line, one long line). Cosmetic only; no action required.
- Info: GH #1426 (/tmp leak) was not observed as a failure in any gate.


## Re-run: #1426 temp-leak guard

Branch `fix/test-temp-leaks-1426`, range `b9728f94..HEAD` (9 commits), backend only. All heavy commands ran with private `TMPDIR`/`GOTMPDIR` under `/var/tmp` (removed afterwards).

**Verdict: PASS. No blocking, no should-fix findings.**

### Gate results

| # | Command | Outcome |
|---|---|---|
| 1 | `cd backend && go build ./... && go vet ./...`; `gofmt -l .` | build OK, vet OK, gofmt empty |
| 2a | `go test -count=1 -race ./internal/testutil/... ./internal/services/... ./internal/api/handlers/...` (private TMPDIR) | all ok (services 504.9s, handlers 349.2s, tmpguard, testutil, remotestorage); private TMPDIR EMPTY afterwards |
| 2b | `go test -count=1 -p 4 ./...` (second private TMPDIR) | exit 0, zero failures; private TMPDIR EMPTY afterwards |
| 3 | `go test -run Discard` + `-run 'Discard\|RestoreBackup\|RestoreDB'` in services | 8 new discard tests pass (success, pre-restore-backup failure, apply failure, pending-file written keeps pending copy, unrecoverable, sidecars, empty-field no-op, already-removed); legacy `FileExists(restoreDBPath)` tests (backup_service_test.go:86, :1370) pass |
| 4 | Scratch copy (`git archive HEAD` under /var/tmp) with a deliberate-leak TestMain package | package FAILS, report names `deliberate-leak.bin (1234 bytes)`, exit 1. Decoys: young prefix dir, prefix-named symlink (to a dir with a file), old prefix-named plain file, old non-prefix dir all SURVIVED (symlink target content intact); old prefix-named dir was swept |
| 5a | `bash scripts/local-patch-report.sh` | artifacts present (md + json); patch coverage 72/72 = 100% (backend), pass |
| 5b | `scripts/go-test-coverage.sh` | exit 0; statements 92.2%, line coverage 89.2% vs gate 87%: met |
| 5c | `lefthook run pre-commit --all-files` | exit 0 (all hooks incl. semgrep: 0 findings, golangci-lint-fast, go-vet, frontend lint/type-check) |
| 5d | `make lint-fast` / `make lint-backend` | 0 issues / 0 issues |
| 6 | CodeQL / Trivy | Deferred to CI per CLAUDE.md (no feat:, no new dependency, no network surface). `./scripts/scan-gorm-security.sh --check`: PASSED, 0 issues |
| 7 | `go test -count>1` on services/handlers | Not run; known pre-existing, GH #1450 |

### Security / compliance audit

- tmpguard sweeper (`backend/internal/testutil/tmpguard/tmpguard.go`): sweeps only entries in `os.TempDir()` whose name has the `charon-gotest-` prefix, `IsDir()` per lstat (symlinks and plain files skipped), and mtime older than 24h. Only entry names are joined to the root, so it cannot reach outside the base (verified empirically in gate 4). `os.RemoveAll` does not follow symlinks. Private root comes from `os.MkdirTemp` (0700, unique).
- Low / informational (tmpguard.go `removeTree`, ~lines 100-107): `os.Chmod` in the pre-removal walk follows symlinks and there is a theoretical check-to-use window between `ReadDir` and the walk. Exploitation needs a hostile local user racing in a shared temp dir, and the chmod would only apply to files the running user owns. Not exploitable in practice; no change requested. If hardened later, use `os.Lstat` before chmod.
- Production `discardRestoreSnapshot` (`backend/internal/services/backup_restore_safe.go:242-255`): removes only `s.restoreDBPath` and its `-wal`/`-shm`. `restoreDBPath` is only ever assigned from `os.CreateTemp` outputs (backup_restore_safe.go:165, backup_service.go:1413-1419), never from user input, and is distinct from the live DB and from the pending-restore file (the pending file is a copy written by `writePendingRestoreFile`; a test asserts the pending copy is kept). The mutex requirement is documented and respected. Missing files are tolerated.
- CONTRIBUTING.md (~lines 407-413): accurate against the code (TestMain wiring in services and handlers, `charon-gotest-` prefix, 24h sweep, `t.TempDir()` advice, `TMPDIR`/`GOTMPDIR` disk-backed example).
- No secrets or tokens in logs or the diff; guard output lists only file names and sizes.
- Commit hygiene: no session IDs, claude.ai links, Co-Authored-By or "generated with" lines in `git log b9728f94..HEAD --format=%B` or in the diff. Subjects use `test:`, `fix:`, `docs:`; no `(security)` scope used (appropriate: the leak fix is not a vulnerability fix).

### Findings

None blocking. One Low informational item (tmpguard `removeTree` chmod follows symlinks, see above).

### Housekeeping

No test artifacts modified, nothing committed or pushed; `/var/tmp/charon-qa-pr1-*` scratch removed; `/tmp` untouched; `charon` container untouched.
