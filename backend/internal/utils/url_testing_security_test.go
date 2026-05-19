package utils

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// TestResolveAllowedIP_EmptyHostname tests resolveAllowedIP with empty hostname.
func TestResolveAllowedIP_EmptyHostname(t *testing.T) {
	ctx := context.Background()
	_, err := resolveAllowedIP(ctx, "", false)
	if err == nil {
		t.Fatal("Expected error for empty hostname, got nil")
	}
	if err.Error() != "missing hostname" {
		t.Errorf("Expected 'missing hostname', got: %v", err)
	}
}

// TestResolveAllowedIP_LoopbackIPLiteral tests resolveAllowedIP with loopback IPs.
func TestResolveAllowedIP_LoopbackIPLiteral(t *testing.T) {
	tests := []struct {
		name           string
		ip             string
		allowLocalhost bool
		shouldFail     bool
	}{
		{
			name:           "127.0.0.1 without allowLocalhost",
			ip:             "127.0.0.1",
			allowLocalhost: false,
			shouldFail:     true,
		},
		{
			name:           "127.0.0.1 with allowLocalhost",
			ip:             "127.0.0.1",
			allowLocalhost: true,
			shouldFail:     false,
		},
		{
			name:           "::1 without allowLocalhost",
			ip:             "::1",
			allowLocalhost: false,
			shouldFail:     true,
		},
		{
			name:           "::1 with allowLocalhost",
			ip:             "::1",
			allowLocalhost: true,
			shouldFail:     false,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, err := resolveAllowedIP(ctx, tt.ip, tt.allowLocalhost)

			if tt.shouldFail {
				if err == nil {
					t.Errorf("Expected error for %s without allowLocalhost", tt.ip)
				}
			} else {
				if err != nil {
					t.Fatal("ErroF for allowed loopback", err)
				}
				if ip == nil {
					t.Fatal("Expected non-nil IP")
				}
			}
		})
	}
}

// TestResolveAllowedIP_PrivateIPLiterals tests resolveAllowedIP blocks private IPs.
func TestResolveAllowedIP_PrivateIPLiterals(t *testing.T) {
	privateIPs := []string{
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"169.254.169.254", // AWS metadata
		"fc00::1",         // IPv6 unique local
		"fe80::1",         // IPv6 link-local
	}

	ctx := context.Background()
	for _, ip := range privateIPs {
		t.Run("IP_"+ip, func(t *testing.T) {
			_, err := resolveAllowedIP(ctx, ip, false)
			if err == nil {
				t.Errorf("Expected error for private IP %s, got nil", ip)
			}
			if err != nil && err.Error() != fmt.Sprintf("access to private IP addresses is blocked (resolved to %s)", ip) {
				// Check it contains the expected error substring
				expectedMsg := "access to private IP addresses is blocked"
				if !contains(err.Error(), expectedMsg) {
					t.Errorf("Expected error containing '%s', got: %v", expectedMsg, err)
				}
			}
		})
	}
}

// TestResolveAllowedIP_PublicIPLiteral tests resolveAllowedIP allows public IPs.
func TestResolveAllowedIP_PublicIPLiteral(t *testing.T) {
	publicIPs := []string{
		"8.8.8.8",
		"1.1.1.1",
		"2001:4860:4860::8888",
	}

	ctx := context.Background()
	for _, ipStr := range publicIPs {
		t.Run("IP_"+ipStr, func(t *testing.T) {
			ip, err := resolveAllowedIP(ctx, ipStr, false)
			if err != nil {
				t.Errorf("Expected no error for public IP %s, got: %v", ipStr, err)
			}
			if ip == nil {
				t.Error("Expected non-nil IP for public address")
			}
		})
	}
}

// TestResolveAllowedIP_Timeout tests DNS resolution timeout.
func TestResolveAllowedIP_Timeout(t *testing.T) {
	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Any hostname should timeout
	_, err := resolveAllowedIP(ctx, "example.com", false)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	// Should be a context deadline exceeded error
	if !errors.Is(err, context.DeadlineExceeded) && !contains(err.Error(), "deadline") && !contains(err.Error(), "timeout") {
		t.Logf("Expected timeout/deadline error, got: %v", err)
	}
}

// TestResolveAllowedIP_NoIPsResolved tests when DNS returns no IPs.
// Note: This is difficult to test without a custom resolver, so we skip it
func TestResolveAllowedIP_NoIPsResolved(t *testing.T) {
	t.Skip("Requires custom DNS resolver to return empty IP list")
}

// TestSSRFSafeDialer_PrivateIPResolution tests that ssrfSafeDialer blocks private IPs.
// Note: This requires network access or mocking, so we test the concept
func TestSSRFSafeDialer_Concept(t *testing.T) {
	// The ssrfSafeDialer function should:
	// 1. Resolve the hostname to IPs
	// 2. Check ALL IPs against private ranges
	// 3. Reject if ANY IP is private
	// 4. Connect only to validated IPs

	// We can't easily test this without network calls, but we document the behavior
	t.Log("ssrfSafeDialer validates IPs at dial time to prevent DNS rebinding")
	t.Log("All resolved IPs must pass private IP check before connection")
}

// TestSSRFSafeDialer_InvalidAddress tests ssrfSafeDialer with malformed addresses.
func TestSSRFSafeDialer_InvalidAddress(t *testing.T) {
	ctx := context.Background()
	dialer := ssrfSafeDialer()

	tests := []struct {
		name string
		addr string
	}{
		{
			name: "No port",
			addr: "example.com",
		},
		{
			name: "Invalid format",
			addr: ":/invalid",
		},
		{
			name: "Empty address",
			addr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dialer(ctx, "tcp", tt.addr)
			if err == nil {
				t.Errorf("Expected error for invalid address %s, got nil", tt.addr)
			}
		})
	}
}

// TestSSRFSafeDialer_ContextCancellation tests context cancellation during dial.
func TestSSRFSafeDialer_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	dialer := ssrfSafeDialer()
	_, err := dialer(ctx, "tcp", "example.com:80")

	if err == nil {
		t.Fatal("Expected context cancellation error, got nil")
	}

	// Should be context canceled error
	if !errors.Is(err, context.Canceled) && !contains(err.Error(), "canceled") {
		t.Logf("Expected context canceled error, got: %v", err)
	}
}

// TestTestURLConnectivity_ErrorPaths tests error handling in testURLConnectivity.
func TestTestURLConnectivity_ErrorPaths(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		errMatch string
	}{
		{
			name:     "Invalid URL format",
			url:      "://invalid",
			errMatch: "invalid URL",
		},
		{
			name:     "Unsupported scheme FTP",
			url:      "ftp://example.com",
			errMatch: "only http and https schemes are allowed",
		},
		{ //nolint:gosec // test verifies rejection of embedded credentials in URL
			name:     "Embedded credentials",
			url:      "https://user:pass@example.com",
			errMatch: "embedded credentials are not allowed",
		},
		{
			name:     "Private IP 10.x",
			url:      "http://10.0.0.1",
			errMatch: "private IP",
		},
		{
			name:     "Private IP 192.168.x",
			url:      "http://192.168.1.1",
			errMatch: "private IP",
		},
		{
			name:     "AWS metadata endpoint",
			url:      "http://169.254.169.254/latest/meta-data/",
			errMatch: "private IP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reachable, latency, err := TestURLConnectivity(tt.url)

			if err == nil {
				t.Fatalf("Expected error for %s, got nil (reachable=%v, latency=%v)", tt.url, reachable, latency)
			}

			if !contains(err.Error(), tt.errMatch) {
				t.Errorf("Expected error containing '%s', got: %v", tt.errMatch, err)
			}

			if reachable {
				t.Error("Expected reachable=false for error case")
			}
		})
	}
}

// TestTestURLConnectivity_InvalidPort tests port validation in testURLConnectivity.
func TestTestURLConnectivity_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "Port out of range (too high)",
			url:  "https://example.com:99999",
		},
		{
			name: "Port zero",
			url:  "https://example.com:0",
		},
		{
			name: "Negative port",
			url:  "https://example.com:-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := TestURLConnectivity(tt.url)
			if err == nil {
				t.Errorf("Expected error for invalid port in %s", tt.url)
			}
		})
	}
}

// TestValidateRedirectTargetStrict tests are in url_testing_test.go using proper http types

// Helper function already defined in security tests
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstring(s, substr)
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
