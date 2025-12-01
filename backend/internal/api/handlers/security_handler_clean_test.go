package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

func setupTestDB(t *testing.T) *gorm.DB {
	// lightweight in-memory DB unique per test run
	dsn := fmt.Sprintf("file:security_handler_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open DB: %v", err)
	}
	if err := db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestSecurityHandler_GetStatus_Clean(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Basic disabled scenario
	cfg := config.SecurityConfig{
		CrowdSecMode:  "disabled",
		WAFMode:       "disabled",
		RateLimitMode: "disabled",
		ACLMode:       "disabled",
	}
	handler := NewSecurityHandler(cfg, nil)
	router := gin.New()
	router.GET("/security/status", handler.GetStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/status", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	// response body intentionally not printed in clean test
	assert.NotNil(t, response["cerberus"])
}

func TestSecurityHandler_Cerberus_DBOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	// set DB to enable cerberus
	if err := db.Create(&models.Setting{Key: "security.cerberus.enabled", Value: "true"}).Error; err != nil {
		t.Fatalf("failed to insert setting: %v", err)
	}

	cfg := config.SecurityConfig{CerberusEnabled: false}
	handler := NewSecurityHandler(cfg, db)
	router := gin.New()
	router.GET("/security/status", handler.GetStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/status", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	cerb := response["cerberus"].(map[string]interface{})
	assert.Equal(t, true, cerb["enabled"].(bool))
}

func TestSecurityHandler_ACL_DBOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	// set DB to enable ACL (override config)
	if err := db.Create(&models.Setting{Key: "security.acl.enabled", Value: "true"}).Error; err != nil {
		t.Fatalf("failed to insert setting: %v", err)
	}
	// Confirm the DB write succeeded
	var s models.Setting
	if err := db.Where("key = ?", "security.acl.enabled").First(&s).Error; err != nil {
		t.Fatalf("setting not found in DB: %v", err)
	}
	if s.Value != "true" {
		t.Fatalf("unexpected value in DB for security.acl.enabled: %s", s.Value)
	}
	// DB write succeeded; no additional dump needed

	// Ensure Cerberus is enabled so ACL can be active
	cfg := config.SecurityConfig{ACLMode: "disabled", CerberusEnabled: true}
	handler := NewSecurityHandler(cfg, db)
	router := gin.New()
	router.GET("/security/status", handler.GetStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/status", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	acl := response["acl"].(map[string]interface{})
	assert.Equal(t, true, acl["enabled"].(bool))
}

func TestSecurityHandler_ACL_DisabledWhenCerberusOff(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	// set DB to enable ACL but disable Cerberus
	if err := db.Create(&models.Setting{Key: "security.acl.enabled", Value: "true"}).Error; err != nil {
		t.Fatalf("failed to insert setting: %v", err)
	}
	if err := db.Create(&models.Setting{Key: "security.cerberus.enabled", Value: "false"}).Error; err != nil {
		t.Fatalf("failed to insert setting: %v", err)
	}

	cfg := config.SecurityConfig{ACLMode: "enabled", CerberusEnabled: true}
	handler := NewSecurityHandler(cfg, db)
	router := gin.New()
	router.GET("/security/status", handler.GetStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/status", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	cerb := response["cerberus"].(map[string]interface{})
	assert.Equal(t, false, cerb["enabled"].(bool))
	acl := response["acl"].(map[string]interface{})
	// ACL must be false because Cerberus is disabled
	assert.Equal(t, false, acl["enabled"].(bool))
}

func TestSecurityHandler_CrowdSec_Mode_DBOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	// set DB to configure crowdsec.mode to local
	if err := db.Create(&models.Setting{Key: "security.crowdsec.mode", Value: "local"}).Error; err != nil {
		t.Fatalf("failed to insert setting: %v", err)
	}

	cfg := config.SecurityConfig{CrowdSecMode: "disabled"}
	handler := NewSecurityHandler(cfg, db)
	router := gin.New()
	router.GET("/security/status", handler.GetStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/status", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	cs := response["crowdsec"].(map[string]interface{})
	assert.Equal(t, "local", cs["mode"].(string))
}

func TestSecurityHandler_CrowdSec_ExternalMappedToDisabled_DBOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	// set DB to configure crowdsec.mode to external
	if err := db.Create(&models.Setting{Key: "security.crowdsec.mode", Value: "external"}).Error; err != nil {
		t.Fatalf("failed to insert setting: %v", err)
	}
	cfg := config.SecurityConfig{CrowdSecMode: "local"}
	handler := NewSecurityHandler(cfg, db)
	router := gin.New()
	router.GET("/security/status", handler.GetStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/status", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	cs := response["crowdsec"].(map[string]interface{})
	assert.Equal(t, "disabled", cs["mode"].(string))
	assert.Equal(t, false, cs["enabled"].(bool))
}

func TestSecurityHandler_ExternalModeMappedToDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.SecurityConfig{
		CrowdSecMode:  "external",
		WAFMode:       "disabled",
		RateLimitMode: "disabled",
		ACLMode:       "disabled",
	}
	handler := NewSecurityHandler(cfg, nil)
	router := gin.New()
	router.GET("/security/status", handler.GetStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/status", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	cs := response["crowdsec"].(map[string]interface{})
	assert.Equal(t, "disabled", cs["mode"].(string))
	assert.Equal(t, false, cs["enabled"].(bool))
}

func TestSecurityHandler_Enable_Disable_WithAdminWhitelistAndToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	// Add SecurityConfig with no admin whitelist - should refuse enable
	sec := models.SecurityConfig{Name: "default", Enabled: false, AdminWhitelist: ""}
	if err := db.Create(&sec).Error; err != nil {
		t.Fatalf("failed to create security config: %v", err)
	}

	handler := NewSecurityHandler(config.SecurityConfig{}, db)
	router := gin.New()
	api := router.Group("/api/v1")
	api.POST("/security/enable", handler.Enable)
	api.POST("/security/disable", handler.Disable)
	api.POST("/security/breakglass/generate", handler.GenerateBreakGlass)

	// Attempt to enable without admin whitelist should be 400
	req := httptest.NewRequest("POST", "/api/v1/security/enable", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusBadRequest, resp.Code)

	// Update config with admin whitelist including 127.0.0.1
	db.Model(&sec).Update("admin_whitelist", "127.0.0.1/32")

	// Enable using admin IP via X-Forwarded-For
	req = httptest.NewRequest("POST", "/api/v1/security/enable", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "127.0.0.1")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)

	// Generate break-glass token
	req = httptest.NewRequest("POST", "/api/v1/security/breakglass/generate", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
	var tokenResp map[string]string
	err := json.Unmarshal(resp.Body.Bytes(), &tokenResp)
	assert.NoError(t, err)
	token := tokenResp["token"]
	assert.NotEmpty(t, token)

	// Disable using token
	req = httptest.NewRequest("POST", "/api/v1/security/disable", strings.NewReader(`{"break_glass_token":"`+token+`"}`))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
}
