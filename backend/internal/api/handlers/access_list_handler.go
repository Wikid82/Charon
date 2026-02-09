package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AccessListHandler handles access list API requests.
type AccessListHandler struct {
	service *services.AccessListService
}

// NewAccessListHandler creates a new AccessListHandler.
func NewAccessListHandler(db *gorm.DB) *AccessListHandler {
	return &AccessListHandler{
		service: services.NewAccessListService(db),
	}
}

// SetGeoIPService sets the GeoIP service for geo-based ACL lookups.
func (h *AccessListHandler) SetGeoIPService(geoipSvc *services.GeoIPService) {
	h.service.SetGeoIPService(geoipSvc)
}

// resolveAccessList resolves an access list by either numeric ID or UUID.
// It first attempts to parse as uint (backward compatibility), then tries UUID.
func (h *AccessListHandler) resolveAccessList(idOrUUID string) (*models.AccessList, error) {
	// Try parsing as numeric ID first (backward compatibility)
	if id, err := strconv.ParseUint(idOrUUID, 10, 32); err == nil {
		return h.service.GetByID(uint(id))
	}

	// Empty string check
	if idOrUUID == "" {
		return nil, fmt.Errorf("invalid ID or UUID")
	}

	// Try as UUID
	return h.service.GetByUUID(idOrUUID)
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
	acl, err := h.resolveAccessList(c.Param("id"))
	if err != nil {
		if err == services.ErrAccessListNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, acl)
}

// Update handles PUT /api/v1/access-lists/:id
func (h *AccessListHandler) Update(c *gin.Context) {
	// Resolve access list first to get the internal ID
	acl, err := h.resolveAccessList(c.Param("id"))
	if err != nil {
		if err == services.ErrAccessListNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	var updates models.AccessList
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.Update(acl.ID, &updates); err != nil {
		if err == services.ErrAccessListNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Fetch updated record
	updatedAcl, _ := h.service.GetByID(acl.ID)
	c.JSON(http.StatusOK, updatedAcl)
}

// Delete handles DELETE /api/v1/access-lists/:id
func (h *AccessListHandler) Delete(c *gin.Context) {
	// Resolve access list first to get the internal ID
	acl, err := h.resolveAccessList(c.Param("id"))
	if err != nil {
		if err == services.ErrAccessListNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	if err := h.service.Delete(acl.ID); err != nil {
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
	// Resolve access list first to get the internal ID
	acl, err := h.resolveAccessList(c.Param("id"))
	if err != nil {
		if err == services.ErrAccessListNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	var req struct {
		IPAddress string `json:"ip_address" binding:"required"`
	}
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": bindErr.Error()})
		return
	}

	allowed, reason, err := h.service.TestIP(acl.ID, req.IPAddress)
	if err != nil {
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
