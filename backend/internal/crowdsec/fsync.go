package crowdsec

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// dirSyncer is the part of an open directory that SyncDir uses; it exists so the error mapping is testable.
type dirSyncer interface {
	Sync() error
	Close() error
}

// SyncDir flushes the directory entries of dir to disk, so a rename into dir survives a crash. A filesystem
// that cannot sync a directory (EINVAL, ENOTSUP) counts as synced, because the rename itself already happened.
func SyncDir(dir string) error {
	d, err := os.Open(filepath.Clean(dir)) //nolint:gosec // G304: the caller wrote into dir itself; it is a directory, not user input
	if err != nil {
		return fmt.Errorf("open directory: %w", err)
	}
	return syncOpenDir(d)
}

// syncOpenDir syncs and closes an open directory, mapping the unsupported-sync errors to success.
func syncOpenDir(d dirSyncer) error {
	err := d.Sync()
	if closeErr := d.Close(); err == nil {
		err = closeErr
	}
	if err == nil || errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTSUP) || errors.Is(err, errors.ErrUnsupported) {
		return nil
	}
	return fmt.Errorf("sync directory: %w", err)
}
