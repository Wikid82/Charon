package handlers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// LogoHandler wraps ImageUploadHandler for the logo asset.
type LogoHandler struct {
	inner *ImageUploadHandler
}

// NewLogoHandler creates a new LogoHandler.
func NewLogoHandler(db *gorm.DB, dataDir string) *LogoHandler {
	cfg := AssetConfig{
		FormField:      "logo",
		URLSettingKey:  "ui.logo_url",
		TypeSettingKey: "ui.logo_type",
		FileBaseName:   "logo",
	}
	return &LogoHandler{inner: NewImageUploadHandler(db, dataDir, cfg)}
}

// UploadLogo handles POST /api/v1/settings/logo.
// Accepts multipart form with field "logo" (image/png, image/jpeg, image/webp only).
// SVG uploads are explicitly rejected — too many XSS vectors to sanitize inline.
// Validates MIME via server-side byte sniffing (does NOT trust multipart Content-Type).
// Max size 2MB enforced via MaxBytesReader before any bytes are read.
func (h *LogoHandler) UploadLogo(c *gin.Context) {
	h.inner.UploadAsset(c)
}

// DeleteLogo handles DELETE /api/v1/settings/logo.
// Clears logo settings from DB and removes the uploaded file.
func (h *LogoHandler) DeleteLogo(c *gin.Context) {
	h.inner.DeleteAsset(c)
}
