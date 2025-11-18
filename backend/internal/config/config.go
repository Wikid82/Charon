package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config captures runtime configuration sourced from environment variables.
type Config struct {
	Environment     string
	HTTPPort        string
	DatabasePath    string
	FrontendDir     string
	CaddyAdminAPI   string
	CaddyConfigDir  string
	CaddyBinary     string
	ImportCaddyfile string
	ImportDir       string
}

// Load reads env vars and falls back to defaults so the server can boot with zero configuration.
func Load() (Config, error) {
	cfg := Config{
		Environment:     getEnv("CPM_ENV", "development"),
		HTTPPort:        getEnv("CPM_HTTP_PORT", "8080"),
		DatabasePath:    getEnv("CPM_DB_PATH", filepath.Join("data", "cpm.db")),
		FrontendDir:     getEnv("CPM_FRONTEND_DIR", filepath.Clean(filepath.Join("..", "frontend", "dist"))),
		CaddyAdminAPI:   getEnv("CPM_CADDY_ADMIN_API", "http://localhost:2019"),
		CaddyConfigDir:  getEnv("CPM_CADDY_CONFIG_DIR", filepath.Join("data", "caddy")),
		CaddyBinary:     getEnv("CPM_CADDY_BINARY", "caddy"),
		ImportCaddyfile: getEnv("CPM_IMPORT_CADDYFILE", "/import/Caddyfile"),
		ImportDir:       getEnv("CPM_IMPORT_DIR", filepath.Join("data", "imports")),
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0o755); err != nil {
		return Config{}, fmt.Errorf("ensure data directory: %w", err)
	}

	if err := os.MkdirAll(cfg.CaddyConfigDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("ensure caddy config directory: %w", err)
	}

	if err := os.MkdirAll(cfg.ImportDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("ensure import directory: %w", err)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	return fallback
}
