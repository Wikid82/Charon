package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config captures runtime configuration sourced from environment variables.
type Config struct {
	Environment  string
	HTTPPort     string
	DatabasePath string
	FrontendDir  string
}

// Load reads env vars and falls back to defaults so the server can boot with zero configuration.
func Load() (Config, error) {
	cfg := Config{
		Environment:  getEnv("CPM_ENV", "development"),
		HTTPPort:     getEnv("CPM_HTTP_PORT", "8080"),
		DatabasePath: getEnv("CPM_DB_PATH", filepath.Join("data", "cpm.db")),
		FrontendDir:  getEnv("CPM_FRONTEND_DIR", filepath.Clean(filepath.Join("..", "frontend", "dist"))),
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0o755); err != nil {
		return Config{}, fmt.Errorf("ensure data directory: %w", err)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	return fallback
}
