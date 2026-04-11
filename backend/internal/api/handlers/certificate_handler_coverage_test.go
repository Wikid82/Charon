package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

func TestCertificateHandler_List_DBError(t *testing.T) {
	db := OpenTestDB(t)
	// Don't migrate to cause error

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.GET("/api/certificates", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/certificates", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCertificateHandler_Delete_InvalidID(t *testing.T) {
	db := OpenTestDBWithMigrations(t)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.DELETE("/api/certificates/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/invalid", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCertificateHandler_Delete_NotFound(t *testing.T) {
	db := OpenTestDBWithMigrations(t)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.DELETE("/api/certificates/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/9999", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCertificateHandler_Delete_NoBackupService(t *testing.T) {
	db := OpenTestDBWithMigrations(t)

	// Create certificate
	cert := models.SSLCertificate{UUID: "test-cert-no-backup", Name: "no-backup-cert", Provider: "custom", Domains: "nobackup.example.com"}
	db.Create(&cert)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)

	// No backup service
	h := NewCertificateHandler(svc, nil, nil)
	r.DELETE("/api/certificates/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+toStr(cert.ID), http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should still succeed without backup service
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCertificateHandler_Delete_CheckUsageDBError(t *testing.T) {
	db := OpenTestDB(t)
	// Only migrate SSLCertificate, not ProxyHost to cause error when checking usage
	_ = db.AutoMigrate(&models.SSLCertificate{})

	// Create certificate
	cert := models.SSLCertificate{UUID: "test-cert-db-err", Name: "db-error-cert", Provider: "custom", Domains: "dberr.example.com"}
	db.Create(&cert)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.DELETE("/api/certificates/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+toStr(cert.ID), http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCertificateHandler_List_WithCertificates(t *testing.T) {
	db := OpenTestDBWithMigrations(t)

	// Create certificates
	db.Create(&models.SSLCertificate{UUID: "cert-1", Name: "Cert 1", Provider: "custom", Domains: "one.example.com"})
	db.Create(&models.SSLCertificate{UUID: "cert-2", Name: "Cert 2", Provider: "custom", Domains: "two.example.com"})

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.GET("/api/certificates", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/certificates", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Cert 1")
	assert.Contains(t, w.Body.String(), "Cert 2")
}

func TestCertificateHandler_Delete_ZeroID(t *testing.T) {
	// Tests the ID=0 validation check (line 149-152 in certificate_handler.go)
	// DELETE /api/certificates/0 should return 400 Bad Request
	db := OpenTestDBWithMigrations(t)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.DELETE("/api/certificates/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/0", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid id")
}

func TestCertificateHandler_DBSetupOrdering(t *testing.T) {
	db := OpenTestDBWithMigrations(t)

	var certTableCount int64
	if err := db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name = ?", "ssl_certificates").Scan(&certTableCount).Error; err != nil {
		t.Fatalf("failed to verify ssl_certificates table: %v", err)
	}
	if certTableCount != 1 {
		t.Fatalf("expected ssl_certificates table to exist before service initialization")
	}

	var proxyHostsTableCount int64
	if err := db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name = ?", "proxy_hosts").Scan(&proxyHostsTableCount).Error; err != nil {
		t.Fatalf("failed to verify proxy_hosts table: %v", err)
	}
	if proxyHostsTableCount != 1 {
		t.Fatalf("expected proxy_hosts table to exist before service initialization")
	}

	r := gin.New()
	r.Use(mockAuthMiddleware())

	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.GET("/api/certificates", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/certificates", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
