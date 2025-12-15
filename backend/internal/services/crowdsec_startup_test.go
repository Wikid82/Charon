package services

import (
	"context"
	"os"
	"path/filepath"
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

// smartMockCrowdsecExecutor returns running=true after Start is called (for post-start verification)
type smartMockCrowdsecExecutor struct {
	startCalled  bool
	startErr     error
	startPid     int
	statusCalled bool
	statusErr    error
}

func (m *smartMockCrowdsecExecutor) Start(ctx context.Context, binPath, configDir string) (int, error) {
	m.startCalled = true
	return m.startPid, m.startErr
}

func (m *smartMockCrowdsecExecutor) Stop(ctx context.Context, configDir string) error {
	return nil
}

func (m *smartMockCrowdsecExecutor) Status(ctx context.Context, configDir string) (bool, int, error) {
	m.statusCalled = true
	// Return running=true if Start was called (simulates successful start)
	if m.startCalled {
		return true, m.startPid, m.statusErr
	}
	return false, 0, m.statusErr
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

// setupCrowdsecTestFixtures creates temporary binary and config directory for testing
func setupCrowdsecTestFixtures(t *testing.T) (binPath, dataDir string, cleanup func()) {
	t.Helper()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "crowdsec-test-*")
	require.NoError(t, err)

	// Create mock binary file
	binPath = filepath.Join(tempDir, "crowdsec")
	err = os.WriteFile(binPath, []byte("#!/bin/sh\nexit 0\n"), 0o755)
	require.NoError(t, err)

	// Create data directory (passed as dataDir to the function)
	dataDir = filepath.Join(tempDir, "data")
	err = os.MkdirAll(dataDir, 0o755)
	require.NoError(t, err)

	// Create config directory inside data dir (validation checks dataDir/config)
	configDir := filepath.Join(dataDir, "config")
	err = os.MkdirAll(configDir, 0o755)
	require.NoError(t, err)

	cleanup = func() {
		os.RemoveAll(tempDir)
	}

	return binPath, dataDir, cleanup
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
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	exec := &mockCrowdsecExecutor{
		running: true,
		pid:     12345,
	}

	// Create SecurityConfig with mode=local
	cfg := models.SecurityConfig{
		CrowdSecMode: "local",
	}
	require.NoError(t, db.Create(&cfg).Error)

	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir)

	assert.True(t, exec.statusCalled)
	assert.False(t, exec.startCalled, "Should not start if already running")
}

func TestReconcileCrowdSecOnStartup_ModeLocal_NotRunning_Starts(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, configDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	// Mock executor returns not running initially, then running after start
	statusCallCount := 0
	exec := &mockCrowdsecExecutor{
		running:  false,
		startPid: 99999,
	}
	// Override Status to return running=true on second call (post-start verification)
	originalStatus := exec.Status
	_ = originalStatus // silence unused warning
	exec.running = false

	// Create SecurityConfig with mode=local
	cfg := models.SecurityConfig{
		CrowdSecMode: "local",
	}
	require.NoError(t, db.Create(&cfg).Error)

	// We need a smarter mock that returns running=true after Start is called
	smartExec := &smartMockCrowdsecExecutor{
		startPid: 99999,
	}

	ReconcileCrowdSecOnStartup(db, smartExec, binPath, configDir)

	assert.True(t, smartExec.statusCalled)
	assert.True(t, smartExec.startCalled, "Should start if mode=local and not running")
	_ = statusCallCount // silence unused warning
}

func TestReconcileCrowdSecOnStartup_ModeLocal_StartError(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

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
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir)

	assert.True(t, exec.startCalled)
}

func TestReconcileCrowdSecOnStartup_StatusError(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	exec := &mockCrowdsecExecutor{
		statusErr: assert.AnError,
	}

	// Create SecurityConfig with mode=local
	cfg := models.SecurityConfig{
		CrowdSecMode: "local",
	}
	require.NoError(t, db.Create(&cfg).Error)

	// Should not panic on status error and should not attempt start
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir)

	assert.True(t, exec.statusCalled)
	assert.False(t, exec.startCalled, "Should not start if status check fails")
}
