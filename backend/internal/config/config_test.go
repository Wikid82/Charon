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
