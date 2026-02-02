package handlers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
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

// TestPullThenApplyIntegration tests the complete pull→apply workflow from the user's perspective.
// This reproduces the scenario where a user pulls a preset and then tries to apply it.
func TestPullThenApplyIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup
	cacheDir := t.TempDir()
	dataDir := t.TempDir()

	cache, err := crowdsec.NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)

	archive := makePresetTarGz(t, map[string]string{
		"config.yaml": "test: config\nversion: 1",
	})

	hub := crowdsec.NewHubService(nil, cache, dataDir)
	hub.HubBaseURL = "http://test.hub"
	hub.HTTPClient = &http.Client{
		Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
			switch req.URL.String() {
			case "http://test.hub/api/index.json":
				body := `{"items":[{"name":"test/preset","title":"Test","description":"Test preset","etag":"abc123","download_url":"http://test.hub/test.tgz","preview_url":"http://test.hub/test.yaml"}]}`
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			case "http://test.hub/test.yaml":
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("preview content")), Header: make(http.Header)}, nil
			case "http://test.hub/test.tgz":
				return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(archive)), Header: make(http.Header)}, nil
			default:
				return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
			}
		}),
	}

	db := OpenTestDB(t)
	handler := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", dataDir)
	handler.Hub = hub

	r := gin.New()
	g := r.Group("/api/v1")
	handler.RegisterRoutes(g)

	// Step 1: Pull the preset
	t.Log("User pulls preset")
	pullPayload, _ := json.Marshal(map[string]string{"slug": "test/preset"})
	pullReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/pull", bytes.NewReader(pullPayload))
	pullReq.Header.Set("Content-Type", "application/json")
	pullResp := httptest.NewRecorder()
	r.ServeHTTP(pullResp, pullReq)

	require.Equal(t, http.StatusOK, pullResp.Code, "Pull should succeed")

	var pullResult map[string]any
	err = json.Unmarshal(pullResp.Body.Bytes(), &pullResult)
	require.NoError(t, err)
	require.Equal(t, "pulled", pullResult["status"])
	require.NotEmpty(t, pullResult["cache_key"], "Pull should return cache_key")
	require.NotEmpty(t, pullResult["preview"], "Pull should return preview")

	t.Log("Pull succeeded, cache_key:", pullResult["cache_key"])

	// Verify cache was populated
	ctx := context.Background()
	cached, err := cache.Load(ctx, "test/preset")
	require.NoError(t, err, "Preset should be cached after pull")
	require.Equal(t, "test/preset", cached.Slug)
	t.Log("Cache verified, slug:", cached.Slug)

	// Step 2: Apply the preset (this should use the cached data)
	t.Log("User applies preset")
	applyPayload, _ := json.Marshal(map[string]string{"slug": "test/preset"})
	applyReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/apply", bytes.NewReader(applyPayload))
	applyReq.Header.Set("Content-Type", "application/json")
	applyResp := httptest.NewRecorder()
	r.ServeHTTP(applyResp, applyReq)

	// This should NOT return "preset not cached" error
	require.Equal(t, http.StatusOK, applyResp.Code, "Apply should succeed after pull. Response: %s", applyResp.Body.String())

	var applyResult map[string]any
	err = json.Unmarshal(applyResp.Body.Bytes(), &applyResult)
	require.NoError(t, err)
	require.Equal(t, "applied", applyResult["status"], "Apply status should be 'applied'")
	require.NotEmpty(t, applyResult["backup"], "Apply should return backup path")

	t.Log("Apply succeeded, backup:", applyResult["backup"])
}

// TestApplyWithoutPullReturnsProperError verifies the error message when applying without pulling first.
func TestApplyWithoutPullReturnsProperError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cacheDir := t.TempDir()
	dataDir := t.TempDir()

	cache, err := crowdsec.NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)

	// Empty cache, no cscli
	hub := crowdsec.NewHubService(nil, cache, dataDir)
	hub.HubBaseURL = "http://test.hub"
	hub.HTTPClient = &http.Client{Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	db := OpenTestDB(t)
	handler := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", dataDir)
	handler.Hub = hub

	r := gin.New()
	g := r.Group("/api/v1")
	handler.RegisterRoutes(g)

	// Try to apply without pulling first
	t.Log("User tries to apply preset without pulling first")
	applyPayload, _ := json.Marshal(map[string]string{"slug": "test/preset"})
	applyReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/apply", bytes.NewReader(applyPayload))
	applyReq.Header.Set("Content-Type", "application/json")
	applyResp := httptest.NewRecorder()
	r.ServeHTTP(applyResp, applyReq)

	require.Equal(t, http.StatusInternalServerError, applyResp.Code, "Apply should fail without cache")

	var errorResult map[string]any
	err = json.Unmarshal(applyResp.Body.Bytes(), &errorResult)
	require.NoError(t, err)

	errorMsg := errorResult["error"].(string)
	require.Contains(t, errorMsg, "Preset cache missing", "Error should mention preset not cached")
	require.Contains(t, errorMsg, "Pull the preset", "Error should guide user to pull first")
	t.Log("Proper error message returned:", errorMsg)
}

func TestApplyRollbackWhenCacheMissingAndRepullFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cacheDir := t.TempDir()
	dataRoot := t.TempDir()
	dataDir := filepath.Join(dataRoot, "crowdsec")
	require.NoError(t, os.MkdirAll(dataDir, 0o750)) // #nosec G301 -- test directory
	originalFile := filepath.Join(dataDir, "config.yaml")
	require.NoError(t, os.WriteFile(originalFile, []byte("original"), 0o600)) // #nosec G306 -- test fixture

	cache, err := crowdsec.NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)

	hub := crowdsec.NewHubService(nil, cache, dataDir)
	hub.HubBaseURL = "http://test.hub"
	hub.HTTPClient = &http.Client{Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
		// Force repull failure
		return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	db := OpenTestDB(t)
	handler := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", dataDir)
	handler.Hub = hub

	r := gin.New()
	g := r.Group("/api/v1")
	handler.RegisterRoutes(g)

	applyPayload, _ := json.Marshal(map[string]string{"slug": "missing/preset"})
	applyReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/apply", bytes.NewReader(applyPayload))
	applyReq.Header.Set("Content-Type", "application/json")
	applyResp := httptest.NewRecorder()
	r.ServeHTTP(applyResp, applyReq)

	require.Equal(t, http.StatusInternalServerError, applyResp.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(applyResp.Body.Bytes(), &body))
	require.NotEmpty(t, body["backup"], "backup path should be returned for rollback traceability")
	require.Contains(t, body["error"], "Preset cache missing", "error should guide user to repull")

	// Original file should remain after rollback
	data, readErr := os.ReadFile(originalFile) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, readErr)
	require.Equal(t, "original", string(data))
}

func makePresetTarGz(t *testing.T, files map[string]string) []byte {
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

type testRoundTripper func(*http.Request) (*http.Response, error)

func (t testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return t(req)
}
