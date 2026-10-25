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

func TestCrowdSecSettingOverride_EnabledKeyWins(t *testing.T) {
	db := openCrowdSecSettingTestDB(t)
	seedCrowdSecSetting(t, db, models.CrowdSecDeprecatedModeSettingKey, "local")
	seedCrowdSecSetting(t, db, models.CrowdSecEnabledSettingKey, "false")

	enabled, found, err := models.CrowdSecSettingOverride(db)
	require.NoError(t, err)
	assert.True(t, found)
	assert.False(t, enabled)
}

func TestCrowdSecSettingOverride_EnabledValueIsCaseInsensitive(t *testing.T) {
	db := openCrowdSecSettingTestDB(t)
	seedCrowdSecSetting(t, db, models.CrowdSecEnabledSettingKey, "TRUE")

	enabled, found, err := models.CrowdSecSettingOverride(db)
	require.NoError(t, err)
	assert.True(t, found)
	assert.True(t, enabled)
}

func TestCrowdSecSettingOverride_EmptyEnabledFallsBackToDeprecatedMode(t *testing.T) {
	db := openCrowdSecSettingTestDB(t)
	seedCrowdSecSetting(t, db, models.CrowdSecEnabledSettingKey, "")
	seedCrowdSecSetting(t, db, models.CrowdSecDeprecatedModeSettingKey, "local")

	enabled, found, err := models.CrowdSecSettingOverride(db)
	require.NoError(t, err)
	assert.True(t, found)
	assert.True(t, enabled)
}

func TestCrowdSecSettingOverride_DeprecatedModeOtherThanLocalIsOff(t *testing.T) {
	db := openCrowdSecSettingTestDB(t)
	seedCrowdSecSetting(t, db, models.CrowdSecDeprecatedModeSettingKey, "remote")

	enabled, found, err := models.CrowdSecSettingOverride(db)
	require.NoError(t, err)
	assert.True(t, found)
	assert.False(t, enabled)
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
