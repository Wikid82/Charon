package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// RedirectionHostHandler handles CRUD operations for redirection hosts,
// following ProxyHostHandler's conventions (structured gin.H{"error": ...}
// errors, server-generated UUIDs, partial-update-via-map[string]any on
// Update) — see docs/plans/current_spec.md §4.4.
type RedirectionHostHandler struct {
	service      *services.RedirectionHostService
	caddyManager *caddy.Manager
	db           *gorm.DB
}

// NewRedirectionHostHandler creates a new RedirectionHostHandler.
func NewRedirectionHostHandler(db *gorm.DB, caddyManager *caddy.Manager) *RedirectionHostHandler {
	return &RedirectionHostHandler{
		service:      services.NewRedirectionHostService(db),
		caddyManager: caddyManager,
		db:           db,
	}
}

// RegisterRoutes registers redirection host routes on the given router group.
func (h *RedirectionHostHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/redirection-hosts", h.List)
	router.POST("/redirection-hosts", h.Create)
	router.GET("/redirection-hosts/:uuid", h.Get)
	router.PUT("/redirection-hosts/:uuid", h.Update)
	router.DELETE("/redirection-hosts/:uuid", h.Delete)
}

// resolveCertificateReference resolves a certificate_id reference, accepting
// either a legacy numeric ID or a UUID string, mirroring
// ProxyHostHandler.resolveCertificateReference.
func (h *RedirectionHostHandler) resolveCertificateReference(value any) (*uint, error) {
	if value == nil {
		return nil, nil
	}

	parsedID, parseErr := parseNullableUintField(value, "certificate_id")
	if parseErr == nil {
		return parsedID, nil
	}

	uuidValue, isString := value.(string)
	if !isString {
		return nil, parseErr
	}

	trimmed := strings.TrimSpace(uuidValue)
	if trimmed == "" {
		return nil, nil
	}

	var cert models.SSLCertificate
	if err := h.db.Select("id").Where("uuid = ?", trimmed).First(&cert).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("certificate not found")
		}
		return nil, fmt.Errorf("failed to resolve certificate")
	}

	id := cert.ID
	return &id, nil
}

// resolveDNSProviderReference resolves a dns_provider_id reference, mirroring
// ProxyHostHandler.resolveDNSProviderReference.
func (h *RedirectionHostHandler) resolveDNSProviderReference(value any) (*uint, error) {
	if value == nil {
		return nil, nil
	}

	parsedID, parseErr := parseNullableUintField(value, "dns_provider_id")
	if parseErr == nil {
		return parsedID, nil
	}

	uuidValue, isString := value.(string)
	if !isString {
		return nil, parseErr
	}

	trimmed := strings.TrimSpace(uuidValue)
	if trimmed == "" {
		return nil, nil
	}

	var provider models.DNSProvider
	if err := h.db.Select("id").Where("uuid = ?", trimmed).First(&provider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("dns provider not found")
		}
		return nil, fmt.Errorf("failed to resolve dns provider")
	}

	id := provider.ID
	return &id, nil
}

// parseStatusCodeField converts a JSON payload value into an int for
// status_code. The bounded-enum check itself (301/302/307/308) happens at
// the service layer (RedirectionHostService.validateRedirectionHost); this
// only handles type coercion so Update's partial-payload map can assign it.
func parseStatusCodeField(value any) (int, error) {
	switch v := value.(type) {
	case float64:
		if v != math.Trunc(v) {
			return 0, fmt.Errorf("status_code must be one of 301, 302, 307, 308")
		}
		return int(v), nil
	case int:
		return v, nil
	case string:
		trimmed := strings.TrimSpace(v)
		code, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, fmt.Errorf("status_code must be one of 301, 302, 307, 308")
		}
		return code, nil
	default:
		return 0, fmt.Errorf("status_code must be one of 301, 302, 307, 308")
	}
}

// List retrieves all redirection hosts.
func (h *RedirectionHostHandler) List(c *gin.Context) {
	hosts, err := h.service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, hosts)
}

// Create creates a new redirection host.
func (h *RedirectionHostHandler) Create(c *gin.Context) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if rawCertRef, ok := payload["certificate_id"]; ok {
		resolvedCertID, resolveErr := h.resolveCertificateReference(rawCertRef)
		if resolveErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": resolveErr.Error()})
			return
		}
		payload["certificate_id"] = resolvedCertID
	}

	if rawDNSProviderRef, ok := payload["dns_provider_id"]; ok {
		resolvedDNSProviderID, resolveErr := h.resolveDNSProviderReference(rawDNSProviderRef)
		if resolveErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": resolveErr.Error()})
			return
		}
		payload["dns_provider_id"] = resolvedDNSProviderID
	}

	payloadBytes, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	var host models.RedirectionHost
	if err := json.Unmarshal(payloadBytes, &host); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	host.UUID = uuid.NewString()

	if err := h.service.Create(&host); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.caddyManager != nil {
		if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
			// Rollback: delete the created host if config application fails,
			// mirroring proxy_host_handler.go's Create rollback pattern.
			middleware.GetRequestLogger(c).WithError(err).Error("Error applying config")
			if deleteErr := h.service.Delete(host.ID); deleteErr != nil {
				idStr := strconv.FormatUint(uint64(host.ID), 10)
				middleware.GetRequestLogger(c).WithField("host_id", idStr).WithError(deleteErr).Error("Critical: Failed to rollback redirection host")
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply configuration: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusCreated, host)
}

// Get retrieves a redirection host by UUID.
func (h *RedirectionHostHandler) Get(c *gin.Context) {
	host, err := h.service.GetByUUID(c.Param("uuid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "redirection host not found"})
		return
	}
	c.JSON(http.StatusOK, host)
}

// Update partially updates an existing redirection host by UUID.
func (h *RedirectionHostHandler) Update(c *gin.Context) {
	host, err := h.service.GetByUUID(c.Param("uuid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "redirection host not found"})
		return
	}

	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if v, ok := payload["name"].(string); ok {
		host.Name = v
	}
	if v, ok := payload["domain_names"].(string); ok {
		host.DomainNames = strings.TrimSpace(v)
	}
	if v, ok := payload["target_url"].(string); ok {
		host.TargetURL = strings.TrimSpace(v)
	}
	if v, ok := payload["status_code"]; ok {
		code, parseErr := parseStatusCodeField(v)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": parseErr.Error()})
			return
		}
		host.StatusCode = code
	}
	if v, ok := payload["preserve_path"].(bool); ok {
		host.PreservePath = v
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
	if v, ok := payload["enabled"].(bool); ok {
		host.Enabled = v
	}
	if v, ok := payload["use_dns_challenge"].(bool); ok {
		host.UseDNSChallenge = v
	}

	if v, ok := payload["certificate_id"]; ok {
		resolvedCertID, resolveErr := h.resolveCertificateReference(v)
		if resolveErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": resolveErr.Error()})
			return
		}
		host.CertificateID = resolvedCertID
	}

	if v, ok := payload["dns_provider_id"]; ok {
		resolvedDNSProviderID, resolveErr := h.resolveDNSProviderReference(v)
		if resolveErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": resolveErr.Error()})
			return
		}
		host.DNSProviderID = resolvedDNSProviderID
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

	c.JSON(http.StatusOK, host)
}

// Delete removes a redirection host by UUID.
func (h *RedirectionHostHandler) Delete(c *gin.Context) {
	host, err := h.service.GetByUUID(c.Param("uuid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "redirection host not found"})
		return
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

	c.JSON(http.StatusOK, gin.H{"message": "redirection host deleted"})
}
