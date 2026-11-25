package services

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
)

// newPrivateKeyStorageDB opens a file-backed database with the certificate and
// key-version tables.
func newPrivateKeyStorageDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "upload.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.EncryptionKeyVersion{}))
	return db
}

func TestUploadCertificate_RefusesPrivateKeyWithoutEncryptionKey(t *testing.T) {
	// E23: with no key service, a key-bearing upload is refused before anything
	// is stored instead of being saved without its private key.
	db := newPrivateKeyStorageDB(t)
	svc := NewCertificateService(t.TempDir(), db, nil)
	certPEM, keyPEM, err := generateSelfSignedCertPEM()
	require.NoError(t, err)

	info, err := svc.UploadCertificate("no-key", certPEM, keyPEM, "")

	require.ErrorIs(t, err, ErrPrivateKeyStorageUnavailable)
	assert.Nil(t, info)
	var count int64
	require.NoError(t, db.Model(&models.SSLCertificate{}).Count(&count).Error)
	assert.Zero(t, count, "a refused upload must store nothing")
}

func TestUploadCertificate_CertificateOnlyStillStoredWithoutEncryptionKey(t *testing.T) {
	// A certificate with no private key has nothing to encrypt, so the refusal
	// does not apply.
	db := newPrivateKeyStorageDB(t)
	svc := NewCertificateService(t.TempDir(), db, nil)
	certPEM, _, err := generateSelfSignedCertPEM()
	require.NoError(t, err)

	info, err := svc.UploadCertificate("cert-only", certPEM, "", "")

	require.NoError(t, err)
	require.NotNil(t, info)
	assert.False(t, info.HasKey)
}

func TestUploadCertificate_EncryptsWithNextKeyVersion(t *testing.T) {
	// E24: with a next key configured, the upload is written with the next
	// key's version and can be read back through the same cipher.
	db := newPrivateKeyStorageDB(t)
	current, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	next, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	rotation, err := crypto.NewRotationService(db, config.EncryptionKeys{Current: current, Next: next})
	require.NoError(t, err)

	svc := NewCertificateService(t.TempDir(), db, rotation)
	certPEM, keyPEM, err := generateSelfSignedCertPEM()
	require.NoError(t, err)

	info, err := svc.UploadCertificate("next-key", certPEM, keyPEM, "")
	require.NoError(t, err)
	require.True(t, info.HasKey)

	var stored models.SSLCertificate
	require.NoError(t, db.Where("uuid = ?", info.UUID).First(&stored).Error)
	assert.Equal(t, 2, stored.KeyVersion, "empty registry assigns next=2")

	plain, err := svc.GetDecryptedPrivateKey(&stored)
	require.NoError(t, err)
	assert.Equal(t, keyPEM, plain)
}
