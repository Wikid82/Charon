package bouncerkey

import (
	"context"
	"io"
	"net/http"
	"time"
)

// Verdict is the three-valued result of asking CrowdSec LAPI whether a key is accepted.
//
// Only a definite answer may change saved state: Accepted keeps the key, Rejected
// allows a new bouncer to be registered. Unknown means the answer could not be
// obtained (LAPI down, slow, erroring or the context was cancelled) and must never
// be read as a rejection.
type Verdict int

const (
	// VerdictUnknown is the zero value. An unfilled Verdict is never a rejection.
	VerdictUnknown Verdict = iota
	// VerdictAccepted means LAPI answered 200 for the key.
	VerdictAccepted
	// VerdictRejected means LAPI answered 403 for the key.
	VerdictRejected
)

// Doer sends an HTTP request. *http.Client satisfies it.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// CheckOptions bounds a key check.
type CheckOptions struct {
	// BaseURL is the LAPI base URL without the endpoint path.
	BaseURL string
	// Wait is the total time budget, including retries.
	Wait time.Duration
	// PerAttempt caps a single request.
	PerAttempt time.Duration
	// FirstBackoff is the sleep after the first failed attempt; it doubles up to MaxBackoff.
	FirstBackoff time.Duration
	// MaxBackoff caps the sleep between attempts.
	MaxBackoff time.Duration
}

// streamPath is the LAPI endpoint used for key checks. It requires authentication.
const streamPath = "/v1/decisions/stream"

// maxDrainBytes bounds how much of an unused response body is read before it is closed.
const maxDrainBytes = 64 << 10

// Check asks LAPI whether key is accepted.
//
// Only 403 is a definite rejection; 200 is an acceptance. Every other outcome
// (any other status including 401 and 3xx, transport errors and per-attempt
// timeouts) is retried with backoff until opts.Wait is used up, then reported as
// VerdictUnknown. A cancelled ctx is VerdictUnknown at once. An empty key is
// VerdictRejected without a request. Check never logs the key.
func Check(ctx context.Context, doer Doer, key string, opts CheckOptions) Verdict {
	if key == "" {
		return VerdictRejected
	}

	deadline := time.Now().Add(opts.Wait)
	backoff := opts.FirstBackoff
	for {
		verdict := checkAttempt(ctx, doer, key, opts)
		if verdict != VerdictUnknown {
			return verdict
		}
		if ctx.Err() != nil {
			return VerdictUnknown
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return VerdictUnknown
		}
		select {
		case <-ctx.Done():
			return VerdictUnknown
		case <-time.After(min(backoff, remaining)):
		}
		backoff = min(backoff*2, opts.MaxBackoff)
	}
}

// checkAttempt makes one request. It returns VerdictUnknown when the attempt
// gave no definite answer and should be retried.
func checkAttempt(ctx context.Context, doer Doer, key string, opts CheckOptions) Verdict {
	attemptCtx, cancel := context.WithTimeout(ctx, opts.PerAttempt)
	defer cancel()

	req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, opts.BaseURL+streamPath, http.NoBody)
	if err != nil {
		return VerdictUnknown
	}
	req.Header.Set("X-Api-Key", key)

	resp, err := doer.Do(req) //nolint:gosec // G704: BaseURL comes from trusted config, not user input
	if err != nil {
		return VerdictUnknown
	}
	defer func() {
		// Drain a bounded amount so the connection can be reused, then close.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainBytes))
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		return VerdictAccepted
	case http.StatusForbidden:
		return VerdictRejected
	default:
		return VerdictUnknown
	}
}
