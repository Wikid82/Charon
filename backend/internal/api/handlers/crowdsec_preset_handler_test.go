package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

type presetRoundTripper func(*http.Request) (*http.Response, error)

func (p presetRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return p(req)
}

func TestListPresetsIncludesCacheAndIndex(t *testing.T) {
	cache, err := crowdsec.NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	_, err = cache.Store(context.Background(), "crowdsecurity/demo", "collections", "etag1", "hub", []byte("name: crowdsecurity/demo\n"))
	require.NoError(t, err)

	hub := crowdsec.NewHubService(nil, cache, t.TempDir())
	hub.HubBaseURL = "https://hub.crowdsec.net"
	hub.HTTPClient = &http.Client{Transport: presetRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == "https://hub.crowdsec.net/api/index.json" {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"items":[{"name":"crowdsecurity/demo","title":"Demo","description":"desc","type":"collection"}]}`)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	db := OpenTestDB(t)
	handler := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	handler.Hub = hub

	r := gin.New()
	g := r.Group("/api/v1")
	handler.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/presets", http.NoBody)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var payload struct {
		Presets []struct {
			Slug   string `json:"slug"`
			Cached bool   `json:"cached"`
		} `json:"presets"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	found := false
	for _, p := range payload.Presets {
		if p.Slug == "crowdsecurity/demo" {
			found = true
			require.True(t, p.Cached)
		}
	}
	require.True(t, found)
}

func TestPullPresetHandlerHubError(t *testing.T) {
	cache, err := crowdsec.NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	hub := crowdsec.NewHubService(nil, cache, t.TempDir())
	hub.HubBaseURL = "https://hub.crowdsec.net"
	hub.HTTPClient = &http.Client{Transport: presetRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = hub

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	body, _ := json.Marshal(map[string]string{"slug": "crowdsecurity/missing"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/pull", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadGateway, w.Code)
}

func TestPullPresetHandlerTimeout(t *testing.T) {
	cache, err := crowdsec.NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	hub := crowdsec.NewHubService(nil, cache, t.TempDir())
	hub.HubBaseURL = "https://hub.crowdsec.net"
	hub.HTTPClient = &http.Client{Transport: presetRoundTripper(func(req *http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})}

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = hub

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	body, _ := json.Marshal(map[string]string{"slug": "crowdsecurity/demo"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/pull", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusGatewayTimeout, w.Code)
	require.Contains(t, w.Body.String(), "deadline")
}

func TestGetCachedPresetNotFound(t *testing.T) {
	cache, err := crowdsec.NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = crowdsec.NewHubService(nil, cache, t.TempDir())

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/presets/cache/unknown", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetCachedPresetServiceUnavailable(t *testing.T) {

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = &crowdsec.HubService{}

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/presets/cache/demo", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestListPresetsMergesCuratedAndHub(t *testing.T) {

	hub := crowdsec.NewHubService(nil, nil, t.TempDir())
	hub.HubBaseURL = "https://hub.crowdsec.net"
	hub.HTTPClient = &http.Client{Transport: presetRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == "https://hub.crowdsec.net/api/index.json" {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"items":[{"name":"crowdsecurity/custom","title":"Custom","description":"d","type":"collection"}]}`)), Header: make(http.Header)}, nil
		}
		return nil, errors.New("unexpected request")
	})}

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = hub

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/presets", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var payload struct {
		Presets []struct {
			Slug   string   `json:"slug"`
			Source string   `json:"source"`
			Tags   []string `json:"tags"`
		} `json:"presets"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))

	foundCurated := false
	foundHub := false
	for _, p := range payload.Presets {
		if p.Slug == "honeypot-friendly-defaults" {
			foundCurated = true
		}
		if p.Slug == "crowdsecurity/custom" {
			foundHub = true
			require.Equal(t, []string{"collection"}, p.Tags)
		}
	}

	require.True(t, foundCurated)
	require.True(t, foundHub)
}

func TestGetCachedPresetSuccess(t *testing.T) {
	t.Setenv("FEATURE_CERBERUS_ENABLED", "true")
	cache, err := crowdsec.NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	const slug = "crowdsecurity/demo"
	_, err = cache.Store(context.Background(), slug, "collections", "etag123", "hub", []byte("preview: body\n"))
	require.NoError(t, err)

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = crowdsec.NewHubService(nil, cache, t.TempDir())
	require.True(t, h.isCerberusEnabled())
	preview, err := h.Hub.Cache.LoadPreview(context.Background(), slug)
	require.NoError(t, err)
	require.Equal(t, "preview: body\n", preview)

	w := getCachedPreset(context.Background(), h, slug)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "preview: body")
	require.Contains(t, w.Body.String(), "etag123")
}

func TestGetCachedPresetSlugRequired(t *testing.T) {
	t.Setenv("FEATURE_CERBERUS_ENABLED", "true")
	cache, err := crowdsec.NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = crowdsec.NewHubService(nil, cache, t.TempDir())

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/presets/cache/%20", http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "slug required")
}

func TestGetCachedPresetPreviewError(t *testing.T) {
	t.Setenv("FEATURE_CERBERUS_ENABLED", "true")
	cacheDir := t.TempDir()
	cache, err := crowdsec.NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)
	const slug = "crowdsecurity/broken"
	_, err = cache.Store(context.Background(), slug, "collections", "etag999", "hub", []byte("a: 1\n"))
	require.NoError(t, err)
	// Change the item so it no longer matches the recorded digest.
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "crowdsecurity", "broken", "item.yaml"), []byte("a: 2\n"), 0o600))

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = crowdsec.NewHubService(nil, cache, t.TempDir())

	w := getCachedPreset(context.Background(), h, slug)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "cache miss")
	require.NoDirExists(t, filepath.Join(cacheDir, "crowdsecurity", "broken"), "an invalid entry is removed")
}

func TestPullCuratedPresetSkipsHub(t *testing.T) {
	t.Setenv("FEATURE_CERBERUS_ENABLED", "true")

	// Setup handler with a hub service that would fail if called
	cache, err := crowdsec.NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	// We don't set HTTPClient, so any network call would panic or fail if not handled
	hub := crowdsec.NewHubService(nil, cache, t.TempDir())

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = hub

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	// Use a known curated preset that doesn't require hub
	slug := "honeypot-friendly-defaults"

	body, _ := json.Marshal(map[string]string{"slug": slug})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/pull", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	require.Equal(t, "pulled", resp["status"])
	require.Equal(t, slug, resp["slug"])
	require.Equal(t, "charon-curated", resp["source"])
	require.Contains(t, resp["preview"], "Curated preset")
	require.Contains(t, resp["preview"], "collections: crowdsecurity/sshd")
	require.Contains(t, resp["preview"], "parsers: crowdsecurity/whitelists")
}

// getCachedPreset calls GetCachedPreset with the slug as the route parameter. The route's single path
// segment cannot carry a slug that contains a slash, so tests bypass the router.
func getCachedPreset(ctx context.Context, h *CrowdsecHandler, slug string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", http.NoBody).WithContext(ctx)
	c.Params = gin.Params{{Key: "slug", Value: slug}}
	h.GetCachedPreset(c)
	return w
}
