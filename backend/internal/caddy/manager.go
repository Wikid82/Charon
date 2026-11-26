package caddy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/redact"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/security/selfhop"
)

// ErrRollbackFailed is wrapped into the ApplyConfig error when Caddy keeps its previous configuration after a failed
// load and the rollback to the last good snapshot fails too.
var ErrRollbackFailed = errors.New("rollback also failed")

// Test hooks to allow overriding OS and JSON functions
var (
	writeFileFunc        = os.WriteFile
	readFileFunc         = os.ReadFile
	removeFileFunc       = os.Remove
	readDirFunc          = os.ReadDir
	statFunc             = os.Stat
	jsonMarshalFunc      = json.MarshalIndent
	jsonMarshalDebugFunc = json.Marshal // For debug logging, separate hook for testing
	// Test hooks for bandaging validation/generation flows
	generateConfigFunc = GenerateConfig
	validateConfigFunc = Validate
)

const (
	minKeepaliveIdleDuration  = time.Second
	maxKeepaliveIdleDuration  = 24 * time.Hour
	minKeepaliveCount         = 1
	maxKeepaliveCount         = 100
	settingCaddyKeepaliveIdle = "caddy.keepalive_idle"
	settingCaddyKeepaliveCnt  = "caddy.keepalive_count"
)

// DNSProviderConfig contains a DNS provider with its decrypted credentials
// for use in Caddy DNS challenge configuration generation
type DNSProviderConfig struct {
	ID                 uint
	ProviderType       string
	PropagationTimeout int

	// Single-credential mode: Use these credentials for all domains
	Credentials map[string]string

	// Multi-credential mode: Use zone-specific credentials
	UseMultiCredentials bool
	ZoneCredentials     map[string]map[string]string // map[baseDomain]credentials
}

// CaddyClient defines the interface for interacting with Caddy Admin API
type CaddyClient interface {
	Load(ctx context.Context, config *Config) error
	Ping(ctx context.Context) error
}

// Manager orchestrates Caddy configuration lifecycle: generate, validate, apply, rollback.
type Manager struct {
	mu          sync.Mutex
	client      CaddyClient
	db          *gorm.DB
	configDir   string
	frontendDir string
	acmeStaging bool
	securityCfg config.SecurityConfig
	secrets     crypto.VersionedCipher
	// forwardGuard refuses stored forward targets that reach an agent listener. It is checked before any host is generated.
	forwardGuard ForwardTargetGuard
	hopSecret    *selfhop.Secret
	hopPort      string
	// adminListen is the admin.listen value written into every generated config. It matches the endpoint the client dials.
	adminListen string
	// geoIPState reports the GeoIP database path and whether the proxy may use it. It is read at every apply.
	geoIPState func() (path string, available bool)
	// enforcement is the report of the last successful apply. It is nil while an apply runs and after any failed one.
	enforcement atomic.Pointer[EnforcementReport]
	// lastApplied is the report of the last successful apply. Unlike enforcement it is not cleared when an apply starts
	// or fails, because on every failure Caddy keeps running that configuration.
	lastApplied atomic.Pointer[EnforcementReport]
	// leftOut holds the UUIDs of the enabled proxy hosts the last successful apply left out of the configuration.
	leftOut atomic.Pointer[map[string]struct{}]
	// upstreamResolver is the loopback answerer every reverse_proxy transport is pointed at when a config is loaded.
	// Nil when the answerer is not running; the config is then loaded unchanged.
	upstreamResolver atomic.Pointer[UpstreamResolver]
}

// EnforcementReport returns the enforcement report of the last successful apply. ok is false when no successful apply
// has completed since the last apply attempt started. It reads memory only, so status requests do not wait for an apply.
func (m *Manager) EnforcementReport() (report EnforcementReport, ok bool) {
	return loadReport(&m.enforcement)
}

// LastAppliedReport returns the report of the last successful apply. Unlike EnforcementReport it stays set while a
// later apply runs or fails, so it describes the configuration Caddy is running. It reads memory only.
func (m *Manager) LastAppliedReport() (report EnforcementReport, ok bool) {
	return loadReport(&m.lastApplied)
}

// loadReport copies the report held by ptr. ok is false when none is held.
func loadReport(ptr *atomic.Pointer[EnforcementReport]) (report EnforcementReport, ok bool) {
	stored := ptr.Load()
	if stored == nil {
		return EnforcementReport{}, false
	}
	return *stored, true
}

// NewManager creates a configuration manager.
func NewManager(client CaddyClient, db *gorm.DB, configDir, frontendDir string, acmeStaging bool, securityCfg config.SecurityConfig) *Manager {
	warnDroppedEnvTrustedProxies(securityCfg.TrustedProxies)
	return &Manager{
		client:      client,
		db:          db,
		configDir:   configDir,
		frontendDir: frontendDir,
		acmeStaging: acmeStaging,
		securityCfg: securityCfg,
		adminListen: adminListenOf(client),
	}
}

// SetSecretCipher configures the cipher for decrypting DNS credentials and private keys in Caddy config generation.
func (m *Manager) SetSecretCipher(secrets crypto.VersionedCipher) {
	m.secrets = secrets
}

// SetForwardTargetGuard sets the guard that drops stored hosts whose forward target reaches an agent listener.
func (m *Manager) SetForwardTargetGuard(g ForwardTargetGuard) {
	m.forwardGuard = g
}

// SetUpstreamResolver sets the answerer that every loaded config points its name lookups at. It is set once at
// startup, before the first apply.
func (m *Manager) SetUpstreamResolver(r UpstreamResolver) {
	m.upstreamResolver.Store(&r)
}

// UpstreamNameChecksAvailable reports whether the upstream resolver is set, so forward target names are checked at
// every connection and not only when a host is saved.
func (m *Manager) UpstreamNameChecksAvailable() bool {
	return m.upstreamResolver.Load() != nil
}

// SetGeoIPState sets the source of the GeoIP database state. It is read at every apply, so a database loaded or
// reloaded after startup reaches the next generated config. Without it every country list is refused.
func (m *Manager) SetGeoIPState(state func() (path string, available bool)) {
	m.geoIPState = state
}

// SetSelfHop enables recognition of requests proxied to Charon's own API on port.
// Snapshots written by this manager carry a placeholder instead of the secret.
// The secret changes at every restart, so a persisted copy goes stale, and any
// file Caddy keeps of its own config has the same trust level as the config.
func (m *Manager) SetSelfHop(secret *selfhop.Secret, port string) {
	m.hopSecret = secret
	m.hopPort = port
}

// ApplyConfig generates configuration from database, validates it, applies to Caddy with rollback on failure.
func (m *Manager) ApplyConfig(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Drop the previous report first: until this apply succeeds, no report describes the running configuration.
	m.enforcement.Store(nil)
	// Fetch all proxy hosts from database
	var hosts []models.ProxyHost
	if err := m.db.Preload("Locations").Preload("Certificate").Preload("AccessList").Preload("SecurityHeaderProfile").Preload("DNSProvider").Find(&hosts).Error; err != nil {
		return fmt.Errorf("fetch proxy hosts: %w", err)
	}
	// Hosts whose stored forward target reaches an agent listener are left out before anything else reads them.
	fetched := hosts
	kept, filterErr := dropRefusedForwardTargets(m.forwardGuard, hosts)
	if filterErr != nil {
		return filterErr
	}
	hosts = kept

	// Fetch all redirection hosts from database (peer resource to ProxyHost,
	// see docs/plans/archive/2026-09-23_redirection-hosts-1367_spec.md §3/§4.2).
	// A missing table (e.g. a test DB that only migrates ProxyHost) is
	// tolerated — redirectHosts is left empty and GenerateConfig behaves
	// exactly as it did before this feature.
	var redirectHosts []models.RedirectionHost
	if m.db.Migrator().HasTable(&models.RedirectionHost{}) {
		if err := m.db.Preload("Certificate").Preload("DNSProvider").Find(&redirectHosts).Error; err != nil {
			return fmt.Errorf("fetch redirection hosts: %w", err)
		}
	}

	// Fetch all DNS providers for DNS challenge configuration
	var dnsProviders []models.DNSProvider
	if err := m.db.Where("enabled = ?", true).Find(&dnsProviders).Error; err != nil {
		logger.Log().WithError(err).Warn("failed to load DNS providers for config generation")
	}

	// Decrypt DNS provider credentials for config generation
	// We need an encryption service to decrypt the credentials
	var dnsProviderConfigs []DNSProviderConfig
	if len(dnsProviders) > 0 {
		if m.secrets != nil {
			// Decrypt each DNS provider's credentials
			for _, provider := range dnsProviders {
				// Skip if provider uses multi-credentials (will be handled in Phase 2)
				if provider.UseMultiCredentials {
					// Add to dnsProviderConfigs with empty Credentials for now
					// Phase 2 will populate ZoneCredentials
					dnsProviderConfigs = append(dnsProviderConfigs, DNSProviderConfig{
						ID:                 provider.ID,
						ProviderType:       provider.ProviderType,
						PropagationTimeout: provider.PropagationTimeout,
						Credentials:        nil, // Will be populated in Phase 2
					})
					continue
				}

				if provider.CredentialsEncrypted == "" {
					continue
				}

				decryptedData, err := m.secrets.DecryptWithVersion(provider.CredentialsEncrypted, provider.KeyVersion)
				if err != nil {
					logger.Log().WithError(err).WithField("provider_id", provider.ID).Warn("failed to decrypt DNS provider credentials")
					continue
				}

				var credentials map[string]string
				if err := json.Unmarshal(decryptedData, &credentials); err != nil {
					logger.Log().WithError(err).WithField("provider_id", provider.ID).Warn("failed to parse DNS provider credentials")
					continue
				}

				dnsProviderConfigs = append(dnsProviderConfigs, DNSProviderConfig{
					ID:                 provider.ID,
					ProviderType:       provider.ProviderType,
					PropagationTimeout: provider.PropagationTimeout,
					Credentials:        credentials,
				})
			}
		} else {
			logger.Log().Warn("CHARON_ENCRYPTION_KEY not set, DNS challenge configuration will be skipped")
		}
	}

	// Phase 2: Resolve zone-specific credentials for multi-credential providers
	// For each provider with UseMultiCredentials=true, build a map of domain->credentials
	// by iterating through all proxy hosts that use DNS challenge
	for i := range dnsProviderConfigs {
		cfg := &dnsProviderConfigs[i]

		// Find the provider in the dnsProviders slice to check UseMultiCredentials
		var provider *models.DNSProvider
		for j := range dnsProviders {
			if dnsProviders[j].ID == cfg.ID {
				provider = &dnsProviders[j]
				break
			}
		}

		// Skip if not multi-credential mode or provider not found
		if provider == nil || !provider.UseMultiCredentials {
			continue
		}

		// Enable multi-credential mode for this provider config
		cfg.UseMultiCredentials = true
		cfg.ZoneCredentials = make(map[string]map[string]string)

		// Preload credentials for this provider (eager loading for better logging)
		if err := m.db.Preload("Credentials").First(provider, provider.ID).Error; err != nil {
			logger.Log().WithError(err).WithField("provider_id", provider.ID).Warn("failed to preload credentials for provider")
			continue
		}

		// Iterate through proxy hosts to find domains that use this provider
		for _, host := range hosts {
			if !host.Enabled || host.DNSProviderID == nil || *host.DNSProviderID != provider.ID {
				continue
			}

			// Extract base domain from host's domain names
			baseDomain := extractBaseDomain(host.DomainNames)
			if baseDomain == "" {
				continue
			}

			// Skip if we already resolved credentials for this domain
			if _, exists := cfg.ZoneCredentials[baseDomain]; exists {
				continue
			}

			// Resolve the appropriate credential for this domain
			credentials, err := m.getCredentialForDomain(provider.ID, baseDomain, provider)
			if err != nil {
				logger.Log().
					WithError(err).
					WithField("provider_id", provider.ID).
					WithField("domain", baseDomain).
					Warn("failed to resolve credential for domain, DNS challenge will be skipped for this domain")
				continue
			}

			// Store resolved credentials for this domain
			cfg.ZoneCredentials[baseDomain] = credentials

			logger.Log().WithFields(map[string]any{
				"provider_id":   provider.ID,
				"provider_type": provider.ProviderType,
				"domain":        baseDomain,
			}).Debug("resolved credential for domain")
		}

		// Log summary of credential resolution for audit trail
		logger.Log().WithFields(map[string]any{
			"provider_id":      provider.ID,
			"provider_type":    provider.ProviderType,
			"domains_resolved": len(cfg.ZoneCredentials),
		}).Info("multi-credential DNS provider resolution complete")
	}

	// Fetch ACME email setting
	var acmeEmailSetting models.Setting
	var acmeEmail string
	if err := m.db.Where("key = ?", "caddy.acme_email").First(&acmeEmailSetting).Error; err == nil {
		acmeEmail = acmeEmailSetting.Value
	}

	// Fetch SSL Provider setting and parse it
	var sslProviderSetting models.Setting
	var sslProviderVal string
	if err := m.db.Where("key = ?", "caddy.ssl_provider").First(&sslProviderSetting).Error; err == nil {
		sslProviderVal = sslProviderSetting.Value
	}

	// Determine effective provider and staging flag based on the setting value
	effectiveProvider := ""
	effectiveStaging := false // Default to production

	switch sslProviderVal {
	case "letsencrypt-staging":
		effectiveProvider = "letsencrypt"
		effectiveStaging = true
	case "letsencrypt-prod":
		effectiveProvider = "letsencrypt"
		effectiveStaging = false
	case "zerossl":
		effectiveProvider = "zerossl"
		effectiveStaging = false
	case "auto":
		effectiveProvider = "" // "both" (auto-select between Let's Encrypt and ZeroSSL)
		effectiveStaging = false
	default:
		// Empty or unrecognized value: fallback to environment variable for backward compatibility
		effectiveProvider = ""
		if sslProviderVal == "" {
			effectiveStaging = m.acmeStaging // Respect env var if setting is unset
		} else {
			effectiveStaging = false // Unknown value defaults to production
		}
	}

	// Compute effective security flags (re-read runtime overrides)
	_, aclEnabled, wafEnabled, rateLimitEnabled, crowdsecEnabled := m.computeEffectiveFlags(ctx)

	keepaliveIdle := ""
	var keepaliveIdleSetting models.Setting
	if err := m.db.Where("key = ?", settingCaddyKeepaliveIdle).First(&keepaliveIdleSetting).Error; err == nil {
		keepaliveIdle = sanitizeKeepaliveIdle(keepaliveIdleSetting.Value)
	}

	keepaliveCount := 0
	var keepaliveCountSetting models.Setting
	if err := m.db.Where("key = ?", settingCaddyKeepaliveCnt).First(&keepaliveCountSetting).Error; err == nil {
		keepaliveCount = sanitizeKeepaliveCount(keepaliveCountSetting.Value)
	}

	// Safety check: if Cerberus is enabled in DB and no admin whitelist configured,
	// warn but allow initial startup to proceed. This prevents total lockout when
	// the user has enabled Cerberus but hasn't configured admin_whitelist yet.
	// The warning alerts them to configure it properly.
	secCfg, secCfgErr := models.FindDefaultSecurityConfig(m.db)
	if secCfgErr == nil {
		if secCfg.Enabled && strings.TrimSpace(secCfg.AdminWhitelist) == "" {
			logger.Log().Warn("Cerberus is enabled but admin_whitelist is empty. " +
				"Security features that depend on admin whitelist will not function correctly. " +
				"Please configure an admin whitelist via Settings → Security to enable full protection.")
		}
	}

	// Load ruleset metadata (WAF/Coraza) for config generation
	var rulesets []models.SecurityRuleSet
	if err := m.db.Find(&rulesets).Error; err != nil {
		// non-fatal: just log the error and continue with empty rules
		logger.Log().WithError(err).Warn("failed to load rulesets for generate config")
	}

	// Load recent security decisions so they can be injected into the generated config
	var decisions []models.SecurityDecision
	if err := m.db.Order("created_at desc").Find(&decisions).Error; err != nil {
		logger.Log().WithError(err).Warn("failed to load security decisions for generate config")
	}

	// Generate Caddy config
	// Read admin whitelist for config generation so handlers can exclude admin IPs
	var adminWhitelist string
	if secCfg.AdminWhitelist != "" {
		adminWhitelist = secCfg.AdminWhitelist
	}
	// Ensure ruleset files exist on disk and build a map of their paths for GenerateConfig
	rulesetPaths := make(map[string]string)
	if len(rulesets) > 0 {
		corazaDir := filepath.Join(m.configDir, "coraza", "rulesets")
		if err := os.MkdirAll(corazaDir, 0o700); err != nil {
			logger.Log().WithError(err).Warn("failed to create coraza rulesets dir")
		}
		for _, rs := range rulesets {
			// A rule set without a rule line would load a WAF that blocks nothing. Skip it before the engine directives
			// are prepended below, which would otherwise make the file look non-empty. Its old file is removed as stale.
			if !models.HasRuleLines(rs.Content) {
				logger.Log().WithField("ruleset", rs.Name).Warn("skipping WAF rule set that has no rules")
				continue
			}
			// Sanitize name to a safe filename - prevent path traversal and special chars
			safeName := strings.ToLower(rs.Name)
			safeName = strings.ReplaceAll(safeName, " ", "-")
			safeName = strings.ReplaceAll(safeName, "/", "-")
			safeName = strings.ReplaceAll(safeName, "\\", "-")
			safeName = strings.ReplaceAll(safeName, "..", "")   // Strip path traversal sequences
			safeName = strings.ReplaceAll(safeName, "\x00", "") // Strip null bytes
			safeName = strings.ReplaceAll(safeName, "%2f", "-") // URL-encoded slash
			safeName = strings.ReplaceAll(safeName, "%2e", "")  // URL-encoded dot
			safeName = strings.Trim(safeName, ".-")             // Trim leading/trailing dots and dashes
			if safeName == "" {
				safeName = "unnamed-ruleset"
			}

			// Prepend required Coraza directives if not already present.
			// These are essential for the WAF to actually enforce rules:
			// - SecRuleEngine On: enables blocking mode (blocks malicious requests)
			// - SecRuleEngine DetectionOnly: monitor mode (logs but doesn't block)
			// - SecRequestBodyAccess On: allows inspecting POST body content
			content := rs.Content
			if !models.HasEngineDirective(content) {
				// Determine WAF engine mode: per-ruleset mode takes precedence,
				// then global WAFMode, defaulting to blocking if neither is set
				engineMode := "On" // default to blocking
				if rs.Mode == "detection" || rs.Mode == "monitor" {
					engineMode = "DetectionOnly"
				} else if rs.Mode == "" && strings.EqualFold(secCfg.WAFMode, "monitor") {
					// No per-ruleset mode set, use global WAFMode
					engineMode = "DetectionOnly"
				}
				content = fmt.Sprintf("SecRuleEngine %s\nSecRequestBodyAccess On\n\n", engineMode) + content
			}

			// Calculate hash of the FINAL content (after prepending mode directives)
			// to ensure filename changes when mode changes, forcing Caddy to reload
			hash := sha256.Sum256([]byte(content))
			shortHash := fmt.Sprintf("%x", hash)[:8]
			filePath := filepath.Join(corazaDir, fmt.Sprintf("%s-%s.conf", safeName, shortHash))

			// Write ruleset file with world-readable permissions so the Caddy
			// process (which may run as an unprivileged user) can read it.
			if err := writeFileFunc(filePath, []byte(content), 0o644); err != nil {
				logger.Log().WithError(err).WithField("ruleset", rs.Name).Warn("failed to write coraza ruleset file")
			} else {
				// Log a short fingerprint for debugging and confirm path
				rulesetPaths[rs.Name] = filePath
				logger.Log().WithField("ruleset", rs.Name).WithField("path", filePath).Info("wrote coraza ruleset file")
			}
		}

		// Cleanup stale ruleset files that are no longer in the database
		if entries, err := readDirFunc(corazaDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				fileName := entry.Name()
				filePath := filepath.Join(corazaDir, fileName)
				// Check if this file is in the current rulesetPaths
				isActive := false
				for _, activePath := range rulesetPaths {
					if activePath == filePath {
						isActive = true
						break
					}
				}
				if !isActive {
					if removeErr := removeFileFunc(filePath); removeErr != nil {
						logger.Log().WithError(removeErr).WithField("path", filePath).Warn("failed to remove stale ruleset file")
					} else {
						logger.Log().WithField("path", filePath).Info("removed stale ruleset file")
					}
				}
			}
		} else {
			logger.Log().WithError(err).Warn("failed to read coraza rulesets dir for cleanup")
		}
	}

	var generatedReport *EnforcementReport
	genOpts := []GenerateConfigOption{
		WithSecretCipher(m.secrets),
		WithRedirectionHosts(redirectHosts),
		WithBouncerKeyPath(bouncerkey.PathFor(m.securityCfg.CrowdSecConfigDir)),
		WithEnforcementSink(func(report EnforcementReport) { generatedReport = &report }),
		WithAdminListen(m.adminListen),
	}
	if m.hopSecret != nil {
		genOpts = append(genOpts, WithSelfHop(m.hopSecret, m.hopPort))
	}
	if m.geoIPState != nil {
		geoPath, geoAvailable := m.geoIPState()
		genOpts = append(genOpts, WithGeoIPDatabase(geoPath, geoAvailable))
	}
	genOpts = append(genOpts, WithTrustedProxies(m.trustedProxyRanges()))

	generatedConfig, err := generateConfigFunc(hosts, filepath.Join(m.configDir, "data"), acmeEmail, m.frontendDir, effectiveProvider, effectiveStaging, crowdsecEnabled, wafEnabled, rateLimitEnabled, aclEnabled, adminWhitelist, rulesets, rulesetPaths, decisions, &secCfg, dnsProviderConfigs, genOpts...)
	if err != nil {
		return fmt.Errorf("generate config: %w", err)
	}

	applyOptionalServerKeepalive(generatedConfig, keepaliveIdle, keepaliveCount)

	// Debug logging: WAF configuration state for troubleshooting integration issues
	logger.Log().WithFields(map[string]any{
		"waf_enabled":       wafEnabled,
		"waf_mode":          secCfg.WAFMode,
		"waf_rules_source":  secCfg.WAFRulesSource,
		"ruleset_count":     len(rulesets),
		"ruleset_paths_len": len(rulesetPaths),
	}).Debug("WAF configuration state")
	for rsName, rsPath := range rulesetPaths {
		logger.Log().WithFields(map[string]any{
			"ruleset_name": rsName,
			"ruleset_path": rsPath,
		}).Debug("WAF ruleset path mapping")
	}

	// Log generated config size and a compact JSON snippet for debugging when in debug mode
	if cfgJSON, jerr := jsonMarshalDebugFunc(generatedConfig); jerr == nil {
		logger.Log().WithField("config_json_len", len(cfgJSON)).Debug("generated Caddy config JSON")
	} else {
		logger.Log().WithError(jerr).Warn("failed to marshal generated config for debug logging")
	}

	// Validate before applying
	if validateErr := validateConfigFunc(generatedConfig); validateErr != nil {
		return fmt.Errorf("validation failed: %w", validateErr)
	}

	// Save snapshot for rollback
	snapshotPath, err := m.saveSnapshot(generatedConfig)
	if err != nil {
		return fmt.Errorf("save snapshot: %w", err)
	}

	// Calculate config hash for audit trail
	configJSON, _ := json.Marshal(generatedConfig)
	configHash := fmt.Sprintf("%x", sha256.Sum256(configJSON))

	// Apply to Caddy
	if err := m.load(ctx, generatedConfig); err != nil {
		// Remove the failed snapshot so rollback uses the previous one
		_ = removeFileFunc(snapshotPath)

		// Rollback on failure
		if rollbackErr := m.rollback(ctx); rollbackErr != nil {
			// If rollback fails, we still want to record the failure
			m.recordConfigChange(configHash, false, err.Error())
			return fmt.Errorf("apply failed: %w, %w: %v", err, ErrRollbackFailed, rollbackErr)
		}

		// Record failed attempt
		m.recordConfigChange(configHash, false, err.Error())
		return fmt.Errorf("apply failed (rolled back): %w", err)
	}

	// Record successful application
	m.recordConfigChange(configHash, true, "")
	m.enforcement.Store(generatedReport)
	m.lastApplied.Store(generatedReport)
	leftOut := droppedHostUUIDs(fetched, hosts)
	m.leftOut.Store(&leftOut)

	// Cleanup old snapshots (keep last 10)
	if err := m.rotateSnapshots(10); err != nil {
		// Non-fatal - log but don't fail
		logger.Log().WithError(err).Warn("warning: snapshot rotation failed")
	}

	return nil
}

func sanitizeKeepaliveIdle(value string) string {
	idle := strings.TrimSpace(value)
	if idle == "" {
		return ""
	}

	d, err := time.ParseDuration(idle)
	if err != nil {
		return ""
	}

	if d < minKeepaliveIdleDuration || d > maxKeepaliveIdleDuration {
		return ""
	}

	return idle
}

func sanitizeKeepaliveCount(value string) int {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return 0
	}

	count, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}

	if count < minKeepaliveCount || count > maxKeepaliveCount {
		return 0
	}

	return count
}

// saveSnapshot stores the config to disk with timestamp.
func (m *Manager) saveSnapshot(conf *Config) (string, error) {
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("config-%d.json", timestamp)
	path := filepath.Join(m.configDir, filename)

	configJSON, err := jsonMarshalFunc(conf, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}

	configJSON = m.redactHopSecret(configJSON)

	if err := writeFileFunc(path, configJSON, 0o644); err != nil {
		return "", fmt.Errorf("write snapshot: %w", err)
	}

	return path, nil
}

// rollback loads the most recent snapshot from disk.
func (m *Manager) rollback(ctx context.Context) error {
	snapshots, err := m.listSnapshots()
	if err != nil || len(snapshots) == 0 {
		return fmt.Errorf("no snapshots available for rollback")
	}

	// Load most recent snapshot
	latestSnapshot := snapshots[len(snapshots)-1]
	configJSON, err := readFileFunc(latestSnapshot)
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}

	configJSON = m.restoreHopSecret(configJSON)

	var conf Config
	if err := json.Unmarshal(configJSON, &conf); err != nil {
		return fmt.Errorf("unmarshal snapshot: %w", err)
	}

	// A snapshot that routes a target the guard now refuses is not reinstated.
	refused, checkErr := snapshotRoutesRefusedTarget(m.forwardGuard, &conf)
	if checkErr != nil {
		return fmt.Errorf("check snapshot targets: %w", checkErr)
	}
	if refused {
		logger.Log().Warn("Rollback skipped: the last snapshot routes a forward target this proxy no longer accepts")
		return errSnapshotRefusedTarget
	}

	// Apply the snapshot
	if err := m.load(ctx, &conf); err != nil {
		if errors.Is(err, ErrUpstreamResolverUnusable) {
			logger.Log().Warn("Rollback skipped: the last snapshot routes a name through a transport that cannot use the upstream name checks")
			return errSnapshotRefusedTarget
		}
		return fmt.Errorf("load snapshot: %w", err)
	}

	return nil
}

// listSnapshots returns all snapshot file paths sorted by modification time.
func (m *Manager) listSnapshots() ([]string, error) {
	entries, err := readDirFunc(m.configDir)
	if err != nil {
		return nil, fmt.Errorf("read config dir: %w", err)
	}

	var snapshots []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		snapshots = append(snapshots, filepath.Join(m.configDir, entry.Name()))
	}

	// Sort by modification time
	sort.Slice(snapshots, func(i, j int) bool {
		infoI, _ := statFunc(snapshots[i])
		infoJ, _ := statFunc(snapshots[j])
		return infoI.ModTime().Before(infoJ.ModTime())
	})

	return snapshots, nil
}

// rotateSnapshots keeps only the N most recent snapshots.
func (m *Manager) rotateSnapshots(keep int) error {
	snapshots, err := m.listSnapshots()
	if err != nil {
		return err
	}

	if len(snapshots) <= keep {
		return nil
	}

	// Delete oldest snapshots
	toDelete := snapshots[:len(snapshots)-keep]
	for _, path := range toDelete {
		if err := removeFileFunc(path); err != nil {
			return fmt.Errorf("delete snapshot %s: %w", path, err)
		}
	}

	return nil
}

// maxStoredApplyErrorBytes bounds the apply message kept in the history. An apply and its rollback can both fail,
// so the joined message can be longer than one admin error.
const maxStoredApplyErrorBytes = 4 << 10

// recordConfigChange stores an audit record in the database.
func (m *Manager) recordConfigChange(configHash string, success bool, errorMsg string) {
	record := models.CaddyConfig{
		ConfigHash: configHash,
		AppliedAt:  time.Now(),
		Success:    success,
		ErrorMsg:   redact.Truncate(errorMsg, maxStoredApplyErrorBytes),
	}

	// Best effort - don't fail if audit logging fails
	m.db.Create(&record)
}

// Ping checks if Caddy is reachable.
func (m *Manager) Ping(ctx context.Context) error {
	return m.client.Ping(ctx)
}

// computeEffectiveFlags reads runtime settings to determine whether Cerberus
// suite and each sub-component (ACL, WAF, RateLimit, CrowdSec) are effectively enabled.
func (m *Manager) computeEffectiveFlags(_ context.Context) (cerbEnabled, aclEnabled, wafEnabled, rateLimitEnabled, crowdsecEnabled bool) {
	// Start with base flags from static config (environment variables)
	cerbEnabled = m.securityCfg.CerberusEnabled
	wafEnabled = models.StaticWAFEnabled(m.securityCfg.WAFMode)
	rateLimitEnabled = m.securityCfg.RateLimitMode == "enabled"
	crowdsecEnabled = m.securityCfg.CrowdSecMode == "local"
	aclEnabled = m.securityCfg.ACLMode == "enabled"

	if m.db != nil {
		// The default row, or the static config when no row exists. CrowdSec is resolved after the settings overrides.
		crowdSecRow := models.SecurityConfig{CrowdSecMode: m.securityCfg.CrowdSecMode}

		// Priority 1: Read from SecurityConfig table (DB overrides static config)
		// A missing row leaves defaultRow zero, which has no opinion on any module.
		var row *models.SecurityConfig
		defaultRow := models.SecurityConfig{}
		if sc, err := models.FindDefaultSecurityConfig(m.db); err == nil {
			crowdSecRow = sc
			row = &sc
			defaultRow = sc
		}

		// Priority 2: Settings table overrides (for feature flags). Cerberus takes the row above, then the feature
		// and legacy keys; boot reads the same decision through CerberusEnabledForProxy.
		cerbEnabled = CerberusEnabledForProxy(m.db, m.securityCfg, row)

		// WAF, rate limit and ACL: settings key, then the default row, then the static configuration.
		wafRes, wafErr := models.ResolveWAF(m.db, defaultRow, wafEnabled)
		logModuleReadError("waf", wafErr)
		wafEnabled = wafRes.Enabled

		rateLimitRes, rateLimitErr := models.ResolveRateLimit(m.db, defaultRow, rateLimitEnabled)
		logModuleReadError("rate_limit", rateLimitErr)
		rateLimitEnabled = rateLimitRes.Enabled

		aclRes, aclErr := models.ResolveACL(m.db, aclEnabled)
		logModuleReadError("acl", aclErr)
		aclEnabled = aclRes.Enabled

		// CrowdSec: the enabled setting, then the default row (shared with status and boot).
		var crowdSecErr error
		crowdsecEnabled, crowdSecErr = models.CrowdSecEnabled(m.db, crowdSecRow)
		if crowdSecErr != nil {
			logger.Log().WithError(crowdSecErr).Warn("Failed to read CrowdSec setting, using the default SecurityConfig row")
		}
	}

	// ACL, WAF, RateLimit and CrowdSec should only be considered enabled if Cerberus is enabled.
	if !cerbEnabled {
		aclEnabled = false
		wafEnabled = false
		rateLimitEnabled = false
		crowdsecEnabled = false
	}

	return cerbEnabled, aclEnabled, wafEnabled, rateLimitEnabled, crowdsecEnabled
}

// logModuleReadError records a failed settings or row read for one security module. The resolved state is used
// either way, so the proxy config still builds.
func logModuleReadError(module string, err error) {
	if err == nil {
		return
	}
	logger.Log().WithError(err).WithField("module", module).Warn("Failed to read security module setting, using the default SecurityConfig row or static configuration")
}

// trustedProxyRanges returns the proxy-layer trust list in force: the saved list when there is one, otherwise the
// CHARON_TRUSTED_PROXIES environment list clamped to the accepted ranges, otherwise loopback only. A failed read of the
// saved list trusts loopback only, so a database fault never widens trust.
func (m *Manager) trustedProxyRanges() []string {
	stored, err := models.ReadTrustedProxiesSetting(m.db)
	if err != nil {
		logger.Log().WithError(err).Warn("Could not read the trusted proxy setting; only loopback is trusted for this apply")
		return security.AlwaysTrustedProxies()
	}
	trust := security.EffectiveTrustedProxies(stored, m.securityCfg.TrustedProxies)
	if trust.StoredError != nil {
		logger.Log().WithError(trust.StoredError).Warn("Saved trusted proxy list no longer validates; only loopback is trusted until it is saved again")
	}
	return trust.Ranges
}

// warnDroppedEnvTrustedProxies logs once, at start, the CHARON_TRUSTED_PROXIES entries the proxy layer refuses. The
// entries stay in the API layer's list, which is unchanged.
func warnDroppedEnvTrustedProxies(envEntries []string) {
	if _, dropped := security.ClampTrustedProxies(envEntries); len(dropped) > 0 {
		logger.Log().WithField("entries", dropped).Warn("CHARON_TRUSTED_PROXIES entries were refused by the proxy layer (invalid or too broad); save a trusted proxy list to replace the environment list")
	}
}

// LeftOutHostUUIDs returns the UUIDs of the enabled proxy hosts that the last
// successful apply left out of the configuration. A nil manager, or one that has
// not applied yet, reports none. The caller must not modify the map.
func (m *Manager) LeftOutHostUUIDs() map[string]struct{} {
	if m == nil {
		return nil
	}
	if out := m.leftOut.Load(); out != nil {
		return *out
	}
	return nil
}

// droppedHostUUIDs returns the UUIDs present in all but missing from kept, that is
// the hosts the forward target filter left out.
func droppedHostUUIDs(all, kept []models.ProxyHost) map[string]struct{} {
	keptUUIDs := make(map[string]struct{}, len(kept))
	for _, h := range kept {
		keptUUIDs[h.UUID] = struct{}{}
	}
	dropped := make(map[string]struct{})
	for _, h := range all {
		if _, ok := keptUUIDs[h.UUID]; !ok {
			dropped[h.UUID] = struct{}{}
		}
	}
	return dropped
}

// rawLoader is implemented by *Client. It posts a config that is already encoded.
type rawLoader interface {
	LoadJSON(ctx context.Context, body []byte) error
}

// load posts a config to Caddy. Validation, snapshots and the audit hash run on conf
// ahead of this call, so none of them carry the resolver. Without a resolver the
// config is passed to the client unchanged. With one, a copy gets the resolver and the
// encoded copy is sent; conf is never modified.
func (m *Manager) load(ctx context.Context, conf *Config) error {
	resolver := m.upstreamResolver.Load()
	if resolver == nil {
		return m.client.Load(ctx, conf)
	}
	reserved, err := m.reservedPorts()
	if err != nil {
		return fmt.Errorf("read reserved ports: %w", err)
	}
	body, err := injectUpstreamResolver(conf, *resolver, reserved)
	if err != nil {
		if errors.Is(err, ErrUpstreamResolverUnusable) {
			logger.Log().Error("Config not loaded: a forward target uses a transport that cannot use the upstream name checks")
		}
		return err
	}
	if raw, ok := m.client.(rawLoader); ok {
		return raw.LoadJSON(ctx, body)
	}
	// Clients without a raw load (test doubles) receive the injected document as a Config.
	var injected Config
	if err := json.Unmarshal(body, &injected); err != nil {
		return fmt.Errorf("decode injected config: %w", err)
	}
	return m.client.Load(ctx, &injected)
}

// reservedPorts returns the reserved listener ports read now, or none when the guard does not report them.
func (m *Manager) reservedPorts() (map[int]struct{}, error) {
	source, ok := m.forwardGuard.(ReservedPortSource)
	if !ok {
		return map[int]struct{}{}, nil
	}
	return source.ReservedPorts()
}

// injectUpstreamResolver returns the encoded copy of conf with the resolver set on
// every reverse_proxy object. Numbers are kept exact by decoding with UseNumber.
func injectUpstreamResolver(conf *Config, resolver UpstreamResolver, reserved map[int]struct{}) ([]byte, error) {
	raw, marshalErr := json.Marshal(conf)
	if marshalErr != nil {
		return nil, fmt.Errorf("marshal config: %w", marshalErr)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode config copy: %w", err)
	}
	if err := ApplyUpstreamResolver(doc, resolver, reserved); err != nil {
		return nil, err
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal injected config: %w", err)
	}
	return body, nil
}
