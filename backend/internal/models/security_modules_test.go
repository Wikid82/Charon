package models_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestSeedDefaultSecurityConfig_KeepsExistingModeColumns pins that seeding over an existing default row
// returns and keeps the stored module mode columns.
func TestSeedDefaultSecurityConfig_KeepsExistingModeColumns(t *testing.T) {
	db := newSeedTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{
		Name:            models.DefaultSecurityConfigName,
		Enabled:         true,
		WAFMode:         "monitor",
		RateLimitMode:   "enabled",
		RateLimitEnable: true,
	}).Error)

	rec, err := models.SeedDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.Equal(t, "monitor", rec.WAFMode)
	assert.Equal(t, "enabled", rec.RateLimitMode)
	assert.True(t, rec.RateLimitEnable)

	var stored models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&stored).Error)
	assert.Equal(t, "monitor", stored.WAFMode)
	assert.Equal(t, "enabled", stored.RateLimitMode)
}

// newModuleTestDB opens an isolated in-memory database with the settings and security config tables.
func newModuleTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:module_resolution_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))
	return db
}

// setModuleSetting stores a settings value for key. An empty value is not stored, so the key is absent.
func setModuleSetting(t *testing.T, db *gorm.DB, key, value string) {
	t.Helper()
	if value == "" {
		return
	}
	require.NoError(t, db.Create(&models.Setting{Key: key, Value: value, Category: "security", Type: "bool"}).Error)
}

// moduleCase is one row of the WAF resolver truth table. An empty setting means the key is absent; a whitespace-only
// setting is stored but counts as absent. noRow passes the zero row (no default row exists).
type moduleCase struct {
	name    string
	setting string
	noRow   bool
	rowWAF  string
	static  bool
	want    bool
}

// TestModuleEnabled_TruthTable describes the shared module resolver: a recognised settings key wins, then the
// default row, then the static configuration. Callers apply the Cerberus-off gate, so this matrix is evaluated with
// Cerberus on. Cerberus off is pinned by the caddy and handler tests.
//
//	| key        | row (WAF mode)        | static | expected |
//	|------------|-----------------------|--------|----------|
//	| "true"     | disabled              | off    | on       |
//	| "false"    | block                 | on     | off      |
//	| absent     | block                 | off    | on       |
//	| absent     | disabled              | on     | off      |
//	| absent     | empty                 | on     | on       |
//	| absent     | empty                 | off    | off      |
//	| "maybe"    | block                 | off    | on       |
//	| " TRUE "   | disabled              | off    | on       |
//	| "   "      | disabled              | on     | off      |
//	| "  false " | block                 | on     | off      |
//	| absent     | "DISABLED" (any case) | on     | off      |
//	| absent     | no row                | on     | on       |
//	| absent     | no row                | off    | off      |
func TestModuleEnabled_TruthTable(t *testing.T) {
	cases := []moduleCase{
		{name: "true key over disabled row", setting: "true", rowWAF: "disabled", static: false, want: true},
		{name: "false key over block row", setting: "false", rowWAF: "block", static: true, want: false},
		{name: "absent key, block row", rowWAF: "block", static: false, want: true},
		{name: "absent key, disabled row", rowWAF: "disabled", static: true, want: false},
		{name: "absent key, empty row, static on", rowWAF: "", static: true, want: true},
		{name: "absent key, empty row, static off", rowWAF: "", static: false, want: false},
		{name: "unrecognised key ignored, block row", setting: "maybe", rowWAF: "block", static: false, want: true},
		{name: "padded mixed-case key recognised", setting: " TRUE ", rowWAF: "disabled", static: false, want: true},
		{name: "whitespace-only key counts as absent", setting: "   ", rowWAF: "disabled", static: true, want: false},
		{name: "padded false key recognised", setting: "  false ", rowWAF: "block", static: true, want: false},
		{name: "row disabled in any case", rowWAF: "DISABLED", static: true, want: false},
		{name: "no row, static on", noRow: true, static: true, want: true},
		{name: "no row, static off", noRow: true, static: false, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newModuleTestDB(t)
			row := models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: true, WAFMode: tc.rowWAF}
			if tc.noRow {
				row = models.SecurityConfig{}
			}
			setModuleSetting(t, db, models.SecurityWAFEnabledSettingKey, tc.setting)

			got, err := models.WAFEnabled(db, row, tc.static)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestModuleEnabled_WrapperCallbacksUseSharedRule pins that the rate-limit and ACL wrappers use the same key rule
// as the WAF, and that the rate-limit row decision ignores thresholds: the key decides, then the row mode, then the
// legacy enable flag, then the static configuration.
func TestModuleEnabled_WrapperCallbacksUseSharedRule(t *testing.T) {
	t.Run("rate limit key false overrides enabled row", func(t *testing.T) {
		db := newModuleTestDB(t)
		setModuleSetting(t, db, models.SecurityRateLimitEnabledSettingKey, "FALSE")
		row := models.SecurityConfig{RateLimitMode: "enabled", RateLimitEnable: true}

		got, err := models.RateLimitEnabled(db, row, true)
		require.NoError(t, err)
		assert.False(t, got)
	})

	t.Run("rate limit legacy enable flag without mode", func(t *testing.T) {
		db := newModuleTestDB(t)
		row := models.SecurityConfig{RateLimitEnable: true}

		got, err := models.RateLimitEnabled(db, row, false)
		require.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("rate limit mode wins over legacy enable flag", func(t *testing.T) {
		db := newModuleTestDB(t)
		row := models.SecurityConfig{RateLimitMode: "disabled", RateLimitEnable: true}

		got, err := models.RateLimitEnabled(db, row, true)
		require.NoError(t, err)
		assert.False(t, got)
	})

	t.Run("rate limit row without opinion keeps static", func(t *testing.T) {
		db := newModuleTestDB(t)

		got, err := models.RateLimitEnabled(db, models.SecurityConfig{}, true)
		require.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("rate limit zero thresholds do not change the key result", func(t *testing.T) {
		db := newModuleTestDB(t)
		row := models.SecurityConfig{RateLimitMode: "enabled", RateLimitRequests: 0, RateLimitWindowSec: 0}

		got, err := models.RateLimitEnabled(db, row, false)
		require.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("acl key is case-insensitive and has no row", func(t *testing.T) {
		db := newModuleTestDB(t)
		setModuleSetting(t, db, models.SecurityACLEnabledSettingKey, "TRUE")

		got, err := models.ACLEnabled(db, false)
		require.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("acl key false over static on", func(t *testing.T) {
		db := newModuleTestDB(t)
		setModuleSetting(t, db, models.SecurityACLEnabledSettingKey, "false")

		got, err := models.ACLEnabled(db, true)
		require.NoError(t, err)
		assert.False(t, got)
	})

	t.Run("acl without key uses static", func(t *testing.T) {
		db := newModuleTestDB(t)

		got, err := models.ACLEnabled(db, true)
		require.NoError(t, err)
		assert.True(t, got)
	})
}

// TestResolveModule_FromSetting pins that the resolution reports whether a settings key decided the state, which the
// status endpoint uses for its displayed mode.
func TestResolveModule_FromSetting(t *testing.T) {
	db := newModuleTestDB(t)
	row := models.SecurityConfig{WAFMode: "block"}

	fromRow, err := models.ResolveWAF(db, row, false)
	require.NoError(t, err)
	assert.Equal(t, models.ModuleResolution{Enabled: true, FromSetting: false}, fromRow)

	setModuleSetting(t, db, models.SecurityWAFEnabledSettingKey, "false")
	fromKey, err := models.ResolveWAF(db, row, true)
	require.NoError(t, err)
	assert.Equal(t, models.ModuleResolution{Enabled: false, FromSetting: true}, fromKey)
}

// TestResolveModule_SettingsQueryErrorFallsBackAndReturnsError pins that a failed settings read is returned together
// with the decision the row or the static configuration gives, and that a nil database skips the settings key.
func TestResolveModule_SettingsQueryErrorFallsBackAndReturnsError(t *testing.T) {
	t.Run("nil database uses row then static", func(t *testing.T) {
		got, err := models.WAFEnabled(nil, models.SecurityConfig{WAFMode: "block"}, false)
		require.NoError(t, err)
		assert.True(t, got)

		got, err = models.WAFEnabled(nil, models.SecurityConfig{}, true)
		require.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("settings query error with row decision", func(t *testing.T) {
		db := newModuleTestDB(t)
		failQueriesOn(db, "settings")

		got, err := models.WAFEnabled(db, models.SecurityConfig{WAFMode: "block"}, false)
		require.Error(t, err)
		assert.ErrorContains(t, err, "resolve "+models.SecurityWAFEnabledSettingKey)
		assert.True(t, got, "the row still decides when the settings read fails")
	})

	t.Run("settings query error without row decision uses static", func(t *testing.T) {
		db := newModuleTestDB(t)
		failQueriesOn(db, "settings")

		got, err := models.WAFEnabled(db, models.SecurityConfig{}, true)
		require.Error(t, err)
		assert.True(t, got)
	})
}

// TestStaticWAFEnabled pins that only an empty or "disabled" static WAF mode leaves the WAF off.
func TestStaticWAFEnabled(t *testing.T) {
	assert.False(t, models.StaticWAFEnabled(""))
	assert.False(t, models.StaticWAFEnabled("disabled"))
	assert.True(t, models.StaticWAFEnabled("monitor"))
	assert.True(t, models.StaticWAFEnabled("block"))
}

// TestDefaultSecurityConfigOrZero pins that the default row is returned when present, that a missing row or a nil
// database gives the zero value without an error, and that other query errors are returned.
func TestDefaultSecurityConfigOrZero(t *testing.T) {
	t.Run("returns the stored row", func(t *testing.T) {
		db := newModuleTestDB(t)
		require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: "monitor"}).Error)

		cfg, err := models.DefaultSecurityConfigOrZero(db)
		require.NoError(t, err)
		assert.Equal(t, "monitor", cfg.WAFMode)
	})

	t.Run("missing row is the zero value", func(t *testing.T) {
		db := newModuleTestDB(t)

		cfg, err := models.DefaultSecurityConfigOrZero(db)
		require.NoError(t, err)
		assert.Equal(t, models.SecurityConfig{}, cfg)
	})

	t.Run("nil database is the zero value", func(t *testing.T) {
		cfg, err := models.DefaultSecurityConfigOrZero(nil)
		require.NoError(t, err)
		assert.Equal(t, models.SecurityConfig{}, cfg)
	})

	t.Run("query error is returned with the zero value", func(t *testing.T) {
		db := newModuleTestDB(t)
		failQueriesOn(db, "security_configs")

		cfg, err := models.DefaultSecurityConfigOrZero(db)
		require.Error(t, err)
		assert.Equal(t, models.SecurityConfig{}, cfg)
	})
}

// failQueriesOn makes every query against table fail, to exercise the resolver's error paths.
func failQueriesOn(db *gorm.DB, table string) {
	_ = db.Callback().Query().Before("gorm:query").Register("test:fail_"+table, func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			_ = tx.AddError(errors.New("forced query failure"))
		}
	})
}
