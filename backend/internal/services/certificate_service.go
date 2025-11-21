package services

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CertificateInfo represents parsed certificate details.
type CertificateInfo struct {
	Domain    string    `json:"domain"`
	Issuer    string    `json:"issuer"`
	ExpiresAt time.Time `json:"expires_at"`
	Status    string    `json:"status"` // "valid", "expiring", "expired"
}

// CertificateService manages certificate retrieval and parsing.
type CertificateService struct {
	dataDir string
}

// NewCertificateService creates a new certificate service.
func NewCertificateService(dataDir string) *CertificateService {
	return &CertificateService{
		dataDir: dataDir,
	}
}

// ListCertificates scans the Caddy data directory for certificates.
// It looks in certificates/acme-v02.api.letsencrypt.org-directory/ and others.
func (s *CertificateService) ListCertificates() ([]CertificateInfo, error) {
	certs := []CertificateInfo{}
	certRoot := filepath.Join(s.dataDir, "certificates")

	// Walk through the certificate directory
	err := filepath.Walk(certRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// If directory doesn't exist yet (fresh install), just return empty
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		// We only care about .crt files
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".crt") {
			cert, err := s.parseCertificate(path)
			if err != nil {
				// Log error but continue scanning other certs
				fmt.Printf("failed to parse cert %s: %v\n", path, err)
				return nil
			}
			certs = append(certs, *cert)
		}
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("walk certificates: %w", err)
	}

	return certs, nil
}

func (s *CertificateService) parseCertificate(path string) (*CertificateInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	status := "valid"
	now := time.Now()
	if now.After(cert.NotAfter) {
		status = "expired"
	} else if now.Add(30 * 24 * time.Hour).After(cert.NotAfter) {
		status = "expiring"
	}

	// Domain is usually the CommonName or the first SAN
	domain := cert.Subject.CommonName
	if domain == "" && len(cert.DNSNames) > 0 {
		domain = cert.DNSNames[0]
	}

	return &CertificateInfo{
		Domain:    domain,
		Issuer:    cert.Issuer.CommonName,
		ExpiresAt: cert.NotAfter,
		Status:    status,
	}, nil
}
