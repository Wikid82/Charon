package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

// TestListPresetsShowsCachedStatus verifies the /presets endpoint marks cached presets.
func TestListPresetsShowsCachedStatus(t *testing.T) {

	cacheDir := t.TempDir()
	dataDir := t.TempDir()

	cache, err := crowdsec.NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)

	// Cache a preset
	ctx := context.Background()
	archive := []byte("archive")
	_, err = cache.Store(ctx, "test/cached", "etag", "hub", "preview", archive)
	require.NoError(t, err)

	// Setup handler
	hub := crowdsec.NewHubService(nil, cache, dataDir)
	db := OpenTestDB(t)
	handler := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", dataDir)
	handler.Hub = hub

	r := gin.New()
	g := r.Group("/api/v1")
	handler.RegisterRoutes(g)

	// List presets
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/presets", http.NoBody)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	var result map[string]any
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)

	presets := result["presets"].([]any)
	require.NotEmpty(t, presets, "Should have at least one preset")

	// Find our cached preset
	found := false
	for _, p := range presets {
		preset := p.(map[string]any)
		if preset["slug"] == "test/cached" {
			found = true
			require.True(t, preset["cached"].(bool), "Preset should be marked as cached")
			require.NotEmpty(t, preset["cache_key"], "Should have cache_key")
		}
	}
	require.True(t, found, "Cached preset should appear in list")
}

// TestCacheKeyPersistence verifies cache keys are consistent and retrievable.
func TestCacheKeyPersistence(t *testing.T) {
	cacheDir := t.TempDir()

	cache, err := crowdsec.NewHubCache(cacheDir, time.Hour)
	require.NoError(t, err)

	// Store a preset
	ctx := context.Background()
	archive := []byte("test archive")
	meta, err := cache.Store(ctx, "test/preset", "etag123", "hub", "preview text", archive)
	require.NoError(t, err)

	originalCacheKey := meta.CacheKey
	require.NotEmpty(t, originalCacheKey, "Cache key should be generated")

	// Load it back
	loaded, err := cache.Load(ctx, "test/preset")
	require.NoError(t, err)
	require.Equal(t, originalCacheKey, loaded.CacheKey, "Cache key should persist")
	require.Equal(t, "test/preset", loaded.Slug)
	require.Equal(t, "etag123", loaded.Etag)
}
