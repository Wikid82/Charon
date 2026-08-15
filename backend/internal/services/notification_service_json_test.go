package services

import (
	"context"
	"encoding/json"
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
		{"telegram", "telegram", true},
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

// TestSendExternal_UsesJSONForSupportedServices exercises Discord dispatch
// after its cutover to the extracted notify module (buildNotifySender).
// Discord's own webhook validation (providers/discord.ValidateWebhookURL)
// only accepts discord.com/canary.discord.com hosts, so an httptest.Server
// URL (as used before cutover) can no longer stand in for a Discord
// webhook — the test instead injects a capturing fake RoundTripper via
// WithNotifyTransportWrapper, matching the pattern
// notify_provider_adapter_test.go uses to test buildNotifySender directly.
func TestSendExternal_UsesJSONForSupportedServices(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	wrapper, rt := newCapturingWrapper()

	provider := models.NotificationProvider{
		Type:             "discord",
		URL:              "https://discord.com/api/webhooks/123456789/notify-json-token",
		Template:         "custom",
		Config:           `{"content": {{toJSON .Message}}}`,
		Enabled:          true,
		NotifyProxyHosts: true,
	}
	db.Create(&provider)

	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapper))
	svc.SendExternal(context.Background(), "proxy_host", "Test", "Message", nil)

	require.Eventually(t, func() bool {
		_, body := rt.last()
		return body != nil
	}, time.Second, 10*time.Millisecond, "notification should have been sent via JSON")

	_, body := rt.last()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	assert.NotNil(t, payload["content"])
}

// TestTestProvider_UsesJSONForSupportedServices is the TestProvider
// (test-send) counterpart of TestSendExternal_UsesJSONForSupportedServices
// — see its comment for why a capturing fake RoundTripper replaces the old
// httptest.Server + validateDiscordProviderURLFunc override.
func TestTestProvider_UsesJSONForSupportedServices(t *testing.T) {
	wrapper, rt := newCapturingWrapper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapper))

	provider := models.NotificationProvider{
		Type:     "discord",
		URL:      "https://discord.com/api/webhooks/123456789/notify-json-test-token",
		Template: "custom",
		Config:   `{"content": {{toJSON .Message}}}`,
	}

	err = svc.TestProvider(provider)
	assert.NoError(t, err)

	_, body := rt.last()
	require.NotNil(t, body)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	assert.NotNil(t, payload["content"])
}
