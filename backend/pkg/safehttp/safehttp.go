// Package safehttp provides validated outbound HTTP for in-tree and
// community DNS provider plugins that cannot import Charon's internal
// packages. It is a deliberately minimal facade over the internal network
// helpers.
package safehttp

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/security"
)

// ErrBlockedAddress is wrapped by every error reporting a destination that the
// address policy rejects. Test for it with errors.Is; the message never
// contains a resolved IP address.
var ErrBlockedAddress = network.ErrBlockedAddress

const maxHostnameLength = 253

// Policy selects which address classes an integration may reach. The zero
// value is the strictest policy (https only, public addresses only). Fields are
// unexported: policies can only be built through the constructors below.
type Policy struct {
	allowHTTP    bool
	allowRFC1918 bool
	allowCGNAT   bool
}

// PublicHTTPSOnly returns the strictest policy: https only, and RFC 1918,
// carrier-grade NAT and transition ranges are blocked.
func PublicHTTPSOnly() Policy {
	return Policy{}
}

// PrivateNetworkOK returns a policy that permits http and https and allows
// RFC 1918 and carrier-grade NAT addresses. Loopback, link-local, unspecified,
// reserved and transition ranges stay blocked.
func PrivateNetworkOK() Policy {
	return Policy{allowHTTP: true, allowRFC1918: true, allowCGNAT: true}
}

// clientOptions maps the policy onto the internal client options.
func (p Policy) clientOptions(timeout time.Duration) []network.Option {
	opts := []network.Option{network.WithTimeout(timeout), network.WithBlockTransitionRanges()}
	if p.allowRFC1918 {
		opts = append(opts, network.WithAllowRFC1918())
	}
	if !p.allowCGNAT {
		opts = append(opts, network.WithBlockCGNAT())
	}
	return opts
}

// ValidateURLSyntax validates raw without any DNS lookups: the scheme allowlist
// for the policy, a present hostname, no userinfo, no query, no fragment, and
// literal-IP hosts checked against the address policy.
func ValidateURLSyntax(raw string, p Policy) (*url.URL, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("url is required")
	}
	if strings.ContainsAny(raw, "#") {
		return nil, errors.New("url must not contain a fragment")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid url format: %w", err)
	}

	switch u.Scheme {
	case "https":
	case "http":
		if !p.allowHTTP {
			return nil, errors.New("http scheme not allowed (use https)")
		}
	default:
		return nil, fmt.Errorf("unsupported scheme %q (only http and https are allowed)", u.Scheme)
	}

	if u.Opaque != "" {
		return nil, errors.New("url must be of the form scheme://host[:port][/path]")
	}
	if u.User != nil {
		return nil, errors.New("urls with embedded credentials are not allowed")
	}
	if u.RawQuery != "" || u.ForceQuery {
		return nil, errors.New("url must not contain a query string")
	}
	host := u.Hostname()
	if host == "" {
		return nil, errors.New("missing hostname in url")
	}
	if len(host) > maxHostnameLength {
		return nil, fmt.Errorf("hostname exceeds maximum length of %d characters", maxHostnameLength)
	}
	if strings.Contains(host, "..") {
		return nil, errors.New("hostname contains suspicious pattern (..)")
	}

	if port := u.Port(); port != "" {
		n, convErr := strconv.Atoi(port)
		if convErr != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("invalid port: %s", port)
		}
	}

	if err := checkLiteralHost(host, p); err != nil {
		return nil, err
	}
	return u, nil
}

// checkLiteralHost applies the address policy to hosts that are literal IPs or
// well-known loopback names. Other hostnames cannot be judged without DNS; the
// dialer in NewClient is authoritative for those.
func checkLiteralHost(host string, p Policy) error {
	name := strings.TrimSuffix(host, ".")
	lower := strings.ToLower(name)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return fmt.Errorf("url host is not allowed: %w", ErrBlockedAddress)
	}

	// Drop an IPv6 zone identifier before parsing; zoned addresses are
	// link-local and are rejected by the range checks below.
	if i := strings.IndexByte(name, '%'); i >= 0 {
		name = name[:i]
	}
	ip := net.ParseIP(name)
	if ip == nil {
		if isNonCanonicalNumericHost(name) {
			return fmt.Errorf("url host is not allowed: %w", ErrBlockedAddress)
		}
		return nil
	}

	blocked := network.IsTransitionRange(ip) ||
		(network.IsCGNAT(ip) && !p.allowCGNAT) ||
		(network.IsPrivateIP(ip) && (!p.allowRFC1918 || !network.IsRFC1918(ip)))
	if blocked {
		return fmt.Errorf("url host is not allowed: %w", ErrBlockedAddress)
	}
	return nil
}

// isNonCanonicalNumericHost reports whether host is made only of numeric
// segments (decimal digits or 0x-prefixed hex) separated by dots but is not a
// canonical dotted-quad address, such as "127.1", "2130706433", "0x7f000001" or
// "0177.0.0.1". Some resolvers and proxies read these as IPv4 addresses, so
// they are refused rather than guessed at. Names that merely contain digits
// (dns1.example.com, 10-0-0-1.example.com) are not affected.
func isNonCanonicalNumericHost(host string) bool {
	if host == "" {
		return false
	}
	for _, seg := range strings.Split(host, ".") {
		if !isNumericSegment(seg) {
			return false
		}
	}
	return true
}

func isNumericSegment(seg string) bool {
	if seg == "" {
		return false
	}
	digits := seg
	isDigit := func(r rune) bool { return r >= '0' && r <= '9' }
	if len(seg) >= 2 && seg[0] == '0' && (seg[1] == 'x' || seg[1] == 'X') {
		digits = seg[2:]
		isDigit = func(r rune) bool {
			return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
		}
	}
	for _, r := range digits {
		if !isDigit(r) {
			return false
		}
	}
	return true
}

// ValidateURL runs ValidateURLSyntax and then a DNS-based address check.
//
// It is an early-error convenience only: the dialer inside NewClient re-checks
// every address at connect time and is the authoritative control. In particular
// the early check does not apply the carrier-grade NAT or transition-range
// rules to resolved hostnames.
func ValidateURL(raw string, p Policy) (*url.URL, error) {
	u, err := ValidateURLSyntax(raw, p)
	if err != nil {
		return nil, err
	}

	opts := []security.ValidationOption{}
	if p.allowHTTP {
		opts = append(opts, security.WithAllowHTTP())
	}
	if p.allowRFC1918 {
		opts = append(opts, security.WithAllowRFC1918())
	}
	// The returned normalised string is discarded on purpose: requests are built
	// from the *url.URL parsed above, never from a re-parsed string.
	if _, err := security.ValidateExternalURL(raw, opts...); err != nil {
		if errors.Is(err, network.ErrBlockedAddress) {
			return nil, fmt.Errorf("url validation failed: %w", ErrBlockedAddress)
		}
		return nil, fmt.Errorf("url validation failed: %w", err)
	}
	return u, nil
}

// NewClient returns an HTTP client that ignores proxy environment variables,
// does not follow redirects, and re-validates the destination address on every
// connection. There is deliberately no way to allow loopback through this API.
func NewClient(p Policy, timeout time.Duration) *http.Client {
	return newClient(p, timeout)
}

// newClient is NewClient plus extra internal options; it exists so this
// package's own tests can reach httptest servers.
func newClient(p Policy, timeout time.Duration, extra ...network.Option) *http.Client {
	return network.NewSafeHTTPClient(append(p.clientOptions(timeout), extra...)...)
}

// JoinPath appends escaped path segments to the path of a validated base URL
// and returns the resulting absolute URL string. Empty segments, dot segments
// (including encoded forms), control characters and separators inside a segment
// are rejected, so each segment always stays exactly one path element.
func JoinPath(base *url.URL, segments ...string) (string, error) {
	if base == nil || base.Host == "" {
		return "", errors.New("base url is required")
	}

	var b strings.Builder
	b.WriteString(base.Scheme)
	b.WriteString("://")
	b.WriteString(base.Host)
	b.WriteString(strings.TrimRight(base.EscapedPath(), "/"))

	for _, seg := range segments {
		if err := checkSegment(seg); err != nil {
			return "", err
		}
		b.WriteByte('/')
		b.WriteString(url.PathEscape(seg))
	}
	return b.String(), nil
}

func checkSegment(seg string) error {
	if seg == "" {
		return errors.New("path segment must not be empty")
	}
	for _, candidate := range []string{seg, unescapeOrSelf(seg)} {
		if candidate == "." || candidate == ".." {
			return errors.New("path segment must not be a dot segment")
		}
		if strings.ContainsAny(candidate, `/\`) {
			return errors.New("path segment must not contain a path separator")
		}
		for _, r := range candidate {
			if r < 0x20 || r == 0x7f {
				return errors.New("path segment must not contain control characters")
			}
		}
	}
	return nil
}

func unescapeOrSelf(s string) string {
	if dec, err := url.PathUnescape(s); err == nil {
		return dec
	}
	return s
}
