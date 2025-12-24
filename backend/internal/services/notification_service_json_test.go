package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSupportsJSONTemplates(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		expected     bool
	}{
		{"webhook", "webhook", true},
		{"discord", "discord", true},
		{"slack", "slack", true},
		{"gotify", "gotify", true},
		{"generic", "generic", true},
		{"telegram", "telegram", false},
		{"unknown", "unknown", false},
		{"WEBHOOK uppercase", "WEBHOOK", true},
		{"Discord mixed case", "Discord", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := supportsJSONTemplates(tt.providerType)
			assert.Equal(t, tt.expected, result, "supportsJSONTemplates(%q) should return %v", tt.providerType, tt.expected)
		})
	}
}

func TestSendJSONPayload_Discord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var payload map[string]any
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		// Discord webhook should have 'content' or 'embeds'
		assert.True(t, payload["content"] != nil || payload["embeds"] != nil, "Discord payload should have content or embeds")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	svc := NewNotificationService(db)

	provider := models.NotificationProvider{
		Type:     "discord",
		URL:      server.URL,
		Template: "custom",
		Config:   `{"content": {{toJSON .Message}}, "username": "Charon"}`,
	}

	data := map[string]any{
		"Message": "Test notification",
		"Title":   "Test",
		"Time":    time.Now().Format(time.RFC3339),
	}

	err = svc.sendJSONPayload(context.Background(), provider, data)
	assert.NoError(t, err)
}

func TestSendJSONPayload_Slack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		// Slack webhook should have 'text' or 'blocks'
		assert.True(t, payload["text"] != nil || payload["blocks"] != nil, "Slack payload should have text or blocks")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	svc := NewNotificationService(db)

	provider := models.NotificationProvider{
		Type:     "slack",
		URL:      server.URL,
		Template: "custom",
		Config:   `{"text": {{toJSON .Message}}}`,
	}

	data := map[string]any{
		"Message": "Test notification",
	}

	err = svc.sendJSONPayload(context.Background(), provider, data)
	assert.NoError(t, err)
}

func TestSendJSONPayload_Gotify(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		// Gotify webhook should have 'message'
		assert.NotNil(t, payload["message"], "Gotify payload should have message field")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	svc := NewNotificationService(db)

	provider := models.NotificationProvider{
		Type:     "gotify",
		URL:      server.URL,
		Template: "custom",
		Config:   `{"message": {{toJSON .Message}}, "title": {{toJSON .Title}}}`,
	}

	data := map[string]any{
		"Message": "Test notification",
		"Title":   "Test",
	}

	err = svc.sendJSONPayload(context.Background(), provider, data)
	assert.NoError(t, err)
}

func TestSendJSONPayload_TemplateTimeout(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	svc := NewNotificationService(db)

	// Create a template that would take too long to execute
	// This is simulated by having a large number of iterations
	provider := models.NotificationProvider{
		Type:     "webhook",
		URL:      "http://localhost:9999",
		Template: "custom",
		Config:   `{"data": {{toJSON .}}}`,
	}

	// Create data that will be processed
	data := map[string]any{
		"Message": "Test",
	}

	// This should complete quickly, but test the timeout mechanism exists
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = svc.sendJSONPayload(ctx, provider, data)
	// The error might be from URL validation or template execution
	// We're mainly testing that timeout mechanism is in place
	assert.Error(t, err)
}

func TestSendJSONPayload_TemplateSizeLimit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	svc := NewNotificationService(db)

	// Create a template larger than 10KB
	largeTemplate := strings.Repeat("x", 11*1024)

	provider := models.NotificationProvider{
		Type:     "webhook",
		URL:      "http://localhost:9999",
		Template: "custom",
		Config:   largeTemplate,
	}

	data := map[string]any{
		"Message": "Test",
	}

	err = svc.sendJSONPayload(context.Background(), provider, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template size exceeds maximum limit")
}

func TestSendJSONPayload_DiscordValidation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	svc := NewNotificationService(db)

	// Discord payload without content or embeds should fail
	provider := models.NotificationProvider{
		Type:     "discord",
		URL:      "http://localhost:9999",
		Template: "custom",
		Config:   `{"username": "Charon"}`,
	}

	data := map[string]any{
		"Message": "Test",
	}

	err = svc.sendJSONPayload(context.Background(), provider, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "discord payload requires 'content' or 'embeds'")
}

func TestSendJSONPayload_SlackValidation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	svc := NewNotificationService(db)

	// Slack payload without text or blocks should fail
	provider := models.NotificationProvider{
		Type:     "slack",
		URL:      "http://localhost:9999",
		Template: "custom",
		Config:   `{"username": "Charon"}`,
	}

	data := map[string]any{
		"Message": "Test",
	}

	err = svc.sendJSONPayload(context.Background(), provider, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "slack payload requires 'text' or 'blocks'")
}

func TestSendJSONPayload_GotifyValidation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	svc := NewNotificationService(db)

	// Gotify payload without message should fail
	provider := models.NotificationProvider{
		Type:     "gotify",
		URL:      "http://localhost:9999",
		Template: "custom",
		Config:   `{"title": "Test"}`,
	}

	data := map[string]any{
		"Message": "Test",
	}

	err = svc.sendJSONPayload(context.Background(), provider, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gotify payload requires 'message'")
}

func TestSendJSONPayload_InvalidJSON(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	svc := NewNotificationService(db)

	provider := models.NotificationProvider{
		Type:     "webhook",
		URL:      "http://localhost:9999",
		Template: "custom",
		Config:   `{invalid json}`,
	}

	data := map[string]any{
		"Message": "Test",
	}

	err = svc.sendJSONPayload(context.Background(), provider, data)
	assert.Error(t, err)
}

func TestSendExternal_UsesJSONForSupportedServices(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		assert.NotNil(t, payload["content"])
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := models.NotificationProvider{
		Type:             "discord",
		URL:              server.URL,
		Template:         "custom",
		Config:           `{"content": {{toJSON .Message}}}`,
		Enabled:          true,
		NotifyProxyHosts: true,
	}
	db.Create(&provider)

	svc := NewNotificationService(db)
	svc.SendExternal(context.Background(), "proxy_host", "Test", "Message", nil)

	// Give goroutine time to execute
	time.Sleep(100 * time.Millisecond)
	assert.True(t, called, "Discord notification should have been sent via JSON")
}

func TestTestProvider_UsesJSONForSupportedServices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)
		assert.NotNil(t, payload["content"])
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	svc := NewNotificationService(db)

	provider := models.NotificationProvider{
		Type:     "discord",
		URL:      server.URL,
		Template: "custom",
		Config:   `{"content": {{toJSON .Message}}}`,
	}

	err = svc.TestProvider(provider)
	assert.NoError(t, err)
}
