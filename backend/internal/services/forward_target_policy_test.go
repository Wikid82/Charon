package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// fakeListenerSource reports a fixed set of live listener ports.
type fakeListenerSource struct{ ports []int }

func (f fakeListenerSource) LiveListenerPorts() []int { return f.ports }

// setupListenerPolicyDB returns a database holding the Orthrus agent table and
// the proxy host tables used by the forward target checks.
func setupListenerPolicyDB(t *testing.T) *gorm.DB {
	t.Helper()
	setHostSideAddrs(t)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.OrthrusAgent{}, &models.ProxyHost{}, &models.Location{}))
	return db
}

// useListenerSource installs a live source for the duration of one test.
func useListenerSource(t *testing.T, src OrthrusListenerSource) {
	t.Helper()
	SetOrthrusListenerSource(src)
	t.Cleanup(func() { SetOrthrusListenerSource(nil) })
}

// stubLookup replaces the name resolver for one test.
func stubLookup(t *testing.T, fn func(ctx context.Context, name string) ([]net.IPAddr, error)) {
	t.Helper()
	prev := lookupForwardHost
	lookupForwardHost = fn
	t.Cleanup(func() { lookupForwardHost = prev })
}

// noLookup fails the test if the resolver is consulted.
func noLookup(t *testing.T) {
	t.Helper()
	stubLookup(t, func(context.Context, string) ([]net.IPAddr, error) {
		t.Errorf("name resolver must not run")
		return nil, errors.New("unexpected lookup")
	})
}

func seedAgentPort(t *testing.T, db *gorm.DB, name string, port int) {
	t.Helper()
	require.NoError(t, db.Create(&models.OrthrusAgent{
		UUID:              name + "-uuid",
		Name:              name,
		AuthKeyHash:       "x",
		ExternalProxyPort: port,
	}).Error)
}

func TestRefusesListenerTarget_LocalNamesAndAddressesOnListenerPort(t *testing.T) {
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "a", 2375)
	useListenerSource(t, fakeListenerSource{})
	stubLookup(t, func(_ context.Context, name string) ([]net.IPAddr, error) {
		if strings.HasSuffix(name, ".invalid") {
			return []net.IPAddr{{IP: net.ParseIP("203.0.113.9")}}, nil
		}
		return nil, errors.New("no such host")
	})

	tests := []struct {
		name string
		host string
		port int
		want bool
	}{
		{"ipv4 loopback", "127.0.0.1", 2375, true},
		{"other loopback address", "127.0.0.2", 2375, true},
		{"localhost name", "localhost", 2375, true},
		{"uppercase localhost with trailing dot", "LOCALHOST.", 2375, true},
		{"ipv6 loopback", "::1", 2375, true},
		{"bracketed ipv6 loopback", "[::1]", 2375, true},
		{"ipv4 mapped loopback", "::ffff:127.0.0.1", 2375, true},
		{"unspecified ipv4", "0.0.0.0", 2375, true},
		{"unspecified ipv6", "::", 2375, true},
		{"scheme and path around loopback", "http://127.0.0.1/x", 2375, true},
		{"legacy two part loopback", "127.1", 2375, true},
		{"legacy single number loopback", "2130706433", 2375, true},
		{"legacy hex loopback", "0x7f.1", 2375, true},
		{"legacy octal loopback", "0177.0.0.1", 2375, true},
		{"legacy zero", "0", 2375, true},
		{"gateway name of the docker host", "host.docker.internal", 2375, true},
		{"loopback on another port", "127.0.0.1", 8080, false},
		{"remote address on listener port", "10.20.30.40", 2375, false},
		{"public name on listener port", "example.invalid", 2375, false},
		{"legacy remote address", "10.1", 2375, false},
		{"unresolvable malformed name is refused on a listener port", "1e3", 2375, true},
		{"malformed address with an oversized part is refused on a listener port", "1.2.3.256", 2375, true},
		{"malformed address with an oversized leading part is refused on a listener port", "300.1.1", 2375, true},
		{"unresolvable underscore name is refused on a listener port", "1_27.1", 2375, true},
		{"empty host", "", 2375, false},
		{"agent-name form is refused on a listener port", "orthrus:abc", 2375, true},
		{"port zero", "127.0.0.1", 0, false},
		{"negative port", "127.0.0.1", -1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := refusesListenerTarget(db, tt.host, tt.port)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRefusesListenerTarget_PortSources(t *testing.T) {
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "disconnected", 2376)
	seedAgentPort(t, db, "unset", 0)
	useListenerSource(t, fakeListenerSource{ports: []int{40001, 0}})
	noLookup(t)

	tests := []struct {
		name string
		port int
		want bool
	}{
		{"persisted port of a disconnected agent", 2376, true},
		{"live ephemeral port", 40001, true},
		{"zero live port is not a listener", 0, false},
		{"unused port", 40002, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := refusesListenerTarget(db, "127.0.0.1", tt.port)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRefusesListenerTarget_NilLiveSourceUsesPersistedPorts(t *testing.T) {
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "a", 2377)
	SetOrthrusListenerSource(nil)
	noLookup(t)

	got, err := refusesListenerTarget(db, "127.0.0.1", 2377)
	require.NoError(t, err)
	assert.True(t, got)

	got, err = refusesListenerTarget(db, "127.0.0.1", 40001)
	require.NoError(t, err)
	assert.False(t, got)
}

func TestRefusesListenerTarget_NilTypedLiveSourceIsSafe(t *testing.T) {
	db := setupListenerPolicyDB(t)
	var typedNil *nilSafeSource
	useListenerSource(t, typedNil)
	noLookup(t)

	got, err := refusesListenerTarget(db, "127.0.0.1", 40001)
	require.NoError(t, err)
	assert.False(t, got)
}

// nilSafeSource mirrors a live source whose methods tolerate a nil receiver.
type nilSafeSource struct{}

func (*nilSafeSource) LiveListenerPorts() []int { return nil }

func TestRefusesListenerTarget_NameResolvingToThisMachine(t *testing.T) {
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "a", 2375)
	useListenerSource(t, fakeListenerSource{})

	stubLookup(t, func(_ context.Context, name string) ([]net.IPAddr, error) {
		switch name {
		case "charon-self":
			return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
		case "unspecified-name":
			return []net.IPAddr{{IP: net.ParseIP("0.0.0.0")}}, nil
		case "public-name":
			return []net.IPAddr{{IP: net.ParseIP("203.0.113.9")}}, nil
		default:
			return nil, errors.New("no such host")
		}
	})

	tests := []struct {
		name string
		host string
		want bool
	}{
		{"name resolves to a local address", "charon-self", true},
		{"name resolves to the unspecified address", "unspecified-name", true},
		{"name resolves to a remote address", "public-name", false},
		{"name that does not resolve is refused", "missing-name", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := refusesListenerTarget(db, tt.host, 2375)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRefusesListenerTarget_NameLookupOnlyForListenerPorts(t *testing.T) {
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "a", 2375)
	useListenerSource(t, fakeListenerSource{})
	noLookup(t)

	got, err := refusesListenerTarget(db, "charon-self", 8080)
	require.NoError(t, err)
	assert.False(t, got)
}

func TestRefusesListenerTarget_DatabaseError(t *testing.T) {
	db := setupListenerPolicyDB(t)
	useListenerSource(t, fakeListenerSource{})
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = refusesListenerTarget(db, "127.0.0.1", 2375)
	require.Error(t, err)
}

func TestParseLegacyIPv4(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{"127.1", "127.0.0.1"},
		{"127.0.1", "127.0.0.1"},
		{"2130706433", "127.0.0.1"},
		{"0x7f000001", "127.0.0.1"},
		{"0177.0.0.1", "127.0.0.1"},
		{"0", "0.0.0.0"},
		{"10.1.2", "10.1.0.2"},
		{"", ""},
		{"example", ""},
		{"1.2.3.4.5", ""},
		{"08.1", ""},
		{"4294967296", ""},
		{"256.1", ""},
		{"1.2.3.256", ""},
		{"127..1", ""},
		{"127.0.0.1.", ""},
	}
	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			got := parseLegacyIPv4(tt.host)
			if tt.want == "" {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.want, got.String())
		})
	}
}

func TestRefuseOrthrusListenerTarget_ReturnsSentinel(t *testing.T) {
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "a", 2375)
	useListenerSource(t, fakeListenerSource{})
	noLookup(t)

	err := refuseOrthrusListenerTarget(db, "127.0.0.1", 2375)
	require.ErrorIs(t, err, ErrOrthrusListenerTarget)
	assert.NoError(t, refuseOrthrusListenerTarget(db, "10.0.0.5", 2375))
}

func TestRefuseOrthrusListenerTarget_WrapsDatabaseError(t *testing.T) {
	db := setupListenerPolicyDB(t)
	useListenerSource(t, fakeListenerSource{})
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	err = refuseOrthrusListenerTarget(db, "127.0.0.1", 2375)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrOrthrusListenerTarget)
}

func TestListenerTargetGuard_RefusesStoredRowsAndNilSafe(t *testing.T) {
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "a", 2375)
	useListenerSource(t, fakeListenerSource{})
	noLookup(t)

	// A row saved before the rule existed is still refused at apply time.
	stored := &models.ProxyHost{DomainNames: "old.example.com", ForwardHost: "127.0.0.1", ForwardPort: 2375}
	require.NoError(t, db.Omit("Locations").Create(stored).Error)

	guard := NewListenerTargetGuard(db)
	refused, err := guard.RefusesForwardTarget(stored.ForwardHost, stored.ForwardPort)
	require.NoError(t, err)
	assert.True(t, refused)

	var nilGuard *ListenerTargetGuard
	refused, err = nilGuard.RefusesForwardTarget("127.0.0.1", 2375)
	require.NoError(t, err)
	assert.False(t, refused)
	refused, err = NewListenerTargetGuard(nil).RefusesForwardTarget("127.0.0.1", 2375)
	require.NoError(t, err)
	assert.False(t, refused)
}

func TestListenerTargetGuard_ReevaluatesAfterExternalProxyPortChange(t *testing.T) {
	db := setupListenerPolicyDB(t)
	useListenerSource(t, fakeListenerSource{})
	noLookup(t)
	guard := NewListenerTargetGuard(db)
	svc := NewOrthrusService(db, nil)

	require.NoError(t, db.Create(&models.OrthrusAgent{UUID: "p-uuid", Name: "p", AuthKeyHash: "x"}).Error)

	// Before the port is set, the host's target is an ordinary local port.
	refused, err := guard.RefusesForwardTarget("127.0.0.1", 3000)
	require.NoError(t, err)
	assert.False(t, refused)

	// Setting the external proxy port to that number makes the same target a listener port.
	_, err = svc.Patch("p-uuid", AgentPatch{ExternalProxyPort: intPtr(3000)})
	require.NoError(t, err)
	refused, err = guard.RefusesForwardTarget("127.0.0.1", 3000)
	require.NoError(t, err)
	assert.True(t, refused)

	// Changing the bind setting does not hide a local target on the listener port.
	bind := "10.0.0.5"
	_, err = svc.Patch("p-uuid", AgentPatch{ExternalProxyBind: &bind})
	require.NoError(t, err)
	refused, err = guard.RefusesForwardTarget("127.0.0.1", 3000)
	require.NoError(t, err)
	assert.True(t, refused)

	// Disabling the external proxy releases the port.
	_, err = svc.Patch("p-uuid", AgentPatch{ExternalProxyPort: intPtr(0)})
	require.NoError(t, err)
	refused, err = guard.RefusesForwardTarget("127.0.0.1", 3000)
	require.NoError(t, err)
	assert.False(t, refused)
}

func TestProxyHostWrite_RefusesListenerTargetForEveryRole(t *testing.T) {
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "a", 2375)
	useListenerSource(t, fakeListenerSource{})
	noLookup(t)
	svc := NewProxyHostService(db)

	for _, admin := range []bool{false, true} {
		host := &models.ProxyHost{DomainNames: fmt.Sprintf("role-%t.example.com", admin), ForwardHost: "127.0.0.1", ForwardPort: 2375}
		err := svc.CreateAs(host, admin)
		require.ErrorIs(t, err, ErrOrthrusListenerTarget, "admin=%t", admin)
	}

	ok := &models.ProxyHost{DomainNames: "fine.example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080}
	require.NoError(t, svc.CreateAs(ok, false))

	// An update that moves an existing host onto a listener port is refused too.
	require.ErrorIs(t, svc.UpdateAs(&models.ProxyHost{ID: ok.ID, DomainNames: ok.DomainNames, ForwardHost: "localhost", ForwardPort: 2375}, true), ErrOrthrusListenerTarget)
}

func TestProxyHostWrite_RefusesListenerTargetInLocation(t *testing.T) {
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "a", 2375)
	useListenerSource(t, fakeListenerSource{})
	noLookup(t)
	svc := NewProxyHostService(db)

	host := &models.ProxyHost{
		DomainNames: "loc.example.com",
		ForwardHost: "10.0.0.5",
		ForwardPort: 8080,
		Locations:   []models.Location{{Path: "/api", ForwardHost: "127.0.0.1", ForwardPort: 2375}},
	}
	require.ErrorIs(t, svc.CreateAs(host, true), ErrOrthrusListenerTarget)
}

func TestProxyHostService_TestConnectionRefusesListenerTarget(t *testing.T) {
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "a", 2375)
	useListenerSource(t, fakeListenerSource{})
	noLookup(t)
	svc := NewProxyHostService(db)

	require.ErrorIs(t, svc.TestConnection("127.0.0.1", 2375), ErrOrthrusListenerTarget)
}

func intPtr(v int) *int { return &v }

func TestRefusesListenerTarget_NonLocalAddressLiteralsOnListenerPort(t *testing.T) {
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "a", 2375)
	useListenerSource(t, fakeListenerSource{})
	noLookup(t)

	for _, host := range []string{"2001:db8::10", "[2001:db8::10]", "192.0.2.10", "::ffff:192.0.2.10"} {
		t.Run(host, func(t *testing.T) {
			refused, err := refusesListenerTarget(db, host, 2375)
			require.NoError(t, err)
			assert.False(t, refused)
		})
	}
}

func TestRefusesListenerTarget_MappedAndBracketedLoopbackOnListenerPort(t *testing.T) {
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "a", 2375)
	useListenerSource(t, fakeListenerSource{})
	noLookup(t)

	for _, host := range []string{"::ffff:127.0.0.1", "[::ffff:7f00:1]", "[::1]", "::", "127.0.0.1.", "LOCALHOST.", "http://[::1]:9/x"} {
		t.Run(host, func(t *testing.T) {
			refused, err := refusesListenerTarget(db, host, 2375)
			require.NoError(t, err)
			assert.True(t, refused)
		})
	}
}

// setHostSideAddrs replaces the host-side address source for one test. With no
// arguments it reports none, so the environment's real gateway never affects a
// test expectation.
func setHostSideAddrs(t *testing.T, addrs ...string) {
	t.Helper()
	prev := hostSideAddrs
	var parsed []netip.Addr
	for _, a := range addrs {
		parsed = append(parsed, netip.MustParseAddr(a))
	}
	hostSideAddrs = func() []netip.Addr { return parsed }
	t.Cleanup(func() { hostSideAddrs = prev })
}

func TestRefusesListenerTarget_AgentNameFormRefusedOnAnyPort(t *testing.T) {
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "a", 2375)
	useListenerSource(t, fakeListenerSource{})
	noLookup(t)

	for _, port := range []int{0, 8080, 2375} {
		got, err := refusesListenerTarget(db, "orthrus:agent-uuid", port)
		require.NoError(t, err)
		assert.True(t, got, "port=%d", port)
	}
	got, err := refusesListenerTarget(db, "  orthrus:agent-uuid", 8080)
	require.NoError(t, err)
	assert.True(t, got)
}

func TestRefuseOrthrusListenerTarget_AgentNameFormForEveryRole(t *testing.T) {
	db := setupListenerPolicyDB(t)
	useListenerSource(t, fakeListenerSource{})
	noLookup(t)
	svc := NewProxyHostService(db)

	for _, admin := range []bool{false, true} {
		host := &models.ProxyHost{DomainNames: fmt.Sprintf("agent-form-%t.example.com", admin), ForwardHost: "orthrus:agent-uuid", ForwardPort: 8080}
		require.ErrorIs(t, svc.CreateAs(host, admin), ErrAgentForwardTarget, "admin=%t", admin)
	}

	loc := &models.ProxyHost{
		DomainNames: "agent-loc.example.com",
		ForwardHost: "10.0.0.5",
		ForwardPort: 8080,
		Locations:   []models.Location{{Path: "/x", ForwardHost: "orthrus:agent-uuid", ForwardPort: 8080}},
	}
	require.ErrorIs(t, svc.CreateAs(loc, false), ErrAgentForwardTarget)

	require.ErrorIs(t, svc.TestConnection("orthrus:agent-uuid", 8080), ErrAgentForwardTarget)
}

func TestListenerTargetGuard_RefusesStoredAgentNameRows(t *testing.T) {
	db := setupListenerPolicyDB(t)
	useListenerSource(t, fakeListenerSource{})
	noLookup(t)

	// A row written before the form was refused is still refused at apply time.
	stored := &models.ProxyHost{DomainNames: "stored-agent.example.com", ForwardHost: "orthrus:agent-uuid", ForwardPort: 8080}
	require.NoError(t, db.Omit("Locations").Create(stored).Error)

	refused, err := NewListenerTargetGuard(db).RefusesForwardTarget(stored.ForwardHost, stored.ForwardPort)
	require.NoError(t, err)
	assert.True(t, refused)
}

func TestRefusesListenerTarget_HostSideAddressesOnListenerPort(t *testing.T) {
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "a", 2375)
	useListenerSource(t, fakeListenerSource{})
	setHostSideAddrs(t, "172.18.0.1", "192.168.0.1")
	stubLookup(t, func(_ context.Context, name string) ([]net.IPAddr, error) {
		if name == "gateway-alias" {
			return []net.IPAddr{{IP: net.ParseIP("172.18.0.1")}}, nil
		}
		return nil, errors.New("no such host")
	})

	tests := []struct {
		name string
		host string
		port int
		want bool
	}{
		{"bridge gateway literal", "172.18.0.1", 2375, true},
		{"default route gateway literal", "192.168.0.1", 2375, true},
		{"name resolving to a host-side address", "gateway-alias", 2375, true},
		{"host gateway name", "host.docker.internal", 2375, true},
		{"host-side address on another port", "172.18.0.1", 8080, false},
		{"other network address on listener port", "172.18.0.9", 2375, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := refusesListenerTarget(db, tt.host, tt.port)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRefusesListenerTarget_UnresolvableNameFailsClosedOnlyOnListenerPort(t *testing.T) {
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "a", 2375)
	useListenerSource(t, fakeListenerSource{})
	stubLookup(t, func(context.Context, string) ([]net.IPAddr, error) {
		return nil, errors.New("temporary failure")
	})

	refused, err := refusesListenerTarget(db, "unknown-service", 2375)
	require.NoError(t, err)
	assert.True(t, refused, "unresolvable name on a listener port is refused")

	refused, err = refusesListenerTarget(db, "unknown-service", 8080)
	require.NoError(t, err)
	assert.False(t, refused, "the lookup does not gate ordinary ports")
}

func TestRefusesListenerTarget_EmptyResolutionFailsClosed(t *testing.T) {
	db := setupListenerPolicyDB(t)
	seedAgentPort(t, db, "a", 2375)
	useListenerSource(t, fakeListenerSource{})
	stubLookup(t, func(context.Context, string) ([]net.IPAddr, error) { return nil, nil })

	refused, err := refusesListenerTarget(db, "empty-answer", 2375)
	require.NoError(t, err)
	assert.True(t, refused)
}

func TestLookupForwardHost_DefaultResolverReturnsLiteral(t *testing.T) {
	// An address literal is answered without a DNS query, so this runs offline.
	addrs, err := lookupForwardHost(context.Background(), "127.0.0.1")
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	assert.True(t, addrs[0].IP.Equal(net.ParseIP("127.0.0.1")))
}

func TestHostSideAddrs_StubIsUsed(t *testing.T) {
	setHostSideAddrs(t, "10.9.9.1")
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("10.9.9.1")}, hostSideAddrs())
}

// Imports and setup write through Create and Update, which share the write checks.
func TestProxyHostCreateAndUpdate_RefuseAgentNameFormForImports(t *testing.T) {
	db := setupListenerPolicyDB(t)
	useListenerSource(t, fakeListenerSource{})
	noLookup(t)
	svc := NewProxyHostService(db)

	imported := &models.ProxyHost{DomainNames: "imported-agent.example.com", ForwardHost: "orthrus:agent-uuid", ForwardPort: 8080}
	require.ErrorIs(t, svc.Create(imported), ErrAgentForwardTarget)

	ok := &models.ProxyHost{DomainNames: "imported-ok.example.com", ForwardHost: "10.0.0.5", ForwardPort: 8080}
	require.NoError(t, svc.Create(ok))
	ok.ForwardHost = "orthrus:agent-uuid"
	require.ErrorIs(t, svc.Update(ok), ErrAgentForwardTarget)
}
