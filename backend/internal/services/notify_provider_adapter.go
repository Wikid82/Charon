package services

import (
	"fmt"
	"strings"

	notify "github.com/Wikid82/go_notify_yourself"
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

// providerConfigMap maps a GORM models.NotificationProvider row's
// type-specific fields onto the map[string]any key convention every
// extracted-module HTTP-based provider's Factory expects (per
// docs/plans/notify_provider_registry_spec.md §3.4: "transport" for the
// shared *transport.Wrapper, provider-specific fields under the lowercase
// snake_case name of their typed Config struct field). w is the single
// shared *transport.Wrapper built by NewNotifyTransportWrapper
// (notify_client_adapter.go), injected into every HTTP-based provider.
//
// Field mappings below were read directly out of the old
// notification_service.go sendJSONPayload's provider-specific branches (not
// guessed from the spec's design summary) and are unchanged from the
// pre-registry switch that used to live in buildNotifySender:
//   - discord: webhook_url <- provider.URL
//   - slack: webhook_url <- provider.Token — Slack's decrypted webhook URL
//     is stored in the Token column (provider.URL is an unused placeholder
//     for Slack, matching the old code's `decryptedWebhookURL := p.Token`)
//   - gotify: url <- provider.URL, token <- provider.Token (sent as the
//     X-Gotify-Key header when non-empty)
//   - pushover: user_key <- provider.URL, api_token <- provider.Token
//     (matching the old code's `jsonPayload["user"] = p.URL` /
//     `decryptedToken := p.Token`); base_url left unset, so
//     providers/pushover defaults to the production API
//   - ntfy: url <- provider.URL, token <- provider.Token (sent as an
//     "Authorization: Bearer <token>" header when non-empty)
//   - telegram: bot_token <- provider.Token, chat_id <- provider.URL
//     (matching the old code's `decryptedToken := p.Token` /
//     `jsonPayload["chat_id"] = p.URL`); base_url left unset, so
//     providers/telegram defaults to the production Bot API
//   - webhook / generic: url <- provider.URL, generic JSON passthrough, no
//     provider-specific payload shape or host allowlist
//
// This per-type field mapping is a Charon persistence-schema fact (which
// GORM column means what for which provider type), not something the
// registry can know — collapsing buildNotifySender's dispatch onto
// notify.New (below) removes the "which Go constructor do I call" branch,
// not this one (spec §3.6.1).
func providerConfigMap(provider models.NotificationProvider, w *transport.Wrapper, template, customTemplate string) map[string]any {
	config := map[string]any{
		"transport":       w,
		"template":        template,
		"custom_template": customTemplate,
	}

	switch strings.ToLower(strings.TrimSpace(provider.Type)) {
	case "discord":
		config["webhook_url"] = provider.URL
	case "slack":
		config["webhook_url"] = provider.Token
	case "gotify":
		config["url"] = provider.URL
		config["token"] = provider.Token
	case "pushover":
		config["user_key"] = provider.URL
		config["api_token"] = provider.Token
	case "ntfy":
		config["url"] = provider.URL
		config["token"] = provider.Token
	case "telegram":
		config["bot_token"] = provider.Token
		config["chat_id"] = provider.URL
	case "webhook", "generic":
		config["url"] = provider.URL
	}

	return config
}

// registryTypeForProvider resolves a Charon provider.Type discriminator to
// the name it is registered under in the go_notify_yourself registry.
// "generic" is Charon's own alias for the module's "webhook" provider (the
// pre-registry switch handled both cases identically via webhook.New); the
// registry itself only knows the canonical "webhook" name, so the alias
// must be resolved here before calling notify.New.
func registryTypeForProvider(providerType string) string {
	t := strings.ToLower(strings.TrimSpace(providerType))
	if t == "generic" {
		return "webhook"
	}
	return t
}

// buildNotifySender maps a GORM models.NotificationProvider row into the
// map[string]any config the extracted module's provider registry expects
// (providerConfigMap, above) and constructs the corresponding notify.Sender
// via notify.New — replacing the per-type switch/constructor-call dispatch
// that used to live here (docs/plans/notify_provider_registry_spec.md
// §3.6.1). notify.New itself returns a descriptive error, never panics, for
// an unregistered provider type or a missing/invalid required config key
// (e.g. "transport" absent or nil) — buildNotifySender wraps that error
// rather than reinterpreting it.
//
// Email is handled separately (notify_email_adapter.go / providers/email),
// not by this function — the module's email package has a different shape
// (Mailer/TemplateRenderer, not a Wrapper-backed Sender) and Charon's
// dispatch code never routes an "email" provider.Type through here.
func buildNotifySender(provider models.NotificationProvider, w *transport.Wrapper) (notify.Sender, error) {
	tmpl, customTemplate := resolveTemplateFields(provider)
	config := providerConfigMap(provider, w, tmpl, customTemplate)

	sender, err := notify.New(registryTypeForProvider(provider.Type), config)
	if err != nil {
		return nil, fmt.Errorf("notify provider adapter: %w", err)
	}
	return sender, nil
}
