package notifications

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPWrapperRejectsOversizedRequestBody(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true

	payload := make([]byte, MaxNotifyRequestBodyBytes+1)
	_, err := wrapper.Send(context.Background(), HTTPWrapperRequest{
		URL:  "http://example.com/hook",
		Body: payload,
	})
	if err == nil || !strings.Contains(err.Error(), "request payload exceeds") {
		t.Fatalf("expected oversized request body error, got: %v", err)
	}
}

func TestHTTPWrapperRejectsTokenizedQueryURL(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true

	_, err := wrapper.Send(context.Background(), HTTPWrapperRequest{
		URL:  "http://example.com/hook?token=secret",
		Body: []byte(`{"message":"hello"}`),
	})
	if err == nil || !strings.Contains(err.Error(), "query authentication is not allowed") {
		t.Fatalf("expected query token rejection, got: %v", err)
	}
}

func TestHTTPWrapperRejectsQueryAuthCaseVariants(t *testing.T) {
	testCases := []string{
		"http://example.com/hook?Token=secret",
		"http://example.com/hook?AUTH=secret",
		"http://example.com/hook?apiKey=secret",
	}

	for _, testURL := range testCases {
		t.Run(testURL, func(t *testing.T) {
			wrapper := NewNotifyHTTPWrapper()
			wrapper.allowHTTP = true

			_, err := wrapper.Send(context.Background(), HTTPWrapperRequest{
				URL:  testURL,
				Body: []byte(`{"message":"hello"}`),
			})
			if err == nil || !strings.Contains(err.Error(), "query authentication is not allowed") {
				t.Fatalf("expected query auth rejection for %q, got: %v", testURL, err)
			}
		})
	}
}

func TestHTTPWrapperSendRejectsRedirectTargetWithDisallowedScheme(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		http.Redirect(w, r, "ftp://example.com/redirected", http.StatusFound)
	}))
	defer server.Close()

	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true
	wrapper.maxRedirects = 3
	wrapper.retryPolicy.MaxAttempts = 1

	_, err := wrapper.Send(context.Background(), HTTPWrapperRequest{
		URL:  server.URL,
		Body: []byte(`{"message":"hello"}`),
	})
	if err == nil || !strings.Contains(err.Error(), "outbound request failed") {
		t.Fatalf("expected outbound failure due to redirect target validation, got: %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("expected only initial request due to blocked redirect, got %d attempts", got)
	}
}

func TestHTTPWrapperSendRejectsRedirectTargetWithMixedCaseQueryAuth(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		http.Redirect(w, r, "https://example.com/redirected?Token=secret", http.StatusFound)
	}))
	defer server.Close()

	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true
	wrapper.maxRedirects = 3
	wrapper.retryPolicy.MaxAttempts = 1

	_, err := wrapper.Send(context.Background(), HTTPWrapperRequest{
		URL:  server.URL,
		Body: []byte(`{"message":"hello"}`),
	})
	if err == nil || !strings.Contains(err.Error(), "outbound request failed") {
		t.Fatalf("expected outbound failure due to redirect query auth validation, got: %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("expected only initial request due to blocked redirect, got %d attempts", got)
	}
}

func TestHTTPWrapperRetriesOn429ThenSucceeds(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&calls, 1)
		if current == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true
	wrapper.sleep = func(time.Duration) {}
	wrapper.jitterNanos = func(int64) int64 { return 0 }

	result, err := wrapper.Send(context.Background(), HTTPWrapperRequest{
		URL:  server.URL,
		Body: []byte(`{"message":"hello"}`),
	})
	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if result.Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", result.Attempts)
	}
}

func TestHTTPWrapperSendSuccessWithValidatedDestination(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("expected default content-type, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true
	wrapper.retryPolicy.MaxAttempts = 1
	wrapper.httpClientFactory = func(bool, int) *http.Client {
		return server.Client()
	}

	result, err := wrapper.Send(context.Background(), HTTPWrapperRequest{
		URL:  server.URL,
		Body: []byte(`{"message":"hello"}`),
	})
	if err != nil {
		t.Fatalf("expected successful send, got error: %v", err)
	}
	if result.Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", result.Attempts)
	}
	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, result.StatusCode)
	}
}

func TestHTTPWrapperSendRejectsUserInfoInDestinationURL(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()

	_, err := wrapper.Send(context.Background(), HTTPWrapperRequest{
		URL:  "https://user:pass@example.com/hook",
		Body: []byte(`{"message":"hello"}`),
	})
	if err == nil || !strings.Contains(err.Error(), "destination URL validation failed") {
		t.Fatalf("expected destination validation failure, got: %v", err)
	}
}

func TestHTTPWrapperSendRejectsFragmentInDestinationURL(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()

	_, err := wrapper.Send(context.Background(), HTTPWrapperRequest{
		URL:  "https://example.com/hook#fragment",
		Body: []byte(`{"message":"hello"}`),
	})
	if err == nil || !strings.Contains(err.Error(), "destination URL validation failed") {
		t.Fatalf("expected destination validation failure, got: %v", err)
	}
}

func TestHTTPWrapperDoesNotRetryOn400(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true
	wrapper.sleep = func(time.Duration) {}
	wrapper.jitterNanos = func(int64) int64 { return 0 }

	_, err := wrapper.Send(context.Background(), HTTPWrapperRequest{
		URL:  server.URL,
		Body: []byte(`{"message":"hello"}`),
	})
	if err == nil || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("expected non-retryable 400 error, got: %v", err)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected exactly one request attempt, got %d", calls)
	}
}

func TestHTTPWrapperResponseBodyCap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, strings.Repeat("x", MaxNotifyResponseBodyBytes+8))
	}))
	defer server.Close()

	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true

	_, err := wrapper.Send(context.Background(), HTTPWrapperRequest{
		URL:  server.URL,
		Body: []byte(`{"message":"hello"}`),
	})
	if err == nil || !strings.Contains(err.Error(), "response payload exceeds") {
		t.Fatalf("expected capped response body error, got: %v", err)
	}
}

func TestSanitizeOutboundHeadersAllowlist(t *testing.T) {
	headers := sanitizeOutboundHeaders(map[string]string{
		"Content-Type":  "application/json",
		"User-Agent":    "Charon",
		"X-Request-ID":  "abc",
		"X-Gotify-Key":  "secret",
		"Authorization": "Bearer token",
		"Cookie":        "sid=1",
	})

	if len(headers) != 4 {
		t.Fatalf("expected 4 allowed headers, got %d", len(headers))
	}
	if _, ok := headers["Authorization"]; ok {
		t.Fatalf("authorization header must be stripped")
	}
	if _, ok := headers["Cookie"]; ok {
		t.Fatalf("cookie header must be stripped")
	}
}

func TestHTTPWrapperGuardOutboundRequestURLRejectsNilRequest(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()

	err := wrapper.guardOutboundRequestURL(nil)
	if err == nil || !strings.Contains(err.Error(), "destination URL validation failed") {
		t.Fatalf("expected validation failure for nil request, got: %v", err)
	}
}

func TestHTTPWrapperGuardOutboundRequestURLRejectsQueryAuth(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true

	httpReq := &http.Request{URL: &neturl.URL{Scheme: "http", Host: "example.com", Path: "/hook", RawQuery: "token=secret"}}
	err := wrapper.guardOutboundRequestURL(httpReq)
	if err == nil || !strings.Contains(err.Error(), "query authentication is not allowed") {
		t.Fatalf("expected query auth rejection, got: %v", err)
	}
}

func TestHTTPWrapperGuardOutboundRequestURLRejectsMixedCaseQueryAuth(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true

	httpReq := &http.Request{URL: &neturl.URL{Scheme: "http", Host: "example.com", Path: "/hook", RawQuery: "apiKey=secret"}}
	err := wrapper.guardOutboundRequestURL(httpReq)
	if err == nil || !strings.Contains(err.Error(), "query authentication is not allowed") {
		t.Fatalf("expected query auth rejection, got: %v", err)
	}
}

func TestHTTPWrapperApplyRedirectGuardPreservesOriginalBehavior(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	baseErr := fmt.Errorf("base redirect policy")
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return baseErr
	}}

	wrapper.applyRedirectGuard(client)
	err := client.CheckRedirect(&http.Request{URL: &neturl.URL{Scheme: "https", Host: "example.com"}}, nil)
	if !errors.Is(err, baseErr) {
		t.Fatalf("expected original redirect policy error, got: %v", err)
	}
}

func TestHTTPWrapperGuardOutboundRequestURLRejectsUnsafeDestination(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = false

	httpReq := &http.Request{URL: &neturl.URL{Scheme: "http", Host: "example.com", Path: "/hook"}}
	err := wrapper.guardOutboundRequestURL(httpReq)
	if err == nil || !strings.Contains(err.Error(), "destination URL validation failed") {
		t.Fatalf("expected destination validation failure, got: %v", err)
	}
}

func TestHTTPWrapperGuardOutboundRequestURLAllowsValidatedDestination(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()

	httpReq := &http.Request{URL: &neturl.URL{Scheme: "https", Host: "example.com", Path: "/hook"}}
	err := wrapper.guardOutboundRequestURL(httpReq)
	if err != nil {
		t.Fatalf("expected validated destination to pass guard, got: %v", err)
	}
}

func TestHTTPWrapperGuardOutboundRequestURLRejectsUserInfo(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true

	httpReq := &http.Request{URL: &neturl.URL{Scheme: "http", Host: "127.0.0.1", User: neturl.UserPassword("user", "pass"), Path: "/hook"}}
	err := wrapper.guardOutboundRequestURL(httpReq)
	if err == nil || !strings.Contains(err.Error(), "destination URL validation failed") {
		t.Fatalf("expected userinfo rejection, got: %v", err)
	}
}

func TestHTTPWrapperGuardOutboundRequestURLRejectsFragment(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()

	httpReq := &http.Request{URL: &neturl.URL{Scheme: "https", Host: "example.com", Path: "/hook", Fragment: "frag"}}
	err := wrapper.guardOutboundRequestURL(httpReq)
	if err == nil || !strings.Contains(err.Error(), "destination URL validation failed") {
		t.Fatalf("expected fragment rejection, got: %v", err)
	}
}

func TestSanitizeTransportErrorReason(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{name: "nil error", err: nil, expected: "connection failed"},
		{name: "dns error", err: errors.New("dial tcp: lookup gotify.example: no such host"), expected: "dns lookup failed"},
		{name: "connection refused", err: errors.New("connect: connection refused"), expected: "connection refused"},
		{name: "network unreachable", err: errors.New("connect: no route to host"), expected: "network unreachable"},
		{name: "timeout", err: errors.New("context deadline exceeded"), expected: "request timed out"},
		{name: "tls failure", err: errors.New("tls: handshake failure"), expected: "tls handshake failed"},
		{name: "fallback", err: errors.New("some unexpected transport error"), expected: "connection failed"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actual := sanitizeTransportErrorReason(testCase.err)
			if actual != testCase.expected {
				t.Fatalf("expected %q, got %q", testCase.expected, actual)
			}
		})
	}
}

func TestBuildSafeRequestURLPreservesHostnameForTLS(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true

	destinationURL := &neturl.URL{
		Scheme: "https",
		Host:   "example.com",
		Path:   "/webhook",
	}

	safeURL, hostHeader, err := wrapper.buildSafeRequestURL(destinationURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if safeURL.Hostname() != "example.com" {
		t.Fatalf("expected hostname 'example.com' preserved in URL for TLS SNI, got %q", safeURL.Hostname())
	}

	if hostHeader != "example.com" {
		t.Fatalf("expected host header 'example.com', got %q", hostHeader)
	}

	if safeURL.Scheme != "https" {
		t.Fatalf("expected scheme 'https', got %q", safeURL.Scheme)
	}

	if safeURL.Path != "/webhook" {
		t.Fatalf("expected path '/webhook', got %q", safeURL.Path)
	}
}

func TestBuildSafeRequestURLDefaultsEmptyPathToSlash(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true

	destinationURL := &neturl.URL{
		Scheme: "http",
		Host:   "localhost",
	}

	safeURL, _, err := wrapper.buildSafeRequestURL(destinationURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if safeURL.Path != "/" {
		t.Fatalf("expected default path '/', got %q", safeURL.Path)
	}
}

func TestBuildSafeRequestURLPreservesQueryString(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true

	destinationURL := &neturl.URL{
		Scheme:   "https",
		Host:     "example.com",
		Path:     "/hook",
		RawQuery: "key=value",
	}

	safeURL, _, err := wrapper.buildSafeRequestURL(destinationURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if safeURL.RawQuery != "key=value" {
		t.Fatalf("expected query 'key=value', got %q", safeURL.RawQuery)
	}
}

func TestBuildSafeRequestURLRejectsNilDestination(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()

	_, _, err := wrapper.buildSafeRequestURL(nil)
	if err == nil || !strings.Contains(err.Error(), "destination URL validation failed") {
		t.Fatalf("expected validation failure for nil URL, got: %v", err)
	}
}

func TestBuildSafeRequestURLRejectsEmptyHostname(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()

	destinationURL := &neturl.URL{
		Scheme: "https",
		Host:   "",
		Path:   "/hook",
	}

	_, _, err := wrapper.buildSafeRequestURL(destinationURL)
	if err == nil || !strings.Contains(err.Error(), "destination URL validation failed") {
		t.Fatalf("expected validation failure for empty hostname, got: %v", err)
	}
}

func TestBuildSafeRequestURLWithTLSServer(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	serverURL, _ := neturl.Parse(server.URL)

	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true

	safeURL, hostHeader, err := wrapper.buildSafeRequestURL(serverURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if safeURL.Host != serverURL.Host {
		t.Fatalf("expected host %q preserved for TLS, got %q", serverURL.Host, safeURL.Host)
	}

	if hostHeader != serverURL.Host {
		t.Fatalf("expected host header %q, got %q", serverURL.Host, hostHeader)
	}
}

// ===== Additional coverage for uncovered paths =====

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("simulated read error")
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestApplyRedirectGuardNilClient(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.applyRedirectGuard(nil)
}

func TestGuardDestinationNilURL(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	err := wrapper.guardDestination(nil)
	if err == nil || !strings.Contains(err.Error(), "destination URL validation failed") {
		t.Fatalf("expected validation failure for nil URL, got: %v", err)
	}
}

func TestGuardDestinationEmptyHostname(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	err := wrapper.guardDestination(&neturl.URL{Scheme: "https", Host: ""})
	if err == nil || !strings.Contains(err.Error(), "destination URL validation failed") {
		t.Fatalf("expected validation failure for empty hostname, got: %v", err)
	}
}

func TestGuardDestinationUserInfoRejection(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	u := &neturl.URL{Scheme: "https", Host: "example.com", User: neturl.User("admin")}
	err := wrapper.guardDestination(u)
	if err == nil || !strings.Contains(err.Error(), "destination URL validation failed") {
		t.Fatalf("expected userinfo rejection, got: %v", err)
	}
}

func TestGuardDestinationFragmentRejection(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	u := &neturl.URL{Scheme: "https", Host: "example.com", Fragment: "section"}
	err := wrapper.guardDestination(u)
	if err == nil || !strings.Contains(err.Error(), "destination URL validation failed") {
		t.Fatalf("expected fragment rejection, got: %v", err)
	}
}

func TestGuardDestinationPrivateIPRejection(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = false
	err := wrapper.guardDestination(&neturl.URL{Scheme: "https", Host: "192.168.1.1"})
	if err == nil || !strings.Contains(err.Error(), "destination URL validation failed") {
		t.Fatalf("expected private IP rejection, got: %v", err)
	}
}

func TestIsAllowedDestinationIPEdgeCases(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = false

	tests := []struct {
		name     string
		hostname string
		ip       net.IP
		expected bool
	}{
		{"nil IP", "", nil, false},
		{"unspecified", "0.0.0.0", net.IPv4zero, false},
		{"multicast", "224.0.0.1", net.ParseIP("224.0.0.1"), false},
		{"link-local unicast", "169.254.1.1", net.ParseIP("169.254.1.1"), false},
		{"loopback without allowHTTP", "127.0.0.1", net.ParseIP("127.0.0.1"), false},
		{"private 10.x", "10.0.0.1", net.ParseIP("10.0.0.1"), false},
		{"private 172.16.x", "172.16.0.1", net.ParseIP("172.16.0.1"), false},
		{"private 192.168.x", "192.168.1.1", net.ParseIP("192.168.1.1"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapper.isAllowedDestinationIP(tt.hostname, tt.ip)
			if result != tt.expected {
				t.Fatalf("isAllowedDestinationIP(%q, %v) = %v, want %v", tt.hostname, tt.ip, result, tt.expected)
			}
		})
	}
}

func TestIsAllowedDestinationIPLoopbackAllowHTTP(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true

	if !wrapper.isAllowedDestinationIP("localhost", net.ParseIP("127.0.0.1")) {
		t.Fatal("expected loopback allowed for localhost with allowHTTP")
	}

	if wrapper.isAllowedDestinationIP("not-localhost", net.ParseIP("127.0.0.1")) {
		t.Fatal("expected loopback rejected for non-localhost hostname")
	}
}

func TestIsLocalDestinationHost(t *testing.T) {
	tests := []struct {
		host     string
		expected bool
	}{
		{"localhost", true},
		{"LOCALHOST", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"example.com", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			if got := isLocalDestinationHost(tt.host); got != tt.expected {
				t.Fatalf("isLocalDestinationHost(%q) = %v, want %v", tt.host, got, tt.expected)
			}
		})
	}
}

func TestShouldRetryComprehensive(t *testing.T) {
	tests := []struct {
		name     string
		resp     *http.Response
		err      error
		expected bool
	}{
		{"nil resp nil err", nil, nil, false},
		{"timeout error string", nil, errors.New("operation timeout"), true},
		{"connection error string", nil, errors.New("connection reset"), true},
		{"unrelated error", nil, errors.New("json parse error"), false},
		{"500 response", &http.Response{StatusCode: 500}, nil, true},
		{"502 response", &http.Response{StatusCode: 502}, nil, true},
		{"503 response", &http.Response{StatusCode: 503}, nil, true},
		{"429 response", &http.Response{StatusCode: 429}, nil, true},
		{"200 response", &http.Response{StatusCode: 200}, nil, false},
		{"400 response", &http.Response{StatusCode: 400}, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRetry(tt.resp, tt.err); got != tt.expected {
				t.Fatalf("shouldRetry = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestShouldRetryNetError(t *testing.T) {
	netErr := &net.DNSError{Err: "no such host", Name: "example.invalid"}
	if !shouldRetry(nil, netErr) {
		t.Fatal("expected net.Error to trigger retry via errors.As fallback")
	}
}

func TestReadCappedResponseBodyReadError(t *testing.T) {
	_, err := readCappedResponseBody(errReader{})
	if err == nil || !strings.Contains(err.Error(), "read response body") {
		t.Fatalf("expected read body error, got: %v", err)
	}
}

func TestReadCappedResponseBodyOversize(t *testing.T) {
	oversized := strings.NewReader(strings.Repeat("x", MaxNotifyResponseBodyBytes+10))
	_, err := readCappedResponseBody(oversized)
	if err == nil || !strings.Contains(err.Error(), "response payload exceeds") {
		t.Fatalf("expected oversize error, got: %v", err)
	}
}

func TestReadCappedResponseBodySuccess(t *testing.T) {
	content, err := readCappedResponseBody(strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("expected 'hello', got %q", string(content))
	}
}

func TestHasDisallowedQueryAuthKeyAllVariants(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{"token", "token", true},
		{"auth", "auth", true},
		{"apikey", "apikey", true},
		{"api_key", "api_key", true},
		{"TOKEN uppercase", "TOKEN", true},
		{"Api_Key mixed", "Api_Key", true},
		{"safe key", "callback", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := neturl.Values{}
			query.Set(tt.key, "secret")
			if got := hasDisallowedQueryAuthKey(query); got != tt.expected {
				t.Fatalf("hasDisallowedQueryAuthKey with key %q = %v, want %v", tt.key, got, tt.expected)
			}
		})
	}
}

func TestHasDisallowedQueryAuthKeyEmptyQuery(t *testing.T) {
	if hasDisallowedQueryAuthKey(neturl.Values{}) {
		t.Fatal("expected empty query to be safe")
	}
}

func TestNotifyMaxRedirects(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected int
	}{
		{"empty", "", 0},
		{"valid 3", "3", 3},
		{"zero", "0", 0},
		{"negative", "-1", 0},
		{"above max", "10", 5},
		{"exactly 5", "5", 5},
		{"invalid", "abc", 0},
		{"whitespace", " 2 ", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CHARON_NOTIFY_MAX_REDIRECTS", tt.envValue)
			if got := notifyMaxRedirects(); got != tt.expected {
				t.Fatalf("notifyMaxRedirects() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestResolveAllowedDestinationIPRejectsPrivateIP(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = false
	_, err := wrapper.resolveAllowedDestinationIP("192.168.1.1")
	if err == nil || !strings.Contains(err.Error(), "destination URL validation failed") {
		t.Fatalf("expected private IP rejection, got: %v", err)
	}
}

func TestResolveAllowedDestinationIPRejectsLoopback(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = false
	_, err := wrapper.resolveAllowedDestinationIP("127.0.0.1")
	if err == nil || !strings.Contains(err.Error(), "destination URL validation failed") {
		t.Fatalf("expected loopback rejection, got: %v", err)
	}
}

func TestResolveAllowedDestinationIPAllowsPublic(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	ip, err := wrapper.resolveAllowedDestinationIP("1.1.1.1")
	if err != nil {
		t.Fatalf("expected public IP to be allowed, got: %v", err)
	}
	if !ip.Equal(net.ParseIP("1.1.1.1")) {
		t.Fatalf("expected 1.1.1.1, got %v", ip)
	}
}

func TestBuildSafeRequestURLRejectsPrivateHostname(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = false
	u := &neturl.URL{Scheme: "https", Host: "192.168.1.1", Path: "/hook"}
	_, _, err := wrapper.buildSafeRequestURL(u)
	if err == nil || !strings.Contains(err.Error(), "destination URL validation failed") {
		t.Fatalf("expected private host rejection, got: %v", err)
	}
}

func TestWaitBeforeRetryBasic(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	var sleptDuration time.Duration
	wrapper.sleep = func(d time.Duration) { sleptDuration = d }
	wrapper.jitterNanos = func(int64) int64 { return 0 }
	wrapper.retryPolicy.BaseDelay = 100 * time.Millisecond
	wrapper.retryPolicy.MaxDelay = 1 * time.Second

	wrapper.waitBeforeRetry(1)
	if sleptDuration != 100*time.Millisecond {
		t.Fatalf("expected 100ms delay for attempt 1, got %v", sleptDuration)
	}

	wrapper.waitBeforeRetry(2)
	if sleptDuration != 200*time.Millisecond {
		t.Fatalf("expected 200ms delay for attempt 2, got %v", sleptDuration)
	}
}

func TestWaitBeforeRetryClampedToMax(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	var sleptDuration time.Duration
	wrapper.sleep = func(d time.Duration) { sleptDuration = d }
	wrapper.jitterNanos = func(int64) int64 { return 0 }
	wrapper.retryPolicy.BaseDelay = 1 * time.Second
	wrapper.retryPolicy.MaxDelay = 2 * time.Second

	wrapper.waitBeforeRetry(5)
	if sleptDuration != 2*time.Second {
		t.Fatalf("expected clamped delay of 2s, got %v", sleptDuration)
	}
}

func TestWaitBeforeRetryDefaultJitter(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.jitterNanos = nil
	wrapper.sleep = func(time.Duration) {}
	wrapper.retryPolicy.BaseDelay = 100 * time.Millisecond
	wrapper.retryPolicy.MaxDelay = 1 * time.Second
	wrapper.waitBeforeRetry(1)
}

func TestHTTPWrapperSendExhaustsRetriesOnTransportError(t *testing.T) {
	var calls int32
	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true
	wrapper.sleep = func(time.Duration) {}
	wrapper.jitterNanos = func(int64) int64 { return 0 }
	wrapper.httpClientFactory = func(bool, int) *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				atomic.AddInt32(&calls, 1)
				return nil, errors.New("connection timeout failure")
			}),
		}
	}

	_, err := wrapper.Send(context.Background(), HTTPWrapperRequest{
		URL:  "http://localhost:19999/hook",
		Body: []byte(`{"msg":"test"}`),
	})
	if err == nil {
		t.Fatal("expected error after transport failures")
	}
	if !strings.Contains(err.Error(), "outbound request failed") {
		t.Fatalf("expected outbound request failed message, got: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

func TestHTTPWrapperSendExhaustsRetriesOn500(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true
	wrapper.sleep = func(time.Duration) {}
	wrapper.jitterNanos = func(int64) int64 { return 0 }

	_, err := wrapper.Send(context.Background(), HTTPWrapperRequest{
		URL:  server.URL,
		Body: []byte(`{"msg":"test"}`),
	})
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("expected 500 status error, got: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 attempts for 500 retries, got %d", got)
	}
}

func TestHTTPWrapperSendTransportErrorNoRetry(t *testing.T) {
	wrapper := NewNotifyHTTPWrapper()
	wrapper.allowHTTP = true
	wrapper.retryPolicy.MaxAttempts = 1
	wrapper.httpClientFactory = func(bool, int) *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("some unretryable error")
			}),
		}
	}

	_, err := wrapper.Send(context.Background(), HTTPWrapperRequest{
		URL:  "http://localhost:19999/hook",
		Body: []byte(`{"msg":"test"}`),
	})
	if err == nil || !strings.Contains(err.Error(), "outbound request failed") {
		t.Fatalf("expected outbound request failed, got: %v", err)
	}
}

func TestSanitizeTransportErrorReasonNetworkUnreachable(t *testing.T) {
	result := sanitizeTransportErrorReason(errors.New("connect: network is unreachable"))
	if result != "network unreachable" {
		t.Fatalf("expected 'network unreachable', got %q", result)
	}
}

func TestSanitizeTransportErrorReasonCertificate(t *testing.T) {
	result := sanitizeTransportErrorReason(errors.New("x509: certificate signed by unknown authority"))
	if result != "tls handshake failed" {
		t.Fatalf("expected 'tls handshake failed', got %q", result)
	}
}

func TestAllowNotifyHTTPOverride(t *testing.T) {
	result := allowNotifyHTTPOverride()
	if !result {
		t.Fatal("expected allowHTTP to be true in test binary")
	}
}

func TestExtractProviderErrorHint(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		expected string
	}{
		{
			name:     "description field",
			body:     []byte(`{"description":"Not Found: chat not found"}`),
			expected: "Not Found: chat not found",
		},
		{
			name:     "message field",
			body:     []byte(`{"message":"Unauthorized"}`),
			expected: "Unauthorized",
		},
		{
			name:     "error field",
			body:     []byte(`{"error":"rate limited"}`),
			expected: "rate limited",
		},
		{
			name:     "error_description field",
			body:     []byte(`{"error_description":"invalid token"}`),
			expected: "invalid token",
		},
		{
			name:     "empty body",
			body:     []byte{},
			expected: "",
		},
		{
			name:     "non-JSON body",
			body:     []byte(`<html>Server Error</html>`),
			expected: "",
		},
		{
			name:     "string over 100 chars truncated",
			body:     []byte(`{"description":"` + strings.Repeat("x", 120) + `"}`),
			expected: strings.Repeat("x", 100) + "...",
		},
		{
			name:     "empty string value ignored",
			body:     []byte(`{"description":"","message":"fallback hint"}`),
			expected: "fallback hint",
		},
		{
			name:     "whitespace-only value ignored",
			body:     []byte(`{"description":"   ","message":"real hint"}`),
			expected: "real hint",
		},
		{
			name:     "non-string value ignored",
			body:     []byte(`{"description":42,"message":"string hint"}`),
			expected: "string hint",
		},
		{
			name:     "priority order: description before message",
			body:     []byte(`{"message":"second","description":"first"}`),
			expected: "first",
		},
		{
			name:     "no recognized fields",
			body:     []byte(`{"status":"error","code":500}`),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractProviderErrorHint(tt.body)
			if result != tt.expected {
				t.Errorf("extractProviderErrorHint(%q) = %q, want %q", string(tt.body), result, tt.expected)
			}
		})
	}
}
