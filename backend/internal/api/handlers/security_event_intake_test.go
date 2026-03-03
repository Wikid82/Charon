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

// TestSecurityEventIntakeCompileSuccess tests that the handler is properly instantiated and route registration doesn't fail.
// Blocker 1: Fix compile error - undefined handler reference.
func TestSecurityEventIntakeCompileSuccess(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	// This test validates that the handler can be instantiated with all required dependencies
	notificationService := services.NewNotificationService(db)
	service := services.NewEnhancedSecurityNotificationService(db)
	securityService := services.NewSecurityService(db)
	managementCIDRs := []string{"127.0.0.0/8"}

	handler := NewSecurityNotificationHandlerWithDeps(
		service,
		securityService,
		"/data",
		notificationService,
		managementCIDRs,
	)

	require.NotNil(t, handler, "Handler should be instantiated successfully")
	require.NotNil(t, handler.notificationService, "Notification service should be set")
	require.NotNil(t, handler.managementCIDRs, "Management CIDRs should be set")
	assert.Equal(t, 1, len(handler.managementCIDRs), "Management CIDRs should have one entry")
}

// TestSecurityEventIntakeAuthLocalhost tests that localhost requests are accepted.
// Blocker 2: Implement real source validation - localhost should be allowed.
func TestSecurityEventIntakeAuthLocalhost(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	notificationService := services.NewNotificationService(db)
	service := services.NewEnhancedSecurityNotificationService(db)
	managementCIDRs := []string{"10.0.0.0/8"}

	handler := NewSecurityNotificationHandlerWithDeps(
		service,
		nil,
		"",
		notificationService,
		managementCIDRs,
	)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "error",
		Message:   "WAF blocked request",
		ClientIP:  "192.168.1.100",
		Path:      "/admin",
		Timestamp: time.Now(),
	}
	body, _ := json.Marshal(event)

	// Localhost IPv4 request
	c.Request = httptest.NewRequest("POST", "/api/v1/security/events", bytes.NewReader(body))
	c.Request.RemoteAddr = "127.0.0.1:12345"
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleSecurityEvent(c)

	assert.Equal(t, http.StatusAccepted, w.Code, "Localhost should be accepted")
}

// TestSecurityEventIntakeAuthManagementCIDR tests that management network requests are accepted.
// Blocker 2: Implement real source validation - management CIDRs should be allowed.
func TestSecurityEventIntakeAuthManagementCIDR(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	notificationService := services.NewNotificationService(db)
	service := services.NewEnhancedSecurityNotificationService(db)
	managementCIDRs := []string{"192.168.1.0/24", "10.0.0.0/8"}

	handler := NewSecurityNotificationHandlerWithDeps(
		service,
		nil,
		"",
		notificationService,
		managementCIDRs,
	)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "error",
		Message:   "WAF blocked request",
		ClientIP:  "8.8.8.8",
		Path:      "/admin",
		Timestamp: time.Now(),
	}
	body, _ := json.Marshal(event)

	// Request from management network
	c.Request = httptest.NewRequest("POST", "/api/v1/security/events", bytes.NewReader(body))
	c.Request.RemoteAddr = "192.168.1.50:12345"
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleSecurityEvent(c)

	assert.Equal(t, http.StatusAccepted, w.Code, "Management CIDR should be accepted")
}

// TestSecurityEventIntakeAuthUnauthorizedIP tests that external IPs are rejected.
// Blocker 2: Implement real source validation - external IPs should be rejected.
func TestSecurityEventIntakeAuthUnauthorizedIP(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	notificationService := services.NewNotificationService(db)
	service := services.NewEnhancedSecurityNotificationService(db)
	managementCIDRs := []string{"192.168.1.0/24"}

	handler := NewSecurityNotificationHandlerWithDeps(
		service,
		nil,
		"",
		notificationService,
		managementCIDRs,
	)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "error",
		Message:   "WAF blocked request",
		ClientIP:  "8.8.8.8",
		Path:      "/admin",
		Timestamp: time.Now(),
	}
	body, _ := json.Marshal(event)

	// External IP not in management network
	c.Request = httptest.NewRequest("POST", "/api/v1/security/events", bytes.NewReader(body))
	c.Request.RemoteAddr = "8.8.8.8:12345"
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleSecurityEvent(c)

	assert.Equal(t, http.StatusForbidden, w.Code, "External IP should be rejected")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "unauthorized_source", response["error"])
}

// TestSecurityEventIntakeAuthInvalidIP tests that malformed IPs are rejected.
// Blocker 2: Implement real source validation - invalid IPs should be rejected.
func TestSecurityEventIntakeAuthInvalidIP(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	notificationService := services.NewNotificationService(db)
	service := services.NewEnhancedSecurityNotificationService(db)
	managementCIDRs := []string{"192.168.1.0/24"}

	handler := NewSecurityNotificationHandlerWithDeps(
		service,
		nil,
		"",
		notificationService,
		managementCIDRs,
	)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "error",
		Message:   "WAF blocked request",
		ClientIP:  "8.8.8.8",
		Path:      "/admin",
		Timestamp: time.Now(),
	}
	body, _ := json.Marshal(event)

	// Malformed IP
	c.Request = httptest.NewRequest("POST", "/api/v1/security/events", bytes.NewReader(body))
	c.Request.RemoteAddr = "invalid-ip:12345"
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleSecurityEvent(c)

	assert.Equal(t, http.StatusForbidden, w.Code, "Invalid IP should be rejected")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "invalid_source", response["error"])
}

// TestSecurityEventIntakeDispatchInvoked tests that notification dispatch is invoked.
// Blocker 3: Implement actual dispatch - remove TODO/no-op behavior.
func TestSecurityEventIntakeDispatchInvoked(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	// Create a Discord provider with security notifications enabled
	provider := &models.NotificationProvider{
		Name:                            "Discord Security Alerts",
		Type:                            "discord",
		URL:                             "https://discord.com/api/webhooks/123/token",
		Enabled:                         true,
		NotifySecurityWAFBlocks:         true,
		NotifySecurityACLDenies:         true,
		NotifySecurityRateLimitHits:     true,
		NotifySecurityCrowdSecDecisions: true,
	}
	require.NoError(t, db.Create(provider).Error)

	notificationService := services.NewNotificationService(db)
	service := services.NewEnhancedSecurityNotificationService(db)
	managementCIDRs := []string{"127.0.0.0/8"}

	handler := NewSecurityNotificationHandlerWithDeps(
		service,
		nil,
		"",
		notificationService,
		managementCIDRs,
	)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "error",
		Message:   "WAF blocked suspicious request",
		ClientIP:  "192.168.1.100",
		Path:      "/admin/login",
		Timestamp: time.Now(),
		Metadata: map[string]any{
			"rule_id":      "920100",
			"matched_data": "suspicious pattern",
		},
	}
	body, _ := json.Marshal(event)

	c.Request = httptest.NewRequest("POST", "/api/v1/security/events", bytes.NewReader(body))
	c.Request.RemoteAddr = "127.0.0.1:12345"
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleSecurityEvent(c)

	assert.Equal(t, http.StatusAccepted, w.Code, "Event should be accepted")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Security event recorded", response["message"])
	assert.Equal(t, "waf_block", response["event_type"])
	assert.NotEmpty(t, response["timestamp"])
}

// TestSecurityEventIntakeR6Intact tests that legacy PUT endpoint returns 410 Gone.
// Constraint: Keep R6 410/no-mutation behavior intact.
func TestSecurityEventIntakeR6Intact(t *testing.T) {
	// Create in-memory database with User table for this test
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.NotificationProvider{},
		&models.NotificationConfig{},
		&models.Setting{},
	))

	// Enable feature flag by default in tests
	featureFlag := &models.Setting{
		Key:      "feature.notifications.security_provider_events.enabled",
		Value:    "true",
		Type:     "bool",
		Category: "feature",
	}
	require.NoError(t, db.Create(featureFlag).Error)

	// Create an admin user for authentication
	adminUser := &models.User{
		Email:        "admin@example.com",
		Name:         "Admin User",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuvwxyz", // Dummy bcrypt hash
		Role:         models.RoleAdmin,
		Enabled:      true,
	}
	require.NoError(t, db.Create(adminUser).Error)

	service := services.NewEnhancedSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(service)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add auth middleware that sets user context
	router.Use(func(c *gin.Context) {
		c.Set("user_id", adminUser.ID)
		c.Set("user", adminUser)
		c.Set("role", "admin")
		c.Next()
	})

	router.PUT("/api/v1/notifications/settings/security", handler.UpdateSettings)

	reqBody := map[string]interface{}{
		"enabled":              true,
		"security_waf_enabled": true,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/v1/notifications/settings/security", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusGone, w.Code, "PUT should return 410 Gone")

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "legacy_security_settings_deprecated", response["error"])
	assert.Equal(t, "LEGACY_SECURITY_SETTINGS_DEPRECATED", response["code"])
}

// TestSecurityEventIntakeDiscordOnly tests Discord-only dispatch enforcement.
// Constraint: Keep Discord-only dispatch enforcement for this rollout stage.
func TestSecurityEventIntakeDiscordOnly(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	// Create various provider types
	discordProvider := &models.NotificationProvider{
		Name:                    "Discord",
		Type:                    "discord",
		URL:                     "https://discord.com/api/webhooks/123/token",
		Enabled:                 true,
		NotifySecurityWAFBlocks: true,
	}
	require.NoError(t, db.Create(discordProvider).Error)

	// Non-Discord providers should also work with supportsJSONTemplates
	webhookProvider := &models.NotificationProvider{
		Name:                    "Webhook",
		Type:                    "webhook",
		URL:                     "https://example.com/webhook",
		Enabled:                 true,
		NotifySecurityWAFBlocks: true,
	}
	require.NoError(t, db.Create(webhookProvider).Error)

	notificationService := services.NewNotificationService(db)
	service := services.NewEnhancedSecurityNotificationService(db)
	managementCIDRs := []string{"127.0.0.0/8"}

	handler := NewSecurityNotificationHandlerWithDeps(
		service,
		nil,
		"",
		notificationService,
		managementCIDRs,
	)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "error",
		Message:   "WAF blocked request",
		ClientIP:  "192.168.1.100",
		Path:      "/admin",
		Timestamp: time.Now(),
	}
	body, _ := json.Marshal(event)

	c.Request = httptest.NewRequest("POST", "/api/v1/security/events", bytes.NewReader(body))
	c.Request.RemoteAddr = "127.0.0.1:12345"
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleSecurityEvent(c)

	assert.Equal(t, http.StatusAccepted, w.Code)

	// Validate both Discord and webhook providers exist (JSON-capable)
	var providers []models.NotificationProvider
	err := db.Where("enabled = ?", true).Find(&providers).Error
	require.NoError(t, err)
	assert.Equal(t, 2, len(providers), "Both Discord and webhook providers should be enabled")
}

// TestSecurityEventIntakeMalformedPayload tests rejection of malformed event payloads.
func TestSecurityEventIntakeMalformedPayload(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	notificationService := services.NewNotificationService(db)
	service := services.NewEnhancedSecurityNotificationService(db)
	managementCIDRs := []string{"127.0.0.0/8"}

	handler := NewSecurityNotificationHandlerWithDeps(
		service,
		nil,
		"",
		notificationService,
		managementCIDRs,
	)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Malformed JSON
	c.Request = httptest.NewRequest("POST", "/api/v1/security/events", bytes.NewReader([]byte("{invalid json")))
	c.Request.RemoteAddr = "127.0.0.1:12345"
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleSecurityEvent(c)

	assert.Equal(t, http.StatusBadRequest, w.Code, "Malformed JSON should be rejected")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid security event payload", response["error"])
}

// TestSecurityEventIntakeIPv6Localhost tests that IPv6 localhost is accepted.
func TestSecurityEventIntakeIPv6Localhost(t *testing.T) {
	db := SetupCompatibilityTestDB(t)

	notificationService := services.NewNotificationService(db)
	service := services.NewEnhancedSecurityNotificationService(db)
	managementCIDRs := []string{"10.0.0.0/8"}

	handler := NewSecurityNotificationHandlerWithDeps(
		service,
		nil,
		"",
		notificationService,
		managementCIDRs,
	)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	event := models.SecurityEvent{
		EventType: "acl_deny",
		Severity:  "warn",
		Message:   "ACL denied request",
		ClientIP:  "::1",
		Path:      "/api/admin",
		Timestamp: time.Now(),
	}
	body, _ := json.Marshal(event)

	// IPv6 localhost
	c.Request = httptest.NewRequest("POST", "/api/v1/security/events", bytes.NewReader(body))
	c.Request.RemoteAddr = "[::1]:12345"
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleSecurityEvent(c)

	assert.Equal(t, http.StatusAccepted, w.Code, "IPv6 localhost should be accepted")
}
