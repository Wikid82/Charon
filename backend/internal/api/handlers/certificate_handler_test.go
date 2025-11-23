package handlers

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func generateTestCert(t *testing.T, domain string) []byte {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: domain,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(24 * time.Hour),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
}

func TestCertificateHandler_List(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()
	caddyDir := filepath.Join(tmpDir, "caddy", "certificates", "acme-v02.api.letsencrypt.org-directory")
	err := os.MkdirAll(caddyDir, 0755)
	require.NoError(t, err)

	// Setup in-memory DB
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

	service := services.NewCertificateService(tmpDir, db)
	ns := services.NewNotificationService(db)
	handler := NewCertificateHandler(service, ns)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/certificates", handler.List)

	req, _ := http.NewRequest("GET", "/certificates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var certs []services.CertificateInfo
	err = json.Unmarshal(w.Body.Bytes(), &certs)
	assert.NoError(t, err)
	assert.Empty(t, certs)
}

func TestCertificateHandler_Upload(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

	service := services.NewCertificateService(tmpDir, db)
	ns := services.NewNotificationService(db)
	handler := NewCertificateHandler(service, ns)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/certificates", handler.Upload)

	// Prepare Multipart Request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("name", "Test Cert")

	certPEM := generateTestCert(t, "test.com")
	part, _ := writer.CreateFormFile("certificate_file", "cert.pem")
	part.Write(certPEM)

	part, _ = writer.CreateFormFile("key_file", "key.pem")
	part.Write([]byte("FAKE KEY")) // Service doesn't validate key structure strictly yet, just PEM decoding?
	// Actually service does: block, _ := pem.Decode([]byte(certPEM)) for cert.
	// It doesn't seem to validate keyPEM in UploadCertificate, just stores it.

	writer.Close()

	req, _ := http.NewRequest("POST", "/certificates", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var cert models.SSLCertificate
	err = json.Unmarshal(w.Body.Bytes(), &cert)
	assert.NoError(t, err)
	assert.Equal(t, "Test Cert", cert.Name)
}

func TestCertificateHandler_Delete(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}))

	// Seed a cert
	cert := models.SSLCertificate{
		UUID: "test-uuid",
		Name: "To Delete",
	}
	err = db.Create(&cert).Error
	require.NoError(t, err)
	require.NotZero(t, cert.ID)

	service := services.NewCertificateService(tmpDir, db)
	ns := services.NewNotificationService(db)
	handler := NewCertificateHandler(service, ns)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.DELETE("/certificates/:id", handler.Delete)

	req, _ := http.NewRequest("DELETE", "/certificates/"+strconv.Itoa(int(cert.ID)), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify deletion
	var deletedCert models.SSLCertificate
	err = db.First(&deletedCert, cert.ID).Error
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}
