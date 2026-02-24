//go:build integration
// +build integration

package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wikid82/charon/backend/internal/notifications"
)

func TestNotificationHTTPWrapperIntegration_RetriesOn429AndSucceeds(t *testing.T) {
	t.Parallel()

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&calls, 1)
		if current == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	wrapper := notifications.NewNotifyHTTPWrapper()
	result, err := wrapper.Send(context.Background(), notifications.HTTPWrapperRequest{
		URL:  server.URL,
		Body: []byte(`{"message":"hello"}`),
	})
	if err != nil {
		t.Fatalf("expected retry success, got error: %v", err)
	}
	if result.Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", result.Attempts)
	}
}

func TestNotificationHTTPWrapperIntegration_DoesNotRetryOn400(t *testing.T) {
	t.Parallel()

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	wrapper := notifications.NewNotifyHTTPWrapper()
	_, err := wrapper.Send(context.Background(), notifications.HTTPWrapperRequest{
		URL:  server.URL,
		Body: []byte(`{"message":"hello"}`),
	})
	if err == nil {
		t.Fatalf("expected non-retryable 400 error")
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected one request attempt, got %d", calls)
	}
}

func TestNotificationHTTPWrapperIntegration_RejectsTokenizedQueryWithoutEcho(t *testing.T) {
	t.Parallel()

	wrapper := notifications.NewNotifyHTTPWrapper()
	secret := "pr1-secret-token-value"
	_, err := wrapper.Send(context.Background(), notifications.HTTPWrapperRequest{
		URL:  "http://example.com/hook?token=" + secret,
		Body: []byte(`{"message":"hello"}`),
	})
	if err == nil {
		t.Fatalf("expected tokenized query rejection")
	}
	if !strings.Contains(err.Error(), "query authentication is not allowed") {
		t.Fatalf("expected sanitized query-auth rejection, got: %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error must not echo secret token")
	}
}

func TestNotificationHTTPWrapperIntegration_HeaderAllowlistSafety(t *testing.T) {
	t.Parallel()

	var seenAuthHeader string
	var seenCookieHeader string
	var seenGotifyKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuthHeader = r.Header.Get("Authorization")
		seenCookieHeader = r.Header.Get("Cookie")
		seenGotifyKey = r.Header.Get("X-Gotify-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	wrapper := notifications.NewNotifyHTTPWrapper()
	_, err := wrapper.Send(context.Background(), notifications.HTTPWrapperRequest{
		URL: server.URL,
		Headers: map[string]string{
			"Authorization": "Bearer should-not-leak",
			"Cookie":        "session=should-not-leak",
			"X-Gotify-Key":  "allowed-token",
		},
		Body: []byte(`{"message":"hello"}`),
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if seenAuthHeader != "" {
		t.Fatalf("authorization header must be stripped")
	}
	if seenCookieHeader != "" {
		t.Fatalf("cookie header must be stripped")
	}
	if seenGotifyKey != "allowed-token" {
		t.Fatalf("expected X-Gotify-Key to pass through")
	}
}
