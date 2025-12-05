package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

func TestCertificateHandler_List_DBError(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	// Don't migrate to cause error

	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := services.NewCertificateService("/tmp", db)
	h := NewCertificateHandler(svc, nil, nil)
	r.GET("/api/certificates", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/certificates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCertificateHandler_Delete_InvalidID(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := services.NewCertificateService("/tmp", db)
	h := NewCertificateHandler(svc, nil, nil)
	r.DELETE("/api/certificates/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCertificateHandler_Delete_NotFound(t *testing.T) {
	// Use unique in-memory DB per test to avoid SQLite locking issues in parallel test runs
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := services.NewCertificateService("/tmp", db)
	h := NewCertificateHandler(svc, nil, nil)
	r.DELETE("/api/certificates/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/9999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCertificateHandler_Delete_NoBackupService(t *testing.T) {
	// Use unique in-memory DB per test to avoid SQLite locking issues in parallel test runs
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{})

	// Create certificate
	cert := models.SSLCertificate{UUID: "test-cert-no-backup", Name: "no-backup-cert", Provider: "custom", Domains: "nobackup.example.com"}
	db.Create(&cert)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := services.NewCertificateService("/tmp", db)
	// No backup service
	h := NewCertificateHandler(svc, nil, nil)
	r.DELETE("/api/certificates/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+toStr(cert.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should still succeed without backup service
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCertificateHandler_Delete_CheckUsageDBError(t *testing.T) {
	// Use unique in-memory DB per test to avoid SQLite locking issues in parallel test runs
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	// Only migrate SSLCertificate, not ProxyHost to cause error when checking usage
	db.AutoMigrate(&models.SSLCertificate{})

	// Create certificate
	cert := models.SSLCertificate{UUID: "test-cert-db-err", Name: "db-error-cert", Provider: "custom", Domains: "dberr.example.com"}
	db.Create(&cert)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := services.NewCertificateService("/tmp", db)
	h := NewCertificateHandler(svc, nil, nil)
	r.DELETE("/api/certificates/:id", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/"+toStr(cert.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCertificateHandler_List_WithCertificates(t *testing.T) {
	// Use unique in-memory DB per test to avoid SQLite locking issues in parallel test runs
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{})

	// Create certificates
	db.Create(&models.SSLCertificate{UUID: "cert-1", Name: "Cert 1", Provider: "custom", Domains: "one.example.com"})
	db.Create(&models.SSLCertificate{UUID: "cert-2", Name: "Cert 2", Provider: "custom", Domains: "two.example.com"})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := services.NewCertificateService("/tmp", db)
	h := NewCertificateHandler(svc, nil, nil)
	r.GET("/api/certificates", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/certificates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Cert 1")
	assert.Contains(t, w.Body.String(), "Cert 2")
}
