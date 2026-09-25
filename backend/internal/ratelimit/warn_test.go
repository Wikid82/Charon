package ratelimit

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWarnBudget_CapsAndCountsSuppressed(t *testing.T) {
	clk := newFakeClock()
	w := NewWarnBudget(3, time.Minute, clk.Now)

	for i := 0; i < 3; i++ {
		ok, suppressed := w.Take()
		assert.True(t, ok)
		assert.Zero(t, suppressed)
	}
	for i := 0; i < 5; i++ {
		ok, _ := w.Take()
		assert.False(t, ok)
	}

	clk.Advance(20 * time.Second) // one token back (3 per minute)
	ok, suppressed := w.Take()
	assert.True(t, ok)
	assert.Equal(t, uint64(5), suppressed, "suppressed count is reported once, then reset")

	ok, _ = w.Take()
	assert.False(t, ok)
	clk.Advance(20 * time.Second)
	ok, suppressed = w.Take()
	assert.True(t, ok)
	assert.Equal(t, uint64(1), suppressed)
}

func TestWarnBudget_OncePerInterval(t *testing.T) {
	clk := newFakeClock()
	w := NewWarnBudget(1, 15*time.Minute, clk.Now)
	ok, _ := w.Take()
	assert.True(t, ok)
	clk.Advance(14 * time.Minute)
	ok, _ = w.Take()
	assert.False(t, ok)
	clk.Advance(time.Minute)
	ok, suppressed := w.Take()
	assert.True(t, ok)
	assert.Equal(t, uint64(1), suppressed)
}

func TestWarnBudget_NilClockAndInvalidInputs(t *testing.T) {
	w := NewWarnBudget(0, 0, nil) // clamped to 1 per second
	ok, _ := w.Take()
	assert.True(t, ok)
}

func TestWarnBudget_Concurrent(t *testing.T) {
	clk := newFakeClock()
	w := NewWarnBudget(10, time.Minute, clk.Now)
	var mu sync.Mutex
	allowed := 0
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if ok, _ := w.Take(); ok {
					mu.Lock()
					allowed++
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 10, allowed)
}
