package dbmaint

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// harness wires Run against a scratch database that has a real settings table.
type harness struct {
	t      *testing.T
	db     *sql.DB
	path   string
	gate   *Gate
	deps   Deps
	fileID string

	convertCalls    atomic.Int32
	checkpointCalls atomic.Int32
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	db, path := newSettingsDB(t)
	fileID, err := FileID(path)
	require.NoError(t, err)

	h := &harness{t: t, db: db, path: path, gate: NewGate(), fileID: fileID}
	h.gate.MarkPlanned()
	h.gate.MarkListenerBound()
	h.gate.ConfigDone(true)
	h.deps = Deps{
		DB:     db,
		DBPath: path,
		Gate:   h.gate,
		Convert: func(context.Context, *sql.Conn) error {
			h.convertCalls.Add(1)
			return nil
		},
		Checkpoint: func(context.Context, Querier) (bool, error) {
			h.checkpointCalls.Add(1)
			return false, nil
		},
		Probe:    func(context.Context, *sql.Conn) error { return nil },
		Now:      func() time.Time { return time.Date(2026, 10, 2, 4, 11, 0, 0, time.UTC) },
		BootTime: time.Date(2026, 10, 2, 4, 0, 0, 0, time.UTC),
		Timings: Timings{
			PlannedMaxWait:       2 * time.Second,
			QuickCheckMaxWait:    2 * time.Second,
			ConnAcquireTimeout:   time.Second,
			ProbeRetries:         2,
			ProbeRetryGap:        5 * time.Millisecond,
			MarkerWriteTimeout:   2 * time.Second,
			CheckpointRetries:    2,
			CheckpointBackoff:    time.Millisecond,
			CheckpointBackoffCap: 4 * time.Millisecond,
		},
	}
	return h
}

func (h *harness) run(ctx context.Context) Outcome {
	h.t.Helper()
	return Run(ctx, h.deps)
}

func (h *harness) state() State {
	h.t.Helper()
	// Load consumes a leftover marker, so read the raw rows instead where that matters.
	st, err := NewStore(h.db).Load(context.Background(), h.fileID)
	require.NoError(h.t, err)
	return st
}

func (h *harness) markerPresent() bool {
	h.t.Helper()
	found := settingFound(h.t, h.db, keyInProgress)
	return found
}

func (h *harness) attempts() int {
	h.t.Helper()
	return h.state().Attempts
}

func (h *harness) lastResult() *LastResult { return h.state().LastResult }

func TestRun_ConvertsAndReleasesEverything(t *testing.T) {
	h := newHarness(t)

	out := h.run(context.Background())

	assert.Equal(t, ResultConverted, out.Result)
	assert.Equal(t, int32(1), h.convertCalls.Load())
	assert.Equal(t, PhaseDone, h.gate.Snapshot().Phase)
	assert.True(t, h.gate.WaitIdle(context.Background()))
	assert.True(t, h.gate.WaitRunner(time.Millisecond), "the runner signals that it returned")
	assert.False(t, h.markerPresent(), "the in-progress marker is cleared")
	assert.Zero(t, h.attempts())
	last := h.lastResult()
	require.NotNil(t, last)
	assert.Equal(t, ResultConverted, last.Outcome)
	assert.Equal(t, h.fileID, last.FileID)
	assert.True(t, h.deps.Now().Equal(last.At))
}

func TestRun_HoldsThePoolAndWritesTheMarkerWhileConverting(t *testing.T) {
	h := newHarness(t)
	var sawPhase Phase
	var sawActive, markerOnConn bool
	h.deps.Convert = func(ctx context.Context, conn *sql.Conn) error {
		sawPhase, sawActive = h.gate.Snapshot().Phase, h.gate.Active()
		var v string
		markerOnConn = conn.QueryRowContext(ctx, `SELECT value FROM settings WHERE "key" = ?`, keyInProgress).Scan(&v) == nil
		return nil
	}

	h.run(context.Background())

	assert.Equal(t, PhaseConverting, sawPhase)
	assert.True(t, sawActive)
	assert.True(t, markerOnConn, "the marker is written before VACUUM, through the pinned connection")
}

func TestRun_CaddyNotAppliedSkips(t *testing.T) {
	h := newHarness(t)
	h.gate = NewGate()
	h.gate.MarkPlanned()
	h.gate.MarkListenerBound()
	h.gate.ConfigDone(false)
	h.deps.Gate = h.gate

	out := h.run(context.Background())

	assert.Equal(t, ResultSkipped, out.Result)
	assert.Equal(t, ReasonCaddyNotReady, out.Reason)
	assert.Zero(t, h.convertCalls.Load())
	assert.Equal(t, PhaseSkipped, h.gate.Snapshot().Phase)
	assert.True(t, h.gate.WaitIdle(context.Background()))
	last := h.lastResult()
	require.NotNil(t, last)
	assert.Equal(t, ResultSkipped, last.Outcome)
	assert.Equal(t, ReasonCaddyNotReady, last.Reason)
}

func TestRun_StartupTimeoutSkipsAndReleases(t *testing.T) {
	h := newHarness(t)
	h.gate = NewGate()
	h.gate.MarkPlanned()
	h.gate.ConfigDone(true) // the listener never binds
	h.deps.Gate = h.gate
	h.deps.Timings.PlannedMaxWait = 30 * time.Millisecond

	out := h.run(context.Background())

	assert.Equal(t, ReasonStartupTimeout, out.Reason)
	assert.Equal(t, PhaseSkipped, h.gate.Snapshot().Phase)
	assert.True(t, h.gate.WaitIdle(context.Background()))
	assert.Zero(t, h.convertCalls.Load())
}

func TestRun_CancelWhilePlannedIsNotPersisted(t *testing.T) {
	h := newHarness(t)
	h.gate = NewGate()
	h.gate.MarkPlanned() // no readiness signal ever arrives
	h.deps.Gate = h.gate
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan Outcome, 1)
	go func() { done <- h.run(ctx) }()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case out := <-done:
		assert.Equal(t, ResultCancelled, out.Result)
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancel")
	}
	assert.True(t, h.gate.WaitIdle(context.Background()), "the pipeline is released")
	assert.Nil(t, h.lastResult())
}

func TestRun_GateReleasedElsewhereEndsTheRun(t *testing.T) {
	h := newHarness(t)
	h.gate.Release()
	out := h.run(context.Background())
	assert.Equal(t, ResultCancelled, out.Result)
	assert.Zero(t, h.convertCalls.Load())
}

func TestRun_QuickCheckVerdicts(t *testing.T) {
	quick := func(verdict string) QuickCheckFunc {
		done := make(chan struct{})
		close(done)
		return func() (<-chan struct{}, func() string) { return done, func() string { return verdict } }
	}

	t.Run("corruption skips", func(t *testing.T) {
		h := newHarness(t)
		h.deps.QuickCheck = quick("row 1 missing from index")
		out := h.run(context.Background())
		assert.Equal(t, ReasonIntegrityCheckFailed, out.Reason)
		assert.Zero(t, h.convertCalls.Load())
		assert.Equal(t, PhaseSkipped, h.gate.Snapshot().Phase)
	})
	t.Run("ok proceeds", func(t *testing.T) {
		h := newHarness(t)
		h.deps.QuickCheck = quick("ok")
		assert.Equal(t, ResultConverted, h.run(context.Background()).Result)
	})
	t.Run("a check that could not run proceeds", func(t *testing.T) {
		h := newHarness(t)
		h.deps.QuickCheck = quick("")
		assert.Equal(t, ResultConverted, h.run(context.Background()).Result)
	})
	t.Run("unregistered path proceeds", func(t *testing.T) {
		h := newHarness(t)
		h.deps.QuickCheck = func() (<-chan struct{}, func() string) { return nil, nil }
		assert.Equal(t, ResultConverted, h.run(context.Background()).Result)
	})
}

func TestRun_TheUIStaysNormalWhileTheQuickCheckIsPending(t *testing.T) {
	h := newHarness(t)
	release := make(chan struct{})
	h.deps.QuickCheck = func() (<-chan struct{}, func() string) {
		return release, func() string { return "ok" }
	}
	done := make(chan Outcome, 1)
	go func() { done <- h.run(context.Background()) }()

	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, PhasePlanned, h.gate.Snapshot().Phase)
	assert.False(t, h.gate.Active(), "planned serves the UI normally")
	assert.True(t, h.gate.Deferring(), "but the pipeline and scheduled backups wait")

	close(release)
	select {
	case out := <-done:
		assert.Equal(t, ResultConverted, out.Result)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not finish after the quick_check completed")
	}
}

func TestRun_QuickCheckWaitIsSeparateFromThePlannedTimer(t *testing.T) {
	h := newHarness(t)
	h.deps.Timings.PlannedMaxWait = 40 * time.Millisecond
	h.deps.Timings.QuickCheckMaxWait = 250 * time.Millisecond
	h.deps.QuickCheck = func() (<-chan struct{}, func() string) {
		return make(chan struct{}), func() string { return "" } // never completes
	}
	start := time.Now()

	out := h.run(context.Background())

	assert.Equal(t, ResultConverted, out.Result, "both signals arrived, so PlannedMaxWait must not expire")
	assert.GreaterOrEqual(t, time.Since(start), 240*time.Millisecond, "the wait ran to QuickCheckMaxWait")
}

func TestRun_CancelDuringQuickCheckWait(t *testing.T) {
	h := newHarness(t)
	h.deps.QuickCheck = func() (<-chan struct{}, func() string) {
		return make(chan struct{}), func() string { return "" }
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(30 * time.Millisecond); cancel() }()
	assert.Equal(t, ResultCancelled, h.run(ctx).Result)
	assert.True(t, h.gate.WaitIdle(context.Background()))
}

func TestRun_ConnAcquireTimeoutSkipsAsDatabaseBusyAndReleases(t *testing.T) {
	h := newHarness(t)
	h.deps.Timings.ConnAcquireTimeout = 60 * time.Millisecond
	h.deps.Timings.MarkerWriteTimeout = 30 * time.Millisecond
	pinned, err := h.db.Conn(context.Background()) // a leaked connection
	require.NoError(t, err)

	out := h.run(context.Background())

	assert.Equal(t, ResultSkipped, out.Result)
	assert.Equal(t, ReasonDatabaseBusy, out.Reason)
	assert.Zero(t, h.convertCalls.Load())
	assert.Equal(t, PhaseSkipped, h.gate.Snapshot().Phase)
	assert.True(t, h.gate.WaitIdle(context.Background()), "the pipeline is released")

	require.NoError(t, pinned.Close())
	assert.Zero(t, h.attempts(), "a busy database is not a failed attempt")
	require.NoError(t, h.db.PingContext(context.Background()), "pool users proceed")
}

func TestRun_WriterLockProbe(t *testing.T) {
	t.Run("busy after the retries skips", func(t *testing.T) {
		h := newHarness(t)
		var probes atomic.Int32
		h.deps.Probe = func(context.Context, *sql.Conn) error {
			probes.Add(1)
			return ErrWriterBusy
		}
		out := h.run(context.Background())
		assert.Equal(t, ReasonDatabaseBusy, out.Reason)
		assert.Equal(t, 1+h.deps.Timings.ProbeRetries, int(probes.Load()))
		assert.Zero(t, h.convertCalls.Load())
		assert.Zero(t, h.attempts())
		require.NoError(t, h.db.PingContext(context.Background()), "the connection was released")
	})
	t.Run("busy then free converts", func(t *testing.T) {
		h := newHarness(t)
		var probes atomic.Int32
		h.deps.Probe = func(context.Context, *sql.Conn) error {
			if probes.Add(1) == 1 {
				return ErrWriterBusy
			}
			return nil
		}
		assert.Equal(t, ResultConverted, h.run(context.Background()).Result)
	})
	t.Run("another error fails without counting", func(t *testing.T) {
		h := newHarness(t)
		h.deps.Probe = func(context.Context, *sql.Conn) error { return errors.New("disk I/O error") }
		out := h.run(context.Background())
		assert.Equal(t, ResultFailed, out.Result)
		assert.Zero(t, h.convertCalls.Load())
		assert.Zero(t, h.attempts())
		assert.Equal(t, PhaseFailed, h.gate.Snapshot().Phase)
	})
	t.Run("cancel while waiting between probes", func(t *testing.T) {
		h := newHarness(t)
		h.deps.Timings.ProbeRetryGap = time.Minute
		ctx, cancel := context.WithCancel(context.Background())
		h.deps.Probe = func(context.Context, *sql.Conn) error {
			go func() { time.Sleep(20 * time.Millisecond); cancel() }()
			return ErrWriterBusy
		}
		assert.Equal(t, ResultCancelled, h.run(ctx).Result)
		assert.True(t, h.gate.WaitIdle(context.Background()))
	})
}

func TestRun_FailedConversionIsCountedAndClearsTheMarker(t *testing.T) {
	h := newHarness(t)
	h.deps.Convert = func(context.Context, *sql.Conn) error { return errors.New("database or disk is full") }

	out := h.run(context.Background())

	assert.Equal(t, ResultFailed, out.Result)
	require.Error(t, out.Err)
	assert.Equal(t, PhaseFailed, h.gate.Snapshot().Phase)
	assert.False(t, h.markerPresent())
	assert.Equal(t, 1, h.attempts())
	assert.Equal(t, ResultFailed, h.lastResult().Outcome)

	h2 := h
	h2.gate = NewGate()
	h2.gate.MarkPlanned()
	h2.gate.MarkListenerBound()
	h2.gate.ConfigDone(true)
	h2.deps.Gate = h2.gate
	h2.run(context.Background())
	assert.Equal(t, 2, h.attempts(), "consecutive failures accumulate")
}

func TestRun_CancelInterruptsTheRebuild(t *testing.T) {
	h := newHarness(t)
	started := make(chan struct{})
	h.deps.Convert = func(ctx context.Context, _ *sql.Conn) error {
		close(started)
		<-ctx.Done()
		return errors.New("interrupted (9)")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan Outcome, 1)
	go func() { done <- h.run(ctx) }()
	<-started
	cancel()

	var out Outcome
	select {
	case out = <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return")
	}
	assert.Equal(t, ResultInterrupted, out.Result)
	assert.False(t, h.markerPresent(), "the marker is cleared through the detached context")
	assert.Zero(t, h.attempts(), "an interruption is not a failed attempt")
	assert.Equal(t, ResultInterrupted, h.lastResult().Outcome)
	assert.True(t, h.gate.WaitIdle(context.Background()))
	assert.Zero(t, h.checkpointCalls.Load(), "no checkpoint after an interrupted conversion")
}

// A cancel that lands in VACUUM's uninterruptible copy-back tail: the
// conversion completes and Convert returns nil although ctx is cancelled.
func TestRun_NilConvertWithACancelledContextIsConverted(t *testing.T) {
	h := newHarness(t)
	started := make(chan struct{})
	h.deps.Convert = func(ctx context.Context, _ *sql.Conn) error {
		close(started)
		<-ctx.Done()
		time.Sleep(30 * time.Millisecond) // ignores the cancel and finishes
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan Outcome, 1)
	go func() { done <- h.run(ctx) }()
	<-started
	cancel()

	var out Outcome
	select {
	case out = <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return")
	}
	assert.Equal(t, ResultConverted, out.Result, "decided by the Convert result, not by ctx.Err()")
	assert.False(t, h.markerPresent())
	assert.Zero(t, h.attempts())
	assert.Equal(t, ResultConverted, h.lastResult().Outcome)
	assert.Equal(t, int32(1), h.checkpointCalls.Load(), "a single checkpoint attempt")
	assert.Equal(t, PhaseDone, h.gate.Snapshot().Phase)
}

// 2b: the result and the marker clear are written BEFORE the single checkpoint
// attempt, so they survive a checkpoint that never returns within the wait.
func TestRun_CancelledPathWritesResultAndClearsMarkerBeforeTheCheckpoint(t *testing.T) {
	h := newHarness(t)
	started := make(chan struct{})
	h.deps.Convert = func(ctx context.Context, _ *sql.Conn) error {
		close(started)
		<-ctx.Done()
		return nil
	}
	var sawMarkerAtCheckpoint, sawResultAtCheckpoint atomic.Bool
	entered := make(chan struct{})
	release := make(chan struct{})
	h.deps.Checkpoint = func(context.Context, Querier) (bool, error) {
		h.checkpointCalls.Add(1)
		marker := settingFound(t, h.db, keyInProgress)
		result := settingFound(t, h.db, keyLastResult)
		sawMarkerAtCheckpoint.Store(marker)
		sawResultAtCheckpoint.Store(result)
		close(entered)
		<-release // a checkpoint that does not return
		return false, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan Outcome, 1)
	go func() { done <- h.run(ctx) }()
	<-started
	cancel()

	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("the checkpoint was never attempted")
	}
	// Observed from this (second) goroutine while Run is stuck in the checkpoint.
	assert.False(t, h.markerPresent(), "the marker is cleared even though the checkpoint never returned")
	assert.NotNil(t, h.lastResult(), "last_result was written before the checkpoint")
	assert.False(t, sawMarkerAtCheckpoint.Load(), "the marker was already cleared when the checkpoint began")
	assert.True(t, sawResultAtCheckpoint.Load(), "last_result was already written when the checkpoint began")

	close(release)
	out := <-done
	assert.Equal(t, ResultConverted, out.Result)
	assert.Equal(t, int32(1), h.checkpointCalls.Load(), "no retry or backoff on the cancelled path")
}

func TestRun_CancelledPathWithBusyCheckpointIsPendingAndNeverRetries(t *testing.T) {
	h := newHarness(t)
	h.deps.Convert = func(ctx context.Context, _ *sql.Conn) error { <-ctx.Done(); return nil }
	h.deps.Checkpoint = func(context.Context, Querier) (bool, error) {
		h.checkpointCalls.Add(1)
		return true, nil // a reader pins the WAL
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(30 * time.Millisecond); cancel() }()

	out := h.run(ctx)

	assert.Equal(t, ResultConvertedPendingCheckpoint, out.Result)
	assert.Equal(t, int32(1), h.checkpointCalls.Load())
	assert.Equal(t, ResultConvertedPendingCheckpoint, h.lastResult().Outcome)
	assert.False(t, h.markerPresent())
}

func TestRun_CancelledPathCheckpointErrorIsPending(t *testing.T) {
	h := newHarness(t)
	h.deps.Convert = func(ctx context.Context, _ *sql.Conn) error { <-ctx.Done(); return nil }
	h.deps.Checkpoint = func(context.Context, Querier) (bool, error) { return false, errors.New("disk I/O error") }
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(30 * time.Millisecond); cancel() }()
	assert.Equal(t, ResultConvertedPendingCheckpoint, h.run(ctx).Result)
}

func TestRun_NormalPathRetriesABusyCheckpointThenSucceeds(t *testing.T) {
	h := newHarness(t)
	h.deps.Checkpoint = func(context.Context, Querier) (bool, error) {
		return h.checkpointCalls.Add(1) < 3, nil
	}
	out := h.run(context.Background())
	assert.Equal(t, ResultConverted, out.Result)
	assert.Equal(t, int32(3), h.checkpointCalls.Load())
}

func TestRun_NormalPathGivesUpAfterTheBoundedRetries(t *testing.T) {
	h := newHarness(t)
	h.deps.Checkpoint = func(context.Context, Querier) (bool, error) {
		h.checkpointCalls.Add(1)
		return false, errors.New("locked")
	}
	out := h.run(context.Background())
	assert.Equal(t, ResultConvertedPendingCheckpoint, out.Result)
	assert.Equal(t, 1+h.deps.Timings.CheckpointRetries, int(h.checkpointCalls.Load()))
	assert.Equal(t, PhaseDone, h.gate.Snapshot().Phase, "the database is converted; the pruner finishes the shrink")
}

func TestRun_CancelDuringCheckpointBackoffStopsRetrying(t *testing.T) {
	h := newHarness(t)
	h.deps.Timings.CheckpointBackoff = time.Minute
	h.deps.Timings.CheckpointBackoffCap = time.Minute
	ctx, cancel := context.WithCancel(context.Background())
	h.deps.Checkpoint = func(context.Context, Querier) (bool, error) {
		h.checkpointCalls.Add(1)
		go func() { time.Sleep(20 * time.Millisecond); cancel() }()
		return true, nil
	}
	out := h.run(ctx)
	assert.Equal(t, ResultConvertedPendingCheckpoint, out.Result)
	assert.Equal(t, int32(1), h.checkpointCalls.Load())
}

func TestRun_PanicInConvertIsRecoveredAndReleasesTheGate(t *testing.T) {
	h := newHarness(t)
	h.deps.Convert = func(context.Context, *sql.Conn) error { panic("boom") }

	var out Outcome
	require.NotPanics(t, func() { out = h.run(context.Background()) })

	assert.Equal(t, ResultFailed, out.Result)
	assert.Equal(t, ReasonInternalError, out.Reason)
	assert.Equal(t, PhaseFailed, h.gate.Snapshot().Phase)
	assert.True(t, h.gate.WaitIdle(context.Background()))
	assert.True(t, h.gate.WaitRunner(time.Millisecond))
}

func TestRun_MissingConvertFailsWithoutCounting(t *testing.T) {
	h := newHarness(t)
	h.deps.Convert = nil
	out := h.run(context.Background())
	assert.Equal(t, ResultFailed, out.Result)
	assert.ErrorIs(t, out.Err, ErrNoConvert)
	assert.Zero(t, h.attempts())
}

func TestRun_MarkerWriteFailureFailsWithoutConverting(t *testing.T) {
	h := newHarness(t)
	_, err := h.db.Exec("DROP TABLE settings")
	require.NoError(t, err)

	out := h.run(context.Background())

	assert.Equal(t, ResultFailed, out.Result)
	assert.Zero(t, h.convertCalls.Load(), "no VACUUM without a marker")
	assert.Equal(t, PhaseFailed, h.gate.Snapshot().Phase)
}

func TestRun_DefaultsWorkAgainstARealDatabase(t *testing.T) {
	h := newHarness(t)
	var converted atomic.Bool
	out := Run(context.Background(), Deps{
		DB:      h.db,
		DBPath:  h.path,
		Gate:    h.gate,
		Convert: func(context.Context, *sql.Conn) error { converted.Store(true); return nil },
	})
	assert.Equal(t, ResultConverted, out.Result)
	assert.True(t, converted.Load())
	assert.Equal(t, ResultConverted, h.lastResult().Outcome)
}

func TestRun_ReleasesAWaiterQueuedBehindTheGate(t *testing.T) {
	h := newHarness(t)
	var wg sync.WaitGroup
	var released atomic.Bool
	wg.Add(1)
	go func() {
		defer wg.Done()
		if h.gate.WaitIdle(context.Background()) {
			released.Store(true)
		}
	}()
	h.run(context.Background())
	wg.Wait()
	assert.True(t, released.Load())
}

func TestIsWriterBusy(t *testing.T) {
	assert.True(t, isWriterBusy(errors.New("database is locked (5) (SQLITE_BUSY)")))
	assert.True(t, isWriterBusy(errors.New("SQLITE_BUSY")))
	assert.False(t, isWriterBusy(errors.New("disk I/O error")))
	assert.False(t, isWriterBusy(nil))
}

func TestProbeWriterLock(t *testing.T) {
	db, path := newSettingsDB(t)
	conn, err := db.Conn(context.Background())
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	t.Run("free database passes and restores the busy timeout", func(t *testing.T) {
		require.NoError(t, ProbeWriterLock(context.Background(), conn))
		var timeout int
		require.NoError(t, conn.QueryRowContext(context.Background(), "PRAGMA busy_timeout").Scan(&timeout))
		assert.Equal(t, busyTimeoutMillis, timeout)
	})

	other := openScratch(t, path, 0)
	t.Run("a concurrent writer fails the probe", func(t *testing.T) {
		otherConn, err := other.Conn(context.Background())
		require.NoError(t, err)
		_, err = otherConn.ExecContext(context.Background(), "BEGIN IMMEDIATE")
		require.NoError(t, err)
		defer func() {
			_, _ = otherConn.ExecContext(context.Background(), "ROLLBACK")
			_ = otherConn.Close()
		}()

		assert.ErrorIs(t, ProbeWriterLock(context.Background(), conn), ErrWriterBusy)
	})

	t.Run("a concurrent reader does not", func(t *testing.T) {
		otherConn, err := other.Conn(context.Background())
		require.NoError(t, err)
		_, err = otherConn.ExecContext(context.Background(), "BEGIN")
		require.NoError(t, err)
		var n int
		require.NoError(t, otherConn.QueryRowContext(context.Background(), "SELECT count(*) FROM settings").Scan(&n))
		defer func() {
			_, _ = otherConn.ExecContext(context.Background(), "ROLLBACK")
			_ = otherConn.Close()
		}()

		assert.NoError(t, ProbeWriterLock(context.Background(), conn))
	})

	t.Run("a probe inside a transaction is an error, not a busy signal", func(t *testing.T) {
		_, err := conn.ExecContext(context.Background(), "BEGIN")
		require.NoError(t, err)
		defer func() { _, _ = conn.ExecContext(context.Background(), "ROLLBACK") }()

		err = ProbeWriterLock(context.Background(), conn)
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrWriterBusy)
	})

	t.Run("a closed connection is an error", func(t *testing.T) {
		require.NoError(t, conn.Close())
		require.Error(t, ProbeWriterLock(context.Background(), conn))
	})
}

// flagHarness returns a harness whose database has the user's request set.
func flagHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	require.NoError(t, NewStore(h.db).SetFlag(context.Background()))
	return h
}

func (h *harness) flagSet() bool {
	h.t.Helper()
	on, err := NewStore(h.db).FlagRequested(context.Background())
	require.NoError(h.t, err)
	return on
}

// L2: the flag is consumed by a conversion that reached a terminal outcome and
// survives every run that merely skipped or was interrupted.
func TestRun_FlagIsConsumedByAConversionAndByACountedFailure(t *testing.T) {
	t.Run("converted", func(t *testing.T) {
		h := flagHarness(t)
		assert.Equal(t, ResultConverted, h.run(context.Background()).Result)
		assert.False(t, h.flagSet())
	})
	t.Run("converted but the checkpoint stays busy", func(t *testing.T) {
		h := flagHarness(t)
		h.deps.Checkpoint = func(context.Context, Querier) (bool, error) { return true, nil }
		assert.Equal(t, ResultConvertedPendingCheckpoint, h.run(context.Background()).Result)
		assert.False(t, h.flagSet())
	})
	t.Run("failed and counted", func(t *testing.T) {
		h := flagHarness(t)
		h.deps.Convert = func(context.Context, *sql.Conn) error { return errors.New("disk full") }
		assert.Equal(t, ResultFailed, h.run(context.Background()).Result)
		assert.False(t, h.flagSet(), "a counted failure consumed the request; the counter now governs retries")
		assert.Equal(t, 1, h.attempts())
	})
}

func TestRun_FlagSurvivesEverySkipAndInterruption(t *testing.T) {
	t.Run("database busy", func(t *testing.T) {
		h := flagHarness(t)
		h.deps.Probe = func(context.Context, *sql.Conn) error { return ErrWriterBusy }
		out := h.run(context.Background())
		assert.Equal(t, ReasonDatabaseBusy, out.Reason)
		assert.True(t, h.flagSet())
	})
	t.Run("caddy not ready", func(t *testing.T) {
		h := flagHarness(t)
		h.gate.ConfigDone(false)
		assert.Equal(t, ReasonCaddyNotReady, h.run(context.Background()).Reason)
		assert.True(t, h.flagSet())
	})
	t.Run("integrity check failed", func(t *testing.T) {
		h := flagHarness(t)
		h.deps.QuickCheck = func() (<-chan struct{}, func() string) {
			done := make(chan struct{})
			close(done)
			return done, func() string { return "row 1 missing from index" }
		}
		assert.Equal(t, ReasonIntegrityCheckFailed, h.run(context.Background()).Reason)
		assert.True(t, h.flagSet())
	})
	t.Run("interrupted by shutdown", func(t *testing.T) {
		h := flagHarness(t)
		started := make(chan struct{})
		h.deps.Convert = func(ctx context.Context, _ *sql.Conn) error {
			close(started)
			<-ctx.Done()
			return errors.New("interrupted (9)")
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan Outcome, 1)
		go func() { done <- h.run(ctx) }()
		<-started
		cancel()
		assert.Equal(t, ResultInterrupted, (<-done).Result)
		assert.True(t, h.flagSet(), "an interrupted run did not consume the request")
	})
}

// A successful conversion ends the streak of failures.
func TestRun_SuccessResetsTheAttemptCounter(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, NewStore(h.db).RecordFailure(context.Background(), h.fileID))
	require.NoError(t, NewStore(h.db).RecordFailure(context.Background(), h.fileID))

	assert.Equal(t, ResultConverted, h.run(context.Background()).Result)
	assert.Zero(t, h.attempts())
}

// File-size verification: a conversion whose file did not shrink is durable
// but is not reported as converted.
func TestRun_FileThatDidNotShrinkIsConvertedPendingCheckpoint(t *testing.T) {
	h := newHarness(t)
	fillScratch(t, h.db, 600, 100000, 3)                                   // two thirds of a 60 MB file are free
	h.deps.Convert = func(context.Context, *sql.Conn) error { return nil } // "converts" without shrinking

	out := h.run(context.Background())

	assert.Equal(t, ResultConvertedPendingCheckpoint, out.Result)
	assert.Equal(t, PhaseDone, h.gate.Snapshot().Phase)
	assert.Equal(t, ResultConvertedPendingCheckpoint, h.lastResult().Outcome)
	assert.False(t, h.markerPresent())
}

func TestRun_RealConvertShrinksTheFileAndIsVerifiedBySize(t *testing.T) {
	h := newHarness(t)
	fillScratch(t, h.db, 600, 100000, 3)
	h.deps.Convert = Convert
	h.deps.Checkpoint = nil // the real wal_checkpoint(TRUNCATE)
	before := fileSize(t, h.path)

	out := h.run(context.Background())

	require.Equal(t, ResultConverted, out.Result, "err: %v", out.Err)
	assert.Less(t, fileSize(t, h.path), before/2)
	assert.Greater(t, out.BytesBefore, out.BytesAfter)
	assert.EqualValues(t, AutoVacuumIncremental, pragmaInt(t, h.db, "auto_vacuum"))
	integrityOK(t, h.db)
	assert.Zero(t, h.attempts())
	assert.False(t, h.markerPresent())
}

func TestRun_UninspectableDatabaseFailsWithoutConvertingOrCounting(t *testing.T) {
	h := newHarness(t)
	h.deps.DBPath = h.path + ".missing" // Inspect stats the file

	out := h.run(context.Background())

	assert.Equal(t, ResultFailed, out.Result)
	assert.Zero(t, h.convertCalls.Load())
	assert.Equal(t, PhaseFailed, h.gate.Snapshot().Phase)
}

// A shutdown that lands just as the conversion starts (before the marker is
// written) is a stop, not a failure: nothing was converted and nothing counts.
func TestRun_CancelJustBeforeTheConversionStartsIsNotAFailure(t *testing.T) {
	h := flagHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	h.deps.DBPath = h.path + ".missing" // the statistics read fails, as a cancelled statement would
	h.deps.Probe = func(context.Context, *sql.Conn) error {
		cancel()
		return nil
	}

	out := h.run(ctx)

	assert.Equal(t, ResultCancelled, out.Result)
	assert.Equal(t, ReasonShuttingDown, out.Reason)
	assert.Zero(t, h.convertCalls.Load())
	assert.Zero(t, h.attempts())
	assert.False(t, h.markerPresent())
	assert.True(t, h.flagSet())
	assert.True(t, h.gate.WaitIdle(context.Background()))
}
