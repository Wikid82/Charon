package handlers

import (
	"net/http"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ForwardAuthHandler handles forward authentication configuration endpoints.
type ForwardAuthHandler struct {
	db *gorm.DB
}

// NewForwardAuthHandler creates a new handler.
func NewForwardAuthHandler(db *gorm.DB) *ForwardAuthHandler {
	return &ForwardAuthHandler{db: db}
}

// GetConfig retrieves the forward auth configuration.
func (h *ForwardAuthHandler) GetConfig(c *gin.Context) {
	var config models.ForwardAuthConfig
	if err := h.db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return default/empty config
			c.JSON(http.StatusOK, models.ForwardAuthConfig{
				Provider:           "custom",
				Address:            "",
				TrustForwardHeader: true,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch config"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// UpdateConfig updates or creates the forward auth configuration.
func (h *ForwardAuthHandler) UpdateConfig(c *gin.Context) {
	var input struct {
		Provider           string `json:"provider" binding:"required,oneof=authelia authentik pomerium custom"`
		Address            string `json:"address" binding:"required,url"`
		TrustForwardHeader bool   `json:"trust_forward_header"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var config models.ForwardAuthConfig
	err := h.db.First(&config).Error

	if err == gorm.ErrRecordNotFound {
		// Create new config
		config = models.ForwardAuthConfig{
			Provider:           input.Provider,
			Address:            input.Address,
			TrustForwardHeader: input.TrustForwardHeader,
		}
		if err := h.db.Create(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create config"})
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch config"})
		return
	} else {
		// Update existing config
		config.Provider = input.Provider
		config.Address = input.Address
		config.TrustForwardHeader = input.TrustForwardHeader
		if err := h.db.Save(&config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update config"})
			return
		}
	}

	c.JSON(http.StatusOK, config)
}

// GetTemplates returns pre-configured templates for popular providers.
func (h *ForwardAuthHandler) GetTemplates(c *gin.Context) {
	templates := map[string]interface{}{
		"authelia": gin.H{
			"provider":             "authelia",
			"address":              "http://authelia:9091/api/verify",
			"trust_forward_header": true,
			"description":          "Authelia authentication server",
		},
		"authentik": gin.H{
			"provider":             "authentik",
			"address":              "http://authentik-server:9000/outpost.goauthentik.io/auth/caddy",
			"trust_forward_header": true,
			"description":          "Authentik SSO provider",
		},
		"pomerium": gin.H{
			"provider":             "pomerium",
			"address":              "https://verify.pomerium.app",
			"trust_forward_header": true,
			"description":          "Pomerium identity-aware proxy",
		},
	}

	c.JSON(http.StatusOK, templates)
}
