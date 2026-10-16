package handlers

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

const cacheRoutePrefix = "/api/v1/admin/crowdsec/presets/cache/"

func cacheRouter(t *testing.T) (r *gin.Engine, cacheDir, cacheKey string) {
	t.Helper()
	t.Setenv("FEATURE_CERBERUS_ENABLED", "true")
	cacheDir = t.TempDir()
	cache, err := crowdsec.NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)
	meta, err := cache.Store(context.Background(), "crowdsecurity/demo", "collections", "etag-demo", "hub", []byte("name: crowdsecurity/demo\n"))
	require.NoError(t, err)

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = crowdsec.NewHubService(nil, cache, t.TempDir())
	r = gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	return r, cacheDir, meta.CacheKey
}

func cacheTreeListing(t *testing.T, dir string) []string {
	t.Helper()
	var names []string
	require.NoError(t, filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		names = append(names, path)
		return err
	}))
	return names
}

func TestCachedPresetRouteServesOwnerNameSlugs(t *testing.T) {
	t.Skip("enabled by a later commit in this series")
	r, _, key := cacheRouter(t)

	for _, path := range []string{"crowdsecurity/demo", "crowdsecurity%2Fdemo"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, cacheRoutePrefix+path, http.NoBody))

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			var body struct {
				Preview  string `json:"preview"`
				CacheKey string `json:"cache_key"`
				Etag     string `json:"etag"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Equal(t, key, body.CacheKey)
			require.Equal(t, "etag-demo", body.Etag)
			require.Contains(t, body.Preview, "crowdsecurity/demo")
		})
	}
}

func TestCachedPresetRouteAnswersOddSlugsWithoutTouchingTheCache(t *testing.T) {
	t.Skip("enabled by a later commit in this series")
	r, cacheDir, _ := cacheRouter(t)
	before := cacheTreeListing(t, cacheDir)

	cases := []struct {
		name string
		path string
		want int
	}{
		{"single segment", "curated-slug", http.StatusNotFound},
		{"empty slug", "", http.StatusBadRequest},
		{"blank slug", "%20", http.StatusBadRequest},
		{"parent reference", "../x", http.StatusNotFound},
		{"three components", "a/b/c", http.StatusNotFound},
		{"dot component", "crowdsecurity/.", http.StatusNotFound},
		{"unknown owner and name", "other/missing", http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, cacheRoutePrefix+tc.path, http.NoBody))

			require.Equal(t, tc.want, w.Code, w.Body.String())
		})
	}
	require.Equal(t, before, cacheTreeListing(t, cacheDir))
}
