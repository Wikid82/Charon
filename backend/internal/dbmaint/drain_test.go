package dbmaint

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lockedBuffer is a goroutine-safe log sink.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// captureLogs routes the global logger into a buffer for the test.
func captureLogs(t *testing.T) *lockedBuffer {
	t.Helper()
	buf := &lockedBuffer{}
	logger.Init(false, buf)
	t.Cleanup(func() { logger.Init(false, os.Stdout) })
	return buf
}

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// drainScratch builds a mode-2 database with well over 10k free pages.
func drainScratch(t *testing.T) (db *sql.DB, path string) {
	t.Helper()
	db, path = newScratchDB(t, scratchOpts{autoVacuum: AutoVacuumIncremental, rows: 600, rowBytes: 100000, keepEvery: 10})
	require.Greater(t, pragmaInt(t, db, "freelist_count"), int64(10000))
	return db, path
}

func TestDrain_FreesAllPagesAboveTheFloorAndShrinksTheFile(t *testing.T) {
	db, path := drainScratch(t)
	free := pragmaInt(t, db, "freelist_count")
	sizeBefore := fileSize(t, path)
	const keepPages = 500
	opts := DrainOptions{PagesPerStep: DrainPagesPerStep, StepPause: time.Millisecond, Budget: time.Minute,
		KeepFreeBytes: keepPages * 4096}

	res, err := Drain(testCtx(t), db, opts)

	require.NoError(t, err)
	wantFreed := free - keepPages
	// page_count also drops by the pointer-map pages released along the way.
	assert.InDelta(t, wantFreed, res.PagesFreed, 64)
	assert.GreaterOrEqual(t, res.PagesFreed, wantFreed)
	assert.EqualValues(t, (wantFreed+DrainPagesPerStep-1)/DrainPagesPerStep, res.Steps)
	assert.EqualValues(t, keepPages, pragmaInt(t, db, "freelist_count"), "the free-list floor stays in place")
	assert.True(t, res.Checkpointed)
	assert.Less(t, fileSize(t, path), sizeBefore/2, "the file shrinks, verified by size")
	integrityOK(t, db)
}

// TestDrain_OneStepFreesPagesPerStepNotOnePage is the Exec-trap guard at the
// Drain level: a Drain regressed to Exec would free one page per step.
func TestDrain_OneStepFreesPagesPerStepNotOnePage(t *testing.T) {
	db, _ := drainScratch(t)
	before := pragmaInt(t, db, "freelist_count")
	// The floor leaves exactly one step of pages to free.
	opts := DrainOptions{PagesPerStep: DrainPagesPerStep, StepPause: -1,
		KeepFreeBytes: (before - DrainPagesPerStep) * 4096}

	res, err := Drain(testCtx(t), db, opts)

	require.NoError(t, err)
	assert.EqualValues(t, 1, res.Steps)
	assert.EqualValues(t, DrainPagesPerStep, before-pragmaInt(t, db, "freelist_count"),
		"freelist_count must fall by DrainPagesPerStep in one step")
	assert.InDelta(t, DrainPagesPerStep, res.PagesFreed, 16)
	assert.GreaterOrEqual(t, res.PagesFreed, int64(DrainPagesPerStep))
}

func TestDrain_BudgetExhaustedStopsBeforeTheNextStep(t *testing.T) {
	db, _ := drainScratch(t)
	before := pragmaInt(t, db, "freelist_count")

	res, err := Drain(testCtx(t), db, DrainOptions{Budget: time.Nanosecond})

	require.NoError(t, err)
	assert.True(t, res.BudgetExhausted)
	assert.Zero(t, res.Steps)
	assert.Equal(t, before, pragmaInt(t, db, "freelist_count"))
}

func TestDrain_DefaultsApplyToZeroOptions(t *testing.T) {
	db, _ := drainScratch(t)
	before := pragmaInt(t, db, "freelist_count")
	require.Greater(t, before*4096, KeepFreeBytes, "scratch must exceed the default floor")

	res, err := Drain(testCtx(t), db, DrainOptions{})

	require.NoError(t, err)
	assert.Positive(t, res.Steps)
	assert.LessOrEqual(t, pragmaInt(t, db, "freelist_count")*4096, KeepFreeBytes)
}

func TestDrain_NothingToDoBelowTheFloor(t *testing.T) {
	db, _ := newScratchDB(t, scratchOpts{autoVacuum: AutoVacuumIncremental, rows: 200, keepEvery: 2})
	before := pragmaInt(t, db, "freelist_count")

	res, err := Drain(testCtx(t), db, DrainOptions{})

	require.NoError(t, err)
	assert.Zero(t, res.Steps)
	assert.False(t, res.Checkpointed, "no checkpoint when nothing was freed")
	assert.Equal(t, before, pragmaInt(t, db, "freelist_count"))
}

func TestDrain_RefusesDatabaseNotInIncrementalMode(t *testing.T) {
	db, _ := newScratchDB(t, scratchOpts{rows: 400, rowBytes: 100000, keepEvery: 10})
	before := pragmaInt(t, db, "freelist_count")

	_, err := Drain(testCtx(t), db, DrainOptions{})

	require.ErrorIs(t, err, ErrNotIncremental)
	assert.Equal(t, before, pragmaInt(t, db, "freelist_count"))
}

func TestDrain_AbortsOnContextCancel(t *testing.T) {
	db, _ := drainScratch(t)
	ctx, cancel := context.WithCancel(testCtx(t))
	cancel()

	res, err := Drain(ctx, db, DrainOptions{})

	require.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, res.Steps)
}

func TestDrain_CancelDuringPauseStopsThePass(t *testing.T) {
	db, _ := drainScratch(t)
	ctx, cancel := context.WithCancel(testCtx(t))
	defer cancel()
	// The second page_count read ends the first step; cancel shortly after it so
	// the cancellation always lands in the pause, never inside the step.
	var pageCountReads atomic.Int32
	q := &interceptQuerier{Querier: db, onQueryRow: func(query string) (string, bool) {
		if query == "PRAGMA page_count" && pageCountReads.Add(1) == 2 {
			time.AfterFunc(50*time.Millisecond, cancel)
		}
		return "", false
	}}

	res, err := Drain(ctx, q, DrainOptions{StepPause: time.Hour, Budget: time.Hour})

	require.ErrorIs(t, err, context.Canceled)
	assert.EqualValues(t, 1, res.Steps, "the pass stops in the pause after the first step")
}

func TestDrain_BusyCheckpointIsNotAnError(t *testing.T) {
	db, path := drainScratch(t)
	reader := openScratch(t, path, 0)
	tx, err := reader.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	var n int
	require.NoError(t, tx.QueryRow("SELECT count(*) FROM t").Scan(&n))
	mustExec(t, db, "PRAGMA busy_timeout=0")
	logs := captureLogs(t)

	res, err := Drain(testCtx(t), db, DrainOptions{StepPause: time.Millisecond})

	require.NoError(t, err, "busy arrives in a column, never as an error")
	assert.Positive(t, res.PagesFreed)
	assert.False(t, res.Checkpointed)
	assert.NotContains(t, logs.String(), `"level":"warning"`)
	require.NoError(t, tx.Rollback())
}

func TestDrain_CheckpointErrorIsReturned(t *testing.T) {
	db, _ := drainScratch(t)
	q := &interceptQuerier{Querier: db, onQueryRow: func(query string) (string, bool) {
		if strings.Contains(query, "wal_checkpoint") {
			return "SELECT * FROM no_such_table", true
		}
		return "", false
	}}

	res, err := Drain(testCtx(t), q, DrainOptions{StepPause: time.Millisecond})

	require.Error(t, err)
	assert.Positive(t, res.PagesFreed, "the result still reports what was freed")
}

// interceptQuerier rewrites or decorates statements before they reach the
// database, to simulate driver regressions and concurrent writers.
type interceptQuerier struct {
	Querier
	onQuery    func(query string) string // returns the statement to run instead
	beforeStep func()
	onQueryRow func(query string) (string, bool)
}

func (q *interceptQuerier) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if strings.HasPrefix(query, "PRAGMA incremental_vacuum") {
		if q.beforeStep != nil {
			q.beforeStep()
		}
		if q.onQuery != nil {
			query = q.onQuery(query)
		}
	}
	return q.Querier.QueryContext(ctx, query, args...)
}

func (q *interceptQuerier) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	if q.onQueryRow != nil {
		if replacement, ok := q.onQueryRow(query); ok {
			query = replacement
		}
	}
	return q.Querier.QueryRowContext(ctx, query, args...)
}

// A driver that frees far fewer pages than requested (the Exec trap) must be
// reported, but never stops the pass.
func TestDrain_WarnsWhenAStepFreesTooFewPagesAndContinues(t *testing.T) {
	db, _ := newScratchDB(t, scratchOpts{autoVacuum: AutoVacuumIncremental, rows: 100, rowBytes: 50000, keepEvery: 10})
	free := pragmaInt(t, db, "freelist_count")
	require.Greater(t, free, int64(300))
	logs := captureLogs(t)
	q := &interceptQuerier{Querier: db, onQuery: func(string) string { return "PRAGMA incremental_vacuum(1)" }}

	res, err := Drain(testCtx(t), q, DrainOptions{PagesPerStep: 100, StepPause: -1, Budget: time.Minute, KeepFreeBytes: -1})

	require.NoError(t, err, "the sanity check is warn-and-continue")
	assert.Greater(t, res.Steps, 1, "the pass keeps going")
	assert.Contains(t, logs.String(), "freed fewer pages than requested")
}

// Concurrent inserts reusing free pages shrink freelist_count without freeing
// anything; measuring by the page_count delta must not raise a false alarm.
func TestDrain_ConcurrentInsertsDoNotFalseTriggerTheSanityCheck(t *testing.T) {
	db, _ := drainScratch(t)
	logs := captureLogs(t)
	q := &interceptQuerier{Querier: db, beforeStep: func() {
		mustExec(t, db, "INSERT INTO t(pad) SELECT randomblob(2000) FROM (SELECT 1 UNION ALL SELECT 2 UNION ALL SELECT 3 UNION ALL SELECT 4)")
	}}

	res, err := Drain(testCtx(t), q, DrainOptions{StepPause: time.Millisecond})

	require.NoError(t, err)
	assert.Positive(t, res.Steps)
	assert.NotContains(t, logs.String(), "freed fewer pages than requested")
}

func TestDrain_StepErrorIsWrapped(t *testing.T) {
	db, _ := drainScratch(t)
	q := &interceptQuerier{Querier: db, onQuery: func(string) string { return "PRAGMA no_such_pragma_error(" }}

	_, err := Drain(testCtx(t), q, DrainOptions{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "incremental_vacuum")
}

func TestDrain_InspectErrorsAreReturned(t *testing.T) {
	db, _ := drainScratch(t)
	require.NoError(t, db.Close())

	_, err := Drain(testCtx(t), db, DrainOptions{})

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotIncremental)
}
