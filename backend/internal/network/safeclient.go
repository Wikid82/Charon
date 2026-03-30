// Package network provides SSRF-safe HTTP client and networking utilities.
// This package implements comprehensive Server-Side Request Forgery (SSRF) protection
// by validating IP addresses at multiple layers: URL validation, DNS resolution, and connection time.
package network

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// privateBlocks holds pre-parsed CIDR blocks for private/reserved IP ranges.
// These are parsed once at package initialization for performance.
var (
	privateBlocks []*net.IPNet
	initOnce      sync.Once
)

// rfc1918Blocks holds pre-parsed CIDR blocks for RFC 1918 private address ranges only.
// Initialized once and used by IsRFC1918 to support the AllowRFC1918 bypass path.
var (
	rfc1918Blocks []*net.IPNet
	rfc1918Once   sync.Once
)

// rfc1918CIDRs enumerates exactly the three RFC 1918 private address ranges.
// Intentionally excludes loopback, link-local, cloud metadata (169.254.x.x),
// and all other reserved ranges — those remain blocked regardless of AllowRFC1918.
var rfc1918CIDRs = []string{
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
}

// privateCIDRs defines all private and reserved IP ranges to block for SSRF protection.
// This list covers:
// - RFC 1918 private networks (10.x, 172.16-31.x, 192.168.x)
// - Loopback addresses (127.x.x.x, ::1)
// - Link-local addresses (169.254.x.x, fe80::) including cloud metadata endpoints
// - Reserved ranges (0.x.x.x, 240.x.x.x, 255.255.255.255)
// - IPv6 unique local addresses (fc00::)
var privateCIDRs = []string{
	// IPv4 Private Networks (RFC 1918)
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",

	// IPv4 Link-Local (RFC 3927) - includes AWS/GCP/Azure metadata service (169.254.169.254)
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

// initPrivateBlocks parses all CIDR blocks once at startup.
func initPrivateBlocks() {
	initOnce.Do(func() {
		privateBlocks = make([]*net.IPNet, 0, len(privateCIDRs))
		for _, cidr := range privateCIDRs {
			_, block, err := net.ParseCIDR(cidr)
			if err != nil {
				// This should never happen with valid CIDR strings
				continue
			}
			privateBlocks = append(privateBlocks, block)
		}
	})
}

// initRFC1918Blocks parses the three RFC 1918 CIDR blocks once at startup.
func initRFC1918Blocks() {
	rfc1918Once.Do(func() {
		rfc1918Blocks = make([]*net.IPNet, 0, len(rfc1918CIDRs))
		for _, cidr := range rfc1918CIDRs {
			_, block, err := net.ParseCIDR(cidr)
			if err != nil {
				// This should never happen with valid CIDR strings
				continue
			}
			rfc1918Blocks = append(rfc1918Blocks, block)
		}
	})
}

// IsPrivateIP checks if an IP address is private, loopback, link-local, or otherwise restricted.
// This function implements comprehensive SSRF protection by blocking:
//   - Private IPv4 ranges (RFC 1918): 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
//   - Loopback addresses: 127.0.0.0/8, ::1/128
//   - Link-local addresses: 169.254.0.0/16, fe80::/10 (includes cloud metadata endpoints)
//   - Reserved ranges: 0.0.0.0/8, 240.0.0.0/4, 255.255.255.255/32
//   - IPv6 unique local addresses: fc00::/7
//
// IPv4-mapped IPv6 addresses (::ffff:x.x.x.x) are correctly handled by extracting
// the IPv4 portion and validating it.
//
// Returns true if the IP should be blocked, false if it's safe for external requests.
func IsPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true // nil IPs should be blocked
	}

	// Ensure private blocks are initialized
	initPrivateBlocks()

	// Handle IPv4-mapped IPv6 addresses (::ffff:x.x.x.x)
	// Convert to IPv4 for consistent checking
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}

	// Check built-in Go functions for common cases (fast path)
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}

	// Check against all private/reserved CIDR blocks
	for _, block := range privateBlocks {
		if block.Contains(ip) {
			return true
		}
	}

	return false
}

// IsRFC1918 reports whether an IP address belongs to one of the three RFC 1918
// private address ranges: 10.0.0.0/8, 172.16.0.0/12, or 192.168.0.0/16.
//
// Unlike IsPrivateIP, this function only covers RFC 1918 ranges. It does NOT
// return true for loopback, link-local (169.254.x.x), cloud metadata endpoints,
// or any other reserved ranges. Use this to implement the AllowRFC1918 bypass
// while keeping all other SSRF protections in place.
//
// Exported so url_validator.go (package security) can call it without duplicating logic.
func IsRFC1918(ip net.IP) bool {
	if ip == nil {
		return false
	}

	initRFC1918Blocks()

	// Normalise IPv4-mapped IPv6 addresses (::ffff:192.168.x.x → 192.168.x.x)
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}

	for _, block := range rfc1918Blocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// ClientOptions configures the behavior of the safe HTTP client.
type ClientOptions struct {
	// Timeout is the total request timeout (default: 10s)
	Timeout time.Duration

	// AllowLocalhost permits connections to localhost/127.0.0.1 (default: false)
	// Use only for testing or when connecting to known-safe local services.
	AllowLocalhost bool

	// AllowedDomains restricts requests to specific domains (optional).
	// If set, only these domains will be allowed (in addition to localhost if AllowLocalhost is true).
	AllowedDomains []string

	// MaxRedirects sets the maximum number of redirects to follow (default: 0)
	// Set to 0 to disable redirects entirely.
	MaxRedirects int

	// DialTimeout is the connection timeout for individual dial attempts (default: 5s)
	DialTimeout time.Duration

	// AllowRFC1918 permits connections to RFC 1918 private address ranges:
	// 10.0.0.0/8, 172.16.0.0/12, and 192.168.0.0/16.
	//
	// SECURITY NOTE: Enable only for admin-configured features (e.g., uptime monitors
	// targeting internal hosts). All other restricted ranges — loopback, link-local,
	// cloud metadata (169.254.x.x), and reserved — remain blocked regardless.
	AllowRFC1918 bool
}

// Option is a functional option for configuring ClientOptions.
type Option func(*ClientOptions)

// defaultOptions returns the default safe client configuration.
func defaultOptions() ClientOptions {
	return ClientOptions{
		Timeout:        10 * time.Second,
		AllowLocalhost: false,
		AllowedDomains: nil,
		MaxRedirects:   0,
		DialTimeout:    5 * time.Second,
	}
}

// WithTimeout sets the total request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(opts *ClientOptions) {
		opts.Timeout = timeout
	}
}

// WithAllowLocalhost permits connections to localhost addresses.
// Use this option only when connecting to known-safe local services (e.g., CrowdSec LAPI).
func WithAllowLocalhost() Option {
	return func(opts *ClientOptions) {
		opts.AllowLocalhost = true
	}
}

// WithAllowedDomains restricts requests to specific domains.
// When set, only requests to these domains will be permitted.
func WithAllowedDomains(domains ...string) Option {
	return func(opts *ClientOptions) {
		opts.AllowedDomains = append(opts.AllowedDomains, domains...)
	}
}

// WithMaxRedirects sets the maximum number of redirects to follow.
// Default is 0 (no redirects). Each redirect target is validated against SSRF rules.
func WithMaxRedirects(maxRedirects int) Option {
	return func(opts *ClientOptions) {
		opts.MaxRedirects = maxRedirects
	}
}

// WithDialTimeout sets the connection timeout for individual dial attempts.
func WithDialTimeout(timeout time.Duration) Option {
	return func(opts *ClientOptions) {
		opts.DialTimeout = timeout
	}
}

// WithAllowRFC1918 permits connections to RFC 1918 private address ranges
// (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16).
//
// Use only for admin-configured features such as uptime monitors that need to
// reach internal hosts. All other SSRF protections remain active.
func WithAllowRFC1918() Option {
	return func(opts *ClientOptions) {
		opts.AllowRFC1918 = true
	}
}

// safeDialer creates a custom dial function that validates IP addresses at connection time.
// This prevents DNS rebinding attacks by:
// 1. Resolving the hostname to IP addresses
// 2. Validating ALL resolved IPs against IsPrivateIP
// 3. Connecting directly to the validated IP (not the hostname)
//
// This approach defeats Time-of-Check to Time-of-Use (TOCTOU) attacks where
// DNS could return different IPs between validation and connection.
func safeDialer(opts *ClientOptions) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// Parse host:port from address
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid address format: %w", err)
		}

		// Check if this is an allowed localhost address
		isLocalhost := host == "localhost" || host == "127.0.0.1" || host == "::1"
		if isLocalhost && opts.AllowLocalhost {
			// Allow localhost connections when explicitly permitted
			dialer := &net.Dialer{Timeout: opts.DialTimeout}
			return dialer.DialContext(ctx, network, addr)
		}

		// Resolve DNS with context timeout
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("DNS resolution failed for %s: %w", host, err)
		}

		if len(ips) == 0 {
			return nil, fmt.Errorf("no IP addresses found for host: %s", host)
		}

		// Validate ALL resolved IPs - if ANY are private, reject the entire request
		// This prevents attackers from using DNS load balancing to mix private/public IPs
		for _, ip := range ips {
			// Allow localhost IPs if AllowLocalhost is set
			if opts.AllowLocalhost && ip.IP.IsLoopback() {
				continue
			}

			// Allow RFC 1918 addresses only when explicitly permitted (e.g., admin-configured
			// uptime monitors targeting internal hosts). Link-local (169.254.x.x), loopback,
			// cloud metadata, and all other restricted ranges remain blocked.
			if opts.AllowRFC1918 && IsRFC1918(ip.IP) {
				continue
			}

			if IsPrivateIP(ip.IP) {
				return nil, fmt.Errorf("connection to private IP blocked: %s resolved to %s", host, ip.IP)
			}
		}

		// Find first valid IP to connect to
		var selectedIP net.IP
		for _, ip := range ips {
			if opts.AllowLocalhost && ip.IP.IsLoopback() {
				selectedIP = ip.IP
				break
			}
			// Select RFC 1918 IPs when the caller has opted in.
			if opts.AllowRFC1918 && IsRFC1918(ip.IP) {
				selectedIP = ip.IP
				break
			}
			if !IsPrivateIP(ip.IP) {
				selectedIP = ip.IP
				break
			}
		}

		if selectedIP == nil {
			return nil, fmt.Errorf("no valid IP addresses found for host: %s", host)
		}

		// Connect to the validated IP (prevents DNS rebinding TOCTOU attacks)
		dialer := &net.Dialer{Timeout: opts.DialTimeout}
		return dialer.DialContext(ctx, network, net.JoinHostPort(selectedIP.String(), port))
	}
}

// validateRedirectTarget checks if a redirect URL is safe to follow.
// Returns an error if the redirect target resolves to private IPs.
//
// TODO: If MaxRedirects is ever re-enabled for uptime monitors, thread AllowRFC1918
// through this function to permit RFC 1918 redirect targets.
func validateRedirectTarget(req *http.Request, opts *ClientOptions) error {
	host := req.URL.Hostname()
	if host == "" {
		return fmt.Errorf("missing hostname in redirect URL")
	}

	// Check localhost
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		if opts.AllowLocalhost {
			return nil
		}
		return fmt.Errorf("redirect to localhost blocked")
	}

	// Resolve and validate IPs
	ctx, cancel := context.WithTimeout(context.Background(), opts.DialTimeout)
	defer cancel()

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("DNS resolution failed for redirect target %s: %w", host, err)
	}

	for _, ip := range ips {
		if opts.AllowLocalhost && ip.IP.IsLoopback() {
			continue
		}
		if IsPrivateIP(ip.IP) {
			return fmt.Errorf("redirect to private IP blocked: %s resolved to %s", host, ip.IP)
		}
	}

	return nil
}

// NewSafeHTTPClient creates an HTTP client with comprehensive SSRF protection.
// The client validates IP addresses at connection time to prevent:
//   - Direct connections to private/reserved IP ranges
//   - DNS rebinding attacks (TOCTOU)
//   - Redirects to private IP addresses
//   - Cloud metadata endpoint access (169.254.169.254)
//
// Default configuration:
//   - 10 second timeout
//   - No redirects (returns http.ErrUseLastResponse)
//   - Keep-alives disabled
//   - Private IPs blocked
//
// Use functional options to customize behavior:
//
//	// Allow localhost for local service communication
//	client := network.NewSafeHTTPClient(network.WithAllowLocalhost())
//
//	// Set custom timeout
//	client := network.NewSafeHTTPClient(network.WithTimeout(5 * time.Second))
//
//	// Allow specific redirects
//	client := network.NewSafeHTTPClient(network.WithMaxRedirects(2))
func NewSafeHTTPClient(opts ...Option) *http.Client {
	cfg := defaultOptions()
	for _, opt := range opts {
		opt(&cfg)
	}

	return &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			// Explicitly ignore proxy environment variables for SSRF-sensitive requests.
			Proxy:                 nil,
			DialContext:           safeDialer(&cfg),
			DisableKeepAlives:     true,
			MaxIdleConns:          1,
			IdleConnTimeout:       cfg.Timeout,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: cfg.Timeout,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// No redirects allowed by default
			if cfg.MaxRedirects == 0 {
				return http.ErrUseLastResponse
			}

			// Check redirect count
			if len(via) >= cfg.MaxRedirects {
				return fmt.Errorf("too many redirects (max %d)", cfg.MaxRedirects)
			}

			// Validate redirect target for SSRF
			if err := validateRedirectTarget(req, &cfg); err != nil {
				return err
			}

			return nil
		},
	}
}
