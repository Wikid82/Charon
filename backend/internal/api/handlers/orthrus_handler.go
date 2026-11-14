package handlers

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
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
// invalidExternalProxyBindMessage is the operator-facing text for a rejected
// listen setting.
const invalidExternalProxyBindMessage = "Listen address must be automatic, all interfaces, an address or a subnet"

type OrthrusHandler struct {
	svc             *services.OrthrusService
	proxyResolver   orthrusProxyStatusResolver
	securityService *services.SecurityService
	// localNetworks reports the host networks Charon can bind; a field so tests
	// can stub the host view.
	localNetworks func() ([]orthrus.LocalNetwork, netip.Addr, orthrus.BindSource)
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
	return &OrthrusHandler{
		svc:             orthrsuSvc,
		securityService: securityService,
		localNetworks:   orthrus.LocalNetworks,
	}
}

// SetProxyResolver wires a live OrthrusServer so that GetProxyStatus can
// return real-time external proxy state for connected agents.
func (h *OrthrusHandler) SetProxyResolver(r orthrusProxyStatusResolver) {
	if r != nil {
		rv := reflect.ValueOf(r)
		if rv.Kind() == reflect.Pointer && rv.IsNil() {
			h.proxyResolver = nil
			return
		}
	}
	h.proxyResolver = r
}

// RegisterRoutes wires the Orthrus agent endpoints. The agent list/summary
// reads that back the role=user-reachable proxy-host create/edit flow are
// registered on read; agent provisioning, mutation, revocation and the
// detail endpoints (install snippets embed a bootstrap token; proxy-status
// exposes agent internals) are registered on admin (deny-by-default for
// role=user). Callers that do not need the split may pass the same group for
// both.
func (h *OrthrusHandler) RegisterRoutes(read, admin *gin.RouterGroup) {
	read.GET("/orthrus/agents", h.List)
	read.GET("/orthrus/agents/:uuid", h.Get)

	admin.POST("/orthrus/agents", h.Provision)
	admin.PATCH("/orthrus/agents/:uuid", h.Patch)
	admin.DELETE("/orthrus/agents/:uuid", h.Delete)
	admin.POST("/orthrus/agents/:uuid/revoke", h.Revoke)
	admin.POST("/orthrus/agents/:uuid/rotate-key", h.RotateKey)
	admin.GET("/orthrus/agents/:uuid/snippets", h.GetInstallSnippets)
	admin.GET("/orthrus/agents/:uuid/proxy-status", h.GetProxyStatus)
	admin.GET("/orthrus/local-networks", h.LocalNetworks)
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
	ExternalProxyBind *string `json:"external_proxy_bind"`
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
	agent, err := h.svc.Patch(uuid, services.AgentPatch{
		Name:              req.Name,
		HecateTunnelUUID:  req.HecateTunnelUUID,
		DeviceID:          req.DeviceID,
		ResolvedAddress:   req.ResolvedAddress,
		ExternalProxyPort: req.ExternalProxyPort,
		ExternalProxyBind: req.ExternalProxyBind,
		WriteEnabled:      req.WriteEnabled,
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		} else if errors.Is(err, services.ErrInvalidExternalProxyBind) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":      invalidExternalProxyBindMessage,
				"error_code": "external_proxy_bind_invalid",
			})
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
			Actor:         auditActor(c),
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

// RotateKey replaces an agent's auth key and returns the new key exactly once.
// The response is never cached. A security audit entry records the rotation
// without the key.
func (h *OrthrusHandler) RotateKey(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	uuid := c.Param("uuid")
	authKey, err := h.svc.RotateKey(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	if h.securityService != nil {
		agentName := ""
		if agent, getErr := h.svc.Get(uuid); getErr == nil {
			agentName = agent.Name
		}
		_ = h.securityService.LogAudit(&models.SecurityAudit{
			Actor:         auditActor(c),
			Action:        "orthrus_key_rotated",
			EventCategory: "orthrus_agent",
			ResourceUUID:  uuid,
			Details:       fmt.Sprintf(`{"agent_name":%q}`, agentName),
		})
	}

	c.JSON(http.StatusOK, gin.H{"auth_key": authKey})
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
		"configured_bind":          agent.ExternalProxyBind,
		"configured_write_enabled": agent.WriteEnabled,
		"active_write_enabled":     false,
		"active":                   false,
		"active_port":              0,
		"active_bind":              "",
		"bind_mode":                "",
		"bind_fallback":            false,
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
			resp["active_bind"] = status.ActiveBind
			resp["bind_mode"] = string(status.BindMode)
			resp["bind_fallback"] = status.BindFallback
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

// localNetworksResponse is the payload of GET /orthrus/local-networks.
type localNetworksResponse struct {
	Auto     localNetworksAuto      `json:"auto"`
	Networks []orthrus.LocalNetwork `json:"networks"`
}

// localNetworksAuto is the address the automatic listen setting resolves to.
type localNetworksAuto struct {
	Address string             `json:"address"`
	Source  orthrus.BindSource `json:"source"`
}

// LocalNetworks lists the host networks the external proxy can listen on, with
// the address the automatic setting would choose.
func (h *OrthrusHandler) LocalNetworks(c *gin.Context) {
	networks, auto, source := h.localNetworks()
	if networks == nil {
		networks = []orthrus.LocalNetwork{}
	}
	c.JSON(http.StatusOK, localNetworksResponse{
		Auto:     localNetworksAuto{Address: auto.String(), Source: source},
		Networks: networks,
	})
}
