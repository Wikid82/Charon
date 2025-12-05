package services

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

// SMTPConfig holds the SMTP server configuration.
type SMTPConfig struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	FromAddress string `json:"from_address"`
	Encryption  string `json:"encryption"` // "none", "ssl", "starttls"
}

// MailService handles sending emails via SMTP.
type MailService struct {
	db *gorm.DB
}

// NewMailService creates a new mail service instance.
func NewMailService(db *gorm.DB) *MailService {
	return &MailService{db: db}
}

// GetSMTPConfig retrieves SMTP settings from the database.
func (s *MailService) GetSMTPConfig() (*SMTPConfig, error) {
	var settings []models.Setting
	if err := s.db.Where("category = ?", "smtp").Find(&settings).Error; err != nil {
		return nil, fmt.Errorf("failed to load SMTP settings: %w", err)
	}

	config := &SMTPConfig{
		Port:       587, // Default port
		Encryption: "starttls",
	}

	for _, setting := range settings {
		switch setting.Key {
		case "smtp_host":
			config.Host = setting.Value
		case "smtp_port":
			if _, err := fmt.Sscanf(setting.Value, "%d", &config.Port); err != nil {
				config.Port = 587
			}
		case "smtp_username":
			config.Username = setting.Value
		case "smtp_password":
			config.Password = setting.Value
		case "smtp_from_address":
			config.FromAddress = setting.Value
		case "smtp_encryption":
			config.Encryption = setting.Value
		}
	}

	return config, nil
}

// SaveSMTPConfig saves SMTP settings to the database.
func (s *MailService) SaveSMTPConfig(config *SMTPConfig) error {
	settings := map[string]string{
		"smtp_host":         config.Host,
		"smtp_port":         fmt.Sprintf("%d", config.Port),
		"smtp_username":     config.Username,
		"smtp_password":     config.Password,
		"smtp_from_address": config.FromAddress,
		"smtp_encryption":   config.Encryption,
	}

	for key, value := range settings {
		setting := models.Setting{
			Key:      key,
			Value:    value,
			Type:     "string",
			Category: "smtp",
		}

		// Upsert: update if exists, create if not
		result := s.db.Where("key = ?", key).First(&models.Setting{})
		if result.Error == gorm.ErrRecordNotFound {
			if err := s.db.Create(&setting).Error; err != nil {
				return fmt.Errorf("failed to create setting %s: %w", key, err)
			}
		} else {
			if err := s.db.Model(&models.Setting{}).Where("key = ?", key).Updates(map[string]interface{}{
				"value":    value,
				"category": "smtp",
			}).Error; err != nil {
				return fmt.Errorf("failed to update setting %s: %w", key, err)
			}
		}
	}

	return nil
}

// IsConfigured returns true if SMTP is properly configured.
func (s *MailService) IsConfigured() bool {
	config, err := s.GetSMTPConfig()
	if err != nil {
		return false
	}
	return config.Host != "" && config.FromAddress != ""
}

// TestConnection tests the SMTP connection without sending an email.
func (s *MailService) TestConnection() error {
	config, err := s.GetSMTPConfig()
	if err != nil {
		return err
	}

	if config.Host == "" {
		return errors.New("SMTP host not configured")
	}

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	// Try to connect based on encryption type
	switch config.Encryption {
	case "ssl":
		tlsConfig := &tls.Config{
			ServerName: config.Host,
			MinVersion: tls.VersionTLS12,
		}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("SSL connection failed: %w", err)
		}
		defer conn.Close()

	case "starttls", "none", "":
		client, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("SMTP connection failed: %w", err)
		}
		defer client.Close()

		if config.Encryption == "starttls" {
			tlsConfig := &tls.Config{
				ServerName: config.Host,
				MinVersion: tls.VersionTLS12,
			}
			if err := client.StartTLS(tlsConfig); err != nil {
				return fmt.Errorf("STARTTLS failed: %w", err)
			}
		}

		// Try authentication if credentials are provided
		if config.Username != "" && config.Password != "" {
			auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
		}
	}

	return nil
}

// SendEmail sends an email using the configured SMTP settings.
func (s *MailService) SendEmail(to, subject, htmlBody string) error {
	config, err := s.GetSMTPConfig()
	if err != nil {
		return err
	}

	if config.Host == "" {
		return errors.New("SMTP not configured")
	}

	// Build the email message
	msg := s.buildEmail(config.FromAddress, to, subject, htmlBody)

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	var auth smtp.Auth
	if config.Username != "" && config.Password != "" {
		auth = smtp.PlainAuth("", config.Username, config.Password, config.Host)
	}

	switch config.Encryption {
	case "ssl":
		return s.sendSSL(addr, config, auth, to, msg)
	case "starttls":
		return s.sendSTARTTLS(addr, config, auth, to, msg)
	default:
		return smtp.SendMail(addr, auth, config.FromAddress, []string{to}, msg)
	}
}

// buildEmail constructs a properly formatted email message.
func (s *MailService) buildEmail(from, to, subject, htmlBody string) []byte {
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	var msg bytes.Buffer
	for key, value := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)

	return msg.Bytes()
}

// sendSSL sends email using direct SSL/TLS connection.
func (s *MailService) sendSSL(addr string, config *SMTPConfig, auth smtp.Auth, to string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: config.Host,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("SSL connection failed: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, config.Host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
	}

	if err := client.Mail(config.FromAddress); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO failed: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA failed: %w", err)
	}

	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

// sendSTARTTLS sends email using STARTTLS.
func (s *MailService) sendSTARTTLS(addr string, config *SMTPConfig, auth smtp.Auth, to string, msg []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("SMTP connection failed: %w", err)
	}
	defer client.Close()

	tlsConfig := &tls.Config{
		ServerName: config.Host,
		MinVersion: tls.VersionTLS12,
	}

	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("STARTTLS failed: %w", err)
	}

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
	}

	if err := client.Mail(config.FromAddress); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO failed: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA failed: %w", err)
	}

	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

// SendInvite sends an invitation email to a new user.
func (s *MailService) SendInvite(email, inviteToken, appName, baseURL string) error {
	inviteURL := fmt.Sprintf("%s/accept-invite?token=%s", strings.TrimSuffix(baseURL, "/"), inviteToken)

	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>You've been invited to {{.AppName}}</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
    <div style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); padding: 30px; border-radius: 10px 10px 0 0; text-align: center;">
        <h1 style="color: white; margin: 0;">{{.AppName}}</h1>
    </div>
    <div style="background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; border: 1px solid #e0e0e0; border-top: none;">
        <h2 style="margin-top: 0;">You've Been Invited!</h2>
        <p>You've been invited to join <strong>{{.AppName}}</strong>. Click the button below to set up your account:</p>
        <div style="text-align: center; margin: 30px 0;">
            <a href="{{.InviteURL}}" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 15px 30px; text-decoration: none; border-radius: 5px; font-weight: bold; display: inline-block;">Accept Invitation</a>
        </div>
        <p style="color: #666; font-size: 14px;">This invitation link will expire in 48 hours.</p>
        <p style="color: #666; font-size: 14px;">If you didn't expect this invitation, you can safely ignore this email.</p>
        <hr style="border: none; border-top: 1px solid #e0e0e0; margin: 20px 0;">
        <p style="color: #999; font-size: 12px;">If the button doesn't work, copy and paste this link into your browser:<br>
        <a href="{{.InviteURL}}" style="color: #667eea;">{{.InviteURL}}</a></p>
    </div>
</body>
</html>
`

	t, err := template.New("invite").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse email template: %w", err)
	}

	var body bytes.Buffer
	data := map[string]string{
		"AppName":   appName,
		"InviteURL": inviteURL,
	}

	if err := t.Execute(&body, data); err != nil {
		return fmt.Errorf("failed to execute email template: %w", err)
	}

	subject := fmt.Sprintf("You've been invited to %s", appName)

	logger.Log().WithField("email", email).Info("Sending invite email")
	return s.SendEmail(email, subject, body.String())
}
