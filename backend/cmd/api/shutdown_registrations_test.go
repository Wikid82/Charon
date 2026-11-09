package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingRegistrationWaiter records the deadline it was given and returns err.
type recordingRegistrationWaiter struct {
	deadline time.Time
	hasDL    bool
	err      error
}

func (w *recordingRegistrationWaiter) WaitRegistrations(ctx context.Context) error {
	w.deadline, w.hasDL = ctx.Deadline()
	return w.err
}

func TestDrainRegistrations_IsBoundedAtThirtySeconds(t *testing.T) {
	w := &recordingRegistrationWaiter{}

	drainRegistrations(w)

	require.True(t, w.hasDL)
	remaining := time.Until(w.deadline)
	assert.LessOrEqual(t, remaining, 30*time.Second)
	assert.Greater(t, remaining, 29*time.Second)
}

func TestDrainRegistrations_NoLifecycleIsANoop(t *testing.T) {
	assert.NotPanics(t, func() { drainRegistrations(nil) })
}

func TestDrainRegistrations_AFailedWaitIsLoggedNotFatal(t *testing.T) {
	w := &recordingRegistrationWaiter{err: context.DeadlineExceeded}
	assert.NotPanics(t, func() { drainRegistrations(w) })
}

func TestRunConcurrently_StepsStartBeforeEitherFinishes(t *testing.T) {
	aStarted, bStarted := make(chan struct{}), make(chan struct{})
	timeout := time.After(2 * time.Second)
	waitFor := func(ch <-chan struct{}) bool {
		select {
		case <-ch:
			return true
		case <-timeout:
			return false
		}
	}
	var aSawB, bSawA bool

	// Each step waits for the other to start: sequential execution would never finish.
	runConcurrently(
		func() { close(aStarted); aSawB = waitFor(bStarted) },
		func() { close(bStarted); bSawA = waitFor(aStarted) },
	)

	assert.True(t, aSawB, "step a ran while step b was running")
	assert.True(t, bSawA, "step b ran while step a was running")
}

// TestRunConcurrently_ReturnsOnlyAfterEveryStep holds two steps on channels. The helper must not return while either
// is still blocked; the release order is driven by the test, so no sleep is involved.
func TestRunConcurrently_ReturnsOnlyAfterEveryStep(t *testing.T) {
	releaseA, releaseC := make(chan struct{}), make(chan struct{})
	returned := make(chan struct{})
	go func() {
		runConcurrently(
			func() { <-releaseA },
			func() {},
			func() { <-releaseC },
		)
		close(returned)
	}()

	close(releaseA)
	select {
	case <-returned:
		t.Fatal("runConcurrently returned while a step was still running")
	default:
	}

	close(releaseC)
	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("runConcurrently did not return after every step finished")
	}
}

func TestRunConcurrently_NoStepsReturns(_ *testing.T) {
	runConcurrently()
}
