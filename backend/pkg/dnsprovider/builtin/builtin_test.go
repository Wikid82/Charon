package builtin

import (
	"testing"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

func TestCloudflareProvider(t *testing.T) {
	p := &CloudflareProvider{}

	if p.Type() != "cloudflare" {
		t.Errorf("expected type cloudflare, got %s", p.Type())
	}

	meta := p.Metadata()
	if meta.Name != "Cloudflare" {
		t.Errorf("expected name Cloudflare, got %s", meta.Name)
	}
	if !meta.IsBuiltIn {
		t.Error("expected IsBuiltIn to be true")
	}

	if err := p.Init(); err != nil {
		t.Errorf("Init failed: %v", err)
	}

	if err := p.Cleanup(); err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}

	required := p.RequiredCredentialFields()
	if len(required) != 1 {
		t.Errorf("expected 1 required field, got %d", len(required))
	}
	if required[0].Name != "api_token" {
		t.Errorf("expected api_token field, got %s", required[0].Name)
	}

	optional := p.OptionalCredentialFields()
	if len(optional) != 1 {
		t.Errorf("expected 1 optional field, got %d", len(optional))
	}
	if optional[0].Name != "zone_id" {
		t.Errorf("expected zone_id field, got %s", optional[0].Name)
	}

	// Test credential validation
	err := p.ValidateCredentials(map[string]string{})
	if err == nil {
		t.Error("expected validation error for empty credentials")
	}

	err = p.ValidateCredentials(map[string]string{"api_token": "test"})
	if err != nil {
		t.Errorf("validation failed: %v", err)
	}

	if p.SupportsMultiCredential() {
		t.Error("expected SupportsMultiCredential to be false")
	}

	config := p.BuildCaddyConfig(map[string]string{"api_token": "test"})
	if config["name"] != "cloudflare" {
		t.Error("expected caddy config name to be cloudflare")
	}
	if config["api_token"] != "test" {
		t.Error("expected api_token in caddy config")
	}

	timeout := p.PropagationTimeout()
	if timeout.Seconds() == 0 {
		t.Error("expected non-zero propagation timeout")
	}

	interval := p.PollingInterval()
	if interval.Seconds() == 0 {
		t.Error("expected non-zero polling interval")
	}
}

func TestRoute53Provider(t *testing.T) {
	p := &Route53Provider{}

	if p.Type() != "route53" {
		t.Errorf("expected type route53, got %s", p.Type())
	}

	required := p.RequiredCredentialFields()
	if len(required) != 2 {
		t.Errorf("expected 2 required fields, got %d", len(required))
	}

	err := p.ValidateCredentials(map[string]string{})
	if err == nil {
		t.Error("expected validation error for empty credentials")
	}

	err = p.ValidateCredentials(map[string]string{
		"access_key_id":     "test",
		"secret_access_key": "test",
	})
	if err != nil {
		t.Errorf("validation failed: %v", err)
	}
}

func TestDigitalOceanProvider(t *testing.T) {
	p := &DigitalOceanProvider{}

	if p.Type() != "digitalocean" {
		t.Errorf("expected type digitalocean, got %s", p.Type())
	}

	required := p.RequiredCredentialFields()
	if len(required) != 1 {
		t.Errorf("expected 1 required field, got %d", len(required))
	}

	err := p.ValidateCredentials(map[string]string{})
	if err == nil {
		t.Error("expected validation error for empty credentials")
	}

	err = p.ValidateCredentials(map[string]string{"api_token": "test"})
	if err != nil {
		t.Errorf("validation failed: %v", err)
	}
}

func TestGoogleCloudDNSProvider(t *testing.T) {
	p := &GoogleCloudDNSProvider{}

	if p.Type() != "googleclouddns" {
		t.Errorf("expected type googleclouddns, got %s", p.Type())
	}

	required := p.RequiredCredentialFields()
	if len(required) != 1 {
		t.Errorf("expected 1 required field, got %d", len(required))
	}

	err := p.ValidateCredentials(map[string]string{})
	if err == nil {
		t.Error("expected validation error for empty credentials")
	}
}

func TestAzureProvider(t *testing.T) {
	p := &AzureProvider{}

	if p.Type() != "azure" {
		t.Errorf("expected type azure, got %s", p.Type())
	}

	required := p.RequiredCredentialFields()
	if len(required) != 5 {
		t.Errorf("expected 5 required fields, got %d", len(required))
	}
}

func TestNamecheapProvider(t *testing.T) {
	p := &NamecheapProvider{}

	if p.Type() != "namecheap" {
		t.Errorf("expected type namecheap, got %s", p.Type())
	}

	required := p.RequiredCredentialFields()
	if len(required) != 2 {
		t.Errorf("expected 2 required fields, got %d", len(required))
	}
}

func TestGoDaddyProvider(t *testing.T) {
	p := &GoDaddyProvider{}

	if p.Type() != "godaddy" {
		t.Errorf("expected type godaddy, got %s", p.Type())
	}

	required := p.RequiredCredentialFields()
	if len(required) != 2 {
		t.Errorf("expected 2 required fields, got %d", len(required))
	}
}

func TestHetznerProvider(t *testing.T) {
	p := &HetznerProvider{}

	if p.Type() != "hetzner" {
		t.Errorf("expected type hetzner, got %s", p.Type())
	}

	required := p.RequiredCredentialFields()
	if len(required) != 1 {
		t.Errorf("expected 1 required field, got %d", len(required))
	}
}

func TestVultrProvider(t *testing.T) {
	p := &VultrProvider{}

	if p.Type() != "vultr" {
		t.Errorf("expected type vultr, got %s", p.Type())
	}

	required := p.RequiredCredentialFields()
	if len(required) != 1 {
		t.Errorf("expected 1 required field, got %d", len(required))
	}
}

func TestDNSimpleProvider(t *testing.T) {
	p := &DNSimpleProvider{}

	if p.Type() != "dnsimple" {
		t.Errorf("expected type dnsimple, got %s", p.Type())
	}

	required := p.RequiredCredentialFields()
	if len(required) != 1 {
		t.Errorf("expected 1 required field, got %d", len(required))
	}
}

func TestProviderRegistration(t *testing.T) {
	// Test that all providers are registered after init
	providers := []string{
		"cloudflare",
		"route53",
		"digitalocean",
		"googleclouddns",
		"azure",
		"namecheap",
		"godaddy",
		"hetzner",
		"vultr",
		"dnsimple",
	}

	for _, providerType := range providers {
		provider, ok := dnsprovider.Global().Get(providerType)
		if !ok {
			t.Errorf("provider %s not registered", providerType)
		}
		if provider == nil {
			t.Errorf("provider %s is nil", providerType)
		}
	}

	// Test GetTypes
	types := dnsprovider.Global().Types()
	if len(types) < len(providers) {
		t.Errorf("expected at least %d types, got %d", len(providers), len(types))
	}

	// Test IsSupported
	for _, providerType := range providers {
		if !dnsprovider.Global().IsSupported(providerType) {
			t.Errorf("provider %s should be supported", providerType)
		}
	}

	if dnsprovider.Global().IsSupported("invalid-provider") {
		t.Error("invalid provider should not be supported")
	}
}
