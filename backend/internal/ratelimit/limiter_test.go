package ratelimit

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// fakeClock is a manually advanced clock for deterministic limiter tests.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)}
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

func newTestLimiter(t *testing.T, requests int, window time.Duration, maxKeys int, clk *fakeClock) *KeyedLimiter {
	t.Helper()
	r, b := PerWindow(requests, window)
	k, err := NewKeyedLimiter(Config{Rate: r, Burst: b, MaxKeys: maxKeys, Now: clk.Now})
	require.NoError(t, err)
	return k
}

func TestKeyedLimiter_BurstThenDeny(t *testing.T) {
	clk := newFakeClock()
	k := newTestLimiter(t, 10, 600*time.Second, 0, clk)

	for i := 0; i < 10; i++ {
		d := k.Allow("198.51.100.1")
		require.True(t, d.Allowed, "request %d should be allowed", i+1)
		assert.Zero(t, d.RetryAfter)
		assert.False(t, d.FirstDenial)
	}

	d := k.Allow("198.51.100.1")
	assert.False(t, d.Allowed)
	assert.True(t, d.FirstDenial)
	assert.Equal(t, 60, RetryAfterSeconds(d.RetryAfter))

	d = k.Allow("198.51.100.1")
	assert.False(t, d.Allowed)
	assert.False(t, d.FirstDenial, "second denial is part of the same episode")
}

func TestKeyedLimiter_RefillAfterInterval(t *testing.T) {
	clk := newFakeClock()
	k := newTestLimiter(t, 10, 600*time.Second, 0, clk)
	for i := 0; i < 10; i++ {
		require.True(t, k.Allow("a").Allowed)
	}
	require.False(t, k.Allow("a").Allowed)

	clk.Advance(59 * time.Second)
	d := k.Allow("a")
	require.False(t, d.Allowed)
	assert.Equal(t, 1, RetryAfterSeconds(d.RetryAfter))

	clk.Advance(1 * time.Second)
	d = k.Allow("a")
	assert.True(t, d.Allowed, "one token refills after 60s")
	assert.False(t, k.Allow("a").Allowed)

	// A new denial after an allowed request starts a new episode.
	clk.Advance(60 * time.Second)
	require.True(t, k.Allow("a").Allowed)
	d = k.Allow("a")
	assert.False(t, d.Allowed)
	assert.True(t, d.FirstDenial)
}

func TestKeyedLimiter_DenialsDoNotConsume(t *testing.T) {
	clk := newFakeClock()
	k := newTestLimiter(t, 10, 600*time.Second, 0, clk)
	for i := 0; i < 10; i++ {
		require.True(t, k.Allow("a").Allowed)
	}
	// Retrying early many times must not push the wait further out.
	for i := 0; i < 50; i++ {
		clk.Advance(time.Second)
		require.False(t, k.Allow("a").Allowed)
	}
	clk.Advance(10 * time.Second) // 60s since exhaustion
	assert.True(t, k.Allow("a").Allowed)
}

func TestKeyedLimiter_IndependentKeys(t *testing.T) {
	clk := newFakeClock()
	k := newTestLimiter(t, 2, time.Minute, 0, clk)
	require.True(t, k.Allow("a").Allowed)
	require.True(t, k.Allow("a").Allowed)
	require.False(t, k.Allow("a").Allowed)
	assert.True(t, k.Allow("b").Allowed)
	assert.True(t, k.Allow("b").Allowed)
	assert.False(t, k.Allow("b").Allowed)
	assert.Equal(t, 2, k.Len())
}

func TestKeyedLimiter_MaxKeysHardCap(t *testing.T) {
	clk := newFakeClock()
	k := newTestLimiter(t, 10, time.Minute, 100, clk)
	for i := 0; i < 10_000; i++ {
		k.Allow(fmt.Sprintf("key-%d", i))
		require.LessOrEqual(t, k.Len(), 100)
	}
	assert.Equal(t, 100, k.Len())
}

func TestKeyedLimiter_DefaultMaxKeys(t *testing.T) {
	k, err := NewKeyedLimiter(Config{Rate: 1, Burst: 1})
	require.NoError(t, err)
	assert.Equal(t, DefaultMaxKeys, k.maxKeys)
}

func TestKeyedLimiter_SweepEvictsOnlyIdleFullBuckets(t *testing.T) {
	clk := newFakeClock()
	// idleTTL = burst/rate = 60s; sweep interval = min(60s, 1m) = 60s.
	k := newTestLimiter(t, 10, time.Minute, 0, clk)
	k.Allow("old")
	clk.Advance(30 * time.Second)
	k.Allow("recent")
	clk.Advance(31 * time.Second) // old idle 61s, recent idle 31s

	k.Allow("trigger") // runs the sweep
	assert.Equal(t, 2, k.Len())
	_, ok := k.lru.Peek("old")
	assert.False(t, ok, "fully refilled idle bucket is swept")
	_, ok = k.lru.Peek("recent")
	assert.True(t, ok)
}

func TestKeyedLimiter_SweepKeepsRecentlyDepletedKey(t *testing.T) {
	clk := newFakeClock()
	k := newTestLimiter(t, 10, 600*time.Second, 0, clk) // idleTTL 600s, sweep every 60s
	for i := 0; i < 10; i++ {
		require.True(t, k.Allow("attacker").Allowed)
	}
	require.False(t, k.Allow("attacker").Allowed)

	for i := 0; i < 5; i++ {
		clk.Advance(61 * time.Second)
		k.Allow(fmt.Sprintf("other-%d", i)) // triggers sweeps
	}
	_, ok := k.lru.Peek("attacker")
	require.True(t, ok, "depleted bucket must survive the sweep")
	// 305s elapsed → 5 tokens refilled, not a full reset to 10.
	allowed := 0
	for i := 0; i < 10; i++ {
		if k.Allow("attacker").Allowed {
			allowed++
		}
	}
	assert.Equal(t, 5, allowed)
}

func TestKeyedLimiter_SweepHonorsIdleTTL(t *testing.T) {
	clk := newFakeClock()
	k := newTestLimiter(t, 1, time.Second, 0, clk) // idleTTL 1s, sweep every 1s
	k.Allow("a")
	clk.Advance(500 * time.Millisecond)
	k.Allow("b")
	assert.Equal(t, 2, k.Len())
	clk.Advance(600 * time.Millisecond) // a idle 1.1s, b idle 0.6s
	k.Allow("c")
	assert.Equal(t, 2, k.Len())
}

func TestKeyedLimiter_StartsNoGoroutines(t *testing.T) {
	before := runtime.NumGoroutine()
	for i := 0; i < 100; i++ {
		k := MustNewKeyedLimiter(Config{Rate: 1, Burst: 1})
		k.Allow("a")
	}
	assert.LessOrEqual(t, runtime.NumGoroutine(), before, "limiters must not start goroutines")
}

func TestKeyedLimiter_ConcurrentSameKeyExactBurst(t *testing.T) {
	clk := newFakeClock() // frozen
	k := newTestLimiter(t, 10, 600*time.Second, 0, clk)
	var allowed atomic.Int64
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				if k.Allow("same").Allowed {
					allowed.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(10), allowed.Load())
}

func TestKeyedLimiter_ConcurrentAllowAndReconfigure(t *testing.T) {
	clk := newFakeClock()
	k := newTestLimiter(t, 10, time.Minute, 50, clk)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				k.Allow(fmt.Sprintf("k-%d-%d", g, i%20))
				if i%50 == 0 {
					assert.NoError(t, k.Reconfigure(rate.Limit(float64(g+1)), g+1))
				}
			}
		}(g)
	}
	wg.Wait()
	assert.LessOrEqual(t, k.Len(), 50)
}

func TestKeyedLimiter_ReconfigureResetsOnlyOnChange(t *testing.T) {
	clk := newFakeClock()
	k := newTestLimiter(t, 2, time.Minute, 0, clk)
	require.True(t, k.Allow("a").Allowed)
	require.True(t, k.Allow("a").Allowed)
	require.False(t, k.Allow("a").Allowed)

	r, b := PerWindow(2, time.Minute)
	require.NoError(t, k.Reconfigure(r, b))
	assert.False(t, k.Allow("a").Allowed, "unchanged config must not reset buckets")

	require.NoError(t, k.Reconfigure(r, 3))
	assert.Equal(t, 0, k.Len())
	for i := 0; i < 3; i++ {
		assert.True(t, k.Allow("a").Allowed)
	}
	assert.False(t, k.Allow("a").Allowed)

	assert.Error(t, k.Reconfigure(0, 1))
	assert.Error(t, k.Reconfigure(1, 0))
}

func TestNewKeyedLimiter_RejectsInvalidConfig(t *testing.T) {
	cases := map[string]Config{
		"zero rate":     {Rate: 0, Burst: 1},
		"negative rate": {Rate: -1, Burst: 1},
		"inf rate":      {Rate: rate.Inf, Burst: 1},
		"nan rate":      {Rate: rate.Limit(math.NaN()), Burst: 1},
		"zero burst":    {Rate: 1, Burst: 0},
		"neg max keys":  {Rate: 1, Burst: 1, MaxKeys: -1},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			k, err := NewKeyedLimiter(cfg)
			assert.Error(t, err)
			assert.Nil(t, k)
		})
	}
}

func TestMustNewKeyedLimiter_PanicsOnInvalidConfig(t *testing.T) {
	assert.Panics(t, func() { MustNewKeyedLimiter(Config{}) })
}

func TestKeyedLimiter_VerySlowRateDoesNotOverflow(t *testing.T) {
	clk := newFakeClock()
	k, err := NewKeyedLimiter(Config{Rate: rate.Limit(1e-12), Burst: 1_000_000, Now: clk.Now})
	require.NoError(t, err)
	assert.Equal(t, time.Duration(math.MaxInt64), k.idleTTL)
	assert.True(t, k.Allow("a").Allowed)
}

func TestPerWindow(t *testing.T) {
	r, b := PerWindow(10, 600*time.Second)
	assert.InDelta(t, 1.0/60.0, float64(r), 1e-12)
	assert.Equal(t, 10, b)

	r, b = PerWindow(7, 100*time.Second)
	assert.InDelta(t, 0.07, float64(r), 1e-12)
	assert.Equal(t, 7, b)

	r, b = PerWindow(60, time.Minute)
	assert.InDelta(t, 1.0, float64(r), 1e-12)
	assert.Equal(t, 60, b)
}

func TestRetryAfterSeconds(t *testing.T) {
	cases := []struct {
		name string
		d    time.Duration
		want int
	}{
		{"10 per 600s", secondsPerToken(10.0 / 600.0), 60},
		{"float noise just under 60s", 60*time.Second - 300*time.Microsecond, 60},
		{"60 per 60s", time.Second, 1},
		{"7 per 100s", secondsPerToken(0.07), 15},
		{"sub-millisecond", 200 * time.Microsecond, 1},
		{"zero", 0, 1},
		{"negative", -time.Second, 1},
		{"just over a second", 1001 * time.Millisecond, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, RetryAfterSeconds(tc.d))
		})
	}
}

// secondsPerToken returns the refill time of one token at ratePerSec, computed
// at run time with float64 math like the limiter itself.
func secondsPerToken(ratePerSec float64) time.Duration {
	return time.Duration(float64(time.Second) / ratePerSec)
}

func TestPerWindow_NonPositiveWindowIsRejected(t *testing.T) {
	r, b := PerWindow(10, 0)
	_, err := NewKeyedLimiter(Config{Rate: r, Burst: b})
	assert.Error(t, err)
}
