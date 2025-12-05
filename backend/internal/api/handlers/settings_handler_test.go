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
	req, _ := http.NewRequest("GET", "/settings", nil)
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
	req, _ := http.NewRequest("GET", "/settings/smtp", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
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
	req, _ := http.NewRequest("GET", "/settings/smtp", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["configured"])
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

	body := map[string]interface{}{
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

	body := map[string]interface{}{
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
	body := map[string]interface{}{
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

	req, _ := http.NewRequest("POST", "/settings/smtp/test", nil)
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

	req, _ := http.NewRequest("POST", "/settings/smtp/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
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
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["success"])
}

func TestMaskPassword(t *testing.T) {
	// Empty password
	assert.Equal(t, "", handlers.MaskPasswordForTest(""))

	// Non-empty password
	assert.Equal(t, "********", handlers.MaskPasswordForTest("secret"))
}
