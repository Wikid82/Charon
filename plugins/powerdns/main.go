package main

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// Plugin is the exported symbol that Charon looks for.
var Plugin dnsprovider.ProviderPlugin = &PowerDNSProvider{}

// PowerDNSProvider implements the ProviderPlugin interface for PowerDNS.
type PowerDNSProvider struct{}

func (p *PowerDNSProvider) Type() string {
	return "powerdns"
}

func (p *PowerDNSProvider) Metadata() dnsprovider.ProviderMetadata {
	return dnsprovider.ProviderMetadata{
		Type:             "powerdns",
		Name:             "PowerDNS",
		Description:      "PowerDNS Authoritative Server with HTTP API",
		DocumentationURL: "https://doc.powerdns.com/authoritative/http-api/",
		Author:           "Charon Community",
		Version:          "1.0.0",
		IsBuiltIn:        false,
		GoVersion:        runtime.Version(),
		InterfaceVersion: dnsprovider.InterfaceVersion,
	}
}

func (p *PowerDNSProvider) Init() error {
	return nil
}

func (p *PowerDNSProvider) Cleanup() error {
	return nil
}

func (p *PowerDNSProvider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "api_url",
			Label:       "API URL",
			Type:        "text",
			Placeholder: "https://pdns.example.com:8081",
			Hint:        "PowerDNS HTTP API endpoint",
		},
		{
			Name:        "api_key",
			Label:       "API Key",
			Type:        "password",
			Placeholder: "Your PowerDNS API key",
			Hint:        "X-API-Key header value",
		},
	}
}

func (p *PowerDNSProvider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "server_id",
			Label:       "Server ID",
			Type:        "text",
			Placeholder: "localhost",
			Hint:        "PowerDNS server ID (default: localhost)",
		},
	}
}

func (p *PowerDNSProvider) ValidateCredentials(creds map[string]string) error {
	if creds["api_url"] == "" {
		return fmt.Errorf("api_url is required")
	}
	if creds["api_key"] == "" {
		return fmt.Errorf("api_key is required")
	}
	return nil
}

func (p *PowerDNSProvider) TestCredentials(creds map[string]string) error {
	if err := p.ValidateCredentials(creds); err != nil {
		return err
	}

	// Test API connectivity
	serverID := creds["server_id"]
	if serverID == "" {
		serverID = "localhost"
	}
	url := fmt.Sprintf("%s/api/v1/servers/%s", creds["api_url"], serverID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-API-Key", creds["api_key"])
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("API connection failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	return nil
}

func (p *PowerDNSProvider) SupportsMultiCredential() bool {
	return false
}

func (p *PowerDNSProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
	serverID := creds["server_id"]
	if serverID == "" {
		serverID = "localhost"
	}
	return map[string]any{
		"name":      "powerdns",
		"api_url":   creds["api_url"],
		"api_key":   creds["api_key"],
		"server_id": serverID,
	}
}

func (p *PowerDNSProvider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	return p.BuildCaddyConfig(creds)
}

func (p *PowerDNSProvider) PropagationTimeout() time.Duration {
	return 60 * time.Second
}

func (p *PowerDNSProvider) PollingInterval() time.Duration {
	return 2 * time.Second
}

func main() {}
