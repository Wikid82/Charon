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

	// AllowCGNAT permits the shared address space used by overlay networks.
	// The cloud metadata alias inside it stays blocked.
	AllowCGNAT bool
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

// WithAllowCGNAT permits the shared address space used by overlay networks.
// The cloud metadata alias inside it stays blocked.
func WithAllowCGNAT() ValidationOption {
	return func(c *ValidationConfig) { c.AllowCGNAT = true }
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
		AllowCGNAT:      true,
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

	// Phase 4: Address Policy
	// Check ALL resolved IPs against the shared address policy. AllowLocalhost
	// is intentionally not part of it: it only short-circuits the literal
	// localhost hosts in Phase 2.
	if config.BlockPrivateIPs {
		policy := network.AddressPolicy{
			AllowRFC1918:    config.AllowRFC1918,
			AllowCGNAT:      config.AllowCGNAT,
			AllowTransition: true,
		}
		for _, ip := range ips {
			if policy.Blocked(ip) {
				return "", blockedIPError(ip)
			}
		}
	}

	// Normalize URL (trim trailing slashes, lowercase host)
	normalized := u.String()

	return normalized, nil
}

// blockedIPError builds the rejection error for an address the policy refused.
// The cloud metadata endpoint, including its IPv4-mapped form, gets a specific
// message; the address shown is always sanitized.
func blockedIPError(ip net.IP) error {
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	sanitized := sanitizeIPForError(ip.String())
	if ip.String() == "169.254.169.254" {
		return blockedAddressError("access to cloud metadata endpoints is blocked for security (detected: %s)", sanitized)
	}
	return blockedAddressError("connection to private ip addresses is blocked for security (detected: %s)", sanitized)
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

// blockedAddressErr carries a policy-rejection message verbatim while letting
// callers detect it with errors.Is(err, network.ErrBlockedAddress).
type blockedAddressErr struct{ msg string }

func (e *blockedAddressErr) Error() string { return e.msg }

func (e *blockedAddressErr) Unwrap() error { return network.ErrBlockedAddress }

// blockedAddressError formats a message for a destination rejected by the
// address policy. Only address rejections use it; scheme, DNS and port
// failures do not wrap the sentinel.
func blockedAddressError(format string, args ...any) error {
	return &blockedAddressErr{msg: fmt.Sprintf(format, args...)}
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
