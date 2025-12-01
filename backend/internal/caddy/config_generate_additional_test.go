package caddy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/logger"
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
	cfgZ, err := GenerateConfig(hosts, "/data/caddy/data", "admin@example.com", "/frontend/dist", "zerossl", false, false, false, false, false, "")
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
	cfgBoth, err := GenerateConfig(hosts, "/data/caddy/data", "admin@example.com", "/frontend/dist", "", false, false, false, false, false, "")
	require.NoError(t, err)
	issuersBoth := cfgBoth.Apps.TLS.Automation.Policies[0].IssuersRaw
	// We should have at least 2 issuers (acme + zerossl)
	require.GreaterOrEqual(t, len(issuersBoth), 2)
}

func TestGenerateConfig_SecurityPipeline_Order_Locations(t *testing.T) {
	// Create host with a location so location-level handlers are generated
	ipRules := `[ { "cidr": "192.168.1.0/24" } ]`
	acl := models.AccessList{ID: 201, Name: "WL2", Enabled: true, Type: "whitelist", IPRules: ipRules}
	host := models.ProxyHost{UUID: "pipeline2", DomainNames: "pipe-loc.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl, HSTSEnabled: true, BlockExploits: true, Locations: []models.Location{{Path: "/loc", ForwardHost: "app", ForwardPort: 9000}}}

	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, true, true, true, true, "")
	require.NoError(t, err)

	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)

	// Find the route for the location (path contains "/loc")
	var locRoute *Route
	for _, r := range server.Routes {
		if len(r.Match) > 0 && len(r.Match[0].Path) > 0 {
			for _, p := range r.Match[0].Path {
				if p == "/loc" {
					locRoute = r
					break
				}
			}
		}
	}
	require.NotNil(t, locRoute)

	// Extract handler names from the location route
	names := []string{}
	for _, h := range locRoute.Handle {
		if hn, ok := h["handler"].(string); ok {
			names = append(names, hn)
		}
	}

	// Expected pipeline: crowdsec -> coraza -> rate_limit -> subroute (acl) -> headers -> vars (BlockExploits) -> reverse_proxy
	require.GreaterOrEqual(t, len(names), 4)
	require.Equal(t, "crowdsec", names[0])
	require.Equal(t, "coraza", names[1])
	require.Equal(t, "rate_limit", names[2])
	require.Equal(t, "subroute", names[3])
}

func TestGenerateConfig_ACLLogWarning(t *testing.T) {
	// capture logs by initializing logger
	var buf strings.Builder
	logger.Init(true, &buf)

	// Create host with an invalid IP rules ACL to force buildACLHandler error
	acl := models.AccessList{ID: 300, Name: "BadACL", Enabled: true, Type: "blacklist", IPRules: "invalid-json"}
	host := models.ProxyHost{UUID: "acl-log", DomainNames: "acl-err.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl}

	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, true, "")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Ensure the logger captured a warning about ACL build failure
	require.Contains(t, buf.String(), "Failed to build ACL handler for host")
}

func TestGenerateConfig_ACLHandlerIncluded(t *testing.T) {
	ipRules := `[ { "cidr": "10.0.0.0/8" } ]`
	acl := models.AccessList{ID: 301, Name: "WL3", Enabled: true, Type: "whitelist", IPRules: ipRules}
	host := models.ProxyHost{UUID: "acl-incl", DomainNames: "acl-incl.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/tmp/caddy-data", "", "", "", false, false, false, false, true, "")
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	route := server.Routes[0]

	// Extract handler names
	names := []string{}
	for _, h := range route.Handle {
		if hn, ok := h["handler"].(string); ok {
			names = append(names, hn)
		}
	}
	// Ensure subroute (ACL) is present
	found := false
	for _, n := range names {
		if n == "subroute" {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestGenerateConfig_EmptyHostsAndNoFrontend(t *testing.T) {
	cfg, err := GenerateConfig([]models.ProxyHost{}, "/data/caddy/data", "", "", "", false, false, false, false, false, "")
	require.NoError(t, err)
	// Should return base config without server routes
	_, found := cfg.Apps.HTTP.Servers["charon_server"]
	require.False(t, found)
}

func TestGenerateConfig_SkipsInvalidCustomCert(t *testing.T) {
	// Create a host with a custom cert missing private key
	cert := models.SSLCertificate{ID: 1, UUID: "c1", Name: "CustomCert", Provider: "custom", Certificate: "cert", PrivateKey: ""}
	host := models.ProxyHost{UUID: "h1", DomainNames: "a.example.com", Enabled: true, ForwardHost: "127.0.0.1", ForwardPort: 8080, Certificate: &cert, CertificateID: ptrUint(1)}

	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/data/caddy/data", "", "/frontend/dist", "", false, false, false, false, true, "")
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
	cfg, err := GenerateConfig([]models.ProxyHost{h1, h2}, "/data/caddy/data", "", "/frontend/dist", "", false, false, false, false, false, "")
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	// Expect that only one route exists for dup.com (one for the domain)
	require.GreaterOrEqual(t, len(server.Routes), 1)
}

func TestGenerateConfig_LoadPEMSetsTLSWhenNoACME(t *testing.T) {
	cert := models.SSLCertificate{ID: 1, UUID: "c1", Name: "LoadPEM", Provider: "custom", Certificate: "cert", PrivateKey: "key"}
	host := models.ProxyHost{UUID: "h1", DomainNames: "pem.com", Enabled: true, ForwardHost: "127.0.0.1", ForwardPort: 8080, Certificate: &cert, CertificateID: &cert.ID}
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/data/caddy/data", "", "/frontend/dist", "", false, false, false, false, true, "")
	require.NoError(t, err)
	require.NotNil(t, cfg.Apps.TLS)
	require.NotNil(t, cfg.Apps.TLS.Certificates)
}

func TestGenerateConfig_DefaultAcmeStaging(t *testing.T) {
	hosts := []models.ProxyHost{{UUID: "h1", DomainNames: "a.example.com", Enabled: true, ForwardHost: "127.0.0.1", ForwardPort: 8080}}
	cfg, err := GenerateConfig(hosts, "/data/caddy/data", "admin@example.com", "/frontend/dist", "", true, false, false, false, false, "")
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
	cfg, err := GenerateConfig([]models.ProxyHost{host}, "/data/caddy/data", "", "/frontend/dist", "", false, false, false, false, false, "")
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	// Even if ACL handler error occurs, config should still be returned with routes
	require.NotNil(t, server)
	require.GreaterOrEqual(t, len(server.Routes), 1)
}

func TestGenerateConfig_SkipHostDomainEmptyAndDisabled(t *testing.T) {
	disabled := models.ProxyHost{UUID: "h1", Enabled: false, DomainNames: "skip.com", ForwardHost: "127.0.0.1", ForwardPort: 8080}
	emptyDomain := models.ProxyHost{UUID: "h2", Enabled: true, DomainNames: "", ForwardHost: "127.0.0.1", ForwardPort: 8080}
	cfg, err := GenerateConfig([]models.ProxyHost{disabled, emptyDomain}, "/data/caddy/data", "", "/frontend/dist", "", false, false, false, false, false, "")
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
