package caddy

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

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
	GetConfig(ctx context.Context) (*Config, error)
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
	encSvc      *crypto.EncryptionService
	orthrusSvc  OrthrusAddrResolver
}

// NewManager creates a configuration manager.
func NewManager(client CaddyClient, db *gorm.DB, configDir, frontendDir string, acmeStaging bool, securityCfg config.SecurityConfig) *Manager {
	return &Manager{
		client:      client,
		db:          db,
		configDir:   configDir,
		frontendDir: frontendDir,
		acmeStaging: acmeStaging,
		securityCfg: securityCfg,
	}
}

// SetEncryptionService configures the encryption service for decrypting private keys in Caddy config generation.
func (m *Manager) SetEncryptionService(svc *crypto.EncryptionService) {
	m.encSvc = svc
}

// SetOrthrusServer configures the Orthrus agent resolver for dynamic upstream host resolution.
func (m *Manager) SetOrthrusServer(s OrthrusAddrResolver) {
	m.orthrusSvc = s
}

// ApplyConfig generates configuration from database, validates it, applies to Caddy with rollback on failure.
func (m *Manager) ApplyConfig(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Fetch all proxy hosts from database
	var hosts []models.ProxyHost
	if err := m.db.Preload("Locations").Preload("Certificate").Preload("AccessList").Preload("SecurityHeaderProfile").Preload("DNSProvider").Find(&hosts).Error; err != nil {
		return fmt.Errorf("fetch proxy hosts: %w", err)
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
		// Try to get encryption key from environment
		encryptionKey := os.Getenv("CHARON_ENCRYPTION_KEY")
		if encryptionKey == "" {
			// Try alternative env vars
			for _, key := range []string{"ENCRYPTION_KEY", "CERBERUS_ENCRYPTION_KEY"} {
				if val := os.Getenv(key); val != "" {
					encryptionKey = val
					break
				}
			}
		}

		if encryptionKey != "" {
			// Import crypto package for inline decryption
			encryptor, err := crypto.NewEncryptionService(encryptionKey)
			if err != nil {
				logger.Log().WithError(err).Warn("failed to initialize encryption service for DNS provider credentials")
			} else {
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

					decryptedData, err := encryptor.Decrypt(provider.CredentialsEncrypted)
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
	var secCfg models.SecurityConfig
	if err := m.db.Where("name = ?", "default").First(&secCfg).Error; err == nil {
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
			if !strings.Contains(strings.ToLower(content), "secruleengine") {
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

	hosts = resolveOrthrusHosts(hosts, m.orthrusSvc)

	generatedConfig, err := generateConfigFunc(hosts, filepath.Join(m.configDir, "data"), acmeEmail, m.frontendDir, effectiveProvider, effectiveStaging, crowdsecEnabled, wafEnabled, rateLimitEnabled, aclEnabled, adminWhitelist, rulesets, rulesetPaths, decisions, &secCfg, dnsProviderConfigs, m.encSvc)
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
	if err := m.client.Load(ctx, generatedConfig); err != nil {
		// Remove the failed snapshot so rollback uses the previous one
		_ = removeFileFunc(snapshotPath)

		// Rollback on failure
		if rollbackErr := m.rollback(ctx); rollbackErr != nil {
			// If rollback fails, we still want to record the failure
			m.recordConfigChange(configHash, false, err.Error())
			return fmt.Errorf("apply failed: %w, rollback also failed: %v", err, rollbackErr)
		}

		// Record failed attempt
		m.recordConfigChange(configHash, false, err.Error())
		return fmt.Errorf("apply failed (rolled back): %w", err)
	}

	// Record successful application
	m.recordConfigChange(configHash, true, "")

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

	var conf Config
	if err := json.Unmarshal(configJSON, &conf); err != nil {
		return fmt.Errorf("unmarshal snapshot: %w", err)
	}

	// Apply the snapshot
	if err := m.client.Load(ctx, &conf); err != nil {
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

// recordConfigChange stores an audit record in the database.
func (m *Manager) recordConfigChange(configHash string, success bool, errorMsg string) {
	record := models.CaddyConfig{
		ConfigHash: configHash,
		AppliedAt:  time.Now(),
		Success:    success,
		ErrorMsg:   errorMsg,
	}

	// Best effort - don't fail if audit logging fails
	m.db.Create(&record)
}

// Ping checks if Caddy is reachable.
func (m *Manager) Ping(ctx context.Context) error {
	return m.client.Ping(ctx)
}

// GetCurrentConfig retrieves the running config from Caddy.
func (m *Manager) GetCurrentConfig(ctx context.Context) (*Config, error) {
	return m.client.GetConfig(ctx)
}

// computeEffectiveFlags reads runtime settings to determine whether Cerberus
// suite and each sub-component (ACL, WAF, RateLimit, CrowdSec) are effectively enabled.
func (m *Manager) computeEffectiveFlags(_ context.Context) (cerbEnabled, aclEnabled, wafEnabled, rateLimitEnabled, crowdsecEnabled bool) {
	// Start with base flags from static config (environment variables)
	cerbEnabled = m.securityCfg.CerberusEnabled
	wafEnabled = m.securityCfg.WAFMode != "" && m.securityCfg.WAFMode != "disabled"
	rateLimitEnabled = m.securityCfg.RateLimitMode == "enabled"
	crowdsecEnabled = m.securityCfg.CrowdSecMode == "local"
	aclEnabled = m.securityCfg.ACLMode == "enabled"

	if m.db != nil {
		// Priority 1: Read from SecurityConfig table (DB overrides static config)
		var sc models.SecurityConfig
		if err := m.db.Where("name = ?", "default").First(&sc).Error; err == nil {
			// SecurityConfig.Enabled controls Cerberus globally
			cerbEnabled = sc.Enabled

			// WAF mode from DB
			if sc.WAFMode != "" {
				wafEnabled = !strings.EqualFold(sc.WAFMode, "disabled")
			}

			// Rate limiting from DB
			if sc.RateLimitMode != "" {
				rateLimitEnabled = strings.EqualFold(sc.RateLimitMode, "enabled")
			} else if sc.RateLimitEnable {
				// Fallback to boolean field for backward compatibility
				rateLimitEnabled = true
			}

			// CrowdSec mode from DB
			if sc.CrowdSecMode != "" {
				crowdsecEnabled = sc.CrowdSecMode == "local"
			}

			// ACL mode (if we add it to SecurityConfig in the future)
			// For now, ACL mode stays at static config value or settings override below
		}

		// Priority 2: Settings table overrides (for feature flags)
		var s models.Setting
		// runtime override for cerberus enabled (check feature flag first, fallback to legacy key)
		if err := m.db.Where("key = ?", "feature.cerberus.enabled").First(&s).Error; err == nil {
			cerbEnabled = strings.EqualFold(s.Value, "true")
		} else if err := m.db.Where("key = ?", "security.cerberus.enabled").First(&s).Error; err == nil {
			cerbEnabled = strings.EqualFold(s.Value, "true")
		}

		// runtime override for ACL enabled
		s = models.Setting{} // Reset to prevent ID leakage from previous query
		if err := m.db.Where("key = ?", "security.acl.enabled").First(&s).Error; err == nil {
			if strings.EqualFold(s.Value, "true") {
				aclEnabled = true
			} else if strings.EqualFold(s.Value, "false") {
				aclEnabled = false
			}
		}

		// runtime override for WAF enabled
		s = models.Setting{} // Reset
		if err := m.db.Where("key = ?", "security.waf.enabled").First(&s).Error; err == nil {
			if strings.EqualFold(s.Value, "true") {
				wafEnabled = true
			} else if strings.EqualFold(s.Value, "false") {
				wafEnabled = false
			}
		}

		// runtime override for Rate Limit enabled
		s = models.Setting{} // Reset
		if err := m.db.Where("key = ?", "security.rate_limit.enabled").First(&s).Error; err == nil {
			if strings.EqualFold(s.Value, "true") {
				rateLimitEnabled = true
			} else if strings.EqualFold(s.Value, "false") {
				rateLimitEnabled = false
			}
		}

		// runtime override for crowdsec mode (mode value determines whether it's local/remote/enabled)
		var cm struct{ Value string }
		if err := m.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.mode").Scan(&cm).Error; err == nil && cm.Value != "" {
			// Only 'local' runtime mode enables CrowdSec; all other values are disabled
			if cm.Value == "local" {
				crowdsecEnabled = true
			} else {
				crowdsecEnabled = false
			}
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
