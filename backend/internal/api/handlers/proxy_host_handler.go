package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
)

// ProxyHostHandler handles CRUD operations for proxy hosts.
type ProxyHostHandler struct {
	service             *services.ProxyHostService
	caddyManager        *caddy.Manager
	notificationService *services.NotificationService
	uptimeService       *services.UptimeService
}

// NewProxyHostHandler creates a new proxy host handler.
func NewProxyHostHandler(db *gorm.DB, caddyManager *caddy.Manager, ns *services.NotificationService, uptimeService *services.UptimeService) *ProxyHostHandler {
	return &ProxyHostHandler{
		service:             services.NewProxyHostService(db),
		caddyManager:        caddyManager,
		notificationService: ns,
		uptimeService:       uptimeService,
	}
}

// RegisterRoutes registers proxy host routes.
func (h *ProxyHostHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/proxy-hosts", h.List)
	router.POST("/proxy-hosts", h.Create)
	router.GET("/proxy-hosts/:uuid", h.Get)
	router.PUT("/proxy-hosts/:uuid", h.Update)
	router.DELETE("/proxy-hosts/:uuid", h.Delete)
	router.POST("/proxy-hosts/test", h.TestConnection)
	router.PUT("/proxy-hosts/bulk-update-acl", h.BulkUpdateACL)
}

// List retrieves all proxy hosts.
func (h *ProxyHostHandler) List(c *gin.Context) {
	hosts, err := h.service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, hosts)
}

// Create creates a new proxy host.
func (h *ProxyHostHandler) Create(c *gin.Context) {
	var host models.ProxyHost
	if err := c.ShouldBindJSON(&host); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate and normalize advanced config if present
	if host.AdvancedConfig != "" {
		var parsed interface{}
		if err := json.Unmarshal([]byte(host.AdvancedConfig), &parsed); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid advanced_config JSON: " + err.Error()})
			return
		}
		parsed = caddy.NormalizeAdvancedConfig(parsed)
		if norm, err := json.Marshal(parsed); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid advanced_config after normalization: " + err.Error()})
			return
		} else {
			host.AdvancedConfig = string(norm)
		}
	}

	host.UUID = uuid.NewString()

	// Assign UUIDs to locations
	for i := range host.Locations {
		host.Locations[i].UUID = uuid.NewString()
	}

	if err := h.service.Create(&host); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.caddyManager != nil {
		if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
			// Rollback: delete the created host if config application fails
			middleware.GetRequestLogger(c).WithError(err).Error("Error applying config")
			if deleteErr := h.service.Delete(host.ID); deleteErr != nil {
				idStr := strconv.FormatUint(uint64(host.ID), 10)
				middleware.GetRequestLogger(c).WithField("host_id", idStr).WithError(deleteErr).Error("Critical: Failed to rollback host")
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply configuration: " + err.Error()})
			return
		}
	}

	// Send Notification
	if h.notificationService != nil {
		h.notificationService.SendExternal(c.Request.Context(),
			"proxy_host",
			"Proxy Host Created",
			fmt.Sprintf("Proxy Host %s (%s) created", util.SanitizeForLog(host.Name), util.SanitizeForLog(host.DomainNames)),
			map[string]interface{}{
				"Name":    util.SanitizeForLog(host.Name),
				"Domains": util.SanitizeForLog(host.DomainNames),
				"Action":  "created",
			},
		)
	}

	c.JSON(http.StatusCreated, host)
}

// Get retrieves a proxy host by UUID.
func (h *ProxyHostHandler) Get(c *gin.Context) {
	uuid := c.Param("uuid")

	host, err := h.service.GetByUUID(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy host not found"})
		return
	}

	c.JSON(http.StatusOK, host)
}

// Update updates an existing proxy host.
func (h *ProxyHostHandler) Update(c *gin.Context) {
	uuid := c.Param("uuid")

	host, err := h.service.GetByUUID(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy host not found"})
		return
	}

	var incoming models.ProxyHost
	if err := c.ShouldBindJSON(&incoming); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Validate and normalize advanced config if present and changed
	if incoming.AdvancedConfig != "" && incoming.AdvancedConfig != host.AdvancedConfig {
		var parsed interface{}
		if err := json.Unmarshal([]byte(incoming.AdvancedConfig), &parsed); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid advanced_config JSON: " + err.Error()})
			return
		}
		parsed = caddy.NormalizeAdvancedConfig(parsed)
		if norm, err := json.Marshal(parsed); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid advanced_config after normalization: " + err.Error()})
			return
		} else {
			incoming.AdvancedConfig = string(norm)
		}
	}

	// Backup advanced config if changed
	if incoming.AdvancedConfig != host.AdvancedConfig {
		incoming.AdvancedConfigBackup = host.AdvancedConfig
	}

	// Copy incoming fields into host
	host.Name = incoming.Name
	host.DomainNames = incoming.DomainNames
	host.ForwardScheme = incoming.ForwardScheme
	host.ForwardHost = incoming.ForwardHost
	host.ForwardPort = incoming.ForwardPort
	host.SSLForced = incoming.SSLForced
	host.HTTP2Support = incoming.HTTP2Support
	host.HSTSEnabled = incoming.HSTSEnabled
	host.HSTSSubdomains = incoming.HSTSSubdomains
	host.BlockExploits = incoming.BlockExploits
	host.WebsocketSupport = incoming.WebsocketSupport
	host.Application = incoming.Application
	host.Enabled = incoming.Enabled
	host.CertificateID = incoming.CertificateID
	host.AccessListID = incoming.AccessListID
	host.Locations = incoming.Locations
	host.AdvancedConfig = incoming.AdvancedConfig
	host.AdvancedConfigBackup = incoming.AdvancedConfigBackup

	if err := h.service.Update(host); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.caddyManager != nil {
		if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply configuration: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, host)
}

// Delete removes a proxy host.
func (h *ProxyHostHandler) Delete(c *gin.Context) {
	uuid := c.Param("uuid")

	host, err := h.service.GetByUUID(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy host not found"})
		return
	}

	// check if we should also delete associated uptime monitors (query param: delete_uptime=true)
	deleteUptime := c.DefaultQuery("delete_uptime", "false") == "true"

	if deleteUptime && h.uptimeService != nil {
		// Find all monitors referencing this proxy host and delete each
		var monitors []models.UptimeMonitor
		if err := h.uptimeService.DB.Where("proxy_host_id = ?", host.ID).Find(&monitors).Error; err == nil {
			for _, m := range monitors {
				_ = h.uptimeService.DeleteMonitor(m.ID)
			}
		}
	}

	if err := h.service.Delete(host.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.caddyManager != nil {
		if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply configuration: " + err.Error()})
			return
		}
	}

	// Send Notification
	if h.notificationService != nil {
		h.notificationService.SendExternal(c.Request.Context(),
			"proxy_host",
			"Proxy Host Deleted",
			fmt.Sprintf("Proxy Host %s deleted", host.Name),
			map[string]interface{}{
				"Name":   host.Name,
				"Action": "deleted",
			},
		)
	}

	c.JSON(http.StatusOK, gin.H{"message": "proxy host deleted"})
}

// TestConnection checks if the proxy host is reachable.
func (h *ProxyHostHandler) TestConnection(c *gin.Context) {
	var req struct {
		ForwardHost string `json:"forward_host" binding:"required"`
		ForwardPort int    `json:"forward_port" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.TestConnection(req.ForwardHost, req.ForwardPort); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Connection successful"})
}

// BulkUpdateACL applies or removes an access list to multiple proxy hosts.
func (h *ProxyHostHandler) BulkUpdateACL(c *gin.Context) {
	var req struct {
		HostUUIDs    []string `json:"host_uuids" binding:"required"`
		AccessListID *uint    `json:"access_list_id"` // nil means remove ACL
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.HostUUIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_uuids cannot be empty"})
		return
	}

	updated := 0
	errors := []map[string]string{}

	for _, uuid := range req.HostUUIDs {
		host, err := h.service.GetByUUID(uuid)
		if err != nil {
			errors = append(errors, map[string]string{
				"uuid":  uuid,
				"error": "proxy host not found",
			})
			continue
		}

		host.AccessListID = req.AccessListID
		if err := h.service.Update(host); err != nil {
			errors = append(errors, map[string]string{
				"uuid":  uuid,
				"error": err.Error(),
			})
			continue
		}

		updated++
	}

	// Apply Caddy config once for all updates
	if updated > 0 && h.caddyManager != nil {
		if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to apply configuration: " + err.Error(),
				"updated": updated,
				"errors":  errors,
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"updated": updated,
		"errors":  errors,
	})
}
