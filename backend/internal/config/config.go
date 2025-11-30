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
	JWTSecret       string
	ACMEStaging     bool
	Debug           bool
	Security        SecurityConfig
}

// SecurityConfig holds configuration for optional security services.
type SecurityConfig struct {
	CrowdSecMode    string
	CrowdSecAPIURL  string
	CrowdSecAPIKey  string
	WAFMode         string
	RateLimitMode   string
	ACLMode         string
	CerberusEnabled bool
}

// Load reads env vars and falls back to defaults so the server can boot with zero configuration.
func Load() (Config, error) {
	cfg := Config{
		Environment:     getEnvAny("development", "CHARON_ENV", "CPM_ENV"),
		HTTPPort:        getEnvAny("8080", "CHARON_HTTP_PORT", "CPM_HTTP_PORT"),
		DatabasePath:    getEnvAny(filepath.Join("data", "charon.db"), "CHARON_DB_PATH", "CPM_DB_PATH"),
		FrontendDir:     getEnvAny(filepath.Clean(filepath.Join("..", "frontend", "dist")), "CHARON_FRONTEND_DIR", "CPM_FRONTEND_DIR"),
		CaddyAdminAPI:   getEnvAny("http://localhost:2019", "CHARON_CADDY_ADMIN_API", "CPM_CADDY_ADMIN_API"),
		CaddyConfigDir:  getEnvAny(filepath.Join("data", "caddy"), "CHARON_CADDY_CONFIG_DIR", "CPM_CADDY_CONFIG_DIR"),
		CaddyBinary:     getEnvAny("caddy", "CHARON_CADDY_BINARY", "CPM_CADDY_BINARY"),
		ImportCaddyfile: getEnvAny("/import/Caddyfile", "CHARON_IMPORT_CADDYFILE", "CPM_IMPORT_CADDYFILE"),
		ImportDir:       getEnvAny(filepath.Join("data", "imports"), "CHARON_IMPORT_DIR", "CPM_IMPORT_DIR"),
		JWTSecret:       getEnvAny("change-me-in-production", "CHARON_JWT_SECRET", "CPM_JWT_SECRET"),
		ACMEStaging:     getEnvAny("", "CHARON_ACME_STAGING", "CPM_ACME_STAGING") == "true",
		Security: SecurityConfig{
			CrowdSecMode:    getEnvAny("disabled", "CERBERUS_SECURITY_CROWDSEC_MODE", "CHARON_SECURITY_CROWDSEC_MODE", "CPM_SECURITY_CROWDSEC_MODE"),
			CrowdSecAPIURL:  getEnvAny("", "CERBERUS_SECURITY_CROWDSEC_API_URL", "CHARON_SECURITY_CROWDSEC_API_URL", "CPM_SECURITY_CROWDSEC_API_URL"),
			CrowdSecAPIKey:  getEnvAny("", "CERBERUS_SECURITY_CROWDSEC_API_KEY", "CHARON_SECURITY_CROWDSEC_API_KEY", "CPM_SECURITY_CROWDSEC_API_KEY"),
			WAFMode:         getEnvAny("disabled", "CERBERUS_SECURITY_WAF_MODE", "CHARON_SECURITY_WAF_MODE", "CPM_SECURITY_WAF_MODE"),
			RateLimitMode:   getEnvAny("disabled", "CERBERUS_SECURITY_RATELIMIT_MODE", "CHARON_SECURITY_RATELIMIT_MODE", "CPM_SECURITY_RATELIMIT_MODE"),
			ACLMode:         getEnvAny("disabled", "CERBERUS_SECURITY_ACL_MODE", "CHARON_SECURITY_ACL_MODE", "CPM_SECURITY_ACL_MODE"),
			CerberusEnabled: getEnvAny("false", "CERBERUS_SECURITY_CERBERUS_ENABLED", "CHARON_SECURITY_CERBERUS_ENABLED", "CPM_SECURITY_CERBERUS_ENABLED") == "true",
		},
		Debug: getEnvAny("false", "CHARON_DEBUG", "CPM_DEBUG") == "true",
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

// NOTE: getEnv was removed in favor of getEnvAny since the latter supports
// checking multiple env var keys with a fallback value.

// getEnvAny checks a list of environment variable names in order and returns
// the first non-empty value. If none are set, it returns the provided fallback.
func getEnvAny(fallback string, keys ...string) string {
	for _, key := range keys {
		if val := os.Getenv(key); val != "" {
			return val
		}
	}
	return fallback
}
