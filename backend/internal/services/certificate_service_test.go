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
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
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

	// Setup in-memory DB
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	if err := db.AutoMigrate(&models.SSLCertificate{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	cs := NewCertificateService(tmpDir, db)

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

func TestCertificateService_UploadAndDelete(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

	cs := NewCertificateService(tmpDir, db)

	// Generate Cert
	domain := "custom.example.com"
	expiry := time.Now().Add(24 * time.Hour)
	certPEM := generateTestCert(t, domain, expiry)
	keyPEM := []byte("FAKE PRIVATE KEY")

	// Test Upload
	cert, err := cs.UploadCertificate("My Custom Cert", string(certPEM), string(keyPEM))
	require.NoError(t, err)
	assert.NotNil(t, cert)
	assert.Equal(t, "My Custom Cert", cert.Name)
	assert.Equal(t, "custom", cert.Provider)
	assert.Equal(t, domain, cert.Domains)

	// Verify it's in List
	certs, err := cs.ListCertificates()
	require.NoError(t, err)
	var found bool
	for _, c := range certs {
		if c.ID == cert.ID {
			found = true
			assert.Equal(t, "custom", c.Provider)
			break
		}
	}
	assert.True(t, found)

	// Test Delete
	err = cs.DeleteCertificate(cert.ID)
	require.NoError(t, err)

	// Verify it's gone
	certs, err = cs.ListCertificates()
	require.NoError(t, err)
	found = false
	for _, c := range certs {
		if c.ID == cert.ID {
			found = true
			break
		}
	}
	assert.False(t, found)
}

func TestCertificateService_Persistence(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

	cs := NewCertificateService(tmpDir, db)

	// 1. Create a fake ACME cert file
	domain := "persist.example.com"
	expiry := time.Now().Add(24 * time.Hour)
	certPEM := generateTestCert(t, domain, expiry)

	certDir := filepath.Join(tmpDir, "certificates", "acme-v02.api.letsencrypt.org-directory", domain)
	err = os.MkdirAll(certDir, 0755)
	require.NoError(t, err)

	certPath := filepath.Join(certDir, domain+".crt")
	err = os.WriteFile(certPath, certPEM, 0644)
	require.NoError(t, err)

	// 2. Call ListCertificates to trigger scan and persistence
	certs, err := cs.ListCertificates()
	require.NoError(t, err)

	// Verify it's in the returned list
	var foundInList bool
	for _, c := range certs {
		if c.Domain == domain {
			foundInList = true
			assert.Equal(t, "letsencrypt", c.Provider)
			break
		}
	}
	assert.True(t, foundInList, "Certificate should be in the returned list")

	// 3. Verify it's in the DB
	var dbCert models.SSLCertificate
	err = db.Where("domains = ? AND provider = ?", domain, "letsencrypt").First(&dbCert).Error
	assert.NoError(t, err, "Certificate should be persisted to DB")
	assert.Equal(t, domain, dbCert.Name)
	assert.Equal(t, string(certPEM), dbCert.Certificate)

	// 4. Delete the certificate via Service (which should delete the file)
	err = cs.DeleteCertificate(dbCert.ID)
	require.NoError(t, err)

	// Verify file is gone
	_, err = os.Stat(certPath)
	assert.True(t, os.IsNotExist(err), "Cert file should be deleted")

	// 5. Call ListCertificates again to trigger cleanup (though DB row is already gone)
	certs, err = cs.ListCertificates()
	require.NoError(t, err)

	// Verify it's NOT in the returned list
	foundInList = false
	for _, c := range certs {
		if c.Domain == domain {
			foundInList = true
			break
		}
	}
	assert.False(t, foundInList, "Certificate should NOT be in the returned list after deletion")

	// 6. Verify it's gone from the DB
	err = db.Where("domains = ? AND provider = ?", domain, "letsencrypt").First(&dbCert).Error
	assert.Error(t, err, "Certificate should be removed from DB")
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}
