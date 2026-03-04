package builtin

import (
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// AzureProvider implements the ProviderPlugin interface for Azure DNS.
type AzureProvider struct{}

func (p *AzureProvider) Type() string {
	return "azure"
}

func (p *AzureProvider) Metadata() dnsprovider.ProviderMetadata {
	return dnsprovider.ProviderMetadata{
		Type:             "azure",
		Name:             "Azure DNS",
		Description:      "Microsoft Azure DNS with service principal authentication",
		DocumentationURL: "https://learn.microsoft.com/en-us/azure/dns/",
		IsBuiltIn:        true,
		Version:          "1.0.0",
	}
}

func (p *AzureProvider) Init() error {
	return nil
}

func (p *AzureProvider) Cleanup() error {
	return nil
}

func (p *AzureProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "tenant_id",
			Label:       "Tenant ID",
			Type:        "text",
			Placeholder: "Enter your Azure AD tenant ID",
			Hint:        "Azure Active Directory tenant ID",
		},
		{
			Name:        "client_id",
			Label:       "Client ID",
			Type:        "text",
			Placeholder: "Enter your service principal client ID",
			Hint:        "Service principal (app registration) client ID",
		},
		{
			Name:        "client_secret",
			Label:       "Client Secret",
			Type:        "password",
			Placeholder: "Enter your client secret",
			Hint:        "Service principal client secret",
		},
		{
			Name:        "subscription_id",
			Label:       "Subscription ID",
			Type:        "text",
			Placeholder: "Enter your Azure subscription ID",
			Hint:        "Azure subscription containing DNS zone",
		},
		{
			Name:        "resource_group",
			Label:       "Resource Group",
			Type:        "text",
			Placeholder: "Enter resource group name",
			Hint:        "Resource group containing the DNS zone",
		},
	}
}

func (p *AzureProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{}
}

func (p *AzureProvider) ValidateCredentials(creds map[string]string) error {
	requiredFields := []string{"tenant_id", "client_id", "client_secret", "subscription_id", "resource_group"}
	for _, field := range requiredFields {
		if creds[field] == "" {
			return fmt.Errorf("%s is required", field)
		}
	}
	return nil
}

func (p *AzureProvider) TestCredentials(creds map[string]string) error {
	return p.ValidateCredentials(creds)
}

func (p *AzureProvider) SupportsMultiCredential() bool {
	return false
}

func (p *AzureProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
	return map[string]any{
		"name":            "azure",
		"tenant_id":       creds["tenant_id"],
		"client_id":       creds["client_id"],
		"client_secret":   creds["client_secret"],
		"subscription_id": creds["subscription_id"],
		"resource_group":  creds["resource_group"],
	}
}

func (p *AzureProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	return p.BuildCaddyConfig(creds)
}

func (p *AzureProvider) PropagationTimeout() time.Duration {
	return 180 * time.Second
}

func (p *AzureProvider) PollingInterval() time.Duration {
	return 10 * time.Second
}
