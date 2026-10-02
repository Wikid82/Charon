package dbmaint

// Regression tests that pin the behaviour of the glebarez/go-sqlite driver
// (github.com/glebarez/go-sqlite v1.23.0) that the maintenance design depends
// on (GH #1422, plan section 2.4). If a driver upgrade changes one of them
// the failing test names the assumption that no longer holds.

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDriver_AutoVacuumPragmaOrder(t *testing.T) {
	t.Run("WAL first leaves the new file in mode 0", func(t *testing.T) {
		db, err := sql.Open(sqlite.DriverName, filepath.Join(t.TempDir(), "a.db"))
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		db.SetMaxOpenConns(1)
		mustExec(t, db, "PRAGMA journal_mode=WAL")
		mustExec(t, db, "PRAGMA auto_vacuum=2")
		mustExec(t, db, "CREATE TABLE t (id INTEGER)")
		assert.EqualValues(t, 0, pragmaInt(t, db, "auto_vacuum"))
	})
	t.Run("auto_vacuum before WAL gives mode 2", func(t *testing.T) {
		db := openScratch(t, filepath.Join(t.TempDir(), "b.db"), AutoVacuumIncremental)
		mustExec(t, db, "CREATE TABLE t (id INTEGER)")
		assert.EqualValues(t, 2, pragmaInt(t, db, "auto_vacuum"))
	})
	t.Run("on a populated file the pragma is a no-op", func(t *testing.T) {
		db, _ := newScratchDB(t, scratchOpts{rows: 100})
		mustExec(t, db, "PRAGMA auto_vacuum=2")
		assert.EqualValues(t, 0, pragmaInt(t, db, "auto_vacuum"))
	})
}

func TestDriver_VacuumOnPinnedConnConvertsAndKeepsWAL(t *testing.T) {
	db, path := newScratchDB(t, scratchOpts{rows: 120, rowBytes: 100000, keepEvery: 10})
	before := fileSize(t, path)
	require.EqualValues(t, 0, pragmaInt(t, db, "auto_vacuum"))

	conn, err := db.Conn(context.Background())
	require.NoError(t, err)
	mustExec(t, conn, "PRAGMA auto_vacuum=2")
	mustExec(t, conn, "VACUUM")
	busy := mustCheckpoint(t, conn)
	require.NoError(t, conn.Close())

	assert.Zero(t, busy)
	assert.EqualValues(t, 2, pragmaInt(t, db, "auto_vacuum"))
	var mode string
	require.NoError(t, db.QueryRow("PRAGMA journal_mode").Scan(&mode))
	assert.Equal(t, "wal", mode)
	assert.Less(t, fileSize(t, path), before, "the file must shrink, verified by size")
	integrityOK(t, db)
}

func TestDriver_BeginExclusiveDetectsWritersOnly(t *testing.T) {
	db, path := newScratchDB(t, scratchOpts{rows: 50})
	other := openScratch(t, path, 0)
	ctx := context.Background()

	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	mustExec(t, conn, "PRAGMA busy_timeout=0")

	t.Run("a concurrent WAL reader does not block", func(t *testing.T) {
		tx, err := other.BeginTx(ctx, nil)
		require.NoError(t, err)
		var n int
		require.NoError(t, tx.QueryRow("SELECT count(*) FROM t").Scan(&n))

		_, err = conn.ExecContext(ctx, "BEGIN EXCLUSIVE")
		require.NoError(t, err)
		mustExec(t, conn, "ROLLBACK")
		require.NoError(t, tx.Rollback())
	})
	t.Run("a concurrent writer fails with SQLITE_BUSY", func(t *testing.T) {
		tx, err := other.BeginTx(ctx, nil)
		require.NoError(t, err)
		_, err = tx.Exec("INSERT INTO t(pad) VALUES (randomblob(10))")
		require.NoError(t, err)

		_, err = conn.ExecContext(ctx, "BEGIN EXCLUSIVE")
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "locked")
		require.NoError(t, tx.Rollback())
	})
}

// queryDrainStep issues PRAGMA incremental_vacuum(n) via QueryContext, iterating
// every row and closing the rows before returning (the only form that frees n
// pages per call).
func queryDrainStep(t *testing.T, q Querier, n int) {
	t.Helper()
	rows, err := q.QueryContext(context.Background(), fmt.Sprintf("PRAGMA incremental_vacuum(%d)", n))
	require.NoError(t, err)
	for rows.Next() {
		// every row is one freed page; iterating them all is what does the work
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
}

func TestDriver_IncrementalVacuumQueryFreesNPagesPerCall(t *testing.T) {
	db, path := newScratchDB(t, scratchOpts{autoVacuum: AutoVacuumIncremental, rows: 600, rowBytes: 100000, keepEvery: 10})
	free := pragmaInt(t, db, "freelist_count")
	require.Greater(t, free, int64(10000), "scratch database must have >= 10k free pages")
	sizeBefore := fileSize(t, path)

	const step = 2000
	steps := int((free + step - 1) / step)
	for i := 0; i < steps; i++ {
		before := pragmaInt(t, db, "freelist_count")
		queryDrainStep(t, db, step)
		drop := before - pragmaInt(t, db, "freelist_count")
		want := int64(step)
		if before < want {
			want = before
		}
		assert.Equal(t, want, drop, "step %d must free min(N, remaining) pages", i)
	}
	assert.Zero(t, pragmaInt(t, db, "freelist_count"))

	mustCheckpoint(t, db)
	assert.Less(t, fileSize(t, path), sizeBefore/2, "the file must shrink after the checkpoint (by size)")
	integrityOK(t, db)
}

// TestDriver_IncrementalVacuumExecFreesOnePage is the trap: Exec steps the
// statement once, freeing ONE page per call whatever N is. If a driver upgrade
// fixes this the test fails and Drain can be simplified; a regression of Drain
// to Exec is caught by the Drain tests.
func TestDriver_IncrementalVacuumExecFreesOnePage(t *testing.T) {
	db, _ := newScratchDB(t, scratchOpts{autoVacuum: AutoVacuumIncremental, rows: 400, rowBytes: 100000, keepEvery: 10})
	before := pragmaInt(t, db, "freelist_count")
	require.Greater(t, before, int64(2000))

	const calls = 5
	for i := 0; i < calls; i++ {
		mustExec(t, db, "PRAGMA incremental_vacuum(2000)")
	}

	assert.EqualValues(t, calls, before-pragmaInt(t, db, "freelist_count"),
		"Exec must free exactly one page per call on this driver")
}

func TestDriver_VacuumIntoKeepsIncrementalMode(t *testing.T) {
	db, _ := newScratchDB(t, scratchOpts{autoVacuum: AutoVacuumIncremental, rows: 500})
	dest := filepath.Join(t.TempDir(), "snapshot.db")

	mustExec(t, db, fmt.Sprintf("VACUUM INTO '%s'", dest))

	snap, err := sql.Open(sqlite.DriverName, dest)
	require.NoError(t, err)
	t.Cleanup(func() { _ = snap.Close() })
	assert.EqualValues(t, 2, pragmaInt(t, snap, "auto_vacuum"))
}

// TestDriver_OpenRowsBlockThePool: with MaxOpenConns(1) any other pool query
// waits until the open rows are closed, so Drain must close them first.
func TestDriver_OpenRowsBlockThePool(t *testing.T) {
	db, _ := newScratchDB(t, scratchOpts{autoVacuum: AutoVacuumIncremental, rows: 100, rowBytes: 50000, keepEvery: 10})

	rows, err := db.QueryContext(context.Background(), "PRAGMA incremental_vacuum(10)")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	var n int64
	err = db.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&n)
	require.Error(t, err, "a query issued while rows are open must block until its context expires")

	for rows.Next() {
	}
	require.NoError(t, rows.Close())

	start := time.Now()
	require.NoError(t, db.QueryRowContext(context.Background(), "PRAGMA freelist_count").Scan(&n))
	assert.Less(t, time.Since(start), 250*time.Millisecond, "after rows.Close() the pool is free again")
}

// TestDriver_CheckpointBusyIsAColumnNotAnError: a blocking reader makes
// wal_checkpoint(TRUNCATE) return busy=1 with a nil error.
func TestDriver_CheckpointBusyIsAColumnNotAnError(t *testing.T) {
	db, path := newScratchDB(t, scratchOpts{rows: 200})
	reader := openScratch(t, path, 0)
	ctx := context.Background()

	tx, err := reader.BeginTx(ctx, nil)
	require.NoError(t, err)
	var n int
	require.NoError(t, tx.QueryRow("SELECT count(*) FROM t").Scan(&n))

	// New WAL frames beyond the reader's snapshot.
	mustExec(t, db, "INSERT INTO t(pad) VALUES (randomblob(100))")
	mustExec(t, db, "PRAGMA busy_timeout=0")

	var busy, logFrames, checkpointed int64
	err = db.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointed)
	require.NoError(t, err, "a blocked checkpoint reports through the busy column, err stays nil")
	assert.EqualValues(t, 1, busy)

	require.NoError(t, tx.Rollback())
	busy = mustCheckpoint(t, db)
	assert.Zero(t, busy)
}

func TestDriver_PreparedStatementsSurviveVacuum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gorm.db")
	gdb, err := gorm.Open(sqlite.Open(path), &gorm.Config{PrepareStmt: true})
	require.NoError(t, err)
	sqlDB, err := gdb.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	type item struct {
		ID  uint
		Val string
	}
	require.NoError(t, gdb.AutoMigrate(&item{}))
	require.NoError(t, gdb.Create(&item{Val: "a"}).Error)
	var got item
	require.NoError(t, gdb.First(&got).Error, "warm the prepared statement cache")

	conn, err := sqlDB.Conn(context.Background())
	require.NoError(t, err)
	mustExec(t, conn, "PRAGMA auto_vacuum=2")
	mustExec(t, conn, "VACUUM")
	require.NoError(t, conn.Close())

	require.NoError(t, gdb.First(&got).Error, "cached statement must still work after the schema cookie bump")
	require.NoError(t, gdb.Create(&item{Val: "b"}).Error)
}

// buildCancelDB returns a database whose VACUUM takes long enough to cancel:
// about 60 MB of which two thirds are free.
func buildCancelDB(t *testing.T) (db *sql.DB, path string) {
	t.Helper()
	return newScratchDB(t, scratchOpts{rows: 600, rowBytes: 100000, keepEvery: 3})
}

// vacuumWithCancel runs VACUUM on a pinned conn, cancelling after delay, and
// reports the result and how long VACUUM ran.
func vacuumWithCancel(t *testing.T, db *sql.DB, delay time.Duration) (time.Duration, error) {
	t.Helper()
	conn, err := db.Conn(context.Background())
	require.NoError(t, err)
	// The pool has one connection: release it before the caller queries again.
	defer func() { _ = conn.Close() }()
	mustExec(t, conn, "PRAGMA auto_vacuum=2")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	timer := time.AfterFunc(delay, cancel)
	defer timer.Stop()

	start := time.Now()
	_, err = conn.ExecContext(ctx, "VACUUM")
	return time.Since(start), err
}

func openTempFDs(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)
	var out []string
	for _, e := range entries {
		link, err := os.Readlink("/proc/self/fd/" + e.Name())
		if err == nil && strings.Contains(link, "etilqs_") {
			out = append(out, link)
		}
	}
	return out
}

// TestDriver_VacuumCancelEarlyInterruptsRebuild: a cancel during the rebuild
// phase interrupts VACUUM, leaves the database intact and in its old mode, and
// leaves no temp file behind.
func TestDriver_VacuumCancelEarlyInterruptsRebuild(t *testing.T) {
	db, _ := buildCancelDB(t)

	_, err := vacuumWithCancel(t, db, 10*time.Millisecond)

	require.Error(t, err, "cancel during the rebuild phase must interrupt VACUUM")
	assert.Regexp(t, "interrupt|cancel", strings.ToLower(err.Error()))
	assert.EqualValues(t, 0, pragmaInt(t, db, "auto_vacuum"), "mode unchanged after an interrupted VACUUM")
	assert.Empty(t, openTempFDs(t), "no temp file left open")
	integrityOK(t, db)
}

// TestDriver_VacuumCancelLateMayComplete: the copy-back tail is not
// interruptible, so a late cancel yields either an interrupt error or nil. The
// test asserts outcomes (database intact, mode consistent with the result),
// never timing.
func TestDriver_VacuumCancelLateMayComplete(t *testing.T) {
	baseline, _ := buildCancelDB(t)
	total, _ := vacuumWithCancel(t, baseline, time.Hour)
	require.NotZero(t, total)

	for _, frac := range []float64{0.6, 0.85, 0.95} {
		db, _ := buildCancelDB(t)

		took, err := vacuumWithCancel(t, db, time.Duration(float64(total)*frac))
		t.Logf("cancel at %.0f%% of %s: VACUUM returned %v after %s", frac*100, total, err, took)

		mode := pragmaInt(t, db, "auto_vacuum")
		if err == nil {
			assert.EqualValues(t, 2, mode, "frac %.2f: a nil result means the conversion completed", frac)
		} else {
			assert.Regexp(t, "interrupt|cancel", strings.ToLower(err.Error()), "frac %.2f", frac)
			assert.EqualValues(t, 0, mode, "frac %.2f: an interrupted VACUUM leaves mode 0", frac)
		}
		assert.Empty(t, openTempFDs(t))
		integrityOK(t, db)
	}
}
