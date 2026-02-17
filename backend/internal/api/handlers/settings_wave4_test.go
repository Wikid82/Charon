package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type wave4CaddyManager struct {
	calls int
	err   error
}

func (m *wave4CaddyManager) ApplyConfig(context.Context) error {
	m.calls++
	return m.err
}

type wave4CacheInvalidator struct {
	calls int
}

func (i *wave4CacheInvalidator) InvalidateCache() {
	i.calls++
}

func registerCreatePermissionDeniedHook(t *testing.T, db *gorm.DB, name string, shouldFail func(*gorm.DB) bool) {
	t.Helper()
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		if shouldFail(tx) {
			_ = tx.AddError(fmt.Errorf("permission denied"))
		}
	}))
	t.Cleanup(func() {
		_ = db.Callback().Create().Remove(name)
	})
}

func settingKeyFromCreateCallback(tx *gorm.DB) string {
	if tx == nil || tx.Statement == nil || tx.Statement.Dest == nil {
		return ""
	}
	switch v := tx.Statement.Dest.(type) {
	case *models.Setting:
		return v.Key
	case models.Setting:
		return v.Key
	default:
		return ""
	}
}

func performUpdateSettingRequest(t *testing.T, h *SettingsHandler, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	g := gin.New()
	g.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	g.POST("/settings", h.UpdateSetting)

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	g.ServeHTTP(w, req)
	return w
}

func performPatchConfigRequest(t *testing.T, h *SettingsHandler, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	g := gin.New()
	g.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	g.PATCH("/config", h.PatchConfig)

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	g.ServeHTTP(w, req)
	return w
}

func TestSettingsHandlerWave4_UpdateSetting_ACLPathsPermissionErrors(t *testing.T) {
	t.Run("feature cerberus upsert permission denied", func(t *testing.T) {
		db := setupSettingsWave3DB(t)
		registerCreatePermissionDeniedHook(t, db, "wave4-deny-feature-cerberus", func(tx *gorm.DB) bool {
			return settingKeyFromCreateCallback(tx) == "feature.cerberus.enabled"
		})

		h := NewSettingsHandler(db)
		h.SecuritySvc = services.NewSecurityService(db)
		h.DataRoot = "/app/data"

		w := performUpdateSettingRequest(t, h, map[string]any{
			"key":   "security.acl.enabled",
			"value": "true",
		})

		require.Equal(t, http.StatusInternalServerError, w.Code)
		require.Contains(t, w.Body.String(), "permissions_write_denied")
	})

}

func TestSettingsHandlerWave4_PatchConfig_SecurityReloadSuccessLogsPath(t *testing.T) {
	db := setupSettingsWave3DB(t)
	mgr := &wave4CaddyManager{}
	inv := &wave4CacheInvalidator{}

	h := NewSettingsHandlerWithDeps(db, mgr, inv, nil, "")
	w := performPatchConfigRequest(t, h, map[string]any{
		"security": map[string]any{
			"waf": map[string]any{"enabled": true},
		},
	})

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, mgr.calls)
	require.Equal(t, 1, inv.calls)
}

func TestSettingsHandlerWave4_UpdateSetting_GenericSaveError(t *testing.T) {
	db := setupSettingsWave3DB(t)
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("wave4-generic-save-error", func(tx *gorm.DB) {
		if settingKeyFromCreateCallback(tx) == "security.waf.enabled" {
			_ = tx.AddError(fmt.Errorf("boom"))
		}
	}))
	t.Cleanup(func() {
		_ = db.Callback().Create().Remove("wave4-generic-save-error")
	})

	h := NewSettingsHandler(db)
	h.SecuritySvc = services.NewSecurityService(db)
	h.DataRoot = "/app/data"

	w := performUpdateSettingRequest(t, h, map[string]any{
		"key":   "security.waf.enabled",
		"value": "true",
	})

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "Failed to save setting")
}

func TestSettingsHandlerWave4_PatchConfig_InvalidAdminWhitelistFromSync(t *testing.T) {
	db := setupSettingsWave3DB(t)
	h := NewSettingsHandler(db)
	h.SecuritySvc = services.NewSecurityService(db)
	h.DataRoot = "/app/data"

	w := performPatchConfigRequest(t, h, map[string]any{
		"security": map[string]any{
			"admin_whitelist": "10.10.10.10/",
		},
	})

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "Invalid admin_whitelist")
}

func TestSettingsHandlerWave4_TestPublicURL_BindError(t *testing.T) {
	db := setupSettingsWave3DB(t)
	h := NewSettingsHandler(db)

	g := gin.New()
	g.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	g.POST("/settings/test-public-url", h.TestPublicURL)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/settings/test-public-url", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	g.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
