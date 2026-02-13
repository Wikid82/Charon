package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

func jsonReader(data interface{}) io.Reader {
	b, _ := json.Marshal(data)
	return bytes.NewReader(b)
}

func setupEmergencyTestDB(t *testing.T) *gorm.DB {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&models.Setting{},
		&models.SecurityConfig{},
		&models.SecurityAudit{},
		&models.SecurityDecision{},
		&models.EmergencyToken{},
	)
	require.NoError(t, err)

	return db
}

func setupEmergencyRouter(handler *EmergencyHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	_ = router.SetTrustedProxies(nil)
	router.POST("/api/v1/emergency/security-reset", handler.SecurityReset)
	return router
}

type mockCaddyManager struct {
	calls int
}

func (m *mockCaddyManager) ApplyConfig(_ context.Context) error {
	m.calls++
	return nil
}

type mockCacheInvalidator struct {
	calls int
}

func (m *mockCacheInvalidator) InvalidateCache() {
	m.calls++
}

func TestEmergencySecurityReset_Success(t *testing.T) {
	// Setup
	db := setupEmergencyTestDB(t)
	handler := NewEmergencyHandler(db)
	router := setupEmergencyRouter(handler)

	// Configure valid token
	validToken := "this-is-a-valid-emergency-token-with-32-chars-minimum"
	_ = os.Setenv(EmergencyTokenEnvVar, validToken)
	defer func() { _ = os.Unsetenv(EmergencyTokenEnvVar) }()

	// Create initial security config to verify it gets disabled
	secConfig := models.SecurityConfig{
		Name:            "default",
		Enabled:         true,
		WAFMode:         "enabled",
		RateLimitMode:   "enabled",
		RateLimitEnable: true,
		CrowdSecMode:    "local",
	}
	require.NoError(t, db.Create(&secConfig).Error)

	// Make request with valid token
	req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
	req.Header.Set(EmergencyTokenHeader, validToken)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))
	assert.NotNil(t, response["disabled_modules"])
	disabledModules := response["disabled_modules"].([]interface{})
	assert.GreaterOrEqual(t, len(disabledModules), 5)

	// Verify settings were updated
	// Note: feature.cerberus.enabled is intentionally NOT disabled
	// The emergency reset only disables individual security modules (ACL, WAF, etc)
	// while keeping the Cerberus framework enabled for break glass testing

	// Verify ACL module is disabled
	var aclSetting models.Setting
	err = db.Where("key = ?", "security.acl.enabled").First(&aclSetting).Error
	require.NoError(t, err)
	assert.Equal(t, "false", aclSetting.Value)

	// Verify CrowdSec mode is disabled
	var crowdsecMode models.Setting
	err = db.Where("key = ?", "security.crowdsec.mode").First(&crowdsecMode).Error
	require.NoError(t, err)
	assert.Equal(t, "disabled", crowdsecMode.Value)

	// Verify admin whitelist is cleared
	var adminWhitelist models.Setting
	err = db.Where("key = ?", "security.admin_whitelist").First(&adminWhitelist).Error
	require.NoError(t, err)
	assert.Equal(t, "", adminWhitelist.Value)

	// Verify SecurityConfig was updated
	var updatedConfig models.SecurityConfig
	err = db.Where("name = ?", "default").First(&updatedConfig).Error
	require.NoError(t, err)
	assert.False(t, updatedConfig.Enabled)
	assert.Equal(t, "disabled", updatedConfig.WAFMode)
	assert.Equal(t, "", updatedConfig.AdminWhitelist)

	// Note: Audit logging is async via SecurityService channel, tested separately
}

func TestEmergencySecurityReset_InvalidToken(t *testing.T) {
	// Setup
	db := setupEmergencyTestDB(t)
	handler := NewEmergencyHandler(db)
	router := setupEmergencyRouter(handler)

	// Configure valid token
	validToken := "this-is-a-valid-emergency-token-with-32-chars-minimum"
	_ = os.Setenv(EmergencyTokenEnvVar, validToken)
	defer func() { _ = os.Unsetenv(EmergencyTokenEnvVar) }()

	// Make request with invalid token
	req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
	req.Header.Set(EmergencyTokenHeader, "wrong-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "unauthorized", response["error"])

	// Note: Audit logging is async via SecurityService channel, tested separately
}

func TestEmergencySecurityReset_MissingToken(t *testing.T) {
	// Setup
	db := setupEmergencyTestDB(t)
	handler := NewEmergencyHandler(db)
	router := setupEmergencyRouter(handler)

	// Configure valid token
	validToken := "this-is-a-valid-emergency-token-with-32-chars-minimum"
	_ = os.Setenv(EmergencyTokenEnvVar, validToken)
	defer func() { _ = os.Unsetenv(EmergencyTokenEnvVar) }()

	// Make request without token header
	req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "unauthorized", response["error"])
	assert.Contains(t, response["message"], "required")

	// Note: Audit logging is async via SecurityService channel, tested separately
}

func TestEmergencySecurityReset_NotConfigured(t *testing.T) {
	// Setup
	db := setupEmergencyTestDB(t)
	handler := NewEmergencyHandler(db)
	router := setupEmergencyRouter(handler)

	// Ensure token is not configured
	_ = os.Unsetenv(EmergencyTokenEnvVar)

	// Make request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
	req.Header.Set(EmergencyTokenHeader, "any-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusNotImplemented, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "not configured", response["error"])
	assert.Contains(t, response["message"], "CHARON_EMERGENCY_TOKEN")

	// Note: Audit logging is async via SecurityService channel, tested separately
}

func TestEmergencySecurityReset_TokenTooShort(t *testing.T) {
	// Setup
	db := setupEmergencyTestDB(t)
	handler := NewEmergencyHandler(db)
	router := setupEmergencyRouter(handler)

	// Configure token that is too short
	shortToken := "too-short"
	require.NoError(t, os.Setenv(EmergencyTokenEnvVar, shortToken))
	defer func() { require.NoError(t, os.Unsetenv(EmergencyTokenEnvVar)) }()

	// Make request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
	req.Header.Set(EmergencyTokenHeader, shortToken)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusNotImplemented, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "not configured", response["error"])
	assert.Contains(t, response["message"], "minimum length")
}

func TestEmergencySecurityReset_NoRateLimit(t *testing.T) {
	// Setup
	db := setupEmergencyTestDB(t)
	handler := NewEmergencyHandler(db)
	router := setupEmergencyRouter(handler)

	validToken := "this-is-a-valid-emergency-token-with-32-chars-minimum"
	require.NoError(t, os.Setenv(EmergencyTokenEnvVar, validToken))
	defer func() { require.NoError(t, os.Unsetenv(EmergencyTokenEnvVar)) }()

	wrongToken := "wrong-token-for-no-rate-limit-test-32chars"

	// Make rapid requests with invalid token; all should be unauthorized
	for i := 0; i < 10; i++ {
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
		req.Header.Set(EmergencyTokenHeader, wrongToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code, "Request %d should be unauthorized", i+1)

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "unauthorized", response["error"])
	}
}

func TestEmergencySecurityReset_TriggersReloadAndCacheInvalidate(t *testing.T) {
	// Setup
	db := setupEmergencyTestDB(t)
	mockCaddy := &mockCaddyManager{}
	mockCache := &mockCacheInvalidator{}
	handler := NewEmergencyHandlerWithDeps(db, mockCaddy, mockCache)
	router := setupEmergencyRouter(handler)

	validToken := "this-is-a-valid-emergency-token-with-32-chars-minimum"
	require.NoError(t, os.Setenv(EmergencyTokenEnvVar, validToken))
	defer func() { require.NoError(t, os.Unsetenv(EmergencyTokenEnvVar)) }()

	// Make request with valid token
	req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
	req.Header.Set(EmergencyTokenHeader, validToken)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, mockCaddy.calls)
	assert.Equal(t, 1, mockCache.calls)
}

func TestEmergencySecurityReset_ClearsBlockDecisions(t *testing.T) {
	db := setupEmergencyTestDB(t)
	handler := NewEmergencyHandler(db)
	router := setupEmergencyRouter(handler)

	validToken := "this-is-a-valid-emergency-token-with-32-chars-minimum"
	require.NoError(t, os.Setenv(EmergencyTokenEnvVar, validToken))
	defer func() { require.NoError(t, os.Unsetenv(EmergencyTokenEnvVar)) }()

	require.NoError(t, db.Create(&models.SecurityDecision{UUID: "dec-1", Source: "manual", Action: "block", IP: "127.0.0.1", CreatedAt: time.Now()}).Error)
	require.NoError(t, db.Create(&models.SecurityDecision{UUID: "dec-2", Source: "manual", Action: "allow", IP: "127.0.0.2", CreatedAt: time.Now()}).Error)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
	req.Header.Set(EmergencyTokenHeader, validToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var remaining []models.SecurityDecision
	require.NoError(t, db.Find(&remaining).Error)
	require.Len(t, remaining, 1)
	assert.Equal(t, "allow", remaining[0].Action)
}

func TestLogEnhancedAudit(t *testing.T) {
	// Setup
	db := setupEmergencyTestDB(t)
	handler := NewEmergencyHandler(db)
	defer handler.Close() // Flush async audit events

	// Test enhanced audit logging
	clientIP := "192.168.1.100"
	action := "emergency_reset_test"
	details := "Test audit log"
	duration := 150 * time.Millisecond

	handler.logEnhancedAudit(clientIP, action, details, true, duration)

	// Close to flush async events before querying DB
	handler.Close()

	// Verify audit log was created
	var audit models.SecurityAudit
	err := db.Where("actor = ?", clientIP).First(&audit).Error
	require.NoError(t, err, "Audit log should be created")

	assert.Equal(t, clientIP, audit.Actor)
	assert.Equal(t, action, audit.Action)
	assert.Contains(t, audit.Details, "result=success")
	assert.Contains(t, audit.Details, "duration=")
	assert.Contains(t, audit.Details, "timestamp=")
}

func TestNewEmergencyTokenHandler(t *testing.T) {
	db := setupEmergencyTestDB(t)

	// Create token service
	tokenService := services.NewEmergencyTokenService(db)

	// Create handler using the token handler constructor
	handler := NewEmergencyTokenHandler(tokenService)

	// Verify handler was created correctly
	require.NotNil(t, handler)
	require.NotNil(t, handler.db)
	require.NotNil(t, handler.tokenService)
	require.Nil(t, handler.securityService) // Token handler doesn't need security service

	// Cleanup
	handler.Close()
}

func TestGenerateToken_Success(t *testing.T) {
	db := setupEmergencyTestDB(t)
	tokenService := services.NewEmergencyTokenService(db)
	handler := NewEmergencyTokenHandler(tokenService)
	defer handler.Close()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/emergency/token", func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		handler.GenerateToken(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/token",
		jsonReader(map[string]interface{}{"expiration_days": 30}))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp["token"])
	assert.Equal(t, "30_days", resp["expiration_policy"])
}

func TestGenerateToken_AdminRequired(t *testing.T) {
	db := setupEmergencyTestDB(t)
	tokenService := services.NewEmergencyTokenService(db)
	handler := NewEmergencyTokenHandler(tokenService)
	defer handler.Close()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/emergency/token", func(c *gin.Context) {
		// No role set - simulating non-admin user
		handler.GenerateToken(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/token",
		jsonReader(map[string]interface{}{"expiration_days": 30}))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGenerateToken_InvalidExpirationDays(t *testing.T) {
	db := setupEmergencyTestDB(t)
	tokenService := services.NewEmergencyTokenService(db)
	handler := NewEmergencyTokenHandler(tokenService)
	defer handler.Close()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/emergency/token", func(c *gin.Context) {
		c.Set("role", "admin")
		handler.GenerateToken(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/token",
		jsonReader(map[string]interface{}{"expiration_days": 500}))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Expiration days must be between 0 and 365")
}

func TestGetTokenStatus_Success(t *testing.T) {
	db := setupEmergencyTestDB(t)
	tokenService := services.NewEmergencyTokenService(db)
	handler := NewEmergencyTokenHandler(tokenService)
	defer handler.Close()

	// Generate a token first
	_, _ = tokenService.Generate(services.GenerateRequest{ExpirationDays: 30})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/emergency/token/status", func(c *gin.Context) {
		c.Set("role", "admin")
		handler.GetTokenStatus(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/emergency/token/status", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	// Check key fields exist
	assert.True(t, resp["configured"].(bool))
	assert.Equal(t, "30_days", resp["expiration_policy"])
}

func TestGetTokenStatus_AdminRequired(t *testing.T) {
	db := setupEmergencyTestDB(t)
	tokenService := services.NewEmergencyTokenService(db)
	handler := NewEmergencyTokenHandler(tokenService)
	defer handler.Close()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/emergency/token/status", handler.GetTokenStatus)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/emergency/token/status", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRevokeToken_Success(t *testing.T) {
	db := setupEmergencyTestDB(t)
	tokenService := services.NewEmergencyTokenService(db)
	handler := NewEmergencyTokenHandler(tokenService)
	defer handler.Close()

	// Generate a token first
	_, _ = tokenService.Generate(services.GenerateRequest{ExpirationDays: 30})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/api/v1/emergency/token", func(c *gin.Context) {
		c.Set("role", "admin")
		handler.RevokeToken(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/emergency/token", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Emergency token revoked")
}

func TestRevokeToken_AdminRequired(t *testing.T) {
	db := setupEmergencyTestDB(t)
	tokenService := services.NewEmergencyTokenService(db)
	handler := NewEmergencyTokenHandler(tokenService)
	defer handler.Close()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/api/v1/emergency/token", handler.RevokeToken)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/emergency/token", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateTokenExpiration_Success(t *testing.T) {
	db := setupEmergencyTestDB(t)
	tokenService := services.NewEmergencyTokenService(db)
	handler := NewEmergencyTokenHandler(tokenService)
	defer handler.Close()

	// Generate a token first
	_, _ = tokenService.Generate(services.GenerateRequest{ExpirationDays: 30})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PATCH("/api/v1/emergency/token/expiration", func(c *gin.Context) {
		c.Set("role", "admin")
		handler.UpdateTokenExpiration(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/emergency/token/expiration",
		jsonReader(map[string]interface{}{"expiration_days": 60}))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "new_expires_at")
}

func TestUpdateTokenExpiration_AdminRequired(t *testing.T) {
	db := setupEmergencyTestDB(t)
	tokenService := services.NewEmergencyTokenService(db)
	handler := NewEmergencyTokenHandler(tokenService)
	defer handler.Close()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PATCH("/api/v1/emergency/token/expiration", handler.UpdateTokenExpiration)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/emergency/token/expiration",
		jsonReader(map[string]interface{}{"expiration_days": 60}))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateTokenExpiration_InvalidDays(t *testing.T) {
	db := setupEmergencyTestDB(t)
	tokenService := services.NewEmergencyTokenService(db)
	handler := NewEmergencyTokenHandler(tokenService)
	defer handler.Close()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PATCH("/api/v1/emergency/token/expiration", func(c *gin.Context) {
		c.Set("role", "admin")
		handler.UpdateTokenExpiration(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/emergency/token/expiration",
		jsonReader(map[string]interface{}{"expiration_days": 400}))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Expiration days must be between 0 and 365")
}
