package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestDeprecatedGetSettings_HeadersSet covers lines 64-68
func TestDeprecatedGetSettings_HeadersSet(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.NotificationConfig{}, &models.Setting{}))

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(service)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/notifications/settings/legacy/security", http.NoBody)

	handler.DeprecatedGetSettings(c)

	assert.Equal(t, http.StatusOK, w.Code)
	// Lines 64-68: deprecated headers should be set
	assert.Equal(t, "true", w.Header().Get("X-Charon-Deprecated"))
	assert.Equal(t, "/api/v1/notifications/settings/security", w.Header().Get("X-Charon-Canonical-Endpoint"))
}

// TestHandleSecurityEvent_InvalidCIDRWarning covers lines 119-120
func TestHandleSecurityEvent_InvalidCIDRWarning(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.NotificationConfig{}, &models.Setting{}))

	service := services.NewEnhancedSecurityNotificationService(db)

	// Invalid CIDR that will trigger warning
	invalidCIDRs := []string{"invalid-cidr", "192.168.1.1/33"}
	handler := NewSecurityNotificationHandlerWithDeps(
		service,
		nil,
		"/tmp",
		nil,
		invalidCIDRs,
	)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "warn",
		Message:   "Test",
		ClientIP:  "192.168.1.1",
		Path:      "/test",
		Timestamp: time.Now(),
	}
	body, _ := json.Marshal(event)
	c.Request = httptest.NewRequest("POST", "/api/v1/security/events", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.RemoteAddr = "127.0.0.1:12345"

	handler.HandleSecurityEvent(c)

	// Should still accept (line 119-120 logs warning but continues)
	assert.Equal(t, http.StatusAccepted, w.Code)
}

// TestHandleSecurityEvent_SeveritySet covers line 146
func TestHandleSecurityEvent_SeveritySet(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.NotificationConfig{}, &models.Setting{}))

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandlerWithDeps(
		service,
		nil,
		"/tmp",
		nil,
		[]string{},
	)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "critical",
		Message:   "Test",
		ClientIP:  "192.168.1.1",
		Path:      "/test",
		Timestamp: time.Now(),
	}
	body, _ := json.Marshal(event)
	c.Request = httptest.NewRequest("POST", "/api/v1/security/events", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.RemoteAddr = "127.0.0.1:12345"

	handler.HandleSecurityEvent(c)

	assert.Equal(t, http.StatusAccepted, w.Code)

	// Line 146: severity should be set in context
	severity, exists := c.Get("security_event_severity")
	assert.True(t, exists)
	assert.Equal(t, "critical", severity)
}

// TestHandleSecurityEvent_DispatchError covers lines 163-164
func TestHandleSecurityEvent_DispatchError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.NotificationConfig{}, &models.Setting{}))

	// Enable feature flag
	db.Create(&models.Setting{
		Key:      "feature.notifications.security_provider_events.enabled",
		Value:    "true",
		Type:     "bool",
		Category: "feature",
	})

	// Create provider with invalid URL to trigger dispatch error
	db.Create(&models.NotificationProvider{
		ID:                      "test",
		Name:                    "Test",
		Type:                    "discord",
		URL:                     "http://invalid-url-that-will-fail",
		Enabled:                 true,
		NotifySecurityWAFBlocks: true,
	})

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandlerWithDeps(
		service,
		nil,
		"/tmp",
		nil,
		[]string{},
	)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "warn",
		Message:   "Test",
		ClientIP:  "192.168.1.1",
		Path:      "/test",
		Timestamp: time.Now(),
	}
	body, _ := json.Marshal(event)
	c.Request = httptest.NewRequest("POST", "/api/v1/security/events", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.RemoteAddr = "127.0.0.1:12345"

	handler.HandleSecurityEvent(c)

	// Should still return 202 even if dispatch fails (lines 163-164 log error)
	assert.Equal(t, http.StatusAccepted, w.Code)
}
