package services

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/security"
	"gorm.io/gorm"
)

// rawCheckResult is the pure outcome of one probe: no persistence, no debounce,
// no notification, no state-map access. The worker pool (uptime_worker_pool.go)
// turns it into a CheckResult / HostCheckResult against the authoritative
// in-memory monState / hostState maps (spec §3.2.1 / §3.3.3).
type rawCheckResult struct {
	Success bool
	Latency int64 // milliseconds
	Message string
}

// heartbeatStatus maps the raw probe outcome onto the "up"/"down" string stored
// verbatim on the heartbeat row.
func (r rawCheckResult) heartbeatStatus() string {
	if r.Success {
		return "up"
	}
	return "down"
}

// uptimeNotifier is the slice of UptimeService the worker pool needs for
// synchronous transition notifications. An interface (not a concrete ref) so
// tests can inject a fake and assert dispatch timing / deadline-bounding
// without a real NotificationService.
type uptimeNotifier interface {
	// NotifyMonitorDown queues a batched down alert. Fast / non-blocking in the
	// real implementation (map insert + timer); ctx accepted for symmetry.
	NotifyMonitorDown(ctx context.Context, monitor models.UptimeMonitor, reason, previousUptime string)
	// NotifyMonitorUp sends a recovery alert. The real implementation performs
	// an external webhook send, so ctx MUST bound it (supervisor change #1).
	NotifyMonitorUp(ctx context.Context, monitor models.UptimeMonitor, downtime string)
}

// uptimeChecker holds everything a probe needs that must NOT be rebuilt per
// call: the shared SSRF-safe keep-alive HTTP client, the shared host dialer,
// a lazy accessor for the Orthrus resolver (bound after construction in
// routes.go), and the notification sink. It performs no DB writes to
// uptime_monitors / uptime_hosts / uptime_heartbeats and never touches the
// pool's debounce maps.
//
// N3 (resolved): the http/https/tcp/orthrus switch here is the single probe
// path. The legacy inline fallback (UptimeService.checkMonitor) no longer
// carries its own switch — it calls probe() directly.
type uptimeChecker struct {
	httpClient     *http.Client
	hostDialer     *net.Dialer
	resolveOrthrus func() orthrusStatusChecker // may be nil; may return nil
	notifier       uptimeNotifier
}

// newUptimeChecker builds the one shared checker for a service: a single
// SSRF-safe keep-alive HTTP client (idleTimeout 30s — spec §3.2.2 / N2), one
// shared host dialer, a lazy Orthrus-resolver accessor (bound after
// construction via SetOrthrusResolver) and the notification sink. Both the
// legacy inline check path (UptimeService.checkMonitor / checkHost) and the
// worker pool use this same instance so there is exactly one connection pool
// and one copy of the probe switch (spec §3.1.5 / N3).
func newUptimeChecker(svc *UptimeService) *uptimeChecker {
	client := network.NewSafeHTTPClient(
		network.WithTimeout(20*time.Second),
		network.WithDialTimeout(3*time.Second),
		network.WithMaxRedirects(0),
		network.WithAllowLocalhost(),
		network.WithAllowRFC1918(),
		network.WithKeepAlive(100, 4, 30*time.Second),
	)
	return &uptimeChecker{
		httpClient:     client,
		hostDialer:     &net.Dialer{Timeout: 3 * time.Second},
		resolveOrthrus: func() orthrusStatusChecker { return svc.orthrusResolver },
		notifier:       svc,
	}
}

// probe runs the monitor's configured check and returns the raw outcome. It is
// the sole probe switch — the Layer-1 ValidateExternalURL call keeps its
// options (double-DNS accepted, spec §3.2.4) and the "401/403 == up but
// protected" allowance — but it constructs nothing (the client is shared) and
// writes nothing. Both the worker pool and the legacy inline
// UptimeService.checkMonitor fallback call straight into here.
func (c *uptimeChecker) probe(ctx context.Context, monitor models.UptimeMonitor) rawCheckResult {
	start := time.Now()
	success := false
	var msg string

	switch strings.ToLower(strings.TrimSpace(monitor.Type)) {
	case "http", "https":
		validatedURL, err := security.ValidateExternalURL(
			monitor.URL,
			// Uptime monitors are an explicit admin-configured feature and
			// commonly target loopback in local/dev setups (and in tests).
			security.WithAllowLocalhost(),
			security.WithAllowHTTP(),
			security.WithTimeout(3*time.Second),
			// Admin-configured uptime monitors may target RFC 1918 private
			// hosts. Link-local (169.254.x.x), cloud metadata, and all other
			// restricted ranges remain blocked at both validation layers.
			security.WithAllowRFC1918(),
		)
		if err != nil {
			msg = fmt.Sprintf("security validation failed: %s", err.Error())
			break
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, validatedURL, http.NoBody)
		if err != nil {
			msg = err.Error()
			break
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			msg = err.Error()
			break
		}
		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				logger.Log().WithError(closeErr).Warn("uptime checker: failed to close response body")
			}
		}()
		// Accept 2xx, 3xx, and 401/403 (protected but up).
		if (resp.StatusCode >= 200 && resp.StatusCode < 400) || resp.StatusCode == 401 || resp.StatusCode == 403 {
			success = true
		}
		msg = fmt.Sprintf("HTTP %d", resp.StatusCode)

	case "tcp":
		// TCP monitors dial the configured host:port directly without URL
		// validation. RFC 1918 addresses are intentionally permitted: TCP
		// monitors are only created for RemoteServer entries, which are
		// admin-configured and whose target is built from trusted fields.
		addr := strings.TrimPrefix(monitor.URL, "tcp://")
		conn, err := c.hostDialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			msg = err.Error()
			break
		}
		if closeErr := conn.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("uptime checker: failed to close tcp connection")
		}
		success = true
		msg = "Connection successful"

	case "orthrus":
		var resolver orthrusStatusChecker
		if c.resolveOrthrus != nil {
			resolver = c.resolveOrthrus()
		}
		switch {
		case resolver == nil:
			msg = "Orthrus subsystem unavailable"
		case monitor.URL == "":
			msg = "Monitor missing agent UUID"
		default:
			if _, ok := resolver.GetProxyAddr(monitor.URL); ok {
				success = true
				msg = "Orthrus session active"
			} else {
				msg = "Orthrus agent not connected"
			}
		}

	default:
		msg = "Unknown monitor type"
	}

	return rawCheckResult{
		Success: success,
		Latency: time.Since(start).Milliseconds(),
		Message: msg,
	}
}

// probeHost performs the host TCP connectivity pre-check: a SINGLE non-blocking
// dial (3s connect timeout via the shared hostDialer) to any one child-monitor
// port — no sleep-retry loop (spec §3.2.3 de-blocking). The consecutive-failure
// debounce is applied by the worker against the authoritative hostState entry,
// so detection semantics are unchanged.
//
// A host with no dialable (non-Orthrus, port-resolvable) child monitors returns
// Success=true so the debounce leaves it alone — matching legacy checkHost,
// which simply returns without touching status in that case.
func (c *uptimeChecker) probeHost(ctx context.Context, host models.UptimeHost, db *gorm.DB) rawCheckResult {
	start := time.Now()

	var monitors []models.UptimeMonitor
	if err := db.WithContext(ctx).Preload("ProxyHost").
		Where("uptime_host_id = ?", host.ID).Find(&monitors).Error; err != nil {
		return rawCheckResult{
			Success: false,
			Latency: time.Since(start).Milliseconds(),
			Message: fmt.Sprintf("host check: load monitors: %v", err),
		}
	}
	if len(monitors) == 0 {
		return rawCheckResult{Success: true, Latency: time.Since(start).Milliseconds(), Message: "no monitors for host"}
	}

	attempted := false
	var lastErr error
	for _, m := range monitors {
		if strings.ToLower(strings.TrimSpace(m.Type)) == "orthrus" {
			continue // Orthrus liveness is per-monitor session state, not a TCP pre-check.
		}

		var port string
		if m.ProxyHost != nil {
			port = fmt.Sprintf("%d", m.ProxyHost.ForwardPort)
		} else {
			port = extractPort(m.URL)
		}
		if port == "" {
			continue
		}

		attempted = true
		addr := net.JoinHostPort(host.Host, port)
		conn, err := c.hostDialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			_ = conn.Close()
			return rawCheckResult{
				Success: true,
				Latency: time.Since(start).Milliseconds(),
				Message: fmt.Sprintf("TCP connection to %s successful", addr),
			}
		}
		lastErr = err
	}

	if !attempted {
		return rawCheckResult{Success: true, Latency: time.Since(start).Milliseconds(), Message: "no dialable ports"}
	}
	return rawCheckResult{
		Success: false,
		Latency: time.Since(start).Milliseconds(),
		Message: fmt.Sprintf("TCP check failed: %v", lastErr),
	}
}
