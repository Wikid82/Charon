package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// createSyncTestRow stores the default row with the given mode and enabled column and returns it as loaded.
func createSyncTestRow(t *testing.T, db *gorm.DB, mode string, enabled bool) models.SecurityConfig {
	t.Helper()
	row := models.SecurityConfig{UUID: "sync-row", Name: models.DefaultSecurityConfigName, CrowdSecMode: mode, Enabled: enabled}
	require.NoError(t, db.Create(&row).Error)
	cfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	return cfg
}

func TestSyncCrowdSecRowToSetting_Table(t *testing.T) {
	tests := []struct {
		name        string
		settingSet  bool   // the enabled setting row is written
		settingVal  string // value of the enabled setting when settingSet is true
		rowMode     string
		rowEnabled  bool
		wantMode    string
		wantEnabled bool // the enabled column must come back unchanged
	}{
		{name: "setting true, row disabled", settingSet: true, settingVal: "true", rowMode: "disabled", wantMode: "local"},
		{name: "setting true, row local", settingSet: true, settingVal: "true", rowMode: "local", wantMode: "local"},
		{name: "setting TRUE with padding, row disabled", settingSet: true, settingVal: " TRUE ", rowMode: "disabled", wantMode: "local"},
		{name: "setting false, row local", settingSet: true, settingVal: "false", rowMode: "local", wantMode: "disabled"},
		{name: "setting false, row disabled", settingSet: true, settingVal: "false", rowMode: "disabled", wantMode: "disabled"},
		{name: "setting blank, row local", settingSet: true, settingVal: "", rowMode: "local", wantMode: "local"},
		{name: "setting whitespace, row disabled", settingSet: true, settingVal: "   ", rowMode: "disabled", wantMode: "disabled"},
		{name: "setting absent, row local", settingSet: false, rowMode: "local", wantMode: "local"},
		{name: "setting absent, row disabled", settingSet: false, rowMode: "disabled", wantMode: "disabled"},
		{name: "setting true, row disabled, enabled column true", settingSet: true, settingVal: "true", rowMode: "disabled", rowEnabled: true, wantMode: "local", wantEnabled: true},
		{name: "setting false, row local, enabled column true", settingSet: true, settingVal: "false", rowMode: "local", rowEnabled: true, wantMode: "disabled", wantEnabled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newRepairTestDB(t, true)
			if tt.settingSet {
				require.NoError(t, db.Create(&models.Setting{Key: models.CrowdSecEnabledSettingKey, Value: tt.settingVal, Category: "security", Type: "bool"}).Error)
			}
			cfg := createSyncTestRow(t, db, tt.rowMode, tt.rowEnabled)

			got, err := models.SyncCrowdSecRowToSetting(db, cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMode, got.CrowdSecMode, "returned row mode")

			stored, err := models.FindDefaultSecurityConfig(db)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMode, stored.CrowdSecMode, "stored row mode")
			assert.Equal(t, tt.rowEnabled, stored.Enabled, "enabled column must not change")
		})
	}
}

func TestSyncCrowdSecRowToSetting_MissingSettingsTableIsNoOp(t *testing.T) {
	db := newRepairTestDB(t, false)
	cfg := createSyncTestRow(t, db, "local", false)

	got, err := models.SyncCrowdSecRowToSetting(db, cfg)
	require.NoError(t, err)
	assert.Equal(t, "local", got.CrowdSecMode)
}

func TestSyncCrowdSecRowToSetting_IsIdempotent(t *testing.T) {
	db := newRepairTestDB(t, true)
	require.NoError(t, db.Create(&models.Setting{Key: models.CrowdSecEnabledSettingKey, Value: "true", Category: "security", Type: "bool"}).Error)
	cfg := createSyncTestRow(t, db, "disabled", false)

	first, err := models.SyncCrowdSecRowToSetting(db, cfg)
	require.NoError(t, err)
	second, err := models.SyncCrowdSecRowToSetting(db, first)
	require.NoError(t, err)
	assert.Equal(t, "local", first.CrowdSecMode)
	assert.Equal(t, first, second)
}

func TestSyncCrowdSecRowToSetting_SettingQueryErrorIsReturned(t *testing.T) {
	db := newRepairTestDB(t, false)
	// A settings table without the key column makes the read fail while the table exists, which is a real error
	// (not the missing-table case that reads as absent).
	require.NoError(t, db.Exec("CREATE TABLE settings (id INTEGER PRIMARY KEY)").Error)
	cfg := createSyncTestRow(t, db, "local", false)

	got, err := models.SyncCrowdSecRowToSetting(db, cfg)
	require.Error(t, err)
	assert.Equal(t, cfg, got, "the row is returned unchanged on error")
}

func TestSyncCrowdSecRowToSetting_WriteErrorIsReturned(t *testing.T) {
	db := newRepairTestDB(t, true)
	require.NoError(t, db.Create(&models.Setting{Key: models.CrowdSecEnabledSettingKey, Value: "true", Category: "security", Type: "bool"}).Error)
	cfg := createSyncTestRow(t, db, "disabled", false)
	require.NoError(t, db.Exec("CREATE TRIGGER fail_security_config_update BEFORE UPDATE ON security_configs BEGIN SELECT RAISE(ABORT, 'forced'); END").Error)

	got, err := models.SyncCrowdSecRowToSetting(db, cfg)
	require.Error(t, err)
	assert.Equal(t, "disabled", got.CrowdSecMode)
}
