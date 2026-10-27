package handlers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type proxyApplyKey struct{}

func TestApplyProxyConfigDetached_CancelledParentStillApplies(t *testing.T) {
	applier := &recordingApplier{}

	err := applyProxyConfigDetached(cancelledRequestContext(), applier, proxyApplyTimeout)

	require.NoError(t, err)
	requireDetachedApply(t, applier)
}

func TestApplyProxyConfigDetached_KeepsRequestValues(t *testing.T) {
	var seenValue any
	apply := applyFunc(func(ctx context.Context) error {
		seenValue = ctx.Value(proxyApplyKey{})
		return nil
	})
	parent := context.WithValue(cancelledRequestContext(), proxyApplyKey{}, "request-scoped")

	require.NoError(t, applyProxyConfigDetached(parent, apply, proxyApplyTimeout))
	assert.Equal(t, "request-scoped", seenValue)
}

func TestApplyProxyConfigDetached_DeadlineFollowsTimeout(t *testing.T) {
	var deadline time.Time
	apply := applyFunc(func(ctx context.Context) error {
		d, ok := ctx.Deadline()
		require.True(t, ok, "the apply must carry a deadline")
		deadline = d
		return nil
	})

	before := time.Now()
	require.NoError(t, applyProxyConfigDetached(context.Background(), apply, 5*time.Second))
	assert.WithinDuration(t, before.Add(5*time.Second), deadline, 2*time.Second)
}

func TestApplyProxyConfigDetached_ReturnsApplyError(t *testing.T) {
	wantErr := errors.New("apply failed")
	apply := applyFunc(func(context.Context) error { return wantErr })

	err := applyProxyConfigDetached(context.Background(), apply, proxyApplyTimeout)

	require.ErrorIs(t, err, wantErr)
}

// applyFunc adapts a function to CaddyConfigManager for one-off apply behavior.
type applyFunc func(ctx context.Context) error

func (f applyFunc) ApplyConfig(ctx context.Context) error { return f(ctx) }
