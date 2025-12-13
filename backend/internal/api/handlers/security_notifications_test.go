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

func setupSecNotifTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationConfig{}))
	return db
}

func TestSecurityNotificationHandler_GetSettings(t *testing.T) {
	db := setupSecNotifTestDB(t)
	svc := services.NewSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(svc)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/security/notifications/settings", http.NoBody)

	handler.GetSettings(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecurityNotificationHandler_UpdateSettings(t *testing.T) {
	db := setupSecNotifTestDB(t)
	svc := services.NewSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(svc)

	body := models.NotificationConfig{
		Enabled:     true,
		MinLogLevel: "warn",
	}
	bodyBytes, _ := json.Marshal(body)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/settings", bytes.NewBuffer(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecurityNotificationHandler_InvalidLevel(t *testing.T) {
	db := setupSecNotifTestDB(t)
	svc := services.NewSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(svc)

	body := models.NotificationConfig{
		MinLogLevel: "invalid",
	}
	bodyBytes, _ := json.Marshal(body)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/settings", bytes.NewBuffer(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecurityNotificationHandler_UpdateSettings_InvalidJSON(t *testing.T) {
	db := setupSecNotifTestDB(t)
	svc := services.NewSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(svc)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/settings", bytes.NewBufferString("{invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecurityNotificationHandler_UpdateSettings_ValidLevels(t *testing.T) {
	db := setupSecNotifTestDB(t)
	svc := services.NewSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(svc)

	validLevels := []string{"debug", "info", "warn", "error"}

	for _, level := range validLevels {
		body := models.NotificationConfig{
			Enabled:     true,
			MinLogLevel: level,
		}
		bodyBytes, _ := json.Marshal(body)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("PUT", "/settings", bytes.NewBuffer(bodyBytes))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.UpdateSettings(c)

		assert.Equal(t, http.StatusOK, w.Code, "Level %s should be valid", level)
	}
}

func TestSecurityNotificationHandler_GetSettings_DatabaseError(t *testing.T) {
	db := setupSecNotifTestDB(t)
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	svc := services.NewSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(svc)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/security/notifications/settings", http.NoBody)

	handler.GetSettings(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSecurityNotificationHandler_GetSettings_EmptySettings(t *testing.T) {
	db := setupSecNotifTestDB(t)
	svc := services.NewSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(svc)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/security/notifications/settings", http.NoBody)

	handler.GetSettings(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.NotificationConfig
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Enabled)
	assert.Equal(t, "error", resp.MinLogLevel)
}
