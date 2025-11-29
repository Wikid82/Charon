package caddy

import (
	"encoding/json"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestGenerateConfig_CatchAllFrontend(t *testing.T) {
	cfg, err := GenerateConfig([]models.ProxyHost{}, "/tmp/caddy-data", "", "/frontend/dist", "", false)
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	require.Len(t, server.Routes, 1)
	r := server.Routes[0]
	// Expect first handler is rewrite to unknown.html
	require.Equal(t, "rewrite", r.Handle[0]["handler"])
}

func TestGenerateConfig_AdvancedInvalidJSON(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:           "adv1",
			DomainNames:    "adv.example.com",
			ForwardHost:    "app",
			ForwardPort:    8080,
			Enabled:        true,
			AdvancedConfig: "{invalid-json",
		},
	}

	cfg, err := GenerateConfig(hosts, "/tmp/caddy-data", "", "", "", false)
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	// Main route should still have ReverseProxy as last handler
	require.Len(t, server.Routes, 1)
	route := server.Routes[0]
	last := route.Handle[len(route.Handle)-1]
	require.Equal(t, "reverse_proxy", last["handler"])
}

func TestGenerateConfig_AdvancedArrayHandler(t *testing.T) {
	array := []map[string]interface{}{{
		"handler": "headers",
		"response": map[string]interface{}{
			"set": map[string][]string{"X-Test": {"1"}},
		},
	}}
	raw, _ := json.Marshal(array)

	hosts := []models.ProxyHost{
		{
			UUID:           "adv2",
			DomainNames:    "arr.example.com",
			ForwardHost:    "app",
			ForwardPort:    8080,
			Enabled:        true,
			AdvancedConfig: string(raw),
		},
	}

	cfg, err := GenerateConfig(hosts, "/tmp/caddy-data", "", "", "", false)
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	route := server.Routes[0]
	// First handler should be our headers handler
	first := route.Handle[0]
	require.Equal(t, "headers", first["handler"])
}

func TestGenerateConfig_LowercaseDomains(t *testing.T) {
	hosts := []models.ProxyHost{
		{UUID: "d1", DomainNames: "UPPER.EXAMPLE.COM", ForwardHost: "a", ForwardPort: 80, Enabled: true},
	}
	cfg, err := GenerateConfig(hosts, "/tmp/caddy-data", "", "", "", false)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	require.Equal(t, []string{"upper.example.com"}, route.Match[0].Host)
}

func TestGenerateConfig_AdvancedObjectHandler(t *testing.T) {
	host := models.ProxyHost{
		UUID:           "advobj",
		DomainNames:    "obj.example.com",
		ForwardHost:    "app",
		ForwardPort:    8080,
		Enabled:        true,
		AdvancedConfig: `{"handler":"headers","response":{"set":{"X-Obj":["1"]}}}`,
	}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	// First handler should be headers
	first := route.Handle[0]
	require.Equal(t, "headers", first["handler"])
}

func TestGenerateConfig_ACLWhitelistIncluded(t *testing.T) {
	// Create a host with a whitelist ACL
	ipRules := `[{"cidr":"192.168.1.0/24"}]`
	acl := models.AccessList{ID: 100, Name: "WL", Enabled: true, Type: "whitelist", IPRules: ipRules}
	host := models.ProxyHost{UUID: "hasacl", DomainNames: "acl.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	// First handler should be an ACL subroute
	first := route.Handle[0]
	require.Equal(t, "subroute", first["handler"])
}

func TestGenerateConfig_SkipsEmptyDomainEntries(t *testing.T) {
	hosts := []models.ProxyHost{{UUID: "u1", DomainNames: ", test.example.com", ForwardHost: "a", ForwardPort: 80, Enabled: true}}
	cfg, err := GenerateConfig(hosts, "/tmp/caddy-data", "", "", "", false)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	require.Equal(t, []string{"test.example.com"}, route.Match[0].Host)
}

func TestGenerateConfig_AdvancedNoHandlerKey(t *testing.T) {
	host := models.ProxyHost{UUID: "adv3", DomainNames: "nohandler.example.com", ForwardHost: "app", ForwardPort: 8080, Enabled: true, AdvancedConfig: `{"foo":"bar"}`}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	// No headers handler appended; last handler is reverse_proxy
	last := route.Handle[len(route.Handle)-1]
	require.Equal(t, "reverse_proxy", last["handler"])
}

func TestGenerateConfig_AdvancedUnexpectedJSONStructure(t *testing.T) {
	host := models.ProxyHost{UUID: "adv4", DomainNames: "struct.example.com", ForwardHost: "app", ForwardPort: 8080, Enabled: true, AdvancedConfig: `42`}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	// Expect main reverse proxy handler exists but no appended advanced handler
	last := route.Handle[len(route.Handle)-1]
	require.Equal(t, "reverse_proxy", last["handler"])
}

// Test buildACLHandler returning nil when an unknown type is supplied but IPRules present
func TestBuildACLHandler_UnknownIPTypeReturnsNil(t *testing.T) {
	acl := &models.AccessList{Type: "custom", IPRules: `[{"cidr":"10.0.0.0/8"}]`}
	h, err := buildACLHandler(acl)
	require.NoError(t, err)
	require.Nil(t, h)
}
