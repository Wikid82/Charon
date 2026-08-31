package services

// This file exists solely for its side effects: each blank import below
// runs that provider package's init(), which self-registers a
// notify.Factory into the go_notify_yourself registry (notify.Register),
// making the provider constructible via notify.New(provider.Type, config)
// from buildNotifySender (notify_provider_adapter.go).
//
// Deliberately NOT importing providers/all here. Charon hand-picks exactly
// the provider types it currently supports (mirrored by
// isSupportedNotificationProviderType in notification_service.go) rather
// than linking every provider the module ships, per the registry design
// spec (docs/plans/notify_provider_registry_spec.md §3.6.3): Charon's own
// allowlist — not what happens to be linked into the binary — remains the
// gate on what's exposed through its API/UI. Importing providers/all would
// add binary size and transitive dependency surface for providers Charon's
// allowlist would reject anyway.
//
// Keep this list in sync with isSupportedNotificationProviderType's
// allowlist by hand; notification_service_allowlist_registry_test.go
// (or equivalent) asserts that allowlist is a subset of
// notify.RegisteredTypes(), catching drift between the two.
import (
	_ "github.com/Wikid82/go_notify_yourself/providers/discord"
	_ "github.com/Wikid82/go_notify_yourself/providers/email"
	_ "github.com/Wikid82/go_notify_yourself/providers/gotify"
	_ "github.com/Wikid82/go_notify_yourself/providers/ntfy"
	_ "github.com/Wikid82/go_notify_yourself/providers/pushover"
	_ "github.com/Wikid82/go_notify_yourself/providers/slack"
	_ "github.com/Wikid82/go_notify_yourself/providers/telegram"
	_ "github.com/Wikid82/go_notify_yourself/providers/webhook"
)
