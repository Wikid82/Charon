package crowdsec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

const (
	// maxKeyRestoreScan bounds how many snapshot files that need work (the destination is absent) are looked
	// at for private key files. Files the new tree already holds are not counted.
	maxKeyRestoreScan = 20000
	// maxKeyRestoreWalk bounds how many snapshot entries (files and folders) the walk visits at all, so that a
	// pathological tree fails closed even when most of it is present in the new tree.
	maxKeyRestoreWalk = 500_000
	// maxKeyRestoreFiles bounds how many key files are put back.
	maxKeyRestoreFiles = 64
	// maxKeyRestoreBytes bounds the size of one key file that is put back.
	maxKeyRestoreBytes = maxInspectEntryBytes
)

// ErrKeyRestoreLimit is returned when the snapshot holds more entries or key files than an import
// looks at or restores.
var ErrKeyRestoreLimit = errors.New("too many files to check for private keys")

// RestoreKeyFiles puts back from the snapshot taken before an import the private key files the export
// leaves out (the same rules: key_path targets, key file names, key armor in the first 64 KiB), because
// the import cleared them with the rest of the configuration. Only paths that do not exist in the new
// tree are written: an existing path of any kind (including the archive's own file and a link) wins,
// and nothing is written through a link or into a non-directory. Files keep their mode, capped at
// 0600, and are created exclusively. It returns the sorted slash paths below dataDir that were restored.
func RestoreKeyFiles(snapshotDir, dataDir string) ([]string, error) {
	return restoreKeyFiles(snapshotDir, dataDir, keyRestoreLimits{scan: maxKeyRestoreScan, walk: maxKeyRestoreWalk})
}

// keyRestoreLimits holds the two tree bounds of a restore; tests pass small ones.
type keyRestoreLimits struct {
	scan int // snapshot files that need work
	walk int // snapshot entries visited
}

// restoreKeyFiles is RestoreKeyFiles with the tree bounds given: scan counts only the snapshot files whose
// destination is absent, walk counts every entry visited.
func restoreKeyFiles(snapshotDir, dataDir string, limits keyRestoreLimits) ([]string, error) {
	snapshotDir, dataDir = filepath.Clean(snapshotDir), filepath.Clean(dataDir)
	dirs := treeNotificationDirs(snapshotDir)
	targets, err := keyPathTargets(context.Background(), snapshotDir, dataDir, dirs)
	if err != nil {
		return nil, fmt.Errorf("scan snapshot key settings: %w", err)
	}
	var restored []string
	scanned, walked := 0, 0
	walkErr := filepath.WalkDir(snapshotDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk snapshot: %w", withoutPath(err))
		}
		walked++
		if walked > limits.walk {
			return ErrKeyRestoreLimit
		}
		rel, relErr := filepath.Rel(snapshotDir, p)
		if relErr != nil {
			return fmt.Errorf("snapshot relative path: %w", relErr)
		}
		if rel != "." && IsExportExcluded(rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		slash := filepath.ToSlash(rel)
		present, presentErr := keyDestinationPresent(dataDir, slash)
		if presentErr != nil {
			return presentErr
		}
		if present {
			return nil // present in any form: the archive's file or a kept link wins, and it needs no work
		}
		scanned++
		if scanned > limits.scan {
			return ErrKeyRestoreLimit
		}
		ok, restoreErr := restoreKeyFile(snapshotDir, dataDir, slash, targets, dirs, len(restored))
		if restoreErr != nil {
			return restoreErr
		}
		if ok {
			restored = append(restored, slash)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Strings(restored)
	return restored, nil
}

// keyDestinationPresent reports whether the slash path below dataDir exists in any form. A file in the
// parent chain counts as absent here; restoring below it is refused later.
func keyDestinationPresent(dataDir, slash string) (bool, error) {
	_, err := os.Lstat(filepath.Join(dataDir, filepath.FromSlash(slash)))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ENOTDIR):
		return false, nil
	}
	return false, fmt.Errorf("inspect %s: %w", util.SanitizeForLog(slash), withoutPath(err))
}

// restoreKeyFile restores one snapshot file when it is a key file the new tree lacks. It reports
// whether the file was written.
func restoreKeyFile(snapshotDir, dataDir, slash string, targets map[string]struct{}, dirs []string, already int) (bool, error) {
	named := namesKeyFile(targets, slash)
	if !named && IsNotificationFile(slash, nil, dirs) == InDirOther {
		return false, nil
	}
	dest := filepath.Join(dataDir, filepath.FromSlash(slash))
	f, info, err := openExportFile(snapshotDir, filepath.FromSlash(slash), exportHooks{})
	if err != nil || f == nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	if !named {
		armored, armorErr := holdsKeyArmor(f)
		if armorErr != nil {
			return false, unreadableFile(slash, armorErr)
		}
		if !armored {
			return false, nil
		}
	}
	if already >= maxKeyRestoreFiles {
		return false, ErrKeyRestoreLimit
	}
	if info.Size() > maxKeyRestoreBytes {
		logger.Log().WithField("file", util.SanitizeForLog(path.Base(slash))).Warn("crowdsec import: a private key file is too large to keep")
		return false, nil
	}
	if !prepareKeyParents(dataDir, path.Dir(slash)) {
		logger.Log().WithField("file", util.SanitizeForLog(path.Base(slash))).Warn("crowdsec import: not restoring a private key file below a link or a non-directory")
		return false, nil
	}
	return writeKeyFile(dest, f, info.Mode().Perm()&0o600)
}

// prepareKeyParents makes sure every folder of the slash path dir below root is a plain directory,
// creating missing ones. It reports false when a component is a link or not a directory.
func prepareKeyParents(root, dir string) bool {
	if dir == "." {
		return true
	}
	cur := root
	for _, part := range strings.Split(dir, "/") {
		cur = filepath.Join(cur, part)
		st, err := os.Lstat(cur)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if mkErr := os.Mkdir(cur, 0o750); mkErr != nil {
				return false
			}
		case err != nil || !st.IsDir():
			return false
		}
	}
	return true
}

// writeKeyFile creates dest exclusively with the content of src and the given mode. It reports whether
// the file was written; a destination that appeared meanwhile is left alone.
func writeKeyFile(dest string, src io.Reader, mode os.FileMode) (bool, error) {
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, mode) // #nosec G304 -- dest is below the data folder, its parents were verified as plain directories and O_EXCL refuses any existing entry
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("create key file: %w", withoutPath(err))
	}
	n, err := io.Copy(out, io.LimitReader(src, maxKeyRestoreBytes+1))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil && n > maxKeyRestoreBytes {
		err = ErrKeyRestoreLimit
	}
	if err != nil {
		_ = os.Remove(dest)
		return false, fmt.Errorf("write key file: %w", withoutPath(err))
	}
	return true, nil
}
