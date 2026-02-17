package security

import "testing"

func TestIsIPInCIDRList(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		list     string
		expected bool
	}{
		{
			name:     "empty list",
			ip:       "127.0.0.1",
			list:     "",
			expected: false,
		},
		{
			name:     "direct IP match",
			ip:       "127.0.0.1",
			list:     "127.0.0.1",
			expected: true,
		},
		{
			name:     "cidr match",
			ip:       "172.16.5.10",
			list:     "172.16.0.0/12",
			expected: true,
		},
		{
			name:     "mixed list with whitespace",
			ip:       "10.0.0.5",
			list:     "192.168.0.0/16, 10.0.0.0/8",
			expected: true,
		},
		{
			name:     "no match",
			ip:       "203.0.113.10",
			list:     "192.168.0.0/16,10.0.0.0/8",
			expected: false,
		},
		{
			name:     "invalid client ip",
			ip:       "not-an-ip",
			list:     "192.168.0.0/16",
			expected: false,
		},
		{
			name:     "IPv6 loopback match",
			ip:       "::1",
			list:     "::1",
			expected: true,
		},
		{
			name:     "IPv6 loopback CIDR match",
			ip:       "::1",
			list:     "::1/128",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsIPInCIDRList(tt.ip, tt.list); got != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}
