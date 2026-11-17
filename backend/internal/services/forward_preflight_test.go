package services

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/network"
)

// resolverAnswers returns a lookup stub that answers from a fixed table and counts its calls.
func resolverAnswers(calls *atomic.Int32, answers map[string][]string) func(context.Context, string) ([]net.IPAddr, error) {
	return func(_ context.Context, name string) ([]net.IPAddr, error) {
		calls.Add(1)
		raw, ok := answers[name]
		if !ok {
			return nil, errors.New("no such host")
		}
		var out []net.IPAddr
		for _, s := range raw {
			out = append(out, net.IPAddr{IP: net.ParseIP(s)})
		}
		return out, nil
	}
}

func TestPreflight_LinkLocalNameRefusedOnlyWhenResolved(t *testing.T) {
	var calls atomic.Int32
	stubLookup(t, resolverAnswers(&calls, map[string][]string{"imds.example": {"169.254.169.254"}}))
	isolateGatewayAddrs(t, nil)
	forwardNameCache.entries = map[string]cachedVerdict{}

	v := preflightForwardNames(context.Background(), []string{"imds.example:80"})
	require.Contains(t, v.names, "imds.example")
	assert.True(t, v.policy.Blocked(net.ParseIP("169.254.169.254")))
	db := setupProxyHostTestDB(t)
	err := refuseForwardTarget(db, "imds.example", 80, v)
	assert.ErrorIs(t, err, ErrForwardTargetAddress)
}

func TestPreflight_FailedLookupIsAcceptedOnOtherPorts(t *testing.T) {
	var calls atomic.Int32
	stubLookup(t, resolverAnswers(&calls, nil))
	isolateGatewayAddrs(t, nil)
	forwardNameCache.entries = map[string]cachedVerdict{}
	db := setupProxyHostTestDB(t)

	v := preflightForwardNames(context.Background(), []string{"unknown.example"})
	assert.Error(t, v.names["unknown.example"].err)
	assert.NoError(t, refuseForwardTarget(db, "unknown.example", 8080, v), "a failed lookup is accepted by the policy")
}

func TestPreflight_FailedLookupIsRefusedOnReservedPort(t *testing.T) {
	var calls atomic.Int32
	stubLookup(t, resolverAnswers(&calls, nil))
	isolateGatewayAddrs(t, nil)
	setHostSideAddrs(t)
	forwardNameCache.entries = map[string]cachedVerdict{}
	db := setupProxyHostTestDB(t)

	v := preflightForwardNames(context.Background(), []string{"unknown.example"})
	assert.ErrorIs(t, refuseForwardTarget(db, "unknown.example", 2019, v), ErrOrthrusListenerTarget,
		"a name whose address cannot be established is refused on a reserved port")
}

func TestPreflight_NameResolvingToThisMachineRefusedOnReservedPort(t *testing.T) {
	var calls atomic.Int32
	stubLookup(t, resolverAnswers(&calls, map[string][]string{"self.example": {"127.0.0.1"}}))
	isolateGatewayAddrs(t, nil)
	setHostSideAddrs(t)
	forwardNameCache.entries = map[string]cachedVerdict{}
	db := setupProxyHostTestDB(t)

	v := preflightForwardNames(context.Background(), []string{"self.example"})
	assert.ErrorIs(t, refuseForwardTarget(db, "self.example", 2019, v), ErrOrthrusListenerTarget)
	assert.NoError(t, refuseForwardTarget(db, "self.example", 8080, v))
}

func TestPreflight_NoLookupInsideWriteLock(t *testing.T) {
	var calls atomic.Int32
	stubLookup(t, resolverAnswers(&calls, map[string][]string{"x.example": {"10.0.0.5"}}))
	isolateGatewayAddrs(t, nil)
	db := setupProxyHostTestDB(t)

	// With no verdict for a name, the in-lock check must not look the name up.
	err := refuseForwardTarget(db, "x.example", 2019, forwardVerdicts{policy: probePolicy()})
	require.NoError(t, err)
	assert.Zero(t, calls.Load(), "the in-lock check ran a name lookup")
}

func TestPreflight_NameLookupsRunOnceAndAreCached(t *testing.T) {
	var calls atomic.Int32
	stubLookup(t, resolverAnswers(&calls, map[string][]string{"cached.example": {"10.0.0.7"}}))
	isolateGatewayAddrs(t, nil)
	forwardNameCache.entries = map[string]cachedVerdict{}

	preflightForwardNames(context.Background(), []string{"cached.example", "cached.example:443", "http://cached.example/x"})
	preflightForwardNames(context.Background(), []string{"cached.example"})
	assert.Equal(t, int32(1), calls.Load(), "one lookup per name, reused within the TTL")
}

func TestPreflight_SkipsLiteralsAgentFormAndThisMachineNames(t *testing.T) {
	var calls atomic.Int32
	stubLookup(t, resolverAnswers(&calls, nil))
	isolateGatewayAddrs(t, nil)
	forwardNameCache.entries = map[string]cachedVerdict{}

	v := preflightForwardNames(context.Background(), []string{"10.0.0.1", "0x7f.1", "orthrus:agent", "localhost", "host.docker.internal", ""})
	assert.Empty(t, v.names)
	assert.Zero(t, calls.Load())
}

func TestPreflight_BudgetExhaustedAcceptsUncachedAndRefusesOnReservedPort(t *testing.T) {
	var calls atomic.Int32
	stubLookup(t, resolverAnswers(&calls, map[string][]string{"late.example": {"169.254.1.9"}}))
	isolateGatewayAddrs(t, nil)
	forwardNameCache.entries = map[string]cachedVerdict{}
	db := setupProxyHostTestDB(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	v := preflightForwardNames(ctx, []string{"late.example"})
	assert.Zero(t, calls.Load(), "no lookup starts after the budget is spent")
	require.Error(t, v.names["late.example"].err)
	assert.NoError(t, refuseForwardTarget(db, "late.example", 8080, v), "accepted by the policy once the budget is spent")
	assert.Empty(t, forwardNameCache.entries, "a budget failure is not cached")
}

func TestVerdictCache_TTLs(t *testing.T) {
	now := time.Unix(1000, 0)
	c := &verdictCache{now: func() time.Time { return now }, entries: map[string]cachedVerdict{}}
	c.put("ok.example", nameVerdict{addrs: []netip.Addr{netip.MustParseAddr("10.0.0.1")}})
	c.put("bad.example", nameVerdict{err: errNoAddresses})

	now = now.Add(20 * time.Second)
	_, okHit := c.get("ok.example")
	_, badHit := c.get("bad.example")
	assert.True(t, okHit, "positive verdicts last 30 s")
	assert.False(t, badHit, "negative verdicts last 10 s")

	now = now.Add(11 * time.Second)
	_, okHit = c.get("ok.example")
	assert.False(t, okHit)
}

func TestRefuseForwardTarget_LiteralPolicyForEveryClass(t *testing.T) {
	db := setupProxyHostTestDB(t)
	setHostSideAddrs(t)
	cases := map[string]bool{
		"169.254.169.254":          true,
		"[::ffff:169.254.169.254]": true,
		"fd00:ec2::254":            true,
		"100.100.100.200":          true,
		"168.63.129.16":            true,
		"0.0.0.0":                  false, // loopback-equivalent on an ordinary port (see forwardLiteralBlocked)
		"10.20.30.40":              false,
		"192.168.1.10":             false,
		"100.64.0.5":               false,
		"fd12::5":                  false,
		"198.18.0.5":               false,
		"8.8.8.8":                  false,
	}
	for raw, refused := range cases {
		err := refuseForwardTarget(db, raw, 8080, verdictsFor(t, raw))
		if refused {
			assert.ErrorIs(t, err, ErrForwardTargetAddress, raw)
		} else {
			assert.NoError(t, err, raw)
		}
	}
}

func TestRefuseForwardTarget_GatewayCarveOutAllowedButReservedPortRefused(t *testing.T) {
	db := setupProxyHostTestDB(t)
	setHostSideAddrs(t)
	gw := []netip.Addr{netip.MustParseAddr("169.254.1.2")}
	isolateGatewayAddrs(t, gw)
	v := preflightForwardNames(context.Background(), []string{"169.254.1.2"})
	assert.NoError(t, refuseForwardTarget(db, "169.254.1.2", 8080, v))
	assert.ErrorIs(t, refuseForwardTarget(db, "169.254.1.2", 2019, v), ErrOrthrusListenerTarget)
	assert.ErrorIs(t, refuseForwardTarget(db, "169.254.1.3", 8080, v), ErrForwardTargetAddress)
}

func TestRefuseForwardTarget_ReservedPortsOnThisMachine(t *testing.T) {
	db := setupProxyHostTestDB(t)
	setHostSideAddrs(t)
	resetEmergency(t)
	useListenerSource(t, fakeListenerSource{})
	v := verdictsFor(t)
	assert.ErrorIs(t, refuseForwardTarget(db, "localhost", 2019, v), ErrOrthrusListenerTarget, "admin port")
	assert.ErrorIs(t, refuseForwardTarget(db, "127.0.0.1", 8085, v), ErrOrthrusListenerTarget, "CrowdSec LAPI port")
	assert.NoError(t, refuseForwardTarget(db, "127.0.0.1", 2020, v), "emergency port is free while disabled")
	SetEmergencyListener(config.EmergencyConfig{Enabled: true, BindAddress: "127.0.0.1:2020"})
	assert.ErrorIs(t, refuseForwardTarget(db, "127.0.0.1", 2020, v), ErrOrthrusListenerTarget, "emergency port while enabled")
	assert.NoError(t, refuseForwardTarget(db, "10.1.2.3", 2019, v), "a remote machine on a reserved port is allowed")
	assert.ErrorIs(t, refuseForwardTarget(db, "orthrus:agent", 2019, v), ErrAgentForwardTarget)
}

func TestRefuseForwardTarget_EmptyHostAndNoPortPass(t *testing.T) {
	db := setupProxyHostTestDB(t)
	assert.NoError(t, refuseForwardTarget(db, "  ", 2019, forwardVerdicts{policy: network.AddressPolicy{}}))
	assert.NoError(t, refuseForwardTarget(db, "10.0.0.1", 0, verdictsFor(t)))
}

func TestRefuseForwardTarget_DatabaseErrorIsNotAnAddressRefusal(t *testing.T) {
	db := setupProxyHostTestDB(t)
	setHostSideAddrs(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	err = refuseForwardTarget(db, "127.0.0.1", 2019, verdictsFor(t))
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrOrthrusListenerTarget))
	assert.False(t, errors.Is(err, ErrForwardTargetAddress))
}

func TestAdvancedConfigTargets_CreateRefusesEachBadDial(t *testing.T) {
	setHostSideAddrs(t)
	isolateGatewayAddrs(t, nil)
	useListenerSource(t, fakeListenerSource{})
	resetEmergency(t)
	cases := map[string]string{
		"link-local":     `{"handler":"reverse_proxy","upstreams":[{"dial":"169.254.169.254:80"}]}`,
		"placeholder":    `{"handler":"reverse_proxy","upstreams":[{"dial":"{http.request.header.X}:80"}]}`,
		"unix socket":    `{"handler":"reverse_proxy","upstreams":[{"dial":"unix//app/data/run/caddy-admin.sock"}]}`,
		"reserved range": `{"handler":"reverse_proxy","upstreams":[{"dial":"tcp/127.0.0.1:2000-2100"}]}`,
		"admin port":     `{"handler":"reverse_proxy","upstreams":[{"dial":"127.0.0.1:2019"}]}`,
		"dynamic":        `{"handler":"reverse_proxy","dynamic_upstreams":{"source":"a"}}`,
		"resolver":       `{"handler":"reverse_proxy","transport":{"protocol":"http","resolver":{"addresses":["10.0.0.2:53"]}}}`,
		"nested":         `{"handler":"subroute","routes":[{"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"169.254.1.5:80"}]}]}]}`,
		"not json shape": `{"handler":`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			db := setupProxyHostTestDB(t)
			svc := NewProxyHostService(db)
			host := &models.ProxyHost{DomainNames: name + ".example.com", ForwardHost: "10.0.0.5", ForwardPort: 8080, AdvancedConfig: raw}
			require.Error(t, svc.CreateAs(host, true))
			var count int64
			require.NoError(t, db.Model(&models.ProxyHost{}).Count(&count).Error)
			assert.Zero(t, count, "a refused advanced config is not stored")
		})
	}
}

func TestAdvancedConfigTargets_StaticUpstreamAccepted(t *testing.T) {
	setHostSideAddrs(t)
	isolateGatewayAddrs(t, nil)
	useListenerSource(t, fakeListenerSource{})
	db := setupProxyHostTestDB(t)
	svc := NewProxyHostService(db)
	host := &models.ProxyHost{
		DomainNames:    "adv-ok.example.com",
		ForwardHost:    "10.0.0.5",
		ForwardPort:    8080,
		AdvancedConfig: `{"handler":"reverse_proxy","upstreams":[{"dial":"10.0.0.9:9000"}]}`,
	}
	require.NoError(t, svc.CreateAs(host, true))
}

func TestRefuseAdvancedConfigTargets_EmptyAndInvalid(t *testing.T) {
	db := setupProxyHostTestDB(t)
	assert.NoError(t, refuseAdvancedConfigTargets(db, "", forwardVerdicts{}))
	assert.Error(t, refuseAdvancedConfigTargets(db, "{", forwardVerdicts{}))
	assert.Empty(t, advancedDialHosts("{"), "invalid JSON contributes no dial hosts")
}

func TestForwardNamesOf_CollectsAllTargets(t *testing.T) {
	host := &models.ProxyHost{
		ForwardHost:    "app.example",
		AdvancedConfig: `{"upstreams":[{"dial":"other.example:80"},{"dial":"unix//x"}]}`,
		Locations:      []models.Location{{ForwardHost: "loc.example"}},
	}
	names := forwardNamesOf(host)
	assert.Equal(t, []string{"app.example", "loc.example", "other.example"}, names)
}

func TestListenerTargetGuard_LiteralPolicyAndAdvancedDials(t *testing.T) {
	db := setupProxyHostTestDB(t)
	setHostSideAddrs(t)
	isolateGatewayAddrs(t, nil)
	resetEmergency(t)
	useListenerSource(t, fakeListenerSource{})
	guard := NewListenerTargetGuard(db)

	refused, err := guard.RefusesForwardTarget("169.254.169.254", 80)
	require.NoError(t, err)
	assert.True(t, refused, "link-local literal in a stored row")
	refused, err = guard.RefusesForwardTarget("10.0.0.5", 80)
	require.NoError(t, err)
	assert.False(t, refused)
	refused, err = guard.RefusesForwardTarget("127.0.0.1", 2019)
	require.NoError(t, err)
	assert.True(t, refused, "admin port on this machine")

	var nilGuard *ListenerTargetGuard
	refused, err = nilGuard.RefusesAdvancedDial("169.254.1.5:80")
	require.NoError(t, err)
	assert.False(t, refused, "a nil guard refuses nothing")

	for dial, want := range map[string]bool{
		"169.254.1.5:80":                true,
		"{http.request.header.X}:80":    true,
		"unix//app/data/run/caddy.sock": true,
		"tcp/10.0.0.5:2000-2100":        false,
		"tcp/127.0.0.1:2000-2100":       true,
		"10.0.0.9:9000":                 false,
	} {
		refusedDial, dialErr := guard.RefusesAdvancedDial(dial)
		require.NoError(t, dialErr, dial)
		assert.Equal(t, want, refusedDial, dial)
	}

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = guard.RefusesAdvancedDial("127.0.0.1:2000-2100")
	assert.Error(t, err, "a database error is returned to the generator")
}

func TestAdvancedConfigTargets_BracketedIPv6Dials(t *testing.T) {
	setHostSideAddrs(t)
	isolateGatewayAddrs(t, nil)
	resetEmergency(t)
	useListenerSource(t, fakeListenerSource{})
	cases := map[string]bool{
		`{"upstreams":[{"dial":"[fd00:ec2::254]:80"}]}`: true,  // instance service stays refused
		`{"upstreams":[{"dial":"[::1]:2019"}]}`:         true,  // reserved port on this machine
		`{"upstreams":[{"dial":"[fd12::5]:8080"}]}`:     false, // unique local is allowed
		`{"upstreams":[{"dial":"tcp/[fd12::5]:8080"}]}`: false,
	}
	for raw, refused := range cases {
		db := setupProxyHostTestDB(t)
		svc := NewProxyHostService(db)
		host := &models.ProxyHost{UUID: "v6-" + strconvQuote(raw), DomainNames: "v6-" + strconvQuote(raw) + ".example.com", ForwardHost: "10.0.0.5", ForwardPort: 8080, AdvancedConfig: raw}
		err := svc.CreateAs(host, true)
		if refused {
			assert.Error(t, err, raw)
		} else {
			assert.NoError(t, err, raw)
		}
	}
}

// strconvQuote gives a short, unique token for a test host name.
func strconvQuote(raw string) string {
	out := make([]byte, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		if (raw[i] >= 'a' && raw[i] <= 'z') || (raw[i] >= '0' && raw[i] <= '9') {
			out = append(out, raw[i])
		}
	}
	return string(out)
}
