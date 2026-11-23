package handlers

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// fakeClock is a settable clock for the session store tests.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTestSessionStore(max int) (*importSessionStore[string], *fakeClock) {
	clock := &fakeClock{now: time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)}
	store := newImportSessionStore[string](importSessionTTL, max)
	store.now = clock.Now
	return store, clock
}

func TestImportSessionStoreReturnsAStoredValue(t *testing.T) {
	t.Parallel()
	store, _ := newTestSessionStore(maxImportSessions)
	store.Put("a", "first")

	v, ok := store.Get("a")

	assert.True(t, ok)
	assert.Equal(t, "first", v)
}

func TestImportSessionStoreExpiresAfterTheTTL(t *testing.T) {
	t.Parallel()
	store, clock := newTestSessionStore(maxImportSessions)
	store.Put("a", "first")

	clock.Advance(importSessionTTL - time.Second)
	_, ok := store.Get("a")
	assert.True(t, ok, "still available just before the TTL")

	clock.Advance(2 * time.Second)
	_, ok = store.Get("a")
	assert.False(t, ok, "expired after the TTL")
}

func TestImportSessionStoreSweepsExpiredEntriesOnPut(t *testing.T) {
	t.Parallel()
	store, clock := newTestSessionStore(maxImportSessions)
	store.Put("old", "x")
	clock.Advance(importSessionTTL + time.Minute)

	store.Put("new", "y")

	assert.Len(t, store.entries, 1, "the expired entry was swept")
	_, ok := store.Get("new")
	assert.True(t, ok)
}

func TestImportSessionStoreEvictsTheOldestWhenFull(t *testing.T) {
	t.Parallel()
	store, clock := newTestSessionStore(2)
	store.Put("first", "1")
	clock.Advance(time.Second)
	store.Put("second", "2")
	clock.Advance(time.Second)

	store.Put("third", "3")

	_, ok := store.Get("first")
	assert.False(t, ok, "the oldest entry is evicted")
	_, ok = store.Get("second")
	assert.True(t, ok)
	_, ok = store.Get("third")
	assert.True(t, ok)
}

func TestImportSessionStoreBoundsTheEntryCount(t *testing.T) {
	t.Parallel()
	store, clock := newTestSessionStore(maxImportSessions)
	for i := range 10 {
		store.Put(fmt.Sprintf("s%d", i), "v")
		clock.Advance(time.Second)
	}
	assert.Len(t, store.entries, maxImportSessions)
	_, ok := store.Get("s9")
	assert.True(t, ok, "the newest entry is kept")
}

func TestImportSessionStorePutReplacesAnEntryWithoutEvicting(t *testing.T) {
	t.Parallel()
	store, _ := newTestSessionStore(2)
	store.Put("a", "1")
	store.Put("b", "2")

	store.Put("a", "1b")

	assert.Len(t, store.entries, 2)
	v, ok := store.Get("a")
	assert.True(t, ok)
	assert.Equal(t, "1b", v)
}

func TestImportSessionStoreDelete(t *testing.T) {
	t.Parallel()
	store, _ := newTestSessionStore(maxImportSessions)
	store.Put("a", "1")

	store.Delete("a")
	store.Delete("missing")

	_, ok := store.Get("a")
	assert.False(t, ok)
}

// Concurrent readers and writers never corrupt the store (run with -race).
func TestImportSessionStoreIsSafeForConcurrentUse(t *testing.T) {
	t.Parallel()
	store := newImportSessionStore[int](importSessionTTL, maxImportSessions)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("g%d", n%3)
			for j := range 50 {
				store.Put(id, j)
				store.Get(id)
				if j%7 == 0 {
					store.Delete(id)
				}
			}
		}(i)
	}
	wg.Wait()
	assert.LessOrEqual(t, len(store.entries), maxImportSessions)
}
