package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

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
				CerberusEnabled: true,
				WAFMode:         "disabled",
				RateLimitMode:   "disabled",
				CrowdSecMode:    "disabled",
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
				CerberusEnabled: true,
				WAFMode:         "disabled",
				RateLimitMode:   "disabled",
				CrowdSecMode:    "disabled",
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
				CerberusEnabled: true,
				WAFMode:         "disabled",
				RateLimitMode:   "disabled",
				CrowdSecMode:    "disabled",
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
				CerberusEnabled: true,
				WAFMode:         "disabled",
				RateLimitMode:   "disabled",
				CrowdSecMode:    "disabled",
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
				CerberusEnabled: true,
				WAFMode:         "enabled",
				RateLimitMode:   "enabled",
				CrowdSecMode:    "local",
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
				CerberusEnabled: true,
				WAFMode:         "enabled",
				RateLimitMode:   "enabled",
				CrowdSecMode:    "local",
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
			req, _ := http.NewRequest("GET", "/security/status", http.NoBody)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			// Check WAF enabled
			waf := response["waf"].(map[string]any)
			assert.Equal(t, tt.expectedWAF, waf["enabled"].(bool), "WAF enabled mismatch")

			// Check Rate Limit enabled
			rateLimit := response["rate_limit"].(map[string]any)
			assert.Equal(t, tt.expectedRate, rateLimit["enabled"].(bool), "Rate Limit enabled mismatch")

			// Check CrowdSec enabled
			crowdsec := response["crowdsec"].(map[string]any)
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
		CerberusEnabled: true,
		WAFMode:         "disabled",
	}
	db.Create(&models.Setting{Key: "security.waf.enabled", Value: "true"})

	handler := NewSecurityHandler(cfg, db, nil)
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
		CerberusEnabled: true,
		RateLimitMode:   "disabled",
	}
	db.Create(&models.Setting{Key: "security.rate_limit.enabled", Value: "true"})

	handler := NewSecurityHandler(cfg, db, nil)
	router := gin.New()
	router.GET("/security/status", handler.GetStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/security/status", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	rateLimit := response["rate_limit"].(map[string]any)
	assert.True(t, rateLimit["enabled"].(bool))
}

func TestSecurityHandler_PatchACL_RequiresAdminWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDBWithMigrations(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", AdminWhitelist: "192.0.2.1/32"}).Error)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.PATCH("/security/acl", handler.PatchACL)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/security/acl", strings.NewReader(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.5:1234"
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSecurityHandler_PatchACL_AllowsWhitelistedIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := OpenTestDBWithMigrations(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", AdminWhitelist: "203.0.113.0/24"}).Error)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.PATCH("/security/acl", handler.PatchACL)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/security/acl", strings.NewReader(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.5:1234"
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var setting models.Setting
	err := db.Where("key = ?", "feature.cerberus.enabled").First(&setting).Error
	require.NoError(t, err)
	assert.Equal(t, "true", setting.Value)

	var cfg models.SecurityConfig
	err = handler.db.Where("name = ?", "default").First(&cfg).Error
	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
}

func TestSecurityHandler_PatchACL_SetsACLAndCerberusSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dsn := "file:TestSecurityHandler_PatchACL_SetsACLAndCerberusSettings?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Set("role", "admin")
	ctx.Set("userID", uint(1))
	ctx.Request, _ = http.NewRequest("PATCH", "/security/acl", strings.NewReader(`{"enabled":true}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.RemoteAddr = "203.0.113.5:1234"

	handler.toggleSecurityModule(ctx, "security.acl.enabled", true)

	assert.Equal(t, http.StatusOK, w.Code)

	var setting models.Setting
	err = db.Where("key = ?", "security.acl.enabled").First(&setting).Error
	require.NoError(t, err)
	assert.Equal(t, "true", setting.Value)

	var cerbSetting models.Setting
	err = db.Where("key = ?", "feature.cerberus.enabled").First(&cerbSetting).Error
	require.NoError(t, err)
	assert.Equal(t, "true", cerbSetting.Value)

	var legacySetting models.Setting
	err = db.Where("key = ?", "security.cerberus.enabled").First(&legacySetting).Error
	require.NoError(t, err)
	assert.Equal(t, "true", legacySetting.Value)
}

func TestSecurityHandler_EnsureSecurityConfigEnabled_CreatesWhenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)

	err := handler.ensureSecurityConfigEnabled()
	require.NoError(t, err)

	var cfg models.SecurityConfig
	err = handler.db.Where("name = ?", "default").First(&cfg).Error
	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
}

func TestSecurityHandler_PatchACL_AllowsEmergencyBypass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", AdminWhitelist: "192.0.2.1/32"}).Error)

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("emergency_bypass", true)
		c.Next()
	})
	router.PATCH("/security/acl", handler.PatchACL)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/security/acl", strings.NewReader(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.5:1234"
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
