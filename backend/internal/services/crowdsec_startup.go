package services

import (
	"context"
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
	if db == nil || executor == nil {
		logger.Log().Debug("CrowdSec reconciliation skipped: nil db or executor")
		return
	}

	// Check if SecurityConfig table exists and has a record with CrowdSecMode = "local"
	if !db.Migrator().HasTable(&models.SecurityConfig{}) {
		logger.Log().Debug("CrowdSec reconciliation skipped: SecurityConfig table not found")
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

	// Only auto-start if CrowdSecMode is "local"
	if cfg.CrowdSecMode != "local" {
		logger.Log().WithField("mode", cfg.CrowdSecMode).Debug("CrowdSec reconciliation skipped: mode is not 'local'")
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
	logger.Log().Info("CrowdSec reconciliation: starting CrowdSec (mode=local, not currently running)")

	startCtx, startCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer startCancel()

	newPid, err := executor.Start(startCtx, binPath, dataDir)
	if err != nil {
		logger.Log().WithError(err).Error("CrowdSec reconciliation: failed to start CrowdSec")
		return
	}

	logger.Log().WithField("pid", newPid).Info("CrowdSec reconciliation: successfully started CrowdSec")
}
