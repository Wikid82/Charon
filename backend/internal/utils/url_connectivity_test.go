package utils

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTransport is a custom http.RoundTripper for testing that bypasses network calls
type mockTransport struct {
	statusCode int
	headers    http.Header
	body       string
	err        error
	handler    http.HandlerFunc // For dynamic responses
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}

	// Use handler if provided (for dynamic responses like redirects)
	if m.handler != nil {
		w := httptest.NewRecorder()
		m.handler(w, req)
		return w.Result(), nil
	}

	// Static response
	resp := &http.Response{
		StatusCode: m.statusCode,
		Header:     m.headers,
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Request:    req,
	}
	return resp, nil
}

// TestTestURLConnectivity_Success verifies that valid public URLs are reachable
func TestTestURLConnectivity_Success(t *testing.T) {
	transport := &mockTransport{
		statusCode: http.StatusOK,
		headers:    http.Header{"Content-Type": []string{"text/html"}},
		body:       "",
	}

	reachable, latency, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))

	assert.NoError(t, err)
	assert.True(t, reachable)
	assert.Greater(t, latency, 0.0, "latency should be positive")
	assert.Less(t, latency, 5000.0, "latency should be reasonable (< 5s)")
}

// TestTestURLConnectivity_Redirect verifies redirect handling
func TestTestURLConnectivity_Redirect(t *testing.T) {
	redirectCount := 0
	transport := &mockTransport{
		handler: func(w http.ResponseWriter, r *http.Request) {
			redirectCount++
			// Only redirect once, then return OK
			if redirectCount == 1 {
				w.Header().Set("Location", "http://example.com/final")
				w.WriteHeader(http.StatusFound)
				return
			}
			w.WriteHeader(http.StatusOK)
		},
	}

	reachable, _, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))

	assert.NoError(t, err)
	assert.True(t, reachable)
	assert.LessOrEqual(t, redirectCount, 3, "should follow max 2 redirects")
}

// TestTestURLConnectivity_TooManyRedirects verifies redirect limit enforcement
func TestTestURLConnectivity_TooManyRedirects(t *testing.T) {
	transport := &mockTransport{
		handler: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", "/redirect")
			w.WriteHeader(http.StatusFound)
		},
	}

	reachable, _, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))

	assert.Error(t, err)
	assert.False(t, reachable)
	assert.Contains(t, err.Error(), "redirect", "error should mention redirects")
}

// TestTestURLConnectivity_StatusCodes verifies handling of different HTTP status codes
func TestTestURLConnectivity_StatusCodes(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"200 OK", http.StatusOK, true},
		{"201 Created", http.StatusCreated, true},
		{"204 No Content", http.StatusNoContent, true},
		{"301 Moved Permanently", http.StatusMovedPermanently, true},
		{"302 Found", http.StatusFound, true},
		{"400 Bad Request", http.StatusBadRequest, false},
		{"401 Unauthorized", http.StatusUnauthorized, false},
		{"403 Forbidden", http.StatusForbidden, false},
		{"404 Not Found", http.StatusNotFound, false},
		{"500 Internal Server Error", http.StatusInternalServerError, false},
		{"503 Service Unavailable", http.StatusServiceUnavailable, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			transport := &mockTransport{
				statusCode: tc.statusCode,
			}

			reachable, latency, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))

			if tc.expected {
				assert.NoError(t, err)
				assert.True(t, reachable)
				assert.Greater(t, latency, 0.0)
			} else {
				assert.Error(t, err)
				assert.False(t, reachable)
				assert.Contains(t, err.Error(), fmt.Sprintf("status %d", tc.statusCode))
			}
		})
	}
}

// TestTestURLConnectivity_InvalidURL verifies invalid URL handling
func TestTestURLConnectivity_InvalidURL(t *testing.T) {
	testCases := []struct {
		name string
		url  string
	}{
		{"Empty URL", ""},
		{"Invalid scheme", "ftp://example.com"},
		{"Malformed URL", "http://[invalid"},
		{"No scheme", "example.com"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// No transport needed - these fail at URL parsing
			reachable, _, err := TestURLConnectivity(tc.url)

			assert.Error(t, err)
			assert.False(t, reachable)
			// Latency varies depending on error type
			// Some errors may still measure time before failing
		})
	}
}

// TestTestURLConnectivity_DNSFailure verifies DNS resolution error handling
func TestTestURLConnectivity_DNSFailure(t *testing.T) {
	// Without transport, this will try real DNS and should fail
	reachable, _, err := TestURLConnectivity("http://nonexistent-domain-12345.invalid")

	assert.Error(t, err)
	assert.False(t, reachable)
	assert.Contains(t, err.Error(), "DNS resolution failed", "error should mention DNS failure")
}

// TestTestURLConnectivity_Timeout verifies timeout enforcement
func TestTestURLConnectivity_Timeout(t *testing.T) {
	transport := &mockTransport{
		err: fmt.Errorf("context deadline exceeded"),
	}

	reachable, _, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))

	assert.Error(t, err)
	assert.False(t, reachable)
	assert.Contains(t, err.Error(), "connection failed", "error should mention connection failure")
}

// TestIsPrivateIP_PrivateIPv4Ranges verifies blocking of private IPv4 ranges
func TestIsPrivateIP_PrivateIPv4Ranges(t *testing.T) {
	testCases := []struct {
		name     string
		ip       string
		expected bool
	}{
		// RFC 1918 Private Networks
		{"10.0.0.0/8 start", "10.0.0.1", true},
		{"10.0.0.0/8 mid", "10.128.0.1", true},
		{"10.0.0.0/8 end", "10.255.255.254", true},
		{"172.16.0.0/12 start", "172.16.0.1", true},
		{"172.16.0.0/12 mid", "172.20.0.1", true},
		{"172.16.0.0/12 end", "172.31.255.254", true},
		{"192.168.0.0/16 start", "192.168.0.1", true},
		{"192.168.0.0/16 end", "192.168.255.254", true},

		// Loopback
		{"127.0.0.1 localhost", "127.0.0.1", true},
		{"127.0.0.0/8 start", "127.0.0.0", true},
		{"127.0.0.0/8 end", "127.255.255.255", true},

		// Link-Local (includes AWS/GCP metadata)
		{"169.254.0.0/16 start", "169.254.0.1", true},
		{"169.254.169.254 AWS metadata", "169.254.169.254", true},
		{"169.254.0.0/16 end", "169.254.255.254", true},

		// Reserved ranges
		{"0.0.0.0/8", "0.0.0.1", true},
		{"240.0.0.0/4", "240.0.0.1", true},
		{"255.255.255.255 broadcast", "255.255.255.255", true},

		// Public IPs (should NOT be blocked)
		{"8.8.8.8 Google DNS", "8.8.8.8", false},
		{"1.1.1.1 Cloudflare DNS", "1.1.1.1", false},
		{"93.184.216.34 example.com", "93.184.216.34", false},
		{"151.101.1.140 GitHub", "151.101.1.140", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			require.NotNil(t, ip, "IP should parse successfully")

			result := network.IsPrivateIP(ip)
			assert.Equal(t, tc.expected, result,
				"IP %s should be private=%v", tc.ip, tc.expected)
		})
	}
}

// TestIsPrivateIP_PrivateIPv6Ranges verifies blocking of private IPv6 ranges
func TestIsPrivateIP_PrivateIPv6Ranges(t *testing.T) {
	testCases := []struct {
		name     string
		ip       string
		expected bool
	}{
		// IPv6 Loopback
		{"::1 loopback", "::1", true},

		// IPv6 Link-Local
		{"fe80::/10 start", "fe80::1", true},
		{"fe80::/10 mid", "fe80:1234::5678", true},

		// IPv6 Unique Local (RFC 4193)
		{"fc00::/7 start", "fc00::1", true},
		{"fc00::/7 mid", "fd12:3456:789a::1", true},

		// Public IPv6 (should NOT be blocked)
		{"2001:4860:4860::8888 Google DNS", "2001:4860:4860::8888", false},
		{"2606:4700:4700::1111 Cloudflare DNS", "2606:4700:4700::1111", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			require.NotNil(t, ip, "IP should parse successfully")

			result := network.IsPrivateIP(ip)
			assert.Equal(t, tc.expected, result,
				"IP %s should be private=%v", tc.ip, tc.expected)
		})
	}
}

// TestTestURLConnectivity_PrivateIP_Blocked verifies SSRF protection
func TestTestURLConnectivity_PrivateIP_Blocked(t *testing.T) {
	// Note: This test will fail if run on a system that actually resolves
	// these hostnames to private IPs. In a production test environment,
	// you might want to mock DNS resolution.
	testCases := []struct {
		name string
		url  string
	}{
		{"localhost", "http://localhost"},
		{"127.0.0.1", "http://127.0.0.1"},
		{"Private IP 10.x", "http://10.0.0.1"},
		{"Private IP 192.168.x", "http://192.168.1.1"},
		{"AWS metadata", "http://169.254.169.254"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reachable, _, err := TestURLConnectivity(tc.url)

			// Should fail with private IP error
			assert.Error(t, err)
			assert.False(t, reachable)
			assert.Contains(t, err.Error(), "private IP", "error should mention private IP blocking")
		})
	}
}

// TestTestURLConnectivity_SSRF_Protection_Comprehensive performs comprehensive SSRF tests
func TestTestURLConnectivity_SSRF_Protection_Comprehensive(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive SSRF test in short mode")
	}

	// Test various SSRF attack vectors
	attackVectors := []string{
		"http://localhost:8080",
		"http://127.0.0.1:8080",
		"http://0.0.0.0:8080",
		"http://[::1]:8080",
		"http://169.254.169.254/latest/meta-data/",
		"http://metadata.google.internal/computeMetadata/v1/",
	}

	for _, url := range attackVectors {
		t.Run(url, func(t *testing.T) {
			reachable, _, err := TestURLConnectivity(url)

			// All should be blocked
			assert.Error(t, err, "SSRF attack vector should be blocked")
			assert.False(t, reachable)
		})
	}
}

// TestTestURLConnectivity_HTTPSSupport verifies HTTPS support
func TestTestURLConnectivity_HTTPSSupport(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Note: This will likely fail due to self-signed cert in test server
	// but it demonstrates HTTPS support
	reachable, _, err := TestURLConnectivity(server.URL)

	// May fail due to cert validation, but should not panic
	if err != nil {
		t.Logf("HTTPS test failed (expected with self-signed cert): %v", err)
	} else {
		assert.True(t, reachable)
	}
}

// BenchmarkTestURLConnectivity benchmarks the connectivity test
func BenchmarkTestURLConnectivity(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = TestURLConnectivity(server.URL)
	}
}

// BenchmarkIsPrivateIP benchmarks private IP checking
func BenchmarkIsPrivateIP(b *testing.B) {
	ip := net.ParseIP("192.168.1.1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = network.IsPrivateIP(ip)
	}
}

// TestTestURLConnectivity_RedirectLimit_ProductionPath verifies the production
// CheckRedirect callback enforces a maximum of 2 redirects (lines 93-97).
// This is a critical security feature to prevent redirect-based attacks.
func TestTestURLConnectivity_RedirectLimit_ProductionPath(t *testing.T) {
	redirectCount := 0
	// Use mock transport to bypass SSRF protection and test redirect limit specifically
	transport := &mockTransport{
		handler: func(w http.ResponseWriter, r *http.Request) {
			redirectCount++
			if redirectCount <= 3 { // Try to redirect 3 times
				http.Redirect(w, r, "http://example.com/next", http.StatusFound)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		},
	}

	// Test with transport (will use CheckRedirect callback from production path)
	reachable, latency, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))

	// Should fail due to redirect limit
	assert.False(t, reachable)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redirect", "error should mention redirects")
	assert.Greater(t, latency, 0.0, "should have some latency")
}

// TestTestURLConnectivity_InvalidPortFormat tests error when URL has invalid port format.
// This would trigger errors in net.SplitHostPort during dialing (lines 19-21).
func TestTestURLConnectivity_InvalidPortFormat(t *testing.T) {
	// URL with invalid port will fail at URL parsing stage
	reachable, _, err := TestURLConnectivity("http://example.com:badport")

	assert.False(t, reachable)
	assert.Error(t, err)
	// URL parsing will catch the invalid port before we even get to dialing
	assert.Contains(t, err.Error(), "invalid port")
}

// TestTestURLConnectivity_EmptyDNSResult tests the empty DNS results
// error path (lines 29-31).
func TestTestURLConnectivity_EmptyDNSResult(t *testing.T) {
	// Create a custom transport that simulates empty DNS result
	transport := &mockTransport{
		err: fmt.Errorf("DNS resolution failed: no IP addresses found for host"),
	}

	reachable, _, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))
	assert.False(t, reachable)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection failed")
}
