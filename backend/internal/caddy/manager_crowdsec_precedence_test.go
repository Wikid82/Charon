package caddy

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// crowdSecAbsent marks a settings key that is not written at all in a precedence case.
const crowdSecAbsent = "<absent>"

// crowdSecPrecedenceCase is one combination of the CrowdSec settings the proxy, the status endpoint and
// boot reconciliation all read. The same case list exists in the services and handlers test packages.
type crowdSecPrecedenceCase struct {
	enabled     string // security.crowdsec.enabled
	mode        string // deprecated security.crowdsec.mode
	row         string // crowd_sec_mode of the default SecurityConfig row
	cerberusOff bool   // the global switch (SecurityConfig.Enabled) is off
	wantOn      bool   // CrowdSec decision of the setting/row precedence, before the global switch
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
	// One extra row with the global switch off: the proxy drops CrowdSec whatever the setting says.
	cases = append(cases, crowdSecPrecedenceCase{enabled: "true", row: "local", cerberusOff: true, wantOn: true})
	return cases
}

func openCrowdSecPrecedenceDB(t *testing.T, tc crowdSecPrecedenceCase) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "charon.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))

	for key, value := range map[string]string{
		"security.crowdsec.enabled": tc.enabled,
		"security.crowdsec.mode":    tc.mode,
	} {
		if value == crowdSecAbsent {
			continue
		}
		require.NoError(t, db.Create(&models.Setting{Key: key, Value: value, Category: "security", Type: "string"}).Error)
	}
	// Cerberus stays on unless the case says otherwise; with it off, computeEffectiveFlags forces CrowdSec off.
	require.NoError(t, db.Create(&models.SecurityConfig{
		Name:         models.DefaultSecurityConfigName,
		Enabled:      !tc.cerberusOff,
		CrowdSecMode: tc.row,
	}).Error)
	return db
}

func TestComputeEffectiveFlags_CrowdSecPrecedenceMatrix(t *testing.T) {
	for _, tc := range crowdSecPrecedenceCases() {
		name := "enabled=" + tc.enabled + "/mode=" + tc.mode + "/row=" + tc.row
		if tc.cerberusOff {
			name += "/cerberus=off"
		}
		t.Run(name, func(t *testing.T) {
			db := openCrowdSecPrecedenceDB(t, tc)
			manager := NewManager(nil, db, "", "", false, config.SecurityConfig{CerberusEnabled: true})

			_, _, _, _, crowdSecOn := manager.computeEffectiveFlags(context.Background())

			require.Equal(t, tc.wantOn && !tc.cerberusOff, crowdSecOn)
		})
	}
}
