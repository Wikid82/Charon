package crowdsec

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func writeFakeCSCLI(t *testing.T, script string) (binDir string) {
	t.Helper()

	binDir = t.TempDir()
	path := filepath.Join(binDir, "cscli")
	requireMode := fs.FileMode(0o755)

	err := os.WriteFile(path, []byte(script), requireMode)
	if err != nil {
		t.Fatalf("failed to write fake cscli: %v", err)
	}
	return binDir
}

func withEnv(t *testing.T, key, value string, fn func()) {
	t.Helper()
	old, had := os.LookupEnv(key)
	if value == "" {
		_ = os.Unsetenv(key)
	} else {
		_ = os.Setenv(key, value)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
	fn()
}

func withPath(t *testing.T, newPath string, fn func()) {
	t.Helper()
	old, had := os.LookupEnv("PATH")
	_ = os.Setenv("PATH", newPath)
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("PATH", old)
		} else {
			_ = os.Unsetenv("PATH")
		}
	})
	fn()
}

func TestGetBouncerAPIKey_FromEnv(t *testing.T) {
	// Save and restore original env
	original := os.Getenv("CROWDSEC_API_KEY")
	defer func() {
		if original != "" {
			_ = os.Setenv("CROWDSEC_API_KEY", original)
		} else {
			_ = os.Unsetenv("CROWDSEC_API_KEY")
		}
	}()

	// Set test value
	_ = os.Setenv("CROWDSEC_API_KEY", "test-api-key-123")

	key := getBouncerAPIKey()
	assert.Equal(t, "test-api-key-123", key)
}

func TestGetBouncerAPIKey_Empty(t *testing.T) {
	// Save and restore original env vars
	envVars := []string{
		"CROWDSEC_API_KEY",
		"CROWDSEC_BOUNCER_API_KEY",
		"CERBERUS_SECURITY_CROWDSEC_API_KEY",
		"CHARON_SECURITY_CROWDSEC_API_KEY",
		"CPM_SECURITY_CROWDSEC_API_KEY",
	}

	originals := make(map[string]string)
	for _, key := range envVars {
		originals[key] = os.Getenv(key)
		_ = os.Unsetenv(key)
	}
	defer func() {
		for key, val := range originals {
			if val != "" {
				_ = os.Setenv(key, val)
			}
		}
	}()

	key := getBouncerAPIKey()
	assert.Empty(t, key)
}

func TestGetBouncerAPIKey_Fallback(t *testing.T) {
	// Test fallback to secondary env var
	envVars := []string{
		"CROWDSEC_API_KEY",
		"CROWDSEC_BOUNCER_API_KEY",
		"CERBERUS_SECURITY_CROWDSEC_API_KEY",
		"CHARON_SECURITY_CROWDSEC_API_KEY",
		"CPM_SECURITY_CROWDSEC_API_KEY",
	}

	originals := make(map[string]string)
	for _, key := range envVars {
		originals[key] = os.Getenv(key)
		_ = os.Unsetenv(key)
	}
	defer func() {
		for key, val := range originals {
			if val != "" {
				_ = os.Setenv(key, val)
			}
		}
	}()

	// Set only the fallback env var
	_ = os.Setenv("CERBERUS_SECURITY_CROWDSEC_API_KEY", "fallback-key-456")

	key := getBouncerAPIKey()
	assert.Equal(t, "fallback-key-456", key)
}

func TestEnsureBouncerRegistered_UsesEnvKey(t *testing.T) {
	withEnv(t, "CROWDSEC_API_KEY", "env-key", func() {
		key, err := EnsureBouncerRegistered(context.Background(), "")
		assert.NoError(t, err)
		assert.Equal(t, "env-key", key)
	})
}

func TestEnsureBouncerRegistered_NoEnvNoCSCLI(t *testing.T) {
	// Ensure all key env vars are empty
	for _, k := range []string{"CROWDSEC_API_KEY", "CROWDSEC_BOUNCER_API_KEY", "CERBERUS_SECURITY_CROWDSEC_API_KEY", "CHARON_SECURITY_CROWDSEC_API_KEY", "CPM_SECURITY_CROWDSEC_API_KEY"} {
		withEnv(t, k, "", func() {})
	}

	withPath(t, "", func() {
		_, err := EnsureBouncerRegistered(context.Background(), "")
		assert.Error(t, err)
	})
}

func TestEnsureBouncerRegistered_ReturnsExistingBouncerKey(t *testing.T) {
	for _, k := range []string{"CROWDSEC_API_KEY", "CROWDSEC_BOUNCER_API_KEY", "CERBERUS_SECURITY_CROWDSEC_API_KEY", "CHARON_SECURITY_CROWDSEC_API_KEY", "CPM_SECURITY_CROWDSEC_API_KEY"} {
		withEnv(t, k, "", func() {})
	}

	origPath := os.Getenv("PATH")

	binDir := writeFakeCSCLI(t, `#!/usr/bin/env bash
set -euo pipefail
if [[ "$1" == "bouncers" && "$2" == "list" ]]; then
  echo '[{"name":"caddy-bouncer","api_key":"existing-key","ip_address":"","valid":true,"created_at":"2025-01-01T00:00:00Z"}]'
  exit 0
fi
echo "unexpected args" >&2
exit 2
`)

	withPath(t, binDir+":"+origPath, func() {
		key, err := EnsureBouncerRegistered(context.Background(), "")
		assert.NoError(t, err)
		assert.Equal(t, "existing-key", key)
	})
}

func TestEnsureBouncerRegistered_RegistersNewWhenNoneExists(t *testing.T) {
	for _, k := range []string{"CROWDSEC_API_KEY", "CROWDSEC_BOUNCER_API_KEY", "CERBERUS_SECURITY_CROWDSEC_API_KEY", "CHARON_SECURITY_CROWDSEC_API_KEY", "CPM_SECURITY_CROWDSEC_API_KEY"} {
		withEnv(t, k, "", func() {})
	}

	origPath := os.Getenv("PATH")

	binDir := writeFakeCSCLI(t, `#!/usr/bin/env bash
set -euo pipefail
if [[ "$1" == "bouncers" && "$2" == "list" ]]; then
  echo '[]'
  exit 0
fi
if [[ "$1" == "bouncers" && "$2" == "add" ]]; then
  echo 'new-key'
  exit 0
fi
echo "unexpected args" >&2
exit 2
`)

	withPath(t, binDir+":"+origPath, func() {
		key, err := EnsureBouncerRegistered(context.Background(), "")
		assert.NoError(t, err)
		assert.Equal(t, "new-key", key)
	})
}

func TestValidateLAPIURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid localhost with port",
			url:     "http://localhost:8085",
			wantErr: false,
		},
		{
			name:    "valid 127.0.0.1",
			url:     "http://127.0.0.1:8085",
			wantErr: false,
		},
		{
			name:        "external URL blocked",
			url:         "http://evil.com",
			wantErr:     true,
			errContains: "must be localhost",
		},
		{
			name:    "HTTPS localhost",
			url:     "https://localhost:8085",
			wantErr: false,
		},
		{
			name:        "invalid scheme",
			url:         "ftp://localhost:8085",
			wantErr:     true,
			errContains: "scheme",
		},
		{
			name:        "no scheme",
			url:         "localhost:8085",
			wantErr:     true,
			errContains: "scheme",
		},
		{
			name:    "empty URL allowed (defaults to localhost)",
			url:     "",
			wantErr: false,
		},
		{
			name:    "IPv6 localhost",
			url:     "http://[::1]:8085",
			wantErr: false,
		},
		{
			name:        "private IP 192.168.x.x blocked (security)",
			url:         "http://192.168.1.100:8085",
			wantErr:     true,
			errContains: "must be localhost",
		},
		{
			name:        "private IP 10.x.x.x blocked (security)",
			url:         "http://10.0.0.50:8085",
			wantErr:     true,
			errContains: "must be localhost",
		},
		{
			name:        "missing hostname",
			url:         "http://:8085",
			wantErr:     true,
			errContains: "missing hostname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLAPIURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEnsureBouncerRegistered_InvalidURL(t *testing.T) {
	// Test that SSRF validation is applied
	tests := []struct {
		name        string
		url         string
		errContains string
	}{
		{
			name:        "external URL rejected",
			url:         "http://attacker.com:8085",
			errContains: "must be localhost",
		},
		{
			name:        "invalid scheme rejected",
			url:         "ftp://localhost:8085",
			errContains: "scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := EnsureBouncerRegistered(context.Background(), tt.url)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}
