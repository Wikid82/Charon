package handlers

import (
	"sync/atomic"
	"testing"
	"time"
)

// TestWaitForCondition_PassesImmediately tests that waitForCondition
// returns immediately when the condition is already true.
func TestWaitForCondition_PassesImmediately(t *testing.T) {
	start := time.Now()
	waitForCondition(t, 1*time.Second, func() bool {
		return true // Always true
	})
	elapsed := time.Since(start)

	// Should complete almost instantly (allow up to 50ms for overhead)
	if elapsed > 50*time.Millisecond {
		t.Errorf("expected immediate return, took %v", elapsed)
	}
}

// TestWaitForCondition_PassesAfterIterations tests that waitForCondition
// waits and retries until the condition becomes true.
func TestWaitForCondition_PassesAfterIterations(t *testing.T) {
	var counter atomic.Int32

	start := time.Now()
	waitForCondition(t, 500*time.Millisecond, func() bool {
		counter.Add(1)
		return counter.Load() >= 3 // Pass after 3 checks
	})
	elapsed := time.Since(start)

	// Should have taken at least 2 polling intervals (20ms minimum)
	// but complete well before timeout
	if elapsed < 20*time.Millisecond {
		t.Errorf("expected at least 2 iterations (~20ms), took only %v", elapsed)
	}
	if elapsed > 400*time.Millisecond {
		t.Errorf("should complete well before timeout, took %v", elapsed)
	}
	if counter.Load() < 3 {
		t.Errorf("expected at least 3 checks, got %d", counter.Load())
	}
}

// TestWaitForConditionWithInterval_PassesImmediately tests that
// waitForConditionWithInterval returns immediately when condition is true.
func TestWaitForConditionWithInterval_PassesImmediately(t *testing.T) {
	start := time.Now()
	waitForConditionWithInterval(t, 1*time.Second, 50*time.Millisecond, func() bool {
		return true
	})
	elapsed := time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("expected immediate return, took %v", elapsed)
	}
}

// TestWaitForConditionWithInterval_CustomInterval tests that the custom
// interval is respected when polling.
func TestWaitForConditionWithInterval_CustomInterval(t *testing.T) {
	var counter atomic.Int32

	start := time.Now()
	waitForConditionWithInterval(t, 500*time.Millisecond, 30*time.Millisecond, func() bool {
		counter.Add(1)
		return counter.Load() >= 3
	})
	elapsed := time.Since(start)

	// With 30ms interval, 3 checks should take at least 60ms
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected at least ~60ms with 30ms interval, took %v", elapsed)
	}
	if counter.Load() < 3 {
		t.Errorf("expected at least 3 checks, got %d", counter.Load())
	}
}
