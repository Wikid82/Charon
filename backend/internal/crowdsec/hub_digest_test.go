package crowdsec

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/hubcachetest"
)

const (
	digestDemoItem = "name: crowdsecurity/demo\ndescription: demo\n"
	digestDemoURL  = "https://hub.crowdsec.net/collections/crowdsecurity/demo.yaml"
	digestDemoSlug = "crowdsecurity/demo"
)

// digestHub serves a raw hub index (the map-of-maps shape that carries per-version digests) and one item body.
// Tests change index between pulls to model a hub whose published records move.
type digestHub struct {
	index     string
	item      string
	downloads int
}

func newDigestHub(t *testing.T) (*HubService, *digestHub) {
	t.Helper()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	hub := &digestHub{item: digestDemoItem}
	svc := NewHubService(nil, cache, t.TempDir())
	svc.HubBaseURL = "https://hub.crowdsec.net"
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://hub.crowdsec.net" + defaultHubIndexPath:
			resp := newResponse(http.StatusOK, hub.index)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		case digestDemoURL:
			hub.downloads++
			return newResponse(http.StatusOK, hub.item), nil
		default:
			return newResponse(http.StatusNotFound, ""), nil
		}
	})}
	return svc, hub
}

// rawDigestIndex builds a raw hub index with one collection. versions is the JSON value of its "versions"
// key, or "" to leave the key out.
func rawDigestIndex(versions string) string {
	entry := `"path":"collections/crowdsecurity/demo.yaml","version":"1.0"`
	if versions != "" {
		entry += `,"versions":` + versions
	}
	return `{"collections":{"crowdsecurity/demo":{` + entry + `}}}`
}

func versionsWithDigest(digest string) string {
	return `{"1.0":{"digest":"` + digest + `"}}`
}

func TestPullVerifiesIndexDigest(t *testing.T) {
	good := hubcachetest.SHA256Hex([]byte(digestDemoItem))
	cases := []struct {
		name     string
		versions string // "" leaves the versions key out of the index
		stored   bool
	}{
		{"matching digest is stored", versionsWithDigest(good), true},
		{"upper-case digest with whitespace is stored", versionsWithDigest(" " + strings.ToUpper(good) + " "), true},
		{"absent digest is stored", "", true},
		{"version missing from the record map is stored", `{"0.9":{"digest":"` + strings.Repeat("0", 64) + `"}}`, true},
		{"malformed digest is refused", versionsWithDigest("not-a-digest"), false},
		{"mismatched digest is refused", versionsWithDigest(strings.Repeat("0", 64)), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc, hub := newDigestHub(t)
			hub.index = rawDigestIndex(tc.versions)

			res, err := svc.Pull(ctx, digestDemoSlug, "")
			_, _, loadErr := svc.Cache.LoadItem(ctx, "collections", digestDemoSlug)
			if tc.stored {
				require.NoError(t, err)
				require.Equal(t, digestDemoItem, res.Preview)
				require.NoError(t, loadErr)
				return
			}
			require.ErrorIs(t, err, ErrPresetHubUpdating)
			require.Error(t, loadErr, "a refused item must not reach the cache")
			require.Equal(t, 1, hub.downloads)
		})
	}
}

func TestPullCacheHitIsNotRecheckedAgainstIndex(t *testing.T) {
	ctx := context.Background()
	svc, hub := newDigestHub(t)
	hub.index = rawDigestIndex(versionsWithDigest(hubcachetest.SHA256Hex([]byte(digestDemoItem))))
	first, err := svc.Pull(ctx, digestDemoSlug, "")
	require.NoError(t, err)

	// The hub now publishes a digest the cached bytes do not match; a cache hit must still be served.
	hub.index = rawDigestIndex(versionsWithDigest(strings.Repeat("0", 64)))
	second, err := svc.Pull(ctx, digestDemoSlug, "")
	require.NoError(t, err)
	require.Equal(t, first.Meta.CacheKey, second.Meta.CacheKey)
	require.Equal(t, 1, hub.downloads)
}

func TestVerifyIndexDigest(t *testing.T) {
	item := []byte(digestDemoItem)
	cases := []struct {
		name    string
		digest  string
		refused bool
	}{
		{"absent digest is accepted", "", false},
		{"matching digest is accepted", hubcachetest.SHA256Hex([]byte(digestDemoItem)), false},
		{"malformed digest is refused", "not-a-digest", true},
		{"mismatched digest is refused", strings.Repeat("0", 64), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyIndexDigest(HubIndexEntry{Name: digestDemoSlug, Digest: tc.digest}, item)
			if !tc.refused {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrHubItemUnverified)
		})
	}
}

func TestHubItemUnverifiedCountsAsHubUpdating(t *testing.T) {
	require.ErrorIs(t, ErrHubItemUnverified, ErrPresetHubUpdating)
	require.NotErrorIs(t, ErrPresetHubUpdating, ErrHubItemUnverified)
}

func TestHubItemUnverifiedHasMessage(t *testing.T) {
	require.NotEmpty(t, ErrHubItemUnverified.Error())
}

// Recorded-format fixtures: the index carries the hub's map-of-maps shape with a bare lower-case sha256 under
// versions[version].digest, and the digest is the sha256 of the item body file.
const (
	recordedIndexFixture = "testdata/hub_digest_index.json"
	recordedItemFixture  = "testdata/hub_digest_item.yaml"
)

func TestPullVerifiesRecordedIndexFormat(t *testing.T) {
	index, err := os.ReadFile(recordedIndexFixture) //nolint:gosec // G304: fixed testdata path
	require.NoError(t, err)
	item, err := os.ReadFile(recordedItemFixture) //nolint:gosec // G304: fixed testdata path
	require.NoError(t, err)
	require.Contains(t, string(index), hubcachetest.SHA256Hex(item), "fixture digest must be the sha256 of the item body")

	t.Run("recorded body is accepted and cached", func(t *testing.T) {
		ctx := context.Background()
		svc, hub := newDigestHub(t)
		hub.index = string(index)
		hub.item = string(item)

		res, err := svc.Pull(ctx, digestDemoSlug, "")
		require.NoError(t, err)
		require.Equal(t, string(item), res.Preview)
		_, _, loadErr := svc.Cache.LoadItem(ctx, "collections", digestDemoSlug)
		require.NoError(t, loadErr)
	})

	t.Run("altered body is refused and not cached", func(t *testing.T) {
		ctx := context.Background()
		svc, hub := newDigestHub(t)
		hub.index = string(index)
		hub.item = strings.Replace(string(item), "demo", "dem0", 1)

		_, err := svc.Pull(ctx, digestDemoSlug, "")
		require.ErrorIs(t, err, ErrPresetHubUpdating)
		_, _, loadErr := svc.Cache.LoadItem(ctx, "collections", digestDemoSlug)
		require.Error(t, loadErr, "a refused item must not reach the cache")
	})
}
