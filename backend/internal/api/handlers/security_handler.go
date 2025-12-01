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

	// Allow runtime overrides for CrowdSec mode + API URL via settings table
	mode := h.cfg.CrowdSecMode
	apiURL := h.cfg.CrowdSecAPIURL
	if h.db != nil {
		var m struct{ Value string }
		if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.mode").Scan(&m).Error; err == nil && m.Value != "" {
			mode = m.Value
		}
		var a struct{ Value string }
		if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.api_url").Scan(&a).Error; err == nil && a.Value != "" {
			apiURL = a.Value
		}
	}

	// Treat external crowdsec mode as unsupported in this release. If configured as 'external',
	// present it as disabled so the UI doesn't attempt to call out to an external agent.
	if mode == "external" {
		mode = "disabled"
		apiURL = ""
	}

	// Allow runtime override for ACL enabled flag via settings table
	aclEnabled := h.cfg.ACLMode == "enabled"
	if h.db != nil {
		var a struct{ Value string }
		if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.acl.enabled").Scan(&a).Error; err == nil {
			if strings.EqualFold(a.Value, "true") {
				aclEnabled = true
			} else if strings.EqualFold(a.Value, "false") {
				aclEnabled = false
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"cerberus": gin.H{"enabled": enabled},
		"crowdsec": gin.H{
			"mode":    mode,
			"api_url": apiURL,
			"enabled": mode == "local",
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
			"enabled": aclEnabled,
		},
	})
}
