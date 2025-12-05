package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestFeatureFlags_UpdateFlags_InvalidPayload(t *testing.T) {
	db := setupFlagsDB(t)

	h := NewFeatureFlagsHandler(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/api/v1/feature-flags", h.UpdateFlags)

	// Send invalid JSON
	req := httptest.NewRequest(http.MethodPut, "/api/v1/feature-flags", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFeatureFlags_UpdateFlags_IgnoresInvalidKeys(t *testing.T) {
	db := setupFlagsDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	h := NewFeatureFlagsHandler(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/api/v1/feature-flags", h.UpdateFlags)

	// Try to update a non-whitelisted key
	payload := []byte(`{"invalid.key": true, "feature.global.enabled": true}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/feature-flags", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify invalid key was NOT saved
	var s models.Setting
	err := db.Where("key = ?", "invalid.key").First(&s).Error
	assert.Error(t, err) // Should not exist

	// Valid key should be saved
	err = db.Where("key = ?", "feature.global.enabled").First(&s).Error
	assert.NoError(t, err)
	assert.Equal(t, "true", s.Value)
}

func TestFeatureFlags_EnvFallback_ShortVariant(t *testing.T) {
	// Test the short env variant (CERBERUS_ENABLED instead of FEATURE_CERBERUS_ENABLED)
	t.Setenv("CERBERUS_ENABLED", "true")

	db := OpenTestDB(t)
	h := NewFeatureFlagsHandler(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Parse response
	var flags map[string]bool
	err := json.Unmarshal(w.Body.Bytes(), &flags)
	require.NoError(t, err)

	// Should be true via short env fallback
	assert.True(t, flags["feature.cerberus.enabled"])
}

func TestFeatureFlags_EnvFallback_WithValue1(t *testing.T) {
	// Test env fallback with "1" as value
	t.Setenv("FEATURE_UPTIME_ENABLED", "1")

	db := OpenTestDB(t)
	h := NewFeatureFlagsHandler(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var flags map[string]bool
	json.Unmarshal(w.Body.Bytes(), &flags)
	assert.True(t, flags["feature.uptime.enabled"])
}

func TestFeatureFlags_EnvFallback_WithValue0(t *testing.T) {
	// Test env fallback with "0" as value (should be false)
	t.Setenv("FEATURE_DOCKER_ENABLED", "0")

	db := OpenTestDB(t)
	h := NewFeatureFlagsHandler(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var flags map[string]bool
	json.Unmarshal(w.Body.Bytes(), &flags)
	assert.False(t, flags["feature.docker.enabled"])
}

func TestFeatureFlags_DBTakesPrecedence(t *testing.T) {
	// Test that DB value takes precedence over env
	t.Setenv("FEATURE_NOTIFICATIONS_ENABLED", "false")

	db := setupFlagsDB(t)
	// Set DB value to true
	db.Create(&models.Setting{Key: "feature.notifications.enabled", Value: "true", Type: "bool", Category: "feature"})

	h := NewFeatureFlagsHandler(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var flags map[string]bool
	json.Unmarshal(w.Body.Bytes(), &flags)
	// DB value (true) should take precedence over env (false)
	assert.True(t, flags["feature.notifications.enabled"])
}

func TestFeatureFlags_DBValueVariations(t *testing.T) {
	db := setupFlagsDB(t)

	// Test various DB value formats
	testCases := []struct {
		key      string
		dbValue  string
		expected bool
	}{
		{"feature.global.enabled", "1", true},
		{"feature.cerberus.enabled", "yes", true},
		{"feature.uptime.enabled", "TRUE", true},
		{"feature.notifications.enabled", "false", false},
		{"feature.docker.enabled", "0", false},
	}

	for _, tc := range testCases {
		db.Create(&models.Setting{Key: tc.key, Value: tc.dbValue, Type: "bool", Category: "feature"})
	}

	h := NewFeatureFlagsHandler(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var flags map[string]bool
	json.Unmarshal(w.Body.Bytes(), &flags)

	for _, tc := range testCases {
		assert.Equal(t, tc.expected, flags[tc.key], "flag %s expected %v", tc.key, tc.expected)
	}
}

func TestFeatureFlags_UpdateMultipleFlags(t *testing.T) {
	db := setupFlagsDB(t)

	h := NewFeatureFlagsHandler(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/api/v1/feature-flags", h.UpdateFlags)
	r.GET("/api/v1/feature-flags", h.GetFlags)

	// Update multiple flags at once
	payload := []byte(`{
		"feature.global.enabled": true,
		"feature.cerberus.enabled": false,
		"feature.uptime.enabled": true
	}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/feature-flags", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify by getting flags
	req = httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var flags map[string]bool
	json.Unmarshal(w.Body.Bytes(), &flags)

	assert.True(t, flags["feature.global.enabled"])
	assert.False(t, flags["feature.cerberus.enabled"])
	assert.True(t, flags["feature.uptime.enabled"])
}

func TestFeatureFlags_ShortEnvFallback_WithUnparseable(t *testing.T) {
	// Test short env fallback with a value that's not directly parseable as bool
	// but is "1" which should be treated as true
	t.Setenv("GLOBAL_ENABLED", "1")

	db := OpenTestDB(t)
	h := NewFeatureFlagsHandler(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var flags map[string]bool
	json.Unmarshal(w.Body.Bytes(), &flags)
	assert.True(t, flags["feature.global.enabled"])
}
