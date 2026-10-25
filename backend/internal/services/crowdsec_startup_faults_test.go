package services

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil/gormfault"
)

const startupFaultLegacyName = "Default Security Config"

// newStartupFaultDB opens the tables the reconciliation reads: the security config rows and the settings.
func newStartupFaultDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupCrowdsecTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	return db
}

// captureStartupLogs routes the logger into a buffer for the rest of the test.
func captureStartupLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	logger.Init(false, buf)
	t.Cleanup(func() { logger.Init(false, io.Discard) })
	return buf
}

// requireNotStarted asserts the reconciliation left the engine alone.
func requireNotStarted(t *testing.T, exec *mockCrowdsecExecutor) {
	t.Helper()
	assert.False(t, exec.startCalled, "a fault must not start the engine")
}

func TestReconcile_RepairFailureIsLoggedAndReconciliationContinues(t *testing.T) {
	db := newStartupFaultDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-legacy", Name: startupFaultLegacyName, CrowdSecMode: "local"}).Error)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName, CrowdSecMode: "disabled"}).Error)
	gormfault.Inject(t, db, gormfault.Query, "security_configs", gormfault.VarIs(startupFaultLegacyName))
	logs := captureStartupLogs(t)
	exec := &mockCrowdsecExecutor{}

	ReconcileCrowdSecOnStartup(db, exec, "crowdsec", t.TempDir(), &mockCommandExecutor{})

	assert.Contains(t, logs.String(), "CrowdSec reconciliation: failed to repair default SecurityConfig name, continuing")
	assert.Contains(t, logs.String(), "CrowdSec reconciliation skipped", "the default row is still read after a failed repair")
	requireNotStarted(t, exec)
}

func TestReconcile_DefaultRowReadFailureStopsReconciliation(t *testing.T) {
	db := newStartupFaultDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName, CrowdSecMode: "local"}).Error)
	gormfault.Inject(t, db, gormfault.Query, "security_configs", gormfault.VarIs(models.DefaultSecurityConfigName))
	logs := captureStartupLogs(t)
	exec := &mockCrowdsecExecutor{}

	ReconcileCrowdSecOnStartup(db, exec, "crowdsec", t.TempDir(), &mockCommandExecutor{})

	assert.Contains(t, logs.String(), "CrowdSec reconciliation: failed to read SecurityConfig")
	requireNotStarted(t, exec)
}

func TestReconcile_SettingsPreferenceUpdateFailureIsLoggedAndStopsReconciliation(t *testing.T) {
	db := newStartupFaultDB(t)
	require.NoError(t, db.Create(&models.Setting{Key: "security.crowdsec.enabled", Value: "true", Category: "security", Type: "bool"}).Error)
	gormfault.Inject(t, db, gormfault.Update, "security_configs", gormfault.UpdateSets("crowd_sec_mode"))
	logs := captureStartupLogs(t)
	exec := &mockCrowdsecExecutor{}

	ReconcileCrowdSecOnStartup(db, exec, "crowdsec", t.TempDir(), &mockCommandExecutor{})

	assert.Contains(t, logs.String(), "CrowdSec reconciliation: failed to set crowd_sec_mode from Settings preference")
	requireNotStarted(t, exec)
}

func TestReconcile_SettingResolverFailureFallsBackToTheRow(t *testing.T) {
	db := newStartupFaultDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName, CrowdSecMode: "disabled"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "security.crowdsec.enabled", Value: "true", Category: "security", Type: "bool"}).Error)
	gormfault.Inject(t, db, gormfault.Query, "settings", gormfault.VarIs("security.crowdsec.enabled"))
	logs := captureStartupLogs(t)
	exec := &mockCrowdsecExecutor{}

	ReconcileCrowdSecOnStartup(db, exec, "crowdsec", t.TempDir(), &mockCommandExecutor{})

	assert.Contains(t, logs.String(), "CrowdSec reconciliation: failed to read CrowdSec setting, using the default SecurityConfig row")
	assert.Contains(t, logs.String(), "CrowdSec reconciliation skipped", "the row says disabled, so the unreadable setting must not start the engine")
	requireNotStarted(t, exec)
}

func TestSecurityServiceGet_DatabaseErrorIsReturnedNotMapped(t *testing.T) {
	db := newStartupFaultDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "u-default", Name: models.DefaultSecurityConfigName}).Error)
	gormfault.Inject(t, db, gormfault.Query, "security_configs", nil)

	cfg, err := NewSecurityService(db).Get()

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.True(t, gormfault.IsInjected(err))
	assert.False(t, errors.Is(err, ErrSecurityConfigNotFound), "a database failure is not a missing row")
}
