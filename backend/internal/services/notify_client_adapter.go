package services

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/go_notify_yourself/transport"
)

// notifyClientTimeout is the outbound HTTP timeout used for every extracted
// notify-module provider dispatch. Matches the timeout the old
// internal/notifications.HTTPWrapper used before extraction (§3.2 Seam 1 of
// docs/plans/notifications_extraction_spec.md).
const notifyClientTimeout = 10 * time.Second

// resolveNotifyAllowHTTP determines whether outbound notification dispatch
// via the extracted notify module may use plain HTTP (and connect to
// localhost/private destinations) instead of requiring HTTPS.
//
// This is a deliberate, documented replacement for the old
// internal/notifications.allowNotifyHTTPOverride, which auto-allowed HTTP
// whenever os.Args[0] ended in ".test" (i.e. whenever the process was a
// compiled `go test` binary, regardless of any env var). That binary-name
// sniffing was flagged as a code smell in the extraction spec (§3.7) because
// it is implicit-environment coupling: a Go test binary is not, by itself, a
// declaration of intent to disable HTTPS enforcement.
//
// The replacement resolution is explicit and env-var-driven:
//
//   - CHARON_ENV=test auto-allows HTTP unconditionally, mirroring what the
//     ".test" suffix check was actually trying to achieve (frictionless
//     httptest.Server-backed unit tests) — but as an explicit opt-in test
//     setups declare via CHARON_ENV rather than something inferred from the
//     compiled binary's name. Test setup that relied on the old implicit
//     detection must set CHARON_ENV=test explicitly once this adapter is
//     wired into production dispatch (a later cutover phase, not this one).
//   - Outside of CHARON_ENV=test, behavior is unchanged from the old logic's
//     second branch: HTTP requires an explicit operator opt-in via
//     CHARON_NOTIFY_ALLOW_HTTP=true (env var name kept for backward
//     compatibility with existing deployments) AND CHARON_ENV set to
//     "development" or "test".
func resolveNotifyAllowHTTP() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("CHARON_ENV")))
	if env == "test" {
		return true
	}

	allowHTTP := strings.EqualFold(strings.TrimSpace(os.Getenv("CHARON_NOTIFY_ALLOW_HTTP")), "true")
	if !allowHTTP {
		return false
	}
	return env == "development" || env == "test"
}

// resolveNotifyMaxRedirects reads CHARON_NOTIFY_MAX_REDIRECTS, clamping the
// result to [0, 5]. Mirrors the old internal/notifications.notifyMaxRedirects
// exactly (same env var name and clamping behavior, for backward
// compatibility with existing deployments).
func resolveNotifyMaxRedirects() int {
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

// notifyClientFactory implements transport.ClientFactory, backed by Charon's
// existing SSRF-safe HTTP client (internal/network) — Seam 1 of the
// extraction spec (§3.2).
func notifyClientFactory(allowHTTP bool, maxRedirects int) *http.Client {
	opts := []network.Option{
		network.WithTimeout(notifyClientTimeout),
		network.WithMaxRedirects(maxRedirects),
	}
	if allowHTTP {
		opts = append(opts, network.WithAllowLocalhost())
	}
	return network.NewSafeHTTPClient(opts...)
}

// notifyURLValidator implements transport.URLValidator, backed by Charon's
// existing SSRF-safe URL validation (internal/security) — Seam 2 of the
// extraction spec (§3.2).
func notifyURLValidator(rawURL string, allowHTTP bool) (string, error) {
	var opts []security.ValidationOption
	if allowHTTP {
		opts = append(opts, security.WithAllowHTTP(), security.WithAllowLocalhost())
	}

	validated, err := security.ValidateExternalURL(rawURL, opts...)
	if err != nil {
		return "", fmt.Errorf("notify client adapter: validate destination URL: %w", err)
	}
	return validated, nil
}

// NewNotifyTransportWrapper builds the single shared *transport.Wrapper
// instance that every extracted notify-module provider package's
// New(cfg, wrapper) call is injected with (§3.6 step 3 of the extraction
// spec). It wires Charon's existing SSRF infrastructure
// (internal/network, internal/security) into the module's
// ClientFactory/URLValidator DI seams, and resolves the same
// CHARON_NOTIFY_ALLOW_HTTP / CHARON_NOTIFY_MAX_REDIRECTS env vars the old
// internal/notifications.HTTPWrapper read, keeping the same env var names for
// backward compatibility with existing deployments.
func NewNotifyTransportWrapper() *transport.Wrapper {
	return transport.NewWrapper(
		transport.WithClientFactory(notifyClientFactory),
		transport.WithURLValidator(notifyURLValidator),
		transport.WithRetryPolicy(transport.RetryPolicy{
			MaxAttempts: 3,
			BaseDelay:   200 * time.Millisecond,
			MaxDelay:    2 * time.Second,
		}),
		transport.WithAllowHTTP(resolveNotifyAllowHTTP()),
		transport.WithMaxRedirects(resolveNotifyMaxRedirects()),
	)
}
