package services

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

func TestEnhancedService_GetProviderAggregatedConfig_CrowdSecFlag(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Setting{}))

	db.Create(&models.Setting{Key: "feature.notifications.security_provider_events.enabled", Value: "true", Type: "bool"})
	db.Create(&models.NotificationProvider{
		ID:                              "discord-crowdsec",
		Name:                            "Discord CrowdSec",
		Type:                            "discord",
		URL:                             "https://discord.com/api/webhooks/123/abc",
		Enabled:                         true,
		ManagedLegacySecurity:           true,
		NotifySecurityCrowdSecDecisions: true,
	})

	service := NewEnhancedSecurityNotificationService(db)
	config, err := service.GetSettings()
	require.NoError(t, err)
	assert.True(t, config.NotifyCrowdSecDecisions)
}

func TestEnhancedService_UpdateManagedProviders_SlackDestinationContributesToAmbiguity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.NotificationConfig{}, &models.Setting{}))

	service := NewEnhancedSecurityNotificationService(db)
	err = service.updateManagedProviders(&models.NotificationConfig{
		DiscordWebhookURL: "https://discord.com/api/webhooks/123/abc",
		SlackWebhookURL:   "https://hooks.slack.com/services/T/B/X",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous destination")
}

func TestEnhancedService_UpdateManagedProviders_QueryManagedProvidersError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.NotificationConfig{}, &models.Setting{}))

	service := NewEnhancedSecurityNotificationService(db)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	err = service.updateManagedProviders(&models.NotificationConfig{DiscordWebhookURL: "https://discord.com/api/webhooks/123/abc"})
	require.Error(t, err)
}

func TestEnhancedService_UpdateManagedProviders_ChangesACLTypeAndToken(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.NotificationConfig{}, &models.Setting{}))

	provider := models.NotificationProvider{
		ID:                          "managed-change",
		Type:                        "webhook",
		URL:                         "https://example.com/webhook",
		Token:                       "old-token",
		Enabled:                     true,
		ManagedLegacySecurity:       true,
		NotifySecurityWAFBlocks:     true,
		NotifySecurityACLDenies:     false,
		NotifySecurityRateLimitHits: false,
	}
	require.NoError(t, db.Create(&provider).Error)

	service := NewEnhancedSecurityNotificationService(db)
	err = service.updateManagedProviders(&models.NotificationConfig{
		NotifyWAFBlocks:     true,
		NotifyACLDenies:     true,
		NotifyRateLimitHits: false,
		GotifyURL:           "https://gotify.example.com",
		GotifyToken:         "new-token",
	})
	require.NoError(t, err)

	var updated models.NotificationProvider
	require.NoError(t, db.First(&updated, "id = ?", "managed-change").Error)
	assert.True(t, updated.NotifySecurityACLDenies)
	assert.Equal(t, "gotify", updated.Type)
	assert.Equal(t, "new-token", updated.Token)
}

func TestEnhancedService_UpdateManagedProviders_SaveError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "enhanced-save-error.db")

	rwDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, rwDB.AutoMigrate(&models.NotificationProvider{}, &models.NotificationConfig{}, &models.Setting{}))
	require.NoError(t, rwDB.Create(&models.NotificationProvider{
		ID:                      "managed-readonly",
		Type:                    "discord",
		URL:                     "https://discord.com/api/webhooks/123/abc",
		Enabled:                 true,
		ManagedLegacySecurity:   true,
		NotifySecurityWAFBlocks: false,
	}).Error)

	rwSQL, err := rwDB.DB()
	require.NoError(t, err)
	require.NoError(t, rwSQL.Close())

	roDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=ro", dbPath)), &gorm.Config{})
	require.NoError(t, err)
	service := NewEnhancedSecurityNotificationService(roDB)

	err = service.updateManagedProviders(&models.NotificationConfig{
		NotifyWAFBlocks:   true,
		DiscordWebhookURL: "https://discord.com/api/webhooks/123/abc",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update provider")
}

func TestEnhancedService_UpdateLegacyConfig_DBErrorAndUpdatePath(t *testing.T) {
	t.Run("db_error", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.NotificationConfig{}))
		service := NewEnhancedSecurityNotificationService(db)

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		err = service.updateLegacyConfig(&models.NotificationConfig{NotifyWAFBlocks: true})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fetch existing config")
	})

	t.Run("update_existing_preserves_id", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.NotificationConfig{}))
		service := NewEnhancedSecurityNotificationService(db)

		existing := models.NotificationConfig{ID: "legacy-id", NotifyWAFBlocks: false}
		require.NoError(t, db.Create(&existing).Error)

		req := &models.NotificationConfig{NotifyWAFBlocks: true}
		require.NoError(t, service.updateLegacyConfig(req))
		assert.Equal(t, "legacy-id", req.ID)
	})
}

func TestEnhancedService_MigrateFromLegacyConfig_PreTransactionErrors(t *testing.T) {
	t.Run("feature_flag_error", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		require.NoError(t, err)
		service := NewEnhancedSecurityNotificationService(db)

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		err = service.MigrateFromLegacyConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "check feature flag")
	})

	t.Run("read_legacy_config_error", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.Setting{}))
		require.NoError(t, db.Create(&models.Setting{Key: "feature.notifications.security_provider_events.enabled", Value: "true", Type: "bool"}).Error)

		service := NewEnhancedSecurityNotificationService(db)
		err = service.MigrateFromLegacyConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read legacy config")
	})
}

func TestEnhancedService_MigrateFromLegacyConfig_InvalidMarkerJSONContinues(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.NotificationConfig{}, &models.Setting{}))
	require.NoError(t, db.Create(&models.Setting{Key: "feature.notifications.security_provider_events.enabled", Value: "true", Type: "bool"}).Error)
	require.NoError(t, db.Create(&models.NotificationConfig{ID: "legacy", NotifyWAFBlocks: true, WebhookURL: "https://example.com/webhook"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "notifications.security_provider_events.migration.v1", Value: "{invalid-json", Type: "json", Category: "notifications"}).Error)

	service := NewEnhancedSecurityNotificationService(db)
	require.NoError(t, service.MigrateFromLegacyConfig())

	var count int64
	require.NoError(t, db.Model(&models.NotificationProvider{}).Where("managed_legacy_security = ?", true).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestEnhancedService_MigrateFromLegacyConfig_TransactionWriteErrors(t *testing.T) {
	t.Run("create_managed_provider_error", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "enhanced-migrate-create-error.db")
		rwDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, rwDB.AutoMigrate(&models.NotificationProvider{}, &models.NotificationConfig{}, &models.Setting{}))
		require.NoError(t, rwDB.Create(&models.Setting{Key: "feature.notifications.security_provider_events.enabled", Value: "true", Type: "bool"}).Error)
		require.NoError(t, rwDB.Create(&models.NotificationConfig{ID: "legacy", NotifyWAFBlocks: true, WebhookURL: "https://example.com/webhook"}).Error)

		rwSQL, err := rwDB.DB()
		require.NoError(t, err)
		require.NoError(t, rwSQL.Close())

		roDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=ro", dbPath)), &gorm.Config{})
		require.NoError(t, err)
		service := NewEnhancedSecurityNotificationService(roDB)

		err = service.MigrateFromLegacyConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "create managed provider")
	})

	t.Run("update_managed_provider_error", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "enhanced-migrate-update-error.db")
		rwDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, rwDB.AutoMigrate(&models.NotificationProvider{}, &models.NotificationConfig{}, &models.Setting{}))
		require.NoError(t, rwDB.Create(&models.Setting{Key: "feature.notifications.security_provider_events.enabled", Value: "true", Type: "bool"}).Error)
		require.NoError(t, rwDB.Create(&models.NotificationConfig{ID: "legacy", NotifyWAFBlocks: true, WebhookURL: "https://example.com/webhook"}).Error)
		require.NoError(t, rwDB.Create(&models.NotificationProvider{ID: "managed", Type: "webhook", URL: "https://old.example.com", Enabled: true, ManagedLegacySecurity: true}).Error)

		rwSQL, err := rwDB.DB()
		require.NoError(t, err)
		require.NoError(t, rwSQL.Close())

		roDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=ro", dbPath)), &gorm.Config{})
		require.NoError(t, err)
		service := NewEnhancedSecurityNotificationService(roDB)

		err = service.MigrateFromLegacyConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "update managed provider")
	})
}

func TestEnhancedService_IsFeatureEnabled_CreateAndRequeryPath(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "feature-flag-requery.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	raceDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)

	injected := false
	callbackName := "test_inject_feature_flag_before_create"
	_ = db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Table != "settings" || injected {
			return
		}
		injected = true
		_ = raceDB.Exec("INSERT OR IGNORE INTO settings (key, value, type, category, updated_at) VALUES (?, ?, ?, ?, ?)",
			"feature.notifications.security_provider_events.enabled",
			"true",
			"bool",
			"feature",
			time.Now(),
		).Error
	})
	defer func() {
		_ = db.Callback().Create().Remove(callbackName)
	}()

	service := NewEnhancedSecurityNotificationService(db)
	enabled, err := service.isFeatureEnabled()
	require.NoError(t, err)
	assert.True(t, enabled)

	raceSQL, sqlErr := raceDB.DB()
	if sqlErr == nil {
		_ = raceSQL.Close()
	}
}

func TestEnhancedService_SendViaProviders_QueryProvidersErrorAndCrowdSecRouting(t *testing.T) {
	t.Run("query_providers_error", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))
		service := NewEnhancedSecurityNotificationService(db)

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		err = service.SendViaProviders(context.Background(), models.SecurityEvent{EventType: "waf_block"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "query providers")
	})

	t.Run("crowdsec_decision_routes_to_subscribed_provider", func(t *testing.T) {
		serverCalls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serverCalls++
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Setting{}))
		require.NoError(t, db.Create(&models.NotificationProvider{
			ID:                              "discord-crowdsec-route",
			Type:                            "discord",
			URL:                             server.URL,
			Enabled:                         true,
			NotifySecurityCrowdSecDecisions: true,
		}).Error)

		service := NewEnhancedSecurityNotificationService(db)
		err = service.SendViaProviders(context.Background(), models.SecurityEvent{
			EventType: "crowdsec_decision",
			Severity:  "warn",
			Message:   "CrowdSec decision",
			Timestamp: time.Now(),
		})
		require.NoError(t, err)
		assert.Equal(t, 1, serverCalls)
	})
}

func TestEnhancedService_SendWebhook_MarshalAndExecuteErrorPaths(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	service := NewEnhancedSecurityNotificationService(db)

	t.Run("marshal_error", func(t *testing.T) {
		err := service.sendWebhook(context.Background(), "http://127.0.0.1:8080/webhook", models.SecurityEvent{
			EventType: "waf_block",
			Metadata: map[string]any{
				"bad": make(chan int),
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "marshal event")
	})

	t.Run("execute_request_error", func(t *testing.T) {
		err := service.sendWebhook(context.Background(), "http://127.0.0.1:1/webhook", models.SecurityEvent{
			EventType: "waf_block",
			Severity:  "warn",
			Message:   "connect failure expected",
			Timestamp: time.Now(),
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "execute request")
	})
}

func TestEnhancedService_UpdateManagedProviders_WrapsManagedQueryError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationConfig{}, &models.Setting{}))
	// notification_providers table intentionally absent

	service := NewEnhancedSecurityNotificationService(db)
	err = service.updateManagedProviders(&models.NotificationConfig{DiscordWebhookURL: "https://discord.com/api/webhooks/123/abc"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query managed providers")
}

func TestEnhancedService_MigrateFromLegacyConfig_WrapsManagedProviderQueryError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationConfig{}, &models.Setting{}))
	require.NoError(t, db.Create(&models.Setting{Key: "feature.notifications.security_provider_events.enabled", Value: "true", Type: "bool"}).Error)
	require.NoError(t, db.Create(&models.NotificationConfig{ID: "legacy", NotifyWAFBlocks: true, WebhookURL: "https://example.com/webhook"}).Error)
	// notification_providers table intentionally absent

	service := NewEnhancedSecurityNotificationService(db)
	err = service.MigrateFromLegacyConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query managed provider")
}

func TestEnhancedService_IsFeatureEnabled_CreateAndRequeryErrorPath(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "feature-flag-requery-error.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	readonlyDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=ro", dbPath)), &gorm.Config{})
	require.NoError(t, err)
	readonlyService := NewEnhancedSecurityNotificationService(readonlyDB)

	_, err = readonlyService.isFeatureEnabled()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create and requery feature flag")

	sqlDB, sqlErr := db.DB()
	if sqlErr == nil {
		_ = sqlDB.Close()
	}
}

func TestEnhancedService_SendViaProviders_RateLimitRoutingBranch(t *testing.T) {
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Setting{}))
	require.NoError(t, db.Create(&models.NotificationProvider{
		ID:                          "discord-rate-limit-route",
		Type:                        "discord",
		URL:                         server.URL,
		Enabled:                     true,
		NotifySecurityRateLimitHits: true,
	}).Error)

	service := NewEnhancedSecurityNotificationService(db)
	err = service.SendViaProviders(context.Background(), models.SecurityEvent{
		EventType: "rate limit hit",
		Severity:  "warn",
		Message:   "Rate limit triggered",
		Timestamp: time.Now(),
	})
	require.NoError(t, err)
	assert.Equal(t, 1, serverCalls)
}
