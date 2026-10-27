package handlers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// TestUpdateConfig_StorageFailureAnswers500 describes a save that cannot read the row: it is a storage failure, not a
// refused value, so it answers 500 with the generic message.
func TestUpdateConfig_StorageFailureAnswers500(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	require.NoError(t, db.Migrator().DropTable(&models.SecurityConfig{}))
	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	t.Cleanup(h.Close)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/security/config", h.UpdateConfig)

	w := serveSecurityPin(t, router, http.MethodPost, "/security/config", "127.0.0.1:5000", `{"waf_mode":"block"}`)
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "failed to save security config")
}

// TestEnableCerberus_WritesFeatureAndLegacyKeys describes the Cerberus feature switch: the feature key and its legacy
// mirror are written together with the category each key has.
func TestEnableCerberus_WritesFeatureAndLegacyKeys(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}, &models.SecurityAudit{}))
	h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	t.Cleanup(h.Close)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.POST("/security/cerberus/enable", h.EnableCerberus)
	router.POST("/security/cerberus/disable", h.DisableCerberus)

	w := serveSecurityPin(t, router, http.MethodPost, "/security/cerberus/enable", "127.0.0.1:5000", `{}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	feature, _ := settingValue(t, db, models.CerberusFeatureSettingKey)
	legacy, _ := settingValue(t, db, models.CerberusLegacySettingKey)
	assert.Equal(t, "true", feature)
	assert.Equal(t, "true", legacy)

	var stored models.Setting
	require.NoError(t, db.Where("key = ?", models.CerberusFeatureSettingKey).First(&stored).Error)
	assert.Equal(t, "feature", stored.Category)

	w = serveSecurityPin(t, router, http.MethodPost, "/security/cerberus/disable", "127.0.0.1:5000", `{}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	feature, _ = settingValue(t, db, models.CerberusFeatureSettingKey)
	legacy, _ = settingValue(t, db, models.CerberusLegacySettingKey)
	assert.Equal(t, "false", feature)
	assert.Equal(t, "false", legacy)
}

// countingInvalidator counts the Cerberus cache drops.
type countingInvalidator struct {
	calls int
}

// InvalidateCache records one drop.
func (c *countingInvalidator) InvalidateCache() { c.calls++ }

// failSettingWritesFor makes the create of the named settings key fail; an empty key fails every settings create.
func failSettingWritesFor(db *gorm.DB, key string) {
	_ = db.Callback().Create().Before("gorm:create").Register("test:fail_setting_create_"+key, func(tx *gorm.DB) {
		if tx.Statement.Table != "settings" {
			return
		}
		if setting, ok := tx.Statement.Dest.(*models.Setting); ok && (key == "" || setting.Key == key) {
			_ = tx.AddError(errors.New("forced create failure"))
		}
	})
}

// failSettingsQueries makes every query on the settings table fail.
func failSettingsQueries(db *gorm.DB) {
	_ = db.Callback().Query().Before("gorm:query").Register("test:fail_settings_query", func(tx *gorm.DB) {
		if tx.Statement.Table == "settings" {
			_ = tx.AddError(errors.New("forced query failure"))
		}
	})
}

// newModuleWriterDB opens a database with the settings and security tables and a default row.
func newModuleWriterDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}, &models.SecurityAudit{}))
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: false, WAFMode: "disabled"}).Error)
	return db
}

// TestModuleWriters_StorageFailuresAnswer500 describes the settings writers failing to read the row or the module
// keys: the request answers 500 and nothing is applied.
func TestModuleWriters_StorageFailuresAnswer500(t *testing.T) {
	t.Run("the row cannot be read", func(t *testing.T) {
		db := newModuleWriterDB(t)
		require.NoError(t, db.Migrator().DropTable(&models.SecurityConfig{}))
		router := newModuleSettingsRouter(t, db, nil)

		w := serveSecurityPin(t, router, http.MethodPost, "/settings", "127.0.0.1:5000", `{"key":"security.waf.enabled","value":"true"}`)
		assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
		w = serveSecurityPin(t, router, http.MethodPatch, "/config", "127.0.0.1:5000", `{"security.waf.enabled":"true"}`)
		assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	})

	t.Run("the settings cannot be read", func(t *testing.T) {
		db := newModuleWriterDB(t)
		failSettingsQueries(db)
		router := newModuleSettingsRouter(t, db, nil)

		w := serveSecurityPin(t, router, http.MethodPost, "/settings", "127.0.0.1:5000", `{"key":"security.waf.enabled","value":"true"}`)
		assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
		w = serveSecurityPin(t, router, http.MethodPatch, "/config", "127.0.0.1:5000", `{"security.waf.enabled":"true"}`)
		assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	})

	t.Run("the toggle cannot capture the state", func(t *testing.T) {
		db := newModuleWriterDB(t)
		failSettingsQueries(db)
		h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
		t.Cleanup(h.Close)

		w := serveSecurityPin(t, newModuleRouter(t, h), http.MethodPost, "/security/waf/enable", "127.0.0.1:5000", `{}`)
		assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	})

	t.Run("the module key cannot be written", func(t *testing.T) {
		db := newModuleWriterDB(t)
		failSettingWritesFor(db, models.SecurityWAFEnabledSettingKey)
		router := newModuleSettingsRouter(t, db, nil)

		w := serveSecurityPin(t, router, http.MethodPost, "/settings", "127.0.0.1:5000", `{"key":"security.waf.enabled","value":"true"}`)
		assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
		w = serveSecurityPin(t, router, http.MethodPatch, "/config", "127.0.0.1:5000", `{"security.waf.enabled":"true"}`)
		assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	})
}

// TestEnableCerberus_FeatureWriteFailuresAnswer500 describes the Cerberus feature switch failing to write either of
// its two keys: the request answers 500.
func TestEnableCerberus_FeatureWriteFailuresAnswer500(t *testing.T) {
	for _, key := range []string{models.CerberusFeatureSettingKey, models.CerberusLegacySettingKey} {
		t.Run(key, func(t *testing.T) {
			db := newModuleWriterDB(t)
			failSettingWritesFor(db, key)
			h := NewSecurityHandler(config.SecurityConfig{}, db, nil)
			t.Cleanup(h.Close)

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("role", "admin")
				c.Next()
			})
			router.POST("/security/cerberus/enable", h.EnableCerberus)

			w := serveSecurityPin(t, router, http.MethodPost, "/security/cerberus/enable", "127.0.0.1:5000", `{}`)
			require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
		})
	}
}

// TestModuleWriters_InvalidateCerberusCache describes the toggle and the config save dropping the Cerberus cache so the
// middleware reads the new state.
func TestModuleWriters_InvalidateCerberusCache(t *testing.T) {
	db := newModuleWriterDB(t)
	cache := &countingInvalidator{}
	h := NewSecurityHandlerWithDeps(config.SecurityConfig{}, db, nil, cache)
	t.Cleanup(h.Close)
	router := newModuleRouter(t, h)

	w := serveSecurityPin(t, router, http.MethodPost, "/security/waf/enable", "127.0.0.1:5000", `{}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 1, cache.calls, "the toggle drops the cache")

	w = serveSecurityPin(t, router, http.MethodPost, "/security/config", "127.0.0.1:5000", `{"waf_mode":"monitor"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 2, cache.calls, "the config save drops the cache")
}

// TestSecurityStatus_ReportsReasonForHostWithoutHandler describes the WAF on with no ruleset left: the apply builds no
// handler, and the status reports the reason and the host count.
func TestSecurityStatus_ReportsReasonForHostWithoutHandler(t *testing.T) {
	h, db := newStatusApplyFixture(t, models.SecurityConfig{Enabled: true, WAFMode: "block"}, noopCaddyClient{})
	require.NoError(t, db.Where("uuid = ?", ruleSetUUID).Delete(&models.SecurityRuleSet{}).Error)
	mustApplyConfig(t, h)

	waf := getSecurityStatus(t, h)["waf"].(map[string]any)
	assert.Equal(t, false, waf["enforced"])
	assert.Equal(t, float64(1), waf["unprotected_hosts"])
	assert.Equal(t, "no_ruleset", waf["reason"])
}
