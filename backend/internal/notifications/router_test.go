package notifications

import "testing"

func TestRouter_ShouldUseNotify(t *testing.T) {
	router := NewRouter()

	flags := map[string]bool{
		FlagNotifyEngineEnabled:   true,
		FlagDiscordServiceEnabled: true,
	}

	if !router.ShouldUseNotify("discord", flags) {
		t.Fatalf("expected notify routing for discord when enabled")
	}

	if router.ShouldUseNotify("telegram", flags) {
		t.Fatalf("expected unsupported service to remain legacy")
	}
}

// TestRouter_ShouldUseNotify_EngineDisabled covers lines 13-14
func TestRouter_ShouldUseNotify_EngineDisabled(t *testing.T) {
	router := NewRouter()

	flags := map[string]bool{
		FlagNotifyEngineEnabled:   false,
		FlagDiscordServiceEnabled: true,
	}

	if router.ShouldUseNotify("discord", flags) {
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

	if router.ShouldUseNotify("discord", flags) {
		t.Fatalf("expected notify routing disabled for discord when FlagDiscordServiceEnabled is false")
	}
}

// TestRouter_ShouldUseNotify_GotifyServiceFlag covers lines 23-24 (gotify case)
func TestRouter_ShouldUseNotify_GotifyServiceFlag(t *testing.T) {
	router := NewRouter()

	// Test with gotify enabled
	flags := map[string]bool{
		FlagNotifyEngineEnabled:  true,
		FlagGotifyServiceEnabled: true,
	}

	if !router.ShouldUseNotify("gotify", flags) {
		t.Fatalf("expected notify routing enabled for gotify when FlagGotifyServiceEnabled is true")
	}

	// Test with gotify disabled
	flags[FlagGotifyServiceEnabled] = false

	if router.ShouldUseNotify("gotify", flags) {
		t.Fatalf("expected notify routing disabled for gotify when FlagGotifyServiceEnabled is false")
	}
}

func TestRouter_ShouldUseNotify_WebhookServiceFlag(t *testing.T) {
	router := NewRouter()

	flags := map[string]bool{
		FlagNotifyEngineEnabled:   true,
		FlagWebhookServiceEnabled: true,
	}

	if !router.ShouldUseNotify("webhook", flags) {
		t.Fatalf("expected notify routing enabled for webhook when FlagWebhookServiceEnabled is true")
	}

	flags[FlagWebhookServiceEnabled] = false
	if router.ShouldUseNotify("webhook", flags) {
		t.Fatalf("expected notify routing disabled for webhook when FlagWebhookServiceEnabled is false")
	}
}
