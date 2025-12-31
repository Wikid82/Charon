package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SecurityHeadersHandler manages security header profiles
type SecurityHeadersHandler struct {
	db           *gorm.DB
	caddyManager *caddy.Manager
	service      *services.SecurityHeadersService
}

// NewSecurityHeadersHandler creates a new handler
func NewSecurityHeadersHandler(db *gorm.DB, caddyManager *caddy.Manager) *SecurityHeadersHandler {
	return &SecurityHeadersHandler{
		db:           db,
		caddyManager: caddyManager,
		service:      services.NewSecurityHeadersService(db),
	}
}

// RegisterRoutes registers all security headers routes
func (h *SecurityHeadersHandler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/security/headers")
	group.GET("/profiles", h.ListProfiles)
	group.GET("/profiles/:id", h.GetProfile)
	group.POST("/profiles", h.CreateProfile)
	group.PUT("/profiles/:id", h.UpdateProfile)
	group.DELETE("/profiles/:id", h.DeleteProfile)

	group.GET("/presets", h.GetPresets)
	group.POST("/presets/apply", h.ApplyPreset)

	group.POST("/score", h.CalculateScore)

	group.POST("/csp/validate", h.ValidateCSP)
	group.POST("/csp/build", h.BuildCSP)
}

// ListProfiles returns all security header profiles
// GET /api/v1/security/headers/profiles
func (h *SecurityHeadersHandler) ListProfiles(c *gin.Context) {
	var profiles []models.SecurityHeaderProfile
	if err := h.db.Order("is_preset DESC, name ASC").Find(&profiles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"profiles": profiles})
}

// GetProfile returns a single profile by ID or UUID
// GET /api/v1/security/headers/profiles/:id
func (h *SecurityHeadersHandler) GetProfile(c *gin.Context) {
	idParam := c.Param("id")

	var profile models.SecurityHeaderProfile

	// Try to parse as uint ID first
	if id, err := strconv.ParseUint(idParam, 10, 32); err == nil {
		if err := h.db.First(&profile, uint(id)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		// Try UUID
		if err := h.db.Where("uuid = ?", idParam).First(&profile).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"profile": profile})
}

// CreateProfile creates a new security header profile
// POST /api/v1/security/headers/profiles
func (h *SecurityHeadersHandler) CreateProfile(c *gin.Context) {
	var req models.SecurityHeaderProfile
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate name is provided
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	// Generate UUID
	req.UUID = uuid.New().String()

	// Calculate security score
	scoreResult := services.CalculateSecurityScore(&req)
	req.SecurityScore = scoreResult.TotalScore

	// Create profile
	if err := h.db.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"profile": req})
}

// UpdateProfile updates an existing profile
// PUT /api/v1/security/headers/profiles/:id
func (h *SecurityHeadersHandler) UpdateProfile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}

	var existing models.SecurityHeaderProfile
	if err := h.db.First(&existing, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Cannot modify presets
	if existing.IsPreset {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot modify system presets"})
		return
	}

	var updates models.SecurityHeaderProfile
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Preserve ID and UUID
	updates.ID = existing.ID
	updates.UUID = existing.UUID

	// Recalculate security score
	scoreResult := services.CalculateSecurityScore(&updates)
	updates.SecurityScore = scoreResult.TotalScore

	if err := h.db.Save(&updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"profile": updates})
}

// DeleteProfile deletes a profile (not presets)
// DELETE /api/v1/security/headers/profiles/:id
func (h *SecurityHeadersHandler) DeleteProfile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}

	var profile models.SecurityHeaderProfile
	if err := h.db.First(&profile, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Cannot delete presets
	if profile.IsPreset {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot delete system presets"})
		return
	}

	// Check if profile is in use by any proxy hosts
	var count int64
	if err := h.db.Model(&models.ProxyHost{}).Where("security_header_profile_id = ?", id).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("profile is in use by %d proxy host(s)", count)})
		return
	}

	if err := h.db.Delete(&profile).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// GetPresets returns the list of built-in presets
// GET /api/v1/security/headers/presets
func (h *SecurityHeadersHandler) GetPresets(c *gin.Context) {
	presets := h.service.GetPresets()
	c.JSON(http.StatusOK, gin.H{"presets": presets})
}

// ApplyPreset applies a preset to create/update a profile
// POST /api/v1/security/headers/presets/apply
func (h *SecurityHeadersHandler) ApplyPreset(c *gin.Context) {
	var req struct {
		PresetType string `json:"preset_type" binding:"required"`
		Name       string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	profile, err := h.service.ApplyPreset(req.PresetType, req.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"profile": profile})
}

// CalculateScore calculates security score for given settings
// POST /api/v1/security/headers/score
func (h *SecurityHeadersHandler) CalculateScore(c *gin.Context) {
	var profile models.SecurityHeaderProfile
	if err := c.ShouldBindJSON(&profile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	scoreResult := services.CalculateSecurityScore(&profile)
	c.JSON(http.StatusOK, scoreResult)
}

// ValidateCSP validates a CSP string
// POST /api/v1/security/headers/csp/validate
func (h *SecurityHeadersHandler) ValidateCSP(c *gin.Context) {
	var req struct {
		CSP string `json:"csp" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	errors := validateCSPString(req.CSP)

	c.JSON(http.StatusOK, gin.H{
		"valid":  len(errors) == 0,
		"errors": errors,
	})
}

// BuildCSP builds a CSP string from directives
// POST /api/v1/security/headers/csp/build
func (h *SecurityHeadersHandler) BuildCSP(c *gin.Context) {
	var req struct {
		Directives []models.CSPDirective `json:"directives" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert directives to map for JSON storage
	directivesMap := make(map[string][]string)
	for _, dir := range req.Directives {
		directivesMap[dir.Directive] = dir.Values
	}

	cspJSON, err := json.Marshal(directivesMap)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build CSP"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"csp": string(cspJSON)})
}

// validateCSPString performs basic validation on a CSP string
func validateCSPString(csp string) []string {
	var errors []string

	if csp == "" {
		errors = append(errors, "CSP cannot be empty")
		return errors
	}

	// Try to parse as JSON
	var directivesMap map[string][]string
	if err := json.Unmarshal([]byte(csp), &directivesMap); err != nil {
		errors = append(errors, "CSP must be valid JSON")
		return errors
	}

	// Validate known directives
	validDirectives := map[string]bool{
		"default-src":               true,
		"script-src":                true,
		"style-src":                 true,
		"img-src":                   true,
		"font-src":                  true,
		"connect-src":               true,
		"frame-src":                 true,
		"object-src":                true,
		"media-src":                 true,
		"worker-src":                true,
		"manifest-src":              true,
		"base-uri":                  true,
		"form-action":               true,
		"frame-ancestors":           true,
		"report-uri":                true,
		"report-to":                 true,
		"upgrade-insecure-requests": true,
		"block-all-mixed-content":   true,
	}

	for directive := range directivesMap {
		if !validDirectives[directive] {
			errors = append(errors, fmt.Sprintf("unknown CSP directive: %s", directive))
		}
	}

	// Warn about unsafe directives
	for directive, values := range directivesMap {
		for _, value := range values {
			if strings.Contains(value, "unsafe-inline") || strings.Contains(value, "unsafe-eval") {
				errors = append(errors, fmt.Sprintf("'%s' contains unsafe directive in %s", value, directive))
			}
		}
	}

	return errors
}
