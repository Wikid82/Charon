package handlers

import (
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
)

// TestFinalBlocker1_DestinationAmbiguous_ZeroManagedProviders tests that destination_ambiguous=true when no managed providers exist.
func TestFinalBlocker1_DestinationAmbiguous_ZeroManagedProviders(t *testing.T) {
	db := setupCompatibilityTestDB(t)

	// Create a non-managed provider
	provider := &models.NotificationProvider{
		Name:                    "Regular Provider",
		Type:                    "webhook",
		Enabled:                 true,
		ManagedLegacySecurity:   false,
		NotifySecurityWAFBlocks: true,
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

	// Zero managed providers should result in ambiguous=true
	assert.True(t, response.DestinationAmbiguous, "destination_ambiguous should be true when zero managed providers exist")
	assert.Empty(t, response.WebhookURL, "No destination should be reported")
}

// TestFinalBlocker1_DestinationAmbiguous_OneManagedProvider tests that destination_ambiguous=false when exactly one managed provider exists.
func TestFinalBlocker1_DestinationAmbiguous_OneManagedProvider(t *testing.T) {
	db := setupCompatibilityTestDB(t)

	// Create one managed provider
	provider := &models.NotificationProvider{
		Name:                    "Managed Provider",
		Type:                    "webhook",
		URL:                     "https://example.com/webhook",
		Enabled:                 true,
		ManagedLegacySecurity:   true,
		NotifySecurityWAFBlocks: true,
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

	// Exactly one managed provider should result in ambiguous=false
	assert.False(t, response.DestinationAmbiguous, "destination_ambiguous should be false when exactly one managed provider exists")
	assert.Equal(t, "https://example.com/webhook", response.WebhookURL, "Destination should be reported")
}

// TestFinalBlocker1_DestinationAmbiguous_MultipleManagedProviders tests that destination_ambiguous=true when multiple managed providers exist.
func TestFinalBlocker1_DestinationAmbiguous_MultipleManagedProviders(t *testing.T) {
	db := setupCompatibilityTestDB(t)

	// Create two managed providers
	provider1 := &models.NotificationProvider{
		Name:                    "Managed Provider 1",
		Type:                    "webhook",
		URL:                     "https://example.com/webhook1",
		Enabled:                 true,
		ManagedLegacySecurity:   true,
		NotifySecurityWAFBlocks: true,
	}
	provider2 := &models.NotificationProvider{
		Name:                    "Managed Provider 2",
		Type:                    "discord",
		URL:                     "https://discord.com/webhook",
		Enabled:                 true,
		ManagedLegacySecurity:   true,
		NotifySecurityACLDenies: true,
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

	// Multiple managed providers should result in ambiguous=true
	assert.True(t, response.DestinationAmbiguous, "destination_ambiguous should be true when multiple managed providers exist")
	assert.Empty(t, response.WebhookURL, "No single destination should be reported")
	assert.Empty(t, response.DiscordWebhookURL, "No single destination should be reported")
}

// TestFinalBlocker2_TokenNotExposed tests that provider tokens are not exposed in API responses.
func TestFinalBlocker2_TokenNotExposed(t *testing.T) {
	db := setupCompatibilityTestDB(t)

	// Create a gotify provider with token
	provider := &models.NotificationProvider{
		Name:                    "Gotify Provider",
		Type:                    "gotify",
		URL:                     "https://gotify.example.com",
		Token:                   "secret_gotify_token_12345",
		Enabled:                 true,
		NotifySecurityWAFBlocks: true,
	}
	require.NoError(t, db.Create(provider).Error)

	// Fetch provider via API simulation
	var fetchedProvider models.NotificationProvider
	err := db.First(&fetchedProvider, "id = ?", provider.ID).Error
	require.NoError(t, err)

	// Serialize to JSON (simulating API response)
	jsonBytes, err := json.Marshal(fetchedProvider)
	require.NoError(t, err)

	// Parse back to map to check field presence
	var response map[string]interface{}
	err = json.Unmarshal(jsonBytes, &response)
	require.NoError(t, err)

	// Token field should NOT be present in JSON
	_, tokenExists := response["token"]
	assert.False(t, tokenExists, "Token field should not be present in JSON response (json:\"-\" tag)")

	// But Token should still be accessible in Go code
	assert.Equal(t, "secret_gotify_token_12345", fetchedProvider.Token, "Token should still be accessible in Go code")
}

// TestFinalBlocker3_SupportedProviderTypes_WebhookDiscordSlackGotifyOnly tests that only supported types are processed.
func TestFinalBlocker3_SupportedProviderTypes_WebhookDiscordSlackGotifyOnly(t *testing.T) {
	db := setupCompatibilityTestDB(t)

	// Create providers of various types
	supportedTypes := []string{"webhook", "discord", "slack", "gotify"}
	unsupportedTypes := []string{"telegram", "generic", "unknown"}

	// Create supported providers
	for _, providerType := range supportedTypes {
		provider := &models.NotificationProvider{
			Name:                    providerType + " provider",
			Type:                    providerType,
			Enabled:                 true,
			NotifySecurityWAFBlocks: true,
		}
		require.NoError(t, db.Create(provider).Error)
	}

	// Create unsupported providers
	for _, providerType := range unsupportedTypes {
		provider := &models.NotificationProvider{
			Name:                    providerType + " provider",
			Type:                    providerType,
			Enabled:                 true,
			NotifySecurityWAFBlocks: true,
		}
		require.NoError(t, db.Create(provider).Error)
	}

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

	// WAF should be enabled because supported types have it enabled
	// Unsupported types should be filtered out and not affect the aggregation
	assert.True(t, response.NotifyWAFBlocks, "WAF should be enabled (supported providers have it enabled)")
}

// TestFinalBlocker3_SupportedProviderTypes_UnsupportedTypesIgnored tests that unsupported types are completely filtered out.
func TestFinalBlocker3_SupportedProviderTypes_UnsupportedTypesIgnored(t *testing.T) {
	db := setupCompatibilityTestDB(t)

	// Create ONLY unsupported providers
	unsupportedTypes := []string{"telegram", "generic"}

	for _, providerType := range unsupportedTypes {
		provider := &models.NotificationProvider{
			Name:                    providerType + " provider",
			Type:                    providerType,
			Enabled:                 true,
			NotifySecurityWAFBlocks: true,
			NotifySecurityACLDenies: true,
		}
		require.NoError(t, db.Create(provider).Error)
	}

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

	// All providers are unsupported, so all flags should be false
	assert.False(t, response.NotifyWAFBlocks, "WAF should be disabled (no supported providers)")
	assert.False(t, response.NotifyACLDenies, "ACL should be disabled (no supported providers)")
	assert.False(t, response.NotifyRateLimitHits, "Rate limit should be disabled (no supported providers)")
}

// TestBlocker2_GETReturnsSecurityFields tests GET returns security_* fields per spec.
// Blocker 2: Compatibility endpoint contract must use explicit security_* payload fields per spec.
func TestBlocker2_GETReturnsSecurityFields(t *testing.T) {
	db := setupCompatibilityTestDB(t)

	provider := &models.NotificationProvider{
		Name:                        "Test Provider",
		Type:                        "webhook",
		Enabled:                     true,
		NotifySecurityWAFBlocks:     true,
		NotifySecurityACLDenies:     false,
		NotifySecurityRateLimitHits: true,
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

	// Blocker 2 Fix: Verify API returns security_* field names per spec
	var rawResponse map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &rawResponse)
	require.NoError(t, err)

	// Check field names in JSON (not Go struct field names)
	assert.Equal(t, true, rawResponse["security_waf_enabled"], "API should return security_waf_enabled=true")
	assert.Equal(t, false, rawResponse["security_acl_enabled"], "API should return security_acl_enabled=false")
	assert.Equal(t, true, rawResponse["security_rate_limit_enabled"], "API should return security_rate_limit_enabled=true")

	// Verify notify_* fields are NOT present (spec drift check)
	_, hasNotifyWAF := rawResponse["notify_waf_blocks"]
	assert.False(t, hasNotifyWAF, "API should NOT expose notify_waf_blocks (use security_waf_enabled)")
}

// TestBlocker2_GotifyTokenNeverExposed_Legacy tests that gotify token is never exposed in GET responses.
func TestBlocker2_GotifyTokenNeverExposed_Legacy(t *testing.T) {
	db := setupCompatibilityTestDB(t)

	// Create gotify provider with token
	provider := &models.NotificationProvider{
		Name:                    "Gotify Provider",
		Type:                    "gotify",
		URL:                     "https://gotify.example.com",
		Token:                   "secret_gotify_token_xyz",
		Enabled:                 true,
		ManagedLegacySecurity:   true,
		NotifySecurityWAFBlocks: true,
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

	// Blocker 2: Gotify token must NEVER be exposed in GET responses
	assert.Empty(t, response.GotifyToken, "Gotify token must not be exposed in GET response")
	assert.Equal(t, "https://gotify.example.com", response.GotifyURL, "Gotify URL should be returned")
}

// TestBlocker3_PUTIdempotency tests that identical PUT requests do not mutate timestamps.
func TestBlocker3_PUTIdempotency(t *testing.T) {
	db := setupCompatibilityTestDB(t)

	// Create managed provider
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

	// Read initial timestamps
	var initial models.NotificationProvider
	require.NoError(t, db.First(&initial, "id = ?", managed.ID).Error)
	initialUpdatedAt := initial.UpdatedAt

	service := services.NewEnhancedSecurityNotificationService(db)

	// Perform identical PUT
	req := &models.NotificationConfig{
		NotifyWAFBlocks:     true,
		NotifyACLDenies:     false,
		NotifyRateLimitHits: true,
	}
	err := service.UpdateSettings(req)
	require.NoError(t, err)

	// Read timestamps again
	var afterPUT models.NotificationProvider
	require.NoError(t, db.First(&afterPUT, "id = ?", managed.ID).Error)

	// Blocker 3: Timestamps must NOT change when effective values are identical
	assert.Equal(t, initialUpdatedAt.Unix(), afterPUT.UpdatedAt.Unix(), "UpdatedAt should not change for identical PUT")
}

// TestBlocker4_MultipleManagedProvidersAllowed tests that multiple managed providers are updated (not 409).
func TestBlocker4_MultipleManagedProvidersAllowed(t *testing.T) {
	db := setupCompatibilityTestDB(t)

	// Create two managed providers (simulating migration state)
	managed1 := &models.NotificationProvider{
		Name:                        "Managed Provider 1",
		Type:                        "webhook",
		Enabled:                     true,
		ManagedLegacySecurity:       true,
		NotifySecurityWAFBlocks:     false,
		NotifySecurityACLDenies:     false,
		NotifySecurityRateLimitHits: false,
	}
	managed2 := &models.NotificationProvider{
		Name:                        "Managed Provider 2",
		Type:                        "discord",
		Enabled:                     true,
		ManagedLegacySecurity:       true,
		NotifySecurityWAFBlocks:     false,
		NotifySecurityACLDenies:     false,
		NotifySecurityRateLimitHits: false,
	}
	require.NoError(t, db.Create(managed1).Error)
	require.NoError(t, db.Create(managed2).Error)

	service := services.NewEnhancedSecurityNotificationService(db)

	// Perform PUT - should update ALL managed providers (not 409)
	req := &models.NotificationConfig{
		NotifyWAFBlocks:     true,
		NotifyACLDenies:     true,
		NotifyRateLimitHits: false,
	}
	err := service.UpdateSettings(req)
	require.NoError(t, err, "Multiple managed providers should be allowed and updated")

	// Verify both providers were updated
	var updated1, updated2 models.NotificationProvider
	require.NoError(t, db.First(&updated1, "id = ?", managed1.ID).Error)
	require.NoError(t, db.First(&updated2, "id = ?", managed2.ID).Error)

	assert.True(t, updated1.NotifySecurityWAFBlocks)
	assert.True(t, updated1.NotifySecurityACLDenies)
	assert.False(t, updated1.NotifySecurityRateLimitHits)

	assert.True(t, updated2.NotifySecurityWAFBlocks)
	assert.True(t, updated2.NotifySecurityACLDenies)
	assert.False(t, updated2.NotifySecurityRateLimitHits)
}

// TestBlocker1_FeatureFlagDefaultProduction tests that prod environment defaults to false.
func TestBlocker1_FeatureFlagDefaultProduction(t *testing.T) {
	db := setupCompatibilityTestDB(t)

	// Clear the auto-created feature flag
	db.Where("key = ?", "feature.notifications.security_provider_events.enabled").Delete(&models.Setting{})

	// Set CHARON_ENV=production
	require.NoError(t, os.Setenv("CHARON_ENV", "production"))
	defer func() { _ = os.Unsetenv("CHARON_ENV") }()

	service := services.NewEnhancedSecurityNotificationService(db)

	// Trigger flag initialization
	_, err := service.GetSettings()
	require.NoError(t, err)

	// Verify flag was created with false default
	var setting models.Setting
	err = db.Where("key = ?", "feature.notifications.security_provider_events.enabled").First(&setting).Error
	require.NoError(t, err)

	assert.Equal(t, "false", setting.Value, "Production must default to false")
}

// TestBlocker1_FeatureFlagDefaultProductionUnsetEnv tests prod default when no env vars set.
func TestBlocker1_FeatureFlagDefaultProductionUnsetEnv(t *testing.T) {
	db := setupCompatibilityTestDB(t)

	// Clear the auto-created feature flag
	db.Where("key = ?", "feature.notifications.security_provider_events.enabled").Delete(&models.Setting{})

	// Clear both CHARON_ENV and GIN_MODE to simulate production without explicit env vars
	_ = os.Unsetenv("CHARON_ENV")
	_ = os.Unsetenv("GIN_MODE")

	service := services.NewEnhancedSecurityNotificationService(db)

	// Trigger flag initialization
	_, err := service.GetSettings()
	require.NoError(t, err)

	// Blocker 1 Fix: When no env vars set, must default to false (production)
	var setting models.Setting
	err = db.Where("key = ?", "feature.notifications.security_provider_events.enabled").First(&setting).Error
	require.NoError(t, err)

	assert.Equal(t, "false", setting.Value, "Unset env vars must default to false (production)")
}

// TestBlocker1_FeatureFlagDefaultDev tests that dev/test environment defaults to true.
func TestBlocker1_FeatureFlagDefaultDev(t *testing.T) {
	db := setupCompatibilityTestDB(t)

	// Clear the auto-created feature flag
	db.Where("key = ?", "feature.notifications.security_provider_events.enabled").Delete(&models.Setting{})

	// Set GIN_MODE=test to simulate test environment
	require.NoError(t, os.Setenv("GIN_MODE", "test"))
	defer func() { _ = os.Unsetenv("GIN_MODE") }()

	// Ensure CHARON_ENV is not set to production
	_ = os.Unsetenv("CHARON_ENV")

	service := services.NewEnhancedSecurityNotificationService(db)

	// Trigger flag initialization
	_, err := service.GetSettings()
	require.NoError(t, err)

	// Verify flag was created with true default (test mode)
	var setting models.Setting
	err = db.Where("key = ?", "feature.notifications.security_provider_events.enabled").First(&setting).Error
	require.NoError(t, err)

	assert.Equal(t, "true", setting.Value, "Dev/test must default to true")
}
