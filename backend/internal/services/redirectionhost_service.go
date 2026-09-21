package services

import (
	"errors"
	"fmt"

	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

// RedirectionHostService encapsulates business logic for redirection host
// management.
//
// NOTE: This is intentionally a minimal version, scoped to Commit 3 of the
// Redirection Hosts feature (docs/plans/current_spec.md §9). It exists here
// only to host the domain-uniqueness checks (own-table + additive
// cross-table) so those checks have a natural home and test coverage ahead
// of the rest of the service. Create/Update/Delete/List/GetByUUID and full
// field validation (URL scheme, status-code enum, self-redirect guard) are
// Commit 4's job — see docs/plans/current_spec.md §4.3/§4.4/§9. Whoever
// implements Commit 4 should extend this struct with those methods and wire
// ValidateUniqueDomain + CheckCrossTableDomainConflict into its Create/Update.
type RedirectionHostService struct {
	db *gorm.DB
}

// NewRedirectionHostService creates a new redirection host service.
func NewRedirectionHostService(db *gorm.DB) *RedirectionHostService {
	return &RedirectionHostService{db: db}
}

// ValidateUniqueDomain ensures no duplicate domains exist among other
// RedirectionHost rows before creation/update. Mirrors
// ProxyHostService.ValidateUniqueDomain's same-table, whole-string
// domain_names comparison (proxyhost_service.go:57-74), scoped to the
// redirection_hosts table instead of proxy_hosts.
func (s *RedirectionHostService) ValidateUniqueDomain(domainNames string, excludeID uint) error {
	var count int64
	query := s.db.Model(&models.RedirectionHost{}).Where("domain_names = ?", domainNames)

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

// CheckCrossTableDomainConflict is the additive cross-table check against
// ProxyHost's table, mirroring the call ProxyHostService.Create/Update make
// against RedirectionHost's table (see domain_uniqueness.go). It is exposed
// as its own method so Commit 4's Create/Update can call it directly once
// the rest of this service (validation, persistence) is built out.
func (s *RedirectionHostService) CheckCrossTableDomainConflict(domainNames string) error {
	return CheckDomainConflict(s.db, domainNames, &models.ProxyHost{})
}
