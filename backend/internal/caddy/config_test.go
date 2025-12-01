package caddy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestGenerateConfig_Empty(t *testing.T) {
	config, err := GenerateConfig([]models.ProxyHost{}, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, true, "", nil, nil, nil, nil)
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

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, true, "", nil, nil, nil, nil)
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
	require.Len(t, server.Routes, 1)

	route := server.Routes[0]
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

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, true, "", nil, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, config.Apps.HTTP.Servers["charon_server"].Routes, 2)
	require.Len(t, config.Apps.HTTP.Servers["charon_server"].Routes, 2)
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
	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, true, "", nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, config.Apps.HTTP)

	route := config.Apps.HTTP.Servers["charon_server"].Routes[0]
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

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil)
	require.NoError(t, err)
	require.Empty(t, config.Apps.HTTP.Servers["charon_server"].Routes)
	// Should produce empty routes (or just catch-all if frontendDir was set, but it's empty here)
	require.Empty(t, config.Apps.HTTP.Servers["charon_server"].Routes)
}

func TestGenerateConfig_Logging(t *testing.T) {
	hosts := []models.ProxyHost{}
	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil)
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

	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, config)
	require.NotNil(t, config)

	server := config.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	// Should have 2 routes: 1 for location /api, 1 for main domain
	require.Len(t, server.Routes, 2)

	// Check Location Route (should be first as it is more specific)
	locRoute := server.Routes[0]
	require.Equal(t, []string{"/api", "/api/*"}, locRoute.Match[0].Path)
	require.Equal(t, []string{"advanced.example.com"}, locRoute.Match[0].Host)

	// Check Main Route
	mainRoute := server.Routes[1]
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
	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "letsencrypt", true, false, false, false, true, "", nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, config.Apps.TLS)
	require.NotNil(t, config.Apps.TLS)
	require.NotNil(t, config.Apps.TLS.Automation)
	require.Len(t, config.Apps.TLS.Automation.Policies, 1)

	issuers := config.Apps.TLS.Automation.Policies[0].IssuersRaw
	require.Len(t, issuers, 1)

	acmeIssuer := issuers[0].(map[string]interface{})
	require.Equal(t, "acme", acmeIssuer["module"])
	require.Equal(t, "admin@example.com", acmeIssuer["email"])
	require.Equal(t, "https://acme-staging-v02.api.letsencrypt.org/directory", acmeIssuer["ca"])

	// Test with staging disabled (production)
	config, err = GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "letsencrypt", false, false, false, false, false, "", nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, config.Apps.TLS)
	require.NotNil(t, config.Apps.TLS.Automation)
	require.Len(t, config.Apps.TLS.Automation.Policies, 1)

	issuers = config.Apps.TLS.Automation.Policies[0].IssuersRaw
	require.Len(t, issuers, 1)

	acmeIssuer = issuers[0].(map[string]interface{})
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
