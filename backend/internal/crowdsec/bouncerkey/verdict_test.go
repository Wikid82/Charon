package bouncerkey

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testCheckKey is the key every Check test sends. It is a fixed, non-secret value.
const testCheckKey = "check-test-key-0000000000000001"

// newCheckDoer returns an HTTP client that returns redirects to the caller
// instead of following them, the same way the shared internal client does.
func newCheckDoer() *http.Client {
	return &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// fastCheckOptions keeps the retry window short so tests stay quick.
func fastCheckOptions(baseURL string) CheckOptions {
	return CheckOptions{
		BaseURL:      baseURL,
		Wait:         300 * time.Millisecond,
		PerAttempt:   100 * time.Millisecond,
		FirstBackoff: 5 * time.Millisecond,
		MaxBackoff:   20 * time.Millisecond,
	}
}

// statusServer answers every request with status and counts the requests it receives.
func statusServer(t *testing.T, status int, requests *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestVerdictZeroValueIsUnknown(t *testing.T) {
	t.Parallel()

	var v Verdict
	require.Equal(t, VerdictUnknown, v)
	require.NotEqual(t, VerdictRejected, v)
	require.NotEqual(t, VerdictAccepted, v)
}

func TestCheckAcceptsOn200AndSendsKeyHeader(t *testing.T) {
	t.Parallel()

	var got atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Store(r.Method + " " + r.URL.Path + " " + r.Header.Get("X-Api-Key"))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	verdict := Check(context.Background(), newCheckDoer(), testCheckKey, fastCheckOptions(srv.URL))

	require.Equal(t, VerdictAccepted, verdict)
	require.Equal(t, "GET /v1/decisions/stream "+testCheckKey, got.Load())
}

func TestCheckRejectsOn403WithoutRetry(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	srv := statusServer(t, http.StatusForbidden, &requests)

	verdict := Check(context.Background(), newCheckDoer(), testCheckKey, fastCheckOptions(srv.URL))

	require.Equal(t, VerdictRejected, verdict)
	require.Equal(t, int32(1), requests.Load())
}

// TestCheckUnauthorizedIsNotRejected pins the 401 mapping: only 403 is a definite rejection.
func TestCheckUnauthorizedIsNotRejected(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	srv := statusServer(t, http.StatusUnauthorized, &requests)

	verdict := Check(context.Background(), newCheckDoer(), testCheckKey, fastCheckOptions(srv.URL))

	require.Equal(t, VerdictUnknown, verdict)
	require.Greater(t, requests.Load(), int32(1), "401 must be retried inside the window")
}

func TestCheckRetriesServerErrorThenAccepts(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	verdict := Check(context.Background(), newCheckDoer(), testCheckKey, fastCheckOptions(srv.URL))

	require.Equal(t, VerdictAccepted, verdict)
	require.Equal(t, int32(2), requests.Load())
}

func TestCheckServerErrorUntilWindowEndsIsUnknown(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	srv := statusServer(t, http.StatusInternalServerError, &requests)

	verdict := Check(context.Background(), newCheckDoer(), testCheckKey, fastCheckOptions(srv.URL))

	require.Equal(t, VerdictUnknown, verdict)
	require.Greater(t, requests.Load(), int32(1))
}

func TestCheckRateLimitedIsUnknown(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	srv := statusServer(t, http.StatusTooManyRequests, &requests)

	verdict := Check(context.Background(), newCheckDoer(), testCheckKey, fastCheckOptions(srv.URL))

	require.Equal(t, VerdictUnknown, verdict)
	require.Greater(t, requests.Load(), int32(1))
}

// TestCheckRedirectIsUnknown pins that a 3xx is an "other status": the client does
// not follow it, so the check retries and ends Unknown instead of trusting the target.
func TestCheckRedirectIsUnknown(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Location", "/elsewhere")
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(srv.Close)

	verdict := Check(context.Background(), newCheckDoer(), testCheckKey, fastCheckOptions(srv.URL))

	require.Equal(t, VerdictUnknown, verdict)
	require.Greater(t, requests.Load(), int32(1))
}

func TestCheckRefusedConnectionIsUnknown(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := srv.URL
	srv.Close() // nothing listens any more: connection refused

	verdict := Check(context.Background(), newCheckDoer(), testCheckKey, fastCheckOptions(baseURL))

	require.Equal(t, VerdictUnknown, verdict)
}

func TestCheckPerAttemptTimeoutIsUnknown(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	start := time.Now()
	verdict := Check(context.Background(), newCheckDoer(), testCheckKey, fastCheckOptions(srv.URL))

	require.Equal(t, VerdictUnknown, verdict, "a slow LAPI must not be read as a 200 or a 403")
	require.Less(t, time.Since(start), 2*time.Second, "the per-attempt timeout bounds each try")
}

func TestCheckConnectionResetIsUnknown(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	t.Cleanup(srv.Close)

	verdict := Check(context.Background(), newCheckDoer(), testCheckKey, fastCheckOptions(srv.URL))

	require.Equal(t, VerdictUnknown, verdict)
}

func TestCheckCancelledBeforeStartIsUnknownWithoutRequest(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	srv := statusServer(t, http.StatusForbidden, &requests)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	verdict := Check(ctx, newCheckDoer(), testCheckKey, fastCheckOptions(srv.URL))

	require.Equal(t, VerdictUnknown, verdict, "a cancelled context is never evidence about the key")
	require.Zero(t, requests.Load())
}

func TestCheckCancelledDuringAttemptIsUnknown(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	opts := fastCheckOptions(srv.URL)
	opts.Wait = 5 * time.Second
	opts.PerAttempt = 5 * time.Second
	verdict := Check(ctx, newCheckDoer(), testCheckKey, opts)

	require.Equal(t, VerdictUnknown, verdict, "a cancelled check must not report Rejected")
}

func TestCheckEmptyKeyIsRejectedWithoutRequest(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	srv := statusServer(t, http.StatusOK, &requests)

	verdict := Check(context.Background(), newCheckDoer(), "", fastCheckOptions(srv.URL))

	require.Equal(t, VerdictRejected, verdict)
	require.Zero(t, requests.Load())
}
