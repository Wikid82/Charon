package caddy

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

// Manager orchestrates Caddy configuration lifecycle: generate, validate, apply, rollback.
type Manager struct {
	client    *Client
	db        *gorm.DB
	configDir string
}

// NewManager creates a configuration manager.
func NewManager(client *Client, db *gorm.DB, configDir string) *Manager {
	return &Manager{
		client:    client,
		db:        db,
		configDir: configDir,
	}
}

// ApplyConfig generates configuration from database, validates it, applies to Caddy with rollback on failure.
func (m *Manager) ApplyConfig(ctx context.Context) error {
	// Fetch all proxy hosts from database
	var hosts []models.ProxyHost
	if err := m.db.Find(&hosts).Error; err != nil {
		return fmt.Errorf("fetch proxy hosts: %w", err)
	}

	// Fetch ACME email setting
	var acmeEmailSetting models.Setting
	var acmeEmail string
	if err := m.db.Where("key = ?", "caddy.acme_email").First(&acmeEmailSetting).Error; err == nil {
		acmeEmail = acmeEmailSetting.Value
	}

	// Generate Caddy config
	config, err := GenerateConfig(hosts, filepath.Join(m.configDir, "data"), acmeEmail)
	if err != nil {
		return fmt.Errorf("generate config: %w", err)
	}

	// Validate before applying
	if err := Validate(config); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Save snapshot for rollback
	if _, err := m.saveSnapshot(config); err != nil {
		return fmt.Errorf("save snapshot: %w", err)
	}

	// Calculate config hash for audit trail
	configJSON, _ := json.Marshal(config)
	configHash := fmt.Sprintf("%x", sha256.Sum256(configJSON))

	// Apply to Caddy
	if err := m.client.Load(ctx, config); err != nil {
		// Rollback on failure
		if rollbackErr := m.rollback(ctx); rollbackErr != nil {
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
		fmt.Printf("warning: snapshot rotation failed: %v\n", err)
	}

	return nil
}

// saveSnapshot stores the config to disk with timestamp.
func (m *Manager) saveSnapshot(config *Config) (string, error) {
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("config-%d.json", timestamp)
	path := filepath.Join(m.configDir, filename)

	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, configJSON, 0644); err != nil {
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
	configJSON, err := os.ReadFile(latestSnapshot)
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
	entries, err := os.ReadDir(m.configDir)
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
		infoI, _ := os.Stat(snapshots[i])
		infoJ, _ := os.Stat(snapshots[j])
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
		if err := os.Remove(path); err != nil {
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
