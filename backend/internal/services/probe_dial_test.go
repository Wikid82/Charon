package services

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/network"
)

// listenLocal opens a loopback listener that accepts and closes connections, and
// returns its port.
func listenLocal(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			_ = c.Close()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

// closedLocalPort returns a loopback port with nothing listening on it.
func closedLocalPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	return port
}

func TestTCPMonitorToLinkLocalGetsFixedMessage(t *testing.T) {
	setHostSideAddrs(t)
	c := newUptimeChecker(&UptimeService{})
	res := c.probe(context.Background(), models.UptimeMonitor{Type: "tcp", URL: "tcp://169.254.169.254:80"})
	assert.False(t, res.Success)
	assert.Equal(t, destinationRefusedMessage, res.Message)
}

func TestHTTPMonitorReservedPortRefusedLoopbackAllowed(t *testing.T) {
	setHostSideAddrs(t)
	resetEmergency(t)
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, nil)
	c := newUptimeChecker(svc)

	refused := c.probe(context.Background(), models.UptimeMonitor{Type: "http", URL: "http://127.0.0.1:2019/"})
	assert.False(t, refused.Success)
	assert.Equal(t, destinationRefusedMessage, refused.Message)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(srv.Close)
	allowed := c.probe(context.Background(), models.UptimeMonitor{Type: "http", URL: srv.URL})
	assert.True(t, allowed.Success, allowed.Message)
}

func TestHostPreCheckSkipsRefusedPort(t *testing.T) {
	setHostSideAddrs(t)
	resetEmergency(t)
	db := setupUptimeTestDB(t)
	c := newUptimeChecker(NewUptimeService(db, nil))
	host := models.UptimeHost{Host: "127.0.0.1", Name: "local", Status: "up"}
	require.NoError(t, db.Create(&host).Error)
	require.NoError(t, db.Create(&models.UptimeMonitor{UptimeHostID: &host.ID, Name: "admin", Type: "tcp", URL: "tcp://127.0.0.1:2019"}).Error)

	res := c.probeHost(context.Background(), host, db)
	assert.False(t, res.Success, "a refused port alone fails the pre-check")
	assert.Equal(t, destinationRefusedMessage, res.Message)

	open := listenLocal(t)
	require.NoError(t, db.Create(&models.UptimeMonitor{UptimeHostID: &host.ID, Name: "open", Type: "tcp", URL: "tcp://127.0.0.1:" + strconv.Itoa(open)}).Error)
	res = c.probeHost(context.Background(), host, db)
	assert.True(t, res.Success, "the pre-check goes on to the next port: %s", res.Message)
}

func TestProbeDialerChecksAddressAtConnect(t *testing.T) {
	setHostSideAddrs(t)
	isolateGatewayAddrs(t, nil)
	resetEmergency(t)
	db := setupProxyHostTestDB(t)

	// A name that resolved to a private address at save time must still be refused
	// for a link-local address at connect time.
	stubLookup(t, resolverAnswers(new(atomic.Int32), map[string][]string{"rebind.example": {"10.0.0.1"}}))
	require.NoError(t, CheckProbeTarget(context.Background(), db, "rebind.example", 80))

	d := NewProbeDialer(db, time.Second)
	assert.ErrorIs(t, d.Control("tcp", "169.254.169.254:80", nil), network.ErrBlockedAddress)
	assert.ErrorIs(t, d.Control("tcp", "[fe80::1%eth0]:80", nil), network.ErrBlockedAddress)
	assert.NoError(t, d.Control("tcp", "10.0.0.1:80", nil))
}

func TestProbeDialerAllowedClassesAndCarveOuts(t *testing.T) {
	setHostSideAddrs(t)
	resetEmergency(t)
	isolateGatewayAddrs(t, []netip.Addr{netip.MustParseAddr("169.254.1.2")})
	db := setupProxyHostTestDB(t)
	d := NewProbeDialer(db, time.Second)

	for _, allowed := range []string{
		"127.0.0.1:8080", "10.1.2.3:443", "100.64.0.9:22", "[fd12::5]:8080", "198.18.0.5:80", "169.254.1.2:8080",
	} {
		assert.NoError(t, d.Control("tcp", allowed, nil), allowed)
	}
	for _, reserved := range []string{"127.0.0.1:2019", "127.0.0.1:8085", "169.254.1.2:2019"} {
		assert.ErrorIs(t, d.Control("tcp", reserved, nil), network.ErrBlockedAddress, reserved)
	}
	assert.ErrorIs(t, d.Control("tcp", "169.254.1.3:8080", nil), network.ErrBlockedAddress)
	assert.ErrorIs(t, d.Control("tcp", "[fd00:ec2::254]:80", nil), network.ErrBlockedAddress, "instance service stays refused")
}

func TestCheckProbeTarget_RefusalsAndSaveTimeLookup(t *testing.T) {
	setHostSideAddrs(t)
	resetEmergency(t)
	isolateGatewayAddrs(t, nil)
	forwardNameCache.entries = map[string]cachedVerdict{}
	db := setupProxyHostTestDB(t)
	ctx := context.Background()

	assert.ErrorIs(t, CheckProbeTarget(ctx, db, "169.254.169.254", 80), ErrDestinationRefused)
	assert.ErrorIs(t, CheckProbeTarget(ctx, db, "127.0.0.1", 2019), ErrDestinationRefused)
	assert.NoError(t, CheckProbeTarget(ctx, db, "127.0.0.1", 8080))

	stubLookup(t, resolverAnswers(new(atomic.Int32), map[string][]string{"meta.example": {"169.254.169.254"}}))
	assert.ErrorIs(t, CheckProbeTarget(ctx, db, "meta.example", 80), ErrDestinationRefused)

	stubLookup(t, resolverAnswers(new(atomic.Int32), nil))
	forwardNameCache.entries = map[string]cachedVerdict{}
	assert.NoError(t, CheckProbeTarget(ctx, db, "later.example", 80), "a failed lookup is accepted at save time")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	err = CheckProbeTarget(ctx, db, "127.0.0.1", 2019)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrDestinationRefused), "a database error is not a refusal")
}

func TestRemoteServerTestDial(t *testing.T) {
	setHostSideAddrs(t)
	resetEmergency(t)
	isolateGatewayAddrs(t, nil)
	db := setupProxyHostTestDB(t)
	svc := NewRemoteServerService(db)
	ctx := context.Background()

	_, err := svc.TestDial(ctx, "169.254.169.254", 80)
	assert.ErrorIs(t, err, ErrDestinationRefused)

	_, err = svc.TestDial(ctx, "127.0.0.1", closedLocalPort(t))
	assert.ErrorIs(t, err, ErrConnectionTestFailed)
	assert.NotContains(t, err.Error(), "127.0.0.1")

	latency, err := svc.TestDial(ctx, "127.0.0.1", listenLocal(t))
	require.NoError(t, err)
	assert.Positive(t, latency, "the latency is the time the dial took")
}

func TestProxyHostTestConnectionOutcomes(t *testing.T) {
	setHostSideAddrs(t)
	resetEmergency(t)
	isolateGatewayAddrs(t, nil)
	useListenerSource(t, fakeListenerSource{})
	db := setupProxyHostTestDB(t)
	svc := NewProxyHostService(db)

	assert.ErrorIs(t, svc.TestConnection("169.254.169.254", 80), ErrForwardTargetAddress)
	assert.ErrorIs(t, svc.TestConnection("127.0.0.1", 2019), ErrOrthrusListenerTarget)
	err := svc.TestConnection("127.0.0.1", closedLocalPort(t))
	assert.ErrorIs(t, err, ErrConnectionTestFailed)
	assert.NotContains(t, err.Error(), "127.0.0.1")
	assert.NoError(t, svc.TestConnection("127.0.0.1", listenLocal(t)))
}

func TestEmergencyPortRefusedAsProbeWhileEnabled(t *testing.T) {
	setHostSideAddrs(t)
	isolateGatewayAddrs(t, nil)
	useListenerSource(t, fakeListenerSource{})
	db := setupProxyHostTestDB(t)
	SetEmergencyListener(config.EmergencyConfig{Enabled: true, BindAddress: "127.0.0.1:2020"})
	t.Cleanup(func() { SetEmergencyListener(config.EmergencyConfig{}) })

	assert.ErrorIs(t, CheckProbeTarget(context.Background(), db, "localhost", 2020), ErrDestinationRefused)
}

func TestHTTPMonitorLoopbackNameIsCheckedNotPreRefused(t *testing.T) {
	setHostSideAddrs(t)
	resetEmergency(t)
	db := setupUptimeTestDB(t)
	c := newUptimeChecker(NewUptimeService(db, nil))
	// 127.0.0.2 is loopback but not an exact localhost form. The monitor is created
	// and probed; the probe must reach the connection step and report how that failed,
	// not the destination refusal.
	res := c.probe(context.Background(), models.UptimeMonitor{Type: "http", URL: "http://127.0.0.2:" + strconv.Itoa(closedLocalPort(t)) + "/"})
	assert.False(t, res.Success)
	assert.NotEqual(t, destinationRefusedMessage, res.Message)
	assert.NotContains(t, res.Message, "security validation failed")
}
