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
	h = ReverseProxyHandler("localhost:8080", true)
	assert.Equal(t, "reverse_proxy", h["handler"])

	// Test HeaderHandler
	h = HeaderHandler(map[string][]string{"X-Test": {"Value"}})
	assert.Equal(t, "headers", h["handler"])

	// Test BlockExploitsHandler
	h = BlockExploitsHandler()
	assert.Equal(t, "vars", h["handler"])
}

func TestForwardAuthHandler(t *testing.T) {
	t.Run("basic forward auth", func(t *testing.T) {
		h := ForwardAuthHandler("localhost:9000", false)
		assert.Equal(t, "reverse_proxy", h["handler"])
		upstreams := h["upstreams"].([]map[string]interface{})
		assert.Equal(t, "localhost:9000", upstreams[0]["dial"])
		// Without trust forward header, no headers section
		assert.Nil(t, h["headers"])
	})

	t.Run("forward auth with trust forward header", func(t *testing.T) {
		h := ForwardAuthHandler("localhost:9000", true)
		assert.Equal(t, "reverse_proxy", h["handler"])
		// With trust forward header, headers should be set
		headers := h["headers"].(map[string]interface{})
		assert.NotNil(t, headers["request"])
	})
}

func TestSecurityAuthHandler(t *testing.T) {
	h := SecurityAuthHandler("my_portal")
	assert.Equal(t, "authentication", h["handler"])
	assert.Equal(t, "my_portal", h["portal"])
}

func TestSecurityAuthzHandler(t *testing.T) {
	h := SecurityAuthzHandler("my_policy")
	assert.Equal(t, "authorization", h["handler"])
	assert.Equal(t, "my_policy", h["policy"])
}
