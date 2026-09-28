package security

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
)

// ipv4InIPv6Bits is the prefix length of the ::ffff:0:0/96 IPv4-mapped block.
const ipv4InIPv6Bits = 96

// ParseTrustedProxy parses one CHARON_TRUSTED_PROXIES entry with Gin's rules
// (gin.Engine.prepareTrustedCIDRs): the entry is trimmed, a bare IP becomes a
// /32 (IPv4) or /128 (IPv6), and everything else must parse as a CIDR.
func ParseTrustedProxy(entry string) (netip.Prefix, error) {
	s := strings.TrimSpace(entry)
	if !strings.Contains(s, "/") {
		ip := net.ParseIP(s)
		if ip == nil {
			return netip.Prefix{}, fmt.Errorf("invalid trusted proxy %q: not an IP address or CIDR", s)
		}
		if ip.To4() != nil {
			s += "/32"
		} else {
			s += "/128"
		}
	}
	_, ipNet, err := net.ParseCIDR(s)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("invalid trusted proxy CIDR: %w", err)
	}
	addr, _ := netip.AddrFromSlice(ipNet.IP)
	ones, _ := ipNet.Mask.Size()
	if addr.Is4In6() && ones >= ipv4InIPv6Bits {
		addr = addr.Unmap()
		ones -= ipv4InIPv6Bits
	}
	return netip.PrefixFrom(addr, ones).Masked(), nil
}

// TrustedProxyMatcher matches TCP peers against a trusted-proxy list using
// parsed netip prefixes. IPv4-mapped peers match IPv4 prefixes; IPv4 and IPv6
// loopback are distinct, as in Gin. The zero value trusts nothing.
type TrustedProxyMatcher struct {
	prefixes []netip.Prefix
}

// NewTrustedProxyMatcher parses entries with ParseTrustedProxy. Invalid entries
// are skipped defensively; config.Load already rejects lists containing them.
func NewTrustedProxyMatcher(entries []string) TrustedProxyMatcher {
	var m TrustedProxyMatcher
	for _, e := range entries {
		if p, err := ParseTrustedProxy(e); err == nil {
			m.prefixes = append(m.prefixes, p)
		}
	}
	return m
}

// Len returns the number of usable prefixes.
func (m TrustedProxyMatcher) Len() int { return len(m.prefixes) }

// Contains reports whether addr (zone stripped, unmapped) is a trusted proxy.
func (m TrustedProxyMatcher) Contains(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	addr = addr.WithZone("").Unmap()
	for _, p := range m.prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// ContainsIP parses ip (e.g. gin.Context.RemoteIP()) and calls Contains.
// Unparsable input is never trusted.
func (m TrustedProxyMatcher) ContainsIP(ip string) bool {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return false
	}
	return m.Contains(addr)
}
