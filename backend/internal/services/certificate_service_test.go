package services

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
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
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
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
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
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
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
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

func TestCertificateService_UploadCertificate_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

	cs := NewCertificateService(tmpDir, db)

	t.Run("invalid PEM format", func(t *testing.T) {
		cert, err := cs.UploadCertificate("Invalid", "not-a-valid-pem", "also-not-valid")
		assert.Error(t, err)
		assert.Nil(t, cert)
		assert.Contains(t, err.Error(), "invalid certificate PEM")
	})

	t.Run("empty certificate", func(t *testing.T) {
		cert, err := cs.UploadCertificate("Empty", "", "some-key")
		assert.Error(t, err)
		assert.Nil(t, cert)
	})

	t.Run("certificate without key allowed", func(t *testing.T) {
		domain := "test.com"
		expiry := time.Now().Add(24 * time.Hour)
		certPEM := generateTestCert(t, domain, expiry)

		cert, err := cs.UploadCertificate("No Key", string(certPEM), "")
		assert.NoError(t, err) // Uploading without key is allowed
		assert.NotNil(t, cert)
		assert.Equal(t, "", cert.PrivateKey)
	})

	t.Run("valid certificate with name", func(t *testing.T) {
		domain := "valid.com"
		expiry := time.Now().Add(24 * time.Hour)
		certPEM := generateTestCert(t, domain, expiry)
		keyPEM := []byte("FAKE PRIVATE KEY")

		cert, err := cs.UploadCertificate("Valid Cert", string(certPEM), string(keyPEM))
		assert.NoError(t, err)
		assert.NotNil(t, cert)
		assert.Equal(t, "Valid Cert", cert.Name)
		assert.Equal(t, domain, cert.Domains)
		assert.Equal(t, "custom", cert.Provider)
	})

	t.Run("expired certificate can be uploaded", func(t *testing.T) {
		domain := "expired-upload.com"
		expiry := time.Now().Add(-24 * time.Hour) // Already expired
		certPEM := generateTestCert(t, domain, expiry)
		keyPEM := []byte("FAKE PRIVATE KEY")

		cert, err := cs.UploadCertificate("Expired Upload", string(certPEM), string(keyPEM))
		// Should still upload successfully, but status will be expired
		assert.NoError(t, err)
		assert.NotNil(t, cert)
		assert.Equal(t, domain, cert.Domains)
	})
}

func TestCertificateService_ListCertificates_EdgeCases(t *testing.T) {
	t.Run("empty certificates directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		cs := NewCertificateService(tmpDir, db)

		certs, err := cs.ListCertificates()
		assert.NoError(t, err)
		assert.Len(t, certs, 0)
	})

	t.Run("certificates directory does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		nonExistentDir := filepath.Join(tmpDir, "does-not-exist")
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		cs := NewCertificateService(nonExistentDir, db)

		certs, err := cs.ListCertificates()
		assert.NoError(t, err)
		assert.Len(t, certs, 0)
	})

	t.Run("invalid certificate files are skipped", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		cs := NewCertificateService(tmpDir, db)

		// Create a cert file with invalid content
		domain := "invalid.com"
		certDir := filepath.Join(tmpDir, "certificates", "acme-v02.api.letsencrypt.org-directory", domain)
		err = os.MkdirAll(certDir, 0755)
		require.NoError(t, err)

		certPath := filepath.Join(certDir, domain+".crt")
		err = os.WriteFile(certPath, []byte("invalid certificate content"), 0644)
		require.NoError(t, err)

		certs, err := cs.ListCertificates()
		assert.NoError(t, err)
		// Invalid certs should be skipped
		assert.Len(t, certs, 0)
	})

	t.Run("multiple certificates from different providers", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		cs := NewCertificateService(tmpDir, db)

		// Create LE cert
		domain1 := "le.example.com"
		expiry1 := time.Now().Add(24 * time.Hour)
		certPEM1 := generateTestCert(t, domain1, expiry1)
		certDir1 := filepath.Join(tmpDir, "certificates", "acme-v02.api.letsencrypt.org-directory", domain1)
		err = os.MkdirAll(certDir1, 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(certDir1, domain1+".crt"), certPEM1, 0644)
		require.NoError(t, err)

		// Create custom cert via upload
		domain2 := "custom.example.com"
		expiry2 := time.Now().Add(48 * time.Hour)
		certPEM2 := generateTestCert(t, domain2, expiry2)
		_, err = cs.UploadCertificate("Custom", string(certPEM2), "FAKE KEY")
		require.NoError(t, err)

		certs, err := cs.ListCertificates()
		assert.NoError(t, err)
		assert.Len(t, certs, 2)

		// Verify both providers exist
		providers := make(map[string]bool)
		for _, c := range certs {
			providers[c.Provider] = true
		}
		assert.True(t, providers["letsencrypt"])
		assert.True(t, providers["custom"])
	})
}

func TestCertificateService_DeleteCertificate_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

	cs := NewCertificateService(tmpDir, db)

	t.Run("delete non-existent certificate", func(t *testing.T) {
		err := cs.DeleteCertificate(99999)
		assert.Error(t, err)
		assert.Equal(t, gorm.ErrRecordNotFound, err)
	})

	t.Run("delete certificate when file already removed", func(t *testing.T) {
		// Create and upload cert
		domain := "to-delete.com"
		expiry := time.Now().Add(24 * time.Hour)
		certPEM := generateTestCert(t, domain, expiry)
		cert, err := cs.UploadCertificate("To Delete", string(certPEM), "FAKE KEY")
		require.NoError(t, err)

		// Manually remove the file (custom certs stored by numeric ID)
		certPath := filepath.Join(tmpDir, "certificates", "custom", "cert.crt")
		os.Remove(certPath)

		// Delete should still work (DB cleanup)
		err = cs.DeleteCertificate(cert.ID)
		assert.NoError(t, err)

		// Verify DB record is gone
		var dbCert models.SSLCertificate
		err = db.First(&dbCert, "id = ?", cert.ID).Error
		assert.Error(t, err)
	})
}
