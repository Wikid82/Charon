package services

// Notification feature-flag keys (models.Setting table). These gate
// per-provider dispatch via NotificationService.isDispatchEnabled /
// getFeatureFlagValue. Moved here from the now-removed internal/notifications
// package (docs/plans/notifications_extraction_spec.md §3.6 step 2) — this is
// Charon policy (which provider types are enabled), not delivery-engine
// logic, so it stays in Charon rather than moving to the extracted
// go_notify_yourself module.
const (
	FlagNotifyEngineEnabled           = "feature.notifications.engine.notify_v1.enabled"
	FlagDiscordServiceEnabled         = "feature.notifications.service.discord.enabled"
	FlagEmailServiceEnabled           = "feature.notifications.service.email.enabled"
	FlagGotifyServiceEnabled          = "feature.notifications.service.gotify.enabled"
	FlagWebhookServiceEnabled         = "feature.notifications.service.webhook.enabled"
	FlagTelegramServiceEnabled        = "feature.notifications.service.telegram.enabled"
	FlagSlackServiceEnabled           = "feature.notifications.service.slack.enabled"
	FlagPushoverServiceEnabled        = "feature.notifications.service.pushover.enabled"
	FlagNtfyServiceEnabled            = "feature.notifications.service.ntfy.enabled"
	FlagSecurityProviderEventsEnabled = "feature.notifications.security_provider_events.enabled"
)
