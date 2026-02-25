package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/trace"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type NotificationProviderHandler struct {
	service         *services.NotificationService
	securityService *services.SecurityService
	dataRoot        string
}

type notificationProviderUpsertRequest struct {
	Name                            string `json:"name"`
	Type                            string `json:"type"`
	URL                             string `json:"url"`
	Config                          string `json:"config"`
	Template                        string `json:"template"`
	Token                           string `json:"token,omitempty"`
	Enabled                         bool   `json:"enabled"`
	NotifyProxyHosts                bool   `json:"notify_proxy_hosts"`
	NotifyRemoteServers             bool   `json:"notify_remote_servers"`
	NotifyDomains                   bool   `json:"notify_domains"`
	NotifyCerts                     bool   `json:"notify_certs"`
	NotifyUptime                    bool   `json:"notify_uptime"`
	NotifySecurityWAFBlocks         bool   `json:"notify_security_waf_blocks"`
	NotifySecurityACLDenies         bool   `json:"notify_security_acl_denies"`
	NotifySecurityRateLimitHits     bool   `json:"notify_security_rate_limit_hits"`
	NotifySecurityCrowdSecDecisions bool   `json:"notify_security_crowdsec_decisions"`
}

type notificationProviderTestRequest struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	Config   string `json:"config"`
	Template string `json:"template"`
	Token    string `json:"token,omitempty"`
}

func (r notificationProviderUpsertRequest) toModel() models.NotificationProvider {
	return models.NotificationProvider{
		Name:                            r.Name,
		Type:                            r.Type,
		URL:                             r.URL,
		Config:                          r.Config,
		Template:                        r.Template,
		Token:                           strings.TrimSpace(r.Token),
		Enabled:                         r.Enabled,
		NotifyProxyHosts:                r.NotifyProxyHosts,
		NotifyRemoteServers:             r.NotifyRemoteServers,
		NotifyDomains:                   r.NotifyDomains,
		NotifyCerts:                     r.NotifyCerts,
		NotifyUptime:                    r.NotifyUptime,
		NotifySecurityWAFBlocks:         r.NotifySecurityWAFBlocks,
		NotifySecurityACLDenies:         r.NotifySecurityACLDenies,
		NotifySecurityRateLimitHits:     r.NotifySecurityRateLimitHits,
		NotifySecurityCrowdSecDecisions: r.NotifySecurityCrowdSecDecisions,
	}
}

func providerRequestID(c *gin.Context) string {
	if value, ok := c.Get(string(trace.RequestIDKey)); ok {
		if requestID, ok := value.(string); ok {
			return requestID
		}
	}
	return ""
}

func respondSanitizedProviderError(c *gin.Context, status int, code, category, message string) {
	response := gin.H{
		"error":    message,
		"code":     code,
		"category": category,
	}
	if requestID := providerRequestID(c); requestID != "" {
		response["request_id"] = requestID
	}
	c.JSON(status, response)
}

var providerStatusCodePattern = regexp.MustCompile(`provider returned status\s+(\d{3})`)

func classifyProviderTestFailure(err error) (code string, category string, message string) {
	if err == nil {
		return "PROVIDER_TEST_FAILED", "dispatch", "Provider test failed"
	}

	errText := strings.ToLower(strings.TrimSpace(err.Error()))

	if strings.Contains(errText, "destination url validation failed") ||
		strings.Contains(errText, "invalid webhook url") ||
		strings.Contains(errText, "invalid discord webhook url") {
		return "PROVIDER_TEST_URL_INVALID", "validation", "Provider URL is invalid or blocked. Verify the URL and try again"
	}

	if statusMatch := providerStatusCodePattern.FindStringSubmatch(errText); len(statusMatch) == 2 {
		switch statusMatch[1] {
		case "401", "403":
			return "PROVIDER_TEST_AUTH_REJECTED", "dispatch", "Provider rejected authentication. Verify your Gotify token"
		case "404":
			return "PROVIDER_TEST_ENDPOINT_NOT_FOUND", "dispatch", "Provider endpoint was not found. Verify the provider URL path"
		default:
			return "PROVIDER_TEST_REMOTE_REJECTED", "dispatch", fmt.Sprintf("Provider rejected the test request (HTTP %s)", statusMatch[1])
		}
	}

	if strings.Contains(errText, "outbound request failed") || strings.Contains(errText, "failed to send webhook") {
		switch {
		case strings.Contains(errText, "dns lookup failed"):
			return "PROVIDER_TEST_DNS_FAILED", "dispatch", "DNS lookup failed for provider host. Verify the hostname in the provider URL"
		case strings.Contains(errText, "connection refused"):
			return "PROVIDER_TEST_CONNECTION_REFUSED", "dispatch", "Provider host refused the connection. Verify port and service availability"
		case strings.Contains(errText, "request timed out"):
			return "PROVIDER_TEST_TIMEOUT", "dispatch", "Provider request timed out. Verify network route and provider responsiveness"
		case strings.Contains(errText, "tls handshake failed"):
			return "PROVIDER_TEST_TLS_FAILED", "dispatch", "TLS handshake failed. Verify HTTPS certificate and URL scheme"
		}
		return "PROVIDER_TEST_UNREACHABLE", "dispatch", "Could not reach provider endpoint. Verify URL, DNS, and network connectivity"
	}

	return "PROVIDER_TEST_FAILED", "dispatch", "Provider test failed"
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
	for i := range providers {
		providers[i].HasToken = providers[i].Token != ""
		providers[i].Token = ""
	}
	c.JSON(http.StatusOK, providers)
}

func (h *NotificationProviderHandler) Create(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var req notificationProviderUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondSanitizedProviderError(c, http.StatusBadRequest, "INVALID_REQUEST", "validation", "Invalid notification provider payload")
		return
	}

	providerType := strings.ToLower(strings.TrimSpace(req.Type))
	if providerType != "discord" && providerType != "gotify" && providerType != "webhook" {
		respondSanitizedProviderError(c, http.StatusBadRequest, "UNSUPPORTED_PROVIDER_TYPE", "validation", "Unsupported notification provider type")
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
			respondSanitizedProviderError(c, http.StatusBadRequest, "PROVIDER_VALIDATION_FAILED", "validation", "Notification provider validation failed")
			return
		}
		if respondPermissionError(c, h.securityService, "notification_provider_save_failed", err, h.dataRoot) {
			return
		}
		respondSanitizedProviderError(c, http.StatusInternalServerError, "PROVIDER_CREATE_FAILED", "internal", "Failed to create provider")
		return
	}
	provider.HasToken = provider.Token != ""
	provider.Token = ""
	c.JSON(http.StatusCreated, provider)
}

func (h *NotificationProviderHandler) Update(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	id := c.Param("id")
	var req notificationProviderUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondSanitizedProviderError(c, http.StatusBadRequest, "INVALID_REQUEST", "validation", "Invalid notification provider payload")
		return
	}

	// Check if existing provider is non-Discord (deprecated)
	var existing models.NotificationProvider
	if err := h.service.DB.Where("id = ?", id).First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondSanitizedProviderError(c, http.StatusNotFound, "PROVIDER_NOT_FOUND", "validation", "Provider not found")
			return
		}
		respondSanitizedProviderError(c, http.StatusInternalServerError, "PROVIDER_READ_FAILED", "internal", "Failed to read provider")
		return
	}

	if strings.TrimSpace(req.Type) != "" && strings.TrimSpace(req.Type) != existing.Type {
		respondSanitizedProviderError(c, http.StatusBadRequest, "PROVIDER_TYPE_IMMUTABLE", "validation", "Provider type cannot be changed")
		return
	}

	providerType := strings.ToLower(strings.TrimSpace(existing.Type))
	if providerType != "discord" && providerType != "gotify" && providerType != "webhook" {
		respondSanitizedProviderError(c, http.StatusBadRequest, "UNSUPPORTED_PROVIDER_TYPE", "validation", "Unsupported notification provider type")
		return
	}

	if providerType == "gotify" && strings.TrimSpace(req.Token) == "" {
		// Keep existing token if update payload omits token
		req.Token = existing.Token
	}
	req.Type = existing.Type

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
			respondSanitizedProviderError(c, http.StatusBadRequest, "PROVIDER_VALIDATION_FAILED", "validation", "Notification provider validation failed")
			return
		}
		if respondPermissionError(c, h.securityService, "notification_provider_save_failed", err, h.dataRoot) {
			return
		}
		respondSanitizedProviderError(c, http.StatusInternalServerError, "PROVIDER_UPDATE_FAILED", "internal", "Failed to update provider")
		return
	}
	provider.HasToken = provider.Token != ""
	provider.Token = ""
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
	var req notificationProviderTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondSanitizedProviderError(c, http.StatusBadRequest, "INVALID_REQUEST", "validation", "Invalid test payload")
		return
	}

	providerType := strings.ToLower(strings.TrimSpace(req.Type))
	if providerType == "gotify" && strings.TrimSpace(req.Token) != "" {
		respondSanitizedProviderError(c, http.StatusBadRequest, "TOKEN_WRITE_ONLY", "validation", "Gotify token is accepted only on provider create/update")
		return
	}

	providerID := strings.TrimSpace(req.ID)
	if providerID == "" {
		respondSanitizedProviderError(c, http.StatusBadRequest, "MISSING_PROVIDER_ID", "validation", "Trusted provider ID is required for test dispatch")
		return
	}

	var provider models.NotificationProvider
	if err := h.service.DB.Where("id = ?", providerID).First(&provider).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondSanitizedProviderError(c, http.StatusNotFound, "PROVIDER_NOT_FOUND", "validation", "Provider not found")
			return
		}
		respondSanitizedProviderError(c, http.StatusInternalServerError, "PROVIDER_READ_FAILED", "internal", "Failed to read provider")
		return
	}

	if strings.TrimSpace(provider.URL) == "" {
		respondSanitizedProviderError(c, http.StatusBadRequest, "PROVIDER_CONFIG_MISSING", "validation", "Trusted provider configuration is incomplete")
		return
	}

	if err := h.service.TestProvider(provider); err != nil {
		// Create internal notification for the failure
		_, _ = h.service.Create(models.NotificationTypeError, "Test Failed", fmt.Sprintf("Provider %s test failed", provider.Name))
		code, category, message := classifyProviderTestFailure(err)
		respondSanitizedProviderError(c, http.StatusBadRequest, code, category, message)
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
		respondSanitizedProviderError(c, http.StatusBadRequest, "INVALID_REQUEST", "validation", "Invalid preview payload")
		return
	}
	if tokenValue, ok := raw["token"]; ok {
		if tokenText, isString := tokenValue.(string); isString && strings.TrimSpace(tokenText) != "" {
			respondSanitizedProviderError(c, http.StatusBadRequest, "TOKEN_WRITE_ONLY", "validation", "Gotify token is accepted only on provider create/update")
			return
		}
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
		_ = rendered
		respondSanitizedProviderError(c, http.StatusBadRequest, "TEMPLATE_PREVIEW_FAILED", "validation", "Template preview failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"rendered": rendered, "parsed": parsed})
}
