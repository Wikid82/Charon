package models_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func openCrowdSecSettingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "charon.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	return db
}

func seedCrowdSecSetting(t *testing.T, db *gorm.DB, key, value string) {
	t.Helper()
	require.NoError(t, db.Create(&models.Setting{Key: key, Value: value, Category: "security", Type: "string"}).Error)
}

func TestCrowdSecSettingOverride_NilDBIsNotFound(t *testing.T) {
	enabled, found, err := models.CrowdSecSettingOverride(nil)
	require.NoError(t, err)
	assert.False(t, enabled)
	assert.False(t, found)
}

func TestCrowdSecSettingOverride_EnabledValueDecides(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "true", value: "true", want: true},
		{name: "true is case insensitive", value: "TRUE", want: true},
		{name: "padded true is trimmed", value: "  true  ", want: true},
		{name: "false", value: "false", want: false},
		{name: "any other value is off", value: "yes", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openCrowdSecSettingTestDB(t)
			seedCrowdSecSetting(t, db, models.CrowdSecEnabledSettingKey, tt.value)

			enabled, found, err := models.CrowdSecSettingOverride(db)
			require.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, tt.want, enabled)
		})
	}
}

func TestCrowdSecSettingOverride_LeftoverModeKeyIsIgnored(t *testing.T) {
	tests := []struct {
		name         string
		enabledValue *string
		modeValue    string
		wantFound    bool
		wantEnabled  bool
	}{
		{name: "enabled absent, mode local", modeValue: "local", wantFound: false},
		{name: "enabled absent, mode disabled", modeValue: "disabled", wantFound: false},
		{name: "enabled blank, mode local", enabledValue: strPtr(" \t "), modeValue: "local", wantFound: false},
		{name: "enabled false, mode local", enabledValue: strPtr("false"), modeValue: "local", wantFound: true, wantEnabled: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openCrowdSecSettingTestDB(t)
			seedCrowdSecSetting(t, db, "security.crowdsec.mode", tt.modeValue)
			if tt.enabledValue != nil {
				seedCrowdSecSetting(t, db, models.CrowdSecEnabledSettingKey, *tt.enabledValue)
			}

			enabled, found, err := models.CrowdSecSettingOverride(db)
			require.NoError(t, err)
			assert.Equal(t, tt.wantFound, found, "an absent or blank enabled key must not decide from the mode key")
			assert.Equal(t, tt.wantEnabled, enabled)
		})
	}
}

func TestCrowdSecSettingOverride_NoRowsIsNotFound(t *testing.T) {
	db := openCrowdSecSettingTestDB(t)

	enabled, found, err := models.CrowdSecSettingOverride(db)
	require.NoError(t, err)
	assert.False(t, enabled)
	assert.False(t, found)
}

func TestCrowdSecSettingOverride_MissingSettingsTableIsNotFound(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "charon.db")), &gorm.Config{})
	require.NoError(t, err)

	enabled, found, err := models.CrowdSecSettingOverride(db)
	require.NoError(t, err)
	assert.False(t, enabled)
	assert.False(t, found)
}

func TestCrowdSecSettingOverride_QueryErrorIsReturned(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "charon.db")), &gorm.Config{})
	require.NoError(t, err)
	// A settings table without the key column makes the lookup fail while the table itself exists.
	require.NoError(t, db.Exec("CREATE TABLE settings (id INTEGER PRIMARY KEY)").Error)

	_, found, err := models.CrowdSecSettingOverride(db)
	require.Error(t, err)
	assert.False(t, found)
}

func TestCrowdSecRowEnabled(t *testing.T) {
	tests := []struct {
		mode string
		want bool
	}{
		{mode: "local", want: true},
		{mode: "disabled", want: false},
		{mode: "external", want: false},
		{mode: "", want: false},
	}

	for _, tt := range tests {
		t.Run("mode "+tt.mode, func(t *testing.T) {
			assert.Equal(t, tt.want, models.CrowdSecRowEnabled(models.SecurityConfig{CrowdSecMode: tt.mode}))
		})
	}
}

func TestCrowdSecEnabled_SettingOverridesRow(t *testing.T) {
	db := openCrowdSecSettingTestDB(t)
	seedCrowdSecSetting(t, db, models.CrowdSecEnabledSettingKey, "false")

	enabled, err := models.CrowdSecEnabled(db, models.SecurityConfig{CrowdSecMode: "local"})
	require.NoError(t, err)
	assert.False(t, enabled)
}

func TestCrowdSecEnabled_NoSettingsTableUsesRow(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "charon.db")), &gorm.Config{})
	require.NoError(t, err)

	on, err := models.CrowdSecEnabled(db, models.SecurityConfig{CrowdSecMode: "local"})
	require.NoError(t, err)
	assert.True(t, on)

	off, err := models.CrowdSecEnabled(db, models.SecurityConfig{CrowdSecMode: "disabled"})
	require.NoError(t, err)
	assert.False(t, off)
}

func TestCrowdSecEnabled_QueryErrorFallsBackToRowAndReturnsError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "charon.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE settings (id INTEGER PRIMARY KEY)").Error)

	on, err := models.CrowdSecEnabled(db, models.SecurityConfig{CrowdSecMode: "local"})
	require.Error(t, err)
	assert.True(t, on)
}

// TestCrowdSecEnabled_LeftoverModeKeyHasNoEffect pins that a mode row left by an older build never changes the
// decision: the enabled key, when it has a value, decides, and otherwise the default row decides.
func TestCrowdSecEnabled_LeftoverModeKeyHasNoEffect(t *testing.T) {
	tests := []struct {
		name    string
		enabled *string
		mode    string
		row     string
		want    bool
	}{
		{name: "no enabled key, row local, mode disabled", mode: "disabled", row: "local", want: true},
		{name: "no enabled key, row disabled, mode local", mode: "local", row: "disabled", want: false},
		{name: "enabled true, row disabled, mode local", enabled: strPtr("true"), mode: "local", row: "disabled", want: true},
		{name: "enabled false, row local, mode local", enabled: strPtr("false"), mode: "local", row: "local", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openCrowdSecSettingTestDB(t)
			seedCrowdSecSetting(t, db, "security.crowdsec.mode", tt.mode)
			if tt.enabled != nil {
				seedCrowdSecSetting(t, db, models.CrowdSecEnabledSettingKey, *tt.enabled)
			}

			got, err := models.CrowdSecEnabled(db, models.SecurityConfig{CrowdSecMode: tt.row})
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func strPtr(s string) *string {
	return &s
}
