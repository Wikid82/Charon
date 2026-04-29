// Package main is the entry point for the Charon backend API.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/server"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/version"
	_ "github.com/Wikid82/charon/backend/pkg/dnsprovider/builtin" // Register built-in DNS providers
	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"
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
			cfg, err := config.Load()
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

			logger.Log().Info("Migration completed successfully")
			return

		case "reset-password":
			if len(os.Args) != 4 {
				log.Fatalf("Usage: %s reset-password <email> <new-password>", os.Args[0])
			}
			email := os.Args[2]
			newPassword := os.Args[3]

			cfg, err := config.Load()
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

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
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
	if err := pluginLoader.LoadAllPlugins(); err != nil {
		logger.Log().WithError(err).Warn("Failed to load external DNS provider plugins")
	}
	logger.Log().Info("Plugin system initialized")

	router := server.NewRouter(cfg.FrontendDir)
	// Initialize structured logger with same writer as stdlib log so both capture logs
	logger.Init(cfg.Debug, mw)
	// Request ID middleware must run before recovery so the recover logs include the request id
	router.Use(middleware.RequestID())
	// Log requests with request-scoped logger
	router.Use(middleware.RequestLogger())
	// Attach a recovery middleware that logs stack traces when debug is enabled
	router.Use(middleware.Recovery(cfg.Debug))

	// Shared Caddy manager and Cerberus instance for API + emergency server
	caddyClient := caddy.NewClient(cfg.CaddyAdminAPI)
	caddyManager := caddy.NewManager(caddyClient, db, cfg.CaddyConfigDir, cfg.FrontendDir, cfg.ACMEStaging, cfg.Security)
	cerb := cerberus.New(cfg.Security, db)

	// Pass config to routes for auth service and certificate service
	// Lifecycle context cancelled on shutdown to stop background goroutines
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	if err := routes.RegisterWithDeps(appCtx, router, db, cfg, caddyManager, cerb); err != nil {
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
	emergencyServer := server.NewEmergencyServerWithDeps(db, cfg.Emergency, caddyManager, cerb)
	if err := emergencyServer.Start(); err != nil {
		logger.Log().WithError(err).Fatal("Failed to start emergency server")
	}

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start main HTTP server in goroutine
	go func() {
		addr := fmt.Sprintf(":%s", cfg.HTTPPort)
		logger.Log().Infof("starting %s backend on %s", version.Name, addr)

		if err := router.Run(addr); err != nil {
			logger.Log().WithError(err).Fatal("server error")
		}
	}()

	// Wait for interrupt signal
	sig := <-quit
	logger.Log().Infof("Received signal %v, initiating graceful shutdown...", sig)

	// Cancel the app-wide context to stop background goroutines (e.g. cert expiry checker)
	appCancel()

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stop emergency server
	if err := emergencyServer.Stop(ctx); err != nil {
		logger.Log().WithError(err).Error("Emergency server shutdown error")
	}

	logger.Log().Info("Server shutdown complete")
}
