package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func createValidSQLiteFile(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`CREATE TABLE backup_records (id INTEGER PRIMARY KEY, filename TEXT, status TEXT)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO backup_records (filename, status) VALUES ('backup_x.zip', 'restore_pending')`)
	require.NoError(t, err)
}

// TestApplyPendingRestore_NoMarker_NoOp proves the overwhelmingly common
// case (no pending restore in flight) is a silent no-op.
func TestApplyPendingRestore_NoMarker_NoOp(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "charon.db")
	require.NoError(t, ApplyPendingRestore(dbPath))
}

// TestApplyPendingRestore_ValidPendingFile_SwapsIntoPlace is required test
// #7 (the "happy path" half): writing a valid pending-restore file and then
// simulating a process restart via ApplyPendingRestore must swap it into
// place at dbPath, removing the marker.
func TestApplyPendingRestore_ValidPendingFile_SwapsIntoPlace(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "charon.db")

	// Simulate the "old" database currently on disk (what a live process
	// would have open).
	createValidSQLiteFile(t, dbPath)
	oldDB, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	var oldStatus string
	require.NoError(t, oldDB.QueryRow(`SELECT status FROM backup_records LIMIT 1`).Scan(&oldStatus))
	require.Equal(t, "restore_pending", oldStatus)
	require.NoError(t, oldDB.Close())

	// Also leave stale -wal/-shm siblings to prove they get cleaned up.
	require.NoError(t, os.WriteFile(dbPath+"-wal", []byte("stale wal"), 0o600))
	require.NoError(t, os.WriteFile(dbPath+"-shm", []byte("stale shm"), 0o600))

	// The already-validated replacement database (spec §3.5 F3): a
	// distinct, valid sqlite file representing the target backup's
	// restored state.
	pendingPath := dbPath + ".pending-restore"
	pendingDB, err := sql.Open("sqlite3", pendingPath)
	require.NoError(t, err)
	_, err = pendingDB.Exec(`CREATE TABLE marker (id INTEGER PRIMARY KEY, value TEXT)`)
	require.NoError(t, err)
	_, err = pendingDB.Exec(`INSERT INTO marker (value) VALUES ('this-is-the-new-database')`)
	require.NoError(t, err)
	require.NoError(t, pendingDB.Close())

	// Simulate the restart: a fresh process boot calling ApplyPendingRestore
	// before database.Connect ever opens dbPath.
	require.NoError(t, ApplyPendingRestore(dbPath))

	require.NoFileExists(t, pendingPath, "the pending marker must be gone after a successful swap")
	require.NoFileExists(t, dbPath+"-wal", "stale -wal sibling must be removed before the swap")
	require.NoFileExists(t, dbPath+"-shm", "stale -shm sibling must be removed before the swap")

	swapped, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer func() { _ = swapped.Close() }()

	var value string
	require.NoError(t, swapped.QueryRow(`SELECT value FROM marker LIMIT 1`).Scan(&value))
	require.Equal(t, "this-is-the-new-database", value, "dbPath must now be the previously-pending database")
}

// TestApplyPendingRestore_CorruptPendingFile_RejectedAndOldDBRetained is
// required test #7 (the "corrupt" half): a pending file that fails a second,
// independent integrity check must be rejected — quarantined, never
// installed — and the prior database must remain completely untouched and
// authoritative.
func TestApplyPendingRestore_CorruptPendingFile_RejectedAndOldDBRetained(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "charon.db")
	createValidSQLiteFile(t, dbPath)

	pendingPath := dbPath + ".pending-restore"
	require.NoError(t, os.WriteFile(pendingPath, []byte("this is not a valid sqlite database file at all"), 0o600))

	err := ApplyPendingRestore(dbPath)
	require.Error(t, err)

	require.NoFileExists(t, pendingPath, "a corrupt pending file must be quarantined, not left in place for a retry")
	require.FileExists(t, pendingPath+".failed", "a corrupt pending file is renamed to .pending-restore.failed for forensic inspection")

	// The old database must be completely untouched.
	oldDB, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer func() { _ = oldDB.Close() }()

	// dbPath itself is never swapped/replaced — but its backup_records
	// status is best-effort updated to restore_failed so the outcome is
	// visible (spec §3.5); this is the one deliberate write ApplyPendingRestore
	// makes to the untouched database on this branch.
	var status string
	require.NoError(t, oldDB.QueryRow(`SELECT status FROM backup_records LIMIT 1`).Scan(&status))
	require.Equal(t, "restore_failed", status, "old dbPath's record must reflect the rejected swap outcome")
}

// TestApplyPendingRestore_CorruptPendingFile_NeverAutoRetried proves a
// second call after quarantine is a no-op (the .failed file is never picked
// back up).
func TestApplyPendingRestore_CorruptPendingFile_NeverAutoRetried(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "charon.db")
	createValidSQLiteFile(t, dbPath)

	pendingPath := dbPath + ".pending-restore"
	require.NoError(t, os.WriteFile(pendingPath, []byte("corrupt"), 0o600))
	require.Error(t, ApplyPendingRestore(dbPath))

	require.NoError(t, ApplyPendingRestore(dbPath), "a second boot must not retry the quarantined .failed file")
	require.FileExists(t, pendingPath+".failed")
}
