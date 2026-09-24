package caddy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestGenerateConfig_RedirectHostsOnly confirms a deployment with only
// RedirectionHosts (no ProxyHost rows at all) still produces a server with
// routes — regression guard for the len(hosts)==0 early-return that used to
// ignore redirectHosts entirely
// (docs/plans/archive/2026-09-23_redirection-hosts-1367_spec.md §4.2).
func TestGenerateConfig_RedirectHostsOnly(t *testing.T) {
	redirectHosts := []models.RedirectionHost{
		{
			UUID:        "rh-1",
			DomainNames: "old.example.com",
			TargetURL:   "https://new.example.com",
			StatusCode:  301,
			Enabled:     true,
		},
	}

	cfg, err := GenerateConfig([]models.ProxyHost{}, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil, WithRedirectionHosts(redirectHosts))
	require.NoError(t, err)
	require.NotNil(t, cfg.Apps.HTTP)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	require.Len(t, server.Routes, 1)
	assert.Equal(t, []string{"old.example.com"}, server.Routes[0].Match[0].Host)
	assert.Equal(t, "static_response", server.Routes[0].Handle[0]["handler"])
}

// TestGenerateConfig_RedirectHostsPrecedeProxyHostRoutes confirms
// BuildRedirectRoutes' routes are appended before the ProxyHost loop's
// routes, per docs/plans/archive/2026-09-23_redirection-hosts-1367_spec.md §4.2.
func TestGenerateConfig_RedirectHostsPrecedeProxyHostRoutes(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "ph-1",
			DomainNames: "proxied.example.com",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}
	redirectHosts := []models.RedirectionHost{
		{
			UUID:        "rh-1",
			DomainNames: "redirected.example.com",
			TargetURL:   "https://elsewhere.example.com",
			StatusCode:  301,
			Enabled:     true,
		},
	}

	cfg, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil, WithRedirectionHosts(redirectHosts))
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	// Redirect route first, then ProxyHost's emergency + main routes.
	require.Len(t, server.Routes, 3)
	assert.Equal(t, []string{"redirected.example.com"}, server.Routes[0].Match[0].Host)
	assert.Equal(t, "static_response", server.Routes[0].Handle[0]["handler"])
}

// TestGenerateConfig_CrossResourceGhostHost_RedirectionWinsOverProxyHost
// confirms a domain claimed by both a RedirectionHost and a ProxyHost (which
// should never happen given the service-layer CheckDomainConflict check, but
// is defended in depth here) results in only the RedirectionHost's route
// being emitted, since its domains are registered into processedDomains
// first.
func TestGenerateConfig_CrossResourceGhostHost_RedirectionWinsOverProxyHost(t *testing.T) {
	hosts := []models.ProxyHost{
		{
			UUID:        "ph-1",
			DomainNames: "shared.example.com",
			ForwardHost: "app",
			ForwardPort: 8080,
			Enabled:     true,
		},
	}
	redirectHosts := []models.RedirectionHost{
		{
			UUID:        "rh-1",
			DomainNames: "shared.example.com",
			TargetURL:   "https://elsewhere.example.com",
			StatusCode:  301,
			Enabled:     true,
		},
	}

	cfg, err := GenerateConfig(hosts, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil, WithRedirectionHosts(redirectHosts))
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	require.Len(t, server.Routes, 1) // ProxyHost's route was dropped as a Ghost Host
	assert.Equal(t, "static_response", server.Routes[0].Handle[0]["handler"])
}

// TestGenerateConfig_RedirectHostDomains_GetTLSAutomationPolicy confirms a
// RedirectionHost's domain is included in the TLS automation policy's
// subjects, so it gets a certificate issued exactly like a ProxyHost domain
// does (docs/plans/archive/2026-09-23_redirection-hosts-1367_spec.md §4.2).
func TestGenerateConfig_RedirectHostDomains_GetTLSAutomationPolicy(t *testing.T) {
	redirectHosts := []models.RedirectionHost{
		{
			UUID:        "rh-1",
			DomainNames: "old.example.com",
			TargetURL:   "https://new.example.com",
			StatusCode:  301,
			Enabled:     true,
		},
	}

	cfg, err := GenerateConfig([]models.ProxyHost{}, "/tmp/caddy-data", "admin@example.com", "", "letsencrypt", false, false, false, false, true, "", nil, nil, nil, nil, nil, WithRedirectionHosts(redirectHosts))
	require.NoError(t, err)
	require.NotNil(t, cfg.Apps.TLS)
	require.NotNil(t, cfg.Apps.TLS.Automation)
	var found bool
	for _, policy := range cfg.Apps.TLS.Automation.Policies {
		for _, subj := range policy.Subjects {
			if subj == "old.example.com" {
				found = true
			}
		}
	}
	assert.True(t, found, "redirect host domain should be present in a TLS automation policy's subjects")
}

// TestGenerateConfig_DisabledRedirectHost_NoRouteNoTLS confirms a disabled
// RedirectionHost contributes neither a route nor a TLS automation subject.
func TestGenerateConfig_DisabledRedirectHost_NoRouteNoTLS(t *testing.T) {
	redirectHosts := []models.RedirectionHost{
		{
			UUID:        "rh-1",
			DomainNames: "disabled.example.com",
			TargetURL:   "https://new.example.com",
			StatusCode:  301,
			Enabled:     false,
		},
	}

	cfg, err := GenerateConfig([]models.ProxyHost{}, "/tmp/caddy-data", "admin@example.com", "", "letsencrypt", false, false, false, false, true, "", nil, nil, nil, nil, nil, WithRedirectionHosts(redirectHosts))
	require.NoError(t, err)
	server := cfg.Apps.HTTP.Servers["charon_server"]
	if server != nil {
		assert.Empty(t, server.Routes)
	}
	if cfg.Apps.TLS != nil && cfg.Apps.TLS.Automation != nil {
		for _, policy := range cfg.Apps.TLS.Automation.Policies {
			assert.NotContains(t, policy.Subjects, "disabled.example.com")
		}
	}
}

// TestGenerateConfig_RedirectHostCustomCertificate confirms a RedirectionHost's
// custom (non-ACME) certificate is included in the TLS load_pem config, the
// same way a ProxyHost's custom certificate is.
func TestGenerateConfig_RedirectHostCustomCertificate(t *testing.T) {
	cert := &models.SSLCertificate{
		UUID:        "cert-1",
		Provider:    "custom",
		Certificate: "-----BEGIN CERTIFICATE-----\nfake\n-----END CERTIFICATE-----",
		PrivateKey:  "-----BEGIN PRIVATE KEY-----\nfake\n-----END PRIVATE KEY-----",
	}
	certID := uint(1)
	cert.ID = certID
	redirectHosts := []models.RedirectionHost{
		{
			UUID:          "rh-1",
			DomainNames:   "old.example.com",
			TargetURL:     "https://new.example.com",
			StatusCode:    301,
			Enabled:       true,
			CertificateID: &certID,
			Certificate:   cert,
		},
	}

	cfg, err := GenerateConfig([]models.ProxyHost{}, "/tmp/caddy-data", "admin@example.com", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil, WithRedirectionHosts(redirectHosts))
	require.NoError(t, err)
	require.NotNil(t, cfg.Apps.TLS)
	require.NotNil(t, cfg.Apps.TLS.Certificates)
	require.Len(t, cfg.Apps.TLS.Certificates.LoadPEM, 1)
	assert.Contains(t, cfg.Apps.TLS.Certificates.LoadPEM[0].Tags, "cert-1")
}
