package services

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func generateTestCert(t *testing.T, domain string, expiry time.Time) []byte {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: domain,
		},
		NotBefore: time.Now(),
		NotAfter:  expiry,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
}

func TestCertificateService_GetCertificateInfo(t *testing.T) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "cert-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cs := NewCertificateService(tmpDir)

	// Case 1: Valid Certificate
	domain := "example.com"
	expiry := time.Now().Add(24 * time.Hour * 60) // 60 days
	certPEM := generateTestCert(t, domain, expiry)

	// Create cert directory
	certDir := filepath.Join(tmpDir, "certificates", "acme-v02.api.letsencrypt.org-directory", domain)
	err = os.MkdirAll(certDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create cert dir: %v", err)
	}

	certPath := filepath.Join(certDir, domain+".crt")
	err = os.WriteFile(certPath, certPEM, 0644)
	if err != nil {
		t.Fatalf("Failed to write cert file: %v", err)
	}

	// List Certificates
	certs, err := cs.ListCertificates()
	assert.NoError(t, err)
	assert.Len(t, certs, 1)
	if len(certs) > 0 {
		assert.Equal(t, domain, certs[0].Domain)
		assert.Equal(t, "valid", certs[0].Status)
		// Check expiry within a margin
		assert.WithinDuration(t, expiry, certs[0].ExpiresAt, time.Second)
	}

	// Case 2: Expired Certificate
	expiredDomain := "expired.com"
	expiredExpiry := time.Now().Add(-24 * time.Hour) // Yesterday
	expiredCertPEM := generateTestCert(t, expiredDomain, expiredExpiry)

	expiredCertDir := filepath.Join(tmpDir, "certificates", "other", expiredDomain)
	err = os.MkdirAll(expiredCertDir, 0755)
	assert.NoError(t, err)

	expiredCertPath := filepath.Join(expiredCertDir, expiredDomain+".crt")
	err = os.WriteFile(expiredCertPath, expiredCertPEM, 0644)
	assert.NoError(t, err)

	certs, err = cs.ListCertificates()
	assert.NoError(t, err)
	assert.Len(t, certs, 2)

	// Find the expired one
	var foundExpired bool
	for _, c := range certs {
		if c.Domain == expiredDomain {
			assert.Equal(t, "expired", c.Status)
			foundExpired = true
		}
	}
	assert.True(t, foundExpired, "Should find expired certificate")
}
