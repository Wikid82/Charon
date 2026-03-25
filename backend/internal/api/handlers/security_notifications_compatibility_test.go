package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// SetupCompatibilityTestDB creates an in-memory database for testing (exported for use by other test files).
func SetupCompatibilityTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.NotificationProvider{},
		&models.NotificationConfig{},
		&models.Setting{},
	))

	// Enable feature flag by default in tests
	featureFlag := &models.Setting{
		Key:      "feature.notifications.security_provider_events.enabled",
		Value:    "true",
		Type:     "bool",
		Category: "feature",
	}
	require.NoError(t, db.Create(featureFlag).Error)

	return db
}

// TestCompatibilityGET_ORAggregation tests that GET uses OR semantics for aggregation.
func TestCompatibilityGET_ORAggregation(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	// Create two providers with different security event settings
	provider1 := &models.NotificationProvider{
		Name:                        "Provider 1",
		Type:                        "webhook",
		Enabled:                     true,
		NotifySecurityWAFBlocks:     true,
		NotifySecurityACLDenies:     false,
		NotifySecurityRateLimitHits: false,
	}
	provider2 := &models.NotificationProvider{
		Name:                        "Provider 2",
		Type:                        "discord",
		Enabled:                     true,
		NotifySecurityWAFBlocks:     false,
		NotifySecurityACLDenies:     true,
		NotifySecurityRateLimitHits: true,
	}
	require.NoError(t, db.Create(provider1).Error)
	require.NoError(t, db.Create(provider2).Error)

	// Create handler with enhanced service
	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(service)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/notifications/settings/security", http.NoBody)

	handler.GetSettings(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.NotificationConfig
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// OR semantics: if ANY provider has true, result is true
	assert.True(t, response.NotifyWAFBlocks, "WAF should be enabled (provider1=true)")
	assert.True(t, response.NotifyACLDenies, "ACL should be enabled (provider2=true)")
	assert.True(t, response.NotifyRateLimitHits, "Rate limit should be enabled (provider2=true)")
}

// TestCompatibilityGET_AllFalse tests that GET returns false when all providers are false.
func TestCompatibilityGET_AllFalse(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	// Create provider with all false
	provider := &models.NotificationProvider{
		Name:                        "Provider 1",
		Type:                        "webhook",
		Enabled:                     true,
		NotifySecurityWAFBlocks:     false,
		NotifySecurityACLDenies:     false,
		NotifySecurityRateLimitHits: false,
	}
	require.NoError(t, db.Create(provider).Error)

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(service)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/notifications/settings/security", http.NoBody)

	handler.GetSettings(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.NotificationConfig
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.False(t, response.NotifyWAFBlocks)
	assert.False(t, response.NotifyACLDenies)
	assert.False(t, response.NotifyRateLimitHits)
}

// TestCompatibilityGET_DisabledProvidersIgnored tests that disabled providers are not included in aggregation.
func TestCompatibilityGET_DisabledProvidersIgnored(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	// Create enabled and disabled providers
	enabled := &models.NotificationProvider{
		Name:                    "Enabled",
		Type:                    "webhook",
		Enabled:                 true,
		NotifySecurityWAFBlocks: false,
	}
	disabled := &models.NotificationProvider{
		Name:                    "Disabled",
		Type:                    "discord",
		Enabled:                 false,
		NotifySecurityWAFBlocks: true, // Should be ignored
	}
	require.NoError(t, db.Create(enabled).Error)
	require.NoError(t, db.Create(disabled).Error)

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(service)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/notifications/settings/security", http.NoBody)

	handler.GetSettings(c)

	var response models.NotificationConfig
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Disabled provider should not affect result
	assert.False(t, response.NotifyWAFBlocks, "Disabled provider should not contribute to OR")
}

// TestCompatibilityPUT_DeterministicTargetSet tests that PUT returns 410 Gone per R6 contract.
func TestCompatibilityPUT_DeterministicTargetSet(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	// Create one managed provider
	managed := &models.NotificationProvider{
		Name:                  "Migrated Security Notifications (Legacy)",
		Type:                  "webhook",
		Enabled:               true,
		ManagedLegacySecurity: true,
	}
	require.NoError(t, db.Create(managed).Error)

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(service)

	body := []byte(`{
		"security_waf_enabled": true,
		"security_acl_enabled": false,
		"security_rate_limit_enabled": true
	}`)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("PUT", "/api/v1/notifications/settings/security", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	// R6 contract: PUT returns 410 Gone
	assert.Equal(t, http.StatusGone, w.Code)

	// Verify no mutations occurred (provider unchanged)
	var updated models.NotificationProvider
	require.NoError(t, db.First(&updated, "id = ?", managed.ID).Error)
	assert.False(t, updated.NotifySecurityWAFBlocks, "Provider should not be mutated by deprecated endpoint")
	assert.False(t, updated.NotifySecurityACLDenies)
	assert.False(t, updated.NotifySecurityRateLimitHits)
}

// TestCompatibilityPUT_CreatesManagedProviderIfNone tests that PUT returns 410 Gone per R6 contract.
func TestCompatibilityPUT_CreatesManagedProviderIfNone(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(service)

	body := []byte(`{
		"security_waf_enabled": true,
		"security_acl_enabled": true,
		"security_rate_limit_enabled": false,
		"webhook_url": "https://example.com/webhook"
	}`)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("PUT", "/api/v1/notifications/settings/security", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	// R6 contract: PUT returns 410 Gone
	assert.Equal(t, http.StatusGone, w.Code)

	// Verify no provider was created
	var count int64
	db.Model(&models.NotificationProvider{}).Where("managed_legacy_security = ?", true).Count(&count)
	assert.Equal(t, int64(0), count, "No provider should be created by deprecated endpoint")
}

// TestCompatibilityPUT_Idempotency tests that PUT returns 410 Gone per R6 contract.
func TestCompatibilityPUT_Idempotency(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	managed := &models.NotificationProvider{
		Name:                        "Migrated Security Notifications (Legacy)",
		Type:                        "webhook",
		Enabled:                     true,
		ManagedLegacySecurity:       true,
		NotifySecurityWAFBlocks:     true,
		NotifySecurityACLDenies:     false,
		NotifySecurityRateLimitHits: true,
	}
	require.NoError(t, db.Create(managed).Error)

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(service)

	body := []byte(`{
		"security_waf_enabled": true,
		"security_acl_enabled": false,
		"security_rate_limit_enabled": true
	}`)

	// First PUT
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	setAdminContext(c1)
	c1.Request = httptest.NewRequest("PUT", "/api/v1/notifications/settings/security", bytes.NewBuffer(body))
	c1.Request.Header.Set("Content-Type", "application/json")
	handler.UpdateSettings(c1)
	assert.Equal(t, http.StatusGone, w1.Code, "R6 contract: PUT returns 410 Gone")

	var afterFirst models.NotificationProvider
	require.NoError(t, db.First(&afterFirst, "id = ?", managed.ID).Error)

	// Second PUT with identical payload
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	setAdminContext(c2)
	c2.Request = httptest.NewRequest("PUT", "/api/v1/notifications/settings/security", bytes.NewBuffer(body))
	c2.Request.Header.Set("Content-Type", "application/json")
	handler.UpdateSettings(c2)
	assert.Equal(t, http.StatusGone, w2.Code, "R6 contract: PUT returns 410 Gone")

	var afterSecond models.NotificationProvider
	require.NoError(t, db.First(&afterSecond, "id = ?", managed.ID).Error)

	// Values should remain identical (no mutations)
	assert.Equal(t, afterFirst.NotifySecurityWAFBlocks, afterSecond.NotifySecurityWAFBlocks)
	assert.Equal(t, afterFirst.NotifySecurityACLDenies, afterSecond.NotifySecurityACLDenies)
	assert.Equal(t, afterFirst.NotifySecurityRateLimitHits, afterSecond.NotifySecurityRateLimitHits)
	// Original values should be preserved
	assert.True(t, afterSecond.NotifySecurityWAFBlocks, "Original values preserved")
	assert.False(t, afterSecond.NotifySecurityACLDenies)
	assert.True(t, afterSecond.NotifySecurityRateLimitHits)
}

// TestCompatibilityPUT_WebhookMapping tests that PUT returns 410 Gone per R6 contract.
func TestCompatibilityPUT_WebhookMapping(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(service)

	body := []byte(`{
		"security_waf_enabled": true,
		"webhook_url": "https://example.com/webhook"
	}`)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("PUT", "/api/v1/notifications/settings/security", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	// R6 contract: PUT returns 410 Gone
	assert.Equal(t, http.StatusGone, w.Code)

	// Verify no provider was created
	var count int64
	db.Model(&models.NotificationProvider{}).Where("managed_legacy_security = ?", true).Count(&count)
	assert.Equal(t, int64(0), count, "No provider should be created by deprecated endpoint")
}

// TestCompatibilityPUT_MultipleDestinations422 tests that PUT returns 410 Gone per R6 contract.
func TestCompatibilityPUT_MultipleDestinations422(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(service)

	body := []byte(`{
		"security_waf_enabled": true,
		"webhook_url": "https://example.com/webhook",
		"discord_webhook_url": "https://discord.com/webhook"
	}`)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("PUT", "/api/v1/notifications/settings/security", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	// R6 contract: PUT returns 410 Gone regardless of payload
	assert.Equal(t, http.StatusGone, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "deprecated")
}

// TestMigrationMarker_Deterministic tests migration marker checksum and rerun logic.
func TestMigrationMarker_Deterministic(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	// Create legacy config
	legacyConfig := &models.NotificationConfig{
		NotifyWAFBlocks:     true,
		NotifyACLDenies:     false,
		NotifyRateLimitHits: true,
		WebhookURL:          "https://example.com/webhook",
	}
	require.NoError(t, db.Create(legacyConfig).Error)

	service := services.NewEnhancedSecurityNotificationService(db)

	// First migration
	err := service.MigrateFromLegacyConfig()
	require.NoError(t, err)

	// Verify migration marker was created
	var marker models.Setting
	err = db.Where("key = ?", "notifications.security_provider_events.migration.v1").First(&marker).Error
	require.NoError(t, err)

	var markerData MigrationMarker
	err = json.Unmarshal([]byte(marker.Value), &markerData)
	require.NoError(t, err)
	assert.Equal(t, "v1", markerData.Version)
	assert.NotEmpty(t, markerData.Checksum)
	assert.Equal(t, "completed", markerData.Result)

	// Verify provider was created
	var provider models.NotificationProvider
	err = db.Where("managed_legacy_security = ?", true).First(&provider).Error
	require.NoError(t, err)
	assert.True(t, provider.NotifySecurityWAFBlocks)
	assert.False(t, provider.NotifySecurityACLDenies)
	assert.True(t, provider.NotifySecurityRateLimitHits)

	// Second migration with same checksum should be no-op
	err = service.MigrateFromLegacyConfig()
	require.NoError(t, err)

	// Count providers - should still be 1
	var count int64
	db.Model(&models.NotificationProvider{}).Where("managed_legacy_security = ?", true).Count(&count)
	assert.Equal(t, int64(1), count)
}

// TestCompatibilityPUT_MultipleManagedProviders_UpdatesAll tests that PUT returns 410 Gone per R6 contract.
func TestCompatibilityPUT_MultipleManagedProviders_UpdatesAll(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	// Create two managed providers (valid scenario after migration)
	managed1 := &models.NotificationProvider{
		Name:                  "Migrated Security Notifications (Legacy)",
		Type:                  "webhook",
		Enabled:               true,
		ManagedLegacySecurity: true,
	}
	managed2 := &models.NotificationProvider{
		Name:                  "Duplicate Managed Provider",
		Type:                  "webhook",
		Enabled:               true,
		ManagedLegacySecurity: true,
	}
	require.NoError(t, db.Create(managed1).Error)
	require.NoError(t, db.Create(managed2).Error)

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(service)

	body := []byte(`{
		"security_waf_enabled": true,
		"security_acl_enabled": false,
		"security_rate_limit_enabled": true
	}`)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("PUT", "/api/v1/notifications/settings/security", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	// R6 contract: PUT returns 410 Gone
	assert.Equal(t, http.StatusGone, w.Code, "PUT must return 410 Gone per R6 deprecation contract")

	// Verify no providers were mutated
	var updated1, updated2 models.NotificationProvider
	require.NoError(t, db.First(&updated1, "id = ?", managed1.ID).Error)
	require.NoError(t, db.First(&updated2, "id = ?", managed2.ID).Error)

	assert.False(t, updated1.NotifySecurityWAFBlocks, "Provider should not be mutated by deprecated endpoint")
	assert.False(t, updated1.NotifySecurityACLDenies)
	assert.False(t, updated1.NotifySecurityRateLimitHits)

	assert.False(t, updated2.NotifySecurityWAFBlocks, "Provider should not be mutated by deprecated endpoint")
	assert.False(t, updated2.NotifySecurityACLDenies)
	assert.False(t, updated2.NotifySecurityRateLimitHits)
}

// TestFeatureFlagDefaultInitialization tests feature flag auto-initialization with correct defaults.
func TestFeatureFlagDefaultInitialization(t *testing.T) {
	tests := []struct {
		name            string
		ginMode         string
		expectedValue   bool
		setupTestMarker bool
	}{
		{
			name:          "Production (no GIN_MODE)",
			ginMode:       "",
			expectedValue: false,
		},
		{
			name:          "Development (GIN_MODE=debug)",
			ginMode:       "debug",
			expectedValue: true,
		},
		{
			name:          "Test (GIN_MODE=test)",
			ginMode:       "test",
			expectedValue: true,
		},
		{
			name:            "Test marker present",
			ginMode:         "",
			expectedValue:   true,
			setupTestMarker: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := SetupCompatibilityTestDB(t)

			// Clear the auto-created feature flag
			db.Where("key = ?", "feature.notifications.security_provider_events.enabled").Delete(&models.Setting{})

			if tt.setupTestMarker {
				testMarker := &models.Setting{
					Key:      "_test_mode_marker",
					Value:    "true",
					Type:     "bool",
					Category: "internal",
				}
				require.NoError(t, db.Create(testMarker).Error)
			}

			// Set GIN_MODE if specified
			if tt.ginMode != "" {
				_ = os.Setenv("GIN_MODE", tt.ginMode)
				defer func() { _ = os.Unsetenv("GIN_MODE") }()
			}

			service := services.NewEnhancedSecurityNotificationService(db)

			// Call method that checks feature flag (should auto-initialize)
			_, err := service.GetSettings()
			require.NoError(t, err)

			// Verify flag was created with correct default
			var setting models.Setting
			err = db.Where("key = ?", "feature.notifications.security_provider_events.enabled").First(&setting).Error
			require.NoError(t, err)

			expectedValueStr := "false"
			if tt.expectedValue {
				expectedValueStr = "true"
			}
			assert.Equal(t, expectedValueStr, setting.Value, "Feature flag should have correct default for %s", tt.name)
		})
	}
}

// TestFeatureFlag_Disabled tests behavior when feature flag is disabled.
func TestFeatureFlag_Disabled(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	// Update feature flag to false (SetupCompatibilityTestDB already created it as true)
	result := db.Model(&models.Setting{}).
		Where("key = ?", "feature.notifications.security_provider_events.enabled").
		Update("value", "false")
	require.NoError(t, result.Error)
	require.Equal(t, int64(1), result.RowsAffected)

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(service)

	// GET should still work via compatibility path
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/notifications/settings/security", http.NoBody)

	handler.GetSettings(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// CompatibilitySecuritySettings represents the compatibility GET response structure.
type CompatibilitySecuritySettings struct {
	SecurityWAFEnabled       bool   `json:"security_waf_enabled"`
	SecurityACLEnabled       bool   `json:"security_acl_enabled"`
	SecurityRateLimitEnabled bool   `json:"security_rate_limit_enabled"`
	Destination              string `json:"destination,omitempty"`
	DestinationAmbiguous     bool   `json:"destination_ambiguous,omitempty"`
}

// MigrationMarker represents the migration marker stored in settings.
type MigrationMarker struct {
	Version         string `json:"version"`
	Checksum        string `json:"checksum"`
	LastCompletedAt string `json:"last_completed_at"`
	Result          string `json:"result"`
}
