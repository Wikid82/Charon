package caddy

import (
	"encoding/json"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCrowdSecHandler_Disabled(t *testing.T) {
	// When crowdsecEnabled is false, should return nil
	h, err := buildCrowdSecHandler(nil, nil, false)
	require.NoError(t, err)
	assert.Nil(t, h)
}

func TestBuildCrowdSecHandler_EnabledWithoutConfig(t *testing.T) {
	// When crowdsecEnabled is true, should return minimal handler
	h, err := buildCrowdSecHandler(nil, nil, true)
	require.NoError(t, err)
	require.NotNil(t, h)

	assert.Equal(t, "crowdsec", h["handler"])
	// No inline config - all config is at app-level
	assert.Nil(t, h["lapi_url"])
	assert.Nil(t, h["api_key"])
}

func TestBuildCrowdSecHandler_EnabledWithEmptyAPIURL(t *testing.T) {
	// When crowdsecEnabled is true, should return minimal handler
	secCfg := &models.SecurityConfig{
		CrowdSecAPIURL: "",
	}
	h, err := buildCrowdSecHandler(nil, secCfg, true)
	require.NoError(t, err)
	require.NotNil(t, h)

	assert.Equal(t, "crowdsec", h["handler"])
	// No inline config - all config is at app-level
	assert.Nil(t, h["lapi_url"])
}

func TestBuildCrowdSecHandler_EnabledWithCustomAPIURL(t *testing.T) {
	// When crowdsecEnabled is true, should return minimal handler
	// Custom API URL is configured at app-level, not in handler
	secCfg := &models.SecurityConfig{
		CrowdSecAPIURL: "http://crowdsec-lapi:8081",
	}
	h, err := buildCrowdSecHandler(nil, secCfg, true)
	require.NoError(t, err)
	require.NotNil(t, h)

	assert.Equal(t, "crowdsec", h["handler"])
	// No inline config - all config is at app-level
	assert.Nil(t, h["lapi_url"])
}

func TestBuildCrowdSecHandler_JSONFormat(t *testing.T) {
	// Test that the handler produces valid JSON with minimal structure
	secCfg := &models.SecurityConfig{
		CrowdSecAPIURL: "http://localhost:8080",
	}
	h, err := buildCrowdSecHandler(nil, secCfg, true)
	require.NoError(t, err)
	require.NotNil(t, h)

	// Marshal to JSON and verify structure
	b, err := json.Marshal(h)
	require.NoError(t, err)
	s := string(b)

	// Verify minimal JSON content
	assert.Contains(t, s, `"handler":"crowdsec"`)
	// Should NOT contain inline config fields
	assert.NotContains(t, s, `"lapi_url"`)
	assert.NotContains(t, s, `"api_key"`)
	assert.NotContains(t, s, `"mode"`)
}

func TestBuildCrowdSecHandler_WithHost(t *testing.T) {
	// Test that host parameter is accepted (even if not currently used)
	host := &models.ProxyHost{
		UUID:        "test-uuid",
		DomainNames: "example.com",
	}
	secCfg := &models.SecurityConfig{
		CrowdSecAPIURL: "http://custom-crowdsec:8080",
	}

	h, err := buildCrowdSecHandler(host, secCfg, true)
	require.NoError(t, err)
	require.NotNil(t, h)

	assert.Equal(t, "crowdsec", h["handler"])
	// No inline config - all config is at app-level
	assert.Nil(t, h["lapi_url"])
}

func TestGenerateConfig_WithCrowdSec(t *testing.T) {
	// Test that CrowdSec is configured at app-level when enabled
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
		CrowdSecMode:   "local",
		CrowdSecAPIURL: "http://localhost:8085",
	}

	// crowdsecEnabled=true should configure app-level CrowdSec
	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, true, false, false, false, "", nil, nil, nil, secCfg, nil)
	require.NoError(t, err)
	require.NotNil(t, config.Apps.HTTP)

	// Check app-level CrowdSec configuration
	require.NotNil(t, config.Apps.CrowdSec, "CrowdSec app config should be present")
	assert.Equal(t, "http://localhost:8085", config.Apps.CrowdSec.APIUrl)
	assert.Equal(t, "60s", config.Apps.CrowdSec.TickerInterval)
	assert.NotNil(t, config.Apps.CrowdSec.EnableStreaming)
	assert.True(t, *config.Apps.CrowdSec.EnableStreaming)

	// Check server-level trusted_proxies configuration
	server := config.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server, "Server should be configured")
	require.NotNil(t, server.TrustedProxies, "TrustedProxies should be configured at server level")
	assert.Equal(t, "static", server.TrustedProxies.Source, "TrustedProxies source should be 'static'")
	assert.Contains(t, server.TrustedProxies.Ranges, "127.0.0.1/32", "Should trust localhost")
	assert.Contains(t, server.TrustedProxies.Ranges, "::1/128", "Should trust IPv6 localhost")
	assert.Contains(t, server.TrustedProxies.Ranges, "172.16.0.0/12", "Should trust Docker networks")
	assert.Contains(t, server.TrustedProxies.Ranges, "10.0.0.0/8", "Should trust private networks")
	assert.Contains(t, server.TrustedProxies.Ranges, "192.168.0.0/16", "Should trust private networks")

	// Check handler is minimal (2 routes: emergency + main)
	require.Len(t, server.Routes, 2)

	route := server.Routes[1] // Main route is at index 1
	// Handlers should include crowdsec + reverse_proxy
	require.GreaterOrEqual(t, len(route.Handle), 2)

	// Find the crowdsec handler
	var foundCrowdSec bool
	for _, h := range route.Handle {
		if h["handler"] == "crowdsec" {
			foundCrowdSec = true
			// Verify it has NO inline config
			assert.Nil(t, h["lapi_url"], "Handler should not have inline lapi_url")
			assert.Nil(t, h["api_key"], "Handler should not have inline api_key")
			break
		}
	}
	require.True(t, foundCrowdSec, "crowdsec handler should be present")
}

func TestGenerateConfig_CrowdSecDisabled(t *testing.T) {
	// Test that CrowdSec is NOT configured when disabled
	hosts := []models.ProxyHost{
		{
			UUID:        "test-uuid",
			DomainNames: "example.com",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}

	// crowdsecEnabled=false should NOT configure CrowdSec
	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, config.Apps.HTTP)

	// No app-level CrowdSec configuration
	assert.Nil(t, config.Apps.CrowdSec, "CrowdSec app config should not be present when disabled")

	server := config.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	require.Len(t, server.Routes, 2) // 2 routes: emergency + main

	route := server.Routes[1] // Main route is at index 1

	// Verify no crowdsec handler
	for _, h := range route.Handle {
		assert.NotEqual(t, "crowdsec", h["handler"], "crowdsec handler should not be present when disabled")
	}
}
