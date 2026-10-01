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
