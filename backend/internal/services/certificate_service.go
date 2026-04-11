package services

import (
	"context"
	crand "crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// ErrCertInUse is returned when a certificate is linked to one or more proxy hosts.
var ErrCertInUse = fmt.Errorf("certificate is in use by one or more proxy hosts")

// ErrCertNotFound is returned when a certificate cannot be found by UUID.
var ErrCertNotFound = fmt.Errorf("certificate not found")

// CertificateInfo represents parsed certificate details for list responses.
type CertificateInfo struct {
	UUID         string    `json:"uuid"`
	Name         string    `json:"name,omitempty"`
	CommonName   string    `json:"common_name,omitempty"`
	Domains      string    `json:"domains"`
	Issuer       string    `json:"issuer"`
	IssuerOrg    string    `json:"issuer_org,omitempty"`
	Fingerprint  string    `json:"fingerprint,omitempty"`
	SerialNumber string    `json:"serial_number,omitempty"`
	KeyType      string    `json:"key_type,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	NotBefore    time.Time `json:"not_before,omitempty"`
	Status       string    `json:"status"`
	Provider     string    `json:"provider"`
	ChainDepth   int       `json:"chain_depth,omitempty"`
	HasKey       bool      `json:"has_key"`
	InUse        bool      `json:"in_use"`
}

// AssignedHostInfo represents a proxy host assigned to a certificate.
type AssignedHostInfo struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	DomainNames string `json:"domain_names"`
}

// ChainEntry represents a single certificate in the chain.
type ChainEntry struct {
	Subject   string    `json:"subject"`
	Issuer    string    `json:"issuer"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CertificateDetail contains full certificate metadata for detail responses.
type CertificateDetail struct {
	UUID          string             `json:"uuid"`
	Name          string             `json:"name,omitempty"`
	CommonName    string             `json:"common_name,omitempty"`
	Domains       string             `json:"domains"`
	Issuer        string             `json:"issuer"`
	IssuerOrg     string             `json:"issuer_org,omitempty"`
	Fingerprint   string             `json:"fingerprint,omitempty"`
	SerialNumber  string             `json:"serial_number,omitempty"`
	KeyType       string             `json:"key_type,omitempty"`
	ExpiresAt     time.Time          `json:"expires_at"`
	NotBefore     time.Time          `json:"not_before,omitempty"`
	Status        string             `json:"status"`
	Provider      string             `json:"provider"`
	ChainDepth    int                `json:"chain_depth,omitempty"`
	HasKey        bool               `json:"has_key"`
	InUse         bool               `json:"in_use"`
	AssignedHosts []AssignedHostInfo `json:"assigned_hosts"`
	Chain         []ChainEntry       `json:"chain"`
	AutoRenew     bool               `json:"auto_renew"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

// CertificateService manages certificate retrieval and parsing.
type CertificateService struct {
	dataDir     string
	db          *gorm.DB
	encSvc      *crypto.EncryptionService
	cache       []CertificateInfo
	cacheMu     sync.RWMutex
	lastScan    time.Time
	scanTTL     time.Duration
	initialized bool
}

// NewCertificateService creates a new certificate service.
func NewCertificateService(dataDir string, db *gorm.DB, encSvc *crypto.EncryptionService) *CertificateService {
	svc := &CertificateService{
		dataDir: dataDir,
		db:      db,
		encSvc:  encSvc,
		scanTTL: 5 * time.Minute,
	}
	return svc
}

// SyncFromDisk scans the certificate directory and syncs with database.
// This is called on startup and can be triggered manually for refresh.
func (s *CertificateService) SyncFromDisk() error {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	certRoot := filepath.Join(s.dataDir, "certificates")
	logger.Log().WithField("certRoot", util.SanitizeForLog(certRoot)).Info("CertificateService: scanning cert directory")

	foundDomains := map[string]struct{}{}

	// If the cert root does not exist, skip scanning but still return DB entries below
	if _, err := os.Stat(certRoot); err == nil {
		_ = filepath.Walk(certRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				logger.Log().WithField("path", util.SanitizeForLog(path)).WithError(err).Error("CertificateService: walk error")
				return nil
			}

			if !info.IsDir() && strings.HasSuffix(info.Name(), ".crt") {
				// #nosec G304 -- path is controlled by filepath.Walk starting from certRoot
				certData, err := os.ReadFile(path)
				if err != nil {
					logger.Log().WithField("path", util.SanitizeForLog(path)).WithError(err).Error("CertificateService: failed to read cert file")
					return nil
				}

				block, _ := pem.Decode(certData)
				if block == nil {
					// Silently skip invalid PEM files
					return nil
				}

				cert, err := x509.ParseCertificate(block.Bytes)
				if err != nil {
					logger.Log().WithField("path", util.SanitizeForLog(path)).WithError(err).Error("CertificateService: failed to parse cert")
					return nil
				}

				domain := cert.Subject.CommonName
				if domain == "" && len(cert.DNSNames) > 0 {
					domain = cert.DNSNames[0]
				}
				if domain == "" {
					return nil
				}

				foundDomains[domain] = struct{}{}

				// Determine expiry
				expiresAt := cert.NotAfter

				// Detect if this is a staging certificate by checking the path
				// Staging certs are in acme-staging-v02.api.letsencrypt.org-directory
				provider := "letsencrypt"
				if strings.Contains(path, "acme-staging") {
					provider = "letsencrypt-staging"
				}

				// Upsert into DB
				var existing models.SSLCertificate
				res := s.db.Where("domains = ?", domain).First(&existing)
				if res.Error != nil {
					if res.Error == gorm.ErrRecordNotFound {
						// Create new record
						now := time.Now()
						newCert := models.SSLCertificate{
							UUID:        uuid.New().String(),
							Name:        domain,
							Provider:    provider,
							Domains:     domain,
							Certificate: string(certData),
							PrivateKey:  "",
							ExpiresAt:   &expiresAt,
							AutoRenew:   true,
							CreatedAt:   now,
							UpdatedAt:   now,
						}
						if err := s.db.Create(&newCert).Error; err != nil {
							logger.Log().WithField("domain", util.SanitizeForLog(domain)).WithError(err).Error("CertificateService: failed to create DB cert")
						}
					} else {
						logger.Log().WithField("domain", util.SanitizeForLog(domain)).WithError(res.Error).Error("CertificateService: db error querying cert")
					}
				} else {
					// Update expiry/certificate content and provider if changed
					// But only upgrade staging->production, never downgrade production->staging
					updated := false
					existing.ExpiresAt = &expiresAt

					// Determine if we should update the cert
					// Production certs always win over staging certs
					isExistingStaging := strings.Contains(existing.Provider, "staging")
					isNewStaging := strings.Contains(provider, "staging")
					shouldUpdateCert := false

					switch {
					case isExistingStaging && !isNewStaging:
						// Upgrade from staging to production - always update
						shouldUpdateCert = true
					case !isExistingStaging && isNewStaging:
						// Don't downgrade from production to staging - skip
					case existing.Certificate != string(certData):
						// Same type but different content - update
						shouldUpdateCert = true
					}

					if shouldUpdateCert {
						existing.Certificate = string(certData)
						existing.Provider = provider
						updated = true
					}
					if updated {
						existing.UpdatedAt = time.Now()
						if err := s.db.Save(&existing).Error; err != nil {
							logger.Log().WithField("domain", util.SanitizeForLog(domain)).WithError(err).Error("CertificateService: failed to update DB cert")
						}
					} else {
						// still update ExpiresAt if needed
						if err := s.db.Model(&existing).Update("expires_at", &expiresAt).Error; err != nil {
							logger.Log().WithField("domain", util.SanitizeForLog(domain)).WithError(err).Error("CertificateService: failed to update expiry")
						}
					}
				}
			}
			return nil
		})
	} else {
		if os.IsNotExist(err) {
			logger.Log().WithField("certRoot", certRoot).Info("CertificateService: cert directory does not exist")
		} else {
			logger.Log().WithError(err).Error("CertificateService: failed to stat cert directory")
		}
	}

	// Delete stale DB entries for ACME certs not found on disk
	var acmeCerts []models.SSLCertificate
	if err := s.db.Where("provider LIKE ?", "letsencrypt%").Find(&acmeCerts).Error; err == nil {
		for _, c := range acmeCerts {
			if _, ok := foundDomains[c.Domains]; !ok {
				// remove stale record
				if err := s.db.Delete(&models.SSLCertificate{}, "id = ?", c.ID).Error; err != nil {
					logger.Log().WithField("domain", util.SanitizeForLog(c.Domains)).WithError(err).Error("CertificateService: failed to delete stale cert")
				} else {
					logger.Log().WithField("domain", util.SanitizeForLog(c.Domains)).Info("CertificateService: removed stale DB cert")
				}
			}
		}
	}

	// Update cache from DB
	if err := s.refreshCacheFromDB(); err != nil {
		return fmt.Errorf("failed to refresh cache: %w", err)
	}

	s.lastScan = time.Now()
	s.initialized = true
	logger.Log().WithField("count", len(s.cache)).Info("CertificateService: disk sync complete")
	return nil
}

// refreshCacheFromDB updates the in-memory cache from the database.
// Must be called with cacheMu held.
func (s *CertificateService) refreshCacheFromDB() error {
	var dbCerts []models.SSLCertificate
	if err := s.db.Find(&dbCerts).Error; err != nil {
		return fmt.Errorf("failed to fetch certs from DB: %w", err)
	}

	// Build a set of certificate IDs that are in use
	certInUse := make(map[uint]bool)
	var proxyHosts []models.ProxyHost
	s.db.Find(&proxyHosts)
	domainToName := make(map[string]string)
	for _, ph := range proxyHosts {
		if ph.CertificateID != nil {
			certInUse[*ph.CertificateID] = true
		}
		if ph.Name == "" {
			continue
		}
		domains := strings.Split(ph.DomainNames, ",")
		for _, d := range domains {
			d = strings.TrimSpace(strings.ToLower(d))
			if d != "" {
				domainToName[d] = ph.Name
			}
		}
	}

	certs := make([]CertificateInfo, 0, len(dbCerts))
	for _, c := range dbCerts {
		status := certStatus(c)

		expires := time.Time{}
		if c.ExpiresAt != nil {
			expires = *c.ExpiresAt
		}

		notBefore := time.Time{}
		if c.NotBefore != nil {
			notBefore = *c.NotBefore
		}

		// Try to get name from proxy host, fall back to cert name or domain
		name := c.Name
		certDomains := strings.Split(c.Domains, ",")
		for _, d := range certDomains {
			d = strings.TrimSpace(strings.ToLower(d))
			if phName, ok := domainToName[d]; ok {
				name = phName
				break
			}
		}

		chainDepth := 0
		if c.CertificateChain != "" {
			rest := []byte(c.CertificateChain)
			for {
				var block *pem.Block
				block, rest = pem.Decode(rest)
				if block == nil {
					break
				}
				chainDepth++
			}
		}

		certs = append(certs, CertificateInfo{
			UUID:         c.UUID,
			Name:         name,
			CommonName:   c.CommonName,
			Domains:      c.Domains,
			Issuer:       c.Provider,
			IssuerOrg:    c.IssuerOrg,
			Fingerprint:  c.Fingerprint,
			SerialNumber: c.SerialNumber,
			KeyType:      c.KeyType,
			ExpiresAt:    expires,
			NotBefore:    notBefore,
			Status:       status,
			Provider:     c.Provider,
			ChainDepth:   chainDepth,
			HasKey:       c.PrivateKeyEncrypted != "",
			InUse:        certInUse[c.ID],
		})
	}

	s.cache = certs
	return nil
}

func certStatus(c models.SSLCertificate) string {
	if strings.Contains(c.Provider, "staging") {
		return "untrusted"
	}
	if c.ExpiresAt != nil {
		if time.Now().After(*c.ExpiresAt) {
			return "expired"
		}
		if time.Now().AddDate(0, 0, 30).After(*c.ExpiresAt) {
			return "expiring"
		}
	}
	return "valid"
}

// ListCertificates returns cached certificate info.
// Fast path: returns from cache if available.
// Triggers background rescan if cache is stale.
func (s *CertificateService) ListCertificates() ([]CertificateInfo, error) {
	s.cacheMu.RLock()
	if s.initialized && time.Since(s.lastScan) < s.scanTTL {
		// Cache is fresh, return it
		result := make([]CertificateInfo, len(s.cache))
		copy(result, s.cache)
		s.cacheMu.RUnlock()
		return result, nil
	}
	s.cacheMu.RUnlock()

	// Cache is stale or not initialized - need to refresh
	// If not initialized, do a blocking sync
	if !s.initialized {
		if err := s.SyncFromDisk(); err != nil {
			// Fall back to DB query
			s.cacheMu.Lock()
			err := s.refreshCacheFromDB()
			s.cacheMu.Unlock()
			if err != nil {
				return nil, err
			}
		}
	} else {
		// Trigger background rescan for stale cache
		go func() {
			if err := s.SyncFromDisk(); err != nil {
				logger.Log().WithError(err).Error("CertificateService: background sync failed")
			}
		}()
	}

	// Return current cache (may be slightly stale)
	s.cacheMu.RLock()
	result := make([]CertificateInfo, len(s.cache))
	copy(result, s.cache)
	s.cacheMu.RUnlock()
	return result, nil
}

// InvalidateCache clears the cache, forcing a blocking resync on next ListCertificates call.
func (s *CertificateService) InvalidateCache() {
	s.cacheMu.Lock()
	s.lastScan = time.Time{}
	s.initialized = false // Force blocking resync
	s.cache = nil
	s.cacheMu.Unlock()
}

// UploadCertificate saves a new custom certificate with full validation and encryption.
func (s *CertificateService) UploadCertificate(name, certPEM, keyPEM, chainPEM string) (*CertificateInfo, error) {
	parsed, err := ParseCertificateInput([]byte(certPEM), []byte(keyPEM), []byte(chainPEM), "")
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate input: %w", err)
	}

	// Validate key matches certificate if key is provided
	if parsed.PrivateKey != nil {
		if err := ValidateKeyMatch(parsed.Leaf, parsed.PrivateKey); err != nil {
			return nil, fmt.Errorf("key validation failed: %w", err)
		}
	}

	// Extract metadata
	meta := ExtractCertificateMetadata(parsed.Leaf)

	domains := meta.CommonName
	if len(parsed.Leaf.DNSNames) > 0 {
		domains = strings.Join(parsed.Leaf.DNSNames, ",")
	}

	notAfter := parsed.Leaf.NotAfter
	notBefore := parsed.Leaf.NotBefore

	sslCert := &models.SSLCertificate{
		UUID:             uuid.New().String(),
		Name:             name,
		Provider:         "custom",
		Domains:          domains,
		CommonName:       meta.CommonName,
		Certificate:      parsed.CertPEM,
		CertificateChain: parsed.ChainPEM,
		Fingerprint:      meta.Fingerprint,
		SerialNumber:     meta.SerialNumber,
		IssuerOrg:        meta.IssuerOrg,
		KeyType:          meta.KeyType,
		ExpiresAt:        &notAfter,
		NotBefore:        &notBefore,
		KeyVersion:       1,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Encrypt private key at rest
	if parsed.KeyPEM != "" && s.encSvc != nil {
		encrypted, err := s.encSvc.Encrypt([]byte(parsed.KeyPEM))
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt private key: %w", err)
		}
		sslCert.PrivateKeyEncrypted = encrypted
	}

	if err := s.db.Create(sslCert).Error; err != nil {
		return nil, fmt.Errorf("failed to save certificate: %w", err)
	}

	s.InvalidateCache()

	chainDepth := len(parsed.Intermediates)

	info := &CertificateInfo{
		UUID:         sslCert.UUID,
		Name:         sslCert.Name,
		CommonName:   sslCert.CommonName,
		Domains:      sslCert.Domains,
		Issuer:       sslCert.Provider,
		IssuerOrg:    sslCert.IssuerOrg,
		Fingerprint:  sslCert.Fingerprint,
		SerialNumber: sslCert.SerialNumber,
		KeyType:      sslCert.KeyType,
		ExpiresAt:    notAfter,
		NotBefore:    notBefore,
		Status:       certStatus(*sslCert),
		Provider:     sslCert.Provider,
		ChainDepth:   chainDepth,
		HasKey:       sslCert.PrivateKeyEncrypted != "",
		InUse:        false,
	}

	return info, nil
}

// GetCertificate returns full certificate detail by UUID.
func (s *CertificateService) GetCertificate(certUUID string) (*CertificateDetail, error) {
	var cert models.SSLCertificate
	if err := s.db.Where("uuid = ?", certUUID).First(&cert).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrCertNotFound
		}
		return nil, fmt.Errorf("failed to fetch certificate: %w", err)
	}

	// Get assigned hosts
	var hosts []models.ProxyHost
	s.db.Where("certificate_id = ?", cert.ID).Find(&hosts)
	assignedHosts := make([]AssignedHostInfo, 0, len(hosts))
	for _, h := range hosts {
		assignedHosts = append(assignedHosts, AssignedHostInfo{
			UUID:        h.UUID,
			Name:        h.Name,
			DomainNames: h.DomainNames,
		})
	}

	// Parse chain entries
	chain := buildChainEntries(cert.Certificate, cert.CertificateChain)

	expires := time.Time{}
	if cert.ExpiresAt != nil {
		expires = *cert.ExpiresAt
	}
	notBefore := time.Time{}
	if cert.NotBefore != nil {
		notBefore = *cert.NotBefore
	}

	detail := &CertificateDetail{
		UUID:          cert.UUID,
		Name:          cert.Name,
		CommonName:    cert.CommonName,
		Domains:       cert.Domains,
		Issuer:        cert.Provider,
		IssuerOrg:     cert.IssuerOrg,
		Fingerprint:   cert.Fingerprint,
		SerialNumber:  cert.SerialNumber,
		KeyType:       cert.KeyType,
		ExpiresAt:     expires,
		NotBefore:     notBefore,
		Status:        certStatus(cert),
		Provider:      cert.Provider,
		ChainDepth:    len(chain),
		HasKey:        cert.PrivateKeyEncrypted != "",
		InUse:         len(hosts) > 0,
		AssignedHosts: assignedHosts,
		Chain:         chain,
		AutoRenew:     cert.AutoRenew,
		CreatedAt:     cert.CreatedAt,
		UpdatedAt:     cert.UpdatedAt,
	}

	return detail, nil
}

// ValidateCertificate validates certificate data without storing.
func (s *CertificateService) ValidateCertificate(certPEM, keyPEM, chainPEM string) (*ValidationResult, error) {
	result := &ValidationResult{
		Warnings: []string{},
		Errors:   []string{},
	}

	parsed, err := ParseCertificateInput([]byte(certPEM), []byte(keyPEM), []byte(chainPEM), "")
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, nil
	}

	meta := ExtractCertificateMetadata(parsed.Leaf)
	result.CommonName = meta.CommonName
	result.Domains = meta.Domains
	result.IssuerOrg = meta.IssuerOrg
	result.ExpiresAt = meta.NotAfter
	result.ChainDepth = len(parsed.Intermediates)

	// Key match check
	if parsed.PrivateKey != nil {
		if err := ValidateKeyMatch(parsed.Leaf, parsed.PrivateKey); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("key mismatch: %s", err.Error()))
		} else {
			result.KeyMatch = true
		}
	}

	// Chain validation (best-effort, warn on failure)
	if len(parsed.Intermediates) > 0 {
		if err := ValidateChain(parsed.Leaf, parsed.Intermediates); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("chain validation: %s", err.Error()))
		} else {
			result.ChainValid = true
		}
	} else {
		// Try verifying with system roots
		if err := ValidateChain(parsed.Leaf, nil); err != nil {
			result.Warnings = append(result.Warnings, "certificate could not be verified against system roots")
		} else {
			result.ChainValid = true
		}
	}

	// Expiry warnings
	daysUntilExpiry := time.Until(parsed.Leaf.NotAfter).Hours() / 24
	if daysUntilExpiry < 0 {
		result.Warnings = append(result.Warnings, "Certificate has expired")
	} else if daysUntilExpiry < 30 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Certificate expires in %.0f days", daysUntilExpiry))
	}

	result.Valid = len(result.Errors) == 0
	return result, nil
}

// IsCertificateInUse checks if a certificate is referenced by any proxy host.
func (s *CertificateService) IsCertificateInUse(id uint) (bool, error) {
	var count int64
	if err := s.db.Model(&models.ProxyHost{}).Where("certificate_id = ?", id).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check certificate linkage: %w", err)
	}
	return count > 0, nil
}

// IsCertificateInUseByUUID checks if a certificate is referenced by any proxy host, looked up by UUID.
func (s *CertificateService) IsCertificateInUseByUUID(certUUID string) (bool, error) {
	var cert models.SSLCertificate
	if err := s.db.Where("uuid = ?", certUUID).First(&cert).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, ErrCertNotFound
		}
		return false, fmt.Errorf("failed to look up certificate: %w", err)
	}
	return s.IsCertificateInUse(cert.ID)
}

// DeleteCertificate removes a certificate by UUID.
func (s *CertificateService) DeleteCertificate(certUUID string) error {
	var cert models.SSLCertificate
	if err := s.db.Where("uuid = ?", certUUID).First(&cert).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrCertNotFound
		}
		return fmt.Errorf("failed to look up certificate: %w", err)
	}

	// Prevent deletion if the certificate is referenced by any proxy host
	inUse, err := s.IsCertificateInUse(cert.ID)
	if err != nil {
		return err
	}
	if inUse {
		return ErrCertInUse
	}

	if cert.Provider == "letsencrypt" || cert.Provider == "letsencrypt-staging" {
		// Best-effort file deletion
		certRoot := filepath.Join(s.dataDir, "certificates")
		_ = filepath.Walk(certRoot, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".crt") {
				if info.Name() == cert.Domains+".crt" {
					logger.Log().WithField("path", path).Info("CertificateService: deleting ACME cert file")
					if err := os.Remove(path); err != nil {
						logger.Log().WithError(err).Error("CertificateService: failed to delete cert file")
					}
					keyPath := strings.TrimSuffix(path, ".crt") + ".key"
					if _, err := os.Stat(keyPath); err == nil {
						if err := os.Remove(keyPath); err != nil {
							logger.Log().WithError(err).Warn("Failed to remove key file")
						}
					}
					jsonPath := strings.TrimSuffix(path, ".crt") + ".json"
					if _, err := os.Stat(jsonPath); err == nil {
						if err := os.Remove(jsonPath); err != nil {
							logger.Log().WithError(err).Warn("Failed to remove JSON file")
						}
					}
				}
			}
			return nil
		})
	}

	if err := s.db.Delete(&models.SSLCertificate{}, "id = ?", cert.ID).Error; err != nil {
		return fmt.Errorf("failed to delete certificate: %w", err)
	}
	s.InvalidateCache()
	return nil
}

// ExportCertificate exports a certificate in the requested format.
// Returns the file data, suggested filename, and any error.
func (s *CertificateService) ExportCertificate(certUUID string, format string, includeKey bool) ([]byte, string, error) {
	var cert models.SSLCertificate
	if err := s.db.Where("uuid = ?", certUUID).First(&cert).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, "", ErrCertNotFound
		}
		return nil, "", fmt.Errorf("failed to fetch certificate: %w", err)
	}

	baseName := cert.Name
	if baseName == "" {
		baseName = "certificate"
	}

	switch strings.ToLower(format) {
	case "pem":
		var buf strings.Builder
		buf.WriteString(cert.Certificate)
		if cert.CertificateChain != "" {
			buf.WriteString("\n")
			buf.WriteString(cert.CertificateChain)
		}
		if includeKey {
			keyPEM, err := s.GetDecryptedPrivateKey(&cert)
			if err != nil {
				return nil, "", fmt.Errorf("failed to decrypt private key: %w", err)
			}
			buf.WriteString("\n")
			buf.WriteString(keyPEM)
		}
		return []byte(buf.String()), baseName + ".pem", nil

	case "der":
		derData, err := ConvertPEMToDER(cert.Certificate)
		if err != nil {
			return nil, "", fmt.Errorf("failed to convert to DER: %w", err)
		}
		return derData, baseName + ".der", nil

	case "pfx", "p12":
		keyPEM, err := s.GetDecryptedPrivateKey(&cert)
		if err != nil {
			return nil, "", fmt.Errorf("failed to decrypt private key for PFX: %w", err)
		}
		pfxData, err := ConvertPEMToPFX(cert.Certificate, keyPEM, cert.CertificateChain, "")
		if err != nil {
			return nil, "", fmt.Errorf("failed to create PFX: %w", err)
		}
		return pfxData, baseName + ".pfx", nil

	default:
		return nil, "", fmt.Errorf("unsupported export format: %s", format)
	}
}

// GetDecryptedPrivateKey decrypts and returns the private key PEM for internal use.
func (s *CertificateService) GetDecryptedPrivateKey(cert *models.SSLCertificate) (string, error) {
	if cert.PrivateKeyEncrypted == "" {
		return "", fmt.Errorf("no encrypted private key stored")
	}
	if s.encSvc == nil {
		return "", fmt.Errorf("encryption service not configured")
	}

	decrypted, err := s.encSvc.Decrypt(cert.PrivateKeyEncrypted)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt private key: %w", err)
	}

	return string(decrypted), nil
}

// MigratePrivateKeys encrypts existing plaintext private keys.
// Idempotent — skips already-migrated rows.
func (s *CertificateService) MigratePrivateKeys() error {
	if s.encSvc == nil {
		logger.Log().Warn("CertificateService: encryption service not configured, skipping key migration")
		return nil
	}

	// Use raw SQL because PrivateKey has gorm:"-" tag
	type rawCert struct {
		ID            uint
		PrivateKey    string
		PrivateKeyEnc string `gorm:"column:private_key_enc"`
	}

	var certs []rawCert
	if err := s.db.Raw("SELECT id, private_key, private_key_enc FROM ssl_certificates WHERE private_key != '' AND (private_key_enc = '' OR private_key_enc IS NULL)").Scan(&certs).Error; err != nil {
		return fmt.Errorf("failed to query certificates for migration: %w", err)
	}

	if len(certs) == 0 {
		logger.Log().Info("CertificateService: no private keys to migrate")
		return nil
	}

	logger.Log().WithField("count", len(certs)).Info("CertificateService: migrating plaintext private keys")

	for _, c := range certs {
		encrypted, err := s.encSvc.Encrypt([]byte(c.PrivateKey))
		if err != nil {
			logger.Log().WithField("cert_id", c.ID).WithError(err).Error("CertificateService: failed to encrypt key during migration")
			continue
		}

		if err := s.db.Exec("UPDATE ssl_certificates SET private_key_enc = ?, key_version = 1, private_key = '' WHERE id = ?", encrypted, c.ID).Error; err != nil {
			logger.Log().WithField("cert_id", c.ID).WithError(err).Error("CertificateService: failed to update migrated key")
			continue
		}

		logger.Log().WithField("cert_id", c.ID).Info("CertificateService: migrated private key")
	}

	return nil
}

// DeleteCertificateByID removes a certificate by numeric ID (legacy compatibility).
func (s *CertificateService) DeleteCertificateByID(id uint) error {
	var cert models.SSLCertificate
	if err := s.db.Where("id = ?", id).First(&cert).Error; err != nil {
		return fmt.Errorf("failed to look up certificate: %w", err)
	}
	return s.DeleteCertificate(cert.UUID)
}

// UpdateCertificate updates certificate metadata (name only) by UUID.
func (s *CertificateService) UpdateCertificate(certUUID string, name string) (*CertificateInfo, error) {
	var cert models.SSLCertificate
	if err := s.db.Where("uuid = ?", certUUID).First(&cert).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrCertNotFound
		}
		return nil, fmt.Errorf("failed to fetch certificate: %w", err)
	}

	cert.Name = name
	if err := s.db.Save(&cert).Error; err != nil {
		return nil, fmt.Errorf("failed to update certificate: %w", err)
	}

	s.InvalidateCache()

	expires := time.Time{}
	if cert.ExpiresAt != nil {
		expires = *cert.ExpiresAt
	}
	notBefore := time.Time{}
	if cert.NotBefore != nil {
		notBefore = *cert.NotBefore
	}

	var chainDepth int
	if cert.CertificateChain != "" {
		certs, _ := parsePEMCertificates([]byte(cert.CertificateChain))
		chainDepth = len(certs)
	}

	inUse, _ := s.IsCertificateInUse(cert.ID)

	return &CertificateInfo{
		UUID:         cert.UUID,
		Name:         cert.Name,
		CommonName:   cert.CommonName,
		Domains:      cert.Domains,
		Issuer:       cert.Provider,
		IssuerOrg:    cert.IssuerOrg,
		Fingerprint:  cert.Fingerprint,
		SerialNumber: cert.SerialNumber,
		KeyType:      cert.KeyType,
		ExpiresAt:    expires,
		NotBefore:    notBefore,
		Status:       certStatus(cert),
		Provider:     cert.Provider,
		ChainDepth:   chainDepth,
		HasKey:       cert.PrivateKeyEncrypted != "",
		InUse:        inUse,
	}, nil
}

// CheckExpiringCertificates returns certificates that are expiring within the given number of days.
func (s *CertificateService) CheckExpiringCertificates(warningDays int) ([]CertificateInfo, error) {
	var certs []models.SSLCertificate
	threshold := time.Now().Add(time.Duration(warningDays) * 24 * time.Hour)

	if err := s.db.Where("provider = ? AND expires_at IS NOT NULL AND expires_at <= ?", "custom", threshold).Find(&certs).Error; err != nil {
		return nil, fmt.Errorf("failed to query expiring certificates: %w", err)
	}

	result := make([]CertificateInfo, 0, len(certs))
	for _, cert := range certs {
		expires := time.Time{}
		if cert.ExpiresAt != nil {
			expires = *cert.ExpiresAt
		}
		notBefore := time.Time{}
		if cert.NotBefore != nil {
			notBefore = *cert.NotBefore
		}

		result = append(result, CertificateInfo{
			UUID:         cert.UUID,
			Name:         cert.Name,
			CommonName:   cert.CommonName,
			Domains:      cert.Domains,
			Issuer:       cert.Provider,
			IssuerOrg:    cert.IssuerOrg,
			Fingerprint:  cert.Fingerprint,
			SerialNumber: cert.SerialNumber,
			KeyType:      cert.KeyType,
			ExpiresAt:    expires,
			NotBefore:    notBefore,
			Status:       certStatus(cert),
			Provider:     cert.Provider,
			HasKey:       cert.PrivateKeyEncrypted != "",
		})
	}

	return result, nil
}

// StartExpiryChecker runs a background goroutine that periodically checks for expiring certificates.
func (s *CertificateService) StartExpiryChecker(ctx context.Context, notificationSvc *NotificationService, warningDays int) {
	// Startup delay: avoid notification bursts during frequent restarts
	startupDelay := 5 * time.Minute
	select {
	case <-ctx.Done():
		return
	case <-time.After(startupDelay):
	}

	// Add random jitter (0-60 minutes) using crypto/rand
	maxJitter := int64(60 * time.Minute)
	n, errRand := crand.Int(crand.Reader, big.NewInt(maxJitter))
	if errRand != nil {
		n = big.NewInt(maxJitter / 2)
	}
	jitter := time.Duration(n.Int64())
	select {
	case <-ctx.Done():
		return
	case <-time.After(jitter):
	}

	s.checkExpiry(ctx, notificationSvc, warningDays)

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkExpiry(ctx, notificationSvc, warningDays)
		}
	}
}

func (s *CertificateService) checkExpiry(ctx context.Context, notificationSvc *NotificationService, warningDays int) {
	if notificationSvc == nil {
		return
	}

	certs, err := s.CheckExpiringCertificates(warningDays)
	if err != nil {
		logger.Log().WithError(err).Error("CertificateService: failed to check expiring certificates")
		return
	}

	for _, cert := range certs {
		daysLeft := time.Until(cert.ExpiresAt).Hours() / 24

		if daysLeft < 0 {
			// Expired
			if _, err := notificationSvc.Create(
				models.NotificationTypeError,
				"Certificate Expired",
				fmt.Sprintf("Certificate %q (%s) has expired.", cert.Name, cert.Domains),
			); err != nil {
				logger.Log().WithError(err).Error("CertificateService: failed to create expiry notification")
			}
			notificationSvc.SendExternal(ctx,
				"cert_expiry",
				"Certificate Expired",
				fmt.Sprintf("Certificate %q (%s) has expired.", cert.Name, cert.Domains),
				map[string]any{"uuid": cert.UUID, "domains": cert.Domains, "status": "expired"},
			)
		} else {
			// Expiring soon
			if _, err := notificationSvc.Create(
				models.NotificationTypeWarning,
				"Certificate Expiring Soon",
				fmt.Sprintf("Certificate %q (%s) expires in %.0f days.", cert.Name, cert.Domains, daysLeft),
			); err != nil {
				logger.Log().WithError(err).Error("CertificateService: failed to create expiry warning notification")
			}
			notificationSvc.SendExternal(ctx,
				"cert_expiry",
				"Certificate Expiring Soon",
				fmt.Sprintf("Certificate %q (%s) expires in %.0f days.", cert.Name, cert.Domains, daysLeft),
				map[string]any{"uuid": cert.UUID, "domains": cert.Domains, "days_left": int(daysLeft)},
			)
		}
	}
}

func buildChainEntries(certPEM, chainPEM string) []ChainEntry {
	var entries []ChainEntry

	// Parse leaf
	if certPEM != "" {
		certs, _ := parsePEMCertificates([]byte(certPEM))
		for _, c := range certs {
			entries = append(entries, ChainEntry{
				Subject:   c.Subject.CommonName,
				Issuer:    c.Issuer.CommonName,
				ExpiresAt: c.NotAfter,
			})
		}
	}

	// Parse chain
	if chainPEM != "" {
		certs, _ := parsePEMCertificates([]byte(chainPEM))
		for _, c := range certs {
			entries = append(entries, ChainEntry{
				Subject:   c.Subject.CommonName,
				Issuer:    c.Issuer.CommonName,
				ExpiresAt: c.NotAfter,
			})
		}
	}

	return entries
}
