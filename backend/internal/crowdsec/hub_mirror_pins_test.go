package crowdsec

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The tests below pin behaviour that later commits of the hub outage fallback implement. Each commit removes the
// skip for its own pins.

const pinMirrorItemURL = "https://raw.githubusercontent.com/crowdsecurity/hub/master/collections/demo.yaml"

// pinHubService serves a hub whose CDN index never answers. When mirrorHangs is set the mirror hangs too; otherwise
// the mirror serves an index listing the demo item and the item itself. PullTimeout is 12 s, so the CDN attempt gets
// its 5 s floor and the rest of the walk is left for the mirror. Every request is recorded in the returned slice.
func pinHubService(t *testing.T, mirrorHangs bool) (svc *HubService, requested *[]string) {
	t.Helper()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	svc = NewHubService(nil, cache, t.TempDir())
	svc.PullTimeout = 12 * time.Second

	var mu sync.Mutex
	var requests []string
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		requests = append(requests, req.URL.String())
		mu.Unlock()
		switch {
		case req.URL.String() == cdnIndexURL:
			return hangUntilDone(req)
		case mirrorHangs && req.URL.String() == rawIndexURL:
			return hangUntilDone(req)
		case req.URL.String() == rawIndexURL:
			resp := newResponse(http.StatusOK, `{"items":[`+demoEntry("collections", pinMirrorItemURL)+`]}`)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		case req.URL.String() == pinMirrorItemURL:
			return newResponse(http.StatusOK, pullDemoItem), nil
		default:
			return newResponse(http.StatusNotFound, ""), nil
		}
	})}
	return svc, &requests
}

// hangUntilDone holds a request until its context ends, the way a dropped connection does.
func hangUntilDone(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, req.Context().Err()
}

func TestHubPinPullTriesMirrorWhenFirstAddressHangs(t *testing.T) {
	t.Parallel()
	svc, _ := pinHubService(t, false)

	start := time.Now()
	res, err := svc.Pull(context.Background(), "crowdsecurity/demo", "collections")
	require.NoError(t, err)
	require.Equal(t, pullDemoItem, res.Preview)
	require.Less(t, time.Since(start), svc.PullTimeout, "the mirror answers inside the pull budget")
}

func TestHubPinFetchIndexWithoutDeadlineBoundedWhenBothAddressesHang(t *testing.T) {
	t.Parallel()
	svc, requested := pinHubService(t, true)

	start := time.Now()
	_, err := svc.FetchIndex(context.Background())
	elapsed := time.Since(start)

	require.ErrorIs(t, err, ErrHubUnreachable)
	require.Equal(t, []string{cdnIndexURL, rawIndexURL}, *requested, "both addresses are tried")
	require.Less(t, elapsed, svc.PullTimeout, "the walk ends inside its own budget")
}

func TestHubPinCSCLIRetriesOnMirrorAfterNetworkFailure(t *testing.T) {
	t.Parallel()
	svc, live := mirrorTestSetup(t)
	exec := &scriptedCSCLI{respond: func(_ context.Context, n int, args []string) ([]byte, error) {
		if n == 0 {
			require.Equal(t, live, configFlag(args))
			return []byte(dialFailureText), errLiveCSCLI
		}
		return nil, nil
	}}
	svc.Exec = exec

	_, err := svc.runHubCSCLI(context.Background(), "collections", "install", "demo")
	require.NoError(t, err)
	calls := exec.recorded()
	require.Len(t, calls, 3)
	require.True(t, isDerivedConfig(configFlag(calls[2])), "the install is retried under the mirror configuration")
}

func TestHubPinCSCLIDoesNotRetryOnNonNetworkFailure(t *testing.T) {
	t.Parallel()
	svc, _ := mirrorTestSetup(t)
	exec := &scriptedCSCLI{respond: func(_ context.Context, _ int, _ []string) ([]byte, error) {
		return []byte("can't find collections/nope"), errLiveCSCLI
	}}
	svc.Exec = exec

	_, err := svc.runHubCSCLI(context.Background(), "collections", "install", "nope")
	require.ErrorIs(t, err, errLiveCSCLI)
	require.Len(t, exec.recorded(), 1)
}

// pinCSCLIIndexExec answers the local cscli listing with one item and fails everything else.
type pinCSCLIIndexExec struct{}

func (pinCSCLIIndexExec) Execute(_ context.Context, _ string, args ...string) ([]byte, error) {
	if len(args) > 0 && args[len(args)-1] == "json" {
		return []byte(`{"collections":[{"name":"crowdsecurity/demo","title":"Demo","type":"collections"}]}`), nil
	}
	return nil, errors.New("unexpected cscli call")
}

// The index says whether the local cscli data answered instead of the HTTP index; the preset list reads that flag.
func TestHubPinPresetListReportsHubOffline(t *testing.T) {
	t.Parallel()
	svc := NewHubService(pinCSCLIIndexExec{}, nil, t.TempDir())

	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial tcp: no route to host")
	})}
	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.True(t, idx.Offline(), "the cscli listing answered for the HTTP index")
	require.NotEmpty(t, idx.Items)

	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		resp := newResponse(http.StatusOK, `{"items":[`+demoEntry("collections", pinMirrorItemURL)+`]}`)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	})}
	idx, err = svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.False(t, idx.Offline(), "the HTTP index answered")
}
