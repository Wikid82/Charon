package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestMetrics_Register(t *testing.T) {
	// Create a new registry for testing
	reg := prometheus.NewRegistry()

	// Register metrics - should not panic
	assert.NotPanics(t, func() {
		Register(reg)
	})

	// Verify metrics are registered by gathering them
	metrics, err := reg.Gather()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(metrics), 3)

	// Check that our WAF metrics exist
	hasWAFMetrics := 0
	for _, m := range metrics {
		name := m.GetName()
		if name == "charon_waf_requests_total" ||
			name == "charon_waf_blocked_total" ||
			name == "charon_waf_monitored_total" {
			hasWAFMetrics++
		}
	}
	assert.Equal(t, 3, hasWAFMetrics, "All three WAF metrics should be registered")
}

func TestMetrics_Increment(t *testing.T) {
	// Test that increment functions don't panic
	assert.NotPanics(t, func() {
		IncWAFRequest()
	})

	assert.NotPanics(t, func() {
		IncWAFBlocked()
	})

	assert.NotPanics(t, func() {
		IncWAFMonitored()
	})

	// Multiple increments should also not panic
	assert.NotPanics(t, func() {
		IncWAFRequest()
		IncWAFRequest()
		IncWAFBlocked()
		IncWAFMonitored()
		IncWAFMonitored()
		IncWAFMonitored()
	})
}
