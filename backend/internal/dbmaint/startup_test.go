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

func TestStartupPlan_IsIdleWhileConversionIsDisabled(t *testing.T) {
	require.False(t, conversionEnabled, "the production Plan stays idle until the conversion exists")
	db, path := newSettingsDB(t)
	require.NoError(t, db.Close(), "nothing may be read while disabled")

	res, err := StartupPlan(context.Background(), db, path, config.DBCompactAuto)
	require.NoError(t, err)
	assert.False(t, res.Decision.Run)
	assert.Empty(t, res.Decision.Reason)
}

func withConversionEnabled(t *testing.T) {
	t.Helper()
	prev := conversionEnabled
	conversionEnabled = true
	t.Cleanup(func() { conversionEnabled = prev })
}

func TestStartupPlan_ReadsThePersistedState(t *testing.T) {
	withConversionEnabled(t)
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
	withConversionEnabled(t)

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

	t.Run("the unmodified production plan leaves the gate idle and defers nothing", func(t *testing.T) {
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
