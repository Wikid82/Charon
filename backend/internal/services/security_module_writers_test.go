package services

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// storedSettingValue returns a settings value and whether the key exists.
func storedSettingValue(t *testing.T, db *gorm.DB, key string) (string, bool) {
	t.Helper()
	var setting models.Setting
	result := db.Where("key = ?", key).Limit(1).Find(&setting)
	require.NoError(t, result.Error)
	return setting.Value, result.RowsAffected > 0
}

// seedDefaultRow stores the default row with the given columns.
func seedDefaultRow(t *testing.T, db *gorm.DB, row models.SecurityConfig) {
	t.Helper()
	row.Name = models.DefaultSecurityConfigName
	require.NoError(t, db.Create(&row).Error)
}

// applyAs runs ApplyUpdate for the default row with a JSON body and fails the test on error.
func applyAs(t *testing.T, svc *SecurityService, body string) {
	t.Helper()
	_, _, err := svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName, decodeUpdateTestBody(t, body))
	require.NoError(t, err)
}

func TestApplyUpdate_WAFKeyFollowsModeWhenItDiffersFromStoredRow(t *testing.T) {
	cases := []struct {
		name      string
		stored    string
		body      string
		wantKey   string
		wantFound bool
		wantRow   string
	}{
		{"disabled to block writes true", "disabled", `{"waf_mode":"block"}`, "true", true, "block"},
		{"disabled to monitor writes true and keeps monitor", "disabled", `{"waf_mode":"monitor"}`, "true", true, "monitor"},
		{"block to disabled writes false", "block", `{"waf_mode":"disabled"}`, "false", true, "disabled"},
		{"block to empty deletes the key", "block", `{"waf_mode":""}`, "", false, ""},
		{"same mode writes nothing", "block", `{"waf_mode":"block"}`, "", false, "block"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := setupSecurityTestDB(t)
			svc := newTestSecurityService(t, db)
			seedDefaultRow(t, db, models.SecurityConfig{Enabled: true, WAFMode: c.stored})

			applyAs(t, svc, c.body)

			value, found := storedSettingValue(t, db, models.SecurityWAFEnabledSettingKey)
			assert.Equal(t, c.wantFound, found)
			if c.wantFound {
				assert.Equal(t, c.wantKey, value)
			}
			var row models.SecurityConfig
			require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&row).Error)
			assert.Equal(t, c.wantRow, row.WAFMode)
		})
	}
}

func TestApplyUpdate_RateLimitKeyFollowsEnableOrMode(t *testing.T) {
	cases := []struct {
		name      string
		stored    models.SecurityConfig
		body      string
		wantKey   string
		wantFound bool
	}{
		{"enable true over false writes true", models.SecurityConfig{RateLimitMode: "disabled"}, `{"rate_limit_enable":true}`, "true", true},
		{"enable false over true writes false", models.SecurityConfig{RateLimitMode: "enabled", RateLimitEnable: true}, `{"rate_limit_enable":false}`, "false", true},
		{"enable equal to stored writes nothing", models.SecurityConfig{RateLimitMode: "enabled", RateLimitEnable: true}, `{"rate_limit_enable":true}`, "", false},
		{"mode enabled over disabled writes true", models.SecurityConfig{RateLimitMode: "disabled"}, `{"rate_limit_mode":"enabled"}`, "true", true},
		{"mode other value writes false", models.SecurityConfig{RateLimitMode: "enabled"}, `{"rate_limit_mode":"disabled"}`, "false", true},
		{"mode empty deletes the key", models.SecurityConfig{RateLimitMode: "enabled"}, `{"rate_limit_mode":""}`, "", false},
		{"mode equal to stored writes nothing", models.SecurityConfig{RateLimitMode: "enabled"}, `{"rate_limit_mode":"enabled"}`, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := setupSecurityTestDB(t)
			svc := newTestSecurityService(t, db)
			seedDefaultRow(t, db, c.stored)

			applyAs(t, svc, c.body)

			value, found := storedSettingValue(t, db, models.SecurityRateLimitEnabledSettingKey)
			assert.Equal(t, c.wantFound, found)
			if c.wantFound {
				assert.Equal(t, c.wantKey, value)
			}
		})
	}
}

func TestApplyUpdate_KeyOnWithEnabledFalseLeavesFrameworkOff(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	seedDefaultRow(t, db, models.SecurityConfig{Enabled: false, WAFMode: "disabled"})

	applyAs(t, svc, `{"waf_mode":"block","enabled":false}`)

	value, _ := storedSettingValue(t, db, models.SecurityWAFEnabledSettingKey)
	assert.Equal(t, "true", value, "the key is written")
	_, cerberusFound := storedSettingValue(t, db, models.CerberusFeatureSettingKey)
	assert.False(t, cerberusFound, "the framework is not switched on by this request")
	var row models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&row).Error)
	assert.False(t, row.Enabled)
}

func TestApplyUpdate_KeyOnStartsFrameworkWhenEnabledIsAbsent(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	seedDefaultRow(t, db, models.SecurityConfig{Enabled: false, WAFMode: "disabled"})

	applyAs(t, svc, `{"waf_mode":"monitor"}`)

	var row models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&row).Error)
	assert.True(t, row.Enabled)
	feature, _ := storedSettingValue(t, db, models.CerberusFeatureSettingKey)
	assert.Equal(t, "true", feature)
}

func TestApplyUpdate_NonDefaultNameWritesRowOnly(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: "other", WAFMode: "disabled"}).Error)

	_, _, err := svc.ApplyUpdate("test-actor", "other", decodeUpdateTestBody(t, `{"waf_mode":"block"}`))
	require.NoError(t, err)

	_, found := storedSettingValue(t, db, models.SecurityWAFEnabledSettingKey)
	assert.False(t, found, "only the default row drives the module keys")
	var row models.SecurityConfig
	require.NoError(t, db.Where("name = ?", "other").First(&row).Error)
	assert.Equal(t, "block", row.WAFMode)
}

func TestApplyUpdate_RefusesInvalidModeAndUnknownRulesSource(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	seedDefaultRow(t, db, models.SecurityConfig{Enabled: true, WAFMode: "block", WAFRulesSource: ""})

	_, _, err := svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName, decodeUpdateTestBody(t, `{"waf_mode":"nope"}`))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidWAFMode)
	var invalid *InvalidConfigError
	assert.True(t, errors.As(err, &invalid), "a refused value is a validation error")

	for _, source := range []string{"no-such-set", "ftp://rules.example.com/x", "http://", "http://[::1"} {
		_, _, err = svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName, SecurityConfigUpdate{WAFRulesSource: &source})
		assert.ErrorIs(t, err, ErrWAFRulesSourceUnknown, "source %q", source)
	}

	var row models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&row).Error)
	assert.Equal(t, "block", row.WAFMode, "a refused save writes nothing")
	assert.Empty(t, row.WAFRulesSource)
}

func TestApplyUpdate_AcceptsRulesSourceNameURLAndEmpty(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	seedDefaultRow(t, db, models.SecurityConfig{Enabled: true})
	require.NoError(t, db.Create(&models.SecurityRuleSet{UUID: "rs-name", Name: "owasp-crs", Content: "SecRule ARGS x id:1,deny"}).Error)

	for _, source := range []string{"owasp-crs", "https://rules.example.com/crs.zip", ""} {
		got, _, err := svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName, SecurityConfigUpdate{WAFRulesSource: &source})
		require.NoError(t, err, "source %q", source)
		assert.Equal(t, source, got.WAFRulesSource)
	}
}

func TestApplyUpdate_ReturnsPreImageThatRestoresState(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	seedDefaultRow(t, db, models.SecurityConfig{Enabled: false, WAFMode: "disabled", AdminWhitelist: "192.0.2.0/24"})
	require.NoError(t, db.Create(&models.Setting{Key: models.SecurityWAFEnabledSettingKey, Value: "false", Category: "security", Type: "bool"}).Error)

	_, snapshot, err := svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName, decodeUpdateTestBody(t, `{"enabled":true,"waf_mode":"block","admin_whitelist":"203.0.113.0/24"}`))
	require.NoError(t, err)

	require.NoError(t, snapshot.RestoreSettings(db))
	require.NoError(t, snapshot.RestoreRow(db))

	value, _ := storedSettingValue(t, db, models.SecurityWAFEnabledSettingKey)
	assert.Equal(t, "false", value)
	_, cerberusFound := storedSettingValue(t, db, models.CerberusFeatureSettingKey)
	assert.False(t, cerberusFound)
	var row models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&row).Error)
	assert.False(t, row.Enabled)
	assert.Equal(t, "disabled", row.WAFMode)
	assert.Equal(t, "192.0.2.0/24", row.AdminWhitelist)
}

func TestApplyUpdate_StorageFailureIsNotAValidationError(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	seedDefaultRow(t, db, models.SecurityConfig{Enabled: true})
	require.NoError(t, db.Migrator().DropTable(&models.Setting{}))

	_, _, err := svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName, decodeUpdateTestBody(t, `{"waf_mode":"block"}`))
	require.Error(t, err)
	var invalid *InvalidConfigError
	assert.False(t, errors.As(err, &invalid), "a storage failure is reported as such, not as a refused value")
}

func TestApplyUpdate_RulesSourceLookupFailureIsReturned(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	seedDefaultRow(t, db, models.SecurityConfig{Enabled: true})
	require.NoError(t, db.Migrator().DropTable(&models.SecurityRuleSet{}))

	source := "owasp-crs"
	_, _, err := svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName, SecurityConfigUpdate{WAFRulesSource: &source})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "look up rule set")
}

func TestInvalidConfigError_UnwrapsAndKeepsMessage(t *testing.T) {
	err := &InvalidConfigError{Err: ErrInvalidWAFMode}
	assert.Equal(t, ErrInvalidWAFMode.Error(), err.Error())
	assert.ErrorIs(t, err, ErrInvalidWAFMode)
}

func TestSecurityService_DeleteRuleSetByUUID(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	rs := models.SecurityRuleSet{UUID: "rs-delete", Name: "extra", Content: "SecRule ARGS x id:3,deny"}
	require.NoError(t, db.Create(&rs).Error)

	assert.ErrorIs(t, svc.DeleteRuleSetByUUID("missing-uuid"), gorm.ErrRecordNotFound)
	require.NoError(t, svc.DeleteRuleSetByUUID("rs-delete"))
	var count int64
	require.NoError(t, db.Model(&models.SecurityRuleSet{}).Where("uuid = ?", "rs-delete").Count(&count).Error)
	assert.Zero(t, count)
}

func TestSecurityService_DeleteRuleSetRefusesTheRulesSource(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	seedDefaultRow(t, db, models.SecurityConfig{Enabled: true, WAFRulesSource: "owasp-crs"})
	rs := models.SecurityRuleSet{UUID: "rs-in-use", Name: "owasp-crs", Content: "SecRule ARGS x id:4,deny"}
	require.NoError(t, db.Create(&rs).Error)

	assert.ErrorIs(t, svc.DeleteRuleSet(rs.ID), ErrRuleSetInUse)
	assert.ErrorIs(t, svc.DeleteRuleSetByUUID("rs-in-use"), ErrRuleSetInUse)

	var count int64
	require.NoError(t, db.Model(&models.SecurityRuleSet{}).Where("uuid = ?", "rs-in-use").Count(&count).Error)
	assert.Equal(t, int64(1), count, "the ruleset in use is kept")
}

func TestSecurityService_DeleteRuleSetByIDRefusesTheRulesSource(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	rs := models.SecurityRuleSet{UUID: "rs-id", Name: "by-id", Content: "SecRule ARGS x id:5,deny"}
	require.NoError(t, db.Create(&rs).Error)
	require.NoError(t, svc.DeleteRuleSet(rs.ID))

	err := svc.DeleteRuleSet(rs.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "a deleted ruleset is not found again")
}

func TestApplyUpdate_ModuleKeyWriteFailureIsReturned(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	seedDefaultRow(t, db, models.SecurityConfig{Enabled: true, WAFMode: "disabled"})
	_ = db.Callback().Create().Before("gorm:create").Register("test:fail_setting_create", func(tx *gorm.DB) {
		if tx.Statement.Table == "settings" {
			_ = tx.AddError(errors.New("forced create failure"))
		}
	})

	_, _, err := svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName, decodeUpdateTestBody(t, `{"waf_mode":"block"}`))
	require.Error(t, err)
	var invalid *InvalidConfigError
	assert.False(t, errors.As(err, &invalid), "a failed key write is a storage failure")
	assert.Contains(t, err.Error(), "apply security module keys")

	// The row write is in the same transaction as the key write, so the failed key write takes the row change with it.
	var row models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&row).Error)
	assert.Equal(t, "disabled", row.WAFMode)
}

func TestSecurityService_DeleteRuleSetReturnsUseCheckFailure(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	rs := models.SecurityRuleSet{UUID: "rs-check", Name: "check", Content: "SecRule ARGS x id:6,deny"}
	require.NoError(t, db.Create(&rs).Error)
	_ = db.Callback().Query().Before("gorm:query").Register("test:fail_config_count", func(tx *gorm.DB) {
		if tx.Statement.Table == "security_configs" {
			_ = tx.AddError(errors.New("forced count failure"))
		}
	})

	err := svc.DeleteRuleSet(rs.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "check rule set use")
}
