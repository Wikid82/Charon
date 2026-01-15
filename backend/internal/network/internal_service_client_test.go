package network

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewInternalServiceHTTPClient(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"with 1 second timeout", 1 * time.Second},
		{"with 5 second timeout", 5 * time.Second},
		{"with 30 second timeout", 30 * time.Second},
		{"with 100ms timeout", 100 * time.Millisecond},
		{"with zero timeout", 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := NewInternalServiceHTTPClient(tt.timeout)
			if client == nil {
				t.Fatal("NewInternalServiceHTTPClient() returned nil")
			}
			if client.Timeout != tt.timeout {
				t.Errorf("expected timeout %v, got %v", tt.timeout, client.Timeout)
			}
		})
	}
}

func TestNewInternalServiceHTTPClient_TransportConfiguration(t *testing.T) {
	t.Parallel()
	timeout := 5 * time.Second
	client := NewInternalServiceHTTPClient(timeout)

	if client.Transport == nil {
		t.Fatal("expected Transport to be set")
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected Transport to be *http.Transport")
	}

	// Verify proxy is nil (ignores proxy environment variables)
	if transport.Proxy != nil {
		t.Error("expected Proxy to be nil for SSRF protection")
	}

	// Verify keep-alives are disabled
	if !transport.DisableKeepAlives {
		t.Error("expected DisableKeepAlives to be true")
	}

	// Verify MaxIdleConns
	if transport.MaxIdleConns != 1 {
		t.Errorf("expected MaxIdleConns to be 1, got %d", transport.MaxIdleConns)
	}

	// Verify timeout settings
	if transport.IdleConnTimeout != timeout {
		t.Errorf("expected IdleConnTimeout %v, got %v", timeout, transport.IdleConnTimeout)
	}
	if transport.TLSHandshakeTimeout != timeout {
		t.Errorf("expected TLSHandshakeTimeout %v, got %v", timeout, transport.TLSHandshakeTimeout)
	}
	if transport.ResponseHeaderTimeout != timeout {
		t.Errorf("expected ResponseHeaderTimeout %v, got %v", timeout, transport.ResponseHeaderTimeout)
	}
}

func TestNewInternalServiceHTTPClient_RedirectsDisabled(t *testing.T) {
	t.Parallel()
	// Create a test server that redirects
	redirectCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectCount++
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/redirected", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("redirected"))
	}))
	defer server.Close()

	client := NewInternalServiceHTTPClient(5 * time.Second)

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Should receive the redirect response, not follow it
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected status %d (redirect not followed), got %d", http.StatusFound, resp.StatusCode)
	}

	// Verify only one request was made (redirect was not followed)
	if redirectCount != 1 {
		t.Errorf("expected exactly 1 request, got %d (redirect was followed)", redirectCount)
	}
}

func TestNewInternalServiceHTTPClient_CheckRedirectReturnsErrUseLastResponse(t *testing.T) {
	t.Parallel()
	client := NewInternalServiceHTTPClient(5 * time.Second)

	if client.CheckRedirect == nil {
		t.Fatal("expected CheckRedirect to be set")
	}

	// Create a dummy request to test CheckRedirect
	req, _ := http.NewRequest("GET", "http://example.com", http.NoBody)
	err := client.CheckRedirect(req, nil)

	if err != http.ErrUseLastResponse {
		t.Errorf("expected CheckRedirect to return http.ErrUseLastResponse, got %v", err)
	}
}

func TestNewInternalServiceHTTPClient_ActualRequest(t *testing.T) {
	t.Parallel()
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewInternalServiceHTTPClient(5 * time.Second)

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestNewInternalServiceHTTPClient_TimeoutEnforced(t *testing.T) {
	t.Parallel()
	// Create a slow server that delays longer than the timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Use a very short timeout
	client := NewInternalServiceHTTPClient(100 * time.Millisecond)

	resp, err := client.Get(server.URL)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestNewInternalServiceHTTPClient_MultipleClients(t *testing.T) {
	t.Parallel()
	// Verify that multiple clients can be created with different timeouts
	client1 := NewInternalServiceHTTPClient(1 * time.Second)
	client2 := NewInternalServiceHTTPClient(10 * time.Second)

	if client1 == client2 {
		t.Error("expected different client instances")
	}

	if client1.Timeout != 1*time.Second {
		t.Errorf("client1 expected timeout 1s, got %v", client1.Timeout)
	}
	if client2.Timeout != 10*time.Second {
		t.Errorf("client2 expected timeout 10s, got %v", client2.Timeout)
	}
}

func TestNewInternalServiceHTTPClient_ProxyIgnored(t *testing.T) {
	t.Parallel()
	// Set up a server to verify no proxy is used
	directServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("direct"))
	}))
	defer directServer.Close()

	client := NewInternalServiceHTTPClient(5 * time.Second)

	// Even if environment has proxy settings, this client should ignore them
	// because transport.Proxy is set to nil
	transport := client.Transport.(*http.Transport)
	if transport.Proxy != nil {
		t.Error("expected Proxy to be nil (proxy env vars should be ignored)")
	}

	resp, err := client.Get(directServer.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestNewInternalServiceHTTPClient_PostRequest(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewInternalServiceHTTPClient(5 * time.Second)

	resp, err := client.Post(server.URL, "application/json", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}
}

// Benchmark tests

func BenchmarkNewInternalServiceHTTPClient(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewInternalServiceHTTPClient(5 * time.Second)
	}
}

func BenchmarkNewInternalServiceHTTPClient_Request(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewInternalServiceHTTPClient(5 * time.Second)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		resp, err := client.Get(server.URL)
		if err == nil {
			_ = resp.Body.Close()
		}
	}
}
