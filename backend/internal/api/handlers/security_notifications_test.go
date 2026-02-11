package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
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

// mockSecurityNotificationService implements the service interface for controlled testing.
type mockSecurityNotificationService struct {
	getSettingsFunc    func() (*models.NotificationConfig, error)
	updateSettingsFunc func(*models.NotificationConfig) error
}

func (m *mockSecurityNotificationService) GetSettings() (*models.NotificationConfig, error) {
	if m.getSettingsFunc != nil {
		return m.getSettingsFunc()
	}
	return &models.NotificationConfig{}, nil
}

func (m *mockSecurityNotificationService) UpdateSettings(c *models.NotificationConfig) error {
	if m.updateSettingsFunc != nil {
		return m.updateSettingsFunc(c)
	}
	return nil
}

func setupSecNotifTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationConfig{}))
	return db
}

// TestNewSecurityNotificationHandler verifies constructor returns non-nil handler.
func TestNewSecurityNotificationHandler(t *testing.T) {
	t.Parallel()

	db := setupSecNotifTestDB(t)
	svc := services.NewSecurityNotificationService(db)
	handler := NewSecurityNotificationHandler(svc)

	assert.NotNil(t, handler, "Handler should not be nil")
}

// TestSecurityNotificationHandler_GetSettings_Success tests successful settings retrieval.
func TestSecurityNotificationHandler_GetSettings_Success(t *testing.T) {
	t.Parallel()

	expectedConfig := &models.NotificationConfig{
		ID:              "test-id",
		Enabled:         true,
		MinLogLevel:     "warn",
		WebhookURL:      "https://example.com/webhook",
		NotifyWAFBlocks: true,
		NotifyACLDenies: false,
	}

	mockService := &mockSecurityNotificationService{
		getSettingsFunc: func() (*models.NotificationConfig, error) {
			return expectedConfig, nil
		},
	}

	handler := NewSecurityNotificationHandler(mockService)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/security/notifications/settings", http.NoBody)

	handler.GetSettings(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var config models.NotificationConfig
	err := json.Unmarshal(w.Body.Bytes(), &config)
	require.NoError(t, err)

	assert.Equal(t, expectedConfig.ID, config.ID)
	assert.Equal(t, expectedConfig.Enabled, config.Enabled)
	assert.Equal(t, expectedConfig.MinLogLevel, config.MinLogLevel)
	assert.Equal(t, expectedConfig.WebhookURL, config.WebhookURL)
	assert.Equal(t, expectedConfig.NotifyWAFBlocks, config.NotifyWAFBlocks)
	assert.Equal(t, expectedConfig.NotifyACLDenies, config.NotifyACLDenies)
}

// TestSecurityNotificationHandler_GetSettings_ServiceError tests service error handling.
func TestSecurityNotificationHandler_GetSettings_ServiceError(t *testing.T) {
	t.Parallel()

	mockService := &mockSecurityNotificationService{
		getSettingsFunc: func() (*models.NotificationConfig, error) {
			return nil, errors.New("database connection failed")
		},
	}

	handler := NewSecurityNotificationHandler(mockService)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/security/notifications/settings", http.NoBody)

	handler.GetSettings(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["error"], "Failed to retrieve settings")
}

// TestSecurityNotificationHandler_UpdateSettings_InvalidJSON tests malformed JSON handling.
func TestSecurityNotificationHandler_UpdateSettings_InvalidJSON(t *testing.T) {
	t.Parallel()

	mockService := &mockSecurityNotificationService{}
	handler := NewSecurityNotificationHandler(mockService)

	malformedJSON := []byte(`{enabled: true, "min_log_level": "error"`)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("PUT", "/settings", bytes.NewBuffer(malformedJSON))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["error"], "Invalid request body")
}

// TestSecurityNotificationHandler_UpdateSettings_InvalidMinLogLevel tests invalid log level rejection.
func TestSecurityNotificationHandler_UpdateSettings_InvalidMinLogLevel(t *testing.T) {
	t.Parallel()

	invalidLevels := []struct {
		name  string
		level string
	}{
		{"trace", "trace"},
		{"critical", "critical"},
		{"fatal", "fatal"},
		{"unknown", "unknown"},
	}

	for _, tc := range invalidLevels {
		t.Run(tc.name, func(t *testing.T) {
			mockService := &mockSecurityNotificationService{}
			handler := NewSecurityNotificationHandler(mockService)

			config := models.NotificationConfig{
				Enabled:         true,
				MinLogLevel:     tc.level,
				NotifyWAFBlocks: true,
			}

			body, err := json.Marshal(config)
			require.NoError(t, err)

			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			setAdminContext(c)
			c.Request = httptest.NewRequest("PUT", "/settings", bytes.NewBuffer(body))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.UpdateSettings(c)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]string
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Contains(t, response["error"], "Invalid min_log_level")
		})
	}
}

// TestSecurityNotificationHandler_UpdateSettings_InvalidWebhookURL_SSRF tests SSRF protection.
func TestSecurityNotificationHandler_UpdateSettings_InvalidWebhookURL_SSRF(t *testing.T) {
	t.Parallel()

	ssrfURLs := []struct {
		name string
		url  string
	}{
		{"AWS Metadata", "http://169.254.169.254/latest/meta-data/"},
		{"GCP Metadata", "http://metadata.google.internal/computeMetadata/v1/"},
		{"Azure Metadata", "http://169.254.169.254/metadata/instance"},
		{"Private IP 10.x", "http://10.0.0.1/admin"},
		{"Private IP 172.16.x", "http://172.16.0.1/config"},
		{"Private IP 192.168.x", "http://192.168.1.1/api"},
		{"Link-local", "http://169.254.1.1/"},
	}

	for _, tc := range ssrfURLs {
		t.Run(tc.name, func(t *testing.T) {
			mockService := &mockSecurityNotificationService{}
			handler := NewSecurityNotificationHandler(mockService)

			config := models.NotificationConfig{
				Enabled:         true,
				MinLogLevel:     "error",
				WebhookURL:      tc.url,
				NotifyWAFBlocks: true,
			}

			body, err := json.Marshal(config)
			require.NoError(t, err)

			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			setAdminContext(c)
			c.Request = httptest.NewRequest("PUT", "/settings", bytes.NewBuffer(body))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.UpdateSettings(c)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Contains(t, response["error"], "Invalid webhook URL")
			if help, ok := response["help"]; ok {
				assert.Contains(t, help, "private networks")
			}
		})
	}
}

// TestSecurityNotificationHandler_UpdateSettings_PrivateIPWebhook tests private IP handling.
func TestSecurityNotificationHandler_UpdateSettings_PrivateIPWebhook(t *testing.T) {
	t.Parallel()

	// Note: localhost is allowed by WithAllowLocalhost() option
	localhostURLs := []string{
		"http://127.0.0.1/hook",
		"http://localhost/webhook",
		"http://[::1]/api",
	}

	for _, url := range localhostURLs {
		t.Run(url, func(t *testing.T) {
			mockService := &mockSecurityNotificationService{
				updateSettingsFunc: func(c *models.NotificationConfig) error {
					return nil
				},
			}
			handler := NewSecurityNotificationHandler(mockService)

			config := models.NotificationConfig{
				Enabled:     true,
				MinLogLevel: "warn",
				WebhookURL:  url,
			}

			body, err := json.Marshal(config)
			require.NoError(t, err)

			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			setAdminContext(c)
			c.Request = httptest.NewRequest("PUT", "/settings", bytes.NewBuffer(body))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.UpdateSettings(c)

			// Localhost should be allowed with AllowLocalhost option
			assert.Equal(t, http.StatusOK, w.Code, "Localhost should be allowed: %s", url)
		})
	}
}

// TestSecurityNotificationHandler_UpdateSettings_ServiceError tests database error handling.
func TestSecurityNotificationHandler_UpdateSettings_ServiceError(t *testing.T) {
	t.Parallel()

	mockService := &mockSecurityNotificationService{
		updateSettingsFunc: func(c *models.NotificationConfig) error {
			return errors.New("database write failed")
		},
	}

	handler := NewSecurityNotificationHandler(mockService)

	config := models.NotificationConfig{
		Enabled:         true,
		MinLogLevel:     "error",
		WebhookURL:      "http://localhost:9090/webhook", // Use localhost
		NotifyWAFBlocks: true,
	}

	body, err := json.Marshal(config)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("PUT", "/settings", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["error"], "Failed to update settings")
}

// TestSecurityNotificationHandler_UpdateSettings_Success tests successful settings update.
func TestSecurityNotificationHandler_UpdateSettings_Success(t *testing.T) {
	t.Parallel()

	var capturedConfig *models.NotificationConfig

	mockService := &mockSecurityNotificationService{
		updateSettingsFunc: func(c *models.NotificationConfig) error {
			capturedConfig = c
			return nil
		},
	}

	handler := NewSecurityNotificationHandler(mockService)

	config := models.NotificationConfig{
		Enabled:         true,
		MinLogLevel:     "warn",
		WebhookURL:      "http://localhost:8080/security", // Use localhost which is allowed
		NotifyWAFBlocks: true,
		NotifyACLDenies: false,
	}

	body, err := json.Marshal(config)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("PUT", "/settings", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Settings updated successfully", response["message"])

	// Verify the service was called with the correct config
	require.NotNil(t, capturedConfig)
	assert.Equal(t, config.Enabled, capturedConfig.Enabled)
	assert.Equal(t, config.MinLogLevel, capturedConfig.MinLogLevel)
	assert.Equal(t, config.WebhookURL, capturedConfig.WebhookURL)
	assert.Equal(t, config.NotifyWAFBlocks, capturedConfig.NotifyWAFBlocks)
	assert.Equal(t, config.NotifyACLDenies, capturedConfig.NotifyACLDenies)
}

// TestSecurityNotificationHandler_UpdateSettings_EmptyWebhookURL tests empty webhook is valid.
func TestSecurityNotificationHandler_UpdateSettings_EmptyWebhookURL(t *testing.T) {
	t.Parallel()

	mockService := &mockSecurityNotificationService{
		updateSettingsFunc: func(c *models.NotificationConfig) error {
			return nil
		},
	}

	handler := NewSecurityNotificationHandler(mockService)

	config := models.NotificationConfig{
		Enabled:         true,
		MinLogLevel:     "info",
		WebhookURL:      "",
		NotifyWAFBlocks: true,
		NotifyACLDenies: true,
	}

	body, err := json.Marshal(config)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAdminContext(c)
	c.Request = httptest.NewRequest("PUT", "/settings", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Settings updated successfully", response["message"])
}
