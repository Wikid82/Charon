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

func TestLegacySweepRetriesAfterAFailedFirstRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cache, err := NewHubCache(dir, time.Hour)
	require.NoError(t, err)
	svc := NewHubService(nil, cache, t.TempDir())
	ctx := context.Background()

	require.NoError(t, os.RemoveAll(dir))
	svc.sweepLegacyCache(ctx) // fails: the cache root is gone
	require.False(t, svc.legacySweepDone, "a failed sweep must not consume the run")

	legacy := filepath.Join(dir, "legacy-entry")
	require.NoError(t, os.MkdirAll(legacy, 0o750))
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	svc.sweepLegacyCache(canceled) // a cancelled caller does not abort the sweep
	require.True(t, svc.legacySweepDone)
	require.NoDirExists(t, legacy)

	require.NoError(t, os.MkdirAll(legacy, 0o750))
	svc.sweepLegacyCache(ctx)
	require.DirExists(t, legacy, "never runs again after success")
}

func TestFreshCachedTypeRefusesAnIndexThatDisagreesWithTheCache(t *testing.T) {
	t.Parallel()
	svc, _ := countingIndexService(t, demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"))
	clk := &fakeClock{t: time.Unix(1_000_000, 0)}
	svc.now = clk.Now
	ctx := context.Background()
	const slug = "crowdsecurity/demo"

	_, err := svc.Pull(ctx, slug, "")
	require.NoError(t, err)
	typ, ok := svc.freshCachedType(ctx, slug)
	require.True(t, ok)
	require.Equal(t, "collections", typ)

	// The retained index no longer lists the name.
	svc.lastIndex = HubIndex{Items: []HubIndexEntry{{Name: "crowdsecurity/other", Type: "collections"}}}
	_, ok = svc.freshCachedType(ctx, slug)
	require.False(t, ok, "name gone from the index")

	// The retained index moved the name to a different section than the cached one.
	svc.lastIndex = HubIndex{Items: []HubIndexEntry{{Name: slug, Type: "parsers"}}}
	_, ok = svc.freshCachedType(ctx, slug)
	require.False(t, ok, "section moved")
}

func TestUntypedPullReportsNotFoundWhenTheFreshIndexDropsTheName(t *testing.T) {
	t.Parallel()
	var entries atomic.Value
	entries.Store(demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"))
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	svc := NewHubService(nil, cache, t.TempDir())
	svc.HubBaseURL = "https://hub.crowdsec.net"
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == "https://hub.crowdsec.net/demo.yaml" {
			return newResponse(http.StatusOK, pullDemoItem), nil
		}
		return newResponse(http.StatusOK, `{"items":[`+entries.Load().(string)+`]}`), nil
	})}
	ctx := context.Background()

	_, err = svc.Pull(ctx, "crowdsecurity/demo", "")
	require.NoError(t, err)
	entries.Store(`{"name":"crowdsecurity/other","type":"collections","etag":"e","download_url":"https://hub.crowdsec.net/demo.yaml"}`)
	svc.lastIndex = HubIndex{Items: []HubIndexEntry{{Name: "crowdsecurity/other", Type: "collections"}}}

	_, err = svc.Pull(ctx, "crowdsecurity/demo", "")
	require.ErrorContains(t, err, "preset not found")
}
