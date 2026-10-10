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

func TestExtractTarGzDirMkdirFailure(t *testing.T) {
	t.Parallel()
	skipIfRoot(t)
	target := t.TempDir()
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(target, "a"), 0o700) }) //nolint:gosec // G302: restore dir access so TempDir cleanup works
	archive := tarGzRaw(t, func(tw *tar.Writer) {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "a/", Typeflag: tar.TypeDir, Mode: 0o000}))
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "a/b/", Typeflag: tar.TypeDir, Mode: 0o700}))
		require.NoError(t, tw.Close())
	})
	err := NewHubService(nil, nil, target).extractTarGz(t.Context(), archive, target)
	require.ErrorContains(t, err, "mkdir a/b")
	require.NotContains(t, err.Error(), target)
}

func TestExtractTarGzTruncatedEntry(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	// Header promises 10 bytes but the stream ends after 3 and the tar is never closed.
	archive := tarGzRaw(t, func(tw *tar.Writer) {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "f.yaml", Mode: 0o600, Size: 10}))
		_, err := tw.Write([]byte("abc"))
		require.NoError(t, err)
	})
	err := NewHubService(nil, nil, target).extractTarGz(t.Context(), archive, target)
	require.ErrorContains(t, err, "write f.yaml")
	require.NotContains(t, err.Error(), target)
}

func TestExtractTarGzDecompressionBomb(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	const size = 100 * 1024 * 1024
	archive := tarGzRaw(t, func(tw *tar.Writer) {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "bomb.bin", Mode: 0o600, Size: size}))
		chunk := make([]byte, 1024*1024)
		for i := 0; i < size/len(chunk); i++ {
			_, err := tw.Write(chunk)
			require.NoError(t, err)
		}
		require.NoError(t, tw.Close())
	})
	err := NewHubService(nil, nil, target).extractTarGz(t.Context(), archive, target)
	require.ErrorContains(t, err, "decompression bomb")
}
