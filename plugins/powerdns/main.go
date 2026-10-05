package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
	"github.com/Wikid82/charon/backend/pkg/safehttp"
)

const (
	defaultServerID = "localhost"
	requestTimeout  = 10 * time.Second
)

// serverIDPattern restricts server_id to a single, safe URL path segment.
var serverIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// endpointBlockedError is the administrator-facing message for an api_url that
// points at an address the policy does not allow. It unwraps to
// safehttp.ErrBlockedAddress.
type endpointBlockedError struct{}

func (endpointBlockedError) Error() string {
	return "api_url points to an address that is not allowed; use a non-loopback address reachable from Charon (for example a private IPv4 LAN address or a 100.64.x.x Tailscale-style address; IPv6 private addresses are not accepted)"
}

func (endpointBlockedError) Unwrap() error { return safehttp.ErrBlockedAddress }

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

// serverID returns the configured server_id, defaulting to "localhost".
func serverID(creds map[string]string) string {
	if id := creds["server_id"]; id != "" {
		return id
	}
	return defaultServerID
}

func validServerID(id string) bool {
	return id != "." && id != ".." && serverIDPattern.MatchString(id)
}

// endpointError converts a URL validation failure into an administrator-facing
// error; blocked addresses get the remedy message.
func endpointError(err error) error {
	if errors.Is(err, safehttp.ErrBlockedAddress) {
		return endpointBlockedError{}
	}
	return fmt.Errorf("api_url is invalid: %w", err)
}

func (p *PowerDNSProvider) ValidateCredentials(creds map[string]string) error {
	if creds["api_url"] == "" {
		return fmt.Errorf("api_url is required")
	}
	if creds["api_key"] == "" {
		return fmt.Errorf("api_key is required")
	}
	if !validServerID(serverID(creds)) {
		return fmt.Errorf("server_id must be 1-64 characters: letters, digits, '.', '_' or '-'")
	}
	// Syntax and literal-address checks only (no DNS), so bad endpoints are also
	// rejected when credentials are saved.
	if _, err := safehttp.ValidateURLSyntax(creds["api_url"], safehttp.PrivateNetworkOK()); err != nil {
		return endpointError(err)
	}
	return nil
}

func (p *PowerDNSProvider) TestCredentials(creds map[string]string) error {
	if err := p.ValidateCredentials(creds); err != nil {
		return err
	}

	// Early, readable error; the client's dialer re-validates every connection.
	base, err := safehttp.ValidateURL(creds["api_url"], safehttp.PrivateNetworkOK())
	if err != nil {
		return endpointError(err)
	}
	client := safehttp.NewClient(safehttp.PrivateNetworkOK(), requestTimeout)
	return p.probe(client, base, serverID(creds), creds["api_key"])
}

// probe performs the status-only connectivity check. The response body is never
// read or returned.
func (p *PowerDNSProvider) probe(client *http.Client, base *url.URL, id, apiKey string) error {
	if !validServerID(id) {
		return fmt.Errorf("server_id must be 1-64 characters: letters, digits, '.', '_' or '-'")
	}
	target, err := safehttp.JoinPath(base, "api", "v1", "servers", id)
	if err != nil {
		return fmt.Errorf("failed to build request URL: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, safehttp.ErrBlockedAddress) {
			return endpointBlockedError{}
		}
		return fmt.Errorf("API connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	return nil
}

func (p *PowerDNSProvider) SupportsMultiCredential() bool {
	return false
}

func (p *PowerDNSProvider) BuildCaddyConfig(creds map[string]string) map[string]any {
	return map[string]any{
		"name":      "powerdns",
		"api_url":   creds["api_url"],
		"api_key":   creds["api_key"],
		"server_id": serverID(creds),
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
