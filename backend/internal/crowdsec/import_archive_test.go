package crowdsec

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// archiveWithEntries builds a tar.gz file of n zero-size regular files and returns its path.
func archiveWithEntries(t *testing.T, n int) string {
	t.Helper()
	data := tarGzRaw(t, func(tw *tar.Writer) {
		for i := range n {
			require.NoError(t, tw.WriteHeader(&tar.Header{Name: fmt.Sprintf("e%05d", i), Typeflag: tar.TypeReg, Mode: 0o600}))
		}
		require.NoError(t, tw.Close())
	})
	return writeArchiveFile(t, data)
}

func writeArchiveFile(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive.tar.gz")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

// countEntries reads every entry of the archive at path and returns their names.
func readArchiveEntryNames(t *testing.T, path string, bounds archiveBounds) ([]string, error) {
	t.Helper()
	ar, err := openImportArchive(path, bounds)
	require.NoError(t, err)
	defer func() { _ = ar.Close() }()
	var names []string
	for {
		hdr, nextErr := ar.Next()
		if errors.Is(nextErr, io.EOF) {
			return names, nil
		}
		if nextErr != nil {
			return names, nextErr
		}
		names = append(names, hdr.Name)
	}
}

func TestImportArchiveCountsEveryEntryAgainstTheBound(t *testing.T) {
	t.Parallel()
	bounds := archiveBounds{entries: 3, uncompressed: MaxImportUncompressedBytes, ratio: MaxImportCompressionRatio}

	names, err := readArchiveEntryNames(t, archiveWithEntries(t, 3), bounds)
	require.NoError(t, err)
	require.Len(t, names, 3, "exactly the bound is accepted")

	_, err = readArchiveEntryNames(t, archiveWithEntries(t, 4), bounds)
	require.ErrorIs(t, err, ErrArchiveTooManyEntries, "zero-size entries count too")
}

func TestImportArchiveReadsAcrossGzipMembers(t *testing.T) {
	t.Parallel()
	// One tar stream split across two gzip members: the reader must see it as a single stream.
	var tarBytes bytes.Buffer
	tw := tar.NewWriter(&tarBytes)
	for _, name := range []string{"a.yaml", "config/config.yaml", "c.yaml"} {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Size: 1, Mode: 0o600}))
		_, err := tw.Write([]byte("x"))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	half := tarBytes.Len() / 2
	var archive bytes.Buffer
	for _, part := range [][]byte{tarBytes.Bytes()[:half], tarBytes.Bytes()[half:]} {
		gw := gzip.NewWriter(&archive)
		_, err := gw.Write(part)
		require.NoError(t, err)
		require.NoError(t, gw.Close())
	}
	path := writeArchiveFile(t, archive.Bytes())

	ar, err := openImportArchive(path, defaultArchiveBounds())
	require.NoError(t, err)
	defer func() { _ = ar.Close() }()
	var names []string
	for {
		hdr, nextErr := ar.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		require.NoError(t, nextErr)
		body, readErr := io.ReadAll(ar)
		require.NoError(t, readErr)
		require.Len(t, body, 1)
		names = append(names, hdr.Name)
	}
	require.Equal(t, []string{"a.yaml", "config/config.yaml", "c.yaml"}, names)
}

func TestImportArchiveStopsEarlyAboveTheCompressionRatio(t *testing.T) {
	t.Parallel()
	const total = 32 << 20
	data := tarGzRaw(t, func(tw *tar.Writer) {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "config.yaml", Typeflag: tar.TypeReg, Size: total, Mode: 0o600}))
		_, err := io.CopyN(tw, zeroReader{}, total)
		require.NoError(t, err)
		require.NoError(t, tw.Close())
	})
	path := writeArchiveFile(t, data)

	ar, err := openImportArchive(path, defaultArchiveBounds())
	require.NoError(t, err)
	defer func() { _ = ar.Close() }()
	_, err = ar.Next()
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, ar)
	require.ErrorIs(t, err, ErrArchiveTooLarge)
	require.Less(t, ar.src.read, int64(total)/4, "the read stops once the ratio ceiling is passed, not at the end of the entry")
}

func TestExportThenImportAtTheEntryBoundary(t *testing.T) {
	t.Parallel()
	const limit = 3
	bounds := archiveBounds{entries: limit, uncompressed: MaxImportUncompressedBytes, ratio: MaxImportCompressionRatio}
	files := func(n int) map[string]string {
		m := map[string]string{}
		for i := range n {
			m[fmt.Sprintf("f%02d.txt", i)] = "x"
		}
		return m
	}

	t.Run("exactly the bound round-trips", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeExportTree(t, dir, files(limit))
		var buf bytes.Buffer
		_, _, err := writeExportArchive(context.Background(), dir, &buf, MaxImportUncompressedBytes, exportHooks{maxEntries: limit})
		require.NoError(t, err)

		names, err := readArchiveEntryNames(t, writeArchiveFile(t, buf.Bytes()), bounds)
		require.NoError(t, err)
		require.Len(t, names, limit)
	})

	t.Run("one entry over the bound is refused at export", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeExportTree(t, dir, files(limit+1))
		var buf bytes.Buffer
		_, _, err := writeExportArchive(context.Background(), dir, &buf, MaxImportUncompressedBytes, exportHooks{maxEntries: limit})
		require.ErrorIs(t, err, ErrExportTooLarge)
	})
}

// zeroReader yields an endless stream of zero bytes.
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// incompressible returns n random bytes. Highly compressible fixtures would trip the compression ratio
// ceiling before the entry bounds they test.
func incompressible(t *testing.T, n int) string {
	t.Helper()
	buf := make([]byte, n)
	_, err := rand.Read(buf)
	require.NoError(t, err)
	return string(buf)
}
