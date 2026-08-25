package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/go_notify_yourself/transport"
)

func TestResolveNotifyAllowHTTP(t *testing.T) {
	tests := []struct {
		name       string
		charonEnv  string
		allowHTTP  string
		wantResult bool
	}{
		{"test env auto-allows regardless of flag", "test", "", true},
		{"test env auto-allows even when flag explicitly false", "test", "false", true},
		{"development env requires explicit flag true", "development", "true", true},
		{"development env without flag stays false", "development", "", false},
		{"development env with flag false stays false", "development", "false", false},
		{"production env ignores flag", "production", "true", false},
		{"unset env ignores flag", "", "true", false},
		{"case-insensitive test env", "TEST", "", true},
		{"case-insensitive flag value", "development", "TRUE", true},
		{"whitespace padded env", "  test  ", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CHARON_ENV", tt.charonEnv)
			t.Setenv("CHARON_NOTIFY_ALLOW_HTTP", tt.allowHTTP)

			if got := resolveNotifyAllowHTTP(); got != tt.wantResult {
				t.Fatalf("resolveNotifyAllowHTTP() = %v, want %v", got, tt.wantResult)
			}
		})
	}
}

func TestResolveNotifyMaxRedirects(t *testing.T) {
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
			if got := resolveNotifyMaxRedirects(); got != tt.expected {
				t.Fatalf("resolveNotifyMaxRedirects() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestNotifyClientFactoryAllowsLocalhostWhenAllowHTTPTrue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := notifyClientFactory(true, 0)
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("expected localhost request to succeed with allowHTTP=true, got error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestNotifyClientFactoryBlocksLocalhostWhenAllowHTTPFalse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := notifyClientFactory(false, 0)
	_, err := client.Get(server.URL)
	if err == nil {
		t.Fatal("expected localhost request to fail with allowHTTP=false")
	}
}

func TestNotifyURLValidatorAllowsLocalhostHTTPWhenAllowed(t *testing.T) {
	validated, err := notifyURLValidator("http://127.0.0.1:8080/webhook", true)
	if err != nil {
		t.Fatalf("expected localhost http URL to validate with allowHTTP=true, got error: %v", err)
	}
	if validated == "" {
		t.Fatal("expected a non-empty normalized URL")
	}
}

func TestNotifyURLValidatorRejectsHTTPWhenNotAllowed(t *testing.T) {
	_, err := notifyURLValidator("http://example.com/webhook", false)
	if err == nil {
		t.Fatal("expected http URL to be rejected when allowHTTP=false")
	}
}

func TestNotifyURLValidatorRejectsPrivateIPWithoutOverride(t *testing.T) {
	_, err := notifyURLValidator("https://192.168.1.5/webhook", false)
	if err == nil {
		t.Fatal("expected private IP destination to be rejected")
	}
}

func TestNewNotifyTransportWrapperEndToEnd(t *testing.T) {
	t.Setenv("CHARON_ENV", "test")
	t.Setenv("CHARON_NOTIFY_ALLOW_HTTP", "")
	t.Setenv("CHARON_NOTIFY_MAX_REDIRECTS", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	wrapper := NewNotifyTransportWrapper()
	if wrapper == nil {
		t.Fatal("expected a non-nil transport.Wrapper")
	}

	result, err := wrapper.Send(context.Background(), transport.Request{
		URL:  server.URL,
		Body: []byte(`{"message":"hi"}`),
	})
	if err != nil {
		t.Fatalf("expected Send to succeed against local test server, got error: %v", err)
	}
	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", result.StatusCode)
	}
}

func TestNewNotifyTransportWrapperBlocksExternalHTTPOutsideTestEnv(t *testing.T) {
	t.Setenv("CHARON_ENV", "production")
	t.Setenv("CHARON_NOTIFY_ALLOW_HTTP", "")
	t.Setenv("CHARON_NOTIFY_MAX_REDIRECTS", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	wrapper := NewNotifyTransportWrapper()

	_, err := wrapper.Send(context.Background(), transport.Request{
		URL:  server.URL,
		Body: []byte(`{"message":"hi"}`),
	})
	if err == nil {
		t.Fatal("expected Send to a local httptest server to fail outside test/dev env")
	}
}
