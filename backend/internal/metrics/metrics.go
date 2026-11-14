// Package metrics provides Prometheus metrics collectors for the application.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Blocked-entry sources the access log records for the two layers that count blocks.
const (
	blockedSourceWAF      = "waf"
	blockedSourceCrowdSec = "crowdsec"
)

var (
	wafRequestsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "charon_waf_requests_total",
		Help: "Total number of requests evaluated by WAF",
	})
	wafBlockedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "charon_waf_blocked_total",
		Help: "Total number of requests blocked by WAF",
	})
	crowdsecRequestsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "charon_crowdsec_requests_total",
		Help: "Total number of requests evaluated by CrowdSec bouncer",
	})
	crowdsecBlockedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "charon_crowdsec_blocked_total",
		Help: "Total number of requests blocked by CrowdSec decisions",
	})
	// authRateLimitedTotal counts sign-in throttle rejections by class. It carries no
	// client labels because /metrics is unauthenticated.
	authRateLimitedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "charon_auth_rate_limited_total",
		Help: "Total number of requests rejected by the sign-in throttle, by class",
	}, []string{"class"})
)

// Register registers every Prometheus collector this package serves on registry. Call once per registry at startup.
func Register(registry *prometheus.Registry) {
	registry.MustRegister(
		wafRequestsTotal, wafBlockedTotal, crowdsecRequestsTotal, crowdsecBlockedTotal, authRateLimitedTotal,
		URLValidationCounter, SSRFBlockCounter, URLTestDuration,
	)
}

// IncWAFRequest increments the evaluated requests counter.
func IncWAFRequest() { wafRequestsTotal.Inc() }

// IncWAFBlocked increments the blocked requests counter.
func IncWAFBlocked() { wafBlockedTotal.Inc() }

// IncCrowdSecRequest increments the CrowdSec evaluated requests counter.
func IncCrowdSecRequest() { crowdsecRequestsTotal.Inc() }

// IncCrowdSecBlocked increments the CrowdSec blocked requests counter.
func IncCrowdSecBlocked() { crowdsecBlockedTotal.Inc() }

// RecordBlockedEntry counts one access-log entry that was blocked, by the layer that blocked it. Proxied traffic never
// reaches the in-process middleware, so the access log is the only place these blocks are seen. Entries from other
// layers are not counted here.
func RecordBlockedEntry(source string) {
	switch source {
	case blockedSourceWAF:
		IncWAFBlocked()
	case blockedSourceCrowdSec:
		IncCrowdSecBlocked()
	}
}

// IncAuthRateLimited increments the sign-in throttle rejection counter for class.
func IncAuthRateLimited(class string) { authRateLimitedTotal.WithLabelValues(class).Inc() }

// AuthRateLimitedCounter returns the rejection counter for class.
//
// It exists only so tests, including those in other packages (a _test.go
// helper is not importable across packages), can assert counter deltas.
// Production code must use IncAuthRateLimited.
func AuthRateLimitedCounter(class string) prometheus.Counter {
	return authRateLimitedTotal.WithLabelValues(class)
}

// WAFBlockedCounter and CrowdSecBlockedCounter return the blocked-request counters. Like AuthRateLimitedCounter, they
// exist so tests in other packages can assert counter deltas; production code uses RecordBlockedEntry or the Inc
// functions.
func WAFBlockedCounter() prometheus.Counter { return wafBlockedTotal }

// CrowdSecBlockedCounter returns the CrowdSec blocked-request counter (see WAFBlockedCounter).
func CrowdSecBlockedCounter() prometheus.Counter { return crowdsecBlockedTotal }
