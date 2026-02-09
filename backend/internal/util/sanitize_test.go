package util

import "testing"

func TestSanitizeForLog(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "clean string",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "string with newline",
			input:    "Hello\nWorld",
			expected: "Hello World",
		},
		{
			name:     "string with carriage return and newline",
			input:    "Hello\r\nWorld",
			expected: "Hello World",
		},
		{
			name:     "string with multiple newlines",
			input:    "Hello\nWorld\nTest",
			expected: "Hello World Test",
		},
		{
			name:     "string with control characters",
			input:    "Hello\x00\x01\x1FWorld",
			expected: "Hello World",
		},
		{
			name:     "string with DEL character (0x7F)",
			input:    "Hello\x7FWorld",
			expected: "Hello World",
		},
		{
			name:     "complex string with mixed control chars",
			input:    "Line1\r\nLine2\nLine3\x00\x01\x7F",
			expected: "Line1 Line2 Line3 ",
		},
		{
			name:     "string with tabs (0x09 is control char)",
			input:    "Hello\tWorld",
			expected: "Hello World",
		},
		{
			name:     "string with only control chars",
			input:    "\x00\x01\x02\x1F\x7F",
			expected: " ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeForLog(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeForLog(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCanonicalizeIPForSecurity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "IPv4 standard",
			input:    "192.168.1.1",
			expected: "192.168.1.1",
		},
		{
			name:     "IPv4 with port (should strip port)",
			input:    "192.168.1.1:8080",
			expected: "192.168.1.1",
		},
		{
			name:     "IPv6 standard",
			input:    "2001:db8::1",
			expected: "2001:db8::1",
		},
		{
			name:     "IPv6 loopback (::1) normalizes to 127.0.0.1",
			input:    "::1",
			expected: "127.0.0.1",
		},
		{
			name:     "IPv6 loopback with brackets",
			input:    "[::1]",
			expected: "127.0.0.1",
		},
		{
			name:     "IPv6 loopback with brackets and port",
			input:    "[::1]:8080",
			expected: "127.0.0.1",
		},
		{
			name:     "IPv4-mapped IPv6 address",
			input:    "::ffff:192.168.1.1",
			expected: "192.168.1.1",
		},
		{
			name:     "IPv4-mapped IPv6 with brackets",
			input:    "[::ffff:192.168.1.1]",
			expected: "192.168.1.1",
		},
		{
			name:     "IPv4 localhost",
			input:    "127.0.0.1",
			expected: "127.0.0.1",
		},
		{
			name:     "IPv4 0.0.0.0",
			input:    "0.0.0.0",
			expected: "0.0.0.0",
		},
		{
			name:     "invalid IP format",
			input:    "invalid",
			expected: "invalid",
		},
		{
			name:     "comma-separated (should take first)",
			input:    "192.168.1.1, 10.0.0.1",
			expected: "192.168.1.1",
		},
		{
			name:     "whitespace",
			input:    "  192.168.1.1  ",
			expected: "192.168.1.1",
		},
		{
			name:     "IPv6 full form",
			input:    "2001:0db8:0000:0000:0000:0000:0000:0001",
			expected: "2001:db8::1",
		},
		{
			name:     "IPv6 with zone",
			input:    "fe80::1%eth0",
			expected: "fe80::1%eth0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanonicalizeIPForSecurity(tt.input)
			if result != tt.expected {
				t.Errorf("CanonicalizeIPForSecurity(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
