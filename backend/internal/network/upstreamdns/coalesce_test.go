package upstreamdns

import (
	"context"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gatedLookup blocks every lookup until release is closed and counts the calls.
type gatedLookup struct {
	release chan struct{}
	calls   atomic.Int32
}

func (g *gatedLookup) lookup(ctx context.Context, _ string) lookupResult {
	g.calls.Add(1)
	select {
	case <-g.release:
		return lookupResult{addrs: []netip.Addr{netip.MustParseAddr("10.0.0.1")}}
	case <-ctx.Done():
		return lookupResult{err: ctx.Err()}
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	require.Eventually(t, cond, 2*time.Second, time.Millisecond)
}

func TestCoalescer_SharesOneLookupPerName(t *testing.T) {
	t.Parallel()
	g := &gatedLookup{release: make(chan struct{})}
	c := newCoalescer(context.Background(), g.lookup, nil, 64, 256)

	var wg sync.WaitGroup
	results := make(chan lookupResult, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, ok := c.resolve("app.test.")
			assert.True(t, ok)
			results <- res
		}()
	}
	waitFor(t, func() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.waiting == 2 })
	close(g.release)
	wg.Wait()
	close(results)
	for res := range results {
		assert.NoError(t, res.err)
	}
	assert.Equal(t, int32(1), g.calls.Load())
}

func TestCoalescer_CapsLookupsAndWaiters(t *testing.T) {
	t.Parallel()
	g := &gatedLookup{release: make(chan struct{})}
	c := newCoalescer(context.Background(), g.lookup, nil, 1, 2)

	done := make(chan struct{}, 2)
	go func() { _, _ = c.resolve("a.test."); done <- struct{}{} }()
	waitFor(t, func() bool { c.mu.Lock(); defer c.mu.Unlock(); return len(c.flights) == 1 })

	_, ok := c.resolve("b.test.")
	assert.False(t, ok, "a second distinct name is dropped while the lookup cap is full")

	go func() { _, _ = c.resolve("a.test."); done <- struct{}{} }()
	waitFor(t, func() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.waiting == 2 })
	_, ok = c.resolve("a.test.")
	assert.False(t, ok, "a third waiting query is dropped at the waiter cap")

	close(g.release)
	<-done
	<-done
	assert.Equal(t, int32(1), g.calls.Load())
}

func TestCoalescer_PanicDropsWaitersAndCallsOnPanic(t *testing.T) {
	t.Parallel()
	var panics atomic.Int32
	c := newCoalescer(context.Background(), func(context.Context, string) lookupResult {
		panic("lookup failed")
	}, func() { panics.Add(1) }, 64, 256)

	_, ok := c.resolve("boom.test.")
	assert.False(t, ok, "a panicking lookup gives the query no reply")
	assert.Equal(t, int32(1), panics.Load())

	c.lookup = func(context.Context, string) lookupResult {
		return lookupResult{addrs: []netip.Addr{netip.MustParseAddr("10.0.0.2")}}
	}
	res, ok := c.resolve("app.test.")
	require.True(t, ok, "the next query is answered")
	assert.Equal(t, netip.MustParseAddr("10.0.0.2"), res.addrs[0])
}

func TestCoalescer_BaseContextCancelDropsWaiters(t *testing.T) {
	t.Parallel()
	g := &gatedLookup{release: make(chan struct{})}
	base, cancel := context.WithCancel(context.Background())
	c := newCoalescer(base, g.lookup, nil, 64, 256)
	done := make(chan bool, 1)
	go func() { _, ok := c.resolve("app.test."); done <- ok }()
	waitFor(t, func() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.waiting == 1 })
	cancel()
	assert.False(t, <-done)
	close(g.release)
}
