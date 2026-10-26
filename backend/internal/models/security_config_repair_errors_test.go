package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil/gormfault"
)

// createLegacyAndDefaultRows stores one legacy row (mode local) and one default row (mode disabled).
func createLegacyAndDefaultRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-legacy", Name: testLegacySecurityConfigName, CrowdSecMode: "local"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName, CrowdSecMode: "disabled"}).Error)
}

// requireRepairFailure asserts the repair returned the injected error and left every row as it was.
func requireRepairFailure(t *testing.T, db *gorm.DB, err error) {
	t.Helper()
	require.Error(t, err)
	assert.True(t, gormfault.IsInjected(err), "the error must be the injected failure, got: %v", err)
	assert.Equal(t, []string{testLegacySecurityConfigName, models.DefaultSecurityConfigName}, securityConfigNames(t, db),
		"a failed repair must leave every row as it was")
}

func TestRepairDefaultSecurityConfigName_LegacyLookupFailureIsReturned(t *testing.T) {
	db := newRepairTestDB(t, false)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-legacy", Name: testLegacySecurityConfigName}).Error)
	gormfault.Inject(t, db, gormfault.Query, "security_configs", nil)

	err := models.RepairDefaultSecurityConfigName(db)

	require.Error(t, err)
	assert.True(t, gormfault.IsInjected(err))
	assert.Contains(t, err.Error(), "find legacy default security config rows")
}

func TestRepairDefaultSecurityConfigName_DefaultCountFailureIsReturned(t *testing.T) {
	db := newRepairTestDB(t, false)
	createLegacyAndDefaultRows(t, db)
	gormfault.Inject(t, db, gormfault.Query, "security_configs", gormfault.VarIs(models.DefaultSecurityConfigName))

	requireRepairFailure(t, db, models.RepairDefaultSecurityConfigName(db))
}

func TestRepairDefaultSecurityConfigName_RenameFailureRollsBack(t *testing.T) {
	db := newRepairTestDB(t, false)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-legacy", Name: testLegacySecurityConfigName}).Error)
	gormfault.Inject(t, db, gormfault.Update, "security_configs", gormfault.UpdateSets("name"))

	err := models.RepairDefaultSecurityConfigName(db)

	require.Error(t, err)
	assert.True(t, gormfault.IsInjected(err))
	assert.Equal(t, []string{testLegacySecurityConfigName}, securityConfigNames(t, db))
}

func TestRepairDefaultSecurityConfigName_LoadDefaultFailureRollsBack(t *testing.T) {
	db := newRepairTestDB(t, false)
	createLegacyAndDefaultRows(t, db)
	// The second default-named query is the load of the row to keep; the first is the existence count.
	gormfault.InjectNth(t, db, gormfault.Query, "security_configs", 2, gormfault.VarIs(models.DefaultSecurityConfigName))

	err := models.RepairDefaultSecurityConfigName(db)

	require.Error(t, err)
	assert.True(t, gormfault.IsInjected(err))
	assert.Contains(t, err.Error(), "load default security config row")
	assert.Equal(t, []string{testLegacySecurityConfigName, models.DefaultSecurityConfigName}, securityConfigNames(t, db))
}

func TestRepairDefaultSecurityConfigName_SettingReadFailureRollsBack(t *testing.T) {
	db := newRepairTestDB(t, true)
	createLegacyAndDefaultRows(t, db)
	require.NoError(t, db.Create(&models.Setting{Key: "security.crowdsec.enabled", Value: "true"}).Error)
	gormfault.Inject(t, db, gormfault.Query, "settings", nil)

	err := models.RepairDefaultSecurityConfigName(db)

	require.Error(t, err)
	assert.True(t, gormfault.IsInjected(err))
	assert.Contains(t, err.Error(), "read security.crowdsec.enabled setting")
	assert.Equal(t, []string{testLegacySecurityConfigName, models.DefaultSecurityConfigName}, securityConfigNames(t, db))
}

func TestRepairDefaultSecurityConfigName_ModeSyncFailureRollsBack(t *testing.T) {
	db := newRepairTestDB(t, false)
	createLegacyAndDefaultRows(t, db)
	gormfault.Inject(t, db, gormfault.Update, "security_configs", gormfault.UpdateSets("crowd_sec_mode"))

	requireRepairFailure(t, db, models.RepairDefaultSecurityConfigName(db))
}

func TestRepairDefaultSecurityConfigName_LegacyDeleteFailureRollsBack(t *testing.T) {
	db := newRepairTestDB(t, false)
	createLegacyAndDefaultRows(t, db)
	// The mode sync has already been written inside the transaction when the delete fails.
	gormfault.Inject(t, db, gormfault.Delete, "security_configs", nil)

	requireRepairFailure(t, db, models.RepairDefaultSecurityConfigName(db))

	var row models.SecurityConfig
	require.NoError(t, db.Where("name = ?", models.DefaultSecurityConfigName).First(&row).Error)
	assert.Equal(t, "disabled", row.CrowdSecMode, "the mode sync must roll back with the failed delete")
}
