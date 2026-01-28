package caddy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestGenerateConfig_CustomCertsAndTLS(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:           "h1",
			DomainNames:    "a.example.com",
			ForwardHost:    "127.0.0.1",
			ForwardPort:    8080,
			Enabled:        true,
			Certificate:    &models.SSLCertificate{ID: 1, UUID: "c1", Name: "CustomCert", Provider: "custom", Certificate: "cert", PrivateKey: "key"},
			CertificateID:  ptrUint(1),
			HSTSEnabled:    true,
			HSTSSubdomains: true,
			BlockExploits:  true,
			Locations:      []models.Location{{Path: "/app", ForwardHost: "127.0.0.1", ForwardPort: 8081}},
		},
	}
	cfg, err := GenerateConfig(hosts, "/data/caddy/data", "admin@example.com", "/frontend/dist", "letsencrypt", true, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	// TLS should be configured
	require.NotNil(t, cfg.Apps.TLS)
	// Custom cert load
	require.NotNil(t, cfg.Apps.TLS.Certificates)
	// One route for the host (with location) plus catch-all -> at least 2 routes
	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.GreaterOrEqual(t, len(server.Routes), 2)
	// Check HSTS header exists in JSON representation
	b, _ := json.Marshal(cfg)
	require.Contains(t, string(b), "Strict-Transport-Security")
}

func ptrUint(v uint) *uint { return &v }

func TestGenerateConfig_EmergencyRoutesBypassSecurity(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "h1",
			DomainNames: "example.com",
			ForwardHost: "127.0.0.1",
			ForwardPort: 8080,
			Enabled:     true,
			AccessList: &models.AccessList{
				Enabled: true,
				Type:    "whitelist",
				IPRules: `[ { "cidr": "10.0.0.0/8", "description": "allow" } ]`,
			},
			AccessListID: ptrUint(1),
		},
	}

	secCfg := &models.SecurityConfig{
		WAFMode:            "enabled",
		WAFRulesSource:     "owasp-crs",
		RateLimitMode:      "enabled",
		RateLimitRequests:  10,
		RateLimitWindowSec: 60,
	}

	rulesets := []models.SecurityRuleSet{
		{Name: "owasp-crs", Content: "SecRuleEngine On"},
	}
	rulesetPaths := map[string]string{"owasp-crs": "/tmp/owasp-crs.conf"}

	cfg, err := GenerateConfig(hosts, "/data/caddy/data", "admin@example.com", "/frontend/dist", "letsencrypt", false, false, true, true, true, "", rulesets, rulesetPaths, nil, secCfg, nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)

	var emergencyRoute *Route
	for _, route := range server.Routes {
		if route == nil {
			continue
		}
		for _, match := range route.Match {
			for _, path := range match.Path {
				if strings.Contains(path, "/api/v1/emergency") || strings.Contains(path, "/emergency/") {
					emergencyRoute = route
					break
				}
			}
		}
	}

	require.NotNil(t, emergencyRoute, "expected emergency bypass route")

	for _, handler := range emergencyRoute.Handle {
		name, _ := handler["handler"].(string)
		require.NotEqual(t, "rate_limit", name)
		require.NotEqual(t, "waf", name)
		require.NotEqual(t, "crowdsec", name)
	}
}
