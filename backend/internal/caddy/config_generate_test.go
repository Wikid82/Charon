package caddy

import (
	"encoding/json"
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
	cfg, err := GenerateConfig(hosts, "/data/caddy/data", "admin@example.com", "/frontend/dist", "letsencrypt", true, false, false, false, false, "")
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
