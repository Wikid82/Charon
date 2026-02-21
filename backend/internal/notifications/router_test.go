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

	if router.ShouldUseLegacyFallback(map[string]bool{FlagLegacyFallbackEnabled: false}) {
		t.Fatalf("expected fallback disabled when flag is false")
	}

	if router.ShouldUseLegacyFallback(map[string]bool{FlagLegacyFallbackEnabled: true}) {
		t.Fatalf("expected fallback disabled even when flag is true")
	}
}
