package dbmaint

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGate_NewGateIsIdle(t *testing.T) {
	g := NewGate()
	snap := g.Snapshot()
	assert.Equal(t, PhaseIdle, snap.Phase)
	assert.False(t, g.Deferring())
	assert.False(t, g.Active())
	assert.True(t, g.WaitIdle(context.Background()))
	assert.True(t, g.WaitRunner(time.Millisecond), "no runner is expected on an idle gate")
}

func TestGate_NilGateBehavesAsIdle(t *testing.T) {
	var g *Gate
	g.MarkPlanned()
	g.ConfigDone(true)
	g.MarkListenerBound()
	g.Release()
	g.RunnerExited()
	assert.Equal(t, PhaseIdle, g.Snapshot().Phase)
	assert.False(t, g.Deferring())
	assert.False(t, g.Active())
	assert.True(t, g.WaitIdle(context.Background()))
	assert.True(t, g.WaitReleased(context.Background()))
	assert.True(t, g.WaitRunner(time.Millisecond))
	assert.NoError(t, g.BeginChecking())
}

func TestGate_PhaseFlagsFollowTheTable(t *testing.T) {
	cases := []struct {
		phase     Phase
		deferring bool
		active    bool
	}{
		{PhaseIdle, false, false},
		{PhasePlanned, true, false},
		{PhaseChecking, true, true},
		{PhaseConverting, true, true},
		{PhaseDone, false, false},
		{PhaseSkipped, false, false},
		{PhaseFailed, false, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.phase), func(t *testing.T) {
			g := NewGate()
			g.phase = tc.phase
			assert.Equal(t, tc.deferring, g.Deferring())
			assert.Equal(t, tc.active, g.Active())
		})
	}
}

func TestGate_MarkPlannedOnlyFromIdle(t *testing.T) {
	g := NewGate()
	g.MarkPlanned()
	require.Equal(t, PhasePlanned, g.Snapshot().Phase)

	g.Finish(FinishInfo{Phase: PhaseDone})
	g.MarkPlanned()
	assert.Equal(t, PhaseDone, g.Snapshot().Phase, "a finished gate is never re-planned")
}

func TestGate_WaitIdleBlocksUntilReleased(t *testing.T) {
	g := NewGate()
	g.MarkPlanned()

	got := make(chan bool, 1)
	go func() { got <- g.WaitIdle(context.Background()) }()

	select {
	case <-got:
		t.Fatal("WaitIdle returned while the gate was planned")
	case <-time.After(50 * time.Millisecond):
	}

	g.Release()
	select {
	case ok := <-got:
		assert.True(t, ok)
	case <-time.After(time.Second):
		t.Fatal("WaitIdle did not return after Release")
	}
}

func TestGate_WaitIdleReturnsFalseWhenContextIsCancelled(t *testing.T) {
	g := NewGate()
	g.MarkPlanned()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.False(t, g.WaitIdle(ctx))
	assert.False(t, g.WaitReleased(ctx))
}

func TestGate_WaitReleasedReturnsAfterEveryTerminalOutcome(t *testing.T) {
	for _, phase := range []Phase{PhaseDone, PhaseSkipped, PhaseFailed} {
		t.Run(string(phase), func(t *testing.T) {
			g := NewGate()
			g.MarkPlanned()
			go func() {
				time.Sleep(20 * time.Millisecond)
				g.Finish(FinishInfo{Phase: phase})
			}()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			assert.True(t, g.WaitReleased(ctx))
			assert.Equal(t, phase, g.Snapshot().Phase)
		})
	}
}

func TestGate_ReleaseIsIdempotentAndKeepsTheFirstOutcome(t *testing.T) {
	g := NewGate()
	g.MarkPlanned()
	g.Finish(FinishInfo{Phase: PhaseFailed, Reason: "boom", Message: "it broke"})
	g.Release()
	g.Finish(FinishInfo{Phase: PhaseDone})

	snap := g.Snapshot()
	assert.Equal(t, PhaseFailed, snap.Phase)
	assert.Equal(t, Reason("boom"), snap.Reason)
	assert.Equal(t, "it broke", snap.Message)
}

func TestGate_ReleaseSkipsAStillDeferringGate(t *testing.T) {
	g := NewGate()
	g.MarkPlanned()
	g.Release()
	assert.Equal(t, PhaseSkipped, g.Snapshot().Phase)
	assert.False(t, g.Deferring())
}

func TestGate_AwaitReady(t *testing.T) {
	t.Run("both signals", func(t *testing.T) {
		g := NewGate()
		g.MarkPlanned()
		g.MarkListenerBound()
		g.ConfigDone(true)
		assert.NoError(t, g.AwaitReady(context.Background(), time.Second))
	})

	t.Run("signals arriving later", func(t *testing.T) {
		g := NewGate()
		g.MarkPlanned()
		go func() {
			time.Sleep(10 * time.Millisecond)
			g.ConfigDone(true)
			time.Sleep(10 * time.Millisecond)
			g.MarkListenerBound()
		}()
		assert.NoError(t, g.AwaitReady(context.Background(), 5*time.Second))
	})

	t.Run("config not applied", func(t *testing.T) {
		g := NewGate()
		g.MarkPlanned()
		g.MarkListenerBound()
		g.ConfigDone(false)
		assert.ErrorIs(t, g.AwaitReady(context.Background(), time.Second), ErrConfigNotApplied)
	})

	t.Run("timeout with a signal missing", func(t *testing.T) {
		g := NewGate()
		g.MarkPlanned()
		g.ConfigDone(true)
		assert.ErrorIs(t, g.AwaitReady(context.Background(), 20*time.Millisecond), ErrStartupTimeout)
	})

	t.Run("context cancelled", func(t *testing.T) {
		g := NewGate()
		g.MarkPlanned()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		assert.ErrorIs(t, g.AwaitReady(ctx, time.Second), context.Canceled)
	})

	t.Run("gate no longer planned", func(t *testing.T) {
		g := NewGate()
		g.Release()
		assert.ErrorIs(t, g.AwaitReady(context.Background(), time.Second), ErrNotPlanned)
	})
}

func TestGate_LifecycleSnapshot(t *testing.T) {
	clock := time.Date(2026, 10, 1, 4, 0, 0, 0, time.UTC)
	g := NewGate()
	g.now = func() time.Time { return clock }

	g.MarkPlanned()
	require.NoError(t, g.BeginChecking())
	assert.Equal(t, PhaseChecking, g.Snapshot().Phase)
	assert.True(t, g.Active())

	clock = clock.Add(3 * time.Second)
	require.NoError(t, g.BeginConverting(1000))
	clock = clock.Add(7 * time.Second)
	snap := g.Snapshot()
	assert.Equal(t, PhaseConverting, snap.Phase)
	assert.Equal(t, int64(1000), snap.BytesBefore)
	assert.Equal(t, int64(10), snap.ElapsedSeconds, "elapsed counts from the moment the pool was taken")

	g.Finish(FinishInfo{Phase: PhaseDone, BytesAfter: 400, Message: "done"})
	clock = clock.Add(time.Hour)
	snap = g.Snapshot()
	assert.Equal(t, PhaseDone, snap.Phase)
	assert.Equal(t, int64(400), snap.BytesAfter)
	assert.Equal(t, int64(10), snap.ElapsedSeconds, "elapsed is frozen when the gate finishes")
}

func TestGate_BeginRequiresAPlannedOrCheckingGate(t *testing.T) {
	g := NewGate()
	assert.ErrorIs(t, g.BeginChecking(), ErrNotPlanned)
	assert.ErrorIs(t, g.BeginConverting(1), ErrNotPlanned)
}

func TestGate_RunnerWait(t *testing.T) {
	g := NewGate()
	g.MarkPlanned()
	assert.False(t, g.WaitRunner(20*time.Millisecond), "the runner has not exited")

	go func() {
		time.Sleep(10 * time.Millisecond)
		g.RunnerExited()
		g.RunnerExited() // idempotent
	}()
	assert.True(t, g.WaitRunner(time.Second))
}
