package services

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestSyncFromDisk_StagingToProductionUpgrade(t *testing.T) {
	tmpDir := t.TempDir()
	certRoot := filepath.Join(tmpDir, "certificates")
	require.NoError(t, os.MkdirAll(certRoot, 0755))

	domain := "staging-upgrade.example.com"
	certPEM, _ := generateTestCertAndKey(t, domain, time.Now().Add(24*time.Hour))

	certFile := filepath.Join(certRoot, domain+".crt")
	require.NoError(t, os.WriteFile(certFile, certPEM, 0600))

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	existing := models.SSLCertificate{
		UUID:        uuid.New().String(),
		Name:        domain,
		Provider:    "letsencrypt-staging",
		Domains:     domain,
		Certificate: "old-content",
	}
	require.NoError(t, db.Create(&existing).Error)

	svc := newTestCertificateService(tmpDir, db)
	require.NoError(t, svc.SyncFromDisk())

	var updated models.SSLCertificate
	require.NoError(t, db.Where("uuid = ?", existing.UUID).First(&updated).Error)
	assert.Equal(t, "letsencrypt", updated.Provider)
}

func TestSyncFromDisk_ExpiryOnlyUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	certRoot := filepath.Join(tmpDir, "certificates")
	require.NoError(t, os.MkdirAll(certRoot, 0755))

	domain := "expiry-only.example.com"
	certPEM, _ := generateTestCertAndKey(t, domain, time.Now().Add(24*time.Hour))

	certFile := filepath.Join(certRoot, domain+".crt")
	require.NoError(t, os.WriteFile(certFile, certPEM, 0600))

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	existing := models.SSLCertificate{
		UUID:        uuid.New().String(),
		Name:        domain,
		Provider:    "letsencrypt",
		Domains:     domain,
		Certificate: string(certPEM), // identical content
	}
	require.NoError(t, db.Create(&existing).Error)

	svc := newTestCertificateService(tmpDir, db)
	require.NoError(t, svc.SyncFromDisk())

	var updated models.SSLCertificate
	require.NoError(t, db.Where("uuid = ?", existing.UUID).First(&updated).Error)
	assert.Equal(t, "letsencrypt", updated.Provider)
	assert.Equal(t, string(certPEM), updated.Certificate)
}

func TestSyncFromDisk_CertRootStatPermissionError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission error as root")
	}

	tmpDir := t.TempDir()
	certRoot := filepath.Join(tmpDir, "certificates")
	require.NoError(t, os.MkdirAll(certRoot, 0755))

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	// Restrict parent dir so os.Stat(certRoot) fails with permission error
	require.NoError(t, os.Chmod(tmpDir, 0))
	defer func() { _ = os.Chmod(tmpDir, 0755) }()

	svc := newTestCertificateService(tmpDir, db)
	err = svc.SyncFromDisk()
	require.NoError(t, err)
}

func TestListCertificates_StaleCache_TriggersBackgroundSync(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	svc := newTestCertificateService(tmpDir, db)

	// Simulate stale cache
	svc.cacheMu.Lock()
	svc.initialized = true
	svc.lastScan = time.Now().Add(-10 * time.Minute)
	before := svc.lastScan
	svc.cacheMu.Unlock()

	_, err = svc.ListCertificates()
	require.NoError(t, err)

	// Background goroutine should update lastScan via SyncFromDisk
	require.Eventually(t, func() bool {
		svc.cacheMu.RLock()
		defer svc.cacheMu.RUnlock()
		return svc.lastScan.After(before)
	}, 2*time.Second, 10*time.Millisecond)
}

func TestGetDecryptedPrivateKey_DecryptFails(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	svc := newTestCertServiceWithEnc(t, tmpDir, db)

	cert := models.SSLCertificate{
		UUID:                uuid.New().String(),
		Name:                "enc-fail",
		Domains:             "encfail.example.com",
		Provider:            "custom",
		PrivateKeyEncrypted: "corrupted-ciphertext",
	}
	require.NoError(t, db.Create(&cert).Error)

	_, err = svc.GetDecryptedPrivateKey(&cert)
	assert.Error(t, err)
}

func TestDeleteCertificate_LetsEncryptProvider_FileCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	certRoot := filepath.Join(tmpDir, "certificates")
	require.NoError(t, os.MkdirAll(certRoot, 0755))

	domain := "le-cleanup.example.com"
	certFile := filepath.Join(certRoot, domain+".crt")
	keyFile := filepath.Join(certRoot, domain+".key")
	jsonFile := filepath.Join(certRoot, domain+".json")

	certPEM, _ := generateTestCertAndKey(t, domain, time.Now().Add(24*time.Hour))
	require.NoError(t, os.WriteFile(certFile, certPEM, 0600))
	require.NoError(t, os.WriteFile(keyFile, []byte("key"), 0600))
	require.NoError(t, os.WriteFile(jsonFile, []byte("{}"), 0600))

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	certUUID := uuid.New().String()
	cert := models.SSLCertificate{
		UUID:     certUUID,
		Name:     domain,
		Provider: "letsencrypt",
		Domains:  domain,
	}
	require.NoError(t, db.Create(&cert).Error)

	svc := newTestCertificateService(tmpDir, db)
	require.NoError(t, svc.DeleteCertificate(certUUID))

	assert.NoFileExists(t, certFile)
	assert.NoFileExists(t, keyFile)
	assert.NoFileExists(t, jsonFile)
}

func TestDeleteCertificate_StagingProvider_FileCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	certRoot := filepath.Join(tmpDir, "certificates")
	require.NoError(t, os.MkdirAll(certRoot, 0755))

	domain := "le-staging-cleanup.example.com"
	certFile := filepath.Join(certRoot, domain+".crt")

	certPEM, _ := generateTestCertAndKey(t, domain, time.Now().Add(24*time.Hour))
	require.NoError(t, os.WriteFile(certFile, certPEM, 0600))

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	certUUID := uuid.New().String()
	cert := models.SSLCertificate{
		UUID:     certUUID,
		Name:     domain,
		Provider: "letsencrypt-staging",
		Domains:  domain,
	}
	require.NoError(t, db.Create(&cert).Error)

	svc := newTestCertificateService(tmpDir, db)
	require.NoError(t, svc.DeleteCertificate(certUUID))

	assert.NoFileExists(t, certFile)
}

func TestCheckExpiringCertificates_DBError(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	// deliberately do NOT AutoMigrate SSLCertificate

	svc := newTestCertificateService(tmpDir, db)
	_, err = svc.CheckExpiringCertificates(30)
	assert.Error(t, err)
}
