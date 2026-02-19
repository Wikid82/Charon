package handlers

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
	"github.com/Wikid82/charon/backend/internal/utils"
)

// ProxyHostWarning represents an advisory warning about proxy host configuration.
type ProxyHostWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ProxyHostResponse wraps a proxy host with optional advisory warnings.
// Uses explicit fields to avoid exposing internal database IDs.
type ProxyHostResponse struct {
	UUID                    string                        `json:"uuid"`
	Name                    string                        `json:"name"`
	DomainNames             string                        `json:"domain_names"`
	ForwardScheme           string                        `json:"forward_scheme"`
	ForwardHost             string                        `json:"forward_host"`
	ForwardPort             int                           `json:"forward_port"`
	SSLForced               bool                          `json:"ssl_forced"`
	HTTP2Support            bool                          `json:"http2_support"`
	HSTSEnabled             bool                          `json:"hsts_enabled"`
	HSTSSubdomains          bool                          `json:"hsts_subdomains"`
	BlockExploits           bool                          `json:"block_exploits"`
	WebsocketSupport        bool                          `json:"websocket_support"`
	Application             string                        `json:"application"`
	Enabled                 bool                          `json:"enabled"`
	CertificateID           *uint                         `json:"certificate_id"`
	Certificate             *models.SSLCertificate        `json:"certificate,omitempty"`
	AccessListID            *uint                         `json:"access_list_id"`
	AccessList              *models.AccessList            `json:"access_list,omitempty"`
	Locations               []models.Location             `json:"locations"`
	AdvancedConfig          string                        `json:"advanced_config"`
	AdvancedConfigBackup    string                        `json:"advanced_config_backup"`
	ForwardAuthEnabled      bool                          `json:"forward_auth_enabled"`
	WAFDisabled             bool                          `json:"waf_disabled"`
	SecurityHeaderProfileID *uint                         `json:"security_header_profile_id"`
	SecurityHeaderProfile   *models.SecurityHeaderProfile `json:"security_header_profile,omitempty"`
	SecurityHeadersEnabled  bool                          `json:"security_headers_enabled"`
	SecurityHeadersCustom   string                        `json:"security_headers_custom"`
	EnableStandardHeaders   *bool                         `json:"enable_standard_headers,omitempty"`
	DNSProviderID           *uint                         `json:"dns_provider_id,omitempty"`
	DNSProvider             *models.DNSProvider           `json:"dns_provider,omitempty"`
	UseDNSChallenge         bool                          `json:"use_dns_challenge"`
	CreatedAt               time.Time                     `json:"created_at"`
	UpdatedAt               time.Time                     `json:"updated_at"`
	Warnings                []ProxyHostWarning            `json:"warnings,omitempty"`
}

// NewProxyHostResponse creates a ProxyHostResponse from a ProxyHost model.
func NewProxyHostResponse(host *models.ProxyHost, warnings []ProxyHostWarning) ProxyHostResponse {
	return ProxyHostResponse{
		UUID:                    host.UUID,
		Name:                    host.Name,
		DomainNames:             host.DomainNames,
		ForwardScheme:           host.ForwardScheme,
		ForwardHost:             host.ForwardHost,
		ForwardPort:             host.ForwardPort,
		SSLForced:               host.SSLForced,
		HTTP2Support:            host.HTTP2Support,
		HSTSEnabled:             host.HSTSEnabled,
		HSTSSubdomains:          host.HSTSSubdomains,
		BlockExploits:           host.BlockExploits,
		WebsocketSupport:        host.WebsocketSupport,
		Application:             host.Application,
		Enabled:                 host.Enabled,
		CertificateID:           host.CertificateID,
		Certificate:             host.Certificate,
		AccessListID:            host.AccessListID,
		AccessList:              host.AccessList,
		Locations:               host.Locations,
		AdvancedConfig:          host.AdvancedConfig,
		AdvancedConfigBackup:    host.AdvancedConfigBackup,
		ForwardAuthEnabled:      host.ForwardAuthEnabled,
		WAFDisabled:             host.WAFDisabled,
		SecurityHeaderProfileID: host.SecurityHeaderProfileID,
		SecurityHeaderProfile:   host.SecurityHeaderProfile,
		SecurityHeadersEnabled:  host.SecurityHeadersEnabled,
		SecurityHeadersCustom:   host.SecurityHeadersCustom,
		EnableStandardHeaders:   host.EnableStandardHeaders,
		DNSProviderID:           host.DNSProviderID,
		DNSProvider:             host.DNSProvider,
		UseDNSChallenge:         host.UseDNSChallenge,
		CreatedAt:               host.CreatedAt,
		UpdatedAt:               host.UpdatedAt,
		Warnings:                warnings,
	}
}

// generateForwardHostWarnings checks the forward_host value and returns advisory warnings.
func generateForwardHostWarnings(forwardHost string) []ProxyHostWarning {
	var warnings []ProxyHostWarning

	if utils.IsDockerBridgeIP(forwardHost) {
		warnings = append(warnings, ProxyHostWarning{
			Field:   "forward_host",
			Message: "This looks like a Docker container IP address. Docker IPs can change when containers restart. Consider using the container name for more reliable connections.",
		})
	} else if ip := net.ParseIP(forwardHost); ip != nil && network.IsPrivateIP(ip) {
		warnings = append(warnings, ProxyHostWarning{
			Field:   "forward_host",
			Message: "Using a private IP address. If this is a Docker container, the IP may change on restart. Container names are more reliable for Docker services.",
		})
	}

	return warnings
}

// ProxyHostHandler handles CRUD operations for proxy hosts.
type ProxyHostHandler struct {
	service             *services.ProxyHostService
	caddyManager        *caddy.Manager
	notificationService *services.NotificationService
	uptimeService       *services.UptimeService
}

// safeIntToUint safely converts int to uint, returning false if negative (gosec G115)
func safeIntToUint(i int) (uint, bool) {
	if i < 0 {
		return 0, false
	}
	return uint(i), true
}

// safeFloat64ToUint safely converts float64 to uint, returning false if invalid (gosec G115)
func safeFloat64ToUint(f float64) (uint, bool) {
	if f < 0 || f != float64(uint(f)) {
		return 0, false
	}
	return uint(f), true
}

func parseNullableUintField(value any, fieldName string) (*uint, bool, error) {
	if value == nil {
		return nil, true, nil
	}

	switch v := value.(type) {
	case float64:
		if id, ok := safeFloat64ToUint(v); ok {
			return &id, true, nil
		}
		return nil, true, fmt.Errorf("invalid %s: unable to convert value %v of type %T to uint", fieldName, value, value)
	case int:
		if id, ok := safeIntToUint(v); ok {
			return &id, true, nil
		}
		return nil, true, fmt.Errorf("invalid %s: unable to convert value %v of type %T to uint", fieldName, value, value)
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, true, nil
		}
		n, err := strconv.ParseUint(trimmed, 10, 32)
		if err != nil {
			return nil, true, fmt.Errorf("invalid %s: unable to convert value %v of type %T to uint", fieldName, value, value)
		}
		id := uint(n)
		return &id, true, nil
	default:
		return nil, true, fmt.Errorf("invalid %s: unable to convert value %v of type %T to uint", fieldName, value, value)
	}
}

func parseForwardPortField(value any) (int, error) {
	switch v := value.(type) {
	case float64:
		if v != math.Trunc(v) {
			return 0, fmt.Errorf("invalid forward_port: must be an integer")
		}
		port := int(v)
		if port < 1 || port > 65535 {
			return 0, fmt.Errorf("invalid forward_port: must be between 1 and 65535")
		}
		return port, nil
	case int:
		if v < 1 || v > 65535 {
			return 0, fmt.Errorf("invalid forward_port: must be between 1 and 65535")
		}
		return v, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, fmt.Errorf("invalid forward_port: must be between 1 and 65535")
		}
		port, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, fmt.Errorf("invalid forward_port: must be an integer")
		}
		if port < 1 || port > 65535 {
			return 0, fmt.Errorf("invalid forward_port: must be between 1 and 65535")
		}
		return port, nil
	default:
		return 0, fmt.Errorf("invalid forward_port: unsupported type %T", value)
	}
}

// NewProxyHostHandler creates a new proxy host handler.
func NewProxyHostHandler(db *gorm.DB, caddyManager *caddy.Manager, ns *services.NotificationService, uptimeService *services.UptimeService) *ProxyHostHandler {
	return &ProxyHostHandler{
		service:             services.NewProxyHostService(db),
		caddyManager:        caddyManager,
		notificationService: ns,
		uptimeService:       uptimeService,
	}
}

// RegisterRoutes registers proxy host routes.
func (h *ProxyHostHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/proxy-hosts", h.List)
	router.POST("/proxy-hosts", h.Create)
	router.GET("/proxy-hosts/:uuid", h.Get)
	router.PUT("/proxy-hosts/:uuid", h.Update)
	router.DELETE("/proxy-hosts/:uuid", h.Delete)
	router.POST("/proxy-hosts/test", h.TestConnection)
	router.PUT("/proxy-hosts/bulk-update-acl", h.BulkUpdateACL)
	router.PUT("/proxy-hosts/bulk-update-security-headers", h.BulkUpdateSecurityHeaders)
}

// List retrieves all proxy hosts.
func (h *ProxyHostHandler) List(c *gin.Context) {
	hosts, err := h.service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, hosts)
}

// Create creates a new proxy host.
func (h *ProxyHostHandler) Create(c *gin.Context) {
	var host models.ProxyHost
	if err := c.ShouldBindJSON(&host); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate and normalize advanced config if present
	if host.AdvancedConfig != "" {
		var parsed any
		if err := json.Unmarshal([]byte(host.AdvancedConfig), &parsed); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid advanced_config JSON: " + err.Error()})
			return
		}
		parsed = caddy.NormalizeAdvancedConfig(parsed)
		if norm, err := json.Marshal(parsed); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid advanced_config after normalization: " + err.Error()})
			return
		} else {
			host.AdvancedConfig = string(norm)
		}
	}

	host.UUID = uuid.NewString()

	// Assign UUIDs to locations
	for i := range host.Locations {
		host.Locations[i].UUID = uuid.NewString()
	}

	if err := h.service.Create(&host); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.caddyManager != nil {
		if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
			// Rollback: delete the created host if config application fails
			middleware.GetRequestLogger(c).WithError(err).Error("Error applying config")
			if deleteErr := h.service.Delete(host.ID); deleteErr != nil {
				idStr := strconv.FormatUint(uint64(host.ID), 10)
				middleware.GetRequestLogger(c).WithField("host_id", idStr).WithError(deleteErr).Error("Critical: Failed to rollback host")
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply configuration: " + err.Error()})
			return
		}
	}

	// Send Notification
	if h.notificationService != nil {
		h.notificationService.SendExternal(c.Request.Context(),
			"proxy_host",
			"Proxy Host Created",
			fmt.Sprintf("Proxy Host %s (%s) created", util.SanitizeForLog(host.Name), util.SanitizeForLog(host.DomainNames)),
			map[string]any{
				"Name":    util.SanitizeForLog(host.Name),
				"Domains": util.SanitizeForLog(host.DomainNames),
				"Action":  "created",
			},
		)
	}

	// Generate advisory warnings for private/Docker IPs
	warnings := generateForwardHostWarnings(host.ForwardHost)

	// Return response with warnings if any
	if len(warnings) > 0 {
		c.JSON(http.StatusCreated, NewProxyHostResponse(&host, warnings))
		return
	}

	c.JSON(http.StatusCreated, host)
}

// Get retrieves a proxy host by UUID.
func (h *ProxyHostHandler) Get(c *gin.Context) {
	uuidStr := c.Param("uuid")

	host, err := h.service.GetByUUID(uuidStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy host not found"})
		return
	}

	c.JSON(http.StatusOK, host)
}

// Update updates an existing proxy host.
func (h *ProxyHostHandler) Update(c *gin.Context) {
	uuidStr := c.Param("uuid")

	host, err := h.service.GetByUUID(uuidStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy host not found"})
		return
	}

	// Perform a partial update: only mutate fields present in payload
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Handle simple scalar fields by json tag names (snake_case)
	if v, ok := payload["name"].(string); ok {
		host.Name = v
	}
	if v, ok := payload["domain_names"].(string); ok {
		host.DomainNames = strings.TrimSpace(v)
	}
	if v, ok := payload["forward_scheme"].(string); ok {
		host.ForwardScheme = v
	}
	if v, ok := payload["forward_host"].(string); ok {
		host.ForwardHost = strings.TrimSpace(v)
	}
	if v, ok := payload["forward_port"]; ok {
		port, parseErr := parseForwardPortField(v)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": parseErr.Error()})
			return
		}
		host.ForwardPort = port
	}
	if v, ok := payload["ssl_forced"].(bool); ok {
		host.SSLForced = v
	}
	if v, ok := payload["http2_support"].(bool); ok {
		host.HTTP2Support = v
	}
	if v, ok := payload["hsts_enabled"].(bool); ok {
		host.HSTSEnabled = v
	}
	if v, ok := payload["hsts_subdomains"].(bool); ok {
		host.HSTSSubdomains = v
	}
	if v, ok := payload["block_exploits"].(bool); ok {
		host.BlockExploits = v
	}
	if v, ok := payload["websocket_support"].(bool); ok {
		host.WebsocketSupport = v
	}
	if v, ok := payload["application"].(string); ok {
		host.Application = v
	}
	if v, ok := payload["enabled"].(bool); ok {
		host.Enabled = v
	}

	// Handle enable_standard_headers (nullable bool - uses pointer pattern like certificate_id)
	if v, ok := payload["enable_standard_headers"]; ok {
		if v == nil {
			host.EnableStandardHeaders = nil // Explicit null → use default behavior
		} else if b, ok := v.(bool); ok {
			host.EnableStandardHeaders = &b // Explicit true/false
		}
	}

	// Handle forward_auth_enabled (regular bool)
	if v, ok := payload["forward_auth_enabled"].(bool); ok {
		host.ForwardAuthEnabled = v
	}

	// Handle waf_disabled (regular bool)
	if v, ok := payload["waf_disabled"].(bool); ok {
		host.WAFDisabled = v
	}

	// Nullable foreign keys
	if v, ok := payload["certificate_id"]; ok {
		parsedID, _, parseErr := parseNullableUintField(v, "certificate_id")
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": parseErr.Error()})
			return
		}
		host.CertificateID = parsedID
	}
	if v, ok := payload["access_list_id"]; ok {
		parsedID, _, parseErr := parseNullableUintField(v, "access_list_id")
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": parseErr.Error()})
			return
		}
		host.AccessListID = parsedID
	}

	if v, ok := payload["dns_provider_id"]; ok {
		parsedID, _, parseErr := parseNullableUintField(v, "dns_provider_id")
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": parseErr.Error()})
			return
		}
		host.DNSProviderID = parsedID
	}

	if v, ok := payload["use_dns_challenge"].(bool); ok {
		host.UseDNSChallenge = v
	}

	// Security Header Profile: update only if provided
	if v, ok := payload["security_header_profile_id"]; ok {
		logger := middleware.GetRequestLogger(c)
		// Sanitize user-provided values for log injection protection (CWE-117)
		safeUUID := sanitizeForLog(uuidStr)
		logger.WithField("host_uuid", safeUUID).WithField("raw_value", sanitizeForLog(fmt.Sprintf("%v", v))).Debug("Processing security_header_profile_id update")

		if v == nil {
			logger.WithField("host_uuid", safeUUID).Debug("Setting security_header_profile_id to nil")
			host.SecurityHeaderProfileID = nil
		} else {
			conversionSuccess := false
			switch t := v.(type) {
			case float64:
				logger.Debug("Received security_header_profile_id as float64")
				if id, ok := safeFloat64ToUint(t); ok {
					host.SecurityHeaderProfileID = &id
					conversionSuccess = true
					logger.Info("Successfully converted security_header_profile_id from float64")
				} else {
					logger.Warn("Failed to convert security_header_profile_id from float64: value is negative or not a valid uint")
				}
			case int:
				logger.Debug("Received security_header_profile_id as int")
				if id, ok := safeIntToUint(t); ok {
					host.SecurityHeaderProfileID = &id
					conversionSuccess = true
					logger.Info("Successfully converted security_header_profile_id from int")
				} else {
					logger.Warn("Failed to convert security_header_profile_id from int: value is negative")
				}
			case string:
				logger.Debug("Received security_header_profile_id as string")
				if n, err := strconv.ParseUint(t, 10, 32); err == nil {
					id := uint(n)
					host.SecurityHeaderProfileID = &id
					conversionSuccess = true
					logger.WithField("host_uuid", safeUUID).WithField("profile_id", id).Info("Successfully converted security_header_profile_id from string")
				} else {
					logger.Warn("Failed to parse security_header_profile_id from string")
				}
			default:
				logger.Warn("Unsupported type for security_header_profile_id")
			}

			if !conversionSuccess {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid security_header_profile_id: unable to convert value %v of type %T to uint", v, v)})
				return
			}
		}
	}

	// Locations: replace only if provided
	if v, ok := payload["locations"].([]any); ok {
		// Rebind to []models.Location
		b, _ := json.Marshal(v)
		var locs []models.Location
		if err := json.Unmarshal(b, &locs); err == nil {
			// Ensure UUIDs exist for any new location entries
			for i := range locs {
				if locs[i].UUID == "" {
					locs[i].UUID = uuid.New().String()
				}
			}
			host.Locations = locs
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid locations payload"})
			return
		}
	}

	// Advanced config: normalize if provided and changed
	if v, ok := payload["advanced_config"].(string); ok {
		if v != "" && v != host.AdvancedConfig {
			var parsed any
			if err := json.Unmarshal([]byte(v), &parsed); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid advanced_config JSON: " + err.Error()})
				return
			}
			parsed = caddy.NormalizeAdvancedConfig(parsed)
			if norm, err := json.Marshal(parsed); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid advanced_config after normalization: " + err.Error()})
				return
			} else {
				// Backup previous
				host.AdvancedConfigBackup = host.AdvancedConfig
				host.AdvancedConfig = string(norm)
			}
		} else if v == "" { // allow clearing advanced config
			host.AdvancedConfigBackup = host.AdvancedConfig
			host.AdvancedConfig = ""
		}
	}

	if err := h.service.Update(host); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.caddyManager != nil {
		if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply configuration: " + err.Error()})
			return
		}
	}

	// Sync associated uptime monitor with updated proxy host values
	if h.uptimeService != nil {
		if err := h.uptimeService.SyncMonitorForHost(host.ID); err != nil {
			middleware.GetRequestLogger(c).WithError(err).WithField("host_id", host.ID).Warn("Failed to sync uptime monitor for host")
			// Don't fail the request if sync fails - the host update succeeded
		}
	}

	// Generate advisory warnings for private/Docker IPs
	warnings := generateForwardHostWarnings(host.ForwardHost)

	// Return response with warnings if any
	if len(warnings) > 0 {
		c.JSON(http.StatusOK, NewProxyHostResponse(host, warnings))
		return
	}

	c.JSON(http.StatusOK, host)
}

// Delete removes a proxy host.
func (h *ProxyHostHandler) Delete(c *gin.Context) {
	uuidStr := c.Param("uuid")

	host, err := h.service.GetByUUID(uuidStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy host not found"})
		return
	}

	// check if we should also delete associated uptime monitors (query param: delete_uptime=true)
	deleteUptime := c.DefaultQuery("delete_uptime", "false") == "true"

	if deleteUptime && h.uptimeService != nil {
		// Find all monitors referencing this proxy host and delete each
		var monitors []models.UptimeMonitor
		if err := h.uptimeService.DB.Where("proxy_host_id = ?", host.ID).Find(&monitors).Error; err == nil {
			for _, m := range monitors {
				_ = h.uptimeService.DeleteMonitor(m.ID)
			}
		}
	}

	if err := h.service.Delete(host.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.caddyManager != nil {
		if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply configuration: " + err.Error()})
			return
		}
	}

	// Send Notification
	if h.notificationService != nil {
		h.notificationService.SendExternal(c.Request.Context(),
			"proxy_host",
			"Proxy Host Deleted",
			fmt.Sprintf("Proxy Host %s deleted", host.Name),
			map[string]any{
				"Name":   host.Name,
				"Action": "deleted",
			},
		)
	}

	c.JSON(http.StatusOK, gin.H{"message": "proxy host deleted"})
}

// TestConnection checks if the proxy host is reachable.
func (h *ProxyHostHandler) TestConnection(c *gin.Context) {
	var req struct {
		ForwardHost string `json:"forward_host" binding:"required"`
		ForwardPort int    `json:"forward_port" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.TestConnection(req.ForwardHost, req.ForwardPort); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Connection successful"})
}

// BulkUpdateACL applies or removes an access list to multiple proxy hosts.
func (h *ProxyHostHandler) BulkUpdateACL(c *gin.Context) {
	var req struct {
		HostUUIDs    []string `json:"host_uuids" binding:"required"`
		AccessListID *uint    `json:"access_list_id"` // nil means remove ACL
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.HostUUIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_uuids cannot be empty"})
		return
	}

	updated := 0
	errors := []map[string]string{}

	for _, hostUUID := range req.HostUUIDs {
		host, err := h.service.GetByUUID(hostUUID)
		if err != nil {
			errors = append(errors, map[string]string{
				"uuid":  hostUUID,
				"error": "proxy host not found",
			})
			continue
		}

		host.AccessListID = req.AccessListID
		if err := h.service.Update(host); err != nil {
			errors = append(errors, map[string]string{
				"uuid":  hostUUID,
				"error": err.Error(),
			})
			continue
		}

		updated++
	}

	// Apply Caddy config once for all updates
	if updated > 0 && h.caddyManager != nil {
		if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to apply configuration: " + err.Error(),
				"updated": updated,
				"errors":  errors,
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"updated": updated,
		"errors":  errors,
	})
}

// BulkUpdateSecurityHeadersRequest represents the request body for bulk security header updates.
type BulkUpdateSecurityHeadersRequest struct {
	HostUUIDs               []string `json:"host_uuids" binding:"required"`
	SecurityHeaderProfileID *uint    `json:"security_header_profile_id"` // nil means remove profile
}

// BulkUpdateSecurityHeaders applies or removes a security header profile to multiple proxy hosts.
func (h *ProxyHostHandler) BulkUpdateSecurityHeaders(c *gin.Context) {
	var req BulkUpdateSecurityHeadersRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.HostUUIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_uuids cannot be empty"})
		return
	}

	// Validate profile exists if provided
	if req.SecurityHeaderProfileID != nil {
		var profile models.SecurityHeaderProfile
		if err := h.service.DB().First(&profile, *req.SecurityHeaderProfileID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusBadRequest, gin.H{"error": "security header profile not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// Start transaction for atomic updates
	tx := h.service.DB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	updated := 0
	errors := []map[string]string{}

	for _, hostUUID := range req.HostUUIDs {
		var host models.ProxyHost
		if err := tx.Where("uuid = ?", hostUUID).First(&host).Error; err != nil {
			errors = append(errors, map[string]string{
				"uuid":  hostUUID,
				"error": "proxy host not found",
			})
			continue
		}

		// Update security header profile ID
		host.SecurityHeaderProfileID = req.SecurityHeaderProfileID
		if err := tx.Model(&host).Where("id = ?", host.ID).Select("SecurityHeaderProfileID").Updates(&host).Error; err != nil {
			errors = append(errors, map[string]string{
				"uuid":  hostUUID,
				"error": err.Error(),
			})
			continue
		}

		updated++
	}

	// Commit transaction only if all updates succeeded
	if len(errors) > 0 && updated == 0 {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "All updates failed",
			"updated": updated,
			"errors":  errors,
		})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
		return
	}

	// Apply Caddy config once for all updates
	if updated > 0 && h.caddyManager != nil {
		if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to apply configuration: " + err.Error(),
				"updated": updated,
				"errors":  errors,
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"updated": updated,
		"errors":  errors,
	})
}
