package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	if err := db.AutoMigrate(&models.Setting{}); err != nil {
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

	cfg := config.SecurityConfig{ACLMode: "disabled"}
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
