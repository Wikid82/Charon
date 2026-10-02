package dbmaint

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
)

// newEligibleDB builds a database with a real settings table that Decide
// converts: about 120 MB of which 90% is free (over the 100 MB floor and the 20%
// ratio). autoVacuum 2 builds the same database already optimized.
func newEligibleDB(t *testing.T, autoVacuum int) (db *sql.DB, path string) {
	t.Helper()
	db, path = newSettingsDBWith(t, scratchOpts{autoVacuum: autoVacuum})
	fillScratch(t, db, 1200, 100000, 10)
	return db, path
}

func TestProduction_EligibleDatabaseConvertsEndToEnd(t *testing.T) {
	db, path := newEligibleDB(t, 0)
	before := fileSize(t, path)
	require.EqualValues(t, AutoVacuumNone, pragmaInt(t, db, "auto_vacuum"))
	gate := NewGate()
	ctx := context.Background()

	// The unmodified production wiring: no Plan and no Convert injected.
	planned := Start(ctx, StartParams{Gate: gate, DB: db, DBPath: path, EnvMode: config.DBCompactAuto})

	require.True(t, planned, "Plan decided to run")
	assert.Equal(t, PhasePlanned, gate.Snapshot().Phase)
	assert.True(t, gate.Deferring())

	pipelineStarted := make(chan struct{})
	go func() {
		if gate.WaitIdle(ctx) {
			close(pipelineStarted)
		}
	}()
	select {
	case <-pipelineStarted:
		t.Fatal("the pipeline must wait while a conversion is planned")
	case <-time.After(50 * time.Millisecond):
	}

	gate.ConfigDone(true)
	gate.MarkListenerBound()
	select {
	case <-pipelineStarted:
	case <-time.After(30 * time.Second):
		t.Fatal("the pipeline did not start after the conversion")
	}
	require.True(t, gate.WaitRunner(10*time.Second))

	snap := gate.Snapshot()
	assert.Equal(t, PhaseDone, snap.Phase)
	assert.False(t, gate.Active())
	assert.False(t, gate.Deferring())
	assert.Greater(t, snap.BytesBefore, snap.BytesAfter)

	assert.EqualValues(t, AutoVacuumIncremental, pragmaInt(t, db, "auto_vacuum"))
	assert.Less(t, fileSize(t, path), before/4, "the file really shrank")
	integrityOK(t, db)

	fileID, err := FileID(path)
	require.NoError(t, err)
	st, err := NewStore(db).Peek(ctx, fileID)
	require.NoError(t, err)
	require.NotNil(t, st.LastResult)
	assert.Equal(t, ResultConverted, st.LastResult.Outcome)
	assert.False(t, settingFound(t, db, keyInProgress))
	assert.Zero(t, st.Attempts)
}

// Every Plan-level skip leaves the gate idle, the pipeline undeferred and the
// database untouched.
func TestProduction_SkipPathsLeaveTheGateIdle(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name       string
		autoVacuum int
		env        string
		attempts   int
		wantReason Reason
	}{
		{"disabled by env", 0, config.DBCompactOff, 0, ReasonDisabledByEnv},
		{"already optimized (mode 2)", AutoVacuumIncremental, config.DBCompactAuto, 0, ReasonAlreadyOptimized},
		{"too many failures", 0, config.DBCompactAuto, MaxConvertAttempts, ReasonTooManyFailures},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, path := newEligibleDB(t, tc.autoVacuum)
			fileID, err := FileID(path)
			require.NoError(t, err)
			for range tc.attempts {
				require.NoError(t, NewStore(db).RecordFailure(ctx, fileID))
			}
			res, err := StartupPlan(ctx, db, path, tc.env)
			require.NoError(t, err)
			assert.Equal(t, tc.wantReason, res.Decision.Reason)

			gate := NewGate()
			planned := Start(ctx, StartParams{Gate: gate, DB: db, DBPath: path, EnvMode: tc.env})

			assert.False(t, planned)
			assert.Equal(t, PhaseIdle, gate.Snapshot().Phase)
			assert.False(t, gate.Deferring())
			assert.True(t, gate.WaitRunner(time.Millisecond), "no runner exists")
			assert.EqualValues(t, tc.autoVacuum, pragmaInt(t, db, "auto_vacuum"), "the database is untouched")
		})
	}
}

func TestProduction_BelowThresholdAndNoDiskAreIdle(t *testing.T) {
	t.Run("under the thresholds", func(t *testing.T) {
		db, path := newSettingsDB(t) // a tiny database
		res, err := StartupPlan(context.Background(), db, path, config.DBCompactAuto)
		require.NoError(t, err)
		assert.Equal(t, ReasonBelowThreshold, res.Decision.Reason)
	})
	t.Run("insufficient disk is decided from the real free space", func(t *testing.T) {
		// The estimate of an absurdly large database cannot fit anywhere.
		huge := Stats{PageSize: 4096, PageCount: 1 << 40, FreelistCount: 1 << 39, AutoVacuum: 0}
		disk, err := BuildDiskReport(huge, t.TempDir())
		require.NoError(t, err)
		d := Decide(Inputs{EnvMode: config.DBCompactAuto, Stats: huge, Disk: disk})
		assert.Equal(t, ReasonInsufficientDisk, d.Reason)
		assert.Greater(t, d.RequiredBytes, d.AvailableBytes)
	})
}

// L2: a set request does not bypass the failure back-off (the run stops and
// the request is cleared), and the button, which resets the counter before it
// sets the request, gets a conversion that consumes the request and leaves the
// counter at zero.
func TestProduction_FlagHonoursTheBackoffAndTheButtonStartsAFreshCycle(t *testing.T) {
	ctx := context.Background()
	db, path := newEligibleDB(t, 0)
	fileID, err := FileID(path)
	require.NoError(t, err)
	store := NewStore(db)
	for range MaxConvertAttempts {
		require.NoError(t, store.RecordFailure(ctx, fileID))
	}
	require.NoError(t, store.SetFlag(ctx))

	require.False(t, Start(ctx, StartParams{Gate: NewGate(), DB: db, DBPath: path, EnvMode: config.DBCompactAuto}))
	on, err := store.FlagRequested(ctx)
	require.NoError(t, err)
	assert.False(t, on, "the back-off stop cleared the request")
	st, err := store.Peek(ctx, fileID)
	require.NoError(t, err)
	require.NotNil(t, st.LastResult)
	assert.Equal(t, ReasonTooManyFailures, st.LastResult.Reason)

	require.NoError(t, store.ResetAttempts(ctx))
	require.NoError(t, store.SetFlag(ctx))
	gate := NewGate()
	gate.ConfigDone(true)
	gate.MarkListenerBound()

	require.True(t, Start(ctx, StartParams{Gate: gate, DB: db, DBPath: path, EnvMode: config.DBCompactAuto}))
	require.True(t, gate.WaitRunner(30*time.Second))

	assert.Equal(t, PhaseDone, gate.Snapshot().Phase)
	on, err = store.FlagRequested(ctx)
	require.NoError(t, err)
	assert.False(t, on, "the conversion consumed the request")
	st, err = store.Peek(ctx, fileID)
	require.NoError(t, err)
	assert.Zero(t, st.Attempts)
}

// 3.5: another writer makes the real probe fail; the run is skipped, the
// request stays set and the pool is usable again.
func TestRun_RealWriterLockSkipsAsDatabaseBusy(t *testing.T) {
	h := flagHarness(t)
	h.deps.Timings.ProbeRetries = 1
	h.deps.Timings.ProbeRetryGap = 5 * time.Millisecond

	other, err := sql.Open(sqlite.DriverName, h.path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = other.Close() })
	tx, err := other.Begin()
	require.NoError(t, err)
	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS lock_holder (id INTEGER)")
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	// The real BEGIN EXCLUSIVE probe; the other writer lets go after the last
	// probe so the run can record its result.
	var probes int
	h.deps.Probe = func(ctx context.Context, conn *sql.Conn) error {
		err := ProbeWriterLock(ctx, conn)
		require.ErrorIs(t, err, ErrWriterBusy)
		probes++
		if probes > h.deps.Timings.ProbeRetries {
			require.NoError(t, tx.Rollback())
		}
		return err
	}

	out := h.run(context.Background())

	assert.Equal(t, ResultSkipped, out.Result)
	assert.Equal(t, ReasonDatabaseBusy, out.Reason)
	assert.Zero(t, h.convertCalls.Load())
	assert.Equal(t, 2, probes)
	last := h.lastResult()
	require.NotNil(t, last, "the skip was recorded")
	assert.Equal(t, ReasonDatabaseBusy, last.Reason)
	assert.True(t, h.flagSet(), "a skipped run keeps the request")
	assert.Zero(t, h.attempts())
}
