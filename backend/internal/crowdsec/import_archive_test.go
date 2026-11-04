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

// readArchiveEntryNames reads every entry header of the archive at path and returns their names.
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

// nearRatioArchive writes an archive of n entries of size bytes each. The entries repeat a seeded set of
// blocks, so the declared content stays just under the compression ratio limit and the tar headers are what
// adds the rest of the stream. It returns the path and the declared content total.
func nearRatioArchive(t *testing.T, n, size, blockCount int) (path string, content int64) {
	t.Helper()
	// A fixed xorshift sequence keeps the fixture reproducible without a random source.
	state := uint32(2463534242)
	next := func() int {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		return int(state % 26)
	}
	const letters = "abcdefghijklmnopqrstuvwxyz"
	blocks := make([][]byte, blockCount)
	for i := range blocks {
		blocks[i] = make([]byte, 256)
		for j := range blocks[i] {
			blocks[i][j] = letters[next()]
		}
	}
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)

	for i := range n {
		body := make([]byte, 0, size)
		for len(body) < size {
			body = append(body, blocks[(i+len(body)/256)%blockCount]...)
		}
		body = body[:size]
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: fmt.Sprintf("d/f%05d.yaml", i), Typeflag: tar.TypeReg, Size: int64(size), Mode: 0o600}))
		_, err := tw.Write(body)
		require.NoError(t, err)
		content += int64(size)
	}
	require.NoError(t, tw.Close())
	var gz bytes.Buffer
	gw := gzip.NewWriter(&gz)
	_, err := gw.Write(raw.Bytes())
	require.NoError(t, err)
	require.NoError(t, gw.Close())
	return writeArchiveFile(t, gz.Bytes()), content
}

func TestImportAcceptsManySmallEntriesNearTheRatioLimit(t *testing.T) {
	t.Parallel()
	path, content := nearRatioArchive(t, 5000, 4096, 21)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.LessOrEqual(t, float64(content)/float64(info.Size()), MaxImportCompressionRatio, "the declared content stays within the ratio rule")

	names, err := readArchiveEntryNames(t, path, defaultArchiveBounds())
	require.NoError(t, err, "tar headers and padding do not push a rule-abiding archive over its size allowance")
	require.Len(t, names, 5000)
}
