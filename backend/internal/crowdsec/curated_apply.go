package crowdsec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

// ErrCSCLIUnavailable is returned when a curated preset is applied without a working cscli.
var ErrCSCLIUnavailable = errors.New("cscli unavailable")

// ErrCrowdSecNotRunning is returned by a ReloadFunc when no managed CrowdSec process is running.
var ErrCrowdSecNotRunning = errors.New("crowdsec is not running")

// ReloadFunc signals the managed CrowdSec process to reload its configuration and hub items.
type ReloadFunc func(ctx context.Context) error

// curatedBackupTimeFormat includes sub-second precision so back-to-back applies never share a backup dir.
const curatedBackupTimeFormat = "20060102-150405.000000"

// ApplyCurated installs every hub item of a Charon-defined preset through cscli and verifies
// each one. DataDir is never renamed or emptied before success is known: a copy-based backup
// is taken first and restored in place if anything fails.
func (s *HubService) ApplyCurated(ctx context.Context, preset Preset) (ApplyResult, error) {
	result := ApplyResult{AppliedPreset: preset.Slug, Status: "failed", CacheKey: "curated-" + preset.Slug}
	fail := func(err error) (ApplyResult, error) {
		result.Status = "failed"
		result.ErrorMessage = err.Error()
		return result, err
	}

	if err := preset.Validate(); err != nil {
		return fail(err)
	}

	applyCtx, cancel := context.WithTimeout(ctx, s.ApplyTimeout)
	defer cancel()

	if !s.hasCSCLI(applyCtx) {
		return fail(fmt.Errorf("curated preset %s: %w", preset.Slug, ErrCSCLIUnavailable))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	backupPath := filepath.Clean(s.DataDir) + ".backup." + time.Now().Format(curatedBackupTimeFormat)
	if err := s.backupCopy(backupPath); err != nil {
		return fail(fmt.Errorf("backup: %w", err))
	}
	result.BackupPath = backupPath

	if err := s.installAndVerify(applyCtx, preset); err != nil {
		if rbErr := s.restoreCopy(backupPath); rbErr != nil {
			logger.Log().WithError(rbErr).WithField("backup_path", util.SanitizeForLog(backupPath)).Error("curated preset rollback failed; backup retained for manual recovery")
			err = fmt.Errorf("%w (rollback failed: %v; backup retained at %s)", err, rbErr, backupPath)
		}
		return fail(err)
	}

	result.Status = "applied"
	result.UsedCSCLI = true
	result.ReloadHint = !s.reloadCrowdSec(applyCtx, "curated preset apply")
	return result, nil
}

// installAndVerify runs hub update, then install + inspect for every item, stopping at the first failure.
func (s *HubService) installAndVerify(ctx context.Context, preset Preset) error {
	if _, err := s.Exec.Execute(ctx, "cscli", "hub", "update"); err != nil {
		logger.Log().WithError(err).Warn("cscli hub update failed; continuing with the local hub index")
	}
	for _, item := range preset.Items {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("install %s %s: %w", item.Type, item.Name, err)
		}
		if _, err := s.Exec.Execute(ctx, "cscli", item.Type, "install", item.Name); err != nil {
			return fmt.Errorf("install %s %s: %w", item.Type, item.Name, err)
		}
		if err := s.verifyItem(ctx, item); err != nil {
			return fmt.Errorf("verify %s %s: %w", item.Type, item.Name, err)
		}
	}
	return nil
}

// verifyItem requires `cscli <type> inspect <name> -o json` to report the item as installed and untainted.
func (s *HubService) verifyItem(ctx context.Context, item PresetItem) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	out, err := s.Exec.Execute(ctx, "cscli", item.Type, "inspect", item.Name, "-o", "json")
	if err != nil {
		return err
	}
	// Field names follow cscli's `-o json` inspect output (installed, tainted).
	var state struct {
		Installed *bool `json:"installed"`
		Tainted   bool  `json:"tainted"`
	}
	if err := json.Unmarshal(out, &state); err != nil {
		return fmt.Errorf("parse inspect output: %w", err)
	}
	if state.Installed == nil {
		return errors.New("inspect output has no installed field")
	}
	if !*state.Installed {
		return errors.New("item reported as not installed")
	}
	if state.Tainted {
		return errors.New("item reported as tainted")
	}
	return nil
}

// The live crowdsec.db (and -wal/-shm) is excluded so rollback can never regress engine state.
// backupCopy copies DataDir (symlinks preserved) into backupPath, leaving DataDir untouched.
func (s *HubService) backupCopy(backupPath string) error {
	if err := os.Mkdir(backupPath, 0o700); err != nil {
		return fmt.Errorf("mkdir backup: %w", err)
	}
	if err := copyDirFiltered(s.DataDir, backupPath, isLiveDBFile); err != nil {
		_ = os.RemoveAll(backupPath)
		return fmt.Errorf("copy backup: %w", err)
	}
	return nil
}

// restoreCopy replaces the contents of DataDir with the backup while keeping DataDir itself in place.
func (s *HubService) restoreCopy(backupPath string) error {
	if err := emptyDirExcept(s.DataDir, isLiveDBFile); err != nil {
		return fmt.Errorf("empty data dir: %w", err)
	}
	if err := copyDirFiltered(backupPath, s.DataDir, isLiveDBFile); err != nil {
		return fmt.Errorf("restore backup: %w", err)
	}
	return nil
}

// isLiveDBFile reports whether name is the live CrowdSec SQLite database or its WAL/SHM sidecars.
// These are owned by the running engine and must never be copied into, or restored from, a backup.
func isLiveDBFile(name string) bool {
	switch name {
	case "crowdsec.db", "crowdsec.db-wal", "crowdsec.db-shm":
		return true
	}
	return false
}

// emptyDirExcept removes the contents of dir except entries (at any depth) for which keep returns true.
// Directories are removed only when nothing kept remains inside them.
func emptyDirExcept(dir string, keep func(name string) bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if keep(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if err := emptyDirExcept(path, keep); err != nil {
				return err
			}
			// A directory that still holds kept files cannot be removed; leave it in place.
			if remaining, rerr := os.ReadDir(path); rerr == nil && len(remaining) > 0 {
				continue
			}
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

// reloadCrowdSec asks the managed CrowdSec process to reload (best-effort) and reports
// whether the reload was performed. A missing reloader or a stopped process is not an error:
// CrowdSec loads the installed items itself on its next start.
func (s *HubService) reloadCrowdSec(ctx context.Context, reason string) bool {
	if s.Reload == nil {
		return false
	}
	err := s.Reload(ctx)
	switch {
	case err == nil:
		return true
	case errors.Is(err, ErrCrowdSecNotRunning):
		logger.Log().WithField("reason", reason).Info("crowdsec is not running; installed items load on next start")
	default:
		logger.Log().WithError(err).WithField("reason", reason).Warn("crowdsec reload failed; restart required")
	}
	return false
}
