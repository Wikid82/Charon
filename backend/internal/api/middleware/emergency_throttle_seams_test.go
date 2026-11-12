package middleware

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/ratelimit"
)

func TestMustNewKeyedLimiter_PanicsOnInvalidConfig(t *testing.T) {
	assert.Panics(t, func() {
		mustNewKeyedLimiter(ratelimit.Config{Rate: 0, Burst: 1})
	})
}

func TestEmergencyResetThrottle_AcquireSlotGivesUpWhenRequestContextEnds(t *testing.T) {
	th := NewEmergencyResetThrottle("main", nil, WithEmergencyResetWait(time.Hour))
	// Fill every check slot so the next acquire has to wait.
	for i := 0; i < EmergencyResetConcurrency; i++ {
		th.slots <- struct{}{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	assert.False(t, th.acquireSlot(ctx), "a request whose context ended must not take a slot")
	assert.Less(t, time.Since(start), time.Minute, "the wait must end with the context, not the slot wait")
	assert.Len(t, th.slots, EmergencyResetConcurrency, "no slot may be taken by the abandoned request")
}

func TestEmergencyResetThrottle_ExpiredWindowWithSuppressedRefusalsIsPrunedAndCounted(t *testing.T) {
	clock := newFakeThrottleClock()
	auditor := &recordingAuditor{}
	th := NewEmergencyResetThrottle("main", auditor, WithEmergencyResetClock(clock.now))
	r := throttleRouter(th, failHandler)

	// Exhaust the budget for one address, then refuse it again so that
	// suppressed refusals accumulate in its audit window.
	floodRefusals(t, r, "192.0.2.90:1", EmergencyResetAttemptsPerWindow+3)
	require.Contains(t, th.auditState, ratelimit.ClientKey("192.0.2.90"))

	// Move past two windows, then let a different address trigger the prune.
	// The stale window is dropped while its suppressed count is still pending.
	clock.advance(2 * EmergencyResetWindow)
	floodRefusals(t, r, "198.51.100.90:1", EmergencyResetAttemptsPerWindow+1)

	assert.NotContains(t, th.auditState, ratelimit.ClientKey("192.0.2.90"), "expired windows must be pruned")
	assert.Len(t, auditor.snapshot(), 2, "one row per address for the first refusal of each window")
	assert.Equal(t, http.StatusTooManyRequests, doReset(r, "198.51.100.90:1").Code)
}

func TestAddressGates_IdleGateIsDroppedAfterRelease(t *testing.T) {
	g := newAddressGates()
	release, ok := g.acquire(context.Background(), "k")
	require.True(t, ok)
	assert.Len(t, g.gates, 1)
	release()
	assert.Empty(t, g.gates, "a released gate with no waiters must be deleted")
}

func TestAddressGates_DifferentKeysDoNotBlockEachOther(t *testing.T) {
	g := newAddressGates()
	releaseA, ok := g.acquire(context.Background(), "a")
	require.True(t, ok)
	defer releaseA()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	releaseB, ok := g.acquire(ctx, "b")
	require.True(t, ok, "another address must not wait on a held gate")
	releaseB()
}

func TestAddressGates_WaiterAbandonedByContextLeavesNoGate(t *testing.T) {
	g := newAddressGates()
	release, ok := g.acquire(context.Background(), "k")
	require.True(t, ok)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, ok = g.acquire(ctx, "k")
	assert.False(t, ok, "a waiter whose context ended must not take the gate")
	assert.Len(t, g.gates, 1, "only the holder's gate remains")

	release()
	assert.Empty(t, g.gates)
}
