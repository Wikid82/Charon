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

func TestRunConcurrently_ReturnsOnlyAfterEveryStep(t *testing.T) {
	done := make([]bool, 3)
	runConcurrently(
		func() { time.Sleep(20 * time.Millisecond); done[0] = true },
		func() { done[1] = true },
		func() { time.Sleep(5 * time.Millisecond); done[2] = true },
	)
	assert.Equal(t, []bool{true, true, true}, done)
}

func TestRunConcurrently_NoStepsReturns(_ *testing.T) {
	runConcurrently()
}
