package caddy

import (
	"os"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"

	_ "github.com/Wikid82/charon/backend/pkg/dnsprovider/builtin" // Auto-register DNS providers
)

func TestGenerateConfig_DNSChallenge_LetsEncrypt_StagingCAAndPropagationTimeout(t *testing.T) {
	providerID := uint(1)
	host := models.ProxyHost{
		Enabled:       true,
		DomainNames:   "*.example.com,example.com",
		DNSProvider:   &models.DNSProvider{ID: providerID, ProviderType: "cloudflare"},
		DNSProviderID: func() *uint { v := providerID; return &v }(),
	}

	conf, err := GenerateConfig(
		[]models.ProxyHost{host},
		t.TempDir(),
		"acme@example.com",
		"",
		"letsencrypt",
		true,
		false, false, false, false,
		"",
		nil,
		nil,
		nil,
		&models.SecurityConfig{},
		[]DNSProviderConfig{{
			ID:                 providerID,
			ProviderType:       "cloudflare",
			PropagationTimeout: 120,
			Credentials:        map[string]string{"api_token": "tok"},
		}},
	)
	require.NoError(t, err)
	require.NotNil(t, conf)
	require.NotNil(t, conf.Apps.TLS)
	require.NotNil(t, conf.Apps.TLS.Automation)
	require.NotEmpty(t, conf.Apps.TLS.Automation.Policies)

	// Find a policy that includes the wildcard subject
	var foundIssuer map[string]any
	for _, p := range conf.Apps.TLS.Automation.Policies {
		if p == nil {
			continue
		}
		for _, s := range p.Subjects {
			if s != "*.example.com" {
				continue
			}
			require.NotEmpty(t, p.IssuersRaw)
			for _, it := range p.IssuersRaw {
				if m, ok := it.(map[string]any); ok {
					if m["module"] == "acme" {
						foundIssuer = m
						break
					}
				}
			}
		}
		if foundIssuer != nil {
			break
		}
	}

	require.NotNil(t, foundIssuer)
	require.Equal(t, "https://acme-staging-v02.api.letsencrypt.org/directory", foundIssuer["ca"])

	challenges, ok := foundIssuer["challenges"].(map[string]any)
	require.True(t, ok)
	dns, ok := challenges["dns"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, int64(120)*1_000_000_000, dns["propagation_timeout"])
}

func TestGenerateConfig_DNSChallenge_ZeroSSL_IssuerShape(t *testing.T) {
	providerID := uint(2)
	host := models.ProxyHost{
		Enabled:       true,
		DomainNames:   "*.example.net",
		DNSProvider:   &models.DNSProvider{ID: providerID, ProviderType: "cloudflare"},
		DNSProviderID: func() *uint { v := providerID; return &v }(),
	}

	conf, err := GenerateConfig(
		[]models.ProxyHost{host},
		t.TempDir(),
		"acme@example.com",
		"",
		"zerossl",
		false,
		false, false, false, false,
		"",
		nil,
		nil,
		nil,
		&models.SecurityConfig{},
		[]DNSProviderConfig{{
			ID:                 providerID,
			ProviderType:       "cloudflare",
			PropagationTimeout: 5,
			Credentials:        map[string]string{"api_token": "tok"},
		}},
	)
	require.NoError(t, err)
	require.NotNil(t, conf)
	require.NotNil(t, conf.Apps.TLS)
	require.NotEmpty(t, conf.Apps.TLS.Automation.Policies)

	// Expect at least one issuer with module zerossl
	found := false
	for _, p := range conf.Apps.TLS.Automation.Policies {
		if p == nil {
			continue
		}
		for _, it := range p.IssuersRaw {
			if m, ok := it.(map[string]any); ok {
				if m["module"] == "zerossl" {
					found = true
				}
			}
		}
	}
	require.True(t, found)
}

func TestGenerateConfig_DNSChallenge_SkipsPolicyWhenProviderConfigMissing(t *testing.T) {
	providerID := uint(3)
	host := models.ProxyHost{
		Enabled:       true,
		DomainNames:   "*.example.org",
		DNSProvider:   &models.DNSProvider{ID: providerID, ProviderType: "cloudflare"},
		DNSProviderID: func() *uint { v := providerID; return &v }(),
	}

	conf, err := GenerateConfig(
		[]models.ProxyHost{host},
		t.TempDir(),
		"acme@example.com",
		"",
		"letsencrypt",
		false,
		false, false, false, false,
		"",
		nil,
		nil,
		nil,
		&models.SecurityConfig{},
		nil, // no provider configs available
	)
	require.NoError(t, err)
	require.NotNil(t, conf)
	require.NotNil(t, conf.Apps.TLS)
	require.NotEmpty(t, conf.Apps.TLS.Automation.Policies)

	// No policy should include the wildcard subject since provider config was missing
	for _, p := range conf.Apps.TLS.Automation.Policies {
		if p == nil {
			continue
		}
		for _, s := range p.Subjects {
			require.NotEqual(t, "*.example.org", s)
		}
	}
}

func TestGenerateConfig_HTTPChallenge_ExcludesIPDomains(t *testing.T) {
	host := models.ProxyHost{Enabled: true, DomainNames: "example.com,192.168.1.1"}

	conf, err := GenerateConfig(
		[]models.ProxyHost{host},
		t.TempDir(),
		"acme@example.com",
		"",
		"letsencrypt",
		false,
		false, false, false, false,
		"",
		nil,
		nil,
		nil,
		&models.SecurityConfig{},
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, conf)
	require.NotNil(t, conf.Apps.TLS)
	require.NotEmpty(t, conf.Apps.TLS.Automation.Policies)

	for _, p := range conf.Apps.TLS.Automation.Policies {
		if p == nil {
			continue
		}
		for _, s := range p.Subjects {
			require.NotEqual(t, "192.168.1.1", s)
		}
	}
}

func TestGetCrowdSecAPIKey_EnvPriority(t *testing.T) {
	os.Unsetenv("CROWDSEC_API_KEY")
	os.Unsetenv("CROWDSEC_BOUNCER_API_KEY")

	t.Setenv("CROWDSEC_BOUNCER_API_KEY", "bouncer")
	t.Setenv("CROWDSEC_API_KEY", "primary")
	require.Equal(t, "primary", getCrowdSecAPIKey())

	os.Unsetenv("CROWDSEC_API_KEY")
	require.Equal(t, "bouncer", getCrowdSecAPIKey())
}

func TestHasWildcard_TrueFalse(t *testing.T) {
	require.True(t, hasWildcard([]string{"*.example.com"}))
	require.False(t, hasWildcard([]string{"example.com"}))
}
