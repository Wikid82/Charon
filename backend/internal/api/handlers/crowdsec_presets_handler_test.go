package handlers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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
	"github.com/Wikid82/charon/backend/internal/models"
)

type presetRoundTripper func(*http.Request) (*http.Response, error)

func (p presetRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return p(req)
}

func makePresetTar(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func TestListPresetsIncludesCacheAndIndex(t *testing.T) {
	cache, err := crowdsec.NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	_, err = cache.Store(context.Background(), "crowdsecurity/demo", "etag1", "hub", "preview", []byte("archive"))
	require.NoError(t, err)

	hub := crowdsec.NewHubService(nil, cache, t.TempDir())
	hub.HubBaseURL = "http://example.com"
	hub.HTTPClient = &http.Client{Transport: presetRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == "http://example.com/api/index.json" {
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

func TestPullPresetHandlerSuccess(t *testing.T) {
	cache, err := crowdsec.NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	archive := makePresetTar(t, map[string]string{"config.yaml": "key: value"})

	hub := crowdsec.NewHubService(nil, cache, dataDir)
	hub.HubBaseURL = "http://example.com"
	hub.HTTPClient = &http.Client{Transport: presetRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "http://example.com/api/index.json":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"items":[{"name":"crowdsecurity/demo","title":"Demo","description":"desc","etag":"e1","download_url":"http://example.com/demo.tgz","preview_url":"http://example.com/demo.yaml"}]}`)), Header: make(http.Header)}, nil
		case "http://example.com/demo.yaml":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("preview")), Header: make(http.Header)}, nil
		case "http://example.com/demo.tgz":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(archive)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
	})}

	handler := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", dataDir)
	handler.Hub = hub

	r := gin.New()
	g := r.Group("/api/v1")
	handler.RegisterRoutes(g)

	body, _ := json.Marshal(map[string]string{"slug": "crowdsecurity/demo"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/pull", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "cache_key")
	require.Contains(t, w.Body.String(), "preview")
}

func TestApplyPresetHandlerAudits(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.CrowdsecPresetEvent{}))

	cache, err := crowdsec.NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	archive := makePresetTar(t, map[string]string{"conf.yaml": "v: 1"})
	_, err = cache.Store(context.Background(), "crowdsecurity/demo", "etag1", "hub", "preview", archive)
	require.NoError(t, err)

	hub := crowdsec.NewHubService(nil, cache, dataDir)

	handler := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", dataDir)
	handler.Hub = hub

	r := gin.New()
	g := r.Group("/api/v1")
	handler.RegisterRoutes(g)

	body, _ := json.Marshal(map[string]string{"slug": "crowdsecurity/demo"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/apply", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var events []models.CrowdsecPresetEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, "applied", events[0].Status)

	// Failure path
	badCache, err := crowdsec.NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	badArchive := makePresetTar(t, map[string]string{"../bad.txt": "x"})
	_, err = badCache.Store(context.Background(), "crowdsecurity/demo", "etag1", "hub", "preview", badArchive)
	require.NoError(t, err)

	badHub := crowdsec.NewHubService(nil, badCache, filepath.Join(t.TempDir(), "crowdsec2"))
	handler.Hub = badHub

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/apply", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusInternalServerError, w2.Code)

	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 2)
	require.Equal(t, "failed", events[1].Status)
}

func TestPullPresetHandlerHubError(t *testing.T) {
	cache, err := crowdsec.NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	hub := crowdsec.NewHubService(nil, cache, t.TempDir())
	hub.HubBaseURL = "http://example.com"
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
	hub.HubBaseURL = "http://example.com"
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

func TestApplyPresetHandlerBackupFailure(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.CrowdsecPresetEvent{}))

	baseDir := t.TempDir()
	dataDir := filepath.Join(baseDir, "crowdsec")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))                                               // #nosec G301 -- test directory
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "keep.txt"), []byte("before"), 0o600)) // #nosec G306 -- test fixture

	hub := crowdsec.NewHubService(nil, nil, dataDir)
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", dataDir)
	h.Hub = hub

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	body, _ := json.Marshal(map[string]string{"slug": "crowdsecurity/demo"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/apply", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	// Verify response includes backup path for traceability
	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	_, hasBackup := response["backup"]
	require.True(t, hasBackup, "Response should include 'backup' field for diagnostics")

	// Verify error message is present
	errorMsg, ok := response["error"].(string)
	require.True(t, ok, "error field should be a string")
	require.Contains(t, errorMsg, "cache", "error should indicate cache is unavailable")

	var events []models.CrowdsecPresetEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, "failed", events[0].Status)
	require.NotEmpty(t, events[0].BackupPath)

	content, readErr := os.ReadFile(filepath.Join(dataDir, "keep.txt")) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, readErr)
	require.Equal(t, "before", string(content))
}

func TestListPresetsMergesCuratedAndHub(t *testing.T) {

	hub := crowdsec.NewHubService(nil, nil, t.TempDir())
	hub.HubBaseURL = "http://hub.example"
	hub.HTTPClient = &http.Client{Transport: presetRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == "http://hub.example/api/index.json" {
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
	const slug = "demo"
	_, err = cache.Store(context.Background(), slug, "etag123", "hub", "preview-body", []byte("tgz"))
	require.NoError(t, err)

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = crowdsec.NewHubService(nil, cache, t.TempDir())
	require.True(t, h.isCerberusEnabled())
	preview, err := h.Hub.Cache.LoadPreview(context.Background(), slug)
	require.NoError(t, err)
	require.Equal(t, "preview-body", preview)

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/presets/cache/"+slug, http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "preview-body")
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
	const slug = "broken"
	meta, err := cache.Store(context.Background(), slug, "etag999", "hub", "will-remove", []byte("tgz"))
	require.NoError(t, err)
	// Remove preview to force LoadPreview read error.
	require.NoError(t, os.Remove(meta.PreviewPath))

	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = crowdsec.NewHubService(nil, cache, t.TempDir())

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/presets/cache/"+slug, http.NoBody)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "no such file")
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
}

func TestApplyCuratedPresetSkipsHub(t *testing.T) {
	t.Setenv("FEATURE_CERBERUS_ENABLED", "true")

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.CrowdsecPresetEvent{}))

	// Setup handler with a hub service that would fail if called
	// We intentionally don't put anything in cache to prove we don't check it
	cache, err := crowdsec.NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	hub := crowdsec.NewHubService(nil, cache, t.TempDir())

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = hub

	r := gin.New()
	g := r.Group("/api/v1")
	h.RegisterRoutes(g)

	// Use a known curated preset that doesn't require hub
	slug := "honeypot-friendly-defaults"

	body, _ := json.Marshal(map[string]string{"slug": slug})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/apply", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	require.Equal(t, "applied", resp["status"])
	require.Equal(t, slug, resp["slug"])

	// Verify event was logged
	var events []models.CrowdsecPresetEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, slug, events[0].Slug)
	require.Equal(t, "applied", events[0].Status)
}
