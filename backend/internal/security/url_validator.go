package security

import (
	"context"
	"fmt"
	"net"
	neturl "net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/network"
)

// InternalServiceHostAllowlistEnvVar controls which *additional* hostnames (exact matches)
// are permitted for internal service HTTP calls (CrowdSec LAPI, Caddy Admin, etc.).
//
// Default policy remains localhost-only.
// Example: CHARON_SSRF_INTERNAL_HOST_ALLOWLIST="crowdsec,caddy"
const InternalServiceHostAllowlistEnvVar = "CHARON_SSRF_INTERNAL_HOST_ALLOWLIST"

// ParseExactHostnameAllowlist parses a comma-separated list of hostnames into an exact-match set.
//
// Notes:
// - Hostnames are lowercased for comparison.
// - Entries containing schemes/paths are ignored.
func ParseExactHostnameAllowlist(csv string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, part := range strings.Split(csv, ",") {
		h := strings.ToLower(strings.TrimSpace(part))
		if h == "" {
			continue
		}
		// Reject obvious non-hostname inputs.
		if strings.Contains(h, "://") || strings.ContainsAny(h, "/@") {
			continue
		}
		out[h] = struct{}{}
	}
	return out
}

// InternalServiceHostAllowlist returns the deny-by-default internal-service hostname allowlist.
//
// Defaults: localhost-only. Docker/service-name deployments must opt-in via
// CHARON_SSRF_INTERNAL_HOST_ALLOWLIST.
func InternalServiceHostAllowlist() map[string]struct{} {
	allow := map[string]struct{}{
		"localhost": {},
		"127.0.0.1": {},
		"::1":       {},
	}
	extra := ParseExactHostnameAllowlist(os.Getenv(InternalServiceHostAllowlistEnvVar))
	for h := range extra {
		allow[h] = struct{}{}
	}
	return allow
}

// ValidateInternalServiceBaseURL validates a configured base URL for an internal service.
//
// Security model:
// - host must be an exact match in allowedHosts
// - port must match expectedPort (including default ports if omitted)
// - proxy env vars must be ignored by callers (client/transport responsibility)
//
// Returns a normalized base URL (scheme://host:expectedPort) suitable for safe request construction.
func ValidateInternalServiceBaseURL(rawURL string, expectedPort int, allowedHosts map[string]struct{}) (*neturl.URL, error) {
	u, err := neturl.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url format: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme: %s (only http and https are allowed)", u.Scheme)
	}
	if u.User != nil {
		return nil, fmt.Errorf("urls with embedded credentials are not allowed")
	}

	host := strings.ToLower(u.Hostname())
	if host == "" {
		return nil, fmt.Errorf("missing hostname in url")
	}
	if _, ok := allowedHosts[host]; !ok {
		return nil, fmt.Errorf("hostname not allowed: %s", host)
	}

	actualPort := 0
	if p := u.Port(); p != "" {
		portNum, perr := strconv.Atoi(p)
		if perr != nil || portNum < 1 || portNum > 65535 {
			return nil, fmt.Errorf("invalid port")
		}
		actualPort = portNum
	} else {
		if u.Scheme == "https" {
			actualPort = 443
		} else {
			actualPort = 80
		}
	}
	if actualPort != expectedPort {
		return nil, fmt.Errorf("unexpected port: %d (expected %d)", actualPort, expectedPort)
	}

	// Normalize to a base URL with an explicit expected port.
	base := &neturl.URL{
		Scheme: u.Scheme,
		Host:   net.JoinHostPort(u.Hostname(), strconv.Itoa(expectedPort)),
	}
	return base, nil
}

// ValidationConfig holds options for URL validation.
type ValidationConfig struct {
	AllowLocalhost  bool
	AllowHTTP       bool
	MaxRedirects    int
	Timeout         time.Duration
	BlockPrivateIPs bool

	// AllowRFC1918 permits addresses in the RFC 1918 private ranges
	// (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16).
	//
	// SECURITY NOTE: Must only be set for admin-configured features such as uptime
	// monitors. Link-local (169.254.x.x), loopback, cloud metadata, and all other
	// restricted ranges remain blocked regardless of this flag.
	AllowRFC1918 bool
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
func WithMaxRedirects(maxRedirects int) ValidationOption {
	return func(c *ValidationConfig) { c.MaxRedirects = maxRedirects }
}

// WithAllowRFC1918 permits addresses in the RFC 1918 private ranges
// (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16).
//
// Use only for admin-configured features (e.g., uptime monitors targeting internal hosts).
// All other SSRF protections remain active.
func WithAllowRFC1918() ValidationOption {
	return func(c *ValidationConfig) { c.AllowRFC1918 = true }
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

	// ENHANCEMENT: Hostname Length Validation (RFC 1035)
	const maxHostnameLength = 253
	if len(host) > maxHostnameLength {
		return "", fmt.Errorf("hostname exceeds maximum length of %d characters", maxHostnameLength)
	}

	// ENHANCEMENT: Suspicious Pattern Detection
	if strings.Contains(host, "..") {
		return "", fmt.Errorf("hostname contains suspicious pattern (..)")
	}

	// Reject URLs with credentials in authority section
	if u.User != nil {
		return "", fmt.Errorf("urls with embedded credentials are not allowed")
	}

	// ENHANCEMENT: Port Range Validation
	if port := u.Port(); port != "" {
		portNum, parseErr := parsePort(port)
		if parseErr != nil {
			return "", fmt.Errorf("invalid port: %w", parseErr)
		}
		if portNum < 1 || portNum > 65535 {
			return "", fmt.Errorf("port out of range: %d", portNum)
		}
		// CRITICAL FIX: Allow standard ports 80/443, block other privileged ports
		standardPorts := map[int]bool{80: true, 443: true}
		if portNum < 1024 && !standardPorts[portNum] && !config.AllowLocalhost {
			return "", fmt.Errorf("non-standard privileged port blocked: %d", portNum)
		}
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
			// ENHANCEMENT: IPv4-mapped IPv6 Detection
			// Prevent bypass via ::ffff:192.168.1.1 format
			if ip.To4() != nil && ip.To16() != nil && isIPv4MappedIPv6(ip) {
				// Extract the IPv4 address from the mapped format
				ipv4 := ip.To4()
				// Allow RFC 1918 IPv4-mapped IPv6 only when the caller has explicitly opted in.
				if config.AllowRFC1918 && network.IsRFC1918(ipv4) {
					continue
				}
				if network.IsPrivateIP(ipv4) {
					// Cloud metadata endpoint must produce the specific error even
					// when the address arrives as an IPv4-mapped IPv6 value.
					if ipv4.String() == "169.254.169.254" {
						return "", fmt.Errorf("access to cloud metadata endpoints is blocked for security (detected: %s)", sanitizeIPForError(ipv4.String()))
					}
					return "", fmt.Errorf("connection to private ip addresses is blocked for security (detected: %s)", sanitizeIPForError(ipv4.String()))
				}
			}

			// Allow RFC 1918 addresses only when the caller has explicitly opted in
			// (e.g., admin-configured uptime monitors targeting internal hosts).
			// Link-local (169.254.x.x), loopback, cloud metadata, and all other
			// restricted ranges remain blocked regardless of this flag.
			if config.AllowRFC1918 && network.IsRFC1918(ip) {
				continue
			}

			// Check if IP is in private/reserved ranges using centralized network.IsPrivateIP
			// This includes:
			// - RFC 1918 private networks (10.x, 172.16.x, 192.168.x)
			// - Loopback (127.x.x.x, ::1)
			// - Link-local (169.254.x.x, fe80::) including cloud metadata
			// - Reserved ranges (0.x.x.x, 240.x.x.x, 255.255.255.255)
			// - IPv6 unique local (fc00::)
			if network.IsPrivateIP(ip) {
				// ENHANCEMENT: Sanitize Error Messages
				// Don't leak internal IPs in error messages to external users
				sanitizedIP := sanitizeIPForError(ip.String())
				if ip.String() == "169.254.169.254" {
					return "", fmt.Errorf("access to cloud metadata endpoints is blocked for security (detected: %s)", sanitizedIP)
				}
				return "", fmt.Errorf("connection to private ip addresses is blocked for security (detected: %s)", sanitizedIP)
			}
		}
	}

	// Normalize URL (trim trailing slashes, lowercase host)
	normalized := u.String()

	return normalized, nil
}

// isIPv4MappedIPv6 detects IPv4-mapped IPv6 addresses (::ffff:192.168.1.1).
// This prevents SSRF bypass via IPv6 notation of private IPv4 addresses.
func isIPv4MappedIPv6(ip net.IP) bool {
	// IPv4-mapped IPv6 addresses have the form ::ffff:a.b.c.d
	// In binary: 80 bits of zeros, 16 bits of ones, 32 bits of IPv4
	if len(ip) != net.IPv6len {
		return false
	}
	// Check for ::ffff: prefix (10 zero bytes, 2 0xff bytes)
	for i := 0; i < 10; i++ {
		if ip[i] != 0 {
			return false
		}
	}
	return ip[10] == 0xff && ip[11] == 0xff
}

// parsePort safely parses a port string to an integer.
func parsePort(port string) (int, error) {
	if port == "" {
		return 0, fmt.Errorf("empty port string")
	}
	var portNum int
	_, err := fmt.Sscanf(port, "%d", &portNum)
	if err != nil {
		return 0, fmt.Errorf("port must be numeric: %s", port)
	}
	return portNum, nil
}

// sanitizeIPForError removes sensitive details from IP addresses in error messages.
// This prevents leaking internal network topology to external users.
func sanitizeIPForError(ip string) string {
	// For private IPs, show only the first octet to avoid leaking network structure
	// Example: 192.168.1.100 -> 192.x.x.x
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return "invalid-ip"
	}

	if parsedIP.To4() != nil {
		// IPv4: show only first octet
		parts := strings.Split(ip, ".")
		if len(parts) == 4 {
			return parts[0] + ".x.x.x"
		}
	} else {
		// IPv6: show only first segment
		parts := strings.Split(ip, ":")
		if len(parts) > 0 {
			return parts[0] + "::"
		}
	}
	return "private-ip"
}
