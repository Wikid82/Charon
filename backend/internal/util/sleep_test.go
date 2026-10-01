package util

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSleepContext_WaitsForDuration(t *testing.T) {
	start := time.Now()

	require.NoError(t, SleepContext(context.Background(), 30*time.Millisecond))

	assert.GreaterOrEqual(t, time.Since(start), 30*time.Millisecond)
}

func TestSleepContext_ReturnsContextErrorOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()

	err := SleepContext(ctx, time.Hour)

	require.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(start), time.Second)
}
