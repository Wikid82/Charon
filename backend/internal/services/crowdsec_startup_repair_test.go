package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestReconcileCrowdSecOnStartup_BootOrder_StaleLocalRowWithSettingFalseDoesNotStart(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	require.NoError(t, db.Create(&models.Setting{Key: "security.crowdsec.enabled", Value: "false", Type: "bool", Category: "security"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-legacy", Name: "Default Security Config", CrowdSecMode: "local"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName, CrowdSecMode: "local"}).Error)

	exec := &smartMockCrowdsecExecutor{startPid: 12345}
	cmdExec := &mockCommandExecutor{}

	// Real boot order: reconciliation runs before the routes migrate, repair and seed.
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, cmdExec)
	require.NoError(t, models.RepairDefaultSecurityConfigName(db))
	_, err := models.SeedDefaultSecurityConfig(db)
	require.NoError(t, err)

	assert.False(t, exec.startCalled, "a stale local row must not start the engine when the setting says disabled")

	var rows []models.SecurityConfig
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, models.DefaultSecurityConfigName, rows[0].Name)
	assert.Equal(t, "disabled", rows[0].CrowdSecMode)
}

func TestReconcileCrowdSecOnStartup_BootOrder_LegacyRowWithSettingTrueStartsOnce(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	require.NoError(t, db.Create(&models.Setting{Key: "security.crowdsec.enabled", Value: "true", Type: "bool", Category: "security"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-legacy", Name: "Default Security Config", CrowdSecMode: "disabled"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName, CrowdSecMode: "disabled"}).Error)

	exec := &smartMockCrowdsecExecutor{startPid: 12345}
	cmdExec := &mockCommandExecutor{}

	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, cmdExec)
	require.NoError(t, models.RepairDefaultSecurityConfigName(db))
	_, err := models.SeedDefaultSecurityConfig(db)
	require.NoError(t, err)

	assert.True(t, exec.startCalled)
	cfg, err := models.FindDefaultSecurityConfig(db)
	require.NoError(t, err)
	assert.Equal(t, "local", cfg.CrowdSecMode)
}

func TestReconcileCrowdSecOnStartup_EmptyTableCreatesExactlyOneDefaultRow(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	ReconcileCrowdSecOnStartup(db, &mockCrowdsecExecutor{}, binPath, dataDir, &mockCommandExecutor{})

	var rows []models.SecurityConfig
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, models.DefaultSecurityConfigName, rows[0].Name)
	assert.NotEmpty(t, rows[0].UUID)
}
