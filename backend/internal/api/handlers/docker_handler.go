package handlers

import (
	"fmt"
	"net/http"

	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
)

type DockerHandler struct {
	dockerService       *services.DockerService
	remoteServerService *services.RemoteServerService
}

func NewDockerHandler(dockerService *services.DockerService, remoteServerService *services.RemoteServerService) *DockerHandler {
	return &DockerHandler{
		dockerService:       dockerService,
		remoteServerService: remoteServerService,
	}
}

func (h *DockerHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/docker/containers", h.ListContainers)
}

func (h *DockerHandler) ListContainers(c *gin.Context) {
	host := c.Query("host")
	serverID := c.Query("server_id")

	// If server_id is provided, look up the remote server
	if serverID != "" {
		server, err := h.remoteServerService.GetByUUID(serverID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Remote server not found"})
			return
		}

		// Construct Docker host string
		// Assuming TCP for now as that's what RemoteServer supports (Host/Port)
		// TODO: Support SSH if/when RemoteServer supports it
		host = fmt.Sprintf("tcp://%s:%d", server.Host, server.Port)
	}

	containers, err := h.dockerService.ListContainers(c.Request.Context(), host)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list containers: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, containers)
}
