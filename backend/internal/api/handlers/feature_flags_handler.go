package handlers

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// FeatureFlagsHandler exposes simple DB-backed feature flags with env fallback.
type FeatureFlagsHandler struct {
	DB *gorm.DB
}

func NewFeatureFlagsHandler(db *gorm.DB) *FeatureFlagsHandler {
	return &FeatureFlagsHandler{DB: db}
}

// defaultFlags lists the canonical feature flags we expose.
var defaultFlags = []string{
	"feature.global.enabled",
	"feature.cerberus.enabled",
	"feature.uptime.enabled",
	"feature.notifications.enabled",
	"feature.docker.enabled",
}

// GetFlags returns a map of feature flag -> bool. DB setting takes precedence
// and falls back to environment variables if present.
func (h *FeatureFlagsHandler) GetFlags(c *gin.Context) {
	result := make(map[string]bool)

	for _, key := range defaultFlags {
		// Try DB
		var s models.Setting
		if err := h.DB.Where("key = ?", key).First(&s).Error; err == nil {
			v := strings.ToLower(strings.TrimSpace(s.Value))
			b := v == "1" || v == "true" || v == "yes"
			result[key] = b
			continue
		}

		// Fallback to env vars. Try FEATURE_... and also stripped service name e.g. CERBERUS_ENABLED
		envKey := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
		if ev, ok := os.LookupEnv(envKey); ok {
			if bv, err := strconv.ParseBool(ev); err == nil {
				result[key] = bv
				continue
			}
			// accept 1/0
			result[key] = ev == "1"
			continue
		}

		// Try shorter variant after removing leading "feature."
		if strings.HasPrefix(key, "feature.") {
			short := strings.ToUpper(strings.ReplaceAll(strings.TrimPrefix(key, "feature."), ".", "_"))
			if ev, ok := os.LookupEnv(short); ok {
				if bv, err := strconv.ParseBool(ev); err == nil {
					result[key] = bv
					continue
				}
				result[key] = ev == "1"
				continue
			}
		}

		// Default false
		result[key] = false
	}

	c.JSON(http.StatusOK, result)
}

// UpdateFlags accepts a JSON object map[string]bool and upserts settings.
func (h *FeatureFlagsHandler) UpdateFlags(c *gin.Context) {
	var payload map[string]bool
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for k, v := range payload {
		// Only allow keys in the default list to avoid arbitrary settings
		allowed := false
		for _, ak := range defaultFlags {
			if ak == k {
				allowed = true
				break
			}
		}
		if !allowed {
			continue
		}

		s := models.Setting{Key: k, Value: strconv.FormatBool(v), Type: "bool", Category: "feature"}
		if err := h.DB.Where(models.Setting{Key: k}).Assign(s).FirstOrCreate(&s).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save setting"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
