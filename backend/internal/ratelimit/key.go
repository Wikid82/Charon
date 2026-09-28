package ratelimit

import (
	"net/netip"

	"github.com/Wikid82/charon/backend/internal/util"
)

// UnknownClientKey is the shared, fail-closed key for requests whose client
// address is empty or unparsable.
const UnknownClientKey = "unknown"

// ipv6KeyBits is the IPv6 aggregation prefix; an IPv6 client typically
// controls a whole /64.
const ipv6KeyBits = 64

// AddrScope classifies a network address for operator guidance.
type AddrScope string

// Address scopes reported by ClassifyAddr.
const (
	ScopeLoopback AddrScope = "loopback"
	ScopePrivate  AddrScope = "private"
	ScopePublic   AddrScope = "public"
)

var cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")

// ClassifyAddr returns ScopeLoopback, ScopePrivate (RFC 1918, ULA fc00::/7,
// CGNAT 100.64.0.0/10, link-local) or ScopePublic. Invalid addresses are
// public so they never produce a trust suggestion.
func ClassifyAddr(a netip.Addr) AddrScope {
	if !a.IsValid() {
		return ScopePublic
	}
	a = a.Unmap()
	switch {
	case a.IsLoopback():
		return ScopeLoopback
	case a.IsPrivate(), cgnatPrefix.Contains(a), a.IsLinkLocalUnicast():
		return ScopePrivate
	default:
		return ScopePublic
	}
}

// ClientKey maps a resolved client IP (normally gin.Context.ClientIP()) to a
// throttle key: IPv4 per address, IPv6 per /64, otherwise UnknownClientKey.
func ClientKey(clientIP string) string {
	addr, err := netip.ParseAddr(util.CanonicalizeIPForSecurity(clientIP))
	if err != nil {
		return UnknownClientKey
	}
	addr = addr.Unmap()
	if addr.Is4() {
		return addr.String()
	}
	return netip.PrefixFrom(addr.WithZone(""), ipv6KeyBits).Masked().String()
}
