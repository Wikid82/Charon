package handlers

import (
	"testing"
	"time"
)

// waitForCondition polls a condition until it returns true or timeout expires.
// This is used to replace time.Sleep() calls with event-driven synchronization
// for faster and more reliable tests.
func waitForCondition(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v timeout", timeout)
}

// waitForConditionWithInterval polls a condition with a custom interval.
func waitForConditionWithInterval(t *testing.T, timeout, interval time.Duration, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(interval)
	}
	t.Fatalf("condition not met within %v timeout", timeout)
}
