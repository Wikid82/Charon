package dbmaint

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStats_Helpers(t *testing.T) {
	s := Stats{PageSize: 4096, PageCount: 1000, FreelistCount: 250, AutoVacuum: AutoVacuumIncremental}

	assert.EqualValues(t, 250*4096, s.ReclaimableBytes())
	assert.EqualValues(t, 750*4096, s.LiveBytes())
	assert.InDelta(t, 0.25, s.FreeRatio(), 1e-9)
	assert.True(t, s.IsIncremental())
	assert.False(t, Stats{AutoVacuum: AutoVacuumNone}.IsIncremental())
	assert.Zero(t, Stats{}.FreeRatio())
}

func TestInspect(t *testing.T) {
	db, path := newScratchDB(t, scratchOpts{autoVacuum: AutoVacuumIncremental, rows: 4000, keepEvery: 4})
	// Leave WAL frames behind so the WAL size is non-zero.
	mustExec(t, db, "INSERT INTO t(pad) VALUES (randomblob(100))")

	s, err := Inspect(context.Background(), db, path)
	require.NoError(t, err)

	assert.EqualValues(t, 4096, s.PageSize)
	assert.Greater(t, s.PageCount, int64(0))
	assert.Greater(t, s.FreelistCount, int64(0))
	assert.True(t, s.IsIncremental())
	assert.Equal(t, fileSize(t, path), s.MainBytes)
	assert.Equal(t, fileSize(t, path+"-wal"), s.WALBytes)
	assert.Greater(t, s.WALBytes, int64(0))
}

func TestInspect_MissingWALIsZero(t *testing.T) {
	db, path := newScratchDB(t, scratchOpts{rows: 10})
	mustCheckpoint(t, db)
	require.NoError(t, os.Remove(path+"-wal"))

	s, err := Inspect(context.Background(), db, path)
	require.NoError(t, err)
	assert.Zero(t, s.WALBytes)
}

func TestInspect_Errors(t *testing.T) {
	db, path := newScratchDB(t, scratchOpts{rows: 10})

	_, err := Inspect(context.Background(), db, path+".missing")
	require.Error(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Inspect(ctx, db, path)
	require.Error(t, err)
}
