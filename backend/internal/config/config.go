// Package config handles configuration loading and validation.
package config

import (
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Wikid82/charon/backend/internal/security"
)

// Config captures runtime configuration sourced from environment variables.
type Config struct {
	Environment     string
	HTTPPort        string
	DatabasePath    string
	ConfigRoot      string
	FrontendDir     string
	CaddyAdminAPI   string
	CaddyConfigDir  string
	CaddyBinary     string
	ImportCaddyfile string
	ImportDir       string
	JWTSecret       string
	EncryptionKey   string
	ACMEStaging     bool
	SingleContainer bool
	PluginsDir      string
	CaddyLogDir     string
	CrowdSecLogDir  string
	Debug           bool
	Security        SecurityConfig
	Emergency       EmergencyConfig
}

// SecurityConfig holds configuration for optional security services.
type SecurityConfig struct {
	CrowdSecMode       string
	CrowdSecAPIURL     string
	CrowdSecAPIKey     string
	CrowdSecConfigDir  string
	WAFMode            string
	RateLimitMode      string
	RateLimitRequests  int
	RateLimitWindowSec int
	RateLimitBurst     int
	ACLMode            string
	CerberusEnabled    bool
	// ManagementCIDRs defines IP ranges allowed to use emergency break glass token
	// Default: RFC1918 private networks (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 127.0.0.0/8)
	ManagementCIDRs []string
}

// EmergencyConfig configures the emergency break glass server (Tier 2)
// This server provides a separate entry point for emergency recovery when
// the main application is blocked by security middleware (Caddy/CrowdSec/ACL).
type EmergencyConfig struct {
	// Enabled controls whether the emergency server starts
	Enabled bool `env:"CHARON_EMERGENCY_SERVER_ENABLED" envDefault:"false"`

	// BindAddress is the address to bind the emergency server to
	// Default: 127.0.0.1:2020 (localhost IPv4 only for security)
	// Note: Port 2020 avoids conflict with Caddy admin API (port 2019)
	//
	// IPv4/IPv6 Binding Options:
	//   - "127.0.0.1:2020"  → IPv4 localhost only (most secure, default)
	//   - "[::1]:2020"      → IPv6 localhost only
	//   - "0.0.0.0:2020"    → All IPv4 interfaces (dual-stack on capable systems)
	//   - "[::]:2020"       → All IPv6 interfaces (dual-stack on capable systems)
	//   - ":2020"          → All interfaces (IPv4/IPv6 based on system config)
	//
	// Production: Should be accessible only via VPN/SSH tunnel
	BindAddress string `env:"CHARON_EMERGENCY_BIND" envDefault:"127.0.0.1:2020"`

	// BasicAuthUsername for emergency server authentication
	// If empty, NO authentication is enforced (not recommended)
	BasicAuthUsername string `env:"CHARON_EMERGENCY_USERNAME" envDefault:""`

	// BasicAuthPassword for emergency server authentication
	// If empty, NO authentication is enforced (not recommended)
	BasicAuthPassword string `env:"CHARON_EMERGENCY_PASSWORD" envDefault:""`
}

// Load reads env vars and falls back to defaults so the server can boot with zero configuration.
func Load() (Config, error) {
	cfg := Config{
		Environment:     getEnvAny("development", "CHARON_ENV", "CPM_ENV"),
		HTTPPort:        getEnvAny("8080", "CHARON_HTTP_PORT", "CPM_HTTP_PORT"),
		DatabasePath:    getEnvAny(filepath.Join("data", "charon.db"), "CHARON_DB_PATH", "CPM_DB_PATH"),
		ConfigRoot:      getEnvAny("/config", "CHARON_CADDY_CONFIG_ROOT"),
		FrontendDir:     getEnvAny(filepath.Clean(filepath.Join("..", "frontend", "dist")), "CHARON_FRONTEND_DIR", "CPM_FRONTEND_DIR"),
		CaddyAdminAPI:   getEnvAny("http://localhost:2019", "CHARON_CADDY_ADMIN_API", "CPM_CADDY_ADMIN_API"),
		CaddyConfigDir:  getEnvAny(filepath.Join("data", "caddy"), "CHARON_CADDY_CONFIG_DIR", "CPM_CADDY_CONFIG_DIR"),
		CaddyBinary:     getEnvAny("caddy", "CHARON_CADDY_BINARY", "CPM_CADDY_BINARY"),
		ImportCaddyfile: getEnvAny("/import/Caddyfile", "CHARON_IMPORT_CADDYFILE", "CPM_IMPORT_CADDYFILE"),
		ImportDir:       getEnvAny(filepath.Join("data", "imports"), "CHARON_IMPORT_DIR", "CPM_IMPORT_DIR"),
		EncryptionKey:   getEnvAny("", "CHARON_ENCRYPTION_KEY"),
		ACMEStaging:     getEnvAny("", "CHARON_ACME_STAGING", "CPM_ACME_STAGING") == "true",
		SingleContainer: strings.EqualFold(getEnvAny("true", "CHARON_SINGLE_CONTAINER_MODE"), "true"),
		PluginsDir:      getEnvAny("/app/plugins", "CHARON_PLUGINS_DIR"),
		CaddyLogDir:     getEnvAny("/var/log/caddy", "CHARON_CADDY_LOG_DIR"),
		CrowdSecLogDir:  getEnvAny("/var/log/crowdsec", "CHARON_CROWDSEC_LOG_DIR"),
		Security:        loadSecurityConfig(),
		Emergency:       loadEmergencyConfig(),
		Debug:           getEnvAny("false", "CHARON_DEBUG", "CPM_DEBUG") == "true",
	}

	// Set JWTSecret using os.Getenv directly so no string literal flows into the
	// field — prevents CodeQL go/parse-jwt-with-hardcoded-key taint from any fallback.
	cfg.JWTSecret = os.Getenv("CHARON_JWT_SECRET")
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = os.Getenv("CPM_JWT_SECRET")
	}

	allowedInternalHosts := security.InternalServiceHostAllowlist()
	normalizedCaddyAdminURL, err := security.ValidateInternalServiceBaseURL(
		cfg.CaddyAdminAPI,
		2019,
		allowedInternalHosts,
	)
	if err != nil {
		return Config{}, fmt.Errorf("validate caddy admin api url: %w", err)
	}
	cfg.CaddyAdminAPI = normalizedCaddyAdminURL.String()

	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0o700); err != nil {
		return Config{}, fmt.Errorf("ensure data directory: %w", err)
	}

	if err := os.MkdirAll(cfg.CaddyConfigDir, 0o700); err != nil {
		return Config{}, fmt.Errorf("ensure caddy config directory: %w", err)
	}

	if err := os.MkdirAll(cfg.ImportDir, 0o700); err != nil {
		return Config{}, fmt.Errorf("ensure import directory: %w", err)
	}

	if cfg.JWTSecret == "" {
		b := make([]byte, 32)
		if _, err := crand.Read(b); err != nil {
			return Config{}, fmt.Errorf("generate fallback jwt secret: %w", err)
		}
		cfg.JWTSecret = hex.EncodeToString(b)
	}

	return cfg, nil
}

// loadSecurityConfig loads the security configuration with proper parsing of array fields
func loadSecurityConfig() SecurityConfig {
	cfg := SecurityConfig{
		CrowdSecMode:       getEnvAny("disabled", "CERBERUS_SECURITY_CROWDSEC_MODE", "CHARON_SECURITY_CROWDSEC_MODE", "CPM_SECURITY_CROWDSEC_MODE"),
		CrowdSecAPIURL:     getEnvAny("", "CERBERUS_SECURITY_CROWDSEC_API_URL", "CHARON_SECURITY_CROWDSEC_API_URL", "CPM_SECURITY_CROWDSEC_API_URL"),
		CrowdSecAPIKey:     getEnvAny("", "CERBERUS_SECURITY_CROWDSEC_API_KEY", "CHARON_SECURITY_CROWDSEC_API_KEY", "CPM_SECURITY_CROWDSEC_API_KEY"),
		CrowdSecConfigDir:  getEnvAny(filepath.Join("data", "crowdsec"), "CHARON_CROWDSEC_CONFIG_DIR", "CPM_CROWDSEC_CONFIG_DIR"),
		WAFMode:            getEnvAny("disabled", "CERBERUS_SECURITY_WAF_MODE", "CHARON_SECURITY_WAF_MODE", "CPM_SECURITY_WAF_MODE"),
		RateLimitMode:      getEnvAny("disabled", "CERBERUS_SECURITY_RATELIMIT_MODE", "CHARON_SECURITY_RATELIMIT_MODE", "CPM_SECURITY_RATELIMIT_MODE"),
		RateLimitRequests:  getEnvIntAny(100, "CERBERUS_SECURITY_RATELIMIT_REQUESTS", "CHARON_SECURITY_RATELIMIT_REQUESTS"),
		RateLimitWindowSec: getEnvIntAny(60, "CERBERUS_SECURITY_RATELIMIT_WINDOW", "CHARON_SECURITY_RATELIMIT_WINDOW"),
		RateLimitBurst:     getEnvIntAny(20, "CERBERUS_SECURITY_RATELIMIT_BURST", "CHARON_SECURITY_RATELIMIT_BURST"),
		ACLMode:            getEnvAny("disabled", "CERBERUS_SECURITY_ACL_MODE", "CHARON_SECURITY_ACL_MODE", "CPM_SECURITY_ACL_MODE"),
		CerberusEnabled:    getEnvAny("true", "CERBERUS_SECURITY_CERBERUS_ENABLED", "CHARON_SECURITY_CERBERUS_ENABLED", "CPM_SECURITY_CERBERUS_ENABLED") != "false",
	}

	// Parse management CIDRs (comma-separated list)
	managementCIDRsStr := getEnvAny("", "CHARON_MANAGEMENT_CIDRS")
	if managementCIDRsStr != "" {
		// Split by comma and trim spaces
		for _, cidr := range splitAndTrim(managementCIDRsStr, ",") {
			if cidr != "" {
				cfg.ManagementCIDRs = append(cfg.ManagementCIDRs, cidr)
			}
		}
	}

	return cfg
}

// loadEmergencyConfig loads the emergency server configuration
func loadEmergencyConfig() EmergencyConfig {
	return EmergencyConfig{
		Enabled:           getEnvAny("false", "CHARON_EMERGENCY_SERVER_ENABLED") == "true",
		BindAddress:       getEnvAny("127.0.0.1:2020", "CHARON_EMERGENCY_BIND"),
		BasicAuthUsername: getEnvAny("", "CHARON_EMERGENCY_USERNAME"),
		BasicAuthPassword: getEnvAny("", "CHARON_EMERGENCY_PASSWORD"),
	}
}

// splitAndTrim splits a string by separator and trims each part
func splitAndTrim(s, sep string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, sep)
	result := []string{}
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
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

// getEnvIntAny checks a list of environment variable names, attempts to parse as int.
// Returns first successfully parsed value. Returns fallback if none found or parsing failed.
func getEnvIntAny(fallback int, keys ...string) int {
	valStr := getEnvAny("", keys...)
	if valStr == "" {
		return fallback
	}
	if val, err := strconv.Atoi(valStr); err == nil {
		return val
	}
	return fallback
}
