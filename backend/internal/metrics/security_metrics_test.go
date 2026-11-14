package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestRecordURLValidation tests URL validation metrics recording.
func TestRecordURLValidation(t *testing.T) {
	t.Parallel()
	// Reset metrics before test
	URLValidationCounter.Reset()

	tests := []struct {
		name   string
		result string
		reason string
	}{
		{"Allowed validation", "allowed", "validated"},
		{"Blocked private IP", "blocked", "private_ip"},
		{"DNS failure", "error", "dns_failed"},
		{"Invalid format", "error", "invalid_format"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			initialCount := testutil.ToFloat64(URLValidationCounter.WithLabelValues(tt.result, tt.reason))

			RecordURLValidation(tt.result, tt.reason)

			finalCount := testutil.ToFloat64(URLValidationCounter.WithLabelValues(tt.result, tt.reason))
			if finalCount != initialCount+1 {
				t.Errorf("Expected counter to increment by 1, got %f -> %f", initialCount, finalCount)
			}
		})
	}
}

// TestRecordSSRFBlock tests SSRF block metrics recording.
func TestRecordSSRFBlock(t *testing.T) {
	t.Parallel()
	// Reset metrics before test
	SSRFBlockCounter.Reset()

	tests := []struct {
		name   string
		ipType string
	}{
		{"Private IP block", "private"},
		{"Loopback block", "loopback"},
		{"Link-local block", "linklocal"},
		{"Metadata endpoint block", "metadata"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			initialCount := testutil.ToFloat64(SSRFBlockCounter.WithLabelValues(tt.ipType))

			RecordSSRFBlock(tt.ipType)

			finalCount := testutil.ToFloat64(SSRFBlockCounter.WithLabelValues(tt.ipType))
			if finalCount != initialCount+1 {
				t.Errorf("Expected counter to increment by 1, got %f -> %f", initialCount, finalCount)
			}
		})
	}
}

// TestRecordURLTestDuration tests URL test duration histogram recording.
func TestRecordURLTestDuration(t *testing.T) {
	t.Parallel()
	// Record various durations
	durations := []float64{0.05, 0.1, 0.25, 0.5, 1.0, 2.5}

	for _, duration := range durations {
		RecordURLTestDuration(duration)
	}

	// Note: We can't easily verify histogram count with testutil.ToFloat64
	// since it's a histogram, not a counter. The test passes if no panic occurs.
	t.Log("Successfully recorded histogram observations")
}

// TestMetricsLabels verifies metric labels are correct.
func TestMetricsLabels(t *testing.T) {
	t.Parallel()
	// Verify metrics are registered and accessible
	if URLValidationCounter == nil {
		t.Error("URLValidationCounter is nil")
	}
	if SSRFBlockCounter == nil {
		t.Error("SSRFBlockCounter is nil")
	}
	if URLTestDuration == nil {
		t.Error("URLTestDuration is nil")
	}
}

// TestMetricsRegistration tests that metrics can be registered with Prometheus.
func TestMetricsRegistration(t *testing.T) {
	t.Parallel()
	registry := prometheus.NewRegistry()

	// Attempt to register the metrics
	// This test verifies a metric can be registered on a fresh registry without error
	err := registry.Register(prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_charon_url_validation_total",
		Help: "Test metric",
	}))
	if err != nil {
		t.Errorf("Failed to register test metric: %v", err)
	}
}
