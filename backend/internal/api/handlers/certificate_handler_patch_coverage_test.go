package handlers

import (
	"bytes"
	"encoding/json"
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

// --- Export handler: re-auth and service error paths ---

func TestExport_IncludeKey_MissingPassword(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.User{}))
	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	h.SetDB(db)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.POST("/api/certificates/:uuid/export", h.Export)

	body := bytes.NewBufferString(`{"format":"pem","include_key":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/"+uuid.New().String()+"/export", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestExport_IncludeKey_NoUserContext(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.User{}))
	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	h.SetDB(db)

	r := gin.New() // no middleware — "user" key absent
	r.POST("/api/certificates/:uuid/export", h.Export)

	body := bytes.NewBufferString(`{"format":"pem","include_key":true,"password":"somepass"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/"+uuid.New().String()+"/export", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestExport_IncludeKey_InvalidClaimsType(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.User{}))
	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	h.SetDB(db)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user", "not-a-map"); c.Next() })
	r.POST("/api/certificates/:uuid/export", h.Export)

	body := bytes.NewBufferString(`{"format":"pem","include_key":true,"password":"somepass"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/"+uuid.New().String()+"/export", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestExport_IncludeKey_UserIDNotInClaims(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.User{}))
	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	h.SetDB(db)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user", map[string]any{}); c.Next() }) // no "id" key
	r.POST("/api/certificates/:uuid/export", h.Export)

	body := bytes.NewBufferString(`{"format":"pem","include_key":true,"password":"somepass"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/"+uuid.New().String()+"/export", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestExport_IncludeKey_UserNotFoundInDB(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.User{}))
	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	h.SetDB(db)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user", map[string]any{"id": float64(9999)}); c.Next() })
	r.POST("/api/certificates/:uuid/export", h.Export)

	body := bytes.NewBufferString(`{"format":"pem","include_key":true,"password":"somepass"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/"+uuid.New().String()+"/export", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestExport_IncludeKey_WrongPassword(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.User{}))

	u := &models.User{UUID: uuid.New().String(), Email: "export@example.com", Name: "Export User"}
	require.NoError(t, u.SetPassword("correctpass"))
	require.NoError(t, db.Create(u).Error)

	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	h.SetDB(db)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user", map[string]any{"id": float64(u.ID)}); c.Next() })
	r.POST("/api/certificates/:uuid/export", h.Export)

	body := bytes.NewBufferString(`{"format":"pem","include_key":true,"password":"wrongpass"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/"+uuid.New().String()+"/export", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestExport_CertNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))
	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.POST("/api/certificates/:uuid/export", h.Export)

	body := bytes.NewBufferString(`{"format":"pem"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/"+uuid.New().String()+"/export", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestExport_ServiceError(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	certUUID := uuid.New().String()
	cert := models.SSLCertificate{UUID: certUUID, Name: "test", Domains: "test.example.com", Provider: "custom"}
	require.NoError(t, db.Create(&cert).Error)

	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.POST("/api/certificates/:uuid/export", h.Export)

	body := bytes.NewBufferString(`{"format":"unsupported_xyz"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/"+certUUID+"/export", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Delete numeric ID paths ---

func TestDelete_NumericID_UsageCheckError(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{})) // no ProxyHost → IsCertificateInUse fails

	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.DELETE("/api/certificates/:uuid", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/1", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDelete_NumericID_LowDiskSpace(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cert := models.SSLCertificate{UUID: uuid.New().String(), Name: "low-space", Domains: "lowspace.example.com", Provider: "custom"}
	require.NoError(t, db.Create(&cert).Error)

	svc := services.NewCertificateService(tmpDir, db, nil)
	backup := &mockBackupService{
		availableSpaceFunc: func() (int64, error) { return 1024, nil }, // < 100 MB
		createFunc:         func() (string, error) { return "", nil },
	}
	h := NewCertificateHandler(svc, backup, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.DELETE("/api/certificates/:uuid", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/certificates/%d", cert.ID), http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInsufficientStorage, w.Code)
}

func TestDelete_NumericID_BackupError(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	cert := models.SSLCertificate{UUID: uuid.New().String(), Name: "backup-err", Domains: "backuperr.example.com", Provider: "custom"}
	require.NoError(t, db.Create(&cert).Error)

	svc := services.NewCertificateService(tmpDir, db, nil)
	backup := &mockBackupService{
		availableSpaceFunc: func() (int64, error) { return 1 << 30, nil }, // 1 GB — plenty
		createFunc:         func() (string, error) { return "", fmt.Errorf("backup create failed") },
	}
	h := NewCertificateHandler(svc, backup, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.DELETE("/api/certificates/:uuid", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/certificates/%d", cert.ID), http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDelete_NumericID_DeleteError(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{})) // no SSLCertificate → DeleteCertificateByID fails

	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.DELETE("/api/certificates/:uuid", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/42", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Delete UUID: internal usage-check error ---

func TestDelete_UUID_UsageCheckInternalError(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{})) // no ProxyHost → IsCertificateInUse fails

	certUUID := uuid.New().String()
	cert := models.SSLCertificate{UUID: certUUID, Name: "uuid-err", Domains: "uuiderr.example.com", Provider: "custom"}
	require.NoError(t, db.Create(&cert).Error)

	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.DELETE("/api/certificates/:uuid", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+certUUID, http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- sendDeleteNotification: rate limit ---

func TestSendDeleteNotification_RateLimit(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	ns := services.NewNotificationService(db, nil)
	svc := services.NewCertificateService(t.TempDir(), db, nil)
	h := NewCertificateHandler(svc, nil, ns)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/", http.NoBody)

	certRef := uuid.New().String()
	h.sendDeleteNotification(ctx, certRef) // first call — sets timestamp
	h.sendDeleteNotification(ctx, certRef) // second call — hits rate limit branch
}

// --- Update: empty UUID param (lines 207-209) ---

func TestUpdate_EmptyUUID(t *testing.T) {
	svc := services.NewCertificateService(t.TempDir(), nil, nil)
	h := NewCertificateHandler(svc, nil, nil)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/certificates/", bytes.NewBufferString(`{"name":"test"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	// No Params set — c.Param("uuid") returns ""
	h.Update(ctx)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Update: DB error (non-ErrCertNotFound) → lines 223-225 ---

func TestUpdate_DBError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	// Deliberately no AutoMigrate → ssl_certificates table absent → "no such table" error

	svc := services.NewCertificateService(t.TempDir(), db, nil)
	h := NewCertificateHandler(svc, nil, nil)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.PUT("/api/certificates/:uuid", h.Update)

	body, _ := json.Marshal(map[string]string{"name": "new-name"})
	req := httptest.NewRequest(http.MethodPut, "/api/certificates/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
