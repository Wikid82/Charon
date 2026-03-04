package handlers

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	securitypkg "github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
)

// WAFExclusionRequest represents a rule exclusion for false positives
type WAFExclusionRequest struct {
	RuleID      int    `json:"rule_id" binding:"required"`
	Target      string `json:"target,omitempty"`      // e.g., "ARGS:password"
	Description string `json:"description,omitempty"` // Human-readable reason
}

// WAFExclusion represents a stored rule exclusion
type WAFExclusion struct {
	RuleID      int    `json:"rule_id"`
	Target      string `json:"target,omitempty"`
	Description string `json:"description,omitempty"`
}

// SecurityHandler handles security-related API requests.
type SecurityHandler struct {
	cfg          config.SecurityConfig
	db           *gorm.DB
	svc          *services.SecurityService
	caddyManager *caddy.Manager
	geoipSvc     *services.GeoIPService
	cerberus     CacheInvalidator
}

// NewSecurityHandler creates a new SecurityHandler.
func NewSecurityHandler(cfg config.SecurityConfig, db *gorm.DB, caddyManager *caddy.Manager) *SecurityHandler {
	svc := services.NewSecurityService(db)
	return &SecurityHandler{cfg: cfg, db: db, svc: svc, caddyManager: caddyManager}
}

// NewSecurityHandlerWithDeps creates a new SecurityHandler with optional cache invalidation.
func NewSecurityHandlerWithDeps(cfg config.SecurityConfig, db *gorm.DB, caddyManager *caddy.Manager, cerberus CacheInvalidator) *SecurityHandler {
	svc := services.NewSecurityService(db)
	return &SecurityHandler{cfg: cfg, db: db, svc: svc, caddyManager: caddyManager, cerberus: cerberus}
}

// SetGeoIPService sets the GeoIP service for the handler.
func (h *SecurityHandler) SetGeoIPService(geoipSvc *services.GeoIPService) {
	h.geoipSvc = geoipSvc
}

// GetStatus returns the current status of all security services.
// Priority chain:
// 1. Settings table (highest - runtime overrides)
// 2. SecurityConfig DB record (middle - user configuration)
// 3. Static config (lowest - defaults)
func (h *SecurityHandler) GetStatus(c *gin.Context) {
	// Start with static config defaults
	enabled := h.cfg.CerberusEnabled
	wafMode := h.cfg.WAFMode
	rateLimitMode := h.cfg.RateLimitMode
	crowdSecMode := h.cfg.CrowdSecMode
	crowdSecAPIURL := h.cfg.CrowdSecAPIURL
	aclMode := h.cfg.ACLMode

	// Override with database SecurityConfig if present (priority 2)
	if h.db != nil {
		var sc models.SecurityConfig
		if err := h.db.Where("name = ?", "default").First(&sc).Error; err == nil {
			// SecurityConfig in DB takes precedence over static config
			enabled = sc.Enabled
			if sc.WAFMode != "" {
				wafMode = sc.WAFMode
			}
			if sc.RateLimitMode != "" {
				rateLimitMode = sc.RateLimitMode
			} else if sc.RateLimitEnable {
				rateLimitMode = "enabled"
			}
			if sc.CrowdSecMode != "" {
				crowdSecMode = sc.CrowdSecMode
			}
			if sc.CrowdSecAPIURL != "" {
				crowdSecAPIURL = sc.CrowdSecAPIURL
			}
		}

		// Check runtime setting overrides from settings table (priority 1 - highest)
		var setting struct{ Value string }

		// Cerberus enabled override
		cerberusOverrideApplied := false
		if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "feature.cerberus.enabled").Scan(&setting).Error; err == nil && setting.Value != "" {
			enabled = strings.EqualFold(setting.Value, "true")
			cerberusOverrideApplied = true
		}

		// Backward-compatible Cerberus enabled override
		if !cerberusOverrideApplied {
			setting = struct{ Value string }{}
			if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.cerberus.enabled").Scan(&setting).Error; err == nil && setting.Value != "" {
				enabled = strings.EqualFold(setting.Value, "true")
			}
		}

		// WAF enabled override
		setting = struct{ Value string }{}
		if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.waf.enabled").Scan(&setting).Error; err == nil && setting.Value != "" {
			if strings.EqualFold(setting.Value, "true") {
				wafMode = "enabled"
			} else {
				wafMode = "disabled"
			}
		}

		// Rate Limit enabled override
		setting = struct{ Value string }{}
		if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.rate_limit.enabled").Scan(&setting).Error; err == nil && setting.Value != "" {
			if strings.EqualFold(setting.Value, "true") {
				rateLimitMode = "enabled"
			} else {
				rateLimitMode = "disabled"
			}
		}

		// CrowdSec enabled override
		crowdSecEnabledOverride := false
		setting = struct{ Value string }{}
		if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.enabled").Scan(&setting).Error; err == nil && setting.Value != "" {
			crowdSecEnabledOverride = true
			if strings.EqualFold(setting.Value, "true") {
				crowdSecMode = "local"
			} else {
				crowdSecMode = "disabled"
			}
		}

		// CrowdSec mode override (deprecated - only applies when enabled override is absent)
		if !crowdSecEnabledOverride {
			setting = struct{ Value string }{}
			if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.mode").Scan(&setting).Error; err == nil && setting.Value != "" {
				crowdSecMode = setting.Value
			}
		}

		// ACL enabled override
		setting = struct{ Value string }{}
		if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.acl.enabled").Scan(&setting).Error; err == nil && setting.Value != "" {
			if strings.EqualFold(setting.Value, "true") {
				aclMode = "enabled"
			} else {
				aclMode = "disabled"
			}
		}
	}

	// Map unknown/external mode to disabled
	if crowdSecMode != "local" && crowdSecMode != "disabled" {
		crowdSecMode = "disabled"
	}

	// Compute effective enabled state for each feature
	wafEnabled := wafMode != "" && wafMode != "disabled"
	rateLimitEnabled := rateLimitMode == "enabled"
	crowdsecEnabled := crowdSecMode == "local"
	aclEnabled := aclMode == "enabled"

	// All features require Cerberus to be enabled
	if !enabled {
		wafEnabled = false
		rateLimitEnabled = false
		crowdsecEnabled = false
		aclEnabled = false
		wafMode = "disabled"
		rateLimitMode = "disabled"
		crowdSecMode = "disabled"
		aclMode = "disabled"
	}

	c.JSON(http.StatusOK, gin.H{
		"cerberus": gin.H{"enabled": enabled},
		"crowdsec": gin.H{
			"mode":    crowdSecMode,
			"api_url": crowdSecAPIURL,
			"enabled": crowdsecEnabled,
		},
		"waf": gin.H{
			"mode":    wafMode,
			"enabled": wafEnabled,
		},
		"rate_limit": gin.H{
			"mode":    rateLimitMode,
			"enabled": rateLimitEnabled,
		},
		"acl": gin.H{
			"mode":    aclMode,
			"enabled": aclEnabled,
		},
		"config_apply": latestConfigApplyState(h.db),
	})
}

func latestConfigApplyState(db *gorm.DB) gin.H {
	state := gin.H{
		"available": false,
		"status":    "unknown",
	}

	if db == nil {
		return state
	}

	var latest models.CaddyConfig
	err := db.Order("applied_at desc").First(&latest).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return state
		}
		return state
	}

	status := "failed"
	if latest.Success {
		status = "applied"
	}

	state["available"] = true
	state["status"] = status
	state["success"] = latest.Success
	state["applied_at"] = latest.AppliedAt
	state["error_msg"] = latest.ErrorMsg

	return state
}

// GetConfig returns the site security configuration from DB or default
func (h *SecurityHandler) GetConfig(c *gin.Context) {
	cfg, err := h.svc.Get()
	if err != nil {
		if err == services.ErrSecurityConfigNotFound {
			c.JSON(http.StatusOK, gin.H{"config": nil})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read security config"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"config": cfg})
}

// UpdateConfig creates or updates the SecurityConfig in DB
func (h *SecurityHandler) UpdateConfig(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var payload models.SecurityConfig
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if payload.Name == "" {
		payload.Name = "default"
	}
	// Sync RateLimitMode with RateLimitEnable for backward compatibility
	if payload.RateLimitEnable {
		payload.RateLimitMode = "enabled"
	} else if payload.RateLimitMode == "" {
		payload.RateLimitMode = "disabled"
	}
	if err := h.svc.Upsert(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Apply updated config to Caddy so WAF mode changes take effect
	if h.caddyManager != nil {
		if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
			log.WithError(err).Warn("failed to apply security config changes to Caddy")
		}
	}
	c.JSON(http.StatusOK, gin.H{"config": payload})
}

// GenerateBreakGlass generates a break-glass token and returns the plaintext token once
func (h *SecurityHandler) GenerateBreakGlass(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	token, err := h.svc.GenerateBreakGlassToken("default")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate break-glass token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

// ListDecisions returns recent security decisions
func (h *SecurityHandler) ListDecisions(c *gin.Context) {
	limit := 50
	if q := c.Query("limit"); q != "" {
		if v, err := strconv.Atoi(q); err == nil {
			limit = v
		}
	}
	list, err := h.svc.ListDecisions(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list decisions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"decisions": list})
}

// CreateDecision creates a manual decision (override) - for now no checks besides payload
func (h *SecurityHandler) CreateDecision(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var payload models.SecurityDecision
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if payload.IP == "" || payload.Action == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ip and action are required"})
		return
	}

	// CRITICAL: Validate IP format to prevent SQL injection via IP field
	// Must accept both single IPs and CIDR ranges
	if !isValidIP(payload.IP) && !isValidCIDR(payload.IP) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid IP address format"})
		return
	}

	// CRITICAL: Validate action enum
	// Only accept known action types to prevent injection via action field
	validActions := []string{"block", "allow", "captcha"}
	if !contains(validActions, payload.Action) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action"})
		return
	}

	// Sanitize details field (limit length, strip control characters)
	payload.Details = sanitizeString(payload.Details, 1000)

	// Populate source
	payload.Source = "manual"
	if err := h.svc.LogDecision(&payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to log decision"})
		return
	}
	// Record an audit entry
	actor := c.GetString("user_id")
	if actor == "" {
		actor = c.ClientIP()
	}
	_ = h.svc.LogAudit(&models.SecurityAudit{Actor: actor, Action: "create_decision", Details: payload.Details})
	c.JSON(http.StatusOK, gin.H{"decision": payload})
}

// ListRuleSets returns the list of known rulesets
func (h *SecurityHandler) ListRuleSets(c *gin.Context) {
	list, err := h.svc.ListRuleSets()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list rule sets"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rulesets": list})
}

// UpsertRuleSet uploads or updates a ruleset
func (h *SecurityHandler) UpsertRuleSet(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var payload models.SecurityRuleSet
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if payload.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	if err := h.svc.UpsertRuleSet(&payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upsert ruleset"})
		return
	}
	if h.caddyManager != nil {
		if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply configuration: " + err.Error()})
			return
		}
	}
	// Create an audit event
	actor := c.GetString("user_id")
	if actor == "" {
		actor = c.ClientIP()
	}
	_ = h.svc.LogAudit(&models.SecurityAudit{Actor: actor, Action: "upsert_ruleset", Details: payload.Name})
	c.JSON(http.StatusOK, gin.H{"ruleset": payload})
}

// DeleteRuleSet removes a ruleset by id
func (h *SecurityHandler) DeleteRuleSet(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	idParam := c.Param("id")
	if idParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.svc.DeleteRuleSet(uint(id)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "ruleset not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete ruleset"})
		return
	}
	if h.caddyManager != nil {
		if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply configuration: " + err.Error()})
			return
		}
	}
	actor := c.GetString("user_id")
	if actor == "" {
		actor = c.ClientIP()
	}
	_ = h.svc.LogAudit(&models.SecurityAudit{Actor: actor, Action: "delete_ruleset", Details: idParam})
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// Enable toggles Cerberus on, validating admin whitelist or break-glass token
func (h *SecurityHandler) Enable(c *gin.Context) {
	// Look for requester's IP and optional breakglass token
	adminIP := c.ClientIP()
	var body struct {
		Token string `json:"break_glass_token"`
	}
	_ = c.ShouldBindJSON(&body)

	// If config exists, require that adminIP is in whitelist or token matches
	cfg, err := h.svc.Get()
	if err != nil && err != services.ErrSecurityConfigNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve security config"})
		return
	}
	if cfg != nil {
		// Check admin whitelist
		if cfg.AdminWhitelist == "" && body.Token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "admin whitelist missing; provide break_glass_token or add admin_whitelist CIDR before enabling"})
			return
		}
		if body.Token != "" {
			ok, err := h.svc.VerifyBreakGlassToken(cfg.Name, body.Token)
			if err == nil && ok {
				// proceed
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "break glass token invalid"})
				return
			}
		} else {
			// verify client IP in admin whitelist
			found := false
			for _, entry := range strings.Split(cfg.AdminWhitelist, ",") {
				entry = strings.TrimSpace(entry)
				if entry == "" {
					continue
				}
				if entry == adminIP {
					found = true
					break
				}
				// If CIDR, check contains
				if _, cidr, err := net.ParseCIDR(entry); err == nil {
					if cidr.Contains(net.ParseIP(adminIP)) {
						found = true
						break
					}
				}
			}
			if !found {
				c.JSON(http.StatusForbidden, gin.H{"error": "admin IP not present in admin_whitelist"})
				return
			}
		}
	}
	// Set enabled true
	newCfg := &models.SecurityConfig{Name: "default", Enabled: true}
	if cfg != nil {
		newCfg = cfg
		newCfg.Enabled = true
	}
	if err := h.svc.Upsert(newCfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enable Cerberus"})
		return
	}
	if h.caddyManager != nil {
		if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply configuration: " + err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"enabled": true})
}

// Disable toggles Cerberus off; requires break-glass token or localhost request
func (h *SecurityHandler) Disable(c *gin.Context) {
	var body struct {
		Token string `json:"break_glass_token"`
	}
	_ = c.ShouldBindJSON(&body)
	// Allow requests from localhost to disable without token
	clientIP := c.ClientIP()
	if clientIP == "127.0.0.1" || clientIP == "::1" {
		cfg, _ := h.svc.Get()
		if cfg == nil {
			cfg = &models.SecurityConfig{Name: "default", Enabled: false}
		} else {
			cfg.Enabled = false
		}
		_ = h.svc.Upsert(cfg)
		if h.caddyManager != nil {
			_ = h.caddyManager.ApplyConfig(c.Request.Context())
		}
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}
	cfg, err := h.svc.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read config"})
		return
	}
	if body.Token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "break glass token required to disable Cerberus from non-localhost"})
		return
	}
	ok, err := h.svc.VerifyBreakGlassToken(cfg.Name, body.Token)
	if err != nil || !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "break glass token invalid"})
		return
	}
	cfg.Enabled = false
	_ = h.svc.Upsert(cfg)
	if h.caddyManager != nil {
		_ = h.caddyManager.ApplyConfig(c.Request.Context())
	}
	c.JSON(http.StatusOK, gin.H{"enabled": false})
}

// GetRateLimitPresets returns predefined rate limit configurations
func (h *SecurityHandler) GetRateLimitPresets(c *gin.Context) {
	presets := []map[string]any{
		{
			"id":          "standard",
			"name":        "Standard Web",
			"description": "Balanced protection for general web applications",
			"requests":    100,
			"window_sec":  60,
			"burst":       20,
		},
		{
			"id":          "api",
			"name":        "API Protection",
			"description": "Stricter limits for API endpoints",
			"requests":    30,
			"window_sec":  60,
			"burst":       10,
		},
		{
			"id":          "login",
			"name":        "Login Protection",
			"description": "Aggressive protection against brute-force",
			"requests":    5,
			"window_sec":  300,
			"burst":       2,
		},
		{
			"id":          "relaxed",
			"name":        "High Traffic",
			"description": "Higher limits for trusted, high-traffic apps",
			"requests":    500,
			"window_sec":  60,
			"burst":       100,
		},
	}
	c.JSON(http.StatusOK, gin.H{"presets": presets})
}

// GetGeoIPStatus returns the current status of the GeoIP service.
func (h *SecurityHandler) GetGeoIPStatus(c *gin.Context) {
	if h.geoipSvc == nil {
		c.JSON(http.StatusOK, gin.H{
			"loaded":  false,
			"message": "GeoIP service not initialized",
			"db_path": "",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"loaded":  h.geoipSvc.IsLoaded(),
		"db_path": h.geoipSvc.GetDatabasePath(),
		"message": "GeoIP service available",
	})
}

// ReloadGeoIP reloads the GeoIP database from disk.
func (h *SecurityHandler) ReloadGeoIP(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	if h.geoipSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "GeoIP service not initialized",
		})
		return
	}

	if err := h.geoipSvc.Load(); err != nil {
		log.WithError(err).Error("Failed to reload GeoIP database")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to reload GeoIP database: " + err.Error(),
		})
		return
	}

	// Log audit event
	actor := c.GetString("user_id")
	if actor == "" {
		actor = c.ClientIP()
	}
	_ = h.svc.LogAudit(&models.SecurityAudit{Actor: actor, Action: "reload_geoip", Details: "GeoIP database reloaded successfully"})

	c.JSON(http.StatusOK, gin.H{
		"message": "GeoIP database reloaded successfully",
		"loaded":  h.geoipSvc.IsLoaded(),
		"db_path": h.geoipSvc.GetDatabasePath(),
	})
}

// LookupGeoIP performs a GeoIP lookup for a given IP address.
func (h *SecurityHandler) LookupGeoIP(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var req struct {
		IPAddress string `json:"ip_address" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ip_address is required"})
		return
	}

	if h.geoipSvc == nil || !h.geoipSvc.IsLoaded() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "GeoIP service not available",
		})
		return
	}

	country, err := h.geoipSvc.LookupCountry(req.IPAddress)
	if err != nil {
		if errors.Is(err, services.ErrInvalidGeoIP) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid IP address"})
			return
		}
		if errors.Is(err, services.ErrCountryNotFound) {
			c.JSON(http.StatusOK, gin.H{
				"ip_address":   req.IPAddress,
				"country_code": "",
				"found":        false,
				"message":      "No country found for this IP address",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "GeoIP lookup failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ip_address":   req.IPAddress,
		"country_code": country,
		"found":        true,
	})
}

// GetWAFExclusions returns current WAF rule exclusions from SecurityConfig
func (h *SecurityHandler) GetWAFExclusions(c *gin.Context) {
	cfg, err := h.svc.Get()
	if err != nil {
		if err == services.ErrSecurityConfigNotFound {
			c.JSON(http.StatusOK, gin.H{"exclusions": []WAFExclusion{}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read security config"})
		return
	}

	var exclusions []WAFExclusion
	if cfg.WAFExclusions != "" {
		if err := json.Unmarshal([]byte(cfg.WAFExclusions), &exclusions); err != nil {
			log.WithError(err).Warn("Failed to parse WAF exclusions")
			exclusions = []WAFExclusion{}
		}
	}

	c.JSON(http.StatusOK, gin.H{"exclusions": exclusions})
}

// AddWAFExclusion adds a rule exclusion to the WAF configuration
func (h *SecurityHandler) AddWAFExclusion(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var req WAFExclusionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rule_id is required"})
		return
	}

	if req.RuleID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rule_id must be a positive integer"})
		return
	}

	cfg, err := h.svc.Get()
	if err != nil {
		if err == services.ErrSecurityConfigNotFound {
			// Create default config with the exclusion
			cfg = &models.SecurityConfig{Name: "default"}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read security config"})
			return
		}
	}

	// Parse existing exclusions
	var exclusions []WAFExclusion
	if cfg.WAFExclusions != "" {
		if unmarshalErr := json.Unmarshal([]byte(cfg.WAFExclusions), &exclusions); unmarshalErr != nil {
			log.WithError(unmarshalErr).Warn("Failed to parse existing WAF exclusions")
			exclusions = []WAFExclusion{}
		}
	}

	// Check for duplicate rule_id with same target
	for _, e := range exclusions {
		if e.RuleID == req.RuleID && e.Target == req.Target {
			c.JSON(http.StatusConflict, gin.H{"error": "exclusion for this rule_id and target already exists"})
			return
		}
	}

	// Add the new exclusion - convert request to WAFExclusion type
	newExclusion := WAFExclusion(req)
	exclusions = append(exclusions, newExclusion)

	// Marshal back to JSON
	exclusionsJSON, err := json.Marshal(exclusions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize exclusions"})
		return
	}

	cfg.WAFExclusions = string(exclusionsJSON)
	if err := h.svc.Upsert(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save exclusion"})
		return
	}

	// Apply updated config to Caddy
	if h.caddyManager != nil {
		if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
			log.WithError(err).Warn("failed to apply WAF exclusion changes to Caddy")
		}
	}

	// Log audit event
	actor := c.GetString("user_id")
	if actor == "" {
		actor = c.ClientIP()
	}
	_ = h.svc.LogAudit(&models.SecurityAudit{
		Actor:   actor,
		Action:  "add_waf_exclusion",
		Details: strconv.Itoa(req.RuleID),
	})

	c.JSON(http.StatusOK, gin.H{"exclusion": newExclusion})
}

// DeleteWAFExclusion removes a rule exclusion by rule_id
func (h *SecurityHandler) DeleteWAFExclusion(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	ruleIDParam := c.Param("rule_id")
	if ruleIDParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rule_id is required"})
		return
	}

	ruleID, err := strconv.Atoi(ruleIDParam)
	if err != nil || ruleID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule_id"})
		return
	}

	// Get optional target query parameter (for exclusions with specific targets)
	target := c.Query("target")

	cfg, err := h.svc.Get()
	if err != nil {
		if err == services.ErrSecurityConfigNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "exclusion not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read security config"})
		return
	}

	// Parse existing exclusions
	var exclusions []WAFExclusion
	if cfg.WAFExclusions != "" {
		if unmarshalErr := json.Unmarshal([]byte(cfg.WAFExclusions), &exclusions); unmarshalErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse exclusions"})
			return
		}
	}

	// Find and remove the exclusion
	found := false
	newExclusions := make([]WAFExclusion, 0, len(exclusions))
	for _, e := range exclusions {
		// Match by rule_id and target (empty target matches exclusions without target)
		if e.RuleID == ruleID && e.Target == target {
			found = true
			continue // Skip this one (delete it)
		}
		newExclusions = append(newExclusions, e)
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "exclusion not found"})
		return
	}

	// Marshal back to JSON
	exclusionsJSON, err := json.Marshal(newExclusions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize exclusions"})
		return
	}

	cfg.WAFExclusions = string(exclusionsJSON)
	if err := h.svc.Upsert(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save exclusions"})
		return
	}

	// Apply updated config to Caddy
	if h.caddyManager != nil {
		if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
			log.WithError(err).Warn("failed to apply WAF exclusion changes to Caddy")
		}
	}

	// Log audit event
	actor := c.GetString("user_id")
	if actor == "" {
		actor = c.ClientIP()
	}
	_ = h.svc.LogAudit(&models.SecurityAudit{
		Actor:   actor,
		Action:  "delete_waf_exclusion",
		Details: ruleIDParam,
	})

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// isValidIP validates that s is a valid IPv4 or IPv6 address
func isValidIP(s string) bool {
	return net.ParseIP(s) != nil
}

// isValidCIDR validates that s is a valid CIDR notation
func isValidCIDR(s string) bool {
	_, _, err := net.ParseCIDR(s)
	return err == nil
}

// contains checks if a string exists in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// sanitizeString removes control characters and enforces max length
func sanitizeString(s string, maxLen int) string {
	// Remove null bytes and other control characters
	s = strings.Map(func(r rune) rune {
		if r == 0 || (r < 32 && r != '\n' && r != '\r' && r != '\t') {
			return -1 // Remove character
		}
		return r
	}, s)

	// Enforce max length
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

// Security module enable/disable endpoints (Phase 2)
// These endpoints allow granular control over individual security modules

// EnableACL enables the  Access Control List security module
// POST /api/v1/security/acl/enable
func (h *SecurityHandler) EnableACL(c *gin.Context) {
	h.toggleSecurityModule(c, "security.acl.enabled", true)
}

// DisableACL disables the Access Control List security module
// POST /api/v1/security/acl/disable
func (h *SecurityHandler) DisableACL(c *gin.Context) {
	h.toggleSecurityModule(c, "security.acl.enabled", false)
}

// PatchACL handles PATCH requests to enable/disable ACL based on JSON body
// PATCH /api/v1/security/acl
// Expects: {"enabled": true/false}
func (h *SecurityHandler) PatchACL(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	h.toggleSecurityModule(c, "security.acl.enabled", req.Enabled)
}

// EnableWAF enables the Web Application Firewall security module
// POST /api/v1/security/waf/enable
func (h *SecurityHandler) EnableWAF(c *gin.Context) {
	h.toggleSecurityModule(c, "security.waf.enabled", true)
}

// DisableWAF disables the Web Application Firewall security module
// POST /api/v1/security/waf/disable
func (h *SecurityHandler) DisableWAF(c *gin.Context) {
	h.toggleSecurityModule(c, "security.waf.enabled", false)
}

// PatchWAF handles PATCH requests to enable/disable WAF based on JSON body
// PATCH /api/v1/security/waf
// Expects: {"enabled": true/false}
func (h *SecurityHandler) PatchWAF(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	h.toggleSecurityModule(c, "security.waf.enabled", req.Enabled)
}

// EnableCerberus enables the Cerberus security monitoring module
// POST /api/v1/security/cerberus/enable
func (h *SecurityHandler) EnableCerberus(c *gin.Context) {
	h.toggleSecurityModule(c, "feature.cerberus.enabled", true)
}

// DisableCerberus disables the Cerberus security monitoring module
// POST /api/v1/security/cerberus/disable
func (h *SecurityHandler) DisableCerberus(c *gin.Context) {
	h.toggleSecurityModule(c, "feature.cerberus.enabled", false)
}

// EnableCrowdSec enables the CrowdSec security module
// POST /api/v1/security/crowdsec/enable
func (h *SecurityHandler) EnableCrowdSec(c *gin.Context) {
	h.toggleSecurityModule(c, "security.crowdsec.enabled", true)
}

// DisableCrowdSec disables the CrowdSec security module
// POST /api/v1/security/crowdsec/disable
func (h *SecurityHandler) DisableCrowdSec(c *gin.Context) {
	h.toggleSecurityModule(c, "security.crowdsec.enabled", false)
}

// PatchCrowdSec handles PATCH requests to enable/disable CrowdSec based on JSON body
// PATCH /api/v1/security/crowdsec
// Expects: {"enabled": true/false}
func (h *SecurityHandler) PatchCrowdSec(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	h.toggleSecurityModule(c, "security.crowdsec.enabled", req.Enabled)
}

// EnableRateLimit enables the Rate Limiting security module
// POST /api/v1/security/rate-limit/enable
func (h *SecurityHandler) EnableRateLimit(c *gin.Context) {
	h.toggleSecurityModule(c, "security.rate_limit.enabled", true)
}

// DisableRateLimit disables the Rate Limiting security module
// POST /api/v1/security/rate-limit/disable
func (h *SecurityHandler) DisableRateLimit(c *gin.Context) {
	h.toggleSecurityModule(c, "security.rate_limit.enabled", false)
}

// PatchRateLimit handles PATCH requests to enable/disable Rate Limiting based on JSON body
// PATCH /api/v1/security/rate-limit
// Expects: {"enabled": true/false}
func (h *SecurityHandler) PatchRateLimit(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	h.toggleSecurityModule(c, "security.rate_limit.enabled", req.Enabled)
}

// toggleSecurityModule is a helper function that handles enabling/disabling security modules
// It updates the setting, invalidates cache, and triggers Caddy config reload
func (h *SecurityHandler) toggleSecurityModule(c *gin.Context, settingKey string, enabled bool) {
	if !requireAdmin(c) {
		return
	}

	settingCategory := "security"
	if strings.HasPrefix(settingKey, "feature.") {
		settingCategory = "feature"
	}

	snapshotKeys := []string{settingKey}
	if enabled && settingKey != "feature.cerberus.enabled" {
		snapshotKeys = append(snapshotKeys, "feature.cerberus.enabled", "security.cerberus.enabled")
	}

	settingSnapshots, err := h.snapshotSettings(snapshotKeys)
	if err != nil {
		log.WithError(err).Error("Failed to snapshot security settings before toggle")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update security module"})
		return
	}

	securityConfigExistsBefore, securityConfigEnabledBefore, err := h.snapshotDefaultSecurityConfigState()
	if err != nil {
		log.WithError(err).Error("Failed to snapshot security config before toggle")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update security module"})
		return
	}

	if settingKey == "security.acl.enabled" && enabled {
		if !h.allowACLEnable(c) {
			return
		}
	}

	if enabled && settingKey != "feature.cerberus.enabled" {
		if err := h.ensureSecurityConfigEnabled(); err != nil {
			log.WithError(err).Error("Failed to enable SecurityConfig while enabling security module")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable security config"})
			return
		}

		cerberusSetting := models.Setting{
			Key:      "feature.cerberus.enabled",
			Value:    "true",
			Category: "feature",
			Type:     "bool",
		}
		if err := h.db.Where(models.Setting{Key: cerberusSetting.Key}).Assign(cerberusSetting).FirstOrCreate(&cerberusSetting).Error; err != nil {
			log.WithError(err).Error("Failed to enable Cerberus while enabling security module")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable Cerberus"})
			return
		}

		legacyCerberus := models.Setting{
			Key:      "security.cerberus.enabled",
			Value:    "true",
			Category: "security",
			Type:     "bool",
		}
		if err := h.db.Where(models.Setting{Key: legacyCerberus.Key}).Assign(legacyCerberus).FirstOrCreate(&legacyCerberus).Error; err != nil {
			log.WithError(err).Error("Failed to enable legacy Cerberus while enabling security module")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable Cerberus"})
			return
		}
	}

	if settingKey == "security.acl.enabled" && enabled {
		if err := h.ensureSecurityConfigEnabled(); err != nil {
			log.WithError(err).Error("Failed to enable SecurityConfig while enabling ACL")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable security config"})
			return
		}
		cerberusSetting := models.Setting{
			Key:      "feature.cerberus.enabled",
			Value:    "true",
			Category: "feature",
			Type:     "bool",
		}
		if err := h.db.Where(models.Setting{Key: cerberusSetting.Key}).Assign(cerberusSetting).FirstOrCreate(&cerberusSetting).Error; err != nil {
			log.WithError(err).Error("Failed to enable Cerberus while enabling ACL")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable Cerberus"})
			return
		}
		legacyCerberus := models.Setting{
			Key:      "security.cerberus.enabled",
			Value:    "true",
			Category: "security",
			Type:     "bool",
		}
		if err := h.db.Where(models.Setting{Key: legacyCerberus.Key}).Assign(legacyCerberus).FirstOrCreate(&legacyCerberus).Error; err != nil {
			log.WithError(err).Error("Failed to enable legacy Cerberus while enabling ACL")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable Cerberus"})
			return
		}
	}

	// Update setting
	value := "false"
	if enabled {
		value = "true"
	}

	setting := models.Setting{
		Key:      settingKey,
		Value:    value,
		Category: settingCategory,
		Type:     "bool",
	}

	if err := h.db.Where(models.Setting{Key: settingKey}).Assign(setting).FirstOrCreate(&setting).Error; err != nil {
		log.WithError(err).Errorf("Failed to update setting %s", settingKey)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update security module"})
		return
	}

	if settingKey == "feature.cerberus.enabled" {
		legacyCerberus := models.Setting{
			Key:      "security.cerberus.enabled",
			Value:    value,
			Category: "security",
			Type:     "bool",
		}
		if err := h.db.Where(models.Setting{Key: legacyCerberus.Key}).Assign(legacyCerberus).FirstOrCreate(&legacyCerberus).Error; err != nil {
			log.WithError(err).Error("Failed to sync legacy Cerberus setting")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update security module"})
			return
		}
	}

	if settingKey == "security.acl.enabled" && enabled {
		var count int64
		if err := h.db.Model(&models.SecurityConfig{}).Count(&count).Error; err != nil {
			log.WithError(err).Error("Failed to count security configs after enabling ACL")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable security config"})
			return
		}
		if count == 0 {
			cfg := models.SecurityConfig{Name: "default", Enabled: true}
			if err := h.db.Create(&cfg).Error; err != nil {
				log.WithError(err).Error("Failed to create security config after enabling ACL")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable security config"})
				return
			}
		} else {
			if err := h.db.Model(&models.SecurityConfig{}).Where("name = ?", "default").Update("enabled", true).Error; err != nil {
				log.WithError(err).Error("Failed to update security config after enabling ACL")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable security config"})
				return
			}
		}
	}

	if h.cerberus != nil {
		h.cerberus.InvalidateCache()
	}

	// Trigger Caddy config reload
	if h.caddyManager != nil {
		if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
			log.WithError(err).Warn("Failed to reload Caddy config after security module toggle")
			if restoreErr := h.restoreSettings(settingSnapshots); restoreErr != nil {
				log.WithError(restoreErr).Error("Failed to restore settings after security module toggle apply failure")
			}
			if restoreErr := h.restoreDefaultSecurityConfigState(securityConfigExistsBefore, securityConfigEnabledBefore); restoreErr != nil {
				log.WithError(restoreErr).Error("Failed to restore security config after security module toggle apply failure")
			}
			if h.cerberus != nil {
				h.cerberus.InvalidateCache()
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload configuration"})
			return
		}
	}

	log.Info("Security module toggled")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"module":  settingKey,
		"enabled": enabled,
		"applied": true,
	})
}

type settingSnapshot struct {
	exists  bool
	setting models.Setting
}

func (h *SecurityHandler) snapshotSettings(keys []string) (map[string]settingSnapshot, error) {
	snapshots := make(map[string]settingSnapshot, len(keys))
	for _, key := range keys {
		if _, exists := snapshots[key]; exists {
			continue
		}

		var existing models.Setting
		err := h.db.Where("key = ?", key).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			snapshots[key] = settingSnapshot{exists: false}
			continue
		}
		if err != nil {
			return nil, err
		}

		snapshots[key] = settingSnapshot{exists: true, setting: existing}
	}

	return snapshots, nil
}

func (h *SecurityHandler) restoreSettings(snapshots map[string]settingSnapshot) error {
	for key, snapshot := range snapshots {
		if snapshot.exists {
			restore := snapshot.setting
			if err := h.db.Where(models.Setting{Key: key}).Assign(restore).FirstOrCreate(&restore).Error; err != nil {
				return err
			}
			continue
		}

		if err := h.db.Where("key = ?", key).Delete(&models.Setting{}).Error; err != nil {
			return err
		}
	}

	return nil
}

func (h *SecurityHandler) snapshotDefaultSecurityConfigState() (bool, bool, error) {
	var cfg models.SecurityConfig
	err := h.db.Where("name = ?", "default").First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}

	return true, cfg.Enabled, nil
}

func (h *SecurityHandler) restoreDefaultSecurityConfigState(exists bool, enabled bool) error {
	if exists {
		return h.db.Model(&models.SecurityConfig{}).Where("name = ?", "default").Update("enabled", enabled).Error
	}

	return h.db.Where("name = ?", "default").Delete(&models.SecurityConfig{}).Error
}

func (h *SecurityHandler) ensureSecurityConfigEnabled() error {
	if h.db == nil {
		return errors.New("security config database not configured")
	}
	cfg := models.SecurityConfig{Name: "default", Enabled: true}
	if err := h.db.Where("name = ?", "default").FirstOrCreate(&cfg).Error; err != nil {
		return err
	}
	if cfg.Enabled {
		return nil
	}
	return h.db.Model(&cfg).Update("enabled", true).Error
}

func (h *SecurityHandler) allowACLEnable(c *gin.Context) bool {
	if bypass, exists := c.Get("emergency_bypass"); exists {
		if bypassActive, ok := bypass.(bool); ok && bypassActive {
			return true
		}
	}

	cfg, err := h.svc.Get()
	if err != nil {
		if errors.Is(err, services.ErrSecurityConfigNotFound) {
			return true
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read security config"})
		return false
	}

	whitelist := strings.TrimSpace(cfg.AdminWhitelist)
	if whitelist == "" {
		return true
	}

	clientIP := util.CanonicalizeIPForSecurity(c.ClientIP())
	if securitypkg.IsIPInCIDRList(clientIP, whitelist) {
		return true
	}

	c.JSON(http.StatusForbidden, gin.H{"error": "admin IP not present in admin_whitelist"})
	return false
}
