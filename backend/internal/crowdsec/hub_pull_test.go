package crowdsec

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const pullDemoItem = "name: crowdsecurity/demo\ndescription: demo\n"

// pullService serves an index with one entry and the given item download.
func pullService(t *testing.T, entryJSON string, item func() (status int, body string)) (svc *HubService, downloads *atomic.Int32) {
	t.Helper()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	downloads = &atomic.Int32{}
	svc = NewHubService(nil, cache, t.TempDir())
	svc.HubBaseURL = "https://hub.crowdsec.net"
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://hub.crowdsec.net" + defaultHubIndexPath:
			return newResponse(http.StatusOK, `{"items":[`+entryJSON+`]}`), nil
		case "https://hub.crowdsec.net/demo.yaml":
			downloads.Add(1)
			status, body := item()
			return newResponse(status, body), nil
		default:
			return newResponse(http.StatusNotFound, ""), nil
		}
	})}
	return svc, downloads
}

func demoEntry(typ, downloadURL string) string {
	return `{"name":"crowdsecurity/demo","title":"Demo","type":"` + typ + `","etag":"etag1","download_url":"` + downloadURL + `"}`
}

func yamlResponse(body string) func() (status int, text string) {
	return func() (int, string) { return http.StatusOK, body }
}

func TestPullStoresRawItem(t *testing.T) {
	t.Parallel()
	svc, _ := pullService(t, demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), yamlResponse(pullDemoItem))

	res, err := svc.Pull(context.Background(), "crowdsecurity/demo", "")
	require.NoError(t, err)
	require.Equal(t, pullDemoItem, res.Preview)
	require.Equal(t, "crowdsecurity/demo", res.Meta.Slug)
	require.Equal(t, "collections", res.Meta.Type)
	require.Equal(t, "etag1", res.Meta.Etag)
	require.Equal(t, "hub", res.Meta.Source)
	require.NotEmpty(t, res.Meta.ItemSHA256)

	cached, item, err := svc.Cache.LoadItem(context.Background(), "collections", "crowdsecurity/demo")
	require.NoError(t, err)
	require.Equal(t, res.Meta.CacheKey, cached.CacheKey)
	require.Equal(t, pullDemoItem, string(item))
}

func TestPullReturnsValidCacheEntryWithoutNetwork(t *testing.T) {
	t.Parallel()
	svc, downloads := pullService(t, demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), yamlResponse(pullDemoItem))
	first, err := svc.Pull(context.Background(), "crowdsecurity/demo", "")
	require.NoError(t, err)
	svc.HTTPClient = nil

	second, err := svc.Pull(context.Background(), "crowdsecurity/demo", "")
	require.NoError(t, err)
	require.Equal(t, first.Meta.CacheKey, second.Meta.CacheKey)
	require.Equal(t, int32(1), downloads.Load())
}

func TestPullEvictsExpiredEntryAndRefreshes(t *testing.T) {
	t.Parallel()
	svc, downloads := pullService(t, demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), yamlResponse(pullDemoItem))
	svc.Cache.ttl = time.Second
	first, err := svc.Pull(context.Background(), "crowdsecurity/demo", "")
	require.NoError(t, err)
	svc.Cache.nowFn = func() time.Time { return time.Now().Add(time.Minute) }

	second, err := svc.Pull(context.Background(), "crowdsecurity/demo", "")
	require.NoError(t, err)
	require.Equal(t, int32(2), downloads.Load())
	require.Equal(t, first.Meta.ItemSHA256, second.Meta.ItemSHA256)
}

func TestPullRedownloadsLegacyEntry(t *testing.T) {
	t.Parallel()
	svc, downloads := pullService(t, demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), yamlResponse(pullDemoItem))
	dir := svc.Cache.baseDir + "/crowdsecurity/demo"
	// A pre-upgrade entry also carries a leftover archive file; the fresh pull replaces the whole entry directory.
	writeFile(t, dir+"/metadata.json", `{"slug":"crowdsecurity/demo","cache_key":"crowdsecurity/demo-1","retrieved_at":"2099-01-01T00:00:00Z"}`)
	writeFile(t, dir+"/bundle.tgz", "archive")

	res, err := svc.Pull(context.Background(), "crowdsecurity/demo", "")
	require.NoError(t, err)
	require.Equal(t, int32(1), downloads.Load())
	require.Contains(t, res.Meta.CacheKey, res.Meta.ItemSHA256)
	require.NoFileExists(t, dir+"/bundle.tgz")
}

func TestPullRefusesUnusablePayloads(t *testing.T) {
	t.Parallel()
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, err := zw.Write([]byte(pullDemoItem))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	cases := map[string]struct {
		entry   string
		item    func() (status int, body string)
		message string
	}{
		"empty download location": {demoEntry("collections", ""), yamlResponse(pullDemoItem), "no download location"},
		"unsupported item type":   {demoEntry("links", "https://hub.crowdsec.net/demo.yaml"), yamlResponse(pullDemoItem), "unsupported hub item type"},
		"HTML page served as 200": {demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), yamlResponse("<!DOCTYPE html><html><body>Not here</body></html>"), "not a YAML mapping"},
		"archive payload":         {demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), yamlResponse(gz.String()), "only raw item files are supported"},
		"YAML list":               {demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), yamlResponse("- a\n- b\n"), "not a YAML mapping"},
		"YAML scalar":             {demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), yamlResponse("just text"), "not a YAML mapping"},
		"two YAML documents":      {demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), yamlResponse("a: 1\n---\nb: 2\n"), "not a YAML mapping"},
		"invalid YAML":            {demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), yamlResponse("a: [unclosed"), "not a YAML mapping"},
		"empty body":              {demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), yamlResponse(""), "not a YAML mapping"},
		"oversize body":           {demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), yamlResponse("a: " + strings.Repeat("b", maxCacheItemBytes)), "payload too large"},
		"download fails":          {demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), func() (int, string) { return http.StatusInternalServerError, "" }, "download preset"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, _ := pullService(t, tc.entry, tc.item)

			_, err := svc.Pull(context.Background(), "crowdsecurity/demo", "")
			require.ErrorContains(t, err, tc.message)
			entries, listErr := svc.Cache.List(context.Background())
			require.NoError(t, listErr)
			require.Empty(t, entries, "a refused payload is never cached")
		})
	}
}

func TestPullRefusesMalformedSlugsWithoutNetwork(t *testing.T) {
	t.Parallel()
	svc, downloads := pullService(t, demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), yamlResponse(pullDemoItem))
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		downloads.Add(1)
		return nil, errors.New("no network expected")
	})}
	for _, slug := range []string{"", " ", "solo", "owner/name+extra", "owner/team/deep", "../x/y", "a//b", "a/metadata.json"} {
		_, err := svc.Pull(context.Background(), slug, "")
		require.ErrorIs(t, err, ErrInvalidSlug, slug)
	}
	require.Zero(t, downloads.Load())
}

func TestPullRequiresCache(t *testing.T) {
	t.Parallel()
	_, err := NewHubService(nil, nil, t.TempDir()).Pull(context.Background(), "crowdsecurity/demo", "")
	require.ErrorContains(t, err, "cache unavailable")
}

func TestPullReportsUnknownPresetAndIndexErrors(t *testing.T) {
	t.Parallel()
	svc, _ := pullService(t, demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), yamlResponse(pullDemoItem))
	_, err := svc.Pull(context.Background(), "crowdsecurity/missing", "")
	require.ErrorContains(t, err, "preset not found in hub")

	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return newResponse(http.StatusServiceUnavailable, ""), nil
	})}
	_, err = svc.Pull(context.Background(), "crowdsecurity/demo", "")
	require.Error(t, err)
}

func TestPullStopsOnCanceledContext(t *testing.T) {
	t.Parallel()
	svc, _ := pullService(t, demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), yamlResponse(pullDemoItem))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := svc.Pull(ctx, "crowdsecurity/demo", "")
	require.ErrorIs(t, err, context.Canceled)
}

// The index fallback worked, but the preset is not cached and the fallback cannot say where to download it: that is
// an unreachable hub, not a missing preset.
func TestPullOfflineIndexHasNoDownloadLocation(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	exec := &recordingExec{outputs: map[string][]byte{"cscli hub list -a -o json": []byte(readCSCLIFixture(t, "hub_list_all.json"))}}
	svc := NewHubService(exec, cache, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network unreachable")
	})}

	_, err = svc.Pull(context.Background(), "crowdsecurity/sshd", "")
	require.ErrorIs(t, err, ErrHubUnreachable)
	require.ErrorContains(t, err, "no download location for this preset")
}

func TestPullDifferentContentInTheSameSecondGetsDifferentKeys(t *testing.T) {
	t.Parallel()
	body := pullDemoItem
	svc, _ := pullService(t, demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), func() (int, string) { return http.StatusOK, body })
	fixed := time.Now()
	svc.Cache.nowFn = func() time.Time { return fixed }

	first, err := svc.Pull(context.Background(), "crowdsecurity/demo", "")
	require.NoError(t, err)
	require.NoError(t, svc.Cache.Evict(context.Background(), "collections", "crowdsecurity/demo"))
	body = "name: crowdsecurity/demo\ndescription: changed\n"
	second, err := svc.Pull(context.Background(), "crowdsecurity/demo", "")
	require.NoError(t, err)
	require.NotEqual(t, first.Meta.CacheKey, second.Meta.CacheKey)
}

// Every name in the recorded index passes the cache slug rule; irregular names are refused by Pull and
// left out of the listing.
func TestIndexFixtureNamesFollowTheSlugRule(t *testing.T) {
	t.Parallel()
	fixtureNames := func(name string) []string {
		var doc map[string]map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(readFixture(t, name)), &doc))
		var names []string
		for _, section := range doc {
			for n := range section {
				names = append(names, n)
			}
		}
		sort.Strings(names)
		return names
	}

	names := fixtureNames("hub_index_fixture.json")
	require.NotEmpty(t, names)
	for _, n := range names {
		require.True(t, ValidCacheSlug(n), n)
	}

	irregular := fixtureNames("hub_index_irregular_names.json")
	require.Len(t, irregular, 4)
	svc, downloads := pullService(t, demoEntry("collections", "https://hub.crowdsec.net/demo.yaml"), yamlResponse(pullDemoItem))
	for _, n := range irregular {
		if ValidCacheSlug(n) {
			continue
		}
		_, err := svc.Pull(context.Background(), n, "")
		require.ErrorIs(t, err, ErrInvalidSlug, n)
	}
	require.Zero(t, downloads.Load())
	cached, err := svc.Cache.List(context.Background())
	require.NoError(t, err)
	require.Empty(t, cached)

	idx, err := parseRawIndex([]byte(readFixture(t, "hub_index_irregular_names.json")), "https://raw.githubusercontent.com/crowdsecurity/hub/master/.index.json")
	require.NoError(t, err)
	require.Equal(t, []string{"crowdsecurity/demo"}, itemNames(idx))
}

func TestFetchWithLimitFromURL(t *testing.T) {
	t.Parallel()
	t.Run("payload over the limit", func(t *testing.T) {
		t.Parallel()
		svc := NewHubService(nil, nil, t.TempDir())
		svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return newResponse(http.StatusOK, strings.Repeat("a", 11)), nil
		})}
		_, err := svc.fetchWithLimitFromURL(context.Background(), "https://hub.crowdsec.net/large.yaml", 10)
		require.ErrorContains(t, err, "payload too large")
	})
	t.Run("payload at the limit", func(t *testing.T) {
		t.Parallel()
		svc := NewHubService(nil, nil, t.TempDir())
		svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return newResponse(http.StatusOK, strings.Repeat("a", 10)), nil
		})}
		data, err := svc.fetchWithLimitFromURL(context.Background(), "https://hub.crowdsec.net/ok.yaml", 10)
		require.NoError(t, err)
		require.Len(t, data, 10)
	})
	t.Run("status error", func(t *testing.T) {
		t.Parallel()
		svc := NewHubService(nil, nil, t.TempDir())
		svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return newResponse(http.StatusNotFound, ""), nil
		})}
		_, err := svc.fetchWithLimitFromURL(context.Background(), "https://hub.crowdsec.net/none.yaml", 10)
		var herr hubHTTPError
		require.ErrorAs(t, err, &herr)
		require.Equal(t, http.StatusNotFound, herr.statusCode)
	})
	t.Run("transport error", func(t *testing.T) {
		t.Parallel()
		svc := NewHubService(nil, nil, t.TempDir())
		svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})}
		_, err := svc.fetchWithLimitFromURL(context.Background(), "https://hub.crowdsec.net/x.yaml", 10)
		require.ErrorContains(t, err, "request")
	})
	t.Run("missing client", func(t *testing.T) {
		t.Parallel()
		svc := NewHubService(nil, nil, t.TempDir())
		svc.HTTPClient = nil
		_, err := svc.fetchWithLimitFromURL(context.Background(), "https://hub.crowdsec.net/demo.yaml", 10)
		require.ErrorContains(t, err, "http client missing")
	})
	t.Run("host outside the allowlist", func(t *testing.T) {
		t.Parallel()
		svc := NewHubService(nil, nil, t.TempDir())
		_, err := svc.fetchWithLimitFromURL(context.Background(), "https://example.invalid/demo.yaml", 10)
		require.ErrorContains(t, err, "invalid hub URL")
	})
}

func TestCheckHubItem(t *testing.T) {
	t.Parallel()
	require.NoError(t, checkHubItem([]byte("a: 1\n")))
	require.Error(t, checkHubItem([]byte{0x1f, 0x8b, 0x08}))
	require.Error(t, checkHubItem([]byte("# only a comment\n")))
	require.Error(t, checkHubItem(nil))
}
