package crowdsec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateFileNoSymlink(t *testing.T) {
	t.Parallel()

	t.Run("creates file and parents, truncating existing", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		dest := filepath.Join(root, "a", "b.txt")
		f, err := CreateFileNoSymlink(root, dest, 0o750, 0o640)
		require.NoError(t, err)
		_, err = f.WriteString("long content")
		require.NoError(t, err)
		require.NoError(t, f.Close())
		f, err = CreateFileNoSymlink(root, dest, 0o750, 0o640)
		require.NoError(t, err)
		_, err = f.WriteString("x")
		require.NoError(t, err)
		require.NoError(t, f.Close())
		got, err := os.ReadFile(dest) // #nosec G304 -- test path
		require.NoError(t, err)
		require.Equal(t, "x", string(got))
	})

	t.Run("rejects escaping path", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		_, err := CreateFileNoSymlink(root, filepath.Join(root, "..", "evil"), 0o750, 0o640)
		require.Error(t, err)
	})

	t.Run("rejects symlinked file", func(t *testing.T) {
		t.Parallel()
		root, outside := t.TempDir(), t.TempDir()
		target := filepath.Join(outside, "secret")
		require.NoError(t, os.WriteFile(target, []byte("keep"), 0o600))
		require.NoError(t, os.Symlink(target, filepath.Join(root, "link")))
		_, err := CreateFileNoSymlink(root, filepath.Join(root, "link"), 0o750, 0o640)
		require.ErrorIs(t, err, ErrSymlinkInPath)
		got, rerr := os.ReadFile(target) // #nosec G304 -- test path
		require.NoError(t, rerr)
		require.Equal(t, "keep", string(got))
	})

	t.Run("rejects symlinked parent dir", func(t *testing.T) {
		t.Parallel()
		root, outside := t.TempDir(), t.TempDir()
		require.NoError(t, os.Symlink(outside, filepath.Join(root, "dir")))
		_, err := CreateFileNoSymlink(root, filepath.Join(root, "dir", "f"), 0o750, 0o640)
		require.ErrorIs(t, err, ErrSymlinkInPath)
		require.NoFileExists(t, filepath.Join(outside, "f"))
	})

	t.Run("mkdir parent failure", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "file"), []byte("x"), 0o600))
		_, err := CreateFileNoSymlink(root, filepath.Join(root, "file", "child"), 0o750, 0o640)
		require.Error(t, err)
	})

	t.Run("open failure on directory", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(root, "d"), 0o750))
		_, err := CreateFileNoSymlink(root, filepath.Join(root, "d"), 0o750, 0o640)
		require.Error(t, err)
	})
}

func TestExtractTarGzRefusesSymlinkedDestination(t *testing.T) {
	t.Parallel()
	svc := NewHubService(nil, nil, t.TempDir())
	target, outside := t.TempDir(), t.TempDir()
	victim := filepath.Join(outside, "victim")
	require.NoError(t, os.WriteFile(victim, []byte("untouched"), 0o600))
	require.NoError(t, os.Symlink(victim, filepath.Join(target, "config.yaml")))

	err := svc.extractTarGz(t.Context(), makeTarGz(t, map[string]string{"config.yaml": "overwritten"}), target)
	require.ErrorIs(t, err, ErrSymlinkInPath)
	got, rerr := os.ReadFile(victim) // #nosec G304 -- test path
	require.NoError(t, rerr)
	require.Equal(t, "untouched", string(got))
}
