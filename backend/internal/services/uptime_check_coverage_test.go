package services

import (
	"context"
	"net"
	"strconv"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// probeHost: the child-monitor query fails -> a failed raw result carrying the
// load error, not a panic.
func TestUptimeChecker_ProbeHost_MonitorLoadError(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)
	host := models.UptimeHost{Host: "127.0.0.1", Name: "h", Status: "up"}
	require.NoError(t, db.Create(&host).Error)
	require.NoError(t, db.Migrator().DropTable(&models.UptimeMonitor{}))

	got := testChecker().probeHost(context.Background(), host, db)
	assert.False(t, got.Success)
	assert.Contains(t, got.Message, "load monitors")
}

// probeHost: a child with an associated ProxyHost dials that ProxyHost's
// ForwardPort; a child whose URL carries no port is skipped (continue).
func TestUptimeChecker_ProbeHost_UsesProxyHostPortAndSkipsPortlessMonitors(t *testing.T) {
	t.Parallel()
	db := setupUptimeTestDB(t)

	addr := tcpListener(t)
	_, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	proxy := models.ProxyHost{UUID: "px-probe", DomainNames: "svc.example.com", ForwardHost: "127.0.0.1", ForwardPort: port}
	require.NoError(t, db.Create(&proxy).Error)

	host := models.UptimeHost{Host: "127.0.0.1", Name: "h", Status: "pending"}
	require.NoError(t, db.Create(&host).Error)

	// Portless URL, no ProxyHost -> extractPort("") == "" -> skipped (continue).
	require.NoError(t, db.Create(&models.UptimeMonitor{
		UptimeHostID: &host.ID, Name: "portless", Type: "tcp", URL: "no-port-here", Enabled: true,
	}).Error)
	// Bound to a ProxyHost -> dial 127.0.0.1:<listener port> -> success.
	require.NoError(t, db.Create(&models.UptimeMonitor{
		UptimeHostID: &host.ID, ProxyHostID: &proxy.ID, Name: "viaproxy", Type: "tcp", URL: "", Enabled: true,
	}).Error)

	got := testChecker().probeHost(context.Background(), host, db)
	assert.True(t, got.Success, "the ProxyHost-bound child provides the dialable port")
}
