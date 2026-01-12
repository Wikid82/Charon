package builtin

import (
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// VultrProvider implements the ProviderPlugin interface for Vultr DNS.
type VultrProvider struct{}

func (p *VultrProvider) Type() string {
	return "vultr"
}

func (p *VultrProvider) Metadata() dnsprovider.ProviderMetadata {
	return dnsprovider.ProviderMetadata{
		Type:             "vultr",
		Name:             "Vultr",
		Description:      "Vultr DNS with API key authentication",
		DocumentationURL: "https://www.vultr.com/api/#tag/dns",
		IsBuiltIn:        true,
		Version:          "1.0.0",
	}
}

func (p *VultrProvider) Init() error {
	return nil
}

func (p *VultrProvider) Cleanup() error {
	return nil
}

func (p *VultrProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "api_key",
			Label:       "API Key",
			Type:        "password",
			Placeholder: "Enter your Vultr API key",
			Hint:        "Generate from Vultr account settings",
		},
	}
}

func (p *VultrProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{}
}

func (p *VultrProvider) ValidateCredentials(creds map[string]string) error {
	if creds["api_key"] == "" {
		return fmt.Errorf("api_key is required")
	}
	return nil
}

func (p *VultrProvider) TestCredentials(creds map[string]string) error {
	return p.ValidateCredentials(creds)
}

func (p *VultrProvider) SupportsMultiCredential() bool {
	return false
}

func (p *VultrProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
	return map[string]any{
		"name":    "vultr",
		"api_key": creds["api_key"],
	}
}

func (p *VultrProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	return p.BuildCaddyConfig(creds)
}

func (p *VultrProvider) PropagationTimeout() time.Duration {
	return 120 * time.Second
}

func (p *VultrProvider) PollingInterval() time.Duration {
	return 5 * time.Second
}
