package services

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testChecker() *uptimeChecker {
	return &uptimeChecker{
		httpClient: network.NewSafeHTTPClient(
			network.WithTimeout(3*time.Second),
			network.WithDialTimeout(2*time.Second),
			network.WithMaxRedirects(0),
			network.WithAllowLocalhost(),
			network.WithAllowRFC1918(),
			network.WithKeepAlive(10, 4, 30*time.Second),
		),
		hostDialer: &net.Dialer{Timeout: 2 * time.Second},
	}
}

// --- probe: HTTP / HTTPS ---

func TestUptimeChecker_Probe_HTTP(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		code    int
		success bool
	}{
		{"200 OK", http.StatusOK, true},
		{"301 redirect status", http.StatusMovedPermanently, true},
		{"401 protected but up", http.StatusUnauthorized, true},
		{"403 protected but up", http.StatusForbidden, true},
		{"500 server error", http.StatusInternalServerError, false},
		{"404 not found", http.StatusNotFound, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.code)
			}))
			defer srv.Close()

			c := testChecker()
			got := c.probe(context.Background(), models.UptimeMonitor{Type: "http", URL: srv.URL})
			assert.Equal(t, tc.success, got.Success)
			assert.Equal(t, fmt.Sprintf("HTTP %d", tc.code), got.Message)
			assert.GreaterOrEqual(t, got.Latency, int64(0))
		})
	}
}

func TestUptimeChecker_Probe_HTTPS_TypeUsesSamePath(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := testChecker()
	// Type "https" exercises the same http/https switch arm; the URL scheme is
	// what ValidateExternalURL + the client actually act on.
	got := c.probe(context.Background(), models.UptimeMonitor{Type: "https", URL: srv.URL})
	assert.True(t, got.Success)
	assert.Equal(t, "HTTP 200", got.Message)
}

func TestUptimeChecker_Probe_HTTP_SecurityValidationFailure(t *testing.T) {
	t.Parallel()
	c := testChecker()
	got := c.probe(context.Background(), models.UptimeMonitor{Type: "http", URL: "http://169.254.169.254/latest/meta-data/"})
	assert.False(t, got.Success)
	assert.Contains(t, got.Message, "security validation failed")
}

func TestUptimeChecker_Probe_HTTP_ConnectionError(t *testing.T) {
	t.Parallel()
	c := testChecker()
	// Nothing listening on this loopback port.
	got := c.probe(context.Background(), models.UptimeMonitor{Type: "http", URL: "http://127.0.0.1:9"})
	assert.False(t, got.Success)
	assert.NotEmpty(t, got.Message)
}

// --- probe: TCP ---

func TestUptimeChecker_Probe_TCP_Success(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	c := testChecker()
	got := c.probe(context.Background(), models.UptimeMonitor{Type: "tcp", URL: ln.Addr().String()})
	assert.True(t, got.Success)
	assert.Equal(t, "Connection successful", got.Message)
}

func TestUptimeChecker_Probe_TCP_StripsSchemePrefix(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	c := testChecker()
	got := c.probe(context.Background(), models.UptimeMonitor{Type: "tcp", URL: "tcp://" + ln.Addr().String()})
	assert.True(t, got.Success)
}

func TestUptimeChecker_Probe_TCP_Refused(t *testing.T) {
	t.Parallel()
	c := testChecker()
	got := c.probe(context.Background(), models.UptimeMonitor{Type: "tcp", URL: "127.0.0.1:9"})
	assert.False(t, got.Success)
	assert.NotEmpty(t, got.Message)
}

// --- probe: Orthrus ---

func TestUptimeChecker_Probe_Orthrus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		resolver func() orthrusStatusChecker
		url      string
		success  bool
		message  string
	}{
		{
			name:     "session active",
			resolver: func() orthrusStatusChecker { return &mockOrthrusResolver{addr: "127.0.0.1:1", ok: true} },
			url:      "agent-uuid",
			success:  true,
			message:  "Orthrus session active",
		},
		{
			name:     "agent not connected",
			resolver: func() orthrusStatusChecker { return &mockOrthrusResolver{ok: false} },
			url:      "agent-uuid",
			success:  false,
			message:  "Orthrus agent not connected",
		},
		{
			name:     "nil resolver accessor",
			resolver: nil,
			url:      "agent-uuid",
			success:  false,
			message:  "Orthrus subsystem unavailable",
		},
		{
			name:     "resolver returns nil",
			resolver: func() orthrusStatusChecker { return nil },
			url:      "agent-uuid",
			success:  false,
			message:  "Orthrus subsystem unavailable",
		},
		{
			name:     "missing agent uuid",
			resolver: func() orthrusStatusChecker { return &mockOrthrusResolver{ok: true} },
			url:      "",
			success:  false,
			message:  "Monitor missing agent UUID",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := testChecker()
			c.resolveOrthrus = tc.resolver
			got := c.probe(context.Background(), models.UptimeMonitor{Type: "orthrus", URL: tc.url})
			assert.Equal(t, tc.success, got.Success)
			assert.Equal(t, tc.message, got.Message)
		})
	}
}

func TestUptimeChecker_Probe_UnknownType(t *testing.T) {
	t.Parallel()
	c := testChecker()
	got := c.probe(context.Background(), models.UptimeMonitor{Type: "ping", URL: "whatever"})
	assert.False(t, got.Success)
	assert.Equal(t, "Unknown monitor type", got.Message)
}

// --- probeHost: single dial, no sleep-retry ---

func TestUptimeChecker_ProbeHost_SingleDial_NoSleepOnRefused(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)

	host := models.UptimeHost{Host: "127.0.0.1", Name: "h", Status: "up"}
	require.NoError(t, db.Create(&host).Error)
	require.NoError(t, db.Create(&models.UptimeMonitor{
		UptimeHostID: &host.ID, Name: "m", Type: "tcp", URL: "127.0.0.1:9", Enabled: true,
	}).Error)

	c := testChecker()
	start := time.Now()
	got := c.probeHost(context.Background(), host, db)
	elapsed := time.Since(start)

	assert.False(t, got.Success)
	assert.Less(t, elapsed, time.Second, "a refused dial must return promptly with no 2s sleep-retry loop")
}

func TestUptimeChecker_ProbeHost_Success(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())

	host := models.UptimeHost{Host: "127.0.0.1", Name: "h", Status: "pending"}
	require.NoError(t, db.Create(&host).Error)
	require.NoError(t, db.Create(&models.UptimeMonitor{
		UptimeHostID: &host.ID, Name: "m", Type: "tcp",
		URL: fmt.Sprintf("127.0.0.1:%s", portStr), Enabled: true,
	}).Error)

	got := testChecker().probeHost(context.Background(), host, db)
	assert.True(t, got.Success)
}

func TestUptimeChecker_ProbeHost_NoMonitors_LeavesHostAlone(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	host := models.UptimeHost{Host: "127.0.0.1", Name: "h", Status: "up"}
	require.NoError(t, db.Create(&host).Error)

	got := testChecker().probeHost(context.Background(), host, db)
	assert.True(t, got.Success, "a host with no dialable monitors is treated as inconclusive-up")
}

func TestUptimeChecker_ProbeHost_OrthrusOnly_SkipsDial(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	host := models.UptimeHost{Host: "192.0.2.1", Name: "h", Status: "up"}
	require.NoError(t, db.Create(&host).Error)
	require.NoError(t, db.Create(&models.UptimeMonitor{
		UptimeHostID: &host.ID, Name: "m", Type: "orthrus", URL: "uuid", Enabled: true,
	}).Error)

	start := time.Now()
	got := testChecker().probeHost(context.Background(), host, db)
	assert.True(t, got.Success)
	assert.Less(t, time.Since(start), time.Second, "orthrus-only host must not attempt a TCP dial")
}

// --- pure debounce functions ---

func TestApplyMonitorDebounce(t *testing.T) {
	t.Parallel()
	now := time.Now()
	old := now.Add(-5 * time.Minute)

	t.Run("first failure from up does not flip until maxRetries", func(t *testing.T) {
		next, changed, _ := applyMonitorDebounce(
			monDebounce{status: "up", lastStatusChange: old}, rawCheckResult{Success: false}, 2, now)
		assert.False(t, changed)
		assert.Equal(t, "up", next.status)
		assert.Equal(t, 1, next.failureCount)
	})

	t.Run("second consecutive failure flips up->down at threshold", func(t *testing.T) {
		next, changed, dur := applyMonitorDebounce(
			monDebounce{status: "up", failureCount: 1, lastStatusChange: old}, rawCheckResult{Success: false}, 2, now)
		assert.True(t, changed)
		assert.Equal(t, "down", next.status)
		assert.Equal(t, 2, next.failureCount)
		assert.NotEmpty(t, dur, "downtime/previous-uptime string is populated on a transition")
		assert.Equal(t, now, next.lastStatusChange)
	})

	t.Run("recovery down->up is immediate", func(t *testing.T) {
		next, changed, _ := applyMonitorDebounce(
			monDebounce{status: "down", failureCount: 7, lastStatusChange: old}, rawCheckResult{Success: true}, 3, now)
		assert.True(t, changed)
		assert.Equal(t, "up", next.status)
		assert.Equal(t, 0, next.failureCount)
	})

	t.Run("pending is never a transition", func(t *testing.T) {
		next, changed, _ := applyMonitorDebounce(
			monDebounce{status: "pending"}, rawCheckResult{Success: true}, 3, now)
		assert.False(t, changed)
		assert.Equal(t, "up", next.status)
	})

	t.Run("success on an already-up monitor is a no-op transition", func(t *testing.T) {
		_, changed, _ := applyMonitorDebounce(
			monDebounce{status: "up", failureCount: 0}, rawCheckResult{Success: true}, 3, now)
		assert.False(t, changed)
	})
}

func TestApplyHostDebounce(t *testing.T) {
	t.Parallel()
	now := time.Now()

	t.Run("first failure keeps status, increments count", func(t *testing.T) {
		next, changed := applyHostDebounce(hostDebounce{status: "up"}, rawCheckResult{Success: false}, 2, now)
		assert.False(t, changed)
		assert.Equal(t, "up", next.status)
		assert.Equal(t, 1, next.failureCount)
	})

	t.Run("second failure flips to down", func(t *testing.T) {
		next, changed := applyHostDebounce(hostDebounce{status: "up", failureCount: 1}, rawCheckResult{Success: false}, 2, now)
		assert.True(t, changed)
		assert.Equal(t, "down", next.status)
	})

	t.Run("pending -> up is not a transition", func(t *testing.T) {
		next, changed := applyHostDebounce(hostDebounce{status: "pending"}, rawCheckResult{Success: true}, 2, now)
		assert.False(t, changed)
		assert.Equal(t, "up", next.status)
	})

	t.Run("success resets failure count", func(t *testing.T) {
		next, _ := applyHostDebounce(hostDebounce{status: "down", failureCount: 9}, rawCheckResult{Success: true}, 2, now)
		assert.Equal(t, 0, next.failureCount)
		assert.Equal(t, "up", next.status)
	})
}

func TestForceMonitorDown(t *testing.T) {
	t.Parallel()
	now := time.Now()

	t.Run("up -> down is a transition, failureCount maxed", func(t *testing.T) {
		next, changed, dur := forceMonitorDown(monDebounce{status: "up", lastStatusChange: now.Add(-time.Hour)}, 3, now)
		assert.True(t, changed)
		assert.Equal(t, "down", next.status)
		assert.Equal(t, 3, next.failureCount)
		assert.NotEmpty(t, dur)
	})

	t.Run("pending -> down is not a transition", func(t *testing.T) {
		_, changed, _ := forceMonitorDown(monDebounce{status: "pending"}, 3, now)
		assert.False(t, changed)
	})
}
