package network

import (
	"net/http"
	"time"
)

// NewInternalServiceHTTPClient returns an HTTP client intended for internal service calls
// that are already constrained by an explicit hostname allowlist + expected port policy.
//
// Security posture:
// - Ignores proxy environment variables.
// - Disables redirects.
// - Uses strict, caller-provided timeouts.
func NewInternalServiceHTTPClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		// Explicitly ignore proxy environment variables for SSRF-sensitive requests.
		Proxy:                 nil,
		DisableKeepAlives:     true,
		MaxIdleConns:          1,
		IdleConnTimeout:       timeout,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		// Explicit redirect policy per call site: disable.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
