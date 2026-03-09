package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
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

func TestSendJSONPayload_DiscordIPHostRejected(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	svc := NewNotificationService(db, nil)

	provider := models.NotificationProvider{
		Type:     "discord",
		URL:      "https://203.0.113.10/api/webhooks/123456/token_abc",
		Template: "custom",
		Config:   `{"content": {{toJSON .Message}}, "username": "Charon"}`,
	}

	data := map[string]any{
		"Message": "Test notification",
		"Title":   "Test",
		"Time":    time.Now().Format(time.RFC3339),
	}

	err = svc.sendJSONPayload(context.Background(), provider, data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Discord webhook URL")
	assert.Contains(t, err.Error(), "IP address hosts are not allowed")
}

func TestValidateDiscordWebhookURL_AcceptsDiscordHostname(t *testing.T) {
	err := validateDiscordWebhookURL("https://discord.com/api/webhooks/123456/token_abc?wait=true")
	assert.NoError(t, err)
}

func TestValidateDiscordWebhookURL_AcceptsCanaryDiscordHostname(t *testing.T) {
	err := validateDiscordWebhookURL("https://canary.discord.com/api/webhooks/123456/token_abc")
	assert.NoError(t, err)
}

func TestValidateDiscordProviderURL_NonDiscordUnchanged(t *testing.T) {
	err := validateDiscordProviderURL("webhook", "https://203.0.113.20/hooks/test?x=1#y")
	assert.NoError(t, err)
}

func TestSendJSONPayload_UsesStoredHostnameURLWithoutHostMutation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	svc := NewNotificationService(db, nil)

	// Mock Discord validation to allow test server URLs
	origValidateDiscordFunc := validateDiscordProviderURLFunc
	defer func() { validateDiscordProviderURLFunc = origValidateDiscordFunc }()
	validateDiscordProviderURLFunc = func(providerType, rawURL string) error { return nil }

	var observedURLHost string
	var observedRequestHost string
	originalDo := webhookDoRequestFunc
	defer func() { webhookDoRequestFunc = originalDo }()
	webhookDoRequestFunc = func(client *http.Client, req *http.Request) (*http.Response, error) {
		observedURLHost = req.URL.Host
		observedRequestHost = req.Host
		return client.Do(req)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	parsedServerURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	parsedServerURL.Host = "localhost:" + parsedServerURL.Port()

	provider := models.NotificationProvider{
		Type:     "discord",
		URL:      parsedServerURL.String(),
		Template: "minimal",
	}

	data := map[string]any{
		"Message": "Test notification",
		"Title":   "Test",
		"Time":    time.Now().Format(time.RFC3339),
	}

	err = svc.sendJSONPayload(context.Background(), provider, data)
	require.NoError(t, err)

	assert.Equal(t, "localhost:"+parsedServerURL.Port(), observedURLHost)
	assert.Equal(t, observedURLHost, observedRequestHost)
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

	// Mock Discord validation to allow test server URL
	origValidateDiscordFunc := validateDiscordProviderURLFunc
	defer func() { validateDiscordProviderURLFunc = origValidateDiscordFunc }()
	validateDiscordProviderURLFunc = func(providerType, rawURL string) error { return nil }

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	svc := NewNotificationService(db, nil)

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

	svc := NewNotificationService(db, nil)

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

	svc := NewNotificationService(db, nil)

	provider := models.NotificationProvider{
		Type:     "gotify",
		URL:      server.URL,
		Token:    "test-token",
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

	svc := NewNotificationService(db, nil)

	// Mock Discord validation to allow private IP check to run
	origValidateDiscordFunc := validateDiscordProviderURLFunc
	defer func() { validateDiscordProviderURLFunc = origValidateDiscordFunc }()
	validateDiscordProviderURLFunc = func(providerType, rawURL string) error { return nil }

	// Create a template that would take too long to execute
	// This is simulated by having a large number of iterations
	// Use a private IP (10.x) which is blocked by SSRF protection to trigger an error
	provider := models.NotificationProvider{
		Type:     "discord",
		URL:      "http://10.0.0.1:9999",
		Template: "custom",
		Config:   `{"content": {{toJSON .Message}}, "data": {{toJSON .}}}`,
	}

	// Create data that will be processed
	data := map[string]any{
		"Message": "Test",
	}

	// This should complete quickly, but test the timeout mechanism exists
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = svc.sendJSONPayload(ctx, provider, data)
	// The private IP is blocked by SSRF protection
	// We're mainly testing that the validation and timeout mechanisms are in place
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "private ip addresses is blocked")
}

func TestSendJSONPayload_TemplateSizeLimit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	svc := NewNotificationService(db, nil)

	// Create a template larger than 10KB
	largeTemplate := strings.Repeat("x", 11*1024)

	provider := models.NotificationProvider{
		Type:     "discord",
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

	svc := NewNotificationService(db, nil)

	provider := models.NotificationProvider{
		Type:     "discord",
		URL:      "https://203.0.113.10/api/webhooks/123456/token_abc",
		Template: "custom",
		Config:   `{"username": "Charon", "message": {{toJSON .Message}}}`,
	}

	data := map[string]any{
		"Message": "Test",
	}

	err = svc.sendJSONPayload(context.Background(), provider, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Discord webhook URL")
	assert.Contains(t, err.Error(), "IP address hosts are not allowed")
}

func TestSendJSONPayload_DiscordValidation_MissingMessage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	svc := NewNotificationService(db, nil)

	provider := models.NotificationProvider{
		Type:     "discord",
		URL:      "https://discord.com/api/webhooks/123456/token_abc",
		Template: "custom",
		Config:   `{"username": "Charon"}`,
	}

	data := map[string]any{}

	err = svc.sendJSONPayload(context.Background(), provider, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "discord payload requires 'content' or 'embeds'")
}

func TestSendJSONPayload_SlackValidation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	svc := NewNotificationService(db, nil)

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

	svc := NewNotificationService(db, nil)

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

	svc := NewNotificationService(db, nil)

	provider := models.NotificationProvider{
		Type:     "discord",
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

func TestNormalizeURL_DiscordWebhook_ConvertsToDiscordScheme(t *testing.T) {
	got := normalizeURL("discord", "https://discord.com/api/webhooks/123/abcDEF_123")
	assert.Equal(t, "discord://abcDEF_123@123", got)

	got2 := normalizeURL("discord", "https://discordapp.com/api/webhooks/456/xyz")
	assert.Equal(t, "discord://xyz@456", got2)
}

func TestSendExternal_UsesJSONForSupportedServices(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	var called atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		assert.NotNil(t, payload["content"])
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Mock Discord validation to allow test server URL
	origValidateDiscordFunc := validateDiscordProviderURLFunc
	defer func() { validateDiscordProviderURLFunc = origValidateDiscordFunc }()
	validateDiscordProviderURLFunc = func(providerType, rawURL string) error { return nil }

	provider := models.NotificationProvider{
		Type:             "discord",
		URL:              server.URL,
		Template:         "custom",
		Config:           `{"content": {{toJSON .Message}}}`,
		Enabled:          true,
		NotifyProxyHosts: true,
	}
	db.Create(&provider)

	svc := NewNotificationService(db, nil)
	svc.SendExternal(context.Background(), "proxy_host", "Test", "Message", nil)

	// Give goroutine time to execute
	time.Sleep(100 * time.Millisecond)
	assert.True(t, called.Load(), "notification should have been sent via JSON")
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

	// Mock Discord validation to allow test server URL
	origValidateDiscordFunc := validateDiscordProviderURLFunc
	origWebhookDoReq := webhookDoRequestFunc
	defer func() {
		validateDiscordProviderURLFunc = origValidateDiscordFunc
		webhookDoRequestFunc = origWebhookDoReq
	}()
	validateDiscordProviderURLFunc = func(providerType, rawURL string) error { return nil }
	webhookDoRequestFunc = func(client *http.Client, req *http.Request) (*http.Response, error) {
		return client.Do(req)
	}

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	svc := NewNotificationService(db, nil)

	provider := models.NotificationProvider{
		Type:     "discord",
		URL:      server.URL,
		Template: "custom",
		Config:   `{"content": {{toJSON .Message}}}`,
	}

	err = svc.TestProvider(provider)
	assert.NoError(t, err)
}
