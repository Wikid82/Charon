package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// TestSecurityHandler_GetStatus_RespectsSettingsTable verifies that GetStatus
// reads WAF, Rate Limit, and CrowdSec enabled states from the settings table,
// overriding the static config values.
func TestSecurityHandler_GetStatus_RespectsSettingsTable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		cfg           config.SecurityConfig
		settings      []models.Setting
		expectedWAF   bool
		expectedRate  bool
		expectedCrowd bool
	}{
		{
			name: "WAF enabled via settings overrides disabled config",
			cfg: config.SecurityConfig{
				WAFMode:       "disabled",
				RateLimitMode: "disabled",
				CrowdSecMode:  "disabled",
			},
			settings: []models.Setting{
				{Key: "security.waf.enabled", Value: "true"},
			},
			expectedWAF:   true,
			expectedRate:  false,
			expectedCrowd: false,
		},
		{
			name: "Rate Limit enabled via settings overrides disabled config",
			cfg: config.SecurityConfig{
				WAFMode:       "disabled",
				RateLimitMode: "disabled",
				CrowdSecMode:  "disabled",
			},
			settings: []models.Setting{
				{Key: "security.rate_limit.enabled", Value: "true"},
			},
			expectedWAF:   false,
			expectedRate:  true,
			expectedCrowd: false,
		},
		{
			name: "CrowdSec enabled via settings overrides disabled config",
			cfg: config.SecurityConfig{
				WAFMode:       "disabled",
				RateLimitMode: "disabled",
				CrowdSecMode:  "disabled",
			},
			settings: []models.Setting{
				{Key: "security.crowdsec.enabled", Value: "true"},
			},
			expectedWAF:   false,
			expectedRate:  false,
			expectedCrowd: true,
		},
		{
			name: "All modules enabled via settings",
			cfg: config.SecurityConfig{
				WAFMode:       "disabled",
				RateLimitMode: "disabled",
				CrowdSecMode:  "disabled",
			},
			settings: []models.Setting{
				{Key: "security.waf.enabled", Value: "true"},
				{Key: "security.rate_limit.enabled", Value: "true"},
				{Key: "security.crowdsec.enabled", Value: "true"},
			},
			expectedWAF:   true,
			expectedRate:  true,
			expectedCrowd: true,
		},
		{
			name: "WAF disabled via settings overrides enabled config",
			cfg: config.SecurityConfig{
				WAFMode:       "enabled",
				RateLimitMode: "enabled",
				CrowdSecMode:  "local",
			},
			settings: []models.Setting{
				{Key: "security.waf.enabled", Value: "false"},
				{Key: "security.rate_limit.enabled", Value: "false"},
				{Key: "security.crowdsec.enabled", Value: "false"},
			},
			expectedWAF:   false,
			expectedRate:  false,
			expectedCrowd: false,
		},
		{
			name: "No settings - falls back to config (enabled)",
			cfg: config.SecurityConfig{
				WAFMode:       "enabled",
				RateLimitMode: "enabled",
				CrowdSecMode:  "local",
			},
			settings:      []models.Setting{},
			expectedWAF:   true,
			expectedRate:  true,
			expectedCrowd: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			require.NoError(t, db.AutoMigrate(&models.Setting{}))

			// Insert settings
			for _, s := range tt.settings {
				db.Create(&s)
			}

			handler := NewSecurityHandler(tt.cfg, db, nil)
			router := gin.New()
			router.GET("/security/status", handler.GetStatus)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/security/status", nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			// Check WAF enabled
			waf := response["waf"].(map[string]interface{})
			assert.Equal(t, tt.expectedWAF, waf["enabled"].(bool), "WAF enabled mismatch")

			// Check Rate Limit enabled
			rateLimit := response["rate_limit"].(map[string]interface{})
			assert.Equal(t, tt.expectedRate, rateLimit["enabled"].(bool), "Rate Limit enabled mismatch")

			// Check CrowdSec enabled
			crowdsec := response["crowdsec"].(map[string]interface{})
			assert.Equal(t, tt.expectedCrowd, crowdsec["enabled"].(bool), "CrowdSec enabled mismatch")
		})
	}
}

// TestSecurityHandler_GetStatus_WAFModeFromSettings verifies that WAF mode
// is properly reflected when enabled via settings.
func TestSecurityHandler_GetStatus_WAFModeFromSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	// WAF config is disabled, but settings says enabled
	cfg := config.SecurityConfig{
		WAFMode: "disabled",
	}
	db.Create(&models.Setting{Key: "security.waf.enabled", Value: "true"})

	handler := NewSecurityHandler(cfg, db, nil)
	router := gin.New()
	router.GET("/security/status", handler.GetStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	waf := response["waf"].(map[string]interface{})
	// When enabled via settings, mode should reflect "enabled" state
	assert.True(t, waf["enabled"].(bool))
}

// TestSecurityHandler_GetStatus_RateLimitModeFromSettings verifies that Rate Limit mode
// is properly reflected when enabled via settings.
func TestSecurityHandler_GetStatus_RateLimitModeFromSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))

	// Rate limit config is disabled, but settings says enabled
	cfg := config.SecurityConfig{
		RateLimitMode: "disabled",
	}
	db.Create(&models.Setting{Key: "security.rate_limit.enabled", Value: "true"})

	handler := NewSecurityHandler(cfg, db, nil)
	router := gin.New()
	router.GET("/security/status", handler.GetStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	rateLimit := response["rate_limit"].(map[string]interface{})
	assert.True(t, rateLimit["enabled"].(bool))
}
