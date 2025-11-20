package services

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

// ProxyHostService encapsulates business logic for proxy host management.
type ProxyHostService struct {
	db *gorm.DB
}

// NewProxyHostService creates a new proxy host service.
func NewProxyHostService(db *gorm.DB) *ProxyHostService {
	return &ProxyHostService{db: db}
}

// ValidateUniqueDomain ensures no duplicate domains exist before creation/update.
func (s *ProxyHostService) ValidateUniqueDomain(domainNames string, excludeID uint) error {
	var count int64
	query := s.db.Model(&models.ProxyHost{}).Where("domain_names = ?", domainNames)

	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}

	if err := query.Count(&count).Error; err != nil {
		return fmt.Errorf("checking domain uniqueness: %w", err)
	}

	if count > 0 {
		return errors.New("domain already exists")
	}

	return nil
}

// Create validates and creates a new proxy host.
func (s *ProxyHostService) Create(host *models.ProxyHost) error {
	if err := s.ValidateUniqueDomain(host.DomainNames, 0); err != nil {
		return err
	}

	return s.db.Create(host).Error
}

// Update validates and updates an existing proxy host.
func (s *ProxyHostService) Update(host *models.ProxyHost) error {
	if err := s.ValidateUniqueDomain(host.DomainNames, host.ID); err != nil {
		return err
	}

	return s.db.Save(host).Error
}

// Delete removes a proxy host.
func (s *ProxyHostService) Delete(id uint) error {
	return s.db.Delete(&models.ProxyHost{}, id).Error
}

// GetByID retrieves a proxy host by ID.
func (s *ProxyHostService) GetByID(id uint) (*models.ProxyHost, error) {
	var host models.ProxyHost
	if err := s.db.First(&host, id).Error; err != nil {
		return nil, err
	}
	return &host, nil
}

// GetByUUID finds a proxy host by UUID.
func (s *ProxyHostService) GetByUUID(uuid string) (*models.ProxyHost, error) {
	var host models.ProxyHost
	if err := s.db.Preload("Locations").Where("uuid = ?", uuid).First(&host).Error; err != nil {
		return nil, err
	}
	return &host, nil
}

// List returns all proxy hosts.
func (s *ProxyHostService) List() ([]models.ProxyHost, error) {
	var hosts []models.ProxyHost
	if err := s.db.Preload("Locations").Order("updated_at desc").Find(&hosts).Error; err != nil {
		return nil, err
	}
	return hosts, nil
}
