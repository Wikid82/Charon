package caddy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
	"github.com/Wikid82/charon/backend/internal/models"
)

const bouncerKeyPathTestKey = "file-key-bouncerkeypath-000000001"

// TestGenerateConfigWithBouncerKeyPathReadsThatFile checks the proxy config carries the key from the file named by
// WithBouncerKeyPath, not from the image default.
func TestGenerateConfigWithBouncerKeyPathReadsThatFile(t *testing.T) {
	clearCrowdSecKeyEnv(t)
	keyPath := filepath.Join(t.TempDir(), "bouncer_key")
	require.NoError(t, os.WriteFile(keyPath, []byte(bouncerKeyPathTestKey+"\n"), 0o600))

	cfg, err := GenerateConfig(noKeyHosts(), "/tmp/caddy-data", "", "", "", false, true, false, false, false, "", nil, nil, nil,
		&models.SecurityConfig{CrowdSecMode: "local"}, nil, WithBouncerKeyPath(keyPath))
	require.NoError(t, err)

	require.NotNil(t, cfg.Apps.CrowdSec, "a key in the configured file must attach the crowdsec app")
	require.Equal(t, bouncerKeyPathTestKey, cfg.Apps.CrowdSec.APIKey)
}

// TestGenerateConfigWithoutBouncerKeyPathIsUnchanged checks an absent option keeps the image default path.
func TestGenerateConfigWithoutBouncerKeyPathIsUnchanged(t *testing.T) {
	clearCrowdSecKeyEnv(t)
	t.Setenv("CROWDSEC_API_KEY", "env-only-key")

	resolved := resolveGenerateConfigOptions(nil)
	require.Equal(t, bouncerkey.DefaultFile, resolved.bouncerKeyPath)

	cfg, err := GenerateConfig(noKeyHosts(), "/tmp/caddy-data", "", "", "", false, true, false, false, false, "", nil, nil, nil,
		&models.SecurityConfig{CrowdSecMode: "local"}, nil)
	require.NoError(t, err)
	require.NotNil(t, cfg.Apps.CrowdSec)
	require.Equal(t, "env-only-key", cfg.Apps.CrowdSec.APIKey)
}

// TestGetCrowdSecAPIKeyHonorsGivenPath checks the key is read from the path it is given, file first.
func TestGetCrowdSecAPIKeyHonorsGivenPath(t *testing.T) {
	clearCrowdSecKeyEnv(t)
	t.Setenv("CROWDSEC_API_KEY", "env-key")
	keyPath := filepath.Join(t.TempDir(), "bouncer_key")
	require.Equal(t, "env-key", getCrowdSecAPIKey(keyPath))

	require.NoError(t, os.WriteFile(keyPath, []byte("path-file-key\n"), 0o600))
	require.Equal(t, "path-file-key", getCrowdSecAPIKey(keyPath))
}

// TestManagerPassesKeyPathFromCrowdSecConfigDir checks Manager hands GenerateConfig the key file under its data
// directory, the same file the handlers and boot use, and the override when one is set.
func TestManagerPassesKeyPathFromCrowdSecConfigDir(t *testing.T) {
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(caddyServer.Close)

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{},
		&models.SSLCertificate{}, &models.SecurityConfig{}, &models.SecurityRuleSet{},
		&models.SecurityDecision{}, &models.DNSProvider{},
	))

	origGen := generateConfigFunc
	t.Cleanup(func() { generateConfigFunc = origGen })
	var captured string
	generateConfigFunc = func(_ []models.ProxyHost, _, _, _, _ string, _, _, _, _, _ bool, _ string,
		_ []models.SecurityRuleSet, _ map[string]string, _ []models.SecurityDecision, _ *models.SecurityConfig,
		_ []DNSProviderConfig, opts ...GenerateConfigOption) (*Config, error) {
		captured = resolveGenerateConfigOptions(opts).bouncerKeyPath
		return &Config{}, nil
	}
	origVal := validateConfigFunc
	t.Cleanup(func() { validateConfigFunc = origVal })
	validateConfigFunc = func(_ *Config) error { return nil }

	dataDir := filepath.Join(t.TempDir(), "alt-crowdsec")
	t.Setenv(bouncerkey.PathEnvName, "")
	manager := NewManager(newTestClient(t, caddyServer.URL), db, t.TempDir(), "", false,
		config.SecurityConfig{CerberusEnabled: true, CrowdSecConfigDir: dataDir})
	require.NoError(t, manager.ApplyConfig(context.Background()))
	require.Equal(t, filepath.Join(dataDir, "bouncer_key"), captured)

	overridePath := filepath.Join(t.TempDir(), "override_key")
	t.Setenv(bouncerkey.PathEnvName, overridePath)
	require.NoError(t, manager.ApplyConfig(context.Background()))
	require.Equal(t, overridePath, captured, "the override wins over the data directory")
}
