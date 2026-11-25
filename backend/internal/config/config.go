// Package config handles configuration loading and validation.
package config

import (
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
	"github.com/Wikid82/charon/backend/internal/logger"
)

// Accepted values of CHARON_DB_COMPACT_ON_START.
const (
	DBCompactAuto = "auto"
	DBCompactOff  = "off"
)

// CaddyAdminAPIForTests, when non-empty, replaces the admin URL that Load returns.
// Only the test helpers that run main in a child process set it. Nothing shipped
// sets it, and no environment variable reaches it.
var CaddyAdminAPIForTests string

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
	// SetupToken is CHARON_SETUP_TOKEN: an operator-chosen first-run setup code
	// for automation. Empty means the setup code is generated at startup.
	SetupToken string
	// EncryptionKey is the current key, the same value as EncryptionKeys.Current.
	// It is kept for callers that only need the current key.
	EncryptionKey string
	// EncryptionKeys holds every configured encryption key (current, next and legacy slots).
	EncryptionKeys  EncryptionKeys
	ACMEStaging     bool
	SingleContainer bool
	PluginsDir      string
	// PluginSignatures is the CHARON_PLUGIN_SIGNATURES allowlist (plugin file
	// name -> "sha256:<hex>"). nil means permissive (variable unset); a non-nil
	// map is strict, and an empty map allows no external plugin. Start-up,
	// reload and enable all use this one list.
	PluginSignatures map[string]string
	// PluginCacheDir is CHARON_PLUGIN_CACHE_DIR: the private folder that holds verified copies of plugins that are
	// loaded. It defaults to plugin-cache inside the data folder. Set it to a folder that allows running program files
	// when the data folder is mounted noexec.
	PluginCacheDir        string
	CaddyLogDir           string
	CrowdSecLogDir        string
	Debug                 bool
	CertExpiryWarningDays int
	// DBCompactOnStart is CHARON_DB_COMPACT_ON_START: DBCompactAuto (default) or
	// DBCompactOff, which disables boot-time database optimization (GH #1422).
	DBCompactOnStart string
	Security         SecurityConfig
	Emergency        EmergencyConfig
	// StartupWarnings collects configuration problems found by Load. They are
	// logged at WARN once the logger is initialized.
	StartupWarnings []string
}

// SecurityConfig holds configuration for optional security services.
type SecurityConfig struct {
	CrowdSecMode       string
	CrowdSecAPIURL     string // ignored: CrowdSec readers always use the local engine address
	CrowdSecAPIKey     string
	CrowdSecConfigDir  string
	WAFMode            string
	RateLimitMode      string
	RateLimitRequests  int
	RateLimitWindowSec int
	RateLimitBurst     int
	ACLMode            string
	CerberusEnabled    bool
	// ManagementCIDRs defines IP ranges allowed to use emergency break glass token.
	// Entries are CIDRs or bare addresses; validation happens in middleware.ParseManagementNets
	// (config cannot import middleware). Default when unset: RFC1918 private networks
	// (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16) plus loopback.
	ManagementCIDRs []string
	// TrustedProxies lists IPs/CIDRs of reverse proxies whose
	// X-Forwarded-Proto/X-Forwarded-Host headers are honored for TLS-
	// termination detection (setSecureCookie) and whose presence enables
	// Gin's own ClientIP() X-Forwarded-For resolution. Empty (default)
	// trusts nothing — forwarded headers are ignored entirely and every
	// trust decision falls back to the raw TCP peer address
	// (c.Request.RemoteAddr), matching Gin's own SetTrustedProxies(nil)
	// default. See docs/configuration/trusted-proxies.md.
	TrustedProxies []string
	// AuthRateLimit configures the always-on per-client sign-in throttle.
	AuthRateLimit AuthRateLimitConfig
}

// EmergencyConfig configures the emergency break glass server (Tier 2)
// This server provides a separate entry point for emergency recovery when
// the main application is blocked by security middleware (Caddy/CrowdSec/ACL).
type EmergencyConfig struct {
	// Enabled controls whether the emergency server starts
	Enabled bool `env:"CHARON_EMERGENCY_SERVER_ENABLED" envDefault:"false"`

	// BindAddress is the address to bind the emergency server to
	// Default: 127.0.0.1:2020 (localhost IPv4 only for security)
	// Note: Port 2020 is the emergency server; the Caddy admin API is a unix socket
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

// EncryptionKeys holds the base64 encryption keys read from the environment.
// Current comes from CHARON_ENCRYPTION_KEY, Next from CHARON_ENCRYPTION_KEY_NEXT
// and Legacy from CHARON_ENCRYPTION_KEY_V1 through CHARON_ENCRYPTION_KEY_V10.
type EncryptionKeys struct {
	Current string
	Next    string
	// Legacy maps a slot number (1-10) to its key. Blank slots are absent.
	Legacy map[int]string
}

// maxLegacyEncryptionKeySlot is the highest CHARON_ENCRYPTION_KEY_V<n> slot.
const maxLegacyEncryptionKeySlot = 10

// LoadEncryptionKeys reads the encryption key variables from the environment.
// Only the CHARON_ names are read. Blank values are treated as unset.
func LoadEncryptionKeys() EncryptionKeys {
	keys := EncryptionKeys{
		Current: getEnvAny("", "CHARON_ENCRYPTION_KEY"),
		Next:    getEnvAny("", "CHARON_ENCRYPTION_KEY_NEXT"),
		Legacy:  make(map[int]string),
	}
	for slot := 1; slot <= maxLegacyEncryptionKeySlot; slot++ {
		if value := getEnvAny("", fmt.Sprintf("CHARON_ENCRYPTION_KEY_V%d", slot)); value != "" {
			keys.Legacy[slot] = value
		}
	}
	return keys
}

// Load reads env vars and falls back to defaults so the server can boot with zero configuration.
func Load() (Config, error) {
	encryptionKeys := LoadEncryptionKeys()
	cfg := Config{
		Environment:     getEnvAny("development", "CHARON_ENV", "CPM_ENV"),
		HTTPPort:        getEnvAny("8080", "CHARON_HTTP_PORT", "CPM_HTTP_PORT"),
		DatabasePath:    getEnvAny(filepath.Join("data", "charon.db"), "CHARON_DB_PATH", "CPM_DB_PATH"),
		ConfigRoot:      getEnvAny("/config", "CHARON_CADDY_CONFIG_ROOT"),
		FrontendDir:     getEnvAny(filepath.Clean(filepath.Join("..", "frontend", "dist")), "CHARON_FRONTEND_DIR", "CPM_FRONTEND_DIR"),
		CaddyAdminAPI:   getEnvAny(DefaultCaddyAdminAPI(), "CHARON_CADDY_ADMIN_API", "CPM_CADDY_ADMIN_API"),
		CaddyConfigDir:  getEnvAny(filepath.Join("data", "caddy"), "CHARON_CADDY_CONFIG_DIR", "CPM_CADDY_CONFIG_DIR"),
		CaddyBinary:     getEnvAny("caddy", "CHARON_CADDY_BINARY", "CPM_CADDY_BINARY"),
		ImportCaddyfile: getEnvAny("/import/Caddyfile", "CHARON_IMPORT_CADDYFILE", "CPM_IMPORT_CADDYFILE"),
		ImportDir:       getEnvAny(filepath.Join("data", "imports"), "CHARON_IMPORT_DIR", "CPM_IMPORT_DIR"),
		EncryptionKey:   encryptionKeys.Current,
		EncryptionKeys:  encryptionKeys,
		ACMEStaging:     getEnvAny("", "CHARON_ACME_STAGING", "CPM_ACME_STAGING") == "true",
		SingleContainer: strings.EqualFold(getEnvAny("true", "CHARON_SINGLE_CONTAINER_MODE"), "true"),
		PluginsDir:      getEnvAny("/app/plugins", "CHARON_PLUGINS_DIR"),
		CaddyLogDir:     getEnvAny("/var/log/caddy", "CHARON_CADDY_LOG_DIR"),
		CrowdSecLogDir:  getEnvAny("/var/log/crowdsec", "CHARON_CROWDSEC_LOG_DIR"),
		Emergency:       loadEmergencyConfig(),
		Debug:           getEnvAny("false", "CHARON_DEBUG", "CPM_DEBUG") == "true",
	}

	securityCfg, securityWarnings := loadSecurityConfig()
	cfg.Security = securityCfg
	cfg.StartupWarnings = append(cfg.StartupWarnings, securityWarnings...)

	trustedProxies, proxyWarnings := ValidateTrustedProxies(cfg.Security.TrustedProxies)
	cfg.Security.TrustedProxies = trustedProxies
	cfg.StartupWarnings = append(cfg.StartupWarnings, proxyWarnings...)

	rawAuthRateLimit, authWarnings := loadAuthRateLimitConfig()
	authRateLimit, authNormWarnings := rawAuthRateLimit.Normalize()
	cfg.Security.AuthRateLimit = authRateLimit
	cfg.StartupWarnings = append(cfg.StartupWarnings, authWarnings...)
	cfg.StartupWarnings = append(cfg.StartupWarnings, authNormWarnings...)

	dbCompact, dbCompactWarnings := loadDBCompactOnStart()
	cfg.DBCompactOnStart = dbCompact
	cfg.StartupWarnings = append(cfg.StartupWarnings, dbCompactWarnings...)

	cfg.PluginSignatures = loadPluginSignatures()
	cfg.PluginCacheDir = getEnvAny(filepath.Join(filepath.Dir(cfg.DatabasePath), "plugin-cache"), "CHARON_PLUGIN_CACHE_DIR")

	cfg.CertExpiryWarningDays = 30
	if days := getEnvAny("", "CHARON_CERT_EXPIRY_WARNING_DAYS"); days != "" {
		if n, err := strconv.Atoi(days); err == nil && n > 0 {
			cfg.CertExpiryWarningDays = n
		}
	}

	// Set JWTSecret using os.Getenv directly so no string literal flows into the
	// field — prevents CodeQL go/parse-jwt-with-hardcoded-key taint from any fallback.
	cfg.JWTSecret = os.Getenv("CHARON_JWT_SECRET")
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = os.Getenv("CPM_JWT_SECRET")
	}

	// Secret files and .env lines often end with a newline or padding; trim it so the
	// token matches what the operator sees in the setup form.
	cfg.SetupToken = strings.TrimSpace(getEnvAny("", "CHARON_SETUP_TOKEN"))

	// The test seam replaces the whole setting, so the value it replaces is not validated.
	// Otherwise a long working directory could fail the default socket path in a test that never uses it.
	if CaddyAdminAPIForTests != "" {
		cfg.CaddyAdminAPI = CaddyAdminAPIForTests
	} else {
		endpoint, legacy, err := resolveCaddyAdminAPI(cfg.CaddyAdminAPI)
		if err != nil {
			return Config{}, fmt.Errorf("validate caddy admin api url: %w", err)
		}
		cfg.CaddyAdminAPI = endpoint
		if legacy {
			cfg.StartupWarnings = append(cfg.StartupWarnings, LegacyAdminURLWarning(endpoint))
		}
	}

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

// loadDBCompactOnStart parses CHARON_DB_COMPACT_ON_START (auto|off). Any other
// value, including the removed "force", falls back to auto with a warning.
func loadDBCompactOnStart() (mode string, warnings []string) {
	raw := strings.ToLower(strings.TrimSpace(getEnvAny(DBCompactAuto, "CHARON_DB_COMPACT_ON_START")))
	switch raw {
	case DBCompactAuto, DBCompactOff:
		return raw, nil
	default:
		return DBCompactAuto, []string{fmt.Sprintf(
			"CHARON_DB_COMPACT_ON_START=%q is not valid (expected auto or off); using auto", raw)}
	}
}

// crowdSecDataAliasEnv is the legacy name for the CrowdSec data directory. It is an alias of
// CHARON_CROWDSEC_CONFIG_DIR; the config-dir variable wins when both are set and differ.
const crowdSecDataAliasEnv = "CHARON_CROWDSEC_DATA"

// loadCrowdSecConfigDir returns the CrowdSec data directory shared by the handlers, boot reconciliation and the
// proxy key file. CHARON_CROWDSEC_CONFIG_DIR (or CPM_CROWDSEC_CONFIG_DIR) wins; CHARON_CROWDSEC_DATA is used
// only when that is unset, and a warning names the conflict when the two differ.
func loadCrowdSecConfigDir() (dir string, warnings []string) {
	configDir := getEnvAny("", "CHARON_CROWDSEC_CONFIG_DIR", "CPM_CROWDSEC_CONFIG_DIR")
	alias := getEnvAny("", crowdSecDataAliasEnv)
	switch {
	case configDir == "" && alias != "":
		return alias, nil
	case configDir == "":
		return filepath.Join("data", "crowdsec"), nil
	case alias != "" && alias != configDir:
		return configDir, []string{fmt.Sprintf(
			"%s=%q is ignored because CHARON_CROWDSEC_CONFIG_DIR=%q is set; keep only CHARON_CROWDSEC_CONFIG_DIR", crowdSecDataAliasEnv, alias, configDir)}
	default:
		return configDir, nil
	}
}

// loadSecurityConfig loads the security configuration with proper parsing of array fields and returns the
// startup warnings found while reading it.
func loadSecurityConfig() (settings SecurityConfig, warnings []string) {
	crowdSecDir, crowdSecWarnings := loadCrowdSecConfigDir()
	cfg := SecurityConfig{
		CrowdSecMode:       getEnvAny("disabled", "CERBERUS_SECURITY_CROWDSEC_MODE", "CHARON_SECURITY_CROWDSEC_MODE", "CPM_SECURITY_CROWDSEC_MODE"),
		CrowdSecAPIURL:     getEnvAny("", "CERBERUS_SECURITY_CROWDSEC_API_URL", "CHARON_SECURITY_CROWDSEC_API_URL", "CPM_SECURITY_CROWDSEC_API_URL"),
		CrowdSecAPIKey:     getEnvAny("", bouncerkey.EnvNames()...),
		CrowdSecConfigDir:  crowdSecDir,
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

	// Parse trusted proxies (comma-separated list)
	trustedProxiesStr := getEnvAny("", "CHARON_TRUSTED_PROXIES")
	if trustedProxiesStr != "" {
		for _, cidr := range splitAndTrim(trustedProxiesStr, ",") {
			if cidr != "" {
				cfg.TrustedProxies = append(cfg.TrustedProxies, cidr)
			}
		}
	}

	return cfg, crowdSecWarnings
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

// loadPluginSignatures reads CHARON_PLUGIN_SIGNATURES and logs the resulting mode.
// A malformed value fails closed: it returns an empty, non-nil allowlist, so no
// external plugin loads until the value is fixed.
func loadPluginSignatures() map[string]string {
	raw := os.Getenv("CHARON_PLUGIN_SIGNATURES")
	signatures, err := parsePluginSignatures(raw)
	if err != nil {
		logger.Log().WithError(err).Error("CHARON_PLUGIN_SIGNATURES is not valid; no external plugins are loaded until it is fixed")
		return map[string]string{}
	}
	switch {
	case signatures == nil:
		logger.Log().Info("Plugin signature verification: PERMISSIVE mode (CHARON_PLUGIN_SIGNATURES not set)")
	case len(signatures) == 0:
		logger.Log().Info("Plugin signature verification: STRICT mode (empty allowlist — no external plugins permitted)")
	default:
		logger.Log().Infof("Plugin signature verification: STRICT mode (%d plugin(s) in allowlist)", len(signatures))
	}
	return signatures
}

// parsePluginSignatures parses the CHARON_PLUGIN_SIGNATURES JSON object.
// An empty raw value returns nil (permissive). Any other valid JSON, including
// null, returns a non-nil map so that the list can never fall back to permissive.
func parsePluginSignatures(raw string) (map[string]string, error) {
	if raw == "" {
		return nil, nil
	}
	var signatures map[string]string
	if err := json.Unmarshal([]byte(raw), &signatures); err != nil {
		return nil, fmt.Errorf("parse plugin signatures JSON: %w", err)
	}
	if signatures == nil {
		signatures = map[string]string{}
	}
	for name, sig := range signatures {
		if !strings.HasPrefix(sig, "sha256:") {
			return nil, fmt.Errorf("signature for plugin %q must begin with \"sha256:\"", name)
		}
	}
	return signatures, nil
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
