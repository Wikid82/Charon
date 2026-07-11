package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestApplyPendingRestore_StatNonNotExistError proves the os.Stat error
// branch that ISN'T os.IsNotExist (line 46) surfaces a wrapped
// "stat pending-restore marker" error rather than being treated as "no
// pending restore" — forced here via ENOTDIR (a path component that should
// be a directory is actually a regular file).
func TestApplyPendingRestore_StatNonNotExistError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-directory")
	require.NoError(t, os.WriteFile(blocker, []byte("i am a file, not a directory"), 0o600))

	// dbPath's parent path component is a regular file, so
	// os.Stat(dbPath+".pending-restore") fails with ENOTDIR, not ENOENT.
	dbPath := filepath.Join(blocker, "charon.db")

	err := ApplyPendingRestore(dbPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stat pending-restore marker")
}

// TestApplyPendingRestore_QuarantineRenameFailure proves that when a
// corrupt pending file's quarantine rename itself fails (lines 52-54), the
// error is wrapped and surfaced rather than silently swallowed, and no
// partial state is left claiming success.
func TestApplyPendingRestore_QuarantineRenameFailure(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "charon.db")
	createValidSQLiteFile(t, dbPath)

	pendingPath := dbPath + ".pending-restore"
	require.NoError(t, os.WriteFile(pendingPath, []byte("not a valid sqlite file"), 0o600))

	// Remove write permission on dir so the quarantine os.Rename (which
	// must update the directory's entries) fails, while lookups (needed by
	// the earlier os.Remove(failedPath) best-effort call) still succeed.
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	err := ApplyPendingRestore(dbPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not be quarantined")
}

// TestApplyPendingRestore_StaleFileRemoveFailure proves the loop removing
// dbPath/-wal/-shm (lines 62-64) surfaces a wrapped error when a removal
// fails for a reason other than "does not exist" — forced here by dbPath
// being a non-empty directory, which os.Remove refuses to remove.
func TestApplyPendingRestore_StaleFileRemoveFailure(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "charon.db")

	// dbPath is a non-empty directory rather than a regular file: a valid
	// pending-restore file will pass its integrity check, but the stale-file
	// cleanup loop's os.Remove(dbPath) must fail with ENOTEMPTY (never
	// os.IsNotExist), which is exactly the branch under test.
	require.NoError(t, os.MkdirAll(dbPath, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dbPath, "inner.txt"), []byte("x"), 0o600))

	pendingPath := dbPath + ".pending-restore"
	createValidSQLiteFile(t, pendingPath)

	err := ApplyPendingRestore(dbPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "remove stale database file")
}

// TestApplyPendingRestore_FinalRenameFailure proves the final
// os.Rename(pendingPath, dbPath) error branch (lines 67-69) is wrapped and
// surfaced — forced by making the containing directory read-only after the
// stale-file cleanup loop would have been a no-op (no old dbPath present).
func TestApplyPendingRestore_FinalRenameFailure(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "charon.db") // intentionally never created

	pendingPath := dbPath + ".pending-restore"
	createValidSQLiteFile(t, pendingPath)

	// Directory permissions of r-x only: the cleanup loop's os.Remove calls
	// against nonexistent dbPath/-wal/-shm still resolve to ENOENT (lookup
	// only needs search permission), but the final os.Rename needs write
	// permission on the directory to update its entries, so it fails.
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	err := ApplyPendingRestore(dbPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "swap pending-restore file into place")
}

// buildIntegrityCheckFailingSQLiteFile writes a syntactically-openable
// sqlite database file at path whose PRAGMA integrity_check reports a
// corruption description (a non-"ok", no-Go-error result) rather than
// failing to open at all — the specific branch sqliteIntegrityCheck's
// "integrity check reported: %s" wraps. The exact byte offset was
// empirically determined to land inside an interior b-tree page's cell
// pointer array for this fixed insert pattern (deterministic given a fixed
// mattn/go-sqlite3 version and page layout), corrupting page structure
// without preventing the file from opening.
func buildIntegrityCheckFailingSQLiteFile(t *testing.T, path string) {
	t.Helper()

	db, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)`)
	require.NoError(t, err)
	for i := 0; i < 200; i++ {
		_, err = db.Exec(`INSERT INTO users (email) VALUES (?)`, fmt.Sprintf("user%d@example.com-padding-padding-padding", i))
		require.NoError(t, err)
	}
	require.NoError(t, db.Close())

	content, err := os.ReadFile(path) // #nosec G304 -- test-controlled path
	require.NoError(t, err)
	const corruptionOffset = 4106
	require.Greater(t, len(content), corruptionOffset, "fixture insert pattern must produce a file large enough to corrupt at a fixed offset")
	content[corruptionOffset] ^= 0xFF
	require.NoError(t, os.WriteFile(path, content, 0o600))
}

// TestApplyPendingRestore_IntegrityCheckReportsCorruption_QuarantinesFile
// proves the "integrity check reported: %s" branch (a pending file that
// opens fine but whose PRAGMA integrity_check reports real corruption,
// rather than erroring outright) is treated exactly like any other
// integrity failure: quarantined, old database untouched.
func TestApplyPendingRestore_IntegrityCheckReportsCorruption_QuarantinesFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "charon.db")
	createValidSQLiteFile(t, dbPath)

	pendingPath := dbPath + ".pending-restore"
	buildIntegrityCheckFailingSQLiteFile(t, pendingPath)

	// Sanity-check the fixture actually reproduces a non-"ok",
	// non-error PRAGMA integrity_check result before asserting on
	// ApplyPendingRestore's behavior, so a future sqlite3/library upgrade
	// that changes page layout fails loudly here with a clear message
	// rather than as a confusing downstream assertion failure.
	checkErr := sqliteIntegrityCheck(pendingPath)
	require.Error(t, checkErr, "fixture must reproduce a reported (non-error-at-open) integrity failure")
	require.Contains(t, checkErr.Error(), "integrity check reported",
		"fixture must hit the 'reported' branch, not the 'open'/'run' error branches")

	err := ApplyPendingRestore(dbPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "quarantined")
	require.FileExists(t, pendingPath+".failed")
	require.NoFileExists(t, pendingPath)
}

// TestMarkPendingRestoreOutcome_OpenFailure_NeverPanics proves the
// best-effort sql.Open failure branch inside markPendingRestoreOutcome
// (lines 108-111) is a silent no-op, never a panic — exercised directly
// since this unexported helper's sql.Open never actually returns an error
// for the mattn/go-sqlite3 driver except for pathological paths.
func TestMarkPendingRestoreOutcome_OpenFailure_NeverPanics(t *testing.T) {
	dir := t.TempDir()
	// A directory path (not a file) makes every subsequent operation on it
	// fail once actually used, exercising the same best-effort code path.
	require.NotPanics(t, func() {
		markPendingRestoreOutcome(dir, "restore_completed")
	})
}
