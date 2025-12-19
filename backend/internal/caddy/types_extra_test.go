package caddy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReverseProxyHandler_PlexAndOthers(t *testing.T) {
	// Plex should include X-Plex headers and X-Real-IP
	h := ReverseProxyHandler("app:32400", false, "plex")
	require.Equal(t, "reverse_proxy", h["handler"])
	// Assert headers exist
	if hdrs, ok := h["headers"].(map[string]interface{}); ok {
		req := hdrs["request"].(map[string]interface{})
		set := req["set"].(map[string][]string)
		require.Contains(t, set, "X-Plex-Client-Identifier")
		require.Contains(t, set, "X-Real-IP")
	} else {
		t.Fatalf("expected headers map for plex")
	}

	// Jellyfin should include X-Real-IP
	h2 := ReverseProxyHandler("app:8096", true, "jellyfin")
	require.Equal(t, "reverse_proxy", h2["handler"])
	if hdrs, ok := h2["headers"].(map[string]interface{}); ok {
		req := hdrs["request"].(map[string]interface{})
		set := req["set"].(map[string][]string)
		require.Contains(t, set, "X-Real-IP")
	} else {
		t.Fatalf("expected headers map for jellyfin")
	}

	// No websocket means no Upgrade header
	h3 := ReverseProxyHandler("app:80", false, "none")
	if hdrs, ok := h3["headers"].(map[string]interface{}); ok {
		if req, ok := hdrs["request"].(map[string]interface{}); ok {
			if set, ok := req["set"].(map[string][]string); ok {
				require.NotContains(t, set, "Upgrade")
			}
		}
	}
}

func TestReverseProxyHandler_WebSocketHeaders(t *testing.T) {
	// Test: WebSocket enabled should include X-Forwarded headers
	h := ReverseProxyHandler("app:8080", true, "none")
	require.Equal(t, "reverse_proxy", h["handler"])

	hdrs, ok := h["headers"].(map[string]interface{})
	require.True(t, ok, "expected headers map when enableWS=true")

	req, ok := hdrs["request"].(map[string]interface{})
	require.True(t, ok, "expected request headers")

	set, ok := req["set"].(map[string][]string)
	require.True(t, ok, "expected set headers")

	// Verify WebSocket passthrough headers
	require.Contains(t, set, "Upgrade", "Upgrade header should be set for WebSocket")
	require.Equal(t, []string{"{http.request.header.Upgrade}"}, set["Upgrade"])

	require.Contains(t, set, "Connection", "Connection header should be set for WebSocket")
	require.Equal(t, []string{"{http.request.header.Connection}"}, set["Connection"])

	// Verify X-Forwarded headers for proxy awareness
	require.Contains(t, set, "X-Forwarded-Proto", "X-Forwarded-Proto should be set for WebSocket")
	require.Equal(t, []string{"{http.request.scheme}"}, set["X-Forwarded-Proto"])

	require.Contains(t, set, "X-Forwarded-Host", "X-Forwarded-Host should be set for WebSocket")
	require.Equal(t, []string{"{http.request.host}"}, set["X-Forwarded-Host"])

	require.Contains(t, set, "X-Real-IP", "X-Real-IP should be set for WebSocket")
	require.Equal(t, []string{"{http.request.remote.host}"}, set["X-Real-IP"])
}

func TestReverseProxyHandler_NoWebSocketNoForwardedHeaders(t *testing.T) {
	// Test: WebSocket disabled with no application should NOT have X-Forwarded headers
	h := ReverseProxyHandler("app:8080", false, "none")
	require.Equal(t, "reverse_proxy", h["handler"])

	// With enableWS=false and application="none", there should be no headers config
	_, ok := h["headers"]
	require.False(t, ok, "expected no headers when enableWS=false and application=none")
}
