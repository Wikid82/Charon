package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestReconcileCrowdSecOnStartup_SyncsDefaultRowToSetting checks the boot decision and the stored default row
// together: after reconciliation the row's crowd_sec_mode must match the enabled setting whenever the setting has a
// value, and the enabled column must never change.
func TestReconcileCrowdSecOnStartup_SyncsDefaultRowToSetting(t *testing.T) {
	tests := []struct {
		name        string
		settingSet  bool
		settingVal  string
		rowMode     string
		wantMode    string
		wantStarted bool
	}{
		{name: "setting true, row disabled", settingSet: true, settingVal: "true", rowMode: "disabled", wantMode: "local", wantStarted: true},
		{name: "setting true, row local", settingSet: true, settingVal: "true", rowMode: "local", wantMode: "local", wantStarted: true},
		{name: "setting false, row local", settingSet: true, settingVal: "false", rowMode: "local", wantMode: "disabled", wantStarted: false},
		{name: "setting false, row disabled", settingSet: true, settingVal: "false", rowMode: "disabled", wantMode: "disabled", wantStarted: false},
		{name: "setting blank, row local keeps row", settingSet: true, settingVal: "", rowMode: "local", wantMode: "local", wantStarted: true},
		{name: "setting blank, row disabled keeps row", settingSet: true, settingVal: "", rowMode: "disabled", wantMode: "disabled", wantStarted: false},
		{name: "setting absent, row local keeps row", settingSet: false, rowMode: "local", wantMode: "local", wantStarted: true},
		{name: "setting absent, row disabled keeps row", settingSet: false, rowMode: "disabled", wantMode: "disabled", wantStarted: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
			defer cleanup()
			db := setupCrowdsecTestDB(t)
			require.NoError(t, db.AutoMigrate(&models.Setting{}))
			if tt.settingSet {
				require.NoError(t, db.Create(&models.Setting{Key: models.CrowdSecEnabledSettingKey, Value: tt.settingVal, Category: "security", Type: "bool"}).Error)
			}
			require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName, CrowdSecMode: tt.rowMode, Enabled: true}).Error)

			exec := &smartMockCrowdsecExecutor{startPid: 12345}
			ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, &mockCommandExecutor{})

			assert.Equal(t, tt.wantStarted, exec.startCalled, "CrowdSec start decision")
			row, err := models.FindDefaultSecurityConfig(db)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMode, row.CrowdSecMode, "default row crowd_sec_mode")
			assert.True(t, row.Enabled, "the enabled column must not change")
		})
	}
}

// TestReconcileCrowdSecOnStartup_RenamedLegacyRowIsSynced covers the rename path: the legacy-named row becomes the
// default row first, and only then is it synced to the setting (the sync must see the renamed row).
func TestReconcileCrowdSecOnStartup_RenamedLegacyRowIsSynced(t *testing.T) {
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()
	db := setupCrowdsecTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	require.NoError(t, db.Create(&models.Setting{Key: models.CrowdSecEnabledSettingKey, Value: "true", Category: "security", Type: "bool"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-legacy", Name: legacyCrowdSecDefaultRowName, CrowdSecMode: "disabled"}).Error)

	exec := &smartMockCrowdsecExecutor{startPid: 12345}
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, &mockCommandExecutor{})

	assert.True(t, exec.startCalled)
	row, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.Equal(t, "local", row.CrowdSecMode)
}

// TestReconcileCrowdSecOnStartup_RowSyncIsIdempotent runs the reconciliation twice; the second run must leave the row
// exactly as the first one did.
func TestReconcileCrowdSecOnStartup_RowSyncIsIdempotent(t *testing.T) {
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()
	db := setupCrowdsecTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	require.NoError(t, db.Create(&models.Setting{Key: models.CrowdSecEnabledSettingKey, Value: "false", Category: "security", Type: "bool"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName, CrowdSecMode: "local"}).Error)

	ReconcileCrowdSecOnStartup(db, &smartMockCrowdsecExecutor{startPid: 12345}, binPath, dataDir, &mockCommandExecutor{})
	first, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)

	ReconcileCrowdSecOnStartup(db, &smartMockCrowdsecExecutor{startPid: 12345}, binPath, dataDir, &mockCommandExecutor{})
	second, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)

	assert.Equal(t, "disabled", first.CrowdSecMode)
	assert.Equal(t, first.CrowdSecMode, second.CrowdSecMode)
}

// TestReconcileCrowdSecOnStartup_RowSyncFailureKeepsSettingDecision forces the row write to fail. Startup logs a
// warning and the decision still follows the setting, so CrowdSec starts while the row stays as it was.
func TestReconcileCrowdSecOnStartup_RowSyncFailureKeepsSettingDecision(t *testing.T) {
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()
	db := setupCrowdsecTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	require.NoError(t, db.Create(&models.Setting{Key: models.CrowdSecEnabledSettingKey, Value: "true", Category: "security", Type: "bool"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName, CrowdSecMode: "disabled"}).Error)
	require.NoError(t, db.Exec("CREATE TRIGGER fail_row_sync BEFORE UPDATE ON security_configs BEGIN SELECT RAISE(ABORT, 'forced'); END").Error)

	exec := &smartMockCrowdsecExecutor{startPid: 12345}
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, &mockCommandExecutor{})

	assert.True(t, exec.startCalled, "the setting decides even when the row cannot be synced")
	row, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.Equal(t, "disabled", row.CrowdSecMode)
}
