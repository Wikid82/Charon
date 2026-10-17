package handlers

import (
	"bytes"
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
	"github.com/Wikid82/charon/backend/internal/crowdsec/hubcachetest"
	"github.com/Wikid82/charon/backend/internal/models"
)

const integrationItem = "name: test/preset\ndescription: integration item\n"

// integrationHub serves one hub item over a stubbed hub and answers cscli as a healthy install of that item.
func integrationHub(t *testing.T, dataDir string) (*crowdsec.HubService, *crowdsec.HubCache) {
	t.Helper()
	cache, err := crowdsec.NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	exec := &scriptedHubExec{fn: func(_ context.Context, cmd string) ([]byte, error) {
		switch cmd {
		case "cscli collections inspect test/preset -o json":
			return inspectWithHash(t, "inspect_installed.json", hubcachetest.SHA256Hex([]byte(integrationItem))), nil
		case "cscli hub list -o json":
			return []byte(`{"collections":[{"name":"test/preset","status":"enabled"}]}`), nil
		}
		return nil, nil
	}}
	hub := crowdsec.NewHubService(exec, cache, dataDir)
	hub.HubBaseURL = "https://hub.crowdsec.net"
	hub.HTTPClient = &http.Client{Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://hub.crowdsec.net/api/index.json":
			body := `{"items":[{"name":"test/preset","title":"Test","description":"Test preset","type":"collections","etag":"abc123","download_url":"https://hub.crowdsec.net/test.yaml"}]}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		case "https://hub.crowdsec.net/test.yaml":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(integrationItem)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
	})}
	return hub, cache
}

func integrationRouter(t *testing.T, hub *crowdsec.HubService, dataDir string) (*gin.Engine, *CrowdsecHandler) {
	t.Helper()
	t.Setenv("FEATURE_CERBERUS_ENABLED", "true")
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.CrowdsecPresetEvent{}))
	handler := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", dataDir)
	handler.Hub = hub
	r := gin.New()
	handler.RegisterRoutes(r.Group("/api/v1"))
	return r, handler
}

func postPresetJSON(r *gin.Engine, target string, payload map[string]string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestPullThenApplyIntegration pulls a preset and applies it with the key from the pull.
func TestPullThenApplyIntegration(t *testing.T) {
	dataDir := t.TempDir()
	hub, cache := integrationHub(t, dataDir)
	r, handler := integrationRouter(t, hub, dataDir)

	pullResp := postPresetJSON(r, "/api/v1/admin/crowdsec/presets/pull", map[string]string{"slug": "test/preset"})
	require.Equal(t, http.StatusOK, pullResp.Code, pullResp.Body.String())
	var pulled map[string]any
	require.NoError(t, json.Unmarshal(pullResp.Body.Bytes(), &pulled))
	require.Equal(t, "pulled", pulled["status"])
	require.Equal(t, "test/preset", pulled["slug"])
	require.Equal(t, integrationItem, pulled["preview"])
	key, _ := pulled["cache_key"].(string)
	require.NotEmpty(t, key)

	cached, err := cache.Load(context.Background(), "collections", "test/preset")
	require.NoError(t, err)
	require.Equal(t, key, cached.CacheKey)

	applyResp := postPresetJSON(r, "/api/v1/admin/crowdsec/presets/apply", map[string]string{"slug": "test/preset", "cache_key": key})
	require.Equal(t, http.StatusOK, applyResp.Code, applyResp.Body.String())
	var applied map[string]any
	require.NoError(t, json.Unmarshal(applyResp.Body.Bytes(), &applied))
	require.Equal(t, "applied", applied["status"])
	require.Equal(t, true, applied["used_cscli"])
	require.Equal(t, key, applied["cache_key"])
	require.NotEmpty(t, applied["backup"])

	var events []models.CrowdsecPresetEvent
	require.NoError(t, handler.DB.Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, "applied", events[0].Status)
	require.Equal(t, key, events[0].CacheKey)
}

func TestPullPresetRejectsInvalidSlug(t *testing.T) {
	dataDir := t.TempDir()
	hub, _ := integrationHub(t, dataDir)
	r, _ := integrationRouter(t, hub, dataDir)

	w := postPresetJSON(r, "/api/v1/admin/crowdsec/presets/pull", map[string]string{"slug": "solo"})
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "invalid slug")
}

func TestPullPresetRefusesNonYAMLDownload(t *testing.T) {
	dataDir := t.TempDir()
	hub, cache := integrationHub(t, dataDir)
	hub.HTTPClient = &http.Client{Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
		body := `{"items":[{"name":"test/preset","type":"collections","download_url":"https://hub.crowdsec.net/test.yaml"}]}`
		if strings.HasSuffix(req.URL.Path, ".yaml") {
			body = "<html><body>maintenance</body></html>"
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	r, _ := integrationRouter(t, hub, dataDir)

	w := postPresetJSON(r, "/api/v1/admin/crowdsec/presets/pull", map[string]string{"slug": "test/preset"})
	require.Equal(t, http.StatusBadGateway, w.Code, w.Body.String())
	_, err := cache.Load(context.Background(), "collections", "test/preset")
	require.ErrorIs(t, err, crowdsec.ErrCacheMiss)
}

// TestApplyWithoutPullReturnsProperError applies a preset that was never pulled.
func TestApplyWithoutPullReturnsProperError(t *testing.T) {
	dataDir := t.TempDir()
	original := filepath.Join(dataDir, "config.yaml")
	require.NoError(t, os.WriteFile(original, []byte("original"), 0o600))
	hub, _ := integrationHub(t, dataDir)
	r, handler := integrationRouter(t, hub, dataDir)

	w := postPresetJSON(r, "/api/v1/admin/crowdsec/presets/apply", map[string]string{"slug": "test/preset", "cache_key": "test/preset-1-" + strings.Repeat("0", 64)})
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "preset_cache_stale", body["error_code"])
	require.Contains(t, body["error"], "Pull the preset again")

	data, err := os.ReadFile(original) // #nosec G304 -- test path
	require.NoError(t, err)
	require.Equal(t, "original", string(data))
	var events []models.CrowdsecPresetEvent
	require.NoError(t, handler.DB.Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, "failed", events[0].Status)
}

func TestApplyPresetRejectsInvalidSlug(t *testing.T) {
	dataDir := t.TempDir()
	hub, _ := integrationHub(t, dataDir)
	r, handler := integrationRouter(t, hub, dataDir)

	w := postPresetJSON(r, "/api/v1/admin/crowdsec/presets/apply", map[string]string{"slug": "bad//slug", "cache_key": "k"})
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	var events []models.CrowdsecPresetEvent
	require.NoError(t, handler.DB.Find(&events).Error)
	require.Empty(t, events, "a malformed slug is never recorded")
}

// A failed backup is reported and leaves the data dir alone.
func TestApplyPresetBackupFailure(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "missing-parent", "crowdsec")
	hub, cache := integrationHub(t, dataDir)
	stored, err := cache.Store(context.Background(), "test/preset", "collections", "e", "hub", []byte(integrationItem))
	require.NoError(t, err)
	r, handler := integrationRouter(t, hub, t.TempDir())

	w := postPresetJSON(r, "/api/v1/admin/crowdsec/presets/apply", map[string]string{"slug": "test/preset", "cache_key": stored.CacheKey})
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "backup")

	var events []models.CrowdsecPresetEvent
	require.NoError(t, handler.DB.Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, "failed", events[0].Status)
	require.Empty(t, events[0].BackupPath)
}

type testRoundTripper func(*http.Request) (*http.Response, error)

func (t testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return t(req)
}
