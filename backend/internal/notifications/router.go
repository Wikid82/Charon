package notifications

import "strings"

// NOTE: used only in tests
type Router struct{}

func NewRouter() *Router {
	return &Router{}
}

func (r *Router) ShouldUseNotify(providerType string, flags map[string]bool) bool {
	if !flags[FlagNotifyEngineEnabled] {
		return false
	}

	switch strings.ToLower(providerType) {
	case "discord":
		return flags[FlagDiscordServiceEnabled]
	case "email":
		return flags[FlagEmailServiceEnabled]
	case "gotify":
		return flags[FlagGotifyServiceEnabled]
	case "webhook":
		return flags[FlagWebhookServiceEnabled]
	default:
		return false
	}
}
