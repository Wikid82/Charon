// Package metrics provides security-specific Prometheus metrics for monitoring SSRF protection.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// URLValidationCounter tracks all URL validation attempts with their results.
	// Labels:
	// - result: "allowed", "blocked", "error"
	// - reason: specific validation failure reason (e.g., "private_ip", "invalid_format", "dns_failed")
	URLValidationCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "charon_url_validation_total",
			Help: "Total number of URL validation attempts by result and reason",
		},
		[]string{"result", "reason"},
	)

	// SSRFBlockCounter tracks blocked SSRF attempts by IP type.
	// Labels:
	// - ip_type: "private", "loopback", "linklocal", "reserved", "metadata", "ipv6_mapped"
	// - user_id: user identifier who attempted the request (for audit trail)
	SSRFBlockCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "charon_ssrf_blocks_total",
			Help: "Total number of SSRF attempts blocked by IP type and user",
		},
		[]string{"ip_type", "user_id"},
	)

	// URLTestDuration tracks the time taken for URL connectivity tests.
	// Buckets are optimized for network latency (10ms to 10s)
	URLTestDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "charon_url_test_duration_seconds",
			Help:    "Duration of URL connectivity tests in seconds",
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
	)
)

// RecordURLValidation records a URL validation attempt.
func RecordURLValidation(result, reason string) {
	URLValidationCounter.WithLabelValues(result, reason).Inc()
}

// RecordSSRFBlock records a blocked SSRF attempt.
func RecordSSRFBlock(ipType, userID string) {
	SSRFBlockCounter.WithLabelValues(ipType, userID).Inc()
}

// RecordURLTestDuration records the duration of a URL test.
func RecordURLTestDuration(durationSeconds float64) {
	URLTestDuration.Observe(durationSeconds)
}
