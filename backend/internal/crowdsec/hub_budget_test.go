package crowdsec

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// remainingWithin reports the time left until ctx's deadline and fails the test when the deadline is absent.
func remainingWithin(t *testing.T, ctx context.Context) time.Duration {
	t.Helper()
	deadline, ok := ctx.Deadline()
	require.True(t, ok, "attempt context must carry a deadline")
	return time.Until(deadline)
}

func TestAttemptContextWithoutDeadlineReturnsParent(t *testing.T) {
	t.Parallel()
	parent := context.Background()
	ctx, cancel := attemptContext(parent, 2)
	require.Equal(t, parent, ctx)
	_, ok := ctx.Deadline()
	require.False(t, ok)
	cancel() // no-op: the caller keeps the parent alive
	require.NoError(t, ctx.Err())
}

func TestAttemptContextSharesRemainingTime(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		attemptsLeft int
		min, max     time.Duration
	}{
		{name: "single attempt takes all", attemptsLeft: 1, min: 29 * time.Second, max: 30 * time.Second},
		{name: "two attempts take half", attemptsLeft: 2, min: 14 * time.Second, max: 15 * time.Second},
		{name: "three attempts take a third", attemptsLeft: 3, min: 9 * time.Second, max: 10 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Each case owns its parent, created when the case runs, so the window is not spent by the
			// scheduling of the other cases.
			parent, cancelParent := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancelParent()
			ctx, cancel := attemptContext(parent, tc.attemptsLeft)
			defer cancel()
			left := remainingWithin(t, ctx)
			require.GreaterOrEqual(t, left, tc.min)
			require.LessOrEqual(t, left, tc.max)
		})
	}
}

func TestAttemptContextFloorApplies(t *testing.T) {
	t.Parallel()
	parent, cancelParent := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelParent()

	// 10 s over 4 attempts is 2.5 s, below the 5 s floor, so the attempt gets the floor.
	ctx, cancel := attemptContext(parent, 4)
	defer cancel()
	left := remainingWithin(t, ctx)
	require.Greater(t, left, 4*time.Second)
	require.LessOrEqual(t, left, minHubAttemptBudget)
}

func TestAttemptContextCapsAtRemainingBelowFloor(t *testing.T) {
	t.Parallel()
	parent, cancelParent := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelParent()

	// Only 2 s remain, so a 5 s floor cannot be honoured: the attempt gets what is left.
	ctx, cancel := attemptContext(parent, 1)
	defer cancel()
	left := remainingWithin(t, ctx)
	require.Greater(t, left, 1500*time.Millisecond)
	require.LessOrEqual(t, left, 2*time.Second)
}

func TestAttemptContextNonPositiveAttemptsCountsAsOne(t *testing.T) {
	t.Parallel()
	parent, cancelParent := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelParent()

	for _, attemptsLeft := range []int{0, -3} {
		ctx, cancel := attemptContext(parent, attemptsLeft)
		left := remainingWithin(t, ctx)
		require.Greater(t, left, 9*time.Second)
		require.LessOrEqual(t, left, 10*time.Second)
		cancel()
	}
}

func TestAttemptContextExpiredParent(t *testing.T) {
	t.Parallel()
	parent, cancelParent := context.WithTimeout(context.Background(), 0)
	defer cancelParent()

	ctx, cancel := attemptContext(parent, 2)
	defer cancel()
	require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
}

func TestAttemptContextCancelledParentStaysCancelled(t *testing.T) {
	t.Parallel()
	parent, cancelParent := context.WithTimeout(context.Background(), time.Minute)
	cancelParent()

	ctx, cancel := attemptContext(parent, 1)
	defer cancel()
	require.ErrorIs(t, ctx.Err(), context.Canceled)
}

func TestAttemptContextCancelReleasesChild(t *testing.T) {
	t.Parallel()
	parent, cancelParent := context.WithTimeout(context.Background(), time.Minute)
	defer cancelParent()

	ctx, cancel := attemptContext(parent, 2)
	cancel()
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	require.NoError(t, parent.Err(), "cancelling an attempt must not cancel its parent")
}

func TestHubIndexOffline(t *testing.T) {
	t.Parallel()
	require.False(t, HubIndex{}.Offline())
	require.True(t, HubIndex{offline: true}.Offline())
}
