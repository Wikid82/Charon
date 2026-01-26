// Package util provides utility functions used across the application.
package util

import (
	"net"
	"regexp"
	"strings"
)

// SanitizeForLog removes control characters and newlines from user content before logging.
func SanitizeForLog(s string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	re := regexp.MustCompile(`[\x00-\x1F\x7F]+`)
	s = re.ReplaceAllString(s, " ")
	return s
}

// CanonicalizeIPForSecurity normalizes an IP string for security decisions
// (rate limiting keys, allow-list CIDR checks, etc.). It preserves Gin's
// trust proxy behavior by operating on the already-resolved client IP string.
//
// Normalizations:
// - IPv6 loopback (::1) -> 127.0.0.1 (stable across IPv4/IPv6 localhost)
// - IPv4-mapped IPv6 (e.g. ::ffff:127.0.0.1) -> 127.0.0.1
func CanonicalizeIPForSecurity(ipStr string) string {
	ipStr = strings.TrimSpace(ipStr)
	if ipStr == "" {
		return ipStr
	}

	// Defensive normalization in case the input is not a plain IP string.
	// Gin's Context.ClientIP() should return an IP, but in proxy/test setups
	// we may still see host:port or comma-separated values.
	if idx := strings.IndexByte(ipStr, ','); idx >= 0 {
		ipStr = strings.TrimSpace(ipStr[:idx])
	}
	if host, _, err := net.SplitHostPort(ipStr); err == nil {
		ipStr = host
	}
	ipStr = strings.Trim(ipStr, "[]")

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ipStr
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	if ip.IsLoopback() {
		return "127.0.0.1"
	}
	return ip.String()
}
