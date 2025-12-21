package caddy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandlers(t *testing.T) {
	// Test RewriteHandler
	h := RewriteHandler("/new-uri")
	assert.Equal(t, "rewrite", h["handler"])
	assert.Equal(t, "/new-uri", h["uri"])

	// Test FileServerHandler
	h = FileServerHandler("/var/www/html")
	assert.Equal(t, "file_server", h["handler"])
	assert.Equal(t, "/var/www/html", h["root"])

	// Test ReverseProxyHandler
	h = ReverseProxyHandler("localhost:8080", true, "plex", true)
	assert.Equal(t, "reverse_proxy", h["handler"])

	// Test HeaderHandler
	h = HeaderHandler(map[string][]string{"X-Test": {"Value"}})
	assert.Equal(t, "headers", h["handler"])

	// Test BlockExploitsHandler
	h = BlockExploitsHandler()
	assert.Equal(t, "vars", h["handler"])
}

func TestReverseProxyHandler_NoWebSocket(t *testing.T) {
	h := ReverseProxyHandler("localhost:8080", false, "none", false)
	assert.Equal(t, "reverse_proxy", h["handler"])

	// Without WebSocket, Upgrade and Connection headers should not be set
	headers, ok := h["headers"]
	if ok {
		headersMap := headers.(map[string]any)
		requestHeaders := headersMap["request"].(map[string]any)
		setHeaders := requestHeaders["set"].(map[string][]string)
		_, hasUpgrade := setHeaders["Upgrade"]
		_, hasConnection := setHeaders["Connection"]
		assert.False(t, hasUpgrade, "Upgrade header should not be set without WebSocket")
		assert.False(t, hasConnection, "Connection header should not be set without WebSocket")
	}
}

func TestReverseProxyHandler_WithWebSocket(t *testing.T) {
	h := ReverseProxyHandler("localhost:8080", true, "none", false)
	assert.Equal(t, "reverse_proxy", h["handler"])

	// With WebSocket, Upgrade and Connection headers should be set
	headers := h["headers"].(map[string]any)
	requestHeaders := headers["request"].(map[string]any)
	setHeaders := requestHeaders["set"].(map[string][]string)
	assert.Contains(t, setHeaders, "Upgrade")
	assert.Contains(t, setHeaders, "Connection")
}

func TestReverseProxyHandler_StandardHeaders(t *testing.T) {
	h := ReverseProxyHandler("localhost:8080", false, "none", true)
	assert.Equal(t, "reverse_proxy", h["handler"])

	// With standard headers enabled, should have X-Real-IP, X-Forwarded-Proto, etc.
	headers := h["headers"].(map[string]any)
	requestHeaders := headers["request"].(map[string]any)
	setHeaders := requestHeaders["set"].(map[string][]string)
	assert.Contains(t, setHeaders, "X-Real-IP")
	assert.Contains(t, setHeaders, "X-Forwarded-Proto")
	assert.Contains(t, setHeaders, "X-Forwarded-Host")
	assert.Contains(t, setHeaders, "X-Forwarded-Port")
}

func TestReverseProxyHandler_Plex(t *testing.T) {
	h := ReverseProxyHandler("localhost:32400", true, "plex", true)
	assert.Equal(t, "reverse_proxy", h["handler"])

	headers := h["headers"].(map[string]any)
	requestHeaders := headers["request"].(map[string]any)
	setHeaders := requestHeaders["set"].(map[string][]string)

	// Plex-specific headers
	assert.Contains(t, setHeaders, "X-Plex-Client-Identifier")
	assert.Contains(t, setHeaders, "X-Plex-Device")
	assert.Contains(t, setHeaders, "X-Plex-Token")
}

func TestReverseProxyHandler_PlexWithoutStandardHeaders(t *testing.T) {
	// Plex without standard headers should still have X-Real-IP and X-Forwarded-Host for backward compat
	h := ReverseProxyHandler("localhost:32400", true, "plex", false)
	assert.Equal(t, "reverse_proxy", h["handler"])

	headers := h["headers"].(map[string]any)
	requestHeaders := headers["request"].(map[string]any)
	setHeaders := requestHeaders["set"].(map[string][]string)

	// Backward compatibility headers for Plex
	assert.Contains(t, setHeaders, "X-Real-IP")
	assert.Contains(t, setHeaders, "X-Forwarded-Host")
}

func TestReverseProxyHandler_Jellyfin(t *testing.T) {
	h := ReverseProxyHandler("localhost:8096", true, "jellyfin", true)
	assert.Equal(t, "reverse_proxy", h["handler"])

	headers := h["headers"].(map[string]any)
	requestHeaders := headers["request"].(map[string]any)
	setHeaders := requestHeaders["set"].(map[string][]string)

	// Standard headers should be present
	assert.Contains(t, setHeaders, "X-Real-IP")
	assert.Contains(t, setHeaders, "X-Forwarded-Proto")
}

func TestReverseProxyHandler_JellyfinWithoutStandardHeaders(t *testing.T) {
	h := ReverseProxyHandler("localhost:8096", true, "jellyfin", false)
	assert.Equal(t, "reverse_proxy", h["handler"])

	headers := h["headers"].(map[string]any)
	requestHeaders := headers["request"].(map[string]any)
	setHeaders := requestHeaders["set"].(map[string][]string)

	// Backward compatibility headers
	assert.Contains(t, setHeaders, "X-Real-IP")
	assert.Contains(t, setHeaders, "X-Forwarded-Host")
}

func TestReverseProxyHandler_Emby(t *testing.T) {
	h := ReverseProxyHandler("localhost:8096", false, "emby", false)
	assert.Equal(t, "reverse_proxy", h["handler"])

	headers := h["headers"].(map[string]any)
	requestHeaders := headers["request"].(map[string]any)
	setHeaders := requestHeaders["set"].(map[string][]string)

	assert.Contains(t, setHeaders, "X-Real-IP")
	assert.Contains(t, setHeaders, "X-Forwarded-Host")
}

func TestReverseProxyHandler_HomeAssistant(t *testing.T) {
	h := ReverseProxyHandler("localhost:8123", true, "homeassistant", false)
	assert.Equal(t, "reverse_proxy", h["handler"])

	headers := h["headers"].(map[string]any)
	requestHeaders := headers["request"].(map[string]any)
	setHeaders := requestHeaders["set"].(map[string][]string)

	assert.Contains(t, setHeaders, "X-Real-IP")
	assert.Contains(t, setHeaders, "X-Forwarded-Host")
}

func TestReverseProxyHandler_Nextcloud(t *testing.T) {
	h := ReverseProxyHandler("localhost:80", false, "nextcloud", false)
	assert.Equal(t, "reverse_proxy", h["handler"])

	headers := h["headers"].(map[string]any)
	requestHeaders := headers["request"].(map[string]any)
	setHeaders := requestHeaders["set"].(map[string][]string)

	assert.Contains(t, setHeaders, "X-Real-IP")
	assert.Contains(t, setHeaders, "X-Forwarded-Host")
}

func TestReverseProxyHandler_Vaultwarden(t *testing.T) {
	h := ReverseProxyHandler("localhost:80", true, "vaultwarden", false)
	assert.Equal(t, "reverse_proxy", h["handler"])

	headers := h["headers"].(map[string]any)
	requestHeaders := headers["request"].(map[string]any)
	setHeaders := requestHeaders["set"].(map[string][]string)

	assert.Contains(t, setHeaders, "X-Real-IP")
	assert.Contains(t, setHeaders, "X-Forwarded-Host")
}

func TestReverseProxyHandler_UnknownApplication(t *testing.T) {
	h := ReverseProxyHandler("localhost:8080", false, "unknown-app", false)
	assert.Equal(t, "reverse_proxy", h["handler"])

	// Unknown apps without standard headers should have minimal/no extra headers
	_, hasHeaders := h["headers"]
	assert.False(t, hasHeaders, "Unknown app without WS or standard headers should not have headers config")
}

func TestReverseProxyHandler_NoHeaders(t *testing.T) {
	h := ReverseProxyHandler("localhost:8080", false, "", false)
	assert.Equal(t, "reverse_proxy", h["handler"])

	// No websocket, no standard headers, no app = no headers config
	_, hasHeaders := h["headers"]
	assert.False(t, hasHeaders, "Should not have headers config when nothing is enabled")
}

func TestHeaderHandler_EmptyHeaders(t *testing.T) {
	h := HeaderHandler(map[string][]string{})
	assert.Equal(t, "headers", h["handler"])

	response := h["response"].(map[string]any)
	setHeaders := response["set"].(map[string][]string)
	assert.Empty(t, setHeaders)
}

func TestHeaderHandler_MultipleHeaders(t *testing.T) {
	h := HeaderHandler(map[string][]string{
		"X-Frame-Options":        {"DENY"},
		"X-Content-Type-Options": {"nosniff"},
		"X-XSS-Protection":       {"1", "mode=block"},
	})
	assert.Equal(t, "headers", h["handler"])

	response := h["response"].(map[string]any)
	setHeaders := response["set"].(map[string][]string)
	assert.Equal(t, []string{"DENY"}, setHeaders["X-Frame-Options"])
	assert.Equal(t, []string{"nosniff"}, setHeaders["X-Content-Type-Options"])
	assert.Equal(t, []string{"1", "mode=block"}, setHeaders["X-XSS-Protection"])
}
