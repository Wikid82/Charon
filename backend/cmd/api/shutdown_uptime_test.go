package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDrainUptime_NilPipelineIsANoop(t *testing.T) {
	assert.NotPanics(t, func() { drainUptime(nil, time.Second) })
}

func TestDrainUptime_RunsTheDrainUnderTheGivenDeadline(t *testing.T) {
	var deadline time.Time
	var hasDeadline bool
	drainUptime(func(ctx context.Context) error {
		deadline, hasDeadline = ctx.Deadline()
		return nil
	}, 10*time.Second)

	require.True(t, hasDeadline)
	remaining := time.Until(deadline)
	assert.LessOrEqual(t, remaining, 10*time.Second)
	assert.Greater(t, remaining, 9*time.Second)
}

func TestDrainUptime_AFailedDrainIsLoggedNotReturned(t *testing.T) {
	called := false
	assert.NotPanics(t, func() {
		drainUptime(func(context.Context) error {
			called = true
			return errors.New("drain stopped early")
		}, time.Second)
	})
	assert.True(t, called, "the drain runs even when it reports an error")
}
