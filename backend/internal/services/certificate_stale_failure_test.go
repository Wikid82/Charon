package services

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// staleCertSetup stores an ACME certificate by scanning disk once, then removes its file so the next scan finds it stale.
func staleCertSetup(t *testing.T) (*CertificateService, *gorm.DB) {
	t.Helper()
	tmpDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))
	cs := newTestCertificateService(tmpDir, db)

	domain := "stale-failure.example.com"
	certDir := filepath.Join(tmpDir, "certificates", "acme-v02.api.letsencrypt.org-directory", domain)
	certPath := filepath.Join(certDir, domain+".crt")
	require.NoError(t, os.MkdirAll(certDir, 0o750))                                                              // #nosec G301 -- test directory
	require.NoError(t, os.WriteFile(certPath, generateTestCert(t, domain, time.Now().Add(24*time.Hour)), 0o600)) // #nosec G306 -- test fixture
	require.NoError(t, cs.SyncFromDisk())
	require.NoError(t, os.Remove(certPath))
	return cs, db
}

func TestSyncFromDisk_StaleRowAlreadyRemovedIsNotAnError(t *testing.T) {
	cs, db := staleCertSetup(t)
	// The first query of the scan reads the stale row, then another request removes it before the delete.
	const callback = "test:remove-stale-after-read"
	done := false
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if done || tx.Statement.Schema == nil || tx.Statement.Schema.Name != "SSLCertificate" {
			return
		}
		done = true
		_ = tx.Session(&gorm.Session{NewDB: true}).Exec("DELETE FROM ssl_certificates").Error
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callback) })

	require.NoError(t, cs.SyncFromDisk())
	require.True(t, done)
}

func TestSyncFromDisk_StaleRowDeleteFailureIsLoggedNotReturned(t *testing.T) {
	cs, db := staleCertSetup(t)
	refuseOn(t, db, "DELETE", "ssl_certificates")

	require.NoError(t, cs.SyncFromDisk(), "a failed stale cleanup is logged and does not fail the scan")
}
