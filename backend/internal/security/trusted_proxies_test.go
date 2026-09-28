package security

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTrustedProxy(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"172.20.0.5", "172.20.0.5/32"},
		{" 172.20.0.5 ", "172.20.0.5/32"},
		{"::1", "::1/128"},
		{"10.0.0.0/8", "10.0.0.0/8"},
		{"10.1.2.3/8", "10.0.0.0/8"},
		{"fc00::/7", "fc00::/7"},
		{"0.0.0.0/0", "0.0.0.0/0"},
		{"::ffff:10.0.0.0/104", "10.0.0.0/8"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			p, err := ParseTrustedProxy(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, p.String())
		})
	}

	for _, bad := range []string{"", "not-an-ip", "10.0.0.0/33", "fe80::1%eth0", "10.0.0.1/abc", "example.com"} {
		t.Run("invalid "+bad, func(t *testing.T) {
			_, err := ParseTrustedProxy(bad)
			assert.Error(t, err)
		})
	}
}

func TestTrustedProxyMatcher(t *testing.T) {
	m := NewTrustedProxyMatcher([]string{"172.20.0.5", "10.0.0.0/24", "::1/128", "garbage"})
	assert.Equal(t, 3, m.Len(), "invalid entries are skipped defensively")

	assert.True(t, m.ContainsIP("172.20.0.5"))
	assert.True(t, m.ContainsIP("::ffff:172.20.0.5"), "IPv4-mapped peers match IPv4 prefixes")
	assert.True(t, m.ContainsIP("10.0.0.200"))
	assert.False(t, m.ContainsIP("10.0.1.1"))
	assert.True(t, m.ContainsIP("::1"))
	assert.False(t, m.ContainsIP("127.0.0.1"), "no IPv4/IPv6 loopback equivalence")
	assert.False(t, m.ContainsIP(""))
	assert.False(t, m.ContainsIP("bogus"))
	assert.False(t, m.Contains(netip.Addr{}))

	v4only := NewTrustedProxyMatcher([]string{"127.0.0.1/32"})
	assert.False(t, v4only.ContainsIP("::1"), "no IPv4/IPv6 loopback equivalence")

	zoned := NewTrustedProxyMatcher([]string{"fe80::/64"})
	assert.True(t, zoned.ContainsIP("fe80::1%eth0"), "zone is stripped from the peer")

	var empty TrustedProxyMatcher
	assert.Equal(t, 0, empty.Len())
	assert.False(t, empty.ContainsIP("10.0.0.1"))
}
