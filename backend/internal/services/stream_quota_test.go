package services

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamQuota_PerKeyLimit(t *testing.T) {
	q := NewStreamQuota(2, 10)

	r1, ok := q.Acquire("alice")
	require.True(t, ok)
	_, ok = q.Acquire("alice")
	require.True(t, ok)
	_, ok = q.Acquire("alice")
	assert.False(t, ok, "third stream for one key is refused")

	_, ok = q.Acquire("bob")
	assert.True(t, ok, "another key is not affected by alice's limit")

	r1()
	_, ok = q.Acquire("alice")
	assert.True(t, ok, "releasing a slot frees it for the same key")
}

func TestStreamQuota_TotalLimit(t *testing.T) {
	q := NewStreamQuota(5, 2)

	_, ok := q.Acquire("alice")
	require.True(t, ok)
	_, ok = q.Acquire("bob")
	require.True(t, ok)
	_, ok = q.Acquire("carol")
	assert.False(t, ok, "the total cap applies across keys")
}

func TestStreamQuota_ReleaseIsIdempotent(t *testing.T) {
	q := NewStreamQuota(1, 1)

	release, ok := q.Acquire("alice")
	require.True(t, ok)
	release()
	release()

	_, ok = q.Acquire("alice")
	assert.True(t, ok)
	_, ok = q.Acquire("bob")
	assert.False(t, ok, "a double release must not free a second slot")
}

func TestStreamQuota_RefusedAcquireReleaseIsNoop(t *testing.T) {
	q := NewStreamQuota(1, 1)
	_, ok := q.Acquire("alice")
	require.True(t, ok)

	release, ok := q.Acquire("alice")
	require.False(t, ok)
	assert.NotPanics(t, release)

	_, ok = q.Acquire("bob")
	assert.False(t, ok, "a refused acquire holds no slot, but the total is still full")
}

func TestStreamQuota_NilAdmitsEverything(t *testing.T) {
	var q *StreamQuota
	for range 100 {
		release, ok := q.Acquire("alice")
		require.True(t, ok)
		release()
	}
}

// TestStreamQuota_ConcurrentNeverExceedsLimits checks under -race that concurrent holders never exceed either cap.
func TestStreamQuota_ConcurrentNeverExceedsLimits(t *testing.T) {
	const perKey, total = 3, 5
	q := NewStreamQuota(perKey, total)

	var (
		mu       sync.Mutex
		held     = map[string]int{}
		sum      int
		violated atomic.Bool
		wg       sync.WaitGroup
	)
	for i := range 200 {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			release, ok := q.Acquire(key)
			if !ok {
				return
			}
			mu.Lock()
			held[key]++
			sum++
			if held[key] > perKey || sum > total {
				violated.Store(true)
			}
			mu.Unlock()

			mu.Lock()
			held[key]--
			sum--
			mu.Unlock()
			release()
		}([]string{"alice", "bob"}[i%2])
	}
	wg.Wait()

	assert.False(t, violated.Load(), "concurrent holders exceeded a cap")
	_, ok := q.Acquire("alice")
	assert.True(t, ok, "all slots are returned after the holders finish")
}
