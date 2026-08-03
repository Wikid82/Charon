package handlers

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"reflect"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/orthrus"
	"github.com/Wikid82/charon/backend/internal/services"
)

// orthrusProxyStatusResolver is satisfied by *orthrus.OrthrusServer.
type orthrusProxyStatusResolver interface {
	GetExternalProxyStatus(agentUUID string) (orthrus.ExternalProxyStatus, bool)
}

// OrthrusHandler handles REST requests for Orthrus agent management.
type OrthrusHandler struct {
	svc             *services.OrthrusService
	proxyResolver   orthrusProxyStatusResolver
	securityService *services.SecurityService
}

// NewOrthrusHandler creates an OrthrusHandler backed by the given service.
// securityService is used to emit an audit entry whenever an operator
// toggles an agent's write-mode flag (see Patch) — a separate, handler-level
// concern from the per-request write-path audit entries Muzzle emits
// directly via the AuditLogger interface (see muzzle.go), following this
// codebase's existing convention of logging admin-initiated audit events at
// the handler layer rather than inside the service (compare
// security_handler.go, crowdsec_handler.go).
func NewOrthrusHandler(orthrsuSvc *services.OrthrusService, securityService *services.SecurityService) *OrthrusHandler {
	return &OrthrusHandler{svc: orthrsuSvc, securityService: securityService}
}

// SetProxyResolver wires a live OrthrusServer so that GetProxyStatus can
// return real-time external proxy state for connected agents.
func (h *OrthrusHandler) SetProxyResolver(r orthrusProxyStatusResolver) {
	if r != nil {
		rv := reflect.ValueOf(r)
		if rv.Kind() == reflect.Ptr && rv.IsNil() {
			h.proxyResolver = nil
			return
		}
	}
	h.proxyResolver = r
}

// RegisterRoutes wires all Orthrus management routes onto the given router group.
func (h *OrthrusHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/orthrus/agents", h.List)
	rg.POST("/orthrus/agents", h.Provision)
	rg.GET("/orthrus/agents/:uuid", h.Get)
	rg.PATCH("/orthrus/agents/:uuid", h.Patch)
	rg.DELETE("/orthrus/agents/:uuid", h.Delete)
	rg.POST("/orthrus/agents/:uuid/revoke", h.Revoke)
	rg.GET("/orthrus/agents/:uuid/snippets", h.GetInstallSnippets)
	rg.GET("/orthrus/agents/:uuid/proxy-status", h.GetProxyStatus)
}

// List returns all registered Orthrus agents.
func (h *OrthrusHandler) List(c *gin.Context) {
	agents, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, agents)
}

// provisionRequest is the payload for registering a new agent.
type provisionRequest struct {
	Name string `json:"name" binding:"required"`
}

// Provision creates a new Orthrus agent and returns the auth key exactly once.
func (h *OrthrusHandler) Provision(c *gin.Context) {
	var req provisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	agent, plainKey, err := h.svc.Provision(req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"agent":    agent,
		"auth_key": plainKey,
	})
}

// Get retrieves a single Orthrus agent by UUID.
func (h *OrthrusHandler) Get(c *gin.Context) {
	uuid := c.Param("uuid")
	agent, err := h.svc.Get(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, agent)
}

// patchAgentRequest is the payload for partially updating an agent.
type patchAgentRequest struct {
	Name              *string `json:"name"`
	HecateTunnelUUID  *string `json:"hecate_tunnel_uuid"`
	DeviceID          *string `json:"device_id"`
	ResolvedAddress   *string `json:"resolved_address"`
	ExternalProxyPort *int    `json:"external_proxy_port"`
	WriteEnabled      *bool   `json:"write_enabled"`
}

// Patch applies a partial update to an Orthrus agent. If WriteEnabled is
// present in the request, this is the one field on this endpoint whose
// change is itself security-relevant enough to warrant its own audit entry
// (distinct from, and in addition to, the per-write-request audit entries
// Muzzle emits directly for actual proxied traffic — see muzzle.go).
func (h *OrthrusHandler) Patch(c *gin.Context) {
	uuid := c.Param("uuid")
	var req patchAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	agent, err := h.svc.Patch(uuid, req.Name, req.HecateTunnelUUID, req.DeviceID, req.ResolvedAddress, req.ExternalProxyPort, req.WriteEnabled)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	if req.WriteEnabled != nil && h.securityService != nil {
		action := "orthrus_write_disabled"
		if *req.WriteEnabled {
			action = "orthrus_write_enabled"
		}
		_ = h.securityService.LogAudit(&models.SecurityAudit{
			Actor:         actorFromContext(c),
			Action:        action,
			EventCategory: "orthrus_write",
			ResourceUUID:  uuid,
			Details:       fmt.Sprintf(`{"agent_name":%q}`, agent.Name),
		})
	}

	c.JSON(http.StatusOK, agent)
}

// Delete removes an Orthrus agent from the database.
func (h *OrthrusHandler) Delete(c *gin.Context) {
	uuid := c.Param("uuid")
	if err := h.svc.Delete(uuid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// Revoke invalidates the auth key for an agent and disconnects its active session.
func (h *OrthrusHandler) Revoke(c *gin.Context) {
	uuid := c.Param("uuid")
	if err := h.svc.Revoke(uuid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "revoked"})
}

// GetInstallSnippets returns platform install templates for an agent.
// The real auth key is never present in snippets; the placeholder <AUTH_KEY>
// must be replaced by the user with the value returned at Provision time.
func (h *OrthrusHandler) GetInstallSnippets(c *gin.Context) {
	uuid := c.Param("uuid")

	charonURL := c.GetHeader("X-Charon-URL")
	if charonURL == "" {
		// NOTE: TLS detection via c.Request.TLS is unreliable when Charon runs behind a
		// reverse proxy (e.g., Caddy) that terminates TLS and strips or rewrites headers.
		// The X-Charon-URL header allows callers to pass the correct public URL explicitly;
		// if absent, we fall back to heuristic detection. Users deploying behind a proxy
		// should set the X-Charon-URL header from the frontend (window.location.origin).
		scheme := "https"
		if c.Request.TLS == nil {
			scheme = "http"
		}
		charonURL = scheme + "://" + c.Request.Host
	}

	snippets, err := h.svc.GetInstallSnippets(uuid, charonURL)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snippets)
}

// resolveExternalProxyHost determines the hostname third-party tools should use
// to reach this Charon instance's external Docker proxy ports. It mirrors the
// X-Charon-URL header pattern used by GetInstallSnippets, but — unlike that
// handler — returns a bare hostname only (no scheme, no port): the external
// proxy's TCP port is independent of Charon's own web port, so the docker
// port is appended separately by the caller.
func resolveExternalProxyHost(c *gin.Context) string {
	if raw := c.GetHeader("X-Charon-URL"); raw != "" {
		if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
			return u.Hostname()
		}
	}
	if host, _, err := net.SplitHostPort(c.Request.Host); err == nil {
		return host
	}
	return c.Request.Host
}

// GetProxyStatus returns the runtime external Docker proxy state for an agent.
// 404 when the agent is not found in the database. When the agent exists but
// is not currently connected, agent_online is false and live fields are zero.
func (h *OrthrusHandler) GetProxyStatus(c *gin.Context) {
	uuid := c.Param("uuid")
	agent, err := h.svc.Get(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}
	resp := gin.H{
		"agent_uuid":               agent.UUID,
		"agent_online":             false,
		"configured_port":          agent.ExternalProxyPort,
		"configured_write_enabled": agent.WriteEnabled,
		"active_write_enabled":     false,
		"active":                   false,
		"active_port":              0,
		"bind_address":             "",
		"connection_string":        "",
		"error":                    "",
	}
	if h.proxyResolver != nil {
		if status, ok := h.proxyResolver.GetExternalProxyStatus(uuid); ok {
			resp["agent_online"] = true
			resp["active"] = status.Active
			resp["active_port"] = status.ActivePort
			resp["bind_address"] = status.BoundAddress
			resp["active_write_enabled"] = status.WriteEnabled
			if status.Active && status.ActivePort > 0 {
				resp["connection_string"] = fmt.Sprintf("tcp://%s:%d", resolveExternalProxyHost(c), status.ActivePort)
			}
			if status.Error != "" {
				resp["error"] = status.Error
			}
		}
	}
	c.JSON(http.StatusOK, resp)
}
