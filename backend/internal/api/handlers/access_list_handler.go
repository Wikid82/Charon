package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AccessListHandler handles access list API requests.
type AccessListHandler struct {
	service      *services.AccessListService
	caddyManager CaddyConfigManager
}

// NewAccessListHandler creates a new AccessListHandler without proxy apply, for callers that only manage lists.
func NewAccessListHandler(db *gorm.DB) *AccessListHandler {
	return NewAccessListHandlerWithDeps(db, nil)
}

// NewAccessListHandlerWithDeps creates an AccessListHandler that re-applies the proxy config when an edit reaches a
// list that hosts use.
func NewAccessListHandlerWithDeps(db *gorm.DB, caddyManager CaddyConfigManager) *AccessListHandler {
	return &AccessListHandler{
		service:      services.NewAccessListService(db),
		caddyManager: nilIfTypedNil(caddyManager),
	}
}

// applyIfInUse re-applies the proxy config after a change to a list. A list no host uses is not part of the proxy
// config, so nothing is applied for it. It returns the apply error, if any.
func (h *AccessListHandler) applyIfInUse(c *gin.Context, aclID uint) error {
	if h.caddyManager == nil {
		return nil
	}
	inUse, err := h.service.InUse(aclID)
	if err != nil {
		return err
	}
	if !inUse {
		return nil
	}
	return applyProxyConfigDetached(c.Request.Context(), h.caddyManager, proxyApplyTimeout)
}

// SetGeoIPService sets the GeoIP service for geo-based ACL lookups.
func (h *AccessListHandler) SetGeoIPService(geoipSvc *services.GeoIPService) {
	h.service.SetGeoIPService(geoipSvc)
}

// accessListRulesRequiredCode is the stable error_code for ErrAccessListRulesRequired; the UI maps it to a message.
const accessListRulesRequiredCode = "access_list_rules_required"

// respondAccessListSaveError answers a refused create or update with 400. Known refusals carry a stable error_code.
func respondAccessListSaveError(c *gin.Context, err error) {
	if errors.Is(err, services.ErrAccessListRulesRequired) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "error_code": accessListRulesRequiredCode})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		respondAccessListSaveError(c, err)
		return
	}

	createdACL, err := h.service.GetByUUID(acl.UUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, createdACL)
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
	err = c.ShouldBindJSON(&updates)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.service.Update(acl.ID, &updates)
	if err != nil {
		if err == services.ErrAccessListNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
			return
		}
		respondAccessListSaveError(c, err)
		return
	}

	// The edit is saved at this point; an apply failure is reported, and the next apply picks the saved rules up.
	if err = h.applyIfInUse(c, acl.ID); err != nil {
		middleware.GetRequestLogger(c).WithError(err).Warn("failed to apply proxy config after access list update")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "saved, but applying the configuration failed"})
		return
	}

	updatedAcl, err := h.service.GetByID(acl.ID)
	if err != nil {
		if err == services.ErrAccessListNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

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
