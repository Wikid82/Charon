// Package custom provides custom DNS provider plugins for non-built-in integrations.
package custom

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// RFC 2136 provider constants.
const (
	RFC2136DefaultPort               = "53"
	RFC2136DefaultAlgorithm          = "hmac-sha256"
	RFC2136DefaultPropagationTimeout = 60 * time.Second
	RFC2136DefaultPollingInterval    = 2 * time.Second
	RFC2136MinPort                   = 1
	RFC2136MaxPort                   = 65535
)

// TSIG algorithm constants.
const (
	TSIGAlgorithmHMACSHA256 = "hmac-sha256"
	TSIGAlgorithmHMACSHA384 = "hmac-sha384"
	TSIGAlgorithmHMACSHA512 = "hmac-sha512"
	TSIGAlgorithmHMACSHA1   = "hmac-sha1"
	TSIGAlgorithmHMACMD5    = "hmac-md5"
)

// ValidTSIGAlgorithms contains all supported TSIG algorithms.
var ValidTSIGAlgorithms = map[string]bool{
	TSIGAlgorithmHMACSHA256: true,
	TSIGAlgorithmHMACSHA384: true,
	TSIGAlgorithmHMACSHA512: true,
	TSIGAlgorithmHMACSHA1:   true,
	TSIGAlgorithmHMACMD5:    true,
}

// RFC2136Provider implements the ProviderPlugin interface for RFC 2136 Dynamic DNS Updates.
// RFC 2136 is supported by BIND, PowerDNS, Knot DNS, and many self-hosted DNS servers.
type RFC2136Provider struct {
	propagationTimeout time.Duration
	pollingInterval    time.Duration
}

// NewRFC2136Provider creates a new RFC2136Provider with default settings.
func NewRFC2136Provider() *RFC2136Provider {
	return &RFC2136Provider{
		propagationTimeout: RFC2136DefaultPropagationTimeout,
		pollingInterval:    RFC2136DefaultPollingInterval,
	}
}

// Type returns the unique provider type identifier.
func (p *RFC2136Provider) Type() string {
	return "rfc2136"
}

// Metadata returns descriptive information about the provider.
func (p *RFC2136Provider) Metadata() dnsprovider.ProviderMetadata {
	return dnsprovider.ProviderMetadata{
		Type:             "rfc2136",
		Name:             "RFC 2136 (Dynamic DNS)",
		Description:      "Dynamic DNS Updates using RFC 2136 protocol with TSIG authentication. Compatible with BIND, PowerDNS, and Knot DNS.",
		DocumentationURL: "https://charon.dev/docs/features/rfc2136-dns",
		IsBuiltIn:        false,
		Version:          "1.0.0",
		InterfaceVersion: dnsprovider.InterfaceVersion,
	}
}

// Init is called after the plugin is registered.
func (p *RFC2136Provider) Init() error {
	return nil
}

// Cleanup is called before the plugin is unregistered.
func (p *RFC2136Provider) Cleanup() error {
	return nil
}

// RequiredCredentialFields returns credential fields that must be provided.
func (p *RFC2136Provider) RequiredCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "nameserver",
			Label:       "DNS Server",
			Type:        "text",
			Placeholder: "ns1.example.com",
			Hint:        "Hostname or IP address of the DNS server accepting dynamic updates",
		},
		{
			Name:        "tsig_key_name",
			Label:       "TSIG Key Name",
			Type:        "text",
			Placeholder: "acme-update-key.example.com",
			Hint:        "The name of the TSIG key configured on your DNS server",
		},
		{
			Name:        "tsig_key_secret",
			Label:       "TSIG Key Secret",
			Type:        "password",
			Placeholder: "",
			Hint:        "Base64-encoded TSIG secret (from tsig-keygen or dnssec-keygen)",
		},
	}
}

// OptionalCredentialFields returns credential fields that may be provided.
func (p *RFC2136Provider) OptionalCredentialFields() []dnsprovider.CredentialFieldSpec {
	return []dnsprovider.CredentialFieldSpec{
		{
			Name:        "port",
			Label:       "Port",
			Type:        "text",
			Placeholder: RFC2136DefaultPort,
			Hint:        fmt.Sprintf("DNS server port (default: %s)", RFC2136DefaultPort),
		},
		{
			Name:        "tsig_algorithm",
			Label:       "TSIG Algorithm",
			Type:        "select",
			Placeholder: "",
			Hint:        "HMAC algorithm for TSIG authentication (hmac-sha256 recommended)",
			Options: []dnsprovider.SelectOption{
				{Value: TSIGAlgorithmHMACSHA256, Label: "HMAC-SHA256 (Recommended)"},
				{Value: TSIGAlgorithmHMACSHA384, Label: "HMAC-SHA384"},
				{Value: TSIGAlgorithmHMACSHA512, Label: "HMAC-SHA512"},
				{Value: TSIGAlgorithmHMACSHA1, Label: "HMAC-SHA1 (Legacy)"},
				{Value: TSIGAlgorithmHMACMD5, Label: "HMAC-MD5 (Deprecated)"},
			},
		},
		{
			Name:        "zone",
			Label:       "Zone",
			Type:        "text",
			Placeholder: "example.com",
			Hint:        "DNS zone to update (auto-detected from domain if empty)",
		},
	}
}

// ValidateCredentials checks if the provided credentials are valid.
func (p *RFC2136Provider) ValidateCredentials(creds map[string]string) error {
	// Validate required fields
	nameserver := strings.TrimSpace(creds["nameserver"])
	if nameserver == "" {
		return fmt.Errorf("nameserver is required")
	}

	tsigKeyName := strings.TrimSpace(creds["tsig_key_name"])
	if tsigKeyName == "" {
		return fmt.Errorf("tsig_key_name is required")
	}

	tsigKeySecret := strings.TrimSpace(creds["tsig_key_secret"])
	if tsigKeySecret == "" {
		return fmt.Errorf("tsig_key_secret is required")
	}

	// Validate base64 encoding of TSIG secret
	if _, err := base64.StdEncoding.DecodeString(tsigKeySecret); err != nil {
		return fmt.Errorf("tsig_key_secret must be valid base64: %w", err)
	}

	// Validate port if provided
	if portStr := strings.TrimSpace(creds["port"]); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("port must be a number: %w", err)
		}
		if port < RFC2136MinPort || port > RFC2136MaxPort {
			return fmt.Errorf("port must be between %d and %d", RFC2136MinPort, RFC2136MaxPort)
		}
	}

	// Validate algorithm if provided
	if algorithm := strings.TrimSpace(creds["tsig_algorithm"]); algorithm != "" {
		algorithm = strings.ToLower(algorithm)
		if !ValidTSIGAlgorithms[algorithm] {
			validAlgorithms := make([]string, 0, len(ValidTSIGAlgorithms))
			for alg := range ValidTSIGAlgorithms {
				validAlgorithms = append(validAlgorithms, alg)
			}
			return fmt.Errorf("tsig_algorithm must be one of: %s", strings.Join(validAlgorithms, ", "))
		}
	}

	return nil
}

// TestCredentials attempts to verify credentials work.
// For RFC 2136, we validate the format but cannot test without making actual DNS queries.
func (p *RFC2136Provider) TestCredentials(creds map[string]string) error {
	return p.ValidateCredentials(creds)
}

// SupportsMultiCredential indicates if the provider can handle zone-specific credentials.
func (p *RFC2136Provider) SupportsMultiCredential() bool {
	return true
}

// BuildCaddyConfig constructs the Caddy DNS challenge configuration.
func (p *RFC2136Provider) BuildCaddyConfig(creds map[string]string) map[string]any {
	config := map[string]any{
		"name":            "rfc2136",
		"nameserver":      strings.TrimSpace(creds["nameserver"]),
		"tsig_key_name":   strings.TrimSpace(creds["tsig_key_name"]),
		"tsig_key_secret": strings.TrimSpace(creds["tsig_key_secret"]),
	}

	// Add port with default
	port := strings.TrimSpace(creds["port"])
	if port == "" {
		port = RFC2136DefaultPort
	}
	config["port"] = port

	// Add algorithm with default
	algorithm := strings.TrimSpace(creds["tsig_algorithm"])
	if algorithm == "" {
		algorithm = RFC2136DefaultAlgorithm
	}
	config["tsig_algorithm"] = strings.ToLower(algorithm)

	// Add zone if specified (optional - Caddy can auto-detect)
	if zone := strings.TrimSpace(creds["zone"]); zone != "" {
		config["zone"] = zone
	}

	return config
}

// BuildCaddyConfigForZone constructs config for a specific zone.
func (p *RFC2136Provider) BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any {
	config := p.BuildCaddyConfig(creds)
	// If zone is not explicitly set, use the base domain
	if _, hasZone := config["zone"]; !hasZone {
		config["zone"] = baseDomain
	}
	return config
}

// PropagationTimeout returns the recommended DNS propagation wait time.
func (p *RFC2136Provider) PropagationTimeout() time.Duration {
	return p.propagationTimeout
}

// PollingInterval returns the recommended polling interval for DNS verification.
func (p *RFC2136Provider) PollingInterval() time.Duration {
	return p.pollingInterval
}

// GetPort returns the configured port or the default.
func (p *RFC2136Provider) GetPort(creds map[string]string) string {
	if port := strings.TrimSpace(creds["port"]); port != "" {
		return port
	}
	return RFC2136DefaultPort
}

// GetAlgorithm returns the configured algorithm or the default.
func (p *RFC2136Provider) GetAlgorithm(creds map[string]string) string {
	if algorithm := strings.TrimSpace(creds["tsig_algorithm"]); algorithm != "" {
		return strings.ToLower(algorithm)
	}
	return RFC2136DefaultAlgorithm
}
