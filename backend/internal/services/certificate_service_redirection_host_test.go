package services

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// --- IsCertificateInUse: RedirectionHost coverage ---
//
// Regression tests for the gap flagged in commit fad3d74b: IsCertificateInUse
// (and therefore DeleteCertificate's ErrCertInUse guard) only checked
// ProxyHost.certificate_id, so deleting a certificate referenced only by a
// RedirectionHost succeeded when it should have been blocked.

func TestIsCertificateInUse_RedirectionHost(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.RedirectionHost{}))

	cs := newTestCertificateService(tmpDir, db)

	cert := models.SSLCertificate{
		UUID: "redir-inuse-test", Name: "Redirection In Use Test", Provider: "custom",
		Domains: "redirect.example.com", CommonName: "redirect.example.com",
	}
	require.NoError(t, db.Create(&cert).Error)

	t.Run("not in use by proxy host or redirection host", func(t *testing.T) {
		inUse, err := cs.IsCertificateInUse(cert.ID)
		require.NoError(t, err)
		assert.False(t, inUse)
	})

	t.Run("in use by redirection host", func(t *testing.T) {
		rh := models.RedirectionHost{
			UUID: "rh-check", Name: "Check Redirect", DomainNames: "redirect.example.com",
			TargetURL: "https://target.example.com", StatusCode: 301, CertificateID: &cert.ID,
		}
		require.NoError(t, db.Create(&rh).Error)

		inUse, err := cs.IsCertificateInUse(cert.ID)
		require.NoError(t, err)
		assert.True(t, inUse)
	})
}

func TestIsCertificateInUse_ProxyHostStillWorksAlongsideRedirectionHostTable(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.RedirectionHost{}))

	cs := newTestCertificateService(tmpDir, db)

	cert := models.SSLCertificate{
		UUID: "proxy-inuse-test", Name: "Proxy In Use Test", Provider: "custom",
		Domains: "proxy.example.com", CommonName: "proxy.example.com",
	}
	require.NoError(t, db.Create(&cert).Error)

	ph := models.ProxyHost{
		UUID: "ph-check-2", Name: "Check Host 2", DomainNames: "proxy.example.com",
		ForwardHost: "localhost", ForwardPort: 8080, CertificateID: &cert.ID,
	}
	require.NoError(t, db.Create(&ph).Error)

	inUse, err := cs.IsCertificateInUse(cert.ID)
	require.NoError(t, err)
	assert.True(t, inUse)
}

// TestIsCertificateInUse_NoRedirectionHostTable confirms the HasTable guard
// prevents a "no such table: redirection_hosts" error for test DBs (and,
// historically, pre-migration production DBs) that never migrated
// RedirectionHost at all — the exact regression from the previous attempt at
// this fix.
func TestIsCertificateInUse_NoRedirectionHostTable(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	// Deliberately do NOT migrate RedirectionHost.
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	require.False(t, db.Migrator().HasTable(&models.RedirectionHost{}))

	cs := newTestCertificateService(tmpDir, db)

	cert := models.SSLCertificate{
		UUID: "no-redir-table-test", Name: "No Redirection Table Test", Provider: "custom",
		Domains: "noredirtable.example.com", CommonName: "noredirtable.example.com",
	}
	require.NoError(t, db.Create(&cert).Error)

	inUse, err := cs.IsCertificateInUse(cert.ID)
	require.NoError(t, err)
	assert.False(t, inUse)
}

// TestDeleteCertificate_BlockedByRedirectionHost confirms DeleteCertificate
// (the actual caller-facing behavior) returns ErrCertInUse when the
// certificate is only referenced by a RedirectionHost.
func TestDeleteCertificate_BlockedByRedirectionHost(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.RedirectionHost{}))

	cs := newTestCertificateService(tmpDir, db)

	cert := models.SSLCertificate{
		UUID: "redir-delete-test", Name: "Redirection Delete Test", Provider: "custom",
		Domains: "redirectdelete.example.com", CommonName: "redirectdelete.example.com",
	}
	require.NoError(t, db.Create(&cert).Error)

	rh := models.RedirectionHost{
		UUID: "rh-delete-check", Name: "Delete Check Redirect", DomainNames: "redirectdelete.example.com",
		TargetURL: "https://target.example.com", StatusCode: 301, CertificateID: &cert.ID,
	}
	require.NoError(t, db.Create(&rh).Error)

	err = cs.DeleteCertificate(cert.UUID)
	require.Error(t, err)
	assert.Equal(t, ErrCertInUse, err)
}
