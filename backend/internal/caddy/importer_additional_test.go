package caddy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImporter_ExtractHosts_DialWithoutPortDefaultsTo80(t *testing.T) {
	importer := NewImporter("caddy")

	rawJSON := []byte("{\"apps\":{\"http\":{\"servers\":{\"srv0\":{\"routes\":[{\"match\":[{\"host\":[\"nop.example.com\"]}],\"handle\":[{\"handler\":\"reverse_proxy\",\"upstreams\":[{\"dial\":\"example.com\"}]}]}]}}}}}")

	res, err := importer.ExtractHosts(rawJSON)
	assert.NoError(t, err)
	assert.Len(t, res.Hosts, 1)
	host := res.Hosts[0]
	assert.Equal(t, "example.com", host.ForwardHost)
	assert.Equal(t, 80, host.ForwardPort)
}

func TestImporter_ExtractHosts_DetectsWebsocketFromHeaders(t *testing.T) {
	importer := NewImporter("caddy")

	rawJSON := []byte("{\"apps\":{\"http\":{\"servers\":{\"srv0\":{\"routes\":[{\"match\":[{\"host\":[\"ws.example.com\"]}],\"handle\":[{\"handler\":\"reverse_proxy\",\"upstreams\":[{\"dial\":\"127.0.0.1:8080\"}],\"headers\":{\"Upgrade\":[\"websocket\"]}}]}]}}}}}")

	res, err := importer.ExtractHosts(rawJSON)
	assert.NoError(t, err)
	assert.Len(t, res.Hosts, 1)
	host := res.Hosts[0]
	assert.True(t, host.WebsocketSupport)
}

func TestImporter_ImportFile_ParseOutputInvalidJSON(t *testing.T) {
	importer := NewImporter("caddy")
	mockExecutor := &MockExecutor{Output: []byte("{invalid"), Err: nil}
	importer.executor = mockExecutor

	// Create a dummy file
	tmpFile := filepath.Join(t.TempDir(), "Caddyfile")
	err := os.WriteFile(tmpFile, []byte("foo"), 0644)
	assert.NoError(t, err)

	_, err = importer.ImportFile(tmpFile)
	assert.Error(t, err)
}

func TestImporter_ImportFile_ExecutorError(t *testing.T) {
	importer := NewImporter("caddy")
	mockExecutor := &MockExecutor{Output: []byte(""), Err: assert.AnError}
	importer.executor = mockExecutor

	// Create a dummy file
	tmpFile := filepath.Join(t.TempDir(), "Caddyfile")
	err := os.WriteFile(tmpFile, []byte("foo"), 0644)
	assert.NoError(t, err)

	_, err = importer.ImportFile(tmpFile)
	assert.Error(t, err)
}
