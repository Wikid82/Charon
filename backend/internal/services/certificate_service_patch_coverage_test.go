package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// --- ExportCertificate DER format ---

func TestExportCertificate_DER(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)
	certPEM, keyPEM := generateTestCertAndKey(t, "der-export.example.com", time.Now().Add(24*time.Hour))
	info, err := cs.UploadCertificate("der-export", string(certPEM), string(keyPEM), "")
	require.NoError(t, err)

	data, filename, err := cs.ExportCertificate(info.UUID, "der", false, "")
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, filename, ".der")
}

// --- ExportCertificate PFX format ---

func TestExportCertificate_PFX(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertServiceWithEnc(t, tmpDir, db)
	certPEM, keyPEM := generateTestCertAndKey(t, "pfx-export.example.com", time.Now().Add(24*time.Hour))
	info, err := cs.UploadCertificate("pfx-export", string(certPEM), string(keyPEM), "")
	require.NoError(t, err)

	data, filename, err := cs.ExportCertificate(info.UUID, "pfx", true, "test-password")
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, filename, ".pfx")
}

func TestExportCertificate_P12(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertServiceWithEnc(t, tmpDir, db)
	certPEM, keyPEM := generateTestCertAndKey(t, "p12-export.example.com", time.Now().Add(24*time.Hour))
	info, err := cs.UploadCertificate("p12-export", string(certPEM), string(keyPEM), "")
	require.NoError(t, err)

	data, filename, err := cs.ExportCertificate(info.UUID, "p12", true, "password")
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, filename, ".pfx")
}

func TestExportCertificate_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)
	certPEM, keyPEM := generateTestCertAndKey(t, "unsupported.example.com", time.Now().Add(24*time.Hour))
	info, err := cs.UploadCertificate("unsupported-fmt", string(certPEM), string(keyPEM), "")
	require.NoError(t, err)

	_, _, err = cs.ExportCertificate(info.UUID, "xml", false, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported export format")
}

func TestExportCertificate_PEMWithKey(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertServiceWithEnc(t, tmpDir, db)
	certPEM, keyPEM := generateTestCertAndKey(t, "pem-key.example.com", time.Now().Add(24*time.Hour))
	info, err := cs.UploadCertificate("pem-key-export", string(certPEM), string(keyPEM), "")
	require.NoError(t, err)

	data, filename, err := cs.ExportCertificate(info.UUID, "pem", true, "")
	require.NoError(t, err)
	assert.Contains(t, string(data), "PRIVATE KEY")
	assert.Contains(t, filename, ".pem")
}

func TestExportCertificate_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)
	_, _, err = cs.ExportCertificate("nonexistent-uuid", "pem", false, "")
	assert.ErrorIs(t, err, ErrCertNotFound)
}

// --- GetDecryptedPrivateKey ---

func TestGetDecryptedPrivateKey_NoEncryptedKey(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)
	cert := &models.SSLCertificate{PrivateKeyEncrypted: ""}
	_, err = cs.GetDecryptedPrivateKey(cert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no encrypted private key")
}

func TestGetDecryptedPrivateKey_NoEncryptionService(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db) // no encSvc
	cert := &models.SSLCertificate{PrivateKeyEncrypted: "some-encrypted-data"}
	_, err = cs.GetDecryptedPrivateKey(cert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "encryption service not configured")
}

// --- MigratePrivateKeys ---

func TestMigratePrivateKeys_NoEncryptionService(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)
	err = cs.MigratePrivateKeys()
	assert.NoError(t, err) // should return nil without error
}

func TestMigratePrivateKeys_NoCertsToMigrate(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))
	// MigratePrivateKeys uses raw SQL against private_key column (gorm:"-"), so add it manually
	db.Exec("ALTER TABLE ssl_certificates ADD COLUMN private_key TEXT DEFAULT ''")

	cs := newTestCertServiceWithEnc(t, tmpDir, db)
	err = cs.MigratePrivateKeys()
	assert.NoError(t, err)
}

func TestMigratePrivateKeys_WithPlaintextKey(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))
	// MigratePrivateKeys uses raw SQL against private_key column (gorm:"-"), so add it manually
	db.Exec("ALTER TABLE ssl_certificates ADD COLUMN private_key TEXT DEFAULT ''")
	cs := newTestCertServiceWithEnc(t, tmpDir, db)
	_, keyPEM := generateTestCertAndKey(t, "migrate.example.com", time.Now().Add(24*time.Hour))

	// Insert a cert with plaintext private_key via raw SQL
	db.Exec("INSERT INTO ssl_certificates (uuid, name, provider, domains, common_name, private_key) VALUES (?, ?, ?, ?, ?, ?)",
		"migrate-uuid", "Migrate Test", "custom", "migrate.example.com", "migrate.example.com", string(keyPEM))

	err = cs.MigratePrivateKeys()
	assert.NoError(t, err)

	// Verify the key was encrypted
	var encKey string
	db.Raw("SELECT private_key_enc FROM ssl_certificates WHERE uuid = ?", "migrate-uuid").Scan(&encKey)
	assert.NotEmpty(t, encKey)

	// Verify plaintext key was cleared
	var plainKey string
	db.Raw("SELECT private_key FROM ssl_certificates WHERE uuid = ?", "migrate-uuid").Scan(&plainKey)
	assert.Empty(t, plainKey)
}

// --- DeleteCertificateByID ---

func TestDeleteCertificateByID_Success(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)
	certPEM, keyPEM := generateTestCertAndKey(t, "byid.example.com", time.Now().Add(24*time.Hour))
	info, err := cs.UploadCertificate("by-id-delete", string(certPEM), string(keyPEM), "")
	require.NoError(t, err)

	var stored models.SSLCertificate
	require.NoError(t, db.Where("uuid = ?", info.UUID).First(&stored).Error)

	err = cs.DeleteCertificateByID(stored.ID)
	assert.NoError(t, err)
}

func TestDeleteCertificateByID_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)
	err = cs.DeleteCertificateByID(99999)
	assert.Error(t, err)
}

// --- UpdateCertificate ---

func TestUpdateCertificate_Success(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)
	certPEM, keyPEM := generateTestCertAndKey(t, "update.example.com", time.Now().Add(24*time.Hour))
	info, err := cs.UploadCertificate("old-name", string(certPEM), string(keyPEM), "")
	require.NoError(t, err)

	updated, err := cs.UpdateCertificate(info.UUID, "new-name")
	require.NoError(t, err)
	assert.Equal(t, "new-name", updated.Name)
}

func TestUpdateCertificate_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)
	_, err = cs.UpdateCertificate("nonexistent", "name")
	assert.ErrorIs(t, err, ErrCertNotFound)
}

// --- IsCertificateInUseByUUID ---

func TestIsCertificateInUseByUUID_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)
	_, err = cs.IsCertificateInUseByUUID("nonexistent-uuid")
	assert.ErrorIs(t, err, ErrCertNotFound)
}

func TestIsCertificateInUseByUUID_NotInUse(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)
	certPEM, keyPEM := generateTestCertAndKey(t, "inuse-uuid.example.com", time.Now().Add(24*time.Hour))
	info, err := cs.UploadCertificate("uuid-inuse-test", string(certPEM), string(keyPEM), "")
	require.NoError(t, err)

	inUse, err := cs.IsCertificateInUseByUUID(info.UUID)
	require.NoError(t, err)
	assert.False(t, inUse)
}

// --- CheckExpiringCertificates ---

func TestCheckExpiringCertificates_WithExpiring(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	// Create a cert expiring in 10 days
	expiry := time.Now().Add(10 * 24 * time.Hour)
	notBefore := time.Now().Add(-24 * time.Hour)
	db.Create(&models.SSLCertificate{
		UUID: "expiring-uuid", Name: "Expiring Cert", Provider: "custom",
		Domains: "expiring.example.com", CommonName: "expiring.example.com",
		ExpiresAt: &expiry, NotBefore: &notBefore,
	})

	certs, err := cs.CheckExpiringCertificates(30)
	require.NoError(t, err)
	assert.Len(t, certs, 1)
	assert.Equal(t, "Expiring Cert", certs[0].Name)
	assert.Equal(t, "expiring", certs[0].Status)
}

func TestCheckExpiringCertificates_WithExpired(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	expiry := time.Now().Add(-24 * time.Hour)
	db.Create(&models.SSLCertificate{
		UUID: "expired-uuid", Name: "Expired Cert", Provider: "custom",
		Domains: "expired.example.com", CommonName: "expired.example.com",
		ExpiresAt: &expiry,
	})

	certs, err := cs.CheckExpiringCertificates(30)
	require.NoError(t, err)
	assert.Len(t, certs, 1)
	assert.Equal(t, "expired", certs[0].Status)
}

func TestCheckExpiringCertificates_NoneExpiring(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	// Cert expiring in 90 days - outside 30 day window
	expiry := time.Now().Add(90 * 24 * time.Hour)
	db.Create(&models.SSLCertificate{
		UUID: "valid-uuid", Name: "Valid Cert", Provider: "custom",
		Domains: "valid.example.com", CommonName: "valid.example.com",
		ExpiresAt: &expiry,
	})

	certs, err := cs.CheckExpiringCertificates(30)
	require.NoError(t, err)
	assert.Empty(t, certs)
}

// --- checkExpiry with notification service ---

func TestCheckExpiry_WithExpiringCerts(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.SSLCertificate{}, &models.ProxyHost{},
		&models.Setting{}, &models.NotificationProvider{},
		&models.Notification{},
	))

	cs := newTestCertificateService(tmpDir, db)

	// Create expiring cert
	expiry := time.Now().Add(10 * 24 * time.Hour)
	db.Create(&models.SSLCertificate{
		UUID: "notify-expiring", Name: "Notify Cert", Provider: "custom",
		Domains: "notify.example.com", CommonName: "notify.example.com",
		ExpiresAt: &expiry,
	})

	notifSvc := NewNotificationService(db, nil)
	cs.checkExpiry(context.Background(), notifSvc, 30)

	// Verify a notification was created
	var count int64
	db.Model(&models.Notification{}).Count(&count)
	assert.Greater(t, count, int64(0))
}

func TestCheckExpiry_WithExpiredCerts(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.SSLCertificate{}, &models.ProxyHost{},
		&models.Setting{}, &models.NotificationProvider{},
		&models.Notification{},
	))

	cs := newTestCertificateService(tmpDir, db)

	expiry := time.Now().Add(-24 * time.Hour)
	db.Create(&models.SSLCertificate{
		UUID: "notify-expired", Name: "Expired Notify", Provider: "custom",
		Domains: "expired-notify.example.com", CommonName: "expired-notify.example.com",
		ExpiresAt: &expiry,
	})

	notifSvc := NewNotificationService(db, nil)
	cs.checkExpiry(context.Background(), notifSvc, 30)

	var count int64
	db.Model(&models.Notification{}).Count(&count)
	assert.Greater(t, count, int64(0))
}

// --- ListCertificates with chain and proxy host ---

func TestListCertificates_WithChainAndProxyHost(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	certPEM, _, err := generateSelfSignedCertPEM()
	require.NoError(t, err)

	chainPEM := certPEM + "\n" + certPEM

	expiry := time.Now().Add(90 * 24 * time.Hour)
	notBefore := time.Now().Add(-1 * time.Hour)
	certID := uint(99)
	db.Create(&models.SSLCertificate{
		ID:               certID,
		UUID:             "chain-test-uuid",
		Name:             "Chain Test",
		Provider:         "custom",
		Domains:          "chain.example.com",
		CommonName:       "chain.example.com",
		Certificate:      certPEM,
		CertificateChain: chainPEM,
		ExpiresAt:        &expiry,
		NotBefore:        &notBefore,
	})

	db.Create(&models.ProxyHost{
		Name:          "My Proxy",
		DomainNames:   "chain.example.com",
		CertificateID: &certID,
	})

	certs, err := cs.ListCertificates()
	require.NoError(t, err)
	require.Len(t, certs, 1)
	assert.Equal(t, 2, certs[0].ChainDepth)
	assert.True(t, certs[0].InUse)
	assert.Equal(t, "chain-test-uuid", certs[0].UUID)
}

// --- UploadCertificate with key ---

func TestUploadCertificate_WithKey(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertServiceWithEnc(t, tmpDir, db)

	certPEM, keyPEM, err := generateSelfSignedCertPEM()
	require.NoError(t, err)

	info, err := cs.UploadCertificate("My Upload", certPEM, keyPEM, "")
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "My Upload", info.Name)
	assert.True(t, info.HasKey)
	assert.NotEmpty(t, info.UUID)
	assert.Equal(t, "custom", info.Provider)
}

// --- ValidateCertificate with key match ---

func TestValidateCertificate_WithKeyMatch(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	certPEM, keyPEM, err := generateSelfSignedCertPEM()
	require.NoError(t, err)

	result, err := cs.ValidateCertificate(certPEM, keyPEM, "")
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.True(t, result.KeyMatch)
	assert.Empty(t, result.Errors)
	assert.Contains(t, result.Warnings, "certificate could not be verified against system roots")
}

// --- UpdateCertificate with chain depth ---

func TestUpdateCertificate_WithChainDepth(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertificateService(tmpDir, db)

	certPEM, _, err := generateSelfSignedCertPEM()
	require.NoError(t, err)

	chainPEM := certPEM + "\n" + certPEM + "\n" + certPEM

	expiry := time.Now().Add(90 * 24 * time.Hour)
	db.Create(&models.SSLCertificate{
		UUID:             "update-chain-uuid",
		Name:             "Chain Update",
		Provider:         "custom",
		Domains:          "update-chain.example.com",
		CommonName:       "update-chain.example.com",
		Certificate:      certPEM,
		CertificateChain: chainPEM,
		ExpiresAt:        &expiry,
	})

	info, err := cs.UpdateCertificate("update-chain-uuid", "Renamed Chain")
	require.NoError(t, err)
	assert.Equal(t, "Renamed Chain", info.Name)
	assert.Equal(t, 3, info.ChainDepth)
}

// --- ExportCertificate PEM with chain ---

func TestExportCertificate_PEMWithChain(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertServiceWithEnc(t, tmpDir, db)

	certPEM, keyPEM, err := generateSelfSignedCertPEM()
	require.NoError(t, err)

	encSvc := newTestEncryptionService(t)
	encKey, err := encSvc.Encrypt([]byte(keyPEM))
	require.NoError(t, err)

	chainPEM := certPEM

	db.Create(&models.SSLCertificate{
		UUID:                "export-chain-uuid",
		Name:                "Export Chain",
		Provider:            "custom",
		Domains:             "export-chain.example.com",
		CommonName:          "export-chain.example.com",
		Certificate:         certPEM,
		CertificateChain:    chainPEM,
		PrivateKeyEncrypted: encKey,
	})

	data, filename, err := cs.ExportCertificate("export-chain-uuid", "pem", true, "")
	require.NoError(t, err)
	assert.Equal(t, "Export Chain.pem", filename)
	assert.Contains(t, string(data), "BEGIN CERTIFICATE")
	assert.Contains(t, string(data), "BEGIN")
}
