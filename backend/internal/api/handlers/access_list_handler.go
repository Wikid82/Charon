package handlers

import (
	"net/http"
	"strconv"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AccessListHandler struct {
	service *services.AccessListService
}

func NewAccessListHandler(db *gorm.DB) *AccessListHandler {
	return &AccessListHandler{
		service: services.NewAccessListService(db),
	}
}

// Create handles POST /api/v1/access-lists
func (h *AccessListHandler) Create(c *gin.Context) {
	var acl models.AccessList
	if err := c.ShouldBindJSON(&acl); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.Create(&acl); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, acl)
}

// List handles GET /api/v1/access-lists
func (h *AccessListHandler) List(c *gin.Context) {
	acls, err := h.service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, acls)
}

// Get handles GET /api/v1/access-lists/:id
func (h *AccessListHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}

	acl, err := h.service.GetByID(uint(id))
	if err != nil {
		if err == services.ErrAccessListNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, acl)
}

// Update handles PUT /api/v1/access-lists/:id
func (h *AccessListHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}

	var updates models.AccessList
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.Update(uint(id), &updates); err != nil {
		if err == services.ErrAccessListNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Fetch updated record
	acl, _ := h.service.GetByID(uint(id))
	c.JSON(http.StatusOK, acl)
}

// Delete handles DELETE /api/v1/access-lists/:id
func (h *AccessListHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}

	if err := h.service.Delete(uint(id)); err != nil {
		if err == services.ErrAccessListNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
			return
		}
		if err == services.ErrAccessListInUse {
			c.JSON(http.StatusConflict, gin.H{"error": "access list is in use by proxy hosts"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "access list deleted"})
}

// TestIP handles POST /api/v1/access-lists/:id/test
func (h *AccessListHandler) TestIP(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}

	var req struct {
		IPAddress string `json:"ip_address" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	allowed, reason, err := h.service.TestIP(uint(id), req.IPAddress)
	if err != nil {
		if err == services.ErrAccessListNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
			return
		}
		if err == services.ErrInvalidIPAddress {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid IP address"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"allowed": allowed,
		"reason":  reason,
	})
}

// GetTemplates handles GET /api/v1/access-lists/templates
func (h *AccessListHandler) GetTemplates(c *gin.Context) {
	templates := h.service.GetTemplates()
	c.JSON(http.StatusOK, templates)
}
