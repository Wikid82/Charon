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
	maxImageSize   = 2 * 1024 * 1024 // 2MB
	maxLogoSize    = maxImageSize     // backward-compat alias used by logo_handler_test.go
	mimeSniffBytes = 512
	logoFilePerm   = 0o644
	uploadsDirPerm = 0o755
)

// AssetConfig parameterizes a specific image asset type.
type AssetConfig struct {
	FormField      string // multipart field name, e.g. "logo" or "banner"
	URLSettingKey  string // e.g. "ui.logo_url" or "ui.banner_url"
	TypeSettingKey string // e.g. "ui.logo_type" or "ui.banner_type"
	FileBaseName   string // e.g. "logo" or "banner" (extension appended at runtime)
}

// ImageUploadHandler handles generic image asset upload and deletion.
// Both UploadAsset and DeleteAsset enforce requireAuthenticatedAdmin internally.
type ImageUploadHandler struct {
	db      *gorm.DB
	dataDir string
	cfg     AssetConfig
}

// NewImageUploadHandler creates a new ImageUploadHandler.
func NewImageUploadHandler(db *gorm.DB, dataDir string, cfg AssetConfig) *ImageUploadHandler {
	return &ImageUploadHandler{db: db, dataDir: dataDir, cfg: cfg}
}

// UploadAsset handles POST for any image asset type.
// Calls requireAuthenticatedAdmin(c) at the top — returns 401/403 and aborts if not satisfied.
func (h *ImageUploadHandler) UploadAsset(c *gin.Context) {
	if !requireAuthenticatedAdmin(c) {
		return
	}

	// Apply size limit BEFORE reading any bytes
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImageSize+1)

	if err := c.Request.ParseMultipartForm(maxImageSize); err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file exceeds 2 MB limit"})
		return
	}

	file, _, err := c.Request.FormFile(h.cfg.FormField)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing " + h.cfg.FormField + " field"})
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
	filename := h.cfg.FileBaseName + ext
	destPath := filepath.Join(uploadsDir, filename)

	// Write file with fixed permissions
	// #nosec G306 -- asset file needs to be world-readable for web serving
	if err := os.WriteFile(destPath, fullContent, logoFilePerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write " + h.cfg.FileBaseName + " file"})
		return
	}

	assetURL := "/uploads/" + filename

	// Save settings to DB
	if err := h.upsertSetting(h.cfg.URLSettingKey, assetURL, "ui", "string"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save " + h.cfg.FileBaseName + " url setting"})
		return
	}
	if err := h.upsertSetting(h.cfg.TypeSettingKey, "upload", "ui", "string"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save " + h.cfg.FileBaseName + " type setting"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": assetURL})
}

// DeleteAsset handles DELETE for any image asset type.
// Calls requireAuthenticatedAdmin(c) at the top — returns 401/403 and aborts if not satisfied.
func (h *ImageUploadHandler) DeleteAsset(c *gin.Context) {
	if !requireAuthenticatedAdmin(c) {
		return
	}

	// Delete DB settings
	h.db.Where("key IN ?", []string{h.cfg.URLSettingKey, h.cfg.TypeSettingKey}).Delete(&models.Setting{})

	// Remove any asset file (try all accepted extensions)
	if h.dataDir != "" {
		uploadsDir := filepath.Join(h.dataDir, "uploads")
		for _, ext := range []string{".png", ".jpg", ".webp"} {
			path := filepath.Join(uploadsDir, h.cfg.FileBaseName+ext)
			_ = os.Remove(path) // ignore not-found errors
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": h.cfg.FileBaseName + " deleted"})
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
func (h *ImageUploadHandler) upsertSetting(key, value, category, settingType string) error {
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
