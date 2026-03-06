package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// TestHandleSecurityEvent_TimestampZero covers line 146
func TestHandleSecurityEvent_TimestampZero(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&models.Setting{}, &models.NotificationProvider{})
	assert.NoError(t, err)

	// Enable feature flag
	setting := models.Setting{
		Key:      "feature.notifications.security_provider_events.enabled",
		Value:    "true",
		Type:     "bool",
		Category: "feature",
	}
	assert.NoError(t, db.Create(&setting).Error)

	enhancedService := services.NewEnhancedSecurityNotificationService(db)
	securityService := services.NewSecurityService(db)
	notificationService := services.NewNotificationService(db, nil)
	h := NewSecurityNotificationHandlerWithDeps(enhancedService, securityService, "/tmp", notificationService, []string{"127.0.0.0/8"})

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	// Event with zero timestamp
	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "warn",
		Message:   "Test",
		ClientIP:  "127.0.0.1",
		Path:      "/test",
		// Timestamp not set - should trigger line 146
	}

	body, _ := json.Marshal(event)
	ctx.Request, _ = http.NewRequest("POST", "/api/v1/security/events", bytes.NewReader(body))
	ctx.Request.RemoteAddr = "127.0.0.1:12345"

	h.HandleSecurityEvent(ctx)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

// mockFailingService is a mock service that always fails SendViaProviders
type mockFailingService struct {
	services.EnhancedSecurityNotificationService
}

func (m *mockFailingService) SendViaProviders(ctx context.Context, event models.SecurityEvent) error {
	return errors.New("mock provider failure")
}

// TestHandleSecurityEvent_SendViaProvidersError covers lines 163-164
func TestHandleSecurityEvent_SendViaProvidersError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&models.Setting{})
	assert.NoError(t, err)

	securityService := services.NewSecurityService(db)
	notificationService := services.NewNotificationService(db, nil)
	mockService := &mockFailingService{}
	h := NewSecurityNotificationHandlerWithDeps(mockService, securityService, "/tmp", notificationService, []string{"127.0.0.0/8"})

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	event := models.SecurityEvent{
		EventType: "acl_deny",
		Severity:  "warn",
		Message:   "Test",
		ClientIP:  "127.0.0.1",
		Path:      "/test",
		Timestamp: time.Now(),
	}

	body, _ := json.Marshal(event)
	ctx.Request, _ = http.NewRequest("POST", "/api/v1/security/events", bytes.NewReader(body))
	ctx.Request.RemoteAddr = "127.0.0.1:12345"

	// Should continue and return Accepted even when SendViaProviders fails (lines 163-164)
	h.HandleSecurityEvent(ctx)

	assert.Equal(t, http.StatusAccepted, w.Code)
}
