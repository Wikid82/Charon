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
