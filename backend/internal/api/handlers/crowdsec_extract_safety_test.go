package handlers

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeRawArchive(t *testing.T, entries [][2]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.tar.gz")
	f, err := os.Create(p) //nolint:gosec // G304: test temp path
	require.NoError(t, err)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for _, e := range entries {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: e[0], Mode: 0o640, Size: int64(len(e[1])), Typeflag: tar.TypeReg}))
		_, err = tw.Write([]byte(e[1]))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())
	return p
}

func TestExtractArchive_RepeatedNameLeavesNoStaleBytes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", dir)
	arc := writeRawArchive(t, [][2]string{{"config.yaml", "a long first version of the file"}, {"config.yaml", "short"}})
	require.NoError(t, h.extractArchive(arc, dir))
	require.Equal(t, "short", readTreeFile(t, filepath.Join(dir, "config.yaml")))
}

func TestExtractArchive_RefusesSymlinkedDestination(t *testing.T) {
	t.Parallel()
	dir, outside := t.TempDir(), t.TempDir()
	victim := filepath.Join(outside, "victim")
	require.NoError(t, os.WriteFile(victim, []byte("untouched"), 0o600))
	require.NoError(t, os.Symlink(victim, filepath.Join(dir, "config.yaml")))
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", dir)
	arc := writeRawArchive(t, [][2]string{{"config.yaml", "overwritten"}})
	require.Error(t, h.extractArchive(arc, dir))
	require.Equal(t, "untouched", readTreeFile(t, victim))
}
