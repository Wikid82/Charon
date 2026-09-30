package dbmaint

import (
	"context"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func enableConversion(t *testing.T) {
	t.Helper()
	orig := conversionEnabled
	conversionEnabled = true
	t.Cleanup(func() { conversionEnabled = orig })
}

// worthwhile is a PlanResult that Decide would convert.
func worthwhile() PlanResult {
	s := statsWith(128000, 64000)
	return PlanResult{Stats: s, Decision: Decide(Inputs{EnvMode: config.DBCompactAuto, Stats: s})}
}

func TestAdvisor_ConversionDisabledIsANoOp(t *testing.T) {
	logs := captureLogs(t)
	db, path := newScratchDB(t, scratchOpts{rows: 40, rowBytes: 50000, keepEvery: 10})
	a := &Advisor{}

	got := a.Advise(context.Background(), db, PlanConfig{DBPath: path, EnvMode: config.DBCompactAuto})

	assert.Equal(t, Advice{}, got)
	assert.Equal(t, Advice{}, a.Current())
	assert.NotContains(t, logs.String(), "optimization pending")
}

func TestAdvisor_RecordLogsTheTransitionOnce(t *testing.T) {
	enableConversion(t)
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
	enableConversion(t)
	captureLogs(t)
	a := &Advisor{}
	a.record(worthwhile())

	small := statsWith(1000, 10)
	got := a.record(PlanResult{Stats: small, Decision: Decide(Inputs{EnvMode: config.DBCompactAuto, Stats: small})})

	assert.False(t, got.Pending)
	assert.Equal(t, ReasonBelowThreshold, got.Reason)
}

func TestAdvisor_Mode2NeverPending(t *testing.T) {
	enableConversion(t)
	logs := captureLogs(t)
	db, path := newScratchDB(t, scratchOpts{autoVacuum: AutoVacuumIncremental, rows: 40, rowBytes: 50000, keepEvery: 10})
	a := &Advisor{}

	got := a.Advise(context.Background(), db, PlanConfig{DBPath: path, EnvMode: config.DBCompactAuto})

	assert.False(t, got.Pending)
	assert.Equal(t, ReasonAlreadyOptimized, got.Reason)
	assert.NotContains(t, logs.String(), "optimization pending")
}

func TestAdvisor_DryRunIgnoresTheFlagAndAgreesWithDecide(t *testing.T) {
	enableConversion(t)
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
	enableConversion(t)
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
