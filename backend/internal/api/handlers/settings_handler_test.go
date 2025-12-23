package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/models"
)

func setupSettingsTestDB(t *testing.T) *gorm.DB {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect to test database")
	}
	db.AutoMigrate(&models.Setting{})
	return db
}

func TestSettingsHandler_GetSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	// Seed data
	db.Create(&models.Setting{Key: "test_key", Value: "test_value", Category: "general", Type: "string"})

	handler := handlers.NewSettingsHandler(db)
	router := gin.New()
	router.GET("/settings", handler.GetSettings)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test_value", response["test_key"])
}

func TestSettingsHandler_UpdateSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := gin.New()
	router.POST("/settings", handler.UpdateSetting)

	// Test Create
	payload := map[string]string{
		"key":      "new_key",
		"value":    "new_value",
		"category": "system",
		"type":     "string",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var setting models.Setting
	db.Where("key = ?", "new_key").First(&setting)
	assert.Equal(t, "new_value", setting.Value)

	// Test Update
	payload["value"] = "updated_value"
	body, _ = json.Marshal(payload)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	db.Where("key = ?", "new_key").First(&setting)
	assert.Equal(t, "updated_value", setting.Value)
}

func TestSettingsHandler_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupSettingsTestDB(t)

	handler := handlers.NewSettingsHandler(db)
	router := gin.New()
	router.POST("/settings", handler.UpdateSetting)

	// Invalid JSON
	req, _ := http.NewRequest("POST", "/settings", bytes.NewBuffer([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Missing Key/Value
	payload := map[string]string{
		"key": "some_key",
		// value missing
	}
	body, _ := json.Marshal(payload)
	req, _ = http.NewRequest("POST", "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============= SMTP Settings Tests =============

func setupSettingsHandlerWithMail(t *testing.T) (*handlers.SettingsHandler, *gorm.DB) {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect to test database")
	}
	db.AutoMigrate(&models.Setting{})
	return handlers.NewSettingsHandler(db), db
}

func TestSettingsHandler_GetSMTPConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupSettingsHandlerWithMail(t)

	// Seed SMTP config
	db.Create(&models.Setting{Key: "smtp_host", Value: "smtp.example.com", Category: "smtp", Type: "string"})
	db.Create(&models.Setting{Key: "smtp_port", Value: "587", Category: "smtp", Type: "number"})
	db.Create(&models.Setting{Key: "smtp_username", Value: "user@example.com", Category: "smtp", Type: "string"})
	db.Create(&models.Setting{Key: "smtp_password", Value: "secret123", Category: "smtp", Type: "string"})
	db.Create(&models.Setting{Key: "smtp_from_address", Value: "noreply@example.com", Category: "smtp", Type: "string"})
	db.Create(&models.Setting{Key: "smtp_encryption", Value: "starttls", Category: "smtp", Type: "string"})

	router := gin.New()
	router.GET("/settings/smtp", handler.GetSMTPConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/smtp", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "smtp.example.com", resp["host"])
	assert.Equal(t, float64(587), resp["port"])
	assert.Equal(t, "********", resp["password"]) // Password should be masked
	assert.Equal(t, true, resp["configured"])
}

func TestSettingsHandler_GetSMTPConfig_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.GET("/settings/smtp", handler.GetSMTPConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/smtp", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["configured"])
}

func TestSettingsHandler_GetSMTPConfig_DatabaseError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupSettingsHandlerWithMail(t)
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	router := gin.New()
	router.GET("/settings/smtp", handler.GetSMTPConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/smtp", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSettingsHandler_UpdateSMTPConfig_NonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	router.PUT("/settings/smtp", handler.UpdateSMTPConfig)

	body := map[string]any{
		"host":         "smtp.example.com",
		"port":         587,
		"from_address": "test@example.com",
		"encryption":   "starttls",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", "/settings/smtp", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSettingsHandler_UpdateSMTPConfig_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.PUT("/settings/smtp", handler.UpdateSMTPConfig)

	req, _ := http.NewRequest("PUT", "/settings/smtp", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSettingsHandler_UpdateSMTPConfig_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.PUT("/settings/smtp", handler.UpdateSMTPConfig)

	body := map[string]any{
		"host":         "smtp.example.com",
		"port":         587,
		"username":     "user@example.com",
		"password":     "password123",
		"from_address": "noreply@example.com",
		"encryption":   "starttls",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", "/settings/smtp", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSettingsHandler_UpdateSMTPConfig_KeepExistingPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupSettingsHandlerWithMail(t)

	// Seed existing password
	db.Create(&models.Setting{Key: "smtp_password", Value: "existingpassword", Category: "smtp", Type: "string"})
	db.Create(&models.Setting{Key: "smtp_host", Value: "old.example.com", Category: "smtp", Type: "string"})
	db.Create(&models.Setting{Key: "smtp_port", Value: "25", Category: "smtp", Type: "number"})
	db.Create(&models.Setting{Key: "smtp_from_address", Value: "old@example.com", Category: "smtp", Type: "string"})
	db.Create(&models.Setting{Key: "smtp_encryption", Value: "none", Category: "smtp", Type: "string"})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.PUT("/settings/smtp", handler.UpdateSMTPConfig)

	// Send masked password (simulating frontend sending back masked value)
	body := map[string]any{
		"host":         "smtp.example.com",
		"port":         587,
		"password":     "********", // Masked
		"from_address": "noreply@example.com",
		"encryption":   "starttls",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", "/settings/smtp", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify password was preserved
	var setting models.Setting
	db.Where("key = ?", "smtp_password").First(&setting)
	assert.Equal(t, "existingpassword", setting.Value)
}

func TestSettingsHandler_TestSMTPConfig_NonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	router.POST("/settings/smtp/test", handler.TestSMTPConfig)

	req, _ := http.NewRequest("POST", "/settings/smtp/test", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSettingsHandler_TestSMTPConfig_NotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/smtp/test", handler.TestSMTPConfig)

	req, _ := http.NewRequest("POST", "/settings/smtp/test", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["success"])
}

func TestSettingsHandler_SendTestEmail_NonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	router.POST("/settings/smtp/send-test", handler.SendTestEmail)

	body := map[string]string{"to": "test@example.com"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/smtp/send-test", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSettingsHandler_SendTestEmail_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/smtp/send-test", handler.SendTestEmail)

	req, _ := http.NewRequest("POST", "/settings/smtp/send-test", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSettingsHandler_SendTestEmail_NotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/smtp/send-test", handler.SendTestEmail)

	body := map[string]string{"to": "test@example.com"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/smtp/send-test", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["success"])
}

func TestMaskPassword(t *testing.T) {
	// Empty password
	assert.Equal(t, "", handlers.MaskPasswordForTest(""))

	// Non-empty password
	assert.Equal(t, "********", handlers.MaskPasswordForTest("secret"))
}

// ============= URL Testing Tests =============

func TestSettingsHandler_ValidatePublicURL_NonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	router.POST("/settings/validate-url", handler.ValidatePublicURL)

	body := map[string]string{"url": "https://example.com"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/validate-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSettingsHandler_ValidatePublicURL_InvalidFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/validate-url", handler.ValidatePublicURL)

	testCases := []struct {
		name string
		url  string
	}{
		{"Missing scheme", "example.com"},
		{"Invalid scheme", "ftp://example.com"},
		{"URL with path", "https://example.com/path"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]string{"url": tc.url}
			jsonBody, _ := json.Marshal(body)
			req, _ := http.NewRequest("POST", "/settings/validate-url", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			var resp map[string]any
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, false, resp["valid"])
		})
	}
}

func TestSettingsHandler_ValidatePublicURL_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/validate-url", handler.ValidatePublicURL)

	testCases := []struct {
		name     string
		url      string
		expected string
	}{
		{"HTTPS URL", "https://example.com", "https://example.com"},
		{"HTTP URL", "http://example.com", "http://example.com"},
		{"URL with port", "https://example.com:8080", "https://example.com:8080"},
		{"URL with trailing slash", "https://example.com/", "https://example.com"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]string{"url": tc.url}
			jsonBody, _ := json.Marshal(body)
			req, _ := http.NewRequest("POST", "/settings/validate-url", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			var resp map[string]any
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, true, resp["valid"])
			assert.Equal(t, tc.expected, resp["normalized"])
		})
	}
}

func TestSettingsHandler_TestPublicURL_NonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	router.POST("/settings/test-url", handler.TestPublicURL)

	body := map[string]string{"url": "https://example.com"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSettingsHandler_TestPublicURL_NoRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	// No role set in context
	router.POST("/settings/test-url", handler.TestPublicURL)

	body := map[string]string{"url": "https://example.com"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSettingsHandler_TestPublicURL_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/test-url", handler.TestPublicURL)

	req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSettingsHandler_TestPublicURL_InvalidURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/test-url", handler.TestPublicURL)

	body := map[string]string{"url": "not-a-valid-url"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["reachable"])
	assert.Contains(t, resp["error"], "Invalid URL")
}

func TestSettingsHandler_TestPublicURL_PrivateIPBlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/test-url", handler.TestPublicURL)

	// Test various private IPs that should be blocked
	testCases := []struct {
		name string
		url  string
	}{
		{"localhost", "http://localhost"},
		{"127.0.0.1", "http://127.0.0.1"},
		{"Private 10.x", "http://10.0.0.1"},
		{"Private 192.168.x", "http://192.168.1.1"},
		{"AWS metadata", "http://169.254.169.254"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]string{"url": tc.url}
			jsonBody, _ := json.Marshal(body)
			req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code) // Returns 200 but with reachable=false
			var resp map[string]any
			json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, false, resp["reachable"])
			assert.Contains(t, resp["error"], "private IP")
		})
	}
}

func TestSettingsHandler_TestPublicURL_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	// NOTE: Using a real public URL instead of httptest.NewServer() because
	// SSRF protection (correctly) blocks localhost/127.0.0.1.
	// Using example.com which is guaranteed to be reachable and is designed for testing
	// Alternative: Refactor handler to accept injectable URL validator (future improvement).
	publicTestURL := "https://example.com"

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/test-url", handler.TestPublicURL)

	body := map[string]string{"url": publicTestURL}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	// The test verifies the handler works with a real public URL
	assert.Equal(t, true, resp["reachable"], "example.com should be reachable")
	assert.NotNil(t, resp["latency"])
	assert.NotNil(t, resp["message"])
}

func TestSettingsHandler_TestPublicURL_DNSFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupSettingsHandlerWithMail(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/settings/test-url", handler.TestPublicURL)

	body := map[string]string{"url": "http://nonexistent-domain-12345.invalid"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/settings/test-url", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code) // Returns 200 but with reachable=false
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["reachable"])
	assert.Contains(t, resp["error"], "DNS")
}
