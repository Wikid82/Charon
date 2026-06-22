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

// minimalWebP is a 1×1 pixel valid WebP (RIFF header).
var minimalWebP = []byte{
	0x52, 0x49, 0x46, 0x46, // "RIFF"
	0x24, 0x00, 0x00, 0x00, // file size (little-endian)
	0x57, 0x45, 0x42, 0x50, // "WEBP"
	0x56, 0x50, 0x38, 0x4C, // "VP8L"
	0x18, 0x00, 0x00, 0x00, // chunk size
	0x2F, 0x00, 0x00, 0x00, // VP8L signature
	0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
}

func setupBannerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:banner_test_%s?mode=memory&cache=shared", t.Name())
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

func buildBannerRouter(db *gorm.DB, dataDir, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if role != "" {
			c.Set("role", role)
			c.Set("userID", uint(1))
		}
		c.Next()
	})
	h := NewBannerHandler(db, dataDir)
	r.POST("/settings/banner", h.UploadBanner)
	r.DELETE("/settings/banner", h.DeleteBanner)
	return r
}

func buildBannerUploadRequest(t *testing.T, filename string, content []byte, contentType string) *http.Request {
	t.Helper()
	const fieldName = "banner"
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

	req, err := http.NewRequest(http.MethodPost, "/settings/banner", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// BN-01: Valid PNG upload returns 200 with url field.
func TestBannerHandler_UploadBanner_ValidPNG(t *testing.T) {
	db := setupBannerTestDB(t)
	dataDir := t.TempDir()
	r := buildBannerRouter(db, dataDir, "admin")

	req := buildBannerUploadRequest(t, "mybanner.png", minimalPNG, "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"url"`)
	assert.Contains(t, w.Body.String(), `/uploads/banner.png`)

	// File must exist on disk
	assert.FileExists(t, filepath.Join(dataDir, "uploads", "banner.png"))

	// Settings must be persisted
	var urlSetting models.Setting
	require.NoError(t, db.Where("key = ?", "ui.banner_url").First(&urlSetting).Error)
	assert.Equal(t, "/uploads/banner.png", urlSetting.Value)

	var typeSetting models.Setting
	require.NoError(t, db.Where("key = ?", "ui.banner_type").First(&typeSetting).Error)
	assert.Equal(t, "upload", typeSetting.Value)
}

// BN-02: Valid WebP upload returns 200.
func TestBannerHandler_UploadBanner_ValidWebP(t *testing.T) {
	db := setupBannerTestDB(t)
	dataDir := t.TempDir()
	r := buildBannerRouter(db, dataDir, "admin")

	req := buildBannerUploadRequest(t, "mybanner.webp", minimalWebP, "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `/uploads/banner.webp`)
}

// BN-03: File > 2MB returns 413.
func TestBannerHandler_UploadBanner_FileTooLarge(t *testing.T) {
	db := setupBannerTestDB(t)
	dataDir := t.TempDir()
	r := buildBannerRouter(db, dataDir, "admin")

	large := make([]byte, maxImageSize+1024)
	copy(large, minimalPNG)

	req := buildBannerUploadRequest(t, "big.png", large, "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

// BN-04: SVG file returns 400 (byte-sniff detection).
func TestBannerHandler_UploadBanner_SVGRejected(t *testing.T) {
	db := setupBannerTestDB(t)
	dataDir := t.TempDir()
	r := buildBannerRouter(db, dataDir, "admin")

	req := buildBannerUploadRequest(t, "banner.svg", minimalSVG, "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// BN-05: SVG bytes with image/png Content-Type header returns 400.
func TestBannerHandler_UploadBanner_SpoofedContentType(t *testing.T) {
	db := setupBannerTestDB(t)
	dataDir := t.TempDir()
	r := buildBannerRouter(db, dataDir, "admin")

	req := buildBannerUploadRequest(t, "totally-a-png.png", minimalSVG, "image/png")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// BN-06: Missing "banner" field returns 400.
func TestBannerHandler_UploadBanner_MissingField(t *testing.T) {
	db := setupBannerTestDB(t)
	dataDir := t.TempDir()
	r := buildBannerRouter(db, dataDir, "admin")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	w2, _ := writer.CreateFormFile("file", "banner.png")
	_, _ = w2.Write(minimalPNG)
	_ = writer.Close()

	req, _ := http.NewRequest(http.MethodPost, "/settings/banner", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "missing banner field")
}

// BN-07: DELETE removes file and clears ui.banner_url and ui.banner_type settings.
func TestBannerHandler_DeleteBanner(t *testing.T) {
	db := setupBannerTestDB(t)
	dataDir := t.TempDir()
	r := buildBannerRouter(db, dataDir, "admin")

	// First upload
	req := buildBannerUploadRequest(t, "banner.png", minimalPNG, "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	bannerPath := filepath.Join(dataDir, "uploads", "banner.png")
	require.FileExists(t, bannerPath)

	// Now delete
	delReq, err := http.NewRequest(http.MethodDelete, "/settings/banner", http.NoBody)
	require.NoError(t, err)
	delW := httptest.NewRecorder()
	r.ServeHTTP(delW, delReq)

	assert.Equal(t, http.StatusOK, delW.Code)
	assert.Contains(t, delW.Body.String(), "banner deleted")

	// File must be gone
	_, statErr := os.Stat(bannerPath)
	assert.True(t, os.IsNotExist(statErr), "banner file should be removed after delete")

	// Settings must be gone
	var s models.Setting
	assert.Error(t, db.Where("key = ?", "ui.banner_url").First(&s).Error)
	assert.Error(t, db.Where("key = ?", "ui.banner_type").First(&s).Error)
}

// BN-08: Unauthenticated POST returns 401.
func TestBannerHandler_UploadBanner_Unauthenticated(t *testing.T) {
	db := setupBannerTestDB(t)
	dataDir := t.TempDir()
	r := buildBannerRouter(db, dataDir, "")

	req := buildBannerUploadRequest(t, "banner.png", minimalPNG, "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// BN-09: Non-admin POST returns 403.
func TestBannerHandler_UploadBanner_NonAdmin(t *testing.T) {
	db := setupBannerTestDB(t)
	dataDir := t.TempDir()
	r := buildBannerRouter(db, dataDir, "user")

	req := buildBannerUploadRequest(t, "banner.png", minimalPNG, "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// BN-10: Unauthenticated DELETE returns 401.
func TestBannerHandler_DeleteBanner_Unauthenticated(t *testing.T) {
	db := setupBannerTestDB(t)
	dataDir := t.TempDir()
	r := buildBannerRouter(db, dataDir, "")

	req, _ := http.NewRequest(http.MethodDelete, "/settings/banner", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
