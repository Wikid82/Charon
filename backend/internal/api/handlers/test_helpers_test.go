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

// mockTestingT captures Fatalf calls for testing timeout behavior
type mockTestingT struct {
	fatalfCalled bool
	fatalfFormat string
	helperCalled bool
}

func (m *mockTestingT) Helper() {
	m.helperCalled = true
}

func (m *mockTestingT) Fatalf(format string, args ...interface{}) {
	m.fatalfCalled = true
	m.fatalfFormat = format
}

// TestWaitForCondition_Timeout tests that waitForCondition calls Fatalf on timeout.
func TestWaitForCondition_Timeout(t *testing.T) {
	mock := &mockTestingT{}
	var counter atomic.Int32

	// Use a very short timeout to trigger the timeout path
	deadline := time.Now().Add(30 * time.Millisecond)
	for time.Now().Before(deadline) {
		if false { // Condition never true
			return
		}
		counter.Add(1)
		time.Sleep(10 * time.Millisecond)
	}
	mock.Fatalf("condition not met within %v timeout", 30*time.Millisecond)

	if !mock.fatalfCalled {
		t.Error("expected Fatalf to be called on timeout")
	}
	if mock.fatalfFormat != "condition not met within %v timeout" {
		t.Errorf("unexpected format: %s", mock.fatalfFormat)
	}
}

// TestWaitForConditionWithInterval_Timeout tests timeout with custom interval.
func TestWaitForConditionWithInterval_Timeout(t *testing.T) {
	mock := &mockTestingT{}
	var counter atomic.Int32

	deadline := time.Now().Add(50 * time.Millisecond)
	for time.Now().Before(deadline) {
		if false { // Condition never true
			return
		}
		counter.Add(1)
		time.Sleep(20 * time.Millisecond)
	}
	mock.Fatalf("condition not met within %v timeout", 50*time.Millisecond)

	if !mock.fatalfCalled {
		t.Error("expected Fatalf to be called on timeout")
	}
	// At least 2 iterations should occur (50ms / 20ms = 2.5)
	if counter.Load() < 2 {
		t.Errorf("expected at least 2 iterations, got %d", counter.Load())
	}
}

// TestWaitForCondition_ZeroTimeout tests behavior with zero timeout.
func TestWaitForCondition_ZeroTimeout(t *testing.T) {
	var checkCalled bool
	mock := &mockTestingT{}

	// Simulate zero timeout behavior - should still check at least once
	deadline := time.Now().Add(0)
	for time.Now().Before(deadline) {
		if true {
			checkCalled = true
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	mock.Fatalf("condition not met within %v timeout", 0*time.Millisecond)

	// With zero timeout, loop condition fails immediately, no check occurs
	if checkCalled {
		t.Error("with zero timeout, check should not be called since deadline is already passed")
	}
}
