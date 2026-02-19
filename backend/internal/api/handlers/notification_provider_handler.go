package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
)

type NotificationProviderHandler struct {
	service         *services.NotificationService
	securityService *services.SecurityService
	dataRoot        string
}

type notificationProviderUpsertRequest struct {
	Name                string `json:"name"`
	Type                string `json:"type"`
	URL                 string `json:"url"`
	Config              string `json:"config"`
	Template            string `json:"template"`
	Enabled             bool   `json:"enabled"`
	NotifyProxyHosts    bool   `json:"notify_proxy_hosts"`
	NotifyRemoteServers bool   `json:"notify_remote_servers"`
	NotifyDomains       bool   `json:"notify_domains"`
	NotifyCerts         bool   `json:"notify_certs"`
	NotifyUptime        bool   `json:"notify_uptime"`
}

func (r notificationProviderUpsertRequest) toModel() models.NotificationProvider {
	return models.NotificationProvider{
		Name:                r.Name,
		Type:                r.Type,
		URL:                 r.URL,
		Config:              r.Config,
		Template:            r.Template,
		Enabled:             r.Enabled,
		NotifyProxyHosts:    r.NotifyProxyHosts,
		NotifyRemoteServers: r.NotifyRemoteServers,
		NotifyDomains:       r.NotifyDomains,
		NotifyCerts:         r.NotifyCerts,
		NotifyUptime:        r.NotifyUptime,
	}
}

func NewNotificationProviderHandler(service *services.NotificationService) *NotificationProviderHandler {
	return NewNotificationProviderHandlerWithDeps(service, nil, "")
}

func NewNotificationProviderHandlerWithDeps(service *services.NotificationService, securityService *services.SecurityService, dataRoot string) *NotificationProviderHandler {
	return &NotificationProviderHandler{service: service, securityService: securityService, dataRoot: dataRoot}
}

func (h *NotificationProviderHandler) List(c *gin.Context) {
	providers, err := h.service.ListProviders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list providers"})
		return
	}
	c.JSON(http.StatusOK, providers)
}

func (h *NotificationProviderHandler) Create(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var req notificationProviderUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	provider := req.toModel()
	// Server-managed migration fields are set by the migration reconciliation logic
	// and must not be set from user input
	provider.Engine = ""
	provider.MigrationState = ""
	provider.MigrationError = ""
	provider.LastMigratedAt = nil
	provider.LegacyURL = ""

	if err := h.service.CreateProvider(&provider); err != nil {
		// If it's a validation error from template parsing, return 400
		if isProviderValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if respondPermissionError(c, h.securityService, "notification_provider_save_failed", err, h.dataRoot) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create provider"})
		return
	}
	c.JSON(http.StatusCreated, provider)
}

func (h *NotificationProviderHandler) Update(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	id := c.Param("id")
	var req notificationProviderUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	provider := req.toModel()
	provider.ID = id
	// Server-managed migration fields must not be modified via user input
	provider.Engine = ""
	provider.MigrationState = ""
	provider.MigrationError = ""
	provider.LastMigratedAt = nil
	provider.LegacyURL = ""

	if err := h.service.UpdateProvider(&provider); err != nil {
		if isProviderValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if respondPermissionError(c, h.securityService, "notification_provider_save_failed", err, h.dataRoot) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update provider"})
		return
	}
	c.JSON(http.StatusOK, provider)
}

func isProviderValidationError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	return strings.Contains(errMsg, "invalid custom template") ||
		strings.Contains(errMsg, "rendered template") ||
		strings.Contains(errMsg, "failed to parse template") ||
		strings.Contains(errMsg, "failed to render template") ||
		strings.Contains(errMsg, "invalid Discord webhook URL")
}

func (h *NotificationProviderHandler) Delete(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	id := c.Param("id")
	if err := h.service.DeleteProvider(id); err != nil {
		if respondPermissionError(c, h.securityService, "notification_provider_delete_failed", err, h.dataRoot) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete provider"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Provider deleted"})
}

func (h *NotificationProviderHandler) Test(c *gin.Context) {
	var provider models.NotificationProvider
	if err := c.ShouldBindJSON(&provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.TestProvider(provider); err != nil {
		// Create internal notification for the failure
		_, _ = h.service.Create(models.NotificationTypeError, "Test Failed", fmt.Sprintf("Provider %s test failed: %v", provider.Name, err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Test notification sent"})
}

// Templates returns a list of built-in templates a provider can use.
func (h *NotificationProviderHandler) Templates(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{
		{"id": "minimal", "name": "Minimal", "description": "Small JSON payload with title, message and time."},
		{"id": "detailed", "name": "Detailed", "description": "Full JSON payload with host, services and all data."},
		{"id": "custom", "name": "Custom", "description": "Use your own JSON template in the Config field."},
	})
}

// Preview renders the template for a provider and returns the resulting JSON object or an error.
func (h *NotificationProviderHandler) Preview(c *gin.Context) {
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var provider models.NotificationProvider
	// Marshal raw into provider to get proper types
	if b, err := json.Marshal(raw); err == nil {
		_ = json.Unmarshal(b, &provider)
	}
	var payload map[string]any
	if d, ok := raw["data"].(map[string]any); ok {
		payload = d
	}

	if payload == nil {
		payload = map[string]any{}
	}

	// Add some defaults for preview
	if _, ok := payload["Title"]; !ok {
		payload["Title"] = "Preview Title"
	}
	if _, ok := payload["Message"]; !ok {
		payload["Message"] = "Preview Message"
	}
	payload["Time"] = time.Now().Format(time.RFC3339)
	payload["EventType"] = "preview"

	rendered, parsed, err := h.service.RenderTemplate(provider, payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "rendered": rendered})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rendered": rendered, "parsed": parsed})
}
