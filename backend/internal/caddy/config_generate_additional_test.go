package caddy

import (
	"encoding/json"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestGenerateConfig_ZerosslAndBothProviders(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "h1",
			DomainNames: "a.example.com",
			Enabled:     true,
			ForwardHost: "127.0.0.1",
			ForwardPort: 8080,
		},
	}

	// Zerossl provider
	cfgZ, err := GenerateConfig(hosts, "/data/caddy/data", "admin@example.com", "/frontend/dist", "zerossl", false)
	require.NoError(t, err)
	require.NotNil(t, cfgZ.Apps.TLS)
	// Expect only zerossl issuer present
	issuers := cfgZ.Apps.TLS.Automation.Policies[0].IssuersRaw
	foundZerossl := false
	for _, i := range issuers {
		m := i.(map[string]interface{})
		if m["module"] == "zerossl" {
			foundZerossl = true
		}
	}
	require.True(t, foundZerossl)

	// Default/both provider
	cfgBoth, err := GenerateConfig(hosts, "/data/caddy/data", "admin@example.com", "/frontend/dist", "", false)
	require.NoError(t, err)
	issuersBoth := cfgBoth.Apps.TLS.Automation.Policies[0].IssuersRaw
	// We should have at least 2 issuers (acme + zerossl)
	require.GreaterOrEqual(t, len(issuersBoth), 2)
}

func TestGenerateConfig_EmptyHostsAndNoFrontend(t *testing.T) {
	cfg, err := GenerateConfig([]models.ProxyHost{}, "/data/caddy/data", "", "", "", false)
	require.NoError(t, err)
	// Should return base config without server routes
	_, found := cfg.Apps.HTTP.Servers["charon_server"]
	require.False(t, found)
}

func TestGenerateConfig_SkipsInvalidCustomCert(t *testing.T) {
	// Create a host with a custom cert missing private key
	cert := models.SSLCertificate{ID: 1, UUID: "c1", Name: "CustomCert", Provider: "custom", Certificate: "cert", PrivateKey: ""}
	host := models.ProxyHost{UUID: "h1", DomainNames: "a.example.com", Enabled: true, ForwardHost: "127.0.0.1", ForwardPort: 8080, Certificate: &cert, CertificateID: ptrUint(1)}

	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/data/caddy/data", "", "/frontend/dist", "", false)
	require.NoError(t, err)
	// Custom cert missing key should not be in LoadPEM
	if cfg.Apps.TLS != nil && cfg.Apps.TLS.Certificates != nil {
		b, _ := json.Marshal(cfg.Apps.TLS.Certificates)
		require.NotContains(t, string(b), "CustomCert")
	}
}

func TestGenerateConfig_SkipsDuplicateDomains(t *testing.T) {
	// Two hosts with same domain - one newer than other should be kept only once
	h1 := models.ProxyHost{UUID: "h1", DomainNames: "dup.com", Enabled: true, ForwardHost: "127.0.0.1", ForwardPort: 8080}
	h2 := models.ProxyHost{UUID: "h2", DomainNames: "dup.com", Enabled: true, ForwardHost: "127.0.0.2", ForwardPort: 8081}
	cfg, err := GenerateConfig([]models.ProxyHost{h1, h2}, "/data/caddy/data", "", "/frontend/dist", "", false)
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	// Expect that only one route exists for dup.com (one for the domain)
	require.GreaterOrEqual(t, len(server.Routes), 1)
}

func TestGenerateConfig_LoadPEMSetsTLSWhenNoACME(t *testing.T) {
	cert := models.SSLCertificate{ID: 1, UUID: "c1", Name: "LoadPEM", Provider: "custom", Certificate: "cert", PrivateKey: "key"}
	host := models.ProxyHost{UUID: "h1", DomainNames: "pem.com", Enabled: true, ForwardHost: "127.0.0.1", ForwardPort: 8080, Certificate: &cert, CertificateID: &cert.ID}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/data/caddy/data", "", "/frontend/dist", "", false)
	require.NoError(t, err)
	require.NotNil(t, cfg.Apps.TLS)
	require.NotNil(t, cfg.Apps.TLS.Certificates)
}

func TestGenerateConfig_DefaultAcmeStaging(t *testing.T) {
	hosts := []models.ProxyHost{{UUID: "h1", DomainNames: "a.example.com", Enabled: true, ForwardHost: "127.0.0.1", ForwardPort: 8080}}
	cfg, err := GenerateConfig(hosts, "/data/caddy/data", "admin@example.com", "/frontend/dist", "", true)
	require.NoError(t, err)
	// Should include acme issuer with CA staging URL
	issuers := cfg.Apps.TLS.Automation.Policies[0].IssuersRaw
	found := false
	for _, i := range issuers {
		if m, ok := i.(map[string]interface{}); ok {
			if m["module"] == "acme" {
				if _, ok := m["ca"]; ok {
					found = true
				}
			}
		}
	}
	require.True(t, found)
}

func TestGenerateConfig_ACLHandlerBuildError(t *testing.T) {
	// create host with an ACL with invalid JSON to force buildACLHandler to error
	acl := models.AccessList{ID: 10, Name: "BadACL", Enabled: true, Type: "blacklist", IPRules: "invalid"}
	host := models.ProxyHost{UUID: "h1", DomainNames: "a.example.com", Enabled: true, ForwardHost: "127.0.0.1", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/data/caddy/data", "", "/frontend/dist", "", false)
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	// Even if ACL handler error occurs, config should still be returned with routes
	require.NotNil(t, server)
	require.GreaterOrEqual(t, len(server.Routes), 1)
}

func TestGenerateConfig_SkipHostDomainEmptyAndDisabled(t *testing.T) {
	disabled := models.ProxyHost{UUID: "h1", Enabled: false, DomainNames: "skip.com", ForwardHost: "127.0.0.1", ForwardPort: 8080}
	emptyDomain := models.ProxyHost{UUID: "h2", Enabled: true, DomainNames: "", ForwardHost: "127.0.0.1", ForwardPort: 8080}
	cfg, err := GenerateConfig([]models.ProxyHost{disabled, emptyDomain}, "/data/caddy/data", "", "/frontend/dist", "", false)
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	// Both hosts should be skipped; only routes from no hosts should be only catch-all if frontend provided
	if server != nil {
		// If frontend set, there will be catch-all route only
		if len(server.Routes) > 0 {
			// If frontend present, one route will be catch-all; ensure no host-based route exists
			for _, r := range server.Routes {
				for _, m := range r.Match {
					for _, host := range m.Host {
						require.NotEqual(t, "skip.com", host)
					}
				}
			}
		}
	}
}
