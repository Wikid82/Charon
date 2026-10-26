package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
	"gorm.io/gorm"
)

// crowdSecVerifyWait is the settle time after a successful Start before reconciliation checks that the process is
// still running. Production keeps the 2s wait; tests shorten it through a serial-only helper.
var crowdSecVerifyWait = 2 * time.Second

// crowdSecProxySuiteOffMessage is the boot note written when the engine runs but the proxy leaves CrowdSec out
// because the security suite (Cerberus) is off.
const crowdSecProxySuiteOffMessage = "CrowdSec engine started; the proxy leaves CrowdSec out while the security suite is off"

// reconcileOptions holds the optional behavior of ReconcileCrowdSecOnStartup. The zero value is the behavior
// without options.
type reconcileOptions struct {
	cerberusOn func() bool
}

// ReconcileOption configures ReconcileCrowdSecOnStartup. Options are variadic so existing callers stay unchanged.
type ReconcileOption func(*reconcileOptions)

// WithCerberusCheck makes boot report, after a verified start, when the proxy will leave CrowdSec out because the
// security suite is off. on reports whether the proxy applies Cerberus; a nil check reports nothing. The engine
// decision does not depend on it.
func WithCerberusCheck(on func() bool) ReconcileOption {
	return func(o *reconcileOptions) {
		o.cerberusOn = on
	}
}

// ProxyCerberusCheck returns a check for WithCerberusCheck that applies the proxy's own Cerberus decision to the
// current default SecurityConfig row and the settings table.
func ProxyCerberusCheck(db *gorm.DB, static config.SecurityConfig) func() bool {
	return func() bool {
		var row *models.SecurityConfig
		if sc, err := models.FindDefaultSecurityConfig(db); err == nil {
			row = &sc
		}
		return caddy.CerberusEnabledForProxy(db, static, row)
	}
}

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
// Auto-start condition: the CrowdSec decision of models.CrowdSecEnabled is on. The enabled setting wins, then the
// deprecated mode setting, then SecurityConfig.crowdsec_mode == "local".
//
// cmdExec is optional; if nil, a real command executor will be used for bouncer registration.
// Tests should pass a mock to avoid executing real cscli commands.
//
// opts are optional; see WithCerberusCheck.
func ReconcileCrowdSecOnStartup(db *gorm.DB, executor CrowdsecProcessManager, binPath, dataDir string, cmdExec CommandExecutor, opts ...ReconcileOption) {
	var options reconcileOptions
	for _, opt := range opts {
		opt(&options)
	}

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

	// Repair legacy-named rows before reading the default row, so the check below never acts on a stale copy.
	// Routes migration and seeding run after this function, so this call is needed on the first boot after upgrade.
	if repairErr := models.RepairDefaultSecurityConfigName(db); repairErr != nil {
		logger.Log().WithError(repairErr).Warn("CrowdSec reconciliation: failed to repair default SecurityConfig name, continuing")
	}

	cfg, findErr := models.FindDefaultSecurityConfig(db)
	if findErr != nil {
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			logger.Log().WithError(findErr).Warn("CrowdSec reconciliation: failed to read SecurityConfig")
			return
		}
		// AUTO-INITIALIZE: seed the default row, then match its mode to the Settings table preference.
		logger.Log().Info("CrowdSec reconciliation: no default SecurityConfig found, checking Settings table for user preference")
		seeded, seedErr := models.SeedDefaultSecurityConfig(db)
		if seedErr != nil {
			logger.Log().WithError(seedErr).Error("CrowdSec reconciliation: failed to create default SecurityConfig")
			return
		}
		cfg = *seeded

		var settingOverride struct{ Value string }
		if rawErr := db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.enabled").Scan(&settingOverride).Error; rawErr == nil && settingOverride.Value != "" {
			logger.Log().WithFields(map[string]any{
				"setting_value": settingOverride.Value,
				"enabled":       strings.EqualFold(settingOverride.Value, "true"),
			}).Info("CrowdSec reconciliation: found existing Settings table preference")
			if strings.EqualFold(settingOverride.Value, "true") && cfg.CrowdSecMode != "local" {
				if updErr := db.Model(&cfg).Update("crowd_sec_mode", "local").Error; updErr != nil {
					logger.Log().WithError(updErr).Error("CrowdSec reconciliation: failed to set crowd_sec_mode from Settings preference")
					return
				}
				cfg.CrowdSecMode = "local"
			}
		}

		logger.Log().WithFields(map[string]any{
			"crowdsec_mode": cfg.CrowdSecMode,
			"source":        "settings_table",
		}).Info("CrowdSec reconciliation: default SecurityConfig created from Settings preference")
	}

	// Decide with the same setting/row precedence the proxy config and the status endpoint use.
	crowdSecEnabled, crowdSecErr := models.CrowdSecEnabled(db, cfg)
	if crowdSecErr != nil {
		logger.Log().WithError(crowdSecErr).Warn("CrowdSec reconciliation: failed to read CrowdSec setting, using the default SecurityConfig row")
	}
	if !crowdSecEnabled {
		logger.Log().WithField("crowdsec_mode", cfg.CrowdSecMode).Info("CrowdSec reconciliation skipped: CrowdSec is disabled by the setting or the default SecurityConfig row")
		return
	}
	logger.Log().WithField("crowdsec_mode", cfg.CrowdSecMode).Info("CrowdSec reconciliation: starting, CrowdSec is enabled by the setting or the default SecurityConfig row")

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
	time.Sleep(crowdSecVerifyWait)

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

	// The engine follows the CrowdSec setting only; say so when the proxy will not apply CrowdSec with it.
	if options.cerberusOn != nil && !options.cerberusOn() {
		logger.Log().Info(crowdSecProxySuiteOffMessage)
	}

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

	res, err := bouncerkey.Ensure(context.Background(), bouncerKeyFile, startupKeyValid, register, crowdsec.SaveBouncerKey)
	logStartupKeyDecision(res, bouncerKeyFile, err)
	return err
}

// startupKeyValid tests a key against LAPI for the shared decision.
func startupKeyValid(_ context.Context, key string) bool {
	return testKeyAgainstLAPIStartup(key)
}

// rejectedEnvKeyLogMessage is the Warn text for a rejected key variable whose replacement was
// registered. It claims the key file holds the key in use only when the save succeeded.
func rejectedEnvKeyLogMessage(keyFile string, saveErr error) string {
	if saveErr != nil {
		return "The configured CrowdSec API key was rejected by CrowdSec and is ignored. " + bouncerkey.SaveFailedNotice(keyFile)
	}
	return "The configured CrowdSec API key was rejected by CrowdSec and is ignored. " +
		"Charon uses the key file (permissions 0600); no action is required."
}

// rejectedEnvRegisterFailedLogMessage is the Warn text for a rejected key variable when no replacement
// key could be registered, so nothing was saved.
func rejectedEnvRegisterFailedLogMessage(keyFile string) string {
	return "The configured CrowdSec API key was rejected by CrowdSec and is ignored. " + bouncerkey.RegisterFailedNotice(keyFile)
}

// logStartupKeyDecision reports the startup key decision. A failed save is logged at Error and does not
// fail startup. Success lines are logged only for the outcome that really happened: a failed registration
// (regErr) logs the rejection with the registration failure and no success line.
func logStartupKeyDecision(res bouncerkey.EnsureResult, keyFile string, regErr error) {
	if res.SaveErr != nil {
		logger.Log().WithError(res.SaveErr).WithField("key_file", keyFile).Error(
			"CrowdSec bouncer: failed to save the key file; CrowdSec blocking may not work until the key file can be written",
		)
	}
	if res.EnvKeyRejected {
		fields := map[string]any{
			"bouncer":    startupBouncerName,
			"key_file":   keyFile,
			"masked_key": util.MaskAPIKey(res.EnvKey),
		}
		if regErr != nil {
			logger.Log().WithFields(fields).Warn(rejectedEnvRegisterFailedLogMessage(keyFile))
		} else {
			logger.Log().WithFields(fields).Warn(rejectedEnvKeyLogMessage(keyFile, res.SaveErr))
		}
	}
	if regErr != nil {
		return
	}

	switch res.Outcome {
	case bouncerkey.OutcomeSavedKeyValid:
		logger.Log().WithField("file", keyFile).Info("CrowdSec bouncer: file-stored key validated successfully")
		if res.EnvKeyIgnored {
			logger.Log().WithField("file", keyFile).Info("CrowdSec bouncer: a key variable is set but the saved key is in use")
		}
	case bouncerkey.OutcomeEnvKeySaved:
		if res.SaveErr == nil {
			logger.Log().WithFields(map[string]any{
				"source":     "environment_variable",
				"key_file":   keyFile,
				"masked_key": util.MaskAPIKey(res.Key),
			}).Info("CrowdSec bouncer: env var key validated and saved to key file")
		}
	case bouncerkey.OutcomeRegistered:
		if res.SaveErr == nil {
			logger.Log().WithFields(map[string]any{
				"bouncer":    startupBouncerName,
				"key_file":   keyFile,
				"masked_key": util.MaskAPIKey(res.Key),
			}).Info("CrowdSec bouncer: successfully registered and saved key")
		}
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
	m := crowdsec.NewApplyJobManager()
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
