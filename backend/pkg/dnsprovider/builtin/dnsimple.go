package builtin

import (
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// DNSimpleProvider implements the ProviderPlugin interface for DNSimple.
type DNSimpleProvider struct{}

func (p *DNSimpleProvider) Type() string {
	return "dnsimple"
}

func (p *DNSimpleProvider) Metadata() dnsprovider.ProviderMetadata {
	return dnsprovider.ProviderMetadata{
		Type:             "dnsimple",
		Name:             "DNSimple",
		Description:      "DNSimple DNS with API token authentication",
		DocumentationURL: "https://developer.dnsimple.com/",
		IsBuiltIn:        true,
		Version:          "1.0.0",
	}
}

func (p *DNSimpleProvider) Init() error {
	return nil
}

func (p *DNSimpleProvider) Cleanup() error {
	return nil
}

func (p *DNSimpleProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "api_token",
			Label:       "API Token",
			Type:        "password",
			Placeholder: "Enter your DNSimple API token",
			Hint:        "OAuth token or Account API token",
		},
	}
}

func (p *DNSimpleProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "account_id",
			Label:       "Account ID",
			Type:        "text",
			Placeholder: "12345",
			Hint:        "Optional: Your DNSimple account ID",
		},
	}
}

func (p *DNSimpleProvider) ValidateCredentials(creds map[string]string) error {
	if creds["api_token"] == "" {
		return fmt.Errorf("api_token is required")
	}
	return nil
}

func (p *DNSimpleProvider) TestCredentials(creds map[string]string) error {
	return p.ValidateCredentials(creds)
}

func (p *DNSimpleProvider) SupportsMultiCredential() bool {
	return false
}

func (p *DNSimpleProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
	config := map[string]any{
		"name":      "dnsimple",
		"api_token": creds["api_token"],
	}
	if accountID := creds["account_id"]; accountID != "" {
		config["account_id"] = accountID
	}
	return config
}

func (p *DNSimpleProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	return p.BuildCaddyConfig(creds)
}

func (p *DNSimpleProvider) PropagationTimeout() time.Duration {
	return 120 * time.Second
}

func (p *DNSimpleProvider) PollingInterval() time.Duration {
	return 5 * time.Second
}
