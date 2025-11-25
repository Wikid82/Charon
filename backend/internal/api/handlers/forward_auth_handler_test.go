package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupForwardAuthTestDB() *gorm.DB {
	db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	db.AutoMigrate(&models.ForwardAuthConfig{})
	return db
}

func TestForwardAuthHandler_GetConfig(t *testing.T) {
	db := setupForwardAuthTestDB()
	h := NewForwardAuthHandler(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/config", h.GetConfig)

	// Test empty config (default)
	req, _ := http.NewRequest("GET", "/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.ForwardAuthConfig
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "custom", resp.Provider)

	// Test existing config
	db.Create(&models.ForwardAuthConfig{
		Provider: "authelia",
		Address:  "http://test",
	})

	req, _ = http.NewRequest("GET", "/config", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "authelia", resp.Provider)
}

func TestForwardAuthHandler_UpdateConfig(t *testing.T) {
	db := setupForwardAuthTestDB()
	h := NewForwardAuthHandler(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/config", h.UpdateConfig)

	// Test Create
	payload := map[string]interface{}{
		"provider":             "authelia",
		"address":              "http://authelia:9091",
		"trust_forward_header": true,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/config", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.ForwardAuthConfig
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "authelia", resp.Provider)

	// Test Update
	payload["provider"] = "authentik"
	body, _ = json.Marshal(payload)
	req, _ = http.NewRequest("POST", "/config", bytes.NewBuffer(body))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "authentik", resp.Provider)

	// Test Validation Error
	payload["address"] = "not-a-url"
	body, _ = json.Marshal(payload)
	req, _ = http.NewRequest("POST", "/config", bytes.NewBuffer(body))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestForwardAuthHandler_GetTemplates(t *testing.T) {
	db := setupForwardAuthTestDB()
	h := NewForwardAuthHandler(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/templates", h.GetTemplates)

	req, _ := http.NewRequest("GET", "/templates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp, "authelia")
	assert.Contains(t, resp, "authentik")
}
