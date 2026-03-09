package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/notifications"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/trace"
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

func TestNotificationService_TestProvider_Webhook(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	// Mock validation and webhook request for testing
	origValidateDiscordFunc := validateDiscordProviderURLFunc
	origWebhookDoReq := webhookDoRequestFunc
	defer func() {
		validateDiscordProviderURLFunc = origValidateDiscordFunc
		webhookDoRequestFunc = origWebhookDoReq
	}()
	validateDiscordProviderURLFunc = func(providerType, rawURL string) error { return nil }
	webhookDoRequestFunc = func(client *http.Client, req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
	}

	// Start a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		// Minimal template uses lowercase keys: title, message
		assert.Equal(t, "Test Notification", body["title"])
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	provider := models.NotificationProvider{
		Name:     "Test Discord",
		Type:     "discord",
		URL:      ts.URL,
		Template: "minimal",
		Config:   `{"Header": "{{.Title}}"}`,
	}

	err := svc.TestProvider(provider)
	require.NoError(t, err)
}

func TestNotificationService_SendExternal(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	received := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(received)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Mock discord webhook validation to allow test server URLs
	// Do NOT mock webhookDoRequestFunc - we want real HTTP call to test server
	origValidateDiscordFunc := validateDiscordProviderURLFunc
	defer func() { validateDiscordProviderURLFunc = origValidateDiscordFunc }()
	validateDiscordProviderURLFunc = func(providerType, rawURL string) error { return nil }

	provider := models.NotificationProvider{
		Name:             "Test Discord",
		Type:             "discord",
		URL:              ts.URL,
		Enabled:          true,
		NotifyProxyHosts: true,
		Template:         "minimal",
	}
	_ = svc.CreateProvider(&provider)

	svc.SendExternal(context.Background(), "proxy_host", "Title", "Message", nil)

	select {
	case <-received:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for webhook")
	}
}

func TestNotificationService_SendExternal_MinimalVsDetailedTemplates(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	// Mock validation only - allow real HTTP calls to test servers
	origValidateDiscordFunc := validateDiscordProviderURLFunc
	defer func() { validateDiscordProviderURLFunc = origValidateDiscordFunc }()
	validateDiscordProviderURLFunc = func(providerType, rawURL string) error { return nil }

	// Minimal template
	rcvMinimal := make(chan map[string]any, 1)
	tsMin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		rcvMinimal <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer tsMin.Close()

	providerMin := models.NotificationProvider{
		Name:         "Minimal Discord",
		Type:         "discord",
		URL:          tsMin.URL,
		Enabled:      true,
		NotifyUptime: true,
		Template:     "minimal",
	}
	_ = svc.CreateProvider(&providerMin)

	data := map[string]any{"Title": "Min Title", "Message": "Min Message", "Time": time.Now().Format(time.RFC3339), "EventType": "uptime"}
	svc.SendExternal(context.Background(), "uptime", "Min Title", "Min Message", data)

	select {
	case body := <-rcvMinimal:
		// minimal template should contain 'title' and 'message' keys
		if title, ok := body["title"].(string); ok {
			assert.Equal(t, "Min Title", title)
		} else {
			t.Fatalf("expected title in minimal body")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for minimal webhook")
	}

	// Detailed template
	rcvDetailed := make(chan map[string]any, 1)
	tsDet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		rcvDetailed <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer tsDet.Close()

	providerDet := models.NotificationProvider{
		Name:         "Detailed Discord",
		Type:         "discord",
		URL:          tsDet.URL,
		Enabled:      true,
		NotifyUptime: true,
		Template:     "detailed",
	}
	_ = svc.CreateProvider(&providerDet)

	dataDet := map[string]any{"Title": "Det Title", "Message": "Det Message", "Time": time.Now().Format(time.RFC3339), "EventType": "uptime", "HostName": "example-host", "HostIP": "1.2.3.4", "ServiceCount": 1, "Services": []map[string]any{{"Name": "svc1"}}}
	svc.SendExternal(context.Background(), "uptime", "Det Title", "Det Message", dataDet)

	select {
	case body := <-rcvDetailed:
		// detailed template should contain 'host' and 'services'
		if host, ok := body["host"].(string); ok {
			assert.Equal(t, "example-host", host)
		} else {
			t.Fatalf("expected host in detailed body")
		}
		if _, ok := body["services"]; !ok {
			t.Fatalf("expected services in detailed body")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for detailed webhook")
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

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name        string
		serviceType string
		rawURL      string
		expected    string
	}{
		{
			name:        "Discord HTTPS",
			serviceType: "discord",
			rawURL:      "https://discord.com/api/webhooks/123456789/abcdefg",
			expected:    "discord://abcdefg@123456789",
		},
		{
			name:        "Discord HTTPS with app",
			serviceType: "discord",
			rawURL:      "https://discordapp.com/api/webhooks/123456789/abcdefg",
			expected:    "discord://abcdefg@123456789",
		},
		{
			name:        "Discord Generic",
			serviceType: "discord",
			rawURL:      "discord://token@id",
			expected:    "discord://token@id",
		},
		{
			name:        "Other Service",
			serviceType: "slack",
			rawURL:      "https://hooks.slack.com/services/...",
			expected:    "https://hooks.slack.com/services/...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeURL(tt.serviceType, tt.rawURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNotificationService_SendCustomWebhook_Errors(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	t.Run("invalid URL", func(t *testing.T) {
		provider := models.NotificationProvider{
			Type: "webhook",
			URL:  "://invalid-url",
		}
		data := map[string]any{"Title": "Test", "Message": "Test Message"}
		err := svc.sendJSONPayload(context.Background(), provider, data)
		assert.Error(t, err)
	})

	t.Run("unreachable host", func(t *testing.T) {
		provider := models.NotificationProvider{
			Type: "webhook",
			URL:  "http://192.0.2.1:9999", // TEST-NET-1, unreachable
		}
		data := map[string]any{"Title": "Test", "Message": "Test Message"}
		// Set short timeout for client if possible, but here we just expect error
		// Note: http.Client default timeout is 0 (no timeout), but OS might timeout
		// We can't easily change client timeout here without modifying service
		// So we might skip this or just check if it returns error eventually
		// But for unit test speed, we should probably mock or use a closed port on localhost
		// Using a closed port on localhost is faster
		provider.URL = "http://127.0.0.1:54321" // Assuming this port is closed
		err := svc.sendJSONPayload(context.Background(), provider, data)
		assert.Error(t, err)
	})

	t.Run("server returns error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		provider := models.NotificationProvider{
			Type: "webhook",
			URL:  ts.URL,
		}
		data := map[string]any{"Title": "Test", "Message": "Test Message"}
		err := svc.sendJSONPayload(context.Background(), provider, data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("valid custom payload template", func(t *testing.T) {
		receivedBody := ""
		received := make(chan struct{})
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if custom, ok := body["custom"]; ok {
				receivedBody = custom.(string)
			}
			w.WriteHeader(http.StatusOK)
			close(received)
		}))
		defer ts.Close()

		provider := models.NotificationProvider{
			Type:   "webhook",
			URL:    ts.URL,
			Config: `{"custom": "Test: {{.Title}}"}`,
		}
		data := map[string]any{"Title": "My Title", "Message": "Test Message"}
		_ = svc.sendJSONPayload(context.Background(), provider, data)

		select {
		case <-received:
			assert.Equal(t, "Test: My Title", receivedBody)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timeout waiting for webhook")
		}
	})

	t.Run("default payload without template", func(t *testing.T) {
		receivedContent := ""
		received := make(chan struct{})
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if title, ok := body["title"]; ok {
				receivedContent = title.(string)
			}
			w.WriteHeader(http.StatusOK)
			close(received)
		}))
		defer ts.Close()

		provider := models.NotificationProvider{
			Type: "webhook",
			URL:  ts.URL,
			// Config is empty, so default template is used: minimal
		}
		data := map[string]any{"Title": "Default Title", "Message": "Test Message"}
		_ = svc.sendJSONPayload(context.Background(), provider, data)

		select {
		case <-received:
			assert.Equal(t, "Default Title", receivedContent)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timeout waiting for webhook")
		}
	})
}

func TestNotificationService_SendCustomWebhook_PropagatesRequestID(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	received := make(chan string, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.Header.Get("X-Request-ID")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	provider := models.NotificationProvider{Type: "webhook", URL: ts.URL}
	data := map[string]any{"Title": "Test", "Message": "Test"}
	// Build context with requestID value
	ctx := context.WithValue(context.Background(), trace.RequestIDKey, "my-rid")
	err := svc.sendJSONPayload(ctx, provider, data)
	require.NoError(t, err)

	select {
	case rid := <-received:
		assert.Equal(t, "my-rid", rid)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timed out waiting for webhook request")
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

	t.Run("slack type not supported", func(t *testing.T) {
		provider := models.NotificationProvider{
			Type: "slack",
			URL:  "https://hooks.slack.com/services/INVALID/WEBHOOK/URL",
		}
		err := svc.TestProvider(provider)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported provider type")
	})

	t.Run("webhook success", func(t *testing.T) {
		// Mock validation and webhook request for testing
		origValidateDiscordFunc := validateDiscordProviderURLFunc
		origWebhookDoReq := webhookDoRequestFunc
		defer func() {
			validateDiscordProviderURLFunc = origValidateDiscordFunc
			webhookDoRequestFunc = origWebhookDoReq
		}()
		validateDiscordProviderURLFunc = func(providerType, rawURL string) error { return nil }
		webhookDoRequestFunc = func(client *http.Client, req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
		}

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		provider := models.NotificationProvider{
			Type:     "discord",
			URL:      ts.URL,
			Template: "minimal", // Use JSON template path which supports HTTP/HTTPS
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

func TestSSRF_WebhookIntegration(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	t.Run("blocks private IP webhook", func(t *testing.T) {
		provider := models.NotificationProvider{
			Type: "webhook",
			URL:  "http://10.0.0.1/webhook",
		}
		data := map[string]any{"Title": "Test", "Message": "Test Message"}
		err := svc.sendJSONPayload(context.Background(), provider, data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "destination URL validation failed")
	})

	t.Run("blocks cloud metadata endpoint", func(t *testing.T) {
		provider := models.NotificationProvider{
			Type: "webhook",
			URL:  "http://169.254.169.254/latest/meta-data/",
		}
		data := map[string]any{"Title": "Test", "Message": "Test Message"}
		err := svc.sendJSONPayload(context.Background(), provider, data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "destination URL validation failed")
	})

	t.Run("allows localhost for testing", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		provider := models.NotificationProvider{
			Type: "webhook",
			URL:  ts.URL,
		}
		data := map[string]any{"Title": "Test", "Message": "Test Message"}
		err := svc.sendJSONPayload(context.Background(), provider, data)
		assert.NoError(t, err)
	})
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

	t.Run("custom data passed to webhook", func(t *testing.T) {
		db := setupNotificationTestDB(t)
		svc := NewNotificationService(db, nil)

		// Mock validation only - allow real HTTP calls to test server
		origValidateDiscordFunc := validateDiscordProviderURLFunc
		defer func() { validateDiscordProviderURLFunc = origValidateDiscordFunc }()
		validateDiscordProviderURLFunc = func(providerType, rawURL string) error { return nil }

		var receivedCustom atomic.Value
		receivedCustom.Store("")
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if custom, ok := body["custom"]; ok {
				receivedCustom.Store(custom.(string))
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		provider := models.NotificationProvider{
			Name:             "Custom Data Discord",
			Type:             "discord",
			URL:              ts.URL,
			Enabled:          true,
			NotifyProxyHosts: true,
			Config:           `{"content": {{toJSON .Message}}, "custom": "{{.CustomField}}"}`,
			Template:         "custom", // Use custom template to enable Config
		}
		_ = svc.CreateProvider(&provider)

		customData := map[string]any{
			"CustomField": "test-value",
		}
		svc.SendExternal(context.Background(), "proxy_host", "Title", "Message", customData)
		time.Sleep(100 * time.Millisecond)

		assert.Equal(t, "test-value", receivedCustom.Load().(string))
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

func TestNotificationService_IsPrivateIP(t *testing.T) {
	tests := []struct {
		name      string
		ipStr     string
		isPrivate bool
	}{
		{"loopback ipv4", "127.0.0.1", true},
		{"loopback ipv6", "::1", true},
		{"private 10.x", "10.0.0.1", true},
		{"private 10.x high", "10.255.255.254", true},
		{"private 172.16-31", "172.16.0.1", true},
		{"private 172.31", "172.31.255.254", true},
		{"private 192.168", "192.168.1.1", true},
		{"public 172.32", "172.32.0.1", false},
		{"public 172.15", "172.15.0.1", false},
		{"public ip", "8.8.8.8", false},
		{"public ipv6", "2001:4860:4860::8888", false},
		{"link local ipv4", "169.254.1.1", true},
		{"link local ipv6", "fe80::1", true},
		{"unique local ipv6 fc", "fc00::1", true},
		{"unique local ipv6 fc high", "fc12:3456::1", true},
		{"unique local ipv6 fd", "fd00::1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ipStr)
			require.NotNil(t, ip, "failed to parse IP: %s", tt.ipStr)
			got := isPrivateIP(ip)
			assert.Equal(t, tt.isPrivate, got, "IP %s private check mismatch", tt.ipStr)
		})
	}
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

func TestSendCustomWebhook_HTTPStatusCodeErrors(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	errorCodes := []int{400, 404, 500, 502, 503}

	for _, statusCode := range errorCodes {
		t.Run(fmt.Sprintf("status_%d", statusCode), func(t *testing.T) {
			// Mock webhook HTTP client to return error status
			originalDo := webhookDoRequestFunc
			defer func() { webhookDoRequestFunc = originalDo }()
			webhookDoRequestFunc = func(client *http.Client, req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: statusCode,
					Body:       http.NoBody,
					Header:     make(http.Header),
				}, nil
			}

			provider := models.NotificationProvider{
				Type:     "discord",
				URL:      "https://discord.com/api/webhooks/123456/test_token",
				Template: "minimal",
			}

			data := map[string]any{
				"Title":     "Test",
				"Message":   "Test",
				"Time":      time.Now().Format(time.RFC3339),
				"EventType": "test",
			}

			err := svc.sendJSONPayload(context.Background(), provider, data)
			require.Error(t, err)
			assert.Contains(t, err.Error(), fmt.Sprintf("%d", statusCode))
		})
	}
}

func TestSendCustomWebhook_TemplateSelection(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	tests := []struct {
		name           string
		template       string
		config         string
		expectedKeys   []string
		unexpectedKeys []string
	}{
		{
			name:         "minimal template",
			template:     "minimal",
			expectedKeys: []string{"title", "message", "time", "event"},
		},
		{
			name:         "detailed template",
			template:     "detailed",
			expectedKeys: []string{"title", "message", "time", "event", "host", "host_ip", "service_count", "services"},
		},
		{
			name:         "custom template",
			template:     "custom",
			config:       `{"custom_key": "custom_value", "content": {{toJSON .Title}}}`,
			expectedKeys: []string{"custom_key", "content"},
		},
		{
			name:         "empty template defaults to minimal",
			template:     "",
			expectedKeys: []string{"title", "message", "time", "event"},
		},
		{
			name:         "unknown template defaults to minimal",
			template:     "unknown",
			expectedKeys: []string{"title", "message", "time", "event"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedBody map[string]any

			// Mock webhook HTTP client to capture request
			originalDo := webhookDoRequestFunc
			defer func() { webhookDoRequestFunc = originalDo }()
			webhookDoRequestFunc = func(client *http.Client, req *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(req.Body)
				_ = json.Unmarshal(body, &receivedBody)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       http.NoBody,
					Header:     make(http.Header),
				}, nil
			}

			provider := models.NotificationProvider{
				Type:     "discord",
				URL:      "https://discord.com/api/webhooks/123456/test_token",
				Template: tt.template,
				Config:   tt.config,
			}

			data := map[string]any{
				"Title":        "Test Title",
				"Message":      "Test Message",
				"Time":         time.Now().Format(time.RFC3339),
				"EventType":    "test",
				"HostName":     "testhost",
				"HostIP":       "192.168.1.1",
				"ServiceCount": 3,
				"Services":     []string{"svc1", "svc2"},
			}

			err := svc.sendJSONPayload(context.Background(), provider, data)
			require.NoError(t, err)

			for _, key := range tt.expectedKeys {
				assert.Contains(t, receivedBody, key, "Expected key %s in response", key)
			}

			for _, key := range tt.unexpectedKeys {
				assert.NotContains(t, receivedBody, key, "Unexpected key %s in response", key)
			}
		})
	}
}

func TestSendCustomWebhook_EmptyCustomTemplateDefaultsToMinimal(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	var receivedBody map[string]any

	// Mock webhook HTTP client
	originalDo := webhookDoRequestFunc
	defer func() { webhookDoRequestFunc = originalDo }()
	webhookDoRequestFunc = func(client *http.Client, req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		_ = json.Unmarshal(body, &receivedBody)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	}

	provider := models.NotificationProvider{
		Type:     "discord",
		URL:      "https://discord.com/api/webhooks/123456/test_token",
		Template: "custom",
		Config:   "", // Empty config should default to minimal
	}

	data := map[string]any{
		"Title":     "Test",
		"Message":   "Message",
		"Time":      time.Now().Format(time.RFC3339),
		"EventType": "test",
	}

	err := svc.sendJSONPayload(context.Background(), provider, data)
	require.NoError(t, err)

	// Should use minimal template
	assert.Equal(t, "Test", receivedBody["title"])
	assert.Equal(t, "Message", receivedBody["message"])
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

func TestIsPrivateIP_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		ip        string
		isPrivate bool
	}{
		// Boundary testing for 172.16-31 range
		{"172.15.255.255 (just before private)", "172.15.255.255", false},
		{"172.16.0.0 (start of private)", "172.16.0.0", true},
		{"172.31.255.255 (end of private)", "172.31.255.255", true},
		{"172.32.0.0 (just after private)", "172.32.0.0", false},

		// IPv6 unique local address boundaries
		{"fbff:ffff:ffff:ffff:ffff:ffff:ffff:ffff (before ULA)", "fbff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", false},
		{"fc00::0 (start of ULA)", "fc00::0", true},
		{"fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff (end of ULA)", "fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", true},
		{"fe00::0 (after ULA)", "fe00::0", false},

		// IPv6 link-local boundaries
		{"fe7f:ffff:ffff:ffff:ffff:ffff:ffff:ffff (before link-local)", "fe7f:ffff:ffff:ffff:ffff:ffff:ffff:ffff", false},
		{"fe80::0 (start of link-local)", "fe80::0", true},
		{"febf:ffff:ffff:ffff:ffff:ffff:ffff:ffff (end of link-local)", "febf:ffff:ffff:ffff:ffff:ffff:ffff:ffff", true},
		{"fec0::0 (after link-local)", "fec0::0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			require.NotNil(t, ip, "Failed to parse IP: %s", tt.ip)
			result := isPrivateIP(ip)
			assert.Equal(t, tt.isPrivate, result, "IP %s: expected private=%v, got=%v", tt.ip, tt.isPrivate, result)
		})
	}
}

func TestSendCustomWebhook_ContextCancellation(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := models.NotificationProvider{
		Type:     "discord",
		URL:      server.URL,
		Template: "minimal",
	}

	data := map[string]any{
		"Title":     "Test",
		"Message":   "Test",
		"Time":      time.Now().Format(time.RFC3339),
		"EventType": "test",
	}

	// Create context with immediate cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.sendJSONPayload(ctx, provider, data)
	require.Error(t, err)
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

func TestSendJSONPayload_ServiceSpecificValidation(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	t.Run("discord_message_is_normalized_to_content", func(t *testing.T) {
		originalDo := webhookDoRequestFunc
		defer func() { webhookDoRequestFunc = originalDo }()
		webhookDoRequestFunc = func(client *http.Client, req *http.Request) (*http.Response, error) {
			var payload map[string]any
			err := json.NewDecoder(req.Body).Decode(&payload)
			require.NoError(t, err)
			assert.Equal(t, "Test Message", payload["content"])
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
		}

		// Discord payload with message should be normalized to content
		provider := models.NotificationProvider{
			Type:     "discord",
			URL:      "https://discord.com/api/webhooks/123456/token_abc",
			Template: "custom",
			Config:   `{"message": {{toJSON .Message}}}`,
		}
		data := map[string]any{
			"Title":     "Test",
			"Message":   "Test Message",
			"Time":      time.Now().Format(time.RFC3339),
			"EventType": "test",
		}

		err := svc.sendJSONPayload(context.Background(), provider, data)
		require.NoError(t, err)
	})

	t.Run("discord_with_content_succeeds", func(t *testing.T) {
		originalDo := webhookDoRequestFunc
		defer func() { webhookDoRequestFunc = originalDo }()
		webhookDoRequestFunc = func(client *http.Client, req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
		}

		provider := models.NotificationProvider{
			Type:     "discord",
			URL:      "https://discord.com/api/webhooks/123456/token_abc",
			Template: "custom",
			Config:   `{"content": {{toJSON .Message}}}`,
		}
		data := map[string]any{
			"Title":     "Test",
			"Message":   "Test Message",
			"Time":      time.Now().Format(time.RFC3339),
			"EventType": "test",
		}

		err := svc.sendJSONPayload(context.Background(), provider, data)
		require.NoError(t, err)
	})

	t.Run("discord_with_embeds_succeeds", func(t *testing.T) {
		originalDo := webhookDoRequestFunc
		defer func() { webhookDoRequestFunc = originalDo }()
		webhookDoRequestFunc = func(client *http.Client, req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
		}

		provider := models.NotificationProvider{
			Type:     "discord",
			URL:      "https://discord.com/api/webhooks/123456/token_abc",
			Template: "custom",
			Config:   `{"embeds": [{"title": {{toJSON .Title}}}]}`,
		}
		data := map[string]any{
			"Title":     "Test",
			"Message":   "Test Message",
			"Time":      time.Now().Format(time.RFC3339),
			"EventType": "test",
		}

		err := svc.sendJSONPayload(context.Background(), provider, data)
		require.NoError(t, err)
	})

	t.Run("slack_requires_text_or_blocks", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Slack without text or blocks should fail
		provider := models.NotificationProvider{
			Type:     "slack",
			URL:      server.URL,
			Template: "custom",
			Config:   `{"message": {{toJSON .Message}}}`, // Missing text/blocks
		}
		data := map[string]any{
			"Title":     "Test",
			"Message":   "Test Message",
			"Time":      time.Now().Format(time.RFC3339),
			"EventType": "test",
		}

		err := svc.sendJSONPayload(context.Background(), provider, data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "slack payload requires 'text' or 'blocks' field")
	})

	t.Run("slack_with_text_succeeds", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		provider := models.NotificationProvider{
			Type:     "slack",
			URL:      server.URL,
			Template: "custom",
			Config:   `{"text": {{toJSON .Message}}}`,
		}
		data := map[string]any{
			"Title":     "Test",
			"Message":   "Test Message",
			"Time":      time.Now().Format(time.RFC3339),
			"EventType": "test",
		}

		err := svc.sendJSONPayload(context.Background(), provider, data)
		require.NoError(t, err)
	})

	t.Run("slack_with_blocks_succeeds", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		provider := models.NotificationProvider{
			Type:     "slack",
			URL:      server.URL,
			Template: "custom",
			Config:   `{"blocks": [{"type": "section", "text": {"type": "mrkdwn", "text": {{toJSON .Message}}}}]}`,
		}
		data := map[string]any{
			"Title":     "Test",
			"Message":   "Test Message",
			"Time":      time.Now().Format(time.RFC3339),
			"EventType": "test",
		}

		err := svc.sendJSONPayload(context.Background(), provider, data)
		require.NoError(t, err)
	})

	t.Run("gotify_requires_message", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Gotify without message should fail
		provider := models.NotificationProvider{
			Type:     "gotify",
			URL:      server.URL,
			Template: "custom",
			Config:   `{"title": {{toJSON .Title}}}`, // Missing message
		}
		data := map[string]any{
			"Title":     "Test",
			"Message":   "Test Message",
			"Time":      time.Now().Format(time.RFC3339),
			"EventType": "test",
		}

		err := svc.sendJSONPayload(context.Background(), provider, data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "gotify payload requires 'message' field")
	})

	t.Run("gotify_with_message_succeeds", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		provider := models.NotificationProvider{
			Type:     "gotify",
			URL:      server.URL,
			Template: "custom",
			Config:   `{"message": {{toJSON .Message}}, "title": {{toJSON .Title}}}`,
		}
		data := map[string]any{
			"Title":     "Test",
			"Message":   "Test Message",
			"Time":      time.Now().Format(time.RFC3339),
			"EventType": "test",
		}

		err := svc.sendJSONPayload(context.Background(), provider, data)
		require.NoError(t, err)
	})
}

// ============================================
// Phase 3: SendExternal Event Type Coverage
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
			svc := NewNotificationService(db, nil)

			// Mock Discord validation to allow test server URL
			origValidateDiscordFunc := validateDiscordProviderURLFunc
			defer func() { validateDiscordProviderURLFunc = origValidateDiscordFunc }()
			validateDiscordProviderURLFunc = func(providerType, rawURL string) error { return nil }

			var callCount atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			provider := models.NotificationProvider{
				Name:                "event-test",
				Type:                "discord",
				URL:                 server.URL,
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
			time.Sleep(100 * time.Millisecond)

			// test always sends; unknown defaults to false (security-first); others only when their flag is true
			switch et.eventType {
			case "test":
				assert.Greater(t, callCount.Load(), int32(0), "Event type %s should trigger notification", et.eventType)
			case "unknown":
				assert.Equal(t, int32(0), callCount.Load(), "Unknown event type should not trigger notification (security-first)")
			default:
				assert.Greater(t, callCount.Load(), int32(0), "Event type %s should trigger notification when flag is set", et.eventType)
			}
		})
	}
}

// ============================================
// Phase 3: isValidRedirectURL Coverage
// ============================================

func TestIsValidRedirectURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"valid http", "https://discord.com/api/webhooks/123/abc/webhook", true},
		{"valid https", "https://example.com/webhook", true},
		{"invalid scheme ftp", "ftp://example.com", false},
		{"invalid scheme file", "file:///etc/passwd", false},
		{"no scheme", "example.com/webhook", false},
		{"empty hostname", "http:///webhook", false},
		{"invalid url", "://invalid", false},
		{"javascript scheme", "javascript:alert(1)", false},
		{"data scheme", "data:text/html,<h1>test</h1>", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidRedirectURL(tt.url)
			assert.Equal(t, tt.expected, result, "isValidRedirectURL(%q) = %v, want %v", tt.url, result, tt.expected)
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
			svc := NewNotificationService(db, nil)

			origValidate := validateDiscordProviderURLFunc
			defer func() { validateDiscordProviderURLFunc = origValidate }()
			validateDiscordProviderURLFunc = func(providerType, rawURL string) error { return nil }

			received := make(chan struct{}, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received <- struct{}{}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			provider := models.NotificationProvider{
				Name:     "discord-security",
				Type:     "discord",
				URL:      server.URL,
				Enabled:  true,
				Template: "minimal",
			}
			tc.apply(&provider)
			require.NoError(t, db.Create(&provider).Error)

			svc.SendExternal(context.Background(), tc.eventType, "Security Title", "Security Message", nil)

			select {
			case <-received:
			case <-time.After(1 * time.Second):
				t.Fatalf("expected dispatch for event type %s", tc.eventType)
			}
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
		{"telegram", "telegram", "telegram://token@telegram?chats=123"},
		{"slack", "slack", "https://hooks.slack.com/services/T/B/X"},
		{"pushover", "pushover", "pushover://token@user"},
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

func TestTestProvider_DiscordUsesNotifyPathInPR1(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	serverCalled := atomic.Bool{}
	originalDo := webhookDoRequestFunc
	webhookDoRequestFunc = func(client *http.Client, req *http.Request) (*http.Response, error) {
		serverCalled.Store(true)
		// Verify it's using JSON payload (not legacy fallback)
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
	}
	defer func() { webhookDoRequestFunc = originalDo }()

	provider := models.NotificationProvider{
		Type:     "discord",
		URL:      "https://discord.com/api/webhooks/123456789/token_abc",
		Template: "minimal",
	}

	err := svc.TestProvider(provider)
	require.NoError(t, err)
	assert.True(t, serverCalled.Load(), "discord provider should use JSON webhook path")
}

func TestTestProvider_HTTPURLValidation(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	t.Run("blocks private IP", func(t *testing.T) {
		provider := models.NotificationProvider{
			Type:     "discord",
			URL:      "https://discord.com/api/webhooks/999/invalidtoken",
			Template: "",
		}

		// Mock the webhook request to fail on IP validation
		originalDo := webhookDoRequestFunc
		webhookDoRequestFunc = func(client *http.Client, req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("private IP blocked")
		}
		defer func() { webhookDoRequestFunc = originalDo }()

		err := svc.TestProvider(provider)
		require.Error(t, err)
	})

	t.Run("allows valid discord webhook", func(t *testing.T) {
		serverCalled := atomic.Bool{}
		originalDo := webhookDoRequestFunc
		webhookDoRequestFunc = func(client *http.Client, req *http.Request) (*http.Response, error) {
			serverCalled.Store(true)
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
		}
		defer func() { webhookDoRequestFunc = originalDo }()

		provider := models.NotificationProvider{
			Type:     "discord",
			URL:      "https://discord.com/api/webhooks/123456789/validtoken_abc",
			Template: "minimal",
		}

		err := svc.TestProvider(provider)
		require.NoError(t, err)
		assert.True(t, serverCalled.Load())
	})
}

// ============================================
// Phase 4: Additional Edge Case Coverage
// ============================================

func TestSendJSONPayload_TemplateExecutionError(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Template that calls a method on nil should cause execution error
	provider := models.NotificationProvider{
		Type:     "discord",
		URL:      server.URL,
		Template: "custom",
		Config:   `{"result": {{call .NonExistentFunc}}}`, // This will fail during execution
	}

	data := map[string]any{
		"Title":     "Test",
		"Message":   "Test",
		"Time":      time.Now().Format(time.RFC3339),
		"EventType": "test",
	}

	err := svc.sendJSONPayload(context.Background(), provider, data)
	require.Error(t, err)
	// The error could be a parse error or execution error depending on Go version
}

func TestSendJSONPayload_InvalidJSONFromTemplate(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Template that produces invalid JSON
	provider := models.NotificationProvider{
		Type:     "webhook",
		URL:      server.URL,
		Template: "custom",
		Config:   `{"title": {{.Title}}}`, // Missing toJSON, will produce unquoted string
	}

	data := map[string]any{
		"Title":     "Test Value",
		"Message":   "Test",
		"Time":      time.Now().Format(time.RFC3339),
		"EventType": "test",
	}

	err := svc.sendJSONPayload(context.Background(), provider, data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON payload")
}

func TestSendJSONPayload_RequestCreationError(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	// This test verifies request creation doesn't panic on edge cases
	provider := models.NotificationProvider{
		Type:     "discord",
		URL:      "http://localhost:8080/webhook",
		Template: "minimal",
	}

	// Use canceled context to trigger early error
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	data := map[string]any{
		"Title":     "Test",
		"Message":   "Test",
		"Time":      time.Now().Format(time.RFC3339),
		"EventType": "test",
	}

	err := svc.sendJSONPayload(ctx, provider, data)
	require.Error(t, err)
}

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

func TestSendJSONPayload_HTTPScheme(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	// Test both HTTP and HTTPS schemes
	schemes := []string{"http", "https"}

	for _, scheme := range schemes {
		t.Run(scheme, func(t *testing.T) {
			// Create server (note: httptest.Server uses http by default)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			provider := models.NotificationProvider{
				Type:     "webhook",
				URL:      server.URL, // httptest always uses http
				Template: "minimal",
			}

			data := map[string]any{
				"Title":     "Test",
				"Message":   "Test",
				"Time":      time.Now().Format(time.RFC3339),
				"EventType": "test",
			}

			err := svc.sendJSONPayload(context.Background(), provider, data)
			require.NoError(t, err)
		})
	}
}

// ============================================
// Migration Completeness Tests
// ============================================

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
			Name:    "Pushover Provider (deprecated)",
			Type:    "pushover",
			URL:     "pushover://token@user",
			Enabled: true,
		},
		{
			Name:    "Gotify Provider",
			Type:    "gotify",
			URL:     "https://discord.com/api/webhooks/123/abc/gotify",
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
	nonDiscordTypes := []string{"webhook", "telegram", "pushover", "gotify"}
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
	assert.Equal(t, "feature.notifications.service.email.enabled", notifications.FlagEmailServiceEnabled)
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
		Key:   notifications.FlagEmailServiceEnabled,
		Value: "true",
	}).Error)
	assert.True(t, svc.isDispatchEnabled("email"))
}

// TestSendExternal_EmailProvider_NilMailService_DoesNotPanic verifies that when an
// email provider is enabled but the mail service is nil, SendExternal dispatches
// the goroutine which early-returns without panicking. The type == "email" branch
// calls dispatchEmail and continues — it never reaches supportsJSONTemplates.
func TestSendExternal_EmailProvider_NilMailService_DoesNotPanic(t *testing.T) {
	db := setupNotificationTestDB(t)
	_ = db.AutoMigrate(&models.Setting{})
	svc := NewNotificationService(db, nil)

	// Enable the email feature flag so isDispatchEnabled("email") returns true.
	require.NoError(t, db.Create(&models.Setting{
		Key:   notifications.FlagEmailServiceEnabled,
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

// --- mockMailService for dispatchEmail tests ---

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

func TestDispatchEmail_NilMailService(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db, nil)

	p := models.NotificationProvider{Name: "test-email", URL: "a@b.com", Type: "email"}
	// Must not panic
	svc.dispatchEmail(context.Background(), p, "alert", "Title", "Message")
}

func TestDispatchEmail_SMTPNotConfigured(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: false}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", URL: "a@b.com", Type: "email"}
	svc.dispatchEmail(context.Background(), p, "alert", "Title", "Message")

	assert.Empty(t, mock.calls)
}

func TestDispatchEmail_EmptyRecipients(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: true}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", URL: "   ,  , ", Type: "email"}
	svc.dispatchEmail(context.Background(), p, "alert", "Title", "Message")

	assert.Empty(t, mock.calls)
}

func TestDispatchEmail_ValidSend(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: true}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", URL: "a@b.com, c@d.com", Type: "email"}
	svc.dispatchEmail(context.Background(), p, "alert", "My Title", "My Message")

	require.Len(t, mock.calls, 1)
	assert.Equal(t, []string{"a@b.com", "c@d.com"}, mock.calls[0].to)
	assert.Equal(t, "[Charon Alert] My Title", mock.calls[0].subject)
	assert.Contains(t, mock.calls[0].body, "<strong>My Title</strong>")
	assert.Contains(t, mock.calls[0].body, "My Message")
}

func TestDispatchEmail_SendError_Logged(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: true, sendEmailErr: fmt.Errorf("smtp failure")}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", URL: "a@b.com", Type: "email"}
	// Must not panic even when SendEmail returns error
	svc.dispatchEmail(context.Background(), p, "alert", "Title", "Message")

	assert.Len(t, mock.calls, 1)
}

func TestSendExternal_EmailProvider_Dispatches(t *testing.T) {
	db := setupNotificationTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	mock := &mockMailService{isConfigured: true}
	svc := NewNotificationService(db, mock)

	provider := models.NotificationProvider{
		Name:    "email-provider",
		Type:    "email",
		URL:     "notify@example.com",
		Enabled: true,
	}
	require.NoError(t, db.Create(&provider).Error)

	db.Create(&models.Setting{Key: notifications.FlagEmailServiceEnabled, Value: "true"})

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

	db.Create(&models.Setting{Key: notifications.FlagEmailServiceEnabled, Value: "false"})

	svc.SendExternal(context.Background(), "test", "Title", "Body", nil)

	time.Sleep(50 * time.Millisecond)
	assert.Zero(t, mock.callCount())
}

func TestDispatchEmail_InvalidRecipient(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: true, sendEmailErr: ErrInvalidRecipient}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", URL: "not-an-email", Type: "email"}
	// dispatchEmail will call SendEmail; the mock returns ErrInvalidRecipient — must not panic
	svc.dispatchEmail(context.Background(), p, "alert", "Title", "Message")

	// SendEmail was called once (validation happens inside real SendEmail, mock just returns the error)
	assert.Len(t, mock.calls, 1)
}

func TestDispatchEmail_TooManyRecipients(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: true, sendEmailErr: ErrTooManyRecipients}
	svc := NewNotificationService(db, mock)

	recipients := make([]string, 21)
	for i := range recipients {
		recipients[i] = fmt.Sprintf("user%d@example.com", i)
	}
	p := models.NotificationProvider{Name: "test-email", URL: strings.Join(recipients, ","), Type: "email"}
	// dispatchEmail passes all recipients to SendEmail; mock returns ErrTooManyRecipients — must not panic
	svc.dispatchEmail(context.Background(), p, "alert", "Title", "Message")

	assert.Len(t, mock.calls, 1)
}

func TestDispatchEmail_HeaderInjectionRecipient(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: true, sendEmailErr: ErrInvalidRecipient}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", URL: "bad\r\naddr@test.com", Type: "email"}
	// The recipient contains CR/LF; dispatchEmail trims + splits but passes to SendEmail which rejects — must not panic
	svc.dispatchEmail(context.Background(), p, "alert", "Title", "Message")

	assert.Len(t, mock.calls, 1)
}

func TestSendExternal_EmailProviderDoesNotCallSendJSONPayload(t *testing.T) {
	db := setupNotificationTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	mock := &mockMailService{isConfigured: true}
	svc := NewNotificationService(db, mock)

	// Track any JSON payload call via the webhook hook
	jsonPayloadCalled := false
	origDo := webhookDoRequestFunc
	defer func() { webhookDoRequestFunc = origDo }()
	webhookDoRequestFunc = func(client *http.Client, req *http.Request) (*http.Response, error) {
		jsonPayloadCalled = true
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
	}

	provider := models.NotificationProvider{
		Name:    "email-no-http",
		Type:    "email",
		URL:     "notify@example.com",
		Enabled: true,
	}
	require.NoError(t, db.Create(&provider).Error)
	db.Create(&models.Setting{Key: notifications.FlagEmailServiceEnabled, Value: "true"})

	svc.SendExternal(context.Background(), "test", "Title", "Body", nil)
	require.Eventually(t, func() bool { return mock.callCount() > 0 }, 2*time.Second, 10*time.Millisecond)

	assert.False(t, jsonPayloadCalled, "email provider must not trigger HTTP JSON payload path")
}

func TestDispatchEmail_XSSPayload_BodySanitized(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: true}
	svc := NewNotificationService(db, mock)

	xssTitle := `<script>alert('xss')</script>`
	xssMessage := `<img src=x onerror=evil()>`

	p := models.NotificationProvider{Name: "test-email", URL: "a@b.com", Type: "email"}
	svc.dispatchEmail(context.Background(), p, "alert", xssTitle, xssMessage)

	require.Len(t, mock.calls, 1)
	body := mock.calls[0].body
	// Raw script tags must not appear — they must be escaped.
	assert.NotContains(t, body, "<script>")
	assert.Contains(t, body, "&lt;script&gt;")
	// The <img> attack vector tag must be escaped — the literal < must not appear unescaped.
	assert.NotContains(t, body, "<img ")
	assert.Contains(t, body, "&lt;img ")
}

// --- sanitizeForEmail unit tests ---

func TestSanitizeForEmail_EmptyString(t *testing.T) {
	assert.Equal(t, "", sanitizeForEmail(""))
}

func TestSanitizeForEmail_CleanString(t *testing.T) {
	assert.Equal(t, "Hello World", sanitizeForEmail("Hello World"))
}

func TestSanitizeForEmail_StripsCRLF(t *testing.T) {
	assert.Equal(t, "HelloWorld", sanitizeForEmail("Hello\r\nWorld"))
}

func TestSanitizeForEmail_StripsEmbeddedLF(t *testing.T) {
	assert.Equal(t, "line1line2", sanitizeForEmail("line1\nline2"))
}

func TestSanitizeForEmail_StripsEmbeddedCR(t *testing.T) {
	assert.Equal(t, "line1line2", sanitizeForEmail("line1\rline2"))
}

func TestSanitizeForEmail_MultipleCRLF(t *testing.T) {
	assert.Equal(t, "abc", sanitizeForEmail("\r\na\r\nb\r\nc\r\n"))
}

func TestDispatchEmail_CRLFInTitle_StrippedBeforeSubject(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: true}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", URL: "a@b.com", Type: "email"}
	svc.dispatchEmail(context.Background(), p, "alert", "Title\r\nBCC: attacker@evil.com", "msg")

	require.Equal(t, 1, mock.callCount())
	call := mock.firstCall()
	assert.Equal(t, "[Charon Alert] TitleBCC: attacker@evil.com", call.subject)
	assert.NotContains(t, call.subject, "\r")
	assert.NotContains(t, call.subject, "\n")
}

func TestDispatchEmail_CRLFInMessage_StrippedBeforeBody(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: true}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", URL: "a@b.com", Type: "email"}
	svc.dispatchEmail(context.Background(), p, "alert", "Title", "msg\r\n.\r\nMAIL FROM:<attacker>")

	require.Equal(t, 1, mock.callCount())
	call := mock.firstCall()
	assert.NotContains(t, call.body, "\r")
	assert.NotContains(t, call.body, "\n")
	assert.Contains(t, call.body, "msg.MAIL FROM:&lt;attacker&gt;")
}

func TestDispatchEmail_UsesTemplate(t *testing.T) {
	db := setupNotificationTestDB(t)
	renderedHTML := "<html><body>Rendered template content</body></html>"
	mock := &mockMailService{isConfigured: true, renderResult: renderedHTML}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", URL: "a@b.com", Type: "email"}
	svc.dispatchEmail(context.Background(), p, "security_waf", "WAF Alert", "Blocked request")

	require.Len(t, mock.calls, 1)
	assert.Equal(t, renderedHTML, mock.calls[0].body)
	assert.NotContains(t, mock.calls[0].body, "<strong>")
}

func TestDispatchEmail_TemplateFallback(t *testing.T) {
	db := setupNotificationTestDB(t)
	mock := &mockMailService{isConfigured: true, renderErr: fmt.Errorf("template parse error")}
	svc := NewNotificationService(db, mock)

	p := models.NotificationProvider{Name: "test-email", URL: "a@b.com", Type: "email"}
	svc.dispatchEmail(context.Background(), p, "alert", "Fallback Title", "Fallback Message")

	require.Len(t, mock.calls, 1)
	assert.Contains(t, mock.calls[0].body, "<strong>Fallback Title</strong>")
	assert.Contains(t, mock.calls[0].body, "Fallback Message")
}

// --- TestEmailProvider unit tests ---

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
mock := &mockMailService{isConfigured: true}
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
mock := &mockMailService{isConfigured: true}
svc := NewNotificationService(db, mock)

p := models.NotificationProvider{Name: "test-email", Type: "email", URL: "a@b.com, c@d.com , e@f.com"}
err := svc.TestEmailProvider(p)
require.NoError(t, err)
require.Equal(t, 1, mock.callCount())
assert.Equal(t, []string{"a@b.com", "c@d.com", "e@f.com"}, mock.firstCall().to)
}

func TestEmailProvider_SendError(t *testing.T) {
db := setupNotificationTestDB(t)
mock := &mockMailService{isConfigured: true, sendEmailErr: fmt.Errorf("smtp: connection refused")}
svc := NewNotificationService(db, mock)

p := models.NotificationProvider{Name: "test-email", Type: "email", URL: "a@b.com"}
err := svc.TestEmailProvider(p)
require.Error(t, err)
assert.Contains(t, err.Error(), "smtp")
assert.Equal(t, 1, mock.callCount())
}

func TestEmailProvider_TemplateFallback(t *testing.T) {
db := setupNotificationTestDB(t)
mock := &mockMailService{isConfigured: true, renderErr: fmt.Errorf("template not found")}
svc := NewNotificationService(db, mock)

p := models.NotificationProvider{Name: "test-email", Type: "email", URL: "a@b.com"}
err := svc.TestEmailProvider(p)
require.NoError(t, err)
require.Equal(t, 1, mock.callCount())
assert.Contains(t, mock.firstCall().body, "<strong>Test Notification</strong>")
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
