package services

import (
	"errors"
	"net/url"
	"strings"

	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

// RedirectionHostService encapsulates business logic for redirection host
// management — full CRUD plus validation (§4.1/§4.3/§4.4 of
// docs/plans/archive/2026-09-23_redirection-hosts-1367_spec.md),
// mirroring ProxyHostService's conventions.
type RedirectionHostService struct {
	db *gorm.DB
}

// NewRedirectionHostService creates a new redirection host service.
func NewRedirectionHostService(db *gorm.DB) *RedirectionHostService {
	return &RedirectionHostService{db: db}
}

// validateRedirectionHost validates and normalizes a RedirectionHost's
// fields before persistence: required fields, target_url scheme/host
// validation, status-code enum membership, and the self-redirect guard
// (docs/plans/archive/2026-09-23_redirection-hosts-1367_spec.md §4.1/§4.3/§4.4/§7).
func (s *RedirectionHostService) validateRedirectionHost(host *models.RedirectionHost) error {
	host.DomainNames = CanonicalDomainNames(host.DomainNames)
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

	if splitDomains(host.DomainNames)[normalizeDomain(parsed.Hostname())] {
		return errors.New("redirect target cannot point back to one of this host's own domains")
	}

	if host.UseDNSChallenge && host.DNSProviderID == nil {
		return errors.New("dns provider is required when use_dns_challenge is enabled")
	}

	return nil
}

// checkHostWrite runs the validation and uniqueness checks shared by Create
// and Update. It must run inside the write-locked transaction so the checks
// and the subsequent write are atomic. A non-nil certificate_id must reference
// an existing certificate.
func (s *RedirectionHostService) checkHostWrite(tx *gorm.DB, host *models.RedirectionHost, excludeID uint) error {
	if err := s.validateRedirectionHost(host); err != nil {
		return err
	}
	if err := checkSameTableDomainConflict(tx, host.DomainNames, &models.RedirectionHost{}, excludeID); err != nil {
		return err
	}
	if err := checkCrossTableDomainConflict(tx, host.DomainNames, &models.ProxyHost{}); err != nil {
		return err
	}
	return ensureCertificateExists(tx, host.CertificateID)
}

// Create validates and creates a new redirection host. The uniqueness checks
// and the insert run in one write-locked transaction. The lock is database
// wide, so it also serializes against concurrent ProxyHostService writes
// (see WithWriteLock).
func (s *RedirectionHostService) Create(host *models.RedirectionHost) error {
	return WithWriteLock(s.db, &models.RedirectionHost{}, func(tx *gorm.DB) error {
		if err := s.checkHostWrite(tx, host, 0); err != nil {
			return err
		}
		return tx.Create(host).Error
	})
}

// Update validates and updates an existing redirection host. The uniqueness
// checks and the write run in one write-locked transaction (see WithWriteLock).
func (s *RedirectionHostService) Update(host *models.RedirectionHost) error {
	return WithWriteLock(s.db, &models.RedirectionHost{}, func(tx *gorm.DB) error {
		if err := s.checkHostWrite(tx, host, host.ID); err != nil {
			return err
		}
		// Use Updates+Select("*") to handle nullable foreign keys properly,
		// mirroring ProxyHostService.Update.
		return tx.Model(&models.RedirectionHost{}).
			Where("id = ?", host.ID).
			Select("*").
			Updates(host).Error
	})
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
