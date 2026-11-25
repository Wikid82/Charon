package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePluginSignatures(t *testing.T) {
	t.Run("empty value is permissive", func(t *testing.T) {
		signatures, err := parsePluginSignatures("")
		require.NoError(t, err)
		assert.Nil(t, signatures)
	})

	t.Run("empty object is strict and empty", func(t *testing.T) {
		signatures, err := parsePluginSignatures(`{}`)
		require.NoError(t, err)
		require.NotNil(t, signatures)
		assert.Empty(t, signatures)
	})

	t.Run("JSON null is strict and empty, never permissive", func(t *testing.T) {
		signatures, err := parsePluginSignatures(`null`)
		require.NoError(t, err)
		require.NotNil(t, signatures)
		assert.Empty(t, signatures)
	})

	t.Run("valid allowlist is parsed", func(t *testing.T) {
		signatures, err := parsePluginSignatures(`{"plugin-a.so":"sha256:abc123","plugin-b.so":"sha256:def456"}`)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"plugin-a.so": "sha256:abc123",
			"plugin-b.so": "sha256:def456",
		}, signatures)
	})

	t.Run("invalid JSON is an error", func(t *testing.T) {
		signatures, err := parsePluginSignatures(`{invalid}`)
		require.Error(t, err)
		assert.Nil(t, signatures)
	})

	t.Run("value without sha256 prefix is an error", func(t *testing.T) {
		signatures, err := parsePluginSignatures(`{"plugin.so":"md5:deadbeef"}`)
		require.Error(t, err)
		assert.Nil(t, signatures)
	})
}

func TestLoadPluginSignatures(t *testing.T) {
	t.Run("unset env is permissive", func(t *testing.T) {
		t.Setenv("CHARON_PLUGIN_SIGNATURES", "")
		assert.Nil(t, loadPluginSignatures())
	})

	t.Run("valid env returns the map", func(t *testing.T) {
		t.Setenv("CHARON_PLUGIN_SIGNATURES", `{"plugin-a.so":"sha256:abc123"}`)
		assert.Equal(t, map[string]string{"plugin-a.so": "sha256:abc123"}, loadPluginSignatures())
	})

	t.Run("malformed env fails closed with an empty list", func(t *testing.T) {
		t.Setenv("CHARON_PLUGIN_SIGNATURES", `not json`)
		signatures := loadPluginSignatures()
		require.NotNil(t, signatures)
		assert.Empty(t, signatures)
	})
}

func TestLoadConfigSetsPluginSignatures(t *testing.T) {
	t.Setenv("CHARON_DB_PATH", t.TempDir()+"/charon.db")
	t.Setenv("CHARON_PLUGIN_SIGNATURES", `{bad`)

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg.PluginSignatures)
	assert.Empty(t, cfg.PluginSignatures)
}

func TestLoadPluginCacheDir(t *testing.T) {
	t.Run("defaults to plugin-cache inside the data folder", func(t *testing.T) {
		dataDir := t.TempDir()
		t.Setenv("CHARON_DB_PATH", filepath.Join(dataDir, "charon.db"))
		t.Setenv("CHARON_PLUGIN_CACHE_DIR", "")

		cfg, err := Load()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dataDir, "plugin-cache"), cfg.PluginCacheDir)
	})

	t.Run("CHARON_PLUGIN_CACHE_DIR overrides the default", func(t *testing.T) {
		t.Setenv("CHARON_DB_PATH", filepath.Join(t.TempDir(), "charon.db"))
		override := filepath.Join(t.TempDir(), "exec-ok")
		t.Setenv("CHARON_PLUGIN_CACHE_DIR", override)

		cfg, err := Load()
		require.NoError(t, err)
		assert.Equal(t, override, cfg.PluginCacheDir)
	})
}
