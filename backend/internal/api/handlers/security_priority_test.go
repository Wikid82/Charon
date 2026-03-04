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

// TestSecurityHandler_Priority_SettingsOverSecurityConfig verifies the three-tier priority system:
// 1. Settings table (highest)
// 2. SecurityConfig DB (middle)
// 3. Static config (lowest)
func TestSecurityHandler_Priority_SettingsOverSecurityConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name              string
		staticCfg         config.SecurityConfig
		dbSecurityConfig  *models.SecurityConfig
		settings          []models.Setting
		expectedWAFMode   string
		expectedWAFEnable bool
	}{
		{
			name: "Settings table overrides SecurityConfig DB",
			staticCfg: config.SecurityConfig{
				CerberusEnabled: true,
				WAFMode:         "disabled",
			},
			dbSecurityConfig: &models.SecurityConfig{
				Name:    "default",
				Enabled: true,
				WAFMode: "enabled",
			},
			settings: []models.Setting{
				{Key: "security.waf.enabled", Value: "false"},
			},
			expectedWAFMode:   "disabled",
			expectedWAFEnable: false,
		},
		{
			name: "SecurityConfig DB overrides static config",
			staticCfg: config.SecurityConfig{
				CerberusEnabled: true,
				WAFMode:         "disabled",
			},
			dbSecurityConfig: &models.SecurityConfig{
				Name:    "default",
				Enabled: true,
				WAFMode: "enabled",
			},
			settings:          []models.Setting{}, // No settings override
			expectedWAFMode:   "enabled",
			expectedWAFEnable: true,
		},
		{
			name: "Static config used when no DB overrides",
			staticCfg: config.SecurityConfig{
				CerberusEnabled: true,
				WAFMode:         "enabled",
			},
			dbSecurityConfig:  nil, // No DB config
			settings:          []models.Setting{},
			expectedWAFMode:   "enabled",
			expectedWAFEnable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))

			// Insert DB SecurityConfig if provided
			if tt.dbSecurityConfig != nil {
				require.NoError(t, db.Create(tt.dbSecurityConfig).Error)
			}

			// Insert settings
			for _, s := range tt.settings {
				require.NoError(t, db.Create(&s).Error)
			}

			handler := NewSecurityHandler(tt.staticCfg, db, nil)
			router := gin.New()
			router.GET("/security/status", handler.GetStatus)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/security/status", http.NoBody)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			waf := response["waf"].(map[string]any)
			assert.Equal(t, tt.expectedWAFMode, waf["mode"].(string), "WAF mode mismatch")
			assert.Equal(t, tt.expectedWAFEnable, waf["enabled"].(bool), "WAF enabled mismatch")
		})
	}
}

// TestSecurityHandler_Priority_AllModules verifies priority system works for all security modules
func TestSecurityHandler_Priority_AllModules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))

	// Static config has everything disabled
	staticCfg := config.SecurityConfig{
		CerberusEnabled: true,
		WAFMode:         "disabled",
		RateLimitMode:   "disabled",
		CrowdSecMode:    "disabled",
		ACLMode:         "disabled",
	}

	// DB SecurityConfig enables everything
	dbSecurityConfig := models.SecurityConfig{
		Name:          "default",
		Enabled:       true,
		WAFMode:       "enabled",
		RateLimitMode: "enabled",
		CrowdSecMode:  "local",
	}
	require.NoError(t, db.Create(&dbSecurityConfig).Error)

	// Settings table selectively overrides (WAF and ACL enabled, Rate Limit disabled)
	settings := []models.Setting{
		{Key: "security.waf.enabled", Value: "true"},
		{Key: "security.rate_limit.enabled", Value: "false"},
		{Key: "security.crowdsec.mode", Value: "disabled"},
		{Key: "security.acl.enabled", Value: "true"},
	}
	for _, s := range settings {
		require.NoError(t, db.Create(&s).Error)
	}

	handler := NewSecurityHandler(staticCfg, db, nil)
	router := gin.New()
	router.GET("/security/status", handler.GetStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/status", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify Settings table took precedence
	waf := response["waf"].(map[string]any)
	assert.True(t, waf["enabled"].(bool), "WAF should be enabled via settings")

	rateLimit := response["rate_limit"].(map[string]any)
	assert.False(t, rateLimit["enabled"].(bool), "Rate Limit should be disabled via settings")

	crowdsec := response["crowdsec"].(map[string]any)
	assert.Equal(t, "disabled", crowdsec["mode"].(string), "CrowdSec should be disabled via settings")
	assert.False(t, crowdsec["enabled"].(bool))

	acl := response["acl"].(map[string]any)
	assert.True(t, acl["enabled"].(bool), "ACL should be enabled via settings")
}
