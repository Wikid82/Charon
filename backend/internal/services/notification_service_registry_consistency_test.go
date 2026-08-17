package services

import (
	"testing"

	notify "github.com/Wikid82/go_notify_yourself"
)

// TestSupportedProviderAllowlistIsSubsetOfRegisteredTypes is a
// build-configuration drift guard, not a runtime assertion (per
// docs/plans/notify_provider_registry_spec.md §3.6.2/§3.10): it asserts
// that every provider type isSupportedNotificationProviderType claims to
// support is actually registered in the go_notify_yourself registry given
// Charon's hand-picked blank imports (notify_providers_import.go).
//
// This deliberately does NOT run in the other direction — the registry may
// (and, per notify_providers_import.go's doc comment, currently does not)
// contain types Charon's allowlist doesn't yet expose; that's the expected,
// intentional state per §3.6.2 Option A (Charon curates its own supported
// surface independently of what's merely constructible). If this test ever
// fails, it means Charon's allowlist claims to support a provider type
// whose package was never blank-imported for registration — a drift bug to
// fix by adding the missing import, not by touching
// isSupportedNotificationProviderType itself.
func TestSupportedProviderAllowlistIsSubsetOfRegisteredTypes(t *testing.T) {
	registered := make(map[string]struct{}, len(notify.RegisteredTypes()))
	for _, name := range notify.RegisteredTypes() {
		registered[name] = struct{}{}
	}

	// Mirrors isSupportedNotificationProviderType's exact case list
	// (notification_service.go) — kept as a literal list here rather than
	// derived from the function itself, since that switch has no
	// enumerable form to introspect.
	supportedTypes := []string{"discord", "email", "gotify", "webhook", "telegram", "slack", "pushover", "ntfy"}

	for _, providerType := range supportedTypes {
		if !isSupportedNotificationProviderType(providerType) {
			t.Fatalf("test bug: %q is not actually in isSupportedNotificationProviderType's allowlist", providerType)
		}
		if _, ok := registered[providerType]; !ok {
			t.Errorf("provider type %q is in isSupportedNotificationProviderType's allowlist but is not registered "+
				"in the notify registry (registered types: %v) — a package that registers it under notify.Register "+
				"is missing a blank import in notify_providers_import.go", providerType, notify.RegisteredTypes())
		}
	}
}
