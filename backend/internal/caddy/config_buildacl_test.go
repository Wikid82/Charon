package caddy

import (
	"encoding/json"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestBuildACLHandler_GeoWhitelist(t *testing.T) {
	acl := &models.AccessList{Type: "geo_whitelist", CountryCodes: "US,CA", Enabled: true}
	h, err := buildACLHandler(acl, "")
	require.NoError(t, err)
	require.NotNil(t, h)

	// Ensure it contains static_response status_code 403
	b, _ := json.Marshal(h)
	require.Contains(t, string(b), "Access denied: Geographic restriction")
}

func TestBuildACLHandler_LocalNetwork(t *testing.T) {
	acl := &models.AccessList{Type: "whitelist", LocalNetworkOnly: true, Enabled: true}
	h, err := buildACLHandler(acl, "")
	require.NoError(t, err)
	require.NotNil(t, h)
	b, _ := json.Marshal(h)
	require.Contains(t, string(b), "Access denied: Not a local network IP")
}

func TestBuildACLHandler_IPRules(t *testing.T) {
	rules := `[ {"cidr": "192.168.1.0/24", "description": "local"} ]`
	acl := &models.AccessList{Type: "blacklist", IPRules: rules, Enabled: true}
	h, err := buildACLHandler(acl, "")
	require.NoError(t, err)
	require.NotNil(t, h)
	b, _ := json.Marshal(h)
	require.Contains(t, string(b), "Access denied: IP blacklisted")
}

func TestBuildACLHandler_InvalidIPJSON(t *testing.T) {
	acl := &models.AccessList{Type: "blacklist", IPRules: `invalid-json`, Enabled: true}
	h, err := buildACLHandler(acl, "")
	require.Error(t, err)
	require.Nil(t, h)
}

func TestBuildACLHandler_NoIPRulesReturnsNil(t *testing.T) {
	acl := &models.AccessList{Type: "blacklist", IPRules: `[]`, Enabled: true}
	h, err := buildACLHandler(acl, "")
	require.NoError(t, err)
	require.Nil(t, h)
}

func TestBuildACLHandler_Whitelist(t *testing.T) {
	rules := `[ { "cidr": "192.168.1.0/24", "description": "local" } ]`
	acl := &models.AccessList{Type: "whitelist", IPRules: rules, Enabled: true}
	h, err := buildACLHandler(acl, "")
	require.NoError(t, err)
	require.NotNil(t, h)
	b, _ := json.Marshal(h)
	require.Contains(t, string(b), "Access denied: IP not in whitelist")
}
