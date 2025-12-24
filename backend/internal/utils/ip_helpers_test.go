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

		// Localhost and special addresses - these are now blocked for SSRF protection
		{"localhost 127.0.0.1", "127.0.0.1", true},
		{"0.0.0.0", "0.0.0.0", true},
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

// ============== Phase 3.3: Additional IP Helpers Tests ==============

func TestIsPrivateIP_CIDRParseError(t *testing.T) {
	// Temporarily modify the private IP ranges to include an invalid CIDR
	// This tests graceful handling of CIDR parse errors

	// Since we can't modify the package-level variable, we test the function behavior
	// with edge cases that might trigger parsing issues

	// Test with various invalid IP formats (should return false gracefully)
	invalidInputs := []string{
		"10.0.0.1/8",      // CIDR notation (not a raw IP)
		"10.0.0.256",      // Invalid octet
		"999.999.999.999", // Out of range
		"10.0.0",          // Incomplete
		"not-an-ip",       // Hostname
		"",                // Empty
		"10.0.0.1.1",      // Too many octets
	}

	for _, input := range invalidInputs {
		t.Run(input, func(t *testing.T) {
			result := IsPrivateIP(input)
			// All invalid inputs should return false (not panic)
			if result {
				t.Errorf("IsPrivateIP(%q) = true, want false for invalid input", input)
			}
		})
	}
}

func TestIsDockerBridgeIP_CIDRParseError(t *testing.T) {
	// Test graceful handling of invalid inputs
	invalidInputs := []string{
		"172.17.0.1/16",   // CIDR notation
		"172.17.0.256",    // Invalid octet
		"999.999.999.999", // Out of range
		"172.17",          // Incomplete
		"not-an-ip",       // Hostname
		"",                // Empty
	}

	for _, input := range invalidInputs {
		t.Run(input, func(t *testing.T) {
			result := IsDockerBridgeIP(input)
			// All invalid inputs should return false (not panic)
			if result {
				t.Errorf("IsDockerBridgeIP(%q) = true, want false for invalid input", input)
			}
		})
	}
}

func TestIsPrivateIP_IPv6Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		// IPv6 Loopback
		{"IPv6 loopback", "::1", false}, // Current implementation treats loopback as non-private
		{"IPv6 loopback expanded", "0000:0000:0000:0000:0000:0000:0000:0001", false},

		// IPv6 Link-Local (fe80::/10)
		{"IPv6 link-local", "fe80::1", false},
		{"IPv6 link-local 2", "fe80::abcd:ef01:2345:6789", false},

		// IPv6 Unique Local (fc00::/7)
		{"IPv6 unique local fc00", "fc00::1", false},
		{"IPv6 unique local fd00", "fd00::1", false},
		{"IPv6 unique local fdff", "fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", false},

		// IPv6 Public addresses
		{"IPv6 public Google DNS", "2001:4860:4860::8888", false},
		{"IPv6 public Cloudflare", "2606:4700:4700::1111", false},

		// IPv6 mapped IPv4
		{"IPv6 mapped private", "::ffff:10.0.0.1", true},
		{"IPv6 mapped public", "::ffff:8.8.8.8", false},

		// Invalid IPv6
		{"Invalid IPv6", "gggg::1", false},
		{"Incomplete IPv6", "2001::", false},
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

func TestIsDockerBridgeIP_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		// Boundaries of 172.16.0.0/12 range
		{"Lower boundary - 1", "172.15.255.255", false}, // Just outside
		{"Lower boundary", "172.16.0.0", true},          // Start of range
		{"Lower boundary + 1", "172.16.0.1", true},

		{"Upper boundary - 1", "172.31.255.254", true},
		{"Upper boundary", "172.31.255.255", true},  // End of range
		{"Upper boundary + 1", "172.32.0.0", false}, // Just outside
		{"Upper boundary + 2", "172.32.0.1", false},

		// Docker default bridge (172.17.0.0/16)
		{"Docker default bridge start", "172.17.0.0", true},
		{"Docker default bridge gateway", "172.17.0.1", true},
		{"Docker default bridge host", "172.17.0.2", true},
		{"Docker default bridge end", "172.17.255.255", true},

		// Docker user-defined networks
		{"User network 1", "172.18.0.1", true},
		{"User network 2", "172.19.0.1", true},
		{"User network 30", "172.30.0.1", true},
		{"User network 31", "172.31.0.1", true},

		// Edge of 172.x range
		{"172.0.0.1", "172.0.0.1", false},             // Below range
		{"172.15.0.1", "172.15.0.1", false},           // Below range
		{"172.32.0.1", "172.32.0.1", false},           // Above range
		{"172.255.255.255", "172.255.255.255", false}, // Above range
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
