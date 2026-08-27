package handlers

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
)

// RemoteServerHandler handles HTTP requests for remote server management.
type RemoteServerHandler struct {
	service             *services.RemoteServerService
	notificationService *services.NotificationService
	uptimeService       *services.UptimeService // nil-guarded; wired via SetUptimeService
}

// NewRemoteServerHandler creates a new remote server handler.
func NewRemoteServerHandler(service *services.RemoteServerService, ns *services.NotificationService) *RemoteServerHandler {
	return &RemoteServerHandler{
		service:             service,
		notificationService: ns,
	}
}

// SetUptimeService wires the uptime service so remote-server CRUD drives an
// immediate targeted monitor sync (spec §3.1.3, user decision #4). Mirrors
// ProxyHostHandler.SetCertificateService — a setter rather than a constructor
// arg keeps the ~20 existing NewRemoteServerHandler call sites unchanged.
func (h *RemoteServerHandler) SetUptimeService(u *services.UptimeService) {
	h.uptimeService = u
}

// RegisterRoutes registers remote server routes.
func (h *RemoteServerHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/remote-servers", h.List)
	router.POST("/remote-servers", h.Create)
	router.GET("/remote-servers/:uuid", h.Get)
	router.PUT("/remote-servers/:uuid", h.Update)
	router.DELETE("/remote-servers/:uuid", h.Delete)
	router.POST("/remote-servers/test", h.TestConnectionCustom)
	router.POST("/remote-servers/:uuid/test", h.TestConnection)
}

// List retrieves all remote servers.
func (h *RemoteServerHandler) List(c *gin.Context) {
	enabledOnly := c.Query("enabled") == "true"

	servers, err := h.service.List(enabledOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, servers)
}

// Create creates a new remote server.
func (h *RemoteServerHandler) Create(c *gin.Context) {
	var server models.RemoteServer
	if err := c.ShouldBindJSON(&server); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	server.UUID = uuid.NewString()

	if err := h.service.Create(&server); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Immediate targeted uptime-monitor sync + check (spec §3.1.3).
	if h.uptimeService != nil {
		go h.uptimeService.SyncAndCheckForRemoteServer(server.ID)
	}

	// Send Notification
	if h.notificationService != nil {
		h.notificationService.SendExternal(c.Request.Context(),
			"remote_server",
			"Remote Server Added",
			"A new remote server was successfully added.",
			map[string]any{
				"Name":   util.SanitizeForLog(server.Name),
				"Host":   util.SanitizeForLog(server.Host),
				"Port":   server.Port,
				"Action": "created",
			},
		)
	}

	c.JSON(http.StatusCreated, server)
}

// Get retrieves a remote server by UUID.
func (h *RemoteServerHandler) Get(c *gin.Context) {
	uuidStr := c.Param("uuid")

	server, err := h.service.GetByUUID(uuidStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	c.JSON(http.StatusOK, server)
}

// Update updates an existing remote server.
func (h *RemoteServerHandler) Update(c *gin.Context) {
	uuidStr := c.Param("uuid")

	server, err := h.service.GetByUUID(uuidStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	if err := c.ShouldBindJSON(server); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.Update(server); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Keep the linked uptime monitor in sync with the edited fields.
	if h.uptimeService != nil {
		go func(id uint) {
			if syncErr := h.uptimeService.SyncMonitorForRemoteServer(id); syncErr != nil {
				logger.Log().WithError(syncErr).WithField("remote_server_id", id).
					Warn("failed to sync uptime monitor after remote server update")
			}
		}(server.ID)
	}

	c.JSON(http.StatusOK, server)
}

// Delete removes a remote server.
func (h *RemoteServerHandler) Delete(c *gin.Context) {
	uuidStr := c.Param("uuid")

	server, err := h.service.GetByUUID(uuidStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	// Clean up any linked uptime monitors before the server row goes away
	// (mirrors ProxyHostHandler.Delete).
	if h.uptimeService != nil {
		var monitors []models.UptimeMonitor
		if findErr := h.uptimeService.DB.Where("remote_server_id = ?", server.ID).Find(&monitors).Error; findErr == nil {
			for _, m := range monitors {
				_ = h.uptimeService.DeleteMonitor(m.ID)
			}
		}
	}

	if err := h.service.Delete(server.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Send Notification
	if h.notificationService != nil {
		h.notificationService.SendExternal(c.Request.Context(),
			"remote_server",
			"Remote Server Deleted",
			"A remote server was successfully deleted.",
			map[string]any{
				"Name":   util.SanitizeForLog(server.Name),
				"Action": "deleted",
			},
		)
	}

	c.JSON(http.StatusNoContent, nil)
}

// TestConnection tests the TCP connection to a remote server.
func (h *RemoteServerHandler) TestConnection(c *gin.Context) {
	uuidStr := c.Param("uuid")

	server, err := h.service.GetByUUID(uuidStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	// Test TCP connection with 5 second timeout
	address := net.JoinHostPort(server.Host, fmt.Sprintf("%d", server.Port))
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)

	result := gin.H{
		"server_uuid": server.UUID,
		"address":     address,
		"timestamp":   time.Now().UTC(),
	}

	if err != nil {
		result["reachable"] = false
		result["error"] = err.Error()

		// Update server reachability status
		server.Reachable = false
		now := time.Now().UTC()
		server.LastChecked = &now
		_ = h.service.Update(server)

		c.JSON(http.StatusOK, result)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Log().WithError(err).Warn("failed to close tcp connection")
		}
	}()

	// Connection successful
	result["reachable"] = true
	result["latency_ms"] = time.Since(time.Now()).Milliseconds()

	// Update server reachability status
	server.Reachable = true
	now := time.Now().UTC()
	server.LastChecked = &now
	_ = h.service.Update(server)

	c.JSON(http.StatusOK, result)
}

// TestConnectionCustom tests connectivity to a host/port provided in the body
func (h *RemoteServerHandler) TestConnectionCustom(c *gin.Context) {
	var req struct {
		Host string `json:"host" binding:"required"`
		Port int    `json:"port" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Test TCP connection with 5 second timeout
	address := net.JoinHostPort(req.Host, fmt.Sprintf("%d", req.Port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)

	result := gin.H{
		"address":   address,
		"timestamp": time.Now().UTC(),
	}

	if err != nil {
		result["reachable"] = false
		result["error"] = err.Error()
		c.JSON(http.StatusOK, result)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Log().WithError(err).Warn("failed to close tcp connection")
		}
	}()

	// Connection successful
	result["reachable"] = true
	result["latency_ms"] = time.Since(start).Milliseconds()

	c.JSON(http.StatusOK, result)
}
