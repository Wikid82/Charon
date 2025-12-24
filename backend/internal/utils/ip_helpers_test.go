package utils

import "testing"

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		// Private IP ranges - Class A (10.0.0.0/8)
		{"10.0.0.1 is private", "10.0.0.1", true},
		{"10.255.255.255 is private", "10.255.255.255", true},
		{"10.10.10.10 is private", "10.10.10.10", true},

		// Private IP ranges - Class B (172.16.0.0/12)
		{"172.16.0.1 is private", "172.16.0.1", true},
		{"172.31.255.255 is private", "172.31.255.255", true},
		{"172.20.0.1 is private", "172.20.0.1", true},

		// Private IP ranges - Class C (192.168.0.0/16)
		{"192.168.1.1 is private", "192.168.1.1", true},
		{"192.168.0.1 is private", "192.168.0.1", true},
		{"192.168.255.255 is private", "192.168.255.255", true},

		// Docker bridge IPs (subset of 172.16.0.0/12)
		{"172.17.0.2 is private", "172.17.0.2", true},
		{"172.18.0.5 is private", "172.18.0.5", true},

		// Public IPs - should return false
		{"8.8.8.8 is public", "8.8.8.8", false},
		{"1.1.1.1 is public", "1.1.1.1", false},
		{"142.250.80.14 is public", "142.250.80.14", false},
		{"203.0.113.50 is public", "203.0.113.50", false},

		// Edge cases for 172.x range (outside 172.16-31)
		{"172.15.0.1 is public", "172.15.0.1", false},
		{"172.32.0.1 is public", "172.32.0.1", false},

		// Hostnames - should return false
		{"nginx hostname", "nginx", false},
		{"my-app hostname", "my-app", false},
		{"app.local hostname", "app.local", false},
		{"example.com hostname", "example.com", false},
		{"my-container.internal hostname", "my-container.internal", false},

		// Invalid inputs - should return false
		{"empty string", "", false},
		{"malformed IP", "192.168.1", false},
		{"too many octets", "192.168.1.1.1", false},
		{"negative octet", "192.168.-1.1", false},
		{"octet out of range", "192.168.256.1", false},
		{"letters in IP", "192.168.a.1", false},
		{"IPv6 address", "::1", false},
		{"IPv6 full address", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", false},

		// Localhost and special addresses
		{"localhost 127.0.0.1", "127.0.0.1", false},
		{"0.0.0.0", "0.0.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPrivateIP(tt.host)
			if result != tt.expected {
				t.Errorf("IsPrivateIP(%q) = %v, want %v", tt.host, result, tt.expected)
			}
		})
	}
}

func TestIsDockerBridgeIP(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		// Docker default bridge (172.17.x.x)
		{"172.17.0.1 is Docker bridge", "172.17.0.1", true},
		{"172.17.0.2 is Docker bridge", "172.17.0.2", true},
		{"172.17.255.255 is Docker bridge", "172.17.255.255", true},

		// Docker user-defined networks (172.18-31.x.x)
		{"172.18.0.1 is Docker bridge", "172.18.0.1", true},
		{"172.18.0.5 is Docker bridge", "172.18.0.5", true},
		{"172.20.0.1 is Docker bridge", "172.20.0.1", true},
		{"172.31.0.1 is Docker bridge", "172.31.0.1", true},
		{"172.31.255.255 is Docker bridge", "172.31.255.255", true},

		// Also matches 172.16.x.x (part of 172.16.0.0/12)
		{"172.16.0.1 is in Docker range", "172.16.0.1", true},

		// Private IPs NOT in Docker bridge range
		{"10.0.0.1 is not Docker bridge", "10.0.0.1", false},
		{"192.168.1.1 is not Docker bridge", "192.168.1.1", false},

		// Public IPs - should return false
		{"8.8.8.8 is public", "8.8.8.8", false},
		{"1.1.1.1 is public", "1.1.1.1", false},

		// Edge cases for 172.x range (outside 172.16-31)
		{"172.15.0.1 is outside Docker range", "172.15.0.1", false},
		{"172.32.0.1 is outside Docker range", "172.32.0.1", false},

		// Hostnames - should return false
		{"nginx hostname", "nginx", false},
		{"my-app hostname", "my-app", false},
		{"container-name hostname", "container-name", false},

		// Invalid inputs - should return false
		{"empty string", "", false},
		{"malformed IP", "172.17.0", false},
		{"too many octets", "172.17.0.2.1", false},
		{"letters in IP", "172.17.a.1", false},
		{"IPv6 address", "::1", false},

		// Special addresses
		{"localhost 127.0.0.1", "127.0.0.1", false},
		{"0.0.0.0", "0.0.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDockerBridgeIP(tt.host)
			if result != tt.expected {
				t.Errorf("IsDockerBridgeIP(%q) = %v, want %v", tt.host, result, tt.expected)
			}
		})
	}
}

// TestIsPrivateIP_IPv4Mapped tests IPv4-mapped IPv6 addresses
func TestIsPrivateIP_IPv4Mapped(t *testing.T) {
	// IPv4-mapped IPv6 addresses should be handled correctly
	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		// net.ParseIP converts IPv4-mapped IPv6 to IPv4
		{"::ffff:10.0.0.1 mapped", "::ffff:10.0.0.1", true},
		{"::ffff:192.168.1.1 mapped", "::ffff:192.168.1.1", true},
		{"::ffff:8.8.8.8 mapped", "::ffff:8.8.8.8", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPrivateIP(tt.host)
			if result != tt.expected {
				t.Errorf("IsPrivateIP(%q) = %v, want %v", tt.host, result, tt.expected)
			}
		})
	}
}
