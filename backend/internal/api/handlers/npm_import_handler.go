package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// npmImportSessions stores parsed NPM exports keyed by session UUID.
// TODO: Implement session expiration to prevent memory leaks (e.g., TTL-based cleanup).
var (
	npmImportSessions   = make(map[string]NPMExport)
	npmImportSessionsMu sync.RWMutex
)

// NPMExport represents the top-level structure of an NPM export file.
type NPMExport struct {
	ProxyHosts   []NPMProxyHost   `json:"proxy_hosts"`
	AccessLists  []NPMAccessList  `json:"access_lists"`
	Certificates []NPMCertificate `json:"certificates"`
}

// NPMProxyHost represents a proxy host from NPM export.
type NPMProxyHost struct {
	ID                       int      `json:"id"`
	DomainNames              []string `json:"domain_names"`
	ForwardScheme            string   `json:"forward_scheme"`
	ForwardHost              string   `json:"forward_host"`
	ForwardPort              int      `json:"forward_port"`
	CertificateID            *int     `json:"certificate_id"`
	SSLForced                bool     `json:"ssl_forced"`
	CachingEnabled           bool     `json:"caching_enabled"`
	BlockExploits            bool     `json:"block_exploits"`
	AdvancedConfig           string   `json:"advanced_config"`
	Meta                     any      `json:"meta"`
	AllowWebsocketUpgrade    bool     `json:"allow_websocket_upgrade"`
	HTTP2Support             bool     `json:"http2_support"`
	HSTSEnabled              bool     `json:"hsts_enabled"`
	HSTSSubdomains           bool     `json:"hsts_subdomains"`
	AccessListID             *int     `json:"access_list_id"`
	Enabled                  bool     `json:"enabled"`
	Locations                []any    `json:"locations"`
	CustomLocations          []any    `json:"custom_locations"`
	OwnerUserID              int      `json:"owner_user_id"`
	UseDefaultLocation       bool     `json:"use_default_location"`
	IPV6                     bool     `json:"ipv6"`
	CreatedOn                string   `json:"created_on"`
	ModifiedOn               string   `json:"modified_on"`
	ForwardDomainName        string   `json:"forward_domain_name"`
	ForwardDomainNameEnabled bool     `json:"forward_domain_name_enabled"`
}

// NPMAccessList represents an access list from NPM export.
type NPMAccessList struct {
	ID                  int             `json:"id"`
	Name                string          `json:"name"`
	PassAuth            int             `json:"pass_auth"`
	SatisfyAny          int             `json:"satisfy_any"`
	OwnerUserID         int             `json:"owner_user_id"`
	Items               []NPMAccessItem `json:"items"`
	Clients             []NPMAccessItem `json:"clients"`
	ProxyHostsCount     int             `json:"proxy_host_count"`
	CreatedOn           string          `json:"created_on"`
	ModifiedOn          string          `json:"modified_on"`
	AuthorizationHeader any             `json:"authorization_header"`
}

// NPMAccessItem represents an item in an NPM access list.
type NPMAccessItem struct {
	ID           int    `json:"id"`
	AccessListID int    `json:"access_list_id"`
	Address      string `json:"address"`
	Directive    string `json:"directive"`
	CreatedOn    string `json:"created_on"`
	ModifiedOn   string `json:"modified_on"`
}

// NPMCertificate represents a certificate from NPM export.
type NPMCertificate struct {
	ID             int      `json:"id"`
	Provider       string   `json:"provider"`
	NiceName       string   `json:"nice_name"`
	DomainNames    []string `json:"domain_names"`
	ExpiresOn      string   `json:"expires_on"`
	CreatedOn      string   `json:"created_on"`
	ModifiedOn     string   `json:"modified_on"`
	IsDNSChallenge bool     `json:"is_dns_challenge"`
	Meta           any      `json:"meta"`
}

// NPMImportHandler handles NPM configuration imports.
type NPMImportHandler struct {
	db           *gorm.DB
	proxyHostSvc *services.ProxyHostService
}

// NewNPMImportHandler creates a new NPM import handler.
func NewNPMImportHandler(db *gorm.DB) *NPMImportHandler {
	return &NPMImportHandler{
		db:           db,
		proxyHostSvc: services.NewProxyHostService(db),
	}
}

// RegisterRoutes registers NPM import routes.
func (h *NPMImportHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/import/npm/upload", h.Upload)
	router.POST("/import/npm/commit", h.Commit)
	router.POST("/import/npm/cancel", h.Cancel)
}

// Upload parses an NPM export JSON and returns a preview with conflict detection.
func (h *NPMImportHandler) Upload(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var npmExport NPMExport
	if err := json.Unmarshal([]byte(req.Content), &npmExport); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid NPM export JSON: %v", err)})
		return
	}

	result := h.convertNPMToImportResult(npmExport)

	if len(result.Hosts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no proxy hosts found in NPM export"})
		return
	}

	// Check for conflicts with existing hosts
	existingHosts, _ := h.proxyHostSvc.List()
	existingDomainsMap := make(map[string]models.ProxyHost)
	for _, eh := range existingHosts {
		existingDomainsMap[eh.DomainNames] = eh
	}

	conflictDetails := make(map[string]gin.H)
	for _, ph := range result.Hosts {
		if existing, found := existingDomainsMap[ph.DomainNames]; found {
			result.Conflicts = append(result.Conflicts, ph.DomainNames)
			conflictDetails[ph.DomainNames] = gin.H{
				"existing": gin.H{
					"forward_scheme": existing.ForwardScheme,
					"forward_host":   existing.ForwardHost,
					"forward_port":   existing.ForwardPort,
					"ssl_forced":     existing.SSLForced,
					"websocket":      existing.WebsocketSupport,
					"enabled":        existing.Enabled,
				},
				"imported": gin.H{
					"forward_scheme": ph.ForwardScheme,
					"forward_host":   ph.ForwardHost,
					"forward_port":   ph.ForwardPort,
					"ssl_forced":     ph.SSLForced,
					"websocket":      ph.WebsocketSupport,
				},
			}
		}
	}

	sid := uuid.NewString()

	// Store the parsed export in session storage for later commit
	npmImportSessionsMu.Lock()
	npmImportSessions[sid] = npmExport
	npmImportSessionsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"session":          gin.H{"id": sid, "state": "transient", "source_type": "npm"},
		"preview":          result,
		"conflict_details": conflictDetails,
		"npm_export": gin.H{
			"proxy_hosts":  len(npmExport.ProxyHosts),
			"access_lists": len(npmExport.AccessLists),
			"certificates": len(npmExport.Certificates),
		},
	})
}

// Commit finalizes the NPM import with user's conflict resolutions.
func (h *NPMImportHandler) Commit(c *gin.Context) {
	var req struct {
		SessionUUID string            `json:"session_uuid" binding:"required"`
		Resolutions map[string]string `json:"resolutions"` // domain -> action
		Names       map[string]string `json:"names"`       // domain -> custom name
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Retrieve the stored NPM export from session
	npmImportSessionsMu.RLock()
	npmExport, ok := npmImportSessions[req.SessionUUID]
	npmImportSessionsMu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found or expired"})
		return
	}

	result := h.convertNPMToImportResult(npmExport)
	proxyHosts := caddy.ConvertToProxyHosts(result.Hosts)

	created := 0
	updated := 0
	skipped := 0
	errors := []string{}

	existingHosts, _ := h.proxyHostSvc.List()
	existingMap := make(map[string]*models.ProxyHost)
	for i := range existingHosts {
		existingMap[existingHosts[i].DomainNames] = &existingHosts[i]
	}

	for _, host := range proxyHosts {
		action := req.Resolutions[host.DomainNames]

		if customName, ok := req.Names[host.DomainNames]; ok && customName != "" {
			host.Name = customName
		}

		if action == "skip" || action == "keep" {
			skipped++
			continue
		}

		if action == "rename" {
			host.DomainNames += "-imported"
		}

		if action == "overwrite" {
			if existing, found := existingMap[host.DomainNames]; found {
				host.ID = existing.ID
				host.UUID = existing.UUID
				host.CertificateID = existing.CertificateID
				host.CreatedAt = existing.CreatedAt

				if err := h.proxyHostSvc.Update(&host); err != nil {
					errors = append(errors, fmt.Sprintf("%s: %s", host.DomainNames, err.Error()))
				} else {
					updated++
				}
				continue
			}
		}

		host.UUID = uuid.NewString()
		if err := h.proxyHostSvc.Create(&host); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", host.DomainNames, err.Error()))
		} else {
			created++
		}
	}

	// Clean up session after successful commit
	npmImportSessionsMu.Lock()
	delete(npmImportSessions, req.SessionUUID)
	npmImportSessionsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"created": created,
		"updated": updated,
		"skipped": skipped,
		"errors":  errors,
	})
}

// Cancel cancels an NPM import session and cleans up resources.
func (h *NPMImportHandler) Cancel(c *gin.Context) {
	var req struct {
		SessionUUID string `json:"session_uuid"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Clean up session if it exists
	npmImportSessionsMu.Lock()
	delete(npmImportSessions, req.SessionUUID)
	npmImportSessionsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

// convertNPMToImportResult converts NPM export format to Charon's ImportResult.
func (h *NPMImportHandler) convertNPMToImportResult(export NPMExport) *caddy.ImportResult {
	result := &caddy.ImportResult{
		Hosts:     []caddy.ParsedHost{},
		Conflicts: []string{},
		Errors:    []string{},
	}

	for _, npmHost := range export.ProxyHosts {
		if len(npmHost.DomainNames) == 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("host %d has no domain names", npmHost.ID))
			continue
		}

		// NPM stores multiple domains as array; join them
		domainNames := ""
		for i, d := range npmHost.DomainNames {
			if i > 0 {
				domainNames += ","
			}
			domainNames += d
		}

		scheme := npmHost.ForwardScheme
		if scheme == "" {
			scheme = "http"
		}

		port := npmHost.ForwardPort
		if port == 0 {
			port = 80
		}

		warnings := []string{}
		if npmHost.CachingEnabled {
			warnings = append(warnings, "Caching not supported - will be disabled")
		}
		if len(npmHost.Locations) > 0 || len(npmHost.CustomLocations) > 0 {
			warnings = append(warnings, "Custom locations not fully supported")
		}
		if npmHost.AdvancedConfig != "" {
			warnings = append(warnings, "Advanced nginx config not compatible - manual review required")
		}
		if npmHost.AccessListID != nil && *npmHost.AccessListID > 0 {
			warnings = append(warnings, fmt.Sprintf("Access list reference (ID: %d) needs manual mapping", *npmHost.AccessListID))
		}

		host := caddy.ParsedHost{
			DomainNames:      domainNames,
			ForwardScheme:    scheme,
			ForwardHost:      npmHost.ForwardHost,
			ForwardPort:      port,
			SSLForced:        npmHost.SSLForced,
			WebsocketSupport: npmHost.AllowWebsocketUpgrade,
			Warnings:         warnings,
		}

		rawJSON, _ := json.Marshal(npmHost)
		host.RawJSON = string(rawJSON)

		result.Hosts = append(result.Hosts, host)
	}

	return result
}
