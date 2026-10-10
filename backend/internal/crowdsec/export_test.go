package crowdsec

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func writeExportTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	}
}

func exportNames(t *testing.T, data []byte) []string {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	tr := tar.NewReader(gr)
	var names []string
	for {
		hdr, nextErr := tr.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		require.NoError(t, nextErr)
		names = append(names, hdr.Name)
	}
	sort.Strings(names)
	return names
}

type failingWriter struct{ err error }

func (f failingWriter) Write([]byte) (int, error) { return 0, f.err }

func TestWriteExportArchiveContentsAndTotal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{
		"a.yaml":                "aaaa",
		"config/config.yaml":    "bb",
		"config/hub/index":      "hub",
		"config/sub/z.yaml":     "z",
		"data/crowdsec.db":      "db",
		"bouncer_key":           "k",
		"zz.yaml":               "zz",
		"config/aaa.yaml":       "a",
		"config/zzz.yaml":       "z",
		"crowdsec.pid":          "1",
		"config/bouncers/k.key": "k",
	})
	require.NoError(t, os.Symlink("/etc/nowhere", filepath.Join(dir, "config", "link.yaml")))

	var buf bytes.Buffer
	total, err := WriteExportArchive(context.Background(), dir, &buf)
	require.NoError(t, err)
	require.Equal(t, []string{"a.yaml", "config/aaa.yaml", "config/config.yaml", "config/sub/z.yaml", "config/zzz.yaml", "zz.yaml"},
		exportNames(t, buf.Bytes()))
	require.EqualValues(t, 4+2+1+1+1+2, total)
}

func TestWriteExportArchiveWriterFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{"a.yaml": "a"})
	sentinel := errors.New("disk full")
	_, err := WriteExportArchive(context.Background(), dir, failingWriter{err: sentinel})
	require.ErrorIs(t, err, sentinel)
}

func TestWriteExportArchiveCancelledContext(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{"a.yaml": "a", "b.yaml": "b"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := WriteExportArchive(ctx, dir, io.Discard)
	require.ErrorIs(t, err, context.Canceled)
}

func TestWriteExportArchiveUnreadableFileFails(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{"a.yaml": "a", "secret.conf": "x"})
	p := filepath.Join(dir, "secret.conf")
	require.NoError(t, os.Chmod(p, 0o000)) // #nosec G302 -- intentional test permission
	t.Cleanup(func() { _ = os.Chmod(p, 0o600) })

	_, err := WriteExportArchive(context.Background(), dir, io.Discard)
	require.ErrorIs(t, err, ErrExportFileUnreadable)
	require.NotContains(t, err.Error(), dir)
}

func TestWriteExportArchiveUnreadableDirectorySkipped(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{"a.yaml": "a", "locked/x.yaml": "x", "z.yaml": "z"})
	locked := filepath.Join(dir, "locked")
	require.NoError(t, os.Chmod(locked, 0o000))       // #nosec G302 -- intentional test permission
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) }) // #nosec G302 -- restore test directory

	var buf bytes.Buffer
	_, err := WriteExportArchive(context.Background(), dir, &buf)
	require.NoError(t, err)
	require.Equal(t, []string{"a.yaml", "z.yaml"}, exportNames(t, buf.Bytes()))
}

func TestWriteExportArchiveSkipsFIFO(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{"a.yaml": "a", "z.yaml": "z"})
	require.NoError(t, syscall.Mkfifo(filepath.Join(dir, "m.pipe"), 0o600))

	done := make(chan error, 1)
	var buf bytes.Buffer
	go func() {
		_, err := WriteExportArchive(context.Background(), dir, &buf)
		done <- err
	}()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("export blocked on a FIFO")
	}
	require.Equal(t, []string{"a.yaml", "z.yaml"}, exportNames(t, buf.Bytes()))
}

func TestWriteExportArchiveFileChangedWhileCopying(t *testing.T) {
	t.Parallel()
	cases := map[string]func(path string){
		"grows": func(path string) {
			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // test fixture
			if err == nil {
				_, _ = f.WriteString("more")
				_ = f.Close()
			}
		},
		"shrinks": func(path string) { _ = os.Truncate(path, 1) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeExportTree(t, dir, map[string]string{"a.yaml": "abcdef"})
			_, err := writeExportArchive(context.Background(), dir, io.Discard, MaxImportUncompressedBytes,
				exportHooks{beforeOpen: mutate})
			require.ErrorIs(t, err, ErrExportFileUnreadable)
		})
	}
}

func TestWriteExportArchiveVanishedFileSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{"a.yaml": "a", "gone.yaml": "g", "z.yaml": "z"})
	var buf bytes.Buffer
	total, err := writeExportArchive(context.Background(), dir, &buf, MaxImportUncompressedBytes, exportHooks{
		beforeOpen: func(path string) {
			if filepath.Base(path) == "gone.yaml" {
				_ = os.Remove(path)
			}
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"a.yaml", "z.yaml"}, exportNames(t, buf.Bytes()))
	require.EqualValues(t, 2, total)
}

func TestWriteExportArchiveSizeCap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{"a.yaml": "12345", "b.yaml": "12345"})
	_, err := writeExportArchive(context.Background(), dir, io.Discard, 8, exportHooks{})
	require.ErrorIs(t, err, ErrExportTooLarge)
}

func TestWriteExportArchiveMissingRoot(t *testing.T) {
	t.Parallel()
	_, err := WriteExportArchive(context.Background(), filepath.Join(t.TempDir(), "absent"), io.Discard)
	require.NoError(t, err) // a vanished root has nothing to export; the handler checks existence first
}

func TestCheckExportImportable(t *testing.T) {
	t.Parallel()
	require.NoError(t, CheckExportImportable(1000, 50_000))
	require.ErrorIs(t, CheckExportImportable(MaxImportCompressedBytes+1, MaxImportCompressedBytes+1), ErrExportTooLarge)
	require.ErrorIs(t, CheckExportImportable(1000, 101_000), ErrExportNotImportable)
	require.NoError(t, CheckExportImportable(1000, 100_000))
	require.NoError(t, CheckExportImportable(0, 0))
	// A tiny but highly compressible export is not "too large".
	require.NotErrorIs(t, CheckExportImportable(10, 10_000), ErrExportTooLarge)
}

// afterNWriter accepts the first n writes and fails every later one.
type afterNWriter struct {
	n   int
	err error
}

func (w *afterNWriter) Write(p []byte) (int, error) {
	if w.n <= 0 {
		return 0, w.err
	}
	w.n--
	return len(p), nil
}

func TestWriteExportArchiveTarCloseFailure(t *testing.T) {
	t.Parallel()
	// No files: the first write into gzip is the tar trailer, which forces the gzip header out and fails.
	_, err := WriteExportArchive(context.Background(), t.TempDir(), &afterNWriter{err: errors.New("disk full")})
	require.ErrorContains(t, err, "close tar writer")
}

func TestWriteExportArchiveGzipCloseFailure(t *testing.T) {
	t.Parallel()
	// The gzip header is written; the compressed trailer is flushed only by Close and fails there.
	_, err := WriteExportArchive(context.Background(), t.TempDir(), &afterNWriter{n: 1, err: errors.New("disk full")})
	require.ErrorContains(t, err, "close gzip writer")
}

func TestWriteExportArchiveCopyFailureIsNotMaskedAsUnreadable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	buf := make([]byte, 1<<20)
	_, err := rand.Read(buf)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "big.bin"), buf, 0o600))

	_, err = WriteExportArchive(context.Background(), dir, &afterNWriter{n: 1, err: errors.New("disk full")})
	require.ErrorContains(t, err, "copy file into archive")
	require.NotErrorIs(t, err, ErrExportFileUnreadable)
}

func TestHandleExportWalkError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	entries, err := os.ReadDir(filepath.Dir(dir))
	require.NoError(t, err)
	var dirEntry fs.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirEntry = e
			break
		}
	}
	require.NotNil(t, dirEntry)

	require.NoError(t, handleExportWalkError("gone", nil, fs.ErrNotExist))
	require.ErrorIs(t, handleExportWalkError("d", dirEntry, fs.ErrPermission), fs.SkipDir)
	// A permission error on a non-directory, and any other error, abort with the fixed sentinel.
	require.ErrorIs(t, handleExportWalkError("f", nil, fs.ErrPermission), ErrExportFileUnreadable)
	require.ErrorIs(t, handleExportWalkError("d", dirEntry, errors.New("io error")), ErrExportFileUnreadable)
}

func TestAddExportFileRefusesWhatLstatRejects(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{"real.yaml": "x"})
	require.NoError(t, os.Symlink(filepath.Join(dir, "real.yaml"), filepath.Join(dir, "link.yaml")))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o750))

	tw := tar.NewWriter(io.Discard)
	cases := map[string]string{
		"symlink swapped in":    "link.yaml",
		"directory swapped in":  "sub",
		"lstat fails (ENOTDIR)": filepath.Join("real.yaml", "child"),
	}
	for name, rel := range cases {
		n, err := addExportFile(tw, dir, rel, 0, MaxImportUncompressedBytes, exportHooks{})
		require.ErrorIs(t, err, ErrExportFileUnreadable, name)
		require.Zero(t, n, name)
	}

	n, err := addExportFile(tw, dir, "missing.yaml", 0, MaxImportUncompressedBytes, exportHooks{})
	require.NoError(t, err, "a vanished entry is skipped")
	require.Zero(t, n)
}
