package services

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// --- buildChainEntries ---

func TestBuildChainEntries(t *testing.T) {
	certPEM := string(generateTestCert(t, "leaf.example.com", time.Now().Add(24*time.Hour)))
	chainPEM := string(generateTestCert(t, "ca.example.com", time.Now().Add(365*24*time.Hour)))

	t.Run("leaf only", func(t *testing.T) {
		entries := buildChainEntries(certPEM, "")
		require.Len(t, entries, 1)
		assert.Equal(t, "leaf.example.com", entries[0].Subject)
	})

	t.Run("leaf and chain", func(t *testing.T) {
		entries := buildChainEntries(certPEM, chainPEM)
		require.Len(t, entries, 2)
		assert.Equal(t, "leaf.example.com", entries[0].Subject)
		assert.Equal(t, "ca.example.com", entries[1].Subject)
	})

	t.Run("empty cert", func(t *testing.T) {
		entries := buildChainEntries("", chainPEM)
		require.Len(t, entries, 1)
		assert.Equal(t, "ca.example.com", entries[0].Subject)
	})

	t.Run("both empty", func(t *testing.T) {
		entries := buildChainEntries("", "")
		assert.Empty(t, entries)
	})

	t.Run("invalid PEM ignored", func(t *testing.T) {
		entries := buildChainEntries("not-pem", "also-not-pem")
		assert.Empty(t, entries)
	})
}

// --- certStatus ---

func TestCertStatus(t *testing.T) {
	now := time.Now()

	t.Run("valid", func(t *testing.T) {
		expiry := now.Add(60 * 24 * time.Hour)
		cert := models.SSLCertificate{ExpiresAt: &expiry, Provider: "custom"}
		assert.Equal(t, "valid", certStatus(cert))
	})

	t.Run("expired", func(t *testing.T) {
		expiry := now.Add(-time.Hour)
		cert := models.SSLCertificate{ExpiresAt: &expiry, Provider: "custom"}
		assert.Equal(t, "expired", certStatus(cert))
	})

	t.Run("expiring soon", func(t *testing.T) {
		expiry := now.Add(15 * 24 * time.Hour) // within 30d window
		cert := models.SSLCertificate{ExpiresAt: &expiry, Provider: "custom"}
		assert.Equal(t, "expiring", certStatus(cert))
	})

	t.Run("staging provider", func(t *testing.T) {
		expiry := now.Add(60 * 24 * time.Hour)
		cert := models.SSLCertificate{ExpiresAt: &expiry, Provider: "letsencrypt-staging"}
		assert.Equal(t, "untrusted", certStatus(cert))
	})

	t.Run("nil expiry", func(t *testing.T) {
		cert := models.SSLCertificate{Provider: "custom"}
		assert.Equal(t, "valid", certStatus(cert))
	})
}

// --- ListCertificates cache paths ---

func TestListCertificates_InitializedAndStale(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	// First call initializes
	certs1, err := cs.ListCertificates()
	require.NoError(t, err)
	assert.Empty(t, certs1)

	// Force stale but initialized
	cs.cacheMu.Lock()
	cs.initialized = true
	cs.lastScan = time.Time{} // zero → stale
	cs.cacheMu.Unlock()

	// Should still return (stale) cache and trigger background sync
	certs2, err := cs.ListCertificates()
	require.NoError(t, err)
	assert.NotNil(t, certs2)
}

func TestListCertificates_CacheFresh(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s_fresh?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)
	cs.cacheMu.Lock()
	cs.initialized = true
	cs.lastScan = time.Now()
	cs.cache = []CertificateInfo{{Name: "cached"}}
	cs.scanTTL = 5 * time.Minute
	cs.cacheMu.Unlock()

	certs, err := cs.ListCertificates()
	require.NoError(t, err)
	require.Len(t, certs, 1)
	assert.Equal(t, "cached", certs[0].Name)
}

// --- ValidateCertificate extra branches ---

func TestValidateCertificate_KeyMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	// Generate two separate cert/key pairs so key doesn't match cert
	certPEM, _ := generateTestCertAndKey(t, "mismatch.example.com", time.Now().Add(24*time.Hour))
	_, keyPEM := generateTestCertAndKey(t, "other.example.com", time.Now().Add(24*time.Hour))

	result, err := cs.ValidateCertificate(string(certPEM), string(keyPEM), "")
	require.NoError(t, err)
	// Key mismatch goes to Errors
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e, "mismatch") {
			found = true
		}
	}
	assert.True(t, found, "expected key mismatch error, got errors: %v, warnings: %v", result.Errors, result.Warnings)
}

// --- UploadCertificate with encryption ---

func TestUploadCertificate_WithEncryption(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertServiceWithEnc(t, tmpDir, db)

	certPEM, keyPEM := generateTestCertAndKey(t, "enc.example.com", time.Now().Add(24*time.Hour))
	info, err := cs.UploadCertificate("encrypted-cert", string(certPEM), string(keyPEM), "")
	require.NoError(t, err)
	assert.Equal(t, "encrypted-cert", info.Name)

	// Verify private key was encrypted in DB
	var stored models.SSLCertificate
	require.NoError(t, db.Where("uuid = ?", info.UUID).First(&stored).Error)
	assert.NotEmpty(t, stored.PrivateKeyEncrypted)
	assert.Empty(t, stored.PrivateKey) // should not store plaintext
}

// --- checkExpiry additional branches ---

func TestCheckExpiry_NoNotificationService(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.Setting{}, &models.NotificationProvider{}))

	cs := &CertificateService{
		dataDir: tmpDir,
		db:      db,
		scanTTL: 5 * time.Minute,
	}
	// No notification service set — should not panic
	cs.checkExpiry(context.Background(), nil, 30)
}

// --- DeleteCertificate with backup service ---

func TestDeleteCertificate_Success(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)
	certPEM, keyPEM := generateTestCertAndKey(t, "delete.example.com", time.Now().Add(24*time.Hour))
	info, err := cs.UploadCertificate("to-delete", string(certPEM), string(keyPEM), "")
	require.NoError(t, err)

	err = cs.DeleteCertificate(info.UUID)
	assert.NoError(t, err)

	// Verify deleted
	_, err = cs.GetCertificate(info.UUID)
	assert.ErrorIs(t, err, ErrCertNotFound)
}

func TestDeleteCertificate_InUse(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)
	certPEM, keyPEM := generateTestCertAndKey(t, "inuse.example.com", time.Now().Add(24*time.Hour))
	info, err := cs.UploadCertificate("in-use-cert", string(certPEM), string(keyPEM), "")
	require.NoError(t, err)

	// Find the cert and assign to a host
	var stored models.SSLCertificate
	require.NoError(t, db.Where("uuid = ?", info.UUID).First(&stored).Error)
	ph := models.ProxyHost{
		UUID:          "ph-inuse",
		Name:          "InUse Host",
		DomainNames:   "inuse.example.com",
		ForwardHost:   "localhost",
		ForwardPort:   8080,
		CertificateID: &stored.ID,
	}
	require.NoError(t, db.Create(&ph).Error)

	err = cs.DeleteCertificate(info.UUID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "in use")
}

// --- IsCertificateInUse ---

func TestIsCertificateInUse(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	cert := models.SSLCertificate{
		UUID: "inuse-test", Name: "In Use Test", Provider: "custom",
		Domains: "test.example.com", CommonName: "test.example.com",
	}
	require.NoError(t, db.Create(&cert).Error)

	t.Run("not in use", func(t *testing.T) {
		inUse, err := cs.IsCertificateInUse(cert.ID)
		require.NoError(t, err)
		assert.False(t, inUse)
	})

	t.Run("in use", func(t *testing.T) {
		ph := models.ProxyHost{
			UUID: "ph-check", Name: "Check Host", DomainNames: "test.example.com",
			ForwardHost: "localhost", ForwardPort: 8080, CertificateID: &cert.ID,
		}
		require.NoError(t, db.Create(&ph).Error)

		inUse, err := cs.IsCertificateInUse(cert.ID)
		require.NoError(t, err)
		assert.True(t, inUse)
	})
}
