package services

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"unicode"

	"golang.org/x/net/idna"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// ErrInvalidHostName is returned when a request host cannot be normalized to a
// plain DNS name or IP literal.
var ErrInvalidHostName = errors.New("invalid host name")

// NormalizeHostName converts a host as seen in a request (optionally with a
// port, bracketed IPv6 literal or trailing dot) into its canonical lowercase,
// ASCII form for exact comparison. Hosts that are empty or contain control
// characters, spaces, '%' or '*' are rejected.
func NormalizeHostName(raw string) (string, error) {
	host := strings.TrimSpace(raw)
	if host == "" {
		return "", ErrInvalidHostName
	}
	for _, r := range host {
		if unicode.IsControl(r) || unicode.IsSpace(r) || r == '%' || r == '*' {
			return "", ErrInvalidHostName
		}
	}

	host = stripPort(host)
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return "", ErrInvalidHostName
	}

	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.Unmap().String(), nil
	}
	if strings.ContainsAny(host, ":[]") {
		return "", ErrInvalidHostName
	}

	host = strings.ToLower(host)
	if !isASCII(host) {
		ascii, err := idna.Lookup.ToASCII(host)
		if err != nil || ascii == "" {
			return "", ErrInvalidHostName
		}
		host = strings.ToLower(ascii)
	}
	return host, nil
}

// stripPort removes a trailing :port and the brackets around IPv6 literals.
func stripPort(host string) string {
	if strings.HasPrefix(host, "[") {
		end := strings.Index(host, "]")
		if end < 0 {
			return host
		}
		rest := host[end+1:]
		if rest == "" || strings.HasPrefix(rest, ":") {
			return host[1:end]
		}
		return host
	}
	// A single colon separates host and port; several colons mean a bare IPv6 literal.
	if strings.Count(host, ":") == 1 {
		h, _, _ := strings.Cut(host, ":")
		return h
	}
	return host
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// MatchProxyHosts returns the hosts whose domain list covers requestHost.
// Domain lists are comma-separated; each entry is compared exactly after
// normalization. A stored wildcard such as "*.example.com" covers exactly one
// additional label. Exact matches take precedence: wildcard matches are only
// returned when no host matches exactly.
func MatchProxyHosts(hosts []models.ProxyHost, requestHost string) ([]models.ProxyHost, error) {
	want, err := NormalizeHostName(requestHost)
	if err != nil {
		return nil, err
	}

	var exact, wildcard []models.ProxyHost
	for i := range hosts {
		kind := matchDomainList(hosts[i].DomainNames, want)
		switch kind {
		case matchExact:
			exact = append(exact, hosts[i])
		case matchWildcard:
			wildcard = append(wildcard, hosts[i])
		}
	}
	if len(exact) > 0 {
		return exact, nil
	}
	return wildcard, nil
}

type matchKind int

const (
	matchNone matchKind = iota
	matchWildcard
	matchExact
)

func matchDomainList(domainNames, want string) matchKind {
	best := matchNone
	for _, entry := range strings.Split(domainNames, ",") {
		entry = strings.TrimSpace(entry)
		isWildcard := false
		if rest, ok := strings.CutPrefix(entry, "*."); ok {
			isWildcard = true
			entry = rest
		}
		have, err := NormalizeHostName(entry)
		if err != nil {
			continue
		}
		switch {
		case !isWildcard && have == want:
			return matchExact
		case isWildcard && coversOneLabel(have, want):
			best = matchWildcard
		}
	}
	return best
}

// coversOneLabel reports whether want is base with exactly one extra label.
func coversOneLabel(base, want string) bool {
	label, ok := strings.CutSuffix(want, "."+base)
	return ok && label != "" && !strings.Contains(label, ".")
}

// FindProxyHostsByDomain loads the proxy hosts whose domain list covers
// requestHost. An empty result with a nil error means no host matches.
func FindProxyHostsByDomain(db *gorm.DB, requestHost string) ([]models.ProxyHost, error) {
	if _, err := NormalizeHostName(requestHost); err != nil {
		return nil, err
	}
	var hosts []models.ProxyHost
	if err := db.Select("id", "domain_names", "forward_auth_enabled").Find(&hosts).Error; err != nil {
		return nil, fmt.Errorf("load proxy hosts: %w", err)
	}
	return MatchProxyHosts(hosts, requestHost)
}
