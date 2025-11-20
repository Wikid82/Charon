package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/api/handlers"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/api/routes"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/database"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/server"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/version"
)

func main() {
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

	// Pass config to routes for auth service and certificate service
	if err := routes.Register(router, db, cfg); err != nil {
		log.Fatalf("register routes: %v", err)
	}

	// Register import handler with config dependencies
	routes.RegisterImportHandler(router, db, cfg.CaddyBinary, cfg.ImportDir)

	// Check for mounted Caddyfile on startup
	if err := handlers.CheckMountedImport(db, cfg.ImportCaddyfile, cfg.CaddyBinary, cfg.ImportDir); err != nil {
		log.Printf("WARNING: failed to process mounted Caddyfile: %v", err)
	}

	addr := fmt.Sprintf(":%s", cfg.HTTPPort)
	log.Printf("starting %s backend on %s", version.Name, addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
