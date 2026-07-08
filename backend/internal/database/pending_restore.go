package database

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// ApplyPendingRestore consumes the durable pending-restore marker written by
// services.BackupService.RestoreBackupSafe's F3 fallback (spec §3.5): when a
// restore could not be completed via the in-process live rehydrate, the
// already-validated replacement database is persisted to
// dbPath+".pending-restore" so a subsequent process start can finish the
// restore before GORM (or anything else) ever opens dbPath. This is the
// previously-missing consumer — v1 shipped a "restart_required" response
// with no code anywhere that acted on a restart, so restarting the
// container did nothing. This function is that consumer.
//
// It must be called before database.Connect(dbPath) on the default startup
// path (backend/cmd/api/main.go) — deliberately NOT from the
// migrate/reset-password CLI subcommands, which are maintenance tools
// rather than the running-server path a `docker restart` takes; wiring the
// swap there would let an unrelated CLI invocation silently mutate data.
//
// Behavior:
//   - No pending file present: no-op, nil error (the overwhelmingly common
//     case on every normal boot).
//   - Pending file present and passes a second, independent
//     PRAGMA integrity_check: stale dbPath/-wal/-shm siblings are removed,
//     the pending file is renamed over dbPath, and the marker is gone. The
//     caller proceeds to open the newly-installed database.
//   - Pending file present but fails integrity_check: it is renamed to
//     ".pending-restore.failed" (kept for forensic inspection, never
//     auto-retried) and dbPath is left completely untouched — boot proceeds
//     on the pre-restore database rather than installing a known-bad one.
func ApplyPendingRestore(dbPath string) error {
	pendingPath := dbPath + ".pending-restore"

	if _, err := os.Stat(pendingPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat pending-restore marker: %w", err)
	}

	if integrityErr := sqliteIntegrityCheck(pendingPath); integrityErr != nil {
		failedPath := pendingPath + ".failed"
		_ = os.Remove(failedPath)
		if renameErr := os.Rename(pendingPath, failedPath); renameErr != nil {
			return fmt.Errorf("pending-restore file failed integrity check (%v) and could not be quarantined: %w", integrityErr, renameErr)
		}
		// The old dbPath is left untouched and remains authoritative — the
		// swap never happened, so we mark the outcome on it directly.
		markPendingRestoreOutcome(dbPath, "restore_failed")
		return fmt.Errorf("pending-restore file failed integrity check, quarantined as %s: %w", failedPath, integrityErr)
	}

	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(dbPath + suffix); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale database file %s: %w", dbPath+suffix, err)
		}
	}

	if err := os.Rename(pendingPath, dbPath); err != nil {
		return fmt.Errorf("swap pending-restore file into place: %w", err)
	}

	// Best-effort: mark the outcome in the newly-installed database. This
	// may be a no-op if the restored database's backup_records table
	// doesn't happen to contain the pending row (it reflects a snapshot
	// taken at backup-creation time) — that's a known, documented
	// limitation, never a reason to fail the boot.
	markPendingRestoreOutcome(dbPath, "restore_completed")

	return nil
}

// sqliteIntegrityCheck runs PRAGMA integrity_check against the sqlite file
// at path and returns an error unless the result is exactly "ok".
func sqliteIntegrityCheck(path string) error {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return fmt.Errorf("open pending-restore database: %w", err)
	}
	defer func() {
		_ = db.Close()
	}()

	var result string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("run integrity check: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(result), "ok") {
		return fmt.Errorf("integrity check reported: %s", result)
	}
	return nil
}

// markPendingRestoreOutcome best-effort updates any backup_records row left
// in status "restore_pending" to reflect the boot-time swap outcome (spec
// §3.5). Every failure mode here (file missing table, locked, whatever) is
// intentionally swallowed: this is a UX nicety for the backup history list,
// never a reason to fail or block startup.
func markPendingRestoreOutcome(dbPath, status string) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return
	}
	defer func() {
		_ = db.Close()
	}()

	_, _ = db.Exec("UPDATE backup_records SET status = ? WHERE status = 'restore_pending'", status)
}
