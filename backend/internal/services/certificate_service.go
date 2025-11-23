package services

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
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
	Status    string    `json:"status"`   // "valid", "expiring", "expired"
	Provider  string    `json:"provider"` // "letsencrypt", "custom"
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
	certs := []CertificateInfo{}

	// 1. Get Custom Certificates from DB
	var dbCerts []models.SSLCertificate
	if err := s.db.Find(&dbCerts).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch custom certs: %w", err)
	}

	for _, c := range dbCerts {
		status := "valid"
		if c.ExpiresAt != nil {
			if time.Now().After(*c.ExpiresAt) {
				status = "expired"
			} else if time.Now().AddDate(0, 0, 30).After(*c.ExpiresAt) {
				status = "expiring"
			}
		}

		certs = append(certs, CertificateInfo{
			ID:        c.ID,
			UUID:      c.UUID,
			Name:      c.Name,
			Domain:    c.Domains,
			Issuer:    c.Provider, // "custom" or "self-signed"
			ExpiresAt: *c.ExpiresAt,
			Status:    status,
			Provider:  c.Provider,
		})
	}

	// 2. Scan Caddy data directory for auto-generated certificates
	certRoot := filepath.Join(s.dataDir, "certificates")
	err := filepath.Walk(certRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".crt") {
			// Parse the certificate
			certData, err := os.ReadFile(path)
			if err != nil {
				return nil // Skip unreadable
			}

			block, _ := pem.Decode(certData)
			if block == nil {
				return nil
			}

			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil
			}

			// Determine status
			status := "valid"
			if time.Now().After(cert.NotAfter) {
				status = "expired"
			} else if time.Now().AddDate(0, 0, 30).After(cert.NotAfter) {
				status = "expiring"
			}

			// Avoid duplicates if we somehow have them (though DB ones are custom)
			certs = append(certs, CertificateInfo{
				Domain:    cert.Subject.CommonName,
				Issuer:    cert.Issuer.CommonName,
				ExpiresAt: cert.NotAfter,
				Status:    status,
				Provider:  "letsencrypt", // Assuming auto-generated are mostly LE/ZeroSSL
			})
		}
		return nil
	})

	if err != nil {
		// Log error but return what we have?
		fmt.Printf("Error walking cert dir: %v\n", err)
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

// DeleteCertificate removes a custom certificate.
func (s *CertificateService) DeleteCertificate(id uint) error {
	return s.db.Delete(&models.SSLCertificate{}, "id = ?", id).Error
}
