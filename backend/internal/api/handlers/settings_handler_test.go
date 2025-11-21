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

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/api/handlers"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
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
