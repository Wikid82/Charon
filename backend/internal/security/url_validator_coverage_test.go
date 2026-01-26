package security

import (
	"os"
	"testing"
)

// TestInternalServiceHostAllowlist tests the internal service hostname allowlist.
func TestInternalServiceHostAllowlist(t *testing.T) {
	// Save original env var
	originalEnv := os.Getenv(InternalServiceHostAllowlistEnvVar)
	defer func() { _ = os.Setenv(InternalServiceHostAllowlistEnvVar, originalEnv) }()

	t.Run("DefaultLocalhostOnly", func(t *testing.T) {
		_ = os.Setenv(InternalServiceHostAllowlistEnvVar, "")
		allowlist := InternalServiceHostAllowlist()

		// Should contain localhost entries
		expected := []string{"localhost", "127.0.0.1", "::1"}
		for _, host := range expected {
			if _, ok := allowlist[host]; !ok {
				t.Errorf("Expected %s to be in default allowlist", host)
			}
		}

		// Should only have 3 localhost entries
		if len(allowlist) != 3 {
			t.Errorf("Expected 3 entries in default allowlist, got %d", len(allowlist))
		}
	})

	t.Run("WithAdditionalHosts", func(t *testing.T) {
		_ = os.Setenv(InternalServiceHostAllowlistEnvVar, "crowdsec,caddy,traefik")
		allowlist := InternalServiceHostAllowlist()

		// Should contain localhost + additional hosts
		expected := []string{"localhost", "127.0.0.1", "::1", "crowdsec", "caddy", "traefik"}
		for _, host := range expected {
			if _, ok := allowlist[host]; !ok {
				t.Errorf("Expected %s to be in allowlist", host)
			}
		}

		if len(allowlist) != 6 {
			t.Errorf("Expected 6 entries in allowlist, got %d", len(allowlist))
		}
	})

	t.Run("WithEmptyAndWhitespaceEntries", func(t *testing.T) {
		_ = os.Setenv(InternalServiceHostAllowlistEnvVar, "  , crowdsec ,  , caddy , ")
		allowlist := InternalServiceHostAllowlist()

		// Should contain localhost + valid hosts (empty and whitespace ignored)
		expected := []string{"localhost", "127.0.0.1", "::1", "crowdsec", "caddy"}
		for _, host := range expected {
			if _, ok := allowlist[host]; !ok {
				t.Errorf("Expected %s to be in allowlist", host)
			}
		}

		if len(allowlist) != 5 {
			t.Errorf("Expected 5 entries in allowlist, got %d", len(allowlist))
		}
	})

	t.Run("WithInvalidEntries", func(t *testing.T) {
		_ = os.Setenv(InternalServiceHostAllowlistEnvVar, "crowdsec,http://invalid,user@host,/path")
		allowlist := InternalServiceHostAllowlist()

		// Should only have localhost + crowdsec (others rejected)
		if _, ok := allowlist["crowdsec"]; !ok {
			t.Error("Expected crowdsec to be in allowlist")
		}
		if _, ok := allowlist["http://invalid"]; ok {
			t.Error("Did not expect http://invalid to be in allowlist")
		}
		if _, ok := allowlist["user@host"]; ok {
			t.Error("Did not expect user@host to be in allowlist")
		}
		if _, ok := allowlist["/path"]; ok {
			t.Error("Did not expect /path to be in allowlist")
		}
	})
}

// TestWithMaxRedirects tests the WithMaxRedirects validation option.
func TestWithMaxRedirects(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		expected int
	}{
		{
			name:     "Zero redirects",
			value:    0,
			expected: 0,
		},
		{
			name:     "Five redirects",
			value:    5,
			expected: 5,
		},
		{
			name:     "Ten redirects",
			value:    10,
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ValidationConfig{}
			opt := WithMaxRedirects(tt.value)
			opt(config)

			if config.MaxRedirects != tt.expected {
				t.Errorf("Expected MaxRedirects=%d, got %d", tt.expected, config.MaxRedirects)
			}
		})
	}
}

// TestValidateInternalServiceBaseURL_AdditionalCases tests edge cases for ValidateInternalServiceBaseURL.
func TestValidateInternalServiceBaseURL_AdditionalCases(t *testing.T) {
	allowlist := map[string]struct{}{
		"localhost": {},
		"caddy":     {},
	}

	t.Run("HTTPSWithDefaultPort", func(t *testing.T) {
		// HTTPS without explicit port should default to 443
		url, err := ValidateInternalServiceBaseURL("https://localhost", 443, allowlist)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if url.String() != "https://localhost:443" {
			t.Errorf("Expected https://localhost:443, got %s", url.String())
		}
	})

	t.Run("HTTPWithDefaultPort", func(t *testing.T) {
		// HTTP without explicit port should default to 80
		url, err := ValidateInternalServiceBaseURL("http://localhost", 80, allowlist)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if url.String() != "http://localhost:80" {
			t.Errorf("Expected http://localhost:80, got %s", url.String())
		}
	})

	t.Run("PortMismatchWithDefaultHTTPS", func(t *testing.T) {
		// HTTPS defaults to 443, but we expect 2019
		_, err := ValidateInternalServiceBaseURL("https://localhost", 2019, allowlist)
		if err == nil {
			t.Fatal("Expected error for port mismatch, got nil")
		}
		if !contains(err.Error(), "unexpected port") {
			t.Errorf("Expected 'unexpected port' error, got %v", err)
		}
	})

	t.Run("PortMismatchWithDefaultHTTP", func(t *testing.T) {
		// HTTP defaults to 80, but we expect 8080
		_, err := ValidateInternalServiceBaseURL("http://localhost", 8080, allowlist)
		if err == nil {
			t.Fatal("Expected error for port mismatch, got nil")
		}
		if !contains(err.Error(), "unexpected port") {
			t.Errorf("Expected 'unexpected port' error, got %v", err)
		}
	})

	t.Run("InvalidPortNumber", func(t *testing.T) {
		_, err := ValidateInternalServiceBaseURL("http://localhost:99999", 99999, allowlist)
		if err == nil {
			t.Fatal("Expected error for invalid port, got nil")
		}
		if !contains(err.Error(), "invalid port") {
			t.Errorf("Expected 'invalid port' error, got %v", err)
		}
	})

	t.Run("NegativePort", func(t *testing.T) {
		_, err := ValidateInternalServiceBaseURL("http://localhost:-1", -1, allowlist)
		if err == nil {
			t.Fatal("Expected error for negative port, got nil")
		}
		if !contains(err.Error(), "invalid port") {
			t.Errorf("Expected 'invalid port' error, got %v", err)
		}
	})

	t.Run("HostNotInAllowlist", func(t *testing.T) {
		_, err := ValidateInternalServiceBaseURL("http://evil.com:80", 80, allowlist)
		if err == nil {
			t.Fatal("Expected error for disallowed host, got nil")
		}
		if !contains(err.Error(), "hostname not allowed") {
			t.Errorf("Expected 'hostname not allowed' error, got %v", err)
		}
	})

	t.Run("EmptyAllowlist", func(t *testing.T) {
		emptyList := map[string]struct{}{}
		_, err := ValidateInternalServiceBaseURL("http://localhost:80", 80, emptyList)
		if err == nil {
			t.Fatal("Expected error for empty allowlist, got nil")
		}
		if !contains(err.Error(), "hostname not allowed") {
			t.Errorf("Expected 'hostname not allowed' error, got %v", err)
		}
	})

	t.Run("CaseInsensitiveHostMatching", func(t *testing.T) {
		// Hostname should be case-insensitive
		url, err := ValidateInternalServiceBaseURL("http://LOCALHOST:2019", 2019, allowlist)
		if err != nil {
			t.Fatalf("Expected no error for uppercase hostname, got %v", err)
		}
		if url.Hostname() != "LOCALHOST" {
			t.Errorf("Expected hostname preservation, got %s", url.Hostname())
		}
	})

	t.Run("AllowedHostDifferentCase", func(t *testing.T) {
		// Caddy in allowlist, CADDY in URL
		url, err := ValidateInternalServiceBaseURL("http://CADDY:2019", 2019, allowlist)
		if err != nil {
			t.Fatalf("Expected no error for case variation, got %v", err)
		}
		if url.Hostname() != "CADDY" {
			t.Errorf("Expected hostname CADDY, got %s", url.Hostname())
		}
	})
}

// TestSanitizeIPForError_AdditionalCases tests additional edge cases for IP sanitization.
func TestSanitizeIPForError_AdditionalCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "InvalidIPString",
			input:    "not-an-ip",
			expected: "invalid-ip",
		},
		{
			name:     "EmptyString",
			input:    "",
			expected: "invalid-ip",
		},
		{
			name:     "IPv4Malformed",
			input:    "192.168",
			expected: "invalid-ip",
		},
		{
			name:     "IPv6SingleSegment",
			input:    "fe80::1",
			expected: "fe80::",
		},
		{
			name:     "IPv6MultipleSegments",
			input:    "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			expected: "2001::",
		},
		{
			name:     "IPv6Compressed",
			input:    "::1",
			expected: "::",
		},
		{
			name:     "IPv4ThreeOctets",
			input:    "192.168.1",
			expected: "invalid-ip",
		},
		{
			name:     "IPv4FiveOctets",
			input:    "192.168.1.1.1",
			expected: "invalid-ip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeIPForError(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
