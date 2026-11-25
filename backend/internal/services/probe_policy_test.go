package services

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
)

// resetEmergency clears the emergency reservation after a test.
func resetEmergency(t *testing.T) {
	t.Helper()
	SetEmergencyListener(config.EmergencyConfig{})
	t.Cleanup(func() { SetEmergencyListener(config.EmergencyConfig{}) })
}

func TestListenPortFrom(t *testing.T) {
	t.Parallel()
	cases := map[string]int{
		"127.0.0.1:2019":          2019,
		"[::1]:2020":              2020,
		":2020":                   2020,
		"http://127.0.0.1:8085":   8085,
		"https://localhost:8443/": 8443,
		"127.0.0.1":               0,
		"127.0.0.1:admin":         0,
		"127.0.0.1:70000":         0,
		"http://localhost":        0,
		"":                        0,
	}
	for in, want := range cases {
		assert.Equal(t, want, listenPortFrom(in), "listenPortFrom(%q)", in)
	}
}

func TestSetEmergencyListener_ReservesOnlyWhileEnabled(t *testing.T) {
	resetEmergency(t)
	SetEmergencyListener(config.EmergencyConfig{Enabled: true, BindAddress: "127.0.0.1:2020"})
	assert.Equal(t, int32(2020), emergencyPort.Load())
	SetEmergencyListener(config.EmergencyConfig{Enabled: false, BindAddress: "127.0.0.1:2020"})
	assert.Equal(t, int32(0), emergencyPort.Load())
	SetEmergencyListener(config.EmergencyConfig{Enabled: true, BindAddress: "not-an-address"})
	assert.Equal(t, int32(0), emergencyPort.Load())
}

func TestReservedListenerPorts_FixedAndLivePortsWithoutDatabase(t *testing.T) {
	resetEmergency(t)
	useListenerSource(t, fakeListenerSource{ports: []int{2222, 0}})

	ports, err := reservedListenerPorts(nil)
	require.NoError(t, err)
	assert.Contains(t, ports, 2019, "Caddy admin port")
	assert.Contains(t, ports, 8085, "bundled CrowdSec LAPI port")
	assert.Contains(t, ports, 2222, "live agent port")
	assert.NotContains(t, ports, 0)
	assert.NotContains(t, ports, 2020, "emergency port is not reserved while disabled")

	SetEmergencyListener(config.EmergencyConfig{Enabled: true, BindAddress: "127.0.0.1:2020"})
	ports, err = reservedListenerPorts(nil)
	require.NoError(t, err)
	assert.Contains(t, ports, 2020, "emergency port is reserved while enabled")
}

func TestReservedListenerPorts_StoredAgentPortsAndMissingTable(t *testing.T) {
	resetEmergency(t)
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "stored", 2375)
	ports, err := reservedListenerPorts(db)
	require.NoError(t, err)
	assert.Contains(t, ports, 2375)
	assert.Contains(t, ports, 2019)

	empty, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "empty.db")), &gorm.Config{})
	require.NoError(t, err)
	ports, err = reservedListenerPorts(empty)
	require.NoError(t, err, "a missing agent table means no stored agents")
	assert.Contains(t, ports, 8085)
}

func TestReservedListenerPorts_DatabaseErrorIsReturned(t *testing.T) {
	db := setupListenerPolicyDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = reservedListenerPorts(db)
	require.Error(t, err)
}

// guardWithClock builds a guard whose clock and loader the test controls.
func guardWithClock(now *time.Time, load func(*gorm.DB) (map[int]struct{}, error)) *ReservedListenerGuard {
	return &ReservedListenerGuard{
		now:  func() time.Time { return *now },
		load: load,
		warn: ratelimit.NewWarnBudget(10, time.Minute, nil),
	}
}

func TestReservedListenerGuard_RefusesReservedPortOnThisMachine(t *testing.T) {
	setHostSideAddrs(t)
	now := time.Unix(1000, 0)
	g := guardWithClock(&now, func(*gorm.DB) (map[int]struct{}, error) {
		return map[int]struct{}{2019: {}}, nil
	})

	err := g.Check(netip.MustParseAddrPort("127.0.0.1:2019"))
	assert.ErrorIs(t, err, network.ErrBlockedAddress)
	assert.NoError(t, g.Check(netip.MustParseAddrPort("127.0.0.1:8080")))
	assert.NoError(t, g.Check(netip.MustParseAddrPort("10.1.2.3:2019")), "other machines are not checked")
}

func TestReservedListenerGuard_IgnoresZoneAndMappedForms(t *testing.T) {
	setHostSideAddrs(t)
	now := time.Unix(1000, 0)
	g := guardWithClock(&now, func(*gorm.DB) (map[int]struct{}, error) {
		return map[int]struct{}{2019: {}}, nil
	})
	assert.ErrorIs(t, g.Check(netip.MustParseAddrPort("[::ffff:127.0.0.1]:2019")), network.ErrBlockedAddress)
	assert.ErrorIs(t, g.Check(netip.MustParseAddrPort("[::1]:2019")), network.ErrBlockedAddress)
	assert.NoError(t, g.Check(netip.AddrPort{}), "an invalid destination is left to the policy")
}

func TestReservedListenerGuard_CachesForFiveSeconds(t *testing.T) {
	setHostSideAddrs(t)
	now := time.Unix(1000, 0)
	calls := 0
	g := guardWithClock(&now, func(*gorm.DB) (map[int]struct{}, error) {
		calls++
		return map[int]struct{}{2019: {}}, nil
	})
	dest := netip.MustParseAddrPort("127.0.0.1:2019")
	require.Error(t, g.Check(dest))
	now = now.Add(4 * time.Second)
	require.Error(t, g.Check(dest))
	assert.Equal(t, 1, calls, "reserved set is reused within the TTL")
	now = now.Add(2 * time.Second)
	require.Error(t, g.Check(dest))
	assert.Equal(t, 2, calls, "reserved set reloads after the TTL")
}

func TestReservedListenerGuard_DatabaseErrorFailsClosedAndIsNotCached(t *testing.T) {
	setHostSideAddrs(t)
	now := time.Unix(1000, 0)
	calls := 0
	g := guardWithClock(&now, func(*gorm.DB) (map[int]struct{}, error) {
		calls++
		return nil, errors.New("database is locked")
	})
	dest := netip.MustParseAddrPort("127.0.0.1:8080")
	err := g.Check(dest)
	assert.ErrorIs(t, err, network.ErrBlockedAddress, "a database error refuses")
	assert.NotContains(t, err.Error(), "127.0.0.1", "error text names no address")
	require.Error(t, g.Check(dest))
	assert.Equal(t, 2, calls, "a failed load is not cached")

	other := netip.MustParseAddrPort("10.1.2.3:8080")
	assert.NoError(t, g.Check(other), "addresses that are not this machine do not touch the database")
	assert.Equal(t, 2, calls)
}

func TestReservedListenerGuard_ConstructorWiresDatabaseLoader(t *testing.T) {
	setHostSideAddrs(t)
	resetEmergency(t)
	db := setupListenerPolicyDB(t)
	g := NewReservedListenerGuard(db)
	assert.NoError(t, g.Check(netip.MustParseAddrPort("127.0.0.1:8080")))
	assert.ErrorIs(t, g.Check(netip.MustParseAddrPort("127.0.0.1:2019")), network.ErrBlockedAddress)
}

func TestProbePolicy_AllowsAndRefusesExpectedClasses(t *testing.T) {
	setHostSideAddrs(t)
	isolateGatewayAddrs(t, nil)
	p := probePolicy()
	cases := map[string]bool{
		"127.0.0.1":       false,
		"10.1.2.3":        false,
		"100.64.0.9":      false,
		"fd12::5":         false,
		"198.18.0.5":      false,
		"169.254.1.3":     true,
		"fe80::1":         true,
		"0.0.0.0":         true,
		"169.254.169.254": true,
		"fd00:ec2::254":   true,
		"fd20:ce::254":    true,
		"100.100.100.200": true,
		"168.63.129.16":   true,
	}
	for raw, blocked := range cases {
		assert.Equal(t, blocked, p.Blocked(net.ParseIP(raw)), "Blocked(%s)", raw)
	}
}

func TestGatewayCarveOut_FromHostsFileOnlyForExactAddresses(t *testing.T) {
	setHostSideAddrs(t)
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts")
	content := "# comment line\n" +
		"169.254.1.2 host.containers.internal # Podman pasta\n" +
		"0.250.250.254 host.docker.internal gateway.docker.internal host.orb.internal\n" +
		"169.254.169.254 gateway.docker.internal\n" +
		"10.9.9.9 unrelated.example\n" +
		"not-an-ip host.containers.internal\n"
	require.NoError(t, os.WriteFile(hosts, []byte(content), 0o600))
	prevPath := hostsFilePath
	hostsFilePath = hosts
	t.Cleanup(func() { hostsFilePath = prevPath })
	stubLookup(t, func(context.Context, string) ([]net.IPAddr, error) { return nil, errors.New("no dns") })
	isolateGatewayAddrs(t, nil)
	prevLoad := gatewayCache.load
	gatewayCache.load = resolveGatewayAddrs
	t.Cleanup(func() {
		gatewayCache.load = prevLoad
		resetProbeAddrCaches()
	})
	resetProbeAddrCaches()

	gw := gatewayAllowAddrs()
	assert.True(t, containsAddr(gw, netip.MustParseAddr("169.254.1.2")))
	assert.True(t, containsAddr(gw, netip.MustParseAddr("0.250.250.254")))
	assert.False(t, containsAddr(gw, netip.MustParseAddr("10.9.9.9")))
	assert.False(t, containsAddr(gw, netip.MustParseAddr("169.254.1.3")))

	p := probePolicy()
	assert.False(t, p.Blocked(net.ParseIP("169.254.1.2")), "Podman gateway address is allowed")
	assert.False(t, p.Blocked(net.ParseIP("0.250.250.254")), "OrbStack gateway address is allowed")
	assert.True(t, p.Blocked(net.ParseIP("169.254.1.3")), "a neighbouring link-local address stays blocked")
	assert.True(t, p.Blocked(net.ParseIP("169.254.169.254")), "always-blocked wins over a gateway entry")

	now := time.Unix(1000, 0)
	g := guardWithClock(&now, func(*gorm.DB) (map[int]struct{}, error) {
		return map[int]struct{}{2019: {}}, nil
	})
	assert.ErrorIs(t, g.Check(netip.MustParseAddrPort("169.254.1.2:2019")), network.ErrBlockedAddress,
		"reserved ports stay refused on a gateway address")
	assert.NoError(t, g.Check(netip.MustParseAddrPort("169.254.1.2:8080")))
}

func TestResolveGatewayAddrs_LookupAddsAddresses(t *testing.T) {
	stubLookup(t, func(_ context.Context, name string) ([]net.IPAddr, error) {
		if name == "host.docker.internal" {
			return []net.IPAddr{{IP: net.ParseIP("192.0.2.77")}}, nil
		}
		return nil, errors.New("not found")
	})
	prevPath := hostsFilePath
	hostsFilePath = filepath.Join(t.TempDir(), "missing")
	t.Cleanup(func() { hostsFilePath = prevPath })

	addrs := resolveGatewayAddrs()
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("192.0.2.77")}, addrs)
}

func TestHostsFileAddrs_MissingFileYieldsNothing(t *testing.T) {
	t.Parallel()
	assert.Nil(t, hostsFileAddrs(filepath.Join(t.TempDir(), "none"), gatewayNames))
}

func TestAddrCache_KeepsLastNonEmptySnapshot(t *testing.T) {
	t.Parallel()
	now := time.Unix(0, 0)
	responses := [][]netip.Addr{
		{netip.MustParseAddr("10.0.0.1")},
		nil,
		{netip.MustParseAddr("10.0.0.2")},
	}
	calls := 0
	c := &addrCache{ttl: time.Second, now: func() time.Time { return now }, load: func() []netip.Addr {
		r := responses[calls]
		calls++
		return r
	}}
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.1")}, c.get())
	now = now.Add(2 * time.Second)
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.1")}, c.get(), "an empty reload keeps the previous snapshot")
	now = now.Add(2 * time.Second)
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.2")}, c.get())
	c.reset()
	assert.False(t, c.loaded)
}

func TestDedupeAndContainsAddr(t *testing.T) {
	t.Parallel()
	a := netip.MustParseAddr("10.0.0.1")
	got := dedupeAddrs([]netip.Addr{a, a, netip.MustParseAddr("10.0.0.2")})
	assert.Len(t, got, 2)
	assert.True(t, containsAddr(got, netip.MustParseAddr("::ffff:10.0.0.1")))
	assert.False(t, containsAddr(got, netip.MustParseAddr("10.0.0.9")))
}

func TestNewProbeDialer_RefusesReservedPortAtConnect(t *testing.T) {
	setHostSideAddrs(t)
	resetEmergency(t)
	db := setupListenerPolicyDB(t)
	d := NewProbeDialer(db, time.Second)
	require.NotNil(t, d.Control)
	assert.ErrorIs(t, d.Control("tcp", "127.0.0.1:2019", nil), network.ErrBlockedAddress)
	assert.ErrorIs(t, d.Control("tcp", "169.254.169.254:80", nil), network.ErrBlockedAddress)
	assert.NoError(t, d.Control("tcp", "127.0.0.1:8080", nil))
}

func TestProbeHTTPOptions_BuildClient(t *testing.T) {
	t.Parallel()
	db := setupListenerPolicyDB(t)
	opts := ProbeHTTPOptions(db)
	assert.NotEmpty(t, opts)
	assert.NotNil(t, network.NewSafeHTTPClient(opts...))
}

// Tests must not depend on the host's gateway names. The default gateway loader
// is replaced with an empty one; tests that exercise the loader restore it.
func init() {
	gatewayCache.load = func() []netip.Addr { return nil }
	lookupForwardHost = offlineLookup
}

// offlineLookup answers IP literals and refuses every name, so tests never wait
// on the network. Tests that need a name install their own stub.
func offlineLookup(_ context.Context, name string) ([]net.IPAddr, error) {
	if ip := net.ParseIP(name); ip != nil {
		return []net.IPAddr{{IP: ip}}, nil
	}
	return nil, errors.New("name lookups are offline in tests")
}
