package services

import (
	"context"
	"fmt"
	"time"

	notify "github.com/Wikid82/go_notify_yourself"
	"github.com/Wikid82/go_notify_yourself/providers/email"
)

// mailServiceMailerAdapter implements email.Mailer by delegating to Charon's
// existing MailServiceInterface.SendEmail — the extracted module never dials
// SMTP directly (§3.3.4 of the extraction spec); Charon supplies transport
// behind this interface.
type mailServiceMailerAdapter struct {
	mailService MailServiceInterface
}

var _ email.Mailer = (*mailServiceMailerAdapter)(nil)

func (a *mailServiceMailerAdapter) Send(ctx context.Context, recipients []string, subject, htmlBody string) error {
	if a.mailService == nil {
		return fmt.Errorf("notify email adapter: mail service is not configured")
	}
	if err := a.mailService.SendEmail(ctx, recipients, subject, htmlBody); err != nil {
		return fmt.Errorf("notify email adapter: send email: %w", err)
	}
	return nil
}

// mailServiceTemplateRendererAdapter implements email.TemplateRenderer by
// delegating to Charon's existing MailServiceInterface.RenderNotificationEmail
// and Charon's five branded HTML templates, mapping notify.Message fields
// onto Charon's EmailTemplateData shape. The extracted module's own neutral
// default template is intentionally never used in production — Charon always
// supplies this renderer (§3.3.4's required-override tradeoff).
type mailServiceTemplateRendererAdapter struct {
	mailService MailServiceInterface
}

var _ email.TemplateRenderer = (*mailServiceTemplateRendererAdapter)(nil)

func (a *mailServiceTemplateRendererAdapter) Render(templateName string, msg notify.Message) (string, error) {
	if a.mailService == nil {
		return "", fmt.Errorf("notify email adapter: mail service is not configured")
	}

	data := EmailTemplateData{
		EventType: msg.EventType,
		Title:     msg.Title,
		Message:   msg.Body,
		Timestamp: msg.Timestamp.Format(time.RFC3339),
	}

	htmlBody, err := a.mailService.RenderNotificationEmail(templateName, data)
	if err != nil {
		return "", fmt.Errorf("notify email adapter: render template: %w", err)
	}
	return htmlBody, nil
}

// NewNotifyEmailConfig builds an email.Config wired to Charon's existing
// mail service, preserving the exact subject-prefix and template-selection
// behavior of the old dispatchEmail/emailTemplateForEventType logic (§3.1.3 /
// §3.6 step 3 of the extraction spec):
//   - SubjectPrefix is "[Charon Alert] ", matching the old
//     fmt.Sprintf("[Charon Alert] %s", safeTitle) subject format exactly
//     (email.Client.Send builds the subject as SubjectPrefix + msg.Title).
//   - TemplateName wraps emailTemplateForEventType (kept in Charon per
//     §3.1.3 — Charon's own event-type -> HTML template name mapping is
//     product logic, not engine logic).
//   - Renderer/Mailer wrap Charon's existing mailService via the two
//     adapters above, so email dispatch keeps using Charon's five branded
//     HTML templates and real SMTP transport — never the extracted module's
//     neutral built-in default.
func NewNotifyEmailConfig(mailService MailServiceInterface, recipients []string) email.Config {
	return email.Config{
		Recipients:    recipients,
		SubjectPrefix: "[Charon Alert] ",
		TemplateName: func(msg notify.Message) string {
			return emailTemplateForEventType(msg.EventType)
		},
		Renderer: &mailServiceTemplateRendererAdapter{mailService: mailService},
		Mailer:   &mailServiceMailerAdapter{mailService: mailService},
	}
}
