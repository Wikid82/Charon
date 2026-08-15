package services

import (
	"fmt"
	"strings"

	notify "github.com/Wikid82/go_notify_yourself"
	"github.com/Wikid82/go_notify_yourself/providers/discord"
	"github.com/Wikid82/go_notify_yourself/providers/gotify"
	"github.com/Wikid82/go_notify_yourself/providers/ntfy"
	"github.com/Wikid82/go_notify_yourself/providers/pushover"
	"github.com/Wikid82/go_notify_yourself/providers/slack"
	"github.com/Wikid82/go_notify_yourself/providers/telegram"
	"github.com/Wikid82/go_notify_yourself/providers/webhook"
	"github.com/Wikid82/go_notify_yourself/transport"

	"github.com/Wikid82/charon/backend/internal/models"
)

// legacyDetailedTemplate reproduces, verbatim in JSON key structure, the old
// Charon `detailedTemplate` const that lived in notification_service.go's
// sendJSONPayload (and its identical copy in the old RenderTemplate):
//
//	{"title": {{toJSON .Title}}, "message": {{toJSON .Message}}, "time": {{toJSON .Time}},
//	 "event": {{toJSON .EventType}}, "host": {{toJSON .HostName}}, "host_ip": {{toJSON .HostIP}},
//	 "service_count": {{toJSON .ServiceCount}}, "services": {{toJSON .Services}}, "data": {{toJSON .}}}
//
// This is a deliberate backward-compatibility decision (extraction spec
// §3.6 step 5 / §7 risk 1b, "err toward the safer option"): the extracted
// module's own built-in "detailed" template (providers/internal/render's
// DetailedTemplate) nests all host-specific extras under a single "data"
// object via notify.Message.Data instead of exposing them as flat top-level
// JSON fields. Falling through to that new built-in template for an
// already-configured "detailed" provider would silently change the JSON
// payload shape delivered to any existing consumer that parses the old flat
// keys (Discord embed parsers, custom webhook receivers, etc.).
// buildNotifySender/resolveTemplateFields below translate a stored
// `provider.Template == "detailed"` into `Template: "custom"`,
// `CustomTemplate: legacyDetailedTemplate`, so already-configured
// "detailed" providers see zero payload-shape change once cutover (a later
// phase — Commits 3-9) wires this adapter into production dispatch.
//
// Field-access note: the module's shared render engine
// (providers/internal/render.TemplateData) exposes host-specific extras
// under a single `.Data` map (notify.Message.Data), not as top-level
// template fields the way Charon's old flat `data map[string]any` did — so
// `.HostName` becomes `(index .Data "HostName")` here. Go's text/template
// `index` returns the map's zero value (nil, renders as JSON `null`) for a
// missing key, matching the old template's behavior when a caller's data
// map lacked one of these optional fields.
//
// One narrow, intentionally-documented difference from the original: the
// old template's final `"data": {{toJSON .}}` serialized the ENTIRE input
// map — which included Title/Message/Time/EventType as well as the
// extras — under "data", since `.` was the whole flat map passed into
// sendJSONPayload. Here, `"data": {{toJSON .Data}}` serializes only
// notify.Message.Data (the caller-supplied extras), because Title/Message/
// Time/EventType are no longer part of that map — they became top-level
// notify.Message fields. What ends up inside `msg.Data` at cutover time is
// decided by Commits 3-9 (SendExternal's new notify.Message construction),
// not by this file; this comment flags the difference explicitly so that
// decision is made consciously rather than by accident, per the extraction
// spec's instruction not to let payload-shape decisions happen implicitly.
const legacyDetailedTemplate = `{"title": {{toJSON .Title}}, "message": {{toJSON .Message}}, "time": {{toJSON .Time}}, "event": {{toJSON .EventType}}, "host": {{toJSON (index .Data "HostName")}}, "host_ip": {{toJSON (index .Data "HostIP")}}, "service_count": {{toJSON (index .Data "ServiceCount")}}, "services": {{toJSON (index .Data "Services")}}, "data": {{toJSON .Data}}}`

// resolveTemplateFields translates a GORM NotificationProvider row's
// Template/Config columns into the (template, customTemplate) pair every
// extracted provider package's Config expects, applying the
// "detailed" -> flat-shape CustomTemplate backward-compat translation
// documented on legacyDetailedTemplate above. "minimal" and "custom" (and
// any other/empty selector, which the module's own render.SelectTemplate
// treats as "custom") pass through unchanged.
func resolveTemplateFields(provider models.NotificationProvider) (template string, customTemplate string) {
	if strings.EqualFold(strings.TrimSpace(provider.Template), "detailed") {
		return "custom", legacyDetailedTemplate
	}
	return provider.Template, provider.Config
}

// buildNotifySender maps a GORM models.NotificationProvider row into the
// matching extracted-module provider Config and constructs the
// corresponding notify.Sender, per §3.6 step 3 of the extraction spec. w is
// the single shared *transport.Wrapper built by NewNotifyTransportWrapper
// (notify_client_adapter.go), injected into every HTTP-based provider
// package.
//
// Field mappings below were read directly out of the old
// notification_service.go sendJSONPayload's provider-specific branches (not
// guessed from the spec's design summary):
//   - discord: WebhookURL <- provider.URL
//   - slack: WebhookURL <- provider.Token — Slack's decrypted webhook URL is
//     stored in the Token column (provider.URL is an unused placeholder for
//     Slack, matching the old code's `decryptedWebhookURL := p.Token`)
//   - gotify: URL <- provider.URL, Token <- provider.Token (sent as the
//     X-Gotify-Key header when non-empty)
//   - pushover: UserKey <- provider.URL, APIToken <- provider.Token
//     (matching the old code's `jsonPayload["user"] = p.URL` /
//     `decryptedToken := p.Token`); BaseURL left empty, so
//     providers/pushover defaults to the production API
//   - ntfy: URL <- provider.URL, Token <- provider.Token (sent as an
//     "Authorization: Bearer <token>" header when non-empty)
//   - telegram: BotToken <- provider.Token, ChatID <- provider.URL (matching
//     the old code's `decryptedToken := p.Token` /
//     `jsonPayload["chat_id"] = p.URL`); BaseURL left empty, so
//     providers/telegram defaults to the production Bot API
//   - webhook / generic: URL <- provider.URL, generic JSON passthrough, no
//     provider-specific payload shape or host allowlist
//
// Returns an error for any provider.Type not supported by the extracted
// module. Email is handled separately (notify_email_adapter.go /
// providers/email), not by this function — the module's email package has a
// different shape (Mailer/TemplateRenderer, not a Wrapper-backed Sender).
func buildNotifySender(provider models.NotificationProvider, w *transport.Wrapper) (notify.Sender, error) {
	tmpl, customTemplate := resolveTemplateFields(provider)

	switch strings.ToLower(strings.TrimSpace(provider.Type)) {
	case "discord":
		return discord.New(discord.Config{
			WebhookURL:     provider.URL,
			Template:       tmpl,
			CustomTemplate: customTemplate,
		}, w), nil

	case "slack":
		return slack.New(slack.Config{
			WebhookURL:     provider.Token,
			Template:       tmpl,
			CustomTemplate: customTemplate,
		}, w), nil

	case "gotify":
		return gotify.New(gotify.Config{
			URL:            provider.URL,
			Token:          provider.Token,
			Template:       tmpl,
			CustomTemplate: customTemplate,
		}, w), nil

	case "pushover":
		return pushover.New(pushover.Config{
			UserKey:        provider.URL,
			APIToken:       provider.Token,
			Template:       tmpl,
			CustomTemplate: customTemplate,
		}, w), nil

	case "ntfy":
		return ntfy.New(ntfy.Config{
			URL:            provider.URL,
			Token:          provider.Token,
			Template:       tmpl,
			CustomTemplate: customTemplate,
		}, w), nil

	case "telegram":
		return telegram.New(telegram.Config{
			BotToken:       provider.Token,
			ChatID:         provider.URL,
			Template:       tmpl,
			CustomTemplate: customTemplate,
		}, w), nil

	case "webhook", "generic":
		return webhook.New(webhook.Config{
			URL:            provider.URL,
			Template:       tmpl,
			CustomTemplate: customTemplate,
		}, w), nil

	default:
		return nil, fmt.Errorf("notify provider adapter: unsupported provider type %q", provider.Type)
	}
}
