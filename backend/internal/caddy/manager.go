package caddy

import (
		"strings"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
)

// Test hooks to allow overriding OS and JSON functions
var (
	writeFileFunc   = os.WriteFile
	readFileFunc    = os.ReadFile
	removeFileFunc  = os.Remove
	readDirFunc     = os.ReadDir
	statFunc        = os.Stat
	jsonMarshalFunc = json.MarshalIndent
	// Test hooks for bandaging validation/generation flows
	generateConfigFunc = GenerateConfig
	validateConfigFunc = Validate
)

// Manager orchestrates Caddy configuration lifecycle: generate, validate, apply, rollback.
type Manager struct {
	client       *Client
	db           *gorm.DB
	configDir    string
	frontendDir  string
	acmeStaging  bool
	securityCfg  config.SecurityConfig
}

// NewManager creates a configuration manager.
func NewManager(client *Client, db *gorm.DB, configDir string, frontendDir string, acmeStaging bool, securityCfg config.SecurityConfig) *Manager {
	return &Manager{
		client:      client,
		db:          db,
		configDir:   configDir,
		frontendDir: frontendDir,
		acmeStaging: acmeStaging,
		securityCfg: securityCfg,
	}
}

// ApplyConfig generates configuration from database, validates it, applies to Caddy with rollback on failure.
func (m *Manager) ApplyConfig(ctx context.Context) error {
	// Fetch all proxy hosts from database
	var hosts []models.ProxyHost
	if err := m.db.Preload("Locations").Preload("Certificate").Preload("AccessList").Find(&hosts).Error; err != nil {
		return fmt.Errorf("fetch proxy hosts: %w", err)
	}

	// Fetch ACME email setting
	var acmeEmailSetting models.Setting
	var acmeEmail string
	if err := m.db.Where("key = ?", "caddy.acme_email").First(&acmeEmailSetting).Error; err == nil {
		acmeEmail = acmeEmailSetting.Value
	}

	// Fetch SSL Provider setting
	var sslProviderSetting models.Setting
	var sslProvider string
	if err := m.db.Where("key = ?", "caddy.ssl_provider").First(&sslProviderSetting).Error; err == nil {
		sslProvider = sslProviderSetting.Value
	}

	// Compute effective security flags (re-read runtime overrides)
	_, aclEnabled, wafEnabled, rateLimitEnabled, crowdsecEnabled := m.computeEffectiveFlags(ctx)

	// Generate Caddy config
	config, err := generateConfigFunc(hosts, filepath.Join(m.configDir, "data"), acmeEmail, m.frontendDir, sslProvider, m.acmeStaging, crowdsecEnabled, wafEnabled, rateLimitEnabled, aclEnabled)
	if err != nil {
		return fmt.Errorf("generate config: %w", err)
	}

	// Validate before applying
	if err := validateConfigFunc(config); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Save snapshot for rollback
	snapshotPath, err := m.saveSnapshot(config)
	if err != nil {
		return fmt.Errorf("save snapshot: %w", err)
	}

	// Calculate config hash for audit trail
	configJSON, _ := json.Marshal(config)
	configHash := fmt.Sprintf("%x", sha256.Sum256(configJSON))

	// Apply to Caddy
	if err := m.client.Load(ctx, config); err != nil {
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

// saveSnapshot stores the config to disk with timestamp.
func (m *Manager) saveSnapshot(config *Config) (string, error) {
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("config-%d.json", timestamp)
	path := filepath.Join(m.configDir, filename)

	configJSON, err := jsonMarshalFunc(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}

	if err := writeFileFunc(path, configJSON, 0644); err != nil {
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

	var config Config
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return fmt.Errorf("unmarshal snapshot: %w", err)
	}

	// Apply the snapshot
	if err := m.client.Load(ctx, &config); err != nil {
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
func (m *Manager) computeEffectiveFlags(ctx context.Context) (cerbEnabled bool, aclEnabled bool, wafEnabled bool, rateLimitEnabled bool, crowdsecEnabled bool) {
	// Base flags from static config
	cerbEnabled = m.securityCfg.CerberusEnabled
	wafEnabled = m.securityCfg.WAFMode == "enabled"
	rateLimitEnabled = m.securityCfg.RateLimitMode == "enabled"
	crowdsecEnabled = m.securityCfg.CrowdSecMode == "local" || m.securityCfg.CrowdSecMode == "remote" || m.securityCfg.CrowdSecMode == "enabled"
	aclEnabled = m.securityCfg.ACLMode == "enabled"

	if m.db != nil {
		var s models.Setting
		// runtime override for cerberus enabled
		if err := m.db.Where("key = ?", "security.cerberus.enabled").First(&s).Error; err == nil {
			cerbEnabled = strings.EqualFold(s.Value, "true")
		}

		// runtime override for ACL enabled
		if err := m.db.Where("key = ?", "security.acl.enabled").First(&s).Error; err == nil {
			if strings.EqualFold(s.Value, "true") {
				aclEnabled = true
			} else if strings.EqualFold(s.Value, "false") {
				aclEnabled = false
			}
		}

		// runtime override for crowdsec mode (mode value determines whether it's local/remote/enabled)
		var cm struct{ Value string }
		if err := m.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.mode").Scan(&cm).Error; err == nil && cm.Value != "" {
			// If crowdsec mode is external, we mark it disabled for our plugin
			if cm.Value == "external" {
				crowdsecEnabled = false
			} else if cm.Value == "local" || cm.Value == "remote" || cm.Value == "enabled" {
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
