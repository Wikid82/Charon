package handlers

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// --- Delete UUID path with backup service ---

func TestDelete_UUID_WithBackup_Success(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	certUUID := uuid.New().String()
	db.Create(&models.SSLCertificate{UUID: certUUID, Name: "backup-uuid", Provider: "custom", Domains: "backup.test"})

	svc := services.NewCertificateService(tmpDir, db, nil)
	mock := &mockBackupService{
		createFunc:         func() (string, error) { return "/tmp/backup.tar.gz", nil },
		availableSpaceFunc: func() (int64, error) { return 1024 * 1024 * 1024, nil },
	}
	h := NewCertificateHandler(svc, mock, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.DELETE("/api/certificates/:uuid", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+certUUID, http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDelete_UUID_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.DELETE("/api/certificates/:uuid", h.Delete)

	nonExistentUUID := uuid.New().String()
	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+nonExistentUUID, http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDelete_UUID_InUse(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	certUUID := uuid.New().String()
	cert := models.SSLCertificate{UUID: certUUID, Name: "inuse-uuid", Provider: "custom", Domains: "inuse.test"}
	db.Create(&cert)
	db.Create(&models.ProxyHost{UUID: "ph-uuid-inuse", Name: "ph", DomainNames: "inuse.test", ForwardHost: "localhost", ForwardPort: 8080, CertificateID: &cert.ID})

	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.DELETE("/api/certificates/:uuid", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+certUUID, http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestDelete_UUID_BackupLowSpace(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	certUUID := uuid.New().String()
	db.Create(&models.SSLCertificate{UUID: certUUID, Name: "low-space", Provider: "custom", Domains: "lowspace.test"})

	svc := services.NewCertificateService(tmpDir, db, nil)
	mock := &mockBackupService{
		availableSpaceFunc: func() (int64, error) { return 1024, nil }, // 1KB - too low
	}
	h := NewCertificateHandler(svc, mock, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.DELETE("/api/certificates/:uuid", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+certUUID, http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInsufficientStorage, w.Code)
}

func TestDelete_UUID_BackupSpaceCheckError(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	certUUID := uuid.New().String()
	db.Create(&models.SSLCertificate{UUID: certUUID, Name: "space-err", Provider: "custom", Domains: "spaceerr.test"})

	svc := services.NewCertificateService(tmpDir, db, nil)
	mock := &mockBackupService{
		availableSpaceFunc: func() (int64, error) { return 0, fmt.Errorf("disk error") },
		createFunc:         func() (string, error) { return "/tmp/backup.tar.gz", nil },
	}
	h := NewCertificateHandler(svc, mock, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.DELETE("/api/certificates/:uuid", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+certUUID, http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Space check error → proceeds with backup → succeeds
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDelete_UUID_BackupCreateError(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	certUUID := uuid.New().String()
	db.Create(&models.SSLCertificate{UUID: certUUID, Name: "backup-fail", Provider: "custom", Domains: "backupfail.test"})

	svc := services.NewCertificateService(tmpDir, db, nil)
	mock := &mockBackupService{
		availableSpaceFunc: func() (int64, error) { return 1024 * 1024 * 1024, nil },
		createFunc:         func() (string, error) { return "", fmt.Errorf("backup creation failed") },
	}
	h := NewCertificateHandler(svc, mock, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.DELETE("/api/certificates/:uuid", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+certUUID, http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Delete UUID with notification service ---

func TestDelete_UUID_WithNotification(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.Setting{}, &models.Notification{}, &models.NotificationProvider{}))

	certUUID := uuid.New().String()
	db.Create(&models.SSLCertificate{UUID: certUUID, Name: "notify-cert", Provider: "custom", Domains: "notify.test"})

	svc := services.NewCertificateService(tmpDir, db, nil)
	notifSvc := services.NewNotificationService(db, nil)
	h := NewCertificateHandler(svc, nil, notifSvc)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.DELETE("/api/certificates/:uuid", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+certUUID, http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- Validate handler ---

func TestValidate_Success(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	certPEM, _, err := generateSelfSignedCertPEM()
	require.NoError(t, err)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("certificate_file", "cert.pem")
	require.NoError(t, err)
	_, err = part.Write([]byte(certPEM))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.POST("/api/certificates/validate", h.Validate)

	req := httptest.NewRequest(http.MethodPost, "/api/certificates/validate", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestValidate_InvalidCert(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("certificate_file", "cert.pem")
	require.NoError(t, err)
	_, err = part.Write([]byte("not a certificate"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.POST("/api/certificates/validate", h.Validate)

	req := httptest.NewRequest(http.MethodPost, "/api/certificates/validate", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "unrecognized certificate format")
}

func TestValidate_NoCertFile(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.POST("/api/certificates/validate", h.Validate)

	req := httptest.NewRequest(http.MethodPost, "/api/certificates/validate", http.NoBody)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestValidate_WithKeyAndChain(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	certPEM, keyPEM, err := generateSelfSignedCertPEM()
	require.NoError(t, err)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	certPart, err := writer.CreateFormFile("certificate_file", "cert.pem")
	require.NoError(t, err)
	_, err = certPart.Write([]byte(certPEM))
	require.NoError(t, err)

	keyPart, err := writer.CreateFormFile("key_file", "key.pem")
	require.NoError(t, err)
	_, err = keyPart.Write([]byte(keyPEM))
	require.NoError(t, err)

	chainPart, err := writer.CreateFormFile("chain_file", "chain.pem")
	require.NoError(t, err)
	_, err = chainPart.Write([]byte(certPEM)) // self-signed chain
	require.NoError(t, err)

	require.NoError(t, writer.Close())

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.POST("/api/certificates/validate", h.Validate)

	req := httptest.NewRequest(http.MethodPost, "/api/certificates/validate", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- Get handler DB error (non-NotFound) ---

func TestGet_DBError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	// Deliberately don't migrate - any query will fail with "no such table"

	svc := services.NewCertificateService(t.TempDir(), db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.GET("/api/certificates/:uuid", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/certificates/"+uuid.New().String(), http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Should be 500 since the table doesn't exist
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
