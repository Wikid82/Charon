package utils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// TestURLConnectivity performs a server-side connectivity test with SSRF protection.
// For testing purposes, an optional http.RoundTripper can be provided to bypass
// DNS resolution and network calls.
// Returns:
// - reachable: true if URL returned 2xx-3xx status
// - latency: round-trip time in milliseconds
// - error: validation or connectivity error
func TestURLConnectivity(rawURL string, transport ...http.RoundTripper) (bool, float64, error) {
	// Parse URL
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false, 0, fmt.Errorf("invalid URL: %w", err)
	}

	// Create HTTP client with optional custom transport
	var client *http.Client
	if len(transport) > 0 && transport[0] != nil {
		// Use provided transport (for testing)
		client = &http.Client{
			Timeout:   5 * time.Second,
			Transport: transport[0],
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 2 {
					return fmt.Errorf("too many redirects (max 2)")
				}
				return nil
			},
		}
	} else {
		// Production path: SSRF protection with DNS resolution
		host := parsed.Hostname()
		port := parsed.Port()
		if port == "" {
			port = map[string]string{"https": "443", "http": "80"}[parsed.Scheme]
		}

		// DNS resolution with timeout (SSRF protection step 1)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return false, 0, fmt.Errorf("DNS resolution failed: %w", err)
		}

		if len(ips) == 0 {
			return false, 0, fmt.Errorf("no IP addresses found for host")
		}

		// SSRF protection: block private/internal IPs
		for _, ip := range ips {
			if isPrivateIP(ip.IP) {
				return false, 0, fmt.Errorf("access to private IP addresses is blocked (resolved to %s)", ip.IP)
			}
		}

		client = &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 2 {
					return fmt.Errorf("too many redirects (max 2)")
				}
				return nil
			},
		}
	}

	// Perform HTTP HEAD request with strict timeout
	ctx := context.Background()
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return false, 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Add custom User-Agent header
	req.Header.Set("User-Agent", "Charon-Health-Check/1.0")

	resp, err := client.Do(req)
	latency := time.Since(start).Seconds() * 1000 // Convert to milliseconds

	if err != nil {
		return false, latency, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	// Accept 2xx and 3xx status codes as "reachable"
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return true, latency, nil
	}

	return false, latency, fmt.Errorf("server returned status %d", resp.StatusCode)
}

// isPrivateIP checks if an IP address is private, loopback, link-local, or otherwise restricted.
// This function implements SSRF protection by blocking:
// - Private IPv4 ranges (RFC 1918)
// - Loopback addresses (127.0.0.0/8, ::1/128)
// - Link-local addresses (169.254.0.0/16, fe80::/10)
// - Private IPv6 ranges (fc00::/7)
// - Reserved ranges (0.0.0.0/8, 240.0.0.0/4, 255.255.255.255/32)
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
