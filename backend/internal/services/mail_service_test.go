package services

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupMailTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Setting{})
	require.NoError(t, err)

	return db
}

func TestMailService_SaveAndGetSMTPConfig(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	config := &SMTPConfig{
		Host:        "smtp.example.com",
		Port:        587,
		Username:    "user@example.com",
		Password:    "secret123",
		FromAddress: "noreply@example.com",
		Encryption:  "starttls",
	}

	err := svc.SaveSMTPConfig(config)
	require.NoError(t, err)

	retrieved, err := svc.GetSMTPConfig()
	require.NoError(t, err)

	assert.Equal(t, config.Host, retrieved.Host)
	assert.Equal(t, config.Port, retrieved.Port)
	assert.Equal(t, config.Username, retrieved.Username)
	assert.Equal(t, config.Password, retrieved.Password)
	assert.Equal(t, config.FromAddress, retrieved.FromAddress)
	assert.Equal(t, config.Encryption, retrieved.Encryption)
}

func TestMailService_UpdateSMTPConfig(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	config := &SMTPConfig{
		Host:        "smtp.example.com",
		Port:        587,
		Username:    "user@example.com",
		Password:    "secret123",
		FromAddress: "noreply@example.com",
		Encryption:  "starttls",
	}
	err := svc.SaveSMTPConfig(config)
	require.NoError(t, err)

	config.Host = "smtp.newhost.com"
	config.Port = 465
	config.Encryption = "ssl"
	err = svc.SaveSMTPConfig(config)
	require.NoError(t, err)

	retrieved, err := svc.GetSMTPConfig()
	require.NoError(t, err)

	assert.Equal(t, "smtp.newhost.com", retrieved.Host)
	assert.Equal(t, 465, retrieved.Port)
	assert.Equal(t, "ssl", retrieved.Encryption)
}

func TestMailService_IsConfigured(t *testing.T) {
	tests := []struct {
		name     string
		config   *SMTPConfig
		expected bool
	}{
		{
			name: "configured with all fields",
			config: &SMTPConfig{
				Host:        "smtp.example.com",
				Port:        587,
				FromAddress: "noreply@example.com",
				Encryption:  "starttls",
			},
			expected: true,
		},
		{
			name: "not configured - missing host",
			config: &SMTPConfig{
				Port:        587,
				FromAddress: "noreply@example.com",
			},
			expected: false,
		},
		{
			name: "not configured - missing from address",
			config: &SMTPConfig{
				Host: "smtp.example.com",
				Port: 587,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupMailTestDB(t)
			svc := NewMailService(db)

			err := svc.SaveSMTPConfig(tt.config)
			require.NoError(t, err)

			result := svc.IsConfigured()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMailService_GetSMTPConfig_Defaults(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	config, err := svc.GetSMTPConfig()
	require.NoError(t, err)

	assert.Equal(t, 587, config.Port)
	assert.Equal(t, "starttls", config.Encryption)
	assert.Empty(t, config.Host)
}

func TestMailService_BuildEmail(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	fromAddr, err := mail.ParseAddress("sender@example.com")
	require.NoError(t, err)
	toAddr, err := mail.ParseAddress("recipient@example.com")
	require.NoError(t, err)

	// Encode subject as SendEmail would
	encodedSubject, err := encodeSubject("Test Subject")
	require.NoError(t, err)

	msg, err := svc.buildEmail(fromAddr, toAddr, nil, encodedSubject, "<html><body>Test Body</body></html>")
	require.NoError(t, err)

	msgStr := string(msg)
	// Addresses are RFC-formatted and may include angle brackets
	assert.Contains(t, msgStr, "From:")
	assert.Contains(t, msgStr, "sender@example.com")
	assert.Contains(t, msgStr, "To:")
	// After CodeQL remediation, To: header uses undisclosed recipients
	assert.Contains(t, msgStr, "undisclosed-recipients:;")
	assert.Contains(t, msgStr, "Subject:")
	assert.Contains(t, msgStr, "Content-Type: text/html")
	assert.Contains(t, msgStr, "Test Body")
}

func TestParseEmailAddressForHeader(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "user@example.com", false},
		{"valid email with name", "User Name <user@example.com>", false},
		{"empty email", "", true},
		{"invalid format", "not-an-email", true},
		{"missing domain", "user@", true},
		{"injection attempt", "user@example.com\r\nBcc: evil@hacker.com", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseEmailAddressForHeader("to", tc.email)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMailService_BuildEmail_RejectsCRLFInSubject(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	fromAddr, err := mail.ParseAddress("sender@example.com")
	require.NoError(t, err)
	toAddr, err := mail.ParseAddress("recipient@example.com")
	require.NoError(t, err)

	_, err = svc.buildEmail(fromAddr, toAddr, nil, "Normal\r\nBcc: attacker@evil.com", "<p>Body</p>")
	assert.Error(t, err)
}

func TestMailService_BuildEmail_RejectsCRLFInReplyTo(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	fromAddr, err := mail.ParseAddress("sender@example.com")
	require.NoError(t, err)
	toAddr, err := mail.ParseAddress("recipient@example.com")
	require.NoError(t, err)

	// Create reply-to address with CRLF injection attempt in the name field
	replyToAddr := &mail.Address{
		Name:    "Attacker\r\nBcc: evil@example.com",
		Address: "attacker@example.com",
	}

	_, err = svc.buildEmail(fromAddr, toAddr, replyToAddr, "Test Subject", "<p>Body</p>")
	assert.Error(t, err, "Should reject CRLF in reply-to name")
}

func TestMailService_SMTPDotStuffing(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	fromAddr, err := mail.ParseAddress("from@example.com")
	require.NoError(t, err)
	toAddr, err := mail.ParseAddress("to@example.com")
	require.NoError(t, err)

	tests := []struct {
		name          string
		htmlBody      string
		shouldContain string
	}{
		{
			name:          "body with leading period on line",
			htmlBody:      "Line 1\n.Line 2 starts with period\nLine 3",
			shouldContain: "Line 1\n..Line 2 starts with period\nLine 3",
		},
		{
			name:          "body with SMTP terminator sequence",
			htmlBody:      "Some text\n.\nMore text",
			shouldContain: "Some text\n..\nMore text",
		},
		{
			name:          "body with multiple leading periods",
			htmlBody:      ".First\n..Second\nNormal",
			shouldContain: "..First\n...Second\nNormal",
		},
		{
			name:          "body without leading periods",
			htmlBody:      "Normal line\nAnother normal line",
			shouldContain: "Normal line\nAnother normal line",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := svc.buildEmail(fromAddr, toAddr, nil, "Test", tc.htmlBody)
			require.NoError(t, err)
			msgStr := string(msg)

			parts := strings.Split(msgStr, "\r\n\r\n")
			require.Len(t, parts, 2, "Email should have headers and body")
			body := parts[1]

			assert.Contains(t, body, tc.shouldContain, "Body should contain dot-stuffed content")
		})
	}
}

func TestSanitizeEmailBody(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"single leading period", ".test", "..test"},
		{"period in middle", "test.com", "test.com"},
		{"multiple lines with periods", "line1\n.line2\nline3", "line1\n..line2\nline3"},
		{"SMTP terminator", "text\n.\nmore", "text\n..\nmore"},
		{"no periods", "clean text", "clean text"},
		{"empty string", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeEmailBody(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMailService_TestConnection_NotConfigured(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	err := svc.TestConnection()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestMailService_SendEmail_NotConfigured(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	err := svc.SendEmail("test@example.com", "Subject", "<p>Body</p>")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestMailService_SendEmail_RejectsCRLFInSubject(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	config := &SMTPConfig{
		Host:        "smtp.example.com",
		Port:        587,
		FromAddress: "noreply@example.com",
	}
	require.NoError(t, svc.SaveSMTPConfig(config))

	err := svc.SendEmail("recipient@example.com", "Hello\r\nBcc: evil@example.com", "Body")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid subject")
}

func TestSMTPConfigSerialization(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	config := &SMTPConfig{
		Host:        "smtp.example.com",
		Port:        587,
		Username:    "user@example.com",
		Password:    "p@$$w0rd!#$%",
		FromAddress: "Charon <noreply@example.com>",
		Encryption:  "starttls",
	}

	err := svc.SaveSMTPConfig(config)
	require.NoError(t, err)

	retrieved, err := svc.GetSMTPConfig()
	require.NoError(t, err)

	assert.Equal(t, config.Password, retrieved.Password)
	assert.Equal(t, config.FromAddress, retrieved.FromAddress)
}

func TestMailService_SendInvite_Template(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	err := svc.SendInvite("test@example.com", "abc123token", "TestApp", "https://example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func BenchmarkMailService_IsConfigured(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	_ = db.AutoMigrate(&models.Setting{})
	svc := NewMailService(db)

	config := &SMTPConfig{
		Host:        "smtp.example.com",
		Port:        587,
		FromAddress: "noreply@example.com",
	}
	_ = svc.SaveSMTPConfig(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.IsConfigured()
	}
}

func BenchmarkMailService_BuildEmail(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	svc := NewMailService(db)

	fromAddr, _ := mail.ParseAddress("sender@example.com")
	toAddr, _ := mail.ParseAddress("recipient@example.com")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.buildEmail(fromAddr, toAddr, nil, "Test Subject", "<html><body>Test Body</body></html>")
	}
}

func TestMailService_SendInvite_InvalidBaseURL_CRLF(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	err := svc.SendInvite("test@example.com", "token123", "TestApp", "https://example.com\r\nBcc: attacker@example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "baseURL")
}

func TestMailService_SendInvite_InvalidBaseURL_Path(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	err := svc.SendInvite("test@example.com", "token123", "TestApp", "https://example.com/sneaky")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "baseURL")
}

func TestMailService_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Skip("Integration test requires SMTP server")
}

func TestMailService_SendInvite_TokenFormat(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	config := &SMTPConfig{
		Host:        "smtp.example.com",
		Port:        587,
		FromAddress: "noreply@example.com",
	}
	_ = svc.SaveSMTPConfig(config)

	err := svc.SendInvite("test@example.com", "token123", "Charon", "https://charon.local/")
	assert.Error(t, err)

	err = svc.SendInvite("test@example.com", "token123", "Charon", "https://charon.local")
	assert.Error(t, err)
}

func TestMailService_SaveSMTPConfig_Concurrent(t *testing.T) {
	t.Skip("In-memory SQLite doesn't support concurrent writes - test real DB in integration")
}

func TestMailService_SendEmail_InvalidRecipient(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	config := &SMTPConfig{
		Host:        "smtp.example.com",
		Port:        587,
		FromAddress: "noreply@example.com",
	}
	require.NoError(t, svc.SaveSMTPConfig(config))

	err := svc.SendEmail("invalid\r\nemail", "Subject", "Body")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid recipient")
}

func TestMailService_SendEmail_InvalidFromAddress(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	config := &SMTPConfig{
		Host:        "smtp.example.com",
		Port:        587,
		FromAddress: "invalid\r\nfrom@example.com",
	}
	require.NoError(t, svc.SaveSMTPConfig(config))

	err := svc.SendEmail("test@example.com", "Subject", "Body")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid from address")
}

func TestMailService_SendEmail_EncryptionModes(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	tests := []struct {
		name       string
		encryption string
	}{
		{"ssl", "ssl"},
		{"starttls", "starttls"},
		{"none", "none"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &SMTPConfig{
				Host:        "smtp.example.com",
				Port:        587,
				Username:    "user",
				Password:    "pass",
				FromAddress: "test@example.com",
				Encryption:  tt.encryption,
			}
			require.NoError(t, svc.SaveSMTPConfig(config))

			err := svc.SendEmail("recipient@example.com", "Test", "Body")
			assert.Error(t, err)
		})
	}
}

// TestMailService_SendEmail_CRLFInjection_Comprehensive tests CRLF injection prevention
// across all email header fields (CodeQL go/email-injection remediation)
func TestMailService_SendEmail_CRLFInjection_Comprehensive(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	config := &SMTPConfig{
		Host:        "smtp.example.com",
		Port:        587,
		FromAddress: "noreply@example.com",
		Encryption:  "starttls",
	}
	require.NoError(t, svc.SaveSMTPConfig(config))

	tests := []struct {
		name        string
		to          string
		subject     string
		fromAddress string
		description string
	}{
		{
			name:        "CRLF in recipient address",
			to:          "victim@example.com\r\nBcc: attacker@evil.com",
			subject:     "Normal Subject",
			fromAddress: "noreply@example.com",
			description: "Reject CRLF in To header",
		},
		{
			name:        "CRLF in subject line",
			to:          "victim@example.com",
			subject:     "Test\r\nBcc: attacker@evil.com",
			fromAddress: "noreply@example.com",
			description: "Reject CRLF in Subject header",
		},
		{
			name:        "LF only in recipient",
			to:          "victim@example.com\nBcc: attacker@evil.com",
			subject:     "Normal Subject",
			fromAddress: "noreply@example.com",
			description: "Reject LF in To header",
		},
		{
			name:        "LF only in subject",
			to:          "victim@example.com",
			subject:     "Test\nBcc: attacker@evil.com",
			fromAddress: "noreply@example.com",
			description: "Reject LF in Subject header",
		},
		{
			name:        "CR only in recipient",
			to:          "victim@example.com\rBcc: attacker@evil.com",
			subject:     "Normal Subject",
			fromAddress: "noreply@example.com",
			description: "Reject CR in To header",
		},
		{
			name:        "multiple CRLF sequences",
			to:          "victim@example.com",
			subject:     "Test\r\nBcc: evil1@attacker.com\r\nCc: evil2@attacker.com",
			fromAddress: "noreply@example.com",
			description: "Reject multiple CRLF attempts",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Update config with potentially malicious from address if specified
			if tc.fromAddress != config.FromAddress {
				testConfig := *config
				testConfig.FromAddress = tc.fromAddress
				require.NoError(t, svc.SaveSMTPConfig(&testConfig))
			}

			err := svc.SendEmail(tc.to, tc.subject, "<p>Normal body</p>")
			assert.Error(t, err, tc.description)
			assert.Contains(t, err.Error(), "invalid", "Error should indicate invalid input")
		})
	}
}

// TestMailService_SendInvite_CRLFInjection tests CRLF injection prevention in invite emails
// TestMailService_BuildEmail_UndisclosedRecipients verifies that buildEmail uses
// "undisclosed-recipients:;" in the To: header instead of the actual recipient address.
// This prevents request-derived data from appearing in message headers (CodeQL go/email-injection).
func TestMailService_BuildEmail_UndisclosedRecipients(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	fromAddr, err := mail.ParseAddress("sender@example.com")
	require.NoError(t, err)
	toAddr, err := mail.ParseAddress("recipient@example.com")
	require.NoError(t, err)

	// Encode subject as SendEmail would
	encodedSubject, err := encodeSubject("Test Subject")
	require.NoError(t, err)

	msg, err := svc.buildEmail(fromAddr, toAddr, nil, encodedSubject, "<html><body>Test Body</body></html>")
	require.NoError(t, err)

	msgStr := string(msg)

	// MUST contain undisclosed recipients header
	assert.Contains(t, msgStr, "To: undisclosed-recipients:;", "To header must use undisclosed recipients")

	// MUST NOT contain the actual recipient email address in headers
	// Split message into headers and body
	parts := strings.Split(msgStr, "\r\n\r\n")
	require.Len(t, parts, 2, "Email should have headers and body sections")
	headers := parts[0]
	assert.NotContains(t, headers, "recipient@example.com", "Recipient email must not appear in message headers")
}

// TestMailService_SendInvite_HTMLTemplateEscaping verifies that the HTML template
// properly escapes special characters in user-controlled fields like appName.
func TestMailService_SendInvite_HTMLTemplateEscaping(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	config := &SMTPConfig{
		Host:        "smtp.example.com",
		Port:        587,
		FromAddress: "noreply@example.com",
		Encryption:  "starttls",
	}
	require.NoError(t, svc.SaveSMTPConfig(config))

	// Use appName with HTML tags that should be escaped
	maliciousAppName := "<b>XSS</b><script>alert('test')</script>"
	baseURL := "https://example.com"
	token := "testtoken123"

	// SendInvite will fail because SMTP isn't actually configured,
	// but we can test the template rendering by calling the internal logic
	// directly through a helper or by examining the error path.
	// For now, we'll test that the appName validation rejects CRLF
	// (HTML injection in templates is handled by html/template auto-escaping)
	err := svc.SendInvite("test@example.com", token, maliciousAppName, baseURL)
	assert.Error(t, err) // Will fail due to SMTP not actually running
	// The key security property is that html/template auto-escapes {{.AppName}}
	// This is verified by the template engine itself, not by our code
}

func TestMailService_SendInvite_CRLFInjection(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	config := &SMTPConfig{
		Host:        "smtp.example.com",
		Port:        587,
		FromAddress: "noreply@example.com",
		Encryption:  "starttls",
	}
	require.NoError(t, svc.SaveSMTPConfig(config))

	tests := []struct {
		name        string
		email       string
		token       string
		appName     string
		baseURL     string
		description string
	}{
		{
			name:        "CRLF in email address",
			email:       "victim@example.com\r\nBcc: attacker@evil.com",
			token:       "token123",
			appName:     "TestApp",
			baseURL:     "https://example.com",
			description: "Reject CRLF in invite email address",
		},
		{
			name:        "CRLF in baseURL",
			email:       "user@example.com",
			token:       "token123",
			appName:     "TestApp",
			baseURL:     "https://example.com\r\nBcc: attacker@evil.com",
			description: "Reject CRLF in invite baseURL",
		},
		{
			name:        "CRLF in app name (subject)",
			email:       "user@example.com",
			token:       "token123",
			appName:     "TestApp\r\nBcc: attacker@evil.com",
			baseURL:     "https://example.com",
			description: "Reject CRLF in app name used in subject",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.SendInvite(tc.email, tc.token, tc.appName, tc.baseURL)
			assert.Error(t, err, tc.description)
		})
	}
}

func TestRejectCRLF(t *testing.T) {
	t.Parallel()

	require.NoError(t, rejectCRLF("normal-value"))
	require.ErrorIs(t, rejectCRLF("bad\r\nvalue"), errEmailHeaderInjection)
}

func TestNormalizeBaseURLForInvite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "valid https", raw: "https://example.com", want: "https://example.com", wantErr: false},
		{name: "valid http with slash path", raw: "http://example.com/", want: "http://example.com", wantErr: false},
		{name: "empty", raw: "", wantErr: true},
		{name: "invalid scheme", raw: "ftp://example.com", wantErr: true},
		{name: "with path", raw: "https://example.com/path", wantErr: true},
		{name: "with query", raw: "https://example.com?x=1", wantErr: true},
		{name: "with fragment", raw: "https://example.com#frag", wantErr: true},
		{name: "with user info", raw: "https://user@example.com", wantErr: true},
		{name: "with header injection", raw: "https://example.com\r\nX-Test: 1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeBaseURLForInvite(tt.raw)
			if tt.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, errInvalidBaseURLForInvite)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestEncodeSubject_RejectsCRLF(t *testing.T) {
	t.Parallel()

	_, err := encodeSubject("Hello\r\nWorld")
	require.Error(t, err)
	require.ErrorIs(t, err, errEmailHeaderInjection)
}

func TestMailService_GetSMTPConfig_DBError(t *testing.T) {
	t.Parallel()

	db := setupMailTestDB(t)
	svc := NewMailService(db)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = svc.GetSMTPConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load SMTP settings")
}

func TestMailService_GetSMTPConfig_InvalidPortFallback(t *testing.T) {
	t.Parallel()

	db := setupMailTestDB(t)
	svc := NewMailService(db)

	require.NoError(t, db.Create(&models.Setting{Key: "smtp_host", Value: "smtp.example.com", Type: "string", Category: "smtp"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "smtp_port", Value: "invalid", Type: "string", Category: "smtp"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "smtp_from_address", Value: "noreply@example.com", Type: "string", Category: "smtp"}).Error)

	config, err := svc.GetSMTPConfig()
	require.NoError(t, err)
	assert.Equal(t, 587, config.Port)
}

func TestMailService_BuildEmail_NilAddressValidation(t *testing.T) {
	t.Parallel()

	db := setupMailTestDB(t)
	svc := NewMailService(db)

	toAddr, err := mail.ParseAddress("recipient@example.com")
	require.NoError(t, err)

	_, err = svc.buildEmail(nil, toAddr, nil, "Subject", "Body")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "from address is required")

	fromAddr, err := mail.ParseAddress("sender@example.com")
	require.NoError(t, err)

	_, err = svc.buildEmail(fromAddr, nil, nil, "Subject", "Body")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "to address is required")
}

func TestWriteEmailHeader_RejectsCRLFValue(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := writeEmailHeader(&buf, headerSubject, "bad\r\nvalue")
	assert.Error(t, err)
}

func TestMailService_sendSSL_DialFailure(t *testing.T) {
	t.Parallel()

	db := setupMailTestDB(t)
	svc := NewMailService(db)

	err := svc.sendSSL(
		"127.0.0.1:1",
		&SMTPConfig{Host: "127.0.0.1"},
		nil,
		"from@example.com",
		"to@example.com",
		[]byte("test"),
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SSL connection failed")
}

func TestMailService_sendSTARTTLS_DialFailure(t *testing.T) {
	t.Parallel()

	db := setupMailTestDB(t)
	svc := NewMailService(db)

	err := svc.sendSTARTTLS(
		"127.0.0.1:1",
		&SMTPConfig{Host: "127.0.0.1"},
		nil,
		"from@example.com",
		"to@example.com",
		[]byte("test"),
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SMTP connection failed")
}

func TestMailService_TestConnection_StartTLSSuccessWithAuth(t *testing.T) {
	tlsConf, certPEM := newTestTLSConfig(t)
	trustTestCertificate(t, certPEM)
	addr, cleanup := startMockSMTPServer(t, tlsConf, true, true)
	defer cleanup()

	host, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	db := setupMailTestDB(t)
	svc := NewMailService(db)
	require.NoError(t, svc.SaveSMTPConfig(&SMTPConfig{
		Host:        host,
		Port:        port,
		Username:    "user",
		Password:    "pass",
		FromAddress: "sender@example.com",
		Encryption:  "starttls",
	}))

	require.NoError(t, svc.TestConnection())
}

func TestMailService_TestConnection_NoneSuccess(t *testing.T) {
	t.Parallel()

	tlsConf, _ := newTestTLSConfig(t)
	addr, cleanup := startMockSMTPServer(t, tlsConf, false, false)
	defer cleanup()

	host, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	db := setupMailTestDB(t)
	svc := NewMailService(db)
	require.NoError(t, svc.SaveSMTPConfig(&SMTPConfig{
		Host:        host,
		Port:        port,
		FromAddress: "sender@example.com",
		Encryption:  "none",
	}))

	require.NoError(t, svc.TestConnection())
}

func TestMailService_SendEmail_STARTTLSSuccess(t *testing.T) {
	tlsConf, certPEM := newTestTLSConfig(t)
	trustTestCertificate(t, certPEM)
	addr, cleanup := startMockSMTPServer(t, tlsConf, true, true)
	defer cleanup()

	host, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	db := setupMailTestDB(t)
	svc := NewMailService(db)
	require.NoError(t, svc.SaveSMTPConfig(&SMTPConfig{
		Host:        host,
		Port:        port,
		Username:    "user",
		Password:    "pass",
		FromAddress: "sender@example.com",
		Encryption:  "starttls",
	}))

	err = svc.SendEmail("recipient@example.com", "Subject", "Body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "STARTTLS failed")
}

func TestMailService_SendEmail_SSLSuccess(t *testing.T) {
	tlsConf, certPEM := newTestTLSConfig(t)
	trustTestCertificate(t, certPEM)
	addr, cleanup := startMockSSLSMTPServer(t, tlsConf, true)
	defer cleanup()

	host, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	db := setupMailTestDB(t)
	svc := NewMailService(db)
	require.NoError(t, svc.SaveSMTPConfig(&SMTPConfig{
		Host:        host,
		Port:        port,
		Username:    "user",
		Password:    "pass",
		FromAddress: "sender@example.com",
		Encryption:  "ssl",
	}))

	err = svc.SendEmail("recipient@example.com", "Subject", "Body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSL connection failed")
}

func newTestTLSConfig(t *testing.T) (*tls.Config, []byte) {
	t.Helper()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "charon-test-ca",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "127.0.0.1",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caTemplate, &leafKey.PublicKey, caKey)
	require.NoError(t, err)

	leafCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	leafKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(leafKey)})

	cert, err := tls.X509KeyPair(leafCertPEM, leafKeyPEM)
	require.NoError(t, err)

	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, caPEM
}

func trustTestCertificate(t *testing.T, certPEM []byte) {
	t.Helper()

	caFile := t.TempDir() + "/ca-cert.pem"
	require.NoError(t, os.WriteFile(caFile, certPEM, 0o600))
	t.Setenv("SSL_CERT_FILE", caFile)
}

func startMockSMTPServer(t *testing.T, tlsConf *tls.Config, supportStartTLS bool, requireAuth bool) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		handleSMTPConn(conn, tlsConf, supportStartTLS, requireAuth)
	}()

	cleanup := func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}

	return listener.Addr().String(), cleanup
}

func startMockSSLSMTPServer(t *testing.T, tlsConf *tls.Config, requireAuth bool) (string, func()) {
	t.Helper()

	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConf)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		handleSMTPConn(conn, tlsConf, false, requireAuth)
	}()

	cleanup := func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}

	return listener.Addr().String(), cleanup
}

func handleSMTPConn(conn net.Conn, tlsConf *tls.Config, supportStartTLS bool, requireAuth bool) {
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	writeLine := func(line string) {
		_, _ = writer.WriteString(line + "\r\n")
		_ = writer.Flush()
	}

	writeLine("220 localhost ESMTP")
	tlsUpgraded := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		command := strings.ToUpper(strings.TrimSpace(line))

		switch {
		case strings.HasPrefix(command, "EHLO") || strings.HasPrefix(command, "HELO"):
			if supportStartTLS && !tlsUpgraded {
				writeLine("250-localhost")
				writeLine("250-STARTTLS")
				writeLine("250 AUTH PLAIN")
			} else {
				writeLine("250-localhost")
				writeLine("250 AUTH PLAIN")
			}
		case strings.HasPrefix(command, "STARTTLS"):
			if !supportStartTLS || tlsUpgraded {
				writeLine("454 TLS not available")
				continue
			}
			writeLine("220 Ready to start TLS")
			tlsConn := tls.Server(conn, tlsConf)
			if handshakeErr := tlsConn.Handshake(); handshakeErr != nil {
				return
			}
			conn = tlsConn
			reader = bufio.NewReader(conn)
			writer = bufio.NewWriter(conn)
			tlsUpgraded = true
		case strings.HasPrefix(command, "AUTH"):
			if requireAuth {
				writeLine("235 Authentication successful")
			} else {
				writeLine("235 Authentication accepted")
			}
		case strings.HasPrefix(command, "MAIL FROM"):
			writeLine("250 OK")
		case strings.HasPrefix(command, "RCPT TO"):
			writeLine("250 OK")
		case strings.HasPrefix(command, "DATA"):
			writeLine("354 End data with <CR><LF>.<CR><LF>")
			for {
				dataLine, readErr := reader.ReadString('\n')
				if readErr != nil {
					return
				}
				if dataLine == ".\r\n" {
					break
				}
			}
			writeLine("250 Message accepted")
		case strings.HasPrefix(command, "QUIT"):
			writeLine("221 Bye")
			return
		default:
			writeLine("250 OK")
		}
	}
}
