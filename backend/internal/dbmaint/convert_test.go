package dbmaint

import (
	"context"
	"database/sql"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// bulkRow is the table newScratchDB fills ("t"), read through gorm so the
// prepared-statement cache is exercised.
type bulkRow struct {
	ID  int64 `gorm:"primaryKey"`
	Pad []byte
}

func (bulkRow) TableName() string { return "t" }

// TestConvert_ScratchDBBecomesIncrementalAndShrinks is the gate test of the
// conversion: a real VACUUM on a scratch database (two thirds free) takes
// mode 0 to mode 2, keeps WAL, keeps every live row, shrinks the file and
// leaves gorm's PrepareStmt cache working.
func TestConvert_ScratchDBBecomesIncrementalAndShrinks(t *testing.T) {
	db, path := newScratchDB(t, scratchOpts{rows: 600, rowBytes: 100000, keepEvery: 3})
	gdb, err := gorm.Open(sqlite.Dialector{Conn: db}, &gorm.Config{PrepareStmt: true})
	require.NoError(t, err)

	var liveRows int64
	require.NoError(t, gdb.Model(&bulkRow{}).Count(&liveRows).Error, "warm the prepared statement cache")
	require.EqualValues(t, 200, liveRows)
	require.EqualValues(t, AutoVacuumNone, pragmaInt(t, db, "auto_vacuum"))
	before := fileSize(t, path)

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	require.NoError(t, Convert(ctx, conn))
	require.NoError(t, conn.Close())

	busy, err := checkpointTruncate(ctx, db)
	require.NoError(t, err)
	assert.False(t, busy)

	assert.EqualValues(t, AutoVacuumIncremental, pragmaInt(t, db, "auto_vacuum"))
	var journal string
	require.NoError(t, db.QueryRow("PRAGMA journal_mode").Scan(&journal))
	assert.Equal(t, "wal", journal, "the conversion must not change the journal mode")
	assert.Zero(t, pragmaInt(t, db, "freelist_count"))
	assert.Less(t, fileSize(t, path), before/2, "the file shrinks by file size, not only by pragma")
	integrityOK(t, db)

	var after int64
	require.NoError(t, gdb.Model(&bulkRow{}).Count(&after).Error, "the cached prepared statement survives the schema cookie bump")
	assert.EqualValues(t, 200, after)
	require.NoError(t, gdb.Create(&bulkRow{Pad: []byte("x")}).Error)
}

func TestConvert_IsANoOpOnAnIncrementalDatabase(t *testing.T) {
	db, _ := newScratchDB(t, scratchOpts{autoVacuum: AutoVacuumIncremental, rows: 10, keepEvery: 2})
	conn, err := db.Conn(context.Background())
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	require.NoError(t, Convert(context.Background(), conn))
	assert.EqualValues(t, AutoVacuumIncremental, pragmaInt(t, conn, "auto_vacuum"))
}

// A context that is already cancelled may or may not stop the statements in
// time (the driver interrupts asynchronously), so the test asserts outcomes:
// an error leaves mode 0, a nil result means the conversion completed, and the
// database is intact either way. Run decides by the result, not by ctx.
func TestConvert_CancelledContextLeavesAConsistentDatabase(t *testing.T) {
	db, _ := newScratchDB(t, scratchOpts{rows: 50, rowBytes: 10000, keepEvery: 3})
	conn, err := db.Conn(context.Background())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	convErr := Convert(ctx, conn)
	require.NoError(t, conn.Close())

	if convErr != nil {
		assert.EqualValues(t, AutoVacuumNone, pragmaInt(t, db, "auto_vacuum"), "a failed conversion leaves the old mode")
	} else {
		assert.EqualValues(t, AutoVacuumIncremental, pragmaInt(t, db, "auto_vacuum"), "a nil result means it completed")
	}
	integrityOK(t, db)
}

func TestConvert_ErrorsNameTheFailingStep(t *testing.T) {
	db, _ := newScratchDB(t, scratchOpts{rows: 10, keepEvery: 2})
	conn, err := db.Conn(context.Background())
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	err = Convert(context.Background(), conn)
	require.Error(t, err)
	assert.ErrorIs(t, err, sql.ErrConnDone)
	assert.Contains(t, err.Error(), "auto_vacuum")
}

func TestConvert_ReportsAModeThatDidNotChange(t *testing.T) {
	// A fake connection whose VACUUM "succeeds" but whose mode stays 0 must
	// not be reported as converted.
	db, _ := newScratchDB(t, scratchOpts{rows: 10, keepEvery: 2})
	conn, err := db.Conn(context.Background())
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	err = verifyIncremental(context.Background(), conn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auto_vacuum")
}
