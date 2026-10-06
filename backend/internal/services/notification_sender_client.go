package services

import (
	"net/http"
	"time"

	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/security"
)

// senderAllowLoopback is a test-only seam. It is always false in production
// and is only toggled from _test.go files (see setSenderAllowLoopbackForTest).
// Tests that toggle it must not call t.Parallel(), as it is shared package state.
var senderAllowLoopback bool

// senderURLOptions returns the URL validation options shared by the
// notification senders. CGNAT (overlay networks) is always allowed; loopback is
// opt-in via the test seam only.
func senderURLOptions() []security.ValidationOption {
	opts := []security.ValidationOption{security.WithAllowHTTP(), security.WithAllowCGNAT()}
	if senderAllowLoopback {
		opts = append(opts, security.WithAllowLocalhost())
	}
	return opts
}

// newSenderHTTPClient builds the validated outbound client shared by the
// notification senders. CGNAT (overlay networks) is always allowed; loopback is
// opt-in via the test seam only.
func newSenderHTTPClient() *http.Client {
	opts := []network.Option{network.WithTimeout(10 * time.Second), network.WithAllowCGNAT()}
	if senderAllowLoopback {
		opts = append(opts, network.WithAllowLocalhost())
	}
	return network.NewSafeHTTPClient(opts...)
}
