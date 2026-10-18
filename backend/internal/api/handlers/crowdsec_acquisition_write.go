package handlers

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

// replaceAcquisitionConfig atomically swaps the acquisition file for content. When a live file exists its
// bytes are first copied to a timestamped backup (whose path is returned), and the new content is written
// to a temporary sibling that is renamed over the live file, so a failure at any step leaves the previous
// live file untouched. The live file's mode is preserved (0o644 for a new file).
func replaceAcquisitionConfig(path, content string, now time.Time) (backupPath string, err error) {
	mode := fs.FileMode(0o644)
	old, readErr := os.ReadFile(path)
	switch {
	case readErr == nil:
		info, statErr := os.Stat(path)
		if statErr != nil {
			return "", fmt.Errorf("stat acquisition config: %w", statErr)
		}
		mode = info.Mode().Perm()
		backupPath = crowdsec.BackupFilePath(path, now)
		if err = os.WriteFile(backupPath, old, mode); err != nil {
			return "", fmt.Errorf("write acquisition backup: %w", err)
		}
	case !errors.Is(readErr, fs.ErrNotExist):
		return "", fmt.Errorf("read acquisition config: %w", readErr)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		removeIfSet(backupPath)
		return "", fmt.Errorf("create temporary acquisition config: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpName)
			removeIfSet(backupPath)
			backupPath = ""
		}
	}()
	if _, err = tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write temporary acquisition config: %w", err)
	}
	if err = tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("set acquisition config mode: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return "", fmt.Errorf("close temporary acquisition config: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return "", fmt.Errorf("replace acquisition config: %w", err)
	}
	return backupPath, nil
}

func removeIfSet(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}
