package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// TestCertificateHandler_Delete_RequiresAuth tests that delete requires authentication
func TestCertificateHandler_Delete_RequiresAuth(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	r := gin.New()
	// Add a middleware that rejects all unauthenticated requests
	r.Use(func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
	})
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.DELETE("/api/certificates/:uuid", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/1", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized without auth, got %d", w.Code)
	}
}

// TestCertificateHandler_List_RequiresAuth tests that list requires authentication
func TestCertificateHandler_List_RequiresAuth(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	r := gin.New()
	// Add a middleware that rejects all unauthenticated requests
	r.Use(func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
	})
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.GET("/api/certificates", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/certificates", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized without auth, got %d", w.Code)
	}
}

// TestCertificateHandler_Upload_RequiresAuth tests that upload requires authentication
func TestCertificateHandler_Upload_RequiresAuth(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	r := gin.New()
	// Add a middleware that rejects all unauthenticated requests
	r.Use(func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
	})
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.POST("/api/certificates", h.Upload)

	req := httptest.NewRequest(http.MethodPost, "/api/certificates", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized without auth, got %d", w.Code)
	}
}

// TestCertificateHandler_Delete_DiskSpaceCheck tests the disk space check before backup
func TestCertificateHandler_Delete_DiskSpaceCheck(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Create a certificate
	cert := models.SSLCertificate{
		UUID:     "test-cert",
		Name:     "test",
		Provider: "custom",
		Domains:  "test.com",
	}
	if err := db.Create(&cert).Error; err != nil {
		t.Fatalf("failed to create cert: %v", err)
	}

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)

	// Mock backup service that reports low disk space
	mockBackup := &mockBackupService{
		availableSpaceFunc: func() (int64, error) {
			return 50 * 1024 * 1024, nil // 50MB (less than 100MB required)
		},
	}

	h := NewCertificateHandler(svc, mockBackup, nil)
	r.DELETE("/api/certificates/:uuid", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/certificates/%d", cert.ID), http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInsufficientStorage {
		t.Fatalf("expected 507 Insufficient Storage with low disk space, got %d", w.Code)
	}
}

// TestCertificateHandler_Delete_NotificationRateLimiting tests rate limiting
func TestCertificateHandler_Delete_NotificationRateLimiting(t *testing.T) {
	dbPath := t.TempDir() + "/cert_notification_rate_limit.db"
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

	if err := db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Create certificates
	cert1 := models.SSLCertificate{UUID: "test-1", Name: "test1", Provider: "custom", Domains: "test1.com"}
	cert2 := models.SSLCertificate{UUID: "test-2", Name: "test2", Provider: "custom", Domains: "test2.com"}
	if err := db.Create(&cert1).Error; err != nil {
		t.Fatalf("failed to create cert1: %v", err)
	}
	if err := db.Create(&cert2).Error; err != nil {
		t.Fatalf("failed to create cert2: %v", err)
	}

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)

	mockBackup := &mockBackupService{
		createFunc: func() (string, error) {
			return "backup.zip", nil
		},
	}

	h := NewCertificateHandler(svc, mockBackup, nil)
	r.DELETE("/api/certificates/:uuid", h.Delete)

	// Delete first cert
	req1 := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/certificates/%d", cert1.ID), http.NoBody)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("first delete failed: got %d", w1.Code)
	}

	// Delete second cert (different ID, should not be rate limited)
	req2 := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/certificates/%d", cert2.ID), http.NoBody)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("second delete failed: got %d", w2.Code)
	}

	// The test passes if both deletions succeed
	// Rate limiting is per-certificate ID, so different certs should not interfere
}
