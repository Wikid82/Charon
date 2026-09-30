package dbmaint

import (
	"context"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReclaimer_DrainsAnIncrementalDatabaseAfterADelete(t *testing.T) {
	db, path := drainScratch(t)
	sizeBefore := fileSize(t, path)
	r := NewReclaimer(db, path, config.DBCompactAuto, &Advisor{})
	r.opts = DrainOptions{StepPause: -1, Budget: DrainBudgetPerPass}

	r.AfterPrune(testCtx(t), 1000)

	assert.LessOrEqual(t, pragmaInt(t, db, "freelist_count")*4096, KeepFreeBytes)
	assert.Less(t, fileSize(t, path), sizeBefore*4/5)
}

func TestReclaimer_NoDeletesAndSmallFreelistDoesNothing(t *testing.T) {
	db, path := newScratchDB(t, scratchOpts{autoVacuum: AutoVacuumIncremental, rows: 400, keepEvery: 2})
	before := pragmaInt(t, db, "freelist_count")
	require.Less(t, before*4096, KeepFreeBytes)
	r := NewReclaimer(db, path, config.DBCompactAuto, &Advisor{})

	r.AfterPrune(testCtx(t), 0)

	assert.Equal(t, before, pragmaInt(t, db, "freelist_count"))
}

func TestReclaimer_LargeFreelistDrainsEvenWithoutDeletes(t *testing.T) {
	db, path := drainScratch(t)
	r := NewReclaimer(db, path, config.DBCompactAuto, &Advisor{})
	r.opts = DrainOptions{StepPause: -1}

	r.AfterPrune(testCtx(t), 0)

	assert.LessOrEqual(t, pragmaInt(t, db, "freelist_count")*4096, KeepFreeBytes)
}

func TestReclaimer_LegacyDatabaseIsNeverDrainedAndIsAdvisedInstead(t *testing.T) {
	captureLogs(t)
	db, path := newScratchDB(t, scratchOpts{rows: 400, rowBytes: 100000, keepEvery: 10})
	before := pragmaInt(t, db, "freelist_count")
	advisor := &Advisor{}
	r := NewReclaimer(db, path, config.DBCompactAuto, advisor)

	r.AfterPrune(testCtx(t), 5000)

	assert.Equal(t, before, pragmaInt(t, db, "freelist_count"), "a mode-0 database is never drained")
	// A 40 MB scratch file is below the 100 MiB floor: the advisor ran and said no.
	assert.False(t, advisor.Current().Pending)
	assert.Equal(t, ReasonBelowThreshold, advisor.Current().Reason)
}

func TestReclaimer_StopsQuietlyWhenTheContextIsCancelled(t *testing.T) {
	db, path := drainScratch(t)
	before := pragmaInt(t, db, "freelist_count")
	ctx, cancel := context.WithCancel(testCtx(t))
	cancel()
	r := NewReclaimer(db, path, config.DBCompactAuto, &Advisor{})

	r.AfterPrune(ctx, 10)

	assert.Equal(t, before, pragmaInt(t, db, "freelist_count"))
}

func TestReclaimer_InspectFailureIsLoggedNotFatal(t *testing.T) {
	db, path := newScratchDB(t, scratchOpts{autoVacuum: AutoVacuumIncremental, rows: 10})
	logs := captureLogs(t)
	r := NewReclaimer(db, path+".missing", config.DBCompactAuto, &Advisor{})

	r.AfterPrune(testCtx(t), 10)

	assert.Contains(t, logs.String(), "could not inspect")
}

func TestReclaimer_DrainFailureIsLoggedNotFatal(t *testing.T) {
	db, path := drainScratch(t)
	logs := captureLogs(t)
	broken := &interceptQuerier{Querier: db, onQuery: func(string) string { return "PRAGMA no_such_pragma_error(" }}
	r := NewReclaimer(broken, path, config.DBCompactAuto, &Advisor{})

	r.AfterPrune(testCtx(t), 10)

	assert.Contains(t, logs.String(), "drain")
}
