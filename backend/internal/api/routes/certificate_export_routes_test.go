package routes

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

const exportRouteTestKey = "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="

func exportTestConfig(login int) config.Config {
	cfg := withAuthBudget(login, 600)
	cfg.EncryptionKey = exportRouteTestKey
	cfg.CaddyConfigDir = ""
	return cfg
}

// seedExportCert stores a certificate with an encrypted private key using the
// same encryption key the route stack was configured with.
func (a *throttledApp) seedExportCert() string {
	a.t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(a.t, err)
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "export.example.com"},
		DNSNames:     []string{"export.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	require.NoError(a.t, err)
	keyDER, err := x509.MarshalECPrivateKey(priv)
	require.NoError(a.t, err)
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))

	enc, err := crypto.NewEncryptionService(exportRouteTestKey)
	require.NoError(a.t, err)
	info, err := services.NewCertificateService(a.t.TempDir(), a.db, enc).UploadCertificate("route-export", certPEM, keyPEM, "")
	require.NoError(a.t, err)
	return info.UUID
}

func exportPath(certUUID string) string { return "/api/v1/certificates/" + certUUID + "/export" }

func TestRegister_CertificateKeyExport_AdminCorrectPasswordSucceeds(t *testing.T) {
	app := newThrottledApp(t, exportTestConfig(50))
	certUUID := app.seedExportCert()
	_, token := app.createUser(models.RoleAdmin)

	w := app.do(http.MethodPost, exportPath(certUUID), token, `{"format":"pem","include_key":true,"password":"correct-password"}`, nil)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "BEGIN CERTIFICATE")
	assert.Contains(t, w.Body.String(), "PRIVATE KEY")
}

func TestRegister_CertificateKeyExport_WrongPasswordRejected(t *testing.T) {
	app := newThrottledApp(t, exportTestConfig(50))
	certUUID := app.seedExportCert()
	_, token := app.createUser(models.RoleAdmin)

	w := app.do(http.MethodPost, exportPath(certUUID), token, `{"format":"pem","include_key":true,"password":"wrong-password"}`, nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.NotContains(t, w.Body.String(), "PRIVATE KEY")
}

func TestRegister_CertificateKeyExport_MissingPasswordRejectedWithoutBudgetUse(t *testing.T) {
	app := newThrottledApp(t, exportTestConfig(1))
	certUUID := app.seedExportCert()
	_, token := app.createUser(models.RoleAdmin)

	for i := 0; i < 3; i++ {
		w := app.do(http.MethodPost, exportPath(certUUID), token, `{"format":"pem","include_key":true}`, nil)
		assert.Equal(t, http.StatusForbidden, w.Code, "attempt %d", i+1)
	}
	w := app.do(http.MethodPost, exportPath(certUUID), token, `{"format":"pem","include_key":true,"password":"correct-password"}`, nil)
	assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
}

func TestRegister_CertificateKeyExport_NonAdminRejected(t *testing.T) {
	app := newThrottledApp(t, exportTestConfig(50))
	certUUID := app.seedExportCert()

	for _, role := range []models.UserRole{models.RoleUser, models.RolePassthrough} {
		_, token := app.createUser(role)
		w := app.do(http.MethodPost, exportPath(certUUID), token, `{"format":"pem","include_key":true,"password":"correct-password"}`, nil)
		assert.Equal(t, http.StatusForbidden, w.Code, "role %s", role)
		assert.NotContains(t, w.Body.String(), "PRIVATE KEY", "role %s", role)
	}
}

func TestRegister_CertificateKeyExport_NoTokenUnauthorized(t *testing.T) {
	app := newThrottledApp(t, exportTestConfig(50))
	certUUID := app.seedExportCert()

	w := app.do(http.MethodPost, exportPath(certUUID), "", `{"format":"pem","include_key":true,"password":"correct-password"}`, nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRegister_CertificateKeyExport_EmergencyBypassRejected(t *testing.T) {
	const token = "test-token-that-meets-minimum-length-requirement-32-chars"
	t.Setenv("CHARON_EMERGENCY_TOKEN", token)
	app := newThrottledApp(t, exportTestConfig(50))
	certUUID := app.seedExportCert()

	send := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, exportPath(certUUID), strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:4000"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Emergency-Token", token)
		w := httptest.NewRecorder()
		app.router.ServeHTTP(w, req)
		return w
	}

	w := send(`{"format":"pem","include_key":true,"password":"correct-password"}`)
	assert.Equal(t, http.StatusForbidden, w.Code, "body: %s", w.Body.String())
	assert.NotContains(t, w.Body.String(), "PRIVATE KEY")

	w = send(`{"format":"pem","include_key":false}`)
	assert.Equal(t, http.StatusOK, w.Code, "certificate-only export stays available: %s", w.Body.String())
	assert.NotContains(t, w.Body.String(), "PRIVATE KEY")
}
