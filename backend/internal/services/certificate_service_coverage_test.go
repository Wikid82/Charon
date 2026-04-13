package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
)

// newTestEncryptionService creates a real EncryptionService for tests.
func newTestEncryptionService(t *testing.T) *crypto.EncryptionService {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	keyB64 := base64.StdEncoding.EncodeToString(key)
	svc, err := crypto.NewEncryptionService(keyB64)
	require.NoError(t, err)
	return svc
}

func newTestCertServiceWithEnc(t *testing.T, dataDir string, db *gorm.DB) *CertificateService {
	t.Helper()
	encSvc := newTestEncryptionService(t)
	return &CertificateService{
		dataDir: dataDir,
		db:      db,
		encSvc:  encSvc,
		scanTTL: 5 * time.Minute,
	}
}

func seedCertWithKey(t *testing.T, db *gorm.DB, encSvc *crypto.EncryptionService, uuid, name, domain string, expiry time.Time) models.SSLCertificate {
	t.Helper()
	certPEM, keyPEM := generateTestCertAndKey(t, domain, expiry)

	encKey, err := encSvc.Encrypt(keyPEM)
	require.NoError(t, err)

	cert := models.SSLCertificate{
		UUID:                uuid,
		Name:                name,
		Provider:            "custom",
		Domains:             domain,
		CommonName:          domain,
		Certificate:         string(certPEM),
		PrivateKeyEncrypted: encKey,
		ExpiresAt:           &expiry,
	}
	require.NoError(t, db.Create(&cert).Error)
	return cert
}

func TestCertificateService_GetCertificate(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	t.Run("not found", func(t *testing.T) {
		_, err := cs.GetCertificate("nonexistent-uuid")
		assert.ErrorIs(t, err, ErrCertNotFound)
	})

	t.Run("found with no hosts", func(t *testing.T) {
		expiry := time.Now().Add(30 * 24 * time.Hour)
		notBefore := time.Now().Add(-time.Hour)
		cert := models.SSLCertificate{
			UUID:       "get-cert-1",
			Name:       "Test Cert",
			Provider:   "custom",
			Domains:    "get.example.com",
			CommonName: "get.example.com",
			ExpiresAt:  &expiry,
			NotBefore:  &notBefore,
		}
		require.NoError(t, db.Create(&cert).Error)

		detail, err := cs.GetCertificate("get-cert-1")
		require.NoError(t, err)
		assert.Equal(t, "get-cert-1", detail.UUID)
		assert.Equal(t, "Test Cert", detail.Name)
		assert.Equal(t, "get.example.com", detail.CommonName)
		assert.False(t, detail.InUse)
		assert.Empty(t, detail.AssignedHosts)
	})

	t.Run("found with assigned host", func(t *testing.T) {
		expiry := time.Now().Add(30 * 24 * time.Hour)
		cert := models.SSLCertificate{
			UUID:       "get-cert-2",
			Name:       "Assigned Cert",
			Provider:   "custom",
			Domains:    "assigned.example.com",
			CommonName: "assigned.example.com",
			ExpiresAt:  &expiry,
		}
		require.NoError(t, db.Create(&cert).Error)

		ph := models.ProxyHost{
			UUID:          "ph-assigned",
			Name:          "My Proxy",
			DomainNames:   "assigned.example.com",
			ForwardHost:   "localhost",
			ForwardPort:   8080,
			CertificateID: &cert.ID,
		}
		require.NoError(t, db.Create(&ph).Error)

		detail, err := cs.GetCertificate("get-cert-2")
		require.NoError(t, err)
		assert.True(t, detail.InUse)
		require.Len(t, detail.AssignedHosts, 1)
		assert.Equal(t, "My Proxy", detail.AssignedHosts[0].Name)
	})

	t.Run("nil expiry and not_before", func(t *testing.T) {
		cert := models.SSLCertificate{
			UUID:     "get-cert-3",
			Name:     "No Dates",
			Provider: "custom",
			Domains:  "nodates.example.com",
		}
		require.NoError(t, db.Create(&cert).Error)

		detail, err := cs.GetCertificate("get-cert-3")
		require.NoError(t, err)
		assert.True(t, detail.ExpiresAt.IsZero())
		assert.True(t, detail.NotBefore.IsZero())
	})
}

func TestCertificateService_ValidateCertificate(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	t.Run("valid cert with key", func(t *testing.T) {
		certPEM, keyPEM := generateTestCertAndKey(t, "validate.example.com", time.Now().Add(24*time.Hour))
		result, err := cs.ValidateCertificate(string(certPEM), string(keyPEM), "")
		require.NoError(t, err)
		assert.True(t, result.Valid)
		assert.True(t, result.KeyMatch)
		assert.Empty(t, result.Errors)
	})

	t.Run("invalid cert data", func(t *testing.T) {
		result, err := cs.ValidateCertificate("not-a-cert", "", "")
		require.NoError(t, err)
		assert.False(t, result.Valid)
		assert.NotEmpty(t, result.Errors)
	})

	t.Run("valid cert without key", func(t *testing.T) {
		certPEM := generateTestCert(t, "nokey.example.com", time.Now().Add(24*time.Hour))
		result, err := cs.ValidateCertificate(string(certPEM), "", "")
		require.NoError(t, err)
		assert.True(t, result.Valid)
		assert.False(t, result.KeyMatch)
		assert.Empty(t, result.Errors)
	})

	t.Run("expired cert", func(t *testing.T) {
		certPEM := generateTestCert(t, "expired.example.com", time.Now().Add(-24*time.Hour))
		result, err := cs.ValidateCertificate(string(certPEM), "", "")
		require.NoError(t, err)
		assert.NotEmpty(t, result.Warnings)
	})
}

func TestCertificateService_UpdateCertificate(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	t.Run("not found", func(t *testing.T) {
		_, err := cs.UpdateCertificate("nonexistent-uuid", "New Name")
		assert.ErrorIs(t, err, ErrCertNotFound)
	})

	t.Run("successful rename", func(t *testing.T) {
		expiry := time.Now().Add(30 * 24 * time.Hour)
		cert := models.SSLCertificate{
			UUID:       "update-cert-1",
			Name:       "Old Name",
			Provider:   "custom",
			Domains:    "update.example.com",
			CommonName: "update.example.com",
			ExpiresAt:  &expiry,
		}
		require.NoError(t, db.Create(&cert).Error)

		info, err := cs.UpdateCertificate("update-cert-1", "New Name")
		require.NoError(t, err)
		assert.Equal(t, "New Name", info.Name)
		assert.Equal(t, "update-cert-1", info.UUID)
		assert.Equal(t, "custom", info.Provider)
	})

	t.Run("updates persist", func(t *testing.T) {
		var cert models.SSLCertificate
		require.NoError(t, db.Where("uuid = ?", "update-cert-1").First(&cert).Error)
		assert.Equal(t, "New Name", cert.Name)
	})

	t.Run("nil expiry and not_before", func(t *testing.T) {
		cert := models.SSLCertificate{
			UUID:     "update-cert-2",
			Name:     "No Dates Cert",
			Provider: "custom",
			Domains:  "nodates-update.example.com",
		}
		require.NoError(t, db.Create(&cert).Error)

		info, err := cs.UpdateCertificate("update-cert-2", "Renamed No Dates")
		require.NoError(t, err)
		assert.Equal(t, "Renamed No Dates", info.Name)
		assert.True(t, info.ExpiresAt.IsZero())
	})
}

func TestCertificateService_IsCertificateInUseByUUID(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	t.Run("not found", func(t *testing.T) {
		_, err := cs.IsCertificateInUseByUUID("nonexistent-uuid")
		assert.ErrorIs(t, err, ErrCertNotFound)
	})

	t.Run("not in use", func(t *testing.T) {
		cert := models.SSLCertificate{UUID: "inuse-1", Name: "Free Cert", Provider: "custom", Domains: "free.example.com"}
		require.NoError(t, db.Create(&cert).Error)

		inUse, err := cs.IsCertificateInUseByUUID("inuse-1")
		require.NoError(t, err)
		assert.False(t, inUse)
	})

	t.Run("in use", func(t *testing.T) {
		cert := models.SSLCertificate{UUID: "inuse-2", Name: "Used Cert", Provider: "custom", Domains: "used.example.com"}
		require.NoError(t, db.Create(&cert).Error)

		ph := models.ProxyHost{UUID: "ph-inuse", Name: "Using Proxy", DomainNames: "used.example.com", ForwardHost: "localhost", ForwardPort: 8080, CertificateID: &cert.ID}
		require.NoError(t, db.Create(&ph).Error)

		inUse, err := cs.IsCertificateInUseByUUID("inuse-2")
		require.NoError(t, err)
		assert.True(t, inUse)
	})
}

func TestCertificateService_DeleteCertificateByID(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	cert := models.SSLCertificate{UUID: "del-by-id-1", Name: "Delete By ID", Provider: "custom", Domains: "delbyid.example.com"}
	require.NoError(t, db.Create(&cert).Error)

	err = cs.DeleteCertificateByID(cert.ID)
	require.NoError(t, err)

	var found models.SSLCertificate
	err = db.Where("uuid = ?", "del-by-id-1").First(&found).Error
	assert.Error(t, err)
}

func TestCertificateService_ExportCertificate(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	encSvc := newTestEncryptionService(t)
	cs := newTestCertServiceWithEnc(t, tmpDir, db)

	domain := "export.example.com"
	expiry := time.Now().Add(30 * 24 * time.Hour)
	cert := seedCertWithKey(t, db, encSvc, "export-cert-1", "Export Cert", domain, expiry)

	t.Run("not found", func(t *testing.T) {
		_, _, err := cs.ExportCertificate("nonexistent", "pem", false, "")
		assert.ErrorIs(t, err, ErrCertNotFound)
	})

	t.Run("pem without key", func(t *testing.T) {
		data, filename, err := cs.ExportCertificate(cert.UUID, "pem", false, "")
		require.NoError(t, err)
		assert.Equal(t, "Export Cert.pem", filename)
		assert.Contains(t, string(data), "BEGIN CERTIFICATE")
	})

	t.Run("pem with key", func(t *testing.T) {
		data, filename, err := cs.ExportCertificate(cert.UUID, "pem", true, "")
		require.NoError(t, err)
		assert.Equal(t, "Export Cert.pem", filename)
		assert.Contains(t, string(data), "BEGIN CERTIFICATE")
		assert.Contains(t, string(data), "PRIVATE KEY")
	})

	t.Run("der format", func(t *testing.T) {
		data, filename, err := cs.ExportCertificate(cert.UUID, "der", false, "")
		require.NoError(t, err)
		assert.Equal(t, "Export Cert.der", filename)
		assert.NotEmpty(t, data)
	})

	t.Run("pfx format", func(t *testing.T) {
		data, filename, err := cs.ExportCertificate(cert.UUID, "pfx", false, "")
		require.NoError(t, err)
		assert.Equal(t, "Export Cert.pfx", filename)
		assert.NotEmpty(t, data)
	})

	t.Run("unsupported format", func(t *testing.T) {
		_, _, err := cs.ExportCertificate(cert.UUID, "jks", false, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported export format")
	})

	t.Run("empty name uses fallback", func(t *testing.T) {
		noNameCert := seedCertWithKey(t, db, encSvc, "export-noname", "", domain, expiry)
		_, filename, err := cs.ExportCertificate(noNameCert.UUID, "pem", false, "")
		require.NoError(t, err)
		assert.Equal(t, "certificate.pem", filename)
	})
}

func TestCertificateService_GetDecryptedPrivateKey(t *testing.T) {
	encSvc := newTestEncryptionService(t)

	t.Run("no encrypted key", func(t *testing.T) {
		cs := &CertificateService{encSvc: encSvc}
		cert := &models.SSLCertificate{PrivateKeyEncrypted: ""}
		_, err := cs.GetDecryptedPrivateKey(cert)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no encrypted private key")
	})

	t.Run("no encryption service", func(t *testing.T) {
		cs := &CertificateService{encSvc: nil}
		cert := &models.SSLCertificate{PrivateKeyEncrypted: "some-data"}
		_, err := cs.GetDecryptedPrivateKey(cert)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "encryption service not configured")
	})

	t.Run("successful decryption", func(t *testing.T) {
		cs := &CertificateService{encSvc: encSvc}
		plaintext := "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----" //nolint:gosec // test data, not real credentials
		encrypted, err := encSvc.Encrypt([]byte(plaintext))
		require.NoError(t, err)

		cert := &models.SSLCertificate{PrivateKeyEncrypted: encrypted}
		result, err := cs.GetDecryptedPrivateKey(cert)
		require.NoError(t, err)
		assert.Equal(t, plaintext, result)
	})
}

func TestCertificateService_CheckExpiringCertificates(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	// Create certs with different expiry states
	expiringSoon := time.Now().Add(5 * 24 * time.Hour)
	expired := time.Now().Add(-24 * time.Hour)
	farFuture := time.Now().Add(365 * 24 * time.Hour)

	db.Create(&models.SSLCertificate{UUID: "exp-soon", Name: "Expiring Soon", Provider: "custom", Domains: "soon.example.com", ExpiresAt: &expiringSoon})
	db.Create(&models.SSLCertificate{UUID: "exp-past", Name: "Already Expired", Provider: "custom", Domains: "expired.example.com", ExpiresAt: &expired})
	db.Create(&models.SSLCertificate{UUID: "exp-far", Name: "Far Future", Provider: "custom", Domains: "far.example.com", ExpiresAt: &farFuture})
	// ACME certs should not be included (only custom)
	db.Create(&models.SSLCertificate{UUID: "exp-le", Name: "LE Cert", Provider: "letsencrypt", Domains: "le.example.com", ExpiresAt: &expiringSoon})

	t.Run("30 day window", func(t *testing.T) {
		certs, err := cs.CheckExpiringCertificates(30)
		require.NoError(t, err)
		assert.Len(t, certs, 2) // expiringSoon and expired

		foundSoon := false
		foundExpired := false
		for _, c := range certs {
			if c.UUID == "exp-soon" {
				foundSoon = true
			}
			if c.UUID == "exp-past" {
				foundExpired = true
			}
		}
		assert.True(t, foundSoon)
		assert.True(t, foundExpired)
	})

	t.Run("1 day window", func(t *testing.T) {
		certs, err := cs.CheckExpiringCertificates(1)
		require.NoError(t, err)
		assert.Len(t, certs, 1) // only the expired one
		assert.Equal(t, "exp-past", certs[0].UUID)
	})
}

func TestCertificateService_CheckExpiry(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.Setting{}, &models.NotificationProvider{}, &models.Notification{}))

	cs := newTestCertificateService(tmpDir, db)
	ns := NewNotificationService(db, nil)

	expiringSoon := time.Now().Add(5 * 24 * time.Hour)
	expired := time.Now().Add(-24 * time.Hour)
	db.Create(&models.SSLCertificate{UUID: "chk-soon", Name: "Expiring", Provider: "custom", Domains: "chksoon.example.com", ExpiresAt: &expiringSoon})
	db.Create(&models.SSLCertificate{UUID: "chk-past", Name: "Expired", Provider: "custom", Domains: "chkpast.example.com", ExpiresAt: &expired})

	t.Run("nil notification service", func(t *testing.T) {
		cs.checkExpiry(context.Background(), nil, 30)
	})

	t.Run("creates notifications for expiring certs", func(t *testing.T) {
		cs.checkExpiry(context.Background(), ns, 30)

		var notifications []models.Notification
		db.Find(&notifications)
		assert.GreaterOrEqual(t, len(notifications), 2)
	})
}

func TestCertificateService_MigratePrivateKeys(t *testing.T) {
	t.Run("no encryption service", func(t *testing.T) {
		cs := &CertificateService{encSvc: nil}
		err := cs.MigratePrivateKeys()
		require.NoError(t, err)
	})

	t.Run("no keys to migrate", func(t *testing.T) {
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))
		// MigratePrivateKeys uses raw SQL referencing private_key column (gorm:"-" tag)
		require.NoError(t, db.Exec("ALTER TABLE ssl_certificates ADD COLUMN private_key TEXT DEFAULT ''").Error)

		encSvc := newTestEncryptionService(t)
		cs := &CertificateService{db: db, encSvc: encSvc}

		err = cs.MigratePrivateKeys()
		require.NoError(t, err)
	})

	t.Run("migrates plaintext key", func(t *testing.T) {
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))
		// MigratePrivateKeys uses raw SQL referencing private_key column (gorm:"-" tag)
		require.NoError(t, db.Exec("ALTER TABLE ssl_certificates ADD COLUMN private_key TEXT DEFAULT ''").Error)

		// Insert cert with plaintext key using raw SQL{}))
		// MigratePrivateKeys uses raw SQL referencing private_key column (gorm:"-" tag)
		require.NoError(t, db.Exec("ALTER TABLE ssl_certificates ADD COLUMN private_key TEXT DEFAULT ''").Error)

		// Insert cert with plaintext key using raw SQL
		require.NoError(t, db.Exec(
			"INSERT INTO ssl_certificates (uuid, name, provider, domains, private_key) VALUES (?, ?, ?, ?, ?)",
			"migrate-1", "Migrate Test", "custom", "migrate.example.com", "plaintext-key-data",
		).Error)

		encSvc := newTestEncryptionService(t)
		cs := &CertificateService{db: db, encSvc: encSvc}

		err = cs.MigratePrivateKeys()
		require.NoError(t, err)

		// Verify the key was encrypted and plaintext cleared
		type rawRow struct {
			PrivateKey    string `gorm:"column:private_key"`
			PrivateKeyEnc string `gorm:"column:private_key_enc"`
		}
		var row rawRow
		require.NoError(t, db.Raw("SELECT private_key, private_key_enc FROM ssl_certificates WHERE uuid = ?", "migrate-1").Scan(&row).Error)
		assert.Empty(t, row.PrivateKey)
		assert.NotEmpty(t, row.PrivateKeyEnc)
	})
}
