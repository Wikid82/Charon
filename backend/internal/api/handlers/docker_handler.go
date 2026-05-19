package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
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

type orthrusProxyResolver interface {
	GetProxyAddr(agentUUID string) (string, bool)
}

type DockerHandler struct {
	dockerService       dockerContainerLister
	remoteServerService remoteServerGetter
	orthrusResolver     orthrusProxyResolver
}

func NewDockerHandler(dockerService dockerContainerLister, remoteServerService remoteServerGetter) *DockerHandler {
	return &DockerHandler{
		dockerService:       dockerService,
		remoteServerService: remoteServerService,
	}
}

func (h *DockerHandler) SetOrthrusResolver(r orthrusProxyResolver) {
	// Guard against the Go typed-nil trap: a nil *T passed as an interface
	// produces a non-nil interface (type descriptor present, data nil), which
	// would bypass the h.orthrusResolver == nil guard and panic in GetProxyAddr.
	if r == nil || (reflect.ValueOf(r).Kind() == reflect.Ptr && reflect.ValueOf(r).IsNil()) {
		h.orthrusResolver = nil
		return
	}
	h.orthrusResolver = r
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
		switch server.ConnectionType {
		case models.ConnectionTypeOrthrus:
			if h.orthrusResolver == nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Orthrus subsystem unavailable"})
				return
			}
			if server.OrthrusAgentUUID == nil || *server.OrthrusAgentUUID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Remote server has no Orthrus agent UUID configured"})
				return
			}
			proxyAddr, ok := h.orthrusResolver.GetProxyAddr(*server.OrthrusAgentUUID)
			if !ok {
				c.JSON(http.StatusBadGateway, gin.H{"error": "Orthrus agent is not currently connected"})
				return
			}
			host = "tcp://" + proxyAddr
		default:
			host = fmt.Sprintf("tcp://%s:%d", server.Host, server.Port)
		}
	}

	containers, err := h.dockerService.ListContainers(c.Request.Context(), host)
	if err != nil {
		var unavailableErr *services.DockerUnavailableError
		if errors.As(err, &unavailableErr) {
			details := unavailableErr.Details()
			if details == "" {
				details = "Cannot connect to Docker. Please ensure Docker is running and the socket is accessible (e.g., /var/run/docker.sock is mounted)."
			}
			log.WithFields(map[string]any{"server_id": util.SanitizeForLog(serverID), "host": util.SanitizeForLog(host), "error": util.SanitizeForLog(err.Error())}).Warn("docker unavailable")
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "Docker daemon unavailable",
				"details": details,
			})
			return
		}

		log.WithFields(map[string]any{"server_id": util.SanitizeForLog(serverID), "host": util.SanitizeForLog(host), "error": util.SanitizeForLog(err.Error())}).Error("failed to list containers")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list containers"})
		return
	}

	c.JSON(http.StatusOK, containers)
}
