---
goal: Fix flaky Go backend tests caused by unclosed SQLite *sql.DB handles racing t.TempDir() cleanup
version: 1.0
date_created: 2026-08-23
status: 'Planned'
tags: [fix, testing, flaky-test, backend, sqlite]
---

# Introduction

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

## Objective

A nightly CI run failed `TestSecurityHandler_CreateAndListDecisionAndRulesets` in
`backend/internal/api/handlers/security_handler_rules_decisions_test.go`. Every
assertion in the test passed — the failure happened afterward, during Go's
automatic `t.TempDir()` cleanup, with:

```
unlinkat ... directory not empty
```

**Confirmed root cause** (already investigated; not re-derived by this plan):
`setupSecurityTestRouterWithExtras` opens a file-backed SQLite database inside
`t.TempDir()` via `gorm.io/driver/sqlite` and never closes the underlying
`*sql.DB`. SQLite's WAL/journal sidecar files (`-wal`, `-shm`) for that
connection are still live when the test function returns. `t.TempDir()`'s
registered cleanup (`os.RemoveAll`) lists the directory, starts removing
entries, and can race a filesystem write against those still-open sidecar
files, causing `os.RemoveAll` to see a "new" entry appear mid-removal and
fail with `ENOTEMPTY`. This is load-dependent and non-deterministic — it does
not reproduce reliably locally, which is why it only surfaces under nightly
CI's heavier scheduling pressure.

**Precedent fix** — commit `35695250` ("fix: apply busy-timeout/WAL pragma to
uptime test DB to resolve SQLite lock flake") fixed a related-but-distinct
flake (`SQLITE_LOCKED_SHAREDCACHE` lock contention, not a TempDir cleanup
race) in `setupTestRouterWithUptime` by swapping a bare `gorm.Open` for the
shared `OpenTestDB(t)` helper, which:

1. Opens SQLite with `_journal_mode=WAL&_busy_timeout=5000` DSN params, and
2. Registers `t.Cleanup(func() { sqlDB, _ := db.DB(); sqlDB.Close() })`
   immediately after `gorm.Open` succeeds.

Point 2 (registering `t.Cleanup` to close the `*sql.DB` immediately after a
successful `Open`) is the mechanism this plan reuses — but note precisely
what `35695250` does and doesn't prove: `setupTestRouterWithUptime`'s DSN was
an **in-memory** shared-cache DSN both before and after that fix, so that
function never had an on-disk file for a `t.TempDir()`-cleanup race to occur
against in the first place. `35695250` fixed a different flake
(`SQLITE_LOCKED_SHAREDCACHE` lock contention) and simply happens to use the
same `t.Cleanup(Close)` idiom as part of adopting the shared `OpenTestDB(t)`
helper — it is precedent for the *code shape*, not empirical proof that this
shape resolves a TempDir race.

The actual reason closing the connection via `t.Cleanup` resolves *our* root
cause is a property of Go's own documented `testing.T.Cleanup` semantics,
independent of `35695250`: cleanup functions registered on a `*testing.T`
run in LIFO order. A `t.Cleanup` registered inside the test body — after
`t.TempDir()` was called earlier in that same test, which itself registers
an implicit `os.RemoveAll` cleanup — therefore always runs *before*
`t.TempDir()`'s own removal. Closing the connection there (triggering a WAL
checkpoint and releasing the `-wal`/`-shm` handles) is consequently always
complete before `os.RemoveAll` runs. Several of the "cleared" files below
already use this exact idiom successfully against real file-backed DSNs
inside `t.TempDir()` (e.g. `orthrus/server_test.go`, `hecate/manager_test.go`),
which is the actual in-repo evidence that the pattern works for this root
cause — not `35695250` itself.

This plan's job is **only** to find every other test in the repo with the
same structural defect (file-backed SQLite DSN sourced from `t.TempDir()`,
no matching close) and apply the same class of fix, consistently. It does
not re-litigate the root-cause mechanism above.

## Goals

- Enumerate the complete, verified set of test files exhibiting this defect.
- Apply the minimal, precedent-consistent fix (`t.Cleanup` closing the
  underlying `*sql.DB`) to each.
- Prove the fix actually resolves the race (not just "looks right") via
  repeated `-race -count=N` runs, both targeted and full-package.
- Ship as a single `fix:`-scoped PR with small, ordered, independently
  buildable commits — no behavior change, no new code paths.

# Research Findings

## Methodology

The task description named a "prior partial spot-check" of at least 14 files
as a starting point, including `security_handler_rules_decisions_test.go`,
`certificate_handler_security_test.go`, `backup_remote_handler_coverage_test.go`,
`system_permissions_handler_test.go`, `import_handler_test.go`, and "several"
in `internal/services`. Every one of those names was individually
re-verified against the actual code rather than taken on faith, because the
task explicitly requires confirming the pattern, not just the filename.

Verification steps performed:

1. `git show 35695250` — read the exact precedent diff (summarized above).
2. `grep -rn "sqlite\.Open(" backend/ --include="*_test.go"` — 601 raw call
   sites across the repo. Far too many to eyeball individually, so:
3. Narrowed to call sites whose DSN is actually **derived from
   `t.TempDir()`** (as opposed to `:memory:`, `file::memory:`,
   `file:X?mode=memory&cache=shared`, or a `t.Name()`-keyed shared-cache
   DSN) via
   `grep -rnE "(dsn|dbPath|dbFile)\s*:?=.*(filepath\.Join\(t\.TempDir\(\)|t\.TempDir\(\)\s*\+)"`.
   This is the only DSN shape that can leave a real file (and real
   `-wal`/`-shm` sidecars) sitting inside the directory `t.TempDir()` will
   later `os.RemoveAll` — an in-memory or named-shared-cache DSN has no
   on-disk file for that race to happen to, so it is **not** an instance of
   this specific root cause even if the same test also calls `t.TempDir()`
   for an unrelated purpose (e.g. a certificate/backup storage directory).
   Also checked for any inline `sqlite.Open(t.TempDir()...)` call with no
   intermediate variable, and any `path :=`/`dbFile :=` variant — none
   found beyond the `dsn`/`dbPath` cases already captured.
4. For every remaining candidate file, read the actual function body (not
   just grep proximity) to determine whether the specific `*sql.DB` behind
   that DSN is closed — via `t.Cleanup`, a `defer`, or an explicit
   mid-test `Close()` — before the function returns. A first-pass grep for
   "does `sqlDB`/`t.Cleanup` appear anywhere in the file" was **not**
   trusted on its own: it produced two false "safe" verdicts on first pass
   (`certificate_handler_test.go`, where `sqlDB, err := db.DB()` is used
   only to call `SetMaxOpenConns`/`SetMaxIdleConns`, never `Close()`; and
   `notification_handler_test.go`, where the only `Close()` calls belong to
   *different* tests that deliberately close the DB early to force a
   500 error, not to a cleanup covering the shared setup helper). Every
   file below was confirmed by reading the actual source, and cross-checked
   by comparing the count of TempDir-derived `gorm.Open` sites in the file
   against the count of genuine `t.Cleanup(...Close...)` blocks.

5. **Correction round**: after this plan's first draft, an independent
   supervisor re-verification found that step 3's regex was incomplete — it
   only matched literal `gorm.Open(sqlite.Open(` call sites and missed
   `database.Connect(dbPath)` (`backend/internal/database/database.go:51`),
   a production helper that wraps `gorm.Open(sqlite.Open(...))` internally
   but doesn't contain that literal string at any *call* site, so step 3's
   grep never surfaced tests that open their DB through it. Re-ran step 3
   with `database.Connect(` added as a second search pattern, across the
   entire `backend/` tree (not limited to previously-named candidates), via
   `grep -rn "database\.Connect(" backend/ --include="*_test.go"`. This
   surfaced 3 additional call sites across 2 files, one of them (`internal/server`)
   an entirely new package not previously considered, plus one additional
   site in an already-known file
   (`certificate_handler_test.go`) that the original `gorm.Open` sweep had
   correctly found the file for but undercounted the sites in. It also
   surfaced `backend/cmd/api/main_test.go` (8 `database.Connect` sites) —
   read in full and confirmed **already correct**: every site registers
   `t.Cleanup` closing the connection, and the file's own `TestMain` doc
   comment explicitly names this exact bug class, indicating this file was
   already hardened against it previously. Also swept the whole
   non-test backend tree for any *other* production wrapper around
   `gorm.Open(sqlite.Open(`/`sql.Open(sqlite.DriverName,` beyond
   `database.Connect` (`grep -rn "gorm\.Open(sqlite\.Open\|sql\.Open(" backend/ --include="*.go" | grep -v _test.go`) —
   the only other wrappers found (`backup_restore_safe.go`,
   `backup_service.go`, `database/errors.go`, `database/pending_restore.go`,
   `database/database.go:114`) are production functions that open **and**
   close their own connection internally within a single call, not
   test-side setup helpers that hand a live connection back to the caller —
   so they cannot exhibit this defect the way a test setup helper can, and
   were not enumerated further. This second pass was believed exhaustive at
   the time — it was not (see step 6).

6. **Second correction round — methodology change, not another regex
   patch.** A second supervisor re-verification found that step 3's regex
   had a deeper structural flaw than step 5 fixed: it required the
   `t.TempDir()` call to appear **on the same line** as the
   `dsn`/`dbPath`/`dbFile` assignment, so it could not see multi-hop
   derivation, e.g.:
   ```go
   tmpDir := t.TempDir()
   dataDir := filepath.Join(tmpDir, "data")      // hop 1 — no TempDir() on this line
   dbPath := filepath.Join(dataDir, "charon.db") // hop 2 — no TempDir() on this line either
   db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{}) // never matched by step 3's regex
   ```
   Two regex passes in a row had now under-counted, so this pass abandoned
   line-level regex matching entirely rather than patching it a third time.
   New method:
   - Enumerated every `backend/` test file containing **any** of
     `gorm.Open(sqlite.Open(`, `database.Connect(`, `sql.Open("sqlite3"`,
     `sql.Open("sqlite"`, `sql.Open(sqlite.DriverName`,
     `sql.Open(glebarezsqlite.DriverName` — 162 files total (154 for the
     first pattern alone).
   - Narrowed to the 71 of those 162 that contain `t.TempDir()`
     **anywhere in the file** (a file-level, not line-level, filter — a
     file with zero `t.TempDir()` calls cannot exhibit this root cause
     regardless of how its DSN is constructed, since there is no temp
     directory for a WAL/-shm sidecar race to happen against; this is not
     the same mistake as steps 3/5, which required line- or call-level
     proximity between the DSN and `t.TempDir()`, not mere file-level
     co-occurrence).
   - Read **every one of those 71 files in full**, manually traced each
     DSN argument back to its origin through however many `filepath.Join`/
     string-concatenation hops, and applied the same close/`t.Cleanup`
     rigor as step 4 to every site that terminated at `t.TempDir()`.
   - This surfaced 8 more affected files (23 more call sites) beyond the
     step-5 total, all inside the already-in-scope `internal/api/handlers`
     and `internal/services` packages — see the affected table below,
     rows 9-16. One of these (`backend/internal/services/backup_service_test.go`,
     line 1663) was not named by the supervisor's own re-verification
     either — it surfaced only from this pass's full-file read, which is
     itself evidence the "full read, not regex" approach was the right
     call.
   - This pass also caught a **false positive** in the supervisor's
     report: `backend/internal/services/backup_restore_safe_error_paths_test.go`'s
     `newLiveDBRestoreErrorTestService` (line 45) was flagged as a leak,
     but reading both of its call sites
     (`TestRestoreBackupSafe_RehydrateFails_NonTransient_WritesPendingFileAndWarns`
     line 75-82, `TestRestoreBackupSafe_RehydrateFails_AndPendingFileWriteFails_ReturnsUnrecoverableError`
     line 106-113) shows both explicitly call
     `sqlDB.Close()` on the returned connection as part of the test's own
     error-forcing logic (deliberately closing the DB to make the
     subsequent `RestoreBackupSafe` call hit a "database is closed" error)
     — the same already-established "deliberate mid-test close" pattern as
     several files in the cleared table below (e.g.
     `hecate_service_test.go`, `notification_service_test.go`). This file
     is **not** added to the affected list; it is listed in the cleared
     table instead, with this reasoning, so a future reader doesn't
     re-flag it from the supervisor message alone without re-checking.
   - **Empirical backstop**: per the management directive, before
     finalizing this list, ran `cd backend && go test ./... -race -count=3`
     against the current, still-unfixed tree (not the plan's proposed
     fixes) specifically watching for `unlinkat`/`ENOTEMPTY`/"directory not
     empty" output, as an independent signal alongside the manual read.
     Results and any further findings from that run are recorded in the
     Verification Plan section. This is inherently non-exhaustive on any
     single run (the task's own confirmed root cause is load-dependent and
     non-deterministic — this is precisely why it was never caught by
     ordinary local test runs before now), so it is a backstop, not a
     replacement for the manual read above.

   This enumeration was believed exhaustive for the `gorm.Open(sqlite.Open(`
   / `database.Connect(` / `sql.Open(` universe sourced from `t.TempDir()`
   specifically — see step 7 for one further correction.

7. **Third correction — `os.MkdirTemp()` is an equivalent trigger to
   `t.TempDir()`.** A third supervisor re-verification pass checked whether
   the file-level `t.TempDir()`-anywhere filter in step 6 could itself drop
   a genuine candidate whose temp-directory trigger isn't `t.TempDir()` at
   all. It found one: `backend/internal/api/handlers/backup_handler_coverage_test.go`'s
   `setupBackupTestWithDB` (lines 40-79) opens `gdb` via
   `gorm.Open(sqlite.Open(dbPath), &gorm.Config{})` (line 58) against a
   `dbPath` that traces back to `os.MkdirTemp("", "cpm-backup-db-test")`
   (line 43), not `t.TempDir()` — so it fell outside step 6's literal scope
   even though the file itself is in the 71-file `t.TempDir()`-containing
   set (for unrelated, already-correctly-cleared fixtures elsewhere in the
   same file) and was read in full. The race mechanism is identical: each
   of `setupBackupTestWithDB`'s 4 callers runs `defer os.RemoveAll(tmpDir)`
   in place of `t.TempDir()`'s implicit cleanup, and an unclosed
   connection's live WAL/-shm sidecars can race that removal exactly as
   they race `t.TempDir()`'s. Every other `os.MkdirTemp(` test file in the
   repo (8 total: `backup_handler_coverage_test.go`, `backup_handler_test.go`,
   `crowdsec_bouncer_test.go`, `logs_handler_test.go`, `backup_service_test.go`,
   `certificate_service_test.go`, `crowdsec_startup_test.go`,
   `log_service_test.go`) was checked and this is the only additional leak —
   the rest either don't open a DB near their `MkdirTemp` call, already
   close it (`backup_handler_test.go`'s `setupBackupTest`,
   `backup_service_test.go`'s `createSQLiteTestDB`), or use an
   in-memory/shared-cache DSN unrelated to the temp dir
   (`certificate_service_test.go`, `crowdsec_startup_test.go`). This
   enumeration is now believed exhaustive for both trigger shapes
   (`t.TempDir()` and `os.MkdirTemp()` + `defer os.RemoveAll`). The
   corrected complete enumeration — **17 files, 41 sites** — is reflected in
   the tables below.

### Files investigated and cleared (do NOT need this fix)

| File | Why it doesn't match |
|---|---|
| `backend/internal/api/handlers/backup_remote_handler_coverage_test.go` | DB is `gorm.Open(sqlite.Open("file::memory:"), ...)` — pure in-memory, no on-disk file. `t.TempDir()` there backs `NewBackupRemoteService`'s storage dir, unrelated to the DB connection. |
| `backend/internal/api/handlers/system_permissions_handler_test.go` | DB is `gorm.Open(sqlite.Open("file::memory:?cache=shared"), ...)` (two sites) — named shared-cache in-memory, no on-disk file. All `t.TempDir()` calls in this file back filesystem-permission test fixtures, unrelated to the DB. |
| `backend/internal/api/handlers/import_handler_test.go` | DB is `gorm.Open(sqlite.Open(":memory:"), ...)` — pure in-memory. `t.TempDir()` calls back import mount-point fixtures, unrelated to the DB. |
| `backend/internal/caddy/manager_test.go`, `manager_patch_coverage_test.go`, `manager_additional_test.go`, `manager_ssl_provider_test.go`, `manager_multicred*_test.go` | DSNs are `file:%s?mode=memory&cache=shared` (keyed by `t.Name()`) or bare `:memory:`. `t.TempDir()` backs Caddyfile output directories, unrelated to the DB connection. |
| `backend/internal/orthrus/server_test.go` | File-backed DSN in `t.TempDir()`, **but already has** `t.Cleanup(func() { _ = sqlDB.Close() })` immediately after `db.DB()` (line 37). Already precedent-correct. |
| `backend/internal/hecate/manager_test.go` | Same — file-backed, already has `t.Cleanup(func() { _ = sqlDB.Close() })` (line 33). |
| `backend/internal/services/credential_service_test.go` | File-backed, already has `t.Cleanup(func() { _ = sqlDB.Close() ... })` (lines 34-36). |
| `backend/internal/services/security_service_test.go` | File-backed, already has `t.Cleanup` closing `sqlDB` with a nil-guard (lines 30-33). |
| `backend/internal/services/stats_ingester_test.go` | File-backed, already has `t.Cleanup` closing `sqlDB` (lines 28-32). |
| `backend/internal/services/uptime_service_test.go` | File-backed, already has `t.Cleanup` closing `sqlDB` (lines 41-45). |
| `backend/internal/services/hecate_service_test.go` | File-backed, already has `t.Cleanup(func() { _ = sqlDB.Close() })` (line 31); extra mid-test `Close()` calls elsewhere in the file are deliberate error-forcing closes on already-cleanup-registered connections, not leaks. |
| `backend/internal/services/stats_service_test.go` | File-backed, already has `t.Cleanup` closing `sqlDB` (lines 29-33). |
| `backend/internal/services/orthrus_service_test.go` | File-backed, already has `t.Cleanup(func() { _ = sqlDB.Close() })` (line 27). |
| `backend/internal/services/backup_service_driver_test.go` | Uses raw `database/sql` + `github.com/glebarez/sqlite` (not GORM) with `t.Cleanup`/`defer` closes on every connection (lines 24-25, 63, 90). Already correct. |
| `backend/internal/services/notification_service_test.go` | The one TempDir-derived site (`TestNotificationService_EnsureNotifyOnlyProviderMigration_UpdateError`, ~line 1777) explicitly closes both the rw and ro connections it opens (lines 1791, 1810-1812) as part of the test's own logic. Already correct. |
| `backend/internal/database/errors_test.go`, `backend/internal/database/pending_restore_test.go`, `backend/internal/services/backup_restore_safe_integrity_test.go` | Use raw `database/sql` (`sql.Open("sqlite3", ...)`) with `defer`/explicit `Close()` on every connection, or exercise error paths where the DB is intentionally never opened (e.g. `missing-parent/missing.db`). Already correct / not applicable. |
| `backend/internal/api/handlers/backup_handler_coverage_test.go`, `backup_handler_v2_test.go` | The `t.TempDir()`-backed `.db` files here are built via `createValidSQLiteDBWithCharonTables`, which uses raw `sql.Open("sqlite3", path)` with `defer db.Close()` — never GORM, always closed. Already correct. |
| `backend/internal/services/backup_service_async_create_test.go` | `t.TempDir()`-backed `.db` paths (`healthy.db`, `not-a-db.db`) are passed to `checkDatabaseIntegrity(dbPath)` / `os.WriteFile`, never opened via `gorm.Open`. Not applicable. |
| `backend/cmd/api/main_test.go` | 8 `database.Connect(dbPath)` sites, all `t.TempDir()`-derived — but every single one already registers `t.Cleanup` closing `sqlDB` (verified by reading all 8 call sites, lines ~46-52, 98-104, 131-137, 164-188 (closes an earlier connection at 180 before reconnecting at 182, then cleans up the new one at 186-188), 233-239, 255-261, 287-293). `TestMain`'s own doc comment (lines 19-23) explicitly names this exact "TempDir RemoveAll cleanup: directory not empty" failure mode, indicating this file was already hardened against it in a prior pass. Already correct. |
| `backend/internal/services/backup_restore_safe_error_paths_test.go` | **Supervisor-flagged false positive (Methodology step 6).** `newLiveDBRestoreErrorTestService` (line 45) is multi-hop `t.TempDir()`-derived and its returned `db` is not closed *inside the helper* — but both of its 2 callers (lines 75, 106) explicitly call `sqlDB.Close()` on it as part of their own error-forcing test logic before the function returns, the same deliberate-mid-test-close pattern already established as safe elsewhere in this table. Not affected. |
| `backend/internal/services/backup_service_v2_hardening_test.go` | Its `createCharonLikeTestDB`/`newHardeningTestService` helpers (2 `t.TempDir()`-derived sites) use raw `database/sql` (`sql.Open("sqlite3", ...)`) with `t.Cleanup(func(){ db.Close() })` (line 32); `newHardeningTestService` passes `nil` as the service's `*gorm.DB` (only `DatabaseName`/`DataDir` are used), so there is no separate leaked GORM connection either. Already correct. |
| `backend/internal/api/handlers/coverage_quick_test.go`, `import_handler_coverage_test.go`, `security_handler_waf_test.go` | Each has 1-2 `t.TempDir()`-derived `gorm.Open`/paired rw+ro sites; each already closes every connection it opens (`coverage_quick_test.go` lines 29 `defer` + 60-64 `t.Cleanup`; `import_handler_coverage_test.go` lines 40-42/49-53 and 483-485/490-492, both rw-then-ro pairs fully closed; `security_handler_waf_test.go` lines 565-573, closes the DB and removes the `-wal`/`-shm` siblings too — this file is actually a good in-repo model of the target fix pattern). Already correct. |
| `backend/internal/database/pending_restore_coverage_test.go`, `pending_restore_process_test.go` | Raw `sql.Open("sqlite3", path)` helpers (`buildIntegrityCheckFailingSQLiteFile`, `createMarkerSQLiteFile`, `readMarkerValue`), each with an explicit `require.NoError(t, db.Close())` or `defer db.Close()`. Already correct. |
| `backend/internal/services/uptime_service_pr1_test.go` | 2 `t.TempDir()`-derived sites (`setupPR1TestDB`, `setupPR1ConcurrentDB`), both already register `t.Cleanup` closing `sqlDB` (lines 41-46, 451-457). Already correct. |
| `backend/internal/api/handlers/certificate_handler_patch_coverage_test.go`, `certificate_handler_upload_export_test.go`, `handlers_blackbox_test.go`, `logo_handler_test.go`, `banner_handler_test.go`, `backup_remote_handler_test.go`, `orthrus_handler_test.go`; `backend/internal/api/routes/routes_test.go`; `backend/internal/crowdsec/console_enroll_test.go`; `backend/internal/services/backup_service_options_test.go`, `backup_restore_safe_async_test.go` (its one `gorm.Open` site, line 139 — the file's other site, line 30, is the already-cleared raw-`sql.Open` `rawDB`), `backup_remote_service_test.go`, `backup_service_v1_compat_test.go`, `backup_settings_v2_test.go`, `crowdsec_whitelist_service_test.go`, `certificate_service_checkexpiry_test.go`, `certificate_service_coverage_test.go`, `certificate_service_extra_coverage_test.go`, `certificate_service_patch_coverage_test.go`, `certificate_service_sync_coverage_test.go`, `certificate_service_test.go` | All DSNs in these files trace back to `t.Name()`-keyed shared-cache in-memory DSNs (`file:%s?mode=memory&cache=shared`, sometimes with a fixed prefix) or bare `:memory:`/`file::memory:` — never to `t.TempDir()`, even where the same file separately calls `t.TempDir()` for unrelated on-disk fixtures (uploaded files, cert/backup storage dirs, mount points). Confirmed by reading every `dsn`/`dbPath` assignment in each file. Not applicable. |

### Files confirmed AFFECTED (need the fix)

**17 files, 41 distinct unclosed-connection call sites** (corrected three
times: first by a supervisor re-verification pass — Methodology step 5 —
from an initial draft of 6 files / 14 sites to 8 files / 17 sites; then by a
second, methodology-changing pass — step 6 — to 16 files / 40 sites; then by
a third, narrowly-scoped pass — step 7 — to the current 17 files / 41
sites). Rows 1-8 are unchanged from the step-5 correction; rows 9-16 are new
in step 6; row 17 is new in step 7:

| # | File | Function(s) | Lines (approx.) | Shape |
|---|---|---|---|---|
| 1 | `backend/internal/api/handlers/security_handler_rules_decisions_test.go` | `setupSecurityTestRouterWithExtras` | 24-30 (func), dsn L27, `gorm.Open` L28 | Shared setup helper, used by multiple tests incl. the originally-reported `TestSecurityHandler_CreateAndListDecisionAndRulesets` |
| 2 | `backend/internal/api/handlers/certificate_handler_security_test.go` | `TestCertificateHandler_Delete_NotificationRateLimiting` | 150-166 (func), dbPath L151, `gorm.Open` L152 | Inline, single site |
| 3 | `backend/internal/api/handlers/certificate_handler_test.go` | `TestDeleteCertificate_CreatesBackup` | 87-91 | Inline, 4 near-identical duplicated blocks with `_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=1` + pool tuning |
| 3 | (same file) | `TestDeleteCertificate_DiskSpaceCheckError` | 641-645 | " |
| 3 | (same file) | `TestDeleteCertificate_ExpiredLetsEncrypt_NotInUse` | 694-696 | " |
| 3 | (same file) | `TestDeleteCertificate_ValidLetsEncrypt_NotInUse` | 752-754 | " |
| 3 | (same file) | `TestDeleteCertificate_NotificationRateLimit` | 842-886 (func), tmpFile L843, `gorm.Open(sqlite.Open(tmpFile), ...)` L844 | **5th site, found in the correction round.** Bare DSN (no WAL/busy-timeout/foreign_keys params, no pool tuning) — structurally distinct from the other 4 in this file, so it is *not* folded into the same helper (see Technical Specifications). |
| 4 | `backend/internal/api/handlers/notification_handler_test.go` | `setupNotificationTestDB` | 20-28 (func), dsn L22, `gorm.Open` L23 | Shared setup helper, used by `TestNotificationHandler_List` and others (two *other* tests in this file deliberately close the DB mid-test to force a 500 — those are fine as-is; the helper itself is the leak) |
| 5 | `backend/internal/services/enhanced_security_notification_service_patch_coverage_test.go` | `TestEnhancedService_UpdateManagedProviders_SaveError` | 421-450ish, dbPath L422, `roDB` opened ~L440, never closed (the paired `rwDB` on the same path *is* closed at ~L438) | Inline, rw closed / ro leaked pair |
| 5 | (same file) | `TestEnhancedService_MigrateFromLegacyConfig_TransactionWriteErrors/create_managed_provider_error` (subtest) | dbPath L529, `roDB` opened ~L541, never closed | " |
| 5 | (same file) | `TestEnhancedService_MigrateFromLegacyConfig_TransactionWriteErrors/update_managed_provider_error` (subtest) | dbPath L550, `roDB` opened ~L562, never closed | " |
| 5 | (same file) | `TestEnhancedService_IsFeatureEnabled_CreateAndRequeryPath` | dbPath L573, primary `db` never closed (only the second `raceDB` connection is closed, ~L629) | Inverse of above: primary leaked, secondary closed |
| 5 | (same file) | `TestEnhancedService_IsFeatureEnabled_CreateAndRequeryErrorPath` | dbPath L713, `readonlyDB` opened ~L718, never closed (only the primary `db` is closed, ~L728) | rw closed / ro leaked pair |
| 6 | `backend/cmd/seed/seed_smoke_test.go` | `TestSeedMain_ForceAdminUpdatesExistingUserPassword` | 42-65ish, dbPath ~L63, `gorm.Open` ~L64 | Inline, package `main`, plain `t.Fatalf` idiom (no testify) |
| 6 | (same file) | `TestSeedMain_ForceAdminWithoutPasswordUpdatesMetadata` | 104-125ish, dbPath ~L123, `gorm.Open` ~L124 | " |
| 7 | `backend/internal/api/handlers/db_health_handler_test.go` | `TestDBHealthHandler_Check_CorruptedDatabase` | 207-256 (func), dbPath L211, `db2` opened via `database.Connect(dbPath)` L227, never closed | **Found in the correction round — entire file previously missed** (uses `database.Connect`, not a literal `gorm.Open(sqlite.Open(` string). The *first* connection in this function (opened L214) is correctly closed at L220-221 before the file is deliberately corrupted; the *second*, reconnected `db2` (L227) is what leaks. All 9 other `database.Connect` sites in this file (lines 42, 90, 124, 152, 182, 271, 278, 317, 359) were individually checked and are either in-memory DSNs (`file::memory:?cache=shared`, not applicable) or already properly closed (L128 `defer`, L282-284 explicit close) — this is the file's only leak. |
| 8 | `backend/internal/server/emergency_server_test.go` | `setupTestDB` | 21-37 (func), tmpFile L26, `database.Connect(tmpFile)` L27 | **Found in the correction round — entire file and package previously missed** (uses `database.Connect`; also `internal/server` was not among the packages the original candidate search touched at all). Shared setup helper used by every test in the file; the only other `Close()` calls in the file are unrelated `resp.Body.Close()` (HTTP response bodies), not the DB. |
| 9 | `backend/internal/services/backup_service_rehydrate_test.go` | `TestBackupService_RehydrateLiveDatabase` | `tmpDir`(34)→`dataDir`(35)→`dbPath`(38), `gorm.Open` L39, never closed | **All 6 sites in this file found in the second correction round (step 6) — multi-hop `t.TempDir()`→`filepath.Join`→`filepath.Join` chains step 3/5's line-level regex could never match.** |
| 9 | (same file) | `TestBackupService_RehydrateLiveDatabase_FromBackupWithWAL` | same chain shape, `gorm.Open` L81, never closed | " |
| 9 | (same file) | `TestBackupService_RehydrateLiveDatabase_InvalidRestoreDB` | `activeDB` L171, never closed | " |
| 9 | (same file) | `TestBackupService_RehydrateLiveDatabase_InvalidTableIdentifier` | `activeDB` L195 **and** `restoreDB` L200, both never closed | " |
| 9 | (same file) | `TestBackupService_RehydrateLiveDatabase_MidLoopFailure_RollsBackAtomically` | `activeDB` L256, never closed (this test's *other* connection, `restoreDB` L268, is a raw `sql.Open("sqlite3", ...)` correctly closed at L280 — not a leak) | " |
| 10 | `backend/internal/services/backup_service_wave4_test.go` | `setupRehydrateDBPair` | `activeDB` L61 **and** `restoreDB` L66, both never closed | Shared helper used by 2 tests (`TestBackupServiceWave4_Rehydrate_DetachErrorNotBusyOrLocked`, `..._WALCheckpointErrorNotBusyOrLocked`) — fixing this one function fixes both callers |
| 10 | (same file) | `TestBackupServiceWave4_Rehydrate_CheckpointWarningPath` | `activeDB` L80, never closed | Inline |
| 10 | (same file) | `TestBackupServiceWave4_Rehydrate_CreateTempFailure` | `activeDB` L100, never closed | Inline |
| 10 | (same file) | `TestBackupServiceWave4_Rehydrate_CopyErrorFromDirectorySource` | `activeDB` L115, never closed | Inline |
| 10 | (same file) | `TestBackupServiceWave4_Rehydrate_CopyTableErrorOnSchemaMismatch` | `activeDB` L134 **and** `restoreDB` L139, both never closed | Inline |
| 10 | (same file) | `TestBackupServiceWave4_Rehydrate_ClearSQLiteSequenceError` | `activeDB` L198 **and** `restoreDB` L203, both never closed | Inline |
| 10 | (same file) | `TestBackupServiceWave4_Rehydrate_CopySQLiteSequenceError` | `activeDB` L223 **and** `restoreDB` L228, both never closed | Inline (11 sites total in this file) |
| 11 | `backend/internal/services/backup_service_wave5_test.go` | `TestBackupServiceWave5_Rehydrate_FallbackWhenRestorePathMissing` | `activeDB` L20, never closed | Inline. (This file's 2 other rehydrate tests call the already-affected `setupRehydrateDBPair` from `backup_service_wave4_test.go`, row 10 — fixed there, not double-counted here.) |
| 12 | `backend/internal/services/backup_service_cleanup_db_test.go` | `TestCleanupOldBackups_ExcludesPreRestoreRecordsFromRetention` | `db` L33, never closed | Inline |
| 13 | `backend/internal/services/backup_service_encryption_required_test.go` | `newRemoteStorageTestService` | `db` L30, never closed (`t.Cleanup(svc.Stop)` L44 stops the service's background scheduler, not the DB) | Shared helper used by 2 subtests of `TestComputeEncryptionKeyRequired_PositiveAndNegative` |
| 14 | `backend/internal/services/backup_restore_safe_coverage_test.go` | `newLiveDBHardeningTestService` | `db` L288, never closed (same `t.Cleanup(svc.Stop)`-but-not-DB pattern as row 13) | Shared helper used by 2 tests. (This file's *other* TempDir-derived site, `rawDB` L139, was already correctly closed and is in the cleared table — not affected.) |
| 15 | `backend/internal/services/backup_service_test.go` | `TestBackupService_RehydrateLiveDatabase_MissingSource` | `tmpDir`(1656)→`dataDir`(1657)→`dbPath`(1660), `gorm.Open` L1663, never closed | **Not named by the supervisor's own re-verification — surfaced only from this pass's full-file read of the package's other files.** `db.DB()`/`Close()` is never called; `os.Remove(dbPath)` (L1672) removes the file but not the open connection. (This file's `createSQLiteTestDB` helper, L21-33, used by many other tests in the same file and package, is already correct — the package's own safe shared fixture helper.) |
| 16 | `backend/internal/api/handlers/crowdsec_wave7_test.go` | `TestCrowdsecWave7_Start_CreateSecurityConfigFailsOnReadOnlyDB` | `roDB` L43, never closed (paired `rwDB` L35 *is* closed at L41, deliberately, to force the read-only failure the test exercises) | Inline, rw closed / ro leaked pair — same shape as row 5's sites |
| 17 | `backend/internal/api/handlers/backup_handler_coverage_test.go` | `setupBackupTestWithDB` | 40-79 (func), `os.MkdirTemp` L43, `gdb` opened via `gorm.Open(sqlite.Open(dbPath), ...)` L58, never closed | **Found in the third correction round (step 7) — trigger is `os.MkdirTemp()` + each of the 4 callers' `defer os.RemoveAll(tmpDir)`, not `t.TempDir()`, but the same unclosed-WAL-sidecar-races-directory-removal mechanism applies.** Shared setup helper used by 4 callers (lines 88, 139, 157, 187), none of which close `gdb` either — only `t.Cleanup(svc.Stop)` (L63, stops the service) and each caller's own `defer os.RemoveAll` run. |

Cross-check: for every affected file, `grep -c "gorm.Open(sqlite.Open"` vs.
`grep -c "t.Cleanup"` was compared to the TempDir-derived-site count above —
`certificate_handler_security_test.go` (5 opens, 0 t.Cleanup),
`certificate_handler_test.go` (19 `gorm.Open` occurrences — most in-memory
and irrelevant, 0 t.Cleanup total, 5 TempDir-derived sites),
`notification_handler_test.go` (1 open, 0 t.Cleanup),
`enhanced_security_notification_service_patch_coverage_test.go` (35 opens —
most in-memory and irrelevant, 0 t.Cleanup total), `seed_smoke_test.go` (2
opens, 3 t.Cleanup — all 3 restore `os.Chdir`, none close the DB),
`db_health_handler_test.go` (10 `database.Connect` sites, 1 leak — read in
full, table above), `emergency_server_test.go` (1 `database.Connect` site,
0 DB-related `t.Cleanup`/`Close`). This matches the per-function reading
above.

Rows 9-16 cross-check: `backup_service_rehydrate_test.go` (6 `gorm.Open`
occurrences in the file, all 6 TempDir-derived, 0 t.Cleanup — every one
affected), `backup_service_wave4_test.go` (11 TempDir-derived
`gorm.Open`/pair sites, 0 t.Cleanup), `backup_service_wave5_test.go` (1
direct site + 2 calls into wave4's already-counted helper), `backup_service_cleanup_db_test.go`
(1 site, 0 t.Cleanup), `backup_service_encryption_required_test.go` (1
site, `t.Cleanup(svc.Stop)` present but not DB-closing),
`backup_restore_safe_coverage_test.go` (2 TempDir-derived sites total — 1
already-correct raw `sql.Open`, 1 leaked `gorm.Open`), `backup_service_test.go`
(2 `gorm.Open` occurrences in the whole 1700+-line file: 1 the package's
already-correct shared helper, 1 leaked), `crowdsec_wave7_test.go` (2
`gorm.Open` sites, 1 closed deliberately, 1 leaked).

**Note on the task's "at least 14 files" figure and the plan's own revision
history**: the enumeration has now been corrected three times. First draft:
6 files / 14 sites. After a supervisor re-verification (Methodology step 5,
which added `database.Connect` to the search): 8 files / 17 sites. After a
second supervisor re-verification that changed the methodology itself
rather than patching the search pattern again (step 6, full manual read of
all 71 `t.TempDir()`-containing candidate files regardless of DSN-line
shape): 16 files / 40 call sites. After a third, narrowly-scoped supervisor
pass that checked whether the `t.TempDir()`-anywhere file filter itself
could drop a genuine candidate using a different temp-directory trigger
(step 7, `os.MkdirTemp()` + `defer os.RemoveAll`): the current, believed-final
**17 files / 41 call sites**. Three of the originally-named files
(`backup_remote_handler_coverage_test.go`, `system_permissions_handler_test.go`,
`import_handler_test.go`) never exhibited this root cause at all (in-memory
DSNs — see cleared table above); one supervisor-flagged site
(`backup_restore_safe_error_paths_test.go`) was a false positive, corrected
in step 6 with evidence (see above) and independently re-confirmed as a
true false-positive in the step-7 review round; one site
(`backup_service_test.go` line 1663) was found by neither the original
spot-check nor either of the first two supervisor passes — only by this
plan's own full-file read in step 6; and one file
(`backup_handler_coverage_test.go`) was found only in the step-7 pass by
checking every `os.MkdirTemp(` test file in the repo (8 total) for the same
missing-cleanup defect under a different trigger shape. Reviewers should
still feel free to re-run the verification independently before
implementation starts — this enumeration has now been corrected three
times in a row, so it is deliberately documented as a re-checkable process
(162 candidate files → 71 containing `t.TempDir()` → 16 confirmed affected
via step 6, plus 1 more via step 7's `os.MkdirTemp()` sweep → 17 total, all
reproducible from the commands in Methodology steps 6-7) rather than
asserted as simply final.

## Existing shared test-DB helpers (why we are NOT adding a new one)

- `backend/internal/api/handlers/testdb.go` already exports `OpenTestDB(t)`
  and `OpenTestDBWithMigrations(t)`: both open an **in-memory**, shared-cache
  SQLite DB (`file:<name>_<random>?mode=memory&cache=shared&_journal_mode=WAL&_busy_timeout=5000`)
  and already register `t.Cleanup` closing the underlying `*sql.DB` (lines
  75-97). These are correct and unaffected — not touched by this plan.
- `backend/internal/testutil` exists (`WithTx`, `GetTestTx`) but is
  transaction-scoped helper logic, not a DB-open helper, and is unrelated to
  this defect.
- `backend/internal/services` (package `services`) has its own existing,
  already-correct shared fixture helper: `createSQLiteTestDB(t, dbPath)`
  in `backup_service_test.go` (lines 21-33) opens a raw `database/sql`
  connection via `sql.Open("sqlite3", dbPath)` and registers
  `t.Cleanup(func(){ db.Close() })`. It is used by many other tests across
  this package's files (e.g. `TestBackupService_CreateSQLiteSnapshot_TempDirInvalid`
  in `backup_service_rehydrate_test.go`, `TestBackupServiceWave4_Rehydrate_CreateTempFailure`
  in `backup_service_wave4_test.go`) and is correctly unaffected — not
  touched by this plan. It was considered as a possible reuse target for
  some of the 8 newly-found `internal/services` sites (row 9-15 above),
  but most of those sites need a live `*gorm.DB` handle back (for
  `AutoMigrate`, `db.Create`, GORM callbacks, etc.), which `createSQLiteTestDB`
  deliberately doesn't return (it returns nothing — it only seeds a file on
  disk for something else to open) — so it isn't a drop-in fit for any of
  them, and each site keeps its own `gorm.Open` call with cleanup added,
  same as every other row in the affected table.
- Every one of the 17 affected files uses a **file-backed** DSN specifically
  (several with an explicit comment: *"Use a file-backed sqlite DB to avoid
  shared memory connection issues in tests"* in
  `security_handler_rules_decisions_test.go`, and *"Use isolated file-backed
  DB to avoid lock flakiness from shared in-memory connections and
  background sync"* in `certificate_handler_test.go`). These comments
  indicate a real, prior functional reason for file-backing (background
  goroutines — e.g. `CertificateService`'s async disk-space/backup sync, or
  `caddy.Manager.ApplyConfig` — opening their own additional connections to
  the same on-disk path). Silently swapping these to the existing in-memory
  `OpenTestDB(t)` helper would be a **behavior change**, not a pure flake
  fix, and is out of scope for a `fix:`-scoped commit. The correct minimal
  fix is to keep each site file-backed and simply close the connection it
  already opens.
- Given that, and given the affected sites span four different Go packages
  (`handlers`, `services`, `server`, and `main` in `cmd/seed`) with no
  existing common import point, introducing a brand-new shared cross-package
  `testutil` helper purely to close a `*sql.DB` would be the kind of "large
  unrelated refactor" CLAUDE.md's DRY guidance explicitly says not to force.
  Instead:
  - Where duplication is **within the same file** (certificate_handler_test.go's
    4 identical blocks — its 5th, structurally different site is fixed
    inline instead, see Technical Specifications; the two near-duplicate
    rw/ro pairs pattern in
    `enhanced_security_notification_service_patch_coverage_test.go`), extract
    a small **file-local, unexported** helper — a same-file, same-package,
    zero-risk DRY win, per CLAUDE.md's "consolidate after the second
    occurrence" rule.
  - Where a site is a shared setup helper already used by multiple tests in
    its file (`setupSecurityTestRouterWithExtras`,
    `setupNotificationTestDB`, `emergency_server_test.go`'s `setupTestDB`,
    `db_health_handler_test.go`'s single site, `backup_service_wave4_test.go`'s
    `setupRehydrateDBPair`, `backup_service_encryption_required_test.go`'s
    `newRemoteStorageTestService`, `backup_restore_safe_coverage_test.go`'s
    `newLiveDBHardeningTestService`), fix that one function/site — every
    caller in the file is fixed for free where applicable.
  - Everywhere else, apply the same 3-line `t.Cleanup` fix inline, matching
    each file's existing idiom (`require.NoError` vs. `t.Fatalf`).

# Technical Specifications

## Chosen fix pattern

The applicable half of the `35695250` precedent (grab `*sql.DB`, register
`t.Cleanup` to close it — **not** the `SetMaxOpenConns(1)` half, which
solved a different, lock-contention flake and is not needed for this
TempDir-cleanup-race root cause; where a site already calls
`SetMaxOpenConns`/`SetMaxIdleConns` for its own pre-existing reasons, that is
left untouched):

### Pattern A — testify (`require`) idiom (all `internal/api/handlers` and `internal/services` sites)

```go
db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
require.NoError(t, err)

// Registered immediately after a successful Open so the connection (and
// its WAL/-shm sidecar files) is always released before t.TempDir()'s own
// cleanup runs — t.Cleanup fires in LIFO order, so this runs first.
sqlDB, err := db.DB()
require.NoError(t, err)
t.Cleanup(func() { _ = sqlDB.Close() })
```

### Pattern B — plain `t.Fatalf` idiom (`backend/cmd/seed/seed_smoke_test.go`, package `main`, no testify import)

```go
db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
if err != nil {
    t.Fatalf("open db: %v", err)
}
sqlDB, err := db.DB()
if err != nil {
    t.Fatalf("access sql db: %v", err)
}
t.Cleanup(func() { _ = sqlDB.Close() })
```

### Pattern C — file-local helper (certificate_handler_test.go's 4 duplicated blocks)

Add one unexported helper near the top of the file, alongside the existing
tests, and replace each of the 4 identical ~10-line open blocks with a
1-line call:

```go
// openCertHandlerTestDB opens a file-backed, single-connection SQLite DB
// under a fresh t.TempDir() (this handler's async backup/disk-space sync
// requires real file-backing, not an in-memory shared-cache DSN — see
// setupSecurityTestRouterWithExtras for the same constraint in this
// package) and registers cleanup to close it before t.TempDir() removes
// the directory.
func openCertHandlerTestDB(t *testing.T, filename string) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), filename)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=1", dbPath)), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to access sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
```
Each of the 4 call sites becomes `db := openCertHandlerTestDB(t, "cert_create_backup.db")` (etc.), dropping the duplicated boilerplate while keeping every site's existing `filepath.Join(t.TempDir(), ...)` — i.e. still one fresh isolated DB file per test, just via a shared local function. Requires adding `"path/filepath"` to this file's import block (not currently imported there).

**`TestDeleteCertificate_NotificationRateLimit` (the file's 5th site, lines
842-886) is deliberately NOT folded into `openCertHandlerTestDB`.** Its
existing DSN is a bare `sqlite.Open(tmpFile)` with no
`_journal_mode`/`_busy_timeout`/`_foreign_keys` query params and no
`SetMaxOpenConns`/`SetMaxIdleConns` pool tuning — routing it through the
helper would silently change its connection behavior, which is out of scope
for a pure flake fix. Instead apply a minimal Pattern-A-style inline fix
that touches nothing but adds the missing cleanup:

```go
tmpFile := t.TempDir() + "/rate_limit_test.db"
db, err := gorm.Open(sqlite.Open(tmpFile), &gorm.Config{})
if err != nil {
	t.Fatalf("failed to open db: %v", err)
}
sqlDB, err := db.DB()
if err != nil {
	t.Fatalf("failed to access sql db: %v", err)
}
t.Cleanup(func() { _ = sqlDB.Close() })
```

### Pattern E — `database.Connect` sites (`db_health_handler_test.go`, `emergency_server_test.go`)

Same fix, same idiom as Pattern A — `database.Connect` returns a `*gorm.DB`
exactly like `gorm.Open(sqlite.Open(...))` does (it's a thin wrapper, see
`backend/internal/database/database.go:51`), so no new pattern shape is
needed, only the addition of the missing cleanup at each site:

`db_health_handler_test.go`, `TestDBHealthHandler_Check_CorruptedDatabase`
(only `db2` needs it — `db`, the first connection, already closes correctly
at lines 220-221):
```go
db2, err := database.Connect(dbPath)
if err != nil {
	t.Skip("Database connection failed immediately on corruption")
}
t.Cleanup(func() {
	if sqlDB2, sqlErr := db2.DB(); sqlErr == nil {
		_ = sqlDB2.Close()
	}
})
```
(Registered after the existing `if err != nil { t.Skip(...) }` guard, since
`t.Skip` exits before `db2` is usable — no point registering a cleanup for a
connection the test already gave up on.)

`emergency_server_test.go`, `setupTestDB` (fixes every caller in the file):
```go
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	tmpFile := t.TempDir() + "/test.db"
	db, err := database.Connect(tmpFile)
	require.NoError(t, err, "Failed to create test database")

	sqlDB, err := db.DB()
	require.NoError(t, err, "Failed to access underlying sql.DB")
	t.Cleanup(func() { _ = sqlDB.Close() })

	err = db.AutoMigrate(
		&models.Setting{},
		&models.SecurityConfig{},
		&models.SecurityAudit{},
	)
	require.NoError(t, err, "Failed to run migrations")

	return db
}
```

### Pattern D — paired rw/ro connections (`enhanced_security_notification_service_patch_coverage_test.go`, 5 sites)

Each of the 5 sites already closes *one* of its two connections as part of
the test's own logic (to force a read-only/locked-DB error path); only the
*other*, previously-unclosed connection needs a cleanup added, matching the
file's existing nil-guard style already used elsewhere in the same file
(e.g. line ~727: `sqlDB, sqlErr := db.DB(); if sqlErr == nil { _ = sqlDB.Close() }`):

```go
roDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=ro", dbPath)), &gorm.Config{})
require.NoError(t, err)
t.Cleanup(func() {
    if roSQLDB, sqlErr := roDB.DB(); sqlErr == nil {
        _ = roSQLDB.Close()
    }
})
```
(For `TestEnhancedService_IsFeatureEnabled_CreateAndRequeryPath`, the leaked
connection is the *primary* `db`, not a `roDB` — same fix, applied to `db`
right after its `gorm.Open` instead.)

### Pattern F — multi-hop `t.TempDir()` sites (affected table rows 9, 11, 12, 15)

These are structurally identical to Pattern A — the only difference is the
DSN is reached through 2-3 `filepath.Join` hops instead of one, which is
exactly what step 6's methodology change was needed to see in the first
place. The fix doesn't care how many hops there were; it only needs the
resulting `*gorm.DB`:

```go
tmpDir := t.TempDir()
dataDir := filepath.Join(tmpDir, "data")
require.NoError(t, os.MkdirAll(dataDir, 0o700))

dbPath := filepath.Join(dataDir, "charon.db")
db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
require.NoError(t, err)

sqlDB, err := db.DB()
require.NoError(t, err)
t.Cleanup(func() { _ = sqlDB.Close() })
```
Applies as-is to `backup_service_rehydrate_test.go`'s 4 single-connection
sites (rows 9: `TestBackupService_RehydrateLiveDatabase`,
`..._FromBackupWithWAL`, `..._InvalidRestoreDB`,
`..._MidLoopFailure_RollsBackAtomically`'s `activeDB`),
`backup_service_wave5_test.go` (row 11), `backup_service_cleanup_db_test.go`
(row 12), and `backup_service_test.go`'s `TestBackupService_RehydrateLiveDatabase_MissingSource`
(row 15). Where a test opens **two** independent connections this way in
the same function with neither closed (`backup_service_rehydrate_test.go`'s
`TestBackupService_RehydrateLiveDatabase_InvalidTableIdentifier`, row 9 —
`activeDB` **and** `restoreDB`), apply the pattern twice, once per
connection, each with its own `t.Cleanup`.

### Pattern G — shared helpers returning one connection to multiple callers (affected table rows 10, 13, 14, 16)

Same underlying fix as Pattern A/D, applied once at the helper definition
so every caller is fixed for free — the shape differs per helper only in
how many connections it opens and what it returns:

**`backup_service_wave4_test.go`'s `setupRehydrateDBPair`** (row 10) opens
*two* connections and returns only one of them (`activeDB`) plus two path
strings — the second connection (`restoreDB`) is deliberately never
returned to the caller (it's only used to seed the restore-source fixture
file before the function returns), so its cleanup must be registered
*inside* the helper, not left to the caller:
```go
func setupRehydrateDBPair(t *testing.T) (db *gorm.DB, activeDataDir, restoreDBPath string) {
	t.Helper()
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))

	activeDBPath := filepath.Join(tmpDir, "active.db")
	activeDB, err := gorm.Open(sqlite.Open(activeDBPath), &gorm.Config{})
	require.NoError(t, err)
	activeSQLDB, err := activeDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = activeSQLDB.Close() })
	require.NoError(t, activeDB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`).Error)

	restoreDBPath = filepath.Join(tmpDir, "restore.db")
	restoreDB, err := gorm.Open(sqlite.Open(restoreDBPath), &gorm.Config{})
	require.NoError(t, err)
	restoreSQLDB, err := restoreDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = restoreSQLDB.Close() })
	require.NoError(t, restoreDB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`).Error)
	require.NoError(t, restoreDB.Exec(`INSERT INTO users (name) VALUES ('alice')`).Error)

	return activeDB, dataDir, restoreDBPath
}
```
This same per-connection `t.Cleanup` is also applied inline to the file's 6
other, non-helper sites in the same file (row 10's remaining entries) —
each is a standalone Pattern F/A-style fix, not routed through this helper.

**`backup_service_encryption_required_test.go`'s `newRemoteStorageTestService`**
(row 13) and **`backup_restore_safe_coverage_test.go`'s
`newLiveDBHardeningTestService`** (row 14) share one shape: both already
call `t.Cleanup(svc.Stop)` to stop the `BackupService`'s background
scheduler, but neither closes the `*gorm.DB` they construct and hand to
`NewBackupService`. Add the missing `*sql.DB` cleanup alongside the
existing one (do not remove `t.Cleanup(svc.Stop)` — both are needed, for
different resources):
```go
db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
require.NoError(t, err)
// ... AutoMigrate / seed data as before ...

sqlDB, err := db.DB()
require.NoError(t, err)
t.Cleanup(func() { _ = sqlDB.Close() })

cfg := &config.Config{DatabasePath: dbPath}
svc := NewBackupService(cfg, db, nil)
t.Cleanup(svc.Stop)
return svc
```

**`crowdsec_wave7_test.go`'s `TestCrowdsecWave7_Start_CreateSecurityConfigFailsOnReadOnlyDB`**
(row 16) is a single inline test, not a shared helper, but structurally
identical to Pattern D's rw-closed/ro-leaked shape — apply Pattern D's
exact fix to its `roDB` (line 43).

**`backup_handler_coverage_test.go`'s `setupBackupTestWithDB`** (row 17) is
a shared helper fixing all 4 of its callers at once, same shape as Pattern
A/G — the only difference from the rest of this plan is that its temp
directory comes from `os.MkdirTemp()` + each caller's own
`defer os.RemoveAll(tmpDir)` rather than `t.TempDir()`'s implicit cleanup
(see Methodology step 7); the fix itself is unchanged:
```go
gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
require.NoError(t, err)

sqlDB, err := gdb.DB()
require.NoError(t, err)
t.Cleanup(func() { _ = sqlDB.Close() })
```
(Registered immediately after line 58's existing `require.NoError(t, err)`,
alongside the helper's existing `t.Cleanup(svc.Stop)` at line 63 — both are
needed, for different resources, same as Pattern G's row 13/14 helpers.)

## Non-goals / explicit exclusions

- No changes to `backend/internal/api/handlers/testdb.go`'s `OpenTestDB` /
  `OpenTestDBWithMigrations` — already correct.
- No changes to any of the "cleared" files in the Research Findings table.
- No new shared cross-package `testutil` DB-open helper.
- No behavior change to any handler, service, or model — this PR touches
  only `_test.go` files.
- No CI/CD, Dockerfile, `.gitignore`, `.dockerignore`, or `.codecov.yml`
  changes — confirmed not needed (see below).

## `.gitignore` / `.dockerignore` / `.codecov.yml` check

Explicitly confirmed: **no changes needed**. `.gitignore` and
`.dockerignore` already ignore `*.db` and `backend/test-output*.txt`
broadly (lines checked: `.gitignore:93` `*.db`, `.dockerignore:63-73` test
output patterns), which already covers anything a fixed or unfixed test
might transiently write. `.codecov.yml` has no per-file exclusions relevant
to the 17 touched files, and this change adds no new files, only edits
existing `_test.go` files in place — patch coverage is unaffected in any
way that requires config changes.

# Implementation Plan

## Phase 1: N/A — no new behavior, no Playwright coverage needed

This is a Go backend test-code-only flakiness fix with zero product-facing
or API-facing behavior change. Per CLAUDE.md's Definition of Done, Playwright
E2E coverage exists for user-facing behavior; there is none to add or run
here. Skip Phase 1 (E2E specs) entirely — do not write or run Playwright
tests for this change.

## Phase 2: Backend fix — `internal/api/handlers` package

Apply Patterns A, C, D, and E to the 7 files in this package (11 of the 41
sites: 1 + 1 + 5 + 1 + 1 + 1 + 1 across
`security_handler_rules_decisions_test.go`,
`certificate_handler_security_test.go`, `certificate_handler_test.go`,
`notification_handler_test.go`, `db_health_handler_test.go`,
`crowdsec_wave7_test.go`, and `backup_handler_coverage_test.go`). See
Commit Slicing Strategy, Commit 1.

## Phase 3a: Backend fix — `internal/services` package, notifications sub-area

Apply Pattern D to `enhanced_security_notification_service_patch_coverage_test.go`
(5 of the 41 sites). See Commit Slicing Strategy, Commit 2.

## Phase 3b: Backend fix — `internal/services` package, backup/restore sub-area

Apply Patterns F and G to the 7 `backup_service_*`/`backup_restore_safe_*`
files (22 of the 41 sites: `backup_service_rehydrate_test.go` (6),
`backup_service_wave4_test.go` (11), `backup_service_wave5_test.go` (1),
`backup_service_cleanup_db_test.go` (1),
`backup_service_encryption_required_test.go` (1),
`backup_restore_safe_coverage_test.go` (1), `backup_service_test.go` (1)).
Split from Phase 3a into its own phase/commit for reviewability — see
Commit Slicing Strategy, Commit 3, for the rationale.

## Phase 4: Backend fix — stragglers (`cmd/seed`)

Apply Pattern B to `backend/cmd/seed/seed_smoke_test.go` (2 of the 41
sites). See Commit Slicing Strategy, Commit 4.

## Phase 5: Backend fix — `internal/server` package

Apply Pattern E to `backend/internal/server/emergency_server_test.go`'s
`setupTestDB` helper (1 of the 41 sites). New package, found in the first
correction round — see Commit Slicing Strategy, Commit 5.

## Phase 6: Verification and documentation

No user-facing docs change (test-only fix). Verification plan below stands
in for "Phase 6" in this case — see Verification Plan section.

# Commit Slicing Strategy

**Decision**: single PR (`fix:`-scoped — flaky test infrastructure, no new
behavior or code paths), with 5 ordered, independently-buildable commits,
grouped by Go package boundary (and, within `internal/services`, by
sub-area — see Commit 3's rationale) as the natural, lowest-risk seam (each
commit's `go test ./...` blast radius is fully contained to one package,
and Commit 3 is further scoped to one functional area within it).

No separate "shared helper / foundation" commit is needed ahead of these
five — per the Research Findings above, no new shared helper is being
introduced; each commit is self-contained (its own inline/file-local fix).

### Commit 1 — `fix: close leaked SQLite test connections in internal/api/handlers`

- **Scope**: Apply Pattern A to `security_handler_rules_decisions_test.go`
  (`setupSecurityTestRouterWithExtras`, lines 24-30) and
  `notification_handler_test.go` (`setupNotificationTestDB`, lines 20-28).
  Apply Pattern A to `certificate_handler_security_test.go`
  (`TestCertificateHandler_Delete_NotificationRateLimiting`, lines
  150-166). Apply Pattern C to `certificate_handler_test.go` (add
  `openCertHandlerTestDB` helper + `"path/filepath"` import; replace the 4
  duplicated blocks at lines ~87-100, ~641-655, ~694-708, ~752-766) **and**
  the inline fix for its 5th, structurally-different site,
  `TestDeleteCertificate_NotificationRateLimit` (lines 842-886 — not folded
  into the helper, see Technical Specifications). Apply Pattern E to
  `db_health_handler_test.go`'s single leak,
  `TestDBHealthHandler_Check_CorruptedDatabase` (lines 207-256, the `db2`
  reconnect at line 227). Apply Pattern D to `crowdsec_wave7_test.go`'s
  single leak, `TestCrowdsecWave7_Start_CreateSecurityConfigFailsOnReadOnlyDB`
  (lines 30-56, the `roDB` at line 43). Apply Pattern A/G to
  `backup_handler_coverage_test.go`'s `setupBackupTestWithDB` (lines 40-79,
  the `gdb` at line 58), fixing its 4 callers for free.
- **Files**:
  - `backend/internal/api/handlers/security_handler_rules_decisions_test.go`
  - `backend/internal/api/handlers/notification_handler_test.go`
  - `backend/internal/api/handlers/certificate_handler_security_test.go`
  - `backend/internal/api/handlers/certificate_handler_test.go`
  - `backend/internal/api/handlers/db_health_handler_test.go`
  - `backend/internal/api/handlers/crowdsec_wave7_test.go`
  - `backend/internal/api/handlers/backup_handler_coverage_test.go`
- **Dependencies**: none (first commit).
- **Validation gate**:
  ```bash
  cd backend
  go build ./...
  go vet ./internal/api/handlers/...
  go test ./internal/api/handlers/... -race -count=1
  go test ./internal/api/handlers/... -run TestSecurityHandler_CreateAndListDecisionAndRulesets -count=10 -race
  go test ./internal/api/handlers/... -run 'TestNotificationHandler_List|TestNotificationHandler_MarkAllAsRead_Error|TestNotificationHandler_DBError' -count=10 -race
  go test ./internal/api/handlers/... -run TestCertificateHandler_Delete_NotificationRateLimiting -count=10 -race
  go test ./internal/api/handlers/... -run 'TestDeleteCertificate_CreatesBackup|TestDeleteCertificate_DiskSpaceCheckError|TestDeleteCertificate_ExpiredLetsEncrypt_NotInUse|TestDeleteCertificate_ValidLetsEncrypt_NotInUse|TestDeleteCertificate_NotificationRateLimit' -count=10 -race
  go test ./internal/api/handlers/... -run TestDBHealthHandler_Check_CorruptedDatabase -count=10 -race
  go test ./internal/api/handlers/... -run TestCrowdsecWave7_Start_CreateSecurityConfigFailsOnReadOnlyDB -count=10 -race
  go test ./internal/api/handlers/... -run TestBackupHandler -count=10 -race  # backend-dev: verify/adjust this -run regex against setupBackupTestWithDB's 4 actual caller test names (lines ~88, 139, 157, 187) before relying on it — name not independently confirmed during planning
  make lint-fast   # staticcheck, blocking per CLAUDE.md
  ```
  All must pass with zero failures and zero `unlinkat`/`ENOTEMPTY` output.

### Commit 2 — `fix: close leaked SQLite test connections in internal/services (notifications)`

- **Scope**: Apply Pattern D to all 5 sites in
  `enhanced_security_notification_service_patch_coverage_test.go`
  (`TestEnhancedService_UpdateManagedProviders_SaveError`,
  `TestEnhancedService_MigrateFromLegacyConfig_TransactionWriteErrors`'s two
  subtests, `TestEnhancedService_IsFeatureEnabled_CreateAndRequeryPath`,
  `TestEnhancedService_IsFeatureEnabled_CreateAndRequeryErrorPath`).
- **Files**:
  - `backend/internal/services/enhanced_security_notification_service_patch_coverage_test.go`
- **Dependencies**: none — independent of Commit 1 (different package,
  could be reordered or cherry-picked separately if needed), but ordered
  second to keep the PR's diff grouped by package for reviewability.
- **Validation gate**:
  ```bash
  cd backend
  go build ./...
  go vet ./internal/services/...
  go test ./internal/services/... -race -count=1
  go test ./internal/services/... -run TestEnhancedService_UpdateManagedProviders_SaveError -count=10 -race
  go test ./internal/services/... -run TestEnhancedService_MigrateFromLegacyConfig_TransactionWriteErrors -count=10 -race
  go test ./internal/services/... -run TestEnhancedService_IsFeatureEnabled_CreateAndRequeryPath -count=10 -race
  go test ./internal/services/... -run TestEnhancedService_IsFeatureEnabled_CreateAndRequeryErrorPath -count=10 -race
  make lint-fast
  ```

### Commit 3 — `fix: close leaked SQLite test connections in internal/services (backup/restore)`

- **Rationale for splitting from Commit 2**: after the second correction
  round, `internal/services` grew to 8 affected files / 27 sites — too
  large and functionally mixed (notifications vs. backup/restore
  internals) for one reviewable commit. Split by sub-area, per the
  management directive: `enhanced_security_notification_service_patch_coverage_test.go`
  stays in Commit 2 (notifications); the `backup_service_*`/
  `backup_restore_safe_*` family — a natural, self-contained functional
  cluster that already share fixtures and helpers with each other (e.g.
  `createSQLiteTestDB`, `setupRehydrateDBPair`) — becomes this commit.
- **Scope**: Apply Pattern F to the single/multi-connection inline sites
  and Pattern G to the 3 shared helpers, across:
  - `backup_service_rehydrate_test.go` — 6 sites (`TestBackupService_RehydrateLiveDatabase`,
    `..._FromBackupWithWAL`, `..._InvalidRestoreDB`,
    `..._InvalidTableIdentifier` (2 connections),
    `..._MidLoopFailure_RollsBackAtomically`)
  - `backup_service_wave4_test.go` — 11 sites (`setupRehydrateDBPair` helper
    fixing 2 callers, plus 5 standalone tests, one with 2 connections)
  - `backup_service_wave5_test.go` — 1 site (`TestBackupServiceWave5_Rehydrate_FallbackWhenRestorePathMissing`)
  - `backup_service_cleanup_db_test.go` — 1 site (`TestCleanupOldBackups_ExcludesPreRestoreRecordsFromRetention`)
  - `backup_service_encryption_required_test.go` — 1 site (`newRemoteStorageTestService` helper)
  - `backup_restore_safe_coverage_test.go` — 1 site (`newLiveDBHardeningTestService` helper)
  - `backup_service_test.go` — 1 site (`TestBackupService_RehydrateLiveDatabase_MissingSource`)
- **Files**:
  - `backend/internal/services/backup_service_rehydrate_test.go`
  - `backend/internal/services/backup_service_wave4_test.go`
  - `backend/internal/services/backup_service_wave5_test.go`
  - `backend/internal/services/backup_service_cleanup_db_test.go`
  - `backend/internal/services/backup_service_encryption_required_test.go`
  - `backend/internal/services/backup_restore_safe_coverage_test.go`
  - `backend/internal/services/backup_service_test.go`
- **Dependencies**: none — independent of Commit 2 (disjoint files within
  the same package); ordered directly after it since both are
  `internal/services`.
- **Validation gate**:
  ```bash
  cd backend
  go build ./...
  go vet ./internal/services/...
  go test ./internal/services/... -race -count=1
  go test ./internal/services/... -run 'TestBackupService_RehydrateLiveDatabase|TestBackupService_RehydrateLiveDatabase_FromBackupWithWAL|TestBackupService_RehydrateLiveDatabase_InvalidRestoreDB|TestBackupService_RehydrateLiveDatabase_InvalidTableIdentifier|TestBackupService_RehydrateLiveDatabase_MidLoopFailure_RollsBackAtomically' -count=10 -race
  go test ./internal/services/... -run 'TestBackupServiceWave4_Rehydrate_DetachErrorNotBusyOrLocked|TestBackupServiceWave4_Rehydrate_WALCheckpointErrorNotBusyOrLocked|TestBackupServiceWave4_Rehydrate_CheckpointWarningPath|TestBackupServiceWave4_Rehydrate_CreateTempFailure|TestBackupServiceWave4_Rehydrate_CopyErrorFromDirectorySource|TestBackupServiceWave4_Rehydrate_CopyTableErrorOnSchemaMismatch|TestBackupServiceWave4_Rehydrate_ClearSQLiteSequenceError|TestBackupServiceWave4_Rehydrate_CopySQLiteSequenceError' -count=10 -race
  go test ./internal/services/... -run TestBackupServiceWave5_Rehydrate_FallbackWhenRestorePathMissing -count=10 -race
  go test ./internal/services/... -run TestCleanupOldBackups_ExcludesPreRestoreRecordsFromRetention -count=10 -race
  go test ./internal/services/... -run TestComputeEncryptionKeyRequired_PositiveAndNegative -count=10 -race
  go test ./internal/services/... -run 'TestRestoreBackupSafe_LiveDBAttached_RehydratesAndReloadsCaddy' -count=10 -race
  go test ./internal/services/... -run TestBackupService_RehydrateLiveDatabase_MissingSource -count=10 -race
  make lint-fast
  ```
  All must pass with zero failures and zero `unlinkat`/`ENOTEMPTY` output.
  Note the first `-run` regex above intentionally also matches
  `TestBackupService_RehydrateLiveDatabase_MissingSource` and
  `..._FromBackupWithWAL` as substring matches of the base name — harmless
  (it just runs a superset), but if tightened, ensure
  `TestBackupService_RehydrateLiveDatabase_MissingSource` (row 15, a
  different file) is still covered by its own explicit run above.

### Commit 4 — `fix: close leaked SQLite test connections in cmd/seed`

- **Scope**: Apply Pattern B to `TestSeedMain_ForceAdminUpdatesExistingUserPassword`
  and `TestSeedMain_ForceAdminWithoutPasswordUpdatesMetadata`.
- **Files**:
  - `backend/cmd/seed/seed_smoke_test.go`
- **Dependencies**: none — independent of Commits 1-3; ordered next as
  the "straggler" grouping (single small file, different package again,
  package `main`).
- **Validation gate**:
  ```bash
  cd backend
  go build ./...
  go vet ./cmd/seed/...
  go test ./cmd/seed/... -race -count=1
  go test ./cmd/seed/... -run 'TestSeedMain_ForceAdminUpdatesExistingUserPassword|TestSeedMain_ForceAdminWithoutPasswordUpdatesMetadata' -count=10 -race
  make lint-fast
  ```

### Commit 5 — `fix: close leaked SQLite test connection in internal/server`

- **Scope**: Apply Pattern E to `setupTestDB` (lines 21-37), fixing every
  test in the file that calls it.
- **Files**:
  - `backend/internal/server/emergency_server_test.go`
- **Dependencies**: none — independent of Commits 1-4; a distinct package,
  found only in the first correction round (see Methodology step 5).
  Ordered last for the same reason as CLAUDE.md's own package-boundary-first
  commit-slicing guidance: it's the smallest, most isolated grouping,
  consistent with treating it as its own seam rather than folding it into
  an unrelated commit.
- **Validation gate**:
  ```bash
  cd backend
  go build ./...
  go vet ./internal/server/...
  go test ./internal/server/... -race -count=1
  go test ./internal/server/... -run TestEmergencyServer -count=10 -race
  make lint-fast
  ```

### PR-level rollback and contingency notes

- Every commit is additive-only within existing test functions (adds a
  `t.Cleanup`/helper call; changes zero assertions, zero production code).
  If any commit's validation gate fails, the fix is isolated to that one
  commit/package (or, for Commits 2/3, sub-area) and can be reverted
  independently without affecting the other four.
- If a `-count=10 -race` run for a given site still intermittently fails
  post-fix, that is a signal the site has a **second, distinct** flake
  (e.g. genuine lock contention, closer to the `35695250` precedent's
  original bug) layered on top of the TempDir-cleanup race — do not
  broaden this PR to chase it; file a follow-up issue and note it in the PR
  description instead, since that would be a different root cause than the
  one this plan is scoped to.
- Because no production code changes, rollback of the entire PR (via
  `git revert`) is always safe and carries zero functional risk — worst
  case reintroduces the pre-existing flake, nothing else.
- Per CLAUDE.md Definition of Done step 3: this is a `fix:`-scoped change
  with **no new code paths** (pure test-code edits). Local CodeQL
  Go/JS scans, Trivy, and the GORM security scan may all be **deferred to
  CI** — do not run them locally for this PR; CI runs them unconditionally
  regardless. `backend-dev` should not spend time on step 3 locally for
  this task.
- Step 1.5 (GORM Security Scan) trigger is `backend/internal/models/**`,
  GORM queries, or migrations — this PR touches none of those (it edits
  test-setup DSN handling, not queries or schema), so it is also skipped
  per its own stated trigger condition, independent of the `fix:` deferral
  above.

# Verification Plan

## Pre-implementation empirical backstop (already run — results below)

Per the management directive accompanying Methodology step 6, before
finalizing the affected-file list this plan ran an independent empirical
check against the **current, unfixed** tree (not a hypothetical — this was
actually executed during planning, not merely specified for later):

```bash
cd backend && go test ./... -race -count=3 2>&1 | tee /tmp/stress_run.log
```

**Result: zero occurrences of `unlinkat`, `ENOTEMPTY`, or "directory not
empty" anywhere in the full output.** (This backstop ran at the point the
manual enumeration stood at 16 files / 40 sites, before Methodology step 7's
further correction to 17/41 — it was not re-run after step 7, since step
7's finding was reachable and reasoned about directly from source, not
dependent on this empirical signal.) This neither confirms nor contradicts
the manual enumeration above — the task's own
confirmed root cause is explicitly load/scheduling-dependent and
non-deterministic (that's why it survived undetected through ordinary
local runs and only surfaced once, under nightly CI's heavier scheduling
pressure), so a clean 3x run is expected regardless of whether the 40
sites are real leaks. Its value is as an independent second signal
alongside the manual read, per the management directive, not as a
replacement for it — and it did not surface any additional site the manual
read missed (rerun it after implementing this plan's fixes, and it should
remain clean at higher `-count`, per item 2 below).

The same run surfaced a large volume of **unrelated pre-existing
failures** that are explicitly out of scope for this plan and must not be
conflated with the TempDir-cleanup-race flake under investigation:
- `internal/crowdsec` package tests failed on network access to
  `hub-data.crowdsec.net` being blocked in this sandbox (403/HTML-instead-
  of-JSON responses) — an environment/connectivity limitation, not a code
  defect.
- A large number of `internal/api/handlers`, `internal/api/routes`,
  `internal/api/middleware`, `internal/caddy`, and `internal/models` tests
  failed with `UNIQUE constraint failed` or similar collisions, consistent
  with `-count=3` replaying the same test functions 3x within one process.
  These collisions were observed but their exact per-test mechanism was not
  individually traced for every failure — the repo is known to contain
  `t.Name()`-keyed shared-cache in-memory DSNs (e.g.
  `file:%s?mode=memory&cache=shared` patterns documented elsewhere in this
  plan's cleared table) which cache by test name at the process level and
  are a plausible source of exactly this kind of same-process-replay
  collision, but that has not been confirmed as the specific cause for
  every failure in this run. (An earlier draft of this section attributed
  the collisions to a `sync.Once`-cached template DB; that claim was
  checked and found to be unsupported — `sync.Once` appears in exactly one
  file in the entire backend test tree, `internal/services/mail_service_test.go`,
  which is not one of the packages where these collisions occurred and is
  not in scope for this plan. That specific mechanism is retracted.) CI
  does not run with `-count>1` for the full suite, so this replay-induced
  collision class does not affect normal CI runs regardless of its exact
  cause; it's an artifact of this specific backstop methodology, not a
  product bug, and is explicitly **not** part of this plan's scope.
- None of these unrelated failures should be fixed as part of this PR —
  doing so would violate the `fix:`-scoped, single-root-cause discipline
  this plan is following. If any of them are still a concern, they belong
  in a separate, independently-scoped issue.

Beyond each commit's own validation gate (above), before marking the PR
ready:

1. **Full affected-package runs, repeated, with the race detector**, to
   confirm no regression and no residual flake anywhere in the touched
   packages (not just the specific fixed functions):
   ```bash
   cd backend
   go test ./internal/api/handlers/... -race -count=5
   go test ./internal/services/... -race -count=5
   go test ./cmd/seed/... -race -count=5
   go test ./internal/server/... -race -count=5
   ```
2. **Targeted stress run on the originally-reported flaky test**, at higher
   count to build real confidence (the nightly failure was
   load/scheduling-dependent, so a single passing run proves little):
   ```bash
   cd backend
   go test ./internal/api/handlers/... -run TestSecurityHandler_CreateAndListDecisionAndRulesets -count=20 -race -v
   ```
   Zero failures, and specifically zero `unlinkat`/`ENOTEMPTY` output in
   `go test`'s cleanup-phase log lines, across all 20 runs.
3. **Whole-module regression run** to confirm nothing outside the 4 touched
   packages was affected (it shouldn't be — no shared code changed) and
   that overall coverage has not regressed:
   ```bash
   cd backend
   go test ./... -count=1
   bash scripts/go-test-coverage.sh
   ```
4. **`bash scripts/local-patch-report.sh`** from repo root (CLAUDE.md
   Definition of Done step 2, mandatory regardless of `fix:` scope) —
   confirm `test-results/local-patch-report.md` and `.json` are produced
   and patch coverage does not regress (trivial for this change since every
   line touched is itself test code already exercised by the tests it's
   inside).
5. **`lefthook run pre-commit`** (step 4, mandatory) and
   **`make lint-fast`** (step 5, mandatory, staticcheck-blocking) on the
   full diff.
6. **`cd backend && go build ./...`** (step 8) — confirm the build is
   unaffected (it should be; no non-test file changes).

No Playwright/E2E run is required (Phase 1 note above) and no
CodeQL/Trivy/GORM local run is required (Commit Slicing Strategy's
rollback/contingency note above) — both explicitly waived per CLAUDE.md's
own stated trigger conditions for a `fix:`-scoped, test-code-only,
non-model, non-query change.

# Acceptance Criteria

- [ ] All 41 identified call sites across the 17 files close their
      underlying `*sql.DB` via `t.Cleanup` (or an equally-deterministic
      explicit close before the function returns) before the enclosing
      test function returns.
- [ ] `certificate_handler_test.go`'s 4 duplicated open blocks are
      consolidated into one file-local `openCertHandlerTestDB` helper; its
      5th, structurally-different site (`TestDeleteCertificate_NotificationRateLimit`)
      gets its own minimal inline fix, not folded into the helper.
- [ ] `db_health_handler_test.go`'s `db2` reconnect in
      `TestDBHealthHandler_Check_CorruptedDatabase` (line 227) is closed.
- [ ] `emergency_server_test.go`'s `setupTestDB` helper (`internal/server`
      package) closes its connection via `t.Cleanup`.
- [ ] `crowdsec_wave7_test.go`'s `roDB` in
      `TestCrowdsecWave7_Start_CreateSecurityConfigFailsOnReadOnlyDB`
      (line 43) is closed.
- [ ] `backup_service_wave4_test.go`'s `setupRehydrateDBPair` helper
      closes both connections it opens (`activeDB` and `restoreDB`), even
      though it only returns one of them.
- [ ] `backup_service_encryption_required_test.go`'s `newRemoteStorageTestService`
      and `backup_restore_safe_coverage_test.go`'s `newLiveDBHardeningTestService`
      each close their `*gorm.DB` via `t.Cleanup`, in addition to (not
      instead of) their existing `t.Cleanup(svc.Stop)`.
- [ ] `backup_service_test.go`'s `TestBackupService_RehydrateLiveDatabase_MissingSource`
      closes its `db` (line 1663).
- [ ] `backup_handler_coverage_test.go`'s `setupBackupTestWithDB` closes its
      `gdb` (line 58) via `t.Cleanup`, in addition to (not instead of) its
      existing `t.Cleanup(svc.Stop)`.
- [ ] `TestSecurityHandler_CreateAndListDecisionAndRulesets` passes
      cleanly at `-count=20 -race` with no `unlinkat`/`ENOTEMPTY` output.
- [ ] `go test ./internal/api/handlers/... ./internal/services/... ./cmd/seed/... ./internal/server/... -race -count=5` is 100% green.
- [ ] `go test ./... -count=1` (whole module) is green — no regression
      outside the 4 touched packages.
- [ ] `scripts/go-test-coverage.sh` reports no coverage regression.
- [ ] `bash scripts/local-patch-report.sh` produces both required
      artifacts with acceptable patch coverage.
- [ ] `lefthook run pre-commit` and `make lint-fast` are clean
      (staticcheck-blocking, zero errors).
- [ ] `cd backend && go build ./...` succeeds.
- [ ] No production code, models, migrations, CI/CD config, Dockerfile,
      `.gitignore`, `.dockerignore`, or `.codecov.yml` files are touched —
      confirmed by `git diff --stat` against `development` showing only
      the 17 `_test.go` files listed above.
- [ ] Definition of Done steps 1 (Playwright) and 3 (local security scans)
      are explicitly and correctly skipped per their own stated trigger
      conditions, not silently forgotten — call this out in the PR
      description.
- [ ] `backup_restore_safe_error_paths_test.go` is left untouched (it was
      investigated and cleared, not affected — see Research Findings), and
      the PR description or commit message should not re-flag it, since a
      future reader without this plan's context could otherwise "fix" a
      site that was never broken.

# Handoff

Once this plan is reviewed and approved, delegate implementation to
`backend-dev` (strict TDD is not really applicable here since these are
pre-existing tests being made deterministic, not new behavior — but
`backend-dev` should still run each commit's validation gate red→green:
i.e., first confirm the *unfixed* site can be observed leaking a handle
— e.g. via a quick local `-race -count=20` loop or manual inspection that
`t.Cleanup` is absent — before applying the fix, to avoid "fixing" a site
that was already fine). Route to `supervisor` for review before merge,
referencing this file and commit `35695250` for precedent comparison.
