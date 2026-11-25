package services

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// legacyKeyFixture is a file-backed SQLite database with a single-connection
// pool, so the pinned connection used by the migration is the only one and
// PRAGMA settings can be observed on it. It also captures the shared logger.
type legacyKeyFixture struct {
	t    *testing.T
	db   *gorm.DB
	path string
	logs *bytes.Buffer
}

// newLegacyKeyFixture builds the schema. withLegacyColumn adds the plaintext
// private_key column that pre-encryption installs have and AutoMigrate never
// creates.
func newLegacyKeyFixture(t *testing.T, withLegacyColumn bool) *legacyKeyFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "certs.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.Exec("PRAGMA journal_mode=WAL").Error)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.EncryptionKeyVersion{}))
	if withLegacyColumn {
		require.NoError(t, db.Exec("ALTER TABLE ssl_certificates ADD COLUMN private_key text NOT NULL DEFAULT ''").Error)
	}

	var buf bytes.Buffer
	logger.Init(true, &buf)
	t.Cleanup(func() { logger.Init(false, os.Stdout) })

	return &legacyKeyFixture{t: t, db: db, path: path, logs: &buf}
}

// insertLegacyCert stores a certificate row whose plaintext key sits in the
// legacy column and returns its id.
func (f *legacyKeyFixture) insertLegacyCert(name, keyPEM string) uint {
	f.t.Helper()
	cert := models.SSLCertificate{UUID: uuid.New().String(), Name: name, Provider: "custom", Certificate: "cert"}
	require.NoError(f.t, f.db.Create(&cert).Error)
	require.NoError(f.t, f.db.Exec("UPDATE ssl_certificates SET private_key = ? WHERE id = ?", keyPEM, cert.ID).Error)
	return cert.ID
}

// storedKey is the persisted state of one certificate's key columns.
type storedKey struct {
	PrivateKey    string
	PrivateKeyEnc string
	KeyVersion    int
}

func (f *legacyKeyFixture) readKey(id uint) storedKey {
	f.t.Helper()
	var row storedKey
	require.NoError(f.t, f.db.Raw(
		"SELECT private_key, private_key_enc, key_version FROM ssl_certificates WHERE id = ?", id,
	).Scan(&row).Error)
	return row
}

func (f *legacyKeyFixture) secureDelete() int {
	f.t.Helper()
	var mode int
	require.NoError(f.t, f.db.Raw("PRAGMA secure_delete").Scan(&mode).Error)
	return mode
}

// flakyCipher wraps a real cipher and fails the failOn-th encrypt call (1-based).
type flakyCipher struct {
	inner  crypto.VersionedCipher
	failOn int
	calls  int
}

func (f *flakyCipher) EncryptWithCurrentKey(plaintext []byte) (ciphertext string, keyVersion int, err error) {
	f.calls++
	if f.calls == f.failOn {
		return "", 0, errors.New("injected encrypt failure")
	}
	return f.inner.EncryptWithCurrentKey(plaintext)
}

func (f *flakyCipher) DecryptWithVersion(ciphertext string, keyVersion int) ([]byte, error) {
	return f.inner.DecryptWithVersion(ciphertext, keyVersion)
}

// rotationCipher returns a real rotation service over the fixture database
// that uses a freshly generated current key.
func (f *legacyKeyFixture) rotationCipher() *crypto.RotationService {
	f.t.Helper()
	key, err := crypto.GenerateNewKey()
	require.NoError(f.t, err)
	rs, err := crypto.NewRotationService(f.db, config.EncryptionKeys{Current: key})
	require.NoError(f.t, err)
	return rs
}

// testPrivateKeyPEM returns a distinct PEM-encoded RSA private key so a test
// can search the database file for the plaintext.
func testPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der := x509.MarshalPKCS1PrivateKey(key)
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}))
}

func TestMigratePrivateKeys_FreshInstallStaysSilent(t *testing.T) {
	// A fresh install never had the legacy column, so the migration must not
	// query it, must not warn, and must not log "no such column".
	for _, withKey := range []bool{true, false} {
		name := "without key"
		if withKey {
			name = "with key"
		}
		t.Run(name, func(t *testing.T) {
			f := newLegacyKeyFixture(t, false)
			var cipher crypto.VersionedCipher
			if withKey {
				cipher = f.rotationCipher()
			}
			svc := NewCertificateService(t.TempDir(), f.db, cipher)

			require.NoError(t, svc.MigratePrivateKeys())

			logs := f.logs.String()
			assert.NotContains(t, logs, "level=warning")
			assert.NotContains(t, logs, "level=error")
			assert.NotContains(t, logs, "no such column")
		})
	}
}

func TestMigratePrivateKeys_ColumnWithoutLegacyRowsIsSilent(t *testing.T) {
	f := newLegacyKeyFixture(t, true)
	svc := NewCertificateService(t.TempDir(), f.db, f.rotationCipher())

	require.NoError(t, svc.MigratePrivateKeys())

	assert.NotContains(t, f.logs.String(), "level=warning")
	assert.NotContains(t, f.logs.String(), "level=error")
}

func TestMigratePrivateKeys_WarnsOnEveryStartWithoutKey(t *testing.T) {
	f := newLegacyKeyFixture(t, true)
	keyA := f.insertLegacyCert("first", testPrivateKeyPEM(t))
	keyB := f.insertLegacyCert("second", testPrivateKeyPEM(t))
	svc := NewCertificateService(t.TempDir(), f.db, nil)

	// Two simulated starts: each one must warn with the count.
	require.NoError(t, svc.MigratePrivateKeys())
	require.NoError(t, svc.MigratePrivateKeys())

	const warning = "2 certificate private keys are stored without encryption"
	assert.Equal(t, 2, strings.Count(f.logs.String(), warning))
	assert.Contains(t, f.logs.String(), "Set CHARON_ENCRYPTION_KEY and restart to encrypt them.")

	// Nothing is changed while no key exists.
	for _, id := range []uint{keyA, keyB} {
		row := f.readKey(id)
		assert.NotEmpty(t, row.PrivateKey)
		assert.Empty(t, row.PrivateKeyEnc)
	}
}

func TestMigratePrivateKeys_EncryptsLegacyKeysWithRealVersion(t *testing.T) {
	f := newLegacyKeyFixture(t, true)
	keyPEM := testPrivateKeyPEM(t)
	id := f.insertLegacyCert("legacy", keyPEM)

	// The first key is registered as version 1; the second key is unknown to
	// the registry, so it is assigned the next version (2).
	first, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	_, err = crypto.NewRotationService(f.db, config.EncryptionKeys{Current: first})
	require.NoError(t, err)
	second, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	cipher, err := crypto.NewRotationService(f.db, config.EncryptionKeys{Current: second})
	require.NoError(t, err)

	svc := NewCertificateService(t.TempDir(), f.db, cipher)
	require.NoError(t, svc.MigratePrivateKeys())

	row := f.readKey(id)
	assert.Empty(t, row.PrivateKey, "plaintext must be cleared")
	assert.NotEmpty(t, row.PrivateKeyEnc)
	assert.Equal(t, 2, row.KeyVersion, "row must carry the version the cipher reported")

	plain, err := cipher.DecryptWithVersion(row.PrivateKeyEnc, row.KeyVersion)
	require.NoError(t, err)
	assert.Equal(t, keyPEM, string(plain))

	// A second run finds nothing to do.
	require.NoError(t, svc.MigratePrivateKeys())
	assert.Equal(t, row, f.readKey(id))
}

func TestMigratePrivateKeys_KeepsSecureDeleteAndScrubsPlaintext(t *testing.T) {
	// E21: the previous secure_delete value is restored, the WAL is checkpointed
	// (no WAL bytes remain) and the plaintext is gone from the database file.
	// SQLite reads the FAST mode back as 1, so only the readable modes 0 and 1
	// are asserted as exact round trips.
	for _, previous := range []int{0, 1} {
		t.Run("previous mode", func(t *testing.T) {
			f := newLegacyKeyFixture(t, true)
			keyPEM := testPrivateKeyPEM(t)
			f.insertLegacyCert("legacy", keyPEM)
			require.NoError(t, f.db.Exec(fmt.Sprintf("PRAGMA secure_delete = %d", previous)).Error)
			require.Equal(t, previous, f.secureDelete())

			svc := NewCertificateService(t.TempDir(), f.db, f.rotationCipher())
			require.NoError(t, svc.MigratePrivateKeys())

			assert.Equal(t, previous, f.secureDelete(), "previous secure_delete must be restored")
			if info, err := os.Stat(f.path + "-wal"); err == nil {
				assert.Zero(t, info.Size(), "WAL must be checkpointed and truncated")
			}
			data, err := os.ReadFile(f.path)
			require.NoError(t, err)
			assert.False(t, bytes.Contains(data, []byte(keyPEM)), "plaintext key must not remain in the database file")
		})
	}
}

func TestMigratePrivateKeys_RestoresSecureDeleteWhenEncryptionFails(t *testing.T) {
	f := newLegacyKeyFixture(t, true)
	id := f.insertLegacyCert("legacy", testPrivateKeyPEM(t))
	require.NoError(t, f.db.Exec("PRAGMA secure_delete = 1").Error)

	cipher := &flakyCipher{inner: f.rotationCipher(), failOn: 1}
	svc := NewCertificateService(t.TempDir(), f.db, cipher)

	require.Error(t, svc.MigratePrivateKeys())
	assert.Equal(t, 1, f.secureDelete(), "secure_delete must be restored on error")
	row := f.readKey(id)
	assert.NotEmpty(t, row.PrivateKey, "failed migration must leave the plaintext in place")
	assert.Empty(t, row.PrivateKeyEnc)
}

func TestMigratePrivateKeys_RollsBackWholeBatchAndRetriesNextStart(t *testing.T) {
	// E22: an encrypt failure on the second row rolls back the first row too;
	// the next start migrates both.
	f := newLegacyKeyFixture(t, true)
	idA := f.insertLegacyCert("first", testPrivateKeyPEM(t))
	idB := f.insertLegacyCert("second", testPrivateKeyPEM(t))
	rotation := f.rotationCipher()

	failing := &flakyCipher{inner: rotation, failOn: 2}
	require.Error(t, NewCertificateService(t.TempDir(), f.db, failing).MigratePrivateKeys())
	for _, id := range []uint{idA, idB} {
		row := f.readKey(id)
		assert.NotEmpty(t, row.PrivateKey, "rolled-back row keeps its plaintext")
		assert.Empty(t, row.PrivateKeyEnc, "rolled-back row keeps no ciphertext")
	}

	require.NoError(t, NewCertificateService(t.TempDir(), f.db, rotation).MigratePrivateKeys())
	for _, id := range []uint{idA, idB} {
		row := f.readKey(id)
		assert.Empty(t, row.PrivateKey)
		assert.NotEmpty(t, row.PrivateKeyEnc)
	}
}
