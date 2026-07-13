package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOAuthStateStore_IssueConsume_HappyPath proves a freshly issued state
// resolves back to the exact targetUUID/provider it was bound to (spec R3).
func TestOAuthStateStore_IssueConsume_HappyPath(t *testing.T) {
	store := NewOAuthStateStore()

	state, err := store.Issue("target-uuid-1", "dropbox")
	require.NoError(t, err)
	require.NotEmpty(t, state)

	targetUUID, provider, ok := store.Consume(state)
	require.True(t, ok)
	assert.Equal(t, "target-uuid-1", targetUUID)
	assert.Equal(t, "dropbox", provider)
}

// TestOAuthStateStore_Consume_ReplayRejected proves a state can only ever
// be consumed once — the second Consume of the same token must fail even
// though the first succeeded within its TTL (spec R3 "single-use").
func TestOAuthStateStore_Consume_ReplayRejected(t *testing.T) {
	store := NewOAuthStateStore()

	state, err := store.Issue("target-uuid-1", "google_drive")
	require.NoError(t, err)

	_, _, ok := store.Consume(state)
	require.True(t, ok, "first consume must succeed")

	_, _, ok = store.Consume(state)
	assert.False(t, ok, "replaying an already-consumed state must be rejected")
}

// TestOAuthStateStore_Consume_UnknownStateRejected proves a state this
// store never issued (forged/guessed) is rejected.
func TestOAuthStateStore_Consume_UnknownStateRejected(t *testing.T) {
	store := NewOAuthStateStore()

	_, _, ok := store.Consume("never-issued-by-this-store")
	assert.False(t, ok)
}

// TestOAuthStateStore_Consume_ExpiredStateRejected proves a state past its
// TTL is rejected even though it was legitimately issued (spec R3
// "time-bound (10 min)"). Reaches into the unexported entry directly
// (same-package white-box test) to simulate the passage of time without a
// real 10-minute sleep.
func TestOAuthStateStore_Consume_ExpiredStateRejected(t *testing.T) {
	store := NewOAuthStateStore()

	state, err := store.Issue("target-uuid-1", "dropbox")
	require.NoError(t, err)

	// Force the entry into the past instead of sleeping 10 real minutes.
	store.mu.Lock()
	entry := store.states[state]
	entry.expiresAt = time.Now().Add(-time.Second)
	store.states[state] = entry
	store.mu.Unlock()

	_, _, ok := store.Consume(state)
	assert.False(t, ok, "an expired state must be rejected even if otherwise valid")
}

// TestOAuthStateStore_Issue_SweepsExpiredEntriesOpportunistically proves
// Issue's opportunistic sweep (spec §3.5) actually removes stale entries
// rather than letting the map grow unbounded.
func TestOAuthStateStore_Issue_SweepsExpiredEntriesOpportunistically(t *testing.T) {
	store := NewOAuthStateStore()

	staleState, err := store.Issue("target-uuid-1", "dropbox")
	require.NoError(t, err)

	store.mu.Lock()
	entry := store.states[staleState]
	entry.expiresAt = time.Now().Add(-time.Second)
	store.states[staleState] = entry
	require.Len(t, store.states, 1)
	store.mu.Unlock()

	_, err = store.Issue("target-uuid-2", "google_drive")
	require.NoError(t, err)

	store.mu.Lock()
	_, staleStillPresent := store.states[staleState]
	remaining := len(store.states)
	store.mu.Unlock()

	assert.False(t, staleStillPresent, "the expired entry must be swept on the next Issue call")
	assert.Equal(t, 1, remaining, "only the freshly issued state should remain after the sweep")
}

// TestOAuthStateStore_Issue_TokensAreUnique proves successive Issue calls
// never collide (crypto/rand-backed, spec §3.5).
func TestOAuthStateStore_Issue_TokensAreUnique(t *testing.T) {
	store := NewOAuthStateStore()

	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		state, err := store.Issue("target-uuid", "dropbox")
		require.NoError(t, err)
		require.False(t, seen[state], "Issue must never produce a duplicate state token")
		seen[state] = true
	}
}
