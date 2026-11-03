package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// A fresh database has no default SecurityConfig row. The seed must end with the row agreeing with the
// security.crowdsec.enabled setting, through the same sync the existing-row path uses.
func TestReconcileCrowdSecOnStartup_FreshDatabaseSeedFollowsTheSetting(t *testing.T) {
	tests := []struct {
		name        string
		hasSetting  bool
		settingVal  string
		wantMode    string
		wantStarted bool
	}{
		{name: "setting true seeds a local row and starts", hasSetting: true, settingVal: "true", wantMode: "local", wantStarted: true},
		{name: "setting false seeds a disabled row and does not start", hasSetting: true, settingVal: "false", wantMode: "disabled"},
		{name: "absent setting seeds a disabled row and does not start", wantMode: "disabled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupCrowdsecTestDB(t)
			binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
			defer cleanup()

			if tt.hasSetting {
				require.NoError(t, db.AutoMigrate(&models.Setting{}))
				require.NoError(t, db.Create(&models.Setting{
					Key:      "security.crowdsec.enabled",
					Value:    tt.settingVal,
					Type:     "bool",
					Category: "security",
				}).Error)
			}

			exec := &smartMockCrowdsecExecutor{startPid: 12345}
			ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, &mockCommandExecutor{})

			var cfg models.SecurityConfig
			require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&cfg).Error)
			assert.Equal(t, tt.wantMode, cfg.CrowdSecMode)
			assert.False(t, cfg.Enabled, "the seeded global Cerberus flag matches a fresh install")
			assert.Equal(t, tt.wantStarted, exec.startCalled)
		})
	}
}
