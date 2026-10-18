package handlers

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

// backupCreateAttempts bounds how many consecutive microsecond timestamps are tried when a backup name
// is already taken.
const backupCreateAttempts = 5

// renameFile is the rename step of replaceAcquisitionConfig, a seam so tests can force its failure modes.
var renameFile = os.Rename

// replaceAcquisitionConfig atomically swaps the acquisition file for content. When a live file exists its
// bytes are first copied to a timestamped backup (whose path is returned), and the new content is written
// to a temporary sibling that is renamed over the live file, so a failure at any step leaves the previous
// live file untouched. The live file's mode is preserved (0o644 for a new file).
//
// A rename over a bind-mounted single file (or across devices, or onto a read-only mount) fails with
// EBUSY/EXDEV/EROFS; then the content is written in place instead and the backup is kept as the safety net.
func replaceAcquisitionConfig(path, content string, now time.Time) (backupPath string, err error) {
	path = filepath.Clean(path)
	mode := fs.FileMode(0o644)
	var createdBackup string // separate from the named return: `return "", err` zeroes backupPath before defers run
	info, statErr := os.Stat(path)
	switch {
	case statErr == nil:
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

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
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
	if writeErr := writeTempAcquisition(tmp, content, mode); writeErr != nil {
		return "", writeErr
	}
	if err = renameFile(tmpName, path); err != nil {
		if !isRenameUnsupported(err) {
			return "", fmt.Errorf("replace acquisition config: %w", err)
		}
		if err = os.WriteFile(path, []byte(content), mode); err != nil {
			createdBackup = "" // the live file may be half-written, so the backup is the only intact copy
			return "", fmt.Errorf("write acquisition config in place: %w", err)
		}
	}
	return createdBackup, nil
}

// writeTempAcquisition fills, chmods, syncs and closes the temporary file.
func writeTempAcquisition(tmp *os.File, content string, mode fs.FileMode) error {
	_, err := tmp.WriteString(content)
	if err == nil {
		err = tmp.Chmod(mode)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write temporary acquisition config: %w", err)
	}
	return nil
}

// writeBackupExclusive writes data to the backup name for now, advancing one microsecond whenever the name
// is already taken so an existing backup is never overwritten.
func writeBackupExclusive(path string, data []byte, mode fs.FileMode, now time.Time) (string, error) {
	var lastErr error
	for i := range backupCreateAttempts {
		name := crowdsec.BackupFilePath(path, now.Add(time.Duration(i)*time.Microsecond))
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode) //nolint:gosec // name derives from the validated acquisition path
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

func writeSyncClose(f *os.File, data []byte) error {
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
