package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveCaddyAdminAPI_SocketPathsAreAbsolute(t *testing.T) {
	wd, err := filepath.Abs(".")
	require.NoError(t, err)

	got, legacy, err := resolveCaddyAdminAPI("unix://data/run/caddy-admin.sock")
	require.NoError(t, err)
	assert.False(t, legacy)
	assert.Equal(t, "unix://"+filepath.Join(wd, "data", "run", "caddy-admin.sock"), got)

	got, legacy, err = resolveCaddyAdminAPI("unix:///run/charon/admin.sock")
	require.NoError(t, err)
	assert.False(t, legacy)
	assert.Equal(t, "unix:///run/charon/admin.sock", got)
}

func TestResolveCaddyAdminAPI_RefusesUnusableSocketPaths(t *testing.T) {
	cases := map[string]string{
		"empty path":            "unix://",
		"pipe separator":        "unix:///run/a|b.sock",
		"double quote":          "unix:///run/a\"b.sock",
		"backslash":             `unix:///run/a\b.sock`,
		"control character":     "unix:///run/a\tb.sock",
		"too long for sun_path": "unix:///" + strings.Repeat("a", 120) + ".sock",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := resolveCaddyAdminAPI(raw)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "socket path")
		})
	}
}

// A loopback URL on the admin port is accepted, but Charon uses the default socket
// instead, so the container never opens a TCP admin listener.
func TestResolveCaddyAdminAPI_LoopbackURLMapsToSocket(t *testing.T) {
	got, legacy, err := resolveCaddyAdminAPI("http://localhost:2019")
	require.NoError(t, err)
	assert.True(t, legacy)
	assert.Equal(t, DefaultCaddyAdminAPI(), got)
}

func TestResolveCaddyAdminAPI_RefusesOtherHosts(t *testing.T) {
	_, _, err := resolveCaddyAdminAPI("http://caddy:2019")
	require.Error(t, err)
	assert.Equal(t, caddyAdminRefusedMessage, err.Error())
}

func TestResolveCaddyAdminAPI_RejectsWrongLoopbackPort(t *testing.T) {
	_, _, err := resolveCaddyAdminAPI("http://localhost:2020")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid caddy admin api url")
}

func TestLoad_LoopbackAdminURLWarnsAndUsesSocket(t *testing.T) {
	setCaddyAdminTestEnv(t, "http://localhost:2019")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, DefaultCaddyAdminAPI(), cfg.CaddyAdminAPI)
	assert.Contains(t, cfg.StartupWarnings, LegacyAdminURLWarning(DefaultCaddyAdminAPI()))
}

func TestCaddyAdminSocketPath(t *testing.T) {
	path, ok := CaddyAdminSocketPath("unix:///run/charon/admin.sock")
	assert.True(t, ok)
	assert.Equal(t, "/run/charon/admin.sock", path)

	_, ok = CaddyAdminSocketPath("http://localhost:2019")
	assert.False(t, ok)
}

func TestCaddyAdminListen(t *testing.T) {
	// Caddy's unix form is "unix/" plus the absolute path, so the path keeps its leading slash.
	assert.Equal(t, "unix//run/charon/admin.sock|0600", CaddyAdminListen("unix:///run/charon/admin.sock"))
	assert.Equal(t, CaddyAdminTCPAddress, CaddyAdminListen("http://localhost:2019"))
}

func TestDefaultCaddyAdminAPI(t *testing.T) {
	got := DefaultCaddyAdminAPI()
	path, ok := CaddyAdminSocketPath(got)
	require.True(t, ok)
	assert.True(t, filepath.IsAbs(path))
	assert.Equal(t, "unix://"+path, got)
}

// chdirToRemovedDirectory makes the working directory unresolvable, so relative
// paths cannot be made absolute. The original directory is restored after the test.
func chdirToRemovedDirectory(t *testing.T) {
	t.Helper()
	gone := filepath.Join(t.TempDir(), "gone")
	require.NoError(t, os.Mkdir(gone, 0o700))
	t.Chdir(gone)
	require.NoError(t, os.Remove(gone))
	if _, err := os.Getwd(); err == nil {
		t.Skip("this platform still resolves a removed working directory")
	}
}

func TestDefaultCaddyAdminAPI_FallsBackToRelativePath(t *testing.T) {
	chdirToRemovedDirectory(t)
	assert.Equal(t, "unix://"+DefaultCaddyAdminSocket, DefaultCaddyAdminAPI())
}

func TestResolveCaddyAdminAPI_ReportsUnresolvableSocketPath(t *testing.T) {
	chdirToRemovedDirectory(t)
	_, _, err := resolveCaddyAdminAPI("unix://relative/admin.sock")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve socket path")
}
