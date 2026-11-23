package caddy

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/caddy/caddymatchtest"
	"github.com/Wikid82/charon/backend/internal/models"
)

// probeConfig generates the configuration for hosts with the given admin allow-list and returns its JSON.
func probeConfig(t *testing.T, hosts []models.ProxyHost, adminWhitelist string, opts ...GenerateConfigOption) []byte {
	t.Helper()
	cfg, err := GenerateConfig(hosts, "/data/caddy/data", "", "/frontend/dist", "", false,
		false, false, false, true, adminWhitelist, nil, nil, nil, nil, nil, opts...)
	require.NoError(t, err)
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	return raw
}

func aclHost(domain string, list *models.AccessList) models.ProxyHost {
	id := uint(1)
	return models.ProxyHost{
		UUID:         "probe-" + domain,
		DomainNames:  domain,
		ForwardHost:  "app.internal",
		ForwardPort:  8000,
		Enabled:      true,
		AccessListID: &id,
		AccessList:   list,
	}
}

// TestProxyChecksJudgeTheVisitorNotThePeer: the proxy's checks use the visitor address Caddy decided on. A trusted
// proxy's own address never decides, and the visitor's address decides wherever the peer is trusted.
func TestProxyChecksJudgeTheVisitorNotThePeer(t *testing.T) {
	blocked := &models.AccessList{Name: "block", Type: "blacklist", IPRules: `[{"cidr":"203.0.113.0/24"}]`, Enabled: true}
	raw := probeConfig(t, []models.ProxyHost{aclHost("blocked.example.com", blocked)}, "")

	tests := []struct {
		name    string
		probe   caddymatchtest.Probe
		refused bool
	}{
		{
			name:    "trusted proxy forwards a blocked visitor",
			probe:   caddymatchtest.Probe{Addr: netip.MustParseAddr("10.0.0.5"), Client: netip.MustParseAddr("203.0.113.9")},
			refused: true,
		},
		{
			name:    "trusted proxy forwards an allowed visitor",
			probe:   caddymatchtest.Probe{Addr: netip.MustParseAddr("10.0.0.5"), Client: netip.MustParseAddr("198.51.100.7")},
			refused: false,
		},
		{
			name:    "a blocked address as the peer does not decide when the visitor is allowed",
			probe:   caddymatchtest.Probe{Addr: netip.MustParseAddr("203.0.113.9"), Client: netip.MustParseAddr("198.51.100.7")},
			refused: false,
		},
		{
			name:    "a direct visitor is judged by the connection address",
			probe:   caddymatchtest.Probe{Addr: netip.MustParseAddr("203.0.113.9")},
			refused: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.refused, caddymatchtest.HostRefuses(t, raw, "blocked.example.com", tt.probe))
		})
	}
}

// TestLoopbackPeerWithForwardedAddressIsJudgedByTheVisitor: a loopback peer (a same-host tunnel) that forwards a
// visitor is trusted, so the visitor address decides a local-network-only list.
func TestLoopbackPeerWithForwardedAddressIsJudgedByTheVisitor(t *testing.T) {
	local := &models.AccessList{Name: "local", Type: "whitelist", LocalNetworkOnly: true, Enabled: true}
	raw := probeConfig(t, []models.ProxyHost{aclHost("local.example.com", local)}, "")

	loopback := netip.MustParseAddr("127.0.0.1")
	require.True(t, caddymatchtest.HostRefuses(t, raw, "local.example.com",
		caddymatchtest.Probe{Addr: loopback, Client: netip.MustParseAddr("198.51.100.7")}),
		"a public visitor behind a loopback tunnel is refused by a local-network-only list")
	require.False(t, caddymatchtest.HostRefuses(t, raw, "local.example.com",
		caddymatchtest.Probe{Addr: loopback, Client: netip.MustParseAddr("192.168.1.20")}),
		"a private visitor behind a loopback tunnel is allowed")
}

// TestAdminAllowListExemptsByVisitorAddress: an admin allow-list address stays exempt when it is the visitor.
func TestAdminAllowListExemptsByVisitorAddress(t *testing.T) {
	blocked := &models.AccessList{Name: "block", Type: "blacklist", IPRules: `[{"cidr":"203.0.113.0/24"}]`, Enabled: true}
	raw := probeConfig(t, []models.ProxyHost{aclHost("admin.example.com", blocked)}, "203.0.113.10")

	require.False(t, caddymatchtest.HostRefuses(t, raw, "admin.example.com",
		caddymatchtest.Probe{Addr: netip.MustParseAddr("10.0.0.5"), Client: netip.MustParseAddr("203.0.113.10")}))
	require.True(t, caddymatchtest.HostRefuses(t, raw, "admin.example.com",
		caddymatchtest.Probe{Addr: netip.MustParseAddr("10.0.0.5"), Client: netip.MustParseAddr("203.0.113.11")}))
}

// TestCountryHandlerReadsTheVisitorAfterTheHeaderSet: the country lookup runs after X-Forwarded-For is set to the
// visitor, so a trusted proxy's forwarded visitor decides the country. The walk fails if the lookup comes first.
func TestCountryHandlerReadsTheVisitorAfterTheHeaderSet(t *testing.T) {
	geo := &models.AccessList{Name: "geo", Type: "geo_blacklist", CountryCodes: "RU", Enabled: true}
	raw := probeConfig(t, []models.ProxyHost{aclHost("geo.example.com", geo)}, "",
		WithGeoIPDatabase("/data/geoip/GeoLite2-Country.mmdb", true))

	require.True(t, caddymatchtest.HostRefuses(t, raw, "geo.example.com",
		caddymatchtest.Probe{Addr: netip.MustParseAddr("10.0.0.5"), Client: netip.MustParseAddr("198.51.100.7"), Country: "RU"}))
	require.False(t, caddymatchtest.HostRefuses(t, raw, "geo.example.com",
		caddymatchtest.Probe{Addr: netip.MustParseAddr("10.0.0.5"), Client: netip.MustParseAddr("198.51.100.7"), Country: "US"}))
}
