package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupSingleSourceTestDB creates a test database for single-source provider-event tests.
func setupSingleSourceTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&models.NotificationProvider{},
		&models.NotificationConfig{},
		&models.Setting{},
	)
	require.NoError(t, err)

	// Enable feature flag for provider-event model
	setting := &models.Setting{
		Key:      "feature.notifications.security_provider_events.enabled",
		Value:    "true",
		Type:     "bool",
		Category: "feature",
	}
	require.NoError(t, db.Create(setting).Error)

	return db
}

// TestR2_ProviderSecurityEventsCrowdSecDecisions tests that provider security controls include CrowdSec decisions (R2).
func TestR2_ProviderSecurityEventsCrowdSecDecisions(t *testing.T) {
	db := setupSingleSourceTestDB(t)

	// Create provider with CrowdSec decisions enabled
	provider := &models.NotificationProvider{
		Name:                            "Test Provider with CrowdSec",
		Type:                            "webhook",
		URL:                             "https://example.com/webhook",
		Enabled:                         true,
		NotifySecurityWAFBlocks:         true,
		NotifySecurityACLDenies:         false,
		NotifySecurityRateLimitHits:     true,
		NotifySecurityCrowdSecDecisions: true,
	}
	require.NoError(t, db.Create(provider).Error)

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(service)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/notifications/settings/security", http.NoBody)

	handler.GetSettings(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.NotificationConfig
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify CrowdSec decisions field is aggregated
	assert.True(t, response.NotifyCrowdSecDecisions, "security_crowdsec_enabled should be true")
	assert.True(t, response.NotifyWAFBlocks, "security_waf_enabled should be true")
	assert.False(t, response.NotifyACLDenies, "security_acl_enabled should be false")
	assert.True(t, response.NotifyRateLimitHits, "security_rate_limit_enabled should be true")
}

// TestR2_ProviderSecurityEventsCrowdSecDecisionsORSemantics tests OR aggregation for CrowdSec decisions.
func TestR2_ProviderSecurityEventsCrowdSecDecisionsORSemantics(t *testing.T) {
	db := setupSingleSourceTestDB(t)

	// Create two providers: one with CrowdSec enabled, one without
	provider1 := &models.NotificationProvider{
		Name:                            "Provider 1",
		Type:                            "webhook",
		Enabled:                         true,
		NotifySecurityCrowdSecDecisions: false,
	}
	provider2 := &models.NotificationProvider{
		Name:                            "Provider 2",
		Type:                            "discord",
		Enabled:                         true,
		NotifySecurityCrowdSecDecisions: true,
	}
	require.NoError(t, db.Create(provider1).Error)
	require.NoError(t, db.Create(provider2).Error)

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(service)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/notifications/settings/security", http.NoBody)

	handler.GetSettings(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.NotificationConfig
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// OR semantics: if ANY provider has true, result is true
	assert.True(t, response.NotifyCrowdSecDecisions, "CrowdSec should be enabled (OR semantics)")
}

// TestR6_LegacySecuritySettingsWrite410Gone tests that legacy write endpoints return 410 Gone (R6).
func TestR6_LegacySecuritySettingsWrite410Gone(t *testing.T) {
	db := setupSingleSourceTestDB(t)

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(service)

	gin.SetMode(gin.TestMode)

	// Test canonical endpoint: PUT /api/v1/notifications/settings/security
	t.Run("CanonicalEndpoint", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"security_waf_enabled":        true,
			"security_acl_enabled":        false,
			"security_rate_limit_enabled": true,
			"security_crowdsec_enabled":   true,
		}
		bodyBytes, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("PUT", "/api/v1/notifications/settings/security", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")
		// Simulate admin context
		c.Set("role", "admin")

		handler.UpdateSettings(c)

		assert.Equal(t, http.StatusGone, w.Code, "Canonical endpoint should return 410 Gone")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Verify 410 Gone contract fields
		assert.Equal(t, "legacy_security_settings_deprecated", response["error"])
		assert.Equal(t, "Use provider Notification Events.", response["message"])
		assert.Equal(t, "LEGACY_SECURITY_SETTINGS_DEPRECATED", response["code"])
	})

	// Test deprecated endpoint: PUT /api/v1/security/notifications/settings
	t.Run("DeprecatedEndpoint", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"security_waf_enabled": true,
		}
		bodyBytes, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("PUT", "/api/v1/security/notifications/settings", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")
		// Simulate admin context
		c.Set("role", "admin")

		handler.DeprecatedUpdateSettings(c)

		assert.Equal(t, http.StatusGone, w.Code, "Deprecated endpoint should return 410 Gone")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Verify 410 Gone contract fields
		assert.Equal(t, "legacy_security_settings_deprecated", response["error"])
		assert.Equal(t, "Use provider Notification Events.", response["message"])
		assert.Equal(t, "LEGACY_SECURITY_SETTINGS_DEPRECATED", response["code"])
	})
}

// TestR6_LegacyWrite410GoneNoMutation tests that 410 Gone does not mutate provider state.
func TestR6_LegacyWrite410GoneNoMutation(t *testing.T) {
	db := setupSingleSourceTestDB(t)

	// Create a provider with specific state
	provider := &models.NotificationProvider{
		Name:                            "Test Provider",
		Type:                            "webhook",
		Enabled:                         true,
		NotifySecurityWAFBlocks:         false,
		NotifySecurityCrowdSecDecisions: false,
	}
	require.NoError(t, db.Create(provider).Error)

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(service)

	gin.SetMode(gin.TestMode)

	// Attempt PUT to canonical endpoint
	reqBody := map[string]interface{}{
		"security_waf_enabled":      true,
		"security_crowdsec_enabled": true,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/v1/notifications/settings/security", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("role", "admin")

	handler.UpdateSettings(c)

	assert.Equal(t, http.StatusGone, w.Code)

	// Verify provider state was NOT mutated
	var fetchedProvider models.NotificationProvider
	err := db.First(&fetchedProvider, "id = ?", provider.ID).Error
	require.NoError(t, err)

	assert.False(t, fetchedProvider.NotifySecurityWAFBlocks, "WAF should remain false (no mutation)")
	assert.False(t, fetchedProvider.NotifySecurityCrowdSecDecisions, "CrowdSec should remain false (no mutation)")
}

// TestProviderCRUD_SecurityEventsIncludeCrowdSec tests provider create/update persists CrowdSec decisions.
func TestProviderCRUD_SecurityEventsIncludeCrowdSec(t *testing.T) {
	db := setupSingleSourceTestDB(t)

	service := services.NewNotificationService(db)
	handler := NewNotificationProviderHandler(service)

	gin.SetMode(gin.TestMode)

	// Test CREATE
	t.Run("CreatePersistsCrowdSec", func(t *testing.T) {
		reqBody := notificationProviderUpsertRequest{
			Name:                            "CrowdSec Provider",
			Type:                            "discord",
			URL:                             "https://discord.com/api/webhooks/123/abc",
			Enabled:                         true,
			NotifySecurityWAFBlocks:         true,
			NotifySecurityCrowdSecDecisions: true,
		}
		bodyBytes, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/v1/notification-providers", bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("role", "admin")

		handler.Create(c)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response models.NotificationProvider
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.NotifySecurityWAFBlocks, "WAF should be persisted")
		assert.True(t, response.NotifySecurityCrowdSecDecisions, "CrowdSec should be persisted")
	})

	// Test UPDATE
	t.Run("UpdatePersistsCrowdSec", func(t *testing.T) {
		// Create initial provider
		provider := &models.NotificationProvider{
			Name:                            "Initial Provider",
			Type:                            "discord",
			URL:                             "https://discord.com/api/webhooks/123/abc",
			Enabled:                         true,
			NotifySecurityCrowdSecDecisions: false,
		}
		require.NoError(t, db.Create(provider).Error)

		// Update to enable CrowdSec
		reqBody := notificationProviderUpsertRequest{
			Name:                            "Updated Provider",
			Type:                            "discord",
			URL:                             "https://discord.com/api/webhooks/123/abc",
			Enabled:                         true,
			NotifySecurityCrowdSecDecisions: true,
		}
		bodyBytes, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("PUT", "/api/v1/notification-providers/"+provider.ID, bytes.NewReader(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "id", Value: provider.ID}}
		c.Set("role", "admin")

		handler.Update(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.NotificationProvider
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.NotifySecurityCrowdSecDecisions, "CrowdSec should be updated to true")
	})
}

// TestR2_CompatibilityGETIncludesCrowdSec tests that compatibility response includes security_crowdsec_enabled.
func TestR2_CompatibilityGETIncludesCrowdSec(t *testing.T) {
	db := setupSingleSourceTestDB(t)

	provider := &models.NotificationProvider{
		Name:                            "Test Provider",
		Type:                            "webhook",
		Enabled:                         true,
		NotifySecurityCrowdSecDecisions: true,
	}
	require.NoError(t, db.Create(provider).Error)

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(service)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/notifications/settings/security", http.NoBody)

	handler.GetSettings(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify JSON response includes correct field name
	var rawResponse map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &rawResponse)
	require.NoError(t, err)

	assert.Equal(t, true, rawResponse["security_crowdsec_enabled"], "API should return security_crowdsec_enabled field")

	// Verify notify_* field is NOT present (API contract check)
	_, hasNotifyCrowdSec := rawResponse["notify_crowdsec_decisions"]
	assert.False(t, hasNotifyCrowdSec, "API should NOT expose notify_crowdsec_decisions (use security_crowdsec_enabled)")
}
