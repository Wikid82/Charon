package builtin

import (
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// DigitalOceanProvider implements the ProviderPlugin interface for DigitalOcean DNS.
type DigitalOceanProvider struct{}

func (p *DigitalOceanProvider) Type() string {
	return "digitalocean"
}

func (p *DigitalOceanProvider) Metadata() dnsprovider.ProviderMetadata {
	return dnsprovider.ProviderMetadata{
		Type:             "digitalocean",
		Name:             "DigitalOcean",
		Description:      "DigitalOcean DNS with API token authentication",
		DocumentationURL: "https://docs.digitalocean.com/reference/api/api-reference/#tag/Domains",
		IsBuiltIn:        true,
		Version:          "1.0.0",
	}
}

func (p *DigitalOceanProvider) Init() error {
	return nil
}

func (p *DigitalOceanProvider) Cleanup() error {
	return nil
}

func (p *DigitalOceanProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "api_token",
			Label:       "API Token",
			Type:        "password",
			Placeholder: "Enter your DigitalOcean API token",
			Hint:        "Generate from API settings in your DigitalOcean account",
		},
	}
}

func (p *DigitalOceanProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{}
}

func (p *DigitalOceanProvider) ValidateCredentials(creds map[string]string) error {
	if creds["api_token"] == "" {
		return fmt.Errorf("api_token is required")
	}
	return nil
}

func (p *DigitalOceanProvider) TestCredentials(creds map[string]string) error {
	return p.ValidateCredentials(creds)
}

func (p *DigitalOceanProvider) SupportsMultiCredential() bool {
	return false
}

func (p *DigitalOceanProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
	return map[string]any{
		"name":      "digitalocean",
		"api_token": creds["api_token"],
	}
}

func (p *DigitalOceanProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	return p.BuildCaddyConfig(creds)
}

func (p *DigitalOceanProvider) PropagationTimeout() time.Duration {
	return 120 * time.Second
}

func (p *DigitalOceanProvider) PollingInterval() time.Duration {
	return 5 * time.Second
}
