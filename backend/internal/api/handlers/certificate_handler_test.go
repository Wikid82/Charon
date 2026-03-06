package handlers

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// mockAuthMiddleware adds a mock user to the context for testing
func mockAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user", map[string]any{"id": 1, "username": "testuser"})
		c.Next()
	}
}

func setupCertTestRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mockAuthMiddleware())

	svc := services.NewCertificateService("/tmp", db)
	h := NewCertificateHandler(svc, nil, nil)
	r.DELETE("/api/certificates/:id", h.Delete)
	return r
}

func TestDeleteCertificate_InUse(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// Migrate minimal models
	if err = db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Create certificate
	cert := models.SSLCertificate{UUID: "test-cert", Name: "example-cert", Provider: "custom", Domains: "example.com"}
	if err = db.Create(&cert).Error; err != nil {
		t.Fatalf("failed to create cert: %v", err)
	}

	// Create proxy host referencing the certificate
	ph := models.ProxyHost{UUID: "ph-1", Name: "ph", DomainNames: "example.com", ForwardHost: "localhost", ForwardPort: 8080, CertificateID: &cert.ID}
	if err := db.Create(&ph).Error; err != nil {
		t.Fatalf("failed to create proxy host: %v", err)
	}

	r := setupCertTestRouter(t, db)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+toStr(cert.ID), http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d, body=%s", w.Code, w.Body.String())
	}
}

func toStr(id uint) string {
	return fmt.Sprintf("%d", id)
}

// Test that deleting a certificate NOT in use creates a backup and deletes successfully
func TestDeleteCertificate_CreatesBackup(t *testing.T) {
	// Use a file-backed DB with busy timeout and single connection to avoid
	// lock contention with CertificateService background sync.
	dbPath := t.TempDir() + "/cert_create_backup.db"
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=1", dbPath)), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to access sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err = db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Create certificate
	cert := models.SSLCertificate{UUID: "test-cert-backup-success", Name: "deletable-cert", Provider: "custom", Domains: "delete.example.com"}
	if err = db.Create(&cert).Error; err != nil {
		t.Fatalf("failed to create cert: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db)

	// Mock BackupService
	backupCalled := false
	mockBackupService := &mockBackupService{
		createFunc: func() (string, error) {
			backupCalled = true
			return "backup-test.tar.gz", nil
		},
	}

	h := NewCertificateHandler(svc, mockBackupService, nil)
	r.DELETE("/api/certificates/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+toStr(cert.ID), http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body=%s", w.Code, w.Body.String())
	}

	if !backupCalled {
		t.Fatal("expected backup to be created before deletion")
	}

	// Verify certificate was deleted
	var found models.SSLCertificate
	err = db.First(&found, cert.ID).Error
	if err == nil {
		t.Fatal("expected certificate to be deleted")
	}
}

// Test that backup failure prevents deletion
func TestDeleteCertificate_BackupFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err = db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Create certificate
	cert := models.SSLCertificate{UUID: "test-cert-backup-fails", Name: "deletable-cert", Provider: "custom", Domains: "delete-fail.example.com"}
	if err = db.Create(&cert).Error; err != nil {
		t.Fatalf("failed to create cert: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db)

	// Mock BackupService that fails
	mockBackupService := &mockBackupService{
		createFunc: func() (string, error) {
			return "", fmt.Errorf("backup creation failed")
		},
	}

	h := NewCertificateHandler(svc, mockBackupService, nil)
	r.DELETE("/api/certificates/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+toStr(cert.ID), http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error, got %d", w.Code)
	}

	// Verify certificate was NOT deleted
	var found models.SSLCertificate
	err = db.First(&found, cert.ID).Error
	if err != nil {
		t.Fatal("expected certificate to still exist after backup failure")
	}
}

// Test that in-use check does not create a backup
func TestDeleteCertificate_InUse_NoBackup(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err = db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Create certificate
	cert := models.SSLCertificate{UUID: "test-cert-in-use-no-backup", Name: "in-use-cert", Provider: "custom", Domains: "inuse.example.com"}
	if err = db.Create(&cert).Error; err != nil {
		t.Fatalf("failed to create cert: %v", err)
	}

	// Create proxy host referencing the certificate
	ph := models.ProxyHost{UUID: "ph-no-backup-test", Name: "ph", DomainNames: "inuse.example.com", ForwardHost: "localhost", ForwardPort: 8080, CertificateID: &cert.ID}
	if err := db.Create(&ph).Error; err != nil {
		t.Fatalf("failed to create proxy host: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db)

	// Mock BackupService
	backupCalled := false
	mockBackupService := &mockBackupService{
		createFunc: func() (string, error) {
			backupCalled = true
			return "backup-test.tar.gz", nil
		},
	}

	h := NewCertificateHandler(svc, mockBackupService, nil)
	r.DELETE("/api/certificates/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+toStr(cert.ID), http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d, body=%s", w.Code, w.Body.String())
	}

	if backupCalled {
		t.Fatal("expected backup NOT to be created when certificate is in use")
	}
}

// Mock BackupService for testing
type mockBackupService struct {
	createFunc         func() (string, error)
	availableSpaceFunc func() (int64, error)
}

func (m *mockBackupService) CreateBackup() (string, error) {
	if m.createFunc != nil {
		return m.createFunc()
	}
	return "", fmt.Errorf("not implemented")
}

func (m *mockBackupService) ListBackups() ([]services.BackupFile, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockBackupService) DeleteBackup(filename string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockBackupService) GetBackupPath(filename string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *mockBackupService) RestoreBackup(filename string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockBackupService) GetAvailableSpace() (int64, error) {
	if m.availableSpaceFunc != nil {
		return m.availableSpaceFunc()
	}
	// Default: return 1GB available
	return 1024 * 1024 * 1024, nil
}

// Test List handler
func TestCertificateHandler_List(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err = db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db)
	h := NewCertificateHandler(svc, nil, nil)
	r.GET("/api/certificates", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/certificates", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body=%s", w.Code, w.Body.String())
	}
}

// Test Upload handler with missing name
func TestCertificateHandler_Upload_MissingName(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err = db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db)
	h := NewCertificateHandler(svc, nil, nil)
	r.POST("/api/certificates", h.Upload)

	// Empty body - no form fields
	req := httptest.NewRequest(http.MethodPost, "/api/certificates", strings.NewReader(""))
	req.Header.Set("Content-Type", "multipart/form-data")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", w.Code)
	}
}

// Test Upload handler missing certificate_file
func TestCertificateHandler_Upload_MissingCertFile(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err = db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db)
	h := NewCertificateHandler(svc, nil, nil)
	r.POST("/api/certificates", h.Upload)

	body := strings.NewReader("name=testcert")
	req := httptest.NewRequest(http.MethodPost, "/api/certificates", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "certificate_file") {
		t.Fatalf("expected error message about certificate_file, got: %s", w.Body.String())
	}
}

// Test Upload handler missing key_file
func TestCertificateHandler_Upload_MissingKeyFile(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err = db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db)
	h := NewCertificateHandler(svc, nil, nil)
	r.POST("/api/certificates", h.Upload)

	body := strings.NewReader("name=testcert")
	req := httptest.NewRequest(http.MethodPost, "/api/certificates", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", w.Code)
	}
}

func TestCertificateHandler_Upload_MissingKeyFile_MultipartWithCert(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err = db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db)
	h := NewCertificateHandler(svc, nil, nil)
	r.POST("/api/certificates", h.Upload)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("name", "testcert")
	part, createErr := writer.CreateFormFile("certificate_file", "cert.pem")
	if createErr != nil {
		t.Fatalf("failed to create form file: %v", createErr)
	}
	_, _ = part.Write([]byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/certificates", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "key_file") {
		t.Fatalf("expected error message about key_file, got: %s", w.Body.String())
	}
}

// Test Upload handler success path using a mock CertificateService
func TestCertificateHandler_Upload_Success(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err = db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mockAuthMiddleware())

	// Create a mock CertificateService that returns a created certificate
	// Create a temporary services.CertificateService with a temp dir and DB
	tmpDir := t.TempDir()
	svc := services.NewCertificateService(tmpDir, db)
	h := NewCertificateHandler(svc, nil, nil)
	r.POST("/api/certificates", h.Upload)

	// Prepare multipart form data
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("name", "uploaded-cert")
	certPEM, keyPEM, err := generateSelfSignedCertPEM()
	if err != nil {
		t.Fatalf("failed to generate cert: %v", err)
	}
	part, _ := writer.CreateFormFile("certificate_file", "cert.pem")
	_, _ = part.Write([]byte(certPEM))
	part2, _ := writer.CreateFormFile("key_file", "key.pem")
	_, _ = part2.Write([]byte(keyPEM))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/certificates", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d, body=%s", w.Code, w.Body.String())
	}
}

func generateSelfSignedCertPEM() (certPEM, keyPEM string, err error) {
	// generate RSA key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	// create a simple self-signed cert
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", "", err
	}
	certBuf := new(bytes.Buffer)
	_ = pem.Encode(certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBuf := new(bytes.Buffer)
	_ = pem.Encode(keyBuf, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	certPEM = certBuf.String()
	keyPEM = keyBuf.String()
	return certPEM, keyPEM, nil
}

// Note: mockCertificateService removed — helper tests now use real service instances or testify mocks inlined where required.

// Test Delete with invalid ID format
func TestDeleteCertificate_InvalidID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err = db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db)
	h := NewCertificateHandler(svc, nil, nil)
	r.DELETE("/api/certificates/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/invalid", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", w.Code)
	}
}

// Test Delete with ID = 0
func TestDeleteCertificate_ZeroID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err = db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db)
	h := NewCertificateHandler(svc, nil, nil)
	r.DELETE("/api/certificates/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/0", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", w.Code)
	}
}

// Test Delete with low disk space
func TestDeleteCertificate_LowDiskSpace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err = db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Create certificate
	cert := models.SSLCertificate{UUID: "test-cert-low-space", Name: "low-space-cert", Provider: "custom", Domains: "lowspace.example.com"}
	if err := db.Create(&cert).Error; err != nil {
		t.Fatalf("failed to create cert: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db)

	// Mock BackupService with low disk space
	mockBackupService := &mockBackupService{
		availableSpaceFunc: func() (int64, error) {
			return 50 * 1024 * 1024, nil // Only 50MB available
		},
	}

	h := NewCertificateHandler(svc, mockBackupService, nil)
	r.DELETE("/api/certificates/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+toStr(cert.ID), http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInsufficientStorage {
		t.Fatalf("expected 507 Insufficient Storage, got %d, body=%s", w.Code, w.Body.String())
	}
}

// Test Delete with disk space check failure (warning but continue)
func TestDeleteCertificate_DiskSpaceCheckError(t *testing.T) {
	// Use isolated file-backed DB to avoid lock flakiness from shared in-memory
	// connections and background sync.
	dbPath := t.TempDir() + "/cert_disk_space_error.db"
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=1", dbPath)), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to access sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err = db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Create certificate
	cert := models.SSLCertificate{UUID: "test-cert-space-err", Name: "space-err-cert", Provider: "custom", Domains: "spaceerr.example.com"}
	if err := db.Create(&cert).Error; err != nil {
		t.Fatalf("failed to create cert: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db)

	// Mock BackupService with space check error but backup succeeds
	mockBackupService := &mockBackupService{
		availableSpaceFunc: func() (int64, error) {
			return 0, fmt.Errorf("failed to check disk space")
		},
		createFunc: func() (string, error) {
			return "backup.tar.gz", nil
		},
	}

	h := NewCertificateHandler(svc, mockBackupService, nil)
	r.DELETE("/api/certificates/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+toStr(cert.ID), http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should succeed even if space check fails (with warning)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body=%s", w.Code, w.Body.String())
	}
}

// Test Delete when IsCertificateInUse fails
func TestDeleteCertificate_UsageCheckError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// Only migrate SSLCertificate, not ProxyHost - this will cause usage check to fail
	if err = db.AutoMigrate(&models.SSLCertificate{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Create certificate
	cert := models.SSLCertificate{UUID: "test-cert-usage-err", Name: "usage-err-cert", Provider: "custom", Domains: "usageerr.example.com"}
	if err := db.Create(&cert).Error; err != nil {
		t.Fatalf("failed to create cert: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db)
	h := NewCertificateHandler(svc, nil, nil)
	r.DELETE("/api/certificates/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+toStr(cert.ID), http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error, got %d, body=%s", w.Code, w.Body.String())
	}
}

// Test notification rate limiting
func TestDeleteCertificate_NotificationRateLimit(t *testing.T) {
	// Use unique file-based temp db to avoid shared memory locking issues
	tmpFile := t.TempDir() + "/rate_limit_test.db"
	db, err := gorm.Open(sqlite.Open(tmpFile), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err = db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.NotificationProvider{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Create two certificates
	cert1 := models.SSLCertificate{UUID: "test-cert-rate-1", Name: "rate-cert-1", Provider: "custom", Domains: "rate1.example.com"}
	cert2 := models.SSLCertificate{UUID: "test-cert-rate-2", Name: "rate-cert-2", Provider: "custom", Domains: "rate2.example.com"}
	if err := db.Create(&cert1).Error; err != nil {
		t.Fatalf("failed to create cert1: %v", err)
	}
	if err := db.Create(&cert2).Error; err != nil {
		t.Fatalf("failed to create cert2: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db)
	ns := services.NewNotificationService(db, nil)

	mockBackupService := &mockBackupService{
		createFunc: func() (string, error) {
			return "backup.tar.gz", nil
		},
	}

	h := NewCertificateHandler(svc, mockBackupService, ns)
	r.DELETE("/api/certificates/:id", h.Delete)

	// Delete first certificate
	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+toStr(cert1.ID), http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for first delete, got %d", w.Code)
	}

	// Delete second certificate immediately (different ID, so should not be rate limited)
	req = httptest.NewRequest(http.MethodDelete, "/api/certificates/"+toStr(cert2.ID), http.NoBody)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for second delete, got %d", w.Code)
	}
}
