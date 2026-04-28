package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	cfprovider "github.com/Wikid82/charon/backend/internal/hecate/providers/cloudflare"
	nbprovider "github.com/Wikid82/charon/backend/internal/hecate/providers/netbird"
	tsprovider "github.com/Wikid82/charon/backend/internal/hecate/providers/tailscale"
	ztprovider "github.com/Wikid82/charon/backend/internal/hecate/providers/zerotier"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// HecateHandler handles REST requests for tunnel configuration and provider data.
type HecateHandler struct {
	svc *services.HecateService
}

// NewHecateHandler creates a HecateHandler backed by the given service.
func NewHecateHandler(svc *services.HecateService) *HecateHandler {
	return &HecateHandler{svc: svc}
}

// RegisterRoutes wires all Hecate management routes onto the given router group.
func (h *HecateHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/hecate/status", h.GetStatus)
	rg.GET("/hecate/tunnels", h.List)
	rg.POST("/hecate/tunnels", h.Create)
	rg.GET("/hecate/tunnels/:uuid", h.Get)
	rg.PUT("/hecate/tunnels/:uuid", h.Update)
	rg.DELETE("/hecate/tunnels/:uuid", h.Delete)
	rg.POST("/hecate/tunnels/:uuid/start", h.Start)
	rg.POST("/hecate/tunnels/:uuid/stop", h.Stop)
	rg.POST("/hecate/tunnels/:uuid/rotate-credentials", h.RotateCredentials)
	rg.GET("/hecate/cloudflare/tunnels", h.ListCloudflareTunnels)
	rg.GET("/hecate/tunnels/:uuid/config/cloudflared", h.GetCloudflaredConfig)
	rg.GET("/hecate/tailscale/devices", h.ListTailscaleDevices)
	rg.POST("/hecate/tailscale/sync", h.SyncTailscale)
	rg.GET("/hecate/zerotier/networks", h.ListZeroTierNetworks)
	rg.GET("/hecate/zerotier/networks/:network_id/members", h.ListZeroTierMembers)
	rg.GET("/hecate/netbird/peers", h.ListNetBirdPeers)
	rg.POST("/hecate/netbird/sync", h.SyncNetBird)
}

// GetStatus returns the runtime status of all managed tunnels.
func (h *HecateHandler) GetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.GetStatus())
}

// List returns all TunnelConfig records.
func (h *HecateHandler) List(c *gin.Context) {
	configs, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, configs)
}

// createRequest is the payload for creating a new tunnel configuration.
type createRequest struct {
	Name        string                    `json:"name" binding:"required"`
	Provider    models.TunnelProviderType `json:"provider" binding:"required,oneof=cloudflare tailscale zerotier netbird"`
	Credentials string                    `json:"credentials" binding:"required"`
	Config      string                    `json:"configuration"`
	IsActive    bool                      `json:"is_active"`
}

// Create persists a new TunnelConfig.
func (h *HecateHandler) Create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := &models.TunnelConfig{
		Name:          req.Name,
		Provider:      req.Provider,
		Configuration: req.Config,
		IsActive:      req.IsActive,
	}

	if err := h.svc.Create(cfg, req.Credentials); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, cfg)
}

// Get retrieves a single TunnelConfig by UUID.
func (h *HecateHandler) Get(c *gin.Context) {
	uuid := c.Param("uuid")
	cfg, err := h.svc.Get(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "tunnel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// updateRequest is the payload for updating an existing tunnel configuration.
type updateRequest struct {
	Name        string                    `json:"name" binding:"required"`
	Provider    models.TunnelProviderType `json:"provider" binding:"required,oneof=cloudflare tailscale zerotier netbird"`
	Credentials *string                   `json:"credentials"`
	Config      string                    `json:"configuration"`
	IsActive    bool                      `json:"is_active"`
}

// Update applies changes to an existing TunnelConfig.
func (h *HecateHandler) Update(c *gin.Context) {
	uuid := c.Param("uuid")
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := &models.TunnelConfig{
		Name:          req.Name,
		Provider:      req.Provider,
		Configuration: req.Config,
		IsActive:      req.IsActive,
	}

	if err := h.svc.Update(uuid, cfg, req.Credentials); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "tunnel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// Delete stops and removes a TunnelConfig.
func (h *HecateHandler) Delete(c *gin.Context) {
	uuid := c.Param("uuid")
	if err := h.svc.Delete(uuid); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "tunnel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// Start activates the tunnel for a given TunnelConfig UUID.
func (h *HecateHandler) Start(c *gin.Context) {
	uuid := c.Param("uuid")
	if err := h.svc.GetManager().StartTunnel(uuid); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "tunnel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "started"})
}

// Stop deactivates the tunnel for a given TunnelConfig UUID.
func (h *HecateHandler) Stop(c *gin.Context) {
	uuid := c.Param("uuid")
	if err := h.svc.GetManager().StopTunnel(uuid); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "tunnel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "stopped"})
}

// rotateCredentialsRequest is the payload for credential rotation.
type rotateCredentialsRequest struct {
	Credentials string `json:"credentials" binding:"required"`
}

// RotateCredentials replaces the credentials for a running tunnel.
func (h *HecateHandler) RotateCredentials(c *gin.Context) {
	uuid := c.Param("uuid")
	var req rotateCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.RotateCredentials(uuid, req.Credentials); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "tunnel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "credentials rotated"})
}

// ListCloudflareTunnels proxies a ListTunnels call to the active Cloudflare provider.
func (h *HecateHandler) ListCloudflareTunnels(c *gin.Context) {
	p, ok := h.svc.GetManager().GetProviderByType(models.ProviderCloudflare)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no active cloudflare provider"})
		return
	}
	cf, ok := p.(*cfprovider.CloudflareTunnelProvider)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected provider type"})
		return
	}
	client := cf.GetClient()
	if client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cloudflare client not initialized"})
		return
	}
	tunnels, err := client.ListTunnels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tunnels)
}

// GetCloudflaredConfig returns a cloudflared YAML configuration template for a tunnel.
func (h *HecateHandler) GetCloudflaredConfig(c *gin.Context) {
	uuid := c.Param("uuid")
	cfg, err := h.svc.Get(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if cfg.Provider != models.ProviderCloudflare {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tunnel is not a cloudflare provider"})
		return
	}
	yaml, err := cfprovider.GenerateCloudflaredConfig(uuid, "/etc/cloudflared/credentials.json", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "text/yaml; charset=utf-8", []byte(yaml))
}

// ListTailscaleDevices proxies a ListDevices call to the active Tailscale provider.
func (h *HecateHandler) ListTailscaleDevices(c *gin.Context) {
	p, ok := h.svc.GetManager().GetProviderByType(models.ProviderTailscale)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no active tailscale provider"})
		return
	}
	ts, ok := p.(*tsprovider.TailscaleProvider)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected provider type"})
		return
	}
	client := ts.GetClient()
	if client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "tailscale client not initialized"})
		return
	}
	devices, err := client.ListDevices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, devices)
}

// SyncTailscale forces a cache refresh of the Tailscale device list.
func (h *HecateHandler) SyncTailscale(c *gin.Context) {
	p, ok := h.svc.GetManager().GetProviderByType(models.ProviderTailscale)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no active tailscale provider"})
		return
	}
	ts, ok := p.(*tsprovider.TailscaleProvider)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected provider type"})
		return
	}
	client := ts.GetClient()
	if client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "tailscale client not initialized"})
		return
	}
	devices, err := client.ForceRefresh(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, devices)
}

// ListZeroTierNetworks proxies a ListNetworks call to the active ZeroTier provider.
func (h *HecateHandler) ListZeroTierNetworks(c *gin.Context) {
	p, ok := h.svc.GetManager().GetProviderByType(models.ProviderZeroTier)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no active zerotier provider"})
		return
	}
	zt, ok := p.(*ztprovider.ZeroTierProvider)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected provider type"})
		return
	}
	client := zt.GetClient()
	if client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "zerotier client not initialized"})
		return
	}
	networks, err := client.ListNetworks(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, networks)
}

// ListZeroTierMembers proxies a ListMembers call to the active ZeroTier provider.
func (h *HecateHandler) ListZeroTierMembers(c *gin.Context) {
	networkID := c.Param("network_id")
	p, ok := h.svc.GetManager().GetProviderByType(models.ProviderZeroTier)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no active zerotier provider"})
		return
	}
	zt, ok := p.(*ztprovider.ZeroTierProvider)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected provider type"})
		return
	}
	client := zt.GetClient()
	if client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "zerotier client not initialized"})
		return
	}
	members, err := client.ListMembers(c.Request.Context(), networkID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, members)
}

// ListNetBirdPeers proxies a ListPeers call to the active NetBird provider.
func (h *HecateHandler) ListNetBirdPeers(c *gin.Context) {
	p, ok := h.svc.GetManager().GetProviderByType(models.ProviderNetBird)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no active netbird provider"})
		return
	}
	nb, ok := p.(*nbprovider.NetBirdProvider)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected provider type"})
		return
	}
	client := nb.GetClient()
	if client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "netbird client not initialized"})
		return
	}
	peers, err := client.ListPeers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, peers)
}

// SyncNetBird forces a cache refresh of the NetBird peer list.
func (h *HecateHandler) SyncNetBird(c *gin.Context) {
	p, ok := h.svc.GetManager().GetProviderByType(models.ProviderNetBird)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no active netbird provider"})
		return
	}
	nb, ok := p.(*nbprovider.NetBirdProvider)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unexpected provider type"})
		return
	}
	client := nb.GetClient()
	if client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "netbird client not initialized"})
		return
	}
	peers, err := client.ForceRefresh(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, peers)
}
