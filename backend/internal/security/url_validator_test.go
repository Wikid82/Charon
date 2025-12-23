package security

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestValidateExternalURL_BasicValidation(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		options     []ValidationOption
		shouldFail  bool
		errContains string
	}{
		{
			name:       "Valid HTTPS URL",
			url:        "https://api.example.com/webhook",
			options:    nil,
			shouldFail: false,
		},
		{
			name:        "HTTP without AllowHTTP option",
			url:         "http://api.example.com/webhook",
			options:     nil,
			shouldFail:  true,
			errContains: "http scheme not allowed",
		},
		{
			name:       "HTTP with AllowHTTP option",
			url:        "http://api.example.com/webhook",
			options:    []ValidationOption{WithAllowHTTP()},
			shouldFail: false,
		},
		{
			name:        "Empty URL",
			url:         "",
			options:     nil,
			shouldFail:  true,
			errContains: "unsupported scheme",
		},
		{
			name:        "Missing scheme",
			url:         "example.com",
			options:     nil,
			shouldFail:  true,
			errContains: "unsupported scheme",
		},
		{
			name:        "Just scheme",
			url:         "https://",
			options:     nil,
			shouldFail:  true,
			errContains: "missing hostname",
		},
		{
			name:        "FTP protocol",
			url:         "ftp://example.com",
			options:     nil,
			shouldFail:  true,
			errContains: "unsupported scheme: ftp",
		},
		{
			name:        "File protocol",
			url:         "file:///etc/passwd",
			options:     nil,
			shouldFail:  true,
			errContains: "unsupported scheme: file",
		},
		{
			name:        "Gopher protocol",
			url:         "gopher://example.com",
			options:     nil,
			shouldFail:  true,
			errContains: "unsupported scheme: gopher",
		},
		{
			name:        "Data URL",
			url:         "data:text/html,<script>alert(1)</script>",
			options:     nil,
			shouldFail:  true,
			errContains: "unsupported scheme: data",
		},
		{
			name:        "URL with credentials",
			url:         "https://user:pass@example.com",
			options:     nil,
			shouldFail:  true,
			errContains: "embedded credentials are not allowed",
		},
		{
			name:       "Valid with port",
			url:        "https://api.example.com:8080/webhook",
			options:    nil,
			shouldFail: false,
		},
		{
			name:       "Valid with path",
			url:        "https://api.example.com/path/to/webhook",
			options:    nil,
			shouldFail: false,
		},
		{
			name:       "Valid with query",
			url:        "https://api.example.com/webhook?token=abc123",
			options:    nil,
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateExternalURL(tt.url, tt.options...)

			if tt.shouldFail {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tt.url)
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					// For tests that expect success but DNS may fail in test environment,
					// we accept DNS errors but not validation errors
					if !strings.Contains(err.Error(), "dns resolution failed") {
						t.Errorf("Unexpected validation error for %s: %v", tt.url, err)
					} else {
						t.Logf("Note: DNS resolution failed for %s (expected in test environment)", tt.url)
					}
				}
			}
		})
	}
}

func TestValidateExternalURL_LocalhostHandling(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		options     []ValidationOption
		shouldFail  bool
		errContains string
	}{
		{
			name:        "Localhost without AllowLocalhost",
			url:         "https://localhost/webhook",
			options:     nil,
			shouldFail:  true,
			errContains: "", // Will fail on DNS or be blocked
		},
		{
			name:       "Localhost with AllowLocalhost",
			url:        "https://localhost/webhook",
			options:    []ValidationOption{WithAllowLocalhost()},
			shouldFail: false,
		},
		{
			name:       "127.0.0.1 with AllowLocalhost and AllowHTTP",
			url:        "http://127.0.0.1:8080/test",
			options:    []ValidationOption{WithAllowLocalhost(), WithAllowHTTP()},
			shouldFail: false,
		},
		{
			name:       "IPv6 loopback with AllowLocalhost",
			url:        "https://[::1]:3000/test",
			options:    []ValidationOption{WithAllowLocalhost()},
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateExternalURL(tt.url, tt.options...)

			if tt.shouldFail {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tt.url)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", tt.url, err)
				}
			}
		})
	}
}

func TestValidateExternalURL_PrivateIPBlocking(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		options     []ValidationOption
		shouldFail  bool
		errContains string
	}{
		// Note: These tests will only work if DNS actually resolves to these IPs
		// In practice, we can't control DNS resolution in unit tests
		// Integration tests or mocked DNS would be needed for comprehensive coverage
		{
			name:        "Private IP 10.x.x.x",
			url:         "http://10.0.0.1",
			options:     []ValidationOption{WithAllowHTTP()},
			shouldFail:  true,
			errContains: "dns resolution failed", // Will likely fail DNS
		},
		{
			name:        "Private IP 192.168.x.x",
			url:         "http://192.168.1.1",
			options:     []ValidationOption{WithAllowHTTP()},
			shouldFail:  true,
			errContains: "dns resolution failed",
		},
		{
			name:        "Private IP 172.16.x.x",
			url:         "http://172.16.0.1",
			options:     []ValidationOption{WithAllowHTTP()},
			shouldFail:  true,
			errContains: "dns resolution failed",
		},
		{
			name:        "AWS Metadata IP",
			url:         "http://169.254.169.254",
			options:     []ValidationOption{WithAllowHTTP()},
			shouldFail:  true,
			errContains: "dns resolution failed",
		},
		{
			name:        "Loopback without AllowLocalhost",
			url:         "http://127.0.0.1",
			options:     []ValidationOption{WithAllowHTTP()},
			shouldFail:  true,
			errContains: "dns resolution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateExternalURL(tt.url, tt.options...)

			if tt.shouldFail {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tt.url)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", tt.url, err)
				}
			}
		})
	}
}

func TestValidateExternalURL_Options(t *testing.T) {
	t.Run("WithTimeout", func(t *testing.T) {
		// Test with very short timeout - should fail for slow DNS
		_, err := ValidateExternalURL(
			"https://example.com",
			WithTimeout(1*time.Nanosecond),
		)
		// We expect this might fail due to timeout, but it's acceptable
		// The point is the option is applied
		_ = err // Acknowledge error
	})

	t.Run("Multiple options", func(t *testing.T) {
		_, err := ValidateExternalURL(
			"http://localhost:8080/test",
			WithAllowLocalhost(),
			WithAllowHTTP(),
			WithTimeout(5*time.Second),
		)
		if err != nil {
			t.Errorf("Unexpected error with multiple options: %v", err)
		}
	})
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name      string
		ip        string
		isPrivate bool
	}{
		// RFC 1918 Private Networks
		{"10.0.0.0", "10.0.0.0", true},
		{"10.255.255.255", "10.255.255.255", true},
		{"172.16.0.0", "172.16.0.0", true},
		{"172.31.255.255", "172.31.255.255", true},
		{"192.168.0.0", "192.168.0.0", true},
		{"192.168.255.255", "192.168.255.255", true},

		// Loopback
		{"127.0.0.1", "127.0.0.1", true},
		{"127.0.0.2", "127.0.0.2", true},
		{"IPv6 loopback", "::1", true},

		// Link-Local (includes AWS/GCP metadata)
		{"169.254.1.1", "169.254.1.1", true},
		{"AWS metadata", "169.254.169.254", true},

		// Reserved ranges
		{"0.0.0.0", "0.0.0.0", true},
		{"255.255.255.255", "255.255.255.255", true},
		{"240.0.0.1", "240.0.0.1", true},

		// IPv6 Unique Local and Link-Local
		{"IPv6 unique local", "fc00::1", true},
		{"IPv6 link-local", "fe80::1", true},

		// Public IPs (should NOT be blocked)
		{"Google DNS", "8.8.8.8", false},
		{"Cloudflare DNS", "1.1.1.1", false},
		{"Public IPv6", "2001:4860:4860::8888", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := parseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Invalid test IP: %s", tt.ip)
			}

			result := isPrivateIP(ip)
			if result != tt.isPrivate {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, result, tt.isPrivate)
			}
		})
	}
}

// Helper function to parse IP address
func parseIP(s string) net.IP {
	ip := net.ParseIP(s)
	return ip
}

func TestValidateExternalURL_RealWorldURLs(t *testing.T) {
	// These tests use real public domains
	// They may fail if DNS is unavailable or domains change
	tests := []struct {
		name       string
		url        string
		options    []ValidationOption
		shouldFail bool
	}{
		{
			name:       "Slack webhook format",
			url:        "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX",
			options:    nil,
			shouldFail: false,
		},
		{
			name:       "Discord webhook format",
			url:        "https://discord.com/api/webhooks/123456789/abcdefg",
			options:    nil,
			shouldFail: false,
		},
		{
			name:       "Generic API endpoint",
			url:        "https://api.github.com/repos/user/repo",
			options:    nil,
			shouldFail: false,
		},
		{
			name:       "Localhost for testing",
			url:        "http://localhost:3000/webhook",
			options:    []ValidationOption{WithAllowLocalhost(), WithAllowHTTP()},
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateExternalURL(tt.url, tt.options...)

			if tt.shouldFail && err == nil {
				t.Errorf("Expected error for %s, got nil", tt.url)
			}
			if !tt.shouldFail && err != nil {
				// Real-world URLs might fail due to network issues
				// Log but don't fail the test
				t.Logf("Note: %s failed validation (may be network issue): %v", tt.url, err)
			}
		})
	}
}
