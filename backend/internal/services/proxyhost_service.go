package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/logger"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// CertificateCacheInvalidator is the minimal interface ProxyHostService needs
// to keep the certificate list's "in_use" status fresh. Satisfied by
// *CertificateService; kept as an interface (rather than a direct dependency)
// to avoid a hard coupling between the two services.
type CertificateCacheInvalidator interface {
	InvalidateCache()
}

// ProxyHostService encapsulates business logic for proxy host management.
type ProxyHostService struct {
	db          *gorm.DB
	certService CertificateCacheInvalidator
}

// NewProxyHostService creates a new proxy host service.
func NewProxyHostService(db *gorm.DB) *ProxyHostService {
	return &ProxyHostService{db: db}
}

// SetCertificateService wires an optional certificate cache invalidator.
// When set, creating, updating, or deleting a proxy host invalidates the
// certificate service's cache so the "in_use" flag reflects the change
// immediately instead of waiting out the cache's scan TTL.
func (s *ProxyHostService) SetCertificateService(certService CertificateCacheInvalidator) {
	s.certService = certService
}

// invalidateCertCache notifies the certificate service (if wired) that a
// certificate<->proxy-host association may have changed.
func (s *ProxyHostService) invalidateCertCache() {
	if s.certService != nil {
		s.certService.InvalidateCache()
	}
}

// ValidateHostname checks if the provided string is a valid hostname or IP address.
func (s *ProxyHostService) ValidateHostname(host string) error {
	// Parse as URL to extract hostname if scheme is present
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		if u, err := url.Parse(host); err == nil {
			host = u.Hostname()
		} else {
			// Fallback to simple prefix stripping
			if len(host) > 8 && host[:8] == "https://" {
				host = host[8:]
			} else if len(host) > 7 && host[:7] == "http://" {
				host = host[7:]
			}
		}
	}

	// Remove port if present
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}

	// Remove any path components
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}

	// Basic check: is it an IP?
	if net.ParseIP(host) != nil {
		return nil
	}

	// Is it a valid hostname/domain?
	// Regex for hostname validation (RFC 1123 mostly)
	// Simple version: alphanumeric, dots, dashes.
	// Allow underscores? Technically usually not in hostnames, but internal docker ones yes.
	for _, r := range host {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '.' && r != '-' && r != '_' {
			// Allow ":" for IPv6 literals if not parsed by ParseIP? ParseIP handles IPv6.
			return errors.New("invalid hostname format")
		}
	}
	return nil
}

func (s *ProxyHostService) validateProxyHost(host *models.ProxyHost) error {
	host.DomainNames = CanonicalDomainNames(host.DomainNames)
	host.ForwardHost = strings.TrimSpace(host.ForwardHost)

	if host.DomainNames == "" {
		return errors.New("domain names is required")
	}

	if host.ForwardHost == "" {
		return errors.New("forward host is required")
	}

	// Basic hostname/IP validation
	target := host.ForwardHost

	// Strip protocol and extract hostname if URL format
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		if u, err := url.Parse(target); err == nil {
			target = u.Hostname()
		} else {
			// Fallback to simple prefix stripping
			target = strings.TrimPrefix(target, "http://")
			target = strings.TrimPrefix(target, "https://")
		}
	}

	// Strip port if present
	if h, _, err := net.SplitHostPort(target); err == nil {
		target = h
	}

	// Remove any path components
	if idx := strings.Index(target, "/"); idx != -1 {
		target = target[:idx]
	}

	// Validate target
	if net.ParseIP(target) == nil {
		// Not a valid IP, check hostname rules
		// Allow: a-z, 0-9, -, ., _ (for docker service names)
		validHostname := true
		for _, r := range target {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '.' && r != '-' && r != '_' {
				validHostname = false
				break
			}
		}
		if !validHostname {
			return errors.New("forward host must be a valid IP address or hostname")
		}
	}

	if host.UseDNSChallenge && host.DNSProviderID == nil {
		return errors.New("dns provider is required when use_dns_challenge is enabled")
	}

	return nil
}

// normalizeAdvancedConfig validates and normalizes a host's advanced_config
// JSON in place. An empty value is left untouched.
func normalizeAdvancedConfig(host *models.ProxyHost) error {
	if host.AdvancedConfig == "" {
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(host.AdvancedConfig), &parsed); err != nil {
		return fmt.Errorf("invalid advanced_config JSON: %w", err)
	}
	norm, err := json.Marshal(caddy.NormalizeAdvancedConfig(parsed))
	if err != nil {
		return fmt.Errorf("invalid advanced_config after normalization: %w", err)
	}
	host.AdvancedConfig = string(norm)
	return nil
}

// checkHostWrite runs the uniqueness and validation checks shared by Create
// and Update. It must run inside the write-locked transaction so the checks
// and the subsequent write are atomic. A non-nil certificate_id must reference
// an existing certificate.
func (s *ProxyHostService) checkHostWrite(tx *gorm.DB, host *models.ProxyHost, excludeID uint) error {
	if err := checkSameTableDomainConflict(tx, host.DomainNames, &models.ProxyHost{}, excludeID); err != nil {
		return err
	}
	if err := checkCrossTableDomainConflict(tx, host.DomainNames, &models.RedirectionHost{}); err != nil {
		return err
	}
	if err := s.validateProxyHost(host); err != nil {
		return err
	}
	if err := ensureCertificateExists(tx, host.CertificateID); err != nil {
		return err
	}
	return normalizeAdvancedConfig(host)
}

// Create validates and creates a new proxy host. The uniqueness checks and the
// insert run in one write-locked transaction (see WithWriteLock).
func (s *ProxyHostService) Create(host *models.ProxyHost) error {
	err := WithWriteLock(s.db, &models.ProxyHost{}, func(tx *gorm.DB) error {
		if err := s.checkHostWrite(tx, host, 0); err != nil {
			return err
		}
		return tx.Create(host).Error
	})
	if err != nil {
		return err
	}
	s.invalidateCertCache()
	return nil
}

// Update validates and updates an existing proxy host. The uniqueness checks
// and the write run in one write-locked transaction (see WithWriteLock).
func (s *ProxyHostService) Update(host *models.ProxyHost) error {
	err := WithWriteLock(s.db, &models.ProxyHost{}, func(tx *gorm.DB) error {
		if err := s.checkHostWrite(tx, host, host.ID); err != nil {
			return err
		}
		// Use Updates to handle nullable foreign keys properly
		// Must use Select to explicitly allow setting nullable fields to nil
		return tx.Model(&models.ProxyHost{}).
			Where("id = ?", host.ID).
			Select("*").
			Updates(host).Error
	})
	if err != nil {
		return err
	}
	s.invalidateCertCache()
	return nil
}

// Delete removes a proxy host.
func (s *ProxyHostService) Delete(id uint) error {
	if err := s.db.Delete(&models.ProxyHost{}, id).Error; err != nil {
		return err
	}
	s.invalidateCertCache()
	return nil
}

// GetByID retrieves a proxy host by ID.
func (s *ProxyHostService) GetByID(id uint) (*models.ProxyHost, error) {
	var host models.ProxyHost
	if err := s.db.Where("id = ?", id).First(&host).Error; err != nil {
		return nil, err
	}
	return &host, nil
}

// GetByUUID finds a proxy host by UUID.
func (s *ProxyHostService) GetByUUID(uuidStr string) (*models.ProxyHost, error) {
	var host models.ProxyHost
	if err := s.db.Preload("Locations").Preload("Certificate").Preload("AccessList").Preload("SecurityHeaderProfile").Preload("ProxyGroup").Where("uuid = ?", uuidStr).First(&host).Error; err != nil {
		return nil, err
	}
	return &host, nil
}

// List returns all proxy hosts.
func (s *ProxyHostService) List() ([]models.ProxyHost, error) {
	var hosts []models.ProxyHost
	if err := s.db.Preload("Locations").Preload("Certificate").Preload("AccessList").Preload("SecurityHeaderProfile").Preload("ProxyGroup").Order("updated_at desc").Find(&hosts).Error; err != nil {
		return nil, err
	}
	return hosts, nil
}

// TestConnection attempts to connect to the target host and port.
func (s *ProxyHostService) TestConnection(host string, port int) error {
	if host == "" || port <= 0 {
		return errors.New("invalid host or port")
	}

	target := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", target, 3*time.Second)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Log().WithError(err).Warn("failed to close tcp connection")
		}
	}()

	return nil
}

// DB returns the underlying database instance for advanced operations.
func (s *ProxyHostService) DB() *gorm.DB {
	return s.db
}
