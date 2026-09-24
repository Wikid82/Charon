package ratelimit

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientKey(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"ipv4", "203.0.113.7", "203.0.113.7"},
		{"ipv4 with spaces", "  203.0.113.7 ", "203.0.113.7"},
		{"ipv4-mapped ipv6", "::ffff:203.0.113.7", "203.0.113.7"},
		{"ipv4 host:port", "203.0.113.7:4431", "203.0.113.7"},
		{"ipv6 aggregated to /64", "2001:db8:1:2:aaaa:bbbb:cccc:dddd", "2001:db8:1:2::/64"},
		{"ipv6 same /64", "2001:db8:1:2::1", "2001:db8:1:2::/64"},
		{"ipv6 different /64", "2001:db8:1:3::1", "2001:db8:1:3::/64"},
		{"ipv6 bracketed host:port", "[2001:db8:1:2::1]:443", "2001:db8:1:2::/64"},
		{"ipv6 loopback canonicalized", "::1", "127.0.0.1"},
		{"ipv4 loopback", "127.0.0.1", "127.0.0.1"},
		{"ipv6 zone stripped", "fe80::1%eth0", "fe80::/64"},
		{"empty", "", UnknownClientKey},
		{"whitespace", "   ", UnknownClientKey},
		{"garbage", "not-an-ip", UnknownClientKey},
		{"hostname", "example.com", UnknownClientKey},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ClientKey(tc.in))
		})
	}
}

func TestClassifyAddr(t *testing.T) {
	cases := []struct {
		in   string
		want AddrScope
	}{
		{"127.0.0.1", ScopeLoopback},
		{"127.8.9.10", ScopeLoopback},
		{"::1", ScopeLoopback},
		{"::ffff:127.0.0.1", ScopeLoopback},
		{"10.1.2.3", ScopePrivate},
		{"172.16.0.1", ScopePrivate},
		{"172.31.255.254", ScopePrivate},
		{"192.168.1.1", ScopePrivate},
		{"::ffff:192.168.1.1", ScopePrivate},
		{"fd00::1", ScopePrivate},
		{"fc00::1", ScopePrivate},
		{"100.64.0.1", ScopePrivate},
		{"100.127.255.254", ScopePrivate},
		{"169.254.1.1", ScopePrivate},
		{"fe80::1", ScopePrivate},
		{"172.32.0.1", ScopePublic},
		{"100.128.0.1", ScopePublic},
		{"203.0.113.9", ScopePublic},
		{"8.8.8.8", ScopePublic},
		{"2001:db8::1", ScopePublic},
		{"2606:4700::1111", ScopePublic},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, ClassifyAddr(netip.MustParseAddr(tc.in)))
		})
	}
	assert.Equal(t, ScopePublic, ClassifyAddr(netip.Addr{}), "invalid address is never treated as local")
}
