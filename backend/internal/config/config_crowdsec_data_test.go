package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadCrowdSecDirEnv sets the database and caddy paths Load needs and clears the CrowdSec data-dir variables.
func loadCrowdSecDirEnv(t *testing.T) {
	t.Helper()
	tempDir := t.TempDir()
	t.Setenv("CHARON_DB_PATH", filepath.Join(tempDir, "test.db"))
	t.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(tempDir, "caddy"))
	t.Setenv("CHARON_IMPORT_DIR", filepath.Join(tempDir, "imports"))
	t.Setenv("CHARON_CROWDSEC_CONFIG_DIR", "")
	t.Setenv("CPM_CROWDSEC_CONFIG_DIR", "")
	t.Setenv("CHARON_CROWDSEC_DATA", "")
}

// crowdSecDirWarnings returns the startup warnings that name the CrowdSec data-dir variables.
func crowdSecDirWarnings(warnings []string) []string {
	var out []string
	for _, w := range warnings {
		if strings.Contains(w, "CHARON_CROWDSEC_DATA") {
			out = append(out, w)
		}
	}
	return out
}

// TestLoad_CrowdSecConfigDirDefault keeps the relative default when neither variable is set.
func TestLoad_CrowdSecConfigDirDefault(t *testing.T) {
	loadCrowdSecDirEnv(t)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("data", "crowdsec"), cfg.Security.CrowdSecConfigDir)
	assert.Empty(t, crowdSecDirWarnings(cfg.StartupWarnings))
}

// TestLoad_CrowdSecDataAliasSetsConfigDir reads CHARON_CROWDSEC_DATA when the config-dir variable is unset.
func TestLoad_CrowdSecDataAliasSetsConfigDir(t *testing.T) {
	loadCrowdSecDirEnv(t)
	alias := filepath.Join("data", "alias-crowdsec")
	t.Setenv("CHARON_CROWDSEC_DATA", alias)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, alias, cfg.Security.CrowdSecConfigDir)
	assert.Empty(t, crowdSecDirWarnings(cfg.StartupWarnings), "an alias alone is not a conflict")
}

// TestLoad_CrowdSecConfigDirWinsOverDataAlias keeps the config-dir value when both variables differ and warns once.
func TestLoad_CrowdSecConfigDirWinsOverDataAlias(t *testing.T) {
	loadCrowdSecDirEnv(t)
	configDir := filepath.Join("data", "crowdsec")
	t.Setenv("CHARON_CROWDSEC_CONFIG_DIR", configDir)
	t.Setenv("CHARON_CROWDSEC_DATA", filepath.Join("data", "other"))

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, configDir, cfg.Security.CrowdSecConfigDir)

	warnings := crowdSecDirWarnings(cfg.StartupWarnings)
	require.Len(t, warnings, 1, "warnings: %v", cfg.StartupWarnings)
	assert.Contains(t, warnings[0], "CHARON_CROWDSEC_CONFIG_DIR")
}

// TestLoad_CrowdSecDataAliasEqualToConfigDirIsSilent accepts the alias when it names the same directory.
func TestLoad_CrowdSecDataAliasEqualToConfigDirIsSilent(t *testing.T) {
	loadCrowdSecDirEnv(t)
	same := filepath.Join("data", "crowdsec")
	t.Setenv("CHARON_CROWDSEC_CONFIG_DIR", same)
	t.Setenv("CHARON_CROWDSEC_DATA", same)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, same, cfg.Security.CrowdSecConfigDir)
	assert.Empty(t, crowdSecDirWarnings(cfg.StartupWarnings))
}
