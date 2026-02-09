package builtin

import (
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// NamecheapProvider implements the ProviderPlugin interface for Namecheap DNS.
type NamecheapProvider struct{}

func (p *NamecheapProvider) Type() string {
	return "namecheap"
}

func (p *NamecheapProvider) Metadata() dnsprovider.ProviderMetadata {
	return dnsprovider.ProviderMetadata{
		Type:             "namecheap",
		Name:             "Namecheap",
		Description:      "Namecheap DNS with API credentials",
		DocumentationURL: "https://www.namecheap.com/support/api/intro/",
		IsBuiltIn:        true,
		Version:          "1.0.0",
	}
}

func (p *NamecheapProvider) Init() error {
	return nil
}

func (p *NamecheapProvider) Cleanup() error {
	return nil
}

func (p *NamecheapProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "api_user",
			Label:       "API User",
			Type:        "text",
			Placeholder: "Enter your Namecheap API username",
			Hint:        "Your Namecheap account username",
		},
		{
			Name:        "api_key",
			Label:       "API Key",
			Type:        "password",
			Placeholder: "Enter your Namecheap API key",
			Hint:        "Enable API access in your Namecheap account",
		},
	}
}

func (p *NamecheapProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "client_ip",
			Label:       "Client IP",
			Type:        "text",
			Placeholder: "1.2.3.4",
			Hint:        "Optional: Whitelisted IP address for API access",
		},
	}
}

func (p *NamecheapProvider) ValidateCredentials(creds map[string]string) error {
	if creds["api_user"] == "" {
		return fmt.Errorf("api_user is required")
	}
	if creds["api_key"] == "" {
		return fmt.Errorf("api_key is required")
	}
	return nil
}

func (p *NamecheapProvider) TestCredentials(creds map[string]string) error {
	return p.ValidateCredentials(creds)
}

func (p *NamecheapProvider) SupportsMultiCredential() bool {
	return false
}

func (p *NamecheapProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
	config := map[string]any{
		"name":     "namecheap",
		"api_user": creds["api_user"],
		"api_key":  creds["api_key"],
	}
	if clientIP := creds["client_ip"]; clientIP != "" {
		config["client_ip"] = clientIP
	}
	return config
}

func (p *NamecheapProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	return p.BuildCaddyConfig(creds)
}

func (p *NamecheapProvider) PropagationTimeout() time.Duration {
	return 300 * time.Second
}

func (p *NamecheapProvider) PollingInterval() time.Duration {
	return 15 * time.Second
}
