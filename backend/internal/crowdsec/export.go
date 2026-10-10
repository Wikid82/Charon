package crowdsec

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

// Limits the configuration import enforces on an uploaded archive. An export is checked against the
// same numbers so that a successful export can always be imported again.
const (
	MaxImportCompressedBytes   int64   = 50 << 20
	MaxImportUncompressedBytes int64   = 500 << 20
	MaxImportCompressionRatio  float64 = 100
)

var (
	// ErrExportTooLarge reports an export that exceeds the size an import accepts.
	ErrExportTooLarge = errors.New("crowdsec config is too large to export")
	// ErrExportNotImportable reports an export whose compression ratio an import would reject.
	ErrExportNotImportable = errors.New("crowdsec config cannot be exported in an importable form")
	// ErrExportFileUnreadable reports a regular file that could not be read or changed while it was copied.
	ErrExportFileUnreadable = errors.New("a file in the CrowdSec folder could not be read")
)

// exportHooks lets tests change the tree between listing a file and reading it, and lower the
// compressed-size limit so the early abort can be exercised without a 50 MiB fixture.
type exportHooks struct {
	beforeOpen    func(path string)
	maxCompressed int64 // zero means MaxImportCompressedBytes
}

// limitWriter forwards to w and fails with ErrExportTooLarge once more than limit bytes have been
// written, so an oversized archive stops growing on disk as soon as an import would reject it.
type limitWriter struct {
	w       io.Writer
	limit   int64
	written int64
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if l.written+int64(len(p)) > l.limit {
		return 0, ErrExportTooLarge
	}
	n, err := l.w.Write(p)
	l.written += int64(n)
	return n, err
}

// CheckExportImportable verifies that an archive of the given compressed and uncompressed size passes
// the import validator's size and compression-ratio limits.
func CheckExportImportable(compressed, uncompressed int64) error {
	return checkExportImportable(compressed, uncompressed, MaxImportCompressedBytes, MaxImportCompressionRatio)
}

func checkExportImportable(compressed, uncompressed, maxCompressed int64, maxRatio float64) error {
	if compressed > maxCompressed {
		return ErrExportTooLarge
	}
	if compressed > 0 && float64(uncompressed)/float64(compressed) > maxRatio {
		return ErrExportNotImportable
	}
	return nil
}

// WriteExportArchive writes the portable configuration below dataDir to w as a tar.gz archive and
// returns the uncompressed byte total. Excluded paths (IsExportExcluded) are left out, directories are
// skipped with fs.SkipDir only when excluded, and non-regular entries (symlinks, pipes, sockets,
// devices) are skipped so that nothing is followed or blocks on open. A file that vanishes is skipped;
// an unreadable directory is logged and skipped; an unreadable regular file, or one whose size changes
// while it is copied, fails the export with ErrExportFileUnreadable.
func WriteExportArchive(ctx context.Context, dataDir string, w io.Writer) (int64, error) {
	return writeExportArchive(ctx, dataDir, w, MaxImportUncompressedBytes, exportHooks{})
}

func writeExportArchive(ctx context.Context, dataDir string, w io.Writer, maxBytes int64, hooks exportHooks) (int64, error) {
	maxCompressed := hooks.maxCompressed
	if maxCompressed <= 0 {
		maxCompressed = MaxImportCompressedBytes
	}
	gw := gzip.NewWriter(&limitWriter{w: w, limit: maxCompressed})
	tw := tar.NewWriter(gw)
	var total int64

	walkErr := filepath.WalkDir(dataDir, func(path string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			return handleExportWalkError(path, d, err)
		}
		rel, relErr := filepath.Rel(dataDir, path)
		if relErr != nil {
			return fmt.Errorf("export relative path: %w", relErr)
		}
		if rel != "." && IsExportExcluded(rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			logger.Log().Warnf("skipping non-regular entry %s during export", util.SanitizeForLog(rel))
			return nil
		}
		n, copyErr := addExportFile(tw, dataDir, rel, total, maxBytes, hooks)
		total += n
		return copyErr
	})
	if walkErr != nil {
		return total, fmt.Errorf("export walk: %w", walkErr)
	}
	if err := tw.Close(); err != nil {
		return total, fmt.Errorf("close tar writer: %w", err)
	}
	if err := gw.Close(); err != nil {
		return total, fmt.Errorf("close gzip writer: %w", err)
	}
	return total, nil
}

// handleExportWalkError applies the single walk-error rule: a vanished entry is skipped, a permission
// error on a directory skips that directory, everything else aborts.
func handleExportWalkError(path string, d fs.DirEntry, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case d != nil && d.IsDir() && errors.Is(err, fs.ErrPermission):
		logger.Log().WithError(err).Warnf("skipping unreadable directory %s during export", util.SanitizeForLog(path))
		return fs.SkipDir
	default:
		logger.Log().WithError(err).Warnf("failed to access %s during export", util.SanitizeForLog(path))
		return ErrExportFileUnreadable
	}
}

// addExportFile copies one regular file into tw and returns the bytes it contributed.
func addExportFile(tw *tar.Writer, dataDir, rel string, total, maxBytes int64, hooks exportHooks) (int64, error) {
	path := filepath.Join(dataDir, rel)
	// Lstat first: the entry came from the directory listing, so a swap to a link or pipe afterwards
	// must not be opened.
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return 0, unreadableFile(rel, err)
	}
	if total+info.Size() > maxBytes {
		return 0, ErrExportTooLarge
	}
	if hooks.beforeOpen != nil {
		hooks.beforeOpen(path)
	}

	// O_NOFOLLOW refuses a link swapped in after the Lstat (ELOOP); O_NONBLOCK keeps an open of a
	// swapped-in FIFO from blocking while the data lock is held. The opened file is verified below.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0) //nolint:gosec // G304: path is dataDir (Charon-owned) joined with a walked entry; O_NOFOLLOW|O_NONBLOCK plus the fstat identity check below guard against a swap after Lstat
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if errors.Is(err, syscall.ELOOP) {
		logger.Log().Warnf("skipping %s during export: replaced by a symlink", util.SanitizeForLog(rel))
		return 0, nil
	}
	if err != nil {
		return 0, unreadableFile(rel, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warnf("failed to close %s while archiving", util.SanitizeForLog(rel))
		}
	}()

	opened, err := f.Stat()
	if err != nil {
		return 0, unreadableFile(rel, err)
	}
	if !opened.Mode().IsRegular() {
		logger.Log().Warnf("skipping %s during export: no longer a regular file", util.SanitizeForLog(rel))
		return 0, nil
	}
	if !os.SameFile(info, opened) {
		return 0, unreadableFile(rel, errors.New("file was replaced while exporting"))
	}

	hdr := &tar.Header{
		Name:    filepath.ToSlash(rel),
		Size:    info.Size(),
		Mode:    int64(info.Mode().Perm()),
		ModTime: info.ModTime(),
	}
	if err = tw.WriteHeader(hdr); err != nil {
		return 0, fmt.Errorf("write tar header: %w", err)
	}
	if _, err = io.CopyN(tw, f, info.Size()); err != nil {
		if errors.Is(err, io.EOF) {
			return 0, unreadableFile(rel, fmt.Errorf("file shrank while copying: %w", err))
		}
		return 0, fmt.Errorf("copy file into archive: %w", err)
	}
	after, err := f.Stat()
	if err != nil {
		return 0, unreadableFile(rel, err)
	}
	if after.Size() != info.Size() {
		return 0, unreadableFile(rel, fmt.Errorf("file size changed while copying: %d -> %d", info.Size(), after.Size()))
	}
	return info.Size(), nil
}

// unreadableFile logs the cause with the sanitized relative path and returns the fixed sentinel.
func unreadableFile(rel string, cause error) error {
	logger.Log().WithError(cause).Warnf("cannot export %s", util.SanitizeForLog(rel))
	return ErrExportFileUnreadable
}
