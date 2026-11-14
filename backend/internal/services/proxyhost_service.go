package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

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

	if err := refuseAdminListenerTarget(host.ForwardHost, host.ForwardPort); err != nil {
		return err
	}

	if err := checkForwardHostForm(host.ForwardHost); err != nil {
		return err
	}

	// Basic hostname/IP validation
	target := forwardTargetHost(host.ForwardHost)

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
	norm, err := normalizeAdvancedJSON(host.AdvancedConfig)
	if err != nil {
		return err
	}
	host.AdvancedConfig = norm
	return nil
}

// normalizeAdvancedJSON parses advanced_config JSON and returns its normalized
// encoding.
func normalizeAdvancedJSON(raw string) (string, error) {
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "", fmt.Errorf("invalid advanced_config JSON: %w", err)
	}
	norm, err := json.Marshal(caddy.NormalizeAdvancedConfig(parsed))
	if err != nil {
		return "", fmt.Errorf("invalid advanced_config after normalization: %w", err)
	}
	return string(norm), nil
}

// checkHostWrite runs the uniqueness and validation checks shared by Create
// and Update. It must run inside the write-locked transaction so the checks
// and the subsequent write are atomic. A non-nil certificate_id must reference
// an existing certificate. When admin is false, the write must not change the
// admin-only fields relative to the stored row.
func (s *ProxyHostService) checkHostWrite(tx *gorm.DB, host *models.ProxyHost, excludeID uint, admin bool) error {
	if err := checkSameTableDomainConflict(tx, host.DomainNames, &models.ProxyHost{}, excludeID); err != nil {
		return err
	}
	if err := checkCrossTableDomainConflict(tx, host.DomainNames, &models.RedirectionHost{}); err != nil {
		return err
	}
	if err := s.validateProxyHost(host); err != nil {
		return err
	}
	if err := refuseListenerHostTargets(tx, host); err != nil {
		return err
	}
	if err := validateLocationTargets(host.Locations); err != nil {
		return err
	}
	if err := ensureCertificateExists(tx, host.CertificateID); err != nil {
		return err
	}
	if err := normalizeAdvancedConfig(host); err != nil {
		return err
	}
	stored, err := storedProxyAdminFields(tx, excludeID)
	if err != nil {
		return err
	}
	return ensureAdminOnlyFieldsUnchanged(admin, stored, proxyAdminFieldsOf(host))
}

// refuseListenerHostTargets rejects a host whose own forward target or any location
// target reaches an agent listener. It runs inside the caller's transaction.
func refuseListenerHostTargets(tx *gorm.DB, host *models.ProxyHost) error {
	if err := refuseOrthrusListenerTarget(tx, host.ForwardHost, host.ForwardPort); err != nil {
		return err
	}
	for i := range host.Locations {
		if err := refuseOrthrusListenerTarget(tx, host.Locations[i].ForwardHost, host.Locations[i].ForwardPort); err != nil {
			return err
		}
	}
	return nil
}

// proxyAdminFieldsOf returns the admin-only fields of an incoming proxy host.
func proxyAdminFieldsOf(host *models.ProxyHost) hostAdminFields {
	return hostAdminFields{
		dnsProviderID:   host.DNSProviderID,
		useDNSChallenge: host.UseDNSChallenge,
		advancedConfig:  host.AdvancedConfig,
		advancedBackup:  host.AdvancedConfigBackup,
	}
}

// errForwardHostForm is returned for a forward host that is not a host, an
// optional port and an optional http:// or https:// prefix.
var errForwardHostForm = errors.New("forward host must be a valid IP address or hostname")

// forwardHostSeparators are the characters a forward host may not contain after
// its optional scheme: path, query, fragment and user-info delimiters. Unix
// socket style values such as "unix//x" are refused through the path separator.
const forwardHostSeparators = "/\\@?#"

// checkForwardHostForm refuses any forward host that carries more than a host and
// an optional port. An optional http:// or https:// prefix is stripped first. The
// empty string passes, because the required-value check runs separately. The
// hostname rules run later; this check stops values that a later truncation would
// quietly shorten to a plausible host.
func checkForwardHostForm(raw string) error {
	if raw == "" {
		return nil
	}
	rest := strings.TrimPrefix(raw, "https://")
	if rest == raw {
		rest = strings.TrimPrefix(raw, "http://")
	}
	if rest == "" || strings.ContainsAny(rest, forwardHostSeparators) || strings.IndexFunc(rest, unicode.IsSpace) >= 0 {
		return errForwardHostForm
	}
	return nil
}

// validateLocationTargets applies the forward target rules to each location.
func validateLocationTargets(locations []models.Location) error {
	for i := range locations {
		if err := refuseAdminListenerTarget(locations[i].ForwardHost, locations[i].ForwardPort); err != nil {
			return err
		}
		if err := checkForwardHostForm(locations[i].ForwardHost); err != nil {
			return err
		}
	}
	return nil
}

// Create validates and creates a new proxy host. It is the trusted internal
// entry point (imports and setup) and applies no admin-only field restriction.
// HTTP handlers acting for a user must call CreateAs.
func (s *ProxyHostService) Create(host *models.ProxyHost) error {
	return s.CreateAs(host, true)
}

// CreateAs validates and creates a new proxy host on behalf of a caller. When
// admin is false, the DNS provider binding, the DNS challenge flag and the
// advanced config must stay empty. sentKeys are the JSON keys the client sent:
// the defaulted boolean fields among them are stored as sent (see
// CreateKeepingSent). The uniqueness checks and the insert run in one
// write-locked transaction (see WithWriteLock).
func (s *ProxyHostService) CreateAs(host *models.ProxyHost, admin bool, sentKeys ...string) error {
	err := WithWriteLock(s.db, &models.ProxyHost{}, func(tx *gorm.DB) error {
		if err := s.checkHostWrite(tx, host, 0, admin); err != nil {
			return err
		}
		sentColumns, err := SentDefaultColumns(tx, &models.ProxyHost{}, sentKeys, reflect.Bool)
		if err != nil {
			return err
		}
		return CreateKeepingSent(tx, host, sentColumns)
	})
	if err != nil {
		return err
	}
	s.invalidateCertCache()
	return nil
}

// Update validates and updates an existing proxy host. It is the trusted
// internal entry point and applies no admin-only field restriction. HTTP
// handlers acting for a user must call UpdateAs.
func (s *ProxyHostService) Update(host *models.ProxyHost) error {
	return s.UpdateAs(host, true)
}

// UpdateAs validates and updates an existing proxy host on behalf of a caller.
// When admin is false, the admin-only fields must match the stored row. The
// uniqueness checks and the write run in one write-locked transaction (see
// WithWriteLock).
func (s *ProxyHostService) UpdateAs(host *models.ProxyHost, admin bool) error {
	err := WithWriteLock(s.db, &models.ProxyHost{}, func(tx *gorm.DB) error {
		if err := s.checkHostWrite(tx, host, host.ID, admin); err != nil {
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
	if err := refuseOrthrusListenerTarget(s.db, host, port); err != nil {
		return err
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
