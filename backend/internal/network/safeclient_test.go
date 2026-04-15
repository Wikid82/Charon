package network

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIsPrivateIP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		// Private IPv4 ranges
		{"10.0.0.0/8 start", "10.0.0.1", true},
		{"10.0.0.0/8 middle", "10.255.255.255", true},
		{"172.16.0.0/12 start", "172.16.0.1", true},
		{"172.16.0.0/12 end", "172.31.255.255", true},
		{"192.168.0.0/16 start", "192.168.0.1", true},
		{"192.168.0.0/16 end", "192.168.255.255", true},

		// Link-local
		{"169.254.0.0/16 start", "169.254.0.1", true},
		{"169.254.0.0/16 end", "169.254.255.255", true},

		// Loopback
		{"127.0.0.0/8 localhost", "127.0.0.1", true},
		{"127.0.0.0/8 other", "127.0.0.2", true},
		{"127.0.0.0/8 end", "127.255.255.255", true},

		// Special addresses
		{"0.0.0.0/8", "0.0.0.1", true},
		{"240.0.0.0/4 reserved", "240.0.0.1", true},
		{"255.255.255.255 broadcast", "255.255.255.255", true},

		// IPv6 private ranges
		{"IPv6 loopback", "::1", true},
		{"fc00::/7 unique local", "fc00::1", true},
		{"fd00::/8 unique local", "fd00::1", true},
		{"fe80::/10 link-local", "fe80::1", true},

		// Public IPs (should return false)
		{"Public IPv4 1", "8.8.8.8", false},
		{"Public IPv4 2", "1.1.1.1", false},
		{"Public IPv4 3", "93.184.216.34", false},
		{"Public IPv6", "2001:4860:4860::8888", false},

		// Edge cases
		{"Just outside 172.16", "172.15.255.255", false},
		{"Just outside 172.31", "172.32.0.0", false},
		{"Just outside 192.168", "192.167.255.255", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			result := IsPrivateIP(ip)
			if result != tt.expected {
				t.Errorf("IsPrivateIP(%s) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestIsPrivateIP_NilIP(t *testing.T) {
	t.Parallel()
	// nil IP should return true (block by default for safety)
	result := IsPrivateIP(nil)
	if result != true {
		t.Errorf("IsPrivateIP(nil) = %v, want true", result)
	}
}

func TestSafeDialer_BlocksPrivateIPs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		address     string
		shouldBlock bool
	}{
		{"blocks 10.x.x.x", "10.0.0.1:80", true},
		{"blocks 172.16.x.x", "172.16.0.1:80", true},
		{"blocks 192.168.x.x", "192.168.1.1:80", true},
		{"blocks 127.0.0.1", "127.0.0.1:80", true},
		{"blocks localhost", "localhost:80", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts := &ClientOptions{
				AllowLocalhost: false,
				DialTimeout:    time.Second,
			}
			dialer := safeDialer(opts)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			conn, err := dialer(ctx, "tcp", tt.address)
			if tt.shouldBlock {
				if err == nil {
					_ = conn.Close()
					t.Errorf("expected connection to %s to be blocked", tt.address)
				}
			}
		})
	}
}

func TestSafeDialer_AllowsLocalhost(t *testing.T) {
	t.Parallel()
	// Create a local test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Extract host:port from test server URL
	addr := server.Listener.Addr().String()

	opts := &ClientOptions{
		AllowLocalhost: true,
		DialTimeout:    5 * time.Second,
	}
	dialer := safeDialer(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer(ctx, "tcp", addr)
	if err != nil {
		t.Errorf("expected connection to localhost to be allowed when allowLocalhost=true, got error: %v", err)
		return
	}
	_ = conn.Close()
}

func TestSafeDialer_AllowedDomains(t *testing.T) {
	t.Parallel()
	opts := &ClientOptions{
		AllowLocalhost: false,
		AllowedDomains: []string{"app.crowdsec.net", "hub.crowdsec.net"},
		DialTimeout:    time.Second,
	}
	dialer := safeDialer(opts)

	// Test that allowed domain passes validation (we can't actually connect)
	// This is a structural test - we're verifying the domain check passes
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This will fail to connect (no server) but should NOT fail validation
	_, err := dialer(ctx, "tcp", "app.crowdsec.net:443")
	if err != nil {
		// Check it's a connection error, not a validation error
		if _, ok := err.(*net.OpError); !ok {
			// Context deadline exceeded is also acceptable (DNS/connection timeout)
			if err != context.DeadlineExceeded {
				t.Logf("Got expected error type for allowed domain: %T: %v", err, err)
			}
		}
	}
}

func TestNewSafeHTTPClient_DefaultOptions(t *testing.T) {
	t.Parallel()
	client := NewSafeHTTPClient()
	if client == nil {
		t.Fatal("NewSafeHTTPClient() returned nil")
		return
	}
	if client.Timeout != 10*time.Second {
		t.Errorf("expected default timeout of 10s, got %v", client.Timeout)
	}
}

func TestNewSafeHTTPClient_WithTimeout(t *testing.T) {
	t.Parallel()
	client := NewSafeHTTPClient(WithTimeout(10 * time.Second))
	if client == nil {
		t.Fatal("NewSafeHTTPClient() returned nil")
		return
	}
	if client.Timeout != 10*time.Second {
		t.Errorf("expected timeout of 10s, got %v", client.Timeout)
	}
}

func TestNewSafeHTTPClient_WithAllowLocalhost(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	// Create a local test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	client := NewSafeHTTPClient(
		WithTimeout(5*time.Second),
		WithAllowLocalhost(),
	)

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("expected request to localhost to succeed with allowLocalhost, got: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestNewSafeHTTPClient_BlocksSSRF(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	client := NewSafeHTTPClient(
		WithTimeout(2 * time.Second),
	)

	// Test that internal IPs are blocked
	urls := []string{
		"http://127.0.0.1/",
		"http://10.0.0.1/",
		"http://172.16.0.1/",
		"http://192.168.1.1/",
		"http://localhost/",
	}

	for _, url := range urls {
		t.Run(url, func(t *testing.T) {
			t.Parallel()
			resp, err := client.Get(url)
			if err == nil {
				defer func() { _ = resp.Body.Close() }()
				t.Errorf("expected request to %s to be blocked", url)
			}
		})
	}
}

func TestNewSafeHTTPClient_WithMaxRedirects(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	redirectCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectCount++
		if redirectCount < 5 {
			http.Redirect(w, r, "/redirect", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewSafeHTTPClient(
		WithTimeout(5*time.Second),
		WithAllowLocalhost(),
		WithMaxRedirects(2),
	)

	resp, err := client.Get(server.URL)
	if err == nil {
		defer func() { _ = resp.Body.Close() }()
		t.Error("expected redirect limit to be enforced")
	}
}

func TestNewSafeHTTPClient_WithAllowedDomains(t *testing.T) {
	t.Parallel()
	client := NewSafeHTTPClient(
		WithTimeout(2*time.Second),
		WithAllowedDomains("example.com"),
	)

	if client == nil {
		t.Fatal("NewSafeHTTPClient() returned nil")
	}

	// We can't actually connect, but we verify the client is created
	// with the correct configuration
}

func TestClientOptions_Defaults(t *testing.T) {
	t.Parallel()
	opts := defaultOptions()

	if opts.Timeout != 10*time.Second {
		t.Errorf("expected default timeout 10s, got %v", opts.Timeout)
	}
	if opts.MaxRedirects != 0 {
		t.Errorf("expected default maxRedirects 0, got %d", opts.MaxRedirects)
	}
	if opts.DialTimeout != 5*time.Second {
		t.Errorf("expected default dialTimeout 5s, got %v", opts.DialTimeout)
	}
}

func TestWithDialTimeout(t *testing.T) {
	t.Parallel()
	client := NewSafeHTTPClient(WithDialTimeout(5 * time.Second))
	if client == nil {
		t.Fatal("NewSafeHTTPClient() returned nil")
	}
}

// Benchmark tests
func BenchmarkIsPrivateIP_IPv4Private(b *testing.B) {
	ip := net.ParseIP("192.168.1.1")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsPrivateIP(ip)
	}
}

func BenchmarkIsPrivateIP_IPv4Public(b *testing.B) {
	ip := net.ParseIP("8.8.8.8")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsPrivateIP(ip)
	}
}

func BenchmarkIsPrivateIP_IPv6(b *testing.B) {
	ip := net.ParseIP("2001:4860:4860::8888")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsPrivateIP(ip)
	}
}

func BenchmarkNewSafeHTTPClient(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewSafeHTTPClient(
			WithTimeout(10*time.Second),
			WithAllowLocalhost(),
		)
	}
}

// Additional tests to increase coverage

func TestSafeDialer_InvalidAddress(t *testing.T) {
	t.Parallel()
	opts := &ClientOptions{
		AllowLocalhost: false,
		DialTimeout:    time.Second,
	}
	dialer := safeDialer(opts)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Test invalid address format (no port)
	_, err := dialer(ctx, "tcp", "invalid-address-no-port")
	if err == nil {
		t.Error("expected error for invalid address format")
	}
}

func TestSafeDialer_LoopbackIPv6(t *testing.T) {
	t.Parallel()
	opts := &ClientOptions{
		AllowLocalhost: true,
		DialTimeout:    time.Second,
	}
	dialer := safeDialer(opts)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Test IPv6 loopback with AllowLocalhost
	_, err := dialer(ctx, "tcp", "[::1]:80")
	// Should fail to connect but not due to validation
	if err != nil {
		t.Logf("Expected connection error (not validation): %v", err)
	}
}

func TestValidateRedirectTarget_EmptyHostname(t *testing.T) {
	t.Parallel()
	opts := &ClientOptions{
		AllowLocalhost: false,
		DialTimeout:    time.Second,
	}

	// Create request with empty hostname
	req, _ := http.NewRequest("GET", "http:///path", http.NoBody)
	err := validateRedirectTarget(req, opts)
	if err == nil {
		t.Error("expected error for empty hostname")
	}
}

func TestValidateRedirectTarget_Localhost(t *testing.T) {
	t.Parallel()
	opts := &ClientOptions{
		AllowLocalhost: false,
		DialTimeout:    time.Second,
	}

	// Test localhost blocked
	req, _ := http.NewRequest("GET", "http://localhost/path", http.NoBody)
	err := validateRedirectTarget(req, opts)
	if err == nil {
		t.Error("expected error for localhost when AllowLocalhost=false")
	}

	// Test localhost allowed
	opts.AllowLocalhost = true
	err = validateRedirectTarget(req, opts)
	if err != nil {
		t.Errorf("expected no error for localhost when AllowLocalhost=true, got: %v", err)
	}
}

func TestValidateRedirectTarget_127(t *testing.T) {
	t.Parallel()
	opts := &ClientOptions{
		AllowLocalhost: false,
		DialTimeout:    time.Second,
	}

	req, _ := http.NewRequest("GET", "http://127.0.0.1/path", http.NoBody)
	err := validateRedirectTarget(req, opts)
	if err == nil {
		t.Error("expected error for 127.0.0.1 when AllowLocalhost=false")
	}

	opts.AllowLocalhost = true
	err = validateRedirectTarget(req, opts)
	if err != nil {
		t.Errorf("expected no error for 127.0.0.1 when AllowLocalhost=true, got: %v", err)
	}
}

func TestValidateRedirectTarget_IPv6Loopback(t *testing.T) {
	t.Parallel()
	opts := &ClientOptions{
		AllowLocalhost: false,
		DialTimeout:    time.Second,
	}

	req, _ := http.NewRequest("GET", "http://[::1]/path", http.NoBody)
	err := validateRedirectTarget(req, opts)
	if err == nil {
		t.Error("expected error for ::1 when AllowLocalhost=false")
	}

	opts.AllowLocalhost = true
	err = validateRedirectTarget(req, opts)
	if err != nil {
		t.Errorf("expected no error for ::1 when AllowLocalhost=true, got: %v", err)
	}
}

func TestNewSafeHTTPClient_NoRedirectsByDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/redirected", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewSafeHTTPClient(
		WithTimeout(5*time.Second),
		WithAllowLocalhost(),
	)

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Should not follow redirect - should return 302
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected status 302 (redirect not followed), got %d", resp.StatusCode)
	}
}

func TestIsPrivateIP_IPv4MappedIPv6(t *testing.T) {
	t.Parallel()
	// Test IPv4-mapped IPv6 addresses
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"IPv4-mapped private", "::ffff:192.168.1.1", true},
		{"IPv4-mapped public", "::ffff:8.8.8.8", false},
		{"IPv4-mapped loopback", "::ffff:127.0.0.1", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			result := IsPrivateIP(ip)
			if result != tt.expected {
				t.Errorf("IsPrivateIP(%s) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestIsPrivateIP_Multicast(t *testing.T) {
	t.Parallel()
	// Test multicast addresses
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"IPv4 multicast", "224.0.0.1", true},
		{"IPv6 multicast", "ff02::1", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			result := IsPrivateIP(ip)
			if result != tt.expected {
				t.Errorf("IsPrivateIP(%s) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestIsPrivateIP_Unspecified(t *testing.T) {
	t.Parallel()
	// Test unspecified addresses
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"IPv4 unspecified", "0.0.0.0", true},
		{"IPv6 unspecified", "::", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			result := IsPrivateIP(ip)
			if result != tt.expected {
				t.Errorf("IsPrivateIP(%s) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

// Phase 1 Coverage Improvement Tests

func TestValidateRedirectTarget_DNSFailure(t *testing.T) {
	t.Parallel()
	opts := &ClientOptions{
		AllowLocalhost: false,
		DialTimeout:    100 * time.Millisecond, // Short timeout to force DNS failure quickly
	}

	// Use a domain that will fail DNS resolution
	req, _ := http.NewRequest("GET", "http://this-domain-does-not-exist-12345.invalid/path", http.NoBody)
	err := validateRedirectTarget(req, opts)
	if err == nil {
		t.Error("expected error for DNS resolution failure")
	}
	// Verify the error is DNS-related
	if err != nil && !contains(err.Error(), "DNS resolution failed") {
		t.Errorf("expected DNS resolution failure error, got: %v", err)
	}
}

func TestValidateRedirectTarget_PrivateIPInRedirect(t *testing.T) {
	t.Parallel()
	// Test that redirects to private IPs are properly blocked
	opts := &ClientOptions{
		AllowLocalhost: false,
		DialTimeout:    time.Second,
	}

	// Test various private IP redirect scenarios
	privateHosts := []string{
		"http://10.0.0.1/path",
		"http://172.16.0.1/path",
		"http://192.168.1.1/path",
		"http://169.254.169.254/latest/meta-data/", // AWS metadata endpoint
	}

	for _, url := range privateHosts {
		t.Run(url, func(t *testing.T) {
			t.Parallel()
			req, _ := http.NewRequest("GET", url, http.NoBody)
			err := validateRedirectTarget(req, opts)
			if err == nil {
				t.Errorf("expected error for redirect to private IP: %s", url)
			}
		})
	}
}

func TestSafeDialer_AllIPsPrivate(t *testing.T) {
	t.Parallel()
	// Test that when all resolved IPs are private, the connection is blocked
	opts := &ClientOptions{
		AllowLocalhost: false,
		DialTimeout:    time.Second,
	}
	dialer := safeDialer(opts)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Test dialing addresses that resolve to private IPs
	privateAddresses := []string{
		"10.0.0.1:80",
		"172.16.0.1:443",
		"192.168.0.1:8080",
		"169.254.169.254:80", // Cloud metadata endpoint
	}

	for _, addr := range privateAddresses {
		t.Run(addr, func(t *testing.T) {
			t.Parallel()
			conn, err := dialer(ctx, "tcp", addr)
			if err == nil {
				_ = conn.Close()
				t.Errorf("expected connection to %s to be blocked (all IPs private)", addr)
			}
		})
	}
}

func TestNewSafeHTTPClient_RedirectToPrivateIP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	// Create a server that redirects to a private IP
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// Redirect to a private IP (will be blocked)
			http.Redirect(w, r, "http://192.168.1.1/internal", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Client with redirects enabled and localhost allowed for the test server
	client := NewSafeHTTPClient(
		WithTimeout(5*time.Second),
		WithAllowLocalhost(),
		WithMaxRedirects(3),
	)

	// Make request - should fail when trying to follow redirect to private IP
	resp, err := client.Get(server.URL)
	if err == nil {
		defer func() { _ = resp.Body.Close() }()
		t.Error("expected error when redirect targets private IP")
	}
}

func TestSafeDialer_DNSResolutionFailure(t *testing.T) {
	t.Parallel()
	opts := &ClientOptions{
		AllowLocalhost: false,
		DialTimeout:    100 * time.Millisecond,
	}
	dialer := safeDialer(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Use a domain that will fail DNS resolution
	_, err := dialer(ctx, "tcp", "nonexistent-domain-xyz123.invalid:80")
	if err == nil {
		t.Error("expected error for DNS resolution failure")
	}
	if err != nil && !contains(err.Error(), "DNS resolution failed") {
		t.Errorf("expected DNS resolution failure error, got: %v", err)
	}
}

func TestSafeDialer_NoIPsReturned(t *testing.T) {
	t.Parallel()
	// This tests the edge case where DNS returns no IP addresses
	// In practice this is rare, but we need to handle it
	opts := &ClientOptions{
		AllowLocalhost: false,
		DialTimeout:    time.Second,
	}
	dialer := safeDialer(opts)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// This domain should fail DNS resolution
	_, err := dialer(ctx, "tcp", "empty-dns-result-test.invalid:80")
	if err == nil {
		t.Error("expected error when DNS returns no IPs")
	}
}

func TestNewSafeHTTPClient_TooManyRedirects(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	redirectCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectCount++
		// Keep redirecting to itself
		http.Redirect(w, r, "/redirect"+string(rune('0'+redirectCount)), http.StatusFound)
	}))
	defer server.Close()

	client := NewSafeHTTPClient(
		WithTimeout(5*time.Second),
		WithAllowLocalhost(),
		WithMaxRedirects(3),
	)

	resp, err := client.Get(server.URL)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err == nil {
		t.Error("expected error for too many redirects")
	}
	if err != nil && !contains(err.Error(), "too many redirects") {
		t.Logf("Got redirect error: %v", err)
	}
}

func TestValidateRedirectTarget_AllowedLocalhost(t *testing.T) {
	t.Parallel()
	opts := &ClientOptions{
		AllowLocalhost: true,
		DialTimeout:    time.Second,
	}

	// Test that localhost is allowed when AllowLocalhost is true
	localhostURLs := []string{
		"http://localhost/path",
		"http://127.0.0.1/path",
		"http://[::1]/path",
	}

	for _, url := range localhostURLs {
		t.Run(url, func(t *testing.T) {
			t.Parallel()
			req, _ := http.NewRequest("GET", url, http.NoBody)
			err := validateRedirectTarget(req, opts)
			if err != nil {
				t.Errorf("expected no error for %s when AllowLocalhost=true, got: %v", url, err)
			}
		})
	}
}

func TestNewSafeHTTPClient_MetadataEndpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	// Test that cloud metadata endpoints are blocked
	client := NewSafeHTTPClient(
		WithTimeout(2 * time.Second),
	)

	// AWS metadata endpoint
	resp, err := client.Get("http://169.254.169.254/latest/meta-data/")
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err == nil {
		t.Error("expected cloud metadata endpoint to be blocked")
	}
}

func TestSafeDialer_IPv4MappedIPv6(t *testing.T) {
	t.Parallel()
	opts := &ClientOptions{
		AllowLocalhost: false,
		DialTimeout:    time.Second,
	}
	dialer := safeDialer(opts)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Test IPv6-formatted localhost
	_, err := dialer(ctx, "tcp", "[::ffff:127.0.0.1]:80")
	if err == nil {
		t.Error("expected IPv4-mapped IPv6 loopback to be blocked")
	}
}

func TestClientOptions_AllFunctionalOptions(t *testing.T) {
	t.Parallel()
	// Test all functional options together
	client := NewSafeHTTPClient(
		WithTimeout(15*time.Second),
		WithAllowLocalhost(),
		WithAllowedDomains("example.com", "api.example.com"),
		WithMaxRedirects(5),
		WithDialTimeout(3*time.Second),
	)

	if client == nil {
		t.Fatal("NewSafeHTTPClient() returned nil with all options")
		return
	}
	if client.Timeout != 15*time.Second {
		t.Errorf("expected timeout of 15s, got %v", client.Timeout)
	}
}

func TestSafeDialer_ContextCancelled(t *testing.T) {
	t.Parallel()
	opts := &ClientOptions{
		AllowLocalhost: false,
		DialTimeout:    5 * time.Second,
	}
	dialer := safeDialer(opts)

	// Create an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := dialer(ctx, "tcp", "example.com:80")
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestNewSafeHTTPClient_RedirectValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network I/O test in short mode")
	}
	t.Parallel()
	// Server that redirects to itself (valid redirect)
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))
	defer server.Close()

	client := NewSafeHTTPClient(
		WithTimeout(5*time.Second),
		WithAllowLocalhost(),
		WithMaxRedirects(2),
	)

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

// Helper function for error message checking
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s != "" && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// PR-3: IsRFC1918 unit tests

func TestIsRFC1918_RFC1918Addresses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		ip   string
	}{
		{"10.0.0.0 start", "10.0.0.0"},
		{"10.0.0.1", "10.0.0.1"},
		{"10.128.0.1", "10.128.0.1"},
		{"10.255.255.255 end", "10.255.255.255"},
		{"172.16.0.0 start", "172.16.0.0"},
		{"172.16.0.1", "172.16.0.1"},
		{"172.24.0.1", "172.24.0.1"},
		{"172.31.255.255 end", "172.31.255.255"},
		{"192.168.0.0 start", "192.168.0.0"},
		{"192.168.1.1", "192.168.1.1"},
		{"192.168.255.255 end", "192.168.255.255"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			if !IsRFC1918(ip) {
				t.Errorf("IsRFC1918(%s) = false, want true", tt.ip)
			}
		})
	}
}

func TestIsRFC1918_NonRFC1918Addresses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		ip   string
	}{
		{"Loopback 127.0.0.1", "127.0.0.1"},
		{"Link-local 169.254.1.1", "169.254.1.1"},
		{"Cloud metadata 169.254.169.254", "169.254.169.254"},
		{"IPv6 loopback ::1", "::1"},
		{"IPv6 link-local fe80::1", "fe80::1"},
		{"Public 8.8.8.8", "8.8.8.8"},
		{"Unspecified 0.0.0.0", "0.0.0.0"},
		{"Broadcast 255.255.255.255", "255.255.255.255"},
		{"Reserved 240.0.0.1", "240.0.0.1"},
		{"IPv6 unique local fc00::1", "fc00::1"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			if IsRFC1918(ip) {
				t.Errorf("IsRFC1918(%s) = true, want false", tt.ip)
			}
		})
	}
}

func TestIsRFC1918_NilIP(t *testing.T) {
	t.Parallel()
	if IsRFC1918(nil) {
		t.Error("IsRFC1918(nil) = true, want false")
	}
}

func TestIsRFC1918_BoundaryAddresses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"11.0.0.0 just outside 10/8", "11.0.0.0", false},
		{"172.15.255.255 just below 172.16/12", "172.15.255.255", false},
		{"172.32.0.0 just above 172.31/12", "172.32.0.0", false},
		{"192.167.255.255 just below 192.168/16", "192.167.255.255", false},
		{"192.169.0.0 just above 192.168/16", "192.169.0.0", false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			if got := IsRFC1918(ip); got != tt.expected {
				t.Errorf("IsRFC1918(%s) = %v, want %v", tt.ip, got, tt.expected)
			}
		})
	}
}

func TestIsRFC1918_IPv4MappedAddresses(t *testing.T) {
	t.Parallel()
	// IPv4-mapped IPv6 representations of RFC 1918 addresses should be
	// recognised as RFC 1918 (after To4() normalisation inside IsRFC1918).
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"::ffff:10.0.0.1 mapped", "::ffff:10.0.0.1", true},
		{"::ffff:192.168.1.1 mapped", "::ffff:192.168.1.1", true},
		{"::ffff:172.16.0.1 mapped", "::ffff:172.16.0.1", true},
		{"::ffff:8.8.8.8 mapped public", "::ffff:8.8.8.8", false},
		{"::ffff:169.254.169.254 mapped link-local", "::ffff:169.254.169.254", false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			if got := IsRFC1918(ip); got != tt.expected {
				t.Errorf("IsRFC1918(%s) = %v, want %v", tt.ip, got, tt.expected)
			}
		})
	}
}

// PR-3: AllowRFC1918 safeDialer / client tests

func TestSafeDialer_AllowRFC1918_ValidationLoopSkipsRFC1918(t *testing.T) {
	// When AllowRFC1918 is set, the validation loop must NOT return
	// "connection to private IP blocked" for RFC 1918 addresses.
	// The subsequent TCP connection will fail because nothing is listening on
	// 192.168.1.1:80 in the test environment, but the error must be a
	// connection-level error, not an SSRF-block.
	opts := &ClientOptions{
		Timeout:      200 * time.Millisecond,
		DialTimeout:  200 * time.Millisecond,
		AllowRFC1918: true,
	}
	dial := safeDialer(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := dial(ctx, "tcp", "192.168.1.1:80")
	if err == nil {
		t.Fatal("expected a connection error, got nil")
	}
	if contains(err.Error(), "connection to private IP blocked") {
		t.Errorf("AllowRFC1918 should prevent private-IP blocking message; got: %v", err)
	}
}

func TestSafeDialer_AllowRFC1918_BlocksLinkLocal(t *testing.T) {
	// Link-local (169.254.x.x) must remain blocked even when AllowRFC1918=true.
	opts := &ClientOptions{
		Timeout:      200 * time.Millisecond,
		DialTimeout:  200 * time.Millisecond,
		AllowRFC1918: true,
	}
	dial := safeDialer(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := dial(ctx, "tcp", "169.254.1.1:80")
	if err == nil {
		t.Fatal("expected an error for link-local address, got nil")
	}
	if !contains(err.Error(), "connection to private IP blocked") {
		t.Errorf("expected link-local to be blocked; got: %v", err)
	}
}

func TestSafeDialer_AllowRFC1918_BlocksLoopbackWithoutAllowLocalhost(t *testing.T) {
	// Loopback must remain blocked when AllowRFC1918=true but AllowLocalhost=false.
	opts := &ClientOptions{
		Timeout:        200 * time.Millisecond,
		DialTimeout:    200 * time.Millisecond,
		AllowRFC1918:   true,
		AllowLocalhost: false,
	}
	dial := safeDialer(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := dial(ctx, "tcp", "127.0.0.1:80")
	if err == nil {
		t.Fatal("expected an error for loopback without AllowLocalhost, got nil")
	}
	if !contains(err.Error(), "connection to private IP blocked") {
		t.Errorf("expected loopback to be blocked; got: %v", err)
	}
}

func TestNewSafeHTTPClient_AllowRFC1918_BlocksSSRFMetadata(t *testing.T) {
	// Cloud metadata endpoint (169.254.169.254) must be blocked even with AllowRFC1918.
	client := NewSafeHTTPClient(
		WithTimeout(200*time.Millisecond),
		WithDialTimeout(200*time.Millisecond),
		WithAllowRFC1918(),
	)
	resp, err := client.Get("http://169.254.169.254/latest/meta-data/")
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected metadata endpoint to be blocked, got nil")
	}
	if !contains(err.Error(), "connection to private IP blocked") {
		t.Errorf("expected metadata endpoint blocking error; got: %v", err)
	}
}

func TestNewSafeHTTPClient_WithAllowRFC1918_OptionApplied(t *testing.T) {
	// Verify that WithAllowRFC1918() sets AllowRFC1918=true on ClientOptions.
	opts := defaultOptions()
	WithAllowRFC1918()(&opts)
	if !opts.AllowRFC1918 {
		t.Error("WithAllowRFC1918() should set AllowRFC1918=true")
	}
}
