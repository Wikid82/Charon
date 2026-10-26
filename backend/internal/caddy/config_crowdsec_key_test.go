package caddy

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
)

// crowdSecNoKeyLogLine is the text the no-key warning must contain exactly once per generated config.
const crowdSecNoKeyLogLine = "CrowdSec is on but no bouncer key exists yet"

// noKeyHosts returns two enabled hosts so a per-host warning would show up twice.
func noKeyHosts() []models.ProxyHost {
	return []models.ProxyHost{
		{UUID: "nk-1", DomainNames: "one.example.com", ForwardHost: "app", ForwardPort: 8080, Enabled: true},
		{UUID: "nk-2", DomainNames: "two.example.com", ForwardHost: "app", ForwardPort: 8081, Enabled: true},
	}
}

// captureCaddyLogs redirects the global logger into a buffer for the test and restores it afterwards.
func captureCaddyLogs(t *testing.T) *strings.Builder {
	t.Helper()
	var buf strings.Builder
	logger.Init(true, &buf)
	t.Cleanup(func() { logger.Init(false, io.Discard) })
	return &buf
}

// routeHasCrowdSecHandler reports whether any route in the generated server carries a crowdsec handler.
func routeHasCrowdSecHandler(t *testing.T, routes []*Route) bool {
	t.Helper()
	for _, r := range routes {
		for _, h := range r.Handle {
			if h["handler"] == "crowdsec" {
				return true
			}
		}
	}
	return false
}

func TestGenerateConfig_CrowdSecOnWithoutKey_LeavesOutAppAndHandlers(t *testing.T) {
	useBouncerKeyFile(t, filepath.Join(t.TempDir(), "missing_bouncer_key"))
	clearCrowdSecKeyEnv(t)
	logs := captureCaddyLogs(t)

	cfg, err := GenerateConfig(noKeyHosts(), "/tmp/caddy-data", "", "", "", false, true, false, false, false, "", nil, nil, nil, &models.SecurityConfig{CrowdSecMode: "local"}, nil)
	require.NoError(t, err)

	require.Nil(t, cfg.Apps.CrowdSec, "apps.crowdsec must be left out while no bouncer key exists")
	server := cfg.Apps.HTTP.Servers["charon_server"]
	require.NotNil(t, server)
	require.False(t, routeHasCrowdSecHandler(t, server.Routes), "no route may load the crowdsec app without a key")

	require.Equal(t, 1, strings.Count(logs.String(), crowdSecNoKeyLogLine), "exactly one warning per generated config, not per host")
}

func TestGenerateConfig_CrowdSecOnWithoutKey_KeepsAccessLogPath(t *testing.T) {
	useBouncerKeyFile(t, filepath.Join(t.TempDir(), "missing_bouncer_key"))
	clearCrowdSecKeyEnv(t)
	captureCaddyLogs(t)

	cfg, err := GenerateConfig(noKeyHosts(), "/tmp/caddy-data", "", "", "", false, true, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	// The engine still reads the access log while CrowdSec is on, so the log path stays on the CrowdSec location.
	require.Equal(t, "/var/log/caddy/access.log", cfg.Logging.Logs["access"].Writer.Filename)
}

func TestGenerateConfig_CrowdSecOff_NoWarningNoAppNoHandler(t *testing.T) {
	useBouncerKeyFile(t, filepath.Join(t.TempDir(), "missing_bouncer_key"))
	clearCrowdSecKeyEnv(t)
	logs := captureCaddyLogs(t)

	cfg, err := GenerateConfig(noKeyHosts(), "/tmp/caddy-data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	require.Nil(t, cfg.Apps.CrowdSec)
	require.False(t, routeHasCrowdSecHandler(t, cfg.Apps.HTTP.Servers["charon_server"].Routes))
	require.NotContains(t, logs.String(), crowdSecNoKeyLogLine)
}

func TestGenerateConfig_CrowdSecOnWithEnvKey_IncludesAppAndHandlers(t *testing.T) {
	useBouncerKeyFile(t, filepath.Join(t.TempDir(), "missing_bouncer_key"))
	clearCrowdSecKeyEnv(t)
	t.Setenv("CROWDSEC_API_KEY", "env-key-for-test")
	logs := captureCaddyLogs(t)

	cfg, err := GenerateConfig(noKeyHosts(), "/tmp/caddy-data", "", "", "", false, true, false, false, false, "", nil, nil, nil, &models.SecurityConfig{CrowdSecMode: "local"}, nil)
	require.NoError(t, err)

	require.NotNil(t, cfg.Apps.CrowdSec)
	require.Equal(t, "env-key-for-test", cfg.Apps.CrowdSec.APIKey)
	require.True(t, routeHasCrowdSecHandler(t, cfg.Apps.HTTP.Servers["charon_server"].Routes))
	require.NotContains(t, logs.String(), crowdSecNoKeyLogLine)
}

func TestGenerateConfig_CrowdSecOnWithSavedKey_IncludesAppAndHandlers(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "bouncer_key")
	require.NoError(t, os.WriteFile(keyPath, []byte("saved-key-for-test\n"), 0o600))
	useBouncerKeyFile(t, keyPath)
	clearCrowdSecKeyEnv(t)

	cfg, err := GenerateConfig(noKeyHosts(), "/tmp/caddy-data", "", "", "", false, true, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	// The saved key file is the source GenerateConfig reads, so it counts as a key.
	require.NotNil(t, cfg.Apps.CrowdSec)
	require.Equal(t, "saved-key-for-test", cfg.Apps.CrowdSec.APIKey)
	require.True(t, routeHasCrowdSecHandler(t, cfg.Apps.HTTP.Servers["charon_server"].Routes))
}

// withTestBouncerKey gives the test a bouncer key from the environment and isolates it from any saved key file.
func withTestBouncerKey(t *testing.T) {
	t.Helper()
	useBouncerKeyFile(t, filepath.Join(t.TempDir(), "missing_bouncer_key"))
	clearCrowdSecKeyEnv(t)
	t.Setenv("CROWDSEC_API_KEY", "test-env-key")
}
