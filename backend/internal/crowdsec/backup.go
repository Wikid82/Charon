package crowdsec

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

const (
	// DefaultBackupRetention is how many full-tree snapshots (<DataDir>.backup.*) are kept.
	DefaultBackupRetention = 5
	// DefaultFileBackupRetention is how many single-file edit backups (<DataDir>.filebackup.*) are kept.
	DefaultFileBackupRetention = 10

	// BackupKindSnapshot and BackupKindFile name the two independent backup namespaces next to DataDir.
	BackupKindSnapshot = "backup"
	BackupKindFile     = "filebackup"

	// backupTimeFormat includes sub-second precision so back-to-back backups never share a directory.
	backupTimeFormat = "20060102-150405.000000"
	// legacyBackupTimeFormat is the second-resolution layout older releases used.
	legacyBackupTimeFormat = "20060102-150405"

	// snapshotMkdirAttempts bounds retries when a microsecond-named directory already exists.
	snapshotMkdirAttempts = 3
)

// isLiveDBFile reports whether name is the live CrowdSec SQLite database or its WAL/SHM sidecars.
// These are owned by the running engine and must never be copied into, or restored from, a backup.
func isLiveDBFile(name string) bool {
	switch name {
	case "crowdsec.db", "crowdsec.db-wal", "crowdsec.db-shm":
		return true
	}
	return false
}

// IsEngineOwnedPath reports whether rel (a slash- or OS-separated path relative to DataDir) is state
// the running engine owns and that snapshots, restores and archive imports must never touch: a live
// db file at any depth, or anything under the top-level data/ (LAPI db and hub data files) or
// hub_cache/ (regenerable download cache). Only the top-level directories are excluded; a nested
// directory with the same name stays part of the configuration.
func IsEngineOwnedPath(rel string) bool {
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || rel == "" {
		return false
	}
	if isLiveDBFile(filepath.Base(rel)) {
		return true
	}
	top, _, _ := strings.Cut(rel, "/")
	return top == "data" || top == "hub_cache"
}

// snapshot copies DataDir into a new <DataDir>.backup.<ts> directory; see Snapshot.
func (s *HubService) snapshot() (string, error) { return Snapshot(s.DataDir) }

// Snapshot copies dataDir (symlinks preserved, engine-owned state excluded) into a new
// <dataDir>.backup.<ts> directory and returns its path. dataDir is never modified.
func Snapshot(dataDir string) (string, error) {
	parent, base := filepath.Split(filepath.Clean(dataDir))
	var path string
	var mkErr error
	for range snapshotMkdirAttempts {
		path = filepath.Join(parent, base+"."+BackupKindSnapshot+"."+time.Now().Format(backupTimeFormat))
		if mkErr = os.Mkdir(path, 0o700); mkErr == nil || !errors.Is(mkErr, fs.ErrExist) {
			break
		}
	}
	if mkErr != nil {
		return "", fmt.Errorf("mkdir backup: %w", mkErr)
	}
	if _, err := os.Stat(dataDir); err == nil {
		if cpErr := copyTree(dataDir, path, IsEngineOwnedPath); cpErr != nil {
			_ = os.RemoveAll(path)
			return "", fmt.Errorf("copy backup: %w", cpErr)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		_ = os.RemoveAll(path)
		return "", fmt.Errorf("stat data dir: %w", err)
	}
	return path, nil
}

// restore replaces the contents of DataDir with the snapshot; see Restore.
func (s *HubService) restore(path string) error { return Restore(path, s.DataDir) }

// Restore replaces the contents of dataDir with the snapshot while keeping dataDir itself (and the
// engine-owned entries inside it) in place.
func Restore(snapshotPath, dataDir string) error {
	return RestoreKeeping(snapshotPath, dataDir, IsEngineOwnedPath)
}

// RestoreKeeping is Restore for callers that preserve more than engine-owned state: entries for which
// keep(rel) returns true are neither removed from dataDir nor overwritten from the snapshot.
func RestoreKeeping(snapshotPath, dataDir string, keep func(rel string) bool) error {
	if err := ClearConfigKeeping(dataDir, keep); err != nil {
		return err
	}
	if err := copyTree(snapshotPath, dataDir, keep); err != nil {
		return fmt.Errorf("restore backup: %w", err)
	}
	return nil
}

// ClearConfig creates dataDir when missing and removes everything in it except engine-owned state
// (live db files, top-level data/ and hub_cache/). dataDir itself is never removed or renamed.
func ClearConfig(dataDir string) error {
	return ClearConfigKeeping(dataDir, IsEngineOwnedPath)
}

// ClearConfigKeeping is ClearConfig with a caller-supplied keep predicate. keep is evaluated before
// the entry type is inspected, so a kept name survives whether it is a file, directory or symlink.
func ClearConfigKeeping(dataDir string, keep func(rel string) bool) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("mkdir data dir: %w", err)
	}
	if err := emptyDirExcept(dataDir, keep); err != nil {
		return fmt.Errorf("empty data dir: %w", err)
	}
	return nil
}

// emptyDirExcept removes the contents of dir except entries for which keep(relPath) returns true.
// Directories are removed only when nothing kept remains inside them.
func emptyDirExcept(dir string, keep func(rel string) bool) error {
	return emptyDirExceptRel(dir, "", keep)
}

func emptyDirExceptRel(dir, rel string, keep func(rel string) bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		entryRel := filepath.Join(rel, entry.Name())
		if keep(entryRel) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if err := emptyDirExceptRel(path, entryRel, keep); err != nil {
				return err
			}
			// A directory that still holds kept files cannot be removed; leave it in place.
			if remaining, rerr := os.ReadDir(path); rerr == nil && len(remaining) > 0 {
				continue
			}
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

// copyTree recursively copies src into dst (which must exist), skipping entries whose path relative
// to src makes skip return true. Symlinks are preserved literally and never followed.
func copyTree(src, dst string, skip func(rel string) bool) error {
	return copyTreeRel(src, dst, "", skip)
}

func copyTreeRel(src, dst, rel string, skip func(rel string) bool) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat src: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("src is not a directory")
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}
	for _, entry := range entries {
		entryRel := filepath.Join(rel, entry.Name())
		if skip != nil && skip(entryRel) {
			continue
		}
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		switch {
		case entry.Type()&fs.ModeSymlink != 0:
			// Hub item entries are symlinks, and dangling links must survive a backup/restore round trip.
			target, err := os.Readlink(srcPath)
			if err != nil {
				return fmt.Errorf("readlink %s: %w", srcPath, err)
			}
			if err := os.Symlink(target, dstPath); err != nil {
				return fmt.Errorf("symlink %s: %w", dstPath, err)
			}
		case entry.IsDir():
			if err := os.MkdirAll(dstPath, 0o700); err != nil {
				return fmt.Errorf("mkdir %s: %w", dstPath, err)
			}
			if err := copyTreeRel(srcPath, dstPath, entryRel, skip); err != nil {
				return err
			}
		default:
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// pruneAfterApply prunes full-tree snapshots, logging (never returning) any failure.
func (s *HubService) pruneAfterApply() {
	if removed, err := PruneBackups(s.DataDir, BackupKindSnapshot, DefaultBackupRetention); err != nil {
		logger.Log().WithError(err).WithField("removed", len(removed)).Warn("crowdsec backup prune incomplete")
	}
}

// backupTimestamp parses the timestamp suffix of a backup directory name.
func backupTimestamp(suffix string) (time.Time, bool) {
	for _, layout := range []string{backupTimeFormat, legacyBackupTimeFormat} {
		if ts, err := time.ParseInLocation(layout, suffix, time.Local); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}

// PruneBackups deletes the oldest <dataDir>.<kind>.<timestamp> sibling directories, keeping the
// newest `keep` (clamped to at least 1). Age comes from the timestamp in the directory name, never
// from file times. Entries that are not real directories or whose names do not parse are ignored.
// Individual removal errors are aggregated and returned alongside the paths that were removed.
func PruneBackups(dataDir, kind string, keep int) (removed []string, err error) {
	if keep < 1 {
		keep = 1
	}
	parent, base := filepath.Split(filepath.Clean(dataDir))
	if parent == "" {
		parent = "."
	}
	prefix := base + "." + kind + "."

	entries, readErr := os.ReadDir(parent)
	if readErr != nil {
		if errors.Is(readErr, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("list backups: %w", readErr)
	}

	type candidate struct {
		path string
		ts   time.Time
	}
	var found []candidate
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		ts, ok := backupTimestamp(strings.TrimPrefix(name, prefix))
		if !ok {
			continue
		}
		path := filepath.Join(parent, name)
		info, lerr := os.Lstat(path)
		if lerr != nil || !info.IsDir() {
			continue
		}
		found = append(found, candidate{path: path, ts: ts})
	}
	sort.Slice(found, func(i, j int) bool {
		if !found[i].ts.Equal(found[j].ts) {
			return found[i].ts.After(found[j].ts)
		}
		return found[i].path > found[j].path
	})
	if len(found) <= keep {
		return nil, nil
	}

	var errs []error
	for _, c := range found[keep:] {
		if rmErr := os.RemoveAll(c.path); rmErr != nil {
			logger.Log().WithError(rmErr).WithField("path", util.SanitizeForLog(c.path)).Warn("failed to prune crowdsec backup")
			errs = append(errs, rmErr)
			continue
		}
		removed = append(removed, c.path)
	}
	return removed, errors.Join(errs...)
}

// BackupID returns the client-safe name of a backup: the final path element, never the absolute
// location. It covers the directory kinds (*.backup.<ts>, *.filebackup.<ts>) and file backups
// (acquis.yaml.backup.<ts>); an empty path yields an empty name.
func BackupID(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}

// rollbackFailure annotates cause with a failed restore; the snapshot location is logged, never returned.
func rollbackFailure(cause, restoreErr error, backupPath string) error {
	logger.Log().WithError(restoreErr).WithField("backup_path", util.SanitizeForLog(backupPath)).Error("preset rollback failed; backup retained for manual recovery")
	return fmt.Errorf("%w (rollback failed; backup retained, see server logs)", cause)
}

// BackupFile copies the single file dataDir/rel into a new <dataDir>.filebackup.<ts>/<rel> directory
// and returns that directory, then prunes older file backups. A target that does not exist yet has
// nothing to back up and yields an empty path. dataDir is never renamed or modified.
func BackupFile(dataDir, rel string) (string, error) {
	src := filepath.Join(dataDir, rel)
	info, err := os.Stat(src)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("stat file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("backup file: %s is not a regular file", rel)
	}

	parent, base := filepath.Split(filepath.Clean(dataDir))
	var dir string
	var mkErr error
	for range snapshotMkdirAttempts {
		dir = filepath.Join(parent, base+"."+BackupKindFile+"."+time.Now().Format(backupTimeFormat))
		if mkErr = os.Mkdir(dir, 0o700); mkErr == nil || !errors.Is(mkErr, fs.ErrExist) {
			break
		}
	}
	if mkErr != nil {
		return "", fmt.Errorf("mkdir file backup: %w", mkErr)
	}
	dst := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("mkdir file backup path: %w", err)
	}
	if err := copyFile(src, dst); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("copy file backup: %w", err)
	}
	if _, pruneErr := PruneBackups(dataDir, BackupKindFile, DefaultFileBackupRetention); pruneErr != nil {
		logger.Log().WithError(pruneErr).Warn("crowdsec file backup prune incomplete")
	}
	return dir, nil
}

// absPathPattern matches an absolute filesystem path that starts the string or follows whitespace,
// a quote, or one of ( [ { < = ,. URL path segments are preceded by ":" or a word character
// and are therefore left alone. A path ends at whitespace, a quote, or one of : ; , ) ] } > <.
var absPathPattern = regexp.MustCompile(`(^|[\s(\[{<=,'"])(/[^\s:;,)\]}>'"<]+)`)

// apiRoutePrefix marks route-looking text (for example " /api/v1/x") that RedactPaths must not
// rewrite, since API routes are not filesystem paths.
const apiRoutePrefix = "/api/"

// RedactPaths replaces absolute filesystem paths in an error message with "<path>" so server
// layout never reaches an API client. Detail belongs in server logs.
//
// Limits: this is a heuristic over free text. Any absolute path in the recognised positions is
// redacted except those under /api/, so a bare route outside /api/ is also redacted (safe
// direction). Paths containing spaces or ":", Windows-style paths, and relative paths are not
// recognised; callers must not rely on it for those, and should prefer fixed messages.
func RedactPaths(msg string) string {
	return absPathPattern.ReplaceAllStringFunc(msg, func(m string) string {
		sub := absPathPattern.FindStringSubmatch(m)
		if strings.HasPrefix(sub[2], apiRoutePrefix) {
			return m
		}
		return sub[1] + "<path>"
	})
}

// withoutPath unwraps a filesystem error to its underlying cause (for example "permission
// denied"), dropping the absolute path that *fs.PathError and *os.LinkError embed.
func withoutPath(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return linkErr.Err
	}
	return err
}
