package handlers

import (
	"context"
	"errors"
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
	connectionTestGate
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

// RegisterRoutes wires the remote-server endpoints. The list/detail reads that
// back the role=user-reachable proxy-host create/edit flow and the Remote
// Servers page are registered on read; create/update/delete and the SSH
// connection-test endpoints (they act on stored SSH targets and credentials)
// are registered on admin (deny-by-default for role=user). Callers that do not
// need the split may pass the same group for both.
func (h *RemoteServerHandler) RegisterRoutes(read, admin *gin.RouterGroup) {
	read.GET("/remote-servers", h.List)
	read.GET("/remote-servers/:uuid", h.Get)

	admin.POST("/remote-servers", h.Create)
	admin.PUT("/remote-servers/:uuid", h.Update)
	admin.DELETE("/remote-servers/:uuid", h.Delete)
	admin.POST("/remote-servers/test", h.TestConnectionCustom)
	admin.POST("/remote-servers/:uuid/test", h.TestConnection)
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
				// Safe: remote_server_id is a uint (DB/route numeric ID), not an injectable string.
				// codeql[go/log-injection]
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

	// Purge linked uptime monitors, THEN delete the RemoteServer row, both under
	// the per-server "remote-<id>" lock RemoveForRemoteServer holds internally.
	// Create fires `go SyncAndCheckForRemoteServer(id)`, so a monitor sync may
	// be in flight: the lock waits out any sync already running (the purge then
	// removes the monitor it created), and a sync that takes the lock after the
	// row is gone hits its "remote server not found" guard and creates nothing.
	// Purge-before-parent-delete keeps the two delete paths identical to the
	// proxy-host one, where that order is required by a real FK.
	var delErr error
	if h.uptimeService != nil {
		delErr = h.uptimeService.RemoveForRemoteServer(server.ID, func() error {
			return h.service.Delete(server.ID)
		})
	} else {
		delErr = h.service.Delete(server.ID)
	}
	if delErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": delErr.Error()})
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

// runRemoteDialTest runs the remote server connection test. It writes the 400 for a
// refused destination and the 500 for a failed check, and reports whether the
// response was written. A failed connection is not an error here: it is returned
// as the dial error with its fixed text, and the caller reports reachable false.
// remoteDialOutcome is the result of a remote server connection test that ran.
type remoteDialOutcome struct {
	latency time.Duration
	err     error
}

func runRemoteDialTest(c *gin.Context, service *services.RemoteServerService, host string, port int) (remoteDialOutcome, bool) {
	ctx := context.Background()
	if c.Request != nil {
		ctx = c.Request.Context()
	}
	latency, err := service.TestDial(ctx, host, port)
	switch {
	case errors.Is(err, services.ErrDestinationRefused):
		logConnectionTest(c, "refused")
		respondDestinationRefused(c)
		return remoteDialOutcome{}, true
	case errors.Is(err, services.ErrConnectionTestFailed):
		logConnectionTest(c, "failed")
	case err != nil:
		logger.Log().WithError(err).Warn("remote server connection test could not run")
		c.JSON(http.StatusInternalServerError, gin.H{"error": connectionTestErrorText})
		return remoteDialOutcome{}, true
	default:
		logConnectionTest(c, "ok")
	}
	return remoteDialOutcome{latency: latency, err: err}, false
}

// TestConnection tests the TCP connection to a remote server.
func (h *RemoteServerHandler) TestConnection(c *gin.Context) {
	if !h.allowConnectionTest(c) {
		return
	}
	uuidStr := c.Param("uuid")

	server, err := h.service.GetByUUID(uuidStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	address := net.JoinHostPort(server.Host, fmt.Sprintf("%d", server.Port))
	dial, written := runRemoteDialTest(c, h.service, server.Host, server.Port)
	if written {
		return
	}

	result := gin.H{
		"server_uuid": server.UUID,
		"address":     address,
		"timestamp":   time.Now().UTC(),
	}

	// Update server reachability status
	now := time.Now().UTC()
	server.LastChecked = &now
	if dial.err != nil {
		result["reachable"] = false
		result["error"] = connectionTestFailedText
		server.Reachable = false
		_ = h.service.Update(server)
		c.JSON(http.StatusOK, result)
		return
	}

	result["reachable"] = true
	result["latency_ms"] = dial.latency.Milliseconds()
	server.Reachable = true
	_ = h.service.Update(server)

	c.JSON(http.StatusOK, result)
}

// TestConnectionCustom tests connectivity to a host/port provided in the body
func (h *RemoteServerHandler) TestConnectionCustom(c *gin.Context) {
	if !h.allowConnectionTest(c) {
		return
	}
	var req struct {
		Host string `json:"host" binding:"required"`
		Port int    `json:"port" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	address := net.JoinHostPort(req.Host, fmt.Sprintf("%d", req.Port))
	dial, written := runRemoteDialTest(c, h.service, req.Host, req.Port)
	if written {
		return
	}

	result := gin.H{
		"address":   address,
		"timestamp": time.Now().UTC(),
	}
	if dial.err != nil {
		result["reachable"] = false
		result["error"] = connectionTestFailedText
		c.JSON(http.StatusOK, result)
		return
	}

	result["reachable"] = true
	result["latency_ms"] = dial.latency.Milliseconds()
	c.JSON(http.StatusOK, result)
}
