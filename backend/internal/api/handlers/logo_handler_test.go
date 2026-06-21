package handlers

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Wikid82/charon/backend/internal/models"
)

// minimalPNG is a 1×1 pixel valid PNG (67 bytes).
var minimalPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk length + type
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // width=1, height=1
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, // bitDepth=8, colorType=RGB
	0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, // IDAT chunk
	0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
	0x00, 0x00, 0x02, 0x00, 0x01, 0xE2, 0x21, 0xBC,
	0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
	0x44, 0xAE, 0x42, 0x60, 0x82, // IEND
}

// minimalSVG bytes — XML-based, DetectContentType returns "text/xml; charset=utf-8"
var minimalSVG = []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"></svg>`)

func setupLogoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:logo_test_%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	return db
}

func buildLogoRouter(db *gorm.DB, dataDir, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if role != "" {
			c.Set("role", role)
			c.Set("userID", uint(1))
		}
		c.Next()
	})
	h := NewLogoHandler(db, dataDir)
	r.POST("/settings/logo", h.UploadLogo)
	r.DELETE("/settings/logo", h.DeleteLogo)
	return r
}

func buildUploadRequest(t *testing.T, filename string, content []byte, contentType string) *http.Request {
	t.Helper()
	const fieldName = "logo"
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	var (
		partWriter io.Writer
		err        error
	)
	if contentType != "" {
		h := make(map[string][]string)
		h["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, filename)}
		h["Content-Type"] = []string{contentType}
		partWriter, err = writer.CreatePart(h)
	} else {
		partWriter, err = writer.CreateFormFile(fieldName, filename)
	}
	require.NoError(t, err)

	_, err = partWriter.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req, err := http.NewRequest(http.MethodPost, "/settings/logo", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// BL-01: Valid PNG upload writes file and returns HTTP 200 with url field.
func TestLogoHandler_UploadLogo_ValidPNG(t *testing.T) {
	db := setupLogoTestDB(t)
	dataDir := t.TempDir()
	r := buildLogoRouter(db, dataDir, "admin")

	req := buildUploadRequest(t,"mylogo.png", minimalPNG, "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"url"`)
	assert.Contains(t, w.Body.String(), `/uploads/logo.png`)

	// File must exist on disk
	assert.FileExists(t, filepath.Join(dataDir, "uploads", "logo.png"))

	// Settings must be persisted
	var urlSetting models.Setting
	require.NoError(t, db.Where("key = ?", "ui.logo_url").First(&urlSetting).Error)
	assert.Equal(t, "/uploads/logo.png", urlSetting.Value)

	var typeSetting models.Setting
	require.NoError(t, db.Where("key = ?", "ui.logo_type").First(&typeSetting).Error)
	assert.Equal(t, "upload", typeSetting.Value)
}

// BL-02: File exceeds 2MB → HTTP 413.
func TestLogoHandler_UploadLogo_FileTooLarge(t *testing.T) {
	db := setupLogoTestDB(t)
	dataDir := t.TempDir()
	r := buildLogoRouter(db, dataDir, "admin")

	// Generate > 2MB of data with PNG header so MIME detection doesn't trip first
	large := make([]byte, maxLogoSize+1024)
	copy(large, minimalPNG)

	req := buildUploadRequest(t,"big.png", large, "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

// BL-03: Non-image MIME (text/html) → HTTP 400.
func TestLogoHandler_UploadLogo_NonImageMIME(t *testing.T) {
	db := setupLogoTestDB(t)
	dataDir := t.TempDir()
	r := buildLogoRouter(db, dataDir, "admin")

	htmlBytes := []byte("<!DOCTYPE html><html><body>not an image</body></html>")
	req := buildUploadRequest(t,"evil.html", htmlBytes, "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// BL-04: SVG file → HTTP 400 (byte detection).
func TestLogoHandler_UploadLogo_SVGRejected(t *testing.T) {
	db := setupLogoTestDB(t)
	dataDir := t.TempDir()
	r := buildLogoRouter(db, dataDir, "admin")

	req := buildUploadRequest(t,"logo.svg", minimalSVG, "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// BL-04b: Spoofed Content-Type (SVG bytes with .png extension and image/png header) → HTTP 400.
func TestLogoHandler_UploadLogo_SpoofedContentType(t *testing.T) {
	db := setupLogoTestDB(t)
	dataDir := t.TempDir()
	r := buildLogoRouter(db, dataDir, "admin")

	// SVG bytes but declared as image/png
	req := buildUploadRequest(t,"totally-a-png.png", minimalSVG, "image/png")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Server-side detection must see SVG bytes and reject
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// BL-04c: No Content-Type header in multipart part, valid PNG bytes → HTTP 200.
func TestLogoHandler_UploadLogo_NoContentTypePNGBytes(t *testing.T) {
	db := setupLogoTestDB(t)
	dataDir := t.TempDir()
	r := buildLogoRouter(db, dataDir, "admin")

	// Use CreateFormFile (no explicit Content-Type on the part)
	req := buildUploadRequest(t,"logo.png", minimalPNG, "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// BL-05: DELETE logo clears setting and removes file → HTTP 200.
func TestLogoHandler_DeleteLogo(t *testing.T) {
	db := setupLogoTestDB(t)
	dataDir := t.TempDir()
	r := buildLogoRouter(db, dataDir, "admin")

	// First upload
	req := buildUploadRequest(t,"logo.png", minimalPNG, "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	logoPath := filepath.Join(dataDir, "uploads", "logo.png")
	require.FileExists(t, logoPath)

	// Now delete
	delReq, err := http.NewRequest(http.MethodDelete, "/settings/logo", http.NoBody)
	require.NoError(t, err)
	delW := httptest.NewRecorder()
	r.ServeHTTP(delW, delReq)

	assert.Equal(t, http.StatusOK, delW.Code)

	// File must be gone
	_, statErr := os.Stat(logoPath)
	assert.True(t, os.IsNotExist(statErr), "logo file should be removed after delete")

	// Settings must be gone
	var s models.Setting
	assert.Error(t, db.Where("key = ?", "ui.logo_url").First(&s).Error)
}

// BL-06: Unauthenticated upload → HTTP 401.
func TestLogoHandler_UploadLogo_Unauthenticated(t *testing.T) {
	db := setupLogoTestDB(t)
	dataDir := t.TempDir()
	// No role set (empty string = no context values)
	r := buildLogoRouter(db, dataDir, "")

	req := buildUploadRequest(t,"logo.png", minimalPNG, "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// BL-07: Non-admin upload → HTTP 403.
func TestLogoHandler_UploadLogo_NonAdmin(t *testing.T) {
	db := setupLogoTestDB(t)
	dataDir := t.TempDir()
	r := buildLogoRouter(db, dataDir, "user")

	req := buildUploadRequest(t,"logo.png", minimalPNG, "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestLogoHandler_UploadLogo_MissingField ensures missing "logo" field returns 400.
func TestLogoHandler_UploadLogo_MissingField(t *testing.T) {
	db := setupLogoTestDB(t)
	dataDir := t.TempDir()
	r := buildLogoRouter(db, dataDir, "admin")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	// Write a different field name
	w2, _ := writer.CreateFormFile("file", "logo.png")
	_, _ = w2.Write(minimalPNG)
	_ = writer.Close()

	req, _ := http.NewRequest(http.MethodPost, "/settings/logo", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAcceptedMIME tests the MIME acceptlist.
func TestAcceptedMIME(t *testing.T) {
	tests := []struct {
		mime    string
		wantExt string
		wantOK  bool
	}{
		{"image/png", ".png", true},
		{"image/jpeg", ".jpg", true},
		{"image/webp", ".webp", true},
		{"image/svg+xml", "", false},
		{"text/html; charset=utf-8", "", false},
		{"application/octet-stream", "", false},
		{"text/xml; charset=utf-8", "", false},
	}
	for _, tt := range tests {
		ext, ok := acceptedMIME(tt.mime)
		assert.Equal(t, tt.wantOK, ok, "mime=%s", tt.mime)
		assert.Equal(t, tt.wantExt, ext, "mime=%s", tt.mime)
	}
}
