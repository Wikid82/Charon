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
	defer os.Setenv("CPM_ENV", originalEnv)

	// Set test env vars
	os.Setenv("CPM_ENV", "test")
	tempDir := t.TempDir()
	os.Setenv("CPM_DB_PATH", filepath.Join(tempDir, "test.db"))
	os.Setenv("CPM_CADDY_CONFIG_DIR", filepath.Join(tempDir, "caddy"))
	os.Setenv("CPM_IMPORT_DIR", filepath.Join(tempDir, "imports"))

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
	os.Unsetenv("CPM_ENV")
	os.Unsetenv("CPM_HTTP_PORT")
	// We need to set paths to a temp dir to avoid creating real dirs in test
	tempDir := t.TempDir()
	os.Setenv("CPM_DB_PATH", filepath.Join(tempDir, "default.db"))
	os.Setenv("CPM_CADDY_CONFIG_DIR", filepath.Join(tempDir, "caddy_default"))
	os.Setenv("CPM_IMPORT_DIR", filepath.Join(tempDir, "imports_default"))

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
	os.Setenv("CHARON_DB_PATH", charonDB)
	os.Setenv("CPM_DB_PATH", cpmDB)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, charonDB, cfg.DatabasePath)
}

func TestLoad_Error(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "file")
	f, err := os.Create(filePath)
	require.NoError(t, err)
	f.Close()

	// Case 1: CaddyConfigDir is a file
	os.Setenv("CPM_CADDY_CONFIG_DIR", filePath)
	// Set other paths to valid locations to isolate the error
	os.Setenv("CPM_DB_PATH", filepath.Join(tempDir, "db", "test.db"))
	os.Setenv("CPM_IMPORT_DIR", filepath.Join(tempDir, "imports"))

	_, err = Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ensure caddy config directory")

	// Case 2: ImportDir is a file
	os.Setenv("CPM_CADDY_CONFIG_DIR", filepath.Join(tempDir, "caddy"))
	os.Setenv("CPM_IMPORT_DIR", filePath)

	_, err = Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ensure import directory")
}

func TestGetEnvAny(t *testing.T) {
	// Test with no env vars set - should return fallback
	result := getEnvAny("fallback_value", "NONEXISTENT_KEY1", "NONEXISTENT_KEY2")
	assert.Equal(t, "fallback_value", result)

	// Test with first key set
	os.Setenv("TEST_KEY1", "value1")
	defer os.Unsetenv("TEST_KEY1")
	result = getEnvAny("fallback", "TEST_KEY1", "TEST_KEY2")
	assert.Equal(t, "value1", result)

	// Test with second key set (first takes precedence)
	os.Setenv("TEST_KEY2", "value2")
	defer os.Unsetenv("TEST_KEY2")
	result = getEnvAny("fallback", "TEST_KEY1", "TEST_KEY2")
	assert.Equal(t, "value1", result)

	// Test with only second key set
	os.Unsetenv("TEST_KEY1")
	result = getEnvAny("fallback", "TEST_KEY1", "TEST_KEY2")
	assert.Equal(t, "value2", result)

	// Test with empty string value (should still be considered set)
	os.Setenv("TEST_KEY3", "")
	defer os.Unsetenv("TEST_KEY3")
	result = getEnvAny("fallback", "TEST_KEY3")
	assert.Equal(t, "fallback", result) // Empty strings are treated as not set
}

func TestLoad_SecurityConfig(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("CHARON_DB_PATH", filepath.Join(tempDir, "test.db"))
	os.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(tempDir, "caddy"))
	os.Setenv("CHARON_IMPORT_DIR", filepath.Join(tempDir, "imports"))

	// Test security settings
	os.Setenv("CERBERUS_SECURITY_CROWDSEC_MODE", "live")
	os.Setenv("CERBERUS_SECURITY_WAF_MODE", "enabled")
	os.Setenv("CERBERUS_SECURITY_CERBERUS_ENABLED", "true")
	defer func() {
		os.Unsetenv("CERBERUS_SECURITY_CROWDSEC_MODE")
		os.Unsetenv("CERBERUS_SECURITY_WAF_MODE")
		os.Unsetenv("CERBERUS_SECURITY_CERBERUS_ENABLED")
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
	f, err := os.Create(blockingFile)
	require.NoError(t, err)
	f.Close()

	// Try to use a path that requires creating a dir inside the blocking file
	os.Setenv("CHARON_DB_PATH", filepath.Join(blockingFile, "data", "test.db"))
	os.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(tempDir, "caddy"))
	os.Setenv("CHARON_IMPORT_DIR", filepath.Join(tempDir, "imports"))
	defer func() {
		os.Unsetenv("CHARON_DB_PATH")
		os.Unsetenv("CHARON_CADDY_CONFIG_DIR")
		os.Unsetenv("CHARON_IMPORT_DIR")
	}()

	_, err = Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ensure data directory")
}

func TestLoad_ACMEStaging(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("CHARON_DB_PATH", filepath.Join(tempDir, "test.db"))
	os.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(tempDir, "caddy"))
	os.Setenv("CHARON_IMPORT_DIR", filepath.Join(tempDir, "imports"))

	// Test ACME staging enabled
	os.Setenv("CHARON_ACME_STAGING", "true")
	defer os.Unsetenv("CHARON_ACME_STAGING")

	cfg, err := Load()
	require.NoError(t, err)
	assert.True(t, cfg.ACMEStaging)

	// Test ACME staging disabled
	os.Setenv("CHARON_ACME_STAGING", "false")
	cfg, err = Load()
	require.NoError(t, err)
	assert.False(t, cfg.ACMEStaging)
}

func TestLoad_DebugMode(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("CHARON_DB_PATH", filepath.Join(tempDir, "test.db"))
	os.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(tempDir, "caddy"))
	os.Setenv("CHARON_IMPORT_DIR", filepath.Join(tempDir, "imports"))

	// Test debug mode enabled
	os.Setenv("CHARON_DEBUG", "true")
	defer os.Unsetenv("CHARON_DEBUG")

	cfg, err := Load()
	require.NoError(t, err)
	assert.True(t, cfg.Debug)

	// Test debug mode disabled
	os.Setenv("CHARON_DEBUG", "false")
	cfg, err = Load()
	require.NoError(t, err)
	assert.False(t, cfg.Debug)
}
