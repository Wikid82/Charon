package notifications

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
