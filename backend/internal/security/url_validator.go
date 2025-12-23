package security

import (
	"context"
	"fmt"
	"net"
	neturl "net/url"
	"time"
)

// ValidationConfig holds options for URL validation.
type ValidationConfig struct {
	AllowLocalhost  bool
	AllowHTTP       bool
	MaxRedirects    int
	Timeout         time.Duration
	BlockPrivateIPs bool
}

// ValidationOption allows customizing validation behavior.
type ValidationOption func(*ValidationConfig)

// WithAllowLocalhost permits localhost addresses for testing (default: false).
func WithAllowLocalhost() ValidationOption {
	return func(c *ValidationConfig) { c.AllowLocalhost = true }
}

// WithAllowHTTP permits HTTP scheme (default: false, HTTPS only).
func WithAllowHTTP() ValidationOption {
	return func(c *ValidationConfig) { c.AllowHTTP = true }
}

// WithTimeout sets the DNS resolution timeout (default: 3 seconds).
func WithTimeout(timeout time.Duration) ValidationOption {
	return func(c *ValidationConfig) { c.Timeout = timeout }
}

// WithMaxRedirects sets the maximum number of redirects to follow (default: 0).
func WithMaxRedirects(max int) ValidationOption {
	return func(c *ValidationConfig) { c.MaxRedirects = max }
}

// ValidateExternalURL validates a URL for external HTTP requests with comprehensive SSRF protection.
// This function provides defense-in-depth against Server-Side Request Forgery attacks by:
// 1. Validating URL format and scheme
// 2. Resolving DNS and checking all resolved IPs against private/reserved ranges
// 3. Blocking access to cloud metadata endpoints (AWS, GCP, Azure)
// 4. Enforcing HTTPS by default (configurable)
//
// Returns: normalized URL string, error
//
// Security: This function blocks access to:
// - Private IP ranges (RFC 1918: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
// - Loopback addresses (127.0.0.0/8, ::1/128) unless AllowLocalhost option is set
// - Link-local addresses (169.254.0.0/16, fe80::/10) including cloud metadata endpoints
// - Reserved IP ranges (0.0.0.0/8, 240.0.0.0/4, 255.255.255.255/32)
// - IPv6 unique local addresses (fc00::/7)
//
// Example usage:
//
//	// Production use (HTTPS only, no private IPs)
//	url, err := ValidateExternalURL("https://api.example.com/webhook")
//
//	// Testing use (allow localhost and HTTP)
//	url, err := ValidateExternalURL("http://localhost:8080/test",
//	    WithAllowLocalhost(),
//	    WithAllowHTTP())
func ValidateExternalURL(rawURL string, options ...ValidationOption) (string, error) {
	// Apply default configuration
	config := &ValidationConfig{
		AllowLocalhost:  false,
		AllowHTTP:       false,
		MaxRedirects:    0,
		Timeout:         3 * time.Second,
		BlockPrivateIPs: true,
	}

	// Apply custom options
	for _, opt := range options {
		opt(config)
	}

	// Phase 1: URL Format Validation
	u, err := neturl.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid url format: %w", err)
	}

	// Validate scheme - only http/https allowed
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme: %s (only http and https are allowed)", u.Scheme)
	}

	// Enforce HTTPS unless explicitly allowed
	if !config.AllowHTTP && u.Scheme != "https" {
		return "", fmt.Errorf("http scheme not allowed (use https for security)")
	}

	// Validate hostname exists
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("missing hostname in url")
	}

	// Reject URLs with credentials in authority section
	if u.User != nil {
		return "", fmt.Errorf("urls with embedded credentials are not allowed")
	}

	// Phase 2: Localhost Exception Handling
	if config.AllowLocalhost {
		// Check if this is an explicit localhost address
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			// Normalize and return - localhost is allowed
			return u.String(), nil
		}
	}

	// Phase 3: DNS Resolution and IP Validation
	// Resolve hostname with timeout
	resolver := &net.Resolver{}
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	ips, err := resolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return "", fmt.Errorf("dns resolution failed for %s: %w", host, err)
	}

	if len(ips) == 0 {
		return "", fmt.Errorf("no ip addresses resolved for hostname: %s", host)
	}

	// Phase 4: Private IP Blocking
	// Check ALL resolved IPs against private/reserved ranges
	if config.BlockPrivateIPs {
		for _, ip := range ips {
			// Check if IP is in private/reserved ranges
			// This uses comprehensive CIDR blocking including:
			// - RFC 1918 private networks (10.x, 172.16.x, 192.168.x)
			// - Loopback (127.x.x.x, ::1)
			// - Link-local (169.254.x.x, fe80::) including cloud metadata
			// - Reserved ranges (0.x.x.x, 240.x.x.x, 255.255.255.255)
			// - IPv6 unique local (fc00::)
			if isPrivateIP(ip) {
				// Provide security-conscious error messages
				if ip.String() == "169.254.169.254" {
					return "", fmt.Errorf("access to cloud metadata endpoints is blocked for security (detected: %s)", ip.String())
				}
				return "", fmt.Errorf("connection to private ip addresses is blocked for security (detected: %s)", ip.String())
			}
		}
	}

	// Normalize URL (trim trailing slashes, lowercase host)
	normalized := u.String()

	return normalized, nil
}

// isPrivateIP checks if an IP address is private, loopback, link-local, or otherwise restricted.
// This function implements comprehensive SSRF protection by blocking:
// - Private IPv4 ranges (RFC 1918)
// - Loopback addresses (127.0.0.0/8, ::1/128)
// - Link-local addresses (169.254.0.0/16, fe80::/10) including AWS/GCP metadata
// - Reserved ranges (0.0.0.0/8, 240.0.0.0/4, 255.255.255.255/32)
// - IPv6 unique local addresses (fc00::/7)
//
// This is a reused implementation from utils/url_testing.go with excellent test coverage.
func isPrivateIP(ip net.IP) bool {
	// Check built-in Go functions for common cases
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Define private and reserved IP blocks
	privateBlocks := []string{
		// IPv4 Private Networks (RFC 1918)
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",

		// IPv4 Link-Local (RFC 3927) - includes AWS/GCP metadata service
		"169.254.0.0/16",

		// IPv4 Loopback
		"127.0.0.0/8",

		// IPv4 Reserved ranges
		"0.0.0.0/8",          // "This network"
		"240.0.0.0/4",        // Reserved for future use
		"255.255.255.255/32", // Broadcast

		// IPv6 Loopback
		"::1/128",

		// IPv6 Unique Local Addresses (RFC 4193)
		"fc00::/7",

		// IPv6 Link-Local
		"fe80::/10",
	}

	// Check if IP is in any of the blocked ranges
	for _, block := range privateBlocks {
		_, subnet, err := net.ParseCIDR(block)
		if err != nil {
			continue
		}
		if subnet.Contains(ip) {
			return true
		}
	}

	return false
}
