package notifications

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/security"
)

const (
	MaxNotifyRequestBodyBytes  = 256 * 1024
	MaxNotifyResponseBodyBytes = 1024 * 1024
)

type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

type HTTPWrapperRequest struct {
	URL     string
	Headers map[string]string
	Body    []byte
}

type HTTPWrapperResult struct {
	StatusCode   int
	ResponseBody []byte
	Attempts     int
}

type HTTPWrapper struct {
	retryPolicy       RetryPolicy
	allowHTTP         bool
	maxRedirects      int
	httpClientFactory func(allowHTTP bool, maxRedirects int) *http.Client
	sleep             func(time.Duration)
	jitterNanos       func(int64) int64
}

func NewNotifyHTTPWrapper() *HTTPWrapper {
	return &HTTPWrapper{
		retryPolicy: RetryPolicy{
			MaxAttempts: 3,
			BaseDelay:   200 * time.Millisecond,
			MaxDelay:    2 * time.Second,
		},
		allowHTTP:    allowNotifyHTTPOverride(),
		maxRedirects: notifyMaxRedirects(),
		httpClientFactory: func(allowHTTP bool, maxRedirects int) *http.Client {
			opts := []network.Option{network.WithTimeout(10 * time.Second), network.WithMaxRedirects(maxRedirects)}
			if allowHTTP {
				opts = append(opts, network.WithAllowLocalhost())
			}
			return network.NewSafeHTTPClient(opts...)
		},
		sleep: time.Sleep,
	}
}

func (w *HTTPWrapper) Send(ctx context.Context, request HTTPWrapperRequest) (*HTTPWrapperResult, error) {
	if len(request.Body) > MaxNotifyRequestBodyBytes {
		return nil, fmt.Errorf("request payload exceeds maximum size")
	}

	validatedURL, err := w.validateURL(request.URL)
	if err != nil {
		return nil, err
	}

	parsedValidatedURL, err := neturl.Parse(validatedURL)
	if err != nil {
		return nil, fmt.Errorf("destination URL validation failed")
	}

	if err := w.guardDestination(parsedValidatedURL); err != nil {
		return nil, err
	}

	headers := sanitizeOutboundHeaders(request.Headers)
	client := w.httpClientFactory(w.allowHTTP, w.maxRedirects)
	w.applyRedirectGuard(client)

	var lastErr error
	for attempt := 1; attempt <= w.retryPolicy.MaxAttempts; attempt++ {
		httpReq, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, parsedValidatedURL.String(), bytes.NewReader(request.Body))
		if reqErr != nil {
			return nil, fmt.Errorf("create outbound request: %w", reqErr)
		}

		for key, value := range headers {
			httpReq.Header.Set(key, value)
		}

		if httpReq.Header.Get("Content-Type") == "" {
			httpReq.Header.Set("Content-Type", "application/json")
		}

		validationOptions := []security.ValidationOption{}
		if w.allowHTTP {
			validationOptions = append(validationOptions, security.WithAllowHTTP(), security.WithAllowLocalhost())
		}

		safeURL, safeURLErr := security.ValidateExternalURL(httpReq.URL.String(), validationOptions...)
		if safeURLErr != nil {
			return nil, fmt.Errorf("destination URL validation failed")
		}

		safeParsedURL, safeParseErr := neturl.Parse(safeURL)
		if safeParseErr != nil {
			return nil, fmt.Errorf("destination URL validation failed")
		}

		if guardErr := w.guardDestination(safeParsedURL); guardErr != nil {
			return nil, guardErr
		}

		httpReq.URL = safeParsedURL

		resp, doErr := client.Do(httpReq)
		if doErr != nil {
			lastErr = doErr
			if attempt < w.retryPolicy.MaxAttempts && shouldRetry(nil, doErr) {
				w.waitBeforeRetry(attempt)
				continue
			}
			return nil, fmt.Errorf("outbound request failed")
		}

		body, bodyErr := readCappedResponseBody(resp.Body)
		closeErr := resp.Body.Close()
		if bodyErr != nil {
			return nil, bodyErr
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close response body: %w", closeErr)
		}

		if shouldRetry(resp, nil) && attempt < w.retryPolicy.MaxAttempts {
			w.waitBeforeRetry(attempt)
			continue
		}

		if resp.StatusCode >= http.StatusBadRequest {
			return nil, fmt.Errorf("provider returned status %d", resp.StatusCode)
		}

		return &HTTPWrapperResult{
			StatusCode:   resp.StatusCode,
			ResponseBody: body,
			Attempts:     attempt,
		}, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("provider request failed after retries")
	}

	return nil, fmt.Errorf("provider request failed")
}

func (w *HTTPWrapper) applyRedirectGuard(client *http.Client) {
	if client == nil {
		return
	}

	originalCheckRedirect := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if originalCheckRedirect != nil {
			if err := originalCheckRedirect(req, via); err != nil {
				return err
			}
		}

		return w.guardOutboundRequestURL(req)
	}
}

func (w *HTTPWrapper) validateURL(rawURL string) (string, error) {
	parsedURL, err := neturl.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid destination URL")
	}

	if hasDisallowedQueryAuthKey(parsedURL.Query()) {
		return "", fmt.Errorf("destination URL query authentication is not allowed")
	}

	options := []security.ValidationOption{}
	if w.allowHTTP {
		options = append(options, security.WithAllowHTTP(), security.WithAllowLocalhost())
	}

	validatedURL, err := security.ValidateExternalURL(rawURL, options...)
	if err != nil {
		return "", fmt.Errorf("destination URL validation failed")
	}

	return validatedURL, nil
}

func hasDisallowedQueryAuthKey(query neturl.Values) bool {
	for key := range query {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		switch normalizedKey {
		case "token", "auth", "apikey", "api_key":
			return true
		}
	}

	return false
}

func (w *HTTPWrapper) guardOutboundRequestURL(httpReq *http.Request) error {
	if httpReq == nil || httpReq.URL == nil {
		return fmt.Errorf("destination URL validation failed")
	}

	reqURL := httpReq.URL.String()
	validatedURL, err := w.validateURL(reqURL)
	if err != nil {
		return err
	}

	parsedValidatedURL, err := neturl.Parse(validatedURL)
	if err != nil {
		return fmt.Errorf("destination URL validation failed")
	}

	return w.guardDestination(parsedValidatedURL)
}

func (w *HTTPWrapper) guardDestination(destinationURL *neturl.URL) error {
	if destinationURL == nil {
		return fmt.Errorf("destination URL validation failed")
	}

	if destinationURL.User != nil || destinationURL.Fragment != "" {
		return fmt.Errorf("destination URL validation failed")
	}

	hostname := strings.TrimSpace(destinationURL.Hostname())
	if hostname == "" {
		return fmt.Errorf("destination URL validation failed")
	}

	if parsedIP := net.ParseIP(hostname); parsedIP != nil {
		if !w.isAllowedDestinationIP(hostname, parsedIP) {
			return fmt.Errorf("destination URL validation failed")
		}
		return nil
	}

	resolvedIPs, err := net.LookupIP(hostname)
	if err != nil || len(resolvedIPs) == 0 {
		return fmt.Errorf("destination URL validation failed")
	}

	for _, resolvedIP := range resolvedIPs {
		if !w.isAllowedDestinationIP(hostname, resolvedIP) {
			return fmt.Errorf("destination URL validation failed")
		}
	}

	return nil
}

func (w *HTTPWrapper) isAllowedDestinationIP(hostname string, ip net.IP) bool {
	if ip == nil {
		return false
	}

	if ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}

	if ip.IsLoopback() {
		return w.allowHTTP && isLocalDestinationHost(hostname)
	}

	if network.IsPrivateIP(ip) {
		return false
	}

	return true
}

func isLocalDestinationHost(host string) bool {
	trimmedHost := strings.TrimSpace(host)
	if strings.EqualFold(trimmedHost, "localhost") {
		return true
	}

	parsedIP := net.ParseIP(trimmedHost)
	return parsedIP != nil && parsedIP.IsLoopback()
}

func shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		var netErr net.Error
		if isNetErr := strings.Contains(strings.ToLower(err.Error()), "timeout") || strings.Contains(strings.ToLower(err.Error()), "connection"); isNetErr {
			return true
		}
		return errors.As(err, &netErr)
	}

	if resp == nil {
		return false
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}

	return resp.StatusCode >= http.StatusInternalServerError
}

func readCappedResponseBody(body io.Reader) ([]byte, error) {
	limited := io.LimitReader(body, MaxNotifyResponseBodyBytes+1)
	content, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if len(content) > MaxNotifyResponseBodyBytes {
		return nil, fmt.Errorf("response payload exceeds maximum size")
	}

	return content, nil
}

func sanitizeOutboundHeaders(headers map[string]string) map[string]string {
	allowed := map[string]struct{}{
		"content-type": {},
		"user-agent":   {},
		"x-request-id": {},
		"x-gotify-key": {},
	}

	sanitized := make(map[string]string)
	for key, value := range headers {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		if _, ok := allowed[normalizedKey]; !ok {
			continue
		}
		sanitized[http.CanonicalHeaderKey(normalizedKey)] = strings.TrimSpace(value)
	}

	return sanitized
}

func (w *HTTPWrapper) waitBeforeRetry(attempt int) {
	delay := w.retryPolicy.BaseDelay << (attempt - 1)
	if delay > w.retryPolicy.MaxDelay {
		delay = w.retryPolicy.MaxDelay
	}

	jitterFn := w.jitterNanos
	if jitterFn == nil {
		jitterFn = func(max int64) int64 {
			if max <= 0 {
				return 0
			}
			n, err := crand.Int(crand.Reader, big.NewInt(max))
			if err != nil {
				return 0
			}
			return n.Int64()
		}
	}

	jitter := time.Duration(jitterFn(int64(delay) / 2))
	sleepFn := w.sleep
	if sleepFn == nil {
		sleepFn = time.Sleep
	}
	sleepFn(delay + jitter)
}

func allowNotifyHTTPOverride() bool {
	if strings.HasSuffix(os.Args[0], ".test") {
		return true
	}

	allowHTTP := strings.EqualFold(strings.TrimSpace(os.Getenv("CHARON_NOTIFY_ALLOW_HTTP")), "true")
	if !allowHTTP {
		return false
	}

	environment := strings.ToLower(strings.TrimSpace(os.Getenv("CHARON_ENV")))
	return environment == "development" || environment == "test"
}

func notifyMaxRedirects() int {
	raw := strings.TrimSpace(os.Getenv("CHARON_NOTIFY_MAX_REDIRECTS"))
	if raw == "" {
		return 0
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}

	if value < 0 {
		return 0
	}
	if value > 5 {
		return 5
	}
	return value
}
