package crowdsec

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The tests below pin behaviour that later commits of the hub outage fallback implement. Each commit removes the
// skip for its own pins; the commit-4 and commit-6 pins stay skipped here.

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
	t.Skip("enabled in commit 4")
}

func TestHubPinCSCLIDoesNotRetryOnNonNetworkFailure(t *testing.T) {
	t.Skip("enabled in commit 4")
}

func TestHubPinPresetListReportsHubOffline(t *testing.T) {
	t.Skip("enabled in commit 6")
}
