package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	notify "github.com/Wikid82/go_notify_yourself"
	"github.com/Wikid82/go_notify_yourself/providers/email"
)

// fakeMailServiceForEmailAdapter is a dedicated MailServiceInterface fake for
// notify_email_adapter.go's tests. It captures every RenderNotificationEmail
// call's templateName + EmailTemplateData (unlike notification_service_test.go's
// mockMailService, which discards the templateName argument) so tests here
// can assert the exact template-name/field wiring the adapter produces.
type fakeMailServiceForEmailAdapter struct {
	isConfigured bool

	sendCalls []struct {
		to      []string
		subject string
		body    string
	}
	sendErr error

	renderCalls []struct {
		templateName string
		data         EmailTemplateData
	}
	renderResult string
	renderErr    error
}

func (f *fakeMailServiceForEmailAdapter) IsConfigured() bool { return f.isConfigured }

func (f *fakeMailServiceForEmailAdapter) SendEmail(_ context.Context, to []string, subject, htmlBody string) error {
	f.sendCalls = append(f.sendCalls, struct {
		to      []string
		subject string
		body    string
	}{to: to, subject: subject, body: htmlBody})
	return f.sendErr
}

func (f *fakeMailServiceForEmailAdapter) RenderNotificationEmail(templateName string, data EmailTemplateData) (string, error) {
	f.renderCalls = append(f.renderCalls, struct {
		templateName string
		data         EmailTemplateData
	}{templateName: templateName, data: data})
	if f.renderErr != nil {
		return "", f.renderErr
	}
	return f.renderResult, nil
}

func TestMailServiceMailerAdapterDelegatesSend(t *testing.T) {
	fake := &fakeMailServiceForEmailAdapter{}
	adapter := &mailServiceMailerAdapter{mailService: fake}

	err := adapter.Send(context.Background(), []string{"a@example.com", "b@example.com"}, "subject line", "<p>body</p>")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	if len(fake.sendCalls) != 1 {
		t.Fatalf("expected 1 SendEmail call, got %d", len(fake.sendCalls))
	}
	call := fake.sendCalls[0]
	if call.subject != "subject line" || call.body != "<p>body</p>" {
		t.Fatalf("unexpected call: %+v", call)
	}
	if len(call.to) != 2 || call.to[0] != "a@example.com" || call.to[1] != "b@example.com" {
		t.Fatalf("unexpected recipients: %v", call.to)
	}
}

func TestMailServiceMailerAdapterPropagatesError(t *testing.T) {
	wantErr := fmt.Errorf("smtp exploded")
	fake := &fakeMailServiceForEmailAdapter{sendErr: wantErr}
	adapter := &mailServiceMailerAdapter{mailService: fake}

	err := adapter.Send(context.Background(), []string{"a@example.com"}, "s", "b")
	if err == nil {
		t.Fatal("expected an error to be propagated")
	}
}

func TestMailServiceMailerAdapterNilMailServiceErrors(t *testing.T) {
	adapter := &mailServiceMailerAdapter{mailService: nil}

	if err := adapter.Send(context.Background(), []string{"a@example.com"}, "s", "b"); err == nil {
		t.Fatal("expected an error when mail service is not configured")
	}
}

func TestMailServiceTemplateRendererAdapterDelegatesRender(t *testing.T) {
	fake := &fakeMailServiceForEmailAdapter{renderResult: "<html>rendered</html>"}
	adapter := &mailServiceTemplateRendererAdapter{mailService: fake}

	ts := time.Date(2026, 8, 15, 9, 30, 0, 0, time.UTC)
	msg := notify.Message{Title: "Cert expiring", Body: "example.com expires soon", EventType: "cert", Timestamp: ts}

	html, err := adapter.Render("email_ssl_event.html", msg)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if html != "<html>rendered</html>" {
		t.Fatalf("unexpected html: %q", html)
	}

	if len(fake.renderCalls) != 1 {
		t.Fatalf("expected 1 RenderNotificationEmail call, got %d", len(fake.renderCalls))
	}
	call := fake.renderCalls[0]
	if call.templateName != "email_ssl_event.html" {
		t.Fatalf("templateName = %q, want %q", call.templateName, "email_ssl_event.html")
	}
	if call.data.EventType != "cert" || call.data.Title != "Cert expiring" || call.data.Message != "example.com expires soon" {
		t.Fatalf("unexpected EmailTemplateData: %+v", call.data)
	}
	if call.data.Timestamp != ts.Format(time.RFC3339) {
		t.Fatalf("Timestamp = %q, want %q", call.data.Timestamp, ts.Format(time.RFC3339))
	}
}

func TestMailServiceTemplateRendererAdapterPropagatesError(t *testing.T) {
	fake := &fakeMailServiceForEmailAdapter{renderErr: fmt.Errorf("template missing")}
	adapter := &mailServiceTemplateRendererAdapter{mailService: fake}

	_, err := adapter.Render("missing.html", notify.Message{})
	if err == nil {
		t.Fatal("expected an error to be propagated")
	}
}

func TestMailServiceTemplateRendererAdapterNilMailServiceErrors(t *testing.T) {
	adapter := &mailServiceTemplateRendererAdapter{mailService: nil}

	if _, err := adapter.Render("t.html", notify.Message{}); err == nil {
		t.Fatal("expected an error when mail service is not configured")
	}
}

func TestNewNotifyEmailConfigPreservesSubjectPrefixAndTemplateSelection(t *testing.T) {
	fake := &fakeMailServiceForEmailAdapter{isConfigured: true}
	cfg := NewNotifyEmailConfig(fake, []string{"ops@example.com"})

	if cfg.SubjectPrefix != "[Charon Alert] " {
		t.Fatalf("SubjectPrefix = %q, want %q", cfg.SubjectPrefix, "[Charon Alert] ")
	}
	if len(cfg.Recipients) != 1 || cfg.Recipients[0] != "ops@example.com" {
		t.Fatalf("unexpected recipients: %v", cfg.Recipients)
	}
	if cfg.Mailer == nil {
		t.Fatal("expected a non-nil Mailer")
	}
	if cfg.Renderer == nil {
		t.Fatal("expected a non-nil Renderer")
	}
	if cfg.TemplateName == nil {
		t.Fatal("expected a non-nil TemplateName selector")
	}

	tests := []struct {
		eventType string
		want      string
	}{
		{"security_waf", "email_security_alert.html"},
		{"security_acl", "email_security_alert.html"},
		{"security_rate_limit", "email_security_alert.html"},
		{"security_crowdsec", "email_security_alert.html"},
		{"cert", "email_ssl_event.html"},
		{"uptime", "email_uptime_event.html"},
		{"proxy_host", "email_system_event.html"},
		{"unknown-event", "email_system_event.html"},
	}
	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			got := cfg.TemplateName(notify.Message{EventType: tt.eventType})
			if got != tt.want {
				t.Fatalf("TemplateName(%q) = %q, want %q", tt.eventType, got, tt.want)
			}
			if got != emailTemplateForEventType(tt.eventType) {
				t.Fatalf("TemplateName(%q) diverges from emailTemplateForEventType", tt.eventType)
			}
		})
	}
}

func TestNewNotifyEmailConfigEndToEndSend(t *testing.T) {
	fake := &fakeMailServiceForEmailAdapter{isConfigured: true, renderResult: "<p>hi</p>"}
	cfg := NewNotifyEmailConfig(fake, []string{"ops@example.com"})

	client := email.New(cfg)

	err := client.Send(context.Background(), notify.Message{Title: "Uptime issue", Body: "host down", EventType: "uptime"})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	if len(fake.renderCalls) != 1 || fake.renderCalls[0].templateName != "email_uptime_event.html" {
		t.Fatalf("unexpected render calls: %+v", fake.renderCalls)
	}
	if len(fake.sendCalls) != 1 {
		t.Fatalf("expected 1 SendEmail call, got %d", len(fake.sendCalls))
	}
	if fake.sendCalls[0].subject != "[Charon Alert] Uptime issue" {
		t.Fatalf("subject = %q, want %q", fake.sendCalls[0].subject, "[Charon Alert] Uptime issue")
	}
}
