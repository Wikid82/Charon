package builtin

import (
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// CloudflareProvider implements the ProviderPlugin interface for Cloudflare DNS.
type CloudflareProvider struct{}

func (p *CloudflareProvider) Type() string {
	return "cloudflare"
}

func (p *CloudflareProvider) Metadata() dnsprovider.ProviderMetadata {
	return dnsprovider.ProviderMetadata{
		Type:             "cloudflare",
		Name:             "Cloudflare",
		Description:      "Cloudflare DNS with API Token authentication",
		DocumentationURL: "https://developers.cloudflare.com/api/tokens/create/",
		IsBuiltIn:        true,
		Version:          "1.0.0",
	}
}

func (p *CloudflareProvider) Init() error {
	return nil
}

func (p *CloudflareProvider) Cleanup() error {
	return nil
}

func (p *CloudflareProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "api_token",
			Label:       "API Token",
			Type:        "password",
			Placeholder: "Enter your Cloudflare API token",
			Hint:        "Token requires Zone:DNS:Edit permission",
		},
	}
}

func (p *CloudflareProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "zone_id",
			Label:       "Zone ID",
			Type:        "text",
			Placeholder: "Optional: Specific zone ID",
			Hint:        "Leave empty to auto-detect zone",
		},
	}
}

func (p *CloudflareProvider) ValidateCredentials(creds map[string]string) error {
	if creds["api_token"] == "" {
		return fmt.Errorf("api_token is required")
	}
	return nil
}

func (p *CloudflareProvider) TestCredentials(creds map[string]string) error {
	return p.ValidateCredentials(creds)
}

func (p *CloudflareProvider) SupportsMultiCredential() bool {
	return false
}

func (p *CloudflareProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
	config := map[string]any{
		"name":      "cloudflare",
		"api_token": creds["api_token"],
	}
	if zoneID := creds["zone_id"]; zoneID != "" {
		config["zone_id"] = zoneID
	}
	return config
}

func (p *CloudflareProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	return p.BuildCaddyConfig(creds)
}

func (p *CloudflareProvider) PropagationTimeout() time.Duration {
	return 120 * time.Second
}

func (p *CloudflareProvider) PollingInterval() time.Duration {
	return 5 * time.Second
}
