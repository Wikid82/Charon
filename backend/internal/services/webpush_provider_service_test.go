package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// setupWebPushServiceTestDB mirrors the shared-cache/singleton-index setup
// webpush_handler_test.go's setupWebPushHandlerTest uses, but scoped to the
// services package so ProvisionWebPush/RegisterWebPushSubscription/etc. get
// direct line coverage here rather than only indirectly via the handlers
// package's own coverage instrumentation (docs/plans/current_spec.md §3.1).
func setupWebPushServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.WebPushSubscription{}, &models.Notification{}, &models.Setting{}))
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_webpush_singleton
		ON notification_providers(type) WHERE type = 'webpush'`).Error)
	return db
}

func TestProvisionWebPush_ValidationErrors(t *testing.T) {
	svc := NewNotificationService(setupWebPushServiceTestDB(t), nil)

	_, err := svc.ProvisionWebPush("", "mailto:ops@example.com")
	assert.ErrorIs(t, err, ErrWebPushInvalidRequest)

	_, err = svc.ProvisionWebPush("  ", "mailto:ops@example.com")
	assert.ErrorIs(t, err, ErrWebPushInvalidRequest)

	_, err = svc.ProvisionWebPush("Browser Push", "not-a-valid-subject")
	assert.ErrorIs(t, err, ErrWebPushInvalidRequest)
}

func TestProvisionWebPush_Success(t *testing.T) {
	svc := NewNotificationService(setupWebPushServiceTestDB(t), nil)

	provider, err := svc.ProvisionWebPush("Browser Push", "mailto:ops@example.com")
	require.NoError(t, err)
	assert.Equal(t, "webpush", provider.Type)
	assert.Equal(t, "Browser Push", provider.Name)
	assert.True(t, provider.Enabled)
	assert.NotEmpty(t, provider.Token, "private VAPID key must be persisted on the provider row")
}

func TestProvisionWebPush_HTTPSSubjectAccepted(t *testing.T) {
	svc := NewNotificationService(setupWebPushServiceTestDB(t), nil)

	provider, err := svc.ProvisionWebPush("Browser Push", "https://example.com/contact")
	require.NoError(t, err)
	assert.Equal(t, "webpush", provider.Type)
}

func TestProvisionWebPush_SecondAttemptReturnsAlreadyProvisioned(t *testing.T) {
	svc := NewNotificationService(setupWebPushServiceTestDB(t), nil)

	_, err := svc.ProvisionWebPush("Browser Push", "mailto:ops@example.com")
	require.NoError(t, err)

	_, err = svc.ProvisionWebPush("Browser Push 2", "mailto:ops2@example.com")
	assert.ErrorIs(t, err, ErrWebPushAlreadyProvisioned)

	var count int64
	require.NoError(t, svc.DB.Model(&models.NotificationProvider{}).Where("type = ?", "webpush").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestGetWebPushVAPIDPublicKey_NotProvisioned(t *testing.T) {
	svc := NewNotificationService(setupWebPushServiceTestDB(t), nil)

	_, err := svc.GetWebPushVAPIDPublicKey()
	assert.ErrorIs(t, err, ErrWebPushNotProvisioned)
}

func TestGetWebPushVAPIDPublicKey_DisabledProviderReturnsNotProvisioned(t *testing.T) {
	db := setupWebPushServiceTestDB(t)
	svc := NewNotificationService(db, nil)

	provider, err := svc.ProvisionWebPush("Browser Push", "mailto:ops@example.com")
	require.NoError(t, err)
	require.NoError(t, db.Model(&models.NotificationProvider{}).Where("id = ?", provider.ID).Update("enabled", false).Error)

	_, err = svc.GetWebPushVAPIDPublicKey()
	assert.ErrorIs(t, err, ErrWebPushNotProvisioned)
}

func TestGetWebPushVAPIDPublicKey_Success(t *testing.T) {
	svc := NewNotificationService(setupWebPushServiceTestDB(t), nil)

	_, err := svc.ProvisionWebPush("Browser Push", "mailto:ops@example.com")
	require.NoError(t, err)

	key, err := svc.GetWebPushVAPIDPublicKey()
	require.NoError(t, err)
	assert.NotEmpty(t, key)
}

func TestGetWebPushVAPIDPublicKey_InvalidServiceConfigJSON(t *testing.T) {
	db := setupWebPushServiceTestDB(t)
	svc := NewNotificationService(db, nil)

	provider, err := svc.ProvisionWebPush("Browser Push", "mailto:ops@example.com")
	require.NoError(t, err)
	require.NoError(t, db.Model(&models.NotificationProvider{}).Where("id = ?", provider.ID).Update("service_config", "not-json").Error)

	_, err = svc.GetWebPushVAPIDPublicKey()
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrWebPushNotProvisioned)
}

func TestRegisterWebPushSubscription_ValidationErrors(t *testing.T) {
	svc := NewNotificationService(setupWebPushServiceTestDB(t), nil)
	_, err := svc.ProvisionWebPush("Browser Push", "mailto:ops@example.com")
	require.NoError(t, err)

	validInput := WebPushSubscribeInput{
		Endpoint: "https://push.example.com/abc",
		P256dh:   "p256dh-key",
		Auth:     "auth-secret",
	}

	_, _, err = svc.RegisterWebPushSubscription("", validInput)
	assert.ErrorIs(t, err, ErrWebPushInvalidRequest)

	noEndpoint := validInput
	noEndpoint.Endpoint = ""
	_, _, err = svc.RegisterWebPushSubscription("user-1", noEndpoint)
	assert.ErrorIs(t, err, ErrWebPushInvalidRequest)

	badScheme := validInput
	badScheme.Endpoint = "http://push.example.com/abc"
	_, _, err = svc.RegisterWebPushSubscription("user-1", badScheme)
	assert.ErrorIs(t, err, ErrWebPushInvalidRequest)

	malformed := validInput
	malformed.Endpoint = "://not-a-url"
	_, _, err = svc.RegisterWebPushSubscription("user-1", malformed)
	assert.ErrorIs(t, err, ErrWebPushInvalidRequest)

	noKeys := validInput
	noKeys.P256dh = ""
	_, _, err = svc.RegisterWebPushSubscription("user-1", noKeys)
	assert.ErrorIs(t, err, ErrWebPushInvalidRequest)
}

func TestRegisterWebPushSubscription_NotProvisioned(t *testing.T) {
	svc := NewNotificationService(setupWebPushServiceTestDB(t), nil)

	_, _, err := svc.RegisterWebPushSubscription("user-1", WebPushSubscribeInput{
		Endpoint: "https://push.example.com/abc",
		P256dh:   "p256dh-key",
		Auth:     "auth-secret",
	})
	assert.ErrorIs(t, err, ErrWebPushNotProvisioned)
}

func TestRegisterWebPushSubscription_ProviderDisabled(t *testing.T) {
	db := setupWebPushServiceTestDB(t)
	svc := NewNotificationService(db, nil)

	provider, err := svc.ProvisionWebPush("Browser Push", "mailto:ops@example.com")
	require.NoError(t, err)
	require.NoError(t, db.Model(&models.NotificationProvider{}).Where("id = ?", provider.ID).Update("enabled", false).Error)

	_, _, err = svc.RegisterWebPushSubscription("user-1", WebPushSubscribeInput{
		Endpoint: "https://push.example.com/abc",
		P256dh:   "p256dh-key",
		Auth:     "auth-secret",
	})
	assert.ErrorIs(t, err, ErrWebPushProviderDisabled)
}

func TestRegisterWebPushSubscription_CreateThenReRegisterUpdatesExisting(t *testing.T) {
	svc := NewNotificationService(setupWebPushServiceTestDB(t), nil)
	_, err := svc.ProvisionWebPush("Browser Push", "mailto:ops@example.com")
	require.NoError(t, err)

	input := WebPushSubscribeInput{
		Endpoint:  "https://push.example.com/abc",
		P256dh:    "p256dh-key",
		Auth:      "auth-secret",
		UserAgent: "firefox",
	}

	sub, created, err := svc.RegisterWebPushSubscription("user-1", input)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "user-1", sub.UserID)

	input.UserAgent = "chrome"
	sub2, created2, err := svc.RegisterWebPushSubscription("user-1", input)
	require.NoError(t, err)
	assert.False(t, created2)
	assert.Equal(t, sub.ID, sub2.ID)
	assert.Equal(t, "chrome", sub2.UserAgent)
}

func TestListWebPushSubscriptionsForUser_ScopedToOwner(t *testing.T) {
	svc := NewNotificationService(setupWebPushServiceTestDB(t), nil)
	_, err := svc.ProvisionWebPush("Browser Push", "mailto:ops@example.com")
	require.NoError(t, err)

	_, _, err = svc.RegisterWebPushSubscription("user-1", WebPushSubscribeInput{
		Endpoint: "https://push.example.com/a", P256dh: "k", Auth: "s",
	})
	require.NoError(t, err)
	_, _, err = svc.RegisterWebPushSubscription("user-2", WebPushSubscribeInput{
		Endpoint: "https://push.example.com/b", P256dh: "k", Auth: "s",
	})
	require.NoError(t, err)

	subs, err := svc.ListWebPushSubscriptionsForUser("user-1")
	require.NoError(t, err)
	require.Len(t, subs, 1)
	assert.Equal(t, "user-1", subs[0].UserID)
}

func TestDeleteWebPushSubscription(t *testing.T) {
	svc := NewNotificationService(setupWebPushServiceTestDB(t), nil)
	_, err := svc.ProvisionWebPush("Browser Push", "mailto:ops@example.com")
	require.NoError(t, err)

	sub, _, err := svc.RegisterWebPushSubscription("user-1", WebPushSubscribeInput{
		Endpoint: "https://push.example.com/a", P256dh: "k", Auth: "s",
	})
	require.NoError(t, err)

	err = svc.DeleteWebPushSubscription("user-2", sub.ID)
	assert.ErrorIs(t, err, ErrWebPushSubscriptionNotFound, "wrong owner must not be able to delete another user's subscription")

	err = svc.DeleteWebPushSubscription("user-1", "nonexistent-id")
	assert.ErrorIs(t, err, ErrWebPushSubscriptionNotFound)

	err = svc.DeleteWebPushSubscription("user-1", sub.ID)
	assert.NoError(t, err)
}

func TestWithTransientSQLiteLockRetry(t *testing.T) {
	attempts := 0
	err := withTransientSQLiteLockRetry(func() error {
		attempts++
		if attempts < 3 {
			return gorm.ErrInvalidTransaction // placeholder non-lock error swapped below
		}
		return nil
	})
	// A non-lock error must not be retried — the first call's error returns immediately.
	require.Error(t, err)
	assert.Equal(t, 1, attempts)

	attempts = 0
	err = withTransientSQLiteLockRetry(func() error {
		attempts++
		if attempts < 3 {
			return errors.New("database table is locked")
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestIsDispatchEnabled_WebPush(t *testing.T) {
	svc := NewNotificationService(setupWebPushServiceTestDB(t), nil)

	assert.True(t, svc.isDispatchEnabled("webpush"), "webpush dispatch defaults to enabled like the other notify-module providers")

	require.NoError(t, svc.DB.Create(&models.Setting{Key: FlagWebPushServiceEnabled, Value: "false"}).Error)
	assert.False(t, svc.isDispatchEnabled("webpush"))
}

// TestSendExternal_WebPushProviderDispatchesViaNotify is SendExternal's
// counterpart to notify_webpush_adapter_test.go's direct
// dispatchWebPushViaNotify tests: it exercises SendExternal's own webpush
// type-branch (routing to dispatchWebPushViaNotify instead of the generic
// notify path), not the adapter's internals.
func TestSendExternal_WebPushProviderDispatchesViaNotify(t *testing.T) {
	db := setupWebPushServiceTestDB(t)
	svc := NewNotificationService(db, nil)

	_, err := svc.ProvisionWebPush("Browser Push", "mailto:ops@example.com")
	require.NoError(t, err)
	require.NoError(t, db.Model(&models.NotificationProvider{}).Where("type = ?", "webpush").
		Update("notify_proxy_hosts", true).Error)

	// No subscriptions are registered, so dispatchWebPushViaNotify is a
	// no-op fan-out — this only needs to prove SendExternal reaches and
	// invokes the webpush branch without panicking.
	svc.SendExternal(context.Background(), "proxy_host", "Title", "Message", nil)
}
