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
