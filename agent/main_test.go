package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shortSocketPath returns a unix socket path short enough for the sun_path limit.
func shortSocketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "orthrus")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "d.sock")
}

func TestCheckDockerSocketReachable(t *testing.T) {
	path := shortSocketPath(t)
	ln, err := net.Listen("unix", path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	assert.NoError(t, checkDockerSocket(path))
}

func TestCheckDockerSocketRelativePathRejected(t *testing.T) {
	err := checkDockerSocket("docker.sock")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be an absolute path")
}

func TestCheckDockerSocketMissing(t *testing.T) {
	err := checkDockerSocket(filepath.Join(t.TempDir(), "missing.sock"))

	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.Contains(t, err.Error(), "not found: is it mounted?")
	assert.NotContains(t, err.Error(), "PGID")
}

func TestCheckDockerSocketRegularFileRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docker.sock")
	require.NoError(t, os.WriteFile(path, []byte("not a socket"), 0o600))

	err := checkDockerSocket(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not a unix socket")
}

func TestCheckDockerSocketPathIsCleaned(t *testing.T) {
	path := shortSocketPath(t)
	ln, err := net.Listen("unix", path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	// Redundant separators and dot segments must resolve to the same socket.
	messy := filepath.Dir(path) + string(filepath.Separator) + "." + string(filepath.Separator) + filepath.Base(path)
	assert.NoError(t, checkDockerSocket(messy))
}

func TestCheckDockerSocketPermissionHint(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses socket permissions")
	}
	path := shortSocketPath(t)
	ln, err := net.Listen("unix", path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	require.NoError(t, os.Chmod(path, 0o000))

	err = checkDockerSocket(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "PGID")
	assert.Contains(t, err.Error(), "docker group")
}

func TestResolveAuthKeyPrecedence(t *testing.T) {
	cases := []struct {
		name       string
		fileSet    bool
		fileKey    string
		envKey     string
		flagKey    string
		wantKey    string
		wantSource authKeySource
	}{
		{name: "file wins over flag", fileSet: true, fileKey: "from-file", flagKey: "from-flag", wantKey: "from-file", wantSource: authKeySourceFile},
		{name: "file alone", fileSet: true, fileKey: "from-file", wantKey: "from-file", wantSource: authKeySourceFile},
		{name: "env over flag", envKey: "from-env", flagKey: "from-flag", wantKey: "from-env", wantSource: authKeySourceEnv},
		{name: "env alone", envKey: "from-env", wantKey: "from-env", wantSource: authKeySourceEnv},
		{name: "flag alone", flagKey: "from-flag", wantKey: "from-flag", wantSource: authKeySourceFlag},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key, source, err := resolveAuthKey(tc.fileSet, tc.fileKey, tc.envKey, tc.flagKey)

			require.NoError(t, err)
			assert.Equal(t, tc.wantKey, key)
			assert.Equal(t, tc.wantSource, source)
		})
	}
}

func TestResolveAuthKeyFileAndEnvBothSetIsFatal(t *testing.T) {
	_, _, err := resolveAuthKey(true, "from-file", "from-env", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "set only one of ORTHRUS_AUTH_KEY_FILE and ORTHRUS_AUTH_KEY")
	assert.NotContains(t, err.Error(), "from-env")
}

func TestResolveAuthKeyNoneSetIsFatal(t *testing.T) {
	_, _, err := resolveAuthKey(false, "", "", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth key required")
}

func TestIsPrivateAgentHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"10.1.2.3", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"172.32.0.1", false},
		{"192.168.1.1", true},
		{"100.64.0.1", true},
		{"100.127.255.255", true},
		{"100.128.0.1", false},
		{"fd7a:115c:a1e0::1", true},
		{"::ffff:10.0.0.1", true},
		{"fe80::1", false},
		{"169.254.1.1", false},
		{"8.8.8.8", false},
		{"localhost", true},
		{"LOCALHOST.", true},
		{"app.localhost", true},
		{"charon", true},
		{"charon.", true},
		{"nas.local", true},
		{"nas.LOCAL.", true},
		{"router.home.arpa", true},
		{"svc.internal", true},
		{"charon.lan", false},
		{"host.tailnet.ts.net", false},
		{"example.com", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			assert.Equal(t, tc.want, isPrivateAgentHost(tc.host))
		})
	}
}

func TestValidateServerURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		optIn   bool
		wantErr bool
	}{
		{"wss to public host", "wss://charon.example.com/x", false, false},
		{"ws to RFC1918", "ws://10.0.0.5:8080/x", false, false},
		{"ws to Tailscale CGNAT", "ws://100.100.1.1:8080/x", false, false},
		{"ws to mDNS name", "ws://nas.local:8080/x", false, false},
		{"ws to compose service name", "ws://charon:8080/x", false, false},
		{"ws to public IP without opt-in", "ws://8.8.8.8:8080/x", false, true},
		{"ws to public IP with opt-in", "ws://8.8.8.8:8080/x", true, false},
		{"ws to link-local without opt-in", "ws://169.254.1.1:8080/x", false, true},
		{"ws to dotted LAN name without opt-in", "ws://charon.lan:8080/x", false, true},
		{"ws to dotted LAN name with opt-in", "ws://charon.lan:8080/x", true, false},
		{"https scheme rejected", "https://charon.example.com", true, true},
		{"unsupported scheme rejected", "ftp://charon.example.com", true, true},
		{"unparsable URL rejected", "://bad", false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateServerURL(tc.rawURL, tc.optIn)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseAllowInsecureWS(t *testing.T) {
	tests := []struct {
		value   string
		want    bool
		wantErr bool
	}{
		{"", false, false},
		{"true", true, false},
		{"1", true, false},
		{"false", false, false},
		{"nope", false, true},
	}
	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			got, err := parseAllowInsecureWS(tc.value)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
