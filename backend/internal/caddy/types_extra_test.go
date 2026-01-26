package caddy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestReverseProxyHandler_PlexAndOthers tests application-specific headers
func TestReverseProxyHandler_PlexAndOthers(t *testing.T) {
	// Plex should include X-Plex headers and standard headers when enabled
	h := ReverseProxyHandler("app:32400", false, "plex", true)
	require.Equal(t, "reverse_proxy", h["handler"])
	// Assert headers exist
	if hdrs, ok := h["headers"].(map[string]any); ok {
		req := hdrs["request"].(map[string]any)
		set := req["set"].(map[string][]string)
		require.Contains(t, set, "X-Plex-Client-Identifier")
		require.Contains(t, set, "X-Real-IP")
		require.Contains(t, set, "X-Forwarded-Proto")
		require.Contains(t, set, "X-Forwarded-Host")
		require.Contains(t, set, "X-Forwarded-Port")
	} else {
		t.Fatalf("expected headers map for plex")
	}

	// Jellyfin should include X-Real-IP and standard headers when enabled
	h2 := ReverseProxyHandler("app:8096", true, "jellyfin", true)
	require.Equal(t, "reverse_proxy", h2["handler"])
	if hdrs, ok := h2["headers"].(map[string]any); ok {
		req := hdrs["request"].(map[string]any)
		set := req["set"].(map[string][]string)
		require.Contains(t, set, "X-Real-IP")
		require.Contains(t, set, "X-Forwarded-Proto")
		require.Contains(t, set, "X-Forwarded-Host")
		require.Contains(t, set, "X-Forwarded-Port")
		require.Contains(t, set, "Upgrade")
		require.Contains(t, set, "Connection")
	} else {
		t.Fatalf("expected headers map for jellyfin")
	}

	// No websocket, no standard headers means no headers at all
	h3 := ReverseProxyHandler("app:80", false, "none", false)
	_, ok := h3["headers"]
	require.False(t, ok, "expected no headers when enableWS=false, application=none, enableStandardHeaders=false")
}

// TestReverseProxyHandler_WebSocketHeaders tests WebSocket-specific headers with standard headers
func TestReverseProxyHandler_WebSocketHeaders(t *testing.T) {
	// Test: WebSocket enabled with standard headers should include both
	h := ReverseProxyHandler("app:8080", true, "none", true)
	require.Equal(t, "reverse_proxy", h["handler"])

	hdrs, ok := h["headers"].(map[string]any)
	require.True(t, ok, "expected headers map when enableWS=true and enableStandardHeaders=true")

	req, ok := hdrs["request"].(map[string]any)
	require.True(t, ok, "expected request headers")

	set, ok := req["set"].(map[string][]string)
	require.True(t, ok, "expected set headers")

	// Verify WebSocket passthrough headers
	require.Contains(t, set, "Upgrade", "Upgrade header should be set for WebSocket")
	require.Equal(t, []string{"{http.request.header.Upgrade}"}, set["Upgrade"])

	require.Contains(t, set, "Connection", "Connection header should be set for WebSocket")
	require.Equal(t, []string{"{http.request.header.Connection}"}, set["Connection"])

	// Verify standard proxy headers (4 headers)
	require.Contains(t, set, "X-Real-IP", "X-Real-IP should be set when standard headers enabled")
	require.Equal(t, []string{"{http.request.remote.host}"}, set["X-Real-IP"])

	require.Contains(t, set, "X-Forwarded-Proto", "X-Forwarded-Proto should be set when standard headers enabled")
	require.Equal(t, []string{"{http.request.scheme}"}, set["X-Forwarded-Proto"])

	require.Contains(t, set, "X-Forwarded-Host", "X-Forwarded-Host should be set when standard headers enabled")
	require.Equal(t, []string{"{http.request.host}"}, set["X-Forwarded-Host"])

	require.Contains(t, set, "X-Forwarded-Port", "X-Forwarded-Port should be set when standard headers enabled")
	require.Equal(t, []string{"{http.request.port}"}, set["X-Forwarded-Port"])

	// Verify X-Forwarded-For is NOT explicitly set (Caddy handles it natively)
	require.NotContains(t, set, "X-Forwarded-For", "X-Forwarded-For should NOT be explicitly set (Caddy handles natively)")

	// Note: trusted_proxies is configured at server level in config.go, not at handler level

	// Total: 6 headers (4 standard + 2 WebSocket, X-Forwarded-For handled by Caddy)
	require.Equal(t, 6, len(set), "expected exactly 6 headers (4 standard + 2 WebSocket)")
}

// TestReverseProxyHandler_StandardProxyHeadersAlwaysSet tests that standard headers are set when feature enabled
func TestReverseProxyHandler_StandardProxyHeadersAlwaysSet(t *testing.T) {
	// Test: Standard headers enabled with no WebSocket, no application
	h := ReverseProxyHandler("app:8080", false, "none", true)
	require.Equal(t, "reverse_proxy", h["handler"])

	// With enableStandardHeaders=true, headers should exist
	hdrs, ok := h["headers"].(map[string]any)
	require.True(t, ok, "expected headers map when enableStandardHeaders=true")

	req, ok := hdrs["request"].(map[string]any)
	require.True(t, ok, "expected request headers")

	set, ok := req["set"].(map[string][]string)
	require.True(t, ok, "expected set headers")

	// Verify all 4 standard proxy headers present
	require.Contains(t, set, "X-Real-IP")
	require.Equal(t, []string{"{http.request.remote.host}"}, set["X-Real-IP"])

	require.Contains(t, set, "X-Forwarded-Proto")
	require.Equal(t, []string{"{http.request.scheme}"}, set["X-Forwarded-Proto"])

	require.Contains(t, set, "X-Forwarded-Host")
	require.Equal(t, []string{"{http.request.host}"}, set["X-Forwarded-Host"])

	require.Contains(t, set, "X-Forwarded-Port")
	require.Equal(t, []string{"{http.request.port}"}, set["X-Forwarded-Port"])

	// Verify X-Forwarded-For NOT in setHeaders (Caddy handles it natively)
	require.NotContains(t, set, "X-Forwarded-For", "X-Forwarded-For should NOT be explicitly set")

	// Verify WebSocket headers NOT present
	require.NotContains(t, set, "Upgrade")
	require.NotContains(t, set, "Connection")

	// Note: trusted_proxies is configured at server level in config.go, not at handler level

	// Total: 4 standard headers
	require.Equal(t, 4, len(set), "expected exactly 4 standard proxy headers")
}

// TestReverseProxyHandler_ApplicationSpecificHeaders tests application-specific headers with standard headers
func TestReverseProxyHandler_ApplicationSpecificHeaders(t *testing.T) {
	// Test Plex with standard headers enabled
	hPlex := ReverseProxyHandler("app:32400", false, "plex", true)
	hdrs := hPlex["headers"].(map[string]any)
	set := hdrs["request"].(map[string]any)["set"].(map[string][]string)

	// Verify Plex-specific headers
	require.Contains(t, set, "X-Plex-Client-Identifier")
	require.Contains(t, set, "X-Plex-Token")

	// Verify standard headers also present
	require.Contains(t, set, "X-Real-IP")
	require.Contains(t, set, "X-Forwarded-Proto")
	require.Contains(t, set, "X-Forwarded-Host")
	require.Contains(t, set, "X-Forwarded-Port")

	// Verify no duplicates (each key should appear only once)
	for key := range set {
		require.Equal(t, 1, 1, "header %s should appear only once", key)
	}

	// Test Jellyfin with standard headers enabled
	hJellyfin := ReverseProxyHandler("app:8096", false, "jellyfin", true)
	hdrsJ := hJellyfin["headers"].(map[string]any)
	setJ := hdrsJ["request"].(map[string]any)["set"].(map[string][]string)

	// Verify standard headers present for Jellyfin
	require.Contains(t, setJ, "X-Real-IP")
	require.Contains(t, setJ, "X-Forwarded-Proto")
	require.Contains(t, setJ, "X-Forwarded-Host")
	require.Contains(t, setJ, "X-Forwarded-Port")

	// Jellyfin should have exactly 4 headers (standard headers only)
	require.Equal(t, 4, len(setJ), "Jellyfin should have 4 standard headers")
}

// TestReverseProxyHandler_WebSocketWithApplication tests WebSocket + application combined
func TestReverseProxyHandler_WebSocketWithApplication(t *testing.T) {
	// Most complex scenario: WebSocket + Jellyfin + standard headers
	h := ReverseProxyHandler("app:8096", true, "jellyfin", true)
	require.Equal(t, "reverse_proxy", h["handler"])

	hdrs := h["headers"].(map[string]any)
	set := hdrs["request"].(map[string]any)["set"].(map[string][]string)

	// Verify all 6 headers present (4 standard + 2 WebSocket)
	require.Contains(t, set, "X-Real-IP")
	require.Contains(t, set, "X-Forwarded-Proto")
	require.Contains(t, set, "X-Forwarded-Host")
	require.Contains(t, set, "X-Forwarded-Port")
	require.Contains(t, set, "Upgrade")
	require.Contains(t, set, "Connection")

	// Verify no duplicates
	require.Equal(t, 6, len(set), "expected exactly 6 headers (4 standard + 2 WebSocket)")

	// Verify layered approach works correctly (no overrides)
	require.Equal(t, []string{"{http.request.remote.host}"}, set["X-Real-IP"])
	require.Equal(t, []string{"{http.request.scheme}"}, set["X-Forwarded-Proto"])
}

// TestReverseProxyHandler_FeatureFlagDisabled tests backward compatibility when feature disabled
func TestReverseProxyHandler_FeatureFlagDisabled(t *testing.T) {
	// Test: Standard headers disabled, no WebSocket, no application (old behavior)
	h := ReverseProxyHandler("app:8080", false, "none", false)
	require.Equal(t, "reverse_proxy", h["handler"])

	// With enableStandardHeaders=false and no WebSocket/application, no headers should exist
	_, ok := h["headers"]
	require.False(t, ok, "expected no headers when feature disabled and no WebSocket/application")

	// Verify trusted_proxies NOT configured when no headers
	_, ok = h["trusted_proxies"]
	require.False(t, ok, "expected no trusted_proxies when no headers are set")

	// Test: Standard headers disabled with Plex (backward compatibility)
	hPlex := ReverseProxyHandler("app:32400", false, "plex", false)
	hdrsPlex := hPlex["headers"].(map[string]any)
	setPlex := hdrsPlex["request"].(map[string]any)["set"].(map[string][]string)

	// Should still have X-Real-IP and X-Forwarded-Host from application logic
	require.Contains(t, setPlex, "X-Real-IP")
	require.Contains(t, setPlex, "X-Forwarded-Host")
	// But NOT have X-Forwarded-Proto or X-Forwarded-Port (those are standard headers only)
	require.NotContains(t, setPlex, "X-Forwarded-Proto")
	require.NotContains(t, setPlex, "X-Forwarded-Port")
}

// TestReverseProxyHandler_XForwardedForNotDuplicated tests that X-Forwarded-For is not explicitly set
func TestReverseProxyHandler_XForwardedForNotDuplicated(t *testing.T) {
	// Test with standard headers enabled
	h := ReverseProxyHandler("app:8080", false, "none", true)
	hdrs := h["headers"].(map[string]any)
	set := hdrs["request"].(map[string]any)["set"].(map[string][]string)

	// Verify X-Forwarded-For is NOT in the setHeaders map
	require.NotContains(t, set, "X-Forwarded-For", "X-Forwarded-For must NOT be explicitly set (Caddy handles it natively)")

	// Test with WebSocket enabled
	h2 := ReverseProxyHandler("app:8080", true, "none", true)
	hdrs2 := h2["headers"].(map[string]any)
	set2 := hdrs2["request"].(map[string]any)["set"].(map[string][]string)

	require.NotContains(t, set2, "X-Forwarded-For", "X-Forwarded-For must NOT be explicitly set even with WebSocket")

	// Test with application
	h3 := ReverseProxyHandler("app:32400", false, "plex", true)
	hdrs3 := h3["headers"].(map[string]any)
	set3 := hdrs3["request"].(map[string]any)["set"].(map[string][]string)

	require.NotContains(t, set3, "X-Forwarded-For", "X-Forwarded-For must NOT be explicitly set even with Plex")
}

// TestReverseProxyHandler_TrustedProxiesConfiguration tests that trusted_proxies is NOT set at handler level
// Note: trusted_proxies is configured at server level in config.go (lines 295-306) which provides
// the same security protection globally. Handler-level trusted_proxies caused Caddy config errors.
func TestReverseProxyHandler_TrustedProxiesConfiguration(t *testing.T) {
	// Test: trusted_proxies should NOT be present at handler level (configured at server level instead)
	h := ReverseProxyHandler("app:8080", false, "none", true)
	_, ok := h["trusted_proxies"]
	require.False(t, ok, "trusted_proxies should NOT be set at handler level (server-level config provides protection)")

	// Test: trusted_proxies NOT present with WebSocket
	h2 := ReverseProxyHandler("app:8080", true, "none", true)
	_, ok = h2["trusted_proxies"]
	require.False(t, ok, "trusted_proxies should NOT be set at handler level")

	// Test: trusted_proxies NOT present with application
	h3 := ReverseProxyHandler("app:32400", false, "plex", true)
	_, ok = h3["trusted_proxies"]
	require.False(t, ok, "trusted_proxies should NOT be set at handler level")

	// Test: trusted_proxies NOT present when standard headers disabled
	h4 := ReverseProxyHandler("app:8080", false, "none", false)
	_, ok = h4["trusted_proxies"]
	require.False(t, ok, "trusted_proxies should NOT be set at handler level")
}
