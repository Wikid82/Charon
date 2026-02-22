package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestEnhancedService_GetSettings_FeatureFlagError covers lines 60-61
func TestEnhancedService_GetSettings_FeatureFlagError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// Don't run migrations - will cause DB error

	service := NewEnhancedSecurityNotificationService(db)

	// Lines 60-61: Should return error when feature flag check fails
	config, err := service.GetSettings()
	assert.Error(t, err)
	assert.Nil(t, config)
}

// TestEnhancedService_GetProviderAggregatedConfig_QueryError covers lines 81-82
func TestEnhancedService_GetProviderAggregatedConfig_QueryError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	// Don't migrate NotificationProvider - will cause query error

	// Enable feature flag
	db.Create(&models.Setting{
		Key:   "feature.notifications.security_provider_events.enabled",
		Value: "true",
		Type:  "bool",
	})

	service := NewEnhancedSecurityNotificationService(db)

	// Lines 81-82: Should return error when provider query fails
	config, err := service.GetSettings()
	assert.Error(t, err)
	assert.Nil(t, config)
}

// TestEnhancedService_GetProviderAggregatedConfig_GotifyNoTokenExposure covers lines 118-119
func TestEnhancedService_GetProviderAggregatedConfig_GotifyNoTokenExposure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Setting{}))

	db.Create(&models.Setting{
		Key:   "feature.notifications.security_provider_events.enabled",
		Value: "true",
		Type:  "bool",
	})

	// Create managed gotify provider
	db.Create(&models.NotificationProvider{
		ID:                      "gotify",
		Name:                    "Test Gotify",
		Type:                    "gotify",
		URL:                     "https://gotify.example.com",
		Enabled:                 true,
		ManagedLegacySecurity:   true,
		NotifySecurityWAFBlocks: true,
	})

	service := NewEnhancedSecurityNotificationService(db)

	config, err := service.GetSettings()
	require.NoError(t, err)

	// Lines 118-119: Should set GotifyURL but NOT expose token
	assert.Equal(t, "https://gotify.example.com", config.GotifyURL)
	// Token field should remain empty/default in response
	assert.Empty(t, config.GotifyToken, "Gotify token must not be exposed in GET response")
}

// TestEnhancedService_UpdateSettings_FeatureFlagError covers lines 173-174
func TestEnhancedService_UpdateSettings_FeatureFlagError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// Don't run migrations - will cause DB error

	service := NewEnhancedSecurityNotificationService(db)

	req := &models.NotificationConfig{
		NotifyWAFBlocks: true,
	}

	// Lines 173-174: Should return error when feature flag check fails
	err = service.UpdateSettings(req)
	assert.Error(t, err)
}

// TestEnhancedService_SendViaProviders_NonDiscordFiltered covers lines 327-329
func TestEnhancedService_SendViaProviders_NonDiscordFiltered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Setting{}))

	db.Create(&models.Setting{
		Key:   "feature.notifications.security_provider_events.enabled",
		Value: "true",
		Type:  "bool",
	})

	// Create non-discord provider
	db.Create(&models.NotificationProvider{
		ID:                      "webhook",
		Name:                    "Webhook",
		Type:                    "webhook",
		URL:                     "https://example.com",
		Enabled:                 true,
		NotifySecurityWAFBlocks: true,
	})

	service := NewEnhancedSecurityNotificationService(db)

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "warn",
		Message:   "Test",
		ClientIP:  "192.168.1.1",
		Timestamp: time.Now(),
	}

	// Lines 327-329: Should filter out non-discord providers
	err = service.SendViaProviders(context.Background(), event)
	assert.NoError(t, err) // No error, but webhook was filtered
}

// TestEnhancedService_SendViaProviders_DisabledProvider covers lines 331-332
func TestEnhancedService_SendViaProviders_DisabledProvider(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Setting{}))

	db.Create(&models.Setting{
		Key:   "feature.notifications.security_provider_events.enabled",
		Value: "true",
		Type:  "bool",
	})

	// Create disabled discord provider
	db.Create(&models.NotificationProvider{
		ID:                      "discord",
		Name:                    "Discord",
		Type:                    "discord",
		URL:                     "https://discord.com/api/webhooks/123/abc",
		Enabled:                 false, // Disabled
		NotifySecurityWAFBlocks: true,
	})

	service := NewEnhancedSecurityNotificationService(db)

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "warn",
		Message:   "Test",
		ClientIP:  "192.168.1.1",
		Timestamp: time.Now(),
	}

	// Lines 331-332: Should skip disabled providers
	err = service.SendViaProviders(context.Background(), event)
	assert.NoError(t, err)
}

// TestEnhancedService_SendViaProviders_EventTypeNotSubscribed covers lines 341-342
func TestEnhancedService_SendViaProviders_EventTypeNotSubscribed(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Setting{}))

	db.Create(&models.Setting{
		Key:   "feature.notifications.security_provider_events.enabled",
		Value: "true",
		Type:  "bool",
	})

	// Create discord provider subscribed to WAF but not ACL
	db.Create(&models.NotificationProvider{
		ID:                      "discord",
		Name:                    "Discord",
		Type:                    "discord",
		URL:                     "https://discord.com/api/webhooks/123/abc",
		Enabled:                 true,
		NotifySecurityWAFBlocks: true,
		NotifySecurityACLDenies: false, // Not subscribed to ACL
	})

	service := NewEnhancedSecurityNotificationService(db)

	event := models.SecurityEvent{
		EventType: "acl_deny", // Event type not subscribed
		Severity:  "warn",
		Message:   "Test",
		ClientIP:  "192.168.1.1",
		Timestamp: time.Now(),
	}

	// Lines 341-342: Should skip when event type not subscribed
	err = service.SendViaProviders(context.Background(), event)
	assert.NoError(t, err)
}

// TestEnhancedService_SendViaProviders_UnknownEventType covers lines 352-353
func TestEnhancedService_SendViaProviders_UnknownEventType(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Setting{}))

	db.Create(&models.Setting{
		Key:   "feature.notifications.security_provider_events.enabled",
		Value: "true",
		Type:  "bool",
	})

	db.Create(&models.NotificationProvider{
		ID:                      "discord",
		Name:                    "Discord",
		Type:                    "discord",
		URL:                     "https://discord.com/api/webhooks/123/abc",
		Enabled:                 true,
		NotifySecurityWAFBlocks: true,
	})

	service := NewEnhancedSecurityNotificationService(db)

	event := models.SecurityEvent{
		EventType: "unknown_event_type", // Unknown event type
		Severity:  "warn",
		Message:   "Test",
		ClientIP:  "192.168.1.1",
		Timestamp: time.Now(),
	}

	// Lines 352-353: Should skip unknown event types (default false)
	err = service.SendViaProviders(context.Background(), event)
	assert.NoError(t, err)
}

// TestEnhancedService_SendViaProviders_HTTPRequestError covers lines 422-423
func TestEnhancedService_SendViaProviders_HTTPRequestError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Setting{}))

	db.Create(&models.Setting{
		Key:   "feature.notifications.security_provider_events.enabled",
		Value: "true",
		Type:  "bool",
	})

	// Create discord provider with invalid URL
	db.Create(&models.NotificationProvider{
		ID:                      "discord",
		Name:                    "Discord",
		Type:                    "discord",
		URL:                     "https://invalid-url-that-will-fail-dns.local",
		Enabled:                 true,
		NotifySecurityWAFBlocks: true,
	})

	service := NewEnhancedSecurityNotificationService(db)

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "warn",
		Message:   "Test",
		ClientIP:  "192.168.1.1",
		Timestamp: time.Now(),
	}

	// Lines 422-423: Should handle HTTP request errors but not fail
	err = service.SendViaProviders(context.Background(), event)
	// Service logs error but doesn't fail - continues to next provider
	assert.NoError(t, err)
}

// TestEnhancedService_SendViaProviders_Non2xxResponse covers lines 436-437
func TestEnhancedService_SendViaProviders_Non2xxResponse(t *testing.T) {
	// Create mock server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Setting{}))

	db.Create(&models.Setting{
		Key:   "feature.notifications.security_provider_events.enabled",
		Value: "true",
		Type:  "bool",
	})

	db.Create(&models.NotificationProvider{
		ID:                      "discord",
		Name:                    "Discord",
		Type:                    "discord",
		URL:                     server.URL,
		Enabled:                 true,
		NotifySecurityWAFBlocks: true,
	})

	service := NewEnhancedSecurityNotificationService(db)

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "warn",
		Message:   "Test",
		ClientIP:  "192.168.1.1",
		Timestamp: time.Now(),
	}

	// Lines 436-437: Should handle non-2xx response but not fail
	err = service.SendViaProviders(context.Background(), event)
	// Service logs error but doesn't fail - continues to next provider
	assert.NoError(t, err)
}
