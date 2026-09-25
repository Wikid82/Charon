// Package metrics provides Prometheus metrics collectors for the application.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
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
	wafMonitoredTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "charon_waf_monitored_total",
		Help: "Total number of requests monitored (not blocked) by WAF",
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

// Register registers Prometheus collectors. Call once at startup.
func Register(registry *prometheus.Registry) {
	registry.MustRegister(wafRequestsTotal, wafBlockedTotal, wafMonitoredTotal, crowdsecRequestsTotal, crowdsecBlockedTotal, authRateLimitedTotal)
}

// IncWAFRequest increments the evaluated requests counter.
func IncWAFRequest() { wafRequestsTotal.Inc() }

// IncWAFBlocked increments the blocked requests counter.
func IncWAFBlocked() { wafBlockedTotal.Inc() }

// IncWAFMonitored increments the monitored requests counter.
func IncWAFMonitored() { wafMonitoredTotal.Inc() }

// IncCrowdSecRequest increments the CrowdSec evaluated requests counter.
func IncCrowdSecRequest() { crowdsecRequestsTotal.Inc() }

// IncCrowdSecBlocked increments the CrowdSec blocked requests counter.
func IncCrowdSecBlocked() { crowdsecBlockedTotal.Inc() }

// IncAuthRateLimited increments the sign-in throttle rejection counter for class.
func IncAuthRateLimited(class string) { authRateLimitedTotal.WithLabelValues(class).Inc() }

// AuthRateLimitedCounter returns the rejection counter for class (for delta assertions in tests).
func AuthRateLimitedCounter(class string) prometheus.Counter {
	return authRateLimitedTotal.WithLabelValues(class)
}
