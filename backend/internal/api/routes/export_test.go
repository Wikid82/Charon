package routes

import "testing"

// IsolatedMemoryDSN exposes isolatedMemoryDSN to the external routes_test package.
func IsolatedMemoryDSN(t *testing.T) string {
	return isolatedMemoryDSN(t)
}
