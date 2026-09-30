package dbmaint

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
)

// scratchOpts shapes a scratch database built by newScratchDB.
type scratchOpts struct {
	// autoVacuum is the mode issued before journal_mode=WAL (0 = leave default).
	autoVacuum int
	// rows inserted (about two 2000-byte rows per 4 KiB page).
	rows int
	// keepEvery keeps one row in N and deletes the rest (0 = delete nothing).
	keepEvery int
}

// newScratchDB builds a WAL database with a single-connection pool, the way
// database.Connect does, and closes it on cleanup.
func newScratchDB(t *testing.T, opts scratchOpts) (db *sql.DB, path string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "scratch.db")
	db = openScratch(t, path, opts.autoVacuum)
	if opts.rows > 0 {
		fillScratch(t, db, opts.rows, opts.keepEvery)
	}
	return db, path
}

// openScratch opens path with auto_vacuum (when non-zero) issued first, then WAL.
func openScratch(t *testing.T, path string, autoVacuum int) *sql.DB {
	t.Helper()
	db, err := sql.Open(sqlite.DriverName, path)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if autoVacuum != 0 {
		mustExec(t, db, fmt.Sprintf("PRAGMA auto_vacuum=%d", autoVacuum))
	}
	mustExec(t, db, "PRAGMA journal_mode=WAL")
	mustExec(t, db, "PRAGMA busy_timeout=5000")
	return db
}

func fillScratch(t *testing.T, db *sql.DB, rows, keepEvery int) {
	t.Helper()
	mustExec(t, db, "CREATE TABLE IF NOT EXISTS t (id INTEGER PRIMARY KEY, pad BLOB)")
	mustExec(t, db, fmt.Sprintf(
		"WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c WHERE x<%d) "+
			"INSERT INTO t(pad) SELECT randomblob(2000) FROM c", rows))
	if keepEvery > 0 {
		mustExec(t, db, fmt.Sprintf("DELETE FROM t WHERE id %% %d <> 0", keepEvery))
	}
	mustCheckpoint(t, db)
}

func mustExec(t *testing.T, q interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, query string) {
	t.Helper()
	_, err := q.ExecContext(context.Background(), query)
	require.NoError(t, err, query)
}

func pragmaInt(t *testing.T, q Querier, pragma string) int64 {
	t.Helper()
	var v int64
	require.NoError(t, q.QueryRowContext(context.Background(), "PRAGMA "+pragma).Scan(&v), pragma)
	return v
}

func mustCheckpoint(t *testing.T, q Querier) (busy int64) {
	t.Helper()
	var logFrames, checkpointed int64
	require.NoError(t, q.QueryRowContext(context.Background(), "PRAGMA wal_checkpoint(TRUNCATE)").
		Scan(&busy, &logFrames, &checkpointed))
	return busy
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Size()
}

func integrityOK(t *testing.T, db *sql.DB) {
	t.Helper()
	var res string
	require.NoError(t, db.QueryRow("PRAGMA integrity_check").Scan(&res))
	require.Equal(t, "ok", res)
}
