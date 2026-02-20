package handlers

import (
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
