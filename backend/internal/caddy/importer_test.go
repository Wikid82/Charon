package caddy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewImporter(t *testing.T) {
	importer := NewImporter("/usr/bin/caddy")
	assert.NotNil(t, importer)
	assert.Equal(t, "/usr/bin/caddy", importer.caddyBinaryPath)

	importerDefault := NewImporter("")
	assert.NotNil(t, importerDefault)
	assert.Equal(t, "caddy", importerDefault.caddyBinaryPath)
}

func TestImporter_ParseCaddyfile_NotFound(t *testing.T) {
	importer := NewImporter("caddy")
	_, err := importer.ParseCaddyfile("non-existent-file")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "caddyfile not found")
}
