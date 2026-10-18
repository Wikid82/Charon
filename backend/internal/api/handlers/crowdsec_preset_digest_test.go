package handlers

import (
	"bytes"
	"encoding/json"
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

const (
	presetPullURL  = "/api/v1/admin/crowdsec/presets/pull"
	presetApplyURL = "/api/v1/admin/crowdsec/presets/apply"
)

func postPresetAny(r *gin.Engine, target string, payload map[string]any) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// pulledPreset pulls test/preset into a cache whose directory the test knows, and returns the pull answer.
func pulledPreset(t *testing.T) (r *gin.Engine, cacheDir string, pulled map[string]any) {
	t.Helper()
	dataDir := t.TempDir()
	hub, _ := integrationHub(t, dataDir)
	cacheDir = t.TempDir()
	cache, err := crowdsec.NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)
	hub.Cache = cache
	r, h := integrationRouter(t, hub, dataDir)
	require.NoError(t, h.DB.AutoMigrate(&models.CrowdsecApplyJob{}))
	t.Cleanup(h.ShutdownApplyJobs)

	w := postPresetAny(r, presetPullURL, map[string]any{"slug": "test/preset"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pulled))
	return r, cacheDir, pulled
}

func TestPullPresetReturnsTheItemDigest(t *testing.T) {
	_, _, pulled := pulledPreset(t)
	require.Equal(t, hubcachetest.SHA256Hex([]byte(integrationItem)), pulled["item_sha256"])
}

func TestApplyPresetItemDigest(t *testing.T) {
	for _, async := range []bool{false, true} {
		name := "sync"
		if async {
			name = "async"
		}
		t.Run(name+" matching digest applies", func(t *testing.T) {
			r, _, pulled := pulledPreset(t)
			w := postPresetAny(r, presetApplyURL, map[string]any{"slug": "test/preset", "cache_key": pulled["cache_key"], "item_sha256": pulled["item_sha256"], "async": async})
			require.Equal(t, map[bool]int{false: http.StatusOK, true: http.StatusAccepted}[async], w.Code, w.Body.String())
		})
		t.Run(name+" absent digest applies", func(t *testing.T) {
			r, _, pulled := pulledPreset(t)
			w := postPresetAny(r, presetApplyURL, map[string]any{"slug": "test/preset", "cache_key": pulled["cache_key"], "async": async})
			require.Equal(t, map[bool]int{false: http.StatusOK, true: http.StatusAccepted}[async], w.Code, w.Body.String())
		})
	}

	t.Run("sync differing digest is stale", func(t *testing.T) {
		r, _, pulled := pulledPreset(t)
		w := postPresetAny(r, presetApplyURL, map[string]any{"slug": "test/preset", "cache_key": pulled["cache_key"], "item_sha256": strings.Repeat("0", 64)})
		require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
		require.Equal(t, codePresetCacheStale, decodeBody(t, w)["error_code"])
	})
	t.Run("sync key mismatch is stale", func(t *testing.T) {
		r, _, _ := pulledPreset(t)
		w := postPresetAny(r, presetApplyURL, map[string]any{"slug": "test/preset", "cache_key": "test/preset-1-" + strings.Repeat("0", 64)})
		require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
		require.Equal(t, codePresetCacheStale, decodeBody(t, w)["error_code"])
	})
	t.Run("sync tampered cache file is stale", func(t *testing.T) {
		r, cacheDir, pulled := pulledPreset(t)
		require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "collections", "test", "preset", "item.yaml"), []byte("name: evil\n"), 0o600))
		w := postPresetAny(r, presetApplyURL, map[string]any{"slug": "test/preset", "cache_key": pulled["cache_key"], "item_sha256": pulled["item_sha256"]})
		require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
		require.Equal(t, codePresetCacheStale, decodeBody(t, w)["error_code"])
	})
}

func TestAsyncApplyPresetDifferingDigestFailsAsStale(t *testing.T) {
	r, _, pulled := pulledPreset(t)
	w := postPresetAny(r, presetApplyURL, map[string]any{"slug": "test/preset", "cache_key": pulled["cache_key"], "item_sha256": strings.Repeat("0", 64), "async": true})
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	id := decodeBody(t, w)["job_id"].(string)

	require.Eventually(t, func() bool {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1"+applyJobsBase+"/"+id, http.NoBody))
		body := decodeBody(t, rec)
		return body["state"] == "failed" && body["error_code"] == codePresetCacheStale
	}, 10*time.Second, 20*time.Millisecond)
}
