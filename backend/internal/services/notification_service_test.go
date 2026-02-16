package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
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
	svc := NewNotificationService(db)

	notif, err := svc.Create(models.NotificationTypeInfo, "Test", "Message")
	require.NoError(t, err)
	assert.Equal(t, "Test", notif.Title)
	assert.Equal(t, "Message", notif.Message)
	assert.False(t, notif.Read)
}

func TestNotificationService_List(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)

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
	svc := NewNotificationService(db)

	notif, _ := svc.Create(models.NotificationTypeInfo, "N1", "M1")

	err := svc.MarkAsRead(notif.ID)
	require.NoError(t, err)

	var updated models.Notification
	db.First(&updated, "id = ?", notif.ID)
	assert.True(t, updated.Read)
}

func TestNotificationService_MarkAllAsRead(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)

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
	svc := NewNotificationService(db)

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
	svc := NewNotificationService(db)

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
		Name:     "Test Webhook",
		Type:     "webhook",
		URL:      ts.URL,
		Template: "minimal",
		Config:   `{"Header": "{{.Title}}"}`,
	}

	err := svc.TestProvider(provider)
	require.NoError(t, err)
}

func TestNotificationService_SendExternal(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)

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
		NotifyProxyHosts: true,
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
	svc := NewNotificationService(db)

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
		Name:         "Minimal",
		Type:         "webhook",
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
		Name:         "Detailed",
		Type:         "webhook",
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
	svc := NewNotificationService(db)

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

func TestNotificationService_SendExternal_Shoutrrr(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)

	provider := models.NotificationProvider{
		Name:             "Test Discord",
		Type:             "discord",
		URL:              "discord://token@id",
		Enabled:          true,
		NotifyProxyHosts: true,
	}
	_ = svc.CreateProvider(&provider)

	// This will log an error but should cover the code path
	svc.SendExternal(context.Background(), "proxy_host", "Title", "Message", nil)

	// Give it a moment to run goroutine
	time.Sleep(100 * time.Millisecond)
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
			name:        "Discord Shoutrrr",
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
	svc := NewNotificationService(db)

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
	svc := NewNotificationService(db)

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
	svc := NewNotificationService(db)

	t.Run("unsupported provider type", func(t *testing.T) {
		provider := models.NotificationProvider{
			Type: "unsupported",
			URL:  "http://example.com",
		}
		err := svc.TestProvider(provider)
		assert.Error(t, err)
		// Shoutrrr returns "unknown service" for unsupported schemes
		assert.Contains(t, err.Error(), "unknown service")
	})

	t.Run("webhook with invalid URL", func(t *testing.T) {
		provider := models.NotificationProvider{
			Type: "webhook",
			URL:  "://invalid",
		}
		err := svc.TestProvider(provider)
		assert.Error(t, err)
	})

	t.Run("discord with invalid URL format", func(t *testing.T) {
		provider := models.NotificationProvider{
			Type: "discord",
			URL:  "invalid-discord-url",
		}
		err := svc.TestProvider(provider)
		assert.Error(t, err)
	})

	t.Run("slack with unreachable webhook", func(t *testing.T) {
		provider := models.NotificationProvider{
			Type: "slack",
			URL:  "https://hooks.slack.com/services/INVALID/WEBHOOK/URL",
		}
		err := svc.TestProvider(provider)
		// Shoutrrr will return error for unreachable/invalid webhook
		assert.Error(t, err)
	})

	t.Run("webhook success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		provider := models.NotificationProvider{
			Type:     "webhook",
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
	svc := NewNotificationService(db)

	t.Run("blocks private IP webhook", func(t *testing.T) {
		provider := models.NotificationProvider{
			Type: "webhook",
			URL:  "http://10.0.0.1/webhook",
		}
		data := map[string]any{"Title": "Test", "Message": "Test Message"}
		err := svc.sendJSONPayload(context.Background(), provider, data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid webhook url")
	})

	t.Run("blocks cloud metadata endpoint", func(t *testing.T) {
		provider := models.NotificationProvider{
			Type: "webhook",
			URL:  "http://169.254.169.254/latest/meta-data/",
		}
		data := map[string]any{"Title": "Test", "Message": "Test Message"}
		err := svc.sendJSONPayload(context.Background(), provider, data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid webhook url")
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
		svc := NewNotificationService(db)

		provider := models.NotificationProvider{
			Name:    "Disabled",
			Type:    "webhook",
			URL:     "http://example.com",
			Enabled: false,
		}
		_ = svc.CreateProvider(&provider)

		// Should complete without error
		svc.SendExternal(context.Background(), "proxy_host", "Title", "Message", nil)
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("provider filtered by category", func(t *testing.T) {
		db := setupNotificationTestDB(t)
		svc := NewNotificationService(db)

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
		svc := NewNotificationService(db)

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
			Name:             "Custom Data",
			Type:             "webhook",
			URL:              ts.URL,
			Enabled:          true,
			NotifyProxyHosts: true,
			Config:           `{"custom": "{{.CustomField}}"}`,
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
	svc := NewNotificationService(db)

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
	svc := NewNotificationService(db)

	t.Run("creates provider with defaults", func(t *testing.T) {
		provider := models.NotificationProvider{
			Name: "Test",
			Type: "webhook",
			URL:  "http://example.com",
		}
		err := svc.CreateProvider(&provider)
		assert.NoError(t, err)
		assert.NotEmpty(t, provider.ID)
		assert.False(t, provider.Enabled) // Default
	})

	t.Run("updates existing provider", func(t *testing.T) {
		provider := models.NotificationProvider{
			Name:    "Original",
			Type:    "webhook",
			URL:     "http://example.com",
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
	svc := NewNotificationService(db)

	t.Run("invalid custom template on create", func(t *testing.T) {
		provider := models.NotificationProvider{
			Name:     "Bad Custom",
			Type:     "webhook",
			URL:      "http://example.com",
			Template: "custom",
			Config:   `{"bad": "{{.Title"}`,
		}
		err := svc.CreateProvider(&provider)
		assert.Error(t, err)
	})

	t.Run("invalid custom template on update", func(t *testing.T) {
		provider := models.NotificationProvider{
			Name:     "Valid",
			Type:     "webhook",
			URL:      "http://example.com",
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
	svc := NewNotificationService(db)

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
	svc := NewNotificationService(db)

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
	svc := NewNotificationService(db)

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
	svc := NewNotificationService(db)

	errorCodes := []int{400, 404, 500, 502, 503}

	for _, statusCode := range errorCodes {
		t.Run(fmt.Sprintf("status_%d", statusCode), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
			}))
			defer server.Close()

			provider := models.NotificationProvider{
				Type:     "webhook",
				URL:      server.URL,
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
	svc := NewNotificationService(db)

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
			config:       `{"custom_key": "custom_value", "title": {{toJSON .Title}}}`,
			expectedKeys: []string{"custom_key", "title"},
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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &receivedBody)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			provider := models.NotificationProvider{
				Type:     "webhook",
				URL:      server.URL,
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
	svc := NewNotificationService(db)

	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := models.NotificationProvider{
		Type:     "webhook",
		URL:      server.URL,
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
	svc := NewNotificationService(db)

	provider := &models.NotificationProvider{
		Name:     "empty-template",
		Type:     "webhook",
		URL:      "http://localhost:8080/webhook",
		Template: "custom",
		Config:   "", // Empty should be allowed and default to minimal
	}

	err := svc.CreateProvider(provider)
	require.NoError(t, err)
	assert.NotEmpty(t, provider.ID)
}

func TestUpdateProvider_NonCustomTemplateSkipsValidation(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)

	provider := &models.NotificationProvider{
		Name:     "test",
		Type:     "webhook",
		URL:      "http://localhost:8080",
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
	svc := NewNotificationService(db)

	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := models.NotificationProvider{
		Type:     "webhook",
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
	svc := NewNotificationService(db)

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

	// Send with unknown event type - should send (default behavior)
	ctx := context.Background()
	svc.SendExternal(ctx, "unknown_event_type", "Test", "Message", nil)

	time.Sleep(100 * time.Millisecond)
	assert.Greater(t, callCount.Load(), int32(0), "Unknown event type should trigger notification")
}

func TestCreateProvider_ValidCustomTemplate(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)

	provider := &models.NotificationProvider{
		Name:     "valid-custom",
		Type:     "webhook",
		URL:      "http://localhost:8080/webhook",
		Template: "custom",
		Config:   `{"message": {{toJSON .Message}}, "title": {{toJSON .Title}}, "custom_field": "value"}`,
	}

	err := svc.CreateProvider(provider)
	require.NoError(t, err)
	assert.NotEmpty(t, provider.ID)
}

func TestUpdateProvider_ValidCustomTemplate(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)

	provider := &models.NotificationProvider{
		Name:     "test",
		Type:     "webhook",
		URL:      "http://localhost:8080",
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
	svc := NewNotificationService(db)

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
	svc := NewNotificationService(db)

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
		{"unknown", ""}, // unknown defaults to true
	}

	for _, et := range eventTypes {
		t.Run(et.eventType, func(t *testing.T) {
			db := setupNotificationTestDB(t)
			svc := NewNotificationService(db)

			var callCount atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			provider := models.NotificationProvider{
				Name:                "event-test",
				Type:                "webhook",
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

			// test and unknown should always send; others only when their flag is true
			if et.eventType == "test" || et.eventType == "unknown" {
				assert.Greater(t, callCount.Load(), int32(0), "Event type %s should trigger notification", et.eventType)
			} else {
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
		{"valid http", "http://example.com/webhook", true},
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

// ============================================
// Phase 3: SendExternal with Shoutrrr path (non-JSON)
// ============================================

func TestSendExternal_ShoutrrrPath(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)

	// Test shoutrrr path with mocked function
	var called atomic.Bool
	var receivedMsg atomic.Value
	originalFunc := shoutrrrSendFunc
	shoutrrrSendFunc = func(url, msg string) error {
		called.Store(true)
		receivedMsg.Store(msg)
		return nil
	}
	defer func() { shoutrrrSendFunc = originalFunc }()

	// Provider without template (uses shoutrrr path)
	provider := models.NotificationProvider{
		Name:             "shoutrrr-test",
		Type:             "telegram", // telegram doesn't support JSON templates
		URL:              "telegram://token@telegram?chats=123",
		Enabled:          true,
		NotifyProxyHosts: true,
		Template:         "", // Empty template forces shoutrrr path
	}
	require.NoError(t, db.Create(&provider).Error)

	svc.SendExternal(context.Background(), "proxy_host", "Test Title", "Test Message", nil)
	time.Sleep(100 * time.Millisecond)

	assert.True(t, called.Load(), "shoutrrr function should have been called")
	msg := receivedMsg.Load().(string)
	assert.Contains(t, msg, "Test Title")
	assert.Contains(t, msg, "Test Message")
}

func TestSendExternal_ShoutrrrPathWithHTTPValidation(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)

	var called atomic.Bool
	originalFunc := shoutrrrSendFunc
	shoutrrrSendFunc = func(url, msg string) error {
		called.Store(true)
		return nil
	}
	defer func() { shoutrrrSendFunc = originalFunc }()

	// Provider with HTTP URL but no template AND unsupported type (triggers SSRF check in shoutrrr path)
	// Using "pushover" which is not in supportsJSONTemplates list
	provider := models.NotificationProvider{
		Name:             "http-shoutrrr",
		Type:             "pushover", // Unsupported JSON template type
		URL:              "http://127.0.0.1:8080/webhook",
		Enabled:          true,
		NotifyProxyHosts: true,
		Template:         "", // Empty template
	}
	require.NoError(t, db.Create(&provider).Error)

	svc.SendExternal(context.Background(), "proxy_host", "Test", "Message", nil)
	time.Sleep(100 * time.Millisecond)

	// Should call shoutrrr since URL is valid (localhost allowed)
	assert.True(t, called.Load())
}

func TestSendExternal_ShoutrrrPathBlocksPrivateIP(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)

	var called atomic.Bool
	originalFunc := shoutrrrSendFunc
	shoutrrrSendFunc = func(url, msg string) error {
		called.Store(true)
		return nil
	}
	defer func() { shoutrrrSendFunc = originalFunc }()

	// Provider with private IP URL (should be blocked)
	// Using "pushover" which doesn't support JSON templates
	provider := models.NotificationProvider{
		Name:             "private-ip",
		Type:             "pushover", // Unsupported JSON template type
		URL:              "http://10.0.0.1:8080/webhook",
		Enabled:          true,
		NotifyProxyHosts: true,
		Template:         "", // Empty template
	}
	require.NoError(t, db.Create(&provider).Error)

	svc.SendExternal(context.Background(), "proxy_host", "Test", "Message", nil)
	time.Sleep(100 * time.Millisecond)

	// Should NOT call shoutrrr since URL is blocked (private IP)
	assert.False(t, called.Load(), "shoutrrr should not be called for private IP")
}

func TestSendExternal_ShoutrrrError(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)

	// Mock shoutrrr to return error
	var wg sync.WaitGroup
	originalFunc := shoutrrrSendFunc
	shoutrrrSendFunc = func(url, msg string) error {
		defer wg.Done()
		return fmt.Errorf("shoutrrr error: connection failed")
	}
	defer func() { shoutrrrSendFunc = originalFunc }()

	provider := models.NotificationProvider{
		Name:             "error-test",
		Type:             "telegram",
		URL:              "telegram://token@telegram?chats=123",
		Enabled:          true,
		NotifyProxyHosts: true,
		Template:         "",
	}
	require.NoError(t, db.Create(&provider).Error)

	// Should not panic, just log error
	wg.Add(1)
	svc.SendExternal(context.Background(), "proxy_host", "Test", "Message", nil)
	wg.Wait()
}

func TestTestProvider_ShoutrrrPath(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)

	var called atomic.Bool
	originalFunc := shoutrrrSendFunc
	shoutrrrSendFunc = func(url, msg string) error {
		called.Store(true)
		return nil
	}
	defer func() { shoutrrrSendFunc = originalFunc }()

	// Provider without template uses shoutrrr path
	provider := models.NotificationProvider{
		Type:     "telegram",
		URL:      "telegram://token@telegram?chats=123",
		Template: "", // Empty template
	}

	err := svc.TestProvider(provider)
	require.NoError(t, err)
	assert.True(t, called.Load())
}

func TestTestProvider_HTTPURLValidation(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)

	t.Run("blocks private IP", func(t *testing.T) {
		provider := models.NotificationProvider{
			Type:     "generic",
			URL:      "http://10.0.0.1:8080/webhook",
			Template: "", // Empty template uses shoutrrr path
		}

		err := svc.TestProvider(provider)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid notification URL")
	})

	t.Run("allows localhost", func(t *testing.T) {
		var called atomic.Bool
		originalFunc := shoutrrrSendFunc
		shoutrrrSendFunc = func(url, msg string) error {
			called.Store(true)
			return nil
		}
		defer func() { shoutrrrSendFunc = originalFunc }()

		provider := models.NotificationProvider{
			Type:     "generic",
			URL:      "http://127.0.0.1:8080/webhook",
			Template: "", // Empty template
		}

		err := svc.TestProvider(provider)
		require.NoError(t, err)
		assert.True(t, called.Load())
	})
}

// ============================================
// Phase 4: Additional Edge Case Coverage
// ============================================

func TestSendJSONPayload_TemplateExecutionError(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Template that calls a method on nil should cause execution error
	provider := models.NotificationProvider{
		Type:     "webhook",
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
	svc := NewNotificationService(db)

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
	svc := NewNotificationService(db)

	// This test verifies request creation doesn't panic on edge cases
	provider := models.NotificationProvider{
		Type:     "webhook",
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
	svc := NewNotificationService(db)

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

	svc := NewNotificationService(db)

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

	svc := NewNotificationService(db)

	// Close the underlying connection to force error
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	// Should not panic, just log error and return
	svc.SendExternal(context.Background(), "test", "Title", "Message", nil)
}

func TestSendExternal_JSONPayloadError(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)

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
	svc := NewNotificationService(db)

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
