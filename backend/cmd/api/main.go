// Package main is the entry point for the Charon backend API.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/api/routes"
	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/cerberus"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/database"
	"github.com/Wikid82/charon/backend/internal/dbmaint"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/server"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/version"
	_ "github.com/Wikid82/charon/backend/pkg/dnsprovider/builtin" // Register built-in DNS providers
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/gorm"
)

// parsePluginSignatures reads the CHARON_PLUGIN_SIGNATURES environment variable
// and returns the parsed signature allowlist for plugin verification.
//
// Modes:
//   - nil return (permissive): Env var unset/empty — all plugins allowed
//   - empty map (strict): Env var set to "{}" — no external plugins allowed
//   - populated map: Only plugins with matching signatures are allowed
func parsePluginSignatures() map[string]string {
	envVal := os.Getenv("CHARON_PLUGIN_SIGNATURES")
	if envVal == "" {
		logger.Log().Info("Plugin signature verification: PERMISSIVE mode (CHARON_PLUGIN_SIGNATURES not set)")
		return nil
	}

	var signatures map[string]string
	if err := json.Unmarshal([]byte(envVal), &signatures); err != nil {
		logger.Log().WithError(err).Error("Failed to parse CHARON_PLUGIN_SIGNATURES JSON — falling back to permissive mode")
		return nil
	}

	// Validate all signatures have sha256: prefix
	for name, sig := range signatures {
		if !strings.HasPrefix(sig, "sha256:") {
			logger.Log().Errorf("Invalid signature for plugin %q: must have sha256: prefix — falling back to permissive mode", name)
			return nil
		}
	}

	if len(signatures) == 0 {
		logger.Log().Info("Plugin signature verification: STRICT mode (empty allowlist — no external plugins permitted)")
	} else {
		logger.Log().Infof("Plugin signature verification: STRICT mode (%d plugin(s) in allowlist)", len(signatures))
	}

	return signatures
}

// loadConfigForDatabase loads the configuration and points SQLite's temp
// directory at the data volume. SQLite only reads SQLITE_TMPDIR before the
// process's first sql.Open, so every entry point calls this before it opens a
// database (GH #1422). An operator-set SQLITE_TMPDIR is left untouched.
func loadConfigForDatabase() (config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, err
	}
	dbmaint.ApplyTempDir(filepath.Dir(filepath.Clean(cfg.DatabasePath)))
	return cfg, nil
}

func main() {
	// Setup logging with rotation
	logDir := "/app/data/logs"
	// #nosec G301 -- Log directory with standard permissions
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		// Fallback to local directory if /app/data fails (e.g. local dev)
		logDir = "data/logs"
		// #nosec G301 -- Fallback log directory with standard permissions
		_ = os.MkdirAll(logDir, 0o755)
	}

	logFile := filepath.Join(logDir, "charon.log")
	rotator := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
		Compress:   true,
	}

	// Ensure legacy cpmp.log exists as symlink for compatibility (cpmp is a legacy name for Charon)
	legacyLog := filepath.Join(logDir, "cpmp.log")
	if _, err := os.Lstat(legacyLog); os.IsNotExist(err) {
		_ = os.Symlink(logFile, legacyLog) // ignore errors
	}

	// Log to both stdout and file
	mw := io.MultiWriter(os.Stdout, rotator)
	log.SetOutput(mw)
	gin.DefaultWriter = mw
	// Initialize a basic logger so CLI and early code can log.
	logger.Init(false, mw)

	// Handle CLI commands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			cfg, err := loadConfigForDatabase()
			if err != nil {
				log.Fatalf("load config: %v", err)
			}

			db, err := database.Connect(cfg.DatabasePath)
			if err != nil {
				log.Fatalf("connect database: %v", err)
			}

			logger.Log().Info("Running database migrations for all models...")
			if err := db.AutoMigrate(
				// Core models
				&models.ProxyGroup{},
				&models.ProxyHost{},
				&models.Location{},
				&models.CaddyConfig{},
				&models.RemoteServer{},
				&models.SSLCertificate{},
				&models.AccessList{},
				&models.SecurityHeaderProfile{},
				&models.User{},
				&models.Setting{},
				&models.ImportSession{},
				&models.Notification{},
				&models.NotificationProvider{},
				&models.NotificationTemplate{},
				&models.NotificationConfig{},
				&models.UptimeMonitor{},
				&models.UptimeHeartbeat{},
				&models.UptimeHost{},
				&models.UptimeNotificationEvent{},
				&models.Domain{},
				&models.UserPermittedHost{},
				// Security models
				&models.SecurityConfig{},
				&models.SecurityDecision{},
				&models.SecurityAudit{},
				&models.SecurityRuleSet{},
				&models.CrowdsecPresetEvent{},
				&models.CrowdsecConsoleEnrollment{},
				&models.EmergencyToken{}, // Phase 2: Database-backed emergency tokens
				// DNS Provider models (Issue #21)
				&models.DNSProvider{},
				&models.DNSProviderCredential{},
				// Plugin model (Phase 5)
				&models.Plugin{},
			); err != nil {
				log.Fatalf("migration failed: %v", err)
			}

			// Deferred composite index the uptime summary endpoint needs
			// (spec §3.5.6). At runtime the retention pruner builds this
			// prune-first, after trimming the table; the migrate CLI is an
			// operator-initiated maintenance window, so it builds unconditionally
			// against the full table with the cost made visible up front (S7).
			logger.Log().Warn("building index idx_heartbeat_monitor_created on uptime_heartbeats; " +
				"on a large database this can take several minutes and holds a write lock for the duration")
			if err := db.Exec(
				"CREATE INDEX IF NOT EXISTS idx_heartbeat_monitor_created ON uptime_heartbeats (monitor_id, created_at)",
			).Error; err != nil {
				log.Fatalf("migration failed: create idx_heartbeat_monitor_created: %v", err)
			}

			// The bare monitor_id index is a strict prefix of both composites; drop
			// it only now that the ordered composite exists. The drop is only
			// an optimisation, so a failure warns instead of aborting the migration.
			dropRedundantMonitorIndex(db)

			logger.Log().Info("Migration completed successfully")
			return

		case "reset-password":
			if len(os.Args) != 4 {
				log.Fatal("Usage: charon reset-password <email> <new-password>")
			}
			email := os.Args[2]
			newPassword := os.Args[3]

			cfg, err := loadConfigForDatabase()
			if err != nil {
				log.Fatalf("load config: %v", err)
			}

			db, err := database.Connect(cfg.DatabasePath)
			if err != nil {
				log.Fatalf("connect database: %v", err)
			}

			var user models.User
			if err := db.Where("email = ?", email).First(&user).Error; err != nil {
				log.Fatalf("user not found: %v", err)
			}

			if err := user.SetPassword(newPassword); err != nil {
				log.Fatalf("failed to hash password: %v", err)
			}

			// Unlock account if locked
			user.LockedUntil = nil
			user.FailedLoginAttempts = 0

			if err := db.Save(&user).Error; err != nil {
				log.Fatalf("failed to save user: %v", err)
			}

			logger.Log().Infof("Password updated successfully for user %s", email)
			return
		}
	}

	logger.Log().Infof("starting %s backend on version %s", version.Name, version.Full())

	cfg, err := loadConfigForDatabase()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Consume any durable pending-restore marker left by a prior
	// RestoreBackupSafe call whose live rehydrate could not complete (Issue
	// #32 spec §3.5). Must run before database.Connect so the swap happens
	// with no live WAL pool to corrupt. Deliberately not wired into the
	// migrate/reset-password CLI subcommands above — this is the
	// running-server startup path only.
	if pendingRestoreErr := database.ApplyPendingRestore(cfg.DatabasePath); pendingRestoreErr != nil {
		logger.Log().WithError(pendingRestoreErr).Error("Failed to apply pending database restore; continuing with existing database")
	}

	db, err := database.Connect(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}

	// Note: All database migrations are centralized in routes.Register()
	// This ensures migrations run exactly once and in the correct order.
	// DO NOT add AutoMigrate calls here - they cause "duplicate column" errors.

	// Reconcile CrowdSec state after migrations, before HTTP server starts
	// This ensures CrowdSec is running if user preference was to have it enabled
	crowdsecBinPath := os.Getenv("CHARON_CROWDSEC_BIN")
	if crowdsecBinPath == "" {
		crowdsecBinPath = "/usr/local/bin/crowdsec"
	}
	crowdsecDataDir := os.Getenv("CHARON_CROWDSEC_DATA")
	if crowdsecDataDir == "" {
		crowdsecDataDir = "/app/data/crowdsec"
	}

	crowdsecExec := handlers.NewDefaultCrowdsecExecutor()
	services.ReconcileCrowdSecOnStartup(db, crowdsecExec, crowdsecBinPath, crowdsecDataDir, nil)

	// Initialize plugin loader and load external DNS provider plugins (Phase 5)
	logger.Log().Info("Initializing DNS provider plugin system...")
	pluginDir := os.Getenv("CHARON_PLUGINS_DIR")
	if pluginDir == "" {
		pluginDir = "/app/plugins"
	}
	pluginLoader := services.NewPluginLoaderService(db, pluginDir, parsePluginSignatures())
	if pluginErr := pluginLoader.LoadAllPlugins(); pluginErr != nil {
		logger.Log().WithError(pluginErr).Warn("Failed to load external DNS provider plugins")
	}
	logger.Log().Info("Plugin system initialized")

	router := server.NewRouter(cfg.FrontendDir, filepath.Dir(cfg.DatabasePath), cfg.Security.TrustedProxies)
	// The database maintenance gate answers the healthcheck and the status
	// endpoint itself and returns 503 while a conversion holds the pool (GH #1422).
	gate := dbmaint.NewGate()
	// Initialize structured logger with same writer as stdlib log so both capture logs
	logger.Init(cfg.Debug, mw)
	logStartupWarnings(logger.Log(), cfg.StartupWarnings)
	// Request ID middleware must run before recovery so the recover logs include the request id
	router.Use(middleware.RequestID())
	// Log requests with request-scoped logger
	router.Use(middleware.RequestLogger())
	// Attach a recovery middleware that logs stack traces when debug is enabled
	router.Use(middleware.Recovery(cfg.Debug))
	// The gate goes before everything that touches the database (EmergencyBypass,
	// RateLimit), which RegisterWithDeps installs afterwards.
	router.Use(gate.Middleware(handlers.HealthHandler))

	// Shared Caddy manager and Cerberus instance for API + emergency server
	caddyClient := caddy.NewClient(cfg.CaddyAdminAPI)
	caddyManager := caddy.NewManager(caddyClient, db, cfg.CaddyConfigDir, cfg.FrontendDir, cfg.ACMEStaging, cfg.Security)
	cerb := cerberus.New(cfg.Security, db)

	// Pass config to routes for auth service and certificate service
	// Lifecycle context cancelled on shutdown to stop background goroutines
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	uptimeShutdown, err := routes.RegisterWithDeps(appCtx, router, db, cfg, caddyManager, cerb, gate)
	if err != nil {
		appCancel()
		log.Fatalf("register routes: %v", err) //nolint:gocritic // exitAfterDefer: appCancel called explicitly above
	}

	// Register import handler with config dependencies
	routes.RegisterImportHandler(router, db, cfg, cfg.CaddyBinary, cfg.ImportDir, cfg.ImportCaddyfile)

	// Check for mounted Caddyfile on startup
	if err := handlers.CheckMountedImport(db, cfg.ImportCaddyfile, cfg.CaddyBinary, cfg.ImportDir); err != nil {
		logger.Log().WithError(err).Warn("WARNING: failed to process mounted Caddyfile")
	}

	// Initialize emergency server (Tier 2 break glass)
	emergencyServer := server.NewEmergencyServerWithDeps(db, cfg.Emergency, caddyManager, cerb, gate)
	if err := emergencyServer.Start(); err != nil {
		logger.Log().WithError(err).Fatal("Failed to start emergency server")
	}

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Bind the main listener explicitly, after every startup step that uses the
	// database, so the maintenance gate can tell "listener bound" (the second of
	// its two readiness signals) from "about to bind".
	addr := fmt.Sprintf(":%s", cfg.HTTPPort)
	listener, listenErr := net.Listen("tcp", addr)
	if listenErr != nil {
		logger.Log().WithError(listenErr).Fatal("failed to listen")
	}
	gate.MarkListenerBound()

	// Start main HTTP server in goroutine
	go func() {
		logger.Log().Infof("starting %s backend on %s", version.Name, addr)

		if err := router.RunListener(listener); err != nil {
			logger.Log().WithError(err).Fatal("server error")
		}
	}()

	// Wait for interrupt signal
	sig := <-quit
	logger.Log().Infof("Received signal %v, initiating graceful shutdown...", sig)

	// Cancel the app-wide context to stop background goroutines (e.g. cert expiry checker)
	appCancel()

	// The maintenance runner is waited on first (bounded), then the ordered
	// uptime teardown (scheduler stops enqueuing → worker pool drains in-flight
	// checks → ingester final flush) so an in-flight check's heartbeat is not
	// lost on shutdown (spec §3.1.4 / S4).
	//
	// Grace is hardCap (20s) + margin. A worker that reaches shutdown mid-check
	// could in theory add the C1 notification dispatch's notifyTimeout (10s) on
	// top of hardCap, exceeding 25s — but it cannot here: appCancel() above has
	// already cancelled the pool's base ctx, so both the probe ctx and the
	// dispatch ctx are born already-done and unwind immediately. The only real
	// bound left is the HTTP client's own 20s timeout on a socket already
	// reading a slow body, which fits inside 25s.
	shutdownDrain(gate, dbmaint.ShutdownRunnerWait, func() {
		if uptimeShutdown == nil {
			return
		}
		drainCtx, drainCancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer drainCancel()
		if drainErr := uptimeShutdown(drainCtx); drainErr != nil {
			logger.Log().WithError(drainErr).Warn("uptime pipeline did not drain within grace period")
		}
	})

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stop emergency server
	if err := emergencyServer.Stop(ctx); err != nil {
		logger.Log().WithError(err).Error("Emergency server shutdown error")
	}

	logger.Log().Info("Server shutdown complete")
}

// dropRedundantMonitorIndex removes the legacy single-column monitor_id index.
// It is purely an optimisation, so a failure is logged and never fatal.
func dropRedundantMonitorIndex(db *gorm.DB) {
	if err := db.Exec(services.DropRedundantMonitorIndexSQL).Error; err != nil {
		logger.Log().WithError(err).Warn("migration: could not drop redundant idx_uptime_heartbeats_monitor_id; continuing")
	}
}

// logStartupWarnings logs every configuration warning collected by
// config.Load, once, after the structured logger is initialized.
func logStartupWarnings(entry *logrus.Entry, warnings []string) {
	for _, w := range warnings {
		entry.WithField("component", "config").Warn(w)
	}
}
