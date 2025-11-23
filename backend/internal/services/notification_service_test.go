package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"net/http/httptest"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
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
		assert.Equal(t, "Test Notification", body["Title"])
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	provider := models.NotificationProvider{
		Name:   "Test Webhook",
		Type:   "webhook",
		URL:    ts.URL,
		Config: `{"Title": "{{.Title}}"}`,
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

	svc.SendExternal("proxy_host", "Title", "Message", nil)

	select {
	case <-received:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for webhook")
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

	svc.SendExternal("proxy_host", "Title", "Message", nil)

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
	svc.SendExternal("proxy_host", "Title", "Message", nil)

	// Give it a moment to run goroutine
	time.Sleep(100 * time.Millisecond)
}
