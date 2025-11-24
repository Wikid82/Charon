package caddy

import (
	"os"
	"path/filepath"
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

type MockExecutor struct {
	Output []byte
	Err    error
}

func (m *MockExecutor) Execute(name string, args ...string) ([]byte, error) {
	return m.Output, m.Err
}

func TestImporter_ParseCaddyfile_Success(t *testing.T) {
	importer := NewImporter("caddy")
	mockExecutor := &MockExecutor{
		Output: []byte(`{"apps": {"http": {"servers": {}}}}`),
		Err:    nil,
	}
	importer.executor = mockExecutor

	// Create a dummy file to bypass os.Stat check
	tmpFile := filepath.Join(t.TempDir(), "Caddyfile")
	err := os.WriteFile(tmpFile, []byte("foo"), 0644)
	assert.NoError(t, err)

	output, err := importer.ParseCaddyfile(tmpFile)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"apps": {"http": {"servers": {}}}}`, string(output))
}

func TestImporter_ParseCaddyfile_Failure(t *testing.T) {
	importer := NewImporter("caddy")
	mockExecutor := &MockExecutor{
		Output: []byte("syntax error"),
		Err:    assert.AnError,
	}
	importer.executor = mockExecutor

	// Create a dummy file
	tmpFile := filepath.Join(t.TempDir(), "Caddyfile")
	err := os.WriteFile(tmpFile, []byte("foo"), 0644)
	assert.NoError(t, err)

	_, err = importer.ParseCaddyfile(tmpFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "caddy adapt failed")
}

func TestImporter_ExtractHosts(t *testing.T) {
	importer := NewImporter("caddy")

	// Test Case 1: Empty Config
	emptyJSON := []byte(`{}`)
	result, err := importer.ExtractHosts(emptyJSON)
	assert.NoError(t, err)
	assert.Empty(t, result.Hosts)

	// Test Case 2: Invalid JSON
	invalidJSON := []byte(`{invalid`)
	_, err = importer.ExtractHosts(invalidJSON)
	assert.Error(t, err)

	// Test Case 3: Valid Config with Reverse Proxy
	validJSON := []byte(`{
		"apps": {
			"http": {
				"servers": {
					"srv0": {
						"routes": [
							{
								"match": [{"host": ["example.com"]}],
								"handle": [
									{
										"handler": "reverse_proxy",
										"upstreams": [{"dial": "127.0.0.1:8080"}]
									}
								]
							}
						]
					}
				}
			}
		}
	}`)
	result, err = importer.ExtractHosts(validJSON)
	assert.NoError(t, err)
	assert.Len(t, result.Hosts, 1)
	assert.Equal(t, "example.com", result.Hosts[0].DomainNames)
	assert.Equal(t, "127.0.0.1", result.Hosts[0].ForwardHost)
	assert.Equal(t, 8080, result.Hosts[0].ForwardPort)

	// Test Case 4: Duplicate Domain
	duplicateJSON := []byte(`{
		"apps": {
			"http": {
				"servers": {
					"srv0": {
						"routes": [
							{
								"match": [{"host": ["example.com"]}],
								"handle": [{"handler": "reverse_proxy"}]
							},
							{
								"match": [{"host": ["example.com"]}],
								"handle": [{"handler": "reverse_proxy"}]
							}
						]
					}
				}
			}
		}
	}`)
	result, err = importer.ExtractHosts(duplicateJSON)
	assert.NoError(t, err)
	assert.Len(t, result.Hosts, 1)
	assert.Len(t, result.Conflicts, 1)
	assert.Equal(t, "example.com", result.Conflicts[0])

	// Test Case 5: Unsupported Features
	unsupportedJSON := []byte(`{
		"apps": {
			"http": {
				"servers": {
					"srv0": {
						"routes": [
							{
								"match": [{"host": ["files.example.com"]}],
								"handle": [
									{"handler": "file_server"},
									{"handler": "rewrite"}
								]
							}
						]
					}
				}
			}
		}
	}`)
	result, err = importer.ExtractHosts(unsupportedJSON)
	assert.NoError(t, err)
	assert.Len(t, result.Hosts, 1)
	assert.Len(t, result.Hosts[0].Warnings, 2)
	assert.Contains(t, result.Hosts[0].Warnings, "File server directives not supported")
	assert.Contains(t, result.Hosts[0].Warnings, "Rewrite rules not supported - manual configuration required")
}

func TestImporter_ImportFile(t *testing.T) {
	importer := NewImporter("caddy")
	mockExecutor := &MockExecutor{
		Output: []byte(`{
			"apps": {
				"http": {
					"servers": {
						"srv0": {
							"routes": [
								{
									"match": [{"host": ["example.com"]}],
									"handle": [
										{
											"handler": "reverse_proxy",
											"upstreams": [{"dial": "127.0.0.1:8080"}]
										}
									]
								}
							]
						}
					}
				}
			}
		}`),
		Err: nil,
	}
	importer.executor = mockExecutor

	// Create a dummy file
	tmpFile := filepath.Join(t.TempDir(), "Caddyfile")
	err := os.WriteFile(tmpFile, []byte("foo"), 0644)
	assert.NoError(t, err)

	result, err := importer.ImportFile(tmpFile)
	assert.NoError(t, err)
	assert.Len(t, result.Hosts, 1)
	assert.Equal(t, "example.com", result.Hosts[0].DomainNames)
}

func TestConvertToProxyHosts(t *testing.T) {
	parsedHosts := []ParsedHost{
		{
			DomainNames:      "example.com",
			ForwardScheme:    "http",
			ForwardHost:      "127.0.0.1",
			ForwardPort:      8080,
			SSLForced:        true,
			WebsocketSupport: true,
		},
		{
			DomainNames: "invalid.com",
			ForwardHost: "", // Invalid
		},
	}

	hosts := ConvertToProxyHosts(parsedHosts)
	assert.Len(t, hosts, 1)
	assert.Equal(t, "example.com", hosts[0].DomainNames)
	assert.Equal(t, "127.0.0.1", hosts[0].ForwardHost)
	assert.Equal(t, 8080, hosts[0].ForwardPort)
	assert.True(t, hosts[0].SSLForced)
	assert.True(t, hosts[0].WebsocketSupport)
}

func TestImporter_ValidateCaddyBinary(t *testing.T) {
	importer := NewImporter("caddy")

	// Success
	importer.executor = &MockExecutor{Output: []byte("v2.0.0"), Err: nil}
	err := importer.ValidateCaddyBinary()
	assert.NoError(t, err)

	// Failure
	importer.executor = &MockExecutor{Output: nil, Err: assert.AnError}
	err = importer.ValidateCaddyBinary()
	assert.Error(t, err)
	assert.Equal(t, "caddy binary not found or not executable", err.Error())
}

func TestBackupCaddyfile(t *testing.T) {
	tmpDir := t.TempDir()
	originalFile := filepath.Join(tmpDir, "Caddyfile")
	err := os.WriteFile(originalFile, []byte("original content"), 0644)
	assert.NoError(t, err)

	backupDir := filepath.Join(tmpDir, "backups")

	// Success
	backupPath, err := BackupCaddyfile(originalFile, backupDir)
	assert.NoError(t, err)
	assert.FileExists(t, backupPath)

	content, err := os.ReadFile(backupPath)
	assert.NoError(t, err)
	assert.Equal(t, "original content", string(content))

	// Failure - Source not found
	_, err = BackupCaddyfile("non-existent", backupDir)
	assert.Error(t, err)
}

func TestDefaultExecutor_Execute(t *testing.T) {
	executor := &DefaultExecutor{}
	output, err := executor.Execute("echo", "hello")
	assert.NoError(t, err)
	assert.Equal(t, "hello\n", string(output))
}
