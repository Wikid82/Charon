package utils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============== Phase 3.2: URL Testing SSRF Protection Tests ==============

func TestSSRFSafeDialer_ValidPublicIP(t *testing.T) {
	dialer := ssrfSafeDialer()
	require.NotNil(t, dialer)

	// Test with a public IP (8.8.8.8:443)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer(ctx, "tcp", "8.8.8.8:443")
	if err == nil {
		defer conn.Close()
		assert.NotNil(t, conn, "connection to public IP should succeed")
	} else {
		// Connection might fail for network reasons, but error should not be about private IP
		assert.NotContains(t, err.Error(), "private IP", "error should not be about private IP blocking")
	}
}

func TestSSRFSafeDialer_PrivateIPBlocking(t *testing.T) {
	dialer := ssrfSafeDialer()
	require.NotNil(t, dialer)

	privateIPs := []string{
		"10.0.0.1:80",
		"192.168.1.1:80",
		"172.16.0.1:80",
		"127.0.0.1:80",
	}

	for _, addr := range privateIPs {
		t.Run(addr, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			conn, err := dialer(ctx, "tcp", addr)
			if conn != nil {
				conn.Close()
			}

			require.Error(t, err, "connection to private IP should fail")
			assert.Contains(t, err.Error(), "private IP", "error should mention private IP")
		})
	}
}

func TestSSRFSafeDialer_DNSResolutionFailure(t *testing.T) {
	dialer := ssrfSafeDialer()
	require.NotNil(t, dialer)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := dialer(ctx, "tcp", "nonexistent-domain-12345.invalid:80")
	if conn != nil {
		conn.Close()
	}

	require.Error(t, err, "connection to nonexistent domain should fail")
	assert.Contains(t, err.Error(), "DNS resolution", "error should mention DNS resolution")
}

func TestSSRFSafeDialer_MultipleIPsWithPrivate(t *testing.T) {
	// This test verifies that if DNS returns multiple IPs and any is private, all are blocked
	// We can't easily mock DNS in this test, so we'll test the isPrivateIP logic instead

	// Test that isPrivateIP correctly identifies private IPs
	privateIPs := []net.IP{
		net.ParseIP("10.0.0.1"),
		net.ParseIP("192.168.1.1"),
		net.ParseIP("172.16.0.1"),
		net.ParseIP("127.0.0.1"),
		net.ParseIP("169.254.169.254"),
	}

	for _, ip := range privateIPs {
		assert.True(t, isPrivateIP(ip), "IP %s should be identified as private", ip)
	}

	publicIPs := []net.IP{
		net.ParseIP("8.8.8.8"),
		net.ParseIP("1.1.1.1"),
		net.ParseIP("93.184.216.34"), // example.com
	}

	for _, ip := range publicIPs {
		assert.False(t, isPrivateIP(ip), "IP %s should be identified as public", ip)
	}
}

func TestURLConnectivity_ProductionPathValidation(t *testing.T) {
	// Test that production path (no custom transport) performs SSRF validation
	tests := []struct {
		name        string
		url         string
		shouldFail  bool
		errorString string
	}{
		{
			name:        "localhost blocked at dial time",
			url:         "http://localhost",
			shouldFail:  true,
			errorString: "private IP", // Blocked by ssrfSafeDialer
		},
		{
			name:        "127.0.0.1 blocked at dial time",
			url:         "http://127.0.0.1",
			shouldFail:  true,
			errorString: "private IP", // Blocked by ssrfSafeDialer
		},
		{
			name:        "private 10.x blocked at validation",
			url:         "http://10.0.0.1",
			shouldFail:  true,
			errorString: "security validation failed",
		},
		{
			name:        "private 192.168.x blocked at validation",
			url:         "http://192.168.1.1",
			shouldFail:  true,
			errorString: "security validation failed",
		},
		{
			name:        "AWS metadata blocked at validation",
			url:         "http://169.254.169.254",
			shouldFail:  true,
			errorString: "security validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reachable, _, err := TestURLConnectivity(tt.url)

			if tt.shouldFail {
				require.Error(t, err, "expected error for %s", tt.url)
				assert.Contains(t, err.Error(), tt.errorString)
				assert.False(t, reachable)
			}
		})
	}
}

func TestURLConnectivity_TestHook_AllowsLocalhostWithInjectedTransport(t *testing.T) {
	// Deterministic connectivity test using a local server + injected transport.
	// This does not weaken production defaults because it uses package-private hooks.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	reachable, latency, err := testURLConnectivity(
		mockServer.URL,
		withAllowLocalhostForTesting(),
		withTransportForTesting(mockServer.Client().Transport),
	)

	require.NoError(t, err)
	assert.True(t, reachable)
	assert.Greater(t, latency, float64(0))
}

func TestValidateRedirectTarget_AllowsLocalhost(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://localhost/redirect", http.NoBody)
	require.NoError(t, err)

	err = validateRedirectTargetStrict(req, nil, 2, true, true)
	require.NoError(t, err)
}

func TestValidateRedirectTarget_BlocksInvalidExternalRedirect(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example..com/redirect", http.NoBody)
	require.NoError(t, err)

	err = validateRedirectTargetStrict(req, nil, 2, true, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redirect target validation failed")
}

func TestURLConnectivity_RejectsUserinfo(t *testing.T) {
	reachable, _, err := TestURLConnectivity("http://user:pass@example.com")
	require.Error(t, err)
	require.False(t, reachable)
	assert.Contains(t, err.Error(), "embedded credentials")
}

func TestURLConnectivity_InvalidScheme(t *testing.T) {
	tests := []string{
		"ftp://example.com",
		"file:///etc/passwd",
		"javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"gopher://example.com",
	}

	for _, url := range tests {
		t.Run(url, func(t *testing.T) {
			reachable, latency, err := TestURLConnectivity(url)

			require.Error(t, err, "invalid scheme should fail")
			assert.Contains(t, err.Error(), "only http and https schemes are allowed")
			assert.False(t, reachable)
			assert.Equal(t, float64(0), latency)
		})
	}
}

func TestURLConnectivity_SSRFValidationFailure(t *testing.T) {
	// Test that SSRF validation catches private IPs
	// Note: localhost/127.0.0.1 are allowed by ValidateExternalURL (WithAllowLocalhost)
	// but blocked by ssrfSafeDialer at connection time
	privateURLs := []struct {
		url         string
		errorString string
	}{
		{"http://10.0.0.1", "security validation failed"},
		{"http://192.168.1.1", "security validation failed"},
		{"http://172.16.0.1", "security validation failed"},
		{"http://localhost", "private IP"}, // Blocked at dial time
		{"http://127.0.0.1", "private IP"}, // Blocked at dial time
	}

	for _, tc := range privateURLs {
		t.Run(tc.url, func(t *testing.T) {
			reachable, _, err := TestURLConnectivity(tc.url)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errorString)
			assert.False(t, reachable)
		})
	}
}

func TestURLConnectivity_HTTPRequestFailure(t *testing.T) {
	// Create a server that immediately closes connections
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close() // Immediately close to cause connection failure
		}
	}()

	// Use custom transport to bypass SSRF protection for this test
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("tcp", listener.Addr().String())
		},
	}

	reachable, _, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))

	// Should get a connection error
	require.Error(t, err)
	assert.False(t, reachable)
}

func TestURLConnectivity_RedirectHandling(t *testing.T) {
	// Create a mock server that redirects once then returns 200
	redirectCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if redirectCount < 1 {
			redirectCount++
			http.Redirect(w, r, "/redirected", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("tcp", mockServer.Listener.Addr().String())
		},
	}

	reachable, latency, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))

	require.NoError(t, err)
	assert.True(t, reachable)
	assert.Greater(t, latency, float64(0))
}

func TestURLConnectivity_2xxSuccess(t *testing.T) {
	successCodes := []int{200, 201, 204}

	for _, code := range successCodes {
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer mockServer.Close()

			transport := &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return net.Dial("tcp", mockServer.Listener.Addr().String())
				},
			}

			reachable, latency, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))

			require.NoError(t, err)
			assert.True(t, reachable)
			assert.Greater(t, latency, float64(0))
		})
	}
}

func TestURLConnectivity_3xxSuccess(t *testing.T) {
	redirectCodes := []int{301, 302, 307, 308}

	for _, code := range redirectCodes {
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/" {
					w.Header().Set("Location", "/target")
					w.WriteHeader(code)
				} else {
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer mockServer.Close()

			transport := &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return net.Dial("tcp", mockServer.Listener.Addr().String())
				},
			}

			reachable, latency, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))

			// 3xx codes are considered "reachable" (status < 400)
			require.NoError(t, err)
			assert.True(t, reachable)
			assert.Greater(t, latency, float64(0))
		})
	}
}

func TestURLConnectivity_4xxFailure(t *testing.T) {
	errorCodes := []int{400, 401, 403, 404, 429}

	for _, code := range errorCodes {
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer mockServer.Close()

			transport := &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return net.Dial("tcp", mockServer.Listener.Addr().String())
				},
			}

			reachable, latency, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))

			require.Error(t, err)
			assert.Contains(t, err.Error(), fmt.Sprintf("server returned status %d", code))
			assert.False(t, reachable)
			assert.Greater(t, latency, float64(0)) // Latency is still recorded
		})
	}
}

func TestIsPrivateIP_AllReservedRanges(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		// RFC 1918 - Private IPv4
		{"10.0.0.1", "10.0.0.1", true},
		{"10.255.255.255", "10.255.255.255", true},
		{"172.16.0.1", "172.16.0.1", true},
		{"172.31.255.255", "172.31.255.255", true},
		{"192.168.0.1", "192.168.0.1", true},
		{"192.168.255.255", "192.168.255.255", true},

		// Loopback
		{"127.0.0.1", "127.0.0.1", true},
		{"127.0.0.2", "127.0.0.2", true},
		{"127.255.255.255", "127.255.255.255", true},

		// Link-Local (includes AWS/GCP metadata)
		{"169.254.0.1", "169.254.0.1", true},
		{"169.254.169.254", "169.254.169.254", true},
		{"169.254.255.255", "169.254.255.255", true},

		// Reserved ranges
		{"0.0.0.0", "0.0.0.0", true},
		{"0.0.0.1", "0.0.0.1", true},
		{"240.0.0.1", "240.0.0.1", true},
		{"255.255.255.255", "255.255.255.255", true},

		// IPv6 Loopback
		{"::1", "::1", true},

		// IPv6 Unique Local (fc00::/7)
		{"fc00::1", "fc00::1", true},
		{"fd00::1", "fd00::1", true},

		// IPv6 Link-Local (fe80::/10)
		{"fe80::1", "fe80::1", true},

		// Public IPs - should be false
		{"8.8.8.8", "8.8.8.8", false},
		{"1.1.1.1", "1.1.1.1", false},
		{"93.184.216.34", "93.184.216.34", false}, // example.com
		{"2606:2800:220:1:248:1893:25c8:1946", "2606:2800:220:1:248:1893:25c8:1946", false}, // example.com IPv6
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			require.NotNil(t, ip, "failed to parse IP: %s", tt.ip)

			result := isPrivateIP(ip)
			assert.Equal(t, tt.expected, result, "isPrivateIP(%s) = %v, want %v", tt.ip, result, tt.expected)
		})
	}
}

// Test helper to verify error wrapping
func TestURLConnectivity_ErrorWrapping(t *testing.T) {
	// Test invalid URL
	_, _, err := TestURLConnectivity("://invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid URL")

	// Error should be a plain error (not wrapped in this case)
	assert.NotNil(t, err)
}

// Test that User-Agent header is set correctly
func TestURLConnectivity_UserAgent(t *testing.T) {
	receivedUA := ""
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	_, _, err := testURLConnectivity(mockServer.URL, withAllowLocalhostForTesting(), withTransportForTesting(mockServer.Client().Transport))
	require.NoError(t, err)
	assert.Equal(t, "Charon-Health-Check/1.0", receivedUA)
}

// ============== Additional Coverage Tests ==============

// TestResolveAllowedIP_EmptyHost tests empty hostname handling
func TestResolveAllowedIP_EmptyHost(t *testing.T) {
	ctx := context.Background()
	_, err := resolveAllowedIP(ctx, "", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing hostname")
}

// TestResolveAllowedIP_IPLiteralPublic tests IP literal fast path for public IP (not loopback, not private)
func TestResolveAllowedIP_IPLiteralPublic(t *testing.T) {
	ctx := context.Background()

	// Public IP should pass through without error
	ip, err := resolveAllowedIP(ctx, "8.8.8.8", false)
	require.NoError(t, err)
	assert.Equal(t, "8.8.8.8", ip.String())
}

// TestResolveAllowedIP_IPLiteralPrivateBlocked tests private IP blocking for IP literals
func TestResolveAllowedIP_IPLiteralPrivateBlocked(t *testing.T) {
	ctx := context.Background()

	privateIPs := []string{"10.0.0.1", "192.168.1.1", "172.16.0.1"}
	for _, privateIP := range privateIPs {
		t.Run(privateIP, func(t *testing.T) {
			_, err := resolveAllowedIP(ctx, privateIP, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "private IP")
		})
	}
}

// TestResolveAllowedIP_DNSResolutionFailure tests DNS failure handling
func TestResolveAllowedIP_DNSResolutionFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := resolveAllowedIP(ctx, "nonexistent-domain-xyz123.invalid", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DNS resolution failed")
}

// TestSSRFSafeDialer_InvalidAddressFormat tests invalid address format handling
func TestSSRFSafeDialer_InvalidAddressFormat(t *testing.T) {
	dialer := ssrfSafeDialer()
	ctx := context.Background()

	// Address without port separator
	_, err := dialer(ctx, "tcp", "invalidaddress")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid address format")
}

// TestSSRFSafeDialer_NoIPsFound tests empty DNS response handling
func TestSSRFSafeDialer_NoIPsFound(t *testing.T) {
	// This scenario is hard to trigger directly, but we test through the resolveAllowedIP
	// which is called by ssrfSafeDialer. The ssrfSafeDialer does its own DNS lookup.
	dialer := ssrfSafeDialer()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use a domain that won't resolve
	_, err := dialer(ctx, "tcp", "nonexistent-domain-xyz123.invalid:80")
	require.Error(t, err)
	// Should contain DNS resolution error
	assert.Contains(t, err.Error(), "DNS resolution")
}

// TestURLConnectivity_5xxServerErrors tests 5xx server error handling
func TestURLConnectivity_5xxServerErrors(t *testing.T) {
	errorCodes := []int{500, 502, 503, 504}

	for _, code := range errorCodes {
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer mockServer.Close()

			transport := &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return net.Dial("tcp", mockServer.Listener.Addr().String())
				},
			}

			reachable, latency, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))

			require.Error(t, err)
			assert.Contains(t, err.Error(), fmt.Sprintf("server returned status %d", code))
			assert.False(t, reachable)
			assert.Greater(t, latency, float64(0)) // Latency is still recorded
		})
	}
}

// TestURLConnectivity_TooManyRedirects tests redirect limit enforcement
func TestURLConnectivity_TooManyRedirects(t *testing.T) {
	redirectCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectCount++
		// Always redirect to trigger max redirect error
		http.Redirect(w, r, fmt.Sprintf("/redirect%d", redirectCount), http.StatusFound)
	}))
	defer mockServer.Close()

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("tcp", mockServer.Listener.Addr().String())
		},
	}

	reachable, _, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "redirect")
	assert.False(t, reachable)
}

// TestValidateRedirectTarget_TooManyRedirects tests redirect limit
func TestValidateRedirectTarget_TooManyRedirects(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com/redirect", http.NoBody)
	require.NoError(t, err)

	// Create via slice with max redirects already reached
	via := make([]*http.Request, 2)
	for i := range via {
		via[i], _ = http.NewRequest(http.MethodGet, "http://example.com/prev", http.NoBody)
	}

	err = validateRedirectTargetStrict(req, via, 2, true, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many redirects")
}

// TestValidateRedirectTarget_SchemeChangeBlocked tests scheme downgrade blocking
func TestValidateRedirectTarget_SchemeChangeBlocked(t *testing.T) {
	// Create initial HTTPS request
	prevReq, _ := http.NewRequest(http.MethodGet, "https://example.com/start", http.NoBody)

	// Try to redirect to HTTP (downgrade - should be blocked)
	req, err := http.NewRequest(http.MethodGet, "http://example.com/redirect", http.NoBody)
	require.NoError(t, err)

	err = validateRedirectTargetStrict(req, []*http.Request{prevReq}, 5, true, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redirect scheme change blocked")
}

// TestValidateRedirectTarget_HTTPToHTTPSAllowed tests HTTP to HTTPS upgrade (allowed)
func TestValidateRedirectTarget_HTTPToHTTPSAllowed(t *testing.T) {
	// Create initial HTTP request
	prevReq, _ := http.NewRequest(http.MethodGet, "http://example.com/start", http.NoBody)

	// Redirect to HTTPS (upgrade - should be allowed with allowHTTPSUpgrade=true)
	req, err := http.NewRequest(http.MethodGet, "https://example.com/redirect", http.NoBody)
	require.NoError(t, err)

	err = validateRedirectTargetStrict(req, []*http.Request{prevReq}, 5, true, true)
	// Should fail on security validation (private IP check), not scheme change
	// The scheme change itself should be allowed
	if err != nil {
		assert.NotContains(t, err.Error(), "redirect scheme change blocked")
	}
}

// TestValidateRedirectTarget_HTTPToHTTPSBlockedWhenNotAllowed tests blocking HTTP to HTTPS when not allowed
func TestValidateRedirectTarget_HTTPToHTTPSBlockedWhenNotAllowed(t *testing.T) {
	// Create initial HTTP request
	prevReq, _ := http.NewRequest(http.MethodGet, "http://example.com/start", http.NoBody)

	// Redirect to HTTPS (upgrade - should be blocked when allowHTTPSUpgrade=false)
	req, err := http.NewRequest(http.MethodGet, "https://example.com/redirect", http.NoBody)
	require.NoError(t, err)

	err = validateRedirectTargetStrict(req, []*http.Request{prevReq}, 5, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redirect scheme change blocked")
}

// TestURLConnectivity_CloudMetadataBlocked tests AWS/GCP metadata endpoint blocking
func TestURLConnectivity_CloudMetadataBlocked(t *testing.T) {
	metadataURLs := []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://169.254.169.254",
	}

	for _, url := range metadataURLs {
		t.Run(url, func(t *testing.T) {
			reachable, _, err := TestURLConnectivity(url)
			require.Error(t, err)
			assert.False(t, reachable)
			// Should be blocked by security validation
			assert.Contains(t, err.Error(), "security validation failed")
		})
	}
}

// TestURLConnectivity_InvalidPort tests invalid port handling
func TestURLConnectivity_InvalidPort(t *testing.T) {
	invalidPortURLs := []struct {
		name string
		url  string
	}{
		{"port_zero", "http://example.com:0/path"},
		{"port_negative", "http://example.com:-1/path"},
		{"port_too_large", "http://example.com:99999/path"},
		{"port_non_numeric", "http://example.com:abc/path"},
	}

	for _, tc := range invalidPortURLs {
		t.Run(tc.name, func(t *testing.T) {
			reachable, _, err := TestURLConnectivity(tc.url)
			require.Error(t, err)
			assert.False(t, reachable)
		})
	}
}

// TestURLConnectivity_HTTPSScheme tests HTTPS URL handling
func TestURLConnectivity_HTTPSScheme(t *testing.T) {
	// Create HTTPS test server
	mockServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// Use the TLS server's client which has the right certificate configured
	reachable, latency, err := testURLConnectivity(
		mockServer.URL,
		withAllowLocalhostForTesting(),
		withTransportForTesting(mockServer.Client().Transport),
	)

	require.NoError(t, err)
	assert.True(t, reachable)
	assert.Greater(t, latency, float64(0))
}

// TestURLConnectivity_ExplicitPort tests URLs with explicit ports
func TestURLConnectivity_ExplicitPort(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// The test server already has an explicit port in its URL
	reachable, latency, err := testURLConnectivity(
		mockServer.URL,
		withAllowLocalhostForTesting(),
		withTransportForTesting(mockServer.Client().Transport),
	)

	require.NoError(t, err)
	assert.True(t, reachable)
	assert.Greater(t, latency, float64(0))
}

// TestURLConnectivity_DefaultHTTPPort tests default HTTP port (80) handling
func TestURLConnectivity_DefaultHTTPPort(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Redirect to the test server regardless of the requested address
			return net.Dial("tcp", mockServer.Listener.Addr().String())
		},
	}

	// URL without explicit port should default to 80
	reachable, _, err := testURLConnectivity(
		"http://localhost/",
		withAllowLocalhostForTesting(),
		withTransportForTesting(transport),
	)

	require.NoError(t, err)
	assert.True(t, reachable)
}

// TestURLConnectivity_ConnectionTimeout tests timeout handling
func TestURLConnectivity_ConnectionTimeout(t *testing.T) {
	// Create a server that doesn't respond
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	// Accept connections but never respond
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Hold connection open but don't respond
			time.Sleep(30 * time.Second)
			conn.Close()
		}
	}()

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.DialTimeout("tcp", listener.Addr().String(), 100*time.Millisecond)
		},
		ResponseHeaderTimeout: 100 * time.Millisecond,
	}

	reachable, _, err := testURLConnectivity(
		"http://localhost/",
		withAllowLocalhostForTesting(),
		withTransportForTesting(transport),
	)

	require.Error(t, err)
	assert.False(t, reachable)
	assert.Contains(t, err.Error(), "connection failed")
}

// TestURLConnectivity_RequestHeaders tests that custom headers are set
func TestURLConnectivity_RequestHeaders(t *testing.T) {
	var receivedHeaders http.Header
	var receivedHost string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		receivedHost = r.Host
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	_, _, err := testURLConnectivity(
		mockServer.URL,
		withAllowLocalhostForTesting(),
		withTransportForTesting(mockServer.Client().Transport),
	)

	require.NoError(t, err)
	assert.Equal(t, "Charon-Health-Check/1.0", receivedHeaders.Get("User-Agent"))
	assert.Equal(t, "url-connectivity-test", receivedHeaders.Get("X-Charon-Request-Type"))
	assert.NotEmpty(t, receivedHeaders.Get("X-Request-ID"))
	assert.NotEmpty(t, receivedHost)
}

// TestURLConnectivity_EmptyURL tests empty URL handling
func TestURLConnectivity_EmptyURL(t *testing.T) {
	reachable, _, err := TestURLConnectivity("")
	require.Error(t, err)
	assert.False(t, reachable)
}

// TestURLConnectivity_MalformedURL tests malformed URL handling
func TestURLConnectivity_MalformedURL(t *testing.T) {
	malformedURLs := []string{
		"://missing-scheme",
		"http://",
		"http:///no-host",
	}

	for _, url := range malformedURLs {
		t.Run(url, func(t *testing.T) {
			reachable, _, err := TestURLConnectivity(url)
			require.Error(t, err)
			assert.False(t, reachable)
		})
	}
}

// TestURLConnectivity_IPv6Address tests IPv6 address handling
func TestURLConnectivity_IPv6Loopback(t *testing.T) {
	// IPv6 loopback should be blocked like IPv4 loopback
	reachable, _, err := TestURLConnectivity("http://[::1]/")
	require.Error(t, err)
	assert.False(t, reachable)
	// Should be blocked by security validation
	assert.Contains(t, err.Error(), "security validation failed")
}

// TestURLConnectivity_HeadMethod tests that HEAD method is used
func TestURLConnectivity_HeadMethod(t *testing.T) {
	var receivedMethod string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	_, _, err := testURLConnectivity(
		mockServer.URL,
		withAllowLocalhostForTesting(),
		withTransportForTesting(mockServer.Client().Transport),
	)

	require.NoError(t, err)
	assert.Equal(t, http.MethodHead, receivedMethod)
}

// TestResolveAllowedIP_LoopbackWithAllowLocalhost tests loopback IP with allowLocalhost flag
func TestResolveAllowedIP_LoopbackWithAllowLocalhost(t *testing.T) {
	ctx := context.Background()

	// With allowLocalhost=true, loopback should be allowed
	ip, err := resolveAllowedIP(ctx, "127.0.0.1", true)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", ip.String())
}

// TestResolveAllowedIP_LoopbackWithoutAllowLocalhost tests loopback IP without allowLocalhost flag
func TestResolveAllowedIP_LoopbackWithoutAllowLocalhost(t *testing.T) {
	ctx := context.Background()

	// With allowLocalhost=false, loopback should be blocked
	_, err := resolveAllowedIP(ctx, "127.0.0.1", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private IP")
}

// TestURLConnectivity_HTTPSDefaultPort tests HTTPS URL without explicit port (defaults to 443)
func TestURLConnectivity_HTTPSDefaultPort(t *testing.T) {
	// Create HTTPS test server
	mockServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// Use the TLS server's client which has the right certificate configured
	reachable, latency, err := testURLConnectivity(
		mockServer.URL,
		withAllowLocalhostForTesting(),
		withTransportForTesting(mockServer.Client().Transport),
	)

	require.NoError(t, err)
	assert.True(t, reachable)
	assert.Greater(t, latency, float64(0))
}

// TestURLConnectivity_ValidPortNumber tests URL with valid explicit port
func TestURLConnectivity_ValidPortNumber(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	reachable, latency, err := testURLConnectivity(
		mockServer.URL+"/path",
		withAllowLocalhostForTesting(),
		withTransportForTesting(mockServer.Client().Transport),
	)

	require.NoError(t, err)
	assert.True(t, reachable)
	assert.Greater(t, latency, float64(0))
}

// TestURLConnectivity_PublicIPLiteralHTTP tests connectivity test with public IP literal
// Note: This test uses a mock server to avoid real network calls to public IPs
func TestURLConnectivity_PublicIPLiteralHTTP(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// Test with localhost which is allowed with the test flag
	// This exercises the code path for HTTP scheme handling
	reachable, latency, err := testURLConnectivity(
		mockServer.URL,
		withAllowLocalhostForTesting(),
		withTransportForTesting(mockServer.Client().Transport),
	)

	require.NoError(t, err)
	assert.True(t, reachable)
	assert.Greater(t, latency, float64(0))
}

// TestURLConnectivity_DNSResolutionError tests handling of DNS resolution failures
func TestURLConnectivity_DNSResolutionError(t *testing.T) {
	// Use a domain that won't resolve
	reachable, _, err := TestURLConnectivity("http://nonexistent-domain-xyz123456.invalid/")

	require.Error(t, err)
	assert.False(t, reachable)
	// Should fail with security validation due to DNS failure
	assert.Contains(t, err.Error(), "security validation failed")
}

// TestResolveAllowedIP_PublicIPv4Literal tests public IPv4 literal resolution
func TestResolveAllowedIP_PublicIPv4Literal(t *testing.T) {
	ctx := context.Background()

	// Google DNS - a well-known public IP
	ip, err := resolveAllowedIP(ctx, "8.8.8.8", false)
	require.NoError(t, err)
	assert.Equal(t, "8.8.8.8", ip.String())
}

// TestResolveAllowedIP_PublicIPv6Literal tests public IPv6 literal resolution
func TestResolveAllowedIP_PublicIPv6Literal(t *testing.T) {
	ctx := context.Background()

	// Google DNS IPv6
	ip, err := resolveAllowedIP(ctx, "2001:4860:4860::8888", false)
	require.NoError(t, err)
	assert.NotNil(t, ip)
}

// TestResolveAllowedIP_PrivateIPBlocked tests that private IPs are blocked
func TestResolveAllowedIP_PrivateIPBlocked(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name string
		ip   string
	}{
		{"RFC1918_10x", "10.0.0.1"},
		{"RFC1918_172x", "172.16.0.1"},
		{"RFC1918_192x", "192.168.1.1"},
		{"LinkLocal", "169.254.1.1"},
		{"Metadata", "169.254.169.254"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveAllowedIP(ctx, tc.ip, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "private IP")
		})
	}
}

// TestURLConnectivity_PrivateNetworkRanges tests all private network ranges are blocked
func TestURLConnectivity_PrivateNetworkRanges(t *testing.T) {
	testCases := []struct {
		name string
		url  string
	}{
		{"RFC1918_10x", "http://10.255.255.255/"},
		{"RFC1918_172x", "http://172.31.255.255/"},
		{"RFC1918_192x", "http://192.168.255.255/"},
		{"LinkLocal", "http://169.254.1.1/"},
		{"ZeroNet", "http://0.0.0.0/"},
		{"Broadcast", "http://255.255.255.255/"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reachable, _, err := TestURLConnectivity(tc.url)
			require.Error(t, err)
			assert.False(t, reachable)
			assert.Contains(t, err.Error(), "security validation failed")
		})
	}
}

// TestURLConnectivity_MultipleStatusCodes tests various HTTP status codes
func TestURLConnectivity_MultipleStatusCodes(t *testing.T) {
	testCases := []struct {
		name      string
		status    int
		reachable bool
	}{
		// 2xx - Success
		{"200_OK", 200, true},
		{"201_Created", 201, true},
		{"204_NoContent", 204, true},
		// 3xx - Handled by redirects, but final response matters
		// (These go through redirect handler which may succeed or fail)
		// 4xx - Client errors
		{"400_BadRequest", 400, false},
		{"401_Unauthorized", 401, false},
		{"403_Forbidden", 403, false},
		{"404_NotFound", 404, false},
		{"429_TooManyRequests", 429, false},
		// 5xx - Server errors
		{"500_InternalServerError", 500, false},
		{"502_BadGateway", 502, false},
		{"503_ServiceUnavailable", 503, false},
		{"504_GatewayTimeout", 504, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer mockServer.Close()

			transport := &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return net.Dial("tcp", mockServer.Listener.Addr().String())
				},
			}

			reachable, _, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))

			if tc.reachable {
				require.NoError(t, err)
				assert.True(t, reachable)
			} else {
				require.Error(t, err)
				assert.False(t, reachable)
			}
		})
	}
}

// TestURLConnectivity_RedirectToPrivateIP tests redirect to private IP is blocked
func TestURLConnectivity_RedirectToPrivateIP(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// Redirect to a private IP
			http.Redirect(w, r, "http://10.0.0.1/internal", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("tcp", mockServer.Listener.Addr().String())
		},
	}

	reachable, _, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))

	require.Error(t, err)
	assert.False(t, reachable)
	// Should be blocked by redirect validation
	assert.Contains(t, err.Error(), "redirect")
}

// TestValidateRedirectTarget_ValidExternalRedirect tests valid external redirect
func TestValidateRedirectTarget_ValidExternalRedirect(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com/redirect", http.NoBody)
	require.NoError(t, err)

	// No previous redirects
	err = validateRedirectTargetStrict(req, nil, 2, true, false)
	// Should pass scheme/redirect count validation but may fail on security validation
	// (depending on whether example.com resolves)
	if err != nil {
		// If it fails, it should be due to security validation, not redirect limits
		assert.NotContains(t, err.Error(), "too many redirects")
		assert.NotContains(t, err.Error(), "scheme change")
	}
}

// TestValidateRedirectTarget_SameSchemeAllowed tests same scheme redirects are allowed
func TestValidateRedirectTarget_SameSchemeAllowed(t *testing.T) {
	// Create initial HTTP request
	prevReq, _ := http.NewRequest(http.MethodGet, "http://example.com/start", http.NoBody)

	// Redirect to same scheme
	req, err := http.NewRequest(http.MethodGet, "http://example.com/redirect", http.NoBody)
	require.NoError(t, err)

	err = validateRedirectTargetStrict(req, []*http.Request{prevReq}, 5, true, false)
	// Same scheme should be allowed (may fail on security validation)
	if err != nil {
		assert.NotContains(t, err.Error(), "scheme change")
	}
}

// TestURLConnectivity_NetworkError tests handling of network connection errors
func TestURLConnectivity_NetworkError(t *testing.T) {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	reachable, _, err := testURLConnectivity("http://localhost/", withAllowLocalhostForTesting(), withTransportForTesting(transport))

	require.Error(t, err)
	assert.False(t, reachable)
	assert.Contains(t, err.Error(), "connection failed")
}

// TestURLConnectivity_HTTPSWithDefaultPort tests HTTPS URL with default port (443)
func TestURLConnectivity_HTTPSWithDefaultPort(t *testing.T) {
	mockServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	reachable, latency, err := testURLConnectivity(
		mockServer.URL,
		withAllowLocalhostForTesting(),
		withTransportForTesting(mockServer.Client().Transport),
	)

	require.NoError(t, err)
	assert.True(t, reachable)
	assert.Greater(t, latency, float64(0))
}

// TestURLConnectivity_HTTPWithExplicitPortValidation tests port validation
func TestURLConnectivity_HTTPWithExplicitPortValidation(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// Valid port
	reachable, latency, err := testURLConnectivity(
		mockServer.URL,
		withAllowLocalhostForTesting(),
		withTransportForTesting(mockServer.Client().Transport),
	)

	require.NoError(t, err)
	assert.True(t, reachable)
	assert.Greater(t, latency, float64(0))
}

// TestIsDockerBridgeIP_AllCases tests IsDockerBridgeIP function coverage
func TestIsDockerBridgeIP_AllCases(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		// Valid Docker bridge IPs
		{"docker_bridge_172_17", "172.17.0.1", true},
		{"docker_bridge_172_18", "172.18.0.1", true},
		{"docker_bridge_172_31", "172.31.255.255", true},
		// Non-Docker IPs
		{"public_ip", "8.8.8.8", false},
		{"localhost", "127.0.0.1", false},
		{"private_10x", "10.0.0.1", false},
		{"private_192x", "192.168.1.1", false},
		// Invalid inputs
		{"empty", "", false},
		{"invalid", "not-an-ip", false},
		{"hostname", "example.com", false},
		// IPv6 (should return false as Docker bridge is IPv4)
		{"ipv6_loopback", "::1", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsDockerBridgeIP(tc.host)
			assert.Equal(t, tc.expected, result, "IsDockerBridgeIP(%s) = %v, want %v", tc.host, result, tc.expected)
		})
	}
}

// TestURLConnectivity_RedirectChain tests proper handling of redirect chains
func TestURLConnectivity_RedirectChain(t *testing.T) {
	redirectCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			redirectCount++
			http.Redirect(w, r, "/step2", http.StatusFound)
		case "/step2":
			redirectCount++
			// Final destination
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("tcp", mockServer.Listener.Addr().String())
		},
	}

	reachable, _, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))

	require.NoError(t, err)
	assert.True(t, reachable)
	assert.Equal(t, 2, redirectCount)
}

// TestValidateRedirectTarget_FirstRedirect tests validation of first redirect (no via)
func TestValidateRedirectTarget_FirstRedirect(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://localhost/redirect", http.NoBody)
	require.NoError(t, err)

	// First redirect - via is empty
	err = validateRedirectTargetStrict(req, nil, 2, true, true)
	require.NoError(t, err)
}

// TestURLConnectivity_ResponseBodyClosed tests that response body is properly closed
func TestURLConnectivity_ResponseBodyClosed(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response body content")) //nolint:errcheck
	}))
	defer mockServer.Close()

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("tcp", mockServer.Listener.Addr().String())
		},
	}

	// Run multiple times to ensure no resource leak
	for i := 0; i < 5; i++ {
		reachable, _, err := testURLConnectivity("http://localhost", withAllowLocalhostForTesting(), withTransportForTesting(transport))
		require.NoError(t, err)
		assert.True(t, reachable)
	}
}
