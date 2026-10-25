package models_test

import (
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

const testLegacySecurityConfigName = "Default Security Config"

func newRepairTestDB(t *testing.T, withSettings bool) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))
	if withSettings {
		require.NoError(t, db.AutoMigrate(&models.Setting{}))
	}
	return db
}

func securityConfigNames(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var rows []models.SecurityConfig
	require.NoError(t, db.Order("id ASC").Find(&rows).Error)
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		names = append(names, r.Name)
	}
	return names
}

func TestSecurityConfigBeforeCreate_FillsEmptyUUID(t *testing.T) {
	db := newRepairTestDB(t, false)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: "a"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: "b"}).Error)

	var rows []models.SecurityConfig
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 2)
	assert.NotEmpty(t, rows[0].UUID)
	assert.NotEqual(t, rows[0].UUID, rows[1].UUID)
}

func TestSecurityConfigBeforeCreate_KeepsGivenUUID(t *testing.T) {
	db := newRepairTestDB(t, false)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "given-uuid", Name: "a"}).Error)

	var row models.SecurityConfig
	require.NoError(t, db.First(&row).Error)
	assert.Equal(t, "given-uuid", row.UUID)
}

func TestRepairDefaultSecurityConfigName_RenamesLegacyRowWhenNoDefault(t *testing.T) {
	db := newRepairTestDB(t, false)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "default", Name: testLegacySecurityConfigName, CrowdSecMode: "local", Enabled: true}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-second", Name: testLegacySecurityConfigName, CrowdSecMode: "disabled"}).Error)

	require.NoError(t, models.RepairDefaultSecurityConfigName(db))

	assert.Equal(t, []string{models.DefaultSecurityConfigName}, securityConfigNames(t, db))
	cfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.NotEqual(t, "default", cfg.UUID, "literal UUID 'default' must be replaced")
	assert.NotEmpty(t, cfg.UUID)
	assert.Equal(t, "local", cfg.CrowdSecMode)
	assert.True(t, cfg.Enabled, "Enabled is kept from the renamed row")
}

func TestRepairDefaultSecurityConfigName_KeepsOtherUUIDWhenRenaming(t *testing.T) {
	db := newRepairTestDB(t, false)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-keep", Name: testLegacySecurityConfigName}).Error)

	require.NoError(t, models.RepairDefaultSecurityConfigName(db))

	cfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.Equal(t, "u-keep", cfg.UUID)
}

func TestRepairDefaultSecurityConfigName_BothRowsSettingTrueSetsLocal(t *testing.T) {
	db := newRepairTestDB(t, true)
	require.NoError(t, db.Create(&models.Setting{Key: "security.crowdsec.enabled", Value: "TRUE"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-legacy", Name: testLegacySecurityConfigName, CrowdSecMode: "disabled", WAFMode: "block"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName, CrowdSecMode: "disabled", WAFMode: "monitor", Enabled: true}).Error)

	require.NoError(t, models.RepairDefaultSecurityConfigName(db))

	assert.Equal(t, []string{models.DefaultSecurityConfigName}, securityConfigNames(t, db))
	cfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.Equal(t, "u-default", cfg.UUID)
	assert.Equal(t, "local", cfg.CrowdSecMode)
	assert.Equal(t, "monitor", cfg.WAFMode, "only crowd_sec_mode is written")
	assert.True(t, cfg.Enabled)
}

func TestRepairDefaultSecurityConfigName_BothRowsSettingFalseSetsDisabled(t *testing.T) {
	db := newRepairTestDB(t, true)
	require.NoError(t, db.Create(&models.Setting{Key: "security.crowdsec.enabled", Value: "false"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-legacy", Name: testLegacySecurityConfigName, CrowdSecMode: "local"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName, CrowdSecMode: "local"}).Error)

	require.NoError(t, models.RepairDefaultSecurityConfigName(db))

	cfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.Equal(t, "disabled", cfg.CrowdSecMode)
}

func TestRepairDefaultSecurityConfigName_BothRowsEmptySettingUsesLegacyMode(t *testing.T) {
	db := newRepairTestDB(t, true)
	require.NoError(t, db.Create(&models.Setting{Key: "security.crowdsec.enabled", Value: ""}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-legacy", Name: testLegacySecurityConfigName, CrowdSecMode: "local"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName, CrowdSecMode: "disabled"}).Error)

	require.NoError(t, models.RepairDefaultSecurityConfigName(db))

	cfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.Equal(t, "local", cfg.CrowdSecMode)
}

func TestRepairDefaultSecurityConfigName_BothRowsNoSettingsTableUsesLegacyMode(t *testing.T) {
	db := newRepairTestDB(t, false)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-legacy", Name: testLegacySecurityConfigName, CrowdSecMode: "local"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName, CrowdSecMode: "disabled"}).Error)

	require.NoError(t, models.RepairDefaultSecurityConfigName(db))

	cfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.Equal(t, "local", cfg.CrowdSecMode)
	assert.Equal(t, []string{models.DefaultSecurityConfigName}, securityConfigNames(t, db))
}

func TestRepairDefaultSecurityConfigName_DeletesEveryLegacyRow(t *testing.T) {
	db := newRepairTestDB(t, false)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-1", Name: testLegacySecurityConfigName}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-2", Name: testLegacySecurityConfigName}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-3", Name: testLegacySecurityConfigName}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName}).Error)

	require.NoError(t, models.RepairDefaultSecurityConfigName(db))

	assert.Equal(t, []string{models.DefaultSecurityConfigName}, securityConfigNames(t, db))
}

func TestRepairDefaultSecurityConfigName_NoLegacyRowIsNoOp(t *testing.T) {
	db := newRepairTestDB(t, true)
	require.NoError(t, db.Create(&models.Setting{Key: "security.crowdsec.enabled", Value: "true"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName, CrowdSecMode: "disabled"}).Error)

	require.NoError(t, models.RepairDefaultSecurityConfigName(db))

	cfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.Equal(t, "u-default", cfg.UUID)
	assert.Equal(t, "disabled", cfg.CrowdSecMode, "setting must not be applied without a legacy row")
}

func TestRepairDefaultSecurityConfigName_EmptyTableIsNoOp(t *testing.T) {
	db := newRepairTestDB(t, false)

	require.NoError(t, models.RepairDefaultSecurityConfigName(db))

	var count int64
	require.NoError(t, db.Model(&models.SecurityConfig{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestRepairDefaultSecurityConfigName_IsIdempotent(t *testing.T) {
	db := newRepairTestDB(t, true)
	require.NoError(t, db.Create(&models.Setting{Key: "security.crowdsec.enabled", Value: "true"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-legacy", Name: testLegacySecurityConfigName}).Error)

	require.NoError(t, models.RepairDefaultSecurityConfigName(db))
	first, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)

	require.NoError(t, models.RepairDefaultSecurityConfigName(db))
	second, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)

	assert.Equal(t, first.UUID, second.UUID)
	assert.Equal(t, first.CrowdSecMode, second.CrowdSecMode)
	assert.Equal(t, []string{models.DefaultSecurityConfigName}, securityConfigNames(t, db))
}

func TestRepairDefaultSecurityConfigName_RollsBackOnFailure(t *testing.T) {
	db := newRepairTestDB(t, false)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-1", Name: testLegacySecurityConfigName}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-2", Name: testLegacySecurityConfigName}).Error)

	// Fail the delete step after the rename has already been written inside the transaction.
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("test:fail_delete", func(tx *gorm.DB) {
		_ = tx.AddError(errors.New("injected delete failure"))
	}))

	err := models.RepairDefaultSecurityConfigName(db)
	require.Error(t, err)

	assert.Equal(t, []string{testLegacySecurityConfigName, testLegacySecurityConfigName}, securityConfigNames(t, db),
		"a failed repair must leave every row as it was")
}
