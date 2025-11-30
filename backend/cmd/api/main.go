package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/api/routes"
	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/database"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/server"
	"github.com/Wikid82/charon/backend/internal/version"
	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"
)

func main() {
	// Setup logging with rotation
	logDir := "/app/data/logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		// Fallback to local directory if /app/data fails (e.g. local dev)
		logDir = "data/logs"
		_ = os.MkdirAll(logDir, 0755)
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

	// Handle CLI commands
	if len(os.Args) > 1 && os.Args[1] == "reset-password" {
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

		log.Printf("Password updated successfully for user %s", email)
		return
	}

	log.Printf("starting %s backend on version %s", version.Name, version.Full())

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := database.Connect(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}

	router := server.NewRouter(cfg.FrontendDir)
	// Initialize structured logger with same writer as stdlib log so both capture logs
	logger.Init(cfg.Debug, mw)
	// Request ID middleware must run before recovery so the recover logs include the request id
	router.Use(middleware.RequestID())
	// Log requests with request-scoped logger
	router.Use(middleware.RequestLogger())
	// Attach a recovery middleware that logs stack traces when debug is enabled
	router.Use(middleware.Recovery(cfg.Debug))

	// Pass config to routes for auth service and certificate service
	if err := routes.Register(router, db, cfg); err != nil {
		log.Fatalf("register routes: %v", err)
	}

	// Register import handler with config dependencies
	routes.RegisterImportHandler(router, db, cfg.CaddyBinary, cfg.ImportDir, cfg.ImportCaddyfile)

	// Check for mounted Caddyfile on startup
	if err := handlers.CheckMountedImport(db, cfg.ImportCaddyfile, cfg.CaddyBinary, cfg.ImportDir); err != nil {
		log.Printf("WARNING: failed to process mounted Caddyfile: %v", err)
	}

	addr := fmt.Sprintf(":%s", cfg.HTTPPort)
	logger.Log().Infof("starting %s backend on %s", version.Name, addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
