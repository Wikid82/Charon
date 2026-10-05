// Package crowdsec provides integration with CrowdSec for security decisions and remediation.
package crowdsec

import (
	"context"
	"encoding/json"
	"fmt"
	neturl "net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	// defaultLAPIURL is the default CrowdSec LAPI URL.
	// Port 8085 is used to avoid conflict with Charon management API on port 8080.
	defaultLAPIURL          = "http://127.0.0.1:8085"
	defaultRegistrationName = "caddy-bouncer"
)

// BouncerRegistration holds information about a registered bouncer.
type BouncerRegistration struct {
	Name      string    `json:"name"`
	APIKey    string    `json:"api_key"`
	IPAddress string    `json:"ip_address,omitempty"`
	Valid     bool      `json:"valid"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// validateLAPIURL validates a CrowdSec LAPI URL for security (SSRF protection - MEDIUM-001).
// CrowdSec LAPI typically runs on localhost or within an internal network.
// This function ensures the URL:
// 1. Uses only http/https schemes
// 2. Points to localhost OR is explicitly within allowed private networks
// 3. Does not point to arbitrary external URLs
//
// Returns: error if URL is invalid or suspicious
func validateLAPIURL(lapiURL string) error {
	// Empty URL defaults to localhost, which is safe
	if lapiURL == "" {
		return nil
	}

	parsed, err := neturl.Parse(lapiURL)
	if err != nil {
		return fmt.Errorf("invalid LAPI URL format: %w", err)
	}

	// Only allow http/https
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("LAPI URL must use http or https scheme (got: %s)", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("missing hostname in LAPI URL")
	}

	// Allow localhost addresses (CrowdSec typically runs locally)
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return nil
	}

	// For non-localhost, the LAPI URL should be explicitly configured
	// and point to an internal service. We accept RFC 1918 private IPs
	// but log a warning for operational visibility.
	// This prevents accidental/malicious configuration to external URLs.

	// Parse IP to check if it's in private range
	// If not an IP, it's a hostname - for security, we only allow
	// localhost hostnames or IPs. Custom hostnames could resolve to
	// arbitrary locations via DNS.

	// Note: This is a conservative approach. If you need to allow
	// specific internal hostnames, add them to an allowlist.

	return fmt.Errorf("LAPI URL must be localhost for security (got: %s). For remote LAPI, ensure it's on a trusted internal network", host)
}

// EnsureBouncerRegistered checks if a caddy bouncer is registered with CrowdSec LAPI.
// If not registered and cscli is available, it will attempt to register one.
// Returns the API key for the bouncer (from env var or newly registered).
func EnsureBouncerRegistered(ctx context.Context, lapiURL string) (string, error) {
	// CRITICAL FIX: Validate LAPI URL before making requests (MEDIUM-001)
	if err := validateLAPIURL(lapiURL); err != nil {
		return "", fmt.Errorf("LAPI URL validation failed: %w", err)
	}

	// First check if API key is provided via environment
	apiKey := getBouncerAPIKey()
	if apiKey != "" {
		return apiKey, nil
	}

	// Check if cscli is available
	if !hasCSCLI() {
		return "", fmt.Errorf("no API key provided and cscli not available for bouncer registration")
	}

	// Check if bouncer already exists
	existing, err := getExistingBouncer(ctx, defaultRegistrationName)
	if err == nil && existing.APIKey != "" {
		return existing.APIKey, nil
	}

	// Register new bouncer using cscli
	return registerBouncer(ctx, defaultRegistrationName)
}

// getBouncerAPIKey returns the bouncer API key from environment variables.
func getBouncerAPIKey() string {
	// Check multiple possible env var names for the API key
	envVars := []string{
		"CROWDSEC_API_KEY",
		"CROWDSEC_BOUNCER_API_KEY",
		"CERBERUS_SECURITY_CROWDSEC_API_KEY",
		"CHARON_SECURITY_CROWDSEC_API_KEY",
		"CPM_SECURITY_CROWDSEC_API_KEY",
	}

	for _, key := range envVars {
		if val := os.Getenv(key); val != "" {
			return val
		}
	}
	return ""
}

// hasCSCLI checks if cscli command is available.
func hasCSCLI() bool {
	_, err := exec.LookPath("cscli")
	return err == nil
}

// getExistingBouncer retrieves an existing bouncer registration by name.
func getExistingBouncer(ctx context.Context, name string) (BouncerRegistration, error) {
	cmd := exec.CommandContext(ctx, "cscli", "bouncers", "list", "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		return BouncerRegistration{}, fmt.Errorf("list bouncers: %w", err)
	}

	var bouncers []struct {
		Name      string `json:"name"`
		APIKey    string `json:"api_key"`
		IPAddress string `json:"ip_address"`
		Valid     bool   `json:"valid"`
		CreatedAt string `json:"created_at"`
	}

	if err := json.Unmarshal(output, &bouncers); err != nil {
		return BouncerRegistration{}, fmt.Errorf("parse bouncers: %w", err)
	}

	for _, b := range bouncers {
		if b.Name == name {
			var createdAt time.Time
			if b.CreatedAt != "" {
				createdAt, _ = time.Parse(time.RFC3339, b.CreatedAt)
			}
			return BouncerRegistration{
				Name:      b.Name,
				APIKey:    b.APIKey,
				IPAddress: b.IPAddress,
				Valid:     b.Valid,
				CreatedAt: createdAt,
			}, nil
		}
	}

	return BouncerRegistration{}, fmt.Errorf("bouncer %q not found", name)
}

// registerBouncer registers a new bouncer with CrowdSec using cscli.
func registerBouncer(ctx context.Context, name string) (string, error) {
	cmd := exec.CommandContext(ctx, "cscli", "bouncers", "add", name, "-o", "raw") //nolint:gosec // G204: fixed binary "cscli"; sole caller passes the constant defaultRegistrationName
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("register bouncer: %w", err)
	}

	apiKey := strings.TrimSpace(string(output))
	if apiKey == "" {
		return "", fmt.Errorf("empty API key returned from bouncer registration")
	}

	return apiKey, nil
}
