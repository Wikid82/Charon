package services

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/network/upstreamdns"
)

// recordTable is a fake DNS zone. The test changes it to simulate a record that moves after save.
type recordTable struct {
	mu      sync.Mutex
	records map[string][]netip.Addr
}

func (r *recordTable) set(name string, addrs ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	parsed := make([]netip.Addr, 0, len(addrs))
	for _, a := range addrs {
		parsed = append(parsed, netip.MustParseAddr(a))
	}
	r.records[name] = parsed
}

func (r *recordTable) lookup(_ context.Context, _, name string) ([]netip.Addr, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if addrs, ok := r.records[name]; ok {
		return addrs, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
}

// useUpstreamNames replaces the lookup, the start function and the retry gap for one test.
func useUpstreamNames(t *testing.T, lookup func(context.Context, string, string) ([]netip.Addr, error)) {
	t.Helper()
	prevLookup, prevStart, prevGap := upstreamLookup, startUpstreamDNS, upstreamBindRetryGap
	upstreamLookup = lookup
	upstreamBindRetryGap = 0
	t.Cleanup(func() {
		upstreamLookup, startUpstreamDNS, upstreamBindRetryGap = prevLookup, prevStart, prevGap
	})
}

// callerResolver is the resolver Caddy builds for a transport resolver (httptransport.go:258-274):
// a Go resolver whose every exchange goes to one configured UDP address.
func callerResolver(address string) *net.Resolver {
	target := address[len("udp/"):]
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "udp", target)
		},
	}
}

func TestUpstreamAnswerFilters_Policy(t *testing.T) {
	setHostSideAddrs(t)
	isolateGatewayAddrs(t, []netip.Addr{netip.MustParseAddr("0.250.250.254")})
	general, reserved := UpstreamAnswerFilters()

	refused := []string{
		"169.254.169.254", "169.254.1.1", "fe80::1", "fd00:ec2::254", "::ffff:169.254.169.254", "0.0.0.0", "::",
	}
	allowed := []string{
		"10.0.0.9", "192.168.1.20", "fd12::5", "100.101.1.2", "198.18.0.7", "198.19.192.5", "0.250.250.254", "127.0.0.1",
	}
	for _, raw := range refused {
		a := netip.MustParseAddr(raw)
		assert.True(t, general(a), "general refuses %s", raw)
	}
	for _, raw := range allowed {
		a := netip.MustParseAddr(raw)
		assert.False(t, general(a), "general allows %s", raw)
	}
	assert.True(t, reserved(netip.MustParseAddr("127.0.0.2")), "reserved refuses this machine")
	assert.True(t, reserved(netip.MustParseAddr("169.254.169.254")), "reserved keeps the general refusals")
	assert.False(t, reserved(netip.MustParseAddr("10.0.0.9")), "reserved allows a LAN address")
	assert.False(t, reserved(netip.MustParseAddr("fd12::5")), "reserved allows a unique local address")
}

func TestUpstreamAnswerFilters_ZoneAndMappedAddresses(t *testing.T) {
	setHostSideAddrs(t)
	isolateGatewayAddrs(t, nil)
	general, reserved := UpstreamAnswerFilters()
	assert.True(t, general(netip.MustParseAddr("fe80::1%eth0")), "a zone does not hide a link-local address")
	assert.True(t, reserved(netip.MustParseAddr("::ffff:127.0.0.1")), "an IPv4-mapped loopback is this machine")
}

func TestUpstreamNameServer_AnswersLikeTheProxyResolver(t *testing.T) {
	setHostSideAddrs(t)
	isolateGatewayAddrs(t, []netip.Addr{netip.MustParseAddr("0.250.250.254")})
	records := &recordTable{records: map[string][]netip.Addr{}}
	useUpstreamNames(t, records.lookup)
	s, err := StartUpstreamNameServer()
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	resolver := s.Resolver()
	require.NotEmpty(t, resolver.General)
	require.NotEmpty(t, resolver.Reserved)
	general := callerResolver(resolver.General)
	reserved := callerResolver(resolver.Reserved)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lookupAddrs := func(r *net.Resolver, name string) ([]netip.Addr, error) {
		return r.LookupNetIP(ctx, "ip", name)
	}

	// A name that resolves to a LAN address is served (E1).
	records.set("lan.test.", "10.0.0.9")
	got, err := lookupAddrs(general, "lan.test.")
	require.NoError(t, err)
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.9")}, got)

	// The address moves to link-local after save: no address is connected (E2).
	records.set("lan.test.", "169.254.169.254")
	_, err = lookupAddrs(general, "lan.test.")
	require.Error(t, err)

	// Mixed answers keep only the allowed address (E3).
	records.set("mixed.test.", "169.254.1.1", "10.0.0.9")
	got, err = lookupAddrs(general, "mixed.test.")
	require.NoError(t, err)
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.9")}, got)

	// The block-list address 0.0.0.0 and the IPv6 refusals connect nowhere (E4, E5).
	records.set("block.test.", "0.0.0.0")
	_, err = lookupAddrs(general, "block.test.")
	require.Error(t, err)
	records.set("v6.test.", "fe80::1", "fd00:ec2::254", "::ffff:169.254.169.254")
	_, err = lookupAddrs(general, "v6.test.")
	require.Error(t, err)

	// Allowed classes are served (E6).
	records.set("classes.test.", "fd12::5", "100.101.1.2", "198.18.0.7", "198.19.192.5")
	got, err = lookupAddrs(general, "classes.test.")
	require.NoError(t, err)
	assert.ElementsMatch(t, []netip.Addr{
		netip.MustParseAddr("fd12::5"), netip.MustParseAddr("100.101.1.2"),
		netip.MustParseAddr("198.18.0.7"), netip.MustParseAddr("198.19.192.5"),
	}, got)

	// Loopback is served on a port Charon does not use and refused on a reserved one (E7, E8).
	records.set("local.test.", "127.0.0.1")
	got, err = lookupAddrs(general, "local.test.")
	require.NoError(t, err, "the general listener serves loopback on a port Charon does not use")
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("127.0.0.1")}, got)
	records.set("local-reserved.test.", "127.0.0.1")
	_, err = lookupAddrs(reserved, "local-reserved.test.")
	require.Error(t, err, "the reserved listener refuses this machine")

	// The container host name is served through the gateway carve-out (E9).
	records.set("host.docker.internal.", "0.250.250.254")
	got, err = lookupAddrs(general, "host.docker.internal.")
	require.NoError(t, err)
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("0.250.250.254")}, got)

	// A name that does not exist is not found, which the client reports as such.
	_, err = lookupAddrs(general, "missing.test.")
	var dnsErr *net.DNSError
	require.ErrorAs(t, err, &dnsErr)
	assert.True(t, dnsErr.IsNotFound)
}

func TestUpstreamNameServer_RefusalLogsCountAndListenerOnly(t *testing.T) {
	setHostSideAddrs(t)
	isolateGatewayAddrs(t, nil)
	records := &recordTable{records: map[string][]netip.Addr{"secret-target.test.": {netip.MustParseAddr("169.254.169.254")}}}
	useUpstreamNames(t, records.lookup)
	hook := logtest.NewLocal(logger.Log().Logger)
	t.Cleanup(hook.Reset)
	s, err := StartUpstreamNameServer()
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = callerResolver(s.Resolver().General).LookupNetIP(ctx, "ip", "secret-target.test.")

	require.Eventually(t, func() bool {
		for _, e := range hook.AllEntries() {
			if e.Message == "Upstream name answer: addresses forward targets may not use were not returned" {
				return true
			}
		}
		return false
	}, 3*time.Second, 10*time.Millisecond)
	for _, e := range hook.AllEntries() {
		assert.NotContains(t, e.Message, "secret-target")
		assert.NotContains(t, e.Message, "169.254")
		for key, value := range e.Data {
			assert.NotContains(t, key, "secret")
			assert.NotContains(t, logrusString(value), "secret-target")
			assert.NotContains(t, logrusString(value), "169.254")
		}
		if e.Message == "Upstream name answer: addresses forward targets may not use were not returned" {
			assert.ElementsMatch(t, []string{"count", "listener"}, dataKeys(e.Data))
		}
	}
}

func logrusString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if err, ok := v.(error); ok {
		return err.Error()
	}
	return ""
}

func dataKeys(data logrus.Fields) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	return keys
}

func TestStartUpstreamNameServer_RetriesThenFailsWithoutResolver(t *testing.T) {
	setHostSideAddrs(t)
	attempts := 0
	useUpstreamNames(t, (&recordTable{}).lookup)
	prev := startUpstreamDNS
	startUpstreamDNS = func(upstreamdns.Config) (*upstreamdns.Server, error) {
		attempts++
		return nil, errors.New("address in use")
	}
	t.Cleanup(func() { startUpstreamDNS = prev })

	s, err := StartUpstreamNameServer()
	require.ErrorContains(t, err, "after 3 attempts")
	assert.Nil(t, s)
	assert.Equal(t, 3, attempts)
	assert.Equal(t, caddy.UpstreamResolver{}, s.Resolver(), "a nil server has no resolver")
}

func TestStartUpstreamNameServer_SecondAttemptBinds(t *testing.T) {
	setHostSideAddrs(t)
	useUpstreamNames(t, (&recordTable{}).lookup)
	prev := startUpstreamDNS
	attempts := 0
	startUpstreamDNS = func(cfg upstreamdns.Config) (*upstreamdns.Server, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("transient bind failure")
		}
		return prev(cfg)
	}
	t.Cleanup(func() { startUpstreamDNS = prev })

	s, err := StartUpstreamNameServer()
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	assert.Equal(t, 2, attempts)
	assert.NotEmpty(t, s.Resolver().General)
}

func TestUpstreamNameServer_NilIsSafe(t *testing.T) {
	t.Parallel()
	var s *UpstreamNameServer
	assert.NoError(t, s.Close())
	assert.Equal(t, caddy.UpstreamResolver{}, s.Resolver())
	assert.NoError(t, (&UpstreamNameServer{}).Close())
}

func TestListenerTargetGuard_ReservedPorts(t *testing.T) {
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "reserved-agent", 2375)
	useListenerSource(t, fakeListenerSource{})

	ports, err := NewListenerTargetGuard(db).ReservedPorts()
	require.NoError(t, err)
	assert.Contains(t, ports, 2375)
	assert.Contains(t, ports, 2019, "the admin port is reserved")

	var nilGuard *ListenerTargetGuard
	ports, err = nilGuard.ReservedPorts()
	require.NoError(t, err)
	assert.Empty(t, ports)
	ports, err = NewListenerTargetGuard(nil).ReservedPorts()
	require.NoError(t, err)
	assert.Empty(t, ports)
}
