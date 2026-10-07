package services

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCertDeleteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.RedirectionHost{}))
	return db
}

func TestDeleteCertificate_NotFound(t *testing.T) {
	db := setupCertDeleteTestDB(t)
	cs := newTestCertificateService(t.TempDir(), db)

	assert.ErrorIs(t, cs.DeleteCertificate("does-not-exist"), ErrCertNotFound)
}

func TestDeleteCertificate_RemovedConcurrentlyReportsNotFound(t *testing.T) {
	db := setupCertDeleteTestDB(t)
	cs := newTestCertificateService(t.TempDir(), db)

	cert := models.SSLCertificate{UUID: "gone-uuid", Name: "gone", Provider: "custom", Domains: "gone.example.com"}
	require.NoError(t, db.Create(&cert).Error)
	require.NoError(t, db.Delete(&cert).Error)

	assert.ErrorIs(t, cs.deleteIfUnreferenced(cert.ID), ErrCertNotFound)
}

func TestDeleteCertificate_BlocksReferenceAddedAfterLookup(t *testing.T) {
	tmpDir := t.TempDir()
	db := setupCertDeleteTestDB(t)
	cs := newTestCertificateService(tmpDir, db)

	domain := "race.example.com"
	certRoot := filepath.Join(tmpDir, "certificates")
	require.NoError(t, os.MkdirAll(certRoot, 0o750))
	certFile := filepath.Join(certRoot, domain+".crt")
	require.NoError(t, os.WriteFile(certFile, []byte("pem"), 0o600))

	cert := models.SSLCertificate{UUID: "race-uuid", Name: "race", Provider: "letsencrypt", Domains: domain}
	require.NoError(t, db.Create(&cert).Error)

	// Simulate a host being assigned after the certificate was looked up but
	// before the delete statement runs.
	injected := false
	require.NoError(t, db.Callback().Delete().Before("gorm:begin_transaction").Register("test:assign_host", func(tx *gorm.DB) {
		if injected {
			return
		}
		injected = true
		ph := models.ProxyHost{UUID: "race-ph", DomainNames: domain, ForwardHost: "127.0.0.1", ForwardPort: 80, CertificateID: &cert.ID}
		require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Create(&ph).Error)
	}))

	assert.ErrorIs(t, cs.DeleteCertificate(cert.UUID), ErrCertInUse)

	var count int64
	require.NoError(t, db.Model(&models.SSLCertificate{}).Where("id = ?", cert.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count, "certificate row must survive")
	assert.FileExists(t, certFile, "ACME files must not be removed when delete is blocked")
}

func TestDeleteCertificate_InUseKeepsACMEFiles(t *testing.T) {
	tmpDir := t.TempDir()
	db := setupCertDeleteTestDB(t)
	cs := newTestCertificateService(tmpDir, db)

	domain := "inuse-files.example.com"
	certRoot := filepath.Join(tmpDir, "certificates")
	require.NoError(t, os.MkdirAll(certRoot, 0o750))
	certFile := filepath.Join(certRoot, domain+".crt")
	require.NoError(t, os.WriteFile(certFile, []byte("pem"), 0o600))

	cert := models.SSLCertificate{UUID: "inuse-files-uuid", Name: "n", Provider: "letsencrypt", Domains: domain}
	require.NoError(t, db.Create(&cert).Error)
	require.NoError(t, db.Create(&models.RedirectionHost{
		UUID: "rh-files", DomainNames: domain, TargetURL: "https://t.example", StatusCode: 301, CertificateID: &cert.ID,
	}).Error)

	assert.ErrorIs(t, cs.DeleteCertificate(cert.UUID), ErrCertInUse)
	assert.FileExists(t, certFile)
}

func TestDeleteCertificate_WithoutRedirectionTable(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))
	cs := newTestCertificateService(t.TempDir(), db)

	cert := models.SSLCertificate{UUID: "no-redir", Name: "n", Provider: "custom", Domains: "x.example.com"}
	require.NoError(t, db.Create(&cert).Error)
	require.NoError(t, cs.DeleteCertificate(cert.UUID))
}

func TestDeleteIfUnreferenced_DBError(t *testing.T) {
	db := setupCertDeleteTestDB(t)
	cs := newTestCertificateService(t.TempDir(), db)
	require.NoError(t, db.Migrator().DropTable(&models.ProxyHost{}))

	err := cs.deleteIfUnreferenced(1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete certificate")
}

func TestDeleteIfUnreferenced_VerifyStateError(t *testing.T) {
	db := setupCertDeleteTestDB(t)
	cs := newTestCertificateService(t.TempDir(), db)

	cert := models.SSLCertificate{UUID: "verify-err", Name: "n", Provider: "custom", Domains: "v.example.com"}
	require.NoError(t, db.Create(&cert).Error)
	require.NoError(t, db.Create(&models.ProxyHost{
		UUID: "verify-ph", DomainNames: "v.example.com", ForwardHost: "127.0.0.1", ForwardPort: 80, CertificateID: &cert.ID,
	}).Error)

	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:fail_query", func(tx *gorm.DB) {
		_ = tx.AddError(fmt.Errorf("forced query failure"))
	}))

	err := cs.deleteIfUnreferenced(cert.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to verify certificate state")
}
