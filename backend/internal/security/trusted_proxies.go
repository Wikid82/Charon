package security

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strings"

	"github.com/Wikid82/charon/backend/internal/network"
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
		if v4 := ip.To4(); v4 != nil {
			// An IPv4-mapped address is written in its IPv4 form, so the /32 applies to the IPv4 address.
			s = v4.String() + "/32"
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

// TrustedProxiesSettingKey is the settings row that holds the proxy-layer trust list: canonical CIDRs, comma-separated.
// Loopback is never stored; it is always trusted.
const TrustedProxiesSettingKey = "security.proxy.trusted_proxies"

// ManagementListsSettingKey is the settings row that holds the access lists protecting Charon's own interface, as a
// JSON array of UUIDs.
const ManagementListsSettingKey = "security.acl.management_lists"

// MaxTrustedProxies bounds how many distinct entries the trust list may hold.
const MaxTrustedProxies = 64

// minTrustedIPv4Bits and minTrustedIPv6Bits are the widest prefixes a trusted proxy may cover. Wider ranges would let
// almost any peer speak for the visitor.
const (
	minTrustedIPv4Bits = 8
	minTrustedIPv6Bits = 24
	// warnTrustedBits marks an IPv4 range wide enough to deserve an operator warning.
	warnTrustedBits = 16
)

// Refusal kinds of the trust list. Match them with errors.Is; TrustedProxyEntryError carries the entry.
var (
	ErrTrustedProxyInvalid   = errors.New("trusted proxy entry is not an IP address or CIDR range")
	ErrTrustedProxyTooBroad  = errors.New("trusted proxy entry covers too wide a range")
	ErrTooManyTrustedProxies = errors.New("too many trusted proxy entries")
)

// TrustedProxyEntryError names the entry that the trust list refused.
type TrustedProxyEntryError struct {
	Entry string
	Err   error
}

func (e *TrustedProxyEntryError) Error() string {
	return fmt.Sprintf("%v: %q", e.Err, e.Entry)
}

func (e *TrustedProxyEntryError) Unwrap() error { return e.Err }

// AlwaysTrustedProxies returns the loopback ranges that every configuration trusts. It returns a fresh slice.
func AlwaysTrustedProxies() []string {
	return []string{"127.0.0.1/32", "::1/128"}
}

// isAlwaysTrusted reports whether p is one of the always-trusted loopback prefixes.
func isAlwaysTrusted(p netip.Prefix) bool {
	for _, always := range AlwaysTrustedProxies() {
		if q := netip.MustParsePrefix(always); q == p {
			return true
		}
	}
	return false
}

// checkTrustedEntry parses one entry and applies the range rules. It returns the canonical prefix and, for a range
// that deserves an operator warning, the warning text.
func checkTrustedEntry(raw string) (netip.Prefix, string, error) {
	entry := strings.TrimSpace(raw)
	p, err := ParseTrustedProxy(entry)
	if err != nil {
		return netip.Prefix{}, "", &TrustedProxyEntryError{Entry: entry, Err: ErrTrustedProxyInvalid}
	}
	minBits := minTrustedIPv4Bits
	if p.Addr().Is6() {
		minBits = minTrustedIPv6Bits
	}
	if p.Bits() < minBits {
		return netip.Prefix{}, "", &TrustedProxyEntryError{Entry: entry, Err: ErrTrustedProxyTooBroad}
	}
	return p, rangeWarning(p), nil
}

// rangeWarning describes why a canonical range is accepted with a warning, or returns "" when it is not wide.
func rangeWarning(p netip.Prefix) string {
	for _, local := range network.LocalNetworkCIDRs() {
		local := netip.MustParsePrefix(local)
		if p.Bits() <= local.Bits() && p.Contains(local.Addr()) {
			return fmt.Sprintf("%s covers all of the local-network range %s", p, local)
		}
	}
	if p.Bits() < warnTrustedBits {
		return fmt.Sprintf("%s is a wide range; every address in it is trusted as a proxy", p)
	}
	return ""
}

// sortedPrefixStrings returns the canonical text of each prefix, sorted.
func sortedPrefixStrings(prefixes []netip.Prefix) []string {
	out := make([]string, 0, len(prefixes))
	for _, p := range prefixes {
		out = append(out, p.String())
	}
	slices.Sort(out)
	return out
}

// NormalizeTrustedProxies validates the operator's list for storage. Entries are trimmed, bare addresses become host
// prefixes, and each prefix is masked and unmapped. Empty entries and the always-trusted loopback are dropped, and
// duplicates are merged. The result is sorted. Warnings name ranges accepted with a caution. An invalid or too-broad
// entry, or more than MaxTrustedProxies distinct entries, returns an error and no list.
func NormalizeTrustedProxies(entries []string) (canonical, warnings []string, err error) {
	seen := make(map[netip.Prefix]bool, len(entries))
	var kept []netip.Prefix
	for _, raw := range entries {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		p, warning, checkErr := checkTrustedEntry(raw)
		if checkErr != nil {
			return nil, nil, checkErr
		}
		if isAlwaysTrusted(p) || seen[p] {
			continue
		}
		seen[p] = true
		if warning != "" {
			warnings = append(warnings, warning)
		}
		kept = append(kept, p)
	}
	if len(kept) > MaxTrustedProxies {
		return nil, nil, ErrTooManyTrustedProxies
	}
	return sortedPrefixStrings(kept), warnings, nil
}

// ClampTrustedProxies applies the same range rules to a list that was not validated on save, such as the
// CHARON_TRUSTED_PROXIES environment value. Invalid and too-broad entries are dropped instead of refused, and so are
// entries past the limit. kept is the canonical list and dropped holds the refused entries in input order.
func ClampTrustedProxies(entries []string) (kept, dropped []string) {
	seen := make(map[netip.Prefix]bool, len(entries))
	var prefixes []netip.Prefix
	for _, raw := range entries {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		p, _, err := checkTrustedEntry(raw)
		if err != nil {
			dropped = append(dropped, strings.TrimSpace(raw))
			continue
		}
		if isAlwaysTrusted(p) || seen[p] {
			continue
		}
		if len(prefixes) >= MaxTrustedProxies {
			dropped = append(dropped, strings.TrimSpace(raw))
			continue
		}
		seen[p] = true
		prefixes = append(prefixes, p)
	}
	return sortedPrefixStrings(prefixes), dropped
}

// ParseStoredTrustedProxies validates the stored setting text, a comma-separated list. An empty value means no
// configured ranges, so only loopback is trusted.
func ParseStoredTrustedProxies(raw string) (canonical, warnings []string, err error) {
	return NormalizeTrustedProxies(strings.Split(raw, ","))
}

// EffectiveTrust describes the proxy-layer trust list in force and where it comes from.
type EffectiveTrust struct {
	// Ranges is every trusted range: the always-trusted loopback first, then the configured ranges.
	Ranges []string
	// Configured is the operator's ranges without loopback.
	Configured []string
	// Source is "setting" when a saved value is in force, "environment" when the environment list is used and
	// "default" when neither is set.
	Source string
	// Dropped holds the entries that were refused from the source.
	Dropped []string
	// StoredError is set when a saved value no longer validates. The list then falls back to loopback only.
	StoredError error
}

// EffectiveTrustedProxies returns the trust list the proxy layer uses. A saved value (stored non-nil) is used alone:
// it is validated, and when it no longer validates only loopback is trusted. Without a saved value the environment
// list is clamped with ClampTrustedProxies, and with neither, loopback alone is trusted.
func EffectiveTrustedProxies(stored *string, env []string) EffectiveTrust {
	if stored != nil {
		configured, _, err := ParseStoredTrustedProxies(*stored)
		trust := EffectiveTrust{Source: "setting", StoredError: err}
		if err != nil {
			_, trust.Dropped = ClampTrustedProxies(strings.Split(*stored, ","))
			trust.Ranges = AlwaysTrustedProxies()
			return trust
		}
		trust.Configured = configured
		trust.Ranges = append(AlwaysTrustedProxies(), configured...)
		return trust
	}

	var envEntries []string
	for _, entry := range env {
		if strings.TrimSpace(entry) != "" {
			envEntries = append(envEntries, entry)
		}
	}
	trust := EffectiveTrust{Source: "default"}
	if len(envEntries) > 0 {
		trust.Source = "environment"
		trust.Configured, trust.Dropped = ClampTrustedProxies(envEntries)
	}
	trust.Ranges = append(AlwaysTrustedProxies(), trust.Configured...)
	return trust
}

// StrictClientAddr returns the visitor address the way Caddy's strict walk does when the connection peer is trusted:
// the forwarded chain is read from the right and the first entry outside trusted is the visitor. Entries may carry a
// port or an IPv6 zone, and unparsable entries are skipped. When every entry is trusted, the walk gives the peer.
// The result is not unmapped, as Caddy does not unmap it either.
func StrictClientAddr(peer netip.Addr, forwardedFor string, trusted []netip.Prefix) netip.Addr {
	parts := strings.Split(forwardedFor, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		host := parts[i]
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		host, _, _ = strings.Cut(strings.TrimSpace(host), "%")
		addr, err := netip.ParseAddr(host)
		if err != nil {
			continue
		}
		if !prefixesContain(trusted, addr) {
			return addr
		}
	}
	return peer
}

// prefixesContain reports whether any prefix in list contains addr.
func prefixesContain(list []netip.Prefix, addr netip.Addr) bool {
	for _, p := range list {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
