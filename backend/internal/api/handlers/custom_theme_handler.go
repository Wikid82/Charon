package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// CustomThemeHandler handles CRUD for user-created named themes.
type CustomThemeHandler struct {
	db *gorm.DB
}

// NewCustomThemeHandler creates a new CustomThemeHandler.
func NewCustomThemeHandler(db *gorm.DB) *CustomThemeHandler {
	return &CustomThemeHandler{db: db}
}

type createThemeRequest struct {
	Name   string `json:"name"   binding:"required,max=100"`
	Colors string `json:"colors" binding:"required"`
}

type updateThemeRequest struct {
	Name   *string `json:"name"`
	Colors *string `json:"colors"`
}

// ListThemes handles GET /api/v1/themes.
// Returns all user-created themes ordered by created_at ASC.
// Always returns a JSON array (never null) — empty array when no themes exist.
func (h *CustomThemeHandler) ListThemes(c *gin.Context) {
	result := []models.CustomTheme{}
	if err := h.db.Order("created_at ASC").Find(&result).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list themes"})
		return
	}
	if result == nil {
		result = []models.CustomTheme{}
	}
	c.JSON(http.StatusOK, result)
}

// CreateTheme handles POST /api/v1/themes.
// Body: { "name": string, "colors": string (JSON) }
// Returns: 201 with the created CustomTheme record.
func (h *CustomThemeHandler) CreateTheme(c *gin.Context) {
	var req createThemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !json.Valid([]byte(req.Colors)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "colors must be valid JSON"})
		return
	}

	theme := models.CustomTheme{
		Name:   req.Name,
		Colors: req.Colors,
	}

	if err := h.db.Create(&theme).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "UNIQUE constraint failed") {
			c.JSON(http.StatusConflict, gin.H{"error": "a theme with that name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create theme"})
		return
	}

	c.JSON(http.StatusCreated, theme)
}

// UpdateTheme handles PUT /api/v1/themes/:id.
// Body: { "name"?: string, "colors"?: string (JSON) }
// Returns: 200 with the updated record.
func (h *CustomThemeHandler) UpdateTheme(c *gin.Context) {
	id := c.Param("id")

	var theme models.CustomTheme
	if err := h.db.First(&theme, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "theme not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch theme"})
		return
	}

	var req updateThemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != nil {
		if len(*req.Name) == 0 || len(*req.Name) > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name cannot be empty"})
			return
		}
		theme.Name = *req.Name
	}

	if req.Colors != nil {
		if !json.Valid([]byte(*req.Colors)) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "colors must be valid JSON"})
			return
		}
		theme.Colors = *req.Colors
	}

	if err := h.db.Save(&theme).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "UNIQUE constraint failed") {
			c.JSON(http.StatusConflict, gin.H{"error": "a theme with that name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update theme"})
		return
	}

	c.JSON(http.StatusOK, theme)
}

// DeleteTheme handles DELETE /api/v1/themes/:id.
// Returns: 200 { "message": "theme deleted" }
func (h *CustomThemeHandler) DeleteTheme(c *gin.Context) {
	id := c.Param("id")

	result := h.db.Delete(&models.CustomTheme{}, "id = ?", id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete theme"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "theme not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "theme deleted"})
}
