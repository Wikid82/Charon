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
	// ErrExportConfigUnsafe reports an engine configuration whose credentials cannot be blanked safely (too
	// large, unparsable, or sharing credential nodes through anchors). Leaving it out would produce an archive
	// that an import refuses, so the export fails instead.
	ErrExportConfigUnsafe = errors.New("the CrowdSec configuration cannot be exported safely")
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
// returns the uncompressed byte total and the slash paths of the private key files it left out. Excluded paths (IsExportExcluded) are left out, directories are
// skipped with fs.SkipDir only when excluded, and non-regular entries (symlinks, pipes, sockets,
// devices) are skipped so that nothing is followed or blocks on open. A file that vanishes is skipped;
// an unreadable directory is logged and skipped; the installed hub items are listed in a generated
// ManifestEntryName entry (only when there are any); notification settings files have their sensitive
// values blanked (RedactNotificationFile) or, when that is not possible, are exported empty, and stray
// non-YAML files in a notification folder are left out; the engine configuration (config.yaml and
// config.yaml.local) has its credentials blanked (RedactConfigFile) or, when that is not possible, fails the
// export with ErrExportConfigUnsafe; an unreadable regular file, or one whose size changes
// while it is copied, fails the export with ErrExportFileUnreadable. Private key files stay on the host:
// the files notification settings name as key_path, files named *.key, *.p12 or *.pfx, and files whose
// first 64 KiB hold private key armor.
func WriteExportArchive(ctx context.Context, dataDir string, w io.Writer) (total int64, omitted []string, err error) {
	return writeExportArchive(ctx, dataDir, w, MaxImportUncompressedBytes, exportHooks{})
}

func writeExportArchive(ctx context.Context, dataDir string, w io.Writer, maxBytes int64, hooks exportHooks) (total int64, omitted []string, err error) {
	maxCompressed := hooks.maxCompressed
	if maxCompressed <= 0 {
		maxCompressed = MaxImportCompressedBytes
	}
	gw := gzip.NewWriter(&limitWriter{w: w, limit: maxCompressed})
	tw := tar.NewWriter(gw)
	dirs := treeNotificationDirs(dataDir)
	keyTargets, err := keyPathTargets(ctx, dataDir, dataDir, dirs)
	if err != nil {
		return 0, nil, err
	}

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
		if IsManifestPath(rel) {
			// The name is reserved for the generated hub item list; a live file never takes its place.
			logger.Log().Warn("skipping a file with the reserved hub item manifest name during export")
			return nil
		}
		if !d.Type().IsRegular() {
			logger.Log().Warnf("skipping non-regular entry %s during export", util.SanitizeForLog(rel))
			return nil
		}
		slash := filepath.ToSlash(rel)
		if namesKeyFile(keyTargets, slash) {
			omitted = append(omitted, slash)
			return nil
		}
		n, copyErr := addExportFile(tw, dataDir, rel, total, maxBytes, dirs, hooks)
		if errors.Is(copyErr, errFileOmitted) {
			omitted = append(omitted, slash)
			return nil
		}
		total += n
		return copyErr
	})
	if walkErr != nil {
		return total, nil, fmt.Errorf("export walk: %w", walkErr)
	}
	n, manifestErr := writeHubManifest(tw, dataDir, total, maxBytes)
	total += n
	if manifestErr != nil {
		return total, nil, fmt.Errorf("export hub item manifest: %w", manifestErr)
	}
	if err := tw.Close(); err != nil {
		return total, nil, fmt.Errorf("close tar writer: %w", err)
	}
	if err := gw.Close(); err != nil {
		return total, nil, fmt.Errorf("close gzip writer: %w", err)
	}
	if len(omitted) > 0 {
		logger.Log().Infof("left %d private key file(s) out of the export; supply them again after restoring", len(omitted))
	}
	return total, omitted, nil
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

// addExportFile copies one regular file into tw and returns the bytes it contributed. Files that are
// notification settings (IsNotificationFile) are rewritten by addRedactedExportFile, or left out.
func addExportFile(tw *tar.Writer, dataDir, rel string, total, maxBytes int64, dirs []string, hooks exportHooks) (int64, error) {
	slash := filepath.ToSlash(rel)
	kind := IsNotificationFile(slash, nil, dirs)
	if kind == InDirOther {
		// CrowdSec never reads these, and they cannot be rewritten structurally.
		logger.Log().Debugf("leaving %s out of the export: not a notification file CrowdSec reads", util.SanitizeForLog(rel))
		return 0, nil
	}
	f, info, err := openExportFile(dataDir, rel, hooks)
	if f == nil || err != nil {
		return 0, err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warnf("failed to close %s while archiving", util.SanitizeForLog(rel))
		}
	}()

	// The content rule runs on the descriptor that is copied next, so a file swapped after the walk is
	// judged by what is actually archived.
	if isEngineFile(slash) {
		logger.Log().Debugf("not sniffing %s for key armor: engine file exported as written", util.SanitizeForLog(rel))
	} else {
		armored, armorErr := holdsKeyArmor(f)
		if armorErr != nil {
			return 0, unreadableFile(rel, armorErr)
		}
		if armored {
			return 0, errFileOmitted
		}
	}

	if isConfigFile(slash) {
		return addConfigExportFile(tw, f, info, rel, total, maxBytes)
	}

	// A YAML file outside the notification folders may still be one (look-alike), so it is read and
	// probed too, as long as it is small enough to be parsed.
	if kind == InDirYAML || (hasYAMLExt(slash) && info.Size() <= maxInspectEntryBytes) {
		return addRedactedExportFile(tw, f, info, rel, kind, dirs, total, maxBytes)
	}
	if total+info.Size() > maxBytes {
		return 0, ErrExportTooLarge
	}
	return copyExportFile(tw, f, info, rel)
}

// openExportFile opens the walked entry rel for reading and verifies that it is still the regular file
// the walk listed. A nil file with a nil error means the entry is skipped (vanished or swapped for a link).
func openExportFile(dataDir, rel string, hooks exportHooks) (*os.File, os.FileInfo, error) {
	path := filepath.Join(dataDir, rel)
	// Lstat first: the entry came from the directory listing, so a swap to a link or pipe afterwards
	// must not be opened.
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return nil, nil, unreadableFile(rel, err)
	}
	if hooks.beforeOpen != nil {
		hooks.beforeOpen(path)
	}

	// O_NOFOLLOW refuses a link swapped in after the Lstat (ELOOP); O_NONBLOCK keeps an open of a
	// swapped-in FIFO from blocking while the data lock is held. The opened file is verified below.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0) //nolint:gosec // G304: path is dataDir (Charon-owned) joined with a walked entry; O_NOFOLLOW|O_NONBLOCK plus the fstat identity check below guard against a swap after Lstat
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if errors.Is(err, syscall.ELOOP) {
		logger.Log().Warnf("skipping %s during export: replaced by a symlink", util.SanitizeForLog(rel))
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, unreadableFile(rel, err)
	}

	opened, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, unreadableFile(rel, err)
	}
	if !opened.Mode().IsRegular() {
		_ = f.Close()
		logger.Log().Warnf("skipping %s during export: no longer a regular file", util.SanitizeForLog(rel))
		return nil, nil, nil
	}
	if !os.SameFile(info, opened) {
		_ = f.Close()
		return nil, nil, unreadableFile(rel, errors.New("file was replaced while exporting"))
	}
	return f, info, nil
}

// copyExportFile streams the opened file into tw byte for byte.
func copyExportFile(tw *tar.Writer, f *os.File, info os.FileInfo, rel string) (int64, error) {
	if err := tw.WriteHeader(exportHeader(rel, info, info.Size())); err != nil {
		return 0, fmt.Errorf("write tar header: %w", err)
	}
	if _, err := io.CopyN(tw, f, info.Size()); err != nil {
		if errors.Is(err, io.EOF) {
			return 0, unreadableFile(rel, fmt.Errorf("file shrank while copying: %w", err))
		}
		return 0, fmt.Errorf("copy file into archive: %w", err)
	}
	if err := checkUnchanged(f, info, rel); err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// checkUnchanged fails when the file no longer has the size it had when it was opened.
func checkUnchanged(f *os.File, info os.FileInfo, rel string) error {
	after, err := f.Stat()
	if err != nil {
		return unreadableFile(rel, err)
	}
	if after.Size() != info.Size() {
		return unreadableFile(rel, fmt.Errorf("file size changed while copying: %d -> %d", info.Size(), after.Size()))
	}
	return nil
}

func exportHeader(rel string, info os.FileInfo, size int64) *tar.Header {
	return &tar.Header{
		Name:    filepath.ToSlash(rel),
		Size:    size,
		Mode:    int64(info.Mode().Perm()),
		ModTime: info.ModTime(),
	}
}

// writeExportEntry writes data as the content of one archive entry and returns its length.
func writeExportEntry(tw *tar.Writer, rel string, info os.FileInfo, data []byte) (int64, error) {
	if err := tw.WriteHeader(exportHeader(rel, info, int64(len(data)))); err != nil {
		return 0, fmt.Errorf("write tar header: %w", err)
	}
	if _, err := tw.Write(data); err != nil {
		return 0, fmt.Errorf("write file into archive: %w", err)
	}
	return int64(len(data)), nil
}

// addRedactedExportFile writes a notification file with its sensitive values blanked. A file that
// cannot be rewritten safely (too large, unparsable, anything RedactNotificationFile refuses) is
// exported as an empty stub instead; the original bytes never reach the archive. The entry size and the
// running total use the length of what is written.
func addRedactedExportFile(tw *tar.Writer, f *os.File, info os.FileInfo, rel string, kind NotificationFileKind, dirs []string, total, maxBytes int64) (int64, error) {
	var out []byte
	if info.Size() > maxInspectEntryBytes {
		out = []byte(RedactStubContent)
		logStubbed(rel)
	} else {
		data, err := readStableExportFile(f, info, rel)
		if err != nil {
			return 0, err
		}
		if kind == NotAFile && IsNotificationFile(filepath.ToSlash(rel), LookAlikeDocuments(data), dirs) == NotAFile {
			out = data
		} else if res, redactErr := RedactNotificationFile(data); redactErr != nil {
			out = []byte(RedactStubContent)
			logStubbed(rel)
		} else {
			out = res.Data
		}
	}
	if total+int64(len(out)) > maxBytes {
		return 0, ErrExportTooLarge
	}
	return writeExportEntry(tw, rel, info, out)
}

// readStableExportFile reads the whole opened file (at most maxInspectEntryBytes) and verifies that it did
// not change size while it was read.
func readStableExportFile(f *os.File, info os.FileInfo, rel string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(f, maxInspectEntryBytes+1))
	if err != nil {
		return nil, unreadableFile(rel, err)
	}
	if int64(len(data)) != info.Size() {
		return nil, unreadableFile(rel, fmt.Errorf("file size changed while copying: %d -> %d", info.Size(), len(data)))
	}
	if err := checkUnchanged(f, info, rel); err != nil {
		return nil, err
	}
	return data, nil
}

// addConfigExportFile writes the engine configuration with its credentials blanked (RedactConfigFile); a
// file without credentials is written as it is. A file that cannot be redacted safely is never exported as
// written and is not left out either, since an archive without it could not be imported: the export fails
// with ErrExportConfigUnsafe.
func addConfigExportFile(tw *tar.Writer, f *os.File, info os.FileInfo, rel string, total, maxBytes int64) (int64, error) {
	if info.Size() > maxInspectEntryBytes {
		return 0, unsafeConfig(rel)
	}
	data, err := readStableExportFile(f, info, rel)
	if err != nil {
		return 0, err
	}
	res, redactErr := RedactConfigFile(data)
	if redactErr != nil {
		return 0, unsafeConfig(rel)
	}
	if total+int64(len(res.Data)) > maxBytes {
		return 0, ErrExportTooLarge
	}
	return writeExportEntry(tw, rel, info, res.Data)
}

// unsafeConfig logs, without any file content, which configuration file blocked the export.
func unsafeConfig(rel string) error {
	logger.Log().Warnf("cannot export %s: its credentials could not be blanked safely", util.SanitizeForLog(rel))
	return ErrExportConfigUnsafe
}

// logStubbed notes, without any file content, that a notification file was exported empty.
func logStubbed(rel string) {
	logger.Log().Warnf("exporting %s as an empty file: it could not be read safely", util.SanitizeForLog(rel))
}

// treeNotificationDirs returns the notification folders of the tree below dataDir (the live tree, an
// extracted archive or a snapshot), honouring a redirected folder named by the tree's own config.yaml.
// An unreadable config means the default folders. Export, merge and scan all resolve through it.
func treeNotificationDirs(dataDir string) []string {
	for _, rel := range []string{"config/config.yaml", "config.yaml"} {
		f, info, err := openExportFile(dataDir, filepath.FromSlash(rel), exportHooks{})
		if err != nil || f == nil {
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(f, maxInspectEntryBytes+1))
		_ = f.Close()
		if readErr != nil || info.Size() > maxInspectEntryBytes || int64(len(data)) > maxInspectEntryBytes {
			continue
		}
		return NotificationDirs(dataDir, data)
	}
	return NotificationDirs(dataDir, nil)
}

// unreadableFile logs the cause with the sanitized relative path and returns the fixed sentinel.
func unreadableFile(rel string, cause error) error {
	logger.Log().WithError(cause).Warnf("cannot read %s", util.SanitizeForLog(rel))
	return ErrExportFileUnreadable
}
