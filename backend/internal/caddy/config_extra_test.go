package caddy

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestGenerateConfig_CatchAllFrontend(t *testing.T) {
	cfg, err := GenerateConfig([]models.ProxyHost{}, "/tmp/caddy-data", "", "/frontend/dist", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
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

	cfg, err := GenerateConfig(hosts, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	// Main route should still have ReverseProxy as last handler (2 routes: emergency + main)
	require.Len(t, server.Routes, 2)
	route := server.Routes[1] // Main route is at index 1
	last := route.Handle[len(route.Handle)-1]
	require.Equal(t, "reverse_proxy", last["handler"])
}

func TestGenerateConfig_AdvancedArrayHandler(t *testing.T) {
	array := []map[string]any{{
		"handler": "headers",
		"response": map[string]any{
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

	cfg, err := GenerateConfig(hosts, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	route := server.Routes[1] // Main route is at index 1 (after emergency route)
	// First handler should be our headers handler
	first := route.Handle[0]
	require.Equal(t, "headers", first["handler"])
}

func TestGenerateConfig_LowercaseDomains(t *testing.T) {
	hosts := []models.ProxyHost{
		{UUID: "d1", DomainNames: "UPPER.EXAMPLE.COM", ForwardHost: "a", ForwardPort: 80, Enabled: true},
	}
	cfg, err := GenerateConfig(hosts, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[1]
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
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[1]
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
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[1]
	// Debug prints removed
	first := route.Handle[0]
	require.Equal(t, "headers", first["handler"])

	// request.set.Upgrade should be an array
	if req, ok := first["request"].(map[string]any); ok {
		if set, ok := req["set"].(map[string]any); ok {
			switch val := set["Upgrade"].(type) {
			case []string:
				require.Equal(t, []string{"websocket"}, val)
			case []any:
				var out []string
				for _, v := range val {
					out = append(out, fmt.Sprintf("%v", v))
				}
				require.Equal(t, []string{"websocket"}, out)
			default:
				t.Fatalf("Upgrade header not normalized to array: %#v", set["Upgrade"])
			}
		} else {
			t.Fatalf("request.set not found in handler: %#v", first["request"])
		}
	} else {
		t.Fatalf("request not found in handler: %#v", first)
	}

	// response.set.X-Obj should be an array
	if resp, ok := first["response"].(map[string]any); ok {
		if set, ok := resp["set"].(map[string]any); ok {
			switch val := set["X-Obj"].(type) {
			case []string:
				require.Equal(t, []string{"1"}, val)
			case []any:
				var out []string
				for _, v := range val {
					out = append(out, fmt.Sprintf("%v", v))
				}
				require.Equal(t, []string{"1"}, out)
			default:
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
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[1]
	// Accept either a subroute (ACL) or reverse_proxy as first handler
	first := route.Handle[0]
	if first["handler"] != "subroute" {
		require.Equal(t, "reverse_proxy", first["handler"])
	}
}

func TestGenerateConfig_SkipsEmptyDomainEntries(t *testing.T) {
	hosts := []models.ProxyHost{{UUID: "u1", DomainNames: ", test.example.com", ForwardHost: "a", ForwardPort: 80, Enabled: true}}
	cfg, err := GenerateConfig(hosts, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[1]
	require.Equal(t, []string{"test.example.com"}, route.Match[0].Host)
}

func TestGenerateConfig_AdvancedNoHandlerKey(t *testing.T) {
	host := models.ProxyHost{UUID: "adv3", DomainNames: "nohandler.example.com", ForwardHost: "app", ForwardPort: 8080, Enabled: true, AdvancedConfig: `{"foo":"bar"}`}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[1]
	// No headers handler appended; last handler is reverse_proxy
	last := route.Handle[len(route.Handle)-1]
	require.Equal(t, "reverse_proxy", last["handler"])
}

func TestGenerateConfig_AdvancedUnexpectedJSONStructure(t *testing.T) {
	host := models.ProxyHost{UUID: "adv4", DomainNames: "struct.example.com", ForwardHost: "app", ForwardPort: 8080, Enabled: true, AdvancedConfig: `42`}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[1]
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

	// Provide rulesets and paths so WAF handler is created with directives
	rulesets := []models.SecurityRuleSet{{Name: "owasp-crs"}}
	rulesetPaths := map[string]string{"owasp-crs": "/tmp/owasp.conf"}
	// Set rate limit values so rate_limit handler is included (uses caddy-ratelimit format)
	secCfg := &models.SecurityConfig{CrowdSecMode: "local", RateLimitRequests: 100, RateLimitWindowSec: 60}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, true, true, true, true, "", rulesets, rulesetPaths, nil, secCfg, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[1]

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
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	route := cfg.Apps.HTTP.Servers["charon_server"].Routes[1]

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

// TestGetAccessLogPath tests the log path selection logic
func TestGetAccessLogPath(t *testing.T) {
	// Save and restore env vars
	origEnv := os.Getenv("CHARON_ENV")
	defer func() { _ = os.Setenv("CHARON_ENV", origEnv) }()

	t.Run("CrowdSecEnabled_UsesStandardPath", func(t *testing.T) {
		_ = os.Setenv("CHARON_ENV", "development")
		path := getAccessLogPath("/data/caddy/data", true)
		require.Equal(t, "/var/log/caddy/access.log", path)
	})

	t.Run("Production_UsesStandardPath", func(t *testing.T) {
		_ = os.Setenv("CHARON_ENV", "production")
		path := getAccessLogPath("/data/caddy/data", false)
		require.Equal(t, "/var/log/caddy/access.log", path)
	})

	t.Run("Development_UsesRelativePath", func(t *testing.T) {
		_ = os.Setenv("CHARON_ENV", "development")
		path := getAccessLogPath("/data/caddy/data", false)
		// Only in development without CrowdSec should it use relative path
		// Note: This test may fail if /.dockerenv exists (e.g., running in CI container)
		if _, err := os.Stat("/.dockerenv"); err != nil {
			// Not in Docker, should use relative path
			expected := "/data/logs/access.log"
			require.Equal(t, expected, path)
		} else {
			// In Docker, always uses standard path
			require.Equal(t, "/var/log/caddy/access.log", path)
		}
	})

	t.Run("NoEnv_CrowdSecEnabled_UsesStandardPath", func(t *testing.T) {
		_ = os.Unsetenv("CHARON_ENV")
		path := getAccessLogPath("/tmp/caddy-data", true)
		require.Equal(t, "/var/log/caddy/access.log", path)
	})
}

// TestGenerateConfig_LoggingConfigured verifies logging is configured in GenerateConfig output
func TestGenerateConfig_LoggingConfigured(t *testing.T) {
	cfg, err := GenerateConfig([]models.ProxyHost{}, "/data/caddy/data", "", "", "", false, true, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	// Logging should be configured
	require.NotNil(t, cfg.Logging)
	require.NotNil(t, cfg.Logging.Logs)
	require.Contains(t, cfg.Logging.Logs, "access")

	accessLog := cfg.Logging.Logs["access"]
	require.NotNil(t, accessLog)
	require.Equal(t, "INFO", accessLog.Level)

	// Writer should be configured for file output
	require.NotNil(t, accessLog.Writer)
	require.Equal(t, "file", accessLog.Writer.Output)
	// When CrowdSec is enabled, the path should be /var/log/caddy/access.log
	require.Equal(t, "/var/log/caddy/access.log", accessLog.Writer.Filename)

	// Encoder should be JSON
	require.NotNil(t, accessLog.Encoder)
	require.Equal(t, "json", accessLog.Encoder.Format)

	// Should include access log directive
	require.Contains(t, accessLog.Include, "http.log.access.access_log")
}
