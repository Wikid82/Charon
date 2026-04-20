package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	r.DELETE("/api/certificates/:uuid", h.Delete)

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
	r.DELETE("/api/certificates/:uuid", h.Delete)

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
	r.DELETE("/api/certificates/:uuid", h.Delete)

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
	r.DELETE("/api/certificates/:uuid", h.Delete)

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
	r.DELETE("/api/certificates/:uuid", h.Delete)

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

// --- Get handler tests ---

func TestCertificateHandler_Get_Success(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	expiry := time.Now().Add(30 * 24 * time.Hour)
	db.Create(&models.SSLCertificate{UUID: "get-uuid-1", Name: "Get Test", Provider: "custom", Domains: "get.example.com", ExpiresAt: &expiry})

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.GET("/api/certificates/:uuid", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/certificates/get-uuid-1", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "get-uuid-1")
	assert.Contains(t, w.Body.String(), "Get Test")
}

func TestCertificateHandler_Get_NotFound(t *testing.T) {
	db := OpenTestDBWithMigrations(t)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.GET("/api/certificates/:uuid", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/certificates/nonexistent-uuid", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCertificateHandler_Get_EmptyUUID(t *testing.T) {
	db := OpenTestDBWithMigrations(t)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	// Route with empty uuid param won't match, test the handler directly with blank uuid
	r.GET("/api/certificates/", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/certificates/", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Empty uuid should return 400 or 404 depending on router handling
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusNotFound)
}

// --- SetDB test ---

func TestCertificateHandler_SetDB(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	assert.Nil(t, h.db)

	h.SetDB(db)
	assert.NotNil(t, h.db)
}

// --- Update handler tests ---

func TestCertificateHandler_Update_Success(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	expiry := time.Now().Add(30 * 24 * time.Hour)
	db.Create(&models.SSLCertificate{UUID: "upd-uuid-1", Name: "Old Name", Provider: "custom", Domains: "update.example.com", ExpiresAt: &expiry})

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.PUT("/api/certificates/:uuid", h.Update)

	body, _ := json.Marshal(map[string]string{"name": "New Name"})
	req := httptest.NewRequest(http.MethodPut, "/api/certificates/upd-uuid-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "New Name")
}

func TestCertificateHandler_Update_NotFound(t *testing.T) {
	db := OpenTestDBWithMigrations(t)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.PUT("/api/certificates/:uuid", h.Update)

	body, _ := json.Marshal(map[string]string{"name": "New Name"})
	req := httptest.NewRequest(http.MethodPut, "/api/certificates/nonexistent-uuid", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCertificateHandler_Update_BadJSON(t *testing.T) {
	db := OpenTestDBWithMigrations(t)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.PUT("/api/certificates/:uuid", h.Update)

	req := httptest.NewRequest(http.MethodPut, "/api/certificates/some-uuid", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCertificateHandler_Update_MissingName(t *testing.T) {
	db := OpenTestDBWithMigrations(t)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.PUT("/api/certificates/:uuid", h.Update)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPut, "/api/certificates/some-uuid", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Validate handler tests ---

func TestCertificateHandler_Validate_Success(t *testing.T) {
	db := OpenTestDBWithMigrations(t)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.POST("/api/certificates/validate", h.Validate)

	certPEM, keyPEM, err := generateSelfSignedCertPEM()
	require.NoError(t, err)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("certificate_file", "cert.pem")
	_, _ = part.Write([]byte(certPEM))
	part2, _ := writer.CreateFormFile("key_file", "key.pem")
	_, _ = part2.Write([]byte(keyPEM))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/certificates/validate", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "valid")
}

func TestCertificateHandler_Validate_NoCertFile(t *testing.T) {
	db := OpenTestDBWithMigrations(t)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.POST("/api/certificates/validate", h.Validate)

	req := httptest.NewRequest(http.MethodPost, "/api/certificates/validate", strings.NewReader(""))
	req.Header.Set("Content-Type", "multipart/form-data")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCertificateHandler_Validate_CertOnly(t *testing.T) {
	db := OpenTestDBWithMigrations(t)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.POST("/api/certificates/validate", h.Validate)

	certPEM, _, err := generateSelfSignedCertPEM()
	require.NoError(t, err)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("certificate_file", "cert.pem")
	_, _ = part.Write([]byte(certPEM))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/certificates/validate", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// --- Export handler tests ---

func TestCertificateHandler_Export_EmptyUUID(t *testing.T) {
	db := OpenTestDBWithMigrations(t)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.POST("/api/certificates/:uuid/export", h.Export)

	body, _ := json.Marshal(map[string]any{"format": "pem"})
	// Use a route that provides :uuid param as empty would not match normal routing
	req := httptest.NewRequest(http.MethodPost, "/api/certificates//export", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Router won't match empty uuid, so 404 or redirect
	assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusMovedPermanently || w.Code == http.StatusBadRequest)
}

func TestCertificateHandler_Export_BadJSON(t *testing.T) {
	db := OpenTestDBWithMigrations(t)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.POST("/api/certificates/:uuid/export", h.Export)

	req := httptest.NewRequest(http.MethodPost, "/api/certificates/some-uuid/export", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCertificateHandler_Export_NotFound(t *testing.T) {
	db := OpenTestDBWithMigrations(t)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.POST("/api/certificates/:uuid/export", h.Export)

	body, _ := json.Marshal(map[string]any{"format": "pem"})
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/nonexistent-uuid/export", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCertificateHandler_Export_PEMSuccess(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	certPEM, _, err := generateSelfSignedCertPEM()
	require.NoError(t, err)

	cert := models.SSLCertificate{UUID: "export-uuid-1", Name: "Export Test", Provider: "custom", Domains: "export.example.com", Certificate: certPEM}
	db.Create(&cert)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	tmpDir := t.TempDir()
	svc := services.NewCertificateService(tmpDir, db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.POST("/api/certificates/:uuid/export", h.Export)

	body, _ := json.Marshal(map[string]any{"format": "pem"})
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/export-uuid-1/export", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Disposition"), "Export Test.pem")
}

func TestCertificateHandler_Export_IncludeKeyNoPassword(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	cert := models.SSLCertificate{UUID: "export-uuid-2", Name: "Key Test", Provider: "custom", Domains: "key.example.com"}
	db.Create(&cert)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.POST("/api/certificates/:uuid/export", h.Export)

	body, _ := json.Marshal(map[string]any{"format": "pem", "include_key": true})
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/export-uuid-2/export", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "password required")
}

func TestCertificateHandler_Export_IncludeKeyNoDBSet(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	cert := models.SSLCertificate{UUID: "export-uuid-3", Name: "No DB Test", Provider: "custom", Domains: "nodb.example.com"}
	db.Create(&cert)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	// h.db is nil - not set via SetDB
	r.POST("/api/certificates/:uuid/export", h.Export)

	body, _ := json.Marshal(map[string]any{"format": "pem", "include_key": true, "password": "test123"})
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/export-uuid-3/export", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "authentication required")
}

// --- Delete via UUID path tests ---

func TestCertificateHandler_Delete_UUIDPath_NotFound(t *testing.T) {
	db := OpenTestDBWithMigrations(t)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.DELETE("/api/certificates/:uuid", h.Delete)

	// Valid UUID format but does not exist
	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/00000000-0000-0000-0000-000000000001", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCertificateHandler_Delete_UUIDPath_InUse(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	cert := models.SSLCertificate{UUID: "11111111-1111-1111-1111-111111111111", Name: "InUse UUID", Provider: "custom", Domains: "uuid-inuse.example.com"}
	db.Create(&cert)

	ph := models.ProxyHost{UUID: "ph-uuid-del", Name: "Proxy", DomainNames: "uuid-inuse.example.com", ForwardHost: "localhost", ForwardPort: 8080, CertificateID: &cert.ID}
	db.Create(&ph)

	r := gin.New()
	r.Use(mockAuthMiddleware())
	svc := services.NewCertificateService("/tmp", db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r.DELETE("/api/certificates/:uuid", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/certificates/11111111-1111-1111-1111-111111111111", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

// --- sanitizeCertRef tests ---

func TestSanitizeCertRef(t *testing.T) {
	assert.Equal(t, "00000000-0000-0000-0000-000000000001", sanitizeCertRef("00000000-0000-0000-0000-000000000001"))
	assert.Equal(t, "123", sanitizeCertRef("123"))
	assert.Equal(t, "[invalid-ref]", sanitizeCertRef("not-valid"))
	assert.Equal(t, "0", sanitizeCertRef("0"))
}
