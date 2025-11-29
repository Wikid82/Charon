package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
)

// SecurityHandler handles security-related API requests.
type SecurityHandler struct {
	cfg config.SecurityConfig
	db  *gorm.DB
}

// NewSecurityHandler creates a new SecurityHandler.
func NewSecurityHandler(cfg config.SecurityConfig, db *gorm.DB) *SecurityHandler {
	return &SecurityHandler{
		cfg: cfg,
		db:  db,
	}
}

// GetStatus returns the current status of all security services.
func (h *SecurityHandler) GetStatus(c *gin.Context) {
	enabled := h.cfg.CerberusEnabled
	// Check runtime setting override
	var settingKey = "security.cerberus.enabled"
	if h.db != nil {
		var setting struct {
			Value string
		}
		if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", settingKey).Scan(&setting).Error; err == nil {
			if strings.EqualFold(setting.Value, "true") {
				enabled = true
			} else {
				enabled = false
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"cerberus": gin.H{"enabled": enabled},
		"crowdsec": gin.H{
			"mode":    h.cfg.CrowdSecMode,
			"api_url": h.cfg.CrowdSecAPIURL,
			"enabled": h.cfg.CrowdSecMode != "disabled",
		},
		"waf": gin.H{
			"mode":    h.cfg.WAFMode,
			"enabled": h.cfg.WAFMode == "enabled",
		},
		"rate_limit": gin.H{
			"mode":    h.cfg.RateLimitMode,
			"enabled": h.cfg.RateLimitMode == "enabled",
		},
		"acl": gin.H{
			"mode":    h.cfg.ACLMode,
			"enabled": h.cfg.ACLMode == "enabled",
		},
	})
}
