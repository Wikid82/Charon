package services

import (
	"context"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// mockCrowdsecExecutor is a test mock for CrowdsecProcessManager interface
type mockCrowdsecExecutor struct {
	startCalled  bool
	startErr     error
	startPid     int
	statusCalled bool
	statusErr    error
	running      bool
	pid          int
}

func (m *mockCrowdsecExecutor) Start(ctx context.Context, binPath, configDir string) (int, error) {
	m.startCalled = true
	return m.startPid, m.startErr
}

func (m *mockCrowdsecExecutor) Stop(ctx context.Context, configDir string) error {
	return nil
}

func (m *mockCrowdsecExecutor) Status(ctx context.Context, configDir string) (bool, int, error) {
	m.statusCalled = true
	return m.running, m.pid, m.statusErr
}

func setupCrowdsecTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.SecurityConfig{})
	require.NoError(t, err)

	return db
}

func TestReconcileCrowdSecOnStartup_NilDB(t *testing.T) {
	exec := &mockCrowdsecExecutor{}

	// Should not panic with nil db
	ReconcileCrowdSecOnStartup(nil, exec, "crowdsec", "/tmp/crowdsec")

	assert.False(t, exec.startCalled)
	assert.False(t, exec.statusCalled)
}

func TestReconcileCrowdSecOnStartup_NilExecutor(t *testing.T) {
	db := setupCrowdsecTestDB(t)

	// Should not panic with nil executor
	ReconcileCrowdSecOnStartup(db, nil, "crowdsec", "/tmp/crowdsec")
}

func TestReconcileCrowdSecOnStartup_NoSecurityConfig(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	exec := &mockCrowdsecExecutor{}

	// No SecurityConfig record - should skip
	ReconcileCrowdSecOnStartup(db, exec, "crowdsec", "/tmp/crowdsec")

	assert.False(t, exec.startCalled)
	assert.False(t, exec.statusCalled)
}

func TestReconcileCrowdSecOnStartup_ModeDisabled(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	exec := &mockCrowdsecExecutor{}

	// Create SecurityConfig with mode=disabled
	cfg := models.SecurityConfig{
		CrowdSecMode: "disabled",
	}
	require.NoError(t, db.Create(&cfg).Error)

	ReconcileCrowdSecOnStartup(db, exec, "crowdsec", "/tmp/crowdsec")

	assert.False(t, exec.startCalled)
	assert.False(t, exec.statusCalled)
}

func TestReconcileCrowdSecOnStartup_ModeLocal_AlreadyRunning(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	exec := &mockCrowdsecExecutor{
		running: true,
		pid:     12345,
	}

	// Create SecurityConfig with mode=local
	cfg := models.SecurityConfig{
		CrowdSecMode: "local",
	}
	require.NoError(t, db.Create(&cfg).Error)

	ReconcileCrowdSecOnStartup(db, exec, "crowdsec", "/tmp/crowdsec")

	assert.True(t, exec.statusCalled)
	assert.False(t, exec.startCalled, "Should not start if already running")
}

func TestReconcileCrowdSecOnStartup_ModeLocal_NotRunning_Starts(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	exec := &mockCrowdsecExecutor{
		running:  false,
		startPid: 99999,
	}

	// Create SecurityConfig with mode=local
	cfg := models.SecurityConfig{
		CrowdSecMode: "local",
	}
	require.NoError(t, db.Create(&cfg).Error)

	ReconcileCrowdSecOnStartup(db, exec, "crowdsec", "/tmp/crowdsec")

	assert.True(t, exec.statusCalled)
	assert.True(t, exec.startCalled, "Should start if mode=local and not running")
}

func TestReconcileCrowdSecOnStartup_ModeLocal_StartError(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	exec := &mockCrowdsecExecutor{
		running:  false,
		startErr: assert.AnError,
	}

	// Create SecurityConfig with mode=local
	cfg := models.SecurityConfig{
		CrowdSecMode: "local",
	}
	require.NoError(t, db.Create(&cfg).Error)

	// Should not panic on start error
	ReconcileCrowdSecOnStartup(db, exec, "crowdsec", "/tmp/crowdsec")

	assert.True(t, exec.startCalled)
}

func TestReconcileCrowdSecOnStartup_StatusError(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	exec := &mockCrowdsecExecutor{
		statusErr: assert.AnError,
	}

	// Create SecurityConfig with mode=local
	cfg := models.SecurityConfig{
		CrowdSecMode: "local",
	}
	require.NoError(t, db.Create(&cfg).Error)

	// Should not panic on status error and should not attempt start
	ReconcileCrowdSecOnStartup(db, exec, "crowdsec", "/tmp/crowdsec")

	assert.True(t, exec.statusCalled)
	assert.False(t, exec.startCalled, "Should not start if status check fails")
}
