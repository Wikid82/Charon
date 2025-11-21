package handlers

import (
	"net/http"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
	"github.com/gin-gonic/gin"
)

type DockerHandler struct {
	dockerService *services.DockerService
}

func NewDockerHandler(dockerService *services.DockerService) *DockerHandler {
	return &DockerHandler{dockerService: dockerService}
}

func (h *DockerHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/docker/containers", h.ListContainers)
}

func (h *DockerHandler) ListContainers(c *gin.Context) {
	host := c.Query("host")
	containers, err := h.dockerService.ListContainers(c.Request.Context(), host)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list containers: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, containers)
}
