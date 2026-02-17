package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/utils"
)

// CaddyConfigManager interface for triggering Caddy config reload
type CaddyConfigManager interface {
	ApplyConfig(ctx context.Context) error
}

// CacheInvalidator interface for invalidating security settings cache
type CacheInvalidator interface {
	InvalidateCache()
}

type SettingsHandler struct {
	DB           *gorm.DB
	MailService  *services.MailService
	CaddyManager CaddyConfigManager // For triggering config reload on security settings change
	Cerberus     CacheInvalidator   // For invalidating cache on security settings change
	SecuritySvc  *services.SecurityService
	DataRoot     string
}

func NewSettingsHandler(db *gorm.DB) *SettingsHandler {
	return &SettingsHandler{
		DB:          db,
		MailService: services.NewMailService(db),
	}
}

// NewSettingsHandlerWithDeps creates a SettingsHandler with all dependencies for config reload
func NewSettingsHandlerWithDeps(db *gorm.DB, caddyMgr CaddyConfigManager, cerberus CacheInvalidator, securitySvc *services.SecurityService, dataRoot string) *SettingsHandler {
	return &SettingsHandler{
		DB:           db,
		MailService:  services.NewMailService(db),
		CaddyManager: caddyMgr,
		Cerberus:     cerberus,
		SecuritySvc:  securitySvc,
		DataRoot:     dataRoot,
	}
}

// GetSettings returns all settings.
func (h *SettingsHandler) GetSettings(c *gin.Context) {
	var settings []models.Setting
	if err := h.DB.Find(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
		return
	}

	// Convert to map for easier frontend consumption
	settingsMap := make(map[string]string)
	for _, s := range settings {
		settingsMap[s.Key] = s.Value
	}

	c.JSON(http.StatusOK, settingsMap)
}

type UpdateSettingRequest struct {
	Key      string `json:"key" binding:"required"`
	Value    string `json:"value" binding:"required"`
	Category string `json:"category"`
	Type     string `json:"type"`
}

// UpdateSetting updates or creates a setting.
func (h *SettingsHandler) UpdateSetting(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var req UpdateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Key == "security.admin_whitelist" {
		if err := validateAdminWhitelist(req.Value); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid admin_whitelist: %v", err)})
			return
		}
	}

	setting := models.Setting{
		Key:   req.Key,
		Value: req.Value,
	}

	if req.Category != "" {
		setting.Category = req.Category
	}
	if req.Type != "" {
		setting.Type = req.Type
	}

	// Upsert
	if err := h.DB.Where(models.Setting{Key: req.Key}).Assign(setting).FirstOrCreate(&setting).Error; err != nil {
		if respondPermissionError(c, h.SecuritySvc, "settings_save_failed", err, h.DataRoot) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save setting"})
		return
	}

	if req.Key == "security.acl.enabled" && strings.EqualFold(strings.TrimSpace(req.Value), "true") {
		cerberusSetting := models.Setting{
			Key:      "feature.cerberus.enabled",
			Value:    "true",
			Category: "feature",
			Type:     "bool",
		}
		if err := h.DB.Where(models.Setting{Key: cerberusSetting.Key}).Assign(cerberusSetting).FirstOrCreate(&cerberusSetting).Error; err != nil {
			if respondPermissionError(c, h.SecuritySvc, "settings_save_failed", err, h.DataRoot) {
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable Cerberus"})
			return
		}
		legacyCerberus := models.Setting{
			Key:      "security.cerberus.enabled",
			Value:    "true",
			Category: "security",
			Type:     "bool",
		}
		if err := h.DB.Where(models.Setting{Key: legacyCerberus.Key}).Assign(legacyCerberus).FirstOrCreate(&legacyCerberus).Error; err != nil {
			if respondPermissionError(c, h.SecuritySvc, "settings_save_failed", err, h.DataRoot) {
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable Cerberus"})
			return
		}
		if err := h.ensureSecurityConfigEnabled(); err != nil {
			if respondPermissionError(c, h.SecuritySvc, "settings_save_failed", err, h.DataRoot) {
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable security config"})
			return
		}
	}

	if req.Key == "security.admin_whitelist" {
		if err := h.syncAdminWhitelist(req.Value); err != nil {
			if errors.Is(err, services.ErrInvalidAdminCIDR) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid admin_whitelist"})
				return
			}
			if respondPermissionError(c, h.SecuritySvc, "settings_save_failed", err, h.DataRoot) {
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update security config"})
			return
		}
	}

	// Trigger cache invalidation and config reload for security settings
	if strings.HasPrefix(req.Key, "security.") {
		// Invalidate Cerberus cache immediately so middleware uses new settings
		if h.Cerberus != nil {
			h.Cerberus.InvalidateCache()
		}

		// Trigger sync Caddy config reload so callers can rely on deterministic applied state
		if h.CaddyManager != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
			defer cancel()

			if err := h.CaddyManager.ApplyConfig(ctx); err != nil {
				logger.Log().WithError(err).Warn("Failed to reload Caddy config after security setting change")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload configuration"})
				return
			}

			logger.Log().WithField("setting_key", req.Key).Info("Caddy config reloaded after security setting change")
		}
	}

	c.JSON(http.StatusOK, setting)
}

// PatchConfig updates multiple configuration settings at once
// PATCH /api/v1/config
// Requires admin authentication
func (h *SettingsHandler) PatchConfig(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	// Parse nested configuration structure
	var configUpdates map[string]interface{}
	if err := c.ShouldBindJSON(&configUpdates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Flatten nested configuration into key-value pairs
	// Example: {"security": {"admin_whitelist": "..."}} -> "security.admin_whitelist": "..."
	updates := make(map[string]string)
	flattenConfig(configUpdates, "", updates)

	adminWhitelist, hasAdminWhitelist := updates["security.admin_whitelist"]

	aclEnabled := false
	if value, ok := updates["security.acl.enabled"]; ok && strings.EqualFold(value, "true") {
		aclEnabled = true
		updates["feature.cerberus.enabled"] = "true"
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		for key, value := range updates {
			if key == "security.admin_whitelist" {
				if err := validateAdminWhitelist(value); err != nil {
					return fmt.Errorf("invalid admin_whitelist: %w", err)
				}
			}

			setting := models.Setting{
				Key:      key,
				Value:    value,
				Category: strings.Split(key, ".")[0],
				Type:     "string",
			}

			if err := tx.Where(models.Setting{Key: key}).Assign(setting).FirstOrCreate(&setting).Error; err != nil {
				return fmt.Errorf("save setting %s: %w", key, err)
			}
		}

		if hasAdminWhitelist {
			if err := h.syncAdminWhitelistWithDB(tx, adminWhitelist); err != nil {
				return err
			}
		}

		if aclEnabled {
			if err := h.ensureSecurityConfigEnabledWithDB(tx); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		if errors.Is(err, services.ErrInvalidAdminCIDR) || strings.Contains(err.Error(), "invalid admin_whitelist") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid admin_whitelist"})
			return
		}
		if respondPermissionError(c, h.SecuritySvc, "settings_save_failed", err, h.DataRoot) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
		return
	}

	// Trigger cache invalidation and Caddy reload for security settings
	needsReload := false
	for key := range updates {
		if strings.HasPrefix(key, "security.") {
			needsReload = true
			break
		}
	}

	if needsReload {
		// Invalidate Cerberus cache
		if h.Cerberus != nil {
			h.Cerberus.InvalidateCache()
		}

		// Trigger sync Caddy config reload so callers can rely on deterministic applied state
		if h.CaddyManager != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
			defer cancel()

			if err := h.CaddyManager.ApplyConfig(ctx); err != nil {
				logger.Log().WithError(err).Warn("Failed to reload Caddy config after security settings change")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload configuration"})
				return
			}

			logger.Log().Info("Caddy config reloaded after security settings change")
		}
	}

	// Return current config state
	var settings []models.Setting
	if err := h.DB.Find(&settings).Error; err != nil {
		if respondPermissionError(c, h.SecuritySvc, "settings_save_failed", err, h.DataRoot) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated config"})
		return
	}

	// Convert to map for response
	settingsMap := make(map[string]string)
	for _, s := range settings {
		settingsMap[s.Key] = s.Value
	}

	c.JSON(http.StatusOK, settingsMap)
}

func (h *SettingsHandler) ensureSecurityConfigEnabled() error {
	return h.ensureSecurityConfigEnabledWithDB(h.DB)
}

func (h *SettingsHandler) ensureSecurityConfigEnabledWithDB(db *gorm.DB) error {
	var cfg models.SecurityConfig
	err := db.Where("name = ?", "default").First(&cfg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cfg = models.SecurityConfig{Name: "default", Enabled: true}
			return db.Create(&cfg).Error
		}
		return err
	}
	if cfg.Enabled {
		return nil
	}
	return db.Model(&cfg).Update("enabled", true).Error
}

// flattenConfig converts nested map to flat key-value pairs with dot notation
func flattenConfig(config map[string]interface{}, prefix string, result map[string]string) {
	for k, v := range config {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		switch value := v.(type) {
		case map[string]interface{}:
			flattenConfig(value, key, result)
		case string:
			result[key] = value
		default:
			result[key] = fmt.Sprintf("%v", value)
		}
	}
}

// validateAdminWhitelist validates IP CIDR format
func validateAdminWhitelist(whitelist string) error {
	if whitelist == "" {
		return nil // Empty is valid (no whitelist)
	}

	cidrs := strings.Split(whitelist, ",")
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}

		// Basic CIDR validation (simple check, more thorough validation happens in security middleware)
		if !strings.Contains(cidr, "/") {
			return fmt.Errorf("invalid CIDR format: %s (must include /prefix)", cidr)
		}
	}

	return nil
}

func (h *SettingsHandler) syncAdminWhitelist(whitelist string) error {
	return h.syncAdminWhitelistWithDB(h.DB, whitelist)
}

func (h *SettingsHandler) syncAdminWhitelistWithDB(db *gorm.DB, whitelist string) error {
	securitySvc := services.NewSecurityService(db)
	cfg, err := securitySvc.Get()
	if err != nil {
		if err != services.ErrSecurityConfigNotFound {
			return err
		}
		cfg = &models.SecurityConfig{Name: "default"}
	}
	if cfg.Name == "" {
		cfg.Name = "default"
	}
	cfg.AdminWhitelist = whitelist
	return securitySvc.Upsert(cfg)
}

// SMTPConfigRequest represents the request body for SMTP configuration.
type SMTPConfigRequest struct {
	Host        string `json:"host" binding:"required"`
	Port        int    `json:"port" binding:"required,min=1,max=65535"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	FromAddress string `json:"from_address" binding:"required,email"`
	Encryption  string `json:"encryption" binding:"required,oneof=none ssl starttls"`
}

// GetSMTPConfig returns the current SMTP configuration.
func (h *SettingsHandler) GetSMTPConfig(c *gin.Context) {
	config, err := h.MailService.GetSMTPConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch SMTP configuration"})
		return
	}

	// Don't expose the password
	c.JSON(http.StatusOK, gin.H{
		"host":         config.Host,
		"port":         config.Port,
		"username":     config.Username,
		"password":     MaskPassword(config.Password),
		"from_address": config.FromAddress,
		"encryption":   config.Encryption,
		"configured":   config.Host != "" && config.FromAddress != "",
	})
}

// MaskPassword masks the password for display.
func MaskPassword(password string) string {
	if password == "" {
		return ""
	}
	return "********"
}

// MaskPasswordForTest is an alias for testing.
func MaskPasswordForTest(password string) string {
	return MaskPassword(password)
}

// UpdateSMTPConfig updates the SMTP configuration.
func (h *SettingsHandler) UpdateSMTPConfig(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var req SMTPConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// If password is masked (i.e., unchanged), keep the existing password
	existingConfig, _ := h.MailService.GetSMTPConfig()
	if req.Password == "********" || req.Password == "" {
		req.Password = existingConfig.Password
	}

	config := &services.SMTPConfig{
		Host:        req.Host,
		Port:        req.Port,
		Username:    req.Username,
		Password:    req.Password,
		FromAddress: req.FromAddress,
		Encryption:  req.Encryption,
	}

	if err := h.MailService.SaveSMTPConfig(config); err != nil {
		if respondPermissionError(c, h.SecuritySvc, "smtp_save_failed", err, h.DataRoot) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save SMTP configuration: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "SMTP configuration saved successfully"})
}

// TestSMTPConfig tests the SMTP connection.
func (h *SettingsHandler) TestSMTPConfig(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	if err := h.MailService.TestConnection(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "SMTP connection successful",
	})
}

// SendTestEmail sends a test email to verify the SMTP configuration.
func (h *SettingsHandler) SendTestEmail(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	type TestEmailRequest struct {
		To string `json:"to" binding:"required,email"`
	}

	var req TestEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	htmlBody := `
<!DOCTYPE html>
<html>
<head>
    <title>Test Email</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;">
    <div style="max-width: 600px; margin: 0 auto; padding: 20px;">
        <h2 style="color: #333;">Test Email from Charon</h2>
        <p>If you received this email, your SMTP configuration is working correctly!</p>
        <p style="color: #666; font-size: 12px;">This is an automated test email.</p>
    </div>
</body>
</html>
`

	if err := h.MailService.SendEmail(req.To, "Charon - Test Email", htmlBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Test email sent successfully",
	})
}

// ValidatePublicURL validates a URL is properly formatted for use as the application URL.
func (h *SettingsHandler) ValidatePublicURL(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	type ValidateURLRequest struct {
		URL string `json:"url" binding:"required"`
	}

	var req ValidateURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	normalized, warning, err := utils.ValidateURL(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"valid": false,
			"error": "URL must start with http:// or https:// and cannot include path components",
		})
		return
	}

	response := gin.H{
		"valid":      true,
		"normalized": normalized,
	}

	if warning != "" {
		response["warning"] = warning
	}

	c.JSON(http.StatusOK, response)
}

// TestPublicURL performs a server-side connectivity test with comprehensive SSRF protection.
// This endpoint implements defense-in-depth security:
// 1. Format validation: Ensures valid HTTP/HTTPS URLs without path components
// 2. SSRF validation: Pre-validates DNS resolution and blocks private/reserved IPs
// 3. Runtime protection: ssrfSafeDialer validates IPs again at connection time
// This multi-layer approach satisfies both static analysis (CodeQL) and runtime security.
func (h *SettingsHandler) TestPublicURL(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	// Parse request body
	type TestURLRequest struct {
		URL string `json:"url" binding:"required"`
	}

	var req TestURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Step 1: Format validation (scheme, no paths)
	_, _, err := utils.ValidateURL(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Step 2: SSRF validation (breaks CodeQL taint chain)
	// This explicitly validates against private IPs, loopback, link-local,
	// and cloud metadata endpoints before any network connection is made.
	validatedURL, err := security.ValidateExternalURL(req.URL, security.WithAllowHTTP())
	if err != nil {
		// Return 200 OK for security blocks (maintains existing API behavior)
		c.JSON(http.StatusOK, gin.H{
			"reachable": false,
			"latency":   0,
			"error":     err.Error(),
		})
		return
	}

	// Step 3: Connectivity test with runtime SSRF protection
	reachable, latency, err := utils.TestURLConnectivity(validatedURL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"reachable": false,
			"error":     err.Error(),
		})
		return
	}

	// Return success response
	c.JSON(http.StatusOK, gin.H{
		"reachable": reachable,
		"latency":   latency,
	})
}
