package notifications

import (
	"context"
	"errors"
	"fmt"
	"io"
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
