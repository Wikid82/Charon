package caddy

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/caddy/caddymatchtest"
	"github.com/Wikid82/charon/backend/internal/models"
)

// refusalCase is one probe and whether the route must refuse it.
type refusalCase struct {
	addr    string
	country string
	refused bool
}

// assertHandlerRefusals checks each probe against one handler, serialized as Caddy receives it.
func assertHandlerRefusals(t *testing.T, h Handler, cases []refusalCase) {
	t.Helper()
	b, err := json.Marshal(h)
	require.NoError(t, err)
	for _, c := range cases {
		probe := caddymatchtest.Probe{Addr: netip.MustParseAddr(c.addr), Country: c.country}
		require.Equal(t, c.refused, caddymatchtest.HandlerRefuses(t, b, probe), "handler: address %s", c.addr)
	}
}

// assertHostRefusals checks each probe against the host's catch-all route in a generated configuration.
func assertHostRefusals(t *testing.T, cfg *Config, domain string, cases []refusalCase) {
	t.Helper()
	b, err := json.Marshal(cfg)
	require.NoError(t, err)
	for _, c := range cases {
		probe := caddymatchtest.Probe{Addr: netip.MustParseAddr(c.addr), Country: c.country}
		require.Equal(t, c.refused, caddymatchtest.HostRefuses(t, b, domain, probe), "host %s: address %s", domain, c.addr)
	}
}

func TestDecisionsAdminExclusionSemantics(t *testing.T) {
	const admin = "10.0.0.0/8,2001:db8::/32"
	tests := []struct {
		name    string
		admin   string
		decided string
		cases   []refusalCase
	}{
		{
			name:    "listed address refused; others and admin addresses allowed",
			admin:   admin,
			decided: "1.2.3.4",
			cases: []refusalCase{
				{addr: "1.2.3.4", refused: true},
				{addr: "203.0.113.9"},
				{addr: "10.1.1.1"},
				{addr: "2001:db8::1"},
			},
		},
		{
			name:    "decision on an admin address does not refuse it",
			admin:   admin,
			decided: "10.0.0.5",
			cases: []refusalCase{
				{addr: "10.0.0.5"},
				{addr: "203.0.113.9"},
			},
		},
		{
			name:    "no admin whitelist: listed address refused, others allowed",
			admin:   "",
			decided: "1.2.3.4",
			cases: []refusalCase{
				{addr: "1.2.3.4", refused: true},
				{addr: "203.0.113.9"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hosts := []models.ProxyHost{
				{UUID: "dec-a", DomainNames: "a.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080},
				{UUID: "dec-b", DomainNames: "b.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080},
			}
			decisions := []models.SecurityDecision{{Action: "block", IP: tt.decided}}
			cfg, err := GenerateConfig(hosts, t.TempDir(), "", "", "", false, false, false, false, false, tt.admin, nil, nil, decisions, nil, nil)
			require.NoError(t, err)
			for _, domain := range []string{"a.example.com", "b.example.com"} {
				assertHostRefusals(t, cfg, domain, tt.cases)
			}
		})
	}
}

func TestIPBlockListAdminExclusionSemantics(t *testing.T) {
	tests := []struct {
		name  string
		admin string
		rules string
		cases []refusalCase
	}{
		{
			name:  "listed ranges refused; admin address inside a listed range allowed",
			admin: "10.0.0.0/8,2001:db8::/32",
			rules: `[{"cidr":"203.0.113.0/24"},{"cidr":"10.5.0.0/16"}]`,
			cases: []refusalCase{
				{addr: "203.0.113.9", refused: true},
				{addr: "198.51.100.1"},
				{addr: "10.1.1.1"},
				{addr: "10.5.0.1"},
				{addr: "2001:db8::1"},
			},
		},
		{
			name:  "no admin whitelist: listed address refused, unlisted allowed",
			admin: "",
			rules: `[{"cidr":"1.2.3.0/24"}]`,
			cases: []refusalCase{
				{addr: "1.2.3.4", refused: true},
				{addr: "10.5.0.1"},
			},
		},
		{
			name:  "IPv6 admin address is exempt from an IPv4 block",
			admin: "2001:db8::/32",
			rules: `[{"cidr":"203.0.113.0/24"}]`,
			cases: []refusalCase{
				{addr: "203.0.113.9", refused: true},
				{addr: "2001:db8::1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acl := &models.AccessList{Type: "blacklist", IPRules: tt.rules, Enabled: true}
			handler, err := buildACLHandler(acl, tt.admin)
			require.NoError(t, err)
			assertHandlerRefusals(t, handler, tt.cases)

			// The same list attached to a host, through the full configuration.
			hostACL := models.AccessList{ID: 9, Name: "block", Type: "blacklist", IPRules: tt.rules, Enabled: true}
			host := models.ProxyHost{
				UUID:         "blk-host",
				DomainNames:  "blk.example.com",
				Enabled:      true,
				ForwardHost:  "app",
				ForwardPort:  8080,
				AccessListID: &hostACL.ID,
				AccessList:   &hostACL,
			}
			cfg, err := GenerateConfig([]models.ProxyHost{host}, t.TempDir(), "", "", "", false, false, false, false, true, tt.admin, nil, nil, nil, nil, nil)
			require.NoError(t, err)
			assertHostRefusals(t, cfg, "blk.example.com", tt.cases)
		})
	}
}
