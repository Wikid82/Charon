package bouncerkey

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// countingDoer forwards to inner and counts the requests it sends.
type countingDoer struct {
	inner Doer
	calls atomic.Int32
}

func (d *countingDoer) Do(req *http.Request) (*http.Response, error) {
	d.calls.Add(1)
	return d.inner.Do(req)
}

// TestCheckMalformedBaseURLIsUnknownWithoutRequest covers the request-construction error arm: a base URL
// with a control character cannot form a request, so no request is sent and the answer is unknown.
func TestCheckMalformedBaseURLIsUnknownWithoutRequest(t *testing.T) {
	t.Parallel()

	doer := &countingDoer{inner: newCheckDoer()}
	opts := fastCheckOptions("http://127.0.0.1:8085\x7f")

	verdict := Check(context.Background(), doer, testCheckKey, opts)

	require.Equal(t, VerdictUnknown, verdict)
	require.Zero(t, doer.calls.Load())
}

// TestCheckCancelledDuringBackoffIsUnknown covers the cancellation arm of the backoff wait: the first
// attempt gets a server error, the check then sleeps, and cancellation during that sleep ends it at once.
func TestCheckCancelledDuringBackoffIsUnknown(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	srv := statusServer(t, http.StatusInternalServerError, &requests)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	time.AfterFunc(100*time.Millisecond, cancel)

	opts := fastCheckOptions(srv.URL)
	opts.Wait = 10 * time.Second
	opts.FirstBackoff = 10 * time.Second
	opts.MaxBackoff = 10 * time.Second

	start := time.Now()
	verdict := Check(ctx, newCheckDoer(), testCheckKey, opts)

	require.Equal(t, VerdictUnknown, verdict)
	require.Less(t, time.Since(start), 5*time.Second, "cancellation must cut the backoff sleep short")
	require.Equal(t, int32(1), requests.Load())
}
