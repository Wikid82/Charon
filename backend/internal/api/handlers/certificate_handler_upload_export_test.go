package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// --- Upload: with chain file (covers chain_file multipart branch) ---

func TestCertificateHandler_Upload_WithChainFile(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	tmpDir := t.TempDir()
	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.POST("/api/certificates", h.Upload)

	certPEM, keyPEM, err := generateSelfSignedCertPEM()
	require.NoError(t, err)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("name", "chain-cert")
	part, _ := writer.CreateFormFile("certificate_file", "cert.pem")
	_, _ = part.Write([]byte(certPEM))
	part2, _ := writer.CreateFormFile("key_file", "key.pem")
	_, _ = part2.Write([]byte(keyPEM))
	part3, _ := writer.CreateFormFile("chain_file", "chain.pem")
	_, _ = part3.Write([]byte(certPEM))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/certificates", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())
}

// --- Upload: invalid cert data ---

func TestCertificateHandler_Upload_InvalidCertData(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	tmpDir := t.TempDir()
	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.POST("/api/certificates", h.Upload)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("name", "bad-cert")
	part, _ := writer.CreateFormFile("certificate_file", "cert.pem")
	_, _ = part.Write([]byte("not-a-cert"))
	part2, _ := writer.CreateFormFile("key_file", "key.pem")
	_, _ = part2.Write([]byte("not-a-key"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/certificates", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Export test helpers ---

func newTestEncSvc(t *testing.T) *crypto.EncryptionService {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	svc, err := crypto.NewEncryptionService(base64.StdEncoding.EncodeToString(key))
	require.NoError(t, err)
	return svc
}

// --- Validate handler with key and chain ---

func TestCertificateHandler_Validate_WithKeyAndChain(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	tmpDir := t.TempDir()
	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.POST("/api/certificates/validate", h.Validate)

	certPEM, keyPEM, err := generateSelfSignedCertPEM()
	require.NoError(t, err)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("certificate_file", "cert.pem")
	_, _ = part.Write([]byte(certPEM))
	part2, _ := writer.CreateFormFile("key_file", "key.pem")
	_, _ = part2.Write([]byte(keyPEM))
	part3, _ := writer.CreateFormFile("chain_file", "chain.pem")
	_, _ = part3.Write([]byte(certPEM))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/certificates/validate", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
}

func TestCertificateHandler_Validate_InvalidCert(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	tmpDir := t.TempDir()
	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.POST("/api/certificates/validate", h.Validate)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("certificate_file", "cert.pem")
	_, _ = part.Write([]byte("not-a-cert"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/certificates/validate", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	errList, ok := resp["errors"].([]any)
	assert.True(t, ok)
	assert.Greater(t, len(errList), 0, "expected validation errors in response")
}

func TestCertificateHandler_Validate_MissingCertFile(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	tmpDir := t.TempDir()
	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.POST("/api/certificates/validate", h.Validate)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("name", "test")
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/certificates/validate", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "certificate_file is required")
}
