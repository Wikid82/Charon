package services

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	notify "github.com/Wikid82/go_notify_yourself"
	"github.com/Wikid82/go_notify_yourself/providers/email"

	"github.com/Wikid82/charon/backend/internal/logger"
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
//
// Render deliberately never returns an error for a template-rendering
// failure: email.Client.Send (providers/email/email.go) aborts before ever
// calling Mailer.Send if Renderer.Render returns an error, which would leave
// a broken/missing HTML template silently dropping notifications. That is
// not an approved behavior change from pre-extraction Charon, where
// dispatchEmail degraded to a manually built plain-HTML body and still sent
// the email. So on a RenderNotificationEmail failure, Render logs a warning
// and returns fallbackEmailBody's plain-HTML rendering instead of an error —
// restoring the old fail-open-with-degraded-body behavior entirely within
// this adapter, without needing error-type introspection across the
// extracted module's boundary. Real Mailer/SMTP transport failures are
// unaffected by this — those still occur (and propagate as errors) later,
// in mailServiceMailerAdapter.Send.
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
		logger.Log().WithError(err).WithField("template", templateName).
			Warn("Email template rendering failed, using fallback body")
		return fallbackEmailBody(msg.Title, msg.Body), nil
	}
	return htmlBody, nil
}

// fallbackEmailBody builds a minimal plain-HTML email body from a message's
// title/body when template rendering fails, matching the exact fallback
// format the old (pre-extraction) dispatchEmail used. By the time Render is
// called, email.Client.Send has already run msg.Normalized() and its own
// sanitizeForEmail over msg.Title/msg.Body, stripping control characters —
// so this only needs to HTML-escape them before embedding.
func fallbackEmailBody(title, body string) string {
	var b strings.Builder
	if title != "" {
		b.WriteString("<strong>")
		b.WriteString(html.EscapeString(title))
		b.WriteString("</strong>")
	}
	if body != "" {
		if b.Len() > 0 {
			b.WriteString("<br>")
		}
		b.WriteString(html.EscapeString(body))
	}
	return b.String()
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
