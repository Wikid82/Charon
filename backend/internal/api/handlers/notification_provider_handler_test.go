package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}))

	service := services.NewNotificationService(db)
	handler := handlers.NewNotificationProviderHandler(service)

	r := gin.Default()
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
	req, _ = http.NewRequest("GET", "/api/v1/notifications/providers", nil)
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
	req, _ = http.NewRequest("DELETE", "/api/v1/notifications/providers/"+created.ID, nil)
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

	req, _ := http.NewRequest("GET", "/api/v1/notifications/templates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var templates []map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &templates)
	require.NoError(t, err)
	assert.Len(t, templates, 3)
}

func TestNotificationProviderHandler_Test(t *testing.T) {
	r, _ := setupNotificationProviderTest(t)

	// Test with invalid provider (should fail validation or service check)
	// Since we don't have a real shoutrrr backend mocked easily here without more work,
	// we expect it might fail or pass depending on service implementation.
	// Looking at service code (not shown but assumed), TestProvider likely calls shoutrrr.Send.
	// If URL is invalid, it should error.

	provider := models.NotificationProvider{
		Type: "discord",
		URL:  "invalid-url",
	}
	body, _ := json.Marshal(provider)
	req, _ := http.NewRequest("POST", "/api/v1/notifications/providers/test", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// It should probably fail with 400
	assert.Equal(t, http.StatusBadRequest, w.Code)
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
		Type:     "webhook",
		URL:      "http://example.com",
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
		Type:     "webhook",
		URL:      "http://example.com",
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
		Type:     "webhook",
		URL:      "http://example.com",
		Template: "minimal",
	}
	body, _ := json.Marshal(provider)
	req, _ := http.NewRequest("POST", "/api/v1/notifications/providers/preview", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
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
