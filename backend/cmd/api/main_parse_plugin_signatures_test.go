package main

import "testing"

func TestParsePluginSignatures(t *testing.T) {
	t.Run("unset env returns nil", func(t *testing.T) {
		t.Setenv("CHARON_PLUGIN_SIGNATURES", "")
		signatures := parsePluginSignatures()
		if signatures != nil {
			t.Fatalf("expected nil signatures when env is unset, got: %#v", signatures)
		}
	})

	t.Run("invalid json returns nil", func(t *testing.T) {
		t.Setenv("CHARON_PLUGIN_SIGNATURES", "{invalid}")
		signatures := parsePluginSignatures()
		if signatures != nil {
			t.Fatalf("expected nil signatures for invalid json, got: %#v", signatures)
		}
	})

	t.Run("invalid prefix returns nil", func(t *testing.T) {
		t.Setenv("CHARON_PLUGIN_SIGNATURES", `{"plugin.so":"md5:deadbeef"}`)
		signatures := parsePluginSignatures()
		if signatures != nil {
			t.Fatalf("expected nil signatures for invalid prefix, got: %#v", signatures)
		}
	})

	t.Run("empty allowlist returns empty map", func(t *testing.T) {
		t.Setenv("CHARON_PLUGIN_SIGNATURES", `{}`)
		signatures := parsePluginSignatures()
		if signatures == nil {
			t.Fatal("expected non-nil empty map for strict empty allowlist")
		}
		if len(signatures) != 0 {
			t.Fatalf("expected empty map, got: %#v", signatures)
		}
	})

	t.Run("valid allowlist returns parsed map", func(t *testing.T) {
		t.Setenv("CHARON_PLUGIN_SIGNATURES", `{"plugin-a.so":"sha256:abc123","plugin-b.so":"sha256:def456"}`)
		signatures := parsePluginSignatures()
		if signatures == nil {
			t.Fatal("expected parsed signatures map, got nil")
		}
		if got := signatures["plugin-a.so"]; got != "sha256:abc123" {
			t.Fatalf("unexpected plugin-a signature: %q", got)
		}
		if got := signatures["plugin-b.so"]; got != "sha256:def456" {
			t.Fatalf("unexpected plugin-b signature: %q", got)
		}
	})
}
