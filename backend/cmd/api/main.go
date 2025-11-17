package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/database"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/server"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/version"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}

	srv, err := server.New(db, cfg)
	if err != nil {
		log.Fatalf("bootstrap server: %v", err)
	}

	log.Printf("starting %s backend on :%s", version.Name, cfg.HTTPPort)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server exited with error: %v", err)
	}
}
