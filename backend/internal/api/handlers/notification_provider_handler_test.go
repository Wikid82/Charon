package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

func setupNotificationProviderTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db := handlers.OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{}))

	service := services.NewNotificationService(db)
	handler := handlers.NewNotificationProviderHandler(service)

	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	api := r.Group("/api/v1")
	providers := api.Group("/notifications/providers")
	providers.GET("", handler.List)
	providers.POST("/preview", handler.Preview)
	providers.POST("", handler.Create)
	providers.PUT("/:id", handler.Update)
	providers.DELETE("/:id", handler.Delete)
	providers.POST("/test", handler.Test)
	api.GET("/notifications/templates", handler.Templates)

	return r, db
}

func TestNotificationProviderHandler_CRUD(t *testing.T) {
	r, db := setupNotificationProviderTest(t)

	// 1. Create
	provider := models.NotificationProvider{
		Name: "Test Discord",
		Type: "discord",
		URL:  "https://discord.com/api/webhooks/...",
	}
	body, _ := json.Marshal(provider)
	req, _ := http.NewRequest("POST", "/api/v1/notifications/providers", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var created models.NotificationProvider
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)
	assert.Equal(t, provider.Name, created.Name)
	assert.NotEmpty(t, created.ID)

	// 2. List
	req, _ = http.NewRequest("GET", "/api/v1/notifications/providers", http.NoBody)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var list []models.NotificationProvider
	err = json.Unmarshal(w.Body.Bytes(), &list)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// 3. Update
	created.Name = "Updated Discord"
	body, _ = json.Marshal(created)
	req, _ = http.NewRequest("PUT", "/api/v1/notifications/providers/"+created.ID, bytes.NewBuffer(body))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var updated models.NotificationProvider
	err = json.Unmarshal(w.Body.Bytes(), &updated)
	require.NoError(t, err)
	assert.Equal(t, "Updated Discord", updated.Name)

	// Verify in DB
	var dbProvider models.NotificationProvider
	db.First(&dbProvider, "id = ?", created.ID)
	assert.Equal(t, "Updated Discord", dbProvider.Name)

	// 4. Delete
	req, _ = http.NewRequest("DELETE", "/api/v1/notifications/providers/"+created.ID, http.NoBody)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify Delete
	var count int64
	db.Model(&models.NotificationProvider{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestNotificationProviderHandler_Templates(t *testing.T) {
	r, _ := setupNotificationProviderTest(t)

	req, _ := http.NewRequest("GET", "/api/v1/notifications/templates", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var templates []map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &templates)
	require.NoError(t, err)
	assert.Len(t, templates, 3)
}

func TestNotificationProviderHandler_Test(t *testing.T) {
	r, db := setupNotificationProviderTest(t)

	stored := models.NotificationProvider{
		ID:      "trusted-provider-id",
		Name:    "Stored Provider",
		Type:    "discord",
		URL:     "invalid-url",
		Enabled: true,
	}
	require.NoError(t, db.Create(&stored).Error)

	payload := map[string]any{
		"id":   stored.ID,
		"type": "discord",
		"url":  "https://discord.com/api/webhooks/123/override",
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/v1/notifications/providers/test", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "PROVIDER_TEST_FAILED")
}

func TestNotificationProviderHandler_Test_RequiresTrustedProviderID(t *testing.T) {
	r, _ := setupNotificationProviderTest(t)

	payload := map[string]any{
		"type": "discord",
		"url":  "https://discord.com/api/webhooks/123/abc",
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/v1/notifications/providers/test", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "MISSING_PROVIDER_ID")
}

func TestNotificationProviderHandler_Test_ReturnsNotFoundForUnknownProvider(t *testing.T) {
	r, _ := setupNotificationProviderTest(t)

	payload := map[string]any{
		"id": "missing-provider-id",
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/v1/notifications/providers/test", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "PROVIDER_NOT_FOUND")
}

func TestNotificationProviderHandler_Errors(t *testing.T) {
	r, _ := setupNotificationProviderTest(t)

	// Create Invalid JSON
	req, _ := http.NewRequest("POST", "/api/v1/notifications/providers", bytes.NewBuffer([]byte("invalid")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Update Invalid JSON
	req, _ = http.NewRequest("PUT", "/api/v1/notifications/providers/123", bytes.NewBuffer([]byte("invalid")))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Test Invalid JSON
	req, _ = http.NewRequest("POST", "/api/v1/notifications/providers/test", bytes.NewBuffer([]byte("invalid")))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNotificationProviderHandler_InvalidCustomTemplate_Rejects(t *testing.T) {
	r, _ := setupNotificationProviderTest(t)

	// Create with invalid custom template should return 400
	provider := models.NotificationProvider{
		Name:     "Bad",
		Type:     "discord",
		URL:      "https://discord.com/api/webhooks/123/abc",
		Template: "custom",
		Config:   `{"broken": "{{.Title"}`,
	}
	body, _ := json.Marshal(provider)
	req, _ := http.NewRequest("POST", "/api/v1/notifications/providers", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Create valid and then attempt update to invalid custom template
	provider = models.NotificationProvider{
		Name:     "Good",
		Type:     "discord",
		URL:      "https://discord.com/api/webhooks/456/def",
		Template: "minimal",
	}
	body, _ = json.Marshal(provider)
	req, _ = http.NewRequest("POST", "/api/v1/notifications/providers", bytes.NewBuffer(body))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
	var created models.NotificationProvider
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	created.Template = "custom"
	created.Config = `{"broken": "{{.Title"}`
	body, _ = json.Marshal(created)
	req, _ = http.NewRequest("PUT", "/api/v1/notifications/providers/"+created.ID, bytes.NewBuffer(body))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNotificationProviderHandler_Preview(t *testing.T) {
	r, _ := setupNotificationProviderTest(t)

	// Minimal template preview
	provider := models.NotificationProvider{
		Type:     "discord",
		URL:      "https://discord.com/api/webhooks/123/abc",
		Template: "minimal",
	}
	body, _ := json.Marshal(provider)
	req, _ := http.NewRequest("POST", "/api/v1/notifications/providers/preview", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp, "rendered")
	assert.Contains(t, resp, "parsed")

	// Invalid template should not succeed
	provider.Config = `{"broken": "{{.Title"}`
	provider.Template = "custom"
	body, _ = json.Marshal(provider)
	req, _ = http.NewRequest("POST", "/api/v1/notifications/providers/preview", bytes.NewBuffer(body))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNotificationProviderHandler_CreateRejectsDiscordIPHost(t *testing.T) {
	r, _ := setupNotificationProviderTest(t)

	provider := models.NotificationProvider{
		Name: "Discord IP",
		Type: "discord",
		URL:  "https://203.0.113.10/api/webhooks/123456/token_abc",
	}
	body, _ := json.Marshal(provider)
	req, _ := http.NewRequest("POST", "/api/v1/notifications/providers", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "PROVIDER_VALIDATION_FAILED")
	assert.Contains(t, w.Body.String(), "validation")
}

func TestNotificationProviderHandler_CreateAcceptsDiscordHostname(t *testing.T) {
	r, _ := setupNotificationProviderTest(t)

	provider := models.NotificationProvider{
		Name: "Discord Host",
		Type: "discord",
		URL:  "https://discord.com/api/webhooks/123456/token_abc",
	}
	body, _ := json.Marshal(provider)
	req, _ := http.NewRequest("POST", "/api/v1/notifications/providers", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestNotificationProviderHandler_CreateIgnoresServerManagedMigrationFields(t *testing.T) {
	r, db := setupNotificationProviderTest(t)

	payload := map[string]any{
		"name":                  "Create Ignore Migration",
		"type":                  "discord",
		"url":                   "https://discord.com/api/webhooks/123/abc",
		"template":              "minimal",
		"enabled":               true,
		"notify_proxy_hosts":    true,
		"notify_remote_servers": true,
		"notify_domains":        true,
		"notify_certs":          true,
		"notify_uptime":         true,
		"engine":                "notify_v1",
		"service_config":        `{"token":"attacker"}`,
		"migration_state":       "migrated",
		"migration_error":       "client-value",
		"legacy_url":            "https://malicious.example",
		"last_migrated_at":      "2020-01-01T00:00:00Z",
		"id":                    "client-controlled-id",
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/v1/notifications/providers", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var created models.NotificationProvider
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	assert.NotEqual(t, "client-controlled-id", created.ID)

	var dbProvider models.NotificationProvider
	err = db.First(&dbProvider, "id = ?", created.ID).Error
	require.NoError(t, err)
	assert.Empty(t, dbProvider.Engine)
	assert.Empty(t, dbProvider.ServiceConfig)
	assert.Empty(t, dbProvider.MigrationState)
	assert.Empty(t, dbProvider.MigrationError)
	assert.Empty(t, dbProvider.LegacyURL)
	assert.Nil(t, dbProvider.LastMigratedAt)
}

func TestNotificationProviderHandler_UpdatePreservesServerManagedMigrationFields(t *testing.T) {
	r, db := setupNotificationProviderTest(t)

	now := time.Now().UTC().Round(time.Second)
	original := models.NotificationProvider{
		Name:                "Original",
		Type:                "discord",
		URL:                 "https://discord.com/api/webhooks/123/abc",
		Template:            "minimal",
		Enabled:             true,
		NotifyProxyHosts:    true,
		NotifyRemoteServers: true,
		NotifyDomains:       true,
		NotifyCerts:         true,
		NotifyUptime:        true,
		Engine:              "notify_v1",
		ServiceConfig:       `{"token":"server"}`,
		MigrationState:      "migrated",
		MigrationError:      "",
		LegacyURL:           "discord://legacy",
		LastMigratedAt:      &now,
	}
	require.NoError(t, db.Create(&original).Error)

	payload := map[string]any{
		"name":                  "Updated Name",
		"type":                  "discord",
		"url":                   "https://discord.com/api/webhooks/456/def",
		"template":              "minimal",
		"enabled":               false,
		"notify_proxy_hosts":    false,
		"notify_remote_servers": false,
		"notify_domains":        false,
		"notify_certs":          false,
		"notify_uptime":         false,
		"engine":                "legacy",
		"service_config":        `{"token":"client-overwrite"}`,
		"migration_state":       "failed",
		"migration_error":       "client-error",
		"legacy_url":            "https://attacker.example",
		"last_migrated_at":      "1999-01-01T00:00:00Z",
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", "/api/v1/notifications/providers/"+original.ID, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var dbProvider models.NotificationProvider
	require.NoError(t, db.First(&dbProvider, "id = ?", original.ID).Error)
	assert.Equal(t, "Updated Name", dbProvider.Name)
	assert.Equal(t, "notify_v1", dbProvider.Engine)
	assert.Equal(t, `{"token":"server"}`, dbProvider.ServiceConfig)
	assert.Equal(t, "migrated", dbProvider.MigrationState)
	assert.Equal(t, "", dbProvider.MigrationError)
	assert.Equal(t, "discord://legacy", dbProvider.LegacyURL)
	require.NotNil(t, dbProvider.LastMigratedAt)
	assert.Equal(t, now, dbProvider.LastMigratedAt.UTC().Round(time.Second))
}
