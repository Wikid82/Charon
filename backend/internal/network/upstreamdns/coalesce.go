package upstreamdns

import (
	"context"
	"sync"
)

// flight is one lookup in progress. Queries for the same name wait on done.
type flight struct {
	done     chan struct{}
	res      lookupResult
	panicked bool
}

// coalescer shares lookups between queries for the same name and caps the work:
// at most maxLookups distinct names in flight and at most maxWaiting queries waiting.
// A query over a cap is dropped, and the client retries.
type coalescer struct {
	base       context.Context
	lookup     func(ctx context.Context, name string) lookupResult
	onPanic    func()
	maxLookups int
	maxWaiting int

	mu      sync.Mutex
	flights map[string]*flight
	waiting int
}

func newCoalescer(base context.Context, lookup func(context.Context, string) lookupResult, onPanic func(), maxLookups, maxWaiting int) *coalescer {
	return &coalescer{
		base:       base,
		lookup:     lookup,
		onPanic:    onPanic,
		maxLookups: maxLookups,
		maxWaiting: maxWaiting,
		flights:    make(map[string]*flight),
	}
}

// resolve returns the lookup result for a rooted, case-folded name. accepted is false
// when the query is dropped: over a cap, the server is closing, or the lookup panicked.
func (c *coalescer) resolve(name string) (res lookupResult, accepted bool) {
	c.mu.Lock()
	if c.waiting >= c.maxWaiting {
		c.mu.Unlock()
		return lookupResult{}, false
	}
	f, shared := c.flights[name]
	if !shared {
		if len(c.flights) >= c.maxLookups {
			c.mu.Unlock()
			return lookupResult{}, false
		}
		f = &flight{done: make(chan struct{})}
		c.flights[name] = f
		go c.run(name, f)
	}
	c.waiting++
	c.mu.Unlock()
	defer c.leave()

	select {
	case <-f.done:
		if f.panicked {
			return lookupResult{}, false
		}
		return f.res, true
	case <-c.base.Done():
		return lookupResult{}, false
	}
}

func (c *coalescer) leave() {
	c.mu.Lock()
	c.waiting--
	c.mu.Unlock()
}

// run performs one lookup and wakes its waiters. A panic in the lookup is recovered
// here, because it happens outside any query goroutine: the waiters get no reply.
func (c *coalescer) run(name string, f *flight) {
	defer func() {
		if recover() != nil {
			f.panicked = true
			if c.onPanic != nil {
				c.onPanic()
			}
		}
		c.mu.Lock()
		delete(c.flights, name)
		c.mu.Unlock()
		close(f.done)
	}()
	f.res = c.lookup(c.base, name)
}
