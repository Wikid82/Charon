package builtin

import (
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// GoDaddyProvider implements the ProviderPlugin interface for GoDaddy DNS.
type GoDaddyProvider struct{}

func (p *GoDaddyProvider) Type() string {
	return "godaddy"
}

func (p *GoDaddyProvider) Metadata() dnsprovider.ProviderMetadata {
	return dnsprovider.ProviderMetadata{
		Type:             "godaddy",
		Name:             "GoDaddy",
		Description:      "GoDaddy DNS with API key and secret",
		DocumentationURL: "https://developer.godaddy.com/doc",
		IsBuiltIn:        true,
		Version:          "1.0.0",
	}
}

func (p *GoDaddyProvider) Init() error {
	return nil
}

func (p *GoDaddyProvider) Cleanup() error {
	return nil
}

func (p *GoDaddyProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "api_key",
			Label:       "API Key",
			Type:        "text",
			Placeholder: "Enter your GoDaddy API key",
			Hint:        "Production API key from GoDaddy developer portal",
		},
		{
			Name:        "api_secret",
			Label:       "API Secret",
			Type:        "password",
			Placeholder: "Enter your GoDaddy API secret",
			Hint:        "Production API secret (stored encrypted)",
		},
	}
}

func (p *GoDaddyProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{}
}

func (p *GoDaddyProvider) ValidateCredentials(creds map[string]string) error {
	if creds["api_key"] == "" {
		return fmt.Errorf("api_key is required")
	}
	if creds["api_secret"] == "" {
		return fmt.Errorf("api_secret is required")
	}
	return nil
}

func (p *GoDaddyProvider) TestCredentials(creds map[string]string) error {
	return p.ValidateCredentials(creds)
}

func (p *GoDaddyProvider) SupportsMultiCredential() bool {
	return false
}

func (p *GoDaddyProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
	return map[string]any{
		"name":       "godaddy",
		"api_key":    creds["api_key"],
		"api_secret": creds["api_secret"],
	}
}

func (p *GoDaddyProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	return p.BuildCaddyConfig(creds)
}

func (p *GoDaddyProvider) PropagationTimeout() time.Duration {
	return 180 * time.Second
}

func (p *GoDaddyProvider) PollingInterval() time.Duration {
	return 10 * time.Second
}
