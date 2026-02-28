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
	host.DomainNames = strings.TrimSpace(host.DomainNames)
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

// Create validates and creates a new proxy host.
func (s *ProxyHostService) Create(host *models.ProxyHost) error {
	if err := s.ValidateUniqueDomain(host.DomainNames, 0); err != nil {
		return err
	}

	if err := s.validateProxyHost(host); err != nil {
		return err
	}

	// Normalize and validate advanced config (if present)
	if host.AdvancedConfig != "" {
		var parsed any
		if err := json.Unmarshal([]byte(host.AdvancedConfig), &parsed); err != nil {
			return fmt.Errorf("invalid advanced_config JSON: %w", err)
		}
		parsed = caddy.NormalizeAdvancedConfig(parsed)
		if norm, err := json.Marshal(parsed); err != nil {
			return fmt.Errorf("invalid advanced_config after normalization: %w", err)
		} else {
			host.AdvancedConfig = string(norm)
		}
	}

	return s.db.Create(host).Error
}

// Update validates and updates an existing proxy host.
func (s *ProxyHostService) Update(host *models.ProxyHost) error {
	if err := s.ValidateUniqueDomain(host.DomainNames, host.ID); err != nil {
		return err
	}

	if err := s.validateProxyHost(host); err != nil {
		return err
	}

	// Normalize and validate advanced config (if present)
	if host.AdvancedConfig != "" {
		var parsed any
		if err := json.Unmarshal([]byte(host.AdvancedConfig), &parsed); err != nil {
			return fmt.Errorf("invalid advanced_config JSON: %w", err)
		}
		parsed = caddy.NormalizeAdvancedConfig(parsed)
		if norm, err := json.Marshal(parsed); err != nil {
			return fmt.Errorf("invalid advanced_config after normalization: %w", err)
		} else {
			host.AdvancedConfig = string(norm)
		}
	}

	// Use Updates to handle nullable foreign keys properly
	// Must use Select to explicitly allow setting nullable fields to nil
	return s.db.Model(&models.ProxyHost{}).
		Where("id = ?", host.ID).
		Select("*").
		Updates(host).Error
}

// Delete removes a proxy host.
func (s *ProxyHostService) Delete(id uint) error {
	return s.db.Delete(&models.ProxyHost{}, id).Error
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
	if err := s.db.Preload("Locations").Preload("Certificate").Preload("AccessList").Preload("SecurityHeaderProfile").Where("uuid = ?", uuidStr).First(&host).Error; err != nil {
		return nil, err
	}
	return &host, nil
}

// List returns all proxy hosts.
func (s *ProxyHostService) List() ([]models.ProxyHost, error) {
	var hosts []models.ProxyHost
	if err := s.db.Preload("Locations").Preload("Certificate").Preload("AccessList").Preload("SecurityHeaderProfile").Order("updated_at desc").Find(&hosts).Error; err != nil {
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
