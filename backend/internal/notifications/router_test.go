package notifications

import "testing"

func TestRouter_ShouldUseNotify(t *testing.T) {
	router := NewRouter()

	flags := map[string]bool{
		FlagNotifyEngineEnabled:   true,
		FlagDiscordServiceEnabled: true,
	}

	if !router.ShouldUseNotify("discord", EngineNotifyV1, flags) {
		t.Fatalf("expected notify routing for discord when enabled")
	}

	if router.ShouldUseNotify("discord", EngineLegacy, flags) {
		t.Fatalf("expected legacy engine to stay on legacy path")
	}

	if router.ShouldUseNotify("telegram", EngineNotifyV1, flags) {
		t.Fatalf("expected unsupported service to remain legacy")
	}
}

func TestRouter_ShouldUseLegacyFallback(t *testing.T) {
	router := NewRouter()

	if router.ShouldUseLegacyFallback(map[string]bool{}) {
		t.Fatalf("expected fallback disabled by default")
	}

	// Note: FlagLegacyFallbackEnabled constant has been removed as part of hard-disable
	// Using string literal for test completeness
	if router.ShouldUseLegacyFallback(map[string]bool{"feature.notifications.legacy.fallback_enabled": false}) {
		t.Fatalf("expected fallback disabled when flag is false")
	}

	if router.ShouldUseLegacyFallback(map[string]bool{"feature.notifications.legacy.fallback_enabled": true}) {
		t.Fatalf("expected fallback disabled even when flag is true (hard-disabled)")
	}
}

// TestRouter_ShouldUseNotify_EngineDisabled covers lines 13-14
func TestRouter_ShouldUseNotify_EngineDisabled(t *testing.T) {
	router := NewRouter()

	flags := map[string]bool{
		FlagNotifyEngineEnabled:   false,
		FlagDiscordServiceEnabled: true,
	}

	if router.ShouldUseNotify("discord", EngineNotifyV1, flags) {
		t.Fatalf("expected notify routing disabled when FlagNotifyEngineEnabled is false")
	}
}

// TestRouter_ShouldUseNotify_DiscordServiceFlag covers lines 23-24
func TestRouter_ShouldUseNotify_DiscordServiceFlag(t *testing.T) {
	router := NewRouter()

	flags := map[string]bool{
		FlagNotifyEngineEnabled:   true,
		FlagDiscordServiceEnabled: false,
	}

	if router.ShouldUseNotify("discord", EngineNotifyV1, flags) {
		t.Fatalf("expected notify routing disabled for discord when FlagDiscordServiceEnabled is false")
	}
}
