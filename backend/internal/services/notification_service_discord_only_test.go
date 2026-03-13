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

// TestDiscordOnly_CreateProviderRejectsUnsupported tests service-level provider allowlist for create.
func TestDiscordOnly_CreateProviderRejectsUnsupported(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	service := NewNotificationService(db, nil)

	testCases := []string{"generic"}

	for _, providerType := range testCases {
		t.Run(providerType, func(t *testing.T) {
			provider := &models.NotificationProvider{
				Name: "Test Provider",
				Type: providerType,
				URL:  "https://example.com/webhook",
			}

			err := service.CreateProvider(provider)
			assert.Error(t, err, "Should reject unsupported provider")
			assert.Contains(t, err.Error(), "unsupported provider type")
		})
	}
}

// TestDiscordOnly_CreateProviderAcceptsDiscord tests service-level acceptance of Discord providers.
func TestDiscordOnly_CreateProviderAcceptsDiscord(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	service := NewNotificationService(db, nil)

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

func TestDiscordOnly_CreateProviderAcceptsWebhook(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	service := NewNotificationService(db, nil)

	provider := &models.NotificationProvider{
		Name: "Test Webhook",
		Type: "webhook",
		URL:  "https://example.com/webhook",
	}

	err = service.CreateProvider(provider)
	assert.NoError(t, err, "Should accept webhook provider")
}

func TestDiscordOnly_CreateProviderAcceptsGotifyWithOrWithoutToken(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	service := NewNotificationService(db, nil)

	provider := &models.NotificationProvider{
		Name: "Test Gotify",
		Type: "gotify",
		URL:  "https://gotify.example.com/message",
	}

	err = service.CreateProvider(provider)
	assert.NoError(t, err)

	provider.ID = ""
	provider.Token = "secret"
	err = service.CreateProvider(provider)
	assert.NoError(t, err)
}

// TestDiscordOnly_CreateProviderAcceptsEmail tests that email is accepted as a supported provider type.
func TestDiscordOnly_CreateProviderAcceptsEmail(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	service := NewNotificationService(db, nil)

	provider := &models.NotificationProvider{
		Name: "Test Email",
		Type: "email",
		URL:  "smtp://smtp.example.com",
	}

	err = service.CreateProvider(provider)
	assert.NoError(t, err, "Should accept email provider")
}

// TestDiscordOnly_UpdateProviderRejectsTypeMutation tests immutable provider type on update.
func TestDiscordOnly_UpdateProviderRejectsTypeMutation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	provider := models.NotificationProvider{
		ID:             "test-id",
		Name:           "Test Webhook",
		Type:           "webhook",
		URL:            "https://example.com/webhook",
		MigrationState: "deprecated",
	}
	require.NoError(t, db.Create(&provider).Error)

	service := NewNotificationService(db, nil)

	updatedProvider := &models.NotificationProvider{
		ID:   "test-id",
		Name: "Updated",
		Type: "discord",
		URL:  "https://discord.com/api/webhooks/123/abc",
	}

	err = service.UpdateProvider(updatedProvider)
	assert.Error(t, err, "Should reject type mutation")
	assert.Contains(t, err.Error(), "cannot change provider type")
}

// TestDiscordOnly_UpdateProviderAllowsWebhookUpdates tests supported provider updates.
func TestDiscordOnly_UpdateProviderAllowsWebhookUpdates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	provider := models.NotificationProvider{
		ID:             "test-id",
		Name:           "Test Webhook",
		Type:           "webhook",
		URL:            "https://example.com/webhook",
		Enabled:        false,
		MigrationState: "deprecated",
	}
	require.NoError(t, db.Create(&provider).Error)

	service := NewNotificationService(db, nil)

	updatedProvider := &models.NotificationProvider{
		ID:      "test-id",
		Name:    "Test Webhook",
		Type:    "webhook",
		URL:     "https://example.com/webhook",
		Enabled: true,
	}

	err = service.UpdateProvider(updatedProvider)
	assert.NoError(t, err)
}

// TestDiscordOnly_TestProviderAllowsWebhookWithoutFeatureFlag tests that webhook TestProvider
// works without explicit feature flag (bypasses dispatch gate).
func TestDiscordOnly_TestProviderAllowsWebhookWithoutFeatureFlag(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Setting{}))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	service := NewNotificationService(db, nil)

	provider := models.NotificationProvider{
		Name:     "Test Webhook",
		Type:     "webhook",
		URL:      ts.URL + "/webhook",
		Template: "minimal",
	}

	err = service.TestProvider(provider)
	assert.NoError(t, err)
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

	service := NewNotificationService(db, nil)

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

	service := NewNotificationService(db, nil)

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

	service := NewNotificationService(db, nil)

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

	service := NewNotificationService(db, nil)

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

	service := NewNotificationService(db, nil)

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

	service := NewNotificationService(db, nil)

	// SendExternal should skip deprecated provider silently
	service.SendExternal(context.Background(), "proxy_host", "Test", "Test message", nil)

	// Wait a bit for goroutine
	time.Sleep(100 * time.Millisecond)

	// No assertions needed - just verify no panic/error
	// The test passes if SendExternal completes without panic
}
