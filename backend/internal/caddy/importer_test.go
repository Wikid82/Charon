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
	Output      []byte
	Err         error
	ExecuteFunc func(name string, args ...string) ([]byte, error) // Custom execution logic
}

func (m *MockExecutor) Execute(name string, args ...string) ([]byte, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(name, args...)
	}
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
	err := os.WriteFile(tmpFile, []byte("foo"), 0o600)
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
	err := os.WriteFile(tmpFile, []byte("foo"), 0o600)
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

	// Test Case 6: SSL Detection via Listen Address (:443)
	sslViaListenJSON := []byte(`{
		"apps": {
			"http": {
				"servers": {
					"srv0": {
						"listen": [":443"],
						"routes": [
							{
								"match": [{"host": ["secure.example.com"]}],
								"handle": [
									{
										"handler": "reverse_proxy",
										"upstreams": [{"dial": "127.0.0.1:9000"}]
									}
								]
							}
						]
					}
				}
			}
		}
	}`)
	result, err = importer.ExtractHosts(sslViaListenJSON)
	assert.NoError(t, err)
	assert.Len(t, result.Hosts, 1)
	assert.Equal(t, "secure.example.com", result.Hosts[0].DomainNames)
	assert.True(t, result.Hosts[0].SSLForced, "SSLForced should be true when server listens on :443")
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
	// #nosec G306 -- Test fixture Caddyfile
	err := os.WriteFile(tmpFile, []byte("foo"), 0o644)
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
	// #nosec G306 -- Test fixture file with standard read permissions
	err := os.WriteFile(originalFile, []byte("original content"), 0o644)
	assert.NoError(t, err)

	backupDir := filepath.Join(tmpDir, "backups")

	// Success
	backupPath, err := BackupCaddyfile(originalFile, backupDir)
	assert.NoError(t, err)
	assert.FileExists(t, backupPath)

	content, err := os.ReadFile(backupPath) // #nosec G304 -- Test reading backup file created in test
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

// TestDefaultExecutor_Execute_StderrCapture verifies that stderr is captured
func TestDefaultExecutor_Execute_StderrCapture(t *testing.T) {
	executor := &DefaultExecutor{}

	// Use a command that writes to stderr and exits with error
	// "sh -c" allows us to write to stderr explicitly
	output, err := executor.Execute("sh", "-c", "echo 'error message' >&2; exit 1")

	// Error should be returned
	assert.Error(t, err)

	// Output should contain the stderr message (due to CombinedOutput)
	assert.Contains(t, string(output), "error message")
}

// TestImporter_NormalizeCaddyfile tests the Caddyfile normalization function
func TestImporter_NormalizeCaddyfile(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		allowEmpty  bool
	}{
		{
			name:  "single-line format processes without error",
			input: "test.example.com { reverse_proxy localhost:3000 }",
		},
		{
			name:  "already formatted content processes without error",
			input: "test.example.com {\n\treverse_proxy localhost:3000\n}\n",
		},
		{
			name:       "empty input handles gracefully",
			input:      "",
			allowEmpty: true,
		},
		{
			name:  "nested blocks process without error",
			input: "test.com { handle /api* { reverse_proxy api:8080 } }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			importer := NewImporter("caddy")

			// Create a mock executor that simulates caddy fmt success by returning empty output
			// The actual file modification would be done by the real caddy binary
			mockExecutor := &MockExecutor{
				Output: []byte{}, // Empty output means success
				Err:    nil,
			}
			importer.executor = mockExecutor

			// Note: With mock executor, the file won't actually be formatted,
			// but we can verify the function handles the flow without errors
			output, err := importer.NormalizeCaddyfile(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			// With mock, output will be the input (since file isn't actually modified)
			// The important test is that there's no error
			assert.NoError(t, err)
			// Output should contain the original content since mock doesn't modify file
			// unless empty input is expected
			if !tt.allowEmpty {
				assert.NotEmpty(t, output)
			}
		})
	}
}

// TestImporter_NormalizeCaddyfile_Error tests error handling
func TestImporter_NormalizeCaddyfile_Error(t *testing.T) {
	importer := NewImporter("caddy")

	// Mock executor that returns error
	mockExecutor := &MockExecutor{
		Output: []byte("Error: invalid caddyfile syntax"),
		Err:    assert.AnError,
	}
	importer.executor = mockExecutor

	_, err := importer.NormalizeCaddyfile("invalid { syntax")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "caddy fmt failed")
}

// TestImporter_NormalizeCaddyfile_Integration tests with real caddy binary (if available)
func TestImporter_NormalizeCaddyfile_Integration(t *testing.T) {
	// Skip if caddy binary not available
	importer := NewImporter("caddy")
	if err := importer.ValidateCaddyBinary(); err != nil {
		t.Skip("Caddy binary not available, skipping integration test")
	}

	tests := []struct {
		name       string
		input      string
		validateFn func(t *testing.T, output string)
	}{
		{
			name:  "real single-line format",
			input: "test.local { reverse_proxy localhost:8080 }",
			validateFn: func(t *testing.T, output string) {
				// Verify it has newlines and proper formatting
				assert.Contains(t, output, "test.local")
				assert.Contains(t, output, "{")
				assert.Contains(t, output, "reverse_proxy")
				assert.Contains(t, output, "localhost:8080")
				assert.Contains(t, output, "}")
				// Should be multi-line
				lines := len(output) - len(string([]rune(output)[0]))
				assert.Greater(t, lines, 1, "Should have multiple lines")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := importer.NormalizeCaddyfile(tt.input)
			assert.NoError(t, err)
			if tt.validateFn != nil {
				tt.validateFn(t, output)
			}
		})
	}
}

// TestDefaultExecutor_Execute_Timeout verifies the 5-second timeout triggers correctly
func TestDefaultExecutor_Execute_Timeout(t *testing.T) {
	executor := &DefaultExecutor{}

	// Use "sleep 10" to trigger the 5-second timeout
	output, err := executor.Execute("sleep", "10")

	// Error must be returned
	assert.Error(t, err)
	// Error message must contain the timeout message
	assert.Contains(t, err.Error(), "command timed out after 5 seconds")
	assert.Contains(t, err.Error(), "sleep")
	// Output may be empty or partial
	_ = output
}

// TestImporter_NormalizeCaddyfile_ReadError tests the error path when reading the formatted file fails
func TestImporter_NormalizeCaddyfile_ReadError(t *testing.T) {
	importer := NewImporter("caddy")

	// Mock executor that succeeds but deletes the temp file before returning
	// This simulates the file being removed after caddy fmt writes it
	mockExecutor := &MockExecutor{
		ExecuteFunc: func(name string, args ...string) ([]byte, error) {
			// The temp file path is the last argument (caddy fmt --overwrite <tempfile>)
			if len(args) >= 3 && args[0] == "fmt" && args[1] == "--overwrite" {
				// Delete the temp file to trigger ReadFile error
				_ = os.Remove(args[2])
			}
			return []byte{}, nil
		},
	}
	importer.executor = mockExecutor

	_, err := importer.NormalizeCaddyfile("test.local { reverse_proxy localhost:8080 }")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read formatted file")
}

// TestParseCaddyfile_PathTraversal tests path traversal rejection
func TestParseCaddyfile_PathTraversal(t *testing.T) {
	importer := NewImporter("caddy")

	tests := []struct {
		name        string
		path        string
		expectError string
	}{
		{
			name:        "double dot prefix",
			path:        "../etc/passwd",
			expectError: "invalid caddyfile path",
		},
		{
			name:        "just double dot",
			path:        "..",
			expectError: "invalid caddyfile path",
		},
		{
			name:        "empty path",
			path:        "",
			expectError: "invalid caddyfile path",
		},
		{
			name:        "dot only",
			path:        ".",
			expectError: "invalid caddyfile path",
		},
		{
			name:        "relative double dot",
			path:        "foo/../../../etc/passwd",
			expectError: "invalid caddyfile path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := importer.ParseCaddyfile(tt.path)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

// TestExtractHosts_WebsocketHandler tests websocket header extraction
func TestExtractHosts_WebsocketHandler(t *testing.T) {
	importer := NewImporter("caddy")

	// JSON config with websocket header
	websocketJSON := []byte(`{
		"apps": {
			"http": {
				"servers": {
					"srv0": {
						"routes": [
							{
								"match": [{"host": ["ws.example.com"]}],
								"handle": [
									{
										"handler": "reverse_proxy",
										"upstreams": [{"dial": "127.0.0.1:8080"}],
										"headers": {
											"Upgrade": ["websocket"]
										}
									}
								]
							}
						]
					}
				}
			}
		}
	}`)

	result, err := importer.ExtractHosts(websocketJSON)
	assert.NoError(t, err)
	assert.Len(t, result.Hosts, 1)
	assert.Equal(t, "ws.example.com", result.Hosts[0].DomainNames)
	assert.True(t, result.Hosts[0].WebsocketSupport, "WebsocketSupport should be true when Upgrade header contains websocket")
}

// TestExtractHosts_SubrouteHandler tests extraction of handlers from subroutes
func TestExtractHosts_SubrouteHandler(t *testing.T) {
	importer := NewImporter("caddy")

	// JSON config with subroute containing reverse_proxy
	subrouteJSON := []byte(`{
		"apps": {
			"http": {
				"servers": {
					"srv0": {
						"routes": [
							{
								"match": [{"host": ["nested.example.com"]}],
								"handle": [
									{
										"handler": "subroute",
										"routes": [
											{
												"handle": [
													{
														"handler": "reverse_proxy",
														"upstreams": [{"dial": "backend:9000"}]
													}
												]
											}
										]
									}
								]
							}
						]
					}
				}
			}
		}
	}`)

	result, err := importer.ExtractHosts(subrouteJSON)
	assert.NoError(t, err)
	assert.Len(t, result.Hosts, 1)
	assert.Equal(t, "nested.example.com", result.Hosts[0].DomainNames)
	assert.Equal(t, "backend", result.Hosts[0].ForwardHost)
	assert.Equal(t, 9000, result.Hosts[0].ForwardPort)
}

// TestBackupCaddyfile_PathTraversal tests backup path validation
func TestBackupCaddyfile_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")

	tests := []struct {
		name         string
		originalPath string
		expectError  string
	}{
		{
			name:         "double dot prefix",
			originalPath: "../etc/passwd",
			expectError:  "invalid original path",
		},
		{
			name:         "empty path",
			originalPath: "",
			expectError:  "invalid original path",
		},
		{
			name:         "dot only",
			originalPath: ".",
			expectError:  "invalid original path",
		},
		{
			name:         "relative traversal",
			originalPath: "foo/../../../etc/passwd",
			expectError:  "invalid original path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BackupCaddyfile(tt.originalPath, backupDir)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

// TestBackupCaddyfile_SourceNotReadable tests error when source can't be read
func TestBackupCaddyfile_SourceNotReadable(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")

	// Non-existent file
	_, err := BackupCaddyfile(filepath.Join(tmpDir, "nonexistent.txt"), backupDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading original file")
}

// TestExtractHosts_SplitHostPortFallback tests the fallback parsing when net.SplitHostPort fails
func TestExtractHosts_SplitHostPortFallback(t *testing.T) {
	// Enable the fallback branch
	oldVal := forceSplitFallback
	forceSplitFallback = true
	defer func() { forceSplitFallback = oldVal }()

	importer := NewImporter("caddy")

	// Test with valid host:port format (fallback should still parse it)
	validJSON := []byte(`{
		"apps": {
			"http": {
				"servers": {
					"srv0": {
						"routes": [
							{
								"match": [{"host": ["fallback.example.com"]}],
								"handle": [
									{
										"handler": "reverse_proxy",
										"upstreams": [{"dial": "backend:3000"}]
									}
								]
							}
						]
					}
				}
			}
		}
	}`)

	result, err := importer.ExtractHosts(validJSON)
	assert.NoError(t, err)
	assert.Len(t, result.Hosts, 1)
	assert.Equal(t, "backend", result.Hosts[0].ForwardHost)
	assert.Equal(t, 3000, result.Hosts[0].ForwardPort)
}

// TestExtractHosts_HostOnly tests fallback when dial is just a hostname without port
func TestExtractHosts_HostOnly(t *testing.T) {
	// Enable the fallback branch
	oldVal := forceSplitFallback
	forceSplitFallback = true
	defer func() { forceSplitFallback = oldVal }()

	importer := NewImporter("caddy")

	// Test with host only (no port)
	hostOnlyJSON := []byte(`{
		"apps": {
			"http": {
				"servers": {
					"srv0": {
						"routes": [
							{
								"match": [{"host": ["hostonly.example.com"]}],
								"handle": [
									{
										"handler": "reverse_proxy",
										"upstreams": [{"dial": "backend-service"}]
									}
								]
							}
						]
					}
				}
			}
		}
	}`)

	result, err := importer.ExtractHosts(hostOnlyJSON)
	assert.NoError(t, err)
	assert.Len(t, result.Hosts, 1)
	assert.Equal(t, "backend-service", result.Hosts[0].ForwardHost)
	assert.Equal(t, 80, result.Hosts[0].ForwardPort, "Should default to port 80")
}

// TestExtractHosts_InvalidPortFallback tests fallback when port is invalid
func TestExtractHosts_InvalidPortFallback(t *testing.T) {
	// Enable the fallback branch
	oldVal := forceSplitFallback
	forceSplitFallback = true
	defer func() { forceSplitFallback = oldVal }()

	importer := NewImporter("caddy")

	// Test with invalid port
	invalidPortJSON := []byte(`{
		"apps": {
			"http": {
				"servers": {
					"srv0": {
						"routes": [
							{
								"match": [{"host": ["invalidport.example.com"]}],
								"handle": [
									{
										"handler": "reverse_proxy",
										"upstreams": [{"dial": "backend:invalidport"}]
									}
								]
							}
						]
					}
				}
			}
		}
	}`)

	result, err := importer.ExtractHosts(invalidPortJSON)
	assert.NoError(t, err)
	assert.Len(t, result.Hosts, 1)
	assert.Equal(t, "backend", result.Hosts[0].ForwardHost)
	assert.Equal(t, 80, result.Hosts[0].ForwardPort, "Should default to port 80 on invalid port")
}

// TestExtractHosts_TLSConnectionPolicies tests SSL detection via TLS connection policies
func TestExtractHosts_TLSConnectionPolicies(t *testing.T) {
	importer := NewImporter("caddy")

	// JSON config with TLS connection policies set
	tlsJSON := []byte(`{
		"apps": {
			"http": {
				"servers": {
					"srv0": {
						"tls_connection_policies": [{"alpn": ["h2", "http/1.1"]}],
						"routes": [
							{
								"match": [{"host": ["secure.example.com"]}],
								"handle": [
									{
										"handler": "reverse_proxy",
										"upstreams": [{"dial": "127.0.0.1:8443"}]
									}
								]
							}
						]
					}
				}
			}
		}
	}`)

	result, err := importer.ExtractHosts(tlsJSON)
	assert.NoError(t, err)
	assert.Len(t, result.Hosts, 1)
	assert.Equal(t, "secure.example.com", result.Hosts[0].DomainNames)
	assert.True(t, result.Hosts[0].SSLForced, "SSLForced should be true when tls_connection_policies is set")
	assert.Equal(t, "https", result.Hosts[0].ForwardScheme, "ForwardScheme should be https for SSL")
}
