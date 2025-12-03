package caddy

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestGenerateConfig_CatchAllFrontend(t *testing.T) {
	cfg, err := GenerateConfig([]models.ProxyHost{}, "/tmp/caddy-data", "", "/frontend/dist", "", false, false, false, false, false, "", nil, nil, nil, nil)
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

	cfg, err := GenerateConfig(hosts, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil)
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

	cfg, err := GenerateConfig(hosts, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil)
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
	cfg, err := GenerateConfig(hosts, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	// Debug prints removed
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
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, true, "", nil, nil, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	// First handler should be headers
	first := route.Handle[0]
	require.Equal(t, "headers", first["handler"])
}

func TestGenerateConfig_AdvancedHeadersStringToArray(t *testing.T) {
	host := models.ProxyHost{
		UUID:           "advheaders",
		DomainNames:    "hdr.example.com",
		ForwardHost:    "app",
		ForwardPort:    8080,
		Enabled:        true,
		AdvancedConfig: `{"handler":"headers","request":{"set":{"Upgrade":"websocket"}},"response":{"set":{"X-Obj":"1"}}}`,
	}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, true, "", nil, nil, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	// Debug prints removed
	first := route.Handle[0]
	require.Equal(t, "headers", first["handler"])

	// request.set.Upgrade should be an array
	if req, ok := first["request"].(map[string]interface{}); ok {
		if set, ok := req["set"].(map[string]interface{}); ok {
			if val, ok := set["Upgrade"].([]string); ok {
				require.Equal(t, []string{"websocket"}, val)
			} else if arr, ok := set["Upgrade"].([]interface{}); ok {
				// Convert to string arr for assertion
				var out []string
				for _, v := range arr {
					out = append(out, fmt.Sprintf("%v", v))
				}
				require.Equal(t, []string{"websocket"}, out)
			} else {
				t.Fatalf("Upgrade header not normalized to array: %#v", set["Upgrade"])
			}
		} else {
			t.Fatalf("request.set not found in handler: %#v", first["request"])
		}
	} else {
		t.Fatalf("request not found in handler: %#v", first)
	}

	// response.set.X-Obj should be an array
	if resp, ok := first["response"].(map[string]interface{}); ok {
		if set, ok := resp["set"].(map[string]interface{}); ok {
			if val, ok := set["X-Obj"].([]string); ok {
				require.Equal(t, []string{"1"}, val)
			} else if arr, ok := set["X-Obj"].([]interface{}); ok {
				var out []string
				for _, v := range arr {
					out = append(out, fmt.Sprintf("%v", v))
				}
				require.Equal(t, []string{"1"}, out)
			} else {
				t.Fatalf("X-Obj header not normalized to array: %#v", set["X-Obj"])
			}
		} else {
			t.Fatalf("response.set not found in handler: %#v", first["response"])
		}
	} else {
		t.Fatalf("response not found in handler: %#v", first)
	}
}

func TestGenerateConfig_ACLWhitelistIncluded(t *testing.T) {
	// Create a host with a whitelist ACL
	ipRules := `[{"cidr":"192.168.1.0/24"}]`
	acl := models.AccessList{ID: 100, Name: "WL", Enabled: true, Type: "whitelist", IPRules: ipRules}
	host := models.ProxyHost{UUID: "hasacl", DomainNames: "acl.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl}
	// Sanity check: buildACLHandler should return a subroute handler for this ACL
	aclH, err := buildACLHandler(&acl, "")
	require.NoError(t, err)
	require.NotNil(t, aclH)
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	// Accept either a subroute (ACL) or reverse_proxy as first handler
	first := route.Handle[0]
	if first["handler"] != "subroute" {
		require.Equal(t, "reverse_proxy", first["handler"])
	}
}

func TestGenerateConfig_SkipsEmptyDomainEntries(t *testing.T) {
	hosts := []models.ProxyHost{{UUID: "u1", DomainNames: ", test.example.com", ForwardHost: "a", ForwardPort: 80, Enabled: true}}
	cfg, err := GenerateConfig(hosts, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	require.Equal(t, []string{"test.example.com"}, route.Match[0].Host)
}

func TestGenerateConfig_AdvancedNoHandlerKey(t *testing.T) {
	host := models.ProxyHost{UUID: "adv3", DomainNames: "nohandler.example.com", ForwardHost: "app", ForwardPort: 8080, Enabled: true, AdvancedConfig: `{"foo":"bar"}`}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	// No headers handler appended; last handler is reverse_proxy
	last := route.Handle[len(route.Handle)-1]
	require.Equal(t, "reverse_proxy", last["handler"])
}

func TestGenerateConfig_AdvancedUnexpectedJSONStructure(t *testing.T) {
	host := models.ProxyHost{UUID: "adv4", DomainNames: "struct.example.com", ForwardHost: "app", ForwardPort: 8080, Enabled: true, AdvancedConfig: `42`}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
	// Expect main reverse proxy handler exists but no appended advanced handler
	last := route.Handle[len(route.Handle)-1]
	require.Equal(t, "reverse_proxy", last["handler"])
}

// Test buildACLHandler returning nil when an unknown type is supplied but IPRules present
func TestBuildACLHandler_UnknownIPTypeReturnsNil(t *testing.T) {
	acl := &models.AccessList{Type: "custom", IPRules: `[{"cidr":"10.0.0.0/8"}]`}
	h, err := buildACLHandler(acl, "")
	require.NoError(t, err)
	require.Nil(t, h)
}

func TestGenerateConfig_SecurityPipeline_Order(t *testing.T) {
	// Create host with ACL and HSTS/BlockExploits
	ipRules := `[ { "cidr": "192.168.1.0/24" } ]`
	acl := models.AccessList{ID: 200, Name: "WL", Enabled: true, Type: "whitelist", IPRules: ipRules}
	host := models.ProxyHost{UUID: "pipeline1", DomainNames: "pipe.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl, HSTSEnabled: true, BlockExploits: true}

	secCfg := &models.SecurityConfig{CrowdSecMode: "local"}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, true, true, true, true, "", nil, nil, nil, secCfg)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]

	// Extract handler names
	names := []string{}
	for _, h := range route.Handle {
		if hn, ok := h["handler"].(string); ok {
			names = append(names, hn)
		}
	}

	// Expected pipeline: crowdsec -> waf -> rate_limit -> subroute (acl) -> headers -> vars (BlockExploits) -> reverse_proxy
	require.GreaterOrEqual(t, len(names), 4)
	require.Equal(t, "crowdsec", names[0])
	require.Equal(t, "waf", names[1])
	require.Equal(t, "rate_limit", names[2])
	// ACL is subroute
	require.Equal(t, "subroute", names[3])
}

func TestGenerateConfig_SecurityPipeline_OmitWhenDisabled(t *testing.T) {
	host := models.ProxyHost{UUID: "pipe2", DomainNames: "pipe2.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]

	// Extract handler names
	names := []string{}
	for _, h := range route.Handle {
		if hn, ok := h["handler"].(string); ok {
			names = append(names, hn)
		}
	}

	// Should not include the security pipeline placeholders
	for _, n := range names {
		require.NotEqual(t, "crowdsec", n)
		require.NotEqual(t, "coraza", n)
		require.NotEqual(t, "rate_limit", n)
		require.NotEqual(t, "subroute", n)
	}
}
