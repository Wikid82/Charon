package crowdsec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCopyDirPreservesSymlinks(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "copy")
	require.NoError(t, os.MkdirAll(dst, 0o700))

	require.NoError(t, os.MkdirAll(filepath.Join(src, "hub", "collections", "crowdsecurity"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(src, "collections"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(src, "hub", "collections", "crowdsecurity", "sshd.yaml"), []byte("name: sshd\n"), 0o600))
	// file link, directory link, dangling link, nested link
	require.NoError(t, os.Symlink("../hub/collections/crowdsecurity/sshd.yaml", filepath.Join(src, "collections", "sshd.yaml")))
	require.NoError(t, os.Symlink("hub/collections", filepath.Join(src, "dirlink")))
	require.NoError(t, os.Symlink("does/not/exist", filepath.Join(src, "dangling")))

	require.NoError(t, copyDir(src, dst))

	for name, want := range map[string]string{
		"collections/sshd.yaml": "../hub/collections/crowdsecurity/sshd.yaml",
		"dirlink":               "hub/collections",
		"dangling":              "does/not/exist",
	} {
		info, err := os.Lstat(filepath.Join(dst, name))
		require.NoError(t, err, name)
		require.NotZero(t, info.Mode()&os.ModeSymlink, "%s must remain a symlink", name)
		got, err := os.Readlink(filepath.Join(dst, name))
		require.NoError(t, err)
		require.Equal(t, want, got)
	}

	data, err := os.ReadFile(filepath.Join(dst, "hub", "collections", "crowdsecurity", "sshd.yaml")) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, err)
	require.Equal(t, "name: sshd\n", string(data))
	info, err := os.Lstat(filepath.Join(dst, "hub", "collections", "crowdsecurity", "sshd.yaml"))
	require.NoError(t, err)
	require.Zero(t, info.Mode()&os.ModeSymlink)
}

func TestCopyDirSymlinkDestinationError(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.Symlink("target", filepath.Join(src, "link")))
	dst := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dst, "link"), []byte("x"), 0o600))
	require.Error(t, copyDir(src, dst))
}
