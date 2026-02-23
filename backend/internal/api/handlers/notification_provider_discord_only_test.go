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

// TestDiscordOnly_CreateRejectsNonDiscord tests that create globally rejects non-Discord providers.
func TestDiscordOnly_CreateRejectsNonDiscord(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{}))

	service := services.NewNotificationService(db)
	handler := NewNotificationProviderHandler(service)

	testCases := []struct {
		name         string
		providerType string
	}{
		{"webhook", "webhook"},
		{"slack", "slack"},
		{"gotify", "gotify"},
		{"telegram", "telegram"},
		{"generic", "generic"},
		{"email", "email"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]interface{}{
				"name":               "Test Provider",
				"type":               tc.providerType,
				"url":                "https://example.com/webhook",
				"enabled":            true,
				"notify_proxy_hosts": true,
			}

			jsonPayload, err := json.Marshal(payload)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("POST", "/api/v1/notifications/providers", bytes.NewBuffer(jsonPayload))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set("role", "admin")
			c.Set("userID", uint(1))

			handler.Create(c)

			assert.Equal(t, http.StatusBadRequest, w.Code, "Should reject non-Discord provider")

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, "PROVIDER_TYPE_DISCORD_ONLY", response["code"])
			assert.Contains(t, response["error"], "discord")
		})
	}
}

// TestDiscordOnly_CreateAcceptsDiscord tests that create accepts Discord providers.
func TestDiscordOnly_CreateAcceptsDiscord(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{}))

	service := services.NewNotificationService(db)
	handler := NewNotificationProviderHandler(service)

	payload := map[string]interface{}{
		"name":               "Test Discord",
		"type":               "discord",
		"url":                "https://discord.com/api/webhooks/123/abc",
		"enabled":            true,
		"notify_proxy_hosts": true,
	}

	jsonPayload, err := json.Marshal(payload)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/api/v1/notifications/providers", bytes.NewBuffer(jsonPayload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("role", "admin")
	c.Set("userID", uint(1))

	handler.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code, "Should accept Discord provider")
}

// TestDiscordOnly_UpdateRejectsTypeMutation tests that update blocks type mutation for deprecated providers.
func TestDiscordOnly_UpdateRejectsTypeMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{}))

	// Create a deprecated webhook provider
	deprecatedProvider := models.NotificationProvider{
		ID:               "test-deprecated",
		Name:             "Deprecated Webhook",
		Type:             "webhook",
		URL:              "https://example.com/webhook",
		Enabled:          false,
		MigrationState:   "deprecated",
		NotifyProxyHosts: true,
	}
	require.NoError(t, db.Create(&deprecatedProvider).Error)

	service := services.NewNotificationService(db)
	handler := NewNotificationProviderHandler(service)

	// Try to change type to discord
	payload := map[string]interface{}{
		"name":               "Deprecated Webhook",
		"type":               "discord",
		"url":                "https://discord.com/api/webhooks/123/abc",
		"enabled":            false,
		"notify_proxy_hosts": true,
	}

	jsonPayload, err := json.Marshal(payload)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("PUT", "/api/v1/notifications/providers/test-deprecated", bytes.NewBuffer(jsonPayload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "test-deprecated"}}
	c.Set("role", "admin")
	c.Set("userID", uint(1))

	handler.Update(c)

	assert.Equal(t, http.StatusBadRequest, w.Code, "Should reject type mutation for deprecated provider")

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "DEPRECATED_PROVIDER_TYPE_IMMUTABLE", response["code"])
	assert.Contains(t, response["error"], "cannot change provider type")
}

// TestDiscordOnly_UpdateRejectsEnable tests that update blocks enabling deprecated providers.
func TestDiscordOnly_UpdateRejectsEnable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{}))

	// Create a deprecated webhook provider (disabled)
	deprecatedProvider := models.NotificationProvider{
		ID:               "test-deprecated",
		Name:             "Deprecated Webhook",
		Type:             "webhook",
		URL:              "https://example.com/webhook",
		Enabled:          false,
		MigrationState:   "deprecated",
		NotifyProxyHosts: true,
	}
	require.NoError(t, db.Create(&deprecatedProvider).Error)

	service := services.NewNotificationService(db)
	handler := NewNotificationProviderHandler(service)

	// Try to enable the deprecated provider
	payload := map[string]interface{}{
		"name":               "Deprecated Webhook",
		"type":               "webhook",
		"url":                "https://example.com/webhook",
		"enabled":            true, // Try to enable
		"notify_proxy_hosts": true,
	}

	jsonPayload, err := json.Marshal(payload)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("PUT", "/api/v1/notifications/providers/test-deprecated", bytes.NewBuffer(jsonPayload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "test-deprecated"}}
	c.Set("role", "admin")
	c.Set("userID", uint(1))

	handler.Update(c)

	assert.Equal(t, http.StatusBadRequest, w.Code, "Should reject enabling deprecated provider")

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "DEPRECATED_PROVIDER_CANNOT_ENABLE", response["code"])
	assert.Contains(t, response["error"], "cannot enable deprecated")
}

// TestDiscordOnly_UpdateAllowsDisabledDeprecated tests that update allows updating disabled deprecated providers (except type/enable).
func TestDiscordOnly_UpdateAllowsDisabledDeprecated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{}))

	// Create a deprecated webhook provider (disabled)
	deprecatedProvider := models.NotificationProvider{
		ID:               "test-deprecated",
		Name:             "Deprecated Webhook",
		Type:             "webhook",
		URL:              "https://example.com/webhook",
		Enabled:          false,
		MigrationState:   "deprecated",
		NotifyProxyHosts: false,
	}
	require.NoError(t, db.Create(&deprecatedProvider).Error)

	service := services.NewNotificationService(db)
	handler := NewNotificationProviderHandler(service)

	// Update name (keeping type and enabled unchanged)
	payload := map[string]interface{}{
		"name":               "Updated Deprecated Name",
		"type":               "webhook",
		"url":                "https://example.com/webhook",
		"enabled":            false,
		"notify_proxy_hosts": true,
	}

	jsonPayload, err := json.Marshal(payload)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("PUT", "/api/v1/notifications/providers/test-deprecated", bytes.NewBuffer(jsonPayload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "test-deprecated"}}
	c.Set("role", "admin")
	c.Set("userID", uint(1))

	handler.Update(c)

	// Should still reject because type must be discord
	assert.Equal(t, http.StatusBadRequest, w.Code, "Should reject non-Discord type even for read-only fields")
}

// TestDiscordOnly_UpdateAcceptsDiscord tests that update accepts Discord provider updates.
func TestDiscordOnly_UpdateAcceptsDiscord(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{}))

	// Create a Discord provider
	discordProvider := models.NotificationProvider{
		ID:                      "test-discord",
		Name:                    "Test Discord",
		Type:                    "discord",
		URL:                     "https://discord.com/api/webhooks/123/abc",
		Enabled:                 true,
		MigrationState:          "migrated",
		NotifySecurityWAFBlocks: false,
	}
	require.NoError(t, db.Create(&discordProvider).Error)

	service := services.NewNotificationService(db)
	handler := NewNotificationProviderHandler(service)

	// Update to enable security notifications
	payload := map[string]interface{}{
		"name":                       "Test Discord",
		"type":                       "discord",
		"url":                        "https://discord.com/api/webhooks/123/abc",
		"enabled":                    true,
		"notify_security_waf_blocks": true,
	}

	jsonPayload, err := json.Marshal(payload)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("PUT", "/api/v1/notifications/providers/test-discord", bytes.NewBuffer(jsonPayload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "test-discord"}}
	c.Set("role", "admin")
	c.Set("userID", uint(1))

	handler.Update(c)

	assert.Equal(t, http.StatusOK, w.Code, "Should accept Discord provider update")
}

// TestDiscordOnly_DeleteAllowsDeprecated tests that delete works for deprecated providers.
func TestDiscordOnly_DeleteAllowsDeprecated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{}))

	// Create a deprecated webhook provider
	deprecatedProvider := models.NotificationProvider{
		ID:               "test-deprecated",
		Name:             "Deprecated Webhook",
		Type:             "webhook",
		URL:              "https://example.com/webhook",
		Enabled:          false,
		MigrationState:   "deprecated",
		NotifyProxyHosts: true,
	}
	require.NoError(t, db.Create(&deprecatedProvider).Error)

	service := services.NewNotificationService(db)
	handler := NewNotificationProviderHandler(service)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("DELETE", "/api/v1/notifications/providers/test-deprecated", nil)
	c.Params = []gin.Param{{Key: "id", Value: "test-deprecated"}}
	c.Set("role", "admin")
	c.Set("userID", uint(1))

	handler.Delete(c)

	assert.Equal(t, http.StatusOK, w.Code, "Should allow deleting deprecated provider")

	// Verify deletion
	var count int64
	db.Model(&models.NotificationProvider{}).Where("id = ?", "test-deprecated").Count(&count)
	assert.Equal(t, int64(0), count, "Provider should be deleted")
}

// TestDiscordOnly_ErrorCodes tests that error codes are deterministic.
func TestDiscordOnly_ErrorCodes(t *testing.T) {
	testCases := []struct {
		name         string
		setupFunc    func(*gorm.DB) string
		requestFunc  func(string) (*http.Request, gin.Params)
		expectedCode string
	}{
		{
			name: "create_non_discord",
			setupFunc: func(db *gorm.DB) string {
				return ""
			},
			requestFunc: func(id string) (*http.Request, gin.Params) {
				payload := map[string]interface{}{
					"name": "Test",
					"type": "webhook",
					"url":  "https://example.com",
				}
				body, _ := json.Marshal(payload)
				req, _ := http.NewRequest("POST", "/api/v1/notifications/providers", bytes.NewBuffer(body))
				return req, nil
			},
			expectedCode: "PROVIDER_TYPE_DISCORD_ONLY",
		},
		{
			name: "update_type_mutation",
			setupFunc: func(db *gorm.DB) string {
				provider := models.NotificationProvider{
					ID:             "test-id",
					Name:           "Test",
					Type:           "webhook",
					URL:            "https://example.com",
					MigrationState: "deprecated",
				}
				db.Create(&provider)
				return "test-id"
			},
			requestFunc: func(id string) (*http.Request, gin.Params) {
				payload := map[string]interface{}{
					"name": "Test",
					"type": "discord",
					"url":  "https://discord.com/api/webhooks/1/a",
				}
				body, _ := json.Marshal(payload)
				req, _ := http.NewRequest("PUT", "/api/v1/notifications/providers/"+id, bytes.NewBuffer(body))
				return req, []gin.Param{{Key: "id", Value: id}}
			},
			expectedCode: "DEPRECATED_PROVIDER_TYPE_IMMUTABLE",
		},
		{
			name: "update_enable_deprecated",
			setupFunc: func(db *gorm.DB) string {
				provider := models.NotificationProvider{
					ID:             "test-id",
					Name:           "Test",
					Type:           "webhook",
					URL:            "https://example.com",
					Enabled:        false,
					MigrationState: "deprecated",
				}
				db.Create(&provider)
				return "test-id"
			},
			requestFunc: func(id string) (*http.Request, gin.Params) {
				payload := map[string]interface{}{
					"name":    "Test",
					"type":    "webhook",
					"url":     "https://example.com",
					"enabled": true,
				}
				body, _ := json.Marshal(payload)
				req, _ := http.NewRequest("PUT", "/api/v1/notifications/providers/"+id, bytes.NewBuffer(body))
				return req, []gin.Param{{Key: "id", Value: id}}
			},
			expectedCode: "DEPRECATED_PROVIDER_CANNOT_ENABLE",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)

			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.Notification{}))

			id := tc.setupFunc(db)

			service := services.NewNotificationService(db)
			handler := NewNotificationProviderHandler(service)

			req, params := tc.requestFunc(id)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			if params != nil {
				c.Params = params
			}
			c.Set("role", "admin")
			c.Set("userID", uint(1))

			if req.Method == "POST" {
				handler.Create(c)
			} else {
				handler.Update(c)
			}

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedCode, response["code"], "Error code should be deterministic")
		})
	}
}
