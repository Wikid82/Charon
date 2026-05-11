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

	"github.com/Wikid82/charon/backend/internal/models"
)

// newTestCertificateService creates a CertificateService for testing without
// starting the background scan goroutine. Tests must call SyncFromDisk() explicitly.
func newTestCertificateService(dataDir string, db *gorm.DB) *CertificateService {
	return &CertificateService{
		dataDir: dataDir,
		db:      db,
		scanTTL: 5 * time.Minute,
	}
}

// certDBID looks up the numeric DB primary key for a certificate by UUID.
func certDBID(t *testing.T, db *gorm.DB, uuid string) uint {
	t.Helper()
	var cert models.SSLCertificate
	require.NoError(t, db.Where("uuid = ?", uuid).First(&cert).Error)
	return cert.ID
}

func TestNewCertificateService(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	// Create the certificates directory
	certDir := filepath.Join(tmpDir, "certificates")
	require.NoError(t, os.MkdirAll(certDir, 0o750)) // #nosec G301 -- test directory

	// Test service creation
	svc := NewCertificateService(tmpDir, db, nil)
	assert.NotNil(t, svc)
	assert.Equal(t, tmpDir, svc.dataDir)
	assert.Equal(t, db, svc.db)
	assert.Equal(t, 5*time.Minute, svc.scanTTL)

	// Give the background goroutine time to complete
	time.Sleep(100 * time.Millisecond)
}

func generateTestCert(t *testing.T, domain string, expiry time.Time) []byte {
	certPEM, _ := generateTestCertAndKey(t, domain, expiry)
	return certPEM
}

func generateTestCertAndKey(t *testing.T, domain string, expiry time.Time) (certPEM, keyPEM []byte) {
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

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return certPEM, keyPEM
}

func TestCertificateService_GetCertificateInfo(t *testing.T) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "cert-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Setup in-memory DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	if err = db.AutoMigrate(&models.SSLCertificate{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	cs := newTestCertificateService(tmpDir, db)

	// Case 1: Valid Certificate
	domain := "example.com"
	expiry := time.Now().Add(24 * time.Hour * 60) // 60 days
	certPEM := generateTestCert(t, domain, expiry)

	// Create cert directory
	certDir := filepath.Join(tmpDir, "certificates", "acme-v02.api.letsencrypt.org-directory", domain)
	err = os.MkdirAll(certDir, 0o750)
	if err != nil {
		t.Fatalf("Failed to create cert dir: %v", err)
	}

	certPath := filepath.Join(certDir, domain+".crt")
	err = os.WriteFile(certPath, certPEM, 0o600) // #nosec G306 -- test certificate
	if err != nil {
		t.Fatalf("Failed to write cert file: %v", err)
	}

	// List Certificates
	certs, err := cs.ListCertificates()
	assert.NoError(t, err)
	assert.Len(t, certs, 1)
	if len(certs) > 0 {
		assert.Equal(t, domain, certs[0].Domains)
		assert.Equal(t, "valid", certs[0].Status)
		// Check expiry within a margin
		assert.WithinDuration(t, expiry, certs[0].ExpiresAt, time.Second)
	}

	// Case 2: Expired Certificate
	expiredDomain := "expired.com"
	expiredExpiry := time.Now().Add(-24 * time.Hour) // Yesterday
	expiredCertPEM := generateTestCert(t, expiredDomain, expiredExpiry)

	expiredCertDir := filepath.Join(tmpDir, "certificates", "other", expiredDomain)
	err = os.MkdirAll(expiredCertDir, 0o750) // #nosec G301 -- test directory
	assert.NoError(t, err)

	expiredCertPath := filepath.Join(expiredCertDir, expiredDomain+".crt")
	err = os.WriteFile(expiredCertPath, expiredCertPEM, 0o600) // #nosec G306 -- test certificate
	assert.NoError(t, err)

	// Force rescan to pick up new cert
	err = cs.SyncFromDisk()
	assert.NoError(t, err)

	certs, err = cs.ListCertificates()
	assert.NoError(t, err)
	assert.Len(t, certs, 2)

	// Find the expired one
	var foundExpired bool
	for _, c := range certs {
		if c.Domains == expiredDomain {
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
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	// Generate Cert
	domain := "custom.example.com"
	expiry := time.Now().Add(24 * time.Hour)
	certPEM, keyPEM := generateTestCertAndKey(t, domain, expiry)

	// Test Upload
	cert, err := cs.UploadCertificate("My Custom Cert", string(certPEM), string(keyPEM), "")
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
		if c.UUID == cert.UUID {
			found = true
			assert.Equal(t, "custom", c.Provider)
			break
		}
	}
	assert.True(t, found)

	// Test Delete
	err = cs.DeleteCertificate(cert.UUID)
	require.NoError(t, err)

	// Verify it's gone
	certs, err = cs.ListCertificates()
	require.NoError(t, err)
	found = false
	for _, c := range certs {
		if c.UUID == cert.UUID {
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
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	// 1. Create a fake ACME cert file
	domain := "persist.example.com"
	expiry := time.Now().Add(24 * time.Hour)
	certPEM := generateTestCert(t, domain, expiry)

	certDir := filepath.Join(tmpDir, "certificates", "acme-v02.api.letsencrypt.org-directory", domain)
	err = os.MkdirAll(certDir, 0o750) // #nosec G301 -- test directory
	require.NoError(t, err)

	certPath := filepath.Join(certDir, domain+".crt")
	err = os.WriteFile(certPath, certPEM, 0o600) // #nosec G306 -- test certificate
	require.NoError(t, err)

	// 2. Sync from disk and call ListCertificates
	err = cs.SyncFromDisk()
	require.NoError(t, err)

	certs, err := cs.ListCertificates()
	require.NoError(t, err)

	// Verify it's in the returned list
	var foundInList bool
	for _, c := range certs {
		if c.Domains == domain {
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
	err = cs.DeleteCertificate(dbCert.UUID)
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
		if c.Domains == domain {
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
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	t.Run("invalid PEM format", func(t *testing.T) {
		cert, err := cs.UploadCertificate("Invalid", "not-a-valid-pem", "also-not-valid", "")
		assert.Error(t, err)
		assert.Nil(t, cert)
		assert.Contains(t, err.Error(), "unrecognized certificate format")
	})

	t.Run("empty certificate", func(t *testing.T) {
		cert, err := cs.UploadCertificate("Empty", "", "some-key", "")
		assert.Error(t, err)
		assert.Nil(t, cert)
	})

	t.Run("certificate without key allowed", func(t *testing.T) {
		domain := "test.com"
		expiry := time.Now().Add(24 * time.Hour)
		certPEM := generateTestCert(t, domain, expiry)

		cert, err := cs.UploadCertificate("No Key", string(certPEM), "", "")
		assert.NoError(t, err) // Uploading without key is allowed
		assert.NotNil(t, cert)
		assert.False(t, cert.HasKey)
	})

	t.Run("valid certificate with name", func(t *testing.T) {
		domain := "valid.com"
		expiry := time.Now().Add(24 * time.Hour)
		certPEM, keyPEM := generateTestCertAndKey(t, domain, expiry)

		cert, err := cs.UploadCertificate("Valid Cert", string(certPEM), string(keyPEM), "")
		assert.NoError(t, err)
		assert.NotNil(t, cert)
		assert.Equal(t, "Valid Cert", cert.Name)
		assert.Equal(t, domain, cert.Domains)
		assert.Equal(t, "custom", cert.Provider)
	})

	t.Run("expired certificate can be uploaded", func(t *testing.T) {
		domain := "expired-upload.com"
		expiry := time.Now().Add(-24 * time.Hour) // Already expired
		certPEM, keyPEM := generateTestCertAndKey(t, domain, expiry)

		cert, err := cs.UploadCertificate("Expired Upload", string(certPEM), string(keyPEM), "")
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

		cs := newTestCertificateService(tmpDir, db)

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

		cs := newTestCertificateService(nonExistentDir, db)

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

		cs := newTestCertificateService(tmpDir, db)

		// Create a cert file with invalid content
		domain := "invalid.com"
		certDir := filepath.Join(tmpDir, "certificates", "acme-v02.api.letsencrypt.org-directory", domain)
		err = os.MkdirAll(certDir, 0o750) // #nosec G301 -- test directory
		require.NoError(t, err)

		certPath := filepath.Join(certDir, domain+".crt")
		err = os.WriteFile(certPath, []byte("invalid certificate content"), 0o600) // #nosec G306 -- test certificate
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

		cs := newTestCertificateService(tmpDir, db)

		// Create LE cert
		domain1 := "le.example.com"
		expiry1 := time.Now().Add(24 * time.Hour)
		certPEM1 := generateTestCert(t, domain1, expiry1)
		certDir1 := filepath.Join(tmpDir, "certificates", "acme-v02.api.letsencrypt.org-directory", domain1)
		err = os.MkdirAll(certDir1, 0o750) // #nosec G301 -- test directory
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(certDir1, domain1+".crt"), certPEM1, 0o600) // #nosec G306 -- test certificate
		require.NoError(t, err)

		// Create custom cert via upload
		domain2 := "custom.example.com"
		expiry2 := time.Now().Add(48 * time.Hour)
		certPEM2 := generateTestCert(t, domain2, expiry2)
		_, err = cs.UploadCertificate("Custom", string(certPEM2), "", "")
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
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	t.Run("delete non-existent certificate", func(t *testing.T) {
		// DeleteCertificate takes UUID string; non-existent UUID returns error
		err := cs.DeleteCertificate("non-existent-uuid")
		assert.Error(t, err)
	})

	t.Run("delete certificate in use returns ErrCertInUse", func(t *testing.T) {
		// Create certificate
		domain := "in-use.com"
		expiry := time.Now().Add(24 * time.Hour)
		certPEM, keyPEM := generateTestCertAndKey(t, domain, expiry)
		cert, err := cs.UploadCertificate("In Use", string(certPEM), string(keyPEM), "")
		require.NoError(t, err)

		// Look up numeric ID for FK
		dbID := certDBID(t, db, cert.UUID)

		// Create proxy host using this certificate
		ph := models.ProxyHost{
			UUID:          "test-ph",
			Name:          "Test Host",
			DomainNames:   "in-use.com",
			ForwardHost:   "localhost",
			ForwardPort:   8080,
			CertificateID: &dbID,
		}
		require.NoError(t, db.Create(&ph).Error)

		// Attempt to delete certificate - should fail with ErrCertInUse
		err = cs.DeleteCertificate(cert.UUID)
		assert.Error(t, err)
		assert.Equal(t, ErrCertInUse, err)

		// Verify certificate still exists
		var dbCert models.SSLCertificate
		err = db.First(&dbCert, "id = ?", dbID).Error
		assert.NoError(t, err)
	})

	t.Run("delete certificate when file already removed", func(t *testing.T) {
		// Create and upload cert
		domain := "to-delete.com"
		expiry := time.Now().Add(24 * time.Hour)
		certPEM, keyPEM := generateTestCertAndKey(t, domain, expiry)
		cert, err := cs.UploadCertificate("To Delete", string(certPEM), string(keyPEM), "")
		require.NoError(t, err)

		// Look up numeric ID for verification
		dbID := certDBID(t, db, cert.UUID)

		// Manually remove the file (custom certs stored by numeric ID)
		certPath := filepath.Join(tmpDir, "certificates", "custom", "cert.crt")
		_ = os.Remove(certPath)

		// Delete should still work (DB cleanup)
		err = cs.DeleteCertificate(cert.UUID)
		assert.NoError(t, err)

		// Verify DB record is gone
		var dbCert models.SSLCertificate
		err = db.First(&dbCert, "id = ?", dbID).Error
		assert.Error(t, err)
	})
}

func TestCertificateService_StagingCertificates(t *testing.T) {
	t.Run("staging certificate detected by path", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		cs := newTestCertificateService(tmpDir, db)

		// Create staging cert in acme-staging directory
		domain := "staging.example.com"
		expiry := time.Now().Add(24 * time.Hour)
		certPEM := generateTestCert(t, domain, expiry)

		// Staging path contains "acme-staging"
		certDir := filepath.Join(tmpDir, "certificates", "acme-staging-v02.api.letsencrypt.org-directory", domain)
		err = os.MkdirAll(certDir, 0o750) // #nosec G301 -- test directory
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(certDir, domain+".crt"), certPEM, 0o600) // #nosec G306 -- test certificate
		require.NoError(t, err)

		err = cs.SyncFromDisk()
		require.NoError(t, err)
		certs, err := cs.ListCertificates()
		require.NoError(t, err)
		require.Len(t, certs, 1)

		// Should be detected as staging
		assert.Equal(t, "letsencrypt-staging", certs[0].Provider)
		assert.Equal(t, "untrusted", certs[0].Status)
	})

	t.Run("production cert preferred over staging", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		cs := newTestCertificateService(tmpDir, db)

		domain := "both.example.com"
		expiry := time.Now().Add(60 * 24 * time.Hour) // 60 days - outside expiring window
		certPEM := generateTestCert(t, domain, expiry)

		// Create staging cert first (alphabetically comes before production)
		stagingDir := filepath.Join(tmpDir, "certificates", "acme-staging-v02.api.letsencrypt.org-directory", domain)
		err = os.MkdirAll(stagingDir, 0o750) // #nosec G301 -- test directory
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(stagingDir, domain+".crt"), certPEM, 0o600) // #nosec G306 -- test certificate
		require.NoError(t, err)

		// Create production cert
		prodDir := filepath.Join(tmpDir, "certificates", "acme-v02.api.letsencrypt.org-directory", domain)
		err = os.MkdirAll(prodDir, 0o750) // #nosec G301 -- test directory
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(prodDir, domain+".crt"), certPEM, 0o600) // #nosec G306 -- test certificate
		require.NoError(t, err)

		err = cs.SyncFromDisk()
		require.NoError(t, err)
		certs, err := cs.ListCertificates()
		require.NoError(t, err)
		require.Len(t, certs, 1)

		// Production should win
		assert.Equal(t, "letsencrypt", certs[0].Provider)
		assert.Equal(t, "valid", certs[0].Status)
	})

	t.Run("upgrade from staging to production", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		cs := newTestCertificateService(tmpDir, db)

		domain := "upgrade.example.com"
		expiry := time.Now().Add(60 * 24 * time.Hour) // 60 days - outside expiring window
		certPEM := generateTestCert(t, domain, expiry)

		// First, create only staging cert
		stagingDir := filepath.Join(tmpDir, "certificates", "acme-staging-v02.api.letsencrypt.org-directory", domain)
		err = os.MkdirAll(stagingDir, 0o750) // #nosec G301 -- test directory
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(stagingDir, domain+".crt"), certPEM, 0o600) // #nosec G306 -- test certificate
		require.NoError(t, err)

		// Scan - should be staging
		err = cs.SyncFromDisk()
		require.NoError(t, err)
		certs, err := cs.ListCertificates()
		require.NoError(t, err)
		require.Len(t, certs, 1)
		assert.Equal(t, "letsencrypt-staging", certs[0].Provider)

		// Now add production cert
		prodDir := filepath.Join(tmpDir, "certificates", "acme-v02.api.letsencrypt.org-directory", domain)
		err = os.MkdirAll(prodDir, 0o750) // #nosec G301 -- test directory
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(prodDir, domain+".crt"), certPEM, 0o600) // #nosec G306 -- test fixture
		require.NoError(t, err)

		// Rescan - should be upgraded to production
		err = cs.SyncFromDisk()
		require.NoError(t, err)
		certs, err = cs.ListCertificates()
		require.NoError(t, err)
		require.Len(t, certs, 1)
		assert.Equal(t, "letsencrypt", certs[0].Provider)
		assert.Equal(t, "valid", certs[0].Status)
	})
}

func TestCertificateService_ExpiringStatus(t *testing.T) {
	t.Run("certificate expiring within 30 days", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		cs := newTestCertificateService(tmpDir, db)

		// Expiring in 15 days (within 30 day threshold)
		domain := "expiring.example.com"
		expiry := time.Now().Add(15 * 24 * time.Hour)
		certPEM := generateTestCert(t, domain, expiry)

		certDir := filepath.Join(tmpDir, "certificates", "acme-v02.api.letsencrypt.org-directory", domain)
		err = os.MkdirAll(certDir, 0o750) // #nosec G301 -- test directory
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(certDir, domain+".crt"), certPEM, 0o600) // #nosec G306 -- test fixture
		require.NoError(t, err)

		err = cs.SyncFromDisk()
		require.NoError(t, err)
		certs, err := cs.ListCertificates()
		require.NoError(t, err)
		require.Len(t, certs, 1)
		assert.Equal(t, "expiring", certs[0].Status)
	})

	t.Run("certificate valid for more than 30 days", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		cs := newTestCertificateService(tmpDir, db)

		// Expiring in 60 days (outside 30 day threshold)
		domain := "valid-long.example.com"
		expiry := time.Now().Add(60 * 24 * time.Hour)
		certPEM := generateTestCert(t, domain, expiry)

		certDir := filepath.Join(tmpDir, "certificates", "acme-v02.api.letsencrypt.org-directory", domain)
		err = os.MkdirAll(certDir, 0o750) // #nosec G301 -- test directory
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(certDir, domain+".crt"), certPEM, 0o600) // #nosec G306 -- test fixture
		require.NoError(t, err)

		err = cs.SyncFromDisk()
		require.NoError(t, err)
		certs, err := cs.ListCertificates()
		require.NoError(t, err)
		require.Len(t, certs, 1)
		assert.Equal(t, "valid", certs[0].Status)
	})

	t.Run("staging cert always untrusted even if expiring", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		cs := newTestCertificateService(tmpDir, db)

		// Staging cert expiring soon
		domain := "staging-expiring.example.com"
		expiry := time.Now().Add(5 * 24 * time.Hour)
		certPEM := generateTestCert(t, domain, expiry)

		certDir := filepath.Join(tmpDir, "certificates", "acme-staging-v02.api.letsencrypt.org-directory", domain)
		err = os.MkdirAll(certDir, 0o750) // #nosec G301 -- test directory
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(certDir, domain+".crt"), certPEM, 0o600) // #nosec G306 -- test fixture
		require.NoError(t, err)

		err = cs.SyncFromDisk()
		require.NoError(t, err)
		certs, err := cs.ListCertificates()
		require.NoError(t, err)
		require.Len(t, certs, 1)
		// Staging takes priority over expiring for status
		assert.Equal(t, "untrusted", certs[0].Status)
		assert.Equal(t, "letsencrypt-staging", certs[0].Provider)
	})
}

func TestCertificateService_StaleCertCleanup(t *testing.T) {
	t.Run("stale DB entries removed when file deleted", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		cs := newTestCertificateService(tmpDir, db)

		domain := "stale.example.com"
		expiry := time.Now().Add(24 * time.Hour)
		certPEM := generateTestCert(t, domain, expiry)

		certDir := filepath.Join(tmpDir, "certificates", "acme-v02.api.letsencrypt.org-directory", domain)
		certPath := filepath.Join(certDir, domain+".crt")
		err = os.MkdirAll(certDir, 0o750) // #nosec G301 -- test directory
		require.NoError(t, err)
		err = os.WriteFile(certPath, certPEM, 0o600) // #nosec G306 -- test fixture
		require.NoError(t, err)

		// First scan - should create DB entry
		err = cs.SyncFromDisk()
		require.NoError(t, err)
		certs, err := cs.ListCertificates()
		require.NoError(t, err)
		require.Len(t, certs, 1)

		// Delete the file
		err = os.Remove(certPath)
		require.NoError(t, err)

		// Second scan - should remove stale DB entry
		err = cs.SyncFromDisk()
		require.NoError(t, err)
		certs, err = cs.ListCertificates()
		require.NoError(t, err)
		assert.Len(t, certs, 0)

		// Verify DB is clean
		var count int64
		db.Model(&models.SSLCertificate{}).Count(&count)
		assert.Equal(t, int64(0), count)
	})
}

func TestCertificateService_CertificateWithSANs(t *testing.T) {
	t.Run("certificate with SANs uses joined domains", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		cs := newTestCertificateService(tmpDir, db)

		// Generate cert with SANs
		domain := "san.example.com"
		expiry := time.Now().Add(24 * time.Hour)
		certPEM := generateTestCertWithSANs(t, domain, []string{"san.example.com", "www.san.example.com", "api.san.example.com"}, expiry)

		cert, err := cs.UploadCertificate("SAN Cert", string(certPEM), "", "")
		require.NoError(t, err)
		assert.NotNil(t, cert)
		// Should have joined SANs
		assert.Contains(t, cert.Domains, "san.example.com")
		assert.Contains(t, cert.Domains, "www.san.example.com")
		assert.Contains(t, cert.Domains, "api.san.example.com")
	})
}

func TestCertificateService_IsCertificateInUse(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	t.Run("certificate not in use", func(t *testing.T) {
		// Create certificate without any proxy hosts
		domain := "unused.com"
		expiry := time.Now().Add(24 * time.Hour)
		certPEM := generateTestCert(t, domain, expiry)
		cert, err := cs.UploadCertificate("Unused", string(certPEM), "", "")
		require.NoError(t, err)

		dbID := certDBID(t, db, cert.UUID)
		inUse, err := cs.IsCertificateInUse(dbID)
		assert.NoError(t, err)
		assert.False(t, inUse)
	})

	t.Run("certificate used by one proxy host", func(t *testing.T) {
		// Create certificate
		domain := "used.com"
		expiry := time.Now().Add(24 * time.Hour)
		certPEM := generateTestCert(t, domain, expiry)
		cert, err := cs.UploadCertificate("Used", string(certPEM), "", "")
		require.NoError(t, err)

		dbID := certDBID(t, db, cert.UUID)

		// Create proxy host using this certificate
		ph := models.ProxyHost{
			UUID:          "ph-1",
			Name:          "Test Host 1",
			DomainNames:   "used.com",
			ForwardHost:   "localhost",
			ForwardPort:   8080,
			CertificateID: &dbID,
		}
		require.NoError(t, db.Create(&ph).Error)

		inUse, err := cs.IsCertificateInUse(dbID)
		assert.NoError(t, err)
		assert.True(t, inUse)
	})

	t.Run("certificate used by multiple proxy hosts", func(t *testing.T) {
		// Create certificate
		domain := "shared.com"
		expiry := time.Now().Add(24 * time.Hour)
		certPEM := generateTestCert(t, domain, expiry)
		cert, err := cs.UploadCertificate("Shared", string(certPEM), "", "")
		require.NoError(t, err)

		dbID := certDBID(t, db, cert.UUID)

		// Create multiple proxy hosts using this certificate
		for i := 1; i <= 3; i++ {
			ph := models.ProxyHost{
				UUID:          fmt.Sprintf("ph-shared-%d", i),
				Name:          fmt.Sprintf("Test Host %d", i),
				DomainNames:   fmt.Sprintf("host%d.shared.com", i),
				ForwardHost:   "localhost",
				ForwardPort:   8080 + i,
				CertificateID: &dbID,
			}
			require.NoError(t, db.Create(&ph).Error)
		}

		inUse, err := cs.IsCertificateInUse(dbID)
		assert.NoError(t, err)
		assert.True(t, inUse)
	})

	t.Run("non-existent certificate", func(t *testing.T) {
		inUse, err := cs.IsCertificateInUse(99999)
		assert.NoError(t, err) // No error, just returns false
		assert.False(t, inUse)
	})

	t.Run("certificate freed after proxy host deletion", func(t *testing.T) {
		// Create certificate
		domain := "freed.com"
		expiry := time.Now().Add(24 * time.Hour)
		certPEM := generateTestCert(t, domain, expiry)
		cert, err := cs.UploadCertificate("Freed", string(certPEM), "", "")
		require.NoError(t, err)

		dbID := certDBID(t, db, cert.UUID)

		// Create proxy host using this certificate
		ph := models.ProxyHost{
			UUID:          "ph-freed",
			Name:          "Test Host Freed",
			DomainNames:   "freed.com",
			ForwardHost:   "localhost",
			ForwardPort:   8080,
			CertificateID: &dbID,
		}
		require.NoError(t, db.Create(&ph).Error)

		// Verify in use
		inUse, err := cs.IsCertificateInUse(dbID)
		assert.NoError(t, err)
		assert.True(t, inUse)

		// Delete the proxy host
		require.NoError(t, db.Delete(&ph).Error)

		// Verify no longer in use
		inUse, err = cs.IsCertificateInUse(dbID)
		assert.NoError(t, err)
		assert.False(t, inUse)

		// Now deletion should succeed
		err = cs.DeleteCertificate(cert.UUID)
		assert.NoError(t, err)
	})
}

func TestCertificateService_CacheBehavior(t *testing.T) {
	t.Run("cache returns consistent results", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		cs := newTestCertificateService(tmpDir, db)

		// Create a cert
		domain := "cache.example.com"
		expiry := time.Now().Add(24 * time.Hour)
		certPEM, keyPEM := generateTestCertAndKey(t, domain, expiry)

		cert, err := cs.UploadCertificate("Cache Test", string(certPEM), string(keyPEM), "")
		require.NoError(t, err)
		require.NotNil(t, cert)

		// First call populates cache
		certs1, err := cs.ListCertificates()
		require.NoError(t, err)
		require.Len(t, certs1, 1)

		// Second call returns from cache
		certs2, err := cs.ListCertificates()
		require.NoError(t, err)
		require.Len(t, certs2, 1)

		// Both should return the same cert
		assert.Equal(t, certs1[0].UUID, certs2[0].UUID)
	})

	t.Run("invalidate cache forces resync", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		cs := newTestCertificateService(tmpDir, db)

		// Create a cert via upload (auto-invalidates)
		certPEM := generateTestCert(t, "invalidate.example.com", time.Now().Add(24*time.Hour))
		_, err = cs.UploadCertificate("Invalidate Test", string(certPEM), "", "")
		require.NoError(t, err)

		// Get list (should have 1)
		certs, err := cs.ListCertificates()
		require.NoError(t, err)
		require.Len(t, certs, 1)

		// Manually add a cert to DB (simulating external change)
		dbCert := models.SSLCertificate{
			Name:        "External Cert",
			Provider:    "custom",
			Domains:     "external.example.com",
			Certificate: "fake-cert",
		}
		require.NoError(t, db.Create(&dbCert).Error)

		// Cache still returns old result
		certs, err = cs.ListCertificates()
		require.NoError(t, err)
		assert.Len(t, certs, 1) // Cache hasn't updated

		// Invalidate and resync
		cs.InvalidateCache()
		certs, err = cs.ListCertificates()
		require.NoError(t, err)
		assert.Len(t, certs, 2) // Now sees both
	})

	t.Run("refreshCacheFromDB used when directory nonexistent", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		// Point to non-existent directory
		cs := newTestCertificateService(filepath.Join(tmpDir, "nonexistent"), db)

		// Pre-populate DB
		expiry := time.Now().Add(24 * time.Hour)
		dbCert := models.SSLCertificate{
			Name:        "DB Cert",
			Provider:    "custom",
			Domains:     "db.example.com",
			ExpiresAt:   &expiry,
			Certificate: "fake-cert",
		}
		require.NoError(t, db.Create(&dbCert).Error)

		// Sync should succeed via DB fallback
		err = cs.SyncFromDisk()
		require.NoError(t, err)

		// List should return cert from DB
		certs, err := cs.ListCertificates()
		require.NoError(t, err)
		require.Len(t, certs, 1)
		assert.Equal(t, "db.example.com", certs[0].Domains)
	})
}

func TestCertificateService_UploadCertificate_ParsingErrors(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

	cs := newTestCertificateService(tmpDir, db)

	t.Run("certificate with corrupted DER bytes", func(t *testing.T) {
		// Valid PEM structure but invalid base64 that decodes but fails x509 parsing
		corruptedPEM := `-----BEGIN CERTIFICATE-----
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
-----END CERTIFICATE-----`

		cert, err := cs.UploadCertificate("Corrupted", corruptedPEM, "", "")
		assert.Error(t, err)
		assert.Nil(t, cert)
		assert.Contains(t, err.Error(), "failed to parse certificate")
	})

	t.Run("valid PEM but wrong type", func(t *testing.T) {
		// Using a private key PEM instead of certificate
		wrongTypePEM := `-----BEGIN PRIVATE KEY-----
MIICdgIBADANBgkqhkiG9w0BAQEFAASCAmAwggJcAgEAAoGBALRiMLAh9iimur8V
A7qVvdqxevEuUkW4K+2KdMXmnQbG9Aa7k7eBjK1S+0LYmVjPKlJGNXHDGuy5Fw/d
7rjVJ0BLB+ubPK8iA/Tw3hLQgXMRRGRXXCn8ikfuQfjUS1uZSatdLB81mydBETlJ
hI6GH4twrbDJCR2Bwy/XWXgqgGRzAgMBAAECgYBYWVtLze8R+KrZdHj0hLjZEPnl
-----END PRIVATE KEY-----`

		cert, err := cs.UploadCertificate("Wrong Type", wrongTypePEM, "", "")
		assert.Error(t, err)
		assert.Nil(t, cert)
		assert.Contains(t, err.Error(), "failed to parse certificate")
	})

	t.Run("certificate with no subject and no SANs", func(t *testing.T) {
		// Create cert with empty subject and no SANs (edge case)
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		template := x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject:      pkix.Name{}, // Empty subject
			NotBefore:    time.Now(),
			NotAfter:     time.Now().Add(24 * time.Hour),
			KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		}

		derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
		require.NoError(t, err)
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

		cert, err := cs.UploadCertificate("Empty Subject", string(certPEM), "", "")
		assert.NoError(t, err) // Upload succeeds
		assert.NotNil(t, cert)
		assert.Equal(t, "", cert.Domains) // Empty domains field
	})
}

func TestCertificateService_SyncFromDisk_ErrorHandling(t *testing.T) {
	t.Run("database error during sync", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		cs := newTestCertificateService(tmpDir, db)

		// Create a valid cert
		domain := "dbtest.com"
		expiry := time.Now().Add(24 * time.Hour)
		certPEM := generateTestCert(t, domain, expiry)

		certDir := filepath.Join(tmpDir, "certificates", "acme-v02.api.letsencrypt.org-directory", domain)
		err = os.MkdirAll(certDir, 0o750) // #nosec G301 -- test directory
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(certDir, domain+".crt"), certPEM, 0o600) // #nosec G306 -- test fixture
		require.NoError(t, err)

		// Close the database connection to simulate DB error
		sqlDB, err := db.DB()
		require.NoError(t, err)
		_ = sqlDB.Close()

		// Sync should handle DB errors gracefully
		err = cs.SyncFromDisk()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to refresh cache")
	})

	t.Run("unreadable certificate directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		// Create cert directory with no read permissions
		certRoot := filepath.Join(tmpDir, "certificates")
		err = os.MkdirAll(certRoot, 0o200) // Write-only, no read
		require.NoError(t, err)

		cs := newTestCertificateService(tmpDir, db)

		// Should handle gracefully
		err = cs.SyncFromDisk()
		// Should complete without crash, possibly with logged error
		assert.NoError(t, err) // Service handles this gracefully

		// Clean up - restore permissions for cleanup
		// #nosec G302 -- Test cleanup restores directory permissions
		_ = os.Chmod(certRoot, 0o700)
	})

	t.Run("certificate file with mixed valid and invalid content", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		cs := newTestCertificateService(tmpDir, db)

		// Create directory with two files: one valid, one invalid
		certDir := filepath.Join(tmpDir, "certificates", "test-provider")
		err = os.MkdirAll(certDir, 0o750) // #nosec G301 -- test directory
		require.NoError(t, err)

		// Valid cert
		validDomain := "valid.com"
		validExpiry := time.Now().Add(24 * time.Hour)
		validCertPEM := generateTestCert(t, validDomain, validExpiry)
		err = os.WriteFile(filepath.Join(certDir, validDomain+".crt"), validCertPEM, 0o600) // #nosec G306 -- test fixture
		require.NoError(t, err)

		// Invalid cert
		err = os.WriteFile(filepath.Join(certDir, "invalid.crt"), []byte("not a cert"), 0o600) // #nosec G306 -- test fixture
		require.NoError(t, err)

		err = cs.SyncFromDisk()
		assert.NoError(t, err)

		// Should have parsed only the valid cert
		certs, err := cs.ListCertificates()
		assert.NoError(t, err)
		assert.Len(t, certs, 1)
		assert.Equal(t, validDomain, certs[0].Domains)
	})
}

func TestCertificateService_RefreshCacheFromDB_EdgeCases(t *testing.T) {
	t.Run("certificate without expiry date", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

		cs := newTestCertificateService(tmpDir, db)

		// Create cert with nil expiry
		cert := models.SSLCertificate{
			UUID:        "test-no-expiry",
			Name:        "No Expiry",
			Provider:    "custom",
			Domains:     "noexpiry.com",
			Certificate: "fake-cert",
			ExpiresAt:   nil, // No expiry
		}
		require.NoError(t, db.Create(&cert).Error)

		cs.InvalidateCache()
		certs, err := cs.ListCertificates()
		assert.NoError(t, err)
		require.Len(t, certs, 1)
		assert.Zero(t, certs[0].ExpiresAt) // Should handle nil expiry
	})

	t.Run("multiple domains comma-separated", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

		cs := newTestCertificateService(tmpDir, db)

		expiry := time.Now().Add(24 * time.Hour)
		cert := models.SSLCertificate{
			UUID:        "test-multi",
			Name:        "Multi Domain",
			Provider:    "custom",
			Domains:     "example.com,www.example.com,api.example.com",
			Certificate: "fake-cert",
			ExpiresAt:   &expiry,
		}
		require.NoError(t, db.Create(&cert).Error)

		// Create proxy host matching one of the domains
		ph := models.ProxyHost{
			UUID:        "ph-match",
			Name:        "Matched Proxy",
			DomainNames: "www.example.com",
			ForwardHost: "localhost",
			ForwardPort: 8080,
		}
		require.NoError(t, db.Create(&ph).Error)

		cs.InvalidateCache()
		certs, err := cs.ListCertificates()
		assert.NoError(t, err)
		require.Len(t, certs, 1)
		// Should use proxy host name
		assert.Equal(t, "Matched Proxy", certs[0].Name)
		assert.Contains(t, certs[0].Domains, "www.example.com")
	})
}

func TestCertificateService_ListCertificates_CacheBehavior(t *testing.T) {
	t.Run("stale cache triggers background rescan", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

		cs := newTestCertificateService(tmpDir, db)

		// Initialize cache
		err = cs.SyncFromDisk()
		require.NoError(t, err)

		// Get fresh cache
		certs1, err := cs.ListCertificates()
		require.NoError(t, err)
		assert.Len(t, certs1, 0)

		// Artificially make cache stale by setting lastScan way in the past
		cs.cacheMu.Lock()
		cs.lastScan = time.Now().Add(-10 * time.Minute) // More than scanTTL (5 min)
		cs.cacheMu.Unlock()

		// This should still return quickly but trigger background rescan
		certs2, err := cs.ListCertificates()
		require.NoError(t, err)
		assert.Len(t, certs2, 0)

		// Give background goroutine time to complete
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("uninitialized service triggers blocking sync", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

		cs := newTestCertificateService(tmpDir, db)

		// Don't call SyncFromDisk - service is uninitialized
		// Mark as uninitialized
		cs.cacheMu.Lock()
		cs.initialized = false
		cs.cacheMu.Unlock()

		// Should trigger blocking sync on first call
		certs, err := cs.ListCertificates()
		require.NoError(t, err)
		assert.NotNil(t, certs)

		// Should now be initialized
		cs.cacheMu.RLock()
		isInit := cs.initialized
		cs.cacheMu.RUnlock()
		assert.True(t, isInit)
	})

	t.Run("fresh cache returns immediately", func(t *testing.T) {
		tmpDir := t.TempDir()
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

		cs := newTestCertificateService(tmpDir, db)

		// Initialize
		err = cs.SyncFromDisk()
		require.NoError(t, err)

		// Multiple calls should hit cache without blocking
		for i := 0; i < 3; i++ {
			certs, err := cs.ListCertificates()
			require.NoError(t, err)
			assert.NotNil(t, certs)
		}
	})
}

// generateTestCertWithSANs generates a test certificate with Subject Alternative Names
func generateTestCertWithSANs(t *testing.T, cn string, sans []string, expiry time.Time) []byte {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: cn,
		},
		DNSNames:  sans,
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
