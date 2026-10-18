package crowdsec

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// countingIndexService serves the given index entries and counts the index requests.
func countingIndexService(t *testing.T, entries string) (svc *HubService, indexRequests *atomic.Int32) {
	t.Helper()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	indexRequests = &atomic.Int32{}
	svc = NewHubService(nil, cache, t.TempDir())
	svc.HubBaseURL = "https://hub.crowdsec.net"
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://hub.crowdsec.net" + defaultHubIndexPath:
			indexRequests.Add(1)
			return newResponse(http.StatusOK, `{"items":[`+entries+`]}`), nil
		case "https://hub.crowdsec.net/demo.yaml":
			return newResponse(http.StatusOK, pullDemoItem), nil
		default:
			return newResponse(http.StatusNotFound, ""), nil
		}
	})}
	return svc, indexRequests
}

// A pull without a type asks the index which section holds the name. A second pull of the same name inside the
// ten-minute window is served from the cache without asking again.
func TestSecondUntypedPullInsideTheWindowMakesNoIndexRequest(t *testing.T) {
	t.Parallel()
	svc, indexRequests := countingIndexService(t, demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"))

	_, err := svc.Pull(context.Background(), "crowdsecurity/demo", "")
	require.NoError(t, err)
	require.Equal(t, int32(1), indexRequests.Load())

	res, err := svc.Pull(context.Background(), "crowdsecurity/demo", "")
	require.NoError(t, err)
	require.Equal(t, pullDemoItem, res.Preview)
	require.Equal(t, int32(1), indexRequests.Load(), "the second pull must not fetch the index")
}

// A name that exists in several sections stays ambiguous however many of its sections are cached and however
// recently the index was read: only the index knows about the sections that are not cached.
func TestSharedNamesStayAmbiguousWithoutAType(t *testing.T) {
	t.Parallel()
	entries := `{"name":"crowdsecurity/demo","type":"appsec-rules","etag":"e1","download_url":"https://hub.crowdsec.net/demo.yaml"},` +
		`{"name":"crowdsecurity/demo","type":"appsec-configs","etag":"e2","download_url":"https://hub.crowdsec.net/demo.yaml"}`
	svc, _ := countingIndexService(t, entries)
	ctx := context.Background()

	_, err := svc.Pull(ctx, "crowdsecurity/demo", "appsec-rules")
	require.NoError(t, err)
	for range 2 {
		_, err = svc.Pull(ctx, "crowdsecurity/demo", "")
		var ambiguous *AmbiguousTypeError
		require.ErrorAs(t, err, &ambiguous, "one cached section must not hide the other")
		require.ErrorIs(t, err, ErrPresetTypeAmbiguous)
		require.Equal(t, []string{"appsec-configs", "appsec-rules"}, ambiguous.Types)
	}

	_, err = svc.Pull(ctx, "crowdsecurity/demo", "appsec-configs")
	require.NoError(t, err)
	_, err = svc.Pull(ctx, "crowdsecurity/demo", "")
	require.ErrorIs(t, err, ErrPresetTypeAmbiguous, "both sections cached")
}

// fakeClock is a settable clock for the index window.
type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time { return c.t }

func TestUntypedPullRefetchesIndexAfterTheWindowExpires(t *testing.T) {
	t.Parallel()
	svc, indexRequests := countingIndexService(t, demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"))
	clk := &fakeClock{t: time.Unix(1_000_000, 0)}
	svc.now = clk.Now
	ctx := context.Background()

	_, err := svc.Pull(ctx, "crowdsecurity/demo", "")
	require.NoError(t, err)
	clk.t = clk.t.Add(indexFreshTTL - time.Second)
	_, err = svc.Pull(ctx, "crowdsecurity/demo", "")
	require.NoError(t, err)
	require.Equal(t, int32(1), indexRequests.Load(), "inside the window")

	clk.t = clk.t.Add(time.Second)
	_, err = svc.Pull(ctx, "crowdsecurity/demo", "")
	require.NoError(t, err)
	require.Equal(t, int32(2), indexRequests.Load(), "window expired")
}

func TestAmbiguousNameStaysAmbiguousWithFreshAndStaleIndex(t *testing.T) {
	t.Parallel()
	entries := `{"name":"crowdsecurity/demo","type":"appsec-rules","etag":"e1","download_url":"https://hub.crowdsec.net/demo.yaml"},` +
		`{"name":"crowdsecurity/demo","type":"appsec-configs","etag":"e2","download_url":"https://hub.crowdsec.net/demo.yaml"}`
	svc, _ := countingIndexService(t, entries)
	clk := &fakeClock{t: time.Unix(1_000_000, 0)}
	svc.now = clk.Now
	ctx := context.Background()

	_, err := svc.Pull(ctx, "crowdsecurity/demo", "") // fetches the index; ambiguous
	require.ErrorIs(t, err, ErrPresetTypeAmbiguous)
	_, err = svc.Pull(ctx, "crowdsecurity/demo", "appsec-rules")
	require.NoError(t, err)

	for _, advance := range []time.Duration{0, indexFreshTTL + time.Minute} {
		clk.t = clk.t.Add(advance)
		_, err = svc.Pull(ctx, "crowdsecurity/demo", "")
		var ambiguous *AmbiguousTypeError
		require.ErrorAs(t, err, &ambiguous)
		require.Equal(t, []string{"appsec-configs", "appsec-rules"}, ambiguous.Types)
	}
}

func TestFailedIndexFetchDoesNotStartTheWindow(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	svc := NewHubService(nil, cache, t.TempDir())
	svc.HubBaseURL = "https://hub.crowdsec.net"
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return newResponse(http.StatusInternalServerError, ""), nil
	})}
	svc.now = (&fakeClock{t: time.Unix(1_000_000, 0)}).Now

	_, err = svc.Pull(context.Background(), "crowdsecurity/demo", "")
	require.Error(t, err)
	require.True(t, svc.lastIndexAt.IsZero())
}

func TestLegacySweepRunsOncePerProcess(t *testing.T) {
	t.Parallel()
	svc, _ := countingIndexService(t, demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"))
	ctx := context.Background()
	legacy := filepath.Join(svc.Cache.baseDir, "legacy-entry")

	require.NoError(t, os.MkdirAll(legacy, 0o750))
	_, err := svc.Pull(ctx, "crowdsecurity/demo", "collections")
	require.NoError(t, err)
	require.NoDirExists(t, legacy, "first pull sweeps")

	require.NoError(t, os.MkdirAll(legacy, 0o750))
	_, err = svc.Pull(ctx, "crowdsecurity/demo", "collections")
	require.NoError(t, err)
	require.DirExists(t, legacy, "later pulls do not sweep")
}
