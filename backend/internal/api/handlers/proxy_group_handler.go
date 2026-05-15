package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// ProxyGroupHandler handles CRUD operations for proxy groups.
type ProxyGroupHandler struct {
	service *services.ProxyGroupService
	db      *gorm.DB
}

// NewProxyGroupHandler creates a new ProxyGroupHandler.
func NewProxyGroupHandler(db *gorm.DB) *ProxyGroupHandler {
	return &ProxyGroupHandler{
		service: services.NewProxyGroupService(db),
		db:      db,
	}
}

// RegisterRoutes registers proxy group routes on the given router group.
func (h *ProxyGroupHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/proxy-groups", h.List)
	router.POST("/proxy-groups", h.Create)
	router.GET("/proxy-groups/:uuid", h.Get)
	router.PUT("/proxy-groups/:uuid", h.Update)
	router.DELETE("/proxy-groups/:uuid", h.Delete)
}

// List returns all proxy groups ordered by name.
func (h *ProxyGroupHandler) List(c *gin.Context) {
	groups, err := h.service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, groups)
}

// Create creates a new proxy group.
func (h *ProxyGroupHandler) Create(c *gin.Context) {
	var payload struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group := models.ProxyGroup{
		Name:        strings.TrimSpace(payload.Name),
		Description: payload.Description,
		Color:       payload.Color,
	}
	if group.Color == "" {
		group.Color = "#6366f1"
	}

	if err := h.service.Create(&group); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, group)
}

// Get returns a single proxy group by UUID, including host count.
func (h *ProxyGroupHandler) Get(c *gin.Context) {
	group, err := h.service.GetByUUID(c.Param("uuid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy group not found"})
		return
	}
	count, _ := h.service.GetHostCount(group.ID)
	c.JSON(http.StatusOK, gin.H{
		"uuid":        group.UUID,
		"name":        group.Name,
		"description": group.Description,
		"color":       group.Color,
		"host_count":  count,
		"created_at":  group.CreatedAt,
		"updated_at":  group.UpdatedAt,
	})
}

// Update updates an existing proxy group by UUID.
func (h *ProxyGroupHandler) Update(c *gin.Context) {
	group, err := h.service.GetByUUID(c.Param("uuid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy group not found"})
		return
	}

	var payload struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Color       *string `json:"color"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if payload.Name != nil {
		trimmed := strings.TrimSpace(*payload.Name)
		if trimmed == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name cannot be empty"})
			return
		}
		group.Name = trimmed
	}
	if payload.Description != nil {
		group.Description = *payload.Description
	}
	if payload.Color != nil {
		group.Color = *payload.Color
	}

	if err := h.service.Update(group); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, group)
}

// Delete removes a proxy group by UUID, clearing host assignments.
func (h *ProxyGroupHandler) Delete(c *gin.Context) {
	group, err := h.service.GetByUUID(c.Param("uuid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy group not found"})
		return
	}
	if err := h.service.Delete(group.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
