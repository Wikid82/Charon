package caddy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/network"
)

// notSetsOfFirstMatch returns the "not" entries of the single matcher set in the first route of a deny handler, as
// Caddy receives them. It fails when the route has no match or the set has no "not".
func notSetsOfFirstMatch(t *testing.T, h Handler) []map[string]any {
	t.Helper()
	b, err := json.Marshal(h)
	require.NoError(t, err)
	var decoded struct {
		Routes []struct {
			Match []map[string]any `json:"match"`
		} `json:"routes"`
	}
	require.NoError(t, json.Unmarshal(b, &decoded))
	require.Len(t, decoded.Routes, 1)
	require.Len(t, decoded.Routes[0].Match, 1)
	raw, ok := decoded.Routes[0].Match[0]["not"].([]any)
	require.True(t, ok, "the matcher set has a not list")
	var sets []map[string]any
	for _, entry := range raw {
		set, ok := entry.(map[string]any)
		require.True(t, ok)
		sets = append(sets, set)
	}
	return sets
}

// requireAdminSetFirst checks that the first "not" entry is the admin set with the given ranges.
func requireAdminSetFirst(t *testing.T, sets []map[string]any, ranges []string) {
	t.Helper()
	require.NotEmpty(t, sets)
	remote, ok := sets[0]["remote_ip"].(map[string]any)
	require.True(t, ok, "the admin set is the first not entry")
	var got []string
	for _, r := range remote["ranges"].([]any) {
		got = append(got, r.(string))
	}
	require.Equal(t, ranges, got)
}

func TestCountryListAdminExclusionSemantics(t *testing.T) {
	const admin = "10.0.0.0/8"
	tests := []struct {
		name     string
		listType string
		codes    string
		admin    string
		cases    []refusalCase
	}{
		{
			name:     "allow-list: listed country and admin address pass; other visitors refused",
			listType: "geo_whitelist",
			codes:    "DE",
			admin:    admin,
			cases: []refusalCase{
				{addr: "198.51.100.7", country: "DE"},
				{addr: "198.51.100.7", country: "US", refused: true},
				{addr: "10.1.1.1", country: "US"},
				{addr: "198.51.100.7", country: "", refused: true},
				{addr: "10.1.1.1", country: ""},
			},
		},
		{
			name:     "block-list: listed country refused unless admin; other countries allowed",
			listType: "geo_blacklist",
			codes:    "RU",
			admin:    admin,
			cases: []refusalCase{
				{addr: "198.51.100.7", country: "RU", refused: true},
				{addr: "10.1.1.1", country: "RU"},
				{addr: "198.51.100.7", country: "US"},
			},
		},
		{
			name:     "allow-list without admin: only listed country passes",
			listType: "geo_whitelist",
			codes:    "DE",
			admin:    "",
			cases: []refusalCase{
				{addr: "198.51.100.7", country: "DE"},
				{addr: "198.51.100.7", country: "US", refused: true},
				{addr: "10.1.1.1", country: "US", refused: true},
			},
		},
		{
			name:     "block-list without admin: listed country refused everywhere",
			listType: "geo_blacklist",
			codes:    "RU",
			admin:    "",
			cases: []refusalCase{
				{addr: "10.1.1.1", country: "RU", refused: true},
				{addr: "198.51.100.7", country: "US"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acl := &models.AccessList{Type: tt.listType, CountryCodes: tt.codes, Enabled: true}
			handler, err := buildACLHandler(acl, tt.admin)
			require.NoError(t, err)
			assertHandlerRefusals(t, handler, tt.cases)

			// The same list attached to a host, with the database available, through the full configuration.
			hostACL := models.AccessList{ID: 11, Name: "countries", Type: tt.listType, CountryCodes: tt.codes, Enabled: true}
			host := models.ProxyHost{
				UUID:         "geo-admin-host",
				DomainNames:  "geo-admin.example.com",
				Enabled:      true,
				ForwardHost:  "app",
				ForwardPort:  8080,
				AccessListID: &hostACL.ID,
				AccessList:   &hostACL,
			}
			cfg, err := GenerateConfig([]models.ProxyHost{host}, t.TempDir(), "", "", "", false, false, false, false, true, tt.admin, nil, nil, nil, nil, nil, WithGeoIPDatabase(testGeoIPDBPath, true))
			require.NoError(t, err)
			assertHostRefusals(t, cfg, "geo-admin.example.com", tt.cases)
		})
	}
}

func TestCountryListMatcherSetShape(t *testing.T) {
	const expr = `{geoip2.country_code} in ["DE"]`
	const adminCIDR = "10.0.0.0/8"

	t.Run("allow-list puts the admin set first in not", func(t *testing.T) {
		h, err := buildACLHandler(&models.AccessList{Type: "geo_whitelist", CountryCodes: "DE"}, adminCIDR)
		require.NoError(t, err)
		sets := notSetsOfFirstMatch(t, h)
		require.Len(t, sets, 2)
		requireAdminSetFirst(t, sets, []string{adminCIDR})
		require.Equal(t, map[string]any{"expression": expr}, sets[1])
	})

	t.Run("allow-list without admin drops the admin set", func(t *testing.T) {
		h, err := buildACLHandler(&models.AccessList{Type: "geo_whitelist", CountryCodes: "DE"}, "")
		require.NoError(t, err)
		sets := notSetsOfFirstMatch(t, h)
		require.Equal(t, []map[string]any{{"expression": expr}}, sets)
	})

	t.Run("block-list keeps the expression and negates the admin set", func(t *testing.T) {
		h, err := buildACLHandler(&models.AccessList{Type: "geo_blacklist", CountryCodes: "DE"}, adminCIDR)
		require.NoError(t, err)
		b, err := json.Marshal(h)
		require.NoError(t, err)
		require.JSONEq(t, `{"handler":"subroute","routes":[{"match":[{"expression":"{geoip2.country_code} in [\"DE\"]","not":[{"remote_ip":{"ranges":["10.0.0.0/8"]}}]}],"handle":[{"handler":"static_response","status_code":403,"body":"Access denied: Geographic restriction"}],"terminal":true}]}`, string(b))
	})

	t.Run("block-list without admin has no not", func(t *testing.T) {
		h, err := buildACLHandler(&models.AccessList{Type: "geo_blacklist", CountryCodes: "DE"}, "")
		require.NoError(t, err)
		b, err := json.Marshal(h)
		require.NoError(t, err)
		require.NotContains(t, string(b), `"not"`)
	})
}

// A country list with no usable database is not enforced: its host is built without a country matcher, so no visitor
// is refused by it, with or without an admin whitelist.
func TestUnavailableDatabaseCountryListIsNotEnforced(t *testing.T) {
	t.Run("allow-list with admin whitelist lets every visitor through", func(t *testing.T) {
		hostACL := models.AccessList{ID: 12, Name: "countries", Type: "geo_whitelist", CountryCodes: "DE", Enabled: true}
		host := models.ProxyHost{
			UUID:         "unavail-host",
			DomainNames:  "unavail.example.com",
			Enabled:      true,
			ForwardHost:  "app",
			ForwardPort:  8080,
			AccessListID: &hostACL.ID,
			AccessList:   &hostACL,
		}
		cfg, err := GenerateConfig([]models.ProxyHost{host}, t.TempDir(), "", "", "", false, false, false, false, true, "10.0.0.0/8", nil, nil, nil, nil, nil, WithGeoIPDatabase(testGeoIPDBPath, false))
		require.NoError(t, err)
		assertHostRefusals(t, cfg, "unavail.example.com", []refusalCase{
			{addr: "198.51.100.7", country: "DE"},
			{addr: "198.51.100.7", country: "US"},
			{addr: "10.1.1.1", country: "US"},
			{addr: "198.51.100.7", country: ""},
		})
	})

	t.Run("block-list without admin whitelist blocks no one", func(t *testing.T) {
		hostACL := models.AccessList{ID: 13, Name: "countries", Type: "geo_blacklist", CountryCodes: "RU", Enabled: true}
		host := models.ProxyHost{
			UUID:         "unavail-none-host",
			DomainNames:  "unavail-none.example.com",
			Enabled:      true,
			ForwardHost:  "app",
			ForwardPort:  8080,
			AccessListID: &hostACL.ID,
			AccessList:   &hostACL,
		}
		cfg, err := GenerateConfig([]models.ProxyHost{host}, t.TempDir(), "", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil, WithGeoIPDatabase(testGeoIPDBPath, false))
		require.NoError(t, err)
		assertHostRefusals(t, cfg, "unavail-none.example.com", []refusalCase{
			{addr: "10.1.1.1", country: "RU"},
			{addr: "198.51.100.7", country: "RU"},
		})
	})
}

func TestLocalNetworkOnlyAdminExclusion(t *testing.T) {
	acl := &models.AccessList{Type: "whitelist", LocalNetworkOnly: true, Enabled: true}

	t.Run("admin address passes whatever its network", func(t *testing.T) {
		handler, err := buildACLHandler(acl, "203.0.113.0/24")
		require.NoError(t, err)
		assertHandlerRefusals(t, handler, []refusalCase{
			{addr: "192.168.1.5"},
			{addr: "203.0.113.9"},
			{addr: "198.51.100.7", refused: true},
		})
	})

	t.Run("without admin, public addresses are refused", func(t *testing.T) {
		handler, err := buildACLHandler(acl, "")
		require.NoError(t, err)
		assertHandlerRefusals(t, handler, []refusalCase{
			{addr: "192.168.1.5"},
			{addr: "203.0.113.9", refused: true},
		})
	})

	t.Run("admin set is first in not and carries the local network ranges", func(t *testing.T) {
		handler, err := buildACLHandler(acl, "203.0.113.0/24")
		require.NoError(t, err)
		sets := notSetsOfFirstMatch(t, handler)
		require.Len(t, sets, 2)
		requireAdminSetFirst(t, sets, []string{"203.0.113.0/24"})
		local, ok := sets[1]["remote_ip"].(map[string]any)
		require.True(t, ok)
		require.NotEmpty(t, local["ranges"])
		require.Contains(t, network.LocalNetworkCIDRs(), "10.0.0.0/8")
	})

	t.Run("without admin the match has only the local network set", func(t *testing.T) {
		handler, err := buildACLHandler(acl, "")
		require.NoError(t, err)
		sets := notSetsOfFirstMatch(t, handler)
		require.Len(t, sets, 1)
		require.Contains(t, sets[0], "remote_ip")
	})
}

func TestEmptyAdminWhitelistAddsNoExemption(t *testing.T) {
	h, err := buildACLHandler(&models.AccessList{Type: "whitelist", LocalNetworkOnly: true}, "  , ")
	require.NoError(t, err)
	require.Len(t, notSetsOfFirstMatch(t, h), 1, "a blank admin whitelist adds no admin set")
	assertHandlerRefusals(t, h, []refusalCase{
		{addr: "192.168.1.5"},
		{addr: "198.51.100.7", refused: true},
	})
}
