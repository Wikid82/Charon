package caddy

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// cerberusDecisionCase is one combination of the Cerberus switches the proxy reads, in precedence order:
// the static config, the default SecurityConfig row, the feature key, then the legacy key.
// crowdSecAbsent is the shared "not written" marker.
type cerberusDecisionCase struct {
	static  bool   // config.SecurityConfig.CerberusEnabled
	row     string // SecurityConfig.Enabled of the default row, or crowdSecAbsent when there is no row
	feature string // feature.cerberus.enabled, or crowdSecAbsent
	legacy  string // security.cerberus.enabled, or crowdSecAbsent
	wantOn  bool
}

// cerberusWantOn applies the documented precedence: a present default row overrides the static config,
// a present feature key overrides the row, and the legacy key is read only when the feature key is absent.
// Only the exact value "true" (case-insensitive) turns a stored switch on.
func cerberusWantOn(static bool, row, feature, legacy string) bool {
	on := static
	if row != crowdSecAbsent {
		on = row == "true"
	}
	switch {
	case feature != crowdSecAbsent:
		on = strings.EqualFold(feature, "true")
	case legacy != crowdSecAbsent:
		on = strings.EqualFold(legacy, "true")
	}
	return on
}

func cerberusDecisionCases() []cerberusDecisionCase {
	var cases []cerberusDecisionCase
	for _, static := range []bool{true, false} {
		for _, row := range []string{crowdSecAbsent, "true", "false"} {
			for _, feature := range []string{crowdSecAbsent, "", "true", "false", "yes"} {
				for _, legacy := range []string{crowdSecAbsent, "true", "false"} {
					cases = append(cases, cerberusDecisionCase{
						static:  static,
						row:     row,
						feature: feature,
						legacy:  legacy,
						wantOn:  cerberusWantOn(static, row, feature, legacy),
					})
				}
			}
		}
	}
	return cases
}

func openCerberusDecisionDB(t *testing.T, tc cerberusDecisionCase) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "charon.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.SecurityConfig{}))

	if tc.row != crowdSecAbsent {
		row := models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: tc.row == "true"}
		require.NoError(t, db.Create(&row).Error)
	}
	for key, value := range map[string]string{
		"feature.cerberus.enabled":  tc.feature,
		"security.cerberus.enabled": tc.legacy,
	} {
		if value == crowdSecAbsent {
			continue
		}
		require.NoError(t, db.Create(&models.Setting{Key: key, Value: value, Category: "feature", Type: "bool"}).Error)
	}
	return db
}

// TestComputeEffectiveFlags_CerberusKeyMatrix pins the proxy's Cerberus decision for every combination of
// the static config, the default row, the feature key and the legacy key.
func TestComputeEffectiveFlags_CerberusKeyMatrix(t *testing.T) {
	for _, tc := range cerberusDecisionCases() {
		name := "static=" + boolWord(tc.static) + "/row=" + tc.row + "/feature=" + tc.feature + "/legacy=" + tc.legacy
		t.Run(name, func(t *testing.T) {
			db := openCerberusDecisionDB(t, tc)
			manager := NewManager(nil, db, "", "", false, config.SecurityConfig{CerberusEnabled: tc.static})

			cerbOn, _, _, _, _ := manager.computeEffectiveFlags(context.Background())

			require.Equal(t, tc.wantOn, cerbOn)
		})
	}
}

func boolWord(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
