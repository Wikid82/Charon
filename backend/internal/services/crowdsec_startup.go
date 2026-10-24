package services

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
	"gorm.io/gorm"
)

// reconcileLock prevents concurrent reconciliation calls.
// This mutex is necessary because reconciliation can be triggered from multiple sources:
// 1. Container startup (main.go calls synchronously during boot)
// 2. Manual GUI toggle (user clicks Start/Stop in Security dashboard)
// 3. Future auto-restart (watchdog could trigger on crash)
// Without this mutex, race conditions could occur:
// - Multiple goroutines starting CrowdSec simultaneously
// - Database race conditions on SecurityConfig table
// - Duplicate process spawning
// - Corrupted state in executor
var reconcileLock sync.Mutex

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
//
// This function is called during container startup (before HTTP server starts) and
// ensures CrowdSec automatically resumes if it was previously enabled. It checks both
// the SecurityConfig table (primary source) and Settings table (fallback/legacy support).
//
// Mutex Protection: This function uses a global mutex to prevent concurrent execution,
// which could occur if multiple startup routines or manual toggles happen simultaneously.
//
// Initialization Order:
// 1. Container boot
// 2. Database migrations (ensures SecurityConfig table exists)
// 3. ReconcileCrowdSecOnStartup (this function) ← YOU ARE HERE
// 4. HTTP server starts
// 5. Routes registered
//
// Auto-start conditions (if ANY true, CrowdSec starts):
// - SecurityConfig.crowdsec_mode == "local"
// - Settings["security.crowdsec.enabled"] == "true"
//
// cmdExec is optional; if nil, a real command executor will be used for bouncer registration.
// Tests should pass a mock to avoid executing real cscli commands.
func ReconcileCrowdSecOnStartup(db *gorm.DB, executor CrowdsecProcessManager, binPath, dataDir string, cmdExec CommandExecutor) {
	// Prevent concurrent reconciliation calls
	reconcileLock.Lock()
	defer reconcileLock.Unlock()

	logger.Log().WithFields(map[string]any{
		"bin_path": binPath,
		"data_dir": dataDir,
	}).Info("CrowdSec reconciliation: starting startup check")

	if db == nil || executor == nil {
		logger.Log().Debug("CrowdSec reconciliation skipped: nil db or executor")
		return
	}

	// An install job the previous process left running died half way; put the pre-install snapshot back before
	// anything (including the engine) reads the configuration. This runs whatever the CrowdSec mode is.
	recoverInterruptedInstalls(db, dataDir)

	// Check if SecurityConfig table exists and has a record with CrowdSecMode = "local"
	if !db.Migrator().HasTable(&models.SecurityConfig{}) {
		logger.Log().Warn("CrowdSec reconciliation skipped: SecurityConfig table not found - run 'charon migrate' to fix")
		return
	}

	var cfg models.SecurityConfig
	if err := db.First(&cfg).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// AUTO-INITIALIZE: Create default SecurityConfig by checking Settings table
			logger.Log().Info("CrowdSec reconciliation: no SecurityConfig found, checking Settings table for user preference")

			// Check if user has already enabled CrowdSec via Settings table (from toggle or legacy config)
			var settingOverride struct{ Value string }
			crowdSecEnabledInSettings := false
			if rawErr := db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.enabled").Scan(&settingOverride).Error; rawErr == nil && settingOverride.Value != "" {
				crowdSecEnabledInSettings = strings.EqualFold(settingOverride.Value, "true")
				logger.Log().WithFields(map[string]any{
					"setting_value": settingOverride.Value,
					"enabled":       crowdSecEnabledInSettings,
				}).Info("CrowdSec reconciliation: found existing Settings table preference")
			}

			// Create SecurityConfig that matches Settings table state
			crowdSecMode := "disabled"
			if crowdSecEnabledInSettings {
				crowdSecMode = "local"
			}

			defaultCfg := models.SecurityConfig{
				UUID:               "default",
				Name:               "Default Security Config",
				Enabled:            true, // Cerberus enabled by default; users can disable via "break glass" toggle
				CrowdSecMode:       crowdSecMode,
				WAFMode:            "disabled",
				WAFParanoiaLevel:   1,
				RateLimitMode:      "disabled",
				RateLimitBurst:     10,
				RateLimitRequests:  100,
				RateLimitWindowSec: 60,
			}

			if createErr := db.Create(&defaultCfg).Error; createErr != nil {
				logger.Log().WithError(createErr).Error("CrowdSec reconciliation: failed to create default SecurityConfig")
				return
			}

			logger.Log().WithFields(map[string]any{
				"crowdsec_mode": defaultCfg.CrowdSecMode,
				"enabled":       defaultCfg.Enabled,
				"source":        "settings_table",
			}).Info("CrowdSec reconciliation: default SecurityConfig created from Settings preference")

			// Continue to process the config (DON'T return early)
			cfg = defaultCfg
		} else {
			logger.Log().WithError(err).Warn("CrowdSec reconciliation: failed to read SecurityConfig")
			return
		}
	}

	// Also check for runtime setting override in settings table
	var settingOverride struct{ Value string }
	crowdSecEnabled := false
	if err := db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.enabled").Scan(&settingOverride).Error; err == nil && settingOverride.Value != "" {
		crowdSecEnabled = strings.EqualFold(settingOverride.Value, "true")
		logger.Log().WithFields(map[string]any{
			"setting_value":    settingOverride.Value,
			"crowdsec_enabled": crowdSecEnabled,
		}).Debug("CrowdSec reconciliation: found runtime setting override")
	}

	// Only auto-start if CrowdSecMode is "local" OR runtime setting is enabled
	if cfg.CrowdSecMode != "local" && !crowdSecEnabled {
		logger.Log().WithFields(map[string]any{
			"db_mode":         cfg.CrowdSecMode,
			"setting_enabled": crowdSecEnabled,
		}).Info("CrowdSec reconciliation skipped: both SecurityConfig and Settings indicate disabled")
		return
	}

	// Log which source triggered the start
	if cfg.CrowdSecMode == "local" {
		logger.Log().WithField("mode", cfg.CrowdSecMode).Info("CrowdSec reconciliation: starting based on SecurityConfig mode='local'")
	} else if crowdSecEnabled {
		logger.Log().WithField("setting", "true").Info("CrowdSec reconciliation: starting based on Settings table override")
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
	logger.Log().WithFields(map[string]any{
		"bin_path": binPath,
		"data_dir": dataDir,
	}).Info("CrowdSec reconciliation: starting CrowdSec (mode=local, not currently running)")

	// Regenerate whitelist YAML before starting so CrowdSec loads the current entries.
	whitelistSvc := NewCrowdSecWhitelistService(db, dataDir)
	if writeErr := whitelistSvc.WriteYAML(context.Background()); writeErr != nil {
		logger.Log().WithError(writeErr).Warn("CrowdSec reconciliation: failed to write whitelist YAML on startup (non-fatal)")
	}

	startCtx, startCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer startCancel()

	newPid, err := executor.Start(startCtx, binPath, dataDir)
	if err != nil {
		logger.Log().WithError(err).WithFields(map[string]any{
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
		logger.Log().WithFields(map[string]any{
			"expected_pid": newPid,
			"actual_pid":   verifyPid,
			"running":      verifyRunning,
		}).Error("CrowdSec reconciliation: process started but is no longer running - may have crashed")
		return
	}

	logger.Log().WithFields(map[string]any{
		"pid":      newPid,
		"verified": true,
	}).Info("CrowdSec reconciliation: successfully started and verified CrowdSec")

	// Register bouncer with LAPI after successful startup
	// This ensures the bouncer API key is registered even if user provided an invalid env var key
	if cmdExec == nil {
		cmdExec = &simpleCommandExecutor{}
	}
	if err := ensureBouncerRegistrationOnStartup(dataDir, cmdExec); err != nil {
		logger.Log().WithError(err).Warn("CrowdSec reconciliation: started successfully but bouncer registration failed")
	}
}

// CommandExecutor abstracts command execution for testing
type CommandExecutor interface {
	Execute(ctx context.Context, name string, args ...string) ([]byte, error)
}

const defaultBouncerKeyFile = bouncerkey.DefaultFile

// startupBouncerName is the bouncer registered with LAPI at startup.
const startupBouncerName = "caddy-bouncer"

// ensureBouncerRegistrationOnStartup registers the caddy-bouncer with LAPI during container startup.
// This is called after CrowdSec LAPI is confirmed running to ensure bouncer key is properly registered.
// Priority: a valid saved key wins, then a valid env key is saved, then a new key is registered.
func ensureBouncerRegistrationOnStartup(dataDir string, cmdExec CommandExecutor) error {
	return ensureBouncerRegistration(dataDir, defaultBouncerKeyFile, cmdExec)
}

// ensureBouncerRegistration implements ensureBouncerRegistrationOnStartup with an explicit key-file path.
func ensureBouncerRegistration(dataDir, bouncerKeyFile string, cmdExec CommandExecutor) error {
	const (
		maxLAPIWait  = 30 * time.Second
		pollInterval = 1 * time.Second
	)
	bouncerKeyFile = filepath.Clean(bouncerKeyFile)

	// Every cscli call below reads the same engine configuration as the running engine (empty when none is found).
	configPath := crowdsec.FindConfigFile(dataDir)

	// Wait for LAPI to be ready (poll cscli lapi status)
	logger.Log().Info("CrowdSec bouncer registration: waiting for LAPI to be ready...")
	deadline := time.Now().Add(maxLAPIWait)
	lapiReady := false

	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := crowdsec.RunCSCLI(ctx, cmdExec, configPath, "lapi", "status")
		cancel()

		if err == nil {
			lapiReady = true
			logger.Log().Info("CrowdSec bouncer registration: LAPI is ready")
			break
		}

		time.Sleep(pollInterval)
	}

	if !lapiReady {
		return fmt.Errorf("LAPI not ready within timeout %v", maxLAPIWait)
	}

	// The shared decision: a valid saved key wins, then a valid env key is saved, then a new bouncer is registered.
	// Startup runs before Caddy is created, so no Caddy re-apply is needed for any outcome.
	register := func(ctx context.Context) (string, error) {
		// Delete existing bouncer if present (stale registration)
		delCtx, delCancel := context.WithTimeout(ctx, 5*time.Second)
		_, _ = crowdsec.RunCSCLI(delCtx, cmdExec, configPath, "bouncers", "delete", startupBouncerName)
		delCancel()

		regCtx, regCancel := context.WithTimeout(ctx, 10*time.Second)
		defer regCancel()

		output, err := crowdsec.RunCSCLI(regCtx, cmdExec, configPath, "bouncers", "add", startupBouncerName, "-o", "raw")
		if err != nil {
			logger.Log().WithError(err).WithField("output", util.DescribeCommandOutput(output)).Error("bouncer registration failed")
			return "", fmt.Errorf("bouncer registration failed: %w", err)
		}

		newKey := strings.TrimSpace(string(output))
		if newKey == "" {
			logger.Log().Error("bouncer registration returned empty key")
			return "", fmt.Errorf("bouncer registration returned empty key")
		}
		return newKey, nil
	}

	res, err := bouncerkey.Ensure(context.Background(), bouncerKeyFile, startupKeyValid, register, saveStartupBouncerKey)
	logStartupKeyDecision(res, bouncerKeyFile)
	return err
}

// startupKeyValid tests a key against LAPI for the shared decision.
func startupKeyValid(_ context.Context, key string) bool {
	return testKeyAgainstLAPIStartup(key)
}

// saveStartupBouncerKey writes the key file that Caddy and the handlers read.
func saveStartupBouncerKey(path, key string) error {
	keyDir := filepath.Dir(path)
	if err := os.MkdirAll(keyDir, 0o750); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(key), 0o600); err != nil {
		return fmt.Errorf("failed to save bouncer key: %w", err)
	}
	return nil
}

// rejectedEnvKeyLogMessage is the Warn text for a rejected key variable. It claims the key file
// holds the key in use only when the save succeeded.
func rejectedEnvKeyLogMessage(keyFile string, saveErr error) string {
	if saveErr != nil {
		return "The configured CrowdSec API key was rejected by CrowdSec and is ignored. " +
			"Charon could not save its new key to " + keyFile + "; the new key is valid for this run only."
	}
	return "The configured CrowdSec API key was rejected by CrowdSec and is ignored. " +
		"Charon uses the key file (permissions 0600); no action is required."
}

// logStartupKeyDecision reports the startup key decision. A failed save is logged at Error and
// does not fail startup; the key is valid for this run.
func logStartupKeyDecision(res bouncerkey.EnsureResult, keyFile string) {
	if res.SaveErr != nil {
		logger.Log().WithError(res.SaveErr).WithField("key_file", keyFile).Error(
			"CrowdSec bouncer: failed to save the key file; the key is valid for this run only",
		)
	}
	if res.EnvKeyRejected {
		logger.Log().WithFields(map[string]any{
			"bouncer":    startupBouncerName,
			"key_file":   keyFile,
			"masked_key": util.MaskAPIKey(res.EnvKey),
		}).Warn(rejectedEnvKeyLogMessage(keyFile, res.SaveErr))
	}

	switch res.Outcome {
	case bouncerkey.OutcomeSavedKeyValid:
		logger.Log().WithField("file", keyFile).Info("CrowdSec bouncer: file-stored key validated successfully")
		if res.EnvKeyIgnored {
			logger.Log().WithField("file", keyFile).Info("CrowdSec bouncer: a key variable is set but the saved key is in use")
		}
	case bouncerkey.OutcomeEnvKeySaved:
		logger.Log().WithFields(map[string]any{
			"source":     "environment_variable",
			"key_file":   keyFile,
			"masked_key": util.MaskAPIKey(res.Key),
		}).Info("CrowdSec bouncer: env var key validated and saved to key file")
	case bouncerkey.OutcomeRegistered:
		logger.Log().WithFields(map[string]any{
			"bouncer":    startupBouncerName,
			"key_file":   keyFile,
			"masked_key": util.MaskAPIKey(res.Key),
		}).Info("CrowdSec bouncer: successfully registered and saved key")
	}
}

// Helper functions for startup bouncer registration (minimal dependencies)

func testKeyAgainstLAPIStartup(apiKey string) bool {
	if apiKey == "" {
		return false
	}

	lapiURL := os.Getenv("CHARON_SECURITY_CROWDSEC_API_URL")
	if lapiURL == "" {
		lapiURL = "http://127.0.0.1:8085"
	}
	endpoint := strings.TrimRight(lapiURL, "/") + "/v1/decisions/stream"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody) //nolint:gosec // G704: endpoint from env config, not user input
	if err != nil {
		return false
	}
	req.Header.Set("X-Api-Key", apiKey)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req) //nolint:gosec // G704: request built from trusted config endpoint
	if err != nil {
		return false
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Debug("Failed to close HTTP response body")
		}
	}()

	return resp.StatusCode == 200
}

// simpleCommandExecutor provides minimal command execution for startup registration
type simpleCommandExecutor struct{}

func (e *simpleCommandExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // G204: command is from internal allowlist, not user input
	cmd.Env = os.Environ()
	return crowdsec.CombinedOutputLimited(cmd)
}

// installRecoveryTimeout bounds the database work of recoverInterruptedInstalls; the restore itself is local
// file copying and is not bounded.
const installRecoveryTimeout = 30 * time.Second

// recoverInterruptedInstalls fails every background preset install job a previous process left active and
// restores the snapshot it took before installing, because the cscli child died with the process and the
// tree may be half installed. A missing or unrestorable snapshot is recorded as a warning on the job and never
// touches the configuration. Nothing here can stop the start.
func recoverInterruptedInstalls(db *gorm.DB, dataDir string) {
	if db == nil || !db.Migrator().HasTable(&models.CrowdsecApplyJob{}) {
		return
	}
	m := crowdsec.NewApplyJobManager(nil)
	m.Configure(crowdsec.ApplyJobConfig{
		Repo: crowdsec.NewApplyJobRepository(db),
		PruneSnapshots: func(protect []string) {
			if _, err := crowdsec.PruneBackupsProtecting(dataDir, crowdsec.BackupKindSnapshot, crowdsec.DefaultBackupRetention, protect); err != nil {
				logger.Log().WithError(err).Warn("CrowdSec reconciliation: pruning snapshots after recovery incomplete")
			}
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), installRecoveryTimeout)
	defer cancel()
	err := m.RecoverInterrupted(ctx, func(backupPath string) error {
		return crowdsec.Restore(backupPath, dataDir)
	})
	if err != nil {
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).Warn("CrowdSec reconciliation: recovering interrupted preset installs failed")
		return
	}
	if err := m.Prune(ctx); err != nil {
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).Warn("CrowdSec reconciliation: pruning preset install jobs failed")
	}
}
