package builtin

import (
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// HetznerProvider implements the ProviderPlugin interface for Hetzner DNS.
type HetznerProvider struct{}

func (p *HetznerProvider) Type() string {
	return "hetzner"
}

func (p *HetznerProvider) Metadata() dnsprovider.ProviderMetadata {
	return dnsprovider.ProviderMetadata{
		Type:             "hetzner",
		Name:             "Hetzner",
		Description:      "Hetzner DNS with API token authentication",
		DocumentationURL: "https://dns.hetzner.com/api-docs",
		IsBuiltIn:        true,
		Version:          "1.0.0",
	}
}

func (p *HetznerProvider) Init() error {
	return nil
}

func (p *HetznerProvider) Cleanup() error {
	return nil
}

func (p *HetznerProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "api_token",
			Label:       "API Token",
			Type:        "password",
			Placeholder: "Enter your Hetzner DNS API token",
			Hint:        "Generate from Hetzner DNS Console",
		},
	}
}

func (p *HetznerProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{}
}

func (p *HetznerProvider) ValidateCredentials(creds map[string]string) error {
	if creds["api_token"] == "" {
		return fmt.Errorf("api_token is required")
	}
	return nil
}

func (p *HetznerProvider) TestCredentials(creds map[string]string) error {
	return p.ValidateCredentials(creds)
}

func (p *HetznerProvider) SupportsMultiCredential() bool {
	return false
}

func (p *HetznerProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
	return map[string]any{
		"name":      "hetzner",
		"api_token": creds["api_token"],
	}
}

func (p *HetznerProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	return p.BuildCaddyConfig(creds)
}

func (p *HetznerProvider) PropagationTimeout() time.Duration {
	return 120 * time.Second
}

func (p *HetznerProvider) PollingInterval() time.Duration {
	return 5 * time.Second
}
