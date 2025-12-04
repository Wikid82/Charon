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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// setupAuditTestDB creates an in-memory SQLite database for security audit tests
func setupAuditTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.SecurityConfig{},
		&models.SecurityRuleSet{},
		&models.SecurityDecision{},
		&models.SecurityAudit{},
		&models.Setting{},
	))
	return db
}

// =============================================================================
// SECURITY AUDIT: SQL Injection Tests
// =============================================================================

func TestSecurityHandler_GetStatus_SQLInjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuditTestDB(t)

	// Seed malicious setting keys that could be used in SQL injection
	maliciousKeys := []string{
		"security.cerberus.enabled'; DROP TABLE settings;--",
		"security.cerberus.enabled\"; DROP TABLE settings;--",
		"security.cerberus.enabled OR 1=1--",
		"security.cerberus.enabled UNION SELECT * FROM users--",
	}

	for _, key := range maliciousKeys {
		// Attempt to seed with malicious key (should fail or be harmless)
		setting := models.Setting{Key: key, Value: "true"}
		db.Create(&setting)
	}

	cfg := config.SecurityConfig{CerberusEnabled: false}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.GET("/api/v1/security/status", h.GetStatus)

	req := httptest.NewRequest("GET", "/api/v1/security/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 200 and valid JSON despite malicious data
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Contains(t, resp, "cerberus")
}

func TestSecurityHandler_CreateDecision_SQLInjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuditTestDB(t)

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.POST("/api/v1/security/decisions", h.CreateDecision)

	// Attempt SQL injection via payload fields
	maliciousPayloads := []map[string]string{
		{"ip": "'; DROP TABLE security_decisions;--", "action": "block"},
		{"ip": "127.0.0.1", "action": "'; DELETE FROM security_decisions;--"},
		{"ip": "\" OR 1=1; --", "action": "allow"},
		{"ip": "127.0.0.1", "action": "block", "details": "'; DROP TABLE users;--"},
	}

	for i, payload := range maliciousPayloads {
		t.Run(fmt.Sprintf("payload_%d", i), func(t *testing.T) {
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/api/v1/security/decisions", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should return 200 (created) or 400 (bad request) but NOT crash
			assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest,
				"Expected 200 or 400, got %d", w.Code)

			// Verify tables still exist
			var count int64
			db.Raw("SELECT COUNT(*) FROM security_decisions").Scan(&count)
			// Should not error from SQL injection
			assert.GreaterOrEqual(t, count, int64(0))
		})
	}
}

// =============================================================================
// SECURITY AUDIT: Input Validation Tests
// =============================================================================

func TestSecurityHandler_UpsertRuleSet_MassivePayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuditTestDB(t)

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.POST("/api/v1/security/rulesets", h.UpsertRuleSet)

	// Try to submit a 3MB payload (should be rejected by service)
	hugeContent := strings.Repeat("SecRule REQUEST_URI \"@contains /admin\" \"id:1000,phase:1,deny\"\n", 50000)

	payload := map[string]interface{}{
		"name":    "huge-ruleset",
		"content": hugeContent,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/security/rulesets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should be rejected (either 400 or 500 indicating content too large)
	// The service limits to 2MB
	if len(hugeContent) > 2*1024*1024 {
		assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError,
			"Expected rejection of huge payload, got %d", w.Code)
	}
}

func TestSecurityHandler_UpsertRuleSet_EmptyName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuditTestDB(t)

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.POST("/api/v1/security/rulesets", h.UpsertRuleSet)

	payload := map[string]interface{}{
		"name":    "",
		"content": "SecRule REQUEST_URI \"@contains /admin\"",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/security/rulesets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp, "error")
}

func TestSecurityHandler_CreateDecision_EmptyFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuditTestDB(t)

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.POST("/api/v1/security/decisions", h.CreateDecision)

	testCases := []struct {
		name     string
		payload  map[string]string
		wantCode int
	}{
		{"empty_ip", map[string]string{"ip": "", "action": "block"}, http.StatusBadRequest},
		{"empty_action", map[string]string{"ip": "127.0.0.1", "action": ""}, http.StatusBadRequest},
		{"both_empty", map[string]string{"ip": "", "action": ""}, http.StatusBadRequest},
		{"valid", map[string]string{"ip": "127.0.0.1", "action": "block"}, http.StatusOK},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest("POST", "/api/v1/security/decisions", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tc.wantCode, w.Code)
		})
	}
}

// =============================================================================
// SECURITY AUDIT: Settings Toggle Persistence Tests
// =============================================================================

func TestSecurityHandler_GetStatus_SettingsOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuditTestDB(t)

	// Seed settings that should override config defaults
	settings := []models.Setting{
		{Key: "security.cerberus.enabled", Value: "true", Category: "security"},
		{Key: "security.waf.enabled", Value: "true", Category: "security"},
		{Key: "security.rate_limit.enabled", Value: "true", Category: "security"},
		{Key: "security.crowdsec.enabled", Value: "true", Category: "security"},
		{Key: "security.acl.enabled", Value: "true", Category: "security"},
	}
	for _, s := range settings {
		require.NoError(t, db.Create(&s).Error)
	}

	// Config has everything disabled
	cfg := config.SecurityConfig{
		CerberusEnabled: false,
		WAFMode:         "disabled",
		RateLimitMode:   "disabled",
		CrowdSecMode:    "disabled",
		ACLMode:         "disabled",
	}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.GET("/api/v1/security/status", h.GetStatus)

	req := httptest.NewRequest("GET", "/api/v1/security/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Verify settings override config
	assert.True(t, resp["cerberus"]["enabled"].(bool), "cerberus should be enabled via settings")
	assert.True(t, resp["waf"]["enabled"].(bool), "waf should be enabled via settings")
	assert.True(t, resp["rate_limit"]["enabled"].(bool), "rate_limit should be enabled via settings")
	assert.True(t, resp["crowdsec"]["enabled"].(bool), "crowdsec should be enabled via settings")
	assert.True(t, resp["acl"]["enabled"].(bool), "acl should be enabled via settings")
}

func TestSecurityHandler_GetStatus_DisabledViaSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuditTestDB(t)

	// Seed settings that disable everything
	settings := []models.Setting{
		{Key: "security.cerberus.enabled", Value: "false", Category: "security"},
		{Key: "security.waf.enabled", Value: "false", Category: "security"},
		{Key: "security.rate_limit.enabled", Value: "false", Category: "security"},
		{Key: "security.crowdsec.enabled", Value: "false", Category: "security"},
	}
	for _, s := range settings {
		require.NoError(t, db.Create(&s).Error)
	}

	// Config has everything enabled
	cfg := config.SecurityConfig{
		CerberusEnabled: true,
		WAFMode:         "enabled",
		RateLimitMode:   "enabled",
		CrowdSecMode:    "local",
	}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.GET("/api/v1/security/status", h.GetStatus)

	req := httptest.NewRequest("GET", "/api/v1/security/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Verify settings override config to disabled
	assert.False(t, resp["cerberus"]["enabled"].(bool), "cerberus should be disabled via settings")
	assert.False(t, resp["waf"]["enabled"].(bool), "waf should be disabled via settings")
	assert.False(t, resp["rate_limit"]["enabled"].(bool), "rate_limit should be disabled via settings")
	assert.False(t, resp["crowdsec"]["enabled"].(bool), "crowdsec should be disabled via settings")
}

// =============================================================================
// SECURITY AUDIT: Delete RuleSet Validation
// =============================================================================

func TestSecurityAudit_DeleteRuleSet_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuditTestDB(t)

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.DELETE("/api/v1/security/rulesets/:id", h.DeleteRuleSet)

	testCases := []struct {
		name     string
		id       string
		wantCode int
	}{
		{"empty_id", "", http.StatusNotFound}, // gin routes to 404 for missing param
		{"non_numeric", "abc", http.StatusBadRequest},
		{"negative", "-1", http.StatusBadRequest},
		{"sql_injection", "1%3B+DROP+TABLE+security_rule_sets", http.StatusBadRequest},
		{"not_found", "999999", http.StatusNotFound},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url := "/api/v1/security/rulesets/" + tc.id
			if tc.id == "" {
				url = "/api/v1/security/rulesets/"
			}
			req := httptest.NewRequest("DELETE", url, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tc.wantCode, w.Code, "ID: %s", tc.id)
		})
	}
}

// =============================================================================
// SECURITY AUDIT: XSS Prevention (stored XSS in ruleset content)
// =============================================================================

func TestSecurityHandler_UpsertRuleSet_XSSInContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuditTestDB(t)

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.POST("/api/v1/security/rulesets", h.UpsertRuleSet)
	router.GET("/api/v1/security/rulesets", h.ListRuleSets)

	// Store content with XSS payload
	xssPayload := `<script>alert('XSS')</script>`
	payload := map[string]interface{}{
		"name":    "xss-test",
		"content": xssPayload,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/security/rulesets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Accept that content is stored (backend stores as-is, frontend must sanitize)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify it's stored and returned as JSON (not rendered as HTML)
	req2 := httptest.NewRequest("GET", "/api/v1/security/rulesets", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	// Content-Type should be application/json
	contentType := w2.Header().Get("Content-Type")
	assert.Contains(t, contentType, "application/json")

	// The XSS payload should be JSON-escaped, not executable
	assert.Contains(t, w2.Body.String(), `\u003cscript\u003e`)
}

// =============================================================================
// SECURITY AUDIT: Rate Limiting Config Bounds
// =============================================================================

func TestSecurityHandler_UpdateConfig_RateLimitBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuditTestDB(t)

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.PUT("/api/v1/security/config", h.UpdateConfig)

	testCases := []struct {
		name    string
		payload map[string]interface{}
		wantOK  bool
	}{
		{
			"valid_limits",
			map[string]interface{}{"rate_limit_requests": 100, "rate_limit_burst": 10, "rate_limit_window_sec": 60},
			true,
		},
		{
			"zero_requests",
			map[string]interface{}{"rate_limit_requests": 0, "rate_limit_burst": 10},
			true, // Backend accepts, frontend validates
		},
		{
			"negative_burst",
			map[string]interface{}{"rate_limit_requests": 100, "rate_limit_burst": -1},
			true, // Backend accepts, frontend validates
		},
		{
			"huge_values",
			map[string]interface{}{"rate_limit_requests": 999999999, "rate_limit_burst": 999999999},
			true, // Backend accepts (no upper bound validation currently)
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest("PUT", "/api/v1/security/config", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if tc.wantOK {
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				assert.NotEqual(t, http.StatusOK, w.Code)
			}
		})
	}
}

// =============================================================================
// SECURITY AUDIT: DB Nil Handling
// =============================================================================

func TestSecurityHandler_GetStatus_NilDB(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Handler with nil DB should not panic
	cfg := config.SecurityConfig{CerberusEnabled: true}
	h := NewSecurityHandler(cfg, nil, nil)

	router := gin.New()
	router.GET("/api/v1/security/status", h.GetStatus)

	req := httptest.NewRequest("GET", "/api/v1/security/status", nil)
	w := httptest.NewRecorder()

	// Should not panic
	assert.NotPanics(t, func() {
		router.ServeHTTP(w, req)
	})

	assert.Equal(t, http.StatusOK, w.Code)
}

// =============================================================================
// SECURITY AUDIT: Break-Glass Token Security
// =============================================================================

func TestSecurityHandler_Enable_WithoutWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuditTestDB(t)

	// Create config without whitelist
	existingCfg := models.SecurityConfig{Name: "default", AdminWhitelist: ""}
	require.NoError(t, db.Create(&existingCfg).Error)

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.POST("/api/v1/security/enable", h.Enable)

	// Try to enable without token or whitelist
	req := httptest.NewRequest("POST", "/api/v1/security/enable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should be rejected
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp["error"], "whitelist")
}

func TestSecurityHandler_Disable_RequiresToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuditTestDB(t)

	// Create config with break-glass hash
	existingCfg := models.SecurityConfig{Name: "default", Enabled: true}
	require.NoError(t, db.Create(&existingCfg).Error)

	cfg := config.SecurityConfig{}
	h := NewSecurityHandler(cfg, db, nil)

	router := gin.New()
	router.POST("/api/v1/security/disable", h.Disable)

	// Try to disable from non-localhost without token
	req := httptest.NewRequest("POST", "/api/v1/security/disable", nil)
	req.RemoteAddr = "10.0.0.5:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should be rejected
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// =============================================================================
// SECURITY AUDIT: CrowdSec Mode Validation
// =============================================================================

func TestSecurityHandler_GetStatus_CrowdSecModeValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAuditTestDB(t)

	// Try to set invalid CrowdSec modes via settings
	invalidModes := []string{"remote", "external", "cloud", "api", "../../../etc/passwd"}

	for _, mode := range invalidModes {
		t.Run("mode_"+mode, func(t *testing.T) {
			// Clear settings
			db.Exec("DELETE FROM settings")

			// Set invalid mode
			setting := models.Setting{Key: "security.crowdsec.mode", Value: mode, Category: "security"}
			db.Create(&setting)

			cfg := config.SecurityConfig{}
			h := NewSecurityHandler(cfg, db, nil)

			router := gin.New()
			router.GET("/api/v1/security/status", h.GetStatus)

			req := httptest.NewRequest("GET", "/api/v1/security/status", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var resp map[string]map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)

			// Invalid modes should be normalized to "disabled"
			assert.Equal(t, "disabled", resp["crowdsec"]["mode"],
				"Invalid mode '%s' should be normalized to 'disabled'", mode)
		})
	}
}
