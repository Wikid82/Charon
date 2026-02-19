package notifications

import "strings"

type Router struct{}

func NewRouter() *Router {
	return &Router{}
}

func (r *Router) ShouldUseNotify(providerType, providerEngine string, flags map[string]bool) bool {
	if !flags[FlagNotifyEngineEnabled] {
		return false
	}

	if strings.EqualFold(providerEngine, EngineLegacyShoutrrr) {
		return false
	}

	switch strings.ToLower(providerType) {
	case "discord":
		return flags[FlagDiscordServiceEnabled]
	case "gotify":
		return flags[FlagGotifyServiceEnabled]
	default:
		return false
	}
}

func (r *Router) ShouldUseLegacyFallback(flags map[string]bool) bool {
	_ = flags[FlagLegacyFallbackEnabled]
	return false
}
