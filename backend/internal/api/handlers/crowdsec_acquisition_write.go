package handlers

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

// backupCreateAttempts bounds how many consecutive microsecond timestamps are tried when a backup name
// is already taken.
const backupCreateAttempts = 5

// staleAcquisitionTempAge is how old a temporary file must be before a save removes it.
const staleAcquisitionTempAge = 10 * time.Minute

// writeFileAtomic is the atomic replacement used by replaceAcquisitionConfig, a seam so tests can force its
// outcomes without touching the file system.
var writeFileAtomic = crowdsec.WriteFileAtomic

// syncDir flushes a directory after a rename into it (see crowdsec.SyncDir); a seam so tests can observe it.
var syncDir = crowdsec.SyncDir

// syncDirAfterRename makes a completed rename durable for the file editor's save. The new content is already in
// place, so a failure is only logged and never fails the save.
func syncDirAfterRename(dir string) {
	if err := syncDir(dir); err != nil {
		logger.Log().WithError(err).WithField("path", util.SanitizeForLog(dir)).Warn("could not sync the directory after replacing a crowdsec file")
	}
}

// acquisitionFile is the part of *os.File the backup writer uses. It exists so tests can make any single step
// fail; *os.File satisfies it.
type acquisitionFile = crowdsec.WriteSyncCloser

// openAcquisitionBackupFile creates a backup exclusively, so an existing backup is never overwritten. The default
// returns a literal nil interface on error (a typed nil *os.File would look like a file).
var openAcquisitionBackupFile = func(name string, mode fs.FileMode) (acquisitionFile, error) {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode) //nolint:gosec // name derives from the validated acquisition path
	if err != nil {
		return nil, err
	}
	return f, nil
}

// replaceAcquisitionConfig atomically swaps the acquisition file for content. When a live file exists its bytes are
// first copied to a timestamped backup (whose path is returned), then the content is replaced through
// crowdsec.WriteFileAtomic, which keeps the live file's owner and mode and falls back to an in-place write when a
// rename is refused. A failure leaves the live file untouched and removes the backup, except when an in-place write
// failed partway: then the backup is the only intact copy and is kept.
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

	_, err = writeFileAtomic(path, []byte(content), mode, crowdsec.AtomicWriteOptions{
		KeepOwner:           true,
		InPlaceFallback:     true,
		SweepTempsOlderThan: staleAcquisitionTempAge,
		Now:                 now,
	})
	if err != nil {
		if !errors.Is(err, crowdsec.ErrPartialInPlaceWrite) {
			removeIfSet(createdBackup)
		}
		return "", fmt.Errorf("replace acquisition config: %w", err)
	}
	return createdBackup, nil
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
		if err = crowdsec.WriteSyncClose(f, data); err != nil {
			_ = os.Remove(name)
			return "", err
		}
		return name, nil
	}
	return "", fmt.Errorf("no free backup name after %d attempts: %w", backupCreateAttempts, lastErr)
}

func removeIfSet(path string) {
	if path != "" {
		_ = os.Remove(path) //nolint:gosec // G703: removes only the temp/backup file this function created
	}
}
