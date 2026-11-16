package utils

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"

	"github.com/Wikid82/charon/backend/internal/metrics"
)

// TestRecordURLRefusal_MapsEachRefusalToItsLabels describes the refusal recorder: a private address and a metadata
// endpoint also move the SSRF series with their IP type, a DNS failure and any other refusal only move the validation
// series, and every refusal moves the blocked validation series with its reason. The counters are read before and after,
// and the test is not parallel.
func TestRecordURLRefusal_MapsEachRefusalToItsLabels(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		reason    string
		ssrfType  string
		countSSRF bool
	}{
		{"private address", errors.New("connection to private ip addresses is blocked for security (detected: 127.0.0.1)"), "private_ip", "private", true},
		{"metadata endpoint", errors.New("access to cloud metadata endpoints is blocked for security (detected: 169.254.169.254)"), "metadata_endpoint", "metadata", true},
		{"dns failure", errors.New("dns resolution failed for example.invalid: no such host"), "dns_failed", "", false},
		{"other refusal", errors.New("unsupported scheme: ftp"), "validation_failed", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := testutil.ToFloat64(metrics.URLValidationCounter.WithLabelValues("blocked", tt.reason))
			var ssrfBefore float64
			if tt.countSSRF {
				ssrfBefore = testutil.ToFloat64(metrics.SSRFBlockCounter.WithLabelValues(tt.ssrfType))
			}

			got := RecordURLRefusal(tt.err)

			assert.Equal(t, tt.reason, got)
			assert.Equal(t, before+1, testutil.ToFloat64(metrics.URLValidationCounter.WithLabelValues("blocked", tt.reason)))
			if tt.countSSRF {
				assert.Equal(t, ssrfBefore+1, testutil.ToFloat64(metrics.SSRFBlockCounter.WithLabelValues(tt.ssrfType)))
			}
		})
	}
}
