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
