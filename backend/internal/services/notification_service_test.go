package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wikid82/go_notify_yourself/transport"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupNotificationTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	_ = db.AutoMigrate(&models.Notification{}, &models.NotificationProvider{})
	return db
}

func TestNotificationService_Create(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	notif, err := svc.Create(models.NotificationTypeInfo, "Test", "Message")
	require.NoError(t, err)
	assert.Equal(t, "Test", notif.Title)
	assert.Equal(t, "Message", notif.Message)
	assert.False(t, notif.Read)
}

func TestNotificationService_List(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	_, _ = svc.Create(models.NotificationTypeInfo, "N1", "M1")
	_, _ = svc.Create(models.NotificationTypeInfo, "N2", "M2")

	list, err := svc.List(false)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// Mark one as read
	db.Model(&models.Notification{}).Where("title = ?", "N1").Update("read", true)

	listUnread, err := svc.List(true)
	require.NoError(t, err)
	assert.Len(t, listUnread, 1)
	assert.Equal(t, "N2", listUnread[0].Title)
}

func TestNotificationService_MarkAsRead(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	notif, _ := svc.Create(models.NotificationTypeInfo, "N1", "M1")

	err := svc.MarkAsRead(notif.ID)
	require.NoError(t, err)

	var updated models.Notification
	db.First(&updated, "id = ?", notif.ID)
	assert.True(t, updated.Read)
}

func TestNotificationService_MarkAllAsRead(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	_, _ = svc.Create(models.NotificationTypeInfo, "N1", "M1")
	_, _ = svc.Create(models.NotificationTypeInfo, "N2", "M2")

	err := svc.MarkAllAsRead()
	require.NoError(t, err)

	var count int64
	db.Model(&models.Notification{}).Where("read = ?", false).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestNotificationService_Providers(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	// Create
	provider := models.NotificationProvider{
		Name: "Discord",
		Type: "discord",
		URL:  "https://discord.com/api/webhooks/123456/token_abc",
	}
	err := svc.CreateProvider(&provider)
	require.NoError(t, err)
	assert.NotEmpty(t, provider.ID)
	assert.Equal(t, "Discord", provider.Name)

	// List
	list, err := svc.ListProviders()
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Update
	provider.Name = "Discord Updated"
	err = svc.UpdateProvider(&provider)
	require.NoError(t, err)
	assert.Equal(t, "Discord Updated", provider.Name)

	// Delete
	err = svc.DeleteProvider(provider.ID)
	require.NoError(t, err)

	list, err = svc.ListProviders()
	require.NoError(t, err)
	assert.Len(t, list, 0)
}

// TestNotificationService_TestProvider_Webhook (despite its name, this
// exercises a Discord provider) verifies TestProvider dispatch after
// Discord's cutover to the extracted notify module (buildNotifySender).
// Discord's own webhook validation only accepts discord.com/
// canary.discord.com hosts, so it can no longer be pointed at an
// httptest.Server the way pre-cutover tests could — a capturing fake
// RoundTripper (via WithNotifyTransportWrapper) stands in instead, mirroring
// notify_provider_adapter_test.go's pattern for testing buildNotifySender.
func TestNotificationService_TestProvider_Webhook(t *testing.T) {
	db := setupNotificationTestDB(t)
	wrapper, rt := newCapturingWrapper()
	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapper))

	provider := models.NotificationProvider{
		Name:     "Test Discord",
		Type:     "discord",
		URL:      "https://discord.com/api/webhooks/123456789/webhook-test-token",
		Template: "minimal",
	}

	err := svc.TestProvider(provider)
	require.NoError(t, err)

	_, body := rt.last()
	require.NotNil(t, body)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	// Minimal template uses lowercase keys: title, message
	assert.Equal(t, "Test Notification", payload["title"])
}

// TestNotificationService_SendExternal exercises SendExternal's async
// Discord dispatch after cutover — see
// TestNotificationService_TestProvider_Webhook's comment for why a
// capturing fake RoundTripper replaces the old httptest.Server.
func TestNotificationService_SendExternal(t *testing.T) {
	db := setupNotificationTestDB(t)
	wrapper, rt := newCapturingWrapper()
	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapper))

	provider := models.NotificationProvider{
		Name:             "Test Discord",
		Type:             "discord",
		URL:              "https://discord.com/api/webhooks/123456789/send-external-token",
		Enabled:          true,
		NotifyProxyHosts: true,
		Template:         "minimal",
	}
	_ = svc.CreateProvider(&provider)

	svc.SendExternal(context.Background(), "proxy_host", "Title", "Message", nil)

	require.Eventually(t, func() bool {
		_, body := rt.last()
		return body != nil
	}, time.Second, 10*time.Millisecond, "Timed out waiting for webhook")
}

// TestNotificationService_SendExternal_MinimalVsDetailedTemplates verifies
// both built-in templates render correctly for a cut-over Discord provider.
// Each phase uses its own capturing wrapper/service instance, and the
// minimal-template provider is deleted before the detailed phase runs, so
// SendExternal's per-provider fan-out never dispatches both providers to
// the same capturing wrapper at once (which would race the "last request"
// assertions below).
func TestNotificationService_SendExternal_MinimalVsDetailedTemplates(t *testing.T) {
	db := setupNotificationTestDB(t)

	// Minimal template phase
	wrapperMin, rtMin := newCapturingWrapper()
	svcMin := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapperMin))

	providerMin := models.NotificationProvider{
		Name:         "Minimal Discord",
		Type:         "discord",
		URL:          "https://discord.com/api/webhooks/1/minimal-token",
		Enabled:      true,
		NotifyUptime: true,
		Template:     "minimal",
	}
	require.NoError(t, svcMin.CreateProvider(&providerMin))

	data := map[string]any{"Title": "Min Title", "Message": "Min Message", "Time": time.Now().Format(time.RFC3339), "EventType": "uptime"}
	svcMin.SendExternal(context.Background(), "uptime", "Min Title", "Min Message", data)

	require.Eventually(t, func() bool {
		_, body := rtMin.last()
		return body != nil
	}, 500*time.Millisecond, 10*time.Millisecond, "Timeout waiting for minimal webhook")

	_, minBody := rtMin.last()
	var minPayload map[string]any
	require.NoError(t, json.Unmarshal(minBody, &minPayload))
	// minimal template should contain 'title' and 'message' keys
	assert.Equal(t, "Min Title", minPayload["title"])

	require.NoError(t, svcMin.DeleteProvider(providerMin.ID))

	// Detailed template phase
	wrapperDet, rtDet := newCapturingWrapper()
	svcDet := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapperDet))

	providerDet := models.NotificationProvider{
		Name:         "Detailed Discord",
		Type:         "discord",
		URL:          "https://discord.com/api/webhooks/2/detailed-token",
		Enabled:      true,
		NotifyUptime: true,
		Template:     "detailed",
	}
	require.NoError(t, svcDet.CreateProvider(&providerDet))

	dataDet := map[string]any{"Title": "Det Title", "Message": "Det Message", "Time": time.Now().Format(time.RFC3339), "EventType": "uptime", "HostName": "example-host", "HostIP": "1.2.3.4", "ServiceCount": 1, "Services": []map[string]any{{"Name": "svc1"}}}
	svcDet.SendExternal(context.Background(), "uptime", "Det Title", "Det Message", dataDet)

	require.Eventually(t, func() bool {
		_, body := rtDet.last()
		return body != nil
	}, 500*time.Millisecond, 10*time.Millisecond, "Timeout waiting for detailed webhook")

	_, detBody := rtDet.last()
	var detPayload map[string]any
	require.NoError(t, json.Unmarshal(detBody, &detPayload))
	// detailed template should contain 'host' and 'services'
	assert.Equal(t, "example-host", detPayload["host"])
	if _, ok := detPayload["services"]; !ok {
		t.Fatalf("expected services in detailed body")
	}
}

func TestNotificationService_SendExternal_Filtered(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	received := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(received)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	provider := models.NotificationProvider{
		Name:             "Test Webhook",
		Type:             "webhook",
		URL:              ts.URL,
		Enabled:          true,
		NotifyProxyHosts: false, // Disabled
	}
	_ = svc.CreateProvider(&provider)
	// Force update to false because GORM default tag might override zero value (false) on Create
	db.Model(&provider).Update("notify_proxy_hosts", false)

	svc.SendExternal(context.Background(), "proxy_host", "Title", "Message", nil)

	select {
	case <-received:
		t.Fatal("Should not have received webhook")
	case <-time.After(100 * time.Millisecond):
		// Success (timeout expected)
	}
}

func TestNotificationService_TestProvider_Errors(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	t.Run("unsupported provider type", func(t *testing.T) {
		provider := models.NotificationProvider{
			Type: "unsupported",
			URL:  "https://discord.com/api/webhooks/123/abc",
		}
		err := svc.TestProvider(provider)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported provider type")
	})

	t.Run("discord with invalid URL format", func(t *testing.T) {
		provider := models.NotificationProvider{
			Type: "discord",
			URL:  "invalid-discord-url",
		}
		err := svc.TestProvider(provider)
		assert.Error(t, err)
	})

	t.Run("slack with missing webhook URL", func(t *testing.T) {
		provider := models.NotificationProvider{
			Type:     "slack",
			URL:      "#alerts",
			Token:    "",
			Template: "minimal",
		}
		err := svc.TestProvider(provider)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "slack webhook URL is not configured")
	})

	t.Run("webhook success", func(t *testing.T) {
		// Discord's own webhook validation only accepts discord.com/
		// canary.discord.com hosts (see TestNotificationService_SendExternal's
		// comment), so a capturing fake RoundTripper stands in for the old
		// httptest.Server here.
		wrapper, _ := newCapturingWrapper()
		svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapper))

		provider := models.NotificationProvider{
			Type:     "discord",
			URL:      "https://discord.com/api/webhooks/1/webhook-success-token",
			Template: "minimal",
		}
		err := svc.TestProvider(provider)
		assert.NoError(t, err)
	})
}

func TestSSRF_URLValidation_PrivateIP(t *testing.T) {
	// Direct IP literal within RFC1918 block should be rejected
	// Using security.ValidateExternalURL with AllowHTTP option
	_, err := security.ValidateExternalURL("http://10.0.0.1", security.WithAllowHTTP())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "private")

	// Loopback allowed when WithAllowLocalhost is set
	validatedURL, err := security.ValidateExternalURL("http://127.0.0.1:8080",
		security.WithAllowHTTP(),
		security.WithAllowLocalhost(),
	)
	assert.NoError(t, err)
	assert.Contains(t, validatedURL, "127.0.0.1")

	// Loopback NOT allowed without WithAllowLocalhost
	_, err = security.ValidateExternalURL("http://127.0.0.1:8080", security.WithAllowHTTP())
	assert.Error(t, err)
}

func TestSSRF_URLValidation_ComprehensiveBlocking(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		shouldBlock bool
		description string
	}{
		// RFC 1918 private ranges
		{"10.0.0.0/8", "http://10.0.0.1", true, "Class A private network"},
		{"10.255.255.254", "http://10.255.255.254", true, "Class A private high end"},
		{"172.16.0.0/12", "http://172.16.0.1", true, "Class B private network start"},
		{"172.31.255.254", "http://172.31.255.254", true, "Class B private network end"},
		{"192.168.0.0/16", "http://192.168.1.1", true, "Class C private network"},

		// Edge cases for 172.x range (16-31 is private, others are not)
		{"172.15.x (not private)", "http://172.15.0.1", false, "Below private range"},
		{"172.32.x (not private)", "http://172.32.0.1", false, "Above private range"},

		// Link-local / Cloud metadata
		{"169.254.169.254", "http://169.254.169.254", true, "AWS/GCP metadata endpoint"},

		// Loopback (blocked without WithAllowLocalhost)
		{"localhost", "http://localhost", true, "Localhost hostname"},
		{"127.0.0.1", "http://127.0.0.1", true, "IPv4 loopback"},
		{"::1", "http://[::1]", true, "IPv6 loopback"},

		// Valid external URLs (should pass)
		{"google.com", "https://google.com", false, "Public external URL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test WITHOUT AllowLocalhost - should block localhost variants
			_, err := security.ValidateExternalURL(tt.url, security.WithAllowHTTP())
			if tt.shouldBlock {
				assert.Error(t, err, "Expected %s to be blocked: %s", tt.url, tt.description)
			} else {
				assert.NoError(t, err, "Expected %s to be allowed: %s", tt.url, tt.description)
			}
		})
	}
}

func TestNotificationService_SendExternal_EdgeCases(t *testing.T) {
	t.Run("no enabled providers", func(t *testing.T) {
		db := setupNotificationTestDB(t)
		svc := NewNotificationService(db, nil)

		provider := models.NotificationProvider{
			Name:    "Disabled",
			Type:    "webhook",
			URL:     "https://discord.com/api/webhooks/123/abc",
			Enabled: false,
		}
		_ = svc.CreateProvider(&provider)

		// Should complete without error
		svc.SendExternal(context.Background(), "proxy_host", "Title", "Message", nil)
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("provider filtered by category", func(t *testing.T) {
		db := setupNotificationTestDB(t)
		svc := NewNotificationService(db, nil)

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Should not call webhook")
		}))
		defer ts.Close()

		provider := models.NotificationProvider{
			Name:             "Filtered",
			Type:             "webhook",
			URL:              ts.URL,
			Enabled:          true,
			NotifyProxyHosts: false,
			NotifyUptime:     false,
			NotifyCerts:      false,
		}
		// Create provider first (might get defaults)
		err := db.Create(&provider).Error
		require.NoError(t, err)

		// Force update to false using map (to bypass zero value check)
		err = db.Model(&provider).Updates(map[string]any{
			"notify_proxy_hosts":    false,
			"notify_uptime":         false,
			"notify_certs":          false,
			"notify_remote_servers": false,
			"notify_domains":        false,
		}).Error
		require.NoError(t, err)

		// Verify DB state
		var saved models.NotificationProvider
		db.First(&saved, "id = ?", provider.ID)
		require.False(t, saved.NotifyProxyHosts, "NotifyProxyHosts should be false")
		require.False(t, saved.NotifyUptime, "NotifyUptime should be false")
		require.False(t, saved.NotifyCerts, "NotifyCerts should be false")

		svc.SendExternal(context.Background(), "proxy_host", "Title", "Message", nil)
		svc.SendExternal(context.Background(), "uptime", "Title", "Message", nil)
		svc.SendExternal(context.Background(), "cert", "Title", "Message", nil)
		time.Sleep(50 * time.Millisecond)
	})

	// TestNotificationService_SendExternal_EdgeCases/custom_data_passed_to_webhook
	// covers a cut-over Discord provider configured with the "detailed"
	// template, verifying that SendExternal's HostName extra (passed in the
	// `data` map, same as before cutover) still reaches the rendered
	// payload via legacyDetailedTemplate's backward-compat translation
	// (notify_provider_adapter.go). Note a scope change from the
	// pre-cutover version of this test: sendJSONPayload's old flat data map
	// exposed ANY caller-supplied key (e.g. an arbitrary "CustomField") to
	// a *custom* template at its top level. The extracted notify module's
	// render.TemplateData only exposes Title/Message/Time/EventType/Data,
	// and dispatchViaNotify (notification_service.go) only populates Data
	// with the four documented keys — a "custom" template referencing
	// {{index .Data "HostName"}} would additionally fail CreateProvider's
	// preview-validation step until the webhook commit (§6 commit 9)
	// updates RenderTemplate's call sites to the new preview payload shape
	// — so this test exercises the documented Data contract via the
	// "detailed" template instead, which bypasses that preview validation.
	t.Run("custom data passed to webhook", func(t *testing.T) {
		db := setupNotificationTestDB(t)
		wrapper, rt := newCapturingWrapper()
		svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapper))

		provider := models.NotificationProvider{
			Name:             "Custom Data Discord",
			Type:             "discord",
			URL:              "https://discord.com/api/webhooks/1/custom-data-token",
			Enabled:          true,
			NotifyProxyHosts: true,
			Template:         "detailed",
		}
		require.NoError(t, svc.CreateProvider(&provider))

		customData := map[string]any{
			"HostName": "test-value",
		}
		svc.SendExternal(context.Background(), "proxy_host", "Title", "Message", customData)

		require.Eventually(t, func() bool {
			_, body := rt.last()
			return body != nil
		}, time.Second, 10*time.Millisecond, "expected webhook to be sent")

		_, body := rt.last()
		var payload map[string]any
		require.NoError(t, json.Unmarshal(body, &payload))
		assert.Equal(t, "test-value", payload["host"])
	})
}

func TestNotificationService_RenderTemplate(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	// Minimal template
	provider := models.NotificationProvider{Type: "webhook", Template: "minimal"}
	data := map[string]any{"Title": "T1", "Message": "M1", "Time": time.Now().Format(time.RFC3339), "EventType": "preview"}
	rendered, parsed, err := svc.RenderTemplate(provider, data)
	require.NoError(t, err)
	assert.Contains(t, rendered, "T1")
	if parsedMap, ok := parsed.(map[string]any); ok {
		assert.Equal(t, "T1", parsedMap["title"])
	}

	// Invalid custom template returns error
	provider = models.NotificationProvider{Type: "webhook", Template: "custom", Config: `{"bad": "{{.Title"}`}
	_, _, err = svc.RenderTemplate(provider, data)
	assert.Error(t, err)
}

func TestNotificationService_CreateProvider_Validation(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	t.Run("creates provider with defaults", func(t *testing.T) {
		provider := models.NotificationProvider{
			Name: "Test Discord",
			Type: "discord",
			URL:  "https://discord.com/api/webhooks/123/abc",
		}
		err := svc.CreateProvider(&provider)
		assert.NoError(t, err)
		assert.NotEmpty(t, provider.ID)
		assert.False(t, provider.Enabled) // Default
	})

	t.Run("updates existing provider", func(t *testing.T) {
		provider := models.NotificationProvider{
			Name:    "Original Discord",
			Type:    "discord",
			URL:     "https://discord.com/api/webhooks/123/abc",
			Enabled: true,
		}
		err := svc.CreateProvider(&provider)
		assert.NoError(t, err)

		provider.Name = "Updated"
		err = svc.UpdateProvider(&provider)
		assert.NoError(t, err)

		var updated models.NotificationProvider
		db.First(&updated, "id = ?", provider.ID)
		assert.Equal(t, "Updated", updated.Name)
	})

	t.Run("deletes non-existent provider", func(t *testing.T) {
		err := svc.DeleteProvider("non-existent-id")
		// Should not error on missing provider
		assert.NoError(t, err)
	})
}

func TestNotificationService_CreateProvider_InvalidCustomTemplate(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	t.Run("invalid custom template on create", func(t *testing.T) {
		provider := models.NotificationProvider{
			Name:     "Bad Custom",
			Type:     "discord",
			URL:      "https://discord.com/api/webhooks/123/abc",
			Template: "custom",
			Config:   `{"bad": "{{.Title"}`,
		}
		err := svc.CreateProvider(&provider)
		assert.Error(t, err)
	})

	t.Run("invalid custom template on update", func(t *testing.T) {
		provider := models.NotificationProvider{
			Name:     "Valid",
			Type:     "discord",
			URL:      "https://discord.com/api/webhooks/123/abc",
			Template: "minimal",
		}
		err := svc.CreateProvider(&provider)
		require.NoError(t, err)

		provider.Template = "custom"
		provider.Config = `{"bad": "{{.Title"}`
		err = svc.UpdateProvider(&provider)
		assert.Error(t, err)
	})
}

// ============================================
// Phase 2.2: Additional Coverage Tests
// ============================================

func TestRenderTemplate_TemplateParseError(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	provider := models.NotificationProvider{
		Template: "custom",
		Config:   `{"invalid": {{.Title}`, // Invalid JSON template - missing closing brace
	}

	data := map[string]any{
		"Title":     "Test",
		"Message":   "Test",
		"Time":      time.Now().Format(time.RFC3339),
		"EventType": "test",
	}

	_, _, err := svc.RenderTemplate(provider, data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestRenderTemplate_TemplateExecutionError(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	provider := models.NotificationProvider{
		Template: "custom",
		Config:   `{"title": {{toJSON .Title}}, "broken": {{.NonExistent}}}`, // References missing field without toJSON
	}

	data := map[string]any{
		"Title":     "Test",
		"Message":   "Test",
		"Time":      time.Now().Format(time.RFC3339),
		"EventType": "test",
	}

	rendered, parsed, err := svc.RenderTemplate(provider, data)
	// Go templates don't error on missing fields, they just render "<no value>"
	// So this should actually succeed but produce invalid JSON
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse rendered template")
	assert.NotEmpty(t, rendered)
	assert.Nil(t, parsed)
}

func TestRenderTemplate_InvalidJSONOutput(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	provider := models.NotificationProvider{
		Template: "custom",
		Config:   `{"title": {{.Title}}}`, // Missing toJSON, will produce invalid JSON
	}

	data := map[string]any{
		"Title":     "Test",
		"Message":   "Test",
		"Time":      time.Now().Format(time.RFC3339),
		"EventType": "test",
	}

	rendered, parsed, err := svc.RenderTemplate(provider, data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse rendered template")
	assert.NotEmpty(t, rendered) // Rendered string returned even on validation error
	assert.Nil(t, parsed)
}

func TestCreateProvider_EmptyCustomTemplateAllowed(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	provider := &models.NotificationProvider{
		Name:     "empty-template",
		Type:     "discord",
		URL:      "https://discord.com/api/webhooks/123456/test_token",
		Template: "custom",
		Config:   "", // Empty should be allowed and default to minimal
	}

	err := svc.CreateProvider(provider)
	require.NoError(t, err)
	assert.NotEmpty(t, provider.ID)
}

func TestUpdateProvider_NonCustomTemplateSkipsValidation(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	provider := &models.NotificationProvider{
		Name:     "test",
		Type:     "discord",
		URL:      "https://discord.com/api/webhooks/123456/test_token",
		Template: "minimal",
	}
	require.NoError(t, db.Create(provider).Error)

	// Update to detailed template (Config can be garbage since it's ignored)
	provider.Template = "detailed"
	provider.Config = "this is not JSON but should be ignored"

	err := svc.UpdateProvider(provider)
	require.NoError(t, err) // Should succeed because detailed template doesn't use Config
}

func TestSendExternal_UnknownEventTypeSendsToAll(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := models.NotificationProvider{
		Name:    "all-disabled",
		Type:    "webhook",
		URL:     server.URL,
		Enabled: true,
		// All notification types disabled
		NotifyProxyHosts:    false,
		NotifyRemoteServers: false,
		NotifyDomains:       false,
		NotifyCerts:         false,
		NotifyUptime:        false,
	}
	require.NoError(t, db.Create(&provider).Error)

	// Force update with map to avoid zero value issues
	require.NoError(t, db.Model(&provider).Updates(map[string]any{
		"notify_proxy_hosts":    false,
		"notify_remote_servers": false,
		"notify_domains":        false,
		"notify_certs":          false,
		"notify_uptime":         false,
	}).Error)

	// Send with unknown event type - should NOT send (security-first: default false)
	ctx := context.Background()
	svc.SendExternal(ctx, "unknown_event_type", "Test", "Message", nil)

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(0), callCount.Load(), "Unknown event type should not trigger notification (security-first)")
}

func TestCreateProvider_ValidCustomTemplate(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	provider := &models.NotificationProvider{
		Name:     "valid-custom",
		Type:     "discord",
		URL:      "https://discord.com/api/webhooks/123456/test_token",
		Template: "custom",
		Config:   `{"message": {{toJSON .Message}}, "title": {{toJSON .Title}}, "custom_field": "value"}`,
	}

	err := svc.CreateProvider(provider)
	require.NoError(t, err)
	assert.NotEmpty(t, provider.ID)
}

func TestUpdateProvider_ValidCustomTemplate(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	provider := &models.NotificationProvider{
		Name:     "test",
		Type:     "discord",
		URL:      "https://discord.com/api/webhooks/123456/test_token",
		Template: "minimal",
	}
	require.NoError(t, db.Create(provider).Error)

	// Update to valid custom template
	provider.Template = "custom"
	provider.Config = `{"msg": {{toJSON .Message}}, "title": {{toJSON .Title}}}`

	err := svc.UpdateProvider(provider)
	require.NoError(t, err)
}

func TestRenderTemplate_MinimalAndDetailedTemplates(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	data := map[string]any{
		"Title":        "Test Title",
		"Message":      "Test Message",
		"Time":         time.Now().Format(time.RFC3339),
		"EventType":    "test",
		"HostName":     "testhost",
		"HostIP":       "192.168.1.1",
		"ServiceCount": 5,
		"Services":     []string{"web", "api"},
	}

	t.Run("minimal template", func(t *testing.T) {
		provider := models.NotificationProvider{
			Template: "minimal",
		}

		rendered, parsed, err := svc.RenderTemplate(provider, data)
		require.NoError(t, err)
		require.NotEmpty(t, rendered)
		require.NotNil(t, parsed)

		parsedMap := parsed.(map[string]any)
		assert.Equal(t, "Test Title", parsedMap["title"])
		assert.Equal(t, "Test Message", parsedMap["message"])
	})

	t.Run("detailed template", func(t *testing.T) {
		provider := models.NotificationProvider{
			Template: "detailed",
		}

		rendered, parsed, err := svc.RenderTemplate(provider, data)
		require.NoError(t, err)
		require.NotEmpty(t, rendered)
		require.NotNil(t, parsed)

		parsedMap := parsed.(map[string]any)
		assert.Equal(t, "Test Title", parsedMap["title"])
		assert.Equal(t, "testhost", parsedMap["host"])
		assert.Equal(t, "192.168.1.1", parsedMap["host_ip"])
		assert.Equal(t, float64(5), parsedMap["service_count"])
	})
}

// ============================================
// Phase 3: Service-Specific Validation Tests
// ============================================

func TestSendExternal_AllEventTypes(t *testing.T) {
	eventTypes := []struct {
		eventType     string
		providerField string
	}{
		{"proxy_host", "NotifyProxyHosts"},
		{"remote_server", "NotifyRemoteServers"},
		{"domain", "NotifyDomains"},
		{"cert", "NotifyCerts"},
		{"uptime", "NotifyUptime"},
		{"test", ""},    // test always sends
		{"unknown", ""}, // unknown defaults to false (security-first)
	}

	for _, et := range eventTypes {
		t.Run(et.eventType, func(t *testing.T) {
			db := setupNotificationTestDB(t)
			wrapper, rt := newCapturingWrapper()
			svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapper))

			provider := models.NotificationProvider{
				Name:                "event-test",
				Type:                "discord",
				URL:                 "https://discord.com/api/webhooks/1/event-test-token",
				Enabled:             true,
				Template:            "minimal",
				NotifyProxyHosts:    et.eventType == "proxy_host",
				NotifyRemoteServers: et.eventType == "remote_server",
				NotifyDomains:       et.eventType == "domain",
				NotifyCerts:         et.eventType == "cert",
				NotifyUptime:        et.eventType == "uptime",
			}
			require.NoError(t, db.Create(&provider).Error)

			// Update with map to ensure zero values are set properly
			require.NoError(t, db.Model(&provider).Updates(map[string]any{
				"notify_proxy_hosts":    et.eventType == "proxy_host",
				"notify_remote_servers": et.eventType == "remote_server",
				"notify_domains":        et.eventType == "domain",
				"notify_certs":          et.eventType == "cert",
				"notify_uptime":         et.eventType == "uptime",
			}).Error)

			svc.SendExternal(context.Background(), et.eventType, "Title", "Message", nil)

			// test always sends; unknown defaults to false (security-first); others only when their flag is true
			switch et.eventType {
			case "unknown":
				time.Sleep(100 * time.Millisecond)
				_, body := rt.last()
				assert.Nil(t, body, "Unknown event type should not trigger notification (security-first)")
			default:
				require.Eventually(t, func() bool {
					_, body := rt.last()
					return body != nil
				}, time.Second, 10*time.Millisecond, "Event type %s should trigger notification", et.eventType)
			}
		})
	}
}

func TestNotificationService_SendExternal_SecurityEventRouting(t *testing.T) {
	eventCases := []struct {
		name      string
		eventType string
		apply     func(p *models.NotificationProvider)
	}{
		{
			name:      "security_waf",
			eventType: "security_waf",
			apply: func(p *models.NotificationProvider) {
				p.NotifySecurityWAFBlocks = true
			},
		},
		{
			name:      "security_acl",
			eventType: "security_acl",
			apply: func(p *models.NotificationProvider) {
				p.NotifySecurityACLDenies = true
			},
		},
		{
			name:      "security_rate_limit",
			eventType: "security_rate_limit",
			apply: func(p *models.NotificationProvider) {
				p.NotifySecurityRateLimitHits = true
			},
		},
		{
			name:      "security_crowdsec",
			eventType: "security_crowdsec",
			apply: func(p *models.NotificationProvider) {
				p.NotifySecurityCrowdSecDecisions = true
			},
		},
	}

	for _, tc := range eventCases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupNotificationTestDB(t)
			wrapper, rt := newCapturingWrapper()
			svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapper))

			provider := models.NotificationProvider{
				Name:     "discord-security",
				Type:     "discord",
				URL:      "https://discord.com/api/webhooks/1/security-token",
				Enabled:  true,
				Template: "minimal",
			}
			tc.apply(&provider)
			require.NoError(t, db.Create(&provider).Error)

			svc.SendExternal(context.Background(), tc.eventType, "Security Title", "Security Message", nil)

			require.Eventually(t, func() bool {
				_, body := rt.last()
				return body != nil
			}, time.Second, 10*time.Millisecond, "expected dispatch for event type %s", tc.eventType)
		})
	}
}

func TestNotificationService_UpdateProvider_ReturnsErrorWhenProviderMissing(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	err := svc.UpdateProvider(&models.NotificationProvider{
		ID:   "missing-id",
		Type: "discord",
		URL:  "https://discord.com/api/webhooks/123/token",
	})
	require.Error(t, err)
}

func TestNotificationService_EnsureNotifyOnlyProviderMigration_QueryProvidersError(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	err = svc.EnsureNotifyOnlyProviderMigration(context.Background())
	require.Error(t, err)
}

func TestNotificationService_EnsureNotifyOnlyProviderMigration_UpdateError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration_update_error.db")

	rwDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, rwDB.AutoMigrate(&models.NotificationProvider{}))
	require.NoError(t, rwDB.Create(&models.NotificationProvider{
		ID:             "provider-to-update",
		Name:           "Legacy Webhook",
		Type:           "webhook",
		URL:            "https://example.com/webhook",
		Enabled:        true,
		MigrationState: "pending",
	}).Error)

	rwSQLDB, err := rwDB.DB()
	require.NoError(t, err)
	require.NoError(t, rwSQLDB.Close())

	roDSN := fmt.Sprintf("file:%s?mode=ro", dbPath)
	roDB, err := gorm.Open(sqlite.Open(roDSN), &gorm.Config{})
	require.NoError(t, err)

	svc := NewNotificationService(roDB, nil)
	err = svc.EnsureNotifyOnlyProviderMigration(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to migrate notification provider")

	roSQLDB, sqlErr := roDB.DB()
	if sqlErr == nil {
		_ = roSQLDB.Close()
	}
	_ = os.Remove(dbPath)
}

func TestNotificationService_EnsureNotifyOnlyProviderMigration_WrapsFindError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// Intentionally do not migrate notification_providers table.

	svc := NewNotificationService(db, nil)
	err = svc.EnsureNotifyOnlyProviderMigration(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch notification providers for migration")
}

func TestTestProvider_NotifyOnlyRejectsUnsupportedProvider(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	// Test truly unsupported providers are rejected
	tests := []struct {
		name         string
		providerType string
		url          string
	}{
		{"sms", "sms", "sms://token@user"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := models.NotificationProvider{
				Type:     tt.providerType,
				URL:      tt.url,
				Template: "",
			}

			err := svc.TestProvider(provider)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unsupported provider type")
		})
	}
}

func TestTestProvider_HTTPURLValidation(t *testing.T) {
	db := setupNotificationTestDB(t)

	t.Run("blocks failed dispatch", func(t *testing.T) {
		rt := &capturingRoundTripper{statusCode: http.StatusInternalServerError}
		wrapper := transport.NewWrapper(
			transport.WithClientFactory(func(bool, int) *http.Client {
				return &http.Client{Transport: rt}
			}),
			transport.WithURLValidator(func(rawURL string, _ bool) (string, error) {
				return rawURL, nil
			}),
			transport.WithRetryPolicy(transport.RetryPolicy{MaxAttempts: 1}),
		)
		svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapper))

		provider := models.NotificationProvider{
			Type:     "discord",
			URL:      "https://discord.com/api/webhooks/999/invalidtoken",
			Template: "minimal",
		}

		err := svc.TestProvider(provider)
		require.Error(t, err)
	})

	t.Run("allows valid discord webhook", func(t *testing.T) {
		wrapper, rt := newCapturingWrapper()
		svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapper))

		provider := models.NotificationProvider{
			Type:     "discord",
			URL:      "https://discord.com/api/webhooks/123456789/validtoken_abc",
			Template: "minimal",
		}

		err := svc.TestProvider(provider)
		require.NoError(t, err)

		_, body := rt.last()
		require.NotNil(t, body)
	})
}

// ============================================
// Phase 4: Additional Edge Case Coverage
// ============================================

func TestRenderTemplate_CustomTemplateWithWhitespace(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	// Test template selection with various whitespace
	tests := []struct {
		name     string
		template string
	}{
		{"detailed with spaces", "  detailed  "},
		{"minimal with tabs", "\tminimal\t"},
		{"custom with newlines", "\ncustom\n"},
		{"DETAILED uppercase", "DETAILED"},
		{"MiNiMaL mixed case", "MiNiMaL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := models.NotificationProvider{
				Template: tt.template,
				Config:   `{"msg": {{toJSON .Message}}}`, // Only used for custom
			}

			data := map[string]any{
				"Title":     "Test",
				"Message":   "Message",
				"Time":      time.Now().Format(time.RFC3339),
				"EventType": "test",
			}

			rendered, parsed, err := svc.RenderTemplate(provider, data)
			require.NoError(t, err)
			require.NotEmpty(t, rendered)
			require.NotNil(t, parsed)
		})
	}
}

func TestListTemplates_DBError(t *testing.T) {
	// Create a DB connection and close it to simulate error
	db, _ := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	_ = db.AutoMigrate(&models.NotificationTemplate{})

	svc := NewNotificationService(db, nil)

	// Close the underlying connection to force error
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	_, err := svc.ListTemplates()
	require.Error(t, err)
}

func TestSendExternal_DBFetchError(t *testing.T) {
	// Create a DB connection and close it to simulate error
	db, _ := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	_ = db.AutoMigrate(&models.NotificationProvider{})

	svc := NewNotificationService(db, nil)

	// Close the underlying connection to force error
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	// Should not panic, just log error and return
	svc.SendExternal(context.Background(), "test", "Title", "Message", nil)
}

func TestSendExternal_JSONPayloadError(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	// Create a provider that will fail JSON validation (discord without content/embeds)
	provider := models.NotificationProvider{
		Name:             "json-error",
		Type:             "discord",
		URL:              "http://localhost:8080/webhook",
		Enabled:          true,
		NotifyProxyHosts: true,
		Template:         "custom",
		Config:           `{"invalid": {{toJSON .Message}}}`, // Discord requires content or embeds
	}
	require.NoError(t, db.Create(&provider).Error)

	// Should not panic, just log error
	svc.SendExternal(context.Background(), "proxy_host", "Test", "Message", nil)
	time.Sleep(100 * time.Millisecond)
}

func TestNotificationService_EnsureNotifyOnlyProviderMigration(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)
	ctx := context.Background()

	// Create test providers: discord (supported) and others (deprecated in discord-only rollout)
	providers := []models.NotificationProvider{
		{
			Name:    "Webhook Provider",
			Type:    "webhook",
			URL:     "https://discord.com/api/webhooks/123/abc/webhook",
			Enabled: true,
		},
		{
			Name:    "Discord Provider",
			Type:    "discord",
			URL:     "https://discord.com/api/webhooks/123/token",
			Enabled: true,
		},
		{
			Name:    "Telegram Provider (deprecated)",
			Type:    "telegram",
			URL:     "telegram://token@telegram?chats=123",
			Enabled: true,
		},
		{
			Name:    "Legacy SMS Provider (deprecated)",
			Type:    "legacy_sms",
			URL:     "sms://token@user",
			Enabled: true,
		},
		{
			Name:    "Gotify Provider",
			Type:    "gotify",
			URL:     "https://discord.com/api/webhooks/123/abc/gotify",
			Enabled: true,
		},
		{
			Name:    "Pushover Provider",
			Type:    "pushover",
			Token:   "pushover-api-token",
			URL:     "pushover-user-key",
			Enabled: true,
		},
	}

	for i := range providers {
		require.NoError(t, db.Create(&providers[i]).Error)
	}

	// Run migration
	err := svc.EnsureNotifyOnlyProviderMigration(ctx)
	require.NoError(t, err)

	// Verify Discord provider is marked as migrated
	var discord models.NotificationProvider
	require.NoError(t, db.Where("type = ?", "discord").First(&discord).Error)
	assert.Equal(t, "notify_v1", discord.Engine)
	assert.Equal(t, "migrated", discord.MigrationState)
	assert.Equal(t, "", discord.MigrationError)
	assert.NotNil(t, discord.LastMigratedAt)
	assert.True(t, discord.Enabled, "discord provider should remain enabled")

	// Verify non-Discord providers are marked as deprecated and disabled
	nonDiscordTypes := []string{"webhook", "telegram", "legacy_sms", "gotify", "pushover"}
	for _, providerType := range nonDiscordTypes {
		var provider models.NotificationProvider
		require.NoError(t, db.Where("type = ?", providerType).First(&provider).Error)
		assert.Equal(t, "deprecated", provider.MigrationState, "%s should be deprecated", providerType)
		assert.Contains(t, provider.MigrationError, "provider type not supported in discord-only rollout",
			"%s should have correct error message", providerType)
		assert.NotNil(t, provider.LastMigratedAt, "%s should have migration timestamp", providerType)
		assert.False(t, provider.Enabled, "%s should be disabled", providerType)
	}
}

func TestNotificationService_EnsureNotifyOnlyProviderMigration_PreservesLegacyURL(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)
	ctx := context.Background()

	// Create provider with URL but no legacy_url
	provider := models.NotificationProvider{
		Name:    "Test Provider",
		Type:    "webhook",
		URL:     "http://old-url.com/webhook",
		Enabled: true,
	}
	require.NoError(t, db.Create(&provider).Error)

	// Run migration
	err := svc.EnsureNotifyOnlyProviderMigration(ctx)
	require.NoError(t, err)

	// Verify legacy_url is preserved
	var updated models.NotificationProvider
	require.NoError(t, db.First(&updated, "id = ?", provider.ID).Error)
	assert.Equal(t, "http://old-url.com/webhook", updated.LegacyURL)
}

func TestNotificationService_EnsureNotifyOnlyProviderMigration_SkipsIfLegacyURLExists(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)
	ctx := context.Background()

	// Create provider with both URL and legacy_url already set
	provider := models.NotificationProvider{
		Name:      "Test Provider",
		Type:      "webhook",
		URL:       "http://new-url.com/webhook",
		LegacyURL: "http://original-url.com/webhook",
		Enabled:   true,
	}
	require.NoError(t, db.Create(&provider).Error)

	// Run migration
	err := svc.EnsureNotifyOnlyProviderMigration(ctx)
	require.NoError(t, err)

	// Verify legacy_url is NOT overwritten
	var updated models.NotificationProvider
	require.NoError(t, db.First(&updated, "id = ?", provider.ID).Error)
	assert.Equal(t, "http://original-url.com/webhook", updated.LegacyURL, "existing legacy_url should be preserved")
}

func TestNotificationService_EnsureNotifyOnlyProviderMigration_DBError(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	_ = db.AutoMigrate(&models.NotificationProvider{})
	svc := NewNotificationService(db, nil)

	// Close DB to force error
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	err := svc.EnsureNotifyOnlyProviderMigration(context.Background())
	require.Error(t, err)
	// Error message varies by GORM/SQLite version, just check it's an error
}

// TestNotificationService_EnsureNotifyOnlyProviderMigration_FailsClosed verifies that the migration
// function returns an error with provider context when an update fails. This is a code inspection test
// since simulating a DB update failure without also failing the fetch is non-trivial with SQLite.
//
// The implementation now:
// 1. Returns error immediately on update failure (fail-closed)
// 2. Includes provider ID, name, and type in error message
// 3. Does NOT log-and-continue on update errors
//
// Success path is tested by TestNotificationService_EnsureNotifyOnlyProviderMigration
func TestNotificationService_EnsureNotifyOnlyProviderMigration_FailsClosed(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)
	ctx := context.Background()

	// Create a Discord provider (the only type that gets migrated)
	provider := models.NotificationProvider{
		Name:    "Discord Provider",
		Type:    "discord",
		URL:     "https://discord.com/api/webhooks/123/abc",
		Enabled: true,
	}
	require.NoError(t, db.Create(&provider).Error)

	// Verify migration succeeds normally
	err := svc.EnsureNotifyOnlyProviderMigration(ctx)
	require.NoError(t, err)

	// Verify Discord provider was updated to migrated state
	var updated models.NotificationProvider
	require.NoError(t, db.First(&updated, "id = ?", provider.ID).Error)
	assert.Equal(t, "migrated", updated.MigrationState)
	assert.Equal(t, "notify_v1", updated.Engine)

	// Code inspection confirms:
	// - If update fails, function returns: fmt.Errorf("failed to migrate notification provider (id=%s, name=%q, type=%q): %w", ...)
	// - No log-and-continue pattern present
	// - Boot will treat migration incompleteness as failure
}

func TestIsDispatchEnabled_GotifyDefaultTrue(t *testing.T) {
	db := setupNotificationTestDB(t)
	_ = db.AutoMigrate(&models.Setting{})
	svc := NewNotificationService(db, nil)

	// No feature flag row exists — should default to true
	assert.True(t, svc.isDispatchEnabled("gotify"))
}

func TestIsDispatchEnabled_WebhookDefaultTrue(t *testing.T) {
	db := setupNotificationTestDB(t)
	_ = db.AutoMigrate(&models.Setting{})
	svc := NewNotificationService(db, nil)

	// No feature flag row exists — should default to true
	assert.True(t, svc.isDispatchEnabled("webhook"))
}

func TestFlagEmailServiceEnabled_ConstantValue(t *testing.T) {
	assert.Equal(t, "feature.notifications.service.email.enabled", FlagEmailServiceEnabled)
}

func TestIsSupportedNotificationProviderType_Email(t *testing.T) {
	assert.True(t, isSupportedNotificationProviderType("email"))
}

func TestIsDispatchEnabled_EmailDefaultFalse(t *testing.T) {
	db := setupNotificationTestDB(t)
	_ = db.AutoMigrate(&models.Setting{})
	svc := NewNotificationService(db, nil)

	// No feature flag row — email defaults to false
	assert.False(t, svc.isDispatchEnabled("email"))

	// Explicitly set flag to true — should now return true
	require.NoError(t, db.Create(&models.Setting{
		Key:   FlagEmailServiceEnabled,
		Value: "true",
	}).Error)
	assert.True(t, svc.isDispatchEnabled("email"))
}

// TestSendExternal_EmailProvider_NilMailService_DoesNotPanic verifies that when an
// email provider is enabled but the mail service is nil, SendExternal dispatches
// the goroutine which early-returns without panicking. The type == "email" branch
// calls dispatchEmailViaNotify directly and continues — it never reaches
// supportsJSONTemplates, which only gates the non-email dispatch goroutine.
func TestSendExternal_EmailProvider_NilMailService_DoesNotPanic(t *testing.T) {
	db := setupNotificationTestDB(t)
	_ = db.AutoMigrate(&models.Setting{})
	svc := NewNotificationService(db, nil)

	// Enable the email feature flag so isDispatchEnabled("email") returns true.
	require.NoError(t, db.Create(&models.Setting{
		Key:   FlagEmailServiceEnabled,
		Value: "true",
	}).Error)

	provider := models.NotificationProvider{
		Name:             "Email Provider",
		Type:             "email",
		URL:              "recipient@example.com",
		Enabled:          true,
		NotifyProxyHosts: true,
	}
	require.NoError(t, db.Create(&provider).Error)

	// Must not panic; goroutine hits supportsJSONTemplates("email") == false → Warn → return.
	svc.SendExternal(context.Background(), "proxy_host", "Title", "Message", nil)
	time.Sleep(50 * time.Millisecond)
}

// TestTestProvider_EmailRejectsJSONTemplateStep covers the TestProvider branch where
// a supported-but-non-JSON-template type (email) returns a clear error rather than
// attempting an HTTP send.
func TestTestProvider_EmailRejectsJSONTemplateStep(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	provider := models.NotificationProvider{
		Type:     "email",
		URL:      "recipient@example.com",
		Template: "minimal",
	}

	err := svc.TestProvider(provider)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support JSON templates")
}

func TestTestProvider_GotifyWorksWithoutFeatureFlag(t *testing.T) {
	// Gotify is cut over to the extracted notify module, whose transport
	// wrapper gates plain-HTTP/localhost dispatch on CHARON_ENV=test
	// explicitly (resolveNotifyAllowHTTP in notify_client_adapter.go)
	// rather than the old implicit os.Args[0]-".test"-suffix detection.
	t.Setenv("CHARON_ENV", "test")

	db := setupNotificationTestDB(t)
	_ = db.AutoMigrate(&models.Setting{})
	svc := NewNotificationService(db, nil)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	provider := models.NotificationProvider{
		Type:     "gotify",
		URL:      ts.URL + "/message",
		Template: "minimal",
	}

	err := svc.TestProvider(provider)
	assert.NoError(t, err)
}

func TestTestProvider_WebhookWorksWithoutFeatureFlag(t *testing.T) {
	// See TestTestProvider_GotifyWorksWithoutFeatureFlag's comment: webhook
	// is also cut over to the extracted notify module.
	t.Setenv("CHARON_ENV", "test")

	db := setupNotificationTestDB(t)
	_ = db.AutoMigrate(&models.Setting{})
	svc := NewNotificationService(db, nil)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	provider := models.NotificationProvider{
		Type:     "webhook",
		URL:      ts.URL + "/webhook",
		Template: "minimal",
	}

	err := svc.TestProvider(provider)
	assert.NoError(t, err)
}

func TestTestProvider_GotifyWorksWhenFlagExplicitlyFalse(t *testing.T) {
	// See TestTestProvider_GotifyWorksWithoutFeatureFlag's comment.
	t.Setenv("CHARON_ENV", "test")

	db := setupNotificationTestDB(t)
	_ = db.AutoMigrate(&models.Setting{})
	svc := NewNotificationService(db, nil)

	// Explicitly set feature flag to false
	db.Create(&models.Setting{Key: "feature.notifications.service.gotify.enabled", Value: "false"})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	provider := models.NotificationProvider{
		Type:     "gotify",
		URL:      ts.URL + "/message",
		Template: "minimal",
	}

	// TestProvider bypasses the dispatch gate, so even with flag=false it should work
	err := svc.TestProvider(provider)
	assert.NoError(t, err)
}

func TestTestProvider_WebhookWorksWhenFlagExplicitlyFalse(t *testing.T) {
	// See TestTestProvider_GotifyWorksWithoutFeatureFlag's comment: webhook
	// is also cut over to the extracted notify module.
	t.Setenv("CHARON_ENV", "test")

	db := setupNotificationTestDB(t)
	_ = db.AutoMigrate(&models.Setting{})
	svc := NewNotificationService(db, nil)

	// Explicitly set feature flag to false
	db.Create(&models.Setting{Key: "feature.notifications.service.webhook.enabled", Value: "false"})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	provider := models.NotificationProvider{
		Type:     "webhook",
		URL:      ts.URL + "/webhook",
		Template: "minimal",
	}

	// TestProvider bypasses the dispatch gate, so even with flag=false it should work
	err := svc.TestProvider(provider)
	assert.NoError(t, err)
}

func TestUpdateProvider_TypeMutationBlocked(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	existing := models.NotificationProvider{
		ID:   "prov-type-mut",
		Type: "webhook",
		Name: "Original",
		URL:  "https://example.com/hook",
	}
	require.NoError(t, db.Create(&existing).Error)

	update := models.NotificationProvider{
		ID:   "prov-type-mut",
		Type: "discord",
		Name: "Changed",
		URL:  "https://discord.com/api/webhooks/123/abc",
	}
	err := svc.UpdateProvider(&update)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot change provider type")
}

func TestUpdateProvider_GotifyKeepsExistingToken(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	existing := models.NotificationProvider{
		ID:    "prov-gotify-token",
		Type:  "gotify",
		Name:  "My Gotify",
		URL:   "https://gotify.example.com",
		Token: "original-secret-token",
	}
	require.NoError(t, db.Create(&existing).Error)

	update := models.NotificationProvider{
		ID:    "prov-gotify-token",
		Type:  "gotify",
		Name:  "My Gotify Updated",
		URL:   "https://gotify.example.com",
		Token: "",
	}
	err := svc.UpdateProvider(&update)
	require.NoError(t, err)
	assert.Equal(t, "original-secret-token", update.Token)
}

func TestGetFeatureFlagValue_FoundSetting(t *testing.T) {
	db := setupNotificationTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	svc := NewNotificationService(db, nil)

	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"true_string", "true", true},
		{"yes_string", "yes", true},
		{"one_string", "1", true},
		{"false_string", "false", false},
		{"no_string", "no", false},
		{"zero_string", "0", false},
		{"whitespace_true", "  True  ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db.Where("key = ?", "test.flag").Delete(&models.Setting{})
			db.Create(&models.Setting{Key: "test.flag", Value: tt.value})
			result := svc.getFeatureFlagValue("test.flag", false)
			assert.Equal(t, tt.expected, result, "value=%q", tt.value)
		})
	}
}

// --- mockMailService for email dispatch/test-provider tests ---

type mockMailService struct {
	mu           sync.Mutex
	isConfigured bool
	sendEmailErr error
	calls        []mockSendEmailCall
	renderResult string
	renderErr    error
}

type mockSendEmailCall struct {
	to      []string
	subject string
	body    string
}

func (m *mockMailService) IsConfigured() bool { return m.isConfigured }

func (m *mockMailService) SendEmail(_ context.Context, to []string, subject, htmlBody string) error {
	m.mu.Lock()
	m.calls = append(m.calls, mockSendEmailCall{to: to, subject: subject, body: htmlBody})
	m.mu.Unlock()
	return m.sendEmailErr
}

func (m *mockMailService) RenderNotificationEmail(_ string, _ EmailTemplateData) (string, error) {
	if m.renderResult != "" || m.renderErr != nil {
		return m.renderResult, m.renderErr
	}
	return "", fmt.Errorf("template rendering not configured in mock")
}

func (m *mockMailService) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *mockMailService) firstCall() mockSendEmailCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls[0]
}

func TestSendExternal_EmailProvider_Dispatches(t *testing.T) {
	db := setupNotificationTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	// renderResult must be set so the notify-module email path's render
	// step succeeds and reaches Mailer.Send — see TestEmailProvider's doc
	// comment for why a render failure now aborts dispatch instead of
	// falling back to a generic body.
	mock := &mockMailService{isConfigured: true, renderResult: "<p>rendered</p>"}
	svc := NewNotificationService(db, mock)

	provider := models.NotificationProvider{
		Name:    "email-provider",
		Type:    "email",
		URL:     "notify@example.com",
		Enabled: true,
	}
	require.NoError(t, db.Create(&provider).Error)

	db.Create(&models.Setting{Key: FlagEmailServiceEnabled, Value: "true"})

	svc.SendExternal(context.Background(), "test", "Title", "Body", nil)

	// Allow goroutine to run
	require.Eventually(t, func() bool { return mock.callCount() > 0 }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"notify@example.com"}, mock.firstCall().to)
}

func TestSendExternal_EmailProvider_FlagDisabled(t *testing.T) {
	db := setupNotificationTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	mock := &mockMailService{isConfigured: true}
	svc := NewNotificationService(db, mock)

	provider := models.NotificationProvider{
		Name:    "email-off",
		Type:    "email",
		URL:     "notify@example.com",
		Enabled: true,
	}
	require.NoError(t, db.Create(&provider).Error)

	db.Create(&models.Setting{Key: FlagEmailServiceEnabled, Value: "false"})

	svc.SendExternal(context.Background(), "test", "Title", "Body", nil)

	time.Sleep(50 * time.Millisecond)
	assert.Zero(t, mock.callCount())
}

func TestEmailProvider_MailServiceNil(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	p := models.NotificationProvider{Name: "test-email", Type: "email", URL: "a@b.com"}
	err := svc.TestEmailProvider(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email service is not configured")
}

func TestEmailProvider_MailServiceNotConfigured(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: false}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", Type: "email", URL: "a@b.com"}
	err := svc.TestEmailProvider(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email service is not configured")
}

func TestEmailProvider_EmptyURL(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: true}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", Type: "email", URL: ""}
	err := svc.TestEmailProvider(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no recipients configured")
	assert.Zero(t, mock.callCount())
}

func TestEmailProvider_BlankWhitespaceURL(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: true}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", Type: "email", URL: "  ,  ,  "}
	err := svc.TestEmailProvider(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no recipients configured")
}

func TestEmailProvider_ValidRecipient(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: true, renderResult: "<p>rendered</p>"}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", Type: "email", URL: "user@example.com"}
	err := svc.TestEmailProvider(p)
	require.NoError(t, err)
	require.Equal(t, 1, mock.callCount())
	call := mock.firstCall()
	assert.Equal(t, []string{"user@example.com"}, call.to)
	assert.Equal(t, "[Charon Test] Test Notification", call.subject)
}

func TestEmailProvider_MultipleRecipients(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: true, renderResult: "<p>rendered</p>"}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", Type: "email", URL: "a@b.com, c@d.com , e@f.com"}
	err := svc.TestEmailProvider(p)
	require.NoError(t, err)
	require.Equal(t, 1, mock.callCount())
	assert.Equal(t, []string{"a@b.com", "c@d.com", "e@f.com"}, mock.firstCall().to)
}

func TestEmailProvider_SendError(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: true, renderResult: "<p>rendered</p>", sendEmailErr: fmt.Errorf("smtp: connection refused")}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", Type: "email", URL: "a@b.com"}
	err := svc.TestEmailProvider(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "smtp")
	assert.Equal(t, 1, mock.callCount())
}

// TestEmailProvider_TemplateFallback verifies that a template rendering
// failure falls back to a manually built plain HTML body and the email is
// still sent — matching pre-extraction dispatchEmail behavior. The fallback
// now lives in mailServiceTemplateRendererAdapter.Render
// (notify_email_adapter.go), which never propagates a render error to
// email.Client.Send; it logs a warning and returns a degraded-but-nonempty
// body instead, so Send proceeds to Mailer.Send as normal.
func TestEmailProvider_TemplateFallback(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: true, renderErr: fmt.Errorf("template not found")}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", Type: "email", URL: "a@b.com"}
	err := svc.TestEmailProvider(p)
	require.NoError(t, err, "email must still be sent when template rendering fails")
	require.Equal(t, 1, mock.callCount(), "SendEmail must still be called with a fallback body")

	call := mock.firstCall()
	assert.Contains(t, call.body, "<strong>Test Notification</strong>")
	assert.Contains(t, call.body, "This is a test notification from Charon. If you received this email, your email notification provider is configured correctly.")
}

// TestEmailProvider_TransportFailureStillErrors confirms that a genuine
// Mailer/SMTP transport failure (as opposed to a template-render failure)
// still correctly propagates as an error from TestEmailProvider, and is not
// silently swallowed by the template-render fallback added to
// mailServiceTemplateRendererAdapter.Render. Template rendering succeeds
// here — only SendEmail (the transport step) fails.
func TestEmailProvider_TransportFailureStillErrors(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{
		isConfigured: true,
		renderResult: "<p>rendered</p>",
		sendEmailErr: fmt.Errorf("smtp: connection refused"),
	}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", Type: "email", URL: "a@b.com"}
	err := svc.TestEmailProvider(p)
	require.Error(t, err, "a real transport failure must still be reported as an error")
	assert.Contains(t, err.Error(), "smtp")
	require.Equal(t, 1, mock.callCount())
	assert.Equal(t, "<p>rendered</p>", mock.firstCall().body, "transport failure must not be confused with a render failure")
}

func TestEmailProvider_UsesRenderedTemplate(t *testing.T) {
	db := setupNotificationTestDB(t)
	rendered := "<html><body>Rendered test email</body></html>"
	mock := &mockMailService{isConfigured: true, renderResult: rendered}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", Type: "email", URL: "a@b.com"}
	err := svc.TestEmailProvider(p)
	require.NoError(t, err)
	require.Equal(t, 1, mock.callCount())
	assert.Equal(t, rendered, mock.firstCall().body)
}

func TestIsDispatchEnabled_TelegramDefaultTrue(t *testing.T) {
	db := setupNotificationTestDB(t)
	_ = db.AutoMigrate(&models.Setting{})
	svc := NewNotificationService(db, nil)

	// No feature flag row exists — telegram defaults to true
	assert.True(t, svc.isDispatchEnabled("telegram"))
}

func TestIsDispatchEnabled_TelegramDisabledByFlag(t *testing.T) {
	db := setupNotificationTestDB(t)
	_ = db.AutoMigrate(&models.Setting{})
	svc := NewNotificationService(db, nil)

	// Explicitly disable telegram via feature flag
	db.Create(&models.Setting{Key: "feature.notifications.service.telegram.enabled", Value: "false"})
	assert.False(t, svc.isDispatchEnabled("telegram"))
}

// TestNotificationService_TestProvider_TelegramUsesNotifyPath verifies
// Telegram TestProvider dispatch after cutover to the extracted notify
// module (buildNotifySender). buildNotifySender leaves telegram.Config's
// BaseURL empty (notify_provider_adapter.go), so — unlike the old
// svc.telegramAPIBaseURL test seam — dispatch always targets the real
// Telegram Bot API; a capturing fake RoundTripper (via
// WithNotifyTransportWrapper) intercepts before any real network call,
// same as the Discord/Slack/Pushover notify-path tests.
func TestNotificationService_TestProvider_TelegramUsesNotifyPath(t *testing.T) {
	db := setupNotificationTestDB(t)
	wrapper, rt := newCapturingWrapper()
	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapper))

	provider := models.NotificationProvider{ //nolint:gosec // G101: test credential
		Type:     "telegram",
		URL:      "123456789",
		Token:    "fake-bot-token",
		Template: "minimal",
	}

	err := svc.TestProvider(provider)
	require.NoError(t, err)

	req, body := rt.last()
	require.NotNil(t, req)
	assert.Equal(t, "https://api.telegram.org/botfake-bot-token/sendMessage", req.URL.String())
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	assert.Equal(t, "123456789", payload["chat_id"])
}

// TestNotificationService_SendExternal_TelegramUsesNotifyPath is the
// SendExternal counterpart of
// TestNotificationService_TestProvider_TelegramUsesNotifyPath.
func TestNotificationService_SendExternal_TelegramUsesNotifyPath(t *testing.T) {
	db := setupNotificationTestDB(t)
	wrapper, rt := newCapturingWrapper()
	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapper))

	provider := models.NotificationProvider{ //nolint:gosec // G101: test credential
		Name:             "Telegram E2E",
		Type:             "telegram",
		URL:              "123456789",
		Token:            "fake-bot-token",
		Enabled:          true,
		NotifyProxyHosts: true,
		Template:         "minimal",
	}
	require.NoError(t, svc.CreateProvider(&provider))

	svc.SendExternal(context.Background(), "proxy_host", "Title", "Message", nil)

	require.Eventually(t, func() bool {
		_, body := rt.last()
		return body != nil
	}, time.Second, 10*time.Millisecond, "expected telegram webhook to be sent")

	_, body := rt.last()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	assert.Equal(t, "123456789", payload["chat_id"])
}

// --- Slack Notification Provider Tests ---

func TestSlackWebhookURLValidation(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid_url", "https://hooks.slack.com/services/T00000000/B00000000/abcdefghijklmnop", false},
		{"valid_url_with_dashes", "https://hooks.slack.com/services/T0-A_z/B0-A_z/abc-def_123", false},
		{"http_scheme", "http://hooks.slack.com/services/T00000000/B00000000/abcdefghijklmnop", true},
		{"wrong_host", "https://evil.com/services/T00000000/B00000000/abcdefghijklmnop", true},
		{"ip_address", "https://192.168.1.1/services/T00000000/B00000000/abcdefghijklmnop", true},
		{"missing_T_prefix", "https://hooks.slack.com/services/X00000000/B00000000/abcdefghijklmnop", true},
		{"missing_B_prefix", "https://hooks.slack.com/services/T00000000/X00000000/abcdefghijklmnop", true},
		{"query_params", "https://hooks.slack.com/services/T00000000/B00000000/abcdefghijklmnop?token=leak", true},
		{"empty_string", "", true},
		{"just_host", "https://hooks.slack.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSlackWebhookURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSlackWebhookURLValidation_RejectsHTTP(t *testing.T) {
	err := validateSlackWebhookURL("http://hooks.slack.com/services/T00000/B00000/token123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Slack webhook URL")
}

func TestSlackWebhookURLValidation_RejectsIPAddress(t *testing.T) {
	err := validateSlackWebhookURL("https://192.168.1.1/services/T00000/B00000/token123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Slack webhook URL")
}

func TestSlackWebhookURLValidation_RejectsWrongHost(t *testing.T) {
	err := validateSlackWebhookURL("https://evil.com/services/T00000/B00000/token123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Slack webhook URL")
}

func TestSlackWebhookURLValidation_RejectsQueryParams(t *testing.T) {
	err := validateSlackWebhookURL("https://hooks.slack.com/services/T00000/B00000/token123?token=leak")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Slack webhook URL")
}

func TestNotificationService_CreateProvider_Slack(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	provider := &models.NotificationProvider{ //nolint:gosec // G101: test credential
		Name:  "Slack Alerts",
		Type:  "slack",
		URL:   "#alerts",
		Token: "https://hooks.slack.com/services/T00000/B00000/xxxx",
	}
	err := svc.CreateProvider(provider)
	require.NoError(t, err)

	var saved models.NotificationProvider
	require.NoError(t, db.Where("id = ?", provider.ID).First(&saved).Error)
	assert.Equal(t, "https://hooks.slack.com/services/T00000/B00000/xxxx", saved.Token)
	assert.Equal(t, "#alerts", saved.URL)
	assert.Equal(t, "slack", saved.Type)
}

func TestNotificationService_CreateProvider_Slack_ClearsTokenField(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	provider := &models.NotificationProvider{
		Name:  "Webhook Test",
		Type:  "webhook",
		URL:   "https://example.com/hook",
		Token: "should-be-cleared",
	}
	err := svc.CreateProvider(provider)
	require.NoError(t, err)

	var saved models.NotificationProvider
	require.NoError(t, db.Where("id = ?", provider.ID).First(&saved).Error)
	assert.Empty(t, saved.Token)
}

func TestNotificationService_UpdateProvider_Slack_PreservesToken(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	existing := models.NotificationProvider{ //nolint:gosec // G101: test credential
		ID:    "prov-slack-token",
		Type:  "slack",
		Name:  "Slack Alerts",
		URL:   "#alerts",
		Token: "https://hooks.slack.com/services/T00000/B00000/xxxx",
	}
	require.NoError(t, db.Create(&existing).Error)

	update := models.NotificationProvider{
		ID:    "prov-slack-token",
		Type:  "slack",
		Name:  "Slack Alerts Updated",
		URL:   "#general",
		Token: "",
	}
	err := svc.UpdateProvider(&update)
	require.NoError(t, err)
	assert.Equal(t, "https://hooks.slack.com/services/T00000/B00000/xxxx", update.Token)
}

// TestNotificationService_TestProvider_Slack verifies TestProvider dispatch
// after Slack's cutover to the extracted notify module
// (buildNotifySender). Slack's own webhook validation
// (providers/slack.ValidateWebhookURL) only accepts hooks.slack.com URLs
// matching the standard incoming-webhook shape — the same shape Charon's
// own validateSlackWebhookURL already enforced — so an httptest.Server URL
// (as used before cutover, via a WithSlackURLValidator override that only
// gated the old service-level check) can no longer stand in for a Slack
// webhook. A capturing fake RoundTripper via WithNotifyTransportWrapper
// replaces it, matching the pattern used for Discord's tests.
func TestNotificationService_TestProvider_Slack(t *testing.T) {
	db := setupNotificationTestDB(t)
	wrapper, rt := newCapturingWrapper()
	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapper))

	provider := models.NotificationProvider{ //nolint:gosec // G101: test credential
		Type:     "slack",
		URL:      "#test",
		Token:    "https://hooks.slack.com/services/T000/B000/xxxxxxxxxxxxxxxxxxxxxxxx",
		Template: "minimal",
	}

	err := svc.TestProvider(provider)
	require.NoError(t, err)

	_, body := rt.last()
	require.NotNil(t, body)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	assert.NotEmpty(t, payload["text"])
}

// TestNotificationService_SendExternal_Slack is the SendExternal
// counterpart of TestNotificationService_TestProvider_Slack — see its
// comment for why a capturing fake RoundTripper replaces the old
// httptest.Server.
func TestNotificationService_SendExternal_Slack(t *testing.T) {
	db := setupNotificationTestDB(t)
	_ = db.AutoMigrate(&models.Setting{})

	wrapper, rt := newCapturingWrapper()
	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapper))

	provider := models.NotificationProvider{ //nolint:gosec // G101: test credential
		Name:             "Slack E2E",
		Type:             "slack",
		URL:              "#alerts",
		Token:            "https://hooks.slack.com/services/T000/B000/xxxxxxxxxxxxxxxxxxxxxxxx",
		Enabled:          true,
		NotifyProxyHosts: true,
		Template:         "minimal",
	}
	require.NoError(t, svc.CreateProvider(&provider))

	svc.SendExternal(context.Background(), "proxy_host", "Title", "Message", nil)

	require.Eventually(t, func() bool {
		_, body := rt.last()
		return body != nil
	}, 2*time.Second, 10*time.Millisecond, "Timed out waiting for slack webhook")

	_, body := rt.last()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	assert.NotEmpty(t, payload["text"])
}

func TestFlagSlackServiceEnabled_ConstantValue(t *testing.T) {
	assert.Equal(t, "feature.notifications.service.slack.enabled", FlagSlackServiceEnabled)
}

func TestNotificationService_Slack_IsDispatchEnabled(t *testing.T) {
	db := setupNotificationTestDB(t)
	_ = db.AutoMigrate(&models.Setting{})
	svc := NewNotificationService(db, nil)

	assert.True(t, svc.isDispatchEnabled("slack"))

	db.Create(&models.Setting{Key: "feature.notifications.service.slack.enabled", Value: "false"})
	assert.False(t, svc.isDispatchEnabled("slack"))
}

func TestNotificationService_Slack_TokenNotExposedInList(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	provider := &models.NotificationProvider{ //nolint:gosec // G101: test credential
		Name:  "Slack Secret",
		Type:  "slack",
		URL:   "#secret",
		Token: "https://hooks.slack.com/services/T00000/B00000/secrettoken",
	}
	require.NoError(t, svc.CreateProvider(provider))

	providers, err := svc.ListProviders()
	require.NoError(t, err)
	require.Len(t, providers, 1)

	providers[0].HasToken = providers[0].Token != ""
	providers[0].Token = ""
	assert.True(t, providers[0].HasToken)
	assert.Empty(t, providers[0].Token)
}

func TestCreateProvider_Slack_EmptyTokenRejected(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	provider := &models.NotificationProvider{
		Name:  "Slack Missing Token",
		Type:  "slack",
		URL:   "#alerts",
		Token: "",
	}
	err := svc.CreateProvider(provider)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slack webhook URL is required")
}

func TestCreateProvider_Slack_WhitespaceOnlyTokenRejected(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	provider := &models.NotificationProvider{
		Name:  "Slack Whitespace Token",
		Type:  "slack",
		URL:   "#alerts",
		Token: "   ",
	}
	err := svc.CreateProvider(provider)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slack webhook URL is required")
}

func TestCreateProvider_Slack_InvalidTokenRejected(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	provider := &models.NotificationProvider{ //nolint:gosec // G101: test credential
		Name:  "Slack Bad Token",
		Type:  "slack",
		URL:   "#alerts",
		Token: "https://evil.com/not-a-slack-webhook",
	}
	err := svc.CreateProvider(provider)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Slack webhook URL")
}

func TestUpdateProvider_Slack_InvalidNewTokenRejected(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	existing := models.NotificationProvider{ //nolint:gosec // G101: test credential
		ID:    "prov-slack-update-invalid",
		Type:  "slack",
		Name:  "Slack Alerts",
		URL:   "#alerts",
		Token: "https://hooks.slack.com/services/T00000/B00000/xxxx",
	}
	require.NoError(t, db.Create(&existing).Error)

	update := models.NotificationProvider{ //nolint:gosec // G101: test credential
		ID:    "prov-slack-update-invalid",
		Type:  "slack",
		Name:  "Slack Alerts",
		URL:   "#alerts",
		Token: "https://evil.com/not-a-slack-webhook",
	}
	err := svc.UpdateProvider(&update)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Slack webhook URL")
}

func TestUpdateProvider_Slack_UnchangedTokenSkipsValidation(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	existing := models.NotificationProvider{ //nolint:gosec // G101: test credential
		ID:    "prov-slack-update-unchanged",
		Type:  "slack",
		Name:  "Slack Alerts",
		URL:   "#alerts",
		Token: "https://hooks.slack.com/services/T00000/B00000/xxxx",
	}
	require.NoError(t, db.Create(&existing).Error)

	// Submitting empty token causes fallback to existing — should not re-validate
	update := models.NotificationProvider{
		ID:    "prov-slack-update-unchanged",
		Type:  "slack",
		Name:  "Slack Alerts Renamed",
		URL:   "#general",
		Token: "",
	}
	err := svc.UpdateProvider(&update)
	require.NoError(t, err)
}

// --- Pushover Notification Provider Tests ---

func TestPushoverDispatch_FeatureFlagDisabled(t *testing.T) {
	db := setupNotificationTestDB(t)
	_ = db.AutoMigrate(&models.Setting{})
	db.Create(&models.Setting{Key: "feature.notifications.service.pushover.enabled", Value: "false"})
	svc := NewNotificationService(db, nil)

	assert.False(t, svc.isDispatchEnabled("pushover"))
}

func TestIsDispatchEnabled_PushoverDefaultTrue(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	// No flag in DB — should default to true (enabled)
	assert.True(t, svc.isDispatchEnabled("pushover"))
}

func TestIsDispatchEnabled_PushoverDisabledByFlag(t *testing.T) {
	db := setupNotificationTestDB(t)
	_ = db.AutoMigrate(&models.Setting{})
	db.Create(&models.Setting{Key: "feature.notifications.service.pushover.enabled", Value: "false"})
	svc := NewNotificationService(db, nil)

	assert.False(t, svc.isDispatchEnabled("pushover"))
}

// TestNotificationService_TestProvider_PushoverUsesNotifyPath verifies
// Pushover TestProvider dispatch after cutover to the extracted notify
// module (buildNotifySender). buildNotifySender leaves pushover.Config's
// BaseURL empty (notify_provider_adapter.go), so — unlike the old
// svc.pushoverAPIBaseURL test seam — dispatch always targets Pushover's
// real production API; a capturing fake RoundTripper (via
// WithNotifyTransportWrapper) intercepts before any real network call,
// same as the Discord/Slack notify-path tests.
func TestNotificationService_TestProvider_PushoverUsesNotifyPath(t *testing.T) {
	db := setupNotificationTestDB(t)
	wrapper, rt := newCapturingWrapper()
	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapper))

	provider := models.NotificationProvider{
		Type:     "pushover",
		Token:    "app-token-abc",
		URL:      "user-key-xyz",
		Template: "minimal",
	}

	err := svc.TestProvider(provider)
	require.NoError(t, err)

	req, body := rt.last()
	require.NotNil(t, req)
	assert.Equal(t, "https://api.pushover.net/1/messages.json", req.URL.String())
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	assert.Equal(t, "app-token-abc", payload["token"])
	assert.Equal(t, "user-key-xyz", payload["user"])
}

// TestNotificationService_SendExternal_PushoverUsesNotifyPath is the
// SendExternal counterpart of
// TestNotificationService_TestProvider_PushoverUsesNotifyPath.
func TestNotificationService_SendExternal_PushoverUsesNotifyPath(t *testing.T) {
	db := setupNotificationTestDB(t)
	wrapper, rt := newCapturingWrapper()
	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(wrapper))

	provider := models.NotificationProvider{
		Name:             "Pushover E2E",
		Type:             "pushover",
		Token:            "app-token-abc",
		URL:              "user-key-xyz",
		Enabled:          true,
		NotifyProxyHosts: true,
		Template:         "minimal",
	}
	require.NoError(t, svc.CreateProvider(&provider))

	svc.SendExternal(context.Background(), "proxy_host", "Title", "Message", nil)

	require.Eventually(t, func() bool {
		_, body := rt.last()
		return body != nil
	}, time.Second, 10*time.Millisecond, "expected pushover webhook to be sent")

	_, body := rt.last()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	assert.Equal(t, "app-token-abc", payload["token"])
	assert.Equal(t, "user-key-xyz", payload["user"])
}

func TestIsSupportedNotificationProviderType_Ntfy(t *testing.T) {
	assert.True(t, isSupportedNotificationProviderType("ntfy"))
	assert.True(t, isSupportedNotificationProviderType("Ntfy"))
	assert.True(t, isSupportedNotificationProviderType(" ntfy "))
}

// TestNotificationService_TestProvider_NtfyUsesNotifyPath verifies Ntfy
// TestProvider dispatch after cutover to the extracted notify module
// (buildNotifySender). Unlike Discord/Slack, Ntfy has no provider-side
// hostname allowlist, so it can still dispatch to a local httptest.Server —
// but that server is plain HTTP, which the extracted module's transport
// wrapper only allows when CHARON_ENV=test is set explicitly (see
// resolveNotifyAllowHTTP in notify_client_adapter.go), replacing the old
// implicit os.Args[0]-based test-binary detection.
func TestNotificationService_TestProvider_NtfyUsesNotifyPath(t *testing.T) {
	t.Setenv("CHARON_ENV", "test")

	var capturedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	provider := models.NotificationProvider{
		Type:     "ntfy",
		URL:      server.URL,
		Token:    "ntfy-token",
		Template: "minimal",
	}

	err := svc.TestProvider(provider)
	require.NoError(t, err)
	assert.Equal(t, "Bearer ntfy-token", capturedAuth)
}

// TestNotificationService_SendExternal_NtfyUsesNotifyPath is the
// SendExternal counterpart of
// TestNotificationService_TestProvider_NtfyUsesNotifyPath.
func TestNotificationService_SendExternal_NtfyUsesNotifyPath(t *testing.T) {
	t.Setenv("CHARON_ENV", "test")

	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	provider := models.NotificationProvider{
		Name:             "Ntfy E2E",
		Type:             "ntfy",
		URL:              server.URL,
		Enabled:          true,
		NotifyProxyHosts: true,
		Template:         "minimal",
	}
	require.NoError(t, svc.CreateProvider(&provider))

	svc.SendExternal(context.Background(), "proxy_host", "Title", "Message", nil)

	select {
	case body := <-received:
		var payload map[string]any
		require.NoError(t, json.Unmarshal(body, &payload))
		assert.NotEmpty(t, payload["message"])
	case <-time.After(time.Second):
		t.Fatal("Timed out waiting for ntfy webhook")
	}
}

func TestIsDispatchEnabled_NtfyDefaultTrue(t *testing.T) {
	db := setupNotificationTestDB(t)
	_ = db.AutoMigrate(&models.Setting{})
	svc := NewNotificationService(db, nil)

	assert.True(t, svc.isDispatchEnabled("ntfy"))
}

func TestIsDispatchEnabled_NtfyDisabledByFlag(t *testing.T) {
	db := setupNotificationTestDB(t)
	_ = db.AutoMigrate(&models.Setting{})
	db.Create(&models.Setting{Key: "feature.notifications.service.ntfy.enabled", Value: "false"})
	svc := NewNotificationService(db, nil)

	assert.False(t, svc.isDispatchEnabled("ntfy"))
}

func TestSupportsJSONTemplates_Ntfy(t *testing.T) {
	assert.True(t, supportsJSONTemplates("ntfy"))
	assert.True(t, supportsJSONTemplates("Ntfy"))
}
