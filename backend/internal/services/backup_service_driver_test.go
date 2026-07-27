package services

import (
	"database/sql"
	"path/filepath"
	"testing"

	glebarezsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
)

// createSQLiteTestDBGlebarez creates a minimal SQLite database at dbPath
// using ONLY the glebarez/modernc pure-Go driver (via database/sql,
// sqlite.DriverName) — deliberately never touching gorm.io/driver/sqlite
// (which wraps mattn/go-sqlite3, CGO) so this helper works identically
// whether or not CGO is available, unlike createSQLiteTestDB (used
// elsewhere in this package's tests) which depends on CGO through
// gorm.io/driver/sqlite.
func createSQLiteTestDBGlebarez(t *testing.T, dbPath string) {
	t.Helper()

	db, err := sql.Open(glebarezsqlite.DriverName, dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS healthcheck (id INTEGER PRIMARY KEY, value TEXT)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO healthcheck (value) VALUES (?)", "ok")
	require.NoError(t, err)
}

// TestCreateSQLiteSnapshot_VacuumIntoViaGlebarezDriver proves plan
// ASSUMPTION-003 (docs/plans/current_spec.md §3.8/§6 commit 2) empirically:
// createSQLiteSnapshot's `VACUUM INTO` call works correctly when both the
// source database and the snapshot are opened exclusively through
// github.com/glebarez/sqlite (sqlite.DriverName) — the same pure-Go,
// non-CGO driver already used for the main app connection
// (database.go:54) — rather than github.com/mattn/go-sqlite3 ("sqlite3").
// No existing test exercised this combination before this plan: the
// pre-existing TestSQLiteSnapshotAndCheckpoint uses gorm.io/driver/sqlite
// (also mattn-based, CGO) to create its fixture DB. Regression coverage:
// if a future change swaps createSQLiteSnapshot back to a driver that
// can't read a glebarez-authored file, or VACUUM INTO's glebarez semantics
// ever diverge (e.g. a partial/empty snapshot), this test fails.
func TestCreateSQLiteSnapshot_VacuumIntoViaGlebarezDriver(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "glebarez-source.db")
	createSQLiteTestDBGlebarez(t, dbPath)

	snapshotPath, cleanup, err := createSQLiteSnapshot(dbPath)
	require.NoError(t, err)
	defer cleanup()
	require.FileExists(t, snapshotPath)

	// Read the snapshot back, exclusively via the glebarez driver, and
	// confirm the VACUUM INTO copy is complete and byte-correct at the SQL
	// level (not just "a file exists").
	snapDB, err := sql.Open(glebarezsqlite.DriverName, snapshotPath)
	require.NoError(t, err)
	defer func() { _ = snapDB.Close() }()

	var value string
	require.NoError(t, snapDB.QueryRow("SELECT value FROM healthcheck LIMIT 1").Scan(&value))
	require.Equal(t, "ok", value)

	var integrity string
	require.NoError(t, snapDB.QueryRow("PRAGMA quick_check").Scan(&integrity))
	require.Equal(t, "ok", integrity)
}

// TestCheckpointSQLiteDatabase_ViaGlebarezDriver proves the companion
// checkpointSQLiteDatabase (WAL checkpoint, run during restore/rehydrate,
// spec §3.8) also works correctly against a glebarez-authored database —
// no existing test exercised this combination either.
func TestCheckpointSQLiteDatabase_ViaGlebarezDriver(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "glebarez-checkpoint.db")
	createSQLiteTestDBGlebarez(t, dbPath)

	require.NoError(t, checkpointSQLiteDatabase(dbPath))

	// The database must still be fully readable/intact after the
	// checkpoint.
	db, err := sql.Open(glebarezsqlite.DriverName, dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var value string
	require.NoError(t, db.QueryRow("SELECT value FROM healthcheck LIMIT 1").Scan(&value))
	require.Equal(t, "ok", value)
}
