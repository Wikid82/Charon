package handlers

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

// backupCreateAttempts bounds how many consecutive microsecond timestamps are tried when a backup name
// is already taken.
const backupCreateAttempts = 5

// renameFile is the rename step of replaceAcquisitionConfig, a seam so tests can force its failure modes.
var renameFile = os.Rename

// syncDir flushes a directory after a rename into it (see crowdsec.SyncDir); a seam so tests can observe it.
var syncDir = crowdsec.SyncDir

// staleAcquisitionTempAge is how old a temporary file must be before a save removes it.
const staleAcquisitionTempAge = 10 * time.Minute

// syncDirAfterRename makes a completed rename durable. The new content is already in place, so a failure is
// only logged and never fails the save.
func syncDirAfterRename(dir string) {
	if err := syncDir(dir); err != nil {
		logger.Log().WithError(err).WithField("path", util.SanitizeForLog(dir)).Warn("could not sync the directory after replacing a crowdsec file")
	}
}

// sweepStaleAcquisitionTemps removes the temporary files that earlier saves of path left behind when the process
// died mid-write. Only regular files named <base>.tmp-* older than staleAcquisitionTempAge are removed; symlinks and
// directories are never touched. The age guard matters only when another process writes into the same directory,
// where a fresh temporary file may still be in use. Failures are logged at debug level and never fail the save.
func sweepStaleAcquisitionTemps(path string, now time.Time) {
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		logger.Log().WithError(err).WithField("path", util.SanitizeForLog(dir)).Debug("could not list the acquisition directory to sweep temporary files")
		return
	}
	prefix := filepath.Base(path) + ".tmp-"
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		tmpPath := filepath.Join(dir, entry.Name())
		info, err := os.Lstat(tmpPath)
		if err != nil {
			logger.Log().WithError(err).WithField("path", util.SanitizeForLog(tmpPath)).Debug("could not inspect a stale acquisition temporary file")
			continue
		}
		if !info.Mode().IsRegular() || now.Sub(info.ModTime()) < staleAcquisitionTempAge {
			continue
		}
		if err := os.Remove(tmpPath); err != nil {
			logger.Log().WithError(err).WithField("path", util.SanitizeForLog(tmpPath)).Debug("could not remove a stale acquisition temporary file")
		}
	}
}

// acquisitionFile is the part of *os.File the acquisition writer uses. It exists so tests can make any
// single step fail; *os.File satisfies it.
type acquisitionFile interface {
	Name() string
	Write(p []byte) (int, error)
	Chmod(mode fs.FileMode) error
	Chown(uid, gid int) error
	Sync() error
	Close() error
}

// The file seams of replaceAcquisitionConfig. The defaults return a literal nil interface on error: a typed
// nil *os.File inside the interface would look like a file.
var (
	// createAcquisitionTempFile creates the temporary sibling the new content is written to.
	createAcquisitionTempFile = func(dir, pattern string) (acquisitionFile, error) {
		f, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	// openAcquisitionBackupFile creates a backup exclusively, so an existing backup is never overwritten.
	openAcquisitionBackupFile = func(name string, mode fs.FileMode) (acquisitionFile, error) {
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode) //nolint:gosec // name derives from the validated acquisition path
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	// openInPlaceFile opens the live file itself for an in-place write (see writeInPlace).
	openInPlaceFile = func(name string, mode fs.FileMode) (acquisitionFile, error) {
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, mode) //nolint:gosec // name is the validated acquisition path
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	// fileOwner reads the owner of a file and processOwner the owner of the running process (see keepOwner).
	fileOwner = func(info fs.FileInfo) (uid, gid int, ok bool) {
		stat, isStat := info.Sys().(*syscall.Stat_t)
		if !isStat {
			return 0, 0, false
		}
		return int(stat.Uid), int(stat.Gid), true
	}
	processOwner = func() (uid, gid int) { return os.Geteuid(), os.Getegid() }
)

// replaceAcquisitionConfig atomically swaps the acquisition file for content. When a live file exists its
// bytes are first copied to a timestamped backup (whose path is returned), and the new content is written
// to a temporary sibling that is renamed over the live file, so a failure at any step leaves the previous
// live file untouched. The live file's mode is preserved (0o644 for a new file).
//
// The temporary file takes the live file's owner (see keepOwner), so the engine keeps reading a file it owns.
//
// A rename over a bind-mounted single file (or across devices, or onto a read-only mount) fails with
// EBUSY/EXDEV/EROFS; then the content is written in place instead and the backup is kept as the safety net.
// The same in-place write is used when chown is not permitted; if even opening the live file is denied, the
// rename is the last resort and the owner changes (logged as a warning).
func replaceAcquisitionConfig(path, content string, now time.Time) (backupPath string, err error) {
	path = filepath.Clean(path)
	sweepStaleAcquisitionTemps(path, now)
	mode := fs.FileMode(0o644)
	var createdBackup string // separate from the named return: `return "", err` zeroes backupPath before defers run
	var live fs.FileInfo     // nil when the file does not exist yet
	info, statErr := os.Stat(path)
	switch {
	case statErr == nil:
		live = info
		mode = info.Mode().Perm()
		old, readErr := os.ReadFile(path) //nolint:gosec // G304: acquisition path is operator config (CHARON_CROWDSEC_ACQUIS_PATH), validated by resolveAcquisitionConfigPath and cleaned above
		if readErr != nil {
			return "", fmt.Errorf("read acquisition config: %w", readErr)
		}
		if createdBackup, err = writeBackupExclusive(path, old, mode, now); err != nil {
			return "", fmt.Errorf("write acquisition backup: %w", err)
		}
	case !errors.Is(statErr, fs.ErrNotExist):
		return "", fmt.Errorf("stat acquisition config: %w", statErr)
	}

	tmp, err := createAcquisitionTempFile(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		removeIfSet(createdBackup)
		return "", fmt.Errorf("create temporary acquisition config: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName) // already gone after a successful rename
		if err != nil {
			removeIfSet(createdBackup)
		}
	}()
	inPlace, writeErr := writeTempAcquisition(tmp, content, mode, live)
	if writeErr != nil {
		return "", writeErr
	}
	if inPlace {
		opened, ipErr := writeInPlace(path, content, mode)
		switch {
		case ipErr == nil:
			return createdBackup, nil
		case !opened && errors.Is(ipErr, fs.ErrPermission):
			logger.Log().WithField("path", util.SanitizeForLog(path)).Warn("acquisition file owner could not be kept; replacing the file instead")
		default:
			createdBackup = "" // the live file may be half-written, so the backup is the only intact copy
			return "", fmt.Errorf("write acquisition config in place: %w", ipErr)
		}
	}
	if err = renameFile(tmpName, path); err == nil {
		syncDirAfterRename(filepath.Dir(path))
		return createdBackup, nil
	}
	if !isRenameUnsupported(err) {
		return "", fmt.Errorf("replace acquisition config: %w", err)
	}
	if _, err = writeInPlace(path, content, mode); err != nil {
		createdBackup = "" // the live file may be half-written, so the backup is the only intact copy
		return "", fmt.Errorf("write acquisition config in place: %w", err)
	}
	return createdBackup, nil
}

// writeTempAcquisition fills the temporary file, chmods it, gives it the live file's owner (see keepOwner),
// then syncs and closes it. inPlace reports that the owner could not be kept, so the caller writes the live
// file itself. live is nil when there is no live file yet.
func writeTempAcquisition(tmp acquisitionFile, content string, mode fs.FileMode, live fs.FileInfo) (inPlace bool, err error) {
	_, err = tmp.Write([]byte(content))
	if err == nil {
		err = tmp.Chmod(mode)
	}
	if err == nil {
		inPlace, err = keepOwner(tmp, live)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return false, fmt.Errorf("write temporary acquisition config: %w", err)
	}
	return inPlace, nil
}

// keepOwner gives the temporary file the owner of the live file, so the rename does not change who owns the
// file the engine reads. A new file (live nil), an unknown owner and an owner that already is the process
// owner need nothing. When chown is not permitted (EPERM), inPlace asks the caller to write the live file
// itself: keeping the inode keeps owner and bind mounts intact. That write is not atomic; the backup is the
// safety net for it, and refusing the save would break root-started containers where the rename works today.
func keepOwner(tmp acquisitionFile, live fs.FileInfo) (inPlace bool, err error) {
	if live == nil {
		return false, nil
	}
	uid, gid, ok := fileOwner(live)
	if !ok {
		return false, nil
	}
	if processUID, processGID := processOwner(); uid == processUID && gid == processGID {
		return false, nil
	}
	switch err = tmp.Chown(uid, gid); {
	case err == nil:
		return false, nil
	case errors.Is(err, fs.ErrPermission):
		return true, nil
	default:
		return false, fmt.Errorf("keep acquisition file owner: %w", err)
	}
}

// writeInPlace replaces the content of the live file at path without renaming it, so the inode, owner and mode
// stay. opened reports whether the file was opened; when it was not, nothing was touched.
func writeInPlace(path, content string, mode fs.FileMode) (opened bool, err error) {
	f, err := openInPlaceFile(path, mode)
	if err != nil {
		return false, fmt.Errorf("open acquisition config: %w", err)
	}
	return true, writeSyncClose(f, []byte(content))
}

// writeBackupExclusive writes data to the backup name for now, advancing one microsecond whenever the name
// is already taken so an existing backup is never overwritten.
func writeBackupExclusive(path string, data []byte, mode fs.FileMode, now time.Time) (string, error) {
	var lastErr error
	for i := range backupCreateAttempts {
		name := crowdsec.BackupFilePath(path, now.Add(time.Duration(i)*time.Microsecond))
		f, err := openAcquisitionBackupFile(name, mode)
		if errors.Is(err, fs.ErrExist) {
			lastErr = err
			continue
		}
		if err != nil {
			return "", err
		}
		if err = writeSyncClose(f, data); err != nil {
			_ = os.Remove(name)
			return "", err
		}
		return name, nil
	}
	return "", fmt.Errorf("no free backup name after %d attempts: %w", backupCreateAttempts, lastErr)
}

func writeSyncClose(f acquisitionFile, data []byte) error {
	_, err := f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// isRenameUnsupported reports whether err means the target cannot be replaced by rename (bind-mounted
// file, cross-device, read-only mount) but may still be writable in place.
func isRenameUnsupported(err error) bool {
	var linkErr *os.LinkError
	if !errors.As(err, &linkErr) {
		return false
	}
	return errors.Is(linkErr.Err, syscall.EBUSY) || errors.Is(linkErr.Err, syscall.EXDEV) || errors.Is(linkErr.Err, syscall.EROFS)
}

func removeIfSet(path string) {
	if path != "" {
		_ = os.Remove(path) //nolint:gosec // G703: removes only the temp/backup file this function created
	}
}
