package services

import (
	"fmt"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/hecate"
	"github.com/Wikid82/charon/backend/internal/hecate/providers/netbird"
	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

// HecateService provides CRUD operations for TunnelConfig records and
// delegates lifecycle management to the TunnelManager.
type HecateService struct {
	db     *gorm.DB
	encSvc *crypto.EncryptionService
	mgr    *hecate.TunnelManager
}

// NewHecateService creates a HecateService and registers all supported provider factories.
func NewHecateService(db *gorm.DB, encSvc *crypto.EncryptionService, mgr *hecate.TunnelManager) *HecateService {
	mgr.RegisterFactory(models.ProviderNetBird, netbird.Factory)
	return &HecateService{db: db, encSvc: encSvc, mgr: mgr}
}

// List returns all TunnelConfig records.
func (s *HecateService) List() ([]models.TunnelConfig, error) {
	var configs []models.TunnelConfig
	if err := s.db.Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("hecate: list configs: %w", err)
	}
	return configs, nil
}

// Create persists a new TunnelConfig. plainCredentials are encrypted before
// storage. If the config is active, the tunnel is started immediately.
func (s *HecateService) Create(cfg *models.TunnelConfig, plainCredentials string) error {
	encrypted, err := s.encSvc.Encrypt([]byte(plainCredentials))
	if err != nil {
		return fmt.Errorf("hecate: encrypt credentials: %w", err)
	}
	cfg.EncryptedCredentials = encrypted

	if err := s.db.Create(cfg).Error; err != nil {
		return fmt.Errorf("hecate: create config: %w", err)
	}

	if cfg.IsActive {
		if err := s.mgr.StartTunnel(cfg.UUID); err != nil {
			return fmt.Errorf("hecate: start tunnel after create: %w", err)
		}
	}
	return nil
}

// Get retrieves a single TunnelConfig by UUID.
func (s *HecateService) Get(uuid string) (*models.TunnelConfig, error) {
	var cfg models.TunnelConfig
	if err := s.db.Where("uuid = ?", uuid).First(&cfg).Error; err != nil {
		return nil, fmt.Errorf("hecate: get config %s: %w", uuid, err)
	}
	return &cfg, nil
}

// Update applies field updates to an existing TunnelConfig. If
// plainCredentials is provided, the credentials are re-encrypted.
func (s *HecateService) Update(uuid string, cfg *models.TunnelConfig, plainCredentials *string) error {
	existing, err := s.Get(uuid)
	if err != nil {
		return err
	}

	if plainCredentials != nil {
		encrypted, encErr := s.encSvc.Encrypt([]byte(*plainCredentials))
		if encErr != nil {
			return fmt.Errorf("hecate: re-encrypt credentials: %w", encErr)
		}
		existing.EncryptedCredentials = encrypted
	}

	existing.Name = cfg.Name
	existing.Provider = cfg.Provider
	existing.Configuration = cfg.Configuration
	existing.IsActive = cfg.IsActive

	if err := s.db.Save(existing).Error; err != nil {
		return fmt.Errorf("hecate: update config %s: %w", uuid, err)
	}
	return nil
}

// Delete stops the tunnel (if running) and removes the TunnelConfig.
func (s *HecateService) Delete(uuid string) error {
	if err := s.mgr.StopTunnel(uuid); err != nil {
		return fmt.Errorf("hecate: stop tunnel before delete: %w", err)
	}
	if err := s.db.Where("uuid = ?", uuid).Delete(&models.TunnelConfig{}).Error; err != nil {
		return fmt.Errorf("hecate: delete config %s: %w", uuid, err)
	}
	return nil
}

// RotateCredentials re-encrypts the credentials for a config and restarts the
// tunnel so the new credentials take effect immediately.
func (s *HecateService) RotateCredentials(uuid, plainCredentials string) error {
	existing, err := s.Get(uuid)
	if err != nil {
		return err
	}

	encrypted, err := s.encSvc.Encrypt([]byte(plainCredentials))
	if err != nil {
		return fmt.Errorf("hecate: encrypt rotated credentials: %w", err)
	}
	existing.EncryptedCredentials = encrypted

	if err := s.db.Save(existing).Error; err != nil {
		return fmt.Errorf("hecate: save rotated credentials: %w", err)
	}

	if err := s.mgr.StopTunnel(uuid); err != nil {
		return fmt.Errorf("hecate: stop tunnel for rotation: %w", err)
	}
	if err := s.mgr.StartTunnel(uuid); err != nil {
		return fmt.Errorf("hecate: restart tunnel after rotation: %w", err)
	}
	return nil
}

// GetStatus returns the current runtime status of all managed tunnels.
func (s *HecateService) GetStatus() []hecate.TunnelStatus {
	return s.mgr.GetStatus()
}

// GetManager returns the underlying TunnelManager for provider-level access.
func (s *HecateService) GetManager() *hecate.TunnelManager {
	return s.mgr
}
