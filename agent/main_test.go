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

func TestCheckDockerSocketMissing(t *testing.T) {
	err := checkDockerSocket(filepath.Join(t.TempDir(), "missing.sock"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "dial docker socket")
	assert.NotContains(t, err.Error(), "PGID")
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
