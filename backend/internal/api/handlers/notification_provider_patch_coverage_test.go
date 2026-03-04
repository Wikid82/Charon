package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestUpdate_BlockTypeMutationForNonDiscord covers lines 137-139
func TestUpdate_BlockTypeMutationForNonDiscord(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{}))

	// Create existing non-Discord provider
	existing := &models.NotificationProvider{
		ID:      "test-provider",
		Name:    "Test Webhook",
		Type:    "webhook",
		URL:     "https://example.com/webhook",
		Enabled: true,
	}
	require.NoError(t, db.Create(existing).Error)

	service := services.NewNotificationService(db)
	handler := NewNotificationProviderHandler(service)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	r.PUT("/api/v1/notifications/providers/:id", handler.Update)

	// Try to mutate type from webhook to discord (should be blocked)
	req := map[string]interface{}{
		"name": "Updated Name",
		"type": "discord", // Trying to change type
		"url":  "https://discord.com/api/webhooks/123/abc",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodPut, "/api/v1/notifications/providers/test-provider", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	// Should block type mutation (lines 137-139)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "PROVIDER_TYPE_IMMUTABLE", response["code"])
}

// TestUpdate_AllowTypeMutationForDiscord verifies Discord can be updated
func TestUpdate_AllowTypeMutationForDiscord(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{}))

	// Create existing Discord provider
	existing := &models.NotificationProvider{
		ID:      "test-provider",
		Name:    "Test Discord",
		Type:    "discord",
		URL:     "https://discord.com/api/webhooks/123/abc",
		Enabled: true,
	}
	require.NoError(t, db.Create(existing).Error)

	service := services.NewNotificationService(db)
	handler := NewNotificationProviderHandler(service)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	r.PUT("/api/v1/notifications/providers/:id", handler.Update)

	// Try to update Discord (type remains discord - should be allowed)
	req := map[string]interface{}{
		"name": "Updated Discord",
		"type": "discord",
		"url":  "https://discord.com/api/webhooks/456/def",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodPut, "/api/v1/notifications/providers/test-provider", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	// Should succeed
	assert.Equal(t, http.StatusOK, w.Code)
}
