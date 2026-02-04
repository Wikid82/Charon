package caddy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestGenerateConfig_Empty(t *testing.T) {
	config, err := GenerateConfig([]models.ProxyHost{}, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, config.Apps.HTTP)
	require.Empty(t, config.Apps.HTTP.Servers)
	require.NotNil(t, config)
	require.NotNil(t, config.Apps.HTTP)
	require.Empty(t, config.Apps.HTTP.Servers)
}

func TestGenerateConfig_SingleHost(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:             "test-uuid",
			Name:             "Media",
			DomainNames:      "media.example.com",
			ForwardScheme:    "http",
			ForwardHost:      "media",
			ForwardPort:      32400,
			SSLForced:        true,
			WebsocketSupport: false,
			Enabled:          true,
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, config.Apps.HTTP)
	require.Len(t, config.Apps.HTTP.Servers, 1)
	require.NotNil(t, config)
	require.NotNil(t, config.Apps.HTTP)
	require.Len(t, config.Apps.HTTP.Servers, 1)

	server := config.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	require.Contains(t, server.Listen, ":80")
	require.Contains(t, server.Listen, ":443")
	require.Len(t, server.Routes, 2) // Emergency + main route

	route := server.Routes[1] // Main route is at index 1
	require.Len(t, route.Match, 1)
	require.Equal(t, []string{"media.example.com"}, route.Match[0].Host)
	require.Len(t, route.Handle, 1)
	require.True(t, route.Terminal)

	handler := route.Handle[0]
	require.Equal(t, "reverse_proxy", handler["handler"])
}

func TestGenerateConfig_MultipleHosts(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "uuid-1",
			DomainNames: "site1.example.com",
			ForwardHost: "app1",
			ForwardPort: 8080,
			Enabled:     true,
		},
		{
			UUID:        "uuid-2",
			DomainNames: "site2.example.com",
			ForwardHost: "app2",
			ForwardPort: 8081,
			Enabled:     true,
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, config.Apps.HTTP.Servers["charon_server"].Routes, 4) // 2 hosts × 2 routes each (emergency + main)
	require.Len(t, config.Apps.HTTP.Servers["charon_server"].Routes, 4) // 2 hosts × 2 routes each
}

func TestGenerateConfig_WebSocketEnabled(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:             "uuid-ws",
			DomainNames:      "ws.example.com",
			ForwardHost:      "wsapp",
			ForwardPort:      3000,
			WebsocketSupport: true,
			Enabled:          true,
		},
	}
	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	route := config.Apps.HTTP.Servers["charon_server"].Routes[1] // Main route is at index 1
	handler := route.Handle[0]

	// Check WebSocket headers are present
	require.NotNil(t, handler["headers"])
}

func TestGenerateConfig_EmptyDomain(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "bad-uuid",
			DomainNames: "",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Empty(t, config.Apps.HTTP.Servers["charon_server"].Routes)
	// Should produce empty routes (or just catch-all if frontendDir was set, but it's empty here)
	require.Empty(t, config.Apps.HTTP.Servers["charon_server"].Routes)
}

func TestGenerateConfig_Logging(t *testing.T) {
	hosts := []models.ProxyHost{}
	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, config.Logging)

	// Verify logging configuration
	require.NotNil(t, config.Logging)
	require.NotNil(t, config.Logging.Logs)
	require.NotNil(t, config.Logging.Logs["access"])
	require.Equal(t, "INFO", config.Logging.Logs["access"].Level)
	require.Contains(t, config.Logging.Logs["access"].Writer.Filename, "access.log")
	require.Equal(t, 10, config.Logging.Logs["access"].Writer.RollSize)
	require.Equal(t, 5, config.Logging.Logs["access"].Writer.RollKeep)
	require.Equal(t, 7, config.Logging.Logs["access"].Writer.RollKeepDays)
}

func TestGenerateConfig_IPHostsSkipAutoHTTPS(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "uuid-ip",
			DomainNames: "192.0.2.10",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	server := config.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	require.Contains(t, server.AutoHTTPS.Skip, "192.0.2.10")

	// Ensure TLS automation adds internal issuer for IP literals
	require.NotNil(t, config.Apps.TLS)
	require.NotNil(t, config.Apps.TLS.Automation)
	require.GreaterOrEqual(t, len(config.Apps.TLS.Automation.Policies), 1)
	foundIPPolicy := false
	for _, p := range config.Apps.TLS.Automation.Policies {
		if len(p.Subjects) == 0 {
			continue
		}
		if p.Subjects[0] == "192.0.2.10" {
			foundIPPolicy = true
			require.Len(t, p.IssuersRaw, 1)
			issuer := p.IssuersRaw[0].(map[string]any)
			require.Equal(t, "internal", issuer["module"])
		}
	}
	require.True(t, foundIPPolicy, "expected internal issuer policy for IP host")
}

func TestGenerateConfig_Advanced(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:           "advanced-uuid",
			Name:           "Advanced",
			DomainNames:    "advanced.example.com",
			ForwardScheme:  "http",
			ForwardHost:    "advanced",
			ForwardPort:    8080,
			SSLForced:      true,
			HSTSEnabled:    true,
			HSTSSubdomains: true,
			BlockExploits:  true,
			Enabled:        true,
			Locations: []models.Location{
				{
					Path:        "/api",
					ForwardHost: "api-service",
					ForwardPort: 9000,
				},
			},
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, config)
	require.NotNil(t, config)

	server := config.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	// Should have 3 routes: location /api, emergency, main
	require.Len(t, server.Routes, 3)

	// Check Location Route (first as it's most specific)
	locRoute := server.Routes[0]
	require.Equal(t, []string{"/api", "/api/*"}, locRoute.Match[0].Path)
	require.Equal(t, []string{"advanced.example.com"}, locRoute.Match[0].Host)

	// Check Main Route (after emergency route)
	mainRoute := server.Routes[2]
	require.Nil(t, mainRoute.Match[0].Path) // No path means all paths
	require.Equal(t, []string{"advanced.example.com"}, mainRoute.Match[0].Host)

	// Check HSTS and BlockExploits handlers in main route
	// Handlers are: [HSTS, BlockExploits, ReverseProxy]
	// But wait, BlockExploitsHandler implementation details?
	// Let's just check count for now or inspect types if possible.
	// Based on code:
	// handlers = append(handlers, HeaderHandler(...)) // HSTS
	// handlers = append(handlers, BlockExploitsHandler()) // BlockExploits
	// mainHandlers = append(handlers, ReverseProxyHandler(...))

	require.Len(t, mainRoute.Handle, 3)

	// Check HSTS
	hstsHandler := mainRoute.Handle[0]
	require.Equal(t, "headers", hstsHandler["handler"])
}

func TestGenerateConfig_ACMEStaging(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "test-uuid",
			DomainNames: "test.example.com",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}

	// Test with staging enabled
	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "letsencrypt", true, false, false, false, true, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, config.Apps.TLS)
	require.NotNil(t, config.Apps.TLS)
	require.NotNil(t, config.Apps.TLS.Automation)
	require.Len(t, config.Apps.TLS.Automation.Policies, 1)

	issuers := config.Apps.TLS.Automation.Policies[0].IssuersRaw
	require.Len(t, issuers, 1)

	acmeIssuer := issuers[0].(map[string]any)
	require.Equal(t, "acme", acmeIssuer["module"])
	require.Equal(t, "admin@example.com", acmeIssuer["email"])
	require.Equal(t, "https://acme-staging-v02.api.letsencrypt.org/directory", acmeIssuer["ca"])

	// Test with staging disabled (production)
	config, err = GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "letsencrypt", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, config.Apps.TLS)
	require.NotNil(t, config.Apps.TLS.Automation)
	require.Len(t, config.Apps.TLS.Automation.Policies, 1)

	issuers = config.Apps.TLS.Automation.Policies[0].IssuersRaw
	require.Len(t, issuers, 1)

	acmeIssuer = issuers[0].(map[string]any)
	require.Equal(t, "acme", acmeIssuer["module"])
	require.Equal(t, "admin@example.com", acmeIssuer["email"])
	_, hasCA := acmeIssuer["ca"]
	require.False(t, hasCA, "Production mode should not set ca field (uses default)")
	// We can't easily check the map content without casting, but we know it's there.
}

func TestBuildACLHandler_WhitelistAndBlacklistAdminMerge(t *testing.T) {
	// Whitelist case: ensure adminWhitelist gets merged into allowed ranges
	acl := &models.AccessList{Type: "whitelist", IPRules: `[{"cidr":"127.0.0.1/32"}]`}
	handler, err := buildACLHandler(acl, "10.0.0.1/32")
	require.NoError(t, err)
	// handler should include both ranges in the remote_ip ranges
	b, _ := json.Marshal(handler)
	s := string(b)
	require.Contains(t, s, "127.0.0.1/32")
	require.Contains(t, s, "10.0.0.1/32")

	// Blacklist case: ensure adminWhitelist excluded from match
	acl2 := &models.AccessList{Type: "blacklist", IPRules: `[{"cidr":"1.2.3.0/24"}]`}
	handler2, err := buildACLHandler(acl2, "192.168.0.1/32")
	require.NoError(t, err)
	b2, _ := json.Marshal(handler2)
	s2 := string(b2)
	require.Contains(t, s2, "1.2.3.0/24")
	require.Contains(t, s2, "192.168.0.1/32")
}

func TestBuildACLHandler_GeoAndLocalNetwork(t *testing.T) {
	// Geo whitelist
	acl := &models.AccessList{Type: "geo_whitelist", CountryCodes: "US,CA"}
	h, err := buildACLHandler(acl, "")
	require.NoError(t, err)
	b, _ := json.Marshal(h)
	s := string(b)
	require.Contains(t, s, "geoip2.country_code")

	// Geo blacklist
	acl2 := &models.AccessList{Type: "geo_blacklist", CountryCodes: "RU"}
	h2, err := buildACLHandler(acl2, "")
	require.NoError(t, err)
	b2, _ := json.Marshal(h2)
	s2 := string(b2)
	require.Contains(t, s2, "geoip2.country_code")

	// Local network only
	acl3 := &models.AccessList{Type: "whitelist", LocalNetworkOnly: true}
	h3, err := buildACLHandler(acl3, "")
	require.NoError(t, err)
	b3, _ := json.Marshal(h3)
	s3 := string(b3)
	require.Contains(t, s3, "10.0.0.0/8")
}

func TestBuildACLHandler_AdminWhitelistParsing(t *testing.T) {
	// Whitelist should trim and include multiple values, skip empties
	acl := &models.AccessList{Type: "whitelist", IPRules: `[{"cidr":"127.0.0.1/32"}]`}
	handler, err := buildACLHandler(acl, " , 10.0.0.1/32, , 192.168.1.5/32 ")
	require.NoError(t, err)
	b, _ := json.Marshal(handler)
	s := string(b)
	require.Contains(t, s, "127.0.0.1/32")
	require.Contains(t, s, "10.0.0.1/32")
	require.Contains(t, s, "192.168.1.5/32")

	// Blacklist parsing too
	acl2 := &models.AccessList{Type: "blacklist", IPRules: `[{"cidr":"1.2.3.0/24"}]`}
	handler2, err := buildACLHandler(acl2, " , 192.168.0.1/32, ")
	require.NoError(t, err)
	b2, _ := json.Marshal(handler2)
	s2 := string(b2)
	require.Contains(t, s2, "1.2.3.0/24")
	require.Contains(t, s2, "192.168.0.1/32")
}

func TestBuildRateLimitHandler_Disabled(t *testing.T) {
	// Test nil secCfg returns nil handler
	h, err := buildRateLimitHandler(nil, nil)
	require.NoError(t, err)
	require.Nil(t, h)
}

func TestBuildRateLimitHandler_InvalidValues(t *testing.T) {
	// Test zero requests returns nil handler
	secCfg := &models.SecurityConfig{
		RateLimitRequests:  0,
		RateLimitWindowSec: 60,
	}
	h, err := buildRateLimitHandler(nil, secCfg)
	require.NoError(t, err)
	require.Nil(t, h)

	// Test zero window returns nil handler
	secCfg2 := &models.SecurityConfig{
		RateLimitRequests:  100,
		RateLimitWindowSec: 0,
	}
	h, err = buildRateLimitHandler(nil, secCfg2)
	require.NoError(t, err)
	require.Nil(t, h)

	// Test negative values returns nil handler
	secCfg3 := &models.SecurityConfig{
		RateLimitRequests:  -1,
		RateLimitWindowSec: 60,
	}
	h, err = buildRateLimitHandler(nil, secCfg3)
	require.NoError(t, err)
	require.Nil(t, h)
}

func TestBuildRateLimitHandler_ValidConfig(t *testing.T) {
	// Test valid configuration produces correct caddy-ratelimit format
	secCfg := &models.SecurityConfig{
		RateLimitRequests:  100,
		RateLimitWindowSec: 60,
		RateLimitBurst:     25,
	}
	h, err := buildRateLimitHandler(nil, secCfg)
	require.NoError(t, err)
	require.NotNil(t, h)

	// Verify handler type
	require.Equal(t, "rate_limit", h["handler"])

	// Verify rate_limits structure
	rateLimits, ok := h["rate_limits"].(map[string]any)
	require.True(t, ok, "rate_limits should be a map")

	staticZone, ok := rateLimits["static"].(map[string]any)
	require.True(t, ok, "static zone should be a map")

	// Verify caddy-ratelimit specific fields
	require.Equal(t, "{http.request.remote.host}", staticZone["key"])
	require.Equal(t, "60s", staticZone["window"])
	require.Equal(t, 100, staticZone["max_events"])
	// Note: caddy-ratelimit doesn't support burst parameter (uses sliding window)
}

func TestBuildRateLimitHandler_JSONFormat(t *testing.T) {
	// Test that the handler produces valid JSON matching caddy-ratelimit schema
	secCfg := &models.SecurityConfig{
		RateLimitRequests:  30,
		RateLimitWindowSec: 10,
		RateLimitBurst:     5,
	}
	h, err := buildRateLimitHandler(nil, secCfg)
	require.NoError(t, err)
	require.NotNil(t, h)

	// Marshal to JSON and verify structure
	b, err := json.Marshal(h)
	require.NoError(t, err)
	s := string(b)

	// Verify expected JSON content
	require.Contains(t, s, `"handler":"rate_limit"`)
	require.Contains(t, s, `"rate_limits"`)
	require.Contains(t, s, `"static"`)
	require.Contains(t, s, `"key":"{http.request.remote.host}"`)
	require.Contains(t, s, `"window":"10s"`)
	require.Contains(t, s, `"max_events":30`)
	// Note: burst field not included (not supported by caddy-ratelimit)
}

func TestGenerateConfig_WithRateLimiting(t *testing.T) {
	// Test that rate limiting is included in generated config when enabled
	hosts := []models.ProxyHost{
		{
			UUID:        "test-uuid",
			DomainNames: "example.com",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}

	secCfg := &models.SecurityConfig{
		RateLimitEnable:    true,
		RateLimitRequests:  60,
		RateLimitWindowSec: 60,
	}

	// rateLimitEnabled=true should include the handler
	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, true, false, "", nil, nil, nil, secCfg, nil)
	require.NoError(t, err)
	require.NotNil(t, config.Apps.HTTP)

	server := config.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	require.Len(t, server.Routes, 2) // Emergency + main route

	route := server.Routes[1] // Main route is at index 1
	// Handlers should include rate_limit + reverse_proxy
	require.GreaterOrEqual(t, len(route.Handle), 2)

	// Find the rate_limit handler
	var foundRateLimit bool
	for _, h := range route.Handle {
		if h["handler"] == "rate_limit" {
			foundRateLimit = true
			// Verify it has the correct structure
			require.NotNil(t, h["rate_limits"])
			break
		}
	}
	require.True(t, foundRateLimit, "rate_limit handler should be present")
}

func TestBuildRateLimitHandler_UsesBurst(t *testing.T) {
	// Verify that burst config value is ignored (caddy-ratelimit doesn't support it)
	secCfg := &models.SecurityConfig{
		RateLimitRequests:  100,
		RateLimitWindowSec: 60,
		RateLimitBurst:     50,
	}
	h, err := buildRateLimitHandler(nil, secCfg)
	require.NoError(t, err)
	require.NotNil(t, h)

	// Handler should be a plain rate_limit (no bypass list)
	require.Equal(t, "rate_limit", h["handler"])

	rateLimits, ok := h["rate_limits"].(map[string]any)
	require.True(t, ok)
	staticZone, ok := rateLimits["static"].(map[string]any)
	require.True(t, ok)

	// Verify burst field is NOT present (not supported by caddy-ratelimit)
	_, hasBurst := staticZone["burst"]
	require.False(t, hasBurst, "burst field should not be included")
}

func TestBuildRateLimitHandler_DefaultBurst(t *testing.T) {
	// Verify that burst field is not included (caddy-ratelimit uses sliding window, no burst)
	secCfg := &models.SecurityConfig{
		RateLimitRequests:  100,
		RateLimitWindowSec: 60,
		RateLimitBurst:     0, // Not set
	}
	h, err := buildRateLimitHandler(nil, secCfg)
	require.NoError(t, err)
	require.NotNil(t, h)

	rateLimits, ok := h["rate_limits"].(map[string]any)
	require.True(t, ok)
	staticZone, ok := rateLimits["static"].(map[string]any)
	require.True(t, ok)

	// Verify burst field is NOT present
	_, hasBurst := staticZone["burst"]
	require.False(t, hasBurst, "burst field should not be included")

	// Test with small requests value - should also not have burst
	secCfg2 := &models.SecurityConfig{
		RateLimitRequests:  3,
		RateLimitWindowSec: 60,
		RateLimitBurst:     0,
	}
	h2, err := buildRateLimitHandler(nil, secCfg2)
	require.NoError(t, err)
	require.NotNil(t, h2)

	rateLimits2, ok := h2["rate_limits"].(map[string]any)
	require.True(t, ok)
	staticZone2, ok := rateLimits2["static"].(map[string]any)
	require.True(t, ok)

	// Verify no burst field here either
	_, hasBurst2 := staticZone2["burst"]
	require.False(t, hasBurst2, "burst field should not be included")
}

// TestGetAccessLogPath_CrowdSecEnabled verifies log path when CrowdSec is explicitly enabled
func TestGetAccessLogPath_CrowdSecEnabled(t *testing.T) {
	// When CrowdSec is enabled, always use standard path
	path := getAccessLogPath("/tmp/caddy-data", true)
	require.Equal(t, "/var/log/caddy/access.log", path)
}

// TestGetAccessLogPath_DockerEnv verifies log path detection via /.dockerenv
func TestGetAccessLogPath_DockerEnv(t *testing.T) {
	// This test can't reliably test /.dockerenv detection without mocking os.Stat
	// But we can test the CHARON_ENV fallback

	// Save original env
	originalEnv := os.Getenv("CHARON_ENV")
	defer func() { _ = os.Setenv("CHARON_ENV", originalEnv) }()

	// Set CHARON_ENV=production
	_ = os.Setenv("CHARON_ENV", "production")
	path := getAccessLogPath("/tmp/caddy-data", false)
	require.Equal(t, "/var/log/caddy/access.log", path)

	// Unset CHARON_ENV - should use development path
	_ = os.Unsetenv("CHARON_ENV")
	path = getAccessLogPath("/tmp/storage/caddy/data", false)
	require.Contains(t, path, "logs/access.log")
	require.Contains(t, path, "/tmp/storage/logs/access.log")
}

// TestGetAccessLogPath_Development verifies development fallback path
func TestGetAccessLogPath_Development(t *testing.T) {
	// Save original env
	originalEnv := os.Getenv("CHARON_ENV")
	defer func() {
		if originalEnv != "" {
			_ = os.Setenv("CHARON_ENV", originalEnv)
		} else {
			_ = os.Unsetenv("CHARON_ENV")
		}
	}()

	// Clear CHARON_ENV to simulate dev environment
	_ = os.Unsetenv("CHARON_ENV")

	// Test with typical dev path
	storageDir := "/home/user/charon/data/caddy/data"
	path := getAccessLogPath(storageDir, false)

	// Should construct path: /home/user/charon/data/logs/access.log
	expectedPath := filepath.Join("/home/user/charon/data/logs", "access.log")
	require.Equal(t, expectedPath, path)
}

// TestBuildPermissionsPolicyString_EmptyAllowlist verifies empty allowlist creates "()"
func TestBuildPermissionsPolicyString_EmptyAllowlist(t *testing.T) {
	permissionsJSON := `[{"feature":"geolocation","allowlist":[]}]`
	result, err := buildPermissionsPolicyString(permissionsJSON)
	require.NoError(t, err)
	require.Equal(t, "geolocation=()", result)
}

// TestBuildPermissionsPolicyString_SelfAndStar verifies self and * handling
func TestBuildPermissionsPolicyString_SelfAndStar(t *testing.T) {
	permissionsJSON := `[{"feature":"camera","allowlist":["self"]},{"feature":"microphone","allowlist":["*"]}]`
	result, err := buildPermissionsPolicyString(permissionsJSON)
	require.NoError(t, err)
	require.Equal(t, "camera=(self), microphone=(*)", result)
}

// TestBuildPermissionsPolicyString_DomainValues verifies domain values are quoted
func TestBuildPermissionsPolicyString_DomainValues(t *testing.T) {
	permissionsJSON := `[{"feature":"payment","allowlist":["https://example.com","https://payment.example.com"]}]`
	result, err := buildPermissionsPolicyString(permissionsJSON)
	require.NoError(t, err)
	require.Equal(t, `payment=("https://example.com" "https://payment.example.com")`, result)
}

// TestBuildPermissionsPolicyString_Mixed verifies mixed allowlist (self + domains)
func TestBuildPermissionsPolicyString_Mixed(t *testing.T) {
	permissionsJSON := `[{"feature":"fullscreen","allowlist":["self","https://cdn.example.com"]}]`
	result, err := buildPermissionsPolicyString(permissionsJSON)
	require.NoError(t, err)
	require.Equal(t, `fullscreen=(self "https://cdn.example.com")`, result)
}

// TestBuildPermissionsPolicyString_InvalidJSON verifies error handling
func TestBuildPermissionsPolicyString_InvalidJSON(t *testing.T) {
	permissionsJSON := `invalid json`
	result, err := buildPermissionsPolicyString(permissionsJSON)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid permissions JSON")
	require.Equal(t, "", result)
}

// TestBuildCSPString_EmptyDirective verifies empty directives return empty string
func TestBuildCSPString_EmptyDirective(t *testing.T) {
	directivesJSON := ``
	result, err := buildCSPString(directivesJSON)
	require.NoError(t, err)
	require.Equal(t, "", result)
}

// TestBuildCSPString_InvalidJSON verifies error handling
func TestBuildCSPString_InvalidJSON(t *testing.T) {
	directivesJSON := `not valid json`
	result, err := buildCSPString(directivesJSON)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid CSP JSON")
	require.Equal(t, "", result)
}

// TestBuildSecurityHeadersHandler_CompleteProfile verifies all headers are set
func TestBuildSecurityHeadersHandler_CompleteProfile(t *testing.T) {
	profile := &models.SecurityHeaderProfile{
		HSTSEnabled:               true,
		HSTSMaxAge:                63072000,
		HSTSIncludeSubdomains:     true,
		HSTSPreload:               true,
		CSPEnabled:                true,
		CSPDirectives:             `{"default-src":["'self'"],"script-src":["'self'","'unsafe-inline'"]}`,
		CSPReportOnly:             false,
		XFrameOptions:             "DENY",
		XContentTypeOptions:       true,
		ReferrerPolicy:            "no-referrer",
		PermissionsPolicy:         `[{"feature":"geolocation","allowlist":[]},{"feature":"camera","allowlist":["self"]}]`,
		CrossOriginOpenerPolicy:   "same-origin-allow-popups",
		CrossOriginResourcePolicy: "cross-origin",
		CrossOriginEmbedderPolicy: "require-corp",
		XSSProtection:             true,
		CacheControlNoStore:       true,
	}

	host := &models.ProxyHost{
		SecurityHeaderProfile: profile,
	}

	h, err := buildSecurityHeadersHandler(host)
	require.NoError(t, err)
	require.NotNil(t, h)
	require.Equal(t, "headers", h["handler"])

	// Check response headers
	response := h["response"].(map[string]any)
	headers := response["set"].(map[string][]string)

	// Verify HSTS
	require.Equal(t, []string{"max-age=63072000; includeSubDomains; preload"}, headers["Strict-Transport-Security"])

	// Verify CSP
	require.Contains(t, headers, "Content-Security-Policy")
	require.Contains(t, headers["Content-Security-Policy"][0], "default-src 'self'")
	require.Contains(t, headers["Content-Security-Policy"][0], "script-src 'self' 'unsafe-inline'")

	// Verify all security headers
	require.Equal(t, []string{"DENY"}, headers["X-Frame-Options"])
	require.Equal(t, []string{"nosniff"}, headers["X-Content-Type-Options"])
	require.Equal(t, []string{"no-referrer"}, headers["Referrer-Policy"])
	require.Equal(t, []string{"same-origin-allow-popups"}, headers["Cross-Origin-Opener-Policy"])
	require.Equal(t, []string{"cross-origin"}, headers["Cross-Origin-Resource-Policy"])
	require.Equal(t, []string{"require-corp"}, headers["Cross-Origin-Embedder-Policy"])
	require.Equal(t, []string{"1; mode=block"}, headers["X-XSS-Protection"])
	require.Equal(t, []string{"no-store"}, headers["Cache-Control"])

	// Verify Permissions-Policy
	require.Contains(t, headers, "Permissions-Policy")
	require.Contains(t, headers["Permissions-Policy"][0], "geolocation=()")
	require.Contains(t, headers["Permissions-Policy"][0], "camera=(self)")
}

// TestGenerateConfig_SSLProviderZeroSSL verifies ZeroSSL issuer configuration
func TestGenerateConfig_SSLProviderZeroSSL(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "test-uuid",
			DomainNames: "test.example.com",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "zerossl", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, config.Apps.TLS)
	require.NotNil(t, config.Apps.TLS.Automation)
	require.Len(t, config.Apps.TLS.Automation.Policies, 1)

	issuers := config.Apps.TLS.Automation.Policies[0].IssuersRaw
	require.Len(t, issuers, 1)

	issuer := issuers[0].(map[string]any)
	require.Equal(t, "zerossl", issuer["module"])
}

// TestGenerateConfig_SSLProviderBoth verifies both Let's Encrypt and ZeroSSL
func TestGenerateConfig_SSLProviderBoth(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "test-uuid",
			DomainNames: "test.example.com",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}

	// Test with "both" provider
	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "both", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, config.Apps.TLS)
	require.NotNil(t, config.Apps.TLS.Automation)
	require.Len(t, config.Apps.TLS.Automation.Policies, 1)

	issuers := config.Apps.TLS.Automation.Policies[0].IssuersRaw
	require.Len(t, issuers, 2)

	// First should be ACME (Let's Encrypt)
	issuer1 := issuers[0].(map[string]any)
	require.Equal(t, "acme", issuer1["module"])

	// Second should be ZeroSSL
	issuer2 := issuers[1].(map[string]any)
	require.Equal(t, "zerossl", issuer2["module"])
}

// TestGenerateConfig_DuplicateDomains verifies Ghost Host duplicate detection
func TestGenerateConfig_DuplicateDomains(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "uuid-1",
			DomainNames: "duplicate.example.com",
			ForwardHost: "app1",
			ForwardPort: 8080,
			Enabled:     true,
		},
		{
			UUID:        "uuid-2",
			DomainNames: "duplicate.example.com", // Same domain
			ForwardHost: "app2",
			ForwardPort: 8081,
			Enabled:     true,
		},
		{
			UUID:        "uuid-3",
			DomainNames: "unique.example.com",
			ForwardHost: "app3",
			ForwardPort: 8082,
			Enabled:     true,
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	server := config.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)

	// Should only have 4 routes (2 hosts × emergency + main, one duplicate filtered out)
	require.Len(t, server.Routes, 4)

	// Verify unique.example.com is present
	var foundUnique bool
	for _, route := range server.Routes {
		if len(route.Match) > 0 && len(route.Match[0].Host) > 0 {
			if route.Match[0].Host[0] == "unique.example.com" {
				foundUnique = true
			}
		}
	}
	require.True(t, foundUnique, "unique.example.com should be present")
}

// TestGenerateConfig_WithCrowdSecApp verifies CrowdSec app configuration
func TestGenerateConfig_WithCrowdSecApp(t *testing.T) {
	const bouncerKeyFile = "/app/data/crowdsec/bouncer_key"

	// Skip if bouncer_key file exists (file takes priority over env vars per Phase 1 of LAPI auth fix)
	if _, err := os.Stat(bouncerKeyFile); err == nil {
		t.Skip("Skipping env var test - bouncer_key file exists (file takes priority over env vars)")
	}

	hosts := []models.ProxyHost{
		{
			UUID:        "test-uuid",
			DomainNames: "test.example.com",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}

	secCfg := &models.SecurityConfig{
		CrowdSecAPIURL: "http://crowdsec:8080",
	}

	// Save original env
	originalAPIKey := os.Getenv("CROWDSEC_API_KEY")
	defer func() {
		if originalAPIKey != "" {
			_ = os.Setenv("CROWDSEC_API_KEY", originalAPIKey)
		} else {
			_ = os.Unsetenv("CROWDSEC_API_KEY")
		}
	}()

	// Set test API key
	_ = os.Setenv("CROWDSEC_API_KEY", "test-api-key-12345")

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, true, false, false, false, "", nil, nil, nil, secCfg, nil)
	require.NoError(t, err)

	// Verify CrowdSec app is configured
	require.NotNil(t, config.Apps.CrowdSec)
	require.Equal(t, "http://crowdsec:8080", config.Apps.CrowdSec.APIUrl)
	require.Equal(t, "test-api-key-12345", config.Apps.CrowdSec.APIKey)
	require.Equal(t, "60s", config.Apps.CrowdSec.TickerInterval)
	require.NotNil(t, config.Apps.CrowdSec.EnableStreaming)
	require.True(t, *config.Apps.CrowdSec.EnableStreaming)
}

// TestGenerateConfig_CrowdSecHandlerAdded verifies CrowdSec handler is added to routes
func TestGenerateConfig_CrowdSecHandlerAdded(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "test-uuid",
			DomainNames: "test.example.com",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, true, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	server := config.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	require.Len(t, server.Routes, 2) // Emergency + main route

	route := server.Routes[1] // Main route is at index 1
	// Should have CrowdSec handler + reverse_proxy handler
	require.GreaterOrEqual(t, len(route.Handle), 2)

	// Find CrowdSec handler
	var foundCrowdSec bool
	for _, h := range route.Handle {
		if h["handler"] == "crowdsec" {
			foundCrowdSec = true
			break
		}
	}
	require.True(t, foundCrowdSec, "CrowdSec handler should be present")
}

// TestGenerateConfig_WithSecurityDecisions verifies manual IP blocks
func TestGenerateConfig_WithSecurityDecisions(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "test-uuid",
			DomainNames: "test.example.com",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}

	decisions := []models.SecurityDecision{
		{IP: "1.2.3.4", Action: "block"},
		{IP: "5.6.7.0/24", Action: "block"},
		{IP: "10.0.0.1", Action: "allow"}, // Should be ignored (not block action)
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, decisions, nil, nil)
	require.NoError(t, err)

	server := config.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	require.Len(t, server.Routes, 2) // Emergency + main route

	route := server.Routes[1] // Main route is at index 1

	// Marshal to JSON for inspection
	b, err := json.Marshal(route.Handle)
	require.NoError(t, err)
	s := string(b)

	// Should contain blocked IPs
	require.Contains(t, s, "1.2.3.4")
	require.Contains(t, s, "5.6.7.0/24")

	// Should NOT contain allowed IP (not a block action)
	require.NotContains(t, s, "10.0.0.1")
}

func TestBuildRateLimitHandler_BypassList(t *testing.T) {
	// Verify bypass list creates subroute structure
	secCfg := &models.SecurityConfig{
		RateLimitRequests:   100,
		RateLimitWindowSec:  60,
		RateLimitBurst:      20,
		RateLimitBypassList: "10.0.0.0/8, 192.168.1.0/24",
	}
	h, err := buildRateLimitHandler(nil, secCfg)
	require.NoError(t, err)
	require.NotNil(t, h)

	// Handler should be a subroute when bypass list is configured
	require.Equal(t, "subroute", h["handler"])

	// Marshal to JSON for easy inspection
	b, err := json.Marshal(h)
	require.NoError(t, err)
	s := string(b)

	// Verify subroute contains bypass IPs
	require.Contains(t, s, "10.0.0.0/8")
	require.Contains(t, s, "192.168.1.0/24")
	require.Contains(t, s, "remote_ip")
	require.Contains(t, s, "rate_limit")
}

func TestBuildRateLimitHandler_BypassList_PlainIPs(t *testing.T) {
	// Verify plain IPs are converted to CIDRs
	secCfg := &models.SecurityConfig{
		RateLimitRequests:   100,
		RateLimitWindowSec:  60,
		RateLimitBurst:      20,
		RateLimitBypassList: "10.0.0.1, 192.168.1.1",
	}
	h, err := buildRateLimitHandler(nil, secCfg)
	require.NoError(t, err)
	require.NotNil(t, h)

	require.Equal(t, "subroute", h["handler"])

	b, err := json.Marshal(h)
	require.NoError(t, err)
	s := string(b)

	// Plain IPs should be converted to /32 CIDRs
	require.Contains(t, s, "10.0.0.1/32")
	require.Contains(t, s, "192.168.1.1/32")
}

func TestBuildRateLimitHandler_BypassList_InvalidEntries(t *testing.T) {
	// Verify invalid entries are ignored
	secCfg := &models.SecurityConfig{
		RateLimitRequests:   100,
		RateLimitWindowSec:  60,
		RateLimitBurst:      20,
		RateLimitBypassList: "invalid, 10.0.0.0/8, also-invalid",
	}
	h, err := buildRateLimitHandler(nil, secCfg)
	require.NoError(t, err)
	require.NotNil(t, h)

	require.Equal(t, "subroute", h["handler"])

	b, err := json.Marshal(h)
	require.NoError(t, err)
	s := string(b)

	// Only valid CIDR should be present
	require.Contains(t, s, "10.0.0.0/8")
	require.NotContains(t, s, "invalid")
	require.NotContains(t, s, "also-invalid")
}

func TestBuildRateLimitHandler_BypassList_Empty(t *testing.T) {
	// Verify empty bypass list returns plain rate_limit handler
	secCfg := &models.SecurityConfig{
		RateLimitRequests:   100,
		RateLimitWindowSec:  60,
		RateLimitBurst:      20,
		RateLimitBypassList: "",
	}
	h, err := buildRateLimitHandler(nil, secCfg)
	require.NoError(t, err)
	require.NotNil(t, h)

	// Should be plain rate_limit, not subroute
	require.Equal(t, "rate_limit", h["handler"])
}

func TestBuildRateLimitHandler_BypassList_AllInvalid(t *testing.T) {
	// Verify all-invalid bypass list returns plain rate_limit handler
	secCfg := &models.SecurityConfig{
		RateLimitRequests:   100,
		RateLimitWindowSec:  60,
		RateLimitBurst:      20,
		RateLimitBypassList: "invalid, also-invalid, not-an-ip",
	}
	h, err := buildRateLimitHandler(nil, secCfg)
	require.NoError(t, err)
	require.NotNil(t, h)

	// Should be plain rate_limit since no valid CIDRs
	require.Equal(t, "rate_limit", h["handler"])
}

func TestParseBypassCIDRs(t *testing.T) {
	// Test various inputs
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty", "", nil},
		{"single_cidr", "10.0.0.0/8", []string{"10.0.0.0/8"}},
		{"multiple_cidrs", "10.0.0.0/8, 192.168.0.0/16", []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"plain_ipv4", "10.0.0.1", []string{"10.0.0.1/32"}},
		{"plain_ipv6", "::1", []string{"::1/128"}},
		{"mixed", "10.0.0.0/8, 192.168.1.1, invalid", []string{"10.0.0.0/8", "192.168.1.1/32"}},
		{"with_spaces", " 10.0.0.0/8 , , 192.168.0.0/16 ", []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"all_invalid", "invalid, bad-ip", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseBypassCIDRs(tt.input)
			if tt.expected == nil {
				require.Nil(t, result)
			} else {
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestBuildWAFHandler_ParanoiaLevel verifies paranoia level is correctly set in directives
func TestBuildWAFHandler_ParanoiaLevel(t *testing.T) {
	rulesetPaths := map[string]string{
		"owasp-crs": "/etc/caddy/rules/owasp-crs.conf",
	}
	rulesets := []models.SecurityRuleSet{
		{Name: "owasp-crs"},
	}

	tests := []struct {
		name           string
		paranoiaLevel  int
		expectedLevel  int
		expectedEngine string
	}{
		{"level_1_default", 0, 1, "On"},
		{"level_1_explicit", 1, 1, "On"},
		{"level_2", 2, 2, "On"},
		{"level_3", 3, 3, "On"},
		{"level_4_max", 4, 4, "On"},
		{"level_invalid_high", 5, 1, "On"}, // Invalid falls back to 1
		{"level_invalid_neg", -1, 1, "On"}, // Invalid falls back to 1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secCfg := &models.SecurityConfig{
				WAFMode:          "block",
				WAFParanoiaLevel: tt.paranoiaLevel,
				WAFRulesSource:   "owasp-crs",
			}

			h, err := buildWAFHandler(nil, rulesets, rulesetPaths, secCfg, true)
			require.NoError(t, err)
			require.NotNil(t, h)
			require.Equal(t, "waf", h["handler"])

			directives := h["directives"].(string)
			require.Contains(t, directives, "SecRuleEngine On")
			require.Contains(t, directives, "SecRequestBodyAccess On")
			require.Contains(t, directives, "tx.paranoia_level="+strconv.Itoa(tt.expectedLevel))
		})
	}
}

// TestBuildWAFHandler_Exclusions verifies SecRuleRemoveById directives are generated
func TestBuildWAFHandler_Exclusions(t *testing.T) {
	rulesetPaths := map[string]string{
		"owasp-crs": "/etc/caddy/rules/owasp-crs.conf",
	}
	rulesets := []models.SecurityRuleSet{
		{Name: "owasp-crs"},
	}

	// Test exclusions without targets (full rule removal)
	exclusionsJSON := `[{"rule_id":942100,"description":"SQL Injection rule"},{"rule_id":941100}]`
	secCfg := &models.SecurityConfig{
		WAFMode:        "block",
		WAFRulesSource: "owasp-crs",
		WAFExclusions:  exclusionsJSON,
	}

	h, err := buildWAFHandler(nil, rulesets, rulesetPaths, secCfg, true)
	require.NoError(t, err)
	require.NotNil(t, h)

	directives := h["directives"].(string)
	require.Contains(t, directives, "SecRuleRemoveById 942100")
	require.Contains(t, directives, "SecRuleRemoveById 941100")
}

// TestBuildWAFHandler_ExclusionsWithTarget verifies SecRuleUpdateTargetById directives
func TestBuildWAFHandler_ExclusionsWithTarget(t *testing.T) {
	rulesetPaths := map[string]string{
		"owasp-crs": "/etc/caddy/rules/owasp-crs.conf",
	}
	rulesets := []models.SecurityRuleSet{
		{Name: "owasp-crs"},
	}

	// Test exclusions with targets (partial rule exclusion)
	exclusionsJSON := `[{"rule_id":942100,"target":"ARGS:password"},{"rule_id":941100,"target":"ARGS:content"}]`
	secCfg := &models.SecurityConfig{
		WAFMode:        "block",
		WAFRulesSource: "owasp-crs",
		WAFExclusions:  exclusionsJSON,
	}

	h, err := buildWAFHandler(nil, rulesets, rulesetPaths, secCfg, true)
	require.NoError(t, err)
	require.NotNil(t, h)

	directives := h["directives"].(string)
	require.Contains(t, directives, `SecRuleUpdateTargetById 942100 "!ARGS:password"`)
	require.Contains(t, directives, `SecRuleUpdateTargetById 941100 "!ARGS:content"`)
}

// TestBuildWAFHandler_PerHostDisabled verifies returns nil when host.WAFDisabled is true
func TestBuildWAFHandler_PerHostDisabled(t *testing.T) {
	rulesetPaths := map[string]string{
		"owasp-crs": "/etc/caddy/rules/owasp-crs.conf",
	}
	rulesets := []models.SecurityRuleSet{
		{Name: "owasp-crs"},
	}
	secCfg := &models.SecurityConfig{
		WAFMode:        "block",
		WAFRulesSource: "owasp-crs",
	}

	// Host with WAF disabled
	host := &models.ProxyHost{
		UUID:        "test-uuid",
		WAFDisabled: true,
	}

	h, err := buildWAFHandler(host, rulesets, rulesetPaths, secCfg, true)
	require.NoError(t, err)
	require.Nil(t, h, "WAF handler should be nil when host.WAFDisabled is true")

	// Host with WAF enabled (default)
	host2 := &models.ProxyHost{
		UUID:        "test-uuid-2",
		WAFDisabled: false,
	}

	h2, err := buildWAFHandler(host2, rulesets, rulesetPaths, secCfg, true)
	require.NoError(t, err)
	require.NotNil(t, h2, "WAF handler should not be nil when host.WAFDisabled is false")
}

// TestBuildWAFHandler_MonitorMode verifies DetectionOnly when mode is "monitor"
func TestBuildWAFHandler_MonitorMode(t *testing.T) {
	rulesetPaths := map[string]string{
		"owasp-crs": "/etc/caddy/rules/owasp-crs.conf",
	}
	rulesets := []models.SecurityRuleSet{
		{Name: "owasp-crs"},
	}

	// Monitor mode
	secCfg := &models.SecurityConfig{
		WAFMode:        "monitor",
		WAFRulesSource: "owasp-crs",
	}

	h, err := buildWAFHandler(nil, rulesets, rulesetPaths, secCfg, true)
	require.NoError(t, err)
	require.NotNil(t, h)

	directives := h["directives"].(string)
	require.Contains(t, directives, "SecRuleEngine DetectionOnly")
	require.NotContains(t, directives, "SecRuleEngine On")

	// Block mode
	secCfg2 := &models.SecurityConfig{
		WAFMode:        "block",
		WAFRulesSource: "owasp-crs",
	}

	h2, err := buildWAFHandler(nil, rulesets, rulesetPaths, secCfg2, true)
	require.NoError(t, err)
	require.NotNil(t, h2)

	directives2 := h2["directives"].(string)
	require.Contains(t, directives2, "SecRuleEngine On")
	require.NotContains(t, directives2, "SecRuleEngine DetectionOnly")
}

// TestBuildWAFHandler_GlobalDisabled verifies handler returns nil when globally disabled
func TestBuildWAFHandler_GlobalDisabled(t *testing.T) {
	rulesetPaths := map[string]string{
		"owasp-crs": "/etc/caddy/rules/owasp-crs.conf",
	}
	rulesets := []models.SecurityRuleSet{
		{Name: "owasp-crs"},
	}

	// WAF disabled via wafEnabled flag
	secCfg := &models.SecurityConfig{
		WAFMode:        "block",
		WAFRulesSource: "owasp-crs",
	}

	h, err := buildWAFHandler(nil, rulesets, rulesetPaths, secCfg, false)
	require.NoError(t, err)
	require.Nil(t, h)

	// WAF disabled via SecurityConfig.WAFMode
	secCfg2 := &models.SecurityConfig{
		WAFMode:        "disabled",
		WAFRulesSource: "owasp-crs",
	}

	h2, err := buildWAFHandler(nil, rulesets, rulesetPaths, secCfg2, true)
	require.NoError(t, err)
	require.Nil(t, h2)
}

// TestBuildWAFHandler_NoRuleset verifies handler returns nil when no ruleset available
// WAF without rules is essentially a no-op, so we return nil.
func TestBuildWAFHandler_NoRuleset(t *testing.T) {
	// Empty rulesets and ruleset paths
	secCfg := &models.SecurityConfig{
		WAFMode: "block",
	}

	h, err := buildWAFHandler(nil, nil, nil, secCfg, true)
	require.NoError(t, err)
	require.Nil(t, h, "WAF handler should be nil when no ruleset is available")
}

// TestParseWAFExclusions verifies exclusion parsing from JSON
func TestParseWAFExclusions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []WAFExclusion
	}{
		{
			name:     "empty",
			input:    "",
			expected: nil,
		},
		{
			name:  "single_exclusion",
			input: `[{"rule_id":942100}]`,
			expected: []WAFExclusion{
				{RuleID: 942100},
			},
		},
		{
			name:  "multiple_exclusions",
			input: `[{"rule_id":942100,"description":"SQL Injection"},{"rule_id":941100,"target":"ARGS:password"}]`,
			expected: []WAFExclusion{
				{RuleID: 942100, Description: "SQL Injection"},
				{RuleID: 941100, Target: "ARGS:password"},
			},
		},
		{
			name:     "invalid_json",
			input:    `invalid json`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseWAFExclusions(tt.input)
			if tt.expected == nil {
				require.Nil(t, result)
			} else {
				require.Equal(t, len(tt.expected), len(result))
				for i, e := range tt.expected {
					require.Equal(t, e.RuleID, result[i].RuleID)
					require.Equal(t, e.Target, result[i].Target)
					require.Equal(t, e.Description, result[i].Description)
				}
			}
		})
	}
}

// TestGenerateConfig_WithWAFPerHostDisabled verifies per-host WAF toggle in full config generation
func TestGenerateConfig_WithWAFPerHostDisabled(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "uuid-waf-enabled",
			DomainNames: "waf-enabled.example.com",
			ForwardHost: "app1",
			ForwardPort: 8080,
			Enabled:     true,
			WAFDisabled: false,
		},
		{
			UUID:        "uuid-waf-disabled",
			DomainNames: "waf-disabled.example.com",
			ForwardHost: "app2",
			ForwardPort: 8081,
			Enabled:     true,
			WAFDisabled: true,
		},
	}

	rulesetPaths := map[string]string{
		"owasp-crs": "/etc/caddy/rules/owasp-crs.conf",
	}
	rulesets := []models.SecurityRuleSet{
		{Name: "owasp-crs"},
	}
	secCfg := &models.SecurityConfig{
		WAFMode:        "block",
		WAFRulesSource: "owasp-crs",
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, true, false, false, "", rulesets, rulesetPaths, nil, secCfg, nil)
	require.NoError(t, err)
	require.NotNil(t, config.Apps.HTTP)

	server := config.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	require.Len(t, server.Routes, 4) // 2 hosts × 2 routes each (emergency + main)

	// Check waf-enabled host has WAF handler
	var wafEnabledRoute, wafDisabledRoute *Route
	for _, route := range server.Routes {
		if len(route.Match) > 0 && len(route.Match[0].Host) > 0 {
			switch route.Match[0].Host[0] {
			case "waf-enabled.example.com":
				wafEnabledRoute = route
			case "waf-disabled.example.com":
				wafDisabledRoute = route
			}
		}
	}

	// WAF-enabled route should have WAF handler
	require.NotNil(t, wafEnabledRoute)
	foundWAF := false
	for _, h := range wafEnabledRoute.Handle {
		if h["handler"] == "waf" {
			foundWAF = true
			break
		}
	}
	require.True(t, foundWAF, "WAF handler should be present for waf-enabled host")

	// WAF-disabled route should NOT have WAF handler
	require.NotNil(t, wafDisabledRoute)
	for _, h := range wafDisabledRoute.Handle {
		require.NotEqual(t, "waf", h["handler"], "WAF handler should NOT be present for waf-disabled host")
	}
}

// TestGenerateConfig_WithDisabledHost verifies disabled hosts are skipped
func TestGenerateConfig_WithDisabledHost(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "uuid-enabled",
			DomainNames: "enabled.example.com",
			ForwardHost: "app1",
			ForwardPort: 8080,
			Enabled:     true,
		},
		{
			UUID:        "uuid-disabled",
			DomainNames: "disabled.example.com",
			ForwardHost: "app2",
			ForwardPort: 8081,
			Enabled:     false, // Disabled
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	server := config.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	// Only 2 routes for the enabled host (emergency + main)
	require.Len(t, server.Routes, 2)
	require.Equal(t, []string{"enabled.example.com"}, server.Routes[1].Match[0].Host) // Main route at index 1
}

// TestGenerateConfig_WithFrontendDir verifies catch-all route with frontend
func TestGenerateConfig_WithFrontendDir(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "uuid-1",
			DomainNames: "app.example.com",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "/var/www/html", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	server := config.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	// Should have 3 routes: emergency + main for the host + catch-all for frontend
	require.Len(t, server.Routes, 3)

	// Last route should be catch-all with file_server
	catchAll := server.Routes[2]
	require.Nil(t, catchAll.Match)
	require.True(t, catchAll.Terminal)

	// Check handlers include rewrite and file_server
	var foundRewrite, foundFileServer bool
	for _, h := range catchAll.Handle {
		if h["handler"] == "rewrite" {
			foundRewrite = true
		}
		if h["handler"] == "file_server" {
			foundFileServer = true
		}
	}
	require.True(t, foundRewrite, "catch-all should have rewrite handler")
	require.True(t, foundFileServer, "catch-all should have file_server handler")
}

// TestGenerateConfig_CustomCertificate verifies custom certificates are loaded
func TestGenerateConfig_CustomCertificate(t *testing.T) {
	certUUID := "cert-uuid-123"
	cert := models.SSLCertificate{
		UUID:        certUUID,
		Name:        "Custom Cert",
		Provider:    "custom",
		Certificate: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
		PrivateKey:  "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----",
	}
	certID := uint(1)

	hosts := []models.ProxyHost{
		{
			UUID:          "uuid-1",
			DomainNames:   "secure.example.com",
			ForwardHost:   "app",
			ForwardPort:   8080,
			Enabled:       true,
			CertificateID: &certID,
			Certificate:   &cert,
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	// Check TLS certificates are loaded
	require.NotNil(t, config.Apps.TLS)
	require.NotNil(t, config.Apps.TLS.Certificates)
	require.NotNil(t, config.Apps.TLS.Certificates.LoadPEM)
	require.Len(t, config.Apps.TLS.Certificates.LoadPEM, 1)

	loadPEM := config.Apps.TLS.Certificates.LoadPEM[0]
	require.Equal(t, cert.Certificate, loadPEM.Certificate)
	require.Equal(t, cert.PrivateKey, loadPEM.Key)
	require.Contains(t, loadPEM.Tags, certUUID)
}

// TestGenerateConfig_CustomCertificateMissingData verifies invalid custom certs are skipped
func TestGenerateConfig_CustomCertificateMissingData(t *testing.T) {
	// Certificate missing private key
	cert := models.SSLCertificate{
		UUID:        "cert-uuid-123",
		Name:        "Bad Cert",
		Provider:    "custom",
		Certificate: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
		PrivateKey:  "", // Missing
	}
	certID := uint(1)

	hosts := []models.ProxyHost{
		{
			UUID:          "uuid-1",
			DomainNames:   "secure.example.com",
			ForwardHost:   "app",
			ForwardPort:   8080,
			Enabled:       true,
			CertificateID: &certID,
			Certificate:   &cert,
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	// TLS should be configured but without the invalid custom cert
	if config.Apps.TLS != nil && config.Apps.TLS.Certificates != nil {
		require.Empty(t, config.Apps.TLS.Certificates.LoadPEM)
	}
}

// TestGenerateConfig_LetsEncryptCertificateNotLoaded verifies ACME certs aren't loaded via LoadPEM
func TestGenerateConfig_LetsEncryptCertificateNotLoaded(t *testing.T) {
	cert := models.SSLCertificate{
		UUID:        "cert-uuid-123",
		Name:        "Let's Encrypt Cert",
		Provider:    "letsencrypt", // Not custom
		Certificate: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
		PrivateKey:  "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----",
	}
	certID := uint(1)

	hosts := []models.ProxyHost{
		{
			UUID:          "uuid-1",
			DomainNames:   "secure.example.com",
			ForwardHost:   "app",
			ForwardPort:   8080,
			Enabled:       true,
			CertificateID: &certID,
			Certificate:   &cert,
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	// Let's Encrypt certs should NOT be loaded via LoadPEM (ACME handles them)
	if config.Apps.TLS != nil && config.Apps.TLS.Certificates != nil {
		require.Empty(t, config.Apps.TLS.Certificates.LoadPEM)
	}
}

// TestGenerateConfig_NormalizeAdvancedConfig verifies advanced config normalization
func TestGenerateConfig_NormalizeAdvancedConfig(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:           "uuid-advanced",
			DomainNames:    "advanced.example.com",
			ForwardHost:    "app",
			ForwardPort:    8080,
			Enabled:        true,
			AdvancedConfig: `{"handler": "headers", "response": {"set": {"X-Custom": "value"}}}`,
		},
	}

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	server := config.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	require.Len(t, server.Routes, 2) // Emergency + main route

	route := server.Routes[1] // Main route is at index 1
	// Should have headers handler + reverse_proxy
	require.GreaterOrEqual(t, len(route.Handle), 2)

	var foundHeaders bool
	for _, h := range route.Handle {
		if h["handler"] == "headers" {
			foundHeaders = true
			break
		}
	}
	require.True(t, foundHeaders, "advanced config handler should be present")
}

// TestGenerateConfig_NoACMEEmailNoTLS verifies no TLS config when no ACME email
func TestGenerateConfig_NoACMEEmailNoTLS(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "uuid-1",
			DomainNames: "app.example.com",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}

	// No ACME email
	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	// TLS automation policies should not be set
	require.Nil(t, config.Apps.TLS)
}

// TestGenerateConfig_SecurityDecisionsWithAdminWhitelist verifies admin bypass for blocks
func TestGenerateConfig_SecurityDecisionsWithAdminWhitelist(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "test-uuid",
			DomainNames: "test.example.com",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}

	decisions := []models.SecurityDecision{
		{IP: "1.2.3.4", Action: "block"},
	}

	// With admin whitelist
	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "10.0.0.1/32", nil, nil, decisions, nil, nil)
	require.NoError(t, err)

	server := config.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)

	route := server.Routes[1] // Main route is at index 1
	b, _ := json.Marshal(route.Handle)
	s := string(b)

	// Should contain blocked IP and admin whitelist exclusion
	require.Contains(t, s, "1.2.3.4")
	require.Contains(t, s, "10.0.0.1/32")
}

// TestBuildSecurityHeadersHandler_DefaultProfile verifies default profile when enabled
func TestBuildSecurityHeadersHandler_DefaultProfile(t *testing.T) {
	host := &models.ProxyHost{
		SecurityHeadersEnabled: true,
		SecurityHeaderProfile:  nil, // Use default
	}

	h, err := buildSecurityHeadersHandler(host)
	require.NoError(t, err)
	require.NotNil(t, h)

	response := h["response"].(map[string]any)
	headers := response["set"].(map[string][]string)

	// Should have default HSTS
	require.Contains(t, headers, "Strict-Transport-Security")
	// Should have X-Frame-Options
	require.Contains(t, headers, "X-Frame-Options")
	// Should have X-Content-Type-Options
	require.Contains(t, headers, "X-Content-Type-Options")
}

// TestHasWildcard verifies wildcard detection
func TestHasWildcard(t *testing.T) {
	tests := []struct {
		name     string
		domains  []string
		expected bool
	}{
		{"no_wildcard", []string{"example.com", "test.com"}, false},
		{"with_wildcard", []string{"example.com", "*.test.com"}, true},
		{"only_wildcard", []string{"*.example.com"}, true},
		{"empty", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasWildcard(tt.domains)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestDedupeDomains verifies domain deduplication
func TestDedupeDomains(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"no_dupes", []string{"a.com", "b.com"}, []string{"a.com", "b.com"}},
		{"with_dupes", []string{"a.com", "b.com", "a.com"}, []string{"a.com", "b.com"}},
		{"all_dupes", []string{"a.com", "a.com", "a.com"}, []string{"a.com"}},
		{"empty", []string{}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dedupeDomains(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestNormalizeAdvancedConfig_NestedRoutes verifies nested route normalization
func TestNormalizeAdvancedConfig_NestedRoutes(t *testing.T) {
	// Test with nested routes structure
	input := map[string]any{
		"handler": "subroute",
		"routes": []any{
			map[string]any{
				"handle": []any{
					map[string]any{
						"handler": "headers",
						"response": map[string]any{
							"set": map[string]any{
								"X-Test": "value", // String should become []string
							},
						},
					},
				},
			},
		},
	}

	result := NormalizeAdvancedConfig(input)
	require.NotNil(t, result)

	// The nested headers should be normalized
	m := result.(map[string]any)
	routes := m["routes"].([]any)
	routeMap := routes[0].(map[string]any)
	handles := routeMap["handle"].([]any)
	handlerMap := handles[0].(map[string]any)
	response := handlerMap["response"].(map[string]any)
	setHeaders := response["set"].(map[string]any)

	// String should be converted to []string
	xTest := setHeaders["X-Test"]
	require.IsType(t, []string{}, xTest)
	require.Equal(t, []string{"value"}, xTest)
}

// TestNormalizeAdvancedConfig_ArrayInput verifies array normalization
func TestNormalizeAdvancedConfig_ArrayInput(t *testing.T) {
	input := []any{
		map[string]any{
			"handler": "headers",
			"response": map[string]any{
				"set": map[string]any{
					"X-Test": "value",
				},
			},
		},
	}

	result := NormalizeAdvancedConfig(input)
	require.NotNil(t, result)

	arr := result.([]any)
	require.Len(t, arr, 1)
}

// TestGetCrowdSecAPIKey verifies API key retrieval from environment
func TestGetCrowdSecAPIKey(t *testing.T) {
	const bouncerKeyFile = "/app/data/crowdsec/bouncer_key"

	// Skip if bouncer_key file exists (file takes priority over env vars per Phase 1 of LAPI auth fix)
	if _, err := os.Stat(bouncerKeyFile); err == nil {
		t.Skip("Skipping env var test - bouncer_key file exists (file takes priority over env vars)")
	}

	// Save original values
	origVars := map[string]string{}
	envVars := []string{"CROWDSEC_API_KEY", "CROWDSEC_BOUNCER_API_KEY", "CERBERUS_SECURITY_CROWDSEC_API_KEY", "CHARON_SECURITY_CROWDSEC_API_KEY", "CPM_SECURITY_CROWDSEC_API_KEY"}
	for _, v := range envVars {
		origVars[v] = os.Getenv(v)
		_ = os.Unsetenv(v)
	}
	defer func() {
		for k, v := range origVars {
			if v != "" {
				_ = os.Setenv(k, v)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}()

	// No keys set - should return empty
	result := getCrowdSecAPIKey()
	require.Equal(t, "", result)

	// Set primary key
	_ = os.Setenv("CROWDSEC_API_KEY", "primary-key")
	result = getCrowdSecAPIKey()
	require.Equal(t, "primary-key", result)

	// Test fallback priority
	_ = os.Unsetenv("CROWDSEC_API_KEY")
	_ = os.Setenv("CROWDSEC_BOUNCER_API_KEY", "bouncer-key")
	result = getCrowdSecAPIKey()
	require.Equal(t, "bouncer-key", result)
}
