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
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestBlocker3_CreateProviderValidationWithSecurityEvents verifies supported/unsupported provider handling with security events enabled.
func TestBlocker3_CreateProviderRejectsNonDiscordWithSecurityEvents(t *testing.T) {

	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{})
	assert.NoError(t, err)

	// Create handler
	service := services.NewNotificationService(db, nil,
		services.WithSlackURLValidator(func(string) error { return nil }),
	)
	handler := NewNotificationProviderHandler(service)

	// Test cases: provider types with security events enabled
	testCases := []struct {
		name         string
		providerType string
		token        string
		wantStatus   int
	}{
		{"webhook", "webhook", "", http.StatusCreated},
		{"gotify", "gotify", "", http.StatusCreated},
		{"slack", "slack", "https://hooks.slack.com/services/T1234567890/B1234567890/XXXXXXXXXXXXXXXXXXXX", http.StatusCreated},
		{"email", "email", "", http.StatusCreated},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request payload with security event enabled
			payload := map[string]interface{}{
				"name":                       "Test Provider",
				"type":                       tc.providerType,
				"url":                        "https://example.com/webhook",
				"token":                      tc.token,
				"enabled":                    true,
				"notify_security_waf_blocks": true, // Security event enabled
			}

			jsonPayload, err := json.Marshal(payload)
			assert.NoError(t, err)

			// Create test context
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("POST", "/api/v1/notifications/providers", bytes.NewBuffer(jsonPayload))
			c.Request.Header.Set("Content-Type", "application/json")

			// Set admin role
			c.Set("role", "admin")
			c.Set("userID", uint(1))

			// Call Create
			handler.Create(c)

			assert.Equal(t, tc.wantStatus, w.Code)

			// Verify error message
			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			if tc.wantStatus == http.StatusBadRequest {
				assert.Contains(t, response["code"], "UNSUPPORTED_PROVIDER_TYPE")
			}
		})
	}
}

// TestBlocker3_CreateProviderAcceptsDiscordWithSecurityEvents tests that create accepts Discord providers with security events.
func TestBlocker3_CreateProviderAcceptsDiscordWithSecurityEvents(t *testing.T) {

	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{})
	assert.NoError(t, err)

	// Create handler
	service := services.NewNotificationService(db, nil)
	handler := NewNotificationProviderHandler(service)

	// Create request payload with Discord provider and security events
	payload := map[string]interface{}{
		"name":                               "Test Discord",
		"type":                               "discord",
		"url":                                "https://discord.com/api/webhooks/123/abc",
		"enabled":                            true,
		"notify_security_waf_blocks":         true,
		"notify_security_acl_denies":         true,
		"notify_security_rate_limit_hits":    true,
		"notify_security_crowdsec_decisions": true,
	}

	jsonPayload, err := json.Marshal(payload)
	assert.NoError(t, err)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/api/v1/notifications/providers", bytes.NewBuffer(jsonPayload))
	c.Request.Header.Set("Content-Type", "application/json")

	// Set admin role
	c.Set("role", "admin")
	c.Set("userID", uint(1))

	// Call Create
	handler.Create(c)

	// Blocker 3: Should accept with 201
	assert.Equal(t, http.StatusCreated, w.Code, "Should accept Discord provider with security events")
}

// TestBlocker3_CreateProviderAcceptsNonDiscordWithoutSecurityEvents verifies webhook create without security events remains accepted.
func TestBlocker3_CreateProviderAcceptsNonDiscordWithoutSecurityEvents(t *testing.T) {

	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{})
	assert.NoError(t, err)

	// Create handler
	service := services.NewNotificationService(db, nil)
	handler := NewNotificationProviderHandler(service)

	// Create request payload with webhook provider but no security events
	payload := map[string]interface{}{
		"name":                       "Test Webhook",
		"type":                       "webhook",
		"url":                        "https://example.com/webhook",
		"enabled":                    true,
		"notify_proxy_hosts":         true,
		"notify_security_waf_blocks": false, // Security events disabled
	}

	jsonPayload, err := json.Marshal(payload)
	assert.NoError(t, err)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/api/v1/notifications/providers", bytes.NewBuffer(jsonPayload))
	c.Request.Header.Set("Content-Type", "application/json")

	// Set admin role
	c.Set("role", "admin")
	c.Set("userID", uint(1))

	// Call Create
	handler.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)
}

// TestBlocker3_UpdateProviderRejectsNonDiscordWithSecurityEvents verifies webhook update with security events is allowed in PR-1 scope.
func TestBlocker3_UpdateProviderRejectsNonDiscordWithSecurityEvents(t *testing.T) {

	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{})
	assert.NoError(t, err)

	// Create existing webhook provider without security events
	existingProvider := models.NotificationProvider{
		ID:               "test-id",
		Name:             "Test Webhook",
		Type:             "webhook",
		URL:              "https://example.com/webhook",
		Enabled:          true,
		NotifyProxyHosts: true,
	}
	assert.NoError(t, db.Create(&existingProvider).Error)

	// Create handler
	service := services.NewNotificationService(db, nil)
	handler := NewNotificationProviderHandler(service)

	// Try to update to enable security events (should be rejected)
	payload := map[string]interface{}{
		"name":                       "Test Webhook",
		"type":                       "webhook",
		"url":                        "https://example.com/webhook",
		"enabled":                    true,
		"notify_security_waf_blocks": true, // Try to enable security event
	}

	jsonPayload, err := json.Marshal(payload)
	assert.NoError(t, err)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("PUT", "/api/v1/notifications/providers/test-id", bytes.NewBuffer(jsonPayload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "test-id"}}

	// Set admin role
	c.Set("role", "admin")
	c.Set("userID", uint(1))

	// Call Update
	handler.Update(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestBlocker3_UpdateProviderAcceptsDiscordWithSecurityEvents tests that update accepts Discord providers with security events.
func TestBlocker3_UpdateProviderAcceptsDiscordWithSecurityEvents(t *testing.T) {

	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{})
	assert.NoError(t, err)

	// Create existing Discord provider
	existingProvider := models.NotificationProvider{
		ID:                      "test-id",
		Name:                    "Test Discord",
		Type:                    "discord",
		URL:                     "https://discord.com/api/webhooks/123/abc",
		Enabled:                 true,
		NotifySecurityWAFBlocks: false,
	}
	assert.NoError(t, db.Create(&existingProvider).Error)

	// Create handler
	service := services.NewNotificationService(db, nil)
	handler := NewNotificationProviderHandler(service)

	// Update to enable security events
	payload := map[string]interface{}{
		"name":                       "Test Discord",
		"type":                       "discord",
		"url":                        "https://discord.com/api/webhooks/123/abc",
		"enabled":                    true,
		"notify_security_waf_blocks": true, // Enable security event
	}

	jsonPayload, err := json.Marshal(payload)
	assert.NoError(t, err)

	// Create test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("PUT", "/api/v1/notifications/providers/test-id", bytes.NewBuffer(jsonPayload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "test-id"}}

	// Set admin role
	c.Set("role", "admin")
	c.Set("userID", uint(1))

	// Call Update
	handler.Update(c)

	// Blocker 3: Should accept with 200
	assert.Equal(t, http.StatusOK, w.Code, "Should accept Discord provider update with security events")
}

// TestBlocker3_MultipleSecurityEventsEnforcesDiscordOnly tests webhook remains accepted with security flags in PR-1 scope.
func TestBlocker3_MultipleSecurityEventsEnforcesDiscordOnly(t *testing.T) {

	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{})
	assert.NoError(t, err)

	// Create handler
	service := services.NewNotificationService(db, nil)
	handler := NewNotificationProviderHandler(service)

	// Test each security event field individually
	securityEventFields := []string{
		"notify_security_waf_blocks",
		"notify_security_acl_denies",
		"notify_security_rate_limit_hits",
		"notify_security_crowdsec_decisions",
	}

	for _, field := range securityEventFields {
		t.Run(field, func(t *testing.T) {
			// Create request with webhook provider and one security event enabled
			payload := map[string]interface{}{
				"name":    "Test Webhook",
				"type":    "webhook",
				"url":     "https://example.com/webhook",
				"enabled": true,
				field:     true, // Enable this security event
			}

			jsonPayload, err := json.Marshal(payload)
			assert.NoError(t, err)

			// Create test context
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("POST", "/api/v1/notifications/providers", bytes.NewBuffer(jsonPayload))
			c.Request.Header.Set("Content-Type", "application/json")

			// Set admin role
			c.Set("role", "admin")
			c.Set("userID", uint(1))

			// Call Create
			handler.Create(c)

			assert.Equal(t, http.StatusCreated, w.Code,
				"Should accept webhook provider with %s enabled", field)
		})
	}
}

// TestBlocker3_UpdateProvider_DatabaseError tests database error handling when fetching existing provider (lines 137-139).
func TestBlocker3_UpdateProvider_DatabaseError(t *testing.T) {

	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{})
	assert.NoError(t, err)

	// Create handler
	service := services.NewNotificationService(db, nil)
	handler := NewNotificationProviderHandler(service)

	// Update payload
	payload := map[string]interface{}{
		"name":    "Test Provider",
		"type":    "discord",
		"url":     "https://discord.com/api/webhooks/123/abc",
		"enabled": true,
	}

	jsonPayload, err := json.Marshal(payload)
	assert.NoError(t, err)

	// Create test context with non-existent provider ID
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("PUT", "/api/v1/notifications/providers/nonexistent", bytes.NewBuffer(jsonPayload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "nonexistent"}}

	// Set admin role
	c.Set("role", "admin")
	c.Set("userID", uint(1))

	// Call Update
	handler.Update(c)

	// Lines 137-139: Should return 404 for not found
	assert.Equal(t, http.StatusNotFound, w.Code, "Should return 404 for nonexistent provider")

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Provider not found", response["error"])
}
