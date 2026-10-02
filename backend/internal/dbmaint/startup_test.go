package dbmaint

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
)

func TestSafePlan(t *testing.T) {
	ctx := context.Background()
	want := PlanResult{Decision: Decision{Run: true}}

	t.Run("passes a result through", func(t *testing.T) {
		got := SafePlan(ctx, func(context.Context) (PlanResult, error) { return want, nil })
		assert.True(t, got.Decision.Run)
	})
	t.Run("an error means idle", func(t *testing.T) {
		got := SafePlan(ctx, func(context.Context) (PlanResult, error) { return want, errors.New("disk report failed") })
		assert.False(t, got.Decision.Run)
	})
	t.Run("a panic means idle", func(t *testing.T) {
		var got PlanResult
		require.NotPanics(t, func() {
			got = SafePlan(ctx, func(context.Context) (PlanResult, error) { panic("boom") })
		})
		assert.False(t, got.Decision.Run)
	})
}

func TestStartupPlan_SmallDatabaseIsBelowThreshold(t *testing.T) {
	db, path := newSettingsDB(t)

	res, err := StartupPlan(context.Background(), db, path, config.DBCompactAuto)
	require.NoError(t, err)
	assert.False(t, res.Decision.Run)
	assert.Equal(t, ReasonBelowThreshold, res.Decision.Reason)
}

func TestStartupPlan_ReadsThePersistedState(t *testing.T) {
	db, path := newSettingsDB(t)
	ctx := context.Background()
	require.NoError(t, NewStore(db).SetFlag(ctx))

	res, err := StartupPlan(ctx, db, path, config.DBCompactAuto)
	require.NoError(t, err)
	assert.False(t, res.Decision.Run)
	assert.Equal(t, ReasonNothingToReclaim, res.Decision.Reason, "the flag was read; a tiny file has nothing to reclaim")
	assert.True(t, res.Decision.ClearFlag)
}

func TestStartupPlan_ErrorsSurface(t *testing.T) {

	t.Run("state cannot be loaded", func(t *testing.T) {
		db, path := newSettingsDB(t)
		_, err := db.Exec("DROP TABLE settings")
		require.NoError(t, err)
		_, err = StartupPlan(context.Background(), db, path, config.DBCompactAuto)
		assert.Error(t, err)
	})
	t.Run("file cannot be identified", func(t *testing.T) {
		db, path := newSettingsDB(t)
		_, err := StartupPlan(context.Background(), db, path+".missing", config.DBCompactAuto)
		assert.Error(t, err)
	})
}

func TestStart(t *testing.T) {
	t.Run("nil gate does nothing", func(t *testing.T) {
		assert.False(t, Start(context.Background(), StartParams{}))
	})

	t.Run("the production plan leaves a database below the thresholds idle and defers nothing", func(t *testing.T) {
		db, path := newSettingsDB(t)
		gate := NewGate()

		planned := Start(context.Background(), StartParams{Gate: gate, DB: db, DBPath: path, EnvMode: config.DBCompactAuto})

		assert.False(t, planned)
		assert.Equal(t, PhaseIdle, gate.Snapshot().Phase)
		assert.False(t, gate.Deferring())
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		assert.True(t, gate.WaitIdle(ctx), "the uptime pipeline would start at once")
		assert.True(t, gate.WaitRunner(time.Millisecond), "and no runner exists")
	})

	t.Run("a skip decision stays idle", func(t *testing.T) {
		for _, reason := range []Reason{ReasonBelowThreshold, ReasonAlreadyOptimized, ReasonInsufficientDisk, ReasonDisabledByEnv, ""} {
			gate := NewGate()
			planned := Start(context.Background(), StartParams{
				Gate: gate,
				Plan: func(context.Context) (PlanResult, error) {
					return PlanResult{Decision: Decision{Reason: reason, RequiredBytes: 2, AvailableBytes: 1}}, nil
				},
			})
			assert.False(t, planned, string(reason))
			assert.Equal(t, PhaseIdle, gate.Snapshot().Phase, string(reason))
		}
	})

	t.Run("a failing plan stays idle", func(t *testing.T) {
		gate := NewGate()
		planned := Start(context.Background(), StartParams{
			Gate: gate,
			Plan: func(context.Context) (PlanResult, error) { panic("boom") },
		})
		assert.False(t, planned)
		assert.Equal(t, PhaseIdle, gate.Snapshot().Phase)
	})

	t.Run("a run decision plans the gate and starts the runner", func(t *testing.T) {
		db, path := newSettingsDB(t)
		gate := NewGate()
		converted := make(chan struct{})

		planned := Start(context.Background(), StartParams{
			Gate:   gate,
			DB:     db,
			DBPath: path,
			Plan: func(context.Context) (PlanResult, error) {
				return PlanResult{Decision: Decision{Run: true}}, nil
			},
			Convert: func(context.Context, *sql.Conn) error { close(converted); return nil },
			Timings: Timings{PlannedMaxWait: 5 * time.Second, CheckpointBackoff: time.Millisecond},
		})

		require.True(t, planned)
		assert.Equal(t, PhasePlanned, gate.Snapshot().Phase)
		assert.True(t, gate.Deferring())

		gate.ConfigDone(true)
		gate.MarkListenerBound()
		select {
		case <-converted:
		case <-time.After(5 * time.Second):
			t.Fatal("the runner did not convert once both readiness signals arrived")
		}
		require.True(t, gate.WaitRunner(5*time.Second))
		assert.Equal(t, PhaseDone, gate.Snapshot().Phase)
	})
}

func TestStart_ACleanableSkipClearsTheFlagAndAnyOtherSkipKeepsIt(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name     string
		decision Decision
		wantFlag bool
	}{
		{"nothing to reclaim", Decision{Reason: ReasonNothingToReclaim, ClearFlag: true}, false},
		{"already optimized", Decision{Reason: ReasonAlreadyOptimized, ClearFlag: true}, false},
		{"insufficient disk keeps the request", Decision{Reason: ReasonInsufficientDisk, RequiredBytes: 2, AvailableBytes: 1}, true},
		{"disabled by env keeps the request", Decision{Reason: ReasonDisabledByEnv}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, path := newSettingsDB(t)
			require.NoError(t, NewStore(db).SetFlag(ctx))

			planned := Start(ctx, StartParams{
				Gate: NewGate(), DB: db, DBPath: path,
				Plan: func(context.Context) (PlanResult, error) { return PlanResult{Decision: tc.decision}, nil },
			})

			assert.False(t, planned)
			on, err := NewStore(db).FlagRequested(ctx)
			require.NoError(t, err)
			assert.Equal(t, tc.wantFlag, on)
		})
	}
}

// A terminal plan-time skip is remembered so Advise does not promise a
// conversion the next boot will refuse again.
func TestStart_TooManyFailuresIsRecordedAsTheLastResult(t *testing.T) {
	ctx := context.Background()
	db, path := newSettingsDB(t)

	Start(ctx, StartParams{
		Gate: NewGate(), DB: db, DBPath: path,
		Plan: func(context.Context) (PlanResult, error) {
			return PlanResult{Decision: Decision{Reason: ReasonTooManyFailures}}, nil
		},
	})

	fileID, err := FileID(path)
	require.NoError(t, err)
	st, err := NewStore(db).Peek(ctx, fileID)
	require.NoError(t, err)
	require.NotNil(t, st.LastResult)
	assert.Equal(t, ResultSkipped, st.LastResult.Outcome)
	assert.Equal(t, ReasonTooManyFailures, st.LastResult.Reason)
	assert.True(t, SuppressesPending(st.LastResult))
}

func TestStart_OtherSkipsDoNotOverwriteTheLastResult(t *testing.T) {
	ctx := context.Background()
	db, path := newSettingsDB(t)
	fileID, err := FileID(path)
	require.NoError(t, err)
	require.NoError(t, NewStore(db).WriteLastResult(ctx, LastResult{Outcome: ResultConverted, FileID: fileID}))

	Start(ctx, StartParams{
		Gate: NewGate(), DB: db, DBPath: path,
		Plan: func(context.Context) (PlanResult, error) {
			return PlanResult{Decision: Decision{Reason: ReasonBelowThreshold}}, nil
		},
	})

	st, err := NewStore(db).Peek(ctx, fileID)
	require.NoError(t, err)
	assert.Equal(t, ResultConverted, st.LastResult.Outcome)
}

func TestStart_PlanSkipThatCannotBeRecordedStillLeavesTheGateIdle(t *testing.T) {
	db, path := newSettingsDB(t)
	require.NoError(t, db.Close())
	gate := NewGate()

	planned := Start(context.Background(), StartParams{
		Gate: gate, DB: db, DBPath: path,
		Plan: func(context.Context) (PlanResult, error) {
			return PlanResult{Decision: Decision{Reason: ReasonTooManyFailures, ClearFlag: true}}, nil
		},
	})

	assert.False(t, planned)
	assert.Equal(t, PhaseIdle, gate.Snapshot().Phase)
}

// A reclaim request bypasses the threshold, not the failure back-off: a start
// that is killed on every boot converts MaxConvertAttempts times, then stops,
// clears the request and records the stop.
func TestStartupPlan_FlaggedCrashLoopStopsAfterMaxConvertAttempts(t *testing.T) {
	ctx := context.Background()
	db, path := newSettingsDBWith(t, scratchOpts{rows: 1300, rowBytes: 100000, keepEvery: 10})
	fileID, err := FileID(path)
	require.NoError(t, err)
	store := NewStore(db)
	require.NoError(t, store.SetFlag(ctx))

	for boot := range MaxConvertAttempts {
		res, planErr := StartupPlan(ctx, db, path, config.DBCompactAuto)
		require.NoError(t, planErr)
		require.True(t, res.Decision.Run, "boot %d must convert", boot)
		// The process is killed mid-conversion: the marker stays behind.
		require.NoError(t, store.SetInProgress(ctx, fileID, time.Now()))
	}

	res, err := StartupPlan(ctx, db, path, config.DBCompactAuto)
	require.NoError(t, err)
	assert.False(t, res.Decision.Run)
	assert.Equal(t, ReasonTooManyFailures, res.Decision.Reason)
	assert.True(t, res.Decision.ClearFlag)

	assert.False(t, Start(ctx, StartParams{Gate: NewGate(), DB: db, DBPath: path, EnvMode: config.DBCompactAuto}))
	on, err := store.FlagRequested(ctx)
	require.NoError(t, err)
	assert.False(t, on, "the back-off stop clears the request")
	assert.False(t, settingFound(t, db, SettingKeyFlag))
	st, err := store.Peek(ctx, fileID)
	require.NoError(t, err)
	require.NotNil(t, st.LastResult)
	assert.Equal(t, ResultSkipped, st.LastResult.Outcome)
	assert.Equal(t, ReasonTooManyFailures, st.LastResult.Reason)
}

func TestLogPlanSkip_LevelsPerReason(t *testing.T) {
	logOf := func(reason Reason) string {
		buf := captureLogs(t)
		logPlanSkip(Decision{Reason: reason})
		return buf.String()
	}

	tooMany := logOf(ReasonTooManyFailures)
	assert.Contains(t, tooMany, `"level":"warning"`)
	assert.Contains(t, tooMany, string(ReasonTooManyFailures))

	optimized := logOf(ReasonAlreadyOptimized)
	assert.Contains(t, optimized, `"level":"info"`)
	assert.NotContains(t, optimized, `"level":"warning"`)

	assert.Empty(t, logOf(ReasonBelowThreshold))
}
