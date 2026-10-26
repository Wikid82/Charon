package crowdsec

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

const (
	// DefaultBackupRetention is how many snapshots (<DataDir>.backup.*) are kept.
	DefaultBackupRetention = 5
	// DefaultFileBackupRetention is how many single-file edit backups (<DataDir>.filebackup.*) are kept.
	DefaultFileBackupRetention = 10
	// MaxRetainedSnapshots is how many snapshots from failed restores keep their retain marker once the
	// oldest of them are past RetainedSnapshotMinAge. See capRetained.
	MaxRetainedSnapshots = 3
	// RetainedSnapshotMinAge is the floor: a kept snapshot younger than this is never un-marked.
	RetainedSnapshotMinAge = 7 * 24 * time.Hour

	// BackupKindSnapshot and BackupKindFile name the two independent backup namespaces next to DataDir.
	BackupKindSnapshot = "backup"
	BackupKindFile     = "filebackup"

	// backupTimeFormat includes sub-second precision so back-to-back backups never share a directory.
	backupTimeFormat = "20060102-150405.000000"
	// legacyBackupTimeFormat is the second-resolution layout older releases used.
	legacyBackupTimeFormat = "20060102-150405"

	// snapshotMkdirAttempts bounds retries when a microsecond-named directory already exists.
	snapshotMkdirAttempts = 3

	// retainMarkerSuffix names the marker that keeps a snapshot after a failed rollback. The marker sits
	// beside the snapshot, never inside it, so a restore can never copy it back into DataDir.
	retainMarkerSuffix = ".keep"
	retainMarkerText   = "kept for manual recovery after a failed rollback. It is protected for 7 days. After that only the newest 3 kept snapshots keep this marker, and an older one that loses it becomes eligible for normal cleanup. Delete this file to release this snapshot for cleanup now.\n"
)

// isLiveDBFile reports whether name is the live CrowdSec SQLite database or its WAL/SHM sidecars.
// These are owned by the running engine and must never be copied into, or restored from, a backup.
// Matching is case-insensitive.
func isLiveDBFile(name string) bool {
	switch strings.ToLower(name) {
	case "crowdsec.db", "crowdsec.db-wal", "crowdsec.db-shm":
		return true
	}
	return false
}

// IsEngineOwnedPath reports whether rel (a slash- or OS-separated path relative to DataDir) is state
// the running engine owns and that snapshots, restores and archive imports must never touch: a live
// db file at any depth, the top-level crowdsec.pid (written by the executor, owned by the running
// process), or anything under the top-level data/ (LAPI db and hub data files), hub_cache/ (regenerable
// download cache) or the snapshot sidecar directory (only a light snapshot writes it, so a configuration
// holding such an entry can never pass for one). Only the top-level directories are excluded; a nested
// directory with the same name stays part of the configuration. Matching is case-insensitive.
func IsEngineOwnedPath(rel string) bool {
	rel = strings.ToLower(filepath.ToSlash(filepath.Clean(rel)))
	if rel == "." || rel == "" {
		return false
	}
	if isLiveDBFile(filepath.Base(rel)) {
		return true
	}
	top, _, _ := strings.Cut(rel, "/")
	return top == "data" || top == "hub_cache" || top == snapshotSidecarDir || rel == "crowdsec.pid"
}

// snapshot copies DataDir into a new <DataDir>.backup.<ts> directory; see Snapshot.
func (s *HubService) snapshot() (string, error) { return Snapshot(s.DataDir) }

// snapshotWithoutHub is the snapshot of a hub apply: the hub tree is described, not copied; see snapshotWithoutHub.
func (s *HubService) snapshotWithoutHub(item itemState) (string, error) {
	return snapshotWithoutHub(s.DataDir, item)
}

// Snapshot copies dataDir (symlinks preserved, engine-owned state excluded) into a new
// <dataDir>.backup.<ts> directory and returns its path. dataDir is never modified.
func Snapshot(dataDir string) (string, error) {
	return snapshotTree(dataDir, IsEngineOwnedPath, nil)
}

// snapshotTree creates the <dataDir>.backup.<ts> directory, copies dataDir into it except the entries skip
// selects, and then lets extra add to it. The directory is removed again when any step fails.
func snapshotTree(dataDir string, skip func(rel string) bool, extra func(path string) error) (string, error) {
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
		if cpErr := copyTree(dataDir, path, skip); cpErr != nil {
			_ = os.RemoveAll(path)
			return "", fmt.Errorf("copy backup: %w", cpErr)
		}
		if extra != nil {
			if exErr := extra(path); exErr != nil {
				_ = os.RemoveAll(path)
				return "", fmt.Errorf("copy backup: %w", exErr)
			}
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
// engine-owned entries inside it) in place. A snapshot taken without the hub tree (see snapshotWithoutHub)
// is restored the light way: the hub tree stays and is trimmed back to what it held.
func Restore(snapshotPath, dataDir string) error {
	if isLightSnapshot(snapshotPath) {
		return restoreWithoutHub(snapshotPath, dataDir)
	}
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

// pruneAfterApply prunes snapshots, logging (never returning) any failure.
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
	return PruneBackupsProtecting(dataDir, kind, keep, nil)
}

// PruneBackupsProtecting is PruneBackups that never deletes the directories in protect (compared by cleaned
// path); they do not count against keep, so a snapshot a job still needs survives however old it is.
func PruneBackupsProtecting(dataDir, kind string, keep int, protect []string) (removed []string, err error) {
	return pruneSiblings(filepath.Clean(dataDir)+"."+kind, keep, fs.FileInfo.IsDir, protect...)
}

// BackupFilePath names the single-file backup of path taken at at: <path>.backup.<timestamp>, with a
// microsecond timestamp. PruneBackupFiles recognises exactly this naming.
func BackupFilePath(path string, at time.Time) string {
	return backupFileStem(path) + "." + at.Format(backupTimeFormat)
}

// backupFileStem is the shared prefix (without the trailing dot) of the single-file backups of path.
func backupFileStem(path string) string {
	return filepath.Clean(path) + "." + BackupKindSnapshot
}

// PruneBackupFiles is PruneBackups for single-file backups named <path>.backup.<timestamp> (for example
// the acquisition config rename-backups): only regular files are considered, the newest `keep` survive.
func PruneBackupFiles(path string, keep int) (removed []string, err error) {
	return pruneSiblings(backupFileStem(path), keep, func(info fs.FileInfo) bool { return info.Mode().IsRegular() })
}

// pruneSiblings removes all but the newest `keep` entries named <stem>.<timestamp> that wanted accepts;
// stem is a cleaned path without the trailing dot. See PruneBackups for the ordering and error contract.
func pruneSiblings(stem string, keep int, wanted func(fs.FileInfo) bool, protect ...string) (removed []string, err error) {
	if keep < 1 {
		keep = 1
	}
	parent, base := filepath.Split(stem)
	if parent == "" {
		parent = "."
	}
	prefix := base + "."

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
		if target, isMarker := strings.CutSuffix(strings.TrimPrefix(name, prefix), retainMarkerSuffix); isMarker {
			if _, ok := backupTimestamp(target); ok {
				pruneOrphanRetainMarker(filepath.Join(parent, name), filepath.Join(parent, prefix+target))
			}
			continue
		}
		ts, ok := backupTimestamp(strings.TrimPrefix(name, prefix))
		if !ok {
			continue
		}
		path := filepath.Join(parent, name)
		if slices.Contains(protect, path) || isRetained(path) {
			continue
		}
		info, lerr := os.Lstat(path)
		if lerr != nil || !wanted(info) {
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

// RetainSnapshot writes the retain marker beside the snapshot directory at backupPath, so pruning never
// deletes it while the marker exists. A missing path or a symlink has nothing to keep and yields nil, and
// retaining an already retained snapshot is not an error. A marker that cannot be written is an error.
func RetainSnapshot(backupPath string) error {
	info, err := os.Lstat(backupPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat snapshot: %w", err)
	}
	if !info.IsDir() {
		return nil
	}
	marker := backupPath + retainMarkerSuffix
	f, err := openRetainMarker(marker)
	if err != nil {
		if errors.Is(err, fs.ErrExist) && isRetained(backupPath) {
			return nil
		}
		return fmt.Errorf("create retain marker: %w", err)
	}
	if _, err := f.Write([]byte(retainMarkerText)); err != nil {
		_ = f.Close()
		_ = os.Remove(marker)
		return fmt.Errorf("write retain marker: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(marker)
		return fmt.Errorf("close retain marker: %w", err)
	}
	capRetained(backupPath, time.Now())
	return nil
}

// removeRetainMarker deletes a retain marker. It is a seam so tests can make removing one fail.
var removeRetainMarker = os.Remove

// capRetained keeps at most MaxRetainedSnapshots snapshots of backupPath's stem marked, by removing the
// retain marker (never the snapshot) of the oldest ones, oldest first. Age comes from the timestamp in the
// name, never from file times. The rules:
//   - a kept snapshot younger than RetainedSnapshotMinAge is never un-marked, even when more than
//     MaxRetainedSnapshots are kept: the floor wins over the cap;
//   - backupPath itself is protected by identity (its cleaned path), never by timestamp order, so a clock
//     step cannot un-mark the snapshot that was just retained;
//   - an un-marked snapshot is not deleted here; PruneBackups removes it at the next cleanup;
//   - a marker that cannot be removed is logged and the next eligible one is tried; a marker that is already
//     gone (removed by an admin) counts as removed.
//
// Invariant: every caller is serialised. The sync apply and the import hold dataMu, the async apply holds the
// job busy flag, and boot recovery runs before routes exist. So two capRetained calls never run at once, and
// the listing read here cannot change under it, except by an admin removing a marker by hand, which is harmless.
func capRetained(backupPath string, now time.Time) {
	clean := filepath.Clean(backupPath)
	dir, name := filepath.Split(clean)
	if dir == "" {
		dir = "."
	}
	idx := strings.LastIndex(name, "."+BackupKindSnapshot+".")
	if idx < 0 {
		return
	}
	// The stem with its trailing dot, "<base>.backup.": only snapshots of this stem are counted.
	prefix := name[:idx+len(BackupKindSnapshot)+2]

	entries, err := os.ReadDir(dir)
	if err != nil {
		logger.Log().WithError(err).WithField("backup_path", util.SanitizeForLog(clean)).Warn("could not list kept crowdsec snapshots; the retain cap was not applied")
		return
	}
	type keptSnapshot struct {
		path string
		ts   time.Time
	}
	var kept []keptSnapshot
	for _, entry := range entries {
		entryName := entry.Name()
		if !entry.IsDir() || !strings.HasPrefix(entryName, prefix) {
			continue
		}
		ts, ok := backupTimestamp(strings.TrimPrefix(entryName, prefix))
		if !ok {
			continue
		}
		if path := filepath.Join(dir, entryName); isRetained(path) {
			kept = append(kept, keptSnapshot{path: path, ts: ts})
		}
	}
	if len(kept) <= MaxRetainedSnapshots {
		return
	}
	sort.Slice(kept, func(i, j int) bool {
		if !kept[i].ts.Equal(kept[j].ts) {
			return kept[i].ts.Before(kept[j].ts)
		}
		return kept[i].path < kept[j].path
	})

	excess := len(kept) - MaxRetainedSnapshots
	for _, k := range kept {
		if excess == 0 {
			break
		}
		if k.path == clean {
			continue
		}
		// kept is oldest first, so the first snapshot inside the floor means every later one is inside it too.
		if now.Sub(k.ts) < RetainedSnapshotMinAge {
			break
		}
		marker := k.path + retainMarkerSuffix
		if err := removeRetainMarker(marker); err != nil && !errors.Is(err, fs.ErrNotExist) {
			logger.Log().WithError(err).WithField("backup_id", BackupID(k.path)).Warn("could not un-mark kept crowdsec snapshot beyond the retention cap")
			continue
		}
		excess--
		logger.Log().WithField("backup_id", BackupID(k.path)).Info("un-marked kept crowdsec snapshot beyond the retention cap; automatic cleanup removes it")
	}
}

// openRetainMarker creates a retain marker exclusively. It is a seam so tests can make writing or closing it fail.
var openRetainMarker = func(name string) (io.WriteCloser, error) {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // G304: the marker is the snapshot path plus a fixed suffix, both set by the server
	if err != nil {
		return nil, err
	}
	return f, nil
}

// RetainForRecovery is RetainSnapshot for failure paths that must not change their own error: a marker that
// cannot be written is logged with the sanitised backup path.
func RetainForRecovery(backupPath string) {
	if err := RetainSnapshot(backupPath); err != nil {
		logger.Log().WithError(err).WithField("backup_path", util.SanitizeForLog(backupPath)).Error("could not keep the backup beyond automatic cleanup")
	}
}

// isRetained reports whether path has a regular retain marker beside it.
func isRetained(path string) bool {
	info, err := os.Lstat(path + retainMarkerSuffix)
	return err == nil && info.Mode().IsRegular()
}

// pruneOrphanRetainMarker removes a retain marker whose snapshot was deleted, for example by an admin.
// Failures are logged at debug level only: a stray marker is harmless and is retried by the next prune.
func pruneOrphanRetainMarker(marker, target string) {
	if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err := os.Remove(marker); err != nil {
		logger.Log().WithError(err).WithField("path", util.SanitizeForLog(marker)).Debug("failed to remove orphaned crowdsec backup marker")
	}
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

// ErrRollbackFailed marks a failed apply whose rollback failed too: the snapshot is retained for manual recovery.
var ErrRollbackFailed = errors.New("rollback failed; backup retained, see server logs")

// rollbackFailure annotates cause with a failed restore; the snapshot location is logged, never returned.
func rollbackFailure(cause, restoreErr error, backupPath string) error {
	logger.Log().WithError(restoreErr).WithField("backup_path", util.SanitizeForLog(backupPath)).Error("preset rollback failed; backup retained for manual recovery")
	RetainForRecovery(backupPath)
	return fmt.Errorf("%w (%w)", cause, ErrRollbackFailed)
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
