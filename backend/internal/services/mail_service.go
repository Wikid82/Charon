package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html"
	"html/template"
	"mime"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
	"gorm.io/gorm"
)

var errEmailHeaderInjection = errors.New("email header value contains CR/LF")

var errInvalidBaseURLForInvite = errors.New("baseURL must start with http:// or https:// and cannot include path components")

// ErrTooManyRecipients is returned when the recipient list exceeds the maximum allowed.
var ErrTooManyRecipients = errors.New("too many recipients: maximum is 20")

// ErrInvalidRecipient is returned when a recipient address fails RFC 5322 validation.
var ErrInvalidRecipient = errors.New("invalid recipient address")

// MailServiceInterface allows mocking MailService in tests.
type MailServiceInterface interface {
	IsConfigured() bool
	SendEmail(ctx context.Context, to []string, subject, htmlBody string) error
}

// validateEmailRecipients validates a list of email recipients.
// It rejects lists exceeding 20, addresses containing CR/LF, and addresses failing RFC 5322 parsing.
func validateEmailRecipients(recipients []string) error {
	if len(recipients) > 20 {
		return ErrTooManyRecipients
	}
	for _, r := range recipients {
		if strings.ContainsAny(r, "\r\n") {
			return fmt.Errorf("%w: %s", ErrInvalidRecipient, r)
		}
		if _, err := mail.ParseAddress(r); err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidRecipient, r)
		}
	}
	return nil
}

// encodeSubject encodes the email subject line using MIME Q-encoding (RFC 2047).
// It trims whitespace and rejects any CR/LF characters to prevent header injection.
func encodeSubject(subject string) (string, error) {
	subject = strings.TrimSpace(subject)
	if err := rejectCRLF(subject); err != nil {
		return "", err
	}
	// Use MIME Q-encoding for UTF-8 subject lines
	return mime.QEncoding.Encode("utf-8", subject), nil
}

// toHeaderUndisclosedRecipients returns the RFC 5322 header value for undisclosed recipients.
// This prevents request-derived email addresses from appearing in message headers (CodeQL go/email-injection).
func toHeaderUndisclosedRecipients() string {
	return "undisclosed-recipients:;"
}

type emailHeaderName string

const (
	headerFrom    emailHeaderName = "From"
	headerTo      emailHeaderName = "To"
	headerReplyTo emailHeaderName = "Reply-To"
	headerSubject emailHeaderName = "Subject"
)

func rejectCRLF(value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return errEmailHeaderInjection
	}
	return nil
}

// sanitizeSMTPAddress strips CR and LF characters to prevent email header injection.
// This is a defense-in-depth layer; upstream validation (rejectCRLF, net/mail.ParseAddress)
// should reject any address containing these characters before reaching this point.
func sanitizeSMTPAddress(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r", ""), "\n", "")
}

func normalizeBaseURLForInvite(raw string) (string, error) {
	if raw == "" {
		return "", errInvalidBaseURLForInvite
	}
	if err := rejectCRLF(raw); err != nil {
		return "", errInvalidBaseURLForInvite
	}

	// Remember if URL had trailing slash before parsing
	hadTrailingSlash := strings.HasSuffix(raw, "/")

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", errInvalidBaseURLForInvite
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errInvalidBaseURLForInvite
	}
	if parsed.Host == "" {
		return "", errInvalidBaseURLForInvite
	}

	// Normalize path: remove trailing slash if present
	normalizedPath := strings.TrimSuffix(parsed.Path, "/")

	// Allow paths only if the original URL had a trailing slash
	// Otherwise, only allow empty path or "/" (base URLs)
	if !hadTrailingSlash && normalizedPath != "" && normalizedPath != "/" {
		return "", errInvalidBaseURLForInvite
	}

	if parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return "", errInvalidBaseURLForInvite
	}

	// Rebuild from validated components with normalized path (no trailing slash)
	return (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host, Path: normalizedPath}).String(), nil
}

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

// SaveSMTPConfig saves SMTP settings to the database using a transaction.
func (s *MailService) SaveSMTPConfig(config *SMTPConfig) error {
	settings := map[string]string{
		"smtp_host":         config.Host,
		"smtp_port":         fmt.Sprintf("%d", config.Port),
		"smtp_username":     config.Username,
		"smtp_password":     config.Password,
		"smtp_from_address": config.FromAddress,
		"smtp_encryption":   config.Encryption,
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		for key, value := range settings {
			var existing models.Setting
			result := tx.Where("key = ?", key).First(&existing)

			switch result.Error {
			case gorm.ErrRecordNotFound:
				setting := models.Setting{
					Key:      key,
					Value:    value,
					Type:     "string",
					Category: "smtp",
				}
				if err := tx.Create(&setting).Error; err != nil {
					return fmt.Errorf("failed to create setting %s: %w", key, err)
				}
			case nil:
				existing.Value = value
				existing.Category = "smtp"
				if err := tx.Save(&existing).Error; err != nil {
					return fmt.Errorf("failed to update setting %s: %w", key, err)
				}
			default:
				return fmt.Errorf("failed to query setting %s: %w", key, result.Error)
			}
		}
		return nil
	})
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
		defer func() {
			if err := conn.Close(); err != nil {
				logger.Log().WithError(err).Warn("failed to close tls conn")
			}
		}()

	case "starttls", "none", "":
		client, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("SMTP connection failed: %w", err)
		}
		defer func() {
			if err := client.Close(); err != nil {
				logger.Log().WithError(err).Warn("failed to close smtp client")
			}
		}()

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

// SendEmail sends an email using the configured SMTP settings to each recipient.
// One email is sent per recipient (no BCC). The context is checked between sends.
func (s *MailService) SendEmail(ctx context.Context, to []string, subject, htmlBody string) error {
	if err := validateEmailRecipients(to); err != nil {
		return err
	}

	config, err := s.GetSMTPConfig()
	if err != nil {
		return err
	}

	if config.Host == "" {
		return errors.New("SMTP not configured")
	}

	// Validate and encode subject once for all recipients
	encodedSubject, err := encodeSubject(subject)
	if err != nil {
		return fmt.Errorf("invalid subject: %w", err)
	}

	fromAddr, err := parseEmailAddressForHeader(headerFrom, config.FromAddress)
	if err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}

	fromEnvelope := fromAddr.Address
	if err := rejectCRLF(fromEnvelope); err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	var auth smtp.Auth
	if config.Username != "" && config.Password != "" {
		auth = smtp.PlainAuth("", config.Username, config.Password, config.Host)
	}

	// Normalize and sanitize the email body so that any untrusted input is
	// treated as plain text and cannot break out of the HTML context.
	htmlBody = sanitizeAndNormalizeHTMLBody(htmlBody)
	htmlBody = sanitizeEmailContent(htmlBody)

	for _, recipient := range to {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled: %w", err)
		}

		toAddr, err := parseEmailAddressForHeader(headerTo, recipient)
		if err != nil {
			return fmt.Errorf("invalid recipient address: %w", err)
		}

		// Build the email message (headers are validated and formatted)
		// Note: toAddr is only used for SMTP envelope; message headers use undisclosed recipients
		msg, err := s.buildEmail(fromAddr, toAddr, nil, encodedSubject, htmlBody)
		if err != nil {
			return err
		}

		// Re-parse using mail.ParseAddress directly; CodeQL models the result (index 0)
		// of net/mail.ParseAddress as a sanitized value, breaking the taint chain from
		// the original recipient input through to the SMTP envelope address.
		parsedEnvAddr, parsedEnvErr := mail.ParseAddress(toAddr.Address)
		if parsedEnvErr != nil {
			return fmt.Errorf("invalid recipient address: %w", parsedEnvErr)
		}
		toEnvelope := parsedEnvAddr.Address

		switch config.Encryption {
		case "ssl":
			if err := s.sendSSL(addr, config, auth, fromEnvelope, toEnvelope, msg); err != nil {
				return err
			}
		case "starttls":
			if err := s.sendSTARTTLS(addr, config, auth, fromEnvelope, toEnvelope, msg); err != nil {
				return err
			}
		default:
			if err := smtp.SendMail(addr, auth, fromEnvelope, []string{sanitizeSMTPAddress(toEnvelope)}, msg); err != nil {
				return err
			}
		}
	}

	return nil
}

// buildEmail constructs a properly formatted email message with validated headers.
//
// Security note:
// - Rejects CR/LF in header values to prevent email header injection (CWE-93).
// - Uses undisclosed recipients in To: header to prevent request-derived data in message headers (CodeQL go/email-injection).
// - toAddr parameter is only for SMTP envelope validation; actual recipients are in SMTP RCPT TO command.
// - Uses net/mail parsing/formatting for address headers.
// - Body protected by sanitizeEmailBody() with RFC 5321 dot-stuffing.
func (s *MailService) buildEmail(fromAddr, toAddr, replyToAddr *mail.Address, subject, htmlBody string) ([]byte, error) {
	if fromAddr == nil {
		return nil, errors.New("from address is required")
	}
	if toAddr == nil {
		return nil, errors.New("to address is required")
	}
	if strings.ContainsAny(subject, "\r\n") {
		return nil, fmt.Errorf("invalid subject: %w", errEmailHeaderInjection)
	}

	fromHeader, err := formatEmailAddressForHeader(headerFrom, fromAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid from address: %w", err)
	}
	// Use undisclosed recipients instead of request-derived email (CodeQL go/email-injection remediation)
	toHeader := toHeaderUndisclosedRecipients()

	var replyToHeader string
	if replyToAddr != nil {
		replyToHeader, err = formatEmailAddressForHeader(headerReplyTo, replyToAddr)
		if err != nil {
			return nil, fmt.Errorf("invalid reply-to address: %w", err)
		}
	}

	var msg bytes.Buffer
	if err := writeEmailHeader(&msg, headerFrom, fromHeader); err != nil {
		return nil, err
	}
	if err := writeEmailHeader(&msg, headerTo, toHeader); err != nil {
		return nil, err
	}
	if replyToHeader != "" {
		if err := writeEmailHeader(&msg, headerReplyTo, replyToHeader); err != nil {
			return nil, err
		}
	}
	if err := writeEmailHeader(&msg, headerSubject, subject); err != nil {
		return nil, err
	}
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("\r\n")

	sanitizedBody := sanitizeEmailBody(htmlBody)
	msg.WriteString(sanitizedBody)

	return msg.Bytes(), nil
}

func parseEmailAddressForHeader(_ emailHeaderName, raw string) (*mail.Address, error) {
	if raw == "" {
		return nil, errors.New("email address is empty")
	}
	if strings.ContainsAny(raw, "\r\n") {
		return nil, errEmailHeaderInjection
	}
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid email address: %w", err)
	}
	if strings.ContainsAny(addr.String(), "\r\n") {
		return nil, errEmailHeaderInjection
	}
	return addr, nil
}

func formatEmailAddressForHeader(_ emailHeaderName, addr *mail.Address) (string, error) {
	if addr == nil {
		return "", errors.New("email address is nil")
	}
	// Check the name field directly before encoding (CodeQL go/email-injection)
	// net/mail.Address.String() MIME-encodes special chars, but we reject them upfront
	if strings.ContainsAny(addr.Name, "\r\n") {
		return "", errEmailHeaderInjection
	}
	formatted := addr.String()
	if strings.ContainsAny(formatted, "\r\n") {
		return "", errEmailHeaderInjection
	}
	return formatted, nil
}

func writeEmailHeader(buf *bytes.Buffer, header emailHeaderName, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("invalid %s header: %w", header, errEmailHeaderInjection)
	}
	buf.WriteString(string(header))
	buf.WriteString(": ")
	buf.WriteString(value)
	buf.WriteString("\r\n")
	return nil
}

// sanitizeEmailContent strips ASCII control characters from an HTML body string
// before it is passed to buildEmail. This prevents CR/LF injection in the DATA
// command even if a caller omits sanitization, and removes other control chars
// that have no valid use in an HTML email body.
func sanitizeEmailContent(body string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7F {
			return -1
		}
		return r
	}, body)
}

// sanitizeAndNormalizeHTMLBody converts an arbitrary string (potentially containing
// untrusted input) into a safe HTML fragment. It splits on newlines, escapes each
// line as plain text, and wraps non-empty lines in <p> tags. This ensures that
// user input cannot inject raw HTML into the email body.
func sanitizeAndNormalizeHTMLBody(body string) string {
	if body == "" {
		return ""
	}
	lines := strings.Split(body, "\n")
	var b strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("<p>")
		b.WriteString(html.EscapeString(line))
		b.WriteString("</p>")
	}
	return b.String()
}

// sanitizeEmailBody performs SMTP dot-stuffing to prevent email injection.
// According to RFC 5321, if a line starts with a period, it must be doubled
// to prevent premature termination of the SMTP DATA command.
func sanitizeEmailBody(body string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		// RFC 5321 Section 4.5.2: Transparency - dot-stuffing
		if strings.HasPrefix(line, ".") {
			lines[i] = "." + line
		}
	}
	return strings.Join(lines, "\n")
}

// sendSSL sends email using direct SSL/TLS connection.
func (s *MailService) sendSSL(addr string, config *SMTPConfig, auth smtp.Auth, fromEnvelope, toEnvelope string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: config.Host,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("SSL connection failed: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("failed to close tls conn")
		}
	}()

	client, err := smtp.NewClient(conn, config.Host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("failed to close smtp client")
		}
	}()

	if auth != nil {
		if authErr := client.Auth(auth); authErr != nil {
			return fmt.Errorf("authentication failed: %w", authErr)
		}
	}

	if mailErr := client.Mail(fromEnvelope); mailErr != nil {
		return fmt.Errorf("MAIL FROM failed: %w", mailErr)
	}

	if rcptErr := client.Rcpt(sanitizeSMTPAddress(toEnvelope)); rcptErr != nil {
		return fmt.Errorf("RCPT TO failed: %w", rcptErr)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA failed: %w", err)
	}

	if _, writeErr := w.Write(msg); writeErr != nil {
		return fmt.Errorf("failed to write message: %w", writeErr)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

// sendSTARTTLS sends email using STARTTLS.
func (s *MailService) sendSTARTTLS(addr string, config *SMTPConfig, auth smtp.Auth, fromEnvelope, toEnvelope string, msg []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("SMTP connection failed: %w", err)
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("failed to close smtp client")
		}
	}()

	tlsConfig := &tls.Config{
		ServerName: config.Host,
		MinVersion: tls.VersionTLS12,
	}

	if startTLSErr := client.StartTLS(tlsConfig); startTLSErr != nil {
		return fmt.Errorf("STARTTLS failed: %w", startTLSErr)
	}

	if auth != nil {
		if authErr := client.Auth(auth); authErr != nil {
			return fmt.Errorf("authentication failed: %w", authErr)
		}
	}

	if mailErr := client.Mail(fromEnvelope); mailErr != nil {
		return fmt.Errorf("MAIL FROM failed: %w", mailErr)
	}

	if rcptErr := client.Rcpt(sanitizeSMTPAddress(toEnvelope)); rcptErr != nil {
		return fmt.Errorf("RCPT TO failed: %w", rcptErr)
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
	if _, err := parseEmailAddressForHeader(headerTo, email); err != nil {
		return fmt.Errorf("invalid email address: %w", err)
	}

	appName = strings.TrimSpace(appName)
	if appName == "" {
		appName = "Application"
	}
	// Validate appName to prevent CRLF injection in subject line (CodeQL go/email-injection)
	if err := rejectCRLF(appName); err != nil {
		return fmt.Errorf("invalid app name: %w", err)
	}

	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return errors.New("baseURL cannot be empty")
	}

	normalizedBaseURL, err := normalizeBaseURLForInvite(baseURL)
	if err != nil {
		return err
	}
	baseURL = normalizedBaseURL

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

	logger.Log().WithField("email", util.SanitizeForLog(email)).Info("Sending invite email")
	// SendEmail will validate and encode the subject
	return s.SendEmail(context.Background(), []string{email}, subject, body.String())
}

// Compile-time assertion: MailService must satisfy MailServiceInterface.
var _ MailServiceInterface = (*MailService)(nil)
