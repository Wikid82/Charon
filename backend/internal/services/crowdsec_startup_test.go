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

func (m *mockCrowdsecExecutor) Status(ctx context.Context, configDir string) (running bool, pid int, err error) {
	m.statusCalled = true
	return m.running, m.pid, m.statusErr
}

// mockCommandExecutor is a test mock for CommandExecutor interface
type mockCommandExecutor struct {
	executeCalls [][]string // Track command invocations
	executeErr   error      // Error to return
	executeOut   []byte     // Output to return
}

func (m *mockCommandExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	m.executeCalls = append(m.executeCalls, append([]string{name}, args...))
	return m.executeOut, m.executeErr
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

func (m *smartMockCrowdsecExecutor) Status(ctx context.Context, configDir string) (running bool, pid int, err error) {
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
	err = os.WriteFile(binPath, []byte("#!/bin/sh\nexit 0\n"), 0o750) // #nosec G306 -- executable test script
	require.NoError(t, err)

	// Create data directory (passed as dataDir to the function)
	dataDir = filepath.Join(tempDir, "data")
	err = os.MkdirAll(dataDir, 0o750) // #nosec G301 -- test directory
	require.NoError(t, err)

	// Create config directory inside data dir (validation checks dataDir/config)
	configDir := filepath.Join(dataDir, "config")
	err = os.MkdirAll(configDir, 0o750) // #nosec G301 -- test directory
	require.NoError(t, err)

	cleanup = func() {
		_ = os.RemoveAll(tempDir)
	}

	return binPath, dataDir, cleanup
}

func TestReconcileCrowdSecOnStartup_NilDB(t *testing.T) {
	exec := &mockCrowdsecExecutor{}
	cmdExec := &mockCommandExecutor{}

	// Should not panic with nil db
	ReconcileCrowdSecOnStartup(nil, exec, "crowdsec", "/tmp/crowdsec", cmdExec)

	assert.False(t, exec.startCalled)
	assert.False(t, exec.statusCalled)
}

func TestReconcileCrowdSecOnStartup_NilExecutor(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	cmdExec := &mockCommandExecutor{}

	// Should not panic with nil executor
	ReconcileCrowdSecOnStartup(db, nil, "crowdsec", "/tmp/crowdsec", cmdExec)
}

func TestReconcileCrowdSecOnStartup_NoSecurityConfig_NoSettings(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	exec := &mockCrowdsecExecutor{}
	cmdExec := &mockCommandExecutor{}

	// No SecurityConfig record, no Settings entry - should create default config with mode=disabled and skip start
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, cmdExec)

	// Verify SecurityConfig was created with disabled mode
	var cfg models.SecurityConfig
	err := db.First(&cfg).Error
	require.NoError(t, err)
	assert.Equal(t, "disabled", cfg.CrowdSecMode)
	// Note: cfg.Enabled is the global Cerberus flag (always true by default), not CrowdSec-specific
	assert.True(t, cfg.Enabled, "Cerberus global flag should be enabled by default")

	// Should not attempt to start since mode is disabled
	assert.False(t, exec.startCalled)
}

func TestReconcileCrowdSecOnStartup_NoSecurityConfig_SettingsEnabled(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	// Create Settings table and add entry for security.crowdsec.enabled=true
	err := db.AutoMigrate(&models.Setting{})
	require.NoError(t, err)

	setting := models.Setting{
		Key:      "security.crowdsec.enabled",
		Value:    "true",
		Type:     "bool",
		Category: "security",
	}
	require.NoError(t, db.Create(&setting).Error)

	// Mock executor that returns running=true after start
	exec := &smartMockCrowdsecExecutor{
		startPid: 12345,
	}
	cmdExec := &mockCommandExecutor{} // Mock command executor to avoid real cscli calls

	// No SecurityConfig record but Settings enabled - should create config with mode=local and start
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, cmdExec)

	// Verify SecurityConfig was created with local mode
	var cfg models.SecurityConfig
	err = db.First(&cfg).Error
	require.NoError(t, err)
	assert.Equal(t, "local", cfg.CrowdSecMode)
	assert.True(t, cfg.Enabled)

	// Should attempt to start since Settings says enabled
	assert.True(t, exec.startCalled, "Should start CrowdSec when Settings table indicates enabled")
	assert.True(t, exec.statusCalled, "Should check status before and after start")
}

func TestReconcileCrowdSecOnStartup_NoSecurityConfig_SettingsDisabled(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	// Create Settings table and add entry for security.crowdsec.enabled=false
	err := db.AutoMigrate(&models.Setting{})
	require.NoError(t, err)

	setting := models.Setting{
		Key:      "security.crowdsec.enabled",
		Value:    "false",
		Type:     "bool",
		Category: "security",
	}
	require.NoError(t, db.Create(&setting).Error)

	exec := &mockCrowdsecExecutor{}
	cmdExec := &mockCommandExecutor{}

	// No SecurityConfig record, Settings disabled - should create config with mode=disabled and skip start
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, cmdExec)

	// Verify SecurityConfig was created with disabled mode
	var cfg models.SecurityConfig
	err = db.First(&cfg).Error
	require.NoError(t, err)
	assert.Equal(t, "disabled", cfg.CrowdSecMode)
	// Note: cfg.Enabled is the global Cerberus flag (always true by default), not CrowdSec-specific
	assert.True(t, cfg.Enabled, "Cerberus global flag should be enabled by default")

	// Should not attempt to start
	assert.False(t, exec.startCalled)
}

func TestReconcileCrowdSecOnStartup_ModeDisabled(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	exec := &mockCrowdsecExecutor{}
	cmdExec := &mockCommandExecutor{}

	// Create SecurityConfig with mode=disabled
	cfg := models.SecurityConfig{
		CrowdSecMode: "disabled",
	}
	require.NoError(t, db.Create(&cfg).Error)

	ReconcileCrowdSecOnStartup(db, exec, "crowdsec", "/tmp/crowdsec", cmdExec)

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
	cmdExec := &mockCommandExecutor{}

	// Create SecurityConfig with mode=local
	cfg := models.SecurityConfig{
		CrowdSecMode: "local",
	}
	require.NoError(t, db.Create(&cfg).Error)

	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, cmdExec)

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
	cmdExec := &mockCommandExecutor{} // Mock to avoid real cscli calls

	ReconcileCrowdSecOnStartup(db, smartExec, binPath, configDir, cmdExec)

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
	cmdExec := &mockCommandExecutor{}

	// Create SecurityConfig with mode=local
	cfg := models.SecurityConfig{
		CrowdSecMode: "local",
	}
	require.NoError(t, db.Create(&cfg).Error)

	// Should not panic on start error
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, cmdExec)

	assert.True(t, exec.startCalled)
}

func TestReconcileCrowdSecOnStartup_StatusError(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	exec := &mockCrowdsecExecutor{
		statusErr: assert.AnError,
	}
	cmdExec := &mockCommandExecutor{}

	// Create SecurityConfig with mode=local
	cfg := models.SecurityConfig{
		CrowdSecMode: "local",
	}
	require.NoError(t, db.Create(&cfg).Error)

	// Should not panic on status error and should not attempt start
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, cmdExec)

	assert.True(t, exec.statusCalled)
	assert.False(t, exec.startCalled, "Should not start if status check fails")
}

// ==========================================================
// Additional Edge Case Tests for 100% Coverage
// ==========================================================

func TestReconcileCrowdSecOnStartup_BinaryNotFound(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	_, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	exec := &smartMockCrowdsecExecutor{
		startPid: 99999,
	}
	cmdExec := &mockCommandExecutor{}

	// Create SecurityConfig with mode=local
	cfg := models.SecurityConfig{
		CrowdSecMode: "local",
	}
	require.NoError(t, db.Create(&cfg).Error)

	// Pass non-existent binary path
	nonExistentBin := filepath.Join(dataDir, "nonexistent_binary")
	ReconcileCrowdSecOnStartup(db, exec, nonExistentBin, dataDir, cmdExec)

	// Should not attempt start when binary doesn't exist
	assert.False(t, exec.startCalled, "Should not start when binary not found")
}

func TestReconcileCrowdSecOnStartup_ConfigDirNotFound(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	exec := &smartMockCrowdsecExecutor{
		startPid: 99999,
	}
	cmdExec := &mockCommandExecutor{}

	// Create SecurityConfig with mode=local
	cfg := models.SecurityConfig{
		CrowdSecMode: "local",
	}
	require.NoError(t, db.Create(&cfg).Error)

	// Delete config directory
	configPath := filepath.Join(dataDir, "config")
	require.NoError(t, os.RemoveAll(configPath))

	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, cmdExec)

	// Should not attempt start when config dir doesn't exist
	assert.False(t, exec.startCalled, "Should not start when config directory not found")
}

func TestReconcileCrowdSecOnStartup_SettingsOverrideEnabled(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	// Create Settings table and add override
	err := db.AutoMigrate(&models.Setting{})
	require.NoError(t, err)

	setting := models.Setting{
		Key:      "security.crowdsec.enabled",
		Value:    "true",
		Type:     "bool",
		Category: "security",
	}
	require.NoError(t, db.Create(&setting).Error)

	// Create SecurityConfig with mode=disabled
	cfg := models.SecurityConfig{
		CrowdSecMode: "disabled",
		Enabled:      false,
	}
	require.NoError(t, db.Create(&cfg).Error)

	exec := &smartMockCrowdsecExecutor{
		startPid: 12345,
	}
	cmdExec := &mockCommandExecutor{} // Mock to avoid real cscli calls

	// Should start based on Settings override even though SecurityConfig says disabled
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, cmdExec)

	assert.True(t, exec.startCalled, "Should start when Settings override is true")
}

func TestReconcileCrowdSecOnStartup_VerificationFails(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	// Use a verification executor that starts but verification returns not running
	exec := &verificationFailExecutor{
		startPid: 12345,
	}
	cmdExec := &mockCommandExecutor{} // Mock to avoid real cscli calls

	// Create SecurityConfig with mode=local
	cfg := models.SecurityConfig{
		CrowdSecMode: "local",
	}
	require.NoError(t, db.Create(&cfg).Error)

	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, cmdExec)

	assert.True(t, exec.startCalled, "Should attempt to start")
	assert.True(t, exec.verifyFailed, "Should detect verification failure")
}

func TestReconcileCrowdSecOnStartup_VerificationError(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	exec := &verificationErrorExecutor{
		startPid: 12345,
	}
	cmdExec := &mockCommandExecutor{} // Mock to avoid real cscli calls

	// Create SecurityConfig with mode=local
	cfg := models.SecurityConfig{
		CrowdSecMode: "local",
	}
	require.NoError(t, db.Create(&cfg).Error)

	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, cmdExec)

	assert.True(t, exec.startCalled, "Should attempt to start")
	assert.True(t, exec.verifyErrorReturned, "Should handle verification error")
}

func TestReconcileCrowdSecOnStartup_DBError(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	exec := &smartMockCrowdsecExecutor{
		startPid: 99999,
	}
	cmdExec := &mockCommandExecutor{}

	// Create SecurityConfig with mode=local
	cfg := models.SecurityConfig{
		UUID:         "test",
		CrowdSecMode: "local",
	}
	require.NoError(t, db.Create(&cfg).Error)

	// Close DB to simulate DB error (this will cause queries to fail)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	// Should handle DB errors gracefully (no panic)
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, cmdExec)

	// Should not start if DB query fails
	assert.False(t, exec.startCalled)
}

func TestReconcileCrowdSecOnStartup_CreateConfigDBError(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	exec := &smartMockCrowdsecExecutor{
		startPid: 99999,
	}
	cmdExec := &mockCommandExecutor{}

	// Close DB immediately to cause Create() to fail
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	// Should handle DB error during Create gracefully (no panic)
	// This tests line 78-80: DB error after creating SecurityConfig
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, cmdExec)

	// Should not start if SecurityConfig creation fails
	assert.False(t, exec.startCalled)
}

func TestReconcileCrowdSecOnStartup_SettingsTableQueryError(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	exec := &smartMockCrowdsecExecutor{
		startPid: 99999,
	}
	cmdExec := &mockCommandExecutor{}

	// Create SecurityConfig with mode=remote (not local)
	cfg := models.SecurityConfig{
		CrowdSecMode: "remote",
		Enabled:      false,
	}
	require.NoError(t, db.Create(&cfg).Error)

	// Don't create Settings table - this will cause the RAW query to fail
	// But gorm will still return nil error with empty result
	// This tests lines 83-90: Settings table query handling

	// Should handle missing settings table gracefully
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, cmdExec)

	// Should not start since mode is not local and no settings override
	assert.False(t, exec.startCalled)
}

func TestReconcileCrowdSecOnStartup_SettingsOverrideNonLocalMode(t *testing.T) {
	db := setupCrowdsecTestDB(t)
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	// Create Settings table and add override
	err := db.AutoMigrate(&models.Setting{})
	require.NoError(t, err)

	setting := models.Setting{
		Key:      "security.crowdsec.enabled",
		Value:    "true",
		Type:     "bool",
		Category: "security",
	}
	require.NoError(t, db.Create(&setting).Error)

	// Create SecurityConfig with mode=remote (not local)
	cfg := models.SecurityConfig{
		CrowdSecMode: "remote",
		Enabled:      false,
	}
	require.NoError(t, db.Create(&cfg).Error)

	exec := &smartMockCrowdsecExecutor{
		startPid: 12345,
	}
	cmdExec := &mockCommandExecutor{} // Mock to avoid real cscli calls

	// This tests lines 92-99: Settings override with non-local mode
	// Should start based on Settings override even though SecurityConfig says mode=remote
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, cmdExec)

	assert.True(t, exec.startCalled, "Should start when Settings override is true even if mode is not local")
}

// ==========================================================
// Helper Mocks for Edge Case Tests
// ==========================================================

// verificationFailExecutor simulates Start succeeding but verification showing not running
type verificationFailExecutor struct {
	startCalled  bool
	startPid     int
	statusCalls  int
	verifyFailed bool
}

func (m *verificationFailExecutor) Start(ctx context.Context, binPath, configDir string) (int, error) {
	m.startCalled = true
	return m.startPid, nil
}

func (m *verificationFailExecutor) Stop(ctx context.Context, configDir string) error {
	return nil
}

func (m *verificationFailExecutor) Status(ctx context.Context, configDir string) (running bool, pid int, err error) {
	m.statusCalls++
	// First call (pre-start check): not running
	// Second call (post-start verify): still not running (FAIL)
	if m.statusCalls > 1 {
		m.verifyFailed = true
		return false, 0, nil
	}
	return false, 0, nil
}

// verificationErrorExecutor simulates Start succeeding but verification returning error
type verificationErrorExecutor struct {
	startCalled         bool
	startPid            int
	statusCalls         int
	verifyErrorReturned bool
}

func (m *verificationErrorExecutor) Start(ctx context.Context, binPath, configDir string) (int, error) {
	m.startCalled = true
	return m.startPid, nil
}

func (m *verificationErrorExecutor) Stop(ctx context.Context, configDir string) error {
	return nil
}

func (m *verificationErrorExecutor) Status(ctx context.Context, configDir string) (running bool, pid int, err error) {
	m.statusCalls++
	// First call: not running
	// Second call: return error during verification
	if m.statusCalls > 1 {
		m.verifyErrorReturned = true
		return false, 0, assert.AnError
	}
	return false, 0, nil
}
