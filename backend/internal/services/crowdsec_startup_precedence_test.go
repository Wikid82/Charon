package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// crowdSecAbsent marks a settings key that is not written at all in a precedence case.
const crowdSecAbsent = "<absent>"

// crowdSecPrecedenceCase is one combination of the CrowdSec settings. The same case list exists in the caddy
// and handlers test packages, so every reader is checked against one precedence rule.
type crowdSecPrecedenceCase struct {
	enabled     string // security.crowdsec.enabled
	mode        string // deprecated security.crowdsec.mode
	row         string // crowd_sec_mode of the default SecurityConfig row
	cerberusOff bool   // the global switch (SecurityConfig.Enabled) is off; the engine decision ignores it
	wantOn      bool   // CrowdSec decision of the setting/row precedence
}

func crowdSecWantOn(enabled, mode, row string) bool {
	if enabled != crowdSecAbsent && enabled != "" {
		return enabled == "true"
	}
	if mode != crowdSecAbsent && mode != "" {
		return mode == "local"
	}
	return row == "local"
}

func crowdSecPrecedenceCases() []crowdSecPrecedenceCase {
	var cases []crowdSecPrecedenceCase
	for _, enabled := range []string{crowdSecAbsent, "", "true", "false"} {
		for _, mode := range []string{crowdSecAbsent, "", "local", "disabled", "remote"} {
			for _, row := range []string{"local", "disabled"} {
				cases = append(cases, crowdSecPrecedenceCase{
					enabled: enabled,
					mode:    mode,
					row:     row,
					wantOn:  crowdSecWantOn(enabled, mode, row),
				})
			}
		}
	}
	// One extra row with the global switch off: the engine decision does not read the switch today.
	cases = append(cases, crowdSecPrecedenceCase{enabled: "true", row: "local", cerberusOff: true, wantOn: true})
	return cases
}

func TestReconcileCrowdSecOnStartup_CrowdSecPrecedenceMatrix(t *testing.T) {
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	for _, tc := range crowdSecPrecedenceCases() {
		name := "enabled=" + tc.enabled + "/mode=" + tc.mode + "/row=" + tc.row
		if tc.cerberusOff {
			name += "/cerberus=off"
		}
		t.Run(name, func(t *testing.T) {
			db := setupCrowdsecTestDB(t)
			require.NoError(t, db.AutoMigrate(&models.Setting{}))
			for key, value := range map[string]string{
				"security.crowdsec.enabled": tc.enabled,
				"security.crowdsec.mode":    tc.mode,
			} {
				if value == crowdSecAbsent {
					continue
				}
				require.NoError(t, db.Create(&models.Setting{Key: key, Value: value, Category: "security", Type: "string"}).Error)
			}
			require.NoError(t, db.Create(&models.SecurityConfig{
				Name:         models.DefaultSecurityConfigName,
				Enabled:      !tc.cerberusOff,
				CrowdSecMode: tc.row,
			}).Error)

			exec := &smartMockCrowdsecExecutor{startPid: 12345}
			ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, &mockCommandExecutor{})

			require.Equal(t, tc.wantOn, exec.startCalled)
		})
	}
}
