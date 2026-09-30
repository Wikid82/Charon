package dbmaint

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// worthwhile is a PlanResult that Decide would convert.
func worthwhile() PlanResult {
	s := statsWith(128000, 64000)
	return PlanResult{Stats: s, Decision: Decide(Inputs{EnvMode: config.DBCompactAuto, Stats: s})}
}

func TestAdvisor_RecordLogsTheTransitionOnce(t *testing.T) {
	logs := captureLogs(t)
	a := &Advisor{}

	first := a.record(worthwhile())
	second := a.record(worthwhile())

	assert.True(t, first.Pending)
	assert.True(t, second.Pending)
	assert.EqualValues(t, 64000*4096, first.ReclaimableBytes)
	assert.Equal(t, 1, strings.Count(logs.String(), "database optimization pending"))
	assert.Contains(t, logs.String(), "next time Charon starts")
	assert.Equal(t, second, a.Current())
}

func TestAdvisor_RecordClearsPendingWhenNoLongerWorthwhile(t *testing.T) {
	captureLogs(t)
	a := &Advisor{}
	a.record(worthwhile())

	small := statsWith(1000, 10)
	got := a.record(PlanResult{Stats: small, Decision: Decide(Inputs{EnvMode: config.DBCompactAuto, Stats: small})})

	assert.False(t, got.Pending)
	assert.Equal(t, ReasonBelowThreshold, got.Reason)
}

func TestAdvisor_Mode2NeverPending(t *testing.T) {
	logs := captureLogs(t)
	db, path := newScratchDB(t, scratchOpts{autoVacuum: AutoVacuumIncremental, rows: 40, rowBytes: 50000, keepEvery: 10})
	a := &Advisor{}

	got := a.Advise(context.Background(), db, PlanConfig{DBPath: path, EnvMode: config.DBCompactAuto})

	assert.False(t, got.Pending)
	assert.Equal(t, ReasonAlreadyOptimized, got.Reason)
	assert.NotContains(t, logs.String(), "optimization pending")
}

func TestAdvisor_DryRunIgnoresTheFlagAndAgreesWithDecide(t *testing.T) {
	captureLogs(t)
	db, path := newScratchDB(t, scratchOpts{rows: 40, rowBytes: 50000, keepEvery: 10})
	a := &Advisor{}

	// A set flag would make Decide run past the ratio, but a 4 MB scratch file
	// is below the floor either way; the dry run must not use the flag at all.
	got := a.Advise(context.Background(), db, PlanConfig{DBPath: path, EnvMode: config.DBCompactAuto, FlagRequested: true, Attempts: 99})

	want, err := Plan(context.Background(), db, PlanConfig{DBPath: path, EnvMode: config.DBCompactAuto})
	require.NoError(t, err)
	assert.Equal(t, want.Decision.Run, got.Pending)
	assert.Equal(t, want.Decision.Reason, got.Reason)
}

func TestAdvisor_PlanErrorKeepsPreviousAdvice(t *testing.T) {
	logs := captureLogs(t)
	db, path := newScratchDB(t, scratchOpts{rows: 10})
	a := &Advisor{}
	a.record(worthwhile())
	prev := a.Current()
	require.NoError(t, db.Close())

	got := a.Advise(context.Background(), db, PlanConfig{DBPath: path})

	assert.Equal(t, prev, got)
	assert.Contains(t, logs.String(), "could not evaluate")
}

func TestAdvisor_TerminalSkipOfTheLastResultSilencesThePendingLine(t *testing.T) {
	logs := captureLogs(t)
	s := statsWith(128000, 64000)
	res := PlanResult{Stats: s, Decision: Decide(Inputs{EnvMode: config.DBCompactAuto, Stats: s})}

	for _, reason := range []Reason{ReasonIntegrityCheckFailed, ReasonTooManyFailures} {
		a := NewAdvisor(func(context.Context) *LastResult { return &LastResult{Outcome: ResultSkipped, Reason: reason} })
		got := a.adviseFrom(context.Background(), res)
		assert.False(t, got.Pending, reason)
		assert.Equal(t, reason, got.Reason)
	}
	assert.NotContains(t, logs.String(), "optimization pending")
}

func TestAdvisor_TransientLastResultDoesNotSilenceIt(t *testing.T) {
	logs := captureLogs(t)
	s := statsWith(128000, 64000)
	res := PlanResult{Stats: s, Decision: Decide(Inputs{EnvMode: config.DBCompactAuto, Stats: s})}
	a := NewAdvisor(func(context.Context) *LastResult {
		return &LastResult{Outcome: ResultSkipped, Reason: ReasonDatabaseBusy}
	})

	got := a.adviseFrom(context.Background(), res)

	assert.True(t, got.Pending)
	assert.Contains(t, logs.String(), "optimization pending")
}

func TestStoreHistory_ReadsTheLastResultOfTheCurrentFile(t *testing.T) {
	db, path := newSettingsDB(t)
	id, err := FileID(path)
	require.NoError(t, err)
	s := NewStore(db)
	ctx := context.Background()
	history := StoreHistory(s, path)

	assert.Nil(t, history(ctx), "no row yet")
	require.NoError(t, s.WriteLastResult(ctx, LastResult{Outcome: ResultSkipped, Reason: ReasonIntegrityCheckFailed, FileID: id}))
	got := history(ctx)
	require.NotNil(t, got)
	assert.Equal(t, ReasonIntegrityCheckFailed, got.Reason)

	require.NoError(t, s.WriteLastResult(ctx, LastResult{Outcome: ResultSkipped, Reason: ReasonIntegrityCheckFailed, FileID: "1"}))
	assert.Nil(t, history(ctx), "a result of another file does not apply")

	require.NoError(t, db.Close())
	assert.Nil(t, history(ctx), "an unreadable store means no history, never a failure")
	assert.Nil(t, StoreHistory(s, filepath.Join(t.TempDir(), "missing"))(ctx), "an unknown file id means no history")
}
