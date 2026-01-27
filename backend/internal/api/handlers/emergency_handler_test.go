package handlers

import (
	"encoding/json"
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
)

func setupEmergencyTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&models.Setting{},
		&models.SecurityConfig{},
		&models.SecurityAudit{},
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

func TestEmergencySecurityReset_Success(t *testing.T) {
	// Setup
	db := setupEmergencyTestDB(t)
	handler := NewEmergencyHandler(db)
	router := setupEmergencyRouter(handler)

	// Configure valid token
	validToken := "this-is-a-valid-emergency-token-with-32-chars-minimum"
	os.Setenv(EmergencyTokenEnvVar, validToken)
	defer os.Unsetenv(EmergencyTokenEnvVar)

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
	var setting models.Setting
	err = db.Where("key = ?", "feature.cerberus.enabled").First(&setting).Error
	require.NoError(t, err)
	assert.Equal(t, "false", setting.Value)

	// Verify SecurityConfig was updated
	var updatedConfig models.SecurityConfig
	err = db.Where("name = ?", "default").First(&updatedConfig).Error
	require.NoError(t, err)
	assert.False(t, updatedConfig.Enabled)
	assert.Equal(t, "disabled", updatedConfig.WAFMode)

	// Note: Audit logging is async via SecurityService channel, tested separately
}

func TestEmergencySecurityReset_InvalidToken(t *testing.T) {
	// Setup
	db := setupEmergencyTestDB(t)
	handler := NewEmergencyHandler(db)
	router := setupEmergencyRouter(handler)

	// Configure valid token
	validToken := "this-is-a-valid-emergency-token-with-32-chars-minimum"
	os.Setenv(EmergencyTokenEnvVar, validToken)
	defer os.Unsetenv(EmergencyTokenEnvVar)

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
	os.Setenv(EmergencyTokenEnvVar, validToken)
	defer os.Unsetenv(EmergencyTokenEnvVar)

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
	os.Unsetenv(EmergencyTokenEnvVar)

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
	os.Setenv(EmergencyTokenEnvVar, shortToken)
	defer os.Unsetenv(EmergencyTokenEnvVar)

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

func TestEmergencyRateLimiter(t *testing.T) {
	// Reset global limiter
	limiter := &emergencyRateLimiter{
		attempts: make(map[string][]time.Time),
	}

	testIP := "192.168.1.100"

	// Test: First 3 attempts should succeed
	for i := 0; i < emergencyRateLimit; i++ {
		limited := limiter.checkRateLimit(testIP)
		assert.False(t, limited, "Attempt %d should not be rate limited", i+1)
	}

	// Test: 4th attempt should be rate limited
	limited := limiter.checkRateLimit(testIP)
	assert.True(t, limited, "4th attempt should be rate limited")

	// Test: Multiple IPs should be tracked independently
	otherIP := "192.168.1.200"
	limited = limiter.checkRateLimit(otherIP)
	assert.False(t, limited, "Different IP should not be rate limited")
}

func TestEmergencySecurityReset_RateLimiting(t *testing.T) {
	// Setup
	db := setupEmergencyTestDB(t)
	handler := NewEmergencyHandler(db)
	router := setupEmergencyRouter(handler)

	validToken := "this-is-a-valid-emergency-token-with-32-chars-minimum"
	os.Setenv(EmergencyTokenEnvVar, validToken)
	defer os.Unsetenv(EmergencyTokenEnvVar)

	// Reset global rate limiter
	globalEmergencyLimiter = &emergencyRateLimiter{
		attempts: make(map[string][]time.Time),
	}

	// Make 3 successful requests (within rate limit)
	for i := 0; i < emergencyRateLimit; i++ {
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
		req.Header.Set(EmergencyTokenHeader, validToken)
		req.RemoteAddr = "192.168.1.100:12345"

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// First 3 should succeed
		assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
	}

	// 4th request should be rate limited
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
	req.Header.Set(EmergencyTokenHeader, validToken)
	req.RemoteAddr = "192.168.1.100:12345"

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code, "4th request should be rate limited")

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "rate limit exceeded", response["error"])
	assert.Contains(t, response["message"], "Maximum 3 attempts per minute")
}

func TestLogEnhancedAudit(t *testing.T) {
	// Setup
	db := setupEmergencyTestDB(t)
	handler := NewEmergencyHandler(db)

	// Test enhanced audit logging
	clientIP := "192.168.1.100"
	action := "emergency_reset_test"
	details := "Test audit log"
	duration := 150 * time.Millisecond

	handler.logEnhancedAudit(clientIP, action, details, true, duration)

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
