package services

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupHostCertificateTestDB(t *testing.T) (*gorm.DB, *models.SSLCertificate) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.RedirectionHost{}, &models.SSLCertificate{}))
	cert := &models.SSLCertificate{UUID: "cert-1", Name: "cert", Provider: "custom", Domains: "a.example.com"}
	require.NoError(t, db.Create(cert).Error)
	return db, cert
}

func TestProxyHostService_CertificateMustExist(t *testing.T) {
	db, cert := setupHostCertificateTestDB(t)
	svc := NewProxyHostService(db)
	missing := cert.ID + 1000

	bad := &models.ProxyHost{UUID: "p-bad", DomainNames: "bad.example.com", ForwardHost: "127.0.0.1", ForwardPort: 80, CertificateID: &missing}
	assert.ErrorIs(t, svc.Create(bad), ErrCertNotFound)

	var count int64
	require.NoError(t, db.Model(&models.ProxyHost{}).Count(&count).Error)
	assert.Zero(t, count, "rejected create must not persist a row")

	good := &models.ProxyHost{UUID: "p-good", DomainNames: "good.example.com", ForwardHost: "127.0.0.1", ForwardPort: 80, CertificateID: &cert.ID}
	require.NoError(t, svc.Create(good))

	good.CertificateID = &missing
	assert.ErrorIs(t, svc.Update(good), ErrCertNotFound)
	var stored models.ProxyHost
	require.NoError(t, db.First(&stored, good.ID).Error)
	require.NotNil(t, stored.CertificateID)
	assert.Equal(t, cert.ID, *stored.CertificateID)

	good.CertificateID = &cert.ID
	require.NoError(t, svc.Update(good))
	good.CertificateID = nil
	require.NoError(t, svc.Update(good))
}

func TestRedirectionHostService_CertificateMustExist(t *testing.T) {
	db, cert := setupHostCertificateTestDB(t)
	svc := NewRedirectionHostService(db)
	missing := cert.ID + 1000

	bad := &models.RedirectionHost{UUID: "r-bad", DomainNames: "bad.example.com", TargetURL: "https://t.example", StatusCode: 301, CertificateID: &missing}
	assert.ErrorIs(t, svc.Create(bad), ErrCertNotFound)

	var count int64
	require.NoError(t, db.Model(&models.RedirectionHost{}).Count(&count).Error)
	assert.Zero(t, count, "rejected create must not persist a row")

	good := &models.RedirectionHost{UUID: "r-good", DomainNames: "good.example.com", TargetURL: "https://t.example", StatusCode: 301, CertificateID: &cert.ID}
	require.NoError(t, svc.Create(good))

	good.CertificateID = &missing
	assert.ErrorIs(t, svc.Update(good), ErrCertNotFound)
	var stored models.RedirectionHost
	require.NoError(t, db.First(&stored, good.ID).Error)
	require.NotNil(t, stored.CertificateID)
	assert.Equal(t, cert.ID, *stored.CertificateID)

	good.CertificateID = &cert.ID
	require.NoError(t, svc.Update(good))
}

func TestEnsureCertificateExists_DBError(t *testing.T) {
	db, cert := setupHostCertificateTestDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.SSLCertificate{}))

	err := ensureCertificateExists(db, &cert.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checking certificate existence")
	assert.NoError(t, ensureCertificateExists(db, nil))
}
