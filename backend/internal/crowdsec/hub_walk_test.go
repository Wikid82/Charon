package crowdsec

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// stalledBody answers the headers of a response and then never sends the body, until the request's context ends.
type stalledBody struct{ ctx context.Context }

func (b stalledBody) Read([]byte) (int, error) {
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}

func (stalledBody) Close() error { return nil }

// The index walk keeps the cscli listing reserve back when it has room for it, and never runs past its parent.
func TestWalkContextSetsAsideCSCLIReserve(t *testing.T) {
	t.Parallel()

	t.Run("no deadline uses the total less the reserve", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := walkContext(context.Background(), 25*time.Second)
		defer cancel()
		require.InDelta(t, float64(23*time.Second), float64(remainingWithin(t, ctx)), float64(time.Second))
	})

	t.Run("zero total falls back to the default pull timeout", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := walkContext(context.Background(), 0)
		defer cancel()
		require.InDelta(t, float64(defaultPullTimeout-cscliListReserve), float64(remainingWithin(t, ctx)), float64(time.Second))
	})

	t.Run("a caller deadline with room keeps the reserve", func(t *testing.T) {
		t.Parallel()
		parent, cancelParent := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelParent()
		ctx, cancel := walkContext(parent, time.Second)
		defer cancel()
		require.InDelta(t, float64(28*time.Second), float64(remainingWithin(t, ctx)), float64(time.Second))
	})

	t.Run("a caller deadline within two floors is the walk's limit", func(t *testing.T) {
		t.Parallel()
		parent, cancelParent := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancelParent()
		ctx, cancel := walkContext(parent, time.Second)
		defer cancel()
		deadline, _ := ctx.Deadline()
		parentDeadline, _ := parent.Deadline()
		require.Equal(t, parentDeadline, deadline)
	})
}

// A hub index walk whose address hangs gives the next address its share and falls through to it.
func TestFetchIndexHangingFirstAddressFallsThroughToMirror(t *testing.T) {
	t.Parallel()
	svc, requested := pinHubService(t, false)
	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.Contains(t, itemNames(idx), "crowdsecurity/demo")
	require.Equal(t, []string{cdnIndexURL, rawIndexURL}, *requested)
}

// The caller's own deadline ends the walk, so the failure is not reported as the hub being unreachable.
func TestFetchIndexCallerDeadlineIsNotWrappedAsUnreachable(t *testing.T) {
	t.Parallel()
	parent, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(hangUntilDone)}

	_, err := svc.FetchIndex(parent)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrHubUnreachable)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

// With no deadline on the caller, two hanging addresses end inside the walk budget as hub unreachable.
func TestFetchIndexWithoutDeadlineStaysWithinPullTimeout(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	svc.PullTimeout = 600 * time.Millisecond
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(hangUntilDone)}

	start := time.Now()
	_, err := svc.FetchIndex(context.Background())
	require.ErrorIs(t, err, ErrHubUnreachable)
	require.Less(t, time.Since(start), svc.PullTimeout+time.Second, "the walk does not run to 1.5 times its budget")
}

// A body that stalls after the headers uses up the attempt's share, and that counts as a transport failure, so the
// next address is tried.
func TestFetchIndexStalledBodyFallsBackToMirror(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	svc.PullTimeout = 12 * time.Second
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case cdnIndexURL:
			resp := newResponse(http.StatusOK, "")
			resp.Header.Set("Content-Type", "application/json")
			resp.Body = stalledBody{ctx: req.Context()}
			return resp, nil
		case rawIndexURL:
			resp := newResponse(http.StatusOK, readCSCLIFixture(t, "hub_index_raw_sample.json"))
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		default:
			return newResponse(http.StatusNotFound, ""), nil
		}
	})}

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.Contains(t, itemNames(idx), "crowdsecurity/sshd")
}

// The local cscli listing still runs after both addresses hang: the walk leaves it the reserve.
func TestFetchIndexCSCLIListingAnswersAfterBothAddressesHang(t *testing.T) {
	t.Parallel()
	exec := &recordingExec{outputs: map[string][]byte{
		"cscli hub list -a -o json": []byte(readCSCLIFixture(t, "hub_list_all.json")),
	}}
	svc := NewHubService(exec, nil, t.TempDir())
	svc.PullTimeout = 12 * time.Second
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(hangUntilDone)}

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.True(t, idx.Offline())
	require.NotEmpty(t, idx.Items)
	require.Contains(t, exec.calls, "cscli hub list -a -o json")
}
