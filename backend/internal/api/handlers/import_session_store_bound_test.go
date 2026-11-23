package handlers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A store built with no room still holds one entry, and storing into it returns instead of looping.
func TestImportSessionStoreWithoutRoomStillHoldsOneEntry(t *testing.T) {
	t.Parallel()
	for _, bound := range []int{0, -3} {
		store := newImportSessionStore[int](time.Hour, bound)
		done := make(chan struct{})
		go func() {
			store.Put("first", 1)
			store.Put("second", 2)
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("Put with a bound of %d did not return", bound)
		}
		_, firstHeld := store.Get("first")
		second, secondHeld := store.Get("second")
		assert.False(t, firstHeld, "the older entry is evicted to make room")
		require.True(t, secondHeld)
		assert.Equal(t, 2, second)
	}
}
