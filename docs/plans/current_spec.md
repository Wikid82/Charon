# Restore-Reliability Remediation — C1/H1/H2/M4/M5 (Pre-Merge Audit Follow-up, PR #1136)

**Branch:** `feature/backuprestore` (existing PR #1136 — this plan adds commits to that same PR; it does **not** open a new PR)
**Supersedes for planning purposes:** the previous `docs/plans/current_spec.md` (the original Issue #32 Phase 2 remote-storage-providers spec, dated 2026-07-13). That spec's `§3.x` section numbers are still the ones cited throughout the codebase's comments (e.g. "spec §3.5 V1-V6", "spec §3.10 concurrency guard") and are preserved as references below — this document does not renumber or replace them, it only adds a remediation layer on top of already-shipped code.
**Source of truth for scope:** `docs/reports/pre_merge_audit_2026-07-14.md` — this plan implements exactly five findings from it (**C1, H1, H2, M4, M5**) and defers the rest.

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Research Findings](#2-research-findings)
3. [Technical Specifications](#3-technical-specifications)
4. [Implementation Plan](#4-implementation-plan)
5. [Acceptance Criteria](#5-acceptance-criteria)
6. [Commit Slicing Strategy](#6-commit-slicing-strategy)
7. [Ignore-File & Repo Hygiene Review](#7-ignore-file--repo-hygiene-review)
8. [Deferred Findings (Explicitly Out of Scope)](#8-deferred-findings-explicitly-out-of-scope)

---

## 1. Introduction

### 1.1 Overview

PR #1136 (`feature/backuprestore`) already passed a full Definition-of-Done QA pass (`docs/reports/qa_report.md`, 2026-07-13, **READY TO MERGE**). The user deliberately held the merge to soak the branch and commissioned a second, adversarial audit (`docs/reports/pre_merge_audit_2026-07-14.md`, 2026-07-14) specifically hunting for defects that pass today's CI gates but would only surface in production — especially during a real restore under stress. That audit found one **Critical**, two **High**, and identified a "Restore Reliability" bucket as the user's top concern.

This plan implements fixes/tests for exactly five of those findings, in priority order:

| ID | Severity | File | One-line description |
|---|---|---|---|
| **C1** | Critical | `backup_restore_safe.go:271-331` (branch at ~292-307) | Double failure (rehydrate fails + pending-file write fails) is reported as `"Backup restored successfully"`, `nil` error |
| **H1** | High | `backup_service.go:1010-1027` (`RehydrateLiveDatabase`) | Per-table DELETE+INSERT swap is not transactional — mid-loop failure leaves the **live** DB half old/half new/half empty |
| **H2** | High | `backup_restore_safe.go:469-475` (`sanityCheckSQLiteFile`) | The actual corruption-rejection branch (`integrity_check != "ok"`) has 0% test coverage |
| **M4** | Medium | `backup_service.go:400-409` (`CleanupOldBackups`) | The `pre_restore`-exclusion filter (protects the crash-recovery safety net from retention pruning) has 0% coverage |
| **M5** | Medium | `backup_service.go:731-748` (`computeEncryptionKeyRequired`) | The positive (`true`) detection branch has never been proven to return `true` |

C1 and H1 are **coupled by the audit's own analysis**: H1's non-transactional swap is *why* C1's rehydrate-failure path exists in the first place, and fixing C1's reporting alone (without H1) would only make the failure "loud" for a half-applied live database that can still occur. Per the task brief, this plan treats them as one tightly-sequenced unit (§6, Commits 4+5) — they must never land independently of each other.

### 1.2 Objectives

- **O1 (C1):** `RestoreBackupSafe` must never return `(result, nil)` with a "success" message when the restore did not actually complete. The double-failure branch must return a real, non-nil `error`, mapped by the handler to a 5xx, surfaced honestly to the operator.
- **O2 (H1):** `RehydrateLiveDatabase`'s per-table swap must be atomic. A mid-loop failure must roll the live database back to its exact pre-rehydrate state — never a mixed old/new/empty state — regardless of how the failure is subsequently reported.
- **O3 (H2):** Prove, with an actually-corrupted SQLite file (not just "wrong tables" or "not a zip"), that `sanityCheckSQLiteFile`'s `PRAGMA integrity_check` rejection branch works as designed, before any live mutation.
- **O4 (M4):** Prove `CleanupOldBackups` never deletes a `pre_restore`-type backup, regardless of age or retention count, when `s.db` is wired.
- **O5 (M5):** Prove `computeEncryptionKeyRequired` returns `true` when encrypted-secret-bearing rows exist, and that this propagates end-to-end into `BackupManifest.EncryptionKeyRequired` and `ValidateBackup`'s response.

### 1.3 Non-Goals

- No fixes for M1 (Google Drive SSRF on `Location` header), M2 (OAuth token-refresh context/timeout), M3 (error-body leakage), H3 (OAuth token-refresh concurrency), or the L1-L4/lint/DRY findings — see §8.
- No new database migrations, models, or GORM schema changes — this is a bug-fix/test-coverage plan against existing code.
- No new REST endpoints. One existing endpoint's error-response shape gains one new `error_code` value (§3.3).
- No change to the V1-V6 validation pipeline's overall structure, S1 pre-restore-backup mechanics, A1 apply-and-rollback (F2) mechanics, or R1 Caddy-reload mechanics — only the specific branches named above.

---

## 2. Research Findings

### 2.1 Current code, re-verified against HEAD (`feature/backuprestore`, 2026-07-14)

All line numbers below were re-confirmed by direct read of the files at HEAD during this planning pass (not copied from the audit verbatim) — they match the audit almost exactly (drift was ≤1 line in all five cases).

**C1 — `backend/internal/services/backup_restore_safe.go`, `RestoreBackupSafe` (function spans 221-332; the untested double-failure branch is lines 292-307):**

```go
result.LiveRehydrateApplied = rehydrated
result.RestartRequired = !rehydrated

if !rehydrated {
    // F3: persist a durable pending-restore file for the next boot, ...
    if pendingErr := s.writePendingRestoreFile(validated.restoreDBPath); pendingErr != nil {
        logger.Log().WithError(pendingErr).Error("failed to persist pending-restore file")
        // <-- falls through. result.DatabaseSwapPending stays false.
    } else {
        result.DatabaseSwapPending = true
        ...
    }
}

// R1: reload Caddy ... (runs regardless of A2's outcome)
...

// R2.
if result.DatabaseSwapPending {
    result.Message = "Backup restored; the database will finish restoring on the next process restart"
} else {
    result.Message = "Backup restored successfully"   // <-- reached even when nothing was restored
}
return result, nil   // <-- nil error, always
```

Confirmed via `go test -run . -coverprofile` that the `pendingErr != nil` branch (line ~296, the `logger.Log()...Error(...)` call) has **0** executions across the whole suite. The closest existing test, `TestRestoreBackupSafe_RehydrateFails_NonTransient_WritesPendingFileAndWarns` (`backup_restore_safe_error_paths_test.go:72-89`), deliberately exercises the *other* branch — rehydrate fails but the pending-file write **succeeds** — and asserts `DatabaseSwapPending: true`. Nobody has forced the pending-file write to *also* fail.

**H1 — `backend/internal/services/backup_service.go`, `RehydrateLiveDatabase` (function spans 913-1060; the untransactional loop is lines 1010-1027, `sqlite_sequence` handling immediately follows at 1029-1041):**

```go
for _, tableName := range currentTables {
    quotedTable, err := quoteSQLiteIdentifier(tableName)
    if err != nil {
        return fmt.Errorf("quote table identifier: %w", err)
    }
    if err := db.Exec("DELETE FROM " + quotedTable).Error; err != nil {
        return fmt.Errorf("clear table %s: %w", tableName, err)
    }
    if _, exists := restoredTableSet[tableName]; !exists {
        continue
    }
    if err := db.Exec("INSERT INTO " + quotedTable + " SELECT * FROM restore_src." + quotedTable).Error; err != nil {
        return fmt.Errorf("copy table %s: %w", tableName, err)
    }
}
// ...sqlite_sequence handling, same non-transactional shape...
```

Every `db.Exec` here is a separate autocommit statement (no `BEGIN`/`COMMIT` anywhere in this function). Confirmed: `internal/database/database.go:144` pins `sqlDB.SetMaxOpenConns(1)` for the production database handle (`"SQLite only allows one writer at a time"`), which is the fact that makes the transactional fix in §3.4.2 safe to reason about (see risk note there) — but it does **not** make the current *non*-transactional loop safe, since a single connection running four autocommit statements in sequence still commits each one individually.

**H2 — `backend/internal/services/backup_restore_safe.go`, `sanityCheckSQLiteFile` (function spans 460-490; untested branch is 473-475):**

```go
var integrity string
if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
    return fmt.Errorf("run integrity check: %w", err)
}
if !strings.EqualFold(strings.TrimSpace(integrity), "ok") {
    return fmt.Errorf("database integrity check failed: %s", integrity)   // <-- 0 coverage
}
```

Existing tests (`backup_restore_safe_coverage_test.go`) exercise: not-a-zip, newer-format-version, missing-DB-entry, and "opens fine but missing users/proxy_hosts tables" (a **structurally valid but non-Charon** SQLite file — `PRAGMA integrity_check` reports `"ok"` for that file, so it never reaches line 473). No test has ever produced a SQLite file where `integrity_check` itself reports a problem.

**M4 — `backend/internal/services/backup_service.go`, `CleanupOldBackups` (function spans 384-429; untested block is 400-409):**

```go
prunable := backups
if s.db != nil {
    prunable = make([]BackupFile, 0, len(backups))
    for _, b := range backups {
        if s.isPreRestoreBackup(b.Filename) {
            continue
        }
        prunable = append(prunable, b)
    }
}
```

Confirmed: every existing `CleanupOldBackups` test (`backup_service_test.go:262`, `TestCleanupOldBackups_PartialFailure`, `TestCleanupOldBackups_ListBackupsError`) constructs `&BackupService{DataDir: ..., BackupDir: ...}` directly, **never setting the `db` field** — so `s.db == nil` in 100% of existing test runs, meaning the `if s.db != nil` block (the actual safety filter) is never entered. `isPreRestoreBackup` (435-444) reads `models.BackupRecord.Type` via `s.db.Where("filename = ?", filename).First(&record)`.

**M5 — `backend/internal/services/backup_service.go`, `computeEncryptionKeyRequired` (function spans 731-748; untested branch is 743-745):**

```go
for _, table := range []string{"dns_provider_credentials", "tunnel_configs", "remote_storage_targets"} {
    var count int64
    if err := s.db.Table(table).Count(&count).Error; err != nil {
        continue
    }
    if count > 0 {
        return true   // <-- 0 coverage
    }
}
return false
```

This function is called exactly once in production code, at `backup_service.go:594`, inside `createBackupLocked`, to populate `BackupManifest.EncryptionKeyRequired` **at backup-creation time** — it is *not* called during `ValidateBackup`/`RestoreBackupSafe`. The manifest value baked in at creation time is what `ValidateBackup` reads back at `backup_restore_safe.go:202` (`result.EncryptionKeyRequired = validated.manifest.EncryptionKeyRequired`). This means the "end-to-end" test for M5 must: (1) seed `s.db` with a row in one of the three tables, (2) call `CreateBackupWithOptions` so the manifest is written with `EncryptionKeyRequired: true`, then (3) call `ValidateBackup` on that archive and assert the field survives the round trip.

### 2.2 Test infrastructure already available (reuse, don't reinvent)

| Helper | File | What it gives us |
|---|---|---|
| `newHardeningTestService(t)` | `backup_service_v2_hardening_test.go:50` | `BackupService` with `s.db == nil`, a Charon-like on-disk DB (`createCharonLikeTestDB`) |
| `createCharonLikeTestDB(t, dbPath, paddingBytes)` | `backup_service_v2_hardening_test.go:27` | Builds `users`/`proxy_hosts` tables directly via `database/sql` |
| `newLiveDBHardeningTestService(t)` | `backup_restore_safe_coverage_test.go:280` | `BackupService` wired to a **real**, WAL-mode `*gorm.DB` (`s.db != nil`), used for the `RestoreBackupSafe` live-rehydrate branch |
| `newLiveDBRestoreErrorTestService(t)` | `backup_restore_safe_error_paths_test.go:35` | Same as above, but returns `(*BackupService, *gorm.DB)` so the test can close the underlying `*sql.DB` to force deterministic rehydrate failure |
| `fakeConfigurableCaddyReloader` | `backup_restore_safe_coverage_test.go:264` | Toggle `ApplyConfig`'s error return |
| `tamperZipEntry(t, zipPath, entryName)` | `backup_service_v2_hardening_test.go:96` | Flips a byte mid-entry inside an existing zip archive |

All new tests in this plan reuse these helpers rather than inventing parallel ones, per the DRY guideline in `CLAUDE.md`.

### 2.3 Empirically verified: how to actually corrupt a SQLite file for H2

Bit-flipping a valid SQLite file does **not** reliably fail `PRAGMA integrity_check` — SQLite's integrity check validates B-tree *structure* (page linkage, cell offsets, index/table row correspondence), not row-content checksums, so a naive byte flip in row payload data is frequently silently tolerated (confirmed empirically below). Truncating the file causes `sql.Open`'s subsequent query to fail at the driver level (`"database disk image is malformed"` returned as a Go `error` from `Scan`, not as a result string) — that exercises the *other* branch (line 470-472, `"run integrity check: %w"`), not the one this plan targets.

This was verified directly (not assumed) using a throwaway probe against `mattn/go-sqlite3` (this repo's actual driver) during planning: for a 3-table/1-index Charon-like DB, flipping specific bytes inside the **index** B-tree page (not page 1, not deep truncation) reliably produces a `Scan`-succeeding, non-`"ok"` **result string** such as `"row 3 missing from index idx_users_email"` — exactly the branch at line 473-475. The reliable offsets were page-relative (inside the index page, at byte offsets divisible by 8 within a narrow range), **not** absolute-file-position-portable across SQLite versions/architectures. §3.4.3 specifies a self-verifying test helper (`corruptSQLiteFileForIntegrityCheck`) that probes for a working offset at test-run time rather than hardcoding one, so the test is not fragile against a different `libsqlite3`/`go-sqlite3` version in CI vs. dev.

### 2.4 Frontend research

- `frontend/src/components/backups/RestoreDialog.tsx` — `handleRestore`'s `onSuccess` branches only on `result.restart_required` (line 51); `onError` (line 57) already does the generic-failure thing: `toast.error(error.message)`, dialog stays open (no `onClose()` call). This is proven by the existing test `'keeps the dialog open and does not call onClose when restore fails'` (`RestoreDialog.test.tsx:148-161`), which already simulates a generic `onError(new Error('wrong passphrase'))` and asserts the dialog stays open.
- **Key finding:** once the backend fix (C1) makes `RestoreBackupSafe` return a real `error` for the double-failure case, the request becomes a non-2xx HTTP response, axios's `client.ts:42-53` interceptor rejects the promise with `error.message` set from `response.data.error`, and TanStack Query's `useRestoreBackup` mutation (`useBackups.ts:51-60`) already routes that into the *existing* `onError` callback. **No new branching logic is needed in `RestoreDialog.tsx`** — the dialog's current architecture already has a correct, tested "real failure" path; it just wasn't reachable for this specific double-failure case because the backend never actually returned an error.
- **Found dead i18n content:** `frontend/src/locales/en/translation.json:961` — `"restoreFailed": "Failed to restore backup: {{error}}"` — is defined but **never referenced** by any `.tsx`/`.ts` file (confirmed via repo-wide grep). Sibling flows in the same file (`Backups.tsx:73`: `toast.error(t('backups.createFailed', { error: error.message }))`; `Backups.tsx:89`: same pattern for `deleteFailed`) **do** use their equivalent keys. `RestoreDialog.tsx:57` is the outlier, calling bare `toast.error(error.message)` instead of `toast.error(t('backups.restoreFailed', { error: error.message }))`. Per `CLAUDE.md`'s CLEAN principle ("remove... unused... ") / DRY guideline ("consolidate duplicate patterns... after the second occurrence" — this is the third occurrence of the same pattern missing it), this plan activates the orphaned key rather than leaving it dead, bringing `RestoreDialog.tsx` in line with its sibling `Backups.tsx` flows. This is the concrete "genuine-failure state" UI change requested by the task brief — not a new visual state, but making the existing generic-failure toast consistent, translatable, and now reachable for this specific scenario.
- `frontend/src/hooks/useBackups.ts:51-60` (`useRestoreBackup`) — no change needed. It's a thin TanStack Query wrapper; `mutationFn` already propagates any thrown/rejected error to the caller's `onError`.
- `frontend/src/api/backups.ts:63-72` (`RestoreResult` interface) — **no new field needed**. See §3.2 for the reasoning: the double-failure case now returns `(nil, error)` from the Go layer, which the handler serializes as `gin.H{"error":..., "error_code":...}`, never as a `RestoreResult` body — so the TS interface describing the *success* body doesn't need to change to describe a failure.

---

## 3. Technical Specifications

### 3.1 EARS Requirements

| ID | Requirement |
|---|---|
| R1 | When `RehydrateLiveDatabase` fails **and** `writePendingRestoreFile` also fails, the system **shall** return a non-nil error from `RestoreBackupSafe` and **shall not** report `Message: "Backup restored successfully"`. |
| R2 | While `RehydrateLiveDatabase`'s per-table swap is executing, if any single table's `DELETE` or `INSERT` fails, the system **shall** leave every table's live data unchanged from its pre-rehydrate state (full rollback), not a mix of old/new/empty. |
| R3 | When a backup archive's extracted database fails `PRAGMA integrity_check` (reports a non-`"ok"` string), the system **shall** reject the restore/validate call with an error containing `"database integrity check failed"`, before any live-database mutation occurs. |
| R4 | When `CleanupOldBackups` is invoked with `s.db` non-nil and a mix of `pre_restore` and non-`pre_restore` `BackupRecord`s exceeding the retention count, the system **shall** never delete a `pre_restore`-type backup, regardless of its age or position in the retention ordering. |
| R5 | When at least one row exists in `dns_provider_credentials`, `tunnel_configs`, or `remote_storage_targets`, `computeEncryptionKeyRequired` **shall** return `true`, and this **shall** propagate into `BackupManifest.EncryptionKeyRequired` and `ValidateBackup`'s `EncryptionKeyRequired` response field. |
| R6 | When the frontend's restore mutation fails for any reason (including the new R1 case), the system **shall** show a translated, actionable error toast and **shall not** close the restore dialog or imply success. |

### 3.2 API Contracts

**No new endpoints.** One existing endpoint's error-response shape gains one new `error_code`.

`POST /api/v1/backups/:filename/restore` (admin) — `backend/internal/api/handlers/backup_handler.go:238` (`BackupHandler.Restore`)

- **Success (200):** `RestoreResult` JSON body — **unchanged** (`backend/internal/services/backup_restore_safe.go:25-38`, mirrored in `frontend/src/api/backups.ts:64-72`). No new fields.
- **Failure — NEW case (500):**
  ```json
  {
    "error": "restore could not be completed and no recovery mechanism succeeded: live database rehydrate failed (...) and the durable pending-restore fallback also failed (...); a pre-restore safety backup \"backup_2026-07-14_12-00-00.zip\" was created before this attempt and can be restored manually",
    "error_code": "backup_restore_unrecoverable"
  }
  ```
  Mapped in `respondRestoreError` (`backup_handler.go:264-282`) via a new `errors.Is(err, services.ErrRestoreUnrecoverable)` case, inserted before the `default:` branch. Status **500** (an internal, non-client-actionable failure — consistent with `default:`'s existing 500, but now with an explicit `error_code` so the frontend/operators/log-scrapers can key off it instead of string-matching `err.Error()`).
- All other existing failure cases (409 `ErrBackupInProgress`, 400 `ErrPassphraseRequired`/`ErrPassphraseInvalid`/`ErrNewerBackupFormat`/`ErrBackupValidationFailed`, 404 not-found) are **unchanged**.

**Design decision — no new `RestoreResult` field:** The task brief asks whether the result struct needs a field distinguishing "pending restart" from "failed, unrecoverable." It does not, by design: every other early-failure path in `RestoreBackupSafe` (F1 validation failure, S1 pre-restore-backup failure, F2 apply-failure rollback) already returns `(nil, err)` — never `(partialResult, err)`. The fix in §3.4.1 preserves that invariant (`err != nil ⇔ result == nil`) for the new branch, which is what `BackupHandler.Restore` (`backup_handler.go:248-257`) already assumes: it only ever serializes `result` on the `err == nil` path. Adding fields to `RestoreResult` for a case that's never serialized would be dead API surface.

### 3.3 New Sentinel Error

`backend/internal/services/backup_service.go`, in the existing `var (...)` sentinel-error block (currently lines 35-58, alongside `ErrBackupInProgress`, `ErrPassphraseRequired`, etc.):

```go
// ErrRestoreUnrecoverable is returned by RestoreBackupSafe when the live
// rehydrate could not be applied (A2) AND the durable pending-restore
// fallback (F3) also failed to persist — spec §3.5's guarantee that a
// restore either completes live or is durably queued for next-boot is
// broken, so this must never be reported as success (audit finding C1).
// The pre-restore safety backup (S1) still exists on disk and is named in
// the wrapped error for manual recovery.
ErrRestoreUnrecoverable = errors.New("restore could not be completed and no recovery mechanism succeeded")
```

### 3.4 Component Design

#### 3.4.1 C1 fix — `RestoreBackupSafe` (`backup_restore_safe.go:271-332`)

Replace the A2/F3 block. Full before/after context (illustrative — backend-dev implements the exact diff):

```go
// A2: live rehydrate ... (retry loop unchanged, lines 271-284)
rehydrated := false
var rehydrateErr error
if s.db != nil {
    for attempt := 0; attempt < 5; attempt++ {
        rehydrateErr = s.RehydrateLiveDatabase(s.db)
        if rehydrateErr == nil {
            rehydrated = true
            break
        }
        if !isSQLiteTransientRestoreError(rehydrateErr) || attempt == 4 {
            break
        }
        time.Sleep(time.Duration(attempt+1) * 150 * time.Millisecond)
    }
    if !rehydrated && rehydrateErr != nil {
        logger.Log().WithError(rehydrateErr).Warn("Backup restored but live database rehydrate failed")
    }
}
result.LiveRehydrateApplied = rehydrated
result.RestartRequired = !rehydrated

// NEW: tracks the C1 double-failure condition without discarding the
// existing single-failure (F3-succeeds) behavior.
var unrecoverableErr error
if !rehydrated {
    if pendingErr := s.writePendingRestoreFile(validated.restoreDBPath); pendingErr != nil {
        logger.Log().WithError(pendingErr).
            WithField("pre_restore_backup", util.SanitizeForLog(preRestoreRecord.Filename)).
            Error("restore could not be completed: live rehydrate failed and the pending-restore fallback also failed")
        unrecoverableErr = fmt.Errorf(
            "%w: live database rehydrate failed (%v) and the durable pending-restore fallback also failed (%v); a pre-restore safety backup %q was created before this attempt and can be restored manually",
            ErrRestoreUnrecoverable, rehydrateErr, pendingErr, preRestoreRecord.Filename,
        )
    } else {
        result.DatabaseSwapPending = true
        if s.db != nil {
            if updErr := s.db.Model(&models.BackupRecord{}).
                Where("filename = ?", validated.cleanName).
                Update("status", "restore_pending").Error; updErr != nil {
                logger.Log().WithError(updErr).Warn("failed to mark backup record as restore_pending")
            }
        }
    }
}

// R1: reload Caddy — UNCHANGED, still runs regardless of A2/F3's outcome
// (A1 already wrote the new Caddy/CrowdSec files to DataDir by this point,
// independent of whether the DB rehydrate/pending-file path succeeded).
if s.caddyReloader != nil {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    if reloadErr := s.caddyReloader.ApplyConfig(ctx); reloadErr != nil {
        logger.Log().WithError(reloadErr).Warn("failed to reload caddy config after restore")
    } else {
        result.CaddyReloaded = true
    }
    cancel()
}

// NEW: the C1 fix itself — never return (result, nil) for this condition.
if unrecoverableErr != nil {
    return nil, unrecoverableErr
}

// R2 — unchanged.
if result.DatabaseSwapPending {
    result.Message = "Backup restored; the database will finish restoring on the next process restart"
} else {
    result.Message = "Backup restored successfully"
}
return result, nil
```

Design notes:
- R1 (Caddy reload) deliberately still runs even in the unrecoverable case — changing that ordering is out of scope (not one of the 5 findings) and would contradict the existing, deliberate "runs regardless of A2's outcome" comment/rationale that already applies identically to the "pending-file-succeeds" partial-failure case.
- **Correction (Supervisor review, 2026-07-14):** in the *current* code, `rehydrateErr` is declared *inside* the `if s.db != nil { ... }` block, so its scope is limited to that block. The fix requires hoisting the `var rehydrateErr error` declaration to *before* the `if s.db != nil` block (as shown in the code sample above) so it remains in scope for the `unrecoverableErr` construction afterward — this is a real, necessary scope change, not a no-op. Backend-dev must not skip this hoist.

#### 3.4.2 H1 fix — `RehydrateLiveDatabase` (`backup_service.go:913-1060`)

Wrap the per-table loop (1010-1027) and the `sqlite_sequence` handling (1029-1041) in a single `db.Transaction(...)`. `ATTACH DATABASE`/`DETACH DATABASE` **must stay outside** the transaction — SQLite disallows `ATTACH`/`DETACH` inside an open transaction, which is why the audit's own suggested fix scopes the transaction to "the loop... and the sqlite_sequence handling," not the whole function.

```go
func (s *BackupService) RehydrateLiveDatabase(db *gorm.DB) error {
    if db == nil {
        return fmt.Errorf("database handle is required")
    }

    // H1 defensive addition: ATTACH DATABASE is connection-scoped in
    // SQLite, and the transaction below must execute on that SAME
    // connection or restore_src.<table> becomes invisible ("no such
    // table"). Production already pins this via
    // internal/database/database.go:144 (sqlDB.SetMaxOpenConns(1)) on the
    // handle passed in here, but enforce it locally too so this function
    // is correct independent of how its caller wired the *gorm.DB —
    // idempotent/cheap if already 1.
    if sqlDB, sqlErr := db.DB(); sqlErr == nil {
        sqlDB.SetMaxOpenConns(1)
    }

    // ...snapshot/copy-to-temp-file logic: UNCHANGED (lines 918-970)...
    // ...PRAGMA foreign_keys=OFF, ATTACH DATABASE, defer DETACH: UNCHANGED (972-994)...
    // ...list currentTables / restoredTableSet: UNCHANGED (996-1008)...

    // H1 fix: the swap must be atomic. Previously each DELETE/INSERT ran
    // as its own autocommit statement; a mid-loop failure (constraint
    // violation, disk-full, interrupted connection) left tables 1..N-1
    // already on the NEW data, table N empty, and the rest still on OLD
    // data — a mixed state visible to every request against the live,
    // currently-serving *gorm.DB until the process restarts.
    if txErr := db.Transaction(func(tx *gorm.DB) error {
        for _, tableName := range currentTables {
            quotedTable, err := quoteSQLiteIdentifier(tableName)
            if err != nil {
                return fmt.Errorf("quote table identifier: %w", err)
            }
            if err := tx.Exec("DELETE FROM " + quotedTable).Error; err != nil {
                return fmt.Errorf("clear table %s: %w", tableName, err)
            }
            if _, exists := restoredTableSet[tableName]; !exists {
                continue
            }
            if err := tx.Exec("INSERT INTO " + quotedTable + " SELECT * FROM restore_src." + quotedTable).Error; err != nil {
                return fmt.Errorf("copy table %s: %w", tableName, err)
            }
        }

        hasSQLiteSequence := false
        if err := tx.Raw(`SELECT COUNT(*) > 0 FROM restore_src.sqlite_master WHERE type='table' AND name='sqlite_sequence'`).Scan(&hasSQLiteSequence).Error; err != nil {
            return fmt.Errorf("check sqlite_sequence presence: %w", err)
        }
        if hasSQLiteSequence {
            if err := tx.Exec("DELETE FROM sqlite_sequence").Error; err != nil {
                return fmt.Errorf("clear sqlite_sequence: %w", err)
            }
            if err := tx.Exec("INSERT INTO sqlite_sequence SELECT * FROM restore_src.sqlite_sequence").Error; err != nil {
                return fmt.Errorf("copy sqlite_sequence: %w", err)
            }
        }
        return nil
    }); txErr != nil {
        return txErr // rolled back atomically; live db is byte-for-byte its pre-rehydrate state
    }

    // ...DETACH DATABASE / PRAGMA wal_checkpoint(TRUNCATE): UNCHANGED (1043-1057)...
    return nil
}
```

Risk notes (both to be called out in the PR description, not fixed further — accepted tradeoffs):
- **Connection-pool assumption:** `db.Transaction()` calls the pool's `Begin()`, which is not guaranteed by `database/sql` to reuse the exact connection ATTACH ran on *unless* the pool has exactly one connection. Production is safe (`MaxOpenConns(1)`, `internal/database/database.go:144`); the defensive `SetMaxOpenConns(1)` call above closes this gap for any other caller (including tests) rather than relying on incidental pool behavior.
- **Shared-state side effect (Supervisor review, 2026-07-14):** `sqlDB.SetMaxOpenConns(1)` mutates the pool configuration of the `*sql.DB` underlying whatever `*gorm.DB` handle is passed in, and this function does **not** restore the caller's prior setting afterward. This is harmless in production (already pinned to 1) but means any future test or caller that intentionally uses a larger pool size on the same handle for unrelated reasons will find it silently clamped to 1 after calling `RehydrateLiveDatabase`. Acceptable for this fix; flag in the PR description so it isn't rediscovered as a surprise later.
- **Deferred vs. immediate BEGIN:** GORM's `Transaction()` issues a plain `BEGIN` (deferred), not `BEGIN IMMEDIATE`. This is fine here specifically because `MaxOpenConns(1)` means there is no other writer to race against for the read→write lock upgrade.
- **Lock hold duration:** the whole per-table swap now holds one open transaction instead of committing incrementally. For a large database this modestly increases how long the (already-exclusive, single-writer) SQLite handle is mid-swap. Accepted: this is the direct, necessary cost of atomicity, and SQLite already serializes all writers to begin with.

#### 3.4.3 H2 test fixture — corrupting a SQLite file for `PRAGMA integrity_check`

New test helper, `backup_restore_safe_integrity_test.go` (new file):

```go
// corruptSQLiteFileForIntegrityCheck takes a path to an already-valid,
// closed SQLite file (built via createCharonLikeTestDB or equivalent, and
// containing at least one indexed column so a "row missing from index"
// class of structural corruption is reachable) and rewrites it in place
// with a single flipped byte chosen so that:
//   - the file still opens via sql.Open + a subsequent query (no driver-
//     level "database disk image is malformed" error from Scan itself)
//   - PRAGMA integrity_check's Scan succeeds and returns a non-"ok" string
//
// This does NOT hardcode a byte offset: naive bit-flips frequently land in
// unused page space or non-structural payload bytes and are silently
// tolerated by integrity_check (verified empirically during planning —
// see spec §2.3), and the "in database main..." style corruption reports
// SQLite produces are page-layout-dependent (sqlite3/go-sqlite3 version,
// page size, row count). Instead, it probes a bounded range of offsets
// against a scratch copy at test-run time and uses the first one that
// reproduces the desired "successful Scan, non-ok result" condition,
// failing the test outright (not skipping) if none is found within the
// bound — a portable fixture generator, not a fragile magic number.
func corruptSQLiteFileForIntegrityCheck(t *testing.T, path string) {
    t.Helper()
    orig, err := os.ReadFile(path)
    require.NoError(t, err)

    const pageSize = 4096
    for offset := pageSize; offset < len(orig); offset += 8 {
        candidate := make([]byte, len(orig))
        copy(candidate, orig)
        candidate[offset] ^= 0xFF

        probePath := path + ".probe"
        require.NoError(t, os.WriteFile(probePath, candidate, 0o600))

        db, openErr := sql.Open("sqlite3", probePath)
        if openErr != nil {
            _ = os.Remove(probePath)
            continue
        }
        var result string
        scanErr := db.QueryRow("PRAGMA integrity_check").Scan(&result)
        _ = db.Close()
        _ = os.Remove(probePath)

        if scanErr == nil && !strings.EqualFold(strings.TrimSpace(result), "ok") {
            require.NoError(t, os.WriteFile(path, candidate, 0o600))
            return
        }
    }
    t.Fatal("could not find a byte offset that reproduces a non-fatal PRAGMA integrity_check corruption result")
}
```

Confirmed during planning (throwaway probe against this repo's actual `mattn/go-sqlite3` dependency, a 3-table + 1-index Charon-like DB): offsets inside the index B-tree page reliably reproduce results like `"row 3 missing from index idx_users_email"` with `scanErr == nil`. The bounded linear probe above finds this class of offset deterministically without hardcoding it.

Two tests consume this helper:
1. **Direct unit test** — `sanityCheckSQLiteFile(corruptPath)` returns an error containing `"database integrity check failed"`.
2. **End-to-end test** — package the corrupted DB into a v2 archive (same `addToZipTracked` + manifest pattern as `TestValidateBackup_SanityCheckFailure_MissingCharonTables`), then assert `svc.ValidateBackup(...)` **and** `svc.RestoreBackupSafe(...)` both reject with `ErrBackupValidationFailed`, and — mirroring `TestRestoreBackupSafe_TamperedChecksum_Rejected`'s assertion style — that `RestoreBackupSafe` creates **no** pre-restore safety backup (proving V6 rejected it before S1/A1 ever ran).

#### 3.4.4 M4 test — `CleanupOldBackups`'s `pre_restore` exclusion

New test in `backup_service_test.go` (or a new `backup_service_cleanup_db_test.go` if preferred for file-size hygiene): build a `BackupService` with a real, migrated `*gorm.DB` (`models.BackupRecord{}` auto-migrated) wired via `NewBackupService(cfg, db, nil)`, matching the pattern already used in `backup_restore_safe_error_paths_test.go:35-59`. Seed:
- 2 `BackupRecord`s with `Type: "pre_restore"`, artificially old timestamps, filenames matching real on-disk files.
- 5 `BackupRecord`s with `Type: "manual"`, filenames matching real on-disk files, retention count set below 5 (e.g. `keep=2`) so several are expected to be pruned.

Call `svc.CleanupOldBackups(2)`. Assert:
- Both `pre_restore` files still exist on disk (`os.Stat` succeeds) after cleanup, regardless of their age or the retention count.
- The reported `deleted` count and remaining `manual` files match "keep 2 of the 5 manual backups, delete 3," i.e. the `pre_restore` entries are excluded from the retention *count* entirely (not merely spared deletion) — matching the existing filter's semantics (`prunable` is built *without* `pre_restore` entries before the keep/delete split is computed).

#### 3.4.5 M5 test — `computeEncryptionKeyRequired`'s positive branch

New test in `backup_service_test.go` or `backup_manifest_test.go`: build a `BackupService` with a real `*gorm.DB` (auto-migrate `models.RemoteStorageTarget{}` alongside the usual `User`/`ProxyHost`/`BackupRecord`), matching `newLiveDBHardeningTestService`'s construction style. Seed one `models.RemoteStorageTarget{Name: "test", Type: "s3"}` row. Then:
1. Direct: `require.True(t, svc.computeEncryptionKeyRequired())`.
2. End-to-end: call `svc.CreateBackupWithOptions(BackupOptions{Type: "manual"})`, then `svc.ValidateBackup(record.Filename, "")`, and assert `result.EncryptionKeyRequired == true` and `result.FormatVersion == 2` (manifest present).
3. Regression guard: repeat with an empty `remote_storage_targets` table (no seed) and assert `false` end-to-end — this is the existing "negative path" the audit says is already covered, kept here only to anchor the positive/negative pair in one readable table-driven test.

### 3.5 Frontend Component Design

`frontend/src/components/backups/RestoreDialog.tsx`, `handleRestore`'s `onError` (currently line 57):

```diff
-        onError: (error: Error) => toast.error(error.message),
+        onError: (error: Error) => toast.error(t('backups.restoreFailed', { error: error.message })),
```

This is the **only** production-code frontend change. `onSuccess` (lines 49-56) is untouched — it never runs for this failure mode once the backend fix lands, since a 500 response rejects the mutation promise before `onSuccess` is reachable.

No changes to:
- `frontend/src/hooks/useBackups.ts` (`useRestoreBackup`) — generic passthrough already correct.
- `frontend/src/api/backups.ts` (`RestoreResult` interface) — no new field (§3.2).
- `frontend/src/locales/en/translation.json` — the `backups.restoreFailed` key (line 961) **already exists**; this plan activates it, it does not add a new key.

New/updated test in `frontend/src/components/backups/__tests__/RestoreDialog.test.tsx`: update the existing `'keeps the dialog open and does not call onClose when restore fails'` test (line 148) to additionally assert the toast text is wrapped by the `restoreFailed` template (e.g. `expect(toast.error).toHaveBeenCalledWith('Failed to restore backup: wrong passphrase')`, mirroring the exact interpolated-string assertion style already used for `certificates.deleteFailed` in `CertificateList.test.tsx:393`).

### 3.6 Data Flow (C1+H1, failure path)

```mermaid
sequenceDiagram
    participant FE as RestoreDialog.tsx
    participant API as POST /restore
    participant RBS as RestoreBackupSafe
    participant RLD as RehydrateLiveDatabase (H1: transactional)
    participant PRF as writePendingRestoreFile

    FE->>API: restore(filename, passphrase)
    API->>RBS: RestoreBackupSafe(filename, passphrase)
    RBS->>RBS: V1-V6 validate, S1 pre-restore backup, A1 apply
    RBS->>RLD: RehydrateLiveDatabase(db) [attempt 1..5]
    RLD-->>RBS: error (all retries exhausted or non-transient)
    Note over RLD: H1 fix: transaction rolled back atomically;<br/>live DB == pre-rehydrate state (never mixed)
    RBS->>PRF: writePendingRestoreFile(restoreDBPath)
    PRF-->>RBS: error (disk full / permission / read-only remount)
    Note over RBS: C1 fix: both recovery paths failed
    RBS-->>API: (nil, ErrRestoreUnrecoverable-wrapped error)
    API-->>FE: 500 {error, error_code: "backup_restore_unrecoverable"}
    FE->>FE: onError fires: toast.error(t('backups.restoreFailed', {error}))
    FE->>FE: dialog stays open, onClose() NOT called
```

### 3.7 Error Handling & Edge Cases

| Case | Behavior |
|---|---|
| Rehydrate fails, pending-file write succeeds | **Unchanged** — `DatabaseSwapPending: true`, `restart_required: true`, 200 response (already covered by `TestRestoreBackupSafe_RehydrateFails_NonTransient_WritesPendingFileAndWarns`). |
| Rehydrate fails, pending-file write **also** fails | **NEW (C1):** `(nil, ErrRestoreUnrecoverable-wrapped)`, 500, `error_code: "backup_restore_unrecoverable"`. |
| Rehydrate succeeds | **Unchanged** — `LiveRehydrateApplied: true`, 200, `"Backup restored successfully"`. |
| Mid-loop table swap failure (any table) | **NEW (H1):** whole transaction rolls back; `RehydrateLiveDatabase` returns the same wrapped error strings as before (`"clear table %s"`, `"copy table %s"`, `"quote table identifier"`) — error *messages* are unchanged, only the *durability* of partial work changes. |
| `BackupRecord` status update (`restore_pending`) fails after a successful pending-file write | **Unchanged** — still a swallowed warning (this is an independent, lower-severity UX-only field, not part of C1/H1's scope). |
| Corrupt SQLite file (integrity_check fails) | **Unchanged behavior, NEW test (H2):** rejected pre-mutation with `ErrBackupValidationFailed` wrapping `"database integrity check failed: ..."`. |
| `CleanupOldBackups` with `pre_restore` records present | **Unchanged behavior, NEW test (M4):** those records are excluded from both the prunable set and the retention-count denominator. |
| `remote_storage_targets`/`dns_provider_credentials`/`tunnel_configs` has rows | **Unchanged behavior, NEW test (M5):** `EncryptionKeyRequired` is `true` end-to-end through `ValidateBackup`. |

---

## 4. Implementation Plan

Per `CLAUDE.md`'s guidance, this is a bug-fix/hardening plan, not a greenfield feature — the "E2E specs first" step of the suggested commit sequence is replaced with **TDD red/green per commit** (a failing/uncovered-case test written before or alongside its fix, in the same commit). No new Playwright E2E scenarios are introduced (none of these 5 findings are user-visible new behavior; C1's frontend change is a one-line consistency fix covered by an existing Vitest suite). Existing Playwright backup/restore specs (if any target this flow) are re-run as part of Phase 4 to confirm no regression.

### Phase 1 — Independent test-coverage commits (H2, M4, M5)

No production-code behavior changes. Each commit adds tests proving already-correct-looking code actually does what it claims.

- **H2:** `corruptSQLiteFileForIntegrityCheck` helper + direct `sanityCheckSQLiteFile` test + end-to-end `ValidateBackup`/`RestoreBackupSafe` rejection test.
- **M4:** `CleanupOldBackups` pre_restore-exclusion test with a real wired `s.db`.
- **M5:** `computeEncryptionKeyRequired` positive-branch test + end-to-end `ValidateBackup.EncryptionKeyRequired` test.

### Phase 2 — Backend coupled fix (H1 + C1)

- **H1:** Red test proving a mid-loop table-swap failure leaves the live DB in a mixed state under the *current* (pre-fix) code (this test is written to fail against today's code, proving the bug exists, then flipped to assert the *fixed* rollback behavior once the transaction wrap lands — both steps in the same commit per TDD red/green). Green: wrap the loop in `db.Transaction(...)`.
- **C1:** Red test forcing both rehydrate failure (closed `*sql.DB`, reusing `newLiveDBRestoreErrorTestService`) and pending-file-write failure (pre-create the target path as a directory so `os.OpenFile(..., O_CREATE|O_TRUNC)` fails with "is a directory" — an isolated, single-purpose failure injection that doesn't disturb A1's unrelated DataDir writes). Green: the `unrecoverableErr` return path + new sentinel error + handler `error_code` mapping.
- These two are implemented and reviewed as one unit — see §6 for why.

### Phase 3 — Frontend (C1 UI consistency)

- Activate `backups.restoreFailed` in `RestoreDialog.tsx`'s `onError`; update the Vitest assertion.

### Phase 4 — Integration and Full DoD Validation

- Full backend suite (`go test ./...`), full frontend suite, `scripts/go-test-coverage.sh`, `scripts/frontend-test-coverage.sh`, `scripts/local-patch-report.sh`, `./scripts/scan-gorm-security.sh --check` (triggered: this PR touches `backend/internal/services/*.go` GORM queries), `lefthook run pre-commit`, `make lint-backend` (full config, since the audit specifically flagged that only the fast subset normally runs), Playwright backup/restore specs, `npm run type-check`, both builds.

### Phase 5 — Documentation

- `docs/reports/pre_merge_audit_2026-07-14.md` is **not** rewritten (it's a dated historical audit record) — this plan (`docs/plans/current_spec.md`) and the PR description serve as the remediation record. Confirm `docs/features.md`/`ARCHITECTURE.md` don't need updates — they describe user-facing behavior, which is unchanged except for the new, correctly-rare 500 case.

---

## 5. Acceptance Criteria

- [ ] **C1:** A test exists that forces both `RehydrateLiveDatabase` and `writePendingRestoreFile` to fail in the same `RestoreBackupSafe` call, and asserts: (a) a non-nil error is returned, (b) the error wraps `ErrRestoreUnrecoverable`, (c) `errors.Is` matches, (d) the error message contains the pre-restore backup's filename.
- [ ] **C1 (handler):** A test asserts `respondRestoreError(c, services.ErrRestoreUnrecoverable-wrapped-err)` yields HTTP 500 and `error_code: "backup_restore_unrecoverable"` in the JSON body.
- [ ] **H1:** A test forces an `INSERT` failure on a table after at least one other table has already been fully swapped in the same call, and asserts every table's live data (including the ones processed *before* the failure) is byte-for-byte/row-for-row identical to its pre-rehydrate state — proving atomic rollback, not just "an error was returned."
- [ ] **H1:** All four pre-existing `RehydrateLiveDatabase` tests (`backup_service_rehydrate_test.go`) still pass unchanged after the transaction wrap.
- [ ] **H2:** A test using a genuinely `PRAGMA integrity_check`-failing SQLite file (not merely "wrong tables" or "not a zip") asserts `sanityCheckSQLiteFile` rejects it with an error containing `"database integrity check failed"`.
- [ ] **H2 (end-to-end):** `ValidateBackup` and `RestoreBackupSafe` both reject the same corrupted archive with `ErrBackupValidationFailed`, and `RestoreBackupSafe` creates zero pre-restore safety backups for it (proving rejection happens before S1/A1).
- [ ] **M4:** A test with `s.db` wired, seeding `pre_restore` and non-`pre_restore` `BackupRecord`s exceeding retention, asserts 100% of `pre_restore` files survive `CleanupOldBackups` regardless of age/count.
- [ ] **M5:** A test seeding a `remote_storage_targets` row asserts `computeEncryptionKeyRequired() == true`, and a follow-up asserts `ValidateBackup(...).EncryptionKeyRequired == true` for a backup created while that row existed.
- [ ] **Frontend:** `RestoreDialog.test.tsx`'s failure-path test asserts the toast text is produced via the `backups.restoreFailed` i18n key (not a bare `error.message`).
- [ ] All 5 findings' fixes/tests pass `go test ./...` / `npm test` with zero failures.
- [ ] `scripts/go-test-coverage.sh` and `scripts/frontend-test-coverage.sh` both report ≥ current baseline (no regression; these are pure-addition test/fix commits so overall % should rise slightly).
- [ ] `./scripts/scan-gorm-security.sh --check` reports zero CRITICAL/HIGH findings (triggered: GORM query changes in H1).
- [ ] `lefthook run pre-commit` and `make lint-backend` (full config) both pass with zero new findings in touched files.
- [ ] `npm run type-check` passes.
- [ ] Both `go build ./...` and `npm run build` succeed.
- [ ] Playwright suite (`npx playwright test --project=firefox`) passes; any backup/restore-flow specs specifically re-verified.
- [ ] M1/M2/M3/H3/L1-L4 are **not** touched by this PR's diff (verifiable via `git diff` scoped to the files this plan names) and remain noted as deferred in the PR description.

---

## 6. Commit Slicing Strategy

**Decision:** Single PR (`feature/backuprestore`, PR #1136 — already open, not a new PR), ordered logical commits within it. Per `CLAUDE.md`: "one feature = one PR... slice commits, not PRs."

Independent test-only commits (H2, M4, M5) are sequenced first — they're the lowest-risk, fastest-to-review changes and establish the corrupted-fixture helper pattern before the harder coupled fix lands. The H1+C1 pair is explicitly **not** independently mergeable/cherry-pickable (see note after Commit 5) — each depends on the other landing, and a reviewer must evaluate them together even though they're split into two commits for diff readability (transaction-wrapping vs. error-surfacing are conceptually distinct changes worth reviewing as distinct diffs, but never as distinct *decisions*).

### Commit 1 — `test(backend): prove sanityCheckSQLiteFile rejects a genuinely corrupt SQLite file (H2)`

- **Scope:** New `corruptSQLiteFileForIntegrityCheck` helper (§3.4.3) + direct unit test + end-to-end `ValidateBackup`/`RestoreBackupSafe` rejection test.
- **Files:** New `backend/internal/services/backup_restore_safe_integrity_test.go`.
- **Depends on:** Nothing (independent).
- **Validation gate:** `go test ./backend/internal/services/... -run 'IntegrityCheck|Corrupt' -v`; confirm via `-coverprofile` that `backup_restore_safe.go:473-475` now shows a non-zero execution count.

### Commit 2 — `test(backend): prove CleanupOldBackups never prunes pre_restore backups (M4)`

- **Scope:** New test wiring a real `s.db`, seeding mixed `BackupRecord.Type`s exceeding retention.
- **Files:** `backend/internal/services/backup_service_test.go` (or new `backup_service_cleanup_db_test.go`).
- **Depends on:** Nothing (independent).
- **Validation gate:** `go test ./backend/internal/services/... -run CleanupOldBackups -v`; confirm `backup_service.go:400-409` coverage is non-zero.

### Commit 3 — `test(backend): prove computeEncryptionKeyRequired's positive branch (M5)`

- **Scope:** New test seeding `remote_storage_targets`, direct + end-to-end (`ValidateBackup.EncryptionKeyRequired`) assertions.
- **Files:** `backend/internal/services/backup_service_test.go` or `backup_manifest_test.go`.
- **Depends on:** Nothing (independent).
- **Validation gate:** `go test ./backend/internal/services/... -run EncryptionKeyRequired -v`; confirm `backup_service.go:743-745` coverage is non-zero.

### Commit 4 — `fix(backend): make RehydrateLiveDatabase's per-table swap transactional (H1)`

- **Scope:** Red test proving mid-loop failure leaves a mixed live-DB state today; green fix wrapping the DELETE+INSERT loop + `sqlite_sequence` handling in `db.Transaction(...)`; defensive `SetMaxOpenConns(1)`.
- **Files:** `backend/internal/services/backup_service.go` (lines ~913-1060), new test(s) in `backend/internal/services/backup_service_rehydrate_test.go`.
- **Depends on:** Nothing structurally, but **must land immediately before/with Commit 5** — see coupling note below.
- **Validation gate:** `go test ./backend/internal/services/... -run RehydrateLiveDatabase -v`; all 4 pre-existing rehydrate tests plus the new mid-loop-failure test pass; `./scripts/scan-gorm-security.sh --check` (GORM query change trigger).

### Commit 5 — `fix(backend): surface RestoreBackupSafe's double-failure path as a real error instead of false success (C1)`

- **Scope:** New `ErrRestoreUnrecoverable` sentinel; red test forcing both rehydrate-fails (closed `*sql.DB`) and pending-file-write-fails (target path pre-created as a directory); green fix per §3.4.1; handler `respondRestoreError` new case + handler test.
- **Files:** `backend/internal/services/backup_service.go` (sentinel error block, ~line 35-58), `backend/internal/services/backup_restore_safe.go` (lines ~271-332), `backend/internal/api/handlers/backup_handler.go` (`respondRestoreError`, ~line 264-282), new test(s) in `backend/internal/services/backup_restore_safe_error_paths_test.go` and `backend/internal/api/handlers/backup_handler_test.go`.
- **Depends on: Commit 4.** This is the explicit coupling the task brief requires calling out: C1's test scenario relies on `RehydrateLiveDatabase` failing cleanly (which, post-H1, means "fully rolled back," not "partially applied") — reviewing/merging C1's error-surfacing fix without H1's atomicity fix would satisfy "the failure is now reported loudly" while leaving "the live DB can still end up half-swapped" completely unaddressed, which is precisely the anti-pattern the audit calls out. **Commits 4 and 5 must be reviewed and merged as a pair — never independently.**
- **Validation gate:** `go test ./backend/internal/services/... ./backend/internal/api/handlers/... -run 'RestoreBackupSafe|RestoreError' -v`; `errors.Is(err, services.ErrRestoreUnrecoverable)` assertion passes; handler test asserts 500 + `error_code`.

### Commit 6 — `fix(frontend): route restore-failure toasts through the existing backups.restoreFailed template (C1 UI)`

- **Scope:** One-line change in `RestoreDialog.tsx`'s `onError`; updated Vitest assertion.
- **Files:** `frontend/src/components/backups/RestoreDialog.tsx`, `frontend/src/components/backups/__tests__/RestoreDialog.test.tsx`.
- **Depends on:** Commit 5 (the new failure mode this activates for is only reachable after the backend fix, though the code change itself is backend-agnostic and would be safe to land alone).
- **Validation gate:** `cd frontend && npx vitest run src/components/backups/__tests__/RestoreDialog.test.tsx`; `npm run type-check`.

### Commit 7 — `chore: full DoD validation pass for restore-reliability remediation`

- **Scope:** No production code changes. Run and attach the full Definition-of-Done gate set; fix any incidental fallout (e.g. a lint finding surfaced by `make lint-backend`'s full config, per the audit's own note that only the fast subset normally runs).
- **Files:** Potentially none, or trivial lint-driven touch-ups within the 6 files already touched above.
- **Depends on:** Commits 1-6.
- **Validation gate:** Full Phase 4 list (§4) — `go test ./...`, `scripts/go-test-coverage.sh`, `scripts/frontend-test-coverage.sh`, `scripts/local-patch-report.sh`, `lefthook run pre-commit`, `make lint-backend`, `./scripts/scan-gorm-security.sh --check`, `npx playwright test --project=firefox`, `npm run type-check`, both builds.

### Rollback & Contingency (PR-level)

- Every commit above is additive/localized to 5 already-identified functions across 4 files (+3 test files) — no schema/migration changes, so rollback is a plain `git revert` of the relevant commit range with no data-migration concerns.
- If Commit 4's transactional wrap surfaces an unexpected GORM/SQLite interaction in CI (e.g. a connection-pool assumption that holds in this repo's test harness but not in some other environment), Commit 5 must **not** proceed until resolved — per the explicit coupling, there is no safe "ship C1 alone" fallback; if timeline pressure demands it, the correct contingency is to hold the entire Phase 2 pair rather than split it.
- Commits 1-3 (H2/M4/M5) and Commit 6 have no interdependency on Phase 2 succeeding and can ship even if Phase 2 needs more iteration — consistent with "independent test-coverage commits can be sequenced early or late."

---

## 7. Ignore-File & Repo Hygiene Review

- **`.gitignore`:** Already contains a blanket `*.db` / `charon.db` exclusion (repo root). All H2 fixture files (valid + corrupted SQLite bytes) are generated at test-runtime inside `t.TempDir()` (outside the repo tree entirely) — no static binary fixture is added to the repo, so no new ignore entries are needed.
- **`codecov.yml`:** Already excludes `**/*_test.go` globally (line 42) and there's no fixture-file category needed since nothing binary is committed. No changes needed.
- **`.dockerignore`:** Already excludes `*.db`/`charon.db` broadly; irrelevant regardless since no fixtures are committed to the repo.
- **`Dockerfile`:** No changes — this plan touches only `backend/internal/services/*.go`, `backend/internal/api/handlers/backup_handler.go`, and two frontend files, none of which affect the build/copy steps.
- **Conclusion:** No ignore-file, Codecov config, or Docker changes are required by this plan.

---

## 8. Deferred Findings (Explicitly Out of Scope)

Per the task brief, the following audit findings are **not** addressed by this plan and remain tracked in `docs/reports/pre_merge_audit_2026-07-14.md` for a future pass:

- **M1** — Google Drive resumable-upload `Location` header trusted with no SSRF/host allow-list check (`remotestorage/googledrive.go`).
- **M2** — OAuth token-refresh HTTP calls not bounded by caller context/timeout (`remotestorage/oauthtoken.go`, `dropbox.go`, `googledrive.go`).
- **M3** — Raw third-party API error bodies flow into `gin.H{"error":...}` and persist in `RemoteStorageTarget.LastError` (Test-connection endpoint).
- **H3** — No concurrency control around the OAuth-secrets read-modify-write cycle (`backup_remote_service.go` `SaveToken`/`CompleteOAuth`).
- **L1-L4** and the lint/DRY findings (bucket 2/3) — `WithPermissiveSSRFForTesting` production-shippable test helper, `ValidateHostSSRF`'s zero-addresses branch, defensive `sql.Open`/`os.Open` branches, `deferInLoop` gocritic finding, `gosec` G117 missing annotations, and the three DRY consolidation opportunities (shared JSON-request helper, shared API-error helper, shared OAuth-uploader-construction helper).

This PR's diff must not touch any file whose *only* relevant change would be one of the above (e.g. do not "opportunistically" fix M1's SSRF gap while in `googledrive.go`, since that file is not otherwise touched by this plan) — keeping the diff scoped to exactly the 5 named findings per the task brief's explicit instruction.
