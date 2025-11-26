package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
)

// SecurityHandler handles security-related API requests.
type SecurityHandler struct {
	cfg config.SecurityConfig
}

// NewSecurityHandler creates a new SecurityHandler.
func NewSecurityHandler(cfg config.SecurityConfig) *SecurityHandler {
	return &SecurityHandler{
		cfg: cfg,
	}
}

// GetStatus returns the current status of all security services.
func (h *SecurityHandler) GetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
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
			"enabled": h.cfg.RateLimitEnabled,
		},
		"acl": gin.H{
			"enabled": h.cfg.ACLEnabled,
		},
	})
}
