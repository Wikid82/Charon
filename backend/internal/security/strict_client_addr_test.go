package security

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStrictClientAddr_CaddyVectors pins the strict walk to the rules of Caddy's strictUntrustedClientIp: read from the
// right, skip unparsable entries, strip ports and zones, and fall back to the peer when every entry is trusted.
func TestStrictClientAddr_CaddyVectors(t *testing.T) {
	trusted := []netip.Prefix{
		netip.MustParsePrefix("127.0.0.1/32"),
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("::1/128"),
		netip.MustParsePrefix("2001:db8:ffff::/48"),
	}
	peer := netip.MustParseAddr("10.0.0.2")
	tests := []struct {
		name   string
		peer   netip.Addr
		header string
		want   string
	}{
		{"single untrusted entry", peer, "198.51.100.7", "198.51.100.7"},
		{"rightmost untrusted entry wins over a spoofed leftmost", peer, "203.0.113.66, 198.51.100.7", "198.51.100.7"},
		{"trusted hops on the right are skipped", peer, "198.51.100.7, 10.0.0.9", "198.51.100.7"},
		{"every entry trusted gives the peer", peer, "10.0.0.5, 10.0.0.6", "10.0.0.2"},
		{"no header gives the peer", peer, "", "10.0.0.2"},
		{"entry with a port", peer, "198.51.100.7:5678", "198.51.100.7"},
		{"bracketed IPv6 with a port", peer, "[2001:db8::1]:443", "2001:db8::1"},
		{"IPv6 chain", peer, "2001:db8::1, 2001:db8:ffff::2", "2001:db8::1"},
		{"IPv6 zone is removed", peer, "fe80::1%eth0", "fe80::1"},
		{"whitespace and empty entries", peer, " , 198.51.100.7 ,  ", "198.51.100.7"},
		{"unparsable entries are skipped", peer, "198.51.100.7, bogus", "198.51.100.7"},
		{"trusted entry with a port is still trusted", peer, "198.51.100.7, 10.0.0.5:80", "198.51.100.7"},
		{"trusted IPv6 entry falls back to the peer", netip.MustParseAddr("::1"), "2001:db8:ffff::9", "::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StrictClientAddr(tt.peer, tt.header, trusted)
			assert.Equal(t, tt.want, got.String())
		})
	}
}

func TestPrefixesContain(t *testing.T) {
	list := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	assert.True(t, prefixesContain(list, netip.MustParseAddr("10.1.2.3")))
	assert.False(t, prefixesContain(list, netip.MustParseAddr("192.0.2.1")))
	assert.False(t, prefixesContain(nil, netip.MustParseAddr("10.1.2.3")))
}
