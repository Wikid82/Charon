package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestBlocker3_SecurityProviderEventsFlagInResponse tests that the feature flag is included in GET response.
func TestBlocker3_SecurityProviderEventsFlagInResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.Setting{})
	assert.NoError(t, err)

	// Create handler
	handler := NewFeatureFlagsHandler(db)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Call GetFlags
	handler.GetFlags(c)

	// Assert response status
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse response
	var response map[string]bool
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Blocker 3: Verify security_provider_events flag is present
	_, exists := response["feature.notifications.security_provider_events.enabled"]
	assert.True(t, exists, "security_provider_events flag should be in response")
}

// TestBlocker3_SecurityProviderEventsFlagDefaultValue tests the default value of the flag.
func TestBlocker3_SecurityProviderEventsFlagDefaultValue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.Setting{})
	assert.NoError(t, err)

	// Create handler
	handler := NewFeatureFlagsHandler(db)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Call GetFlags
	handler.GetFlags(c)

	// Assert response status
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse response
	var response map[string]bool
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Blocker 3: Verify default value is false for this stage
	assert.False(t, response["feature.notifications.security_provider_events.enabled"],
		"security_provider_events flag should default to false for this stage")
}

// TestBlocker3_SecurityProviderEventsFlagCanBeEnabled tests that the flag can be enabled.
func TestBlocker3_SecurityProviderEventsFlagCanBeEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.Setting{})
	assert.NoError(t, err)

	// Create setting with flag enabled
	setting := models.Setting{
		Key:      "feature.notifications.security_provider_events.enabled",
		Value:    "true",
		Type:     "bool",
		Category: "feature",
	}
	assert.NoError(t, db.Create(&setting).Error)

	// Create handler
	handler := NewFeatureFlagsHandler(db)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Call GetFlags
	handler.GetFlags(c)

	// Assert response status
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse response
	var response map[string]bool
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Blocker 3: Verify flag can be enabled
	assert.True(t, response["feature.notifications.security_provider_events.enabled"],
		"security_provider_events flag should be true when enabled in DB")
}

// TestLegacyFallbackRemoved_UpdateFlagsRejectsTrue tests that attempting to set legacy fallback to true returns error code LEGACY_FALLBACK_REMOVED.
func TestLegacyFallbackRemoved_UpdateFlagsRejectsTrue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&models.Setting{}))

	handler := NewFeatureFlagsHandler(db)

	// Attempt to set legacy fallback to true
	payload := map[string]bool{
		"feature.notifications.legacy.fallback_enabled": true,
	}
	jsonPayload, err := json.Marshal(payload)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("PUT", "/api/v1/feature-flags", bytes.NewBuffer(jsonPayload))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateFlags(c)

	// Must return 400 with code LEGACY_FALLBACK_REMOVED
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["error"], "retired")
	assert.Equal(t, "LEGACY_FALLBACK_REMOVED", response["code"])
}

// TestLegacyFallbackRemoved_UpdateFlagsAcceptsFalse tests that setting legacy fallback to false is allowed (forced false).
func TestLegacyFallbackRemoved_UpdateFlagsAcceptsFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&models.Setting{}))

	handler := NewFeatureFlagsHandler(db)

	// Set legacy fallback to false (should be accepted and forced)
	payload := map[string]bool{
		"feature.notifications.legacy.fallback_enabled": false,
	}
	jsonPayload, err := json.Marshal(payload)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("PUT", "/api/v1/feature-flags", bytes.NewBuffer(jsonPayload))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateFlags(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify in DB that it's false
	var setting models.Setting
	db.Where("key = ?", "feature.notifications.legacy.fallback_enabled").First(&setting)
	assert.Equal(t, "false", setting.Value)
}

// TestLegacyFallbackRemoved_GetFlagsReturnsHardFalse tests that GET always returns false for legacy fallback.
func TestLegacyFallbackRemoved_GetFlagsReturnsHardFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&models.Setting{}))

	handler := NewFeatureFlagsHandler(db)

	// Scenario 1: No DB entry
	t.Run("no_db_entry", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/api/v1/feature-flags", nil)

		handler.GetFlags(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]bool
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.False(t, response["feature.notifications.legacy.fallback_enabled"], "Must return hard-false when no DB entry")
	})

	// Scenario 2: DB entry says true (invalid, forced false)
	t.Run("db_entry_true", func(t *testing.T) {
		// Force a true value in DB (simulating legacy state)
		setting := models.Setting{
			Key:      "feature.notifications.legacy.fallback_enabled",
			Value:    "true",
			Type:     "bool",
			Category: "feature",
		}
		db.Create(&setting)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/api/v1/feature-flags", nil)

		handler.GetFlags(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]bool
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.False(t, response["feature.notifications.legacy.fallback_enabled"], "Must return hard-false even when DB says true")

		// Clean up
		db.Unscoped().Delete(&setting)
	})

	// Scenario 3: DB entry says false
	t.Run("db_entry_false", func(t *testing.T) {
		setting := models.Setting{
			Key:      "feature.notifications.legacy.fallback_enabled",
			Value:    "false",
			Type:     "bool",
			Category: "feature",
		}
		db.Create(&setting)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/api/v1/feature-flags", nil)

		handler.GetFlags(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]bool
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.False(t, response["feature.notifications.legacy.fallback_enabled"], "Must return hard-false when DB says false")

		// Clean up
		db.Unscoped().Delete(&setting)
	})
}
