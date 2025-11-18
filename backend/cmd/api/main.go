package main

import (
	"fmt"
	"log"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/api/handlers"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/api/routes"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/database"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/server"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/version"
)

func main() {
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

	if err := routes.Register(router, db); err != nil {
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
