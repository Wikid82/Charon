package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/services"
)

// jsonImportSession stores the parsed content for a JSON import session.
type jsonImportSession struct {
	SourceType   string // "charon" or "npm"
	CharonExport *CharonExport
	NPMExport    *NPMExport
}

// jsonImportSessions holds parsed exports keyed by session UUID until they are committed, cancelled or expire.
var jsonImportSessions = newImportSessionStore[jsonImportSession](importSessionTTL, maxImportSessions)

// CharonExport represents the top-level structure of a Charon export file.
type CharonExport struct {
	Version     string             `json:"version"`
	ExportedAt  time.Time          `json:"exported_at"`
	ProxyHosts  []CharonProxyHost  `json:"proxy_hosts"`
	AccessLists []CharonAccessList `json:"access_lists"`
	DNSRecords  []CharonDNSRecord  `json:"dns_records"`
}

// CharonProxyHost represents a proxy host in Charon export format.
type CharonProxyHost struct {
	UUID             string `json:"uuid"`
	Name             string `json:"name"`
	DomainNames      string `json:"domain_names"`
	ForwardScheme    string `json:"forward_scheme"`
	ForwardHost      string `json:"forward_host"`
	ForwardPort      int    `json:"forward_port"`
	SSLForced        bool   `json:"ssl_forced"`
	HTTP2Support     bool   `json:"http2_support"`
	HSTSEnabled      bool   `json:"hsts_enabled"`
	HSTSSubdomains   bool   `json:"hsts_subdomains"`
	BlockExploits    bool   `json:"block_exploits"`
	WebsocketSupport bool   `json:"websocket_support"`
	Application      string `json:"application"`
	Enabled          bool   `json:"enabled"`
	AdvancedConfig   string `json:"advanced_config"`
	WAFDisabled      bool   `json:"waf_disabled"`
	UseDNSChallenge  bool   `json:"use_dns_challenge"`
}

// CharonAccessList represents an access list in Charon export format.
type CharonAccessList struct {
	UUID             string `json:"uuid"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Type             string `json:"type"`
	IPRules          string `json:"ip_rules"`
	CountryCodes     string `json:"country_codes"`
	LocalNetworkOnly bool   `json:"local_network_only"`
	Enabled          bool   `json:"enabled"`
}

// CharonDNSRecord represents a DNS record in Charon export format.
type CharonDNSRecord struct {
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Value      string `json:"value"`
	TTL        int    `json:"ttl"`
	ProviderID uint   `json:"provider_id"`
}

// JSONImportHandler handles JSON configuration imports (both Charon and NPM formats).
type JSONImportHandler struct {
	db           *gorm.DB
	proxyHostSvc *services.ProxyHostService
}

// NewJSONImportHandler creates a new JSON import handler.
func NewJSONImportHandler(db *gorm.DB) *JSONImportHandler {
	return &JSONImportHandler{
		db:           db,
		proxyHostSvc: services.NewProxyHostService(db),
	}
}

// RegisterRoutes registers JSON import routes.
func (h *JSONImportHandler) RegisterRoutes(router *gin.RouterGroup) {
	body := limitRequestBody(maxJSONImportBodyBytes)
	router.POST("/import/json/upload", body, h.Upload)
	router.POST("/import/json/commit", body, h.Commit)
	router.POST("/import/json/cancel", body, h.Cancel)
}

// Upload parses a JSON export (Charon or NPM format) and returns a preview.
func (h *JSONImportHandler) Upload(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if !bindJSON(c, &req) {
		return
	}

	// Try Charon format first
	var charonExport CharonExport
	if err := json.Unmarshal([]byte(req.Content), &charonExport); err == nil && h.isCharonFormat(charonExport) {
		h.handleCharonUpload(c, charonExport)
		return
	}

	// Fall back to NPM format
	var npmExport NPMExport
	if err := json.Unmarshal([]byte(req.Content), &npmExport); err == nil && len(npmExport.ProxyHosts) > 0 {
		h.handleNPMUpload(c, npmExport)
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": "unrecognized JSON format - must be Charon or NPM export"})
}

// isCharonFormat checks if the export is in Charon format.
func (h *JSONImportHandler) isCharonFormat(export CharonExport) bool {
	return export.Version != "" || len(export.ProxyHosts) > 0
}

// handleCharonUpload processes a Charon format export.
func (h *JSONImportHandler) handleCharonUpload(c *gin.Context, export CharonExport) {
	result := h.convertCharonToImportResult(export)

	if len(result.Hosts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no proxy hosts found in Charon export"})
		return
	}

	existingHosts, ok := listExistingHostsForImport(c, h.proxyHostSvc)
	if !ok {
		return
	}
	conflicts, conflictDetails := detectImportConflicts(existingHosts, result.Hosts)
	result.Conflicts = append(result.Conflicts, conflicts...)

	sid := uuid.NewString()

	// Store the parsed export in session storage for later commit
	jsonImportSessions.Put(sid, jsonImportSession{
		SourceType:   "charon",
		CharonExport: &export,
	})

	c.JSON(http.StatusOK, gin.H{
		"session":          gin.H{"id": sid, "state": "transient", "source_type": "charon"},
		"preview":          result,
		"conflict_details": conflictDetails,
		"charon_export": gin.H{
			"version":      export.Version,
			"exported_at":  export.ExportedAt,
			"proxy_hosts":  len(export.ProxyHosts),
			"access_lists": len(export.AccessLists),
			"dns_records":  len(export.DNSRecords),
		},
	})
}

// handleNPMUpload processes an NPM format export.
func (h *JSONImportHandler) handleNPMUpload(c *gin.Context, export NPMExport) {
	npmHandler := NewNPMImportHandler(h.db)
	result := npmHandler.convertNPMToImportResult(export)

	if len(result.Hosts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no proxy hosts found in NPM export"})
		return
	}

	existingHosts, ok := listExistingHostsForImport(c, h.proxyHostSvc)
	if !ok {
		return
	}
	conflicts, conflictDetails := detectImportConflicts(existingHosts, result.Hosts)
	result.Conflicts = append(result.Conflicts, conflicts...)

	sid := uuid.NewString()

	// Store the parsed export in session storage for later commit
	jsonImportSessions.Put(sid, jsonImportSession{
		SourceType: "npm",
		NPMExport:  &export,
	})

	c.JSON(http.StatusOK, gin.H{
		"session":          gin.H{"id": sid, "state": "transient", "source_type": "npm"},
		"preview":          result,
		"conflict_details": conflictDetails,
		"npm_export": gin.H{
			"proxy_hosts":  len(export.ProxyHosts),
			"access_lists": len(export.AccessLists),
			"certificates": len(export.Certificates),
		},
	})
}

// Commit finalizes the JSON import with user's conflict resolutions.
func (h *JSONImportHandler) Commit(c *gin.Context) {
	var req struct {
		SessionUUID string            `json:"session_uuid" binding:"required"`
		Resolutions map[string]string `json:"resolutions"`
		Names       map[string]string `json:"names"`
	}

	if !bindJSON(c, &req) {
		return
	}

	// Retrieve the stored session
	session, ok := jsonImportSessions.Get(req.SessionUUID)

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found or expired"})
		return
	}

	// Route to the appropriate commit handler based on source type
	if session.SourceType == "charon" && session.CharonExport != nil {
		h.commitCharonImport(c, *session.CharonExport, req.Resolutions, req.Names, req.SessionUUID)
		return
	}

	if session.SourceType == "npm" && session.NPMExport != nil {
		h.commitNPMImport(c, *session.NPMExport, req.Resolutions, req.Names, req.SessionUUID)
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session state"})
}

// Cancel cancels a JSON import session and cleans up resources.
func (h *JSONImportHandler) Cancel(c *gin.Context) {
	var req struct {
		SessionUUID string `json:"session_uuid"`
	}

	if !bindJSON(c, &req) {
		return
	}

	if strings.TrimSpace(req.SessionUUID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_uuid required"})
		return
	}

	// Clean up session if it exists
	jsonImportSessions.Delete(req.SessionUUID)

	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

// commitCharonImport commits a Charon format import.
func (h *JSONImportHandler) commitCharonImport(c *gin.Context, export CharonExport, resolutions, names map[string]string, sessionUUID string) {
	budget := services.NewImportNameBudget()
	defer budget.Close()
	result := h.convertCharonToImportResult(export)
	proxyHosts := caddy.ConvertToProxyHosts(result.Hosts)

	created := 0
	updated := 0
	skipped := 0
	errors := []string{}

	existingHosts, ok := listExistingHostsForImport(c, h.proxyHostSvc)
	if !ok {
		return
	}

	for _, host := range proxyHosts {
		action := resolutions[host.DomainNames]

		if customName, ok := names[host.DomainNames]; ok && customName != "" {
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
			if existing, found := findOverlappingHost(existingHosts, host.DomainNames); found {
				host.ID = existing.ID
				host.UUID = existing.UUID
				host.CertificateID = existing.CertificateID
				host.CreatedAt = existing.CreatedAt

				if err := h.proxyHostSvc.UpdateImported(budget, &host); err != nil {
					errors = append(errors, fmt.Sprintf("%s: %s", host.DomainNames, importErrorText(err)))
				} else {
					updated++
				}
				continue
			}
		}

		host.UUID = uuid.NewString()
		if err := h.proxyHostSvc.CreateImported(budget, &host); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", host.DomainNames, importErrorText(err)))
		} else {
			created++
		}
	}

	// Clean up session after successful commit
	jsonImportSessions.Delete(sessionUUID)

	c.JSON(http.StatusOK, gin.H{
		"created": created,
		"updated": updated,
		"skipped": skipped,
		"errors":  errors,
	})
}

// commitNPMImport commits an NPM format import.
func (h *JSONImportHandler) commitNPMImport(c *gin.Context, export NPMExport, resolutions, names map[string]string, sessionUUID string) {
	budget := services.NewImportNameBudget()
	defer budget.Close()
	npmHandler := NewNPMImportHandler(h.db)
	result := npmHandler.convertNPMToImportResult(export)
	proxyHosts := caddy.ConvertToProxyHosts(result.Hosts)

	created := 0
	updated := 0
	skipped := 0
	errors := []string{}

	existingHosts, ok := listExistingHostsForImport(c, h.proxyHostSvc)
	if !ok {
		return
	}

	for _, host := range proxyHosts {
		action := resolutions[host.DomainNames]

		if customName, ok := names[host.DomainNames]; ok && customName != "" {
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
			if existing, found := findOverlappingHost(existingHosts, host.DomainNames); found {
				host.ID = existing.ID
				host.UUID = existing.UUID
				host.CertificateID = existing.CertificateID
				host.CreatedAt = existing.CreatedAt

				if err := h.proxyHostSvc.UpdateImported(budget, &host); err != nil {
					errors = append(errors, fmt.Sprintf("%s: %s", host.DomainNames, importErrorText(err)))
				} else {
					updated++
				}
				continue
			}
		}

		host.UUID = uuid.NewString()
		if err := h.proxyHostSvc.CreateImported(budget, &host); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", host.DomainNames, importErrorText(err)))
		} else {
			created++
		}
	}

	// Clean up session after successful commit
	jsonImportSessions.Delete(sessionUUID)

	c.JSON(http.StatusOK, gin.H{
		"created": created,
		"updated": updated,
		"skipped": skipped,
		"errors":  errors,
	})
}

// convertCharonToImportResult converts Charon export format to ImportResult.
func (h *JSONImportHandler) convertCharonToImportResult(export CharonExport) *caddy.ImportResult {
	result := &caddy.ImportResult{
		Hosts:     []caddy.ParsedHost{},
		Conflicts: []string{},
		Errors:    []string{},
	}

	for _, ch := range export.ProxyHosts {
		if ch.DomainNames == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("host %s has no domain names", ch.UUID))
			continue
		}

		scheme := ch.ForwardScheme
		if scheme == "" {
			scheme = "http"
		}

		port := ch.ForwardPort
		if port == 0 {
			port = 80
		}

		warnings := []string{}
		if ch.AdvancedConfig != "" && !isValidJSON(ch.AdvancedConfig) {
			warnings = append(warnings, "Advanced config may need review")
		}

		host := caddy.ParsedHost{
			DomainNames:      ch.DomainNames,
			ForwardScheme:    scheme,
			ForwardHost:      ch.ForwardHost,
			ForwardPort:      port,
			SSLForced:        ch.SSLForced,
			WebsocketSupport: ch.WebsocketSupport,
			Warnings:         warnings,
		}

		rawJSON, _ := json.Marshal(ch)
		host.RawJSON = string(rawJSON)

		result.Hosts = append(result.Hosts, host)
	}

	return result
}

// isValidJSON checks if a string is valid JSON.
func isValidJSON(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}
