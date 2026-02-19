package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
	"github.com/gin-gonic/gin"
)

type dockerContainerLister interface {
	ListContainers(ctx context.Context, host string) ([]services.DockerContainer, error)
}

type remoteServerGetter interface {
	GetByUUID(uuidStr string) (*models.RemoteServer, error)
}

type DockerHandler struct {
	dockerService       dockerContainerLister
	remoteServerService remoteServerGetter
}

func NewDockerHandler(dockerService dockerContainerLister, remoteServerService remoteServerGetter) *DockerHandler {
	return &DockerHandler{
		dockerService:       dockerService,
		remoteServerService: remoteServerService,
	}
}

func (h *DockerHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/docker/containers", h.ListContainers)
}

func (h *DockerHandler) ListContainers(c *gin.Context) {
	log := middleware.GetRequestLogger(c)

	host := strings.TrimSpace(c.Query("host"))
	serverID := strings.TrimSpace(c.Query("server_id"))

	// SSRF hardening: do not accept arbitrary host values from the client.
	// Only allow explicit local selection ("local") or empty (default local).
	if host != "" && host != "local" {
		log.WithFields(map[string]any{"host": util.SanitizeForLog(host)}).Warn("rejected docker host query param")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid docker host selector"})
		return
	}

	// If server_id is provided, look up the remote server
	if serverID != "" {
		server, err := h.remoteServerService.GetByUUID(serverID)
		if err != nil {
			log.WithFields(map[string]any{"server_id": util.SanitizeForLog(serverID)}).Warn("remote server not found")
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
		var unavailableErr *services.DockerUnavailableError
		if errors.As(err, &unavailableErr) {
			log.WithFields(map[string]any{"server_id": util.SanitizeForLog(serverID), "host": util.SanitizeForLog(host), "error": util.SanitizeForLog(err.Error())}).Warn("docker unavailable")
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "Docker daemon unavailable",
				"details": "Cannot connect to Docker. Please ensure Docker is running and the socket is accessible (e.g., /var/run/docker.sock is mounted).",
			})
			return
		}

		log.WithFields(map[string]any{"server_id": util.SanitizeForLog(serverID), "host": util.SanitizeForLog(host), "error": util.SanitizeForLog(err.Error())}).Error("failed to list containers")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list containers"})
		return
	}

	c.JSON(http.StatusOK, containers)
}
