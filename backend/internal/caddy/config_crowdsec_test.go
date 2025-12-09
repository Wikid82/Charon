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
	// When crowdsecEnabled is true but no secCfg, should use default localhost URL
	h, err := buildCrowdSecHandler(nil, nil, true)
	require.NoError(t, err)
	require.NotNil(t, h)

	assert.Equal(t, "crowdsec", h["handler"])
	assert.Equal(t, "http://localhost:8080", h["api_url"])
}

func TestBuildCrowdSecHandler_EnabledWithEmptyAPIURL(t *testing.T) {
	// When crowdsecEnabled is true but CrowdSecAPIURL is empty, should use default
	secCfg := &models.SecurityConfig{
		CrowdSecAPIURL: "",
	}
	h, err := buildCrowdSecHandler(nil, secCfg, true)
	require.NoError(t, err)
	require.NotNil(t, h)

	assert.Equal(t, "crowdsec", h["handler"])
	assert.Equal(t, "http://localhost:8080", h["api_url"])
}

func TestBuildCrowdSecHandler_EnabledWithCustomAPIURL(t *testing.T) {
	// When crowdsecEnabled is true and CrowdSecAPIURL is set, should use custom URL
	secCfg := &models.SecurityConfig{
		CrowdSecAPIURL: "http://crowdsec-lapi:8081",
	}
	h, err := buildCrowdSecHandler(nil, secCfg, true)
	require.NoError(t, err)
	require.NotNil(t, h)

	assert.Equal(t, "crowdsec", h["handler"])
	assert.Equal(t, "http://crowdsec-lapi:8081", h["api_url"])
}

func TestBuildCrowdSecHandler_JSONFormat(t *testing.T) {
	// Test that the handler produces valid JSON matching caddy-crowdsec-bouncer schema
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

	// Verify expected JSON content
	assert.Contains(t, s, `"handler":"crowdsec"`)
	assert.Contains(t, s, `"api_url":"http://localhost:8080"`)
	// Should NOT contain old "mode" field
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
	assert.Equal(t, "http://custom-crowdsec:8080", h["api_url"])
}

func TestGenerateConfig_WithCrowdSec(t *testing.T) {
	// Test that CrowdSec handler is included in generated config when enabled
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
		CrowdSecAPIURL: "http://localhost:8080",
	}

	// crowdsecEnabled=true should include the handler
	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, true, false, false, false, "", nil, nil, nil, secCfg)
	require.NoError(t, err)
	require.NotNil(t, config.Apps.HTTP)

	server := config.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	require.Len(t, server.Routes, 1)

	route := server.Routes[0]
	// Handlers should include crowdsec + reverse_proxy
	require.GreaterOrEqual(t, len(route.Handle), 2)

	// Find the crowdsec handler
	var foundCrowdSec bool
	for _, h := range route.Handle {
		if h["handler"] == "crowdsec" {
			foundCrowdSec = true
			// Verify it has api_url
			assert.Equal(t, "http://localhost:8080", h["api_url"])
			break
		}
	}
	require.True(t, foundCrowdSec, "crowdsec handler should be present")
}

func TestGenerateConfig_CrowdSecDisabled(t *testing.T) {
	// Test that CrowdSec handler is NOT included when disabled
	hosts := []models.ProxyHost{
		{
			UUID:        "test-uuid",
			DomainNames: "example.com",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}

	// crowdsecEnabled=false should NOT include the handler
	config, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, false, "", nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, config.Apps.HTTP)

	server := config.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	require.Len(t, server.Routes, 1)

	route := server.Routes[0]

	// Verify no crowdsec handler
	for _, h := range route.Handle {
		assert.NotEqual(t, "crowdsec", h["handler"], "crowdsec handler should not be present when disabled")
	}
}
