package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// legacyCrowdSecDefaultRowName is the name of the singleton row before #1727; RepairDefaultSecurityConfigName renames it.
const legacyCrowdSecDefaultRowName = "Default Security Config"

// TestCrowdSecModeCarryOver_StartupPaths runs the startup order used by main.go (retire the old mode setting, then
// reconcile) over the two readers of the enabled key that never looked at the mode key: the repair of legacy row
// names and the seed of a missing default row. The decision must not change, and the stored row must agree with it.
func TestCrowdSecModeCarryOver_StartupPaths(t *testing.T) {
	tests := []struct {
		name        string
		legacyRow   bool   // a row under the legacy name exists
		defaultRow  bool   // a row under the current name exists
		modeValue   string // security.crowdsec.mode, "" means the row is not written
		wantStarted bool
		wantRowMode string // crowd_sec_mode of the default row after startup; "" means no default row is asserted
	}{
		{
			// Merge path of the repair: the default row keeps its columns and takes crowd_sec_mode from the enabled
			// key. Without the carry-over the row would stay "disabled" while the mode key started CrowdSec.
			name:        "legacy row and default row, mode local, no enabled key",
			legacyRow:   true,
			defaultRow:  true,
			modeValue:   "local",
			wantStarted: true,
			wantRowMode: "local",
		},
		{
			// Rename path of the repair: the legacy row becomes the default row unchanged, and the carried enabled
			// key still starts CrowdSec.
			name:        "legacy row only, mode local, no enabled key",
			legacyRow:   true,
			modeValue:   "local",
			wantStarted: true,
		},
		{
			// Seed path: the reconciliation creates the default row and sets it to local when the enabled key is true.
			name:        "empty database, mode local, no enabled key",
			modeValue:   "local",
			wantStarted: true,
			wantRowMode: "local",
		},
		{
			name:        "empty database, mode disabled, no enabled key",
			modeValue:   "disabled",
			wantStarted: false,
			wantRowMode: "disabled",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
			defer cleanup()

			db := setupCrowdsecTestDB(t)
			require.NoError(t, db.AutoMigrate(&models.Setting{}))
			if tt.legacyRow {
				require.NoError(t, db.Create(&models.SecurityConfig{Name: legacyCrowdSecDefaultRowName, UUID: "legacy-row", CrowdSecMode: "disabled"}).Error)
			}
			if tt.defaultRow {
				require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, UUID: "default-row", CrowdSecMode: "disabled"}).Error)
			}
			require.NoError(t, db.Create(&models.Setting{Key: models.CrowdSecDeprecatedModeSettingKey, Value: tt.modeValue, Category: "security", Type: "string"}).Error)

			retired, err := models.RetireCrowdSecModeSetting(db)
			require.NoError(t, err)
			require.True(t, retired)

			exec := &smartMockCrowdsecExecutor{startPid: 12345}
			ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, &mockCommandExecutor{})
			assert.Equal(t, tt.wantStarted, exec.startCalled, "CrowdSec start decision")

			if tt.wantRowMode != "" {
				row, findErr := models.FindDefaultSecurityConfig(db)
				require.NoError(t, findErr)
				assert.Equal(t, tt.wantRowMode, row.CrowdSecMode)
			}
		})
	}
}
