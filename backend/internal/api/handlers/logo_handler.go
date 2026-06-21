package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

const (
	maxLogoSize    = 2 * 1024 * 1024 // 2MB
	mimeSniffBytes = 512
	logoFilePerm   = 0o644
	uploadsDirPerm = 0o755
)

// LogoHandler handles logo upload and deletion.
type LogoHandler struct {
	db      *gorm.DB
	dataDir string
}

// NewLogoHandler creates a new LogoHandler.
func NewLogoHandler(db *gorm.DB, dataDir string) *LogoHandler {
	return &LogoHandler{db: db, dataDir: dataDir}
}

// UploadLogo handles POST /api/v1/settings/logo.
// Accepts multipart form with field "logo" (image/png, image/jpeg, image/webp only).
// SVG uploads are explicitly rejected — too many XSS vectors to sanitize inline.
// Validates MIME via server-side byte sniffing (does NOT trust multipart Content-Type).
// Max size 2MB enforced via MaxBytesReader before any bytes are read.
func (h *LogoHandler) UploadLogo(c *gin.Context) {
	if !requireAuthenticatedAdmin(c) {
		return
	}

	// Apply size limit BEFORE reading any bytes
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxLogoSize+1)

	if err := c.Request.ParseMultipartForm(maxLogoSize); err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file exceeds 2 MB limit"})
		return
	}

	file, _, err := c.Request.FormFile("logo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing logo field"})
		return
	}
	defer func() { _ = file.Close() }()

	// Read first 512 bytes for MIME detection
	sniff := make([]byte, mimeSniffBytes)
	n, err := io.ReadFull(file, sniff)
	if err != nil && err != io.ErrUnexpectedEOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file"})
		return
	}
	sniff = sniff[:n]

	// Server-side MIME detection — do NOT trust multipart Content-Type header
	detected := http.DetectContentType(sniff)

	ext, ok := acceptedMIME(detected)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unsupported file type: %s", detected)})
		return
	}

	// Read the remaining bytes and reconstruct full file content
	rest, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file exceeds 2 MB limit"})
		return
	}

	fullContent := append(sniff, rest...) //nolint:gocritic // intentional concat

	// Ensure uploads directory exists
	uploadsDir := filepath.Join(h.dataDir, "uploads")
	// #nosec G301 -- uploads directory needs to be accessible by web server
	if err := os.MkdirAll(uploadsDir, uploadsDirPerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create uploads directory"})
		return
	}

	// Filename is always normalized — no user input in filename (prevents path traversal)
	filename := "logo" + ext
	destPath := filepath.Join(uploadsDir, filename)

	// Write file with fixed permissions
	// #nosec G306 -- logo file needs to be world-readable for web serving
	if err := os.WriteFile(destPath, fullContent, logoFilePerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write logo file"})
		return
	}

	logoURL := "/uploads/" + filename

	// Save settings to DB
	if err := h.upsertSetting("ui.logo_url", logoURL, "ui", "string"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save logo url setting"})
		return
	}
	if err := h.upsertSetting("ui.logo_type", "upload", "ui", "string"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save logo type setting"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": logoURL})
}

// DeleteLogo handles DELETE /api/v1/settings/logo.
// Clears logo settings from DB and removes the uploaded file.
func (h *LogoHandler) DeleteLogo(c *gin.Context) {
	if !requireAuthenticatedAdmin(c) {
		return
	}

	// Delete DB settings
	h.db.Where("key IN ?", []string{"ui.logo_url", "ui.logo_type"}).Delete(&models.Setting{})

	// Remove any logo file (try all accepted extensions)
	if h.dataDir != "" {
		uploadsDir := filepath.Join(h.dataDir, "uploads")
		for _, ext := range []string{".png", ".jpg", ".webp"} {
			path := filepath.Join(uploadsDir, "logo"+ext)
			_ = os.Remove(path) // ignore not-found errors
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "logo deleted"})
}

// acceptedMIME maps a detected MIME type to a file extension.
// Returns ("", false) for disallowed types (including SVG).
func acceptedMIME(mime string) (string, bool) {
	switch mime {
	case "image/png":
		return ".png", true
	case "image/jpeg":
		return ".jpg", true
	case "image/webp":
		return ".webp", true
	default:
		return "", false
	}
}

// upsertSetting creates or updates a single Setting row.
func (h *LogoHandler) upsertSetting(key, value, category, settingType string) error {
	s := models.Setting{
		Key:      key,
		Value:    value,
		Category: category,
		Type:     settingType,
	}
	if err := h.db.Where(models.Setting{Key: key}).Assign(s).FirstOrCreate(&s).Error; err != nil {
		return fmt.Errorf("upsert setting %s: %w", key, err)
	}
	return nil
}
