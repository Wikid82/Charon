package handlers

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// SecurityHandler handles security-related API requests.
type SecurityHandler struct {
	cfg config.SecurityConfig
	db  *gorm.DB
	svc *services.SecurityService
}

// NewSecurityHandler creates a new SecurityHandler.
func NewSecurityHandler(cfg config.SecurityConfig, db *gorm.DB) *SecurityHandler {
	svc := services.NewSecurityService(db)
	return &SecurityHandler{cfg: cfg, db: db, svc: svc}
}

// GetStatus returns the current status of all security services.
func (h *SecurityHandler) GetStatus(c *gin.Context) {
	enabled := h.cfg.CerberusEnabled
	// Check runtime setting override
	var settingKey = "security.cerberus.enabled"
	if h.db != nil {
		var setting struct{ Value string }
		if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", settingKey).Scan(&setting).Error; err == nil && setting.Value != "" {
			if strings.EqualFold(setting.Value, "true") {
				enabled = true
			} else {
				enabled = false
			}
		}
	}

	// Allow runtime overrides for CrowdSec mode + API URL via settings table
	mode := h.cfg.CrowdSecMode
	apiURL := h.cfg.CrowdSecAPIURL
	if h.db != nil {
		var m struct{ Value string }
		if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.mode").Scan(&m).Error; err == nil && m.Value != "" {
			mode = m.Value
		}
		var a struct{ Value string }
		if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.crowdsec.api_url").Scan(&a).Error; err == nil && a.Value != "" {
			apiURL = a.Value
		}
	}

	// Only allow 'local' as an enabled mode. Any other value should be treated as disabled.
	if mode != "local" {
		mode = "disabled"
		apiURL = ""
	}

	// Allow runtime override for ACL enabled flag via settings table
	aclEnabled := h.cfg.ACLMode == "enabled"
	aclEffective := aclEnabled && enabled
	if h.db != nil {
		var a struct{ Value string }
		if err := h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "security.acl.enabled").Scan(&a).Error; err == nil && a.Value != "" {
			if strings.EqualFold(a.Value, "true") {
				aclEnabled = true
			} else if strings.EqualFold(a.Value, "false") {
				aclEnabled = false
			}

			// If Cerberus is disabled, ACL should not be considered enabled even
			// if the ACL setting is true. This keeps ACL tied to the Cerberus
			// suite state in the UI and APIs.
			aclEffective = aclEnabled && enabled
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"cerberus": gin.H{"enabled": enabled},
		"crowdsec": gin.H{
			"mode":    mode,
			"api_url": apiURL,
			"enabled": mode == "local",
		},
		"waf": gin.H{
			"mode":    h.cfg.WAFMode,
			"enabled": h.cfg.WAFMode != "" && h.cfg.WAFMode != "disabled",
		},
		"rate_limit": gin.H{
			"mode":    h.cfg.RateLimitMode,
			"enabled": h.cfg.RateLimitMode == "enabled",
		},
		"acl": gin.H{
			"mode":    h.cfg.ACLMode,
			"enabled": aclEffective,
		},
	})
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
	var payload models.SecurityConfig
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if payload.Name == "" {
		payload.Name = "default"
	}
	if err := h.svc.Upsert(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"config": payload})
}

// GenerateBreakGlass generates a break-glass token and returns the plaintext token once
func (h *SecurityHandler) GenerateBreakGlass(c *gin.Context) {
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
	var payload models.SecurityDecision
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if payload.IP == "" || payload.Action == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ip and action are required"})
		return
	}
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
	var body struct{ Token string `json:"break_glass_token"` }
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
	c.JSON(http.StatusOK, gin.H{"enabled": true})
}

// Disable toggles Cerberus off; requires break-glass token or localhost request
func (h *SecurityHandler) Disable(c *gin.Context) {
	var body struct{ Token string `json:"break_glass_token"` }
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
	c.JSON(http.StatusOK, gin.H{"enabled": false})
}
