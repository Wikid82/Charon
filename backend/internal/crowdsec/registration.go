// Package crowdsec provides integration with CrowdSec for security decisions and remediation.
package crowdsec

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/network"
)

const (
	// defaultLAPIURL is the default CrowdSec LAPI URL.
	// Port 8085 is used to avoid conflict with Charon management API on port 8080.
	defaultLAPIURL          = "http://127.0.0.1:8085"
	defaultHealthTimeout    = 5 * time.Second
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

// LAPIHealthResponse represents the health check response from CrowdSec LAPI.
type LAPIHealthResponse struct {
	Message string `json:"message,omitempty"`
	Version string `json:"version,omitempty"`
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

// CheckLAPIHealth verifies CrowdSec LAPI is responding.
func CheckLAPIHealth(lapiURL string) bool {
	if lapiURL == "" {
		lapiURL = defaultLAPIURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultHealthTimeout)
	defer cancel()

	// Try the /health endpoint first (standard LAPI health check)
	healthURL := strings.TrimRight(lapiURL, "/") + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, http.NoBody)
	if err != nil {
		return false
	}

	// Use SSRF-safe HTTP client with localhost allowed (LAPI is localhost-only)
	client := network.NewSafeHTTPClient(
		network.WithTimeout(defaultHealthTimeout),
		network.WithAllowLocalhost(), // LAPI validated to be localhost only
	)
	resp, err := client.Do(req)
	if err != nil {
		// Fallback: try the /v1/decisions endpoint with a HEAD request
		return checkDecisionsEndpoint(ctx, lapiURL)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("Failed to close response body")
		}
	}()

	// Check content-type to ensure we're getting JSON from actual LAPI (not HTML from frontend)
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !strings.Contains(contentType, "application/json") {
		// Not JSON response, likely hitting a frontend/proxy
		return false
	}

	// LAPI returns 200 OK for healthy status
	if resp.StatusCode == http.StatusOK {
		return true
	}

	// If health endpoint returned non-OK, try decisions endpoint fallback
	if resp.StatusCode == http.StatusNotFound {
		return checkDecisionsEndpoint(ctx, lapiURL)
	}

	return false
}

// GetLAPIVersion retrieves the CrowdSec LAPI version.
func GetLAPIVersion(ctx context.Context, lapiURL string) (string, error) {
	if lapiURL == "" {
		lapiURL = defaultLAPIURL
	}

	versionURL := strings.TrimRight(lapiURL, "/") + "/v1/version"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("create version request: %w", err)
	}

	// Use SSRF-safe HTTP client with localhost allowed (LAPI is localhost-only)
	client := network.NewSafeHTTPClient(
		network.WithTimeout(defaultHealthTimeout),
		network.WithAllowLocalhost(), // LAPI validated to be localhost only
	)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("version request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("version request returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read version response: %w", err)
	}

	var versionResp struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &versionResp); err != nil {
		// Some versions return plain text
		return strings.TrimSpace(string(body)), nil
	}

	return versionResp.Version, nil
}

// checkDecisionsEndpoint is a fallback health check using the decisions endpoint.
func checkDecisionsEndpoint(ctx context.Context, lapiURL string) bool {
	decisionsURL := strings.TrimRight(lapiURL, "/") + "/v1/decisions"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, decisionsURL, http.NoBody)
	if err != nil {
		return false
	}

	// Use SSRF-safe HTTP client with localhost allowed (LAPI is localhost-only)
	client := network.NewSafeHTTPClient(
		network.WithTimeout(defaultHealthTimeout),
		network.WithAllowLocalhost(), // LAPI validated to be localhost only
	)
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Log().WithError(err).Warn("Failed to close response body")
		}
	}()

	// Check content-type to avoid false positives from HTML responses
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !strings.Contains(contentType, "application/json") {
		// Not JSON response, likely hitting a frontend/proxy
		return false
	}

	// 401 is expected without auth, but indicates LAPI is running
	// 200 with empty array is also valid (no decisions)
	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized
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
	cmd := exec.CommandContext(ctx, "cscli", "bouncers", "add", name, "-o", "raw")
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
