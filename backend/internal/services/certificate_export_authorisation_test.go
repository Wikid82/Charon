package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func seedExportAuthorisationCert(t *testing.T, name string) (svc *CertificateService, certUUID string) {
	t.Helper()
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cs := newTestCertServiceWithEnc(t, tmpDir, db)
	certPEM, keyPEM := generateTestCertAndKey(t, name+".example.com", time.Now().Add(24*time.Hour))
	info, err := cs.UploadCertificate(name, string(certPEM), string(keyPEM), "")
	require.NoError(t, err)
	return cs, info.UUID
}

func TestExportCertificate_KeyExportWithoutAuthorisationRefused(t *testing.T) {
	cs, uuid := seedExportAuthorisationCert(t, "auth-refused")

	cases := []struct {
		name   string
		format string
		opts   ExportOptions
	}{
		{name: "pfx without include key", format: "pfx", opts: ExportOptions{PFXPassword: "pw"}},
		{name: "p12 without include key", format: "p12", opts: ExportOptions{PFXPassword: "pw"}},
		{name: "upper-case PFX without include key", format: "PFX", opts: ExportOptions{}},
		{name: "pem with include key", format: "pem", opts: ExportOptions{IncludeKey: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, filename, err := cs.ExportCertificate(uuid, tc.format, tc.opts)
			assert.ErrorIs(t, err, ErrKeyExportNotAuthorized)
			assert.Nil(t, data)
			assert.Empty(t, filename)
		})
	}
}

func TestExportCertificate_KeyExportWithAuthorisationSucceeds(t *testing.T) {
	cs, uuid := seedExportAuthorisationCert(t, "auth-granted")

	for _, format := range []string{"pfx", "p12"} {
		t.Run(format, func(t *testing.T) {
			data, filename, err := cs.ExportCertificate(uuid, format, ExportOptions{PFXPassword: "pw", KeyAuthorized: true})
			require.NoError(t, err)
			assert.NotEmpty(t, data)
			assert.Contains(t, filename, ".pfx")
		})
	}
}

func TestExportCertificate_CertificateOnlyNeedsNoAuthorisation(t *testing.T) {
	cs, uuid := seedExportAuthorisationCert(t, "auth-cert-only")

	pemData, _, err := cs.ExportCertificate(uuid, "pem", ExportOptions{})
	require.NoError(t, err)
	assert.NotContains(t, string(pemData), "PRIVATE KEY")

	derData, _, err := cs.ExportCertificate(uuid, "der", ExportOptions{})
	require.NoError(t, err)
	assert.NotEmpty(t, derData)
}

func TestFormatCarriesPrivateKey(t *testing.T) {
	assert.True(t, FormatCarriesPrivateKey("pfx"))
	assert.True(t, FormatCarriesPrivateKey("P12"))
	assert.False(t, FormatCarriesPrivateKey("pem"))
	assert.False(t, FormatCarriesPrivateKey("der"))
	assert.False(t, FormatCarriesPrivateKey(""))
}
