package services

import (
	"encoding/json"
	"fmt"
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"net/http/httptest"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupNotificationTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	db.AutoMigrate(&models.Notification{}, &models.NotificationProvider{})
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

	svc.Create(models.NotificationTypeInfo, "N1", "M1")
	svc.Create(models.NotificationTypeInfo, "N2", "M2")

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

	err := svc.MarkAsRead(fmt.Sprintf("%s", notif.ID))
	require.NoError(t, err)

	var updated models.Notification
	db.First(&updated, "id = ?", notif.ID)
	assert.True(t, updated.Read)
}

func TestNotificationService_MarkAllAsRead(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)

	svc.Create(models.NotificationTypeInfo, "N1", "M1")
	svc.Create(models.NotificationTypeInfo, "N2", "M2")

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
		URL:  "http://example.com",
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
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		// Minimal template uses lowercase keys: title, message
		assert.Equal(t, "Test Notification", body["title"])
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	provider := models.NotificationProvider{
		Name:   "Test Webhook",
		Type:   "webhook",
		URL:    ts.URL,
		Template: "minimal",
		Config: `{"Header": "{{.Title}}"}`,
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
	svc.CreateProvider(&provider)

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
	rcvMinimal := make(chan map[string]interface{}, 1)
	tsMin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		rcvMinimal <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer tsMin.Close()

	providerMin := models.NotificationProvider{
		Name:    "Minimal",
		Type:    "webhook",
		URL:     tsMin.URL,
		Enabled: true,
		NotifyUptime: true,
		Template: "minimal",
	}
	svc.CreateProvider(&providerMin)

	data := map[string]interface{}{"Title": "Min Title", "Message": "Min Message", "Time": time.Now().Format(time.RFC3339), "EventType": "uptime"}
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
	rcvDetailed := make(chan map[string]interface{}, 1)
	tsDet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		rcvDetailed <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer tsDet.Close()

	providerDet := models.NotificationProvider{
		Name:    "Detailed",
		Type:    "webhook",
		URL:     tsDet.URL,
		Enabled: true,
		NotifyUptime: true,
		Template: "detailed",
	}
	svc.CreateProvider(&providerDet)

	dataDet := map[string]interface{}{"Title": "Det Title", "Message": "Det Message", "Time": time.Now().Format(time.RFC3339), "EventType": "uptime", "HostName": "example-host", "HostIP": "1.2.3.4", "ServiceCount": 1, "Services": []map[string]interface{}{{"Name": "svc1"}}}
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
	svc.CreateProvider(&provider)
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
	svc.CreateProvider(&provider)

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
		data := map[string]interface{}{"Title": "Test", "Message": "Test Message"}
		err := svc.sendCustomWebhook(context.Background(), provider, data)
		assert.Error(t, err)
	})

	t.Run("unreachable host", func(t *testing.T) {
		provider := models.NotificationProvider{
			Type: "webhook",
			URL:  "http://192.0.2.1:9999", // TEST-NET-1, unreachable
		}
		data := map[string]interface{}{"Title": "Test", "Message": "Test Message"}
		// Set short timeout for client if possible, but here we just expect error
		// Note: http.Client default timeout is 0 (no timeout), but OS might timeout
		// We can't easily change client timeout here without modifying service
		// So we might skip this or just check if it returns error eventually
		// But for unit test speed, we should probably mock or use a closed port on localhost
		// Using a closed port on localhost is faster
		provider.URL = "http://127.0.0.1:54321" // Assuming this port is closed
		err := svc.sendCustomWebhook(context.Background(), provider, data)
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
		data := map[string]interface{}{"Title": "Test", "Message": "Test Message"}
		err := svc.sendCustomWebhook(context.Background(), provider, data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("valid custom payload template", func(t *testing.T) {
		receivedBody := ""
		received := make(chan struct{})
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
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
		data := map[string]interface{}{"Title": "My Title", "Message": "Test Message"}
		svc.sendCustomWebhook(context.Background(), provider, data)

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
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
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
		data := map[string]interface{}{"Title": "Default Title", "Message": "Test Message"}
		svc.sendCustomWebhook(context.Background(), provider, data)

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
	data := map[string]interface{}{"Title": "Test", "Message": "Test"}
	// Build context with requestID value
	ctx := context.WithValue(context.Background(), "requestID", "my-rid")
	err := svc.sendCustomWebhook(ctx, provider, data)
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
			Type: "webhook",
			URL:  ts.URL,
		}
		err := svc.TestProvider(provider)
		assert.NoError(t, err)
	})
}

func TestValidateWebhookURL_PrivateIP(t *testing.T) {
	// Direct IP literal within RFC1918 block should be rejected
	_, err := validateWebhookURL("http://10.0.0.1")
	assert.Error(t, err)

	// Loopback allowed
	u, err := validateWebhookURL("http://127.0.0.1:8080")
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1", u.Hostname())
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
		svc.CreateProvider(&provider)

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
		err = db.Model(&provider).Updates(map[string]interface{}{
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
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
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
		svc.CreateProvider(&provider)

		customData := map[string]interface{}{
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
	data := map[string]interface{}{"Title": "T1", "Message": "M1", "Time": time.Now().Format(time.RFC3339), "EventType": "preview"}
	rendered, parsed, err := svc.RenderTemplate(provider, data)
	require.NoError(t, err)
	assert.Contains(t, rendered, "T1")
	if parsedMap, ok := parsed.(map[string]interface{}); ok {
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

func TestNotificationService_CreateProvider_InvalidCustomTemplate(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)

	t.Run("invalid custom template on create", func(t *testing.T) {
		provider := models.NotificationProvider{
			Name:   "Bad Custom",
			Type:   "webhook",
			URL:    "http://example.com",
			Template: "custom",
			Config: `{"bad": "{{.Title"}`,
		}
		err := svc.CreateProvider(&provider)
		assert.Error(t, err)
	})

	t.Run("invalid custom template on update", func(t *testing.T) {
		provider := models.NotificationProvider{
			Name:   "Valid",
			Type:   "webhook",
			URL:    "http://example.com",
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
