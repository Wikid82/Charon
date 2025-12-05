package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

type SettingsHandler struct {
	DB          *gorm.DB
	MailService *services.MailService
}

func NewSettingsHandler(db *gorm.DB) *SettingsHandler {
	return &SettingsHandler{
		DB:          db,
		MailService: services.NewMailService(db),
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
	var req UpdateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save setting"})
		return
	}

	c.JSON(http.StatusOK, setting)
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
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save SMTP configuration: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "SMTP configuration saved successfully"})
}

// TestSMTPConfig tests the SMTP connection.
func (h *SettingsHandler) TestSMTPConfig(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
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
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
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
