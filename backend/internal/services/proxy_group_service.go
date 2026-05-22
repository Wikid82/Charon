package services

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

var (
	ErrProxyGroupNotFound  = errors.New("proxy group not found")
	ErrProxyGroupNameEmpty = errors.New("proxy group name cannot be empty")
)

// ProxyGroupService handles CRUD operations for proxy groups.
type ProxyGroupService struct {
	db *gorm.DB
}

// NewProxyGroupService creates a new ProxyGroupService.
func NewProxyGroupService(db *gorm.DB) *ProxyGroupService {
	return &ProxyGroupService{db: db}
}

// Create persists a new proxy group.
func (s *ProxyGroupService) Create(group *models.ProxyGroup) error {
	group.Name = strings.TrimSpace(group.Name)
	if group.Name == "" {
		return ErrProxyGroupNameEmpty
	}
	return s.db.Create(group).Error
}

// List returns all proxy groups ordered by name ascending.
func (s *ProxyGroupService) List() ([]models.ProxyGroup, error) {
	var groups []models.ProxyGroup
	if err := s.db.Order("name asc").Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// GetByUUID returns the proxy group with the given UUID.
func (s *ProxyGroupService) GetByUUID(uuidStr string) (*models.ProxyGroup, error) {
	var group models.ProxyGroup
	result := s.db.Where("uuid = ?", uuidStr).Limit(1).Find(&group)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrProxyGroupNotFound
	}
	return &group, nil
}

// Update saves changes to an existing proxy group.
func (s *ProxyGroupService) Update(group *models.ProxyGroup) error {
	return s.db.Save(group).Error
}

// Delete removes the proxy group and unassigns any hosts that belonged to it.
func (s *ProxyGroupService) Delete(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.ProxyHost{}).
			Where("proxy_group_id = ?", id).
			Update("proxy_group_id", nil).Error; err != nil {
			return fmt.Errorf("failed to unassign proxy hosts: %w", err)
		}
		return tx.Delete(&models.ProxyGroup{}, id).Error
	})
}

// GetHostCount returns the number of proxy hosts assigned to the given group.
func (s *ProxyGroupService) GetHostCount(id uint) (int64, error) {
	var count int64
	err := s.db.Model(&models.ProxyHost{}).Where("proxy_group_id = ?", id).Count(&count).Error
	return count, err
}
