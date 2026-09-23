package services

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

// RedirectionHostService encapsulates business logic for redirection host
// management — full CRUD plus validation (§4.1/§4.3/§4.4 of
// docs/plans/current_spec.md), mirroring ProxyHostService's conventions.
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
// against RedirectionHost's table (see domain_uniqueness.go).
func (s *RedirectionHostService) CheckCrossTableDomainConflict(domainNames string) error {
	return CheckDomainConflict(s.db, domainNames, &models.ProxyHost{})
}

// validateRedirectionHost validates and normalizes a RedirectionHost's
// fields before persistence: required fields, target_url scheme/host
// validation, status-code enum membership, and the self-redirect guard
// (docs/plans/current_spec.md §4.1/§4.3/§4.4/§7).
func (s *RedirectionHostService) validateRedirectionHost(host *models.RedirectionHost) error {
	host.DomainNames = strings.TrimSpace(host.DomainNames)
	host.TargetURL = strings.TrimSpace(host.TargetURL)

	if host.DomainNames == "" {
		return errors.New("domain names is required")
	}

	if host.TargetURL == "" {
		return errors.New("target_url is required")
	}

	parsed, err := url.Parse(host.TargetURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("target_url must be a valid http(s) URL")
	}

	if !models.ValidRedirectStatusCodes[host.StatusCode] {
		return errors.New("status_code must be one of 301, 302, 307, 308")
	}

	targetHost := strings.ToLower(parsed.Hostname())
	for _, d := range strings.Split(host.DomainNames, ",") {
		d = strings.ToLower(strings.TrimSpace(d))
		if d != "" && d == targetHost {
			return errors.New("redirect target cannot point back to one of this host's own domains")
		}
	}

	if host.UseDNSChallenge && host.DNSProviderID == nil {
		return errors.New("dns provider is required when use_dns_challenge is enabled")
	}

	return nil
}

// Create validates and creates a new redirection host.
func (s *RedirectionHostService) Create(host *models.RedirectionHost) error {
	if err := s.validateRedirectionHost(host); err != nil {
		return err
	}

	if err := s.ValidateUniqueDomain(host.DomainNames, 0); err != nil {
		return err
	}

	if err := s.CheckCrossTableDomainConflict(host.DomainNames); err != nil {
		return err
	}

	return s.db.Create(host).Error
}

// Update validates and updates an existing redirection host.
func (s *RedirectionHostService) Update(host *models.RedirectionHost) error {
	if err := s.validateRedirectionHost(host); err != nil {
		return err
	}

	if err := s.ValidateUniqueDomain(host.DomainNames, host.ID); err != nil {
		return err
	}

	if err := s.CheckCrossTableDomainConflict(host.DomainNames); err != nil {
		return err
	}

	// Use Updates+Select("*") to handle nullable foreign keys properly,
	// mirroring ProxyHostService.Update.
	return s.db.Model(&models.RedirectionHost{}).
		Where("id = ?", host.ID).
		Select("*").
		Updates(host).Error
}

// Delete removes a redirection host.
func (s *RedirectionHostService) Delete(id uint) error {
	return s.db.Delete(&models.RedirectionHost{}, id).Error
}

// GetByID retrieves a redirection host by numeric ID.
func (s *RedirectionHostService) GetByID(id uint) (*models.RedirectionHost, error) {
	var host models.RedirectionHost
	if err := s.db.Where("id = ?", id).First(&host).Error; err != nil {
		return nil, err
	}
	return &host, nil
}

// GetByUUID finds a redirection host by UUID.
func (s *RedirectionHostService) GetByUUID(uuidStr string) (*models.RedirectionHost, error) {
	var host models.RedirectionHost
	if err := s.db.Preload("Certificate").Preload("DNSProvider").Where("uuid = ?", uuidStr).First(&host).Error; err != nil {
		return nil, err
	}
	return &host, nil
}

// List returns all redirection hosts, newest-updated first.
func (s *RedirectionHostService) List() ([]models.RedirectionHost, error) {
	var hosts []models.RedirectionHost
	if err := s.db.Preload("Certificate").Preload("DNSProvider").Order("updated_at desc").Find(&hosts).Error; err != nil {
		return nil, err
	}
	return hosts, nil
}

// DB returns the underlying database instance for advanced operations.
func (s *RedirectionHostService) DB() *gorm.DB {
	return s.db
}
