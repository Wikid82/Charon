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

// Phase 1.2 Additional Tests

// TestSecurityNotificationService_Send_EventTypeFiltering_WAFDisabled tests WAF filtering.
func TestSecurityNotificationService_Send_EventTypeFiltering_WAFDisabled(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	webhookCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &models.NotificationConfig{
		Enabled:         true,
		MinLogLevel:     "info",
		WebhookURL:      server.URL,
		NotifyWAFBlocks: false, // WAF blocks disabled
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
	assert.False(t, webhookCalled, "Webhook should not be called when WAF blocks are disabled")
}

// TestSecurityNotificationService_Send_EventTypeFiltering_ACLDisabled tests ACL filtering.
func TestSecurityNotificationService_Send_EventTypeFiltering_ACLDisabled(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	webhookCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &models.NotificationConfig{
		Enabled:         true,
		MinLogLevel:     "info",
		WebhookURL:      server.URL,
		NotifyWAFBlocks: true,
		NotifyACLDenies: false, // ACL denies disabled
	}
	require.NoError(t, svc.UpdateSettings(config))

	event := models.SecurityEvent{
		EventType: "acl_deny",
		Severity:  "warn",
		Message:   "Should be filtered",
	}

	err := svc.Send(context.Background(), event)
	assert.NoError(t, err)
	assert.False(t, webhookCalled, "Webhook should not be called when ACL denies are disabled")
}

// TestSecurityNotificationService_Send_SeverityBelowThreshold tests severity filtering.
func TestSecurityNotificationService_Send_SeverityBelowThreshold(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	webhookCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &models.NotificationConfig{
		Enabled:         true,
		MinLogLevel:     "error", // Minimum: error
		WebhookURL:      server.URL,
		NotifyWAFBlocks: true,
	}
	require.NoError(t, svc.UpdateSettings(config))

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "debug", // Below threshold
		Message:   "Should be filtered",
	}

	err := svc.Send(context.Background(), event)
	assert.NoError(t, err)
	assert.False(t, webhookCalled, "Webhook should not be called when severity is below threshold")
}

// TestSecurityNotificationService_Send_WebhookSuccess tests successful webhook dispatch.
func TestSecurityNotificationService_Send_WebhookSuccess(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	var receivedEvent models.SecurityEvent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Charon-Cerberus/1.0", r.Header.Get("User-Agent"))

		err := json.NewDecoder(r.Body).Decode(&receivedEvent)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &models.NotificationConfig{
		Enabled:         true,
		MinLogLevel:     "warn",
		WebhookURL:      server.URL,
		NotifyWAFBlocks: true,
	}
	require.NoError(t, svc.UpdateSettings(config))

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "error",
		Message:   "SQL injection detected",
		ClientIP:  "203.0.113.42",
		Path:      "/api/users?id=1' OR '1'='1",
		Timestamp: time.Now(),
	}

	err := svc.Send(context.Background(), event)
	assert.NoError(t, err)
	assert.Equal(t, event.EventType, receivedEvent.EventType)
	assert.Equal(t, event.Severity, receivedEvent.Severity)
	assert.Equal(t, event.Message, receivedEvent.Message)
	assert.Equal(t, event.ClientIP, receivedEvent.ClientIP)
	assert.Equal(t, event.Path, receivedEvent.Path)
}

// TestSecurityNotificationService_sendWebhook_SSRFBlocked tests SSRF protection.
func TestSecurityNotificationService_sendWebhook_SSRFBlocked(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	ssrfURLs := []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1/admin",
		"http://172.16.0.1/config",
		"http://192.168.1.1/api",
	}

	for _, url := range ssrfURLs {
		t.Run(url, func(t *testing.T) {
			event := models.SecurityEvent{
				EventType: "waf_block",
				Severity:  "error",
				Message:   "Test SSRF",
			}

			err := svc.sendWebhook(context.Background(), url, event)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid webhook URL")
		})
	}
}

// TestSecurityNotificationService_sendWebhook_MarshalError tests JSON marshal error handling.
func TestSecurityNotificationService_sendWebhook_MarshalError(t *testing.T) {
	// Note: With the current SecurityEvent model, it's difficult to trigger a marshal error
	// since all fields are standard types. This test documents the expected behavior.
	// In practice, marshal errors would only occur with custom types that implement
	// json.Marshaler incorrectly, which is not the case with SecurityEvent.
	t.Skip("JSON marshal error cannot be easily triggered with current SecurityEvent structure")
}

// TestSecurityNotificationService_sendWebhook_RequestCreationError tests request creation error.
func TestSecurityNotificationService_sendWebhook_RequestCreationError(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	// Use a canceled context to trigger request creation error
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "error",
		Message:   "Test",
	}

	// Note: With a canceled context, the error may occur during request execution
	// rather than creation, so we just verify an error occurs
	err := svc.sendWebhook(ctx, "https://example.com/webhook", event)
	assert.Error(t, err)
}

// TestSecurityNotificationService_sendWebhook_RequestExecutionError tests HTTP client error.
func TestSecurityNotificationService_sendWebhook_RequestExecutionError(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	// Use an invalid URL that will fail DNS resolution
	// Note: DNS resolution failures are caught by SSRF validation,
	// so this tests the error path through SSRF validator
	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "error",
		Message:   "Test execution error",
	}

	err := svc.sendWebhook(context.Background(), "https://invalid-nonexistent-domain-12345.test/hook", event)
	assert.Error(t, err)
	// The error should be from the SSRF validation layer (DNS resolution)
	assert.Contains(t, err.Error(), "invalid webhook URL")
}

// TestSecurityNotificationService_sendWebhook_Non200Status tests non-2xx HTTP status handling.
func TestSecurityNotificationService_sendWebhook_Non200Status(t *testing.T) {
	db := setupSecurityNotifTestDB(t)
	svc := NewSecurityNotificationService(db)

	statusCodes := []int{400, 404, 500, 502, 503}

	for _, statusCode := range statusCodes {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
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
				Message:   "Test non-2xx status",
			}

			err := svc.Send(context.Background(), event)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "webhook returned status")
		})
	}
}

// TestShouldNotify_AllSeverityCombinations tests all severity combinations.
func TestShouldNotify_AllSeverityCombinations(t *testing.T) {
	tests := []struct {
		eventSeverity string
		minLevel      string
		expected      bool
		description   string
	}{
		// debug (0) combinations
		{"debug", "debug", true, "debug >= debug"},
		{"debug", "info", false, "debug < info"},
		{"debug", "warn", false, "debug < warn"},
		{"debug", "error", false, "debug < error"},

		// info (1) combinations
		{"info", "debug", true, "info >= debug"},
		{"info", "info", true, "info >= info"},
		{"info", "warn", false, "info < warn"},
		{"info", "error", false, "info < error"},

		// warn (2) combinations
		{"warn", "debug", true, "warn >= debug"},
		{"warn", "info", true, "warn >= info"},
		{"warn", "warn", true, "warn >= warn"},
		{"warn", "error", false, "warn < error"},

		// error (3) combinations
		{"error", "debug", true, "error >= debug"},
		{"error", "info", true, "error >= info"},
		{"error", "warn", true, "error >= warn"},
		{"error", "error", true, "error >= error"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := shouldNotify(tt.eventSeverity, tt.minLevel)
			assert.Equal(t, tt.expected, result, "Expected %v for %s", tt.expected, tt.description)
		})
	}
}
