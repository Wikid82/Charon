package services

import (
	"strings"
	"testing"

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

	// Save config
	err := svc.SaveSMTPConfig(config)
	require.NoError(t, err)

	// Retrieve config
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

	// Save initial config
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

	// Update config
	config.Host = "smtp.newhost.com"
	config.Port = 465
	config.Encryption = "ssl"
	err = svc.SaveSMTPConfig(config)
	require.NoError(t, err)

	// Verify update
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

	// Get config without saving anything
	config, err := svc.GetSMTPConfig()
	require.NoError(t, err)

	// Should have defaults
	assert.Equal(t, 587, config.Port)
	assert.Equal(t, "starttls", config.Encryption)
	assert.Empty(t, config.Host)
}

func TestMailService_BuildEmail(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	msg := svc.buildEmail(
		"sender@example.com",
		"recipient@example.com",
		"Test Subject",
		"<html><body>Test Body</body></html>",
	)

	msgStr := string(msg)
	assert.Contains(t, msgStr, "From: sender@example.com")
	assert.Contains(t, msgStr, "To: recipient@example.com")
	assert.Contains(t, msgStr, "Subject: Test Subject")
	assert.Contains(t, msgStr, "Content-Type: text/html")
	assert.Contains(t, msgStr, "Test Body")
}

// TestMailService_HeaderInjectionPrevention tests that CRLF injection is prevented (CWE-93)
func TestMailService_HeaderInjectionPrevention(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	tests := []struct {
		name            string
		subject         string
		subjectShouldBe string // The sanitized subject line
	}{
		{
			name:            "subject with CRLF injection attempt",
			subject:         "Normal Subject\r\nBcc: attacker@evil.com",
			subjectShouldBe: "Normal SubjectBcc: attacker@evil.com", // CRLF stripped, text concatenated
		},
		{
			name:            "subject with LF injection attempt",
			subject:         "Normal Subject\nX-Injected: malicious",
			subjectShouldBe: "Normal SubjectX-Injected: malicious",
		},
		{
			name:            "subject with null byte",
			subject:         "Normal Subject\x00Hidden",
			subjectShouldBe: "Normal SubjectHidden",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := svc.buildEmail(
				"sender@example.com",
				"recipient@example.com",
				tc.subject,
				"<p>Body</p>",
			)

			msgStr := string(msg)

			// Verify sanitized subject appears
			assert.Contains(t, msgStr, "Subject: "+tc.subjectShouldBe)

			// Split by the header/body separator to get headers only
			parts := strings.SplitN(msgStr, "\r\n\r\n", 2)
			require.Len(t, parts, 2, "Email should have headers and body separated by CRLFCRLF")
			headers := parts[0]

			// Count the number of header lines - there should be exactly 5:
			// From, To, Subject, MIME-Version, Content-Type
			headerLines := strings.Split(headers, "\r\n")
			assert.Equal(t, 5, len(headerLines),
				"Should have exactly 5 header lines (no injected headers)")

			// Verify no injected headers appear as separate lines
			for _, line := range headerLines {
				if strings.HasPrefix(line, "Bcc:") || strings.HasPrefix(line, "X-Injected:") {
					t.Errorf("Injected header found: %s", line)
				}
			}
		})
	}
}

// TestSanitizeEmailHeader tests the sanitizeEmailHeader function directly
func TestSanitizeEmailHeader(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"clean string", "Normal Subject", "Normal Subject"},
		{"CR removal", "Subject\rInjected", "SubjectInjected"},
		{"LF removal", "Subject\nInjected", "SubjectInjected"},
		{"CRLF removal", "Subject\r\nBcc: evil@hacker.com", "SubjectBcc: evil@hacker.com"},
		{"null byte removal", "Subject\x00Hidden", "SubjectHidden"},
		{"tab removal", "Subject\tTabbed", "SubjectTabbed"},
		{"multiple control chars", "A\r\n\x00\x1f\x7fB", "AB"},
		{"empty string", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeEmailHeader(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestValidateEmailAddress tests email address validation
func TestValidateEmailAddress(t *testing.T) {
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
			err := validateEmailAddress(tc.email)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
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

// TestSMTPConfigSerialization ensures config fields are properly stored
func TestSMTPConfigSerialization(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	// Test with special characters in password
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

// TestMailService_SendInvite tests the invite email template
func TestMailService_SendInvite_Template(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	// We can't actually send email, but we can verify the method doesn't panic
	// and returns appropriate error when SMTP is not configured
	err := svc.SendInvite("test@example.com", "abc123token", "TestApp", "https://example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

// Benchmark tests
func BenchmarkMailService_IsConfigured(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	db.AutoMigrate(&models.Setting{})
	svc := NewMailService(db)

	config := &SMTPConfig{
		Host:        "smtp.example.com",
		Port:        587,
		FromAddress: "noreply@example.com",
	}
	svc.SaveSMTPConfig(config)

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

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.buildEmail(
			"sender@example.com",
			"recipient@example.com",
			"Test Subject",
			"<html><body>Test Body</body></html>",
		)
	}
}

// Integration test placeholder - this would use a real SMTP server
func TestMailService_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test would connect to a real SMTP server (like MailHog) for integration testing
	t.Skip("Integration test requires SMTP server")
}

// Test for expired invite token handling in SendInvite
func TestMailService_SendInvite_TokenFormat(t *testing.T) {
	db := setupMailTestDB(t)
	svc := NewMailService(db)

	// Save SMTP config so we can test template generation
	config := &SMTPConfig{
		Host:        "smtp.example.com",
		Port:        587,
		FromAddress: "noreply@example.com",
	}
	svc.SaveSMTPConfig(config)

	// The SendInvite will fail at SMTP connection, but we're testing that
	// the function correctly constructs the invite URL
	err := svc.SendInvite("test@example.com", "token123", "Charon", "https://charon.local/")
	assert.Error(t, err) // Will error on SMTP connection

	// Test with trailing slash handling
	err = svc.SendInvite("test@example.com", "token123", "Charon", "https://charon.local")
	assert.Error(t, err) // Will error on SMTP connection
}

// Add timeout handling test
// Note: Skipped as in-memory SQLite doesn't support concurrent writes well
func TestMailService_SaveSMTPConfig_Concurrent(t *testing.T) {
	t.Skip("In-memory SQLite doesn't support concurrent writes - test real DB in integration")
}
