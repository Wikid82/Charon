package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
)

// RemoteServerHandler handles HTTP requests for remote server management.
type RemoteServerHandler struct {
	service *services.RemoteServerService
}

// NewRemoteServerHandler creates a new remote server handler.
func NewRemoteServerHandler(db *gorm.DB) *RemoteServerHandler {
	return &RemoteServerHandler{
		service: services.NewRemoteServerService(db),
	}
}

// RegisterRoutes registers remote server routes.
func (h *RemoteServerHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/remote-servers", h.List)
	router.POST("/remote-servers", h.Create)
	router.GET("/remote-servers/:uuid", h.Get)
	router.PUT("/remote-servers/:uuid", h.Update)
	router.DELETE("/remote-servers/:uuid", h.Delete)
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

	c.JSON(http.StatusCreated, server)
}

// Get retrieves a remote server by UUID.
func (h *RemoteServerHandler) Get(c *gin.Context) {
	uuid := c.Param("uuid")

	server, err := h.service.GetByUUID(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	c.JSON(http.StatusOK, server)
}

// Update updates an existing remote server.
func (h *RemoteServerHandler) Update(c *gin.Context) {
	uuid := c.Param("uuid")

	server, err := h.service.GetByUUID(uuid)
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

	c.JSON(http.StatusOK, server)
}

// Delete removes a remote server.
func (h *RemoteServerHandler) Delete(c *gin.Context) {
	uuid := c.Param("uuid")

	server, err := h.service.GetByUUID(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	if err := h.service.Delete(server.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
