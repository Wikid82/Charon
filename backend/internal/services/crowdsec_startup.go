package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

// CrowdsecProcessManager abstracts starting/stopping/status of CrowdSec process.
// This interface is structurally compatible with handlers.CrowdsecExecutor.
type CrowdsecProcessManager interface {
	Start(ctx context.Context, binPath, configDir string) (int, error)
	Stop(ctx context.Context, configDir string) error
	Status(ctx context.Context, configDir string) (running bool, pid int, err error)
}

// ReconcileCrowdSecOnStartup checks if CrowdSec should be running based on DB settings
// and starts it if necessary. This handles container restart scenarios where the
// user's preference was to have CrowdSec enabled.
func ReconcileCrowdSecOnStartup(db *gorm.DB, executor CrowdsecProcessManager, binPath, dataDir string) {
	logger.Log().WithFields(map[string]interface{}{
		"bin_path": binPath,
		"data_dir": dataDir,
	}).Info("CrowdSec reconciliation: starting startup check")

	if db == nil || executor == nil {
		logger.Log().Debug("CrowdSec reconciliation skipped: nil db or executor")
		return
	}

	// Check if SecurityConfig table exists and has a record with CrowdSecMode = "local"
	if !db.Migrator().HasTable(&models.SecurityConfig{}) {
		logger.Log().Warn("CrowdSec reconciliation skipped: SecurityConfig table not found - run 'charon migrate' to fix")
		return
	}

	var cfg models.SecurityConfig
	if err := db.First(&cfg).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Log().Debug("CrowdSec reconciliation skipped: no SecurityConfig record found")
			return
		}
		logger.Log().WithError(err).Warn("CrowdSec reconciliation: failed to read SecurityConfig")
		return
	}

	// Also check for runtime setting override in settings table
	var settingOverride struct{ Value string }
	crowdSecEnabled := false
	if err := db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.enabled").Scan(&settingOverride).Error; err == nil && settingOverride.Value != "" {
		crowdSecEnabled = strings.EqualFold(settingOverride.Value, "true")
		logger.Log().WithFields(map[string]interface{}{
			"setting_value":    settingOverride.Value,
			"crowdsec_enabled": crowdSecEnabled,
		}).Debug("CrowdSec reconciliation: found runtime setting override")
	}

	// Only auto-start if CrowdSecMode is "local" OR runtime setting is enabled
	if cfg.CrowdSecMode != "local" && !crowdSecEnabled {
		logger.Log().WithFields(map[string]interface{}{
			"db_mode":         cfg.CrowdSecMode,
			"setting_enabled": crowdSecEnabled,
		}).Debug("CrowdSec reconciliation skipped: mode is not 'local' and setting not enabled")
		return
	}

	// VALIDATE: Ensure binary exists
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		logger.Log().WithField("path", binPath).Error("CrowdSec reconciliation: binary not found, cannot start")
		return
	}

	// VALIDATE: Ensure config directory exists
	configPath := filepath.Join(dataDir, "config")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		logger.Log().WithField("path", configPath).Error("CrowdSec reconciliation: config directory not found, cannot start")
		return
	}

	// Check if CrowdSec is already running
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	running, pid, err := executor.Status(ctx, dataDir)
	if err != nil {
		logger.Log().WithError(err).Warn("CrowdSec reconciliation: failed to check status")
		return
	}

	if running {
		logger.Log().WithField("pid", pid).Info("CrowdSec reconciliation: already running")
		return
	}

	// CrowdSec should be running but isn't - start it
	logger.Log().WithFields(map[string]interface{}{
		"bin_path": binPath,
		"data_dir": dataDir,
	}).Info("CrowdSec reconciliation: starting CrowdSec (mode=local, not currently running)")

	startCtx, startCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer startCancel()

	newPid, err := executor.Start(startCtx, binPath, dataDir)
	if err != nil {
		logger.Log().WithError(err).WithFields(map[string]interface{}{
			"bin_path": binPath,
			"data_dir": dataDir,
		}).Error("CrowdSec reconciliation: FAILED to start CrowdSec - check binary and config")
		return
	}

	// VERIFY: Wait briefly and confirm process is actually running
	time.Sleep(2 * time.Second)

	verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer verifyCancel()

	verifyRunning, verifyPid, verifyErr := executor.Status(verifyCtx, dataDir)
	if verifyErr != nil {
		logger.Log().WithError(verifyErr).WithField("expected_pid", newPid).Warn("CrowdSec reconciliation: started but failed to verify status")
		return
	}

	if !verifyRunning {
		logger.Log().WithFields(map[string]interface{}{
			"expected_pid": newPid,
			"actual_pid":   verifyPid,
			"running":      verifyRunning,
		}).Error("CrowdSec reconciliation: process started but is no longer running - may have crashed")
		return
	}

	logger.Log().WithFields(map[string]interface{}{
		"pid":      newPid,
		"verified": true,
	}).Info("CrowdSec reconciliation: successfully started and verified CrowdSec")
}
