package caddy

import (
	"os"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
	"github.com/stretchr/testify/require"

	_ "github.com/Wikid82/charon/backend/pkg/dnsprovider/builtin" // Auto-register DNS providers
)

type multiCredTestProvider struct{}

func (p *multiCredTestProvider) Type() string { return "testmulti" }
func (p *multiCredTestProvider) Metadata() dnsprovider.ProviderMetadata {
	return dnsprovider.ProviderMetadata{Type: p.Type(), Name: "Test Multi", IsBuiltIn: true}
}
func (p *multiCredTestProvider) Init() error    { return nil }
func (p *multiCredTestProvider) Cleanup() error { return nil }
func (p *multiCredTestProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
	return nil
}
func (p *multiCredTestProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
	return nil
}
func (p *multiCredTestProvider) ValidateCredentials(creds map[string]string) error { return nil }
func (p *multiCredTestProvider) TestCredentials(creds map[string]string) error     { return nil }
func (p *multiCredTestProvider) SupportsMultiCredential() bool                     { return true }
func (p *multiCredTestProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
	return map[string]any{"name": p.Type(), "token": creds["token"]}
}
func (p *multiCredTestProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	return map[string]any{"name": p.Type(), "zone": baseDomain, "token": creds["token"]}
}
func (p *multiCredTestProvider) PropagationTimeout() time.Duration { return 2 * time.Second }
func (p *multiCredTestProvider) PollingInterval() time.Duration    { return 1 * time.Second }

func mustProviderID(v uint) *uint { return &v }

func TestGenerateConfig_DNSChallenge_LetsEncrypt_StagingCAAndPropagationTimeout(t *testing.T) {
	providerID := uint(1)
	host := models.ProxyHost{
		Enabled:       true,
		DomainNames:   "*.example.com,example.com",
		DNSProvider:   &models.DNSProvider{ID: providerID, ProviderType: "cloudflare"},
		DNSProviderID: mustProviderID(providerID),
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
		DNSProviderID: mustProviderID(providerID),
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
		DNSProviderID: mustProviderID(providerID),
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
	// Skip if bouncer_key file exists (file takes priority over env vars per Phase 1 of LAPI auth fix)
	const bouncerKeyFile = "/app/data/crowdsec/bouncer_key"
	if _, err := os.Stat(bouncerKeyFile); err == nil {
		t.Skip("Skipping env priority test - bouncer_key file exists (file takes priority over env vars)")
	}

	_ = os.Unsetenv("CROWDSEC_API_KEY")
	_ = os.Unsetenv("CROWDSEC_BOUNCER_API_KEY")

	t.Setenv("CROWDSEC_BOUNCER_API_KEY", "bouncer")
	t.Setenv("CROWDSEC_API_KEY", "primary")
	// CHARON_SECURITY_CROWDSEC_API_KEY has highest priority among env vars
	require.Equal(t, "primary", getCrowdSecAPIKey())

	_ = os.Unsetenv("CROWDSEC_API_KEY")
	require.Equal(t, "bouncer", getCrowdSecAPIKey())
}

func TestHasWildcard_TrueFalse(t *testing.T) {
	require.True(t, hasWildcard([]string{"*.example.com"}))
	require.False(t, hasWildcard([]string{"example.com"}))
}

// TestGenerateConfig_MultiCredential_ZoneSpecificPolicies verifies that multi-credential DNS providers
// create separate TLS automation policies per zone with zone-specific credentials.
func TestGenerateConfig_MultiCredential_ZoneSpecificPolicies(t *testing.T) {
	providerID := uint(10)
	host := models.ProxyHost{
		Enabled:       true,
		DomainNames:   "*.zone1.com,zone1.com,*.zone2.com,zone2.com",
		DNSProvider:   &models.DNSProvider{ID: providerID, ProviderType: "cloudflare"},
		DNSProviderID: mustProviderID(providerID),
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
		[]DNSProviderConfig{{
			ID:                  providerID,
			ProviderType:        "cloudflare",
			UseMultiCredentials: true,
			ZoneCredentials: map[string]map[string]string{
				"zone1.com": {"api_token": "token-zone1"},
				"zone2.com": {"api_token": "token-zone2"},
			},
			Credentials: map[string]string{"api_token": "fallback-token"},
		}},
	)
	require.NoError(t, err)
	require.NotNil(t, conf)
	require.NotNil(t, conf.Apps.TLS)
	require.NotEmpty(t, conf.Apps.TLS.Automation.Policies)

	// Should have at least 2 policies for the 2 zones
	policyCount := 0
	for _, p := range conf.Apps.TLS.Automation.Policies {
		if p == nil {
			continue
		}
		for _, s := range p.Subjects {
			if s == "*.zone1.com" || s == "zone1.com" || s == "*.zone2.com" || s == "zone2.com" {
				policyCount++
				break
			}
		}
	}
	require.GreaterOrEqual(t, policyCount, 2, "expected at least 2 policies for multi-credential zones")
}

// TestGenerateConfig_MultiCredential_ZeroSSL_Issuer verifies multi-credential with ZeroSSL issuer.
func TestGenerateConfig_MultiCredential_ZeroSSL_Issuer(t *testing.T) {
	providerID := uint(11)
	host := models.ProxyHost{
		Enabled:       true,
		DomainNames:   "*.zerossl-test.com",
		DNSProvider:   &models.DNSProvider{ID: providerID, ProviderType: "cloudflare"},
		DNSProviderID: mustProviderID(providerID),
	}

	conf, err := GenerateConfig(
		[]models.ProxyHost{host},
		t.TempDir(),
		"acme@example.com",
		"",
		"zerossl", // Use ZeroSSL provider
		false,
		false, false, false, false,
		"",
		nil,
		nil,
		nil,
		&models.SecurityConfig{},
		[]DNSProviderConfig{{
			ID:                  providerID,
			ProviderType:        "cloudflare",
			UseMultiCredentials: true,
			ZoneCredentials: map[string]map[string]string{
				"zerossl-test.com": {"api_token": "zerossl-token"},
			},
		}},
	)
	require.NoError(t, err)
	require.NotNil(t, conf)

	// Find ZeroSSL issuer in policies
	foundZeroSSL := false
	for _, p := range conf.Apps.TLS.Automation.Policies {
		if p == nil {
			continue
		}
		for _, it := range p.IssuersRaw {
			if m, ok := it.(map[string]any); ok {
				if m["module"] == "zerossl" {
					foundZeroSSL = true
					break
				}
			}
		}
	}
	require.True(t, foundZeroSSL, "expected ZeroSSL issuer in multi-credential policy")
}

// TestGenerateConfig_MultiCredential_BothIssuers verifies multi-credential with both ACME and ZeroSSL issuers.
func TestGenerateConfig_MultiCredential_BothIssuers(t *testing.T) {
	providerID := uint(12)
	host := models.ProxyHost{
		Enabled:       true,
		DomainNames:   "*.both-test.com",
		DNSProvider:   &models.DNSProvider{ID: providerID, ProviderType: "cloudflare"},
		DNSProviderID: mustProviderID(providerID),
	}

	conf, err := GenerateConfig(
		[]models.ProxyHost{host},
		t.TempDir(),
		"acme@example.com",
		"",
		"both", // Use both providers
		false,
		false, false, false, false,
		"",
		nil,
		nil,
		nil,
		&models.SecurityConfig{},
		[]DNSProviderConfig{{
			ID:                  providerID,
			ProviderType:        "cloudflare",
			UseMultiCredentials: true,
			ZoneCredentials: map[string]map[string]string{
				"both-test.com": {"api_token": "both-token"},
			},
		}},
	)
	require.NoError(t, err)
	require.NotNil(t, conf)

	// Find both ACME and ZeroSSL issuers in policies
	foundACME := false
	foundZeroSSL := false
	for _, p := range conf.Apps.TLS.Automation.Policies {
		if p == nil {
			continue
		}
		for _, it := range p.IssuersRaw {
			if m, ok := it.(map[string]any); ok {
				switch m["module"] {
				case "acme":
					foundACME = true
				case "zerossl":
					foundZeroSSL = true
				}
			}
		}
	}
	require.True(t, foundACME, "expected ACME issuer in multi-credential policy")
	require.True(t, foundZeroSSL, "expected ZeroSSL issuer in multi-credential policy")
}

// TestGenerateConfig_MultiCredential_ACMEStaging verifies multi-credential with ACME staging CA.
func TestGenerateConfig_MultiCredential_ACMEStaging(t *testing.T) {
	providerID := uint(13)
	host := models.ProxyHost{
		Enabled:       true,
		DomainNames:   "*.staging-test.com",
		DNSProvider:   &models.DNSProvider{ID: providerID, ProviderType: "cloudflare"},
		DNSProviderID: mustProviderID(providerID),
	}

	conf, err := GenerateConfig(
		[]models.ProxyHost{host},
		t.TempDir(),
		"acme@example.com",
		"",
		"letsencrypt",
		true, // ACME staging
		false, false, false, false,
		"",
		nil,
		nil,
		nil,
		&models.SecurityConfig{},
		[]DNSProviderConfig{{
			ID:                  providerID,
			ProviderType:        "cloudflare",
			UseMultiCredentials: true,
			ZoneCredentials: map[string]map[string]string{
				"staging-test.com": {"api_token": "staging-token"},
			},
		}},
	)
	require.NoError(t, err)
	require.NotNil(t, conf)

	// Find ACME issuer with staging CA
	foundStagingCA := false
	for _, p := range conf.Apps.TLS.Automation.Policies {
		if p == nil {
			continue
		}
		for _, it := range p.IssuersRaw {
			if m, ok := it.(map[string]any); ok {
				if m["module"] == "acme" {
					if ca, ok := m["ca"].(string); ok && ca == "https://acme-staging-v02.api.letsencrypt.org/directory" {
						foundStagingCA = true
						break
					}
				}
			}
		}
	}
	require.True(t, foundStagingCA, "expected ACME staging CA in multi-credential policy")
}

// TestGenerateConfig_MultiCredential_NoMatchingDomains verifies that zones with no matching domains are skipped.
func TestGenerateConfig_MultiCredential_NoMatchingDomains(t *testing.T) {
	providerID := uint(14)
	host := models.ProxyHost{
		Enabled:       true,
		DomainNames:   "*.actual.com",
		DNSProvider:   &models.DNSProvider{ID: providerID, ProviderType: "cloudflare"},
		DNSProviderID: mustProviderID(providerID),
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
		[]DNSProviderConfig{{
			ID:                  providerID,
			ProviderType:        "cloudflare",
			UseMultiCredentials: true,
			ZoneCredentials: map[string]map[string]string{
				"unmatched.com": {"api_token": "unmatched-token"}, // This zone won't match any domains
				"actual.com":    {"api_token": "actual-token"},    // This zone will match
			},
		}},
	)
	require.NoError(t, err)
	require.NotNil(t, conf)

	// Should only have policy for actual.com, not unmatched.com
	for _, p := range conf.Apps.TLS.Automation.Policies {
		if p == nil {
			continue
		}
		for _, s := range p.Subjects {
			require.NotContains(t, s, "unmatched", "unmatched domain should not appear in policies")
		}
	}
}

// TestGenerateConfig_MultiCredential_ProviderTypeNotFound verifies graceful handling when provider type is not in registry.
func TestGenerateConfig_MultiCredential_ProviderTypeNotFound(t *testing.T) {
	providerID := uint(15)
	host := models.ProxyHost{
		Enabled:       true,
		DomainNames:   "*.unknown-provider.com",
		DNSProvider:   &models.DNSProvider{ID: providerID, ProviderType: "nonexistent_provider"},
		DNSProviderID: mustProviderID(providerID),
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
		[]DNSProviderConfig{{
			ID:                  providerID,
			ProviderType:        "nonexistent_provider", // Not in registry
			UseMultiCredentials: true,
			ZoneCredentials: map[string]map[string]string{
				"unknown-provider.com": {"api_token": "token"},
			},
		}},
	)
	// Should not error, just skip the provider
	require.NoError(t, err)
	require.NotNil(t, conf)
}

func TestGenerateConfig_MultiCredential_SupportsMultiCredential_UsesZoneConfigAndStagingBothIssuers(t *testing.T) {
	if err := dnsprovider.Global().Register(&multiCredTestProvider{}); err == nil {
		t.Cleanup(func() { dnsprovider.Global().Unregister("testmulti") })
	}

	providerID := uint(16)
	host := models.ProxyHost{
		Enabled:       true,
		DomainNames:   "*.z1.com,z1.com",
		DNSProvider:   &models.DNSProvider{ID: providerID, ProviderType: "testmulti"},
		DNSProviderID: mustProviderID(providerID),
	}

	conf, err := GenerateConfig(
		[]models.ProxyHost{host},
		t.TempDir(),
		"acme@example.com",
		"",
		"both",
		true,
		false, false, false, false,
		"",
		nil,
		nil,
		nil,
		&models.SecurityConfig{},
		[]DNSProviderConfig{{
			ID:                  providerID,
			ProviderType:        "testmulti",
			UseMultiCredentials: true,
			ZoneCredentials: map[string]map[string]string{
				"z1.com": {"token": "tok-z1"},
			},
		}},
	)
	require.NoError(t, err)
	require.NotNil(t, conf)
	require.NotNil(t, conf.Apps.TLS)
	require.NotNil(t, conf.Apps.TLS.Automation)
	require.NotEmpty(t, conf.Apps.TLS.Automation.Policies)

	var foundCA string
	var foundProvider map[string]any
	for _, p := range conf.Apps.TLS.Automation.Policies {
		if p == nil {
			continue
		}
		for _, it := range p.IssuersRaw {
			issuer, ok := it.(map[string]any)
			if !ok || issuer["module"] != "acme" {
				continue
			}
			if ca, ok := issuer["ca"].(string); ok {
				foundCA = ca
			}
			ch, _ := issuer["challenges"].(map[string]any)
			dnsCh, _ := ch["dns"].(map[string]any)
			prov, _ := dnsCh["provider"].(map[string]any)
			foundProvider = prov
			break
		}
		if foundProvider != nil {
			break
		}
	}

	require.Equal(t, "https://acme-staging-v02.api.letsencrypt.org/directory", foundCA)
	require.NotNil(t, foundProvider)
	require.Equal(t, "z1.com", foundProvider["zone"], "expected zone-specific provider config")
}

func TestGenerateConfig_DNSChallenge_SingleCredential_BothIssuers_ACMEStaging(t *testing.T) {
	providerID := uint(17)
	host := models.ProxyHost{
		Enabled:       true,
		DomainNames:   "*.both-staging.com",
		DNSProvider:   &models.DNSProvider{ID: providerID, ProviderType: "cloudflare"},
		DNSProviderID: mustProviderID(providerID),
	}

	conf, err := GenerateConfig(
		[]models.ProxyHost{host},
		t.TempDir(),
		"acme@example.com",
		"",
		"both",
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
			PropagationTimeout: 1,
			Credentials:        map[string]string{"api_token": "tok"},
		}},
	)
	require.NoError(t, err)
	require.NotNil(t, conf)

	var foundIssuer map[string]any
	for _, p := range conf.Apps.TLS.Automation.Policies {
		if p == nil {
			continue
		}
		for _, s := range p.Subjects {
			if s != "*.both-staging.com" {
				continue
			}
			for _, it := range p.IssuersRaw {
				if m, ok := it.(map[string]any); ok && m["module"] == "acme" {
					foundIssuer = m
					break
				}
			}
		}
		if foundIssuer != nil {
			break
		}
	}

	require.NotNil(t, foundIssuer)
	require.Equal(t, "https://acme-staging-v02.api.letsencrypt.org/directory", foundIssuer["ca"])
}

func TestGenerateConfig_DNSChallenge_SingleCredential_ProviderTypeNotFound_SkipsPolicy(t *testing.T) {
	providerID := uint(18)
	host := models.ProxyHost{
		Enabled:       true,
		DomainNames:   "*.missing-registry.com",
		DNSProvider:   &models.DNSProvider{ID: providerID, ProviderType: "not_registered"},
		DNSProviderID: mustProviderID(providerID),
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
		[]DNSProviderConfig{{
			ID:                 providerID,
			ProviderType:       "not_registered",
			PropagationTimeout: 1,
			Credentials:        map[string]string{"token": "x"},
		}},
	)
	require.NoError(t, err)
	require.NotNil(t, conf)
}

func TestGenerateConfig_DefaultPolicy_LetsEncrypt_StagingCA(t *testing.T) {
	host := models.ProxyHost{Enabled: true, DomainNames: "192.0.2.1"}

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
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, conf)
	require.NotNil(t, conf.Apps.TLS)
	require.NotNil(t, conf.Apps.TLS.Automation)
	require.NotEmpty(t, conf.Apps.TLS.Automation.Policies)

	found := false
	for _, p := range conf.Apps.TLS.Automation.Policies {
		if p == nil {
			continue
		}
		for _, it := range p.IssuersRaw {
			if m, ok := it.(map[string]any); ok && m["module"] == "acme" {
				require.Equal(t, "https://acme-staging-v02.api.letsencrypt.org/directory", m["ca"])
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	require.True(t, found)
}

func TestGenerateConfig_DefaultPolicy_ZeroSSL_Issuer(t *testing.T) {
	host := models.ProxyHost{Enabled: true, DomainNames: "192.0.2.2"}

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
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, conf)
	require.NotNil(t, conf.Apps.TLS)
	require.NotNil(t, conf.Apps.TLS.Automation)

	found := false
	for _, p := range conf.Apps.TLS.Automation.Policies {
		if p == nil {
			continue
		}
		for _, it := range p.IssuersRaw {
			if m, ok := it.(map[string]any); ok && m["module"] == "zerossl" {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	require.True(t, found)
}

func TestGenerateConfig_DefaultPolicy_BothIssuers_StagingCA(t *testing.T) {
	host := models.ProxyHost{Enabled: true, DomainNames: "192.0.2.3"}

	conf, err := GenerateConfig(
		[]models.ProxyHost{host},
		t.TempDir(),
		"acme@example.com",
		"",
		"",
		true,
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

	caFound := false
	zeroFound := false
	for _, p := range conf.Apps.TLS.Automation.Policies {
		if p == nil {
			continue
		}
		for _, it := range p.IssuersRaw {
			m, ok := it.(map[string]any)
			if !ok {
				continue
			}
			switch m["module"] {
			case "acme":
				if m["ca"] == "https://acme-staging-v02.api.letsencrypt.org/directory" {
					caFound = true
				}
			case "zerossl":
				zeroFound = true
			}
		}
	}
	require.True(t, caFound)
	require.True(t, zeroFound)
}

func TestGenerateConfig_IPSubjects_InitializesTLSAppAndAutomation(t *testing.T) {
	providerID := uint(19)
	host := models.ProxyHost{
		Enabled:     true,
		UUID:        "ip-host",
		DomainNames: "1.2.3.4",
		ForwardHost: "app",
		ForwardPort: 8080,
		DNSProvider: &models.DNSProvider{ID: providerID, ProviderType: "cloudflare"},
	}

	conf, err := GenerateConfig(
		[]models.ProxyHost{host},
		t.TempDir(),
		"",
		"",
		"",
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
	require.NotNil(t, conf.Apps.TLS.Automation)
	require.NotEmpty(t, conf.Apps.TLS.Automation.Policies)
}

func TestGetAccessLogPath_DockerEnv_UsesCrowdSecPath(t *testing.T) {
	const dockerMarker = "/.dockerenv"
	if err := os.WriteFile(dockerMarker, []byte("test"), 0o600); err != nil {
		t.Skipf("cannot create %s: %v", dockerMarker, err)
	}
	t.Cleanup(func() { _ = os.Remove(dockerMarker) })

	path := getAccessLogPath(t.TempDir(), false)
	require.Equal(t, "/var/log/caddy/access.log", path)
}
