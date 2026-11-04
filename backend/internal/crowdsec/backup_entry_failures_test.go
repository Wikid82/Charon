package crowdsec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The tests in this file provoke the per-entry failures of a tree copy and a restore clear. Permission cases
// skip as root, where a restricted directory is still readable.

func TestCopyTreeReportsAnUnreadableSymlink(t *testing.T) {
	skipIfRoot(t)
	t.Parallel()
	src, dst := t.TempDir(), t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(src, "sub"), 0o700))
	require.NoError(t, os.Symlink("../hub/x.yaml", filepath.Join(src, "sub", "link.yaml")))
	// Names can be listed without search access, but a link cannot be read without it.
	lockDir(t, filepath.Join(src, "sub"), 0o400)

	require.ErrorContains(t, copyTree(src, dst, nil), "readlink")
}

func TestCopyTreeReportsAnUnstattableDestination(t *testing.T) {
	skipIfRoot(t)
	t.Parallel()
	src, dst := t.TempDir(), t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(src, "sub"), 0o700))
	require.NoError(t, os.Symlink("../hub/x.yaml", filepath.Join(src, "sub", "link.yaml")))
	require.NoError(t, os.MkdirAll(filepath.Join(dst, "sub"), 0o700))
	// The destination directory cannot be searched, so the link's place cannot be looked up.
	lockDir(t, filepath.Join(dst, "sub"), 0o600)

	require.ErrorContains(t, copyTree(src, dst, nil), "stat ")
}

func TestCopyTreeReportsASymlinkThatCannotBeCreated(t *testing.T) {
	skipIfRoot(t)
	t.Parallel()
	src, dst := t.TempDir(), t.TempDir()
	require.NoError(t, os.Symlink("../hub/x.yaml", filepath.Join(src, "link.yaml")))
	// The destination directory is read-only, so nothing can be created in it.
	lockDir(t, dst, 0o500)

	require.ErrorContains(t, copyTree(src, dst, nil), "symlink ")
}

// TestCopyTreeReportsARefusedLinkRemoval refuses removals below dst, so it must not run in parallel.
func TestCopyTreeReportsARefusedLinkRemoval(t *testing.T) {
	t.Run("file where a link must go", func(t *testing.T) {
		src, dst := t.TempDir(), t.TempDir()
		require.NoError(t, os.Symlink("../hub/x.yaml", filepath.Join(src, "link.yaml")))
		writeFile(t, filepath.Join(dst, "link.yaml"), "live")
		refuseRemovalUnder(t, dst)

		require.ErrorContains(t, copyTree(src, dst, nil), "remove ")
		require.Equal(t, "live", readFile(t, filepath.Join(dst, "link.yaml")))
	})
	t.Run("link where a regular file must go", func(t *testing.T) {
		src, dst := t.TempDir(), t.TempDir()
		writeFile(t, filepath.Join(src, "config.yaml"), "snapshot")
		require.NoError(t, os.Symlink("/elsewhere", filepath.Join(dst, "config.yaml")))
		refuseRemovalUnder(t, dst)

		require.ErrorContains(t, copyTree(src, dst, nil), "remove ")
	})
	t.Run("link where a directory must go", func(t *testing.T) {
		src, dst, outside := t.TempDir(), t.TempDir(), t.TempDir()
		writeFile(t, filepath.Join(src, "sub", "a.yaml"), "a")
		require.NoError(t, os.Symlink(outside, filepath.Join(dst, "sub")))
		refuseRemovalUnder(t, dst)

		require.ErrorContains(t, copyTree(src, dst, nil), "remove ")
		require.NoFileExists(t, filepath.Join(outside, "a.yaml"), "nothing is written through the link")
	})
}
