package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Save original env vars
	originalEnv := os.Getenv("CPM_ENV")
	defer func() { _ = os.Setenv("CPM_ENV", originalEnv) }()

	// Set test env vars
	_ = os.Setenv("CPM_ENV", "test")
	tempDir := t.TempDir()
	_ = os.Setenv("CPM_DB_PATH", filepath.Join(tempDir, "test.db"))
	_ = os.Setenv("CPM_CADDY_CONFIG_DIR", filepath.Join(tempDir, "caddy"))
	_ = os.Setenv("CPM_IMPORT_DIR", filepath.Join(tempDir, "imports"))

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "test", cfg.Environment)
	assert.Equal(t, filepath.Join(tempDir, "test.db"), cfg.DatabasePath)
	assert.DirExists(t, filepath.Dir(cfg.DatabasePath))
	assert.DirExists(t, cfg.CaddyConfigDir)
	assert.DirExists(t, cfg.ImportDir)
}

func TestLoad_Defaults(t *testing.T) {
	// Clear env vars to test defaults
	_ = os.Unsetenv("CPM_ENV")
	_ = os.Unsetenv("CPM_HTTP_PORT")
	// We need to set paths to a temp dir to avoid creating real dirs in test
	tempDir := t.TempDir()
	_ = os.Setenv("CPM_DB_PATH", filepath.Join(tempDir, "default.db"))
	_ = os.Setenv("CPM_CADDY_CONFIG_DIR", filepath.Join(tempDir, "caddy_default"))
	_ = os.Setenv("CPM_IMPORT_DIR", filepath.Join(tempDir, "imports_default"))

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, "8080", cfg.HTTPPort)
}

func TestLoad_CharonPrefersOverCPM(t *testing.T) {
	// Ensure CHARON_ variables take precedence over CPM_ fallback
	tempDir := t.TempDir()
	charonDB := filepath.Join(tempDir, "charon.db")
	cpmDB := filepath.Join(tempDir, "cpm.db")
	_ = os.Setenv("CHARON_DB_PATH", charonDB)
	_ = os.Setenv("CPM_DB_PATH", cpmDB)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, charonDB, cfg.DatabasePath)
}

func TestLoad_Error(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "file")
	f, err := os.Create(filePath) // #nosec G304 -- Test creates temp config file
	require.NoError(t, err)
	_ = f.Close()

	// Case 1: CaddyConfigDir is a file
	_ = os.Setenv("CPM_CADDY_CONFIG_DIR", filePath)
	// Set other paths to valid locations to isolate the error
	_ = os.Setenv("CPM_DB_PATH", filepath.Join(tempDir, "db", "test.db"))
	_ = os.Setenv("CPM_IMPORT_DIR", filepath.Join(tempDir, "imports"))

	_, err = Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ensure caddy config directory")

	// Case 2: ImportDir is a file
	_ = os.Setenv("CPM_CADDY_CONFIG_DIR", filepath.Join(tempDir, "caddy"))
	_ = os.Setenv("CPM_IMPORT_DIR", filePath)

	_, err = Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ensure import directory")
}

func TestGetEnvAny(t *testing.T) {
	// Test with no env vars set - should return fallback
	result := getEnvAny("fallback_value", "NONEXISTENT_KEY1", "NONEXISTENT_KEY2")
	assert.Equal(t, "fallback_value", result)

	// Test with first key set
	_ = os.Setenv("TEST_KEY1", "value1")
	defer func() { _ = os.Unsetenv("TEST_KEY1") }()
	result = getEnvAny("fallback", "TEST_KEY1", "TEST_KEY2")
	assert.Equal(t, "value1", result)

	// Test with second key set (first takes precedence)
	_ = os.Setenv("TEST_KEY2", "value2")
	defer func() { _ = os.Unsetenv("TEST_KEY2") }()
	result = getEnvAny("fallback", "TEST_KEY1", "TEST_KEY2")
	assert.Equal(t, "value1", result)

	// Test with only second key set
	_ = os.Unsetenv("TEST_KEY1")
	result = getEnvAny("fallback", "TEST_KEY1", "TEST_KEY2")
	assert.Equal(t, "value2", result)

	// Test with empty string value (should still be considered set)
	_ = os.Setenv("TEST_KEY3", "")
	defer func() { _ = os.Unsetenv("TEST_KEY3") }()
	result = getEnvAny("fallback", "TEST_KEY3")
	assert.Equal(t, "fallback", result) // Empty strings are treated as not set
}

func TestLoad_SecurityConfig(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.Setenv("CHARON_DB_PATH", filepath.Join(tempDir, "test.db"))
	_ = os.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(tempDir, "caddy"))
	_ = os.Setenv("CHARON_IMPORT_DIR", filepath.Join(tempDir, "imports"))

	// Test security settings
	_ = os.Setenv("CERBERUS_SECURITY_CROWDSEC_MODE", "live")
	_ = os.Setenv("CERBERUS_SECURITY_WAF_MODE", "enabled")
	_ = os.Setenv("CERBERUS_SECURITY_CERBERUS_ENABLED", "true")
	defer func() {
		_ = os.Unsetenv("CERBERUS_SECURITY_CROWDSEC_MODE")
		_ = os.Unsetenv("CERBERUS_SECURITY_WAF_MODE")
		_ = os.Unsetenv("CERBERUS_SECURITY_CERBERUS_ENABLED")
	}()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "live", cfg.Security.CrowdSecMode)
	assert.Equal(t, "enabled", cfg.Security.WAFMode)
	assert.True(t, cfg.Security.CerberusEnabled)
}

func TestLoad_DatabasePathError(t *testing.T) {
	tempDir := t.TempDir()

	// Create a file where the data directory should be created
	blockingFile := filepath.Join(tempDir, "blocking")
	f, err := os.Create(blockingFile) // #nosec G304 -- Test creates blocking file for error condition
	require.NoError(t, err)
	_ = f.Close()

	// Try to use a path that requires creating a dir inside the blocking file
	_ = os.Setenv("CHARON_DB_PATH", filepath.Join(blockingFile, "data", "test.db"))
	_ = os.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(tempDir, "caddy"))
	_ = os.Setenv("CHARON_IMPORT_DIR", filepath.Join(tempDir, "imports"))
	defer func() {
		_ = os.Unsetenv("CHARON_DB_PATH")
		_ = os.Unsetenv("CHARON_CADDY_CONFIG_DIR")
		_ = os.Unsetenv("CHARON_IMPORT_DIR")
	}()

	_, err = Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ensure data directory")
}

func TestLoad_ACMEStaging(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.Setenv("CHARON_DB_PATH", filepath.Join(tempDir, "test.db"))
	_ = os.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(tempDir, "caddy"))
	_ = os.Setenv("CHARON_IMPORT_DIR", filepath.Join(tempDir, "imports"))

	// Test ACME staging enabled
	_ = os.Setenv("CHARON_ACME_STAGING", "true")
	defer func() { _ = os.Unsetenv("CHARON_ACME_STAGING") }()

	cfg, err := Load()
	require.NoError(t, err)
	assert.True(t, cfg.ACMEStaging)

	// Test ACME staging disabled
	require.NoError(t, os.Setenv("CHARON_ACME_STAGING", "false"))
	cfg, err = Load()
	require.NoError(t, err)
	assert.False(t, cfg.ACMEStaging)
}

func TestLoad_DebugMode(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.Setenv("CHARON_DB_PATH", filepath.Join(tempDir, "test.db")))
	require.NoError(t, os.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(tempDir, "caddy")))
	require.NoError(t, os.Setenv("CHARON_IMPORT_DIR", filepath.Join(tempDir, "imports")))

	// Test debug mode enabled
	require.NoError(t, os.Setenv("CHARON_DEBUG", "true"))
	defer func() { require.NoError(t, os.Unsetenv("CHARON_DEBUG")) }()

	cfg, err := Load()
	require.NoError(t, err)
	assert.True(t, cfg.Debug)

	// Test debug mode disabled
	require.NoError(t, os.Setenv("CHARON_DEBUG", "false"))
	cfg, err = Load()
	require.NoError(t, err)
	assert.False(t, cfg.Debug)
}

func TestLoad_EmergencyConfig(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.Setenv("CHARON_DB_PATH", filepath.Join(tempDir, "test.db")))
	require.NoError(t, os.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(tempDir, "caddy")))
	require.NoError(t, os.Setenv("CHARON_IMPORT_DIR", filepath.Join(tempDir, "imports")))

	// Test emergency config defaults
	cfg, err := Load()
	require.NoError(t, err)
	assert.False(t, cfg.Emergency.Enabled, "Emergency server should be disabled by default")
	assert.Equal(t, "127.0.0.1:2020", cfg.Emergency.BindAddress, "Default emergency bind should be port 2020 (avoids Caddy admin API on 2019)")
	assert.Equal(t, "", cfg.Emergency.BasicAuthUsername, "Basic auth username should be empty by default")
	assert.Equal(t, "", cfg.Emergency.BasicAuthPassword, "Basic auth password should be empty by default")

	// Test emergency config with custom values
	_ = os.Setenv("CHARON_EMERGENCY_SERVER_ENABLED", "true")
	_ = os.Setenv("CHARON_EMERGENCY_BIND", "0.0.0.0:2020")
	_ = os.Setenv("CHARON_EMERGENCY_USERNAME", "admin")
	_ = os.Setenv("CHARON_EMERGENCY_PASSWORD", "testpass")
	defer func() {
		_ = os.Unsetenv("CHARON_EMERGENCY_SERVER_ENABLED")
		_ = os.Unsetenv("CHARON_EMERGENCY_BIND")
		_ = os.Unsetenv("CHARON_EMERGENCY_USERNAME")
		_ = os.Unsetenv("CHARON_EMERGENCY_PASSWORD")
	}()

	cfg, err = Load()
	require.NoError(t, err)
	assert.True(t, cfg.Emergency.Enabled)
	assert.Equal(t, "0.0.0.0:2020", cfg.Emergency.BindAddress)
	assert.Equal(t, "admin", cfg.Emergency.BasicAuthUsername)
	assert.Equal(t, "testpass", cfg.Emergency.BasicAuthPassword)
}

// ============================================
// splitAndTrim Tests
// ============================================

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		sep      string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			sep:      ",",
			expected: nil,
		},
		{
			name:     "comma-separated values",
			input:    "a,b,c",
			sep:      ",",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "with whitespace",
			input:    " a , b , c ",
			sep:      ",",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "single value",
			input:    "test",
			sep:      ",",
			expected: []string{"test"},
		},
		{
			name:     "single value with whitespace",
			input:    "  test  ",
			sep:      ",",
			expected: []string{"test"},
		},
		{
			name:     "empty parts filtered",
			input:    "a,,b,  ,c",
			sep:      ",",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "semicolon separator",
			input:    "10.0.0.0/8;172.16.0.0/12;192.168.0.0/16",
			sep:      ";",
			expected: []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
		},
		{
			name:     "mixed whitespace and empty",
			input:    " , , a , , b , , ",
			sep:      ",",
			expected: []string{"a", "b"},
		},
		{
			name:     "tabs and newlines",
			input:    "a\t,\tb\n,\nc",
			sep:      ",",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "CIDR list example",
			input:    "10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 127.0.0.0/8",
			sep:      ",",
			expected: []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitAndTrim(tt.input, tt.sep)
			assert.Equal(t, tt.expected, result)
		})
	}
}
