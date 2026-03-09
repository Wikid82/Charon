package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderNotificationEmail_ValidTemplates(t *testing.T) {
	ms := &MailService{}

	templates := []struct {
		name      string
		data      EmailTemplateData
		wantTitle string
	}{
		{
			name: "email_security_alert.html",
			data: EmailTemplateData{
				EventType: "security_waf",
				Title:     "WAF Block Detected",
				Message:   "Blocked suspicious request",
				Timestamp: "2026-03-07T10:00:00Z",
				SourceIP:  "192.168.1.100",
			},
			wantTitle: "WAF Block Detected",
		},
		{
			name: "email_ssl_event.html",
			data: EmailTemplateData{
				EventType:  "cert",
				Title:      "Certificate Expiring",
				Message:    "Certificate will expire soon",
				Timestamp:  "2026-03-07T10:00:00Z",
				Domain:     "example.com",
				ExpiryDate: "2026-04-07",
			},
			wantTitle: "Certificate Expiring",
		},
		{
			name: "email_uptime_event.html",
			data: EmailTemplateData{
				EventType:  "uptime",
				Title:      "Host Down",
				Message:    "Host is unreachable",
				Timestamp:  "2026-03-07T10:00:00Z",
				HostName:   "web-server-01",
				StatusCode: "503",
			},
			wantTitle: "Host Down",
		},
		{
			name: "email_system_event.html",
			data: EmailTemplateData{
				EventType: "proxy_host",
				Title:     "Proxy Host Updated",
				Message:   "Configuration has changed",
				Timestamp: "2026-03-07T10:00:00Z",
			},
			wantTitle: "Proxy Host Updated",
		},
	}

	for _, tc := range templates {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ms.RenderNotificationEmail(tc.name, tc.data)
			require.NoError(t, err)
			assert.Contains(t, result, tc.wantTitle)
			assert.Contains(t, result, "Charon")
			assert.Contains(t, result, "Charon Reverse Proxy Manager")
			assert.Contains(t, result, tc.data.Timestamp)
			assert.Contains(t, result, tc.data.EventType)
			assert.Contains(t, result, "<!DOCTYPE html>")
		})
	}
}

func TestRenderNotificationEmail_XSSPrevention(t *testing.T) {
	ms := &MailService{}

	data := EmailTemplateData{
		EventType: "security_waf",
		Title:     "<script>alert('xss')</script>",
		Message:   "<img src=x onerror=evil()>",
		Timestamp: "2026-03-07T10:00:00Z",
		SourceIP:  "<b>injected</b>",
	}

	result, err := ms.RenderNotificationEmail("email_security_alert.html", data)
	require.NoError(t, err)

	assert.NotContains(t, result, "<script>")
	assert.Contains(t, result, "&lt;script&gt;")
	assert.NotContains(t, result, "<img ")
	assert.Contains(t, result, "&lt;img ")
	assert.NotContains(t, result, "<b>injected</b>")
	assert.Contains(t, result, "&lt;b&gt;injected&lt;/b&gt;")
}

func TestRenderNotificationEmail_MissingTemplate(t *testing.T) {
	ms := &MailService{}

	data := EmailTemplateData{Title: "Test"}

	_, err := ms.RenderNotificationEmail("nonexistent.html", data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent.html")
	assert.Contains(t, err.Error(), "not found")
}

func TestRenderNotificationEmail_EmptyFields(t *testing.T) {
	ms := &MailService{}

	data := EmailTemplateData{
		EventType: "security_waf",
		Title:     "Alert Title",
		Message:   "Alert message body",
		Timestamp: "2026-03-07T10:00:00Z",
	}

	result, err := ms.RenderNotificationEmail("email_security_alert.html", data)
	require.NoError(t, err)

	assert.NotContains(t, result, "{{.SourceIP}}")
	assert.NotContains(t, result, "{{.Domain}}")
	assert.NotContains(t, result, "{{.ExpiryDate}}")
	assert.NotContains(t, result, "{{.HostName}}")
	assert.NotContains(t, result, "{{.StatusCode}}")

	assert.Contains(t, result, "Alert Title")
	assert.Contains(t, result, "Alert message body")

	result2, err := ms.RenderNotificationEmail("email_ssl_event.html", data)
	require.NoError(t, err)
	assert.NotContains(t, result2, "{{.Domain}}")
	assert.NotContains(t, result2, "{{.ExpiryDate}}")

	result3, err := ms.RenderNotificationEmail("email_uptime_event.html", data)
	require.NoError(t, err)
	assert.NotContains(t, result3, "{{.HostName}}")
	assert.NotContains(t, result3, "{{.StatusCode}}")
}

func TestRenderNotificationEmail_OptionalFieldsRendered(t *testing.T) {
	ms := &MailService{}

	data := EmailTemplateData{
		EventType:  "cert",
		Title:      "SSL Event",
		Message:    "Certificate info",
		Timestamp:  "2026-03-07T10:00:00Z",
		Domain:     "example.com",
		ExpiryDate: "2026-04-07",
	}

	result, err := ms.RenderNotificationEmail("email_ssl_event.html", data)
	require.NoError(t, err)
	assert.Contains(t, result, "example.com")
	assert.Contains(t, result, "2026-04-07")
}

func TestEmailTemplateForEventType(t *testing.T) {
	tests := []struct {
		eventType string
		want      string
	}{
		{"security_waf", "email_security_alert.html"},
		{"security_acl", "email_security_alert.html"},
		{"security_rate_limit", "email_security_alert.html"},
		{"security_crowdsec", "email_security_alert.html"},
		{"SECURITY_WAF", "email_security_alert.html"},
		{"cert", "email_ssl_event.html"},
		{"uptime", "email_uptime_event.html"},
		{"proxy_host", "email_system_event.html"},
		{"remote_server", "email_system_event.html"},
		{"domain", "email_system_event.html"},
		{"test", "email_system_event.html"},
		{"unknown", "email_system_event.html"},
		{"", "email_system_event.html"},
	}

	for _, tc := range tests {
		t.Run(tc.eventType, func(t *testing.T) {
			got := emailTemplateForEventType(tc.eventType)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRenderNotificationEmail_BaseTemplateStructure(t *testing.T) {
	ms := &MailService{}

	data := EmailTemplateData{
		EventType: "test",
		Title:     "Structure Test",
		Message:   "Verify base template wraps content",
		Timestamp: "2026-03-07T10:00:00Z",
	}

	result, err := ms.RenderNotificationEmail("email_system_event.html", data)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(strings.TrimSpace(result), "<!DOCTYPE html>"))
	assert.Contains(t, result, "<h1")
	assert.Contains(t, result, "Charon</h1>")
	assert.Contains(t, result, "#1a1a2e")
	assert.Contains(t, result, "600px")
}
