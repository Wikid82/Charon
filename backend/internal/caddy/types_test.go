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
