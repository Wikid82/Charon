package crowdsec

import (
	"context"
	"errors"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A CDN request that fails at the transport layer (no HTTP status) falls through to the GitHub mirror.
func TestFetchIndexFallsBackToMirrorOnCDNTransportError(t *testing.T) {
	transport := &hubTransport{
		errs:   map[string]error{cdnIndexURL: errors.New("connection reset by peer")},
		bodies: map[string]string{rawIndexURL: readCSCLIFixture(t, "hub_index_raw_sample.json")},
	}
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: transport}

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.Contains(t, itemNames(idx), "crowdsecurity/sshd")
	require.Equal(t, []string{cdnIndexURL, rawIndexURL}, transport.seen())
}

// A CDN request that times out (a per-request timeout, not the caller's deadline) falls through to the mirror.
func TestFetchIndexFallsBackToMirrorOnCDNTimeout(t *testing.T) {
	transport := &hubTransport{
		errs:   map[string]error{cdnIndexURL: os.ErrDeadlineExceeded},
		bodies: map[string]string{rawIndexURL: readCSCLIFixture(t, "hub_index_raw_sample.json")},
	}
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: transport}

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.Contains(t, itemNames(idx), "crowdsecurity/sshd")
	require.Equal(t, []string{cdnIndexURL, rawIndexURL}, transport.seen())
}

// When the caller's context is cancelled or its deadline has passed, the candidate loop stops at the first failure.
func TestFetchIndexStopsWhenCallerContextEnds(t *testing.T) {
	cases := []struct {
		name     string
		ctx      func() (context.Context, context.CancelFunc)
		failWith error
	}{
		{
			name: "cancelled",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			failWith: context.Canceled,
		},
		{
			name: "deadline expired",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			},
			failWith: context.DeadlineExceeded,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := tc.ctx()
			defer cancel()
			var seen []string
			svc := NewHubService(nil, nil, t.TempDir())
			svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				seen = append(seen, req.URL.String())
				return nil, &neturl.Error{Op: req.Method, URL: req.URL.String(), Err: tc.failWith}
			})}

			_, err := svc.FetchIndex(ctx)
			require.Error(t, err)
			require.ErrorIs(t, err, tc.failWith)
			require.Equal(t, []string{cdnIndexURL}, seen, "no candidate is tried after the caller gave up")
		})
	}
}

// When every candidate fails, each URL appears exactly once in the aggregated error, with transport and HTTP failures alike.
func TestHubIndexErrorNamesEachURLOnce(t *testing.T) {
	captureHubIndexLogs(t)
	mirrorAPI := defaultHubMirrorBaseURL + defaultHubIndexPath
	transport := &hubTransport{
		errs:   map[string]error{cdnIndexURL: errors.New("connection refused")},
		status: map[string]int{rawIndexURL: http.StatusServiceUnavailable, mirrorAPI: http.StatusNotFound},
	}
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: transport}

	_, err := svc.FetchIndex(context.Background())
	require.Error(t, err)

	msg := err.Error()
	for _, u := range []string{cdnIndexURL, rawIndexURL, mirrorAPI} {
		require.Equal(t, 1, strings.Count(msg, u), "URL %s must appear exactly once in: %s", u, msg)
	}
	require.Contains(t, msg, "connection refused")
	require.Contains(t, msg, "status 503")
	require.Contains(t, msg, "status 404")
}

// An operator-set hub URL that does not parse is logged as a placeholder, never echoed back.
func TestHubURLForLogMarksUnparseableURL(t *testing.T) {
	require.Equal(t, "<invalid hub URL>", hubURLForLog("https://[::1"))
}

// A body that fails to close is logged, and the index it carried is still used.
func TestFetchIndexLogsBodyCloseFailure(t *testing.T) {
	logs := captureHubIndexLogs(t)
	body := readCSCLIFixture(t, "hub_index_raw_sample.json")
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		resp := newResponse(http.StatusOK, body)
		resp.Header.Set("Content-Type", "application/json")
		resp.Body = failingCloseBody{Reader: strings.NewReader(body)}
		return resp, nil
	})}

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, idx.Items)
	require.Contains(t, logs.String(), "Failed to close response body")
}

// failingCloseBody reads its content and then fails on Close.
type failingCloseBody struct {
	io.Reader
}

func (failingCloseBody) Close() error { return errors.New("close failed") }
