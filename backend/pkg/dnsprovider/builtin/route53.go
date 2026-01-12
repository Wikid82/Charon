package builtin

import (
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// Route53Provider implements the ProviderPlugin interface for AWS Route53.
type Route53Provider struct{}

func (p *Route53Provider) Type() string {
	return "route53"
}

func (p *Route53Provider) Metadata() dnsprovider.ProviderMetadata {
	return dnsprovider.ProviderMetadata{
		Type:             "route53",
		Name:             "AWS Route53",
		Description:      "Amazon Route53 DNS with IAM credentials",
		DocumentationURL: "https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/",
		IsBuiltIn:        true,
		Version:          "1.0.0",
	}
}

func (p *Route53Provider) Init() error {
	return nil
}

func (p *Route53Provider) Cleanup() error {
	return nil
}

func (p *Route53Provider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "access_key_id",
			Label:       "Access Key ID",
			Type:        "text",
			Placeholder: "Enter your AWS Access Key ID",
			Hint:        "IAM user with Route53 permissions",
		},
		{
			Name:        "secret_access_key",
			Label:       "Secret Access Key",
			Type:        "password",
			Placeholder: "Enter your AWS Secret Access Key",
			Hint:        "Stored encrypted",
		},
	}
}

func (p *Route53Provider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "region",
			Label:       "AWS Region",
			Type:        "text",
			Placeholder: "us-east-1",
			Hint:        "AWS region (default: us-east-1)",
		},
		{
			Name:        "hosted_zone_id",
			Label:       "Hosted Zone ID",
			Type:        "text",
			Placeholder: "Z1234567890ABC",
			Hint:        "Optional: Specific hosted zone ID",
		},
	}
}

func (p *Route53Provider) ValidateCredentials(creds map[string]string) error {
	if creds["access_key_id"] == "" {
		return fmt.Errorf("access_key_id is required")
	}
	if creds["secret_access_key"] == "" {
		return fmt.Errorf("secret_access_key is required")
	}
	return nil
}

func (p *Route53Provider) TestCredentials(creds map[string]string) error {
	return p.ValidateCredentials(creds)
}

func (p *Route53Provider) SupportsMultiCredential() bool {
	return false
}

func (p *Route53Provider) BuildCaddyConfig(creds map[string]string) map[string]any {
	config := map[string]any{
		"name":              "route53",
		"access_key_id":     creds["access_key_id"],
		"secret_access_key": creds["secret_access_key"],
	}
	if region := creds["region"]; region != "" {
		config["region"] = region
	}
	if zoneID := creds["hosted_zone_id"]; zoneID != "" {
		config["hosted_zone_id"] = zoneID
	}
	return config
}

func (p *Route53Provider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	return p.BuildCaddyConfig(creds)
}

func (p *Route53Provider) PropagationTimeout() time.Duration {
	return 180 * time.Second
}

func (p *Route53Provider) PollingInterval() time.Duration {
	return 10 * time.Second
}
