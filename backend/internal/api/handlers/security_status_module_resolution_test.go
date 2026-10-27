package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// statusModules is the subset of the status response this file asserts on.
type statusModules struct {
	WAF struct {
		Mode    string `json:"mode"`
		Enabled bool   `json:"enabled"`
	} `json:"waf"`
	RateLimit struct {
		Mode    string `json:"mode"`
		Enabled bool   `json:"enabled"`
	} `json:"rate_limit"`
}

// getStatusModules calls GetStatus against db with the static configuration cfg and returns the module block.
func getStatusModules(t *testing.T, db *gorm.DB, cfg config.SecurityConfig) statusModules {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewSecurityHandler(cfg, db, nil)
	t.Cleanup(h.Close)

	router := gin.New()
	router.GET("/api/v1/security/status", h.GetStatus)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/security/status", http.NoBody))
	require.Equal(t, http.StatusOK, w.Code)

	var body statusModules
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

// seedStatusModuleState creates the default row and an optional WAF settings key.
func seedStatusModuleState(t *testing.T, db *gorm.DB, row models.SecurityConfig, wafKey string) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	row.Name = models.DefaultSecurityConfigName
	row.Enabled = true
	require.NoError(t, db.Create(&row).Error)
	if wafKey != "" {
		require.NoError(t, db.Create(&models.Setting{Key: models.SecurityWAFEnabledSettingKey, Value: wafKey, Category: "security", Type: "bool"}).Error)
	}
}

// TestGetStatus_SettingKeyDecidesDisplayedMode pins that a recognised WAF or rate-limit key sets both the enabled
// flag and the displayed mode, using the same padded, case-insensitive reading as the proxy.
func TestGetStatus_SettingKeyDecidesDisplayedMode(t *testing.T) {
	t.Run("false key over block row shows disabled", func(t *testing.T) {
		db := setupTestDB(t)
		seedStatusModuleState(t, db, models.SecurityConfig{WAFMode: "block"}, "false")

		got := getStatusModules(t, db, config.SecurityConfig{CerberusEnabled: true})
		assert.False(t, got.WAF.Enabled)
		assert.Equal(t, "disabled", got.WAF.Mode)
	})

	t.Run("padded mixed-case true key over disabled row shows enabled", func(t *testing.T) {
		db := setupTestDB(t)
		seedStatusModuleState(t, db, models.SecurityConfig{WAFMode: "disabled"}, " TRUE ")

		got := getStatusModules(t, db, config.SecurityConfig{CerberusEnabled: true})
		assert.True(t, got.WAF.Enabled)
		assert.Equal(t, "enabled", got.WAF.Mode)
	})

	t.Run("rate limit key false over enabled row shows disabled", func(t *testing.T) {
		db := setupTestDB(t)
		seedStatusModuleState(t, db, models.SecurityConfig{RateLimitMode: "enabled", RateLimitEnable: true}, "")
		require.NoError(t, db.Create(&models.Setting{Key: models.SecurityRateLimitEnabledSettingKey, Value: "false", Category: "security", Type: "bool"}).Error)

		got := getStatusModules(t, db, config.SecurityConfig{CerberusEnabled: true})
		assert.False(t, got.RateLimit.Enabled)
		assert.Equal(t, "disabled", got.RateLimit.Mode)
	})
}

// TestGetStatus_UnrecognisedKeyUsesRow pins that a value other than true or false does not decide the module, so
// the default row shows through.
func TestGetStatus_UnrecognisedKeyUsesRow(t *testing.T) {
	db := setupTestDB(t)
	seedStatusModuleState(t, db, models.SecurityConfig{WAFMode: "block"}, "maybe")

	got := getStatusModules(t, db, config.SecurityConfig{CerberusEnabled: true})
	assert.True(t, got.WAF.Enabled)
	assert.Equal(t, "block", got.WAF.Mode)
}

// TestGetStatus_SettingsReadErrorKeepsRowState pins that a failed settings read still reports the default row and
// the static configuration, instead of failing the status request.
func TestGetStatus_SettingsReadErrorKeepsRowState(t *testing.T) {
	db := setupTestDB(t)
	seedStatusModuleState(t, db, models.SecurityConfig{WAFMode: "block", RateLimitMode: "enabled"}, "")
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:fail_settings_status", func(tx *gorm.DB) {
		if tx.Statement.Table == "settings" {
			_ = tx.AddError(errors.New("forced settings failure"))
		}
	}))

	got := getStatusModules(t, db, config.SecurityConfig{CerberusEnabled: true})
	assert.True(t, got.WAF.Enabled)
	assert.Equal(t, "block", got.WAF.Mode)
	assert.True(t, got.RateLimit.Enabled)
}

// TestGetStatus_WAFModeLabelFollowsStoredAction pins the displayed WAF mode when a settings key decides the state:
// an ON WAF shows the stored block or monitor mode, and any other stored value keeps the enabled label.
func TestGetStatus_WAFModeLabelFollowsStoredAction(t *testing.T) {
	tests := []struct {
		name        string
		row         models.SecurityConfig
		wafKey      string
		static      config.SecurityConfig
		wantEnabled bool
		wantMode    string
	}{
		{
			name:        "key true over block row shows block",
			row:         models.SecurityConfig{WAFMode: "block"},
			wafKey:      "true",
			wantEnabled: true,
			wantMode:    "block",
		},
		{
			name:        "key true over padded mixed-case monitor row shows lowercase monitor",
			row:         models.SecurityConfig{WAFMode: " Monitor "},
			wafKey:      "true",
			wantEnabled: true,
			wantMode:    "monitor",
		},
		{
			name:        "key true over disabled row shows enabled",
			row:         models.SecurityConfig{WAFMode: "disabled"},
			wafKey:      "true",
			wantEnabled: true,
			wantMode:    "enabled",
		},
		{
			name:        "key true over empty row shows enabled",
			row:         models.SecurityConfig{},
			wafKey:      "true",
			wantEnabled: true,
			wantMode:    "enabled",
		},
		{
			name:        "key false over block row shows disabled",
			row:         models.SecurityConfig{WAFMode: "block"},
			wafKey:      "false",
			wantEnabled: false,
			wantMode:    "disabled",
		},
		{
			name:        "key absent over block row shows the stored block mode",
			row:         models.SecurityConfig{WAFMode: "block"},
			wantEnabled: true,
			wantMode:    "block",
		},
		{
			name:        "static WAF only shows the static mode",
			static:      config.SecurityConfig{WAFMode: "monitor"},
			wantEnabled: true,
			wantMode:    "monitor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			// An empty row has no WAF opinion, so the static-only case behaves as if no row existed.
			seedStatusModuleState(t, db, tt.row, tt.wafKey)

			static := tt.static
			static.CerberusEnabled = true
			got := getStatusModules(t, db, static)
			assert.Equal(t, tt.wantEnabled, got.WAF.Enabled)
			assert.Equal(t, tt.wantMode, got.WAF.Mode)
		})
	}
}
