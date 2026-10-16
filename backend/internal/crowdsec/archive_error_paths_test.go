package crowdsec

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithoutPath(t *testing.T) {
	t.Parallel()
	cause := errors.New("permission denied")
	require.Equal(t, cause, withoutPath(&fs.PathError{Op: "open", Path: "/secret/x", Err: cause}))
	require.Equal(t, cause, withoutPath(&os.LinkError{Op: "rename", Old: "/a", New: "/b", Err: cause}))
	require.Equal(t, cause, withoutPath(cause))
}

func TestCreateFileNoSymlinkMkdirParentFailure(t *testing.T) {
	t.Parallel()
	skipIfRoot(t)
	root := t.TempDir()
	// parentMode 0 lets MkdirAll create "a" but not its child, so the mkdir fails.
	dest := filepath.Join(root, "a", "b", "c.txt")
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "a"), 0o700) }) //nolint:gosec // G302: restore dir access so TempDir cleanup works
	f, err := CreateFileNoSymlink(root, dest, 0o000, 0o600)
	require.Nil(t, f)
	require.ErrorContains(t, err, "mkdir parent")
	require.NotContains(t, err.Error(), root)
}

func tarGzRaw(t *testing.T, build func(tw *tar.Writer)) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)
	build(tw)
	require.NoError(t, gw.Close())
	return buf.Bytes()
}
