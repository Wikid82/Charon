package middleware

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/security"
)

// TrustedPeer reports whether the request's TCP peer (its raw RemoteAddr, never a
// header) is an address in the configured trusted-proxy set. A zero matcher trusts
// nobody.
func TrustedPeer(c *gin.Context, trusted security.TrustedProxyMatcher) bool {
	if c == nil || c.Request == nil || trusted.Len() == 0 {
		return false
	}
	host := strings.TrimSpace(c.Request.RemoteAddr)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return trusted.ContainsIP(strings.Trim(host, "[]"))
}

// ForwardedRequest returns the scheme and host entries the client addressed, as
// reported along the proxy chain. Callers fall back to the request itself for any
// value that comes back empty.
//
// A verified self-proxy hop record is used first. Forwarded headers are read only
// when the TCP peer is a trusted proxy: X-Forwarded-Proto supplies the scheme when
// the hop did not, and X-Forwarded-Host adds every comma-separated host entry.
// Entries are trimmed and empty entries are dropped.
func ForwardedRequest(c *gin.Context, trusted security.TrustedProxyMatcher) (scheme string, hosts []string) {
	if origin, ok := RequestOriginFrom(c); ok {
		scheme = origin.Scheme
		if origin.Host != "" {
			hosts = append(hosts, origin.Host)
		}
	}
	if !TrustedPeer(c, trusted) {
		return scheme, hosts
	}
	if scheme == "" {
		if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
			scheme = strings.ToLower(firstListEntry(proto))
		}
	}
	for _, part := range strings.Split(c.GetHeader("X-Forwarded-Host"), ",") {
		if host := strings.TrimSpace(part); host != "" {
			hosts = append(hosts, host)
		}
	}
	return scheme, hosts
}
