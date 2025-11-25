package services

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

// CertificateInfo represents parsed certificate details.
type CertificateInfo struct {
	ID        uint      `json:"id,omitempty"`
	UUID      string    `json:"uuid,omitempty"`
	Name      string    `json:"name,omitempty"`
	Domain    string    `json:"domain"`
	Issuer    string    `json:"issuer"`
	ExpiresAt time.Time `json:"expires_at"`
	Status    string    `json:"status"`   // "valid", "expiring", "expired", "untrusted"
	Provider  string    `json:"provider"` // "letsencrypt", "letsencrypt-staging", "custom"
}

// CertificateService manages certificate retrieval and parsing.
type CertificateService struct {
	dataDir string
	db      *gorm.DB
}

// NewCertificateService creates a new certificate service.
func NewCertificateService(dataDir string, db *gorm.DB) *CertificateService {
	return &CertificateService{
		dataDir: dataDir,
		db:      db,
	}
}

// ListCertificates returns both auto-generated and custom certificates.
func (s *CertificateService) ListCertificates() ([]CertificateInfo, error) {
	// First, scan Caddy data directory for auto-generated certificates and persist them.
	certRoot := filepath.Join(s.dataDir, "certificates")
	log.Printf("CertificateService: scanning cert directory: %s\n", certRoot)

	foundDomains := map[string]struct{}{}

	// If the cert root does not exist, skip scanning but still return DB entries below
	if _, err := os.Stat(certRoot); err == nil {
		_ = filepath.Walk(certRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Printf("CertificateService: walk error for %s: %v\n", path, err)
				return nil
			}

			if !info.IsDir() && strings.HasSuffix(info.Name(), ".crt") {
				log.Printf("CertificateService: found cert file: %s\n", path)
				certData, err := os.ReadFile(path)
				if err != nil {
					log.Printf("CertificateService: failed to read cert file %s: %v\n", path, err)
					return nil
				}

				block, _ := pem.Decode(certData)
				if block == nil {
					log.Printf("CertificateService: pem decode failed for %s\n", path)
					return nil
				}

				cert, err := x509.ParseCertificate(block.Bytes)
				if err != nil {
					log.Printf("CertificateService: failed to parse cert %s: %v\n", path, err)
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
					log.Printf("CertificateService: detected staging certificate for %s (path: %s)", domain, path)
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
							log.Printf("CertificateService: failed to create DB cert for %s: %v\n", domain, err)
						}
					} else {
						log.Printf("CertificateService: db error querying cert %s: %v\n", domain, res.Error)
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

					if isExistingStaging && !isNewStaging {
						// Upgrade from staging to production - always update
						shouldUpdateCert = true
						log.Printf("CertificateService: upgrading %s from staging to production cert", domain)
					} else if !isExistingStaging && isNewStaging {
						// Don't downgrade from production to staging - skip
						log.Printf("CertificateService: skipping staging cert for %s - production cert already exists", domain)
					} else if existing.Certificate != string(certData) {
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
							log.Printf("CertificateService: failed to update DB cert for %s: %v\n", domain, err)
						}
					} else {
						// still update ExpiresAt if needed
						if err := s.db.Model(&existing).Update("expires_at", &expiresAt).Error; err != nil {
							log.Printf("CertificateService: failed to update expiry for %s: %v\n", domain, err)
						}
					}
				}
			}
			return nil
		})
	} else {
		if os.IsNotExist(err) {
			log.Printf("CertificateService: cert directory does not exist: %s\n", certRoot)
		} else {
			log.Printf("CertificateService: failed to stat cert directory: %v\n", err)
		}
	}

	// Delete stale DB entries for ACME certs not found on disk
	var acmeCerts []models.SSLCertificate
	if err := s.db.Where("provider LIKE ?", "letsencrypt%").Find(&acmeCerts).Error; err == nil {
		for _, c := range acmeCerts {
			if _, ok := foundDomains[c.Domains]; !ok {
				// remove stale record
				if err := s.db.Delete(&models.SSLCertificate{}, "id = ?", c.ID).Error; err != nil {
					log.Printf("CertificateService: failed to delete stale cert %s: %v\n", c.Domains, err)
				} else {
					log.Printf("CertificateService: removed stale DB cert for %s\n", c.Domains)
				}
			}
		}
	}

	// Finally, fetch all certificates from DB to build the response (includes custom and persisted ACME)
	certs := []CertificateInfo{}
	var dbCerts []models.SSLCertificate
	if err := s.db.Find(&dbCerts).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch certs from DB: %w", err)
	}

	for _, c := range dbCerts {
		status := "valid"

		// Staging certificates are untrusted by browsers
		if strings.Contains(c.Provider, "staging") {
			status = "untrusted"
		} else if c.ExpiresAt != nil {
			if time.Now().After(*c.ExpiresAt) {
				status = "expired"
			} else if time.Now().AddDate(0, 0, 30).After(*c.ExpiresAt) {
				status = "expiring"
			}
		}

		expires := time.Time{}
		if c.ExpiresAt != nil {
			expires = *c.ExpiresAt
		}

		certs = append(certs, CertificateInfo{
			ID:        c.ID,
			UUID:      c.UUID,
			Name:      c.Name,
			Domain:    c.Domains,
			Issuer:    c.Provider,
			ExpiresAt: expires,
			Status:    status,
			Provider:  c.Provider,
		})
	}

	return certs, nil
}

// UploadCertificate saves a new custom certificate.
func (s *CertificateService) UploadCertificate(name, certPEM, keyPEM string) (*models.SSLCertificate, error) {
	// Validate PEM
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, fmt.Errorf("invalid certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Create DB entry
	sslCert := &models.SSLCertificate{
		UUID:        uuid.New().String(),
		Name:        name,
		Provider:    "custom",
		Domains:     cert.Subject.CommonName, // Or SANs
		Certificate: certPEM,
		PrivateKey:  keyPEM,
		ExpiresAt:   &cert.NotAfter,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Handle SANs if present
	if len(cert.DNSNames) > 0 {
		sslCert.Domains = strings.Join(cert.DNSNames, ",")
	}

	if err := s.db.Create(sslCert).Error; err != nil {
		return nil, err
	}

	return sslCert, nil
}

// DeleteCertificate removes a certificate.
func (s *CertificateService) DeleteCertificate(id uint) error {
	var cert models.SSLCertificate
	if err := s.db.First(&cert, id).Error; err != nil {
		return err
	}

	if cert.Provider == "letsencrypt" {
		// Best-effort file deletion
		certRoot := filepath.Join(s.dataDir, "certificates")
		_ = filepath.Walk(certRoot, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".crt") {
				if info.Name() == cert.Domains+".crt" {
					// Found it
					log.Printf("CertificateService: deleting ACME cert file %s", path)
					if err := os.Remove(path); err != nil {
						log.Printf("CertificateService: failed to delete cert file: %v", err)
					}
					// Try to delete key as well
					keyPath := strings.TrimSuffix(path, ".crt") + ".key"
					if _, err := os.Stat(keyPath); err == nil {
						os.Remove(keyPath)
					}
					// Also try to delete the json meta file
					jsonPath := strings.TrimSuffix(path, ".crt") + ".json"
					if _, err := os.Stat(jsonPath); err == nil {
						os.Remove(jsonPath)
					}
				}
			}
			return nil
		})
	}

	return s.db.Delete(&models.SSLCertificate{}, "id = ?", id).Error
}
