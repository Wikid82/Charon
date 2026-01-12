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

	// Increment each metric at least once so they appear in Gather()
	IncWAFRequest()
	IncWAFBlocked()
	IncWAFMonitored()
	IncCrowdSecRequest()
	IncCrowdSecBlocked()

	// Verify metrics are registered by gathering them
	metrics, err := reg.Gather()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(metrics), 5)

	// Check that our WAF and CrowdSec metrics exist
	expectedMetrics := map[string]bool{
		"charon_waf_requests_total":      false,
		"charon_waf_blocked_total":       false,
		"charon_waf_monitored_total":     false,
		"charon_crowdsec_requests_total": false,
		"charon_crowdsec_blocked_total":  false,
	}

	for _, m := range metrics {
		name := m.GetName()
		if _, ok := expectedMetrics[name]; ok {
			expectedMetrics[name] = true
		}
	}

	for name, found := range expectedMetrics {
		assert.True(t, found, "Metric %s should be registered", name)
	}
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

	assert.NotPanics(t, func() {
		IncCrowdSecRequest()
	})

	assert.NotPanics(t, func() {
		IncCrowdSecBlocked()
	})

	// Multiple increments should also not panic
	assert.NotPanics(t, func() {
		IncWAFRequest()
		IncWAFRequest()
		IncWAFBlocked()
		IncWAFMonitored()
		IncWAFMonitored()
		IncWAFMonitored()
		IncCrowdSecRequest()
		IncCrowdSecBlocked()
	})
}
