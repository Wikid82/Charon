package models_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// readSettingValue returns the stored value of a settings key and whether the key exists.
func readSettingValue(t *testing.T, db *gorm.DB, key string) (string, bool) {
	t.Helper()
	var setting models.Setting
	result := db.Where("key = ?", key).Limit(1).Find(&setting)
	require.NoError(t, result.Error)
	return setting.Value, result.RowsAffected > 0
}

// readDefaultRow returns the default row, failing the test when it is missing.
func readDefaultRow(t *testing.T, db *gorm.DB) models.SecurityConfig {
	t.Helper()
	row, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	return row
}

func TestIsSecurityModuleKey_ListsTheThreeSwitches(t *testing.T) {
	assert.True(t, models.IsSecurityModuleKey(models.SecurityWAFEnabledSettingKey))
	assert.True(t, models.IsSecurityModuleKey(models.SecurityRateLimitEnabledSettingKey))
	assert.True(t, models.IsSecurityModuleKey(models.SecurityACLEnabledSettingKey))
	assert.False(t, models.IsSecurityModuleKey(models.CerberusFeatureSettingKey))
	assert.False(t, models.IsSecurityModuleKey(""))
}

func TestParseModuleSettingValue_TrimsAndIgnoresCase(t *testing.T) {
	on, ok := models.ParseModuleSettingValue("  TRUE ")
	assert.True(t, ok)
	assert.True(t, on)

	on, ok = models.ParseModuleSettingValue("False")
	assert.True(t, ok)
	assert.False(t, on)

	_, ok = models.ParseModuleSettingValue("yes")
	assert.False(t, ok)
}

func TestSetModuleState_RejectsUnknownKey(t *testing.T) {
	db := newModuleTestDB(t)
	require.Error(t, models.SetModuleState(db, "security.something.else", true, "test"))
	require.Error(t, models.ClearModuleKey(db, "security.something.else", "test"))
}

func TestSetModuleState_OffWritesKeyFalseAndLeavesRow(t *testing.T) {
	db := newModuleTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: true, WAFMode: "block", RateLimitMode: "enabled", RateLimitEnable: true}).Error)

	require.NoError(t, models.SetModuleState(db, models.SecurityWAFEnabledSettingKey, false, "test"))
	require.NoError(t, models.SetModuleState(db, models.SecurityRateLimitEnabledSettingKey, false, "test"))

	value, found := readSettingValue(t, db, models.SecurityWAFEnabledSettingKey)
	assert.True(t, found)
	assert.Equal(t, "false", value)
	row := readDefaultRow(t, db)
	assert.Equal(t, "block", row.WAFMode, "a switch off never touches the row")
	assert.Equal(t, "enabled", row.RateLimitMode)
	assert.True(t, row.RateLimitEnable)
	assert.True(t, row.Enabled)
}

func TestSetModuleState_WAFOnMirrorsDisabledAndEmptyModesToBlock(t *testing.T) {
	cases := []struct {
		name string
		mode string
		want string
	}{
		{"disabled becomes block", "disabled", "block"},
		{"empty becomes block", "", "block"},
		{"monitor is kept", "monitor", "monitor"},
		{"block is kept", "block", "block"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := newModuleTestDB(t)
			require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: c.mode}).Error)

			require.NoError(t, models.SetModuleState(db, models.SecurityWAFEnabledSettingKey, true, "test"))

			value, _ := readSettingValue(t, db, models.SecurityWAFEnabledSettingKey)
			assert.Equal(t, "true", value)
			assert.Equal(t, c.want, readDefaultRow(t, db).WAFMode)
			assert.True(t, readDefaultRow(t, db).Enabled, "the framework is enabled")
		})
	}
}

func TestSetModuleState_RateLimitOnMirrorsRowAndStartsFramework(t *testing.T) {
	db := newModuleTestDB(t)

	require.NoError(t, models.SetModuleState(db, models.SecurityRateLimitEnabledSettingKey, true, "test"))

	row := readDefaultRow(t, db)
	assert.Equal(t, "enabled", row.RateLimitMode)
	assert.True(t, row.RateLimitEnable)
	assert.True(t, row.Enabled)
	feature, _ := readSettingValue(t, db, models.CerberusFeatureSettingKey)
	assert.Equal(t, "true", feature)
	legacy, _ := readSettingValue(t, db, models.CerberusLegacySettingKey)
	assert.Equal(t, "true", legacy)
}

func TestSetModuleStateOpt_WithoutFrameworkLeavesEnabledAlone(t *testing.T) {
	db := newModuleTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: false}).Error)

	require.NoError(t, models.SetModuleStateOpt(db, models.SecurityWAFEnabledSettingKey, true, "test", false))

	value, _ := readSettingValue(t, db, models.SecurityWAFEnabledSettingKey)
	assert.Equal(t, "true", value)
	assert.False(t, readDefaultRow(t, db).Enabled)
	_, found := readSettingValue(t, db, models.CerberusFeatureSettingKey)
	assert.False(t, found, "the framework switch is not written")
}

func TestSetModuleState_ErrorsWhenSettingsWriteFails(t *testing.T) {
	db := newModuleTestDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.Setting{}))

	assert.Error(t, models.SetModuleState(db, models.SecurityWAFEnabledSettingKey, true, "test"))
}

func TestSetModuleState_ErrorsWhenRowMirrorFails(t *testing.T) {
	db := newModuleTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName}).Error)
	failQueriesOn(db, "security_configs")

	assert.Error(t, models.SetModuleState(db, models.SecurityRateLimitEnabledSettingKey, true, "test"))
}

func TestClearModuleKey_RemovesTheKey(t *testing.T) {
	db := newModuleTestDB(t)
	setModuleSetting(t, db, models.SecurityWAFEnabledSettingKey, "true")

	require.NoError(t, models.ClearModuleKey(db, models.SecurityWAFEnabledSettingKey, "test"))

	_, found := readSettingValue(t, db, models.SecurityWAFEnabledSettingKey)
	assert.False(t, found)
}

func TestClearModuleKey_ErrorsWhenDeleteFails(t *testing.T) {
	db := newModuleTestDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.Setting{}))

	assert.Error(t, models.ClearModuleKey(db, models.SecurityWAFEnabledSettingKey, "test"))
}

func TestEnableSecurityFramework_SeedsMissingRowAndSetsSwitches(t *testing.T) {
	db := newModuleTestDB(t)

	require.NoError(t, models.EnableSecurityFramework(db))

	assert.True(t, readDefaultRow(t, db).Enabled)
	feature, _ := readSettingValue(t, db, models.CerberusFeatureSettingKey)
	assert.Equal(t, "true", feature)
}

func TestEnableSecurityFramework_KeepsExistingRowColumns(t *testing.T) {
	db := newModuleTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: "monitor", Enabled: false}).Error)

	require.NoError(t, models.EnableSecurityFramework(db))

	row := readDefaultRow(t, db)
	assert.True(t, row.Enabled)
	assert.Equal(t, "monitor", row.WAFMode)
}

func TestEnableSecurityFramework_ErrorsWhenTablesFail(t *testing.T) {
	t.Run("row lookup fails", func(t *testing.T) {
		db := newModuleTestDB(t)
		failQueriesOn(db, "security_configs")
		assert.Error(t, models.EnableSecurityFramework(db))
	})
	t.Run("settings table is missing", func(t *testing.T) {
		db := newModuleTestDB(t)
		require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName}).Error)
		require.NoError(t, db.Migrator().DropTable(&models.Setting{}))
		assert.Error(t, models.EnableSecurityFramework(db))
	})
}

func TestReconcileWAFDetectionMode_ChangesOnlyKeyOnDisabledOrEmptyRows(t *testing.T) {
	db := newModuleTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: "disabled"}).Error)
	setModuleSetting(t, db, models.SecurityWAFEnabledSettingKey, "true")

	changed, err := models.ReconcileWAFDetectionMode(db)
	require.NoError(t, err)
	assert.Equal(t, int64(1), changed)
	assert.Equal(t, "monitor", readDefaultRow(t, db).WAFMode)

	again, err := models.ReconcileWAFDetectionMode(db)
	require.NoError(t, err)
	assert.Zero(t, again)
}

func TestReconcileWAFDetectionMode_NoRowOrNoKeyChangesNothing(t *testing.T) {
	t.Run("no row", func(t *testing.T) {
		db := newModuleTestDB(t)
		setModuleSetting(t, db, models.SecurityWAFEnabledSettingKey, "true")
		changed, err := models.ReconcileWAFDetectionMode(db)
		require.NoError(t, err)
		assert.Zero(t, changed)
	})
	t.Run("key off", func(t *testing.T) {
		db := newModuleTestDB(t)
		require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: "disabled"}).Error)
		setModuleSetting(t, db, models.SecurityWAFEnabledSettingKey, "false")
		changed, err := models.ReconcileWAFDetectionMode(db)
		require.NoError(t, err)
		assert.Zero(t, changed)
		assert.Equal(t, "disabled", readDefaultRow(t, db).WAFMode)
	})
}

func TestReconcileWAFDetectionMode_ErrorsWhenKeyReadFails(t *testing.T) {
	db := newModuleTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: "disabled"}).Error)
	failQueriesOn(db, "settings")

	_, err := models.ReconcileWAFDetectionMode(db)
	assert.Error(t, err)
}

func TestReadModuleRow_NilWhenMissingAndErrorOnFailure(t *testing.T) {
	db := newModuleTestDB(t)
	row, err := models.ReadModuleRow(db, models.DefaultSecurityConfigName)
	require.NoError(t, err)
	assert.Nil(t, row)

	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: "block"}).Error)
	row, err = models.ReadModuleRow(db, models.DefaultSecurityConfigName)
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.Equal(t, "block", row.WAFMode)

	failQueriesOn(db, "security_configs")
	_, err = models.ReadModuleRow(db, models.DefaultSecurityConfigName)
	assert.Error(t, err)
}

func TestCaptureAndRestoreModuleState_RoundTrip(t *testing.T) {
	db := newModuleTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{
		Name:               models.DefaultSecurityConfigName,
		Enabled:            true,
		WAFMode:            "block",
		WAFRulesSource:     "owasp-crs",
		AdminWhitelist:     "192.0.2.0/24",
		RateLimitMode:      "enabled",
		RateLimitEnable:    true,
		RateLimitRequests:  50,
		RateLimitWindowSec: 60,
		RateLimitBurst:     20,
	}).Error)
	setModuleSetting(t, db, models.SecurityWAFEnabledSettingKey, "false")

	row := readDefaultRow(t, db)
	snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, &row, "security.extra.key")
	require.NoError(t, err)

	// Change everything the snapshot covers, and add a key that did not exist.
	require.NoError(t, models.SetModuleState(db, models.SecurityWAFEnabledSettingKey, true, "test"))
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).
		Updates(map[string]any{"waf_mode": "disabled", "admin_whitelist": "", "rate_limit_requests": 0, "enabled": false}).Error)
	setModuleSetting(t, db, "security.extra.key", "x")

	require.NoError(t, snap.RestoreSettings(db))
	require.NoError(t, snap.RestoreRow(db))

	value, _ := readSettingValue(t, db, models.SecurityWAFEnabledSettingKey)
	assert.Equal(t, "false", value)
	_, extraFound := readSettingValue(t, db, "security.extra.key")
	assert.False(t, extraFound, "a key that did not exist before is removed")
	restored := readDefaultRow(t, db)
	assert.Equal(t, "block", restored.WAFMode)
	assert.Equal(t, "192.0.2.0/24", restored.AdminWhitelist)
	assert.Equal(t, 50, restored.RateLimitRequests)
	assert.True(t, restored.Enabled)
}

func TestCaptureModuleState_AbsentRowRestoresByDeleting(t *testing.T) {
	db := newModuleTestDB(t)
	snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, nil)
	require.NoError(t, err)

	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName}).Error)
	require.NoError(t, snap.RestoreRow(db))

	_, err = models.FindDefaultSecurityConfig(db)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestCaptureModuleState_ErrorsWhenSettingsReadFails(t *testing.T) {
	db := newModuleTestDB(t)
	failQueriesOn(db, "settings")
	_, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, nil)
	assert.Error(t, err)
}

func TestModuleStateRestore_ErrorsWhenWritesFail(t *testing.T) {
	db := newModuleTestDB(t)
	setModuleSetting(t, db, models.SecurityWAFEnabledSettingKey, "true")
	snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, nil)
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable(&models.Setting{}))
	assert.Error(t, snap.RestoreSettings(db))

	db2 := newModuleTestDB(t)
	require.NoError(t, db2.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName}).Error)
	row := readDefaultRow(t, db2)
	snapRow, err := models.CaptureModuleState(db2, models.DefaultSecurityConfigName, &row)
	require.NoError(t, err)
	require.NoError(t, db2.Migrator().DropTable(&models.SecurityConfig{}))
	assert.Error(t, snapRow.RestoreRow(db2))
}

// failCreatesOn makes every create on table fail.
func failCreatesOn(db *gorm.DB, table string) {
	_ = db.Callback().Create().Before("gorm:create").Register("test:fail_create_"+table, func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			_ = tx.AddError(errors.New("forced create failure"))
		}
	})
}

// failUpdateColumnOn makes a map update on the default row table fail when the map sets column.
func failUpdateColumnOn(db *gorm.DB, column string) {
	table := "security_configs"
	_ = db.Callback().Update().Before("gorm:update").Register("test:fail_update_"+table+"_"+column, func(tx *gorm.DB) {
		if tx.Statement.Table != table {
			return
		}
		if values, ok := tx.Statement.Dest.(map[string]any); ok {
			if _, set := values[column]; set {
				_ = tx.AddError(errors.New("forced update failure"))
			}
		}
	})
}

func TestSetModuleState_MirrorWriteFailuresAreReturned(t *testing.T) {
	t.Run("waf mirror", func(t *testing.T) {
		db := newModuleTestDB(t)
		require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: "disabled"}).Error)
		failUpdateColumnOn(db, "waf_mode")
		assert.Error(t, models.SetModuleState(db, models.SecurityWAFEnabledSettingKey, true, "test"))
	})
	t.Run("rate limit mirror", func(t *testing.T) {
		db := newModuleTestDB(t)
		require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName}).Error)
		failUpdateColumnOn(db, "rate_limit_mode")
		assert.Error(t, models.SetModuleState(db, models.SecurityRateLimitEnabledSettingKey, true, "test"))
	})
}

func TestEnableSecurityFramework_WriteFailuresAreReturned(t *testing.T) {
	t.Run("seed of a missing row fails", func(t *testing.T) {
		db := newModuleTestDB(t)
		failCreatesOn(db, "security_configs")
		assert.Error(t, models.EnableSecurityFramework(db))
	})
	t.Run("enabling the row fails", func(t *testing.T) {
		db := newModuleTestDB(t)
		require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName}).Error)
		failUpdateColumnOn(db, "enabled")
		assert.Error(t, models.EnableSecurityFramework(db))
	})
	t.Run("the cerberus switch cannot be written", func(t *testing.T) {
		db := newModuleTestDB(t)
		require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName}).Error)
		failCreatesOn(db, "settings")
		assert.Error(t, models.EnableSecurityFramework(db))
	})
}

func TestReconcileWAFDetectionMode_WriteFailuresAreReturned(t *testing.T) {
	t.Run("row read fails", func(t *testing.T) {
		db := newModuleTestDB(t)
		require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: "disabled"}).Error)
		setModuleSetting(t, db, models.SecurityWAFEnabledSettingKey, "true")
		failQueriesOn(db, "security_configs")
		_, err := models.ReconcileWAFDetectionMode(db)
		assert.Error(t, err)
	})
	t.Run("mode update fails", func(t *testing.T) {
		db := newModuleTestDB(t)
		require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFMode: "disabled"}).Error)
		setModuleSetting(t, db, models.SecurityWAFEnabledSettingKey, "true")
		failUpdateColumnOn(db, "waf_mode")
		_, err := models.ReconcileWAFDetectionMode(db)
		assert.Error(t, err)
	})
}

func TestCaptureModuleState_ExtraKeyThatIsAlreadyCapturedIsCountedOnce(t *testing.T) {
	db := newModuleTestDB(t)
	setModuleSetting(t, db, models.SecurityWAFEnabledSettingKey, "true")
	snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, nil, models.SecurityWAFEnabledSettingKey)
	require.NoError(t, err)

	require.NoError(t, models.ClearModuleKey(db, models.SecurityWAFEnabledSettingKey, "test"))
	require.NoError(t, snap.RestoreSettings(db))
	value, found := readSettingValue(t, db, models.SecurityWAFEnabledSettingKey)
	assert.True(t, found)
	assert.Equal(t, "true", value)
}

func TestModuleStateRestore_DeleteFailuresAreReturned(t *testing.T) {
	t.Run("an absent key cannot be removed", func(t *testing.T) {
		db := newModuleTestDB(t)
		snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, nil)
		require.NoError(t, err)
		require.NoError(t, db.Migrator().DropTable(&models.Setting{}))
		assert.Error(t, snap.RestoreSettings(db))
	})
	t.Run("an absent row cannot be removed", func(t *testing.T) {
		db := newModuleTestDB(t)
		snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, nil)
		require.NoError(t, err)
		require.NoError(t, db.Migrator().DropTable(&models.SecurityConfig{}))
		assert.Error(t, snap.RestoreRow(db))
	})
}
