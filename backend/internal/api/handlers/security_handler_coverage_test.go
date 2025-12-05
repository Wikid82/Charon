package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// Tests for UpdateConfig handler to improve coverage (currently 46%)
func TestSecurityHandler_UpdateConfig_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.SecurityRuleSet{}, &models.SecurityDecision{}, &models.SecurityAudit{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/config", handler.UpdateConfig)

	payload := map[string]interface{}{
		"name":            "default",
		"admin_whitelist": "192.168.1.0/24",
		"waf_mode":        "monitor",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotNil(t, resp["config"])
}

func TestSecurityHandler_UpdateConfig_DefaultName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.SecurityRuleSet{}, &models.SecurityDecision{}, &models.SecurityAudit{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/config", handler.UpdateConfig)

	// Payload without name - should default to "default"
	payload := map[string]interface{}{
		"admin_whitelist": "10.0.0.0/8",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecurityHandler_UpdateConfig_InvalidPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/config", handler.UpdateConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/config", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Tests for GetConfig handler
func TestSecurityHandler_GetConfig_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	// Create a config
	cfg := models.SecurityConfig{Name: "default", AdminWhitelist: "127.0.0.1"}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.GET("/security/config", handler.GetConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/config", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotNil(t, resp["config"])
}

func TestSecurityHandler_GetConfig_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.GET("/security/config", handler.GetConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/config", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Nil(t, resp["config"])
}

// Tests for ListDecisions handler
func TestSecurityHandler_ListDecisions_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}))

	// Create some decisions with UUIDs
	db.Create(&models.SecurityDecision{UUID: uuid.New().String(), IP: "1.2.3.4", Action: "block", Source: "waf"})
	db.Create(&models.SecurityDecision{UUID: uuid.New().String(), IP: "5.6.7.8", Action: "allow", Source: "acl"})

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.GET("/security/decisions", handler.ListDecisions)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/decisions", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	decisions := resp["decisions"].([]interface{})
	assert.Len(t, decisions, 2)
}

func TestSecurityHandler_ListDecisions_WithLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}))

	// Create 5 decisions with unique UUIDs
	for i := 0; i < 5; i++ {
		db.Create(&models.SecurityDecision{UUID: uuid.New().String(), IP: fmt.Sprintf("1.2.3.%d", i), Action: "block", Source: "waf"})
	}

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.GET("/security/decisions", handler.ListDecisions)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/decisions?limit=2", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	decisions := resp["decisions"].([]interface{})
	assert.Len(t, decisions, 2)
}

// Tests for CreateDecision handler
func TestSecurityHandler_CreateDecision_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityAudit{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/decisions", handler.CreateDecision)

	payload := map[string]interface{}{
		"ip":      "10.0.0.1",
		"action":  "block",
		"reason":  "manual block",
		"details": "Test manual override",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/decisions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecurityHandler_CreateDecision_MissingIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/decisions", handler.CreateDecision)

	payload := map[string]interface{}{
		"action": "block",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/decisions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecurityHandler_CreateDecision_MissingAction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/decisions", handler.CreateDecision)

	payload := map[string]interface{}{
		"ip": "10.0.0.1",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/decisions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecurityHandler_CreateDecision_InvalidPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/decisions", handler.CreateDecision)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/decisions", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Tests for ListRuleSets handler
func TestSecurityHandler_ListRuleSets_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityRuleSet{}))

	// Create some rulesets with UUIDs
	db.Create(&models.SecurityRuleSet{UUID: uuid.New().String(), Name: "owasp-crs", Mode: "blocking", Content: "# OWASP rules"})
	db.Create(&models.SecurityRuleSet{UUID: uuid.New().String(), Name: "custom", Mode: "detection", Content: "# Custom rules"})

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.GET("/security/rulesets", handler.ListRuleSets)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/rulesets", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	rulesets := resp["rulesets"].([]interface{})
	assert.Len(t, rulesets, 2)
}

// Tests for UpsertRuleSet handler
func TestSecurityHandler_UpsertRuleSet_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityRuleSet{}, &models.SecurityAudit{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/rulesets", handler.UpsertRuleSet)

	payload := map[string]interface{}{
		"name":    "test-ruleset",
		"mode":    "blocking",
		"content": "# Test rules",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/rulesets", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecurityHandler_UpsertRuleSet_MissingName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityRuleSet{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/rulesets", handler.UpsertRuleSet)

	payload := map[string]interface{}{
		"mode":    "blocking",
		"content": "# Test rules",
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/rulesets", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecurityHandler_UpsertRuleSet_InvalidPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityRuleSet{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/rulesets", handler.UpsertRuleSet)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/rulesets", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Tests for DeleteRuleSet handler (currently 52%)
func TestSecurityHandler_DeleteRuleSet_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityRuleSet{}, &models.SecurityAudit{}))

	// Create a ruleset to delete
	ruleset := models.SecurityRuleSet{Name: "delete-me", Mode: "blocking"}
	db.Create(&ruleset)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.DELETE("/security/rulesets/:id", handler.DeleteRuleSet)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/security/rulesets/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["deleted"].(bool))
}

func TestSecurityHandler_DeleteRuleSet_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityRuleSet{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.DELETE("/security/rulesets/:id", handler.DeleteRuleSet)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/security/rulesets/999", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSecurityHandler_DeleteRuleSet_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityRuleSet{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.DELETE("/security/rulesets/:id", handler.DeleteRuleSet)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/security/rulesets/invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecurityHandler_DeleteRuleSet_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityRuleSet{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	// Note: This route pattern won't match empty ID, but testing the handler directly
	router.DELETE("/security/rulesets/:id", handler.DeleteRuleSet)

	// This should hit the "id is required" check if we bypass routing
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/security/rulesets/", nil)
	router.ServeHTTP(w, req)

	// Router won't match this path, so 404
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Tests for Enable handler
func TestSecurityHandler_Enable_NoConfigNoWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/enable", handler.Enable)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/enable", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Should succeed when no config exists - creates new config
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecurityHandler_Enable_WithWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	// Create config with whitelist containing 127.0.0.1
	cfg := models.SecurityConfig{Name: "default", AdminWhitelist: "127.0.0.1"}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/enable", handler.Enable)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/enable", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345" // Use RemoteAddr for ClientIP
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecurityHandler_Enable_IPNotInWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	// Create config with whitelist that doesn't include test IP
	cfg := models.SecurityConfig{Name: "default", AdminWhitelist: "10.0.0.0/8"}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/enable", handler.Enable)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/enable", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.1:12345" // Not in 10.0.0.0/8
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSecurityHandler_Enable_WithValidBreakGlassToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/breakglass/generate", handler.GenerateBreakGlass)
	router.POST("/security/enable", handler.Enable)

	// First, create a config with no whitelist
	cfg := models.SecurityConfig{Name: "default", AdminWhitelist: ""}
	db.Create(&cfg)

	// Generate a break-glass token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/breakglass/generate", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var tokenResp map[string]string
	json.Unmarshal(w.Body.Bytes(), &tokenResp)
	token := tokenResp["token"]

	// Now try to enable with the token
	payload := map[string]string{"break_glass_token": token}
	body, _ := json.Marshal(payload)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/security/enable", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecurityHandler_Enable_WithInvalidBreakGlassToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	// Create config with no whitelist
	cfg := models.SecurityConfig{Name: "default", AdminWhitelist: ""}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/enable", handler.Enable)

	payload := map[string]string{"break_glass_token": "invalid-token"}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/enable", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// Tests for Disable handler (currently 44%)
func TestSecurityHandler_Disable_FromLocalhost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	// Create enabled config
	cfg := models.SecurityConfig{Name: "default", Enabled: true}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/disable", func(c *gin.Context) {
		// Simulate localhost request
		c.Request.RemoteAddr = "127.0.0.1:12345"
		handler.Disable(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/disable", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp["enabled"].(bool))
}

func TestSecurityHandler_Disable_FromRemoteWithToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/breakglass/generate", handler.GenerateBreakGlass)
	router.POST("/security/disable", func(c *gin.Context) {
		c.Request.RemoteAddr = "192.168.1.100:12345" // Remote IP
		handler.Disable(c)
	})

	// Create enabled config
	cfg := models.SecurityConfig{Name: "default", Enabled: true}
	db.Create(&cfg)

	// Generate token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/breakglass/generate", nil)
	router.ServeHTTP(w, req)
	var tokenResp map[string]string
	json.Unmarshal(w.Body.Bytes(), &tokenResp)
	token := tokenResp["token"]

	// Disable with token
	payload := map[string]string{"break_glass_token": token}
	body, _ := json.Marshal(payload)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/security/disable", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecurityHandler_Disable_FromRemoteNoToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	// Create enabled config
	cfg := models.SecurityConfig{Name: "default", Enabled: true}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/disable", func(c *gin.Context) {
		c.Request.RemoteAddr = "192.168.1.100:12345" // Remote IP
		handler.Disable(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/disable", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSecurityHandler_Disable_FromRemoteInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	// Create enabled config
	cfg := models.SecurityConfig{Name: "default", Enabled: true}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/disable", func(c *gin.Context) {
		c.Request.RemoteAddr = "192.168.1.100:12345" // Remote IP
		handler.Disable(c)
	})

	payload := map[string]string{"break_glass_token": "invalid-token"}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/disable", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// Tests for GenerateBreakGlass handler
func TestSecurityHandler_GenerateBreakGlass_NoConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/breakglass/generate", handler.GenerateBreakGlass)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/breakglass/generate", nil)
	router.ServeHTTP(w, req)

	// Should succeed and create a new config with the token
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp["token"])
}

// Test Enable with IPv6 localhost
func TestSecurityHandler_Disable_FromIPv6Localhost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	// Create enabled config
	cfg := models.SecurityConfig{Name: "default", Enabled: true}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/disable", func(c *gin.Context) {
		c.Request.RemoteAddr = "[::1]:12345" // IPv6 localhost
		handler.Disable(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/disable", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// Test Enable with CIDR whitelist matching
func TestSecurityHandler_Enable_WithCIDRWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	// Create config with CIDR whitelist
	cfg := models.SecurityConfig{Name: "default", AdminWhitelist: "192.168.0.0/16, 10.0.0.0/8"}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/enable", handler.Enable)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/enable", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.50:12345" // In 192.168.0.0/16
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// Test Enable with exact IP in whitelist
func TestSecurityHandler_Enable_WithExactIPWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))

	// Create config with exact IP whitelist
	cfg := models.SecurityConfig{Name: "default", AdminWhitelist: "192.168.1.100"}
	db.Create(&cfg)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.POST("/security/enable", handler.Enable)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/security/enable", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.100:12345"
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
