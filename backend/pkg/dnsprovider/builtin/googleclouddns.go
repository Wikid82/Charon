package builtin

import (
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// GoogleCloudDNSProvider implements the ProviderPlugin interface for Google Cloud DNS.
type GoogleCloudDNSProvider struct{}

func (p *GoogleCloudDNSProvider) Type() string {
	return "googleclouddns"
}

func (p *GoogleCloudDNSProvider) Metadata() dnsprovider.ProviderMetadata {
	return dnsprovider.ProviderMetadata{
		Type:             "googleclouddns",
		Name:             "Google Cloud DNS",
		Description:      "Google Cloud DNS with service account credentials",
		DocumentationURL: "https://cloud.google.com/dns/docs",
		IsBuiltIn:        true,
		Version:          "1.0.0",
	}
}

func (p *GoogleCloudDNSProvider) Init() error {
	return nil
}

func (p *GoogleCloudDNSProvider) Cleanup() error {
	return nil
}

func (p *GoogleCloudDNSProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "service_account_json",
			Label:       "Service Account JSON",
			Type:        "textarea",
			Placeholder: "Paste your service account JSON key",
			Hint:        "JSON key file content for service account with DNS admin role",
		},
	}
}

func (p *GoogleCloudDNSProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "project_id",
			Label:       "Project ID",
			Type:        "text",
			Placeholder: "my-gcp-project",
			Hint:        "Optional: GCP project ID (auto-detected from service account if not provided)",
		},
	}
}

func (p *GoogleCloudDNSProvider) ValidateCredentials(creds map[string]string) error {
	if creds["service_account_json"] == "" {
		return fmt.Errorf("service_account_json is required")
	}
	return nil
}

func (p *GoogleCloudDNSProvider) TestCredentials(creds map[string]string) error {
	return p.ValidateCredentials(creds)
}

func (p *GoogleCloudDNSProvider) SupportsMultiCredential() bool {
	return false
}

func (p *GoogleCloudDNSProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
	config := map[string]any{
		"name":                 "googleclouddns",
		"service_account_json": creds["service_account_json"],
	}
	if projectID := creds["project_id"]; projectID != "" {
		config["project_id"] = projectID
	}
	return config
}

func (p *GoogleCloudDNSProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	return p.BuildCaddyConfig(creds)
}

func (p *GoogleCloudDNSProvider) PropagationTimeout() time.Duration {
	return 180 * time.Second
}

func (p *GoogleCloudDNSProvider) PollingInterval() time.Duration {
	return 10 * time.Second
}
