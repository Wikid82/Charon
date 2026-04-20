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

// --- Export re-authentication flow ---

func setupExportRouter(t *testing.T, db *gorm.DB) (*gin.Engine, *CertificateHandler) {
	t.Helper()
	tmpDir := t.TempDir()
	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	h.SetDB(db)

	r := gin.New()
	return r, h
}

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

func TestCertificateHandler_Export_IncludeKeySuccess(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.User{}))

	user := models.User{UUID: "export-user-1", Email: "export@test.com", Name: "Exporter"}
	require.NoError(t, user.SetPassword("correctpassword"))
	require.NoError(t, db.Create(&user).Error)

	encSvc := newTestEncSvc(t)
	tmpDir := t.TempDir()
	svc := services.NewCertificateService(tmpDir, db, encSvc)
	h := NewCertificateHandler(svc, nil, nil)
	h.SetDB(db)

	certPEM, keyPEM, err := generateSelfSignedCertPEM()
	require.NoError(t, err)
	info, err := svc.UploadCertificate("export-cert", certPEM, keyPEM, "")
	require.NoError(t, err)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", map[string]any{"id": user.ID})
		c.Next()
	})
	r.POST("/api/certificates/:uuid/export", h.Export)

	payload, _ := json.Marshal(map[string]any{
		"format":      "pem",
		"include_key": true,
		"password":    "correctpassword",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/"+info.UUID+"/export", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Header().Get("Content-Disposition"), "export-cert.pem")
}

func TestCertificateHandler_Export_IncludeKeyWrongPassword(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.User{}))

	r, h := setupExportRouter(t, db)

	user := models.User{UUID: "wrong-pw-user", Email: "wrong@test.com", Name: "Wrong"}
	require.NoError(t, user.SetPassword("rightpass"))
	require.NoError(t, db.Create(&user).Error)

	r.Use(func(c *gin.Context) {
		c.Set("user", map[string]any{"id": user.ID})
		c.Next()
	})
	r.POST("/api/certificates/:uuid/export", h.Export)

	payload, _ := json.Marshal(map[string]any{
		"format":      "pem",
		"include_key": true,
		"password":    "wrongpass",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/fake-uuid/export", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "incorrect password")
}

func TestCertificateHandler_Export_NoUserInContext(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.User{}))

	r, h := setupExportRouter(t, db)
	r.POST("/api/certificates/:uuid/export", h.Export)

	payload, _ := json.Marshal(map[string]any{
		"format":      "pem",
		"include_key": true,
		"password":    "anything",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/fake-uuid/export", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "authentication required")
}

func TestCertificateHandler_Export_InvalidSession(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.User{}))

	r, h := setupExportRouter(t, db)
	r.Use(func(c *gin.Context) {
		c.Set("user", "not-a-map")
		c.Next()
	})
	r.POST("/api/certificates/:uuid/export", h.Export)

	payload, _ := json.Marshal(map[string]any{
		"format":      "pem",
		"include_key": true,
		"password":    "anything",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/fake-uuid/export", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "invalid session")
}

func TestCertificateHandler_Export_MissingUserID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.User{}))

	r, h := setupExportRouter(t, db)
	r.Use(func(c *gin.Context) {
		c.Set("user", map[string]any{"name": "test"})
		c.Next()
	})
	r.POST("/api/certificates/:uuid/export", h.Export)

	payload, _ := json.Marshal(map[string]any{
		"format":      "pem",
		"include_key": true,
		"password":    "anything",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/fake-uuid/export", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "invalid session")
}

func TestCertificateHandler_Export_UserNotFound(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.User{}))

	r, h := setupExportRouter(t, db)
	r.Use(func(c *gin.Context) {
		c.Set("user", map[string]any{"id": uint(9999)})
		c.Next()
	})
	r.POST("/api/certificates/:uuid/export", h.Export)

	payload, _ := json.Marshal(map[string]any{
		"format":      "pem",
		"include_key": true,
		"password":    "anything",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/fake-uuid/export", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "user not found")
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
