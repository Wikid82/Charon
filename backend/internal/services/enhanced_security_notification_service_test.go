package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupEnhancedServiceDB(t *testing.T) *gorm.DB {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.NotificationConfig{}, &models.Setting{}))
	return db
}

func TestNewEnhancedSecurityNotificationService(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)
	assert.NotNil(t, service)
	assert.Equal(t, db, service.db)
}

func TestGetSettings_FeatureFlagDisabled_ReturnsLegacyConfig(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// Set feature flag to false
	require.NoError(t, db.Create(&models.Setting{
		Key:   "feature.notifications.security_provider_events.enabled",
		Value: "false",
		Type:  "bool",
	}).Error)

	// Create legacy config
	legacyConfig := &models.NotificationConfig{
		NotifyWAFBlocks:     true,
		NotifyACLDenies:     false,
		NotifyRateLimitHits: true,
		WebhookURL:          "https://example.com/webhook",
	}
	require.NoError(t, db.Create(legacyConfig).Error)

	// Test
	config, err := service.GetSettings()
	require.NoError(t, err)
	assert.True(t, config.NotifyWAFBlocks)
	assert.False(t, config.NotifyACLDenies)
	assert.True(t, config.NotifyRateLimitHits)
	assert.Equal(t, "https://example.com/webhook", config.WebhookURL)
}

func TestGetSettings_FeatureFlagEnabled_ReturnsAggregatedConfig(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// Set feature flag to true
	require.NoError(t, db.Create(&models.Setting{
		Key:   "feature.notifications.security_provider_events.enabled",
		Value: "true",
		Type:  "bool",
	}).Error)

	// Create providers with different event types enabled
	providers := []models.NotificationProvider{
		{
			ID:                          "p1",
			Type:                        "discord",
			Enabled:                     true,
			NotifySecurityWAFBlocks:     true,
			NotifySecurityACLDenies:     false,
			NotifySecurityRateLimitHits: false,
			URL:                         "https://discord.com/webhook/1",
		},
		{
			ID:                          "p2",
			Type:                        "discord",
			Enabled:                     true,
			NotifySecurityWAFBlocks:     false,
			NotifySecurityACLDenies:     true,
			NotifySecurityRateLimitHits: true,
			URL:                         "https://discord.com/webhook/2",
		},
	}

	for _, p := range providers {
		require.NoError(t, db.Create(&p).Error)
	}

	// Test
	config, err := service.GetSettings()
	require.NoError(t, err)
	// OR aggregation: at least one provider has each flag true
	assert.True(t, config.NotifyWAFBlocks)
	assert.True(t, config.NotifyACLDenies)
	assert.True(t, config.NotifyRateLimitHits)
}

func TestGetProviderAggregatedConfig_ORSemantics(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// Create providers where different providers have different flags
	providers := []models.NotificationProvider{
		{ID: "p1", Type: "discord", Enabled: true, NotifySecurityWAFBlocks: true, NotifySecurityACLDenies: false, NotifySecurityRateLimitHits: false},
		{ID: "p2", Type: "discord", Enabled: true, NotifySecurityWAFBlocks: false, NotifySecurityACLDenies: true, NotifySecurityRateLimitHits: false},
		{ID: "p3", Type: "discord", Enabled: true, NotifySecurityWAFBlocks: false, NotifySecurityACLDenies: false, NotifySecurityRateLimitHits: true},
	}

	for _, p := range providers {
		require.NoError(t, db.Create(&p).Error)
	}

	// Test
	config, err := service.getProviderAggregatedConfig()
	require.NoError(t, err)
	assert.True(t, config.NotifyWAFBlocks, "OR: p1 has WAF=true")
	assert.True(t, config.NotifyACLDenies, "OR: p2 has ACL=true")
	assert.True(t, config.NotifyRateLimitHits, "OR: p3 has RateLimit=true")
}

func TestGetProviderAggregatedConfig_FiltersSupportedTypes(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// Create providers with both supported and unsupported types
	providers := []models.NotificationProvider{
		{ID: "discord", Type: "discord", Enabled: true, NotifySecurityWAFBlocks: true},
		{ID: "webhook", Type: "webhook", Enabled: true, NotifySecurityWAFBlocks: true},
		{ID: "slack", Type: "slack", Enabled: true, NotifySecurityACLDenies: true},
		{ID: "gotify", Type: "gotify", Enabled: true, NotifySecurityRateLimitHits: true},
		{ID: "telegram", Type: "telegram", Enabled: true, NotifySecurityWAFBlocks: true}, // Telegram is now supported
	}

	for _, p := range providers {
		require.NoError(t, db.Create(&p).Error)
	}

	// Test
	config, err := service.getProviderAggregatedConfig()
	require.NoError(t, err)
	// All provider types including telegram contribute to aggregation
	assert.True(t, config.NotifyWAFBlocks, "Discord, webhook, and telegram have WAF=true")
	assert.True(t, config.NotifyACLDenies, "Slack has ACL=true")
	assert.True(t, config.NotifyRateLimitHits, "Gotify has RateLimit=true")
}

func TestGetProviderAggregatedConfig_DestinationReporting_SingleManaged(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	testCases := []struct {
		name          string
		providerType  string
		url           string
		expectedField string
	}{
		{
			name:          "webhook",
			providerType:  "webhook",
			url:           "https://example.com/webhook",
			expectedField: "WebhookURL",
		},
		{
			name:          "discord",
			providerType:  "discord",
			url:           "https://discord.com/webhook/123",
			expectedField: "DiscordWebhookURL",
		},
		{
			name:          "slack",
			providerType:  "slack",
			url:           "https://hooks.slack.com/services/T/B/X",
			expectedField: "SlackWebhookURL",
		},
		{
			name:          "gotify",
			providerType:  "gotify",
			url:           "https://gotify.example.com",
			expectedField: "GotifyURL",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up
			db.Exec("DELETE FROM notification_providers")

			// Create single managed provider
			provider := models.NotificationProvider{
				ID:                    "managed",
				Type:                  tc.providerType,
				URL:                   tc.url,
				Enabled:               true,
				ManagedLegacySecurity: true,
			}
			require.NoError(t, db.Create(&provider).Error)

			// Test
			config, err := service.getProviderAggregatedConfig()
			require.NoError(t, err)
			assert.False(t, config.DestinationAmbiguous, "Single managed provider = not ambiguous")

			// Verify correct field is populated
			switch tc.expectedField {
			case "WebhookURL":
				assert.Equal(t, tc.url, config.WebhookURL)
			case "DiscordWebhookURL":
				assert.Equal(t, tc.url, config.DiscordWebhookURL)
			case "SlackWebhookURL":
				assert.Equal(t, tc.url, config.SlackWebhookURL)
			case "GotifyURL":
				assert.Equal(t, tc.url, config.GotifyURL)
			}
		})
	}
}

func TestGetProviderAggregatedConfig_DestinationReporting_MultipleManaged_Ambiguous(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// Create multiple managed providers
	providers := []models.NotificationProvider{
		{ID: "m1", Type: "discord", URL: "https://discord.com/webhook/1", Enabled: true, ManagedLegacySecurity: true},
		{ID: "m2", Type: "discord", URL: "https://discord.com/webhook/2", Enabled: true, ManagedLegacySecurity: true},
	}

	for _, p := range providers {
		require.NoError(t, db.Create(&p).Error)
	}

	// Test
	config, err := service.getProviderAggregatedConfig()
	require.NoError(t, err)
	assert.True(t, config.DestinationAmbiguous, "Multiple managed providers = ambiguous")
	assert.Empty(t, config.WebhookURL)
	assert.Empty(t, config.DiscordWebhookURL)
	assert.Empty(t, config.SlackWebhookURL)
	assert.Empty(t, config.GotifyURL)
}

func TestGetProviderAggregatedConfig_DestinationReporting_ZeroManaged_Ambiguous(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// Create provider without managed flag
	provider := models.NotificationProvider{
		ID:                    "unmanaged",
		Type:                  "discord",
		URL:                   "https://discord.com/webhook/1",
		Enabled:               true,
		ManagedLegacySecurity: false,
	}
	require.NoError(t, db.Create(&provider).Error)

	// Test
	config, err := service.getProviderAggregatedConfig()
	require.NoError(t, err)
	assert.True(t, config.DestinationAmbiguous, "Zero managed providers = ambiguous")
}

func TestGetSettings_LegacyConfig_NotFound_ReturnsDefaults(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// Set feature flag to false to use legacy path
	require.NoError(t, db.Create(&models.Setting{
		Key:   "feature.notifications.security_provider_events.enabled",
		Value: "false",
		Type:  "bool",
	}).Error)

	// Don't create legacy config

	// Test
	config, err := service.GetSettings()
	require.NoError(t, err)
	assert.True(t, config.NotifyWAFBlocks, "Default WAF=true")
	assert.True(t, config.NotifyACLDenies, "Default ACL=true")
	assert.True(t, config.NotifyRateLimitHits, "Default RateLimit=true")
}

func TestUpdateSettings_FeatureFlagDisabled_UpdatesLegacyConfig(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// Set feature flag to false
	require.NoError(t, db.Create(&models.Setting{
		Key:   "feature.notifications.security_provider_events.enabled",
		Value: "false",
		Type:  "bool",
	}).Error)

	// Update
	req := &models.NotificationConfig{
		NotifyWAFBlocks:     false,
		NotifyACLDenies:     true,
		NotifyRateLimitHits: false,
		WebhookURL:          "https://updated.com/webhook",
	}
	err := service.UpdateSettings(req)
	require.NoError(t, err)

	// Verify
	var saved models.NotificationConfig
	require.NoError(t, db.First(&saved).Error)
	assert.False(t, saved.NotifyWAFBlocks)
	assert.True(t, saved.NotifyACLDenies)
	assert.False(t, saved.NotifyRateLimitHits)
	assert.Equal(t, "https://updated.com/webhook", saved.WebhookURL)
}

func TestUpdateSettings_FeatureFlagEnabled_UpdatesManagedProviders(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// Set feature flag to true
	require.NoError(t, db.Create(&models.Setting{
		Key:   "feature.notifications.security_provider_events.enabled",
		Value: "true",
		Type:  "bool",
	}).Error)

	// Create managed provider
	provider := models.NotificationProvider{
		ID:                          "managed",
		Type:                        "discord",
		URL:                         "https://discord.com/webhook/old",
		Enabled:                     true,
		ManagedLegacySecurity:       true,
		NotifySecurityWAFBlocks:     true,
		NotifySecurityACLDenies:     true,
		NotifySecurityRateLimitHits: true,
	}
	require.NoError(t, db.Create(&provider).Error)

	// Update
	req := &models.NotificationConfig{
		NotifyWAFBlocks:     false,
		NotifyACLDenies:     true,
		NotifyRateLimitHits: false,
		DiscordWebhookURL:   "https://discord.com/webhook/new",
	}
	err := service.UpdateSettings(req)
	require.NoError(t, err)

	// Verify
	var updated models.NotificationProvider
	require.NoError(t, db.First(&updated, "id = ?", "managed").Error)
	assert.False(t, updated.NotifySecurityWAFBlocks)
	assert.True(t, updated.NotifySecurityACLDenies)
	assert.False(t, updated.NotifySecurityRateLimitHits)
	assert.Equal(t, "https://discord.com/webhook/new", updated.URL)
	assert.Equal(t, "discord", updated.Type)
}

func TestUpdateManagedProviders_CreatesProviderIfNoneExist(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// No existing providers

	// Update
	req := &models.NotificationConfig{
		NotifyWAFBlocks:   true,
		NotifyACLDenies:   false,
		DiscordWebhookURL: "https://discord.com/webhook/1",
	}
	err := service.updateManagedProviders(req)
	require.NoError(t, err)

	// Verify
	var providers []models.NotificationProvider
	require.NoError(t, db.Find(&providers).Error)
	require.Len(t, providers, 1)
	assert.Equal(t, "discord", providers[0].Type)
	assert.Equal(t, "https://discord.com/webhook/1", providers[0].URL)
	assert.True(t, providers[0].ManagedLegacySecurity)
	assert.True(t, providers[0].NotifySecurityWAFBlocks)
	assert.False(t, providers[0].NotifySecurityACLDenies)
}

func TestUpdateManagedProviders_RejectsMultipleDestinations(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// Try to set multiple destinations
	req := &models.NotificationConfig{
		WebhookURL:        "https://example.com/webhook",
		DiscordWebhookURL: "https://discord.com/webhook/1",
	}

	err := service.updateManagedProviders(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous destination")
}

func TestUpdateManagedProviders_GotifyValidation_RequiresBothURLAndToken(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	testCases := []struct {
		name        string
		gotifyURL   string
		gotifyToken string
		expectError bool
	}{
		{
			name:        "both_present",
			gotifyURL:   "https://gotify.example.com",
			gotifyToken: "token123",
			expectError: false,
		},
		{
			name:        "only_url",
			gotifyURL:   "https://gotify.example.com",
			gotifyToken: "",
			expectError: true,
		},
		{
			name:        "only_token",
			gotifyURL:   "",
			gotifyToken: "token123",
			expectError: true,
		},
		{
			name:        "both_empty",
			gotifyURL:   "",
			gotifyToken: "",
			expectError: false, // No gotify config = valid
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &models.NotificationConfig{
				GotifyURL:   tc.gotifyURL,
				GotifyToken: tc.gotifyToken,
			}

			err := service.updateManagedProviders(req)
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "incomplete gotify configuration")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateManagedProviders_Idempotency_NoUpdateIfNoChange(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// Create managed provider
	initialTime := time.Now().Add(-1 * time.Hour)
	provider := models.NotificationProvider{
		ID:                          "managed",
		Type:                        "discord",
		URL:                         "https://discord.com/webhook/1",
		Enabled:                     true,
		ManagedLegacySecurity:       true,
		NotifySecurityWAFBlocks:     true,
		NotifySecurityACLDenies:     false,
		NotifySecurityRateLimitHits: true,
	}
	provider.CreatedAt = initialTime
	provider.UpdatedAt = initialTime
	require.NoError(t, db.Create(&provider).Error)

	// Update with same values
	req := &models.NotificationConfig{
		NotifyWAFBlocks:     true,
		NotifyACLDenies:     false,
		NotifyRateLimitHits: true,
		DiscordWebhookURL:   "https://discord.com/webhook/1",
	}
	err := service.updateManagedProviders(req)
	require.NoError(t, err)

	// Verify UpdatedAt didn't change
	var updated models.NotificationProvider
	require.NoError(t, db.First(&updated, "id = ?", "managed").Error)
	assert.Equal(t, initialTime.Unix(), updated.UpdatedAt.Unix(), "UpdatedAt should not change if values unchanged")
}

func TestExtractDestinationURL(t *testing.T) {
	service := &EnhancedSecurityNotificationService{}

	testCases := []struct {
		name     string
		req      *models.NotificationConfig
		expected string
	}{
		{
			name:     "webhook",
			req:      &models.NotificationConfig{WebhookURL: "https://example.com/webhook"},
			expected: "https://example.com/webhook",
		},
		{
			name:     "discord",
			req:      &models.NotificationConfig{DiscordWebhookURL: "https://discord.com/webhook/1"},
			expected: "https://discord.com/webhook/1",
		},
		{
			name:     "slack",
			req:      &models.NotificationConfig{SlackWebhookURL: "https://hooks.slack.com/services/T/B/X"},
			expected: "https://hooks.slack.com/services/T/B/X",
		},
		{
			name:     "gotify",
			req:      &models.NotificationConfig{GotifyURL: "https://gotify.example.com"},
			expected: "https://gotify.example.com",
		},
		{
			name:     "empty",
			req:      &models.NotificationConfig{},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := service.extractDestinationURL(tc.req)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExtractDestinationToken(t *testing.T) {
	service := &EnhancedSecurityNotificationService{}

	testCases := []struct {
		name     string
		req      *models.NotificationConfig
		expected string
	}{
		{
			name:     "gotify_token",
			req:      &models.NotificationConfig{GotifyToken: "token123"},
			expected: "token123",
		},
		{
			name:     "empty",
			req:      &models.NotificationConfig{},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := service.extractDestinationToken(tc.req)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMigrateFromLegacyConfig_FeatureFlagDisabled_ReadOnlyMode(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// Set feature flag to false
	require.NoError(t, db.Create(&models.Setting{
		Key:   "feature.notifications.security_provider_events.enabled",
		Value: "false",
		Type:  "bool",
	}).Error)

	// Create legacy config
	legacyConfig := models.NotificationConfig{
		NotifyWAFBlocks: true,
		WebhookURL:      "https://example.com/webhook",
	}
	require.NoError(t, db.Create(&legacyConfig).Error)

	// Migrate
	err := service.MigrateFromLegacyConfig()
	require.NoError(t, err)

	// Verify NO provider was created (read-only mode)
	var providers []models.NotificationProvider
	require.NoError(t, db.Find(&providers).Error)
	assert.Len(t, providers, 0, "Feature flag disabled = no provider mutation")
}

func TestMigrateFromLegacyConfig_NoLegacyConfig_NoOp(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// Enable feature flag
	require.NoError(t, db.Create(&models.Setting{
		Key:   "feature.notifications.security_provider_events.enabled",
		Value: "true",
		Type:  "bool",
	}).Error)

	// No legacy config

	// Migrate
	err := service.MigrateFromLegacyConfig()
	require.NoError(t, err)

	// Verify NO provider was created
	var providers []models.NotificationProvider
	require.NoError(t, db.Find(&providers).Error)
	assert.Len(t, providers, 0)
}

func TestMigrateFromLegacyConfig_CreatesManagedProvider(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// Enable feature flag
	require.NoError(t, db.Create(&models.Setting{
		Key:   "feature.notifications.security_provider_events.enabled",
		Value: "true",
		Type:  "bool",
	}).Error)

	// Create legacy config
	legacyConfig := models.NotificationConfig{
		NotifyWAFBlocks:     true,
		NotifyACLDenies:     false,
		NotifyRateLimitHits: true,
		WebhookURL:          "https://example.com/webhook",
	}
	require.NoError(t, db.Create(&legacyConfig).Error)

	// Migrate
	err := service.MigrateFromLegacyConfig()
	require.NoError(t, err)

	// Verify provider created
	var providers []models.NotificationProvider
	require.NoError(t, db.Find(&providers).Error)
	require.Len(t, providers, 1)
	assert.True(t, providers[0].ManagedLegacySecurity)
	assert.Equal(t, "webhook", providers[0].Type)
	assert.Equal(t, "https://example.com/webhook", providers[0].URL)
	assert.True(t, providers[0].NotifySecurityWAFBlocks)
	assert.False(t, providers[0].NotifySecurityACLDenies)
	assert.True(t, providers[0].NotifySecurityRateLimitHits)
}

func TestMigrateFromLegacyConfig_Idempotent_SameChecksum(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// Enable feature flag
	require.NoError(t, db.Create(&models.Setting{
		Key:   "feature.notifications.security_provider_events.enabled",
		Value: "true",
		Type:  "bool",
	}).Error)

	// Create legacy config
	legacyConfig := models.NotificationConfig{
		NotifyWAFBlocks: true,
		WebhookURL:      "https://example.com/webhook",
	}
	require.NoError(t, db.Create(&legacyConfig).Error)

	// First migration
	err := service.MigrateFromLegacyConfig()
	require.NoError(t, err)

	// Get provider count after first migration
	var providersAfterFirst []models.NotificationProvider
	require.NoError(t, db.Find(&providersAfterFirst).Error)
	firstCount := len(providersAfterFirst)

	// Second migration (should be no-op due to checksum match)
	err = service.MigrateFromLegacyConfig()
	require.NoError(t, err)

	// Verify no duplicate provider created
	var providersAfterSecond []models.NotificationProvider
	require.NoError(t, db.Find(&providersAfterSecond).Error)
	assert.Equal(t, firstCount, len(providersAfterSecond), "Idempotent migration should not create duplicates")
}

func TestComputeConfigChecksum_Deterministic(t *testing.T) {
	config1 := models.NotificationConfig{
		NotifyWAFBlocks:     true,
		NotifyACLDenies:     false,
		NotifyRateLimitHits: true,
		WebhookURL:          "https://example.com/webhook",
	}

	config2 := models.NotificationConfig{
		NotifyWAFBlocks:     true,
		NotifyACLDenies:     false,
		NotifyRateLimitHits: true,
		WebhookURL:          "https://example.com/webhook",
	}

	checksum1 := computeConfigChecksum(config1)
	checksum2 := computeConfigChecksum(config2)

	assert.Equal(t, checksum1, checksum2, "Same config should produce same checksum")
	assert.NotEmpty(t, checksum1)
}

func TestComputeConfigChecksum_DifferentForDifferentConfigs(t *testing.T) {
	config1 := models.NotificationConfig{
		NotifyWAFBlocks: true,
	}

	config2 := models.NotificationConfig{
		NotifyWAFBlocks: false,
	}

	checksum1 := computeConfigChecksum(config1)
	checksum2 := computeConfigChecksum(config2)

	assert.NotEqual(t, checksum1, checksum2, "Different configs should produce different checksums")
}

func TestIsFeatureEnabled_NotFound_CreatesDefault(t *testing.T) {
	// Save and restore env vars
	origCharonEnv := os.Getenv("CHARON_ENV")
	origGinMode := os.Getenv("GIN_MODE")
	defer func() {
		_ = os.Setenv("CHARON_ENV", origCharonEnv)
		_ = os.Setenv("GIN_MODE", origGinMode)
	}()

	testCases := []struct {
		name        string
		charonEnv   string
		ginMode     string
		expected    bool
		description string
	}{
		{
			name:        "production_explicit",
			charonEnv:   "production",
			ginMode:     "",
			expected:    false,
			description: "CHARON_ENV=production should default to false",
		},
		{
			name:        "prod_explicit",
			charonEnv:   "prod",
			ginMode:     "",
			expected:    false,
			description: "CHARON_ENV=prod should default to false",
		},
		{
			name:        "gin_debug",
			charonEnv:   "",
			ginMode:     "debug",
			expected:    true,
			description: "GIN_MODE=debug should default to true",
		},
		{
			name:        "gin_test",
			charonEnv:   "",
			ginMode:     "test",
			expected:    true,
			description: "GIN_MODE=test should default to true",
		},
		{
			name:        "both_unset",
			charonEnv:   "",
			ginMode:     "",
			expected:    false,
			description: "Both unset should default to false (production)",
		},
		{
			name:        "development",
			charonEnv:   "development",
			ginMode:     "",
			expected:    true,
			description: "CHARON_ENV=development should default to true",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupEnhancedServiceDB(t)
			service := NewEnhancedSecurityNotificationService(db)

			// Set environment
			_ = os.Setenv("CHARON_ENV", tc.charonEnv)
			_ = os.Setenv("GIN_MODE", tc.ginMode)

			// Test
			enabled, err := service.isFeatureEnabled()
			require.NoError(t, err)
			assert.Equal(t, tc.expected, enabled, tc.description)

			// Verify setting was created
			var setting models.Setting
			require.NoError(t, db.Where("key = ?", "feature.notifications.security_provider_events.enabled").First(&setting).Error)
			expectedValue := "false"
			if tc.expected {
				expectedValue = "true"
			}
			assert.Equal(t, expectedValue, setting.Value)
		})
	}
}

func TestSendWebhook_SSRFValidation(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	testCases := []struct {
		name        string
		webhookURL  string
		shouldFail  bool
		description string
	}{
		{
			name:        "valid_https",
			webhookURL:  "https://example.com/webhook",
			shouldFail:  false,
			description: "HTTPS should be allowed",
		},
		{
			name:        "valid_http",
			webhookURL:  "http://example.com/webhook",
			shouldFail:  false,
			description: "HTTP should be allowed for backwards compatibility",
		},
		{
			name:        "empty_url",
			webhookURL:  "",
			shouldFail:  true,
			description: "Empty URL should fail validation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			event := models.SecurityEvent{
				EventType: "waf_block",
				Severity:  "high",
				Message:   "Test event",
			}

			err := service.sendWebhook(context.Background(), tc.webhookURL, event)
			if tc.shouldFail {
				assert.Error(t, err, tc.description)
			} else if err != nil {
				// May fail with network error but should pass SSRF validation
				// We're testing the validation step, not the actual HTTP call
				assert.NotContains(t, err.Error(), "ssrf validation failed", tc.description)
			}
		})
	}
}

func TestSendWebhook_Success(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// Create test server
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Charon-Cerberus/1.0", r.Header.Get("User-Agent"))

		// Verify payload
		var event models.SecurityEvent
		err := json.NewDecoder(r.Body).Decode(&event)
		assert.NoError(t, err)
		assert.Equal(t, "waf_block", event.EventType)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "high",
		Message:   "Test event",
	}

	err := service.sendWebhook(context.Background(), server.URL, event)
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestNormalizeSecurityEventType(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"WAF Block", "waf_block"},
		{"waf_block", "waf_block"},
		{"ACL Deny", "acl_deny"},
		{"acl_deny", "acl_deny"},
		{"Rate Limit", "rate_limit"},
		{"rate_limit", "rate_limit"},
		{"CrowdSec Decision", "crowdsec_decision"},
		{"crowdsec_decision", "crowdsec_decision"},
		{"unknown_event", "unknown_event"},
		{" WAF ", "waf_block"},
		{" ACL ", "acl_deny"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := normalizeSecurityEventType(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetDefaultFeatureFlagValue(t *testing.T) {
	// Save and restore env vars
	origCharonEnv := os.Getenv("CHARON_ENV")
	origGinMode := os.Getenv("GIN_MODE")
	defer func() {
		_ = os.Setenv("CHARON_ENV", origCharonEnv)
		_ = os.Setenv("GIN_MODE", origGinMode)
	}()

	testCases := []struct {
		name      string
		charonEnv string
		ginMode   string
		expected  string
	}{
		{"production", "production", "", "false"},
		{"prod", "prod", "", "false"},
		{"debug", "", "debug", "true"},
		{"test", "", "test", "true"},
		{"both_unset", "", "", "false"},
		{"development", "development", "", "true"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupEnhancedServiceDB(t)
			service := NewEnhancedSecurityNotificationService(db)

			_ = os.Setenv("CHARON_ENV", tc.charonEnv)
			_ = os.Setenv("GIN_MODE", tc.ginMode)

			result := service.getDefaultFeatureFlagValue()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetDefaultFeatureFlagValue_TestMode(t *testing.T) {
	db := setupEnhancedServiceDB(t)
	service := NewEnhancedSecurityNotificationService(db)

	// Create test mode marker
	require.NoError(t, db.Create(&models.Setting{
		Key:   "_test_mode_marker",
		Value: "true",
		Type:  "bool",
	}).Error)

	result := service.getDefaultFeatureFlagValue()
	assert.Equal(t, "true", result, "Test mode should return true")
}
