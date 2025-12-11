package services

import (
	"context"
	"encoding/json"
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

func setupSecurityNotifTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationConfig{}))
	return db
}

func TestNewSecurityNotificationService(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)
	assert.NotNil(t, svc)
}

func TestSecurityNotificationService_GetSettings_Default(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	config, err := svc.GetSettings()
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.False(t, config.Enabled)
	assert.Equal(t, "error", config.MinLogLevel)
	assert.True(t, config.NotifyWAFBlocks)
	assert.True(t, config.NotifyACLDenies)
}

func TestSecurityNotificationService_UpdateSettings(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	config := &models.NotificationConfig{
		Enabled:         true,
		MinLogLevel:     "warn",
		WebhookURL:      "https://example.com/webhook",
		NotifyWAFBlocks: true,
		NotifyACLDenies: false,
	}

	err := svc.UpdateSettings(config)
	require.NoError(t, err)

	// Retrieve and verify
	retrieved, err := svc.GetSettings()
	require.NoError(t, err)
	assert.True(t, retrieved.Enabled)
	assert.Equal(t, "warn", retrieved.MinLogLevel)
	assert.Equal(t, "https://example.com/webhook", retrieved.WebhookURL)
	assert.True(t, retrieved.NotifyWAFBlocks)
	assert.False(t, retrieved.NotifyACLDenies)
}

func TestSecurityNotificationService_UpdateSettings_Existing(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	// Create initial config
	initial := &models.NotificationConfig{
		Enabled:     false,
		MinLogLevel: "error",
	}
	require.NoError(t, svc.UpdateSettings(initial))

	// Update config
	updated := &models.NotificationConfig{
		Enabled:     true,
		MinLogLevel: "info",
	}
	require.NoError(t, svc.UpdateSettings(updated))

	// Verify update
	retrieved, err := svc.GetSettings()
	require.NoError(t, err)
	assert.True(t, retrieved.Enabled)
	assert.Equal(t, "info", retrieved.MinLogLevel)
}

func TestSecurityNotificationService_Send_Disabled(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "error",
		Message:   "Test event",
	}

	// Should not error when disabled
	err := svc.Send(context.Background(), event)
	assert.NoError(t, err)
}

func TestSecurityNotificationService_Send_FilteredByEventType(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	// Enable but disable WAF notifications
	config := &models.NotificationConfig{
		Enabled:         true,
		MinLogLevel:     "info",
		NotifyWAFBlocks: false,
		NotifyACLDenies: true,
	}
	require.NoError(t, svc.UpdateSettings(config))

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "error",
		Message:   "Should be filtered",
	}

	err := svc.Send(context.Background(), event)
	assert.NoError(t, err)
}

func TestSecurityNotificationService_Send_FilteredBySeverity(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	config := &models.NotificationConfig{
		Enabled:         true,
		MinLogLevel:     "error",
		NotifyWAFBlocks: true,
	}
	require.NoError(t, svc.UpdateSettings(config))

	// Info event should be filtered (min level is error)
	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "info",
		Message:   "Should be filtered",
	}

	err := svc.Send(context.Background(), event)
	assert.NoError(t, err)
}

func TestSecurityNotificationService_Send_WebhookSuccess(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	// Mock webhook server
	received := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var event models.SecurityEvent
		err := json.NewDecoder(r.Body).Decode(&event)
		require.NoError(t, err)
		assert.Equal(t, "waf_block", event.EventType)
		assert.Equal(t, "Test webhook", event.Message)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Configure webhook
	config := &models.NotificationConfig{
		Enabled:         true,
		MinLogLevel:     "info",
		WebhookURL:      server.URL,
		NotifyWAFBlocks: true,
	}
	require.NoError(t, svc.UpdateSettings(config))

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "warn",
		Message:   "Test webhook",
		ClientIP:  "192.168.1.1",
		Path:      "/test",
		Timestamp: time.Now(),
	}

	err := svc.Send(context.Background(), event)
	assert.NoError(t, err)
	assert.True(t, received, "Webhook should have been called")
}

func TestSecurityNotificationService_Send_WebhookFailure(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	// Mock webhook server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := &models.NotificationConfig{
		Enabled:         true,
		MinLogLevel:     "info",
		WebhookURL:      server.URL,
		NotifyWAFBlocks: true,
	}
	require.NoError(t, svc.UpdateSettings(config))

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "error",
		Message:   "Test failure",
	}

	err := svc.Send(context.Background(), event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "webhook returned status 500")
}

func TestShouldNotify(t *testing.T) {
	tests := []struct {
		name          string
		eventSeverity string
		minLevel      string
		expected      bool
	}{
		{"error >= error", "error", "error", true},
		{"warn < error", "warn", "error", false},
		{"error >= warn", "error", "warn", true},
		{"info >= info", "info", "info", true},
		{"debug < info", "debug", "info", false},
		{"error >= debug", "error", "debug", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldNotify(tt.eventSeverity, tt.minLevel)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSecurityNotificationService_Send_ACLDeny(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	// Mock webhook server
	received := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		var event models.SecurityEvent
		_ = json.NewDecoder(r.Body).Decode(&event)
		assert.Equal(t, "acl_deny", event.EventType)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &models.NotificationConfig{
		Enabled:         true,
		MinLogLevel:     "warn",
		WebhookURL:      server.URL,
		NotifyACLDenies: true,
	}
	require.NoError(t, svc.UpdateSettings(config))

	event := models.SecurityEvent{
		EventType: "acl_deny",
		Severity:  "warn",
		Message:   "ACL blocked",
		ClientIP:  "10.0.0.1",
	}

	err := svc.Send(context.Background(), event)
	assert.NoError(t, err)
	assert.True(t, received)
}

func TestSecurityNotificationService_Send_ContextTimeout(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	// Server that delays
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &models.NotificationConfig{
		Enabled:         true,
		MinLogLevel:     "info",
		WebhookURL:      server.URL,
		NotifyWAFBlocks: true,
	}
	require.NoError(t, svc.UpdateSettings(config))

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "error",
		Message:   "Test timeout",
	}

	// Context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	err := svc.Send(ctx, event)
	assert.Error(t, err)
}
