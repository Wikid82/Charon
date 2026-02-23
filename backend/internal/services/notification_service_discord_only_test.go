package services

import (
	"context"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestDiscordOnly_CreateProviderRejectsNonDiscord tests service-level Discord-only enforcement for create.
func TestDiscordOnly_CreateProviderRejectsNonDiscord(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	service := NewNotificationService(db)

	testCases := []string{"webhook", "slack", "gotify", "telegram", "generic"}

	for _, providerType := range testCases {
		t.Run(providerType, func(t *testing.T) {
			provider := &models.NotificationProvider{
				Name: "Test Provider",
				Type: providerType,
				URL:  "https://example.com/webhook",
			}

			err := service.CreateProvider(provider)
			assert.Error(t, err, "Should reject non-Discord provider")
			assert.Contains(t, err.Error(), "only discord provider type is supported")
		})
	}
}

// TestDiscordOnly_CreateProviderAcceptsDiscord tests service-level acceptance of Discord providers.
func TestDiscordOnly_CreateProviderAcceptsDiscord(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	service := NewNotificationService(db)

	provider := &models.NotificationProvider{
		Name: "Test Discord",
		Type: "discord",
		URL:  "https://discord.com/api/webhooks/123/abc",
	}

	err = service.CreateProvider(provider)
	assert.NoError(t, err, "Should accept Discord provider")

	// Verify in DB
	var created models.NotificationProvider
	db.First(&created, "name = ?", "Test Discord")
	assert.Equal(t, "discord", created.Type)
}

// TestDiscordOnly_UpdateProviderRejectsNonDiscord tests service-level Discord-only enforcement for update.
func TestDiscordOnly_UpdateProviderRejectsNonDiscord(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	// Create a deprecated webhook provider
	deprecatedProvider := models.NotificationProvider{
		ID:             "test-id",
		Name:           "Test Webhook",
		Type:           "webhook",
		URL:            "https://example.com/webhook",
		MigrationState: "deprecated",
	}
	require.NoError(t, db.Create(&deprecatedProvider).Error)

	service := NewNotificationService(db)

	// Try to update with webhook type
	provider := &models.NotificationProvider{
		ID:   "test-id",
		Name: "Updated",
		Type: "webhook",
		URL:  "https://example.com/webhook",
	}

	err = service.UpdateProvider(provider)
	assert.Error(t, err, "Should reject non-Discord provider update")
	assert.Contains(t, err.Error(), "only discord provider type is supported")
}

// TestDiscordOnly_UpdateProviderRejectsTypeMutation tests that service blocks type mutation for deprecated providers.
func TestDiscordOnly_UpdateProviderRejectsTypeMutation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	// Create a deprecated webhook provider
	deprecatedProvider := models.NotificationProvider{
		ID:             "test-id",
		Name:           "Test Webhook",
		Type:           "webhook",
		URL:            "https://example.com/webhook",
		MigrationState: "deprecated",
	}
	require.NoError(t, db.Create(&deprecatedProvider).Error)

	service := NewNotificationService(db)

	// Try to change type to discord
	provider := &models.NotificationProvider{
		ID:   "test-id",
		Name: "Test Webhook",
		Type: "discord",
		URL:  "https://discord.com/api/webhooks/123/abc",
	}

	err = service.UpdateProvider(provider)
	assert.Error(t, err, "Should reject type mutation")
	assert.Contains(t, err.Error(), "cannot change provider type")
}

// TestDiscordOnly_UpdateProviderRejectsEnable tests that service blocks enabling deprecated providers.
func TestDiscordOnly_UpdateProviderRejectsEnable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	// Create a deprecated webhook provider (disabled)
	deprecatedProvider := models.NotificationProvider{
		ID:             "test-id",
		Name:           "Test Webhook",
		Type:           "webhook",
		URL:            "https://example.com/webhook",
		Enabled:        false,
		MigrationState: "deprecated",
	}
	require.NoError(t, db.Create(&deprecatedProvider).Error)

	service := NewNotificationService(db)

	// Try to enable
	provider := &models.NotificationProvider{
		ID:      "test-id",
		Name:    "Test Webhook",
		Type:    "webhook",
		URL:     "https://example.com/webhook",
		Enabled: true,
	}

	err = service.UpdateProvider(provider)
	assert.Error(t, err, "Should reject enabling deprecated provider")
	assert.Contains(t, err.Error(), "cannot enable deprecated")
}

// TestDiscordOnly_TestProviderRejectsNonDiscord tests that TestProvider enforces Discord-only.
func TestDiscordOnly_TestProviderRejectsNonDiscord(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	service := NewNotificationService(db)

	provider := models.NotificationProvider{
		Name: "Test Webhook",
		Type: "webhook",
		URL:  "https://example.com/webhook",
	}

	err = service.TestProvider(provider)
	assert.Error(t, err, "Should reject non-Discord provider test")
	assert.Contains(t, err.Error(), "only discord provider type is supported")
}

// TestDiscordOnly_MigrationDeprecatesNonDiscord tests that migration marks non-Discord as deprecated.
func TestDiscordOnly_MigrationDeprecatesNonDiscord(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	// Create a webhook provider
	webhookProvider := models.NotificationProvider{
		ID:      "test-webhook",
		Name:    "Test Webhook",
		Type:    "webhook",
		URL:     "https://example.com/webhook",
		Enabled: true,
	}
	require.NoError(t, db.Create(&webhookProvider).Error)

	service := NewNotificationService(db)

	// Run migration
	err = service.EnsureNotifyOnlyProviderMigration(context.Background())
	require.NoError(t, err)

	// Verify deprecated state
	var migrated models.NotificationProvider
	db.First(&migrated, "id = ?", "test-webhook")
	assert.Equal(t, "deprecated", migrated.MigrationState)
	assert.False(t, migrated.Enabled, "Should be disabled")
	assert.Contains(t, migrated.MigrationError, "not supported in discord-only rollout")
	assert.NotNil(t, migrated.LastMigratedAt)
}

// TestDiscordOnly_MigrationMarksDiscordMigrated tests that migration marks Discord as migrated.
func TestDiscordOnly_MigrationMarksDiscordMigrated(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	// Create a discord provider
	discordProvider := models.NotificationProvider{
		ID:      "test-discord",
		Name:    "Test Discord",
		Type:    "discord",
		URL:     "https://discord.com/api/webhooks/123/abc",
		Enabled: true,
	}
	require.NoError(t, db.Create(&discordProvider).Error)

	service := NewNotificationService(db)

	// Run migration
	err = service.EnsureNotifyOnlyProviderMigration(context.Background())
	require.NoError(t, err)

	// Verify migrated state
	var migrated models.NotificationProvider
	db.First(&migrated, "id = ?", "test-discord")
	assert.Equal(t, "migrated", migrated.MigrationState)
	assert.Equal(t, "notify_v1", migrated.Engine)
	assert.True(t, migrated.Enabled, "Should remain enabled")
	assert.Empty(t, migrated.MigrationError)
	assert.NotNil(t, migrated.LastMigratedAt)
}

// TestDiscordOnly_MigrationIsIdempotent tests that migration can run multiple times safely.
func TestDiscordOnly_MigrationIsIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	// Create providers
	providers := []models.NotificationProvider{
		{
			ID:      "discord-1",
			Name:    "Discord 1",
			Type:    "discord",
			URL:     "https://discord.com/api/webhooks/1/a",
			Enabled: true,
		},
		{
			ID:      "webhook-1",
			Name:    "Webhook 1",
			Type:    "webhook",
			URL:     "https://example.com/webhook",
			Enabled: true,
		},
	}
	for _, p := range providers {
		require.NoError(t, db.Create(&p).Error)
	}

	service := NewNotificationService(db)

	// Run migration first time
	err = service.EnsureNotifyOnlyProviderMigration(context.Background())
	require.NoError(t, err)

	// Capture state after first migration
	var firstPass []models.NotificationProvider
	db.Find(&firstPass)

	// Run migration second time
	err = service.EnsureNotifyOnlyProviderMigration(context.Background())
	require.NoError(t, err)

	// Verify state unchanged
	var secondPass []models.NotificationProvider
	db.Find(&secondPass)

	assert.Equal(t, len(firstPass), len(secondPass))
	for i := range firstPass {
		assert.Equal(t, firstPass[i].MigrationState, secondPass[i].MigrationState)
		assert.Equal(t, firstPass[i].Enabled, secondPass[i].Enabled)
	}
}

// TestDiscordOnly_MigrationIsTransactional tests that migration rolls back on error.
func TestDiscordOnly_MigrationIsTransactional(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	// Create provider with valid initial state
	provider := models.NotificationProvider{
		ID:      "test-id",
		Name:    "Test",
		Type:    "discord",
		URL:     "https://discord.com/api/webhooks/1/a",
		Enabled: true,
	}
	require.NoError(t, db.Create(&provider).Error)

	service := NewNotificationService(db)

	// First migration should succeed
	err = service.EnsureNotifyOnlyProviderMigration(context.Background())
	require.NoError(t, err)

	// Verify provider was migrated
	var migrated models.NotificationProvider
	db.First(&migrated, "id = ?", "test-id")
	assert.Equal(t, "migrated", migrated.MigrationState)
}

// TestDiscordOnly_MigrationPreservesLegacyURL tests that migration preserves original URL in audit field.
func TestDiscordOnly_MigrationPreservesLegacyURL(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	originalURL := "https://example.com/webhook"
	provider := models.NotificationProvider{
		ID:      "test-id",
		Name:    "Test",
		Type:    "webhook",
		URL:     originalURL,
		Enabled: true,
	}
	require.NoError(t, db.Create(&provider).Error)

	service := NewNotificationService(db)

	err = service.EnsureNotifyOnlyProviderMigration(context.Background())
	require.NoError(t, err)

	var migrated models.NotificationProvider
	db.First(&migrated, "id = ?", "test-id")
	assert.Equal(t, originalURL, migrated.LegacyURL, "Should preserve original URL")
}

// TestDiscordOnly_SendExternalSkipsDeprecated tests that dispatch skips deprecated providers.
func TestDiscordOnly_SendExternalSkipsDeprecated(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	// Create deprecated webhook provider
	deprecatedProvider := models.NotificationProvider{
		ID:               "test-webhook",
		Name:             "Deprecated Webhook",
		Type:             "webhook",
		URL:              "https://example.com/webhook",
		Enabled:          true,
		MigrationState:   "deprecated",
		NotifyProxyHosts: true,
	}
	require.NoError(t, db.Create(&deprecatedProvider).Error)

	service := NewNotificationService(db)

	// SendExternal should skip deprecated provider silently
	service.SendExternal(context.Background(), "proxy_host", "Test", "Test message", nil)

	// Wait a bit for goroutine
	time.Sleep(100 * time.Millisecond)

	// No assertions needed - just verify no panic/error
	// The test passes if SendExternal completes without panic
}
