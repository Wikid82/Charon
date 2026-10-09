package crowdsec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// ErrCSCLIUnavailable is returned when a curated preset is applied without a working cscli.
var ErrCSCLIUnavailable = errors.New("cscli unavailable")

// ErrCrowdSecNotRunning is returned by a ReloadFunc when no managed CrowdSec process is running.
var ErrCrowdSecNotRunning = errors.New("crowdsec is not running")

// ReloadFunc signals the managed CrowdSec process to reload its configuration and hub items.
type ReloadFunc func(ctx context.Context) error

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

	backupPath, err := s.snapshot()
	if err != nil {
		return fail(fmt.Errorf("backup: %w", err))
	}
	result.BackupPath = backupPath
	defer s.pruneAfterApply()

	if err := s.installAndVerify(applyCtx, preset); err != nil {
		if rbErr := s.restore(backupPath); rbErr != nil {
			err = rollbackFailure(err, rbErr, backupPath)
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
