package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

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

func TestEmergencySecurityReset_RateLimit(t *testing.T) {
	// Setup
	db := setupEmergencyTestDB(t)
	handler := NewEmergencyHandler(db)
	router := setupEmergencyRouter(handler)

	// Configure valid token
	validToken := "this-is-a-valid-emergency-token-with-32-chars-minimum"
	os.Setenv(EmergencyTokenEnvVar, validToken)
	defer os.Unsetenv(EmergencyTokenEnvVar)

	// Make 5 requests (the limit)
	for i := 0; i < MaxAttemptsPerWindow; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
		req.Header.Set(EmergencyTokenHeader, "wrong-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		// These should all be 401 Unauthorized (invalid token), not rate limited yet
		assert.Equal(t, http.StatusUnauthorized, w.Code, "Request %d should be 401", i+1)
	}

	// 6th request should be rate limited
	req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
	req.Header.Set(EmergencyTokenHeader, "wrong-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert rate limit response
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "rate limit exceeded", response["error"])

	// Note: Audit logging is async via SecurityService channel, tested separately
}

func TestEmergencySecurityReset_RateLimitWithValidToken(t *testing.T) {
	// Setup
	db := setupEmergencyTestDB(t)
	handler := NewEmergencyHandler(db)
	router := setupEmergencyRouter(handler)

	// Configure valid token
	validToken := "this-is-a-valid-emergency-token-with-32-chars-minimum"
	os.Setenv(EmergencyTokenEnvVar, validToken)
	defer os.Unsetenv(EmergencyTokenEnvVar)

	// Exhaust rate limit with invalid tokens
	for i := 0; i < MaxAttemptsPerWindow; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
		req.Header.Set(EmergencyTokenHeader, "wrong-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	// Even with valid token, should be rate limited
	req := httptest.NewRequest(http.MethodPost, "/api/v1/emergency/security-reset", nil)
	req.Header.Set(EmergencyTokenHeader, validToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert rate limit response (rate limiting happens before token validation)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestConstantTimeCompare(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{
			name:     "equal strings",
			a:        "hello-world-token",
			b:        "hello-world-token",
			expected: true,
		},
		{
			name:     "different strings",
			a:        "hello-world-token",
			b:        "goodbye-world-token",
			expected: false,
		},
		{
			name:     "different lengths",
			a:        "short",
			b:        "much-longer-string",
			expected: false,
		},
		{
			name:     "empty strings",
			a:        "",
			b:        "",
			expected: true,
		},
		{
			name:     "one empty",
			a:        "not-empty",
			b:        "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := constantTimeCompare(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCheckRateLimit(t *testing.T) {
	db := setupEmergencyTestDB(t)
	handler := NewEmergencyHandler(db)

	ip := "192.168.1.100"

	// First MaxAttemptsPerWindow attempts should pass
	for i := 0; i < MaxAttemptsPerWindow; i++ {
		allowed := handler.checkRateLimit(ip)
		assert.True(t, allowed, "Attempt %d should be allowed", i+1)
	}

	// Next attempt should be blocked
	allowed := handler.checkRateLimit(ip)
	assert.False(t, allowed, "Attempt after limit should be blocked")

	// Different IP should still be allowed
	differentIP := "192.168.1.101"
	allowed = handler.checkRateLimit(differentIP)
	assert.True(t, allowed, "Different IP should be allowed")
}
