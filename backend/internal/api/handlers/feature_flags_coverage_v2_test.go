package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestResolveRetiredLegacyFallback_InvalidPersistedValue covers lines 139-140
func TestResolveRetiredLegacyFallback_InvalidPersistedValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	// Create setting with invalid value for retired fallback flag
	db.Create(&models.Setting{
		Key:      "feature.notifications.legacy.fallback_enabled",
		Value:    "invalid_value_not_bool",
		Type:     "bool",
		Category: "feature",
	})

	h := NewFeatureFlagsHandler(db)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Should log warning and return false (lines 139-140)
	var flags map[string]bool
	err = json.Unmarshal(w.Body.Bytes(), &flags)
	require.NoError(t, err)

	assert.False(t, flags["feature.notifications.legacy.fallback_enabled"])
}

// TestResolveRetiredLegacyFallback_InvalidEnvValue covers lines 149-150
func TestResolveRetiredLegacyFallback_InvalidEnvValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	// Set invalid env var for retired fallback flag
	t.Setenv("CHARON_LEGACY_FALLBACK_ENABLED", "not_a_boolean")

	h := NewFeatureFlagsHandler(db)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Should log warning and return false (lines 149-150)
	var flags map[string]bool
	err = json.Unmarshal(w.Body.Bytes(), &flags)
	require.NoError(t, err)

	assert.False(t, flags["feature.notifications.legacy.fallback_enabled"])
}

// TestResolveRetiredLegacyFallback_DefaultFalse covers lines 157-158
func TestResolveRetiredLegacyFallback_DefaultFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	// No DB value, no env vars - should default to false
	h := NewFeatureFlagsHandler(db)
	r := gin.New()
	r.GET("/api/v1/feature-flags", h.GetFlags)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feature-flags", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Should return false (lines 157-158)
	var flags map[string]bool
	err = json.Unmarshal(w.Body.Bytes(), &flags)
	require.NoError(t, err)

	assert.False(t, flags["feature.notifications.legacy.fallback_enabled"])
}
