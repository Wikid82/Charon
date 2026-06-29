package handlers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BannerHandler wraps ImageUploadHandler for the banner asset.
type BannerHandler struct {
	inner *ImageUploadHandler
}

// NewBannerHandler creates a new BannerHandler.
func NewBannerHandler(db *gorm.DB, dataDir string) *BannerHandler {
	cfg := AssetConfig{
		FormField:      "banner",
		URLSettingKey:  "ui.banner_url",
		TypeSettingKey: "ui.banner_type",
		FileBaseName:   "banner",
	}
	return &BannerHandler{inner: NewImageUploadHandler(db, dataDir, cfg)}
}

// UploadBanner handles POST /api/v1/settings/banner.
// Accepts multipart form with field "banner" (image/png, image/jpeg, image/webp only).
// SVG uploads are explicitly rejected — too many XSS vectors to sanitize inline.
// Validates MIME via server-side byte sniffing (does NOT trust multipart Content-Type).
// Max size 2MB enforced via MaxBytesReader before any bytes are read.
func (h *BannerHandler) UploadBanner(c *gin.Context) {
	h.inner.UploadAsset(c)
}

// DeleteBanner handles DELETE /api/v1/settings/banner.
// Clears banner settings from DB and removes the uploaded file.
func (h *BannerHandler) DeleteBanner(c *gin.Context) {
	h.inner.DeleteAsset(c)
}
