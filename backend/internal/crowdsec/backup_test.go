package crowdsec

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, err)
	return string(b)
}

func dirSize(t *testing.T, dir string) int64 {
	t.Helper()
	var total int64
	require.NoError(t, filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	}))
	return total
}

// seedPersistentTree mimics /app/data/crowdsec: config, bouncer key, engine-owned state.
func seedPersistentTree(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "crowdsec")
	writeFile(t, filepath.Join(dir, "config", "config.yaml"), "original: true\n")
	writeFile(t, filepath.Join(dir, "config", "data", "nested.yaml"), "nested: true\n")
	writeFile(t, filepath.Join(dir, "bouncer_key"), "secret")
	writeFile(t, filepath.Join(dir, "crowdsec.db"), "live-db")
	writeFile(t, filepath.Join(dir, "crowdsec.db-wal"), "wal")
	writeFile(t, filepath.Join(dir, "crowdsec.db-shm"), "shm")
	writeFile(t, filepath.Join(dir, "config", "crowdsec.db"), "nested-db")
	writeFile(t, filepath.Join(dir, "data", "crowdsec.db"), "lapi-db")
	writeFile(t, filepath.Join(dir, "data", "GeoLite2-City.mmdb"), "mmdb")
	writeFile(t, filepath.Join(dir, "hub_cache", "test", "bundle.tgz"), "archive")
	require.NoError(t, os.Symlink("../hub/x.yaml", filepath.Join(dir, "config", "link.yaml")))
	return dir
}

func TestIsEngineOwnedPath(t *testing.T) {
	t.Parallel()
	owned := []string{"crowdsec.db", "crowdsec.db-wal", "crowdsec.db-shm", "config/crowdsec.db", "data", "data/x", "data/a/b", "hub_cache", "hub_cache/y", "./data/x"}
	for _, p := range owned {
		require.True(t, IsEngineOwnedPath(p), p)
	}
	notOwned := []string{"", ".", "config/data", "config/data/x", "config/hub_cache", "config.yaml", "database/x", "datax", "bouncer_key"}
	for _, p := range notOwned {
		require.False(t, IsEngineOwnedPath(p), p)
	}
}

func TestSnapshotExcludesEngineOwnedState(t *testing.T) {
	t.Parallel()
	dir := seedPersistentTree(t)
	svc := NewHubService(nil, nil, dir)

	path, err := svc.snapshot()
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(path, dir+".backup."), path)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm())

	require.Equal(t, "original: true\n", readFile(t, filepath.Join(path, "config", "config.yaml")))
	require.Equal(t, "secret", readFile(t, filepath.Join(path, "bouncer_key")))
	// A nested directory named data stays part of the configuration.
	require.Equal(t, "nested: true\n", readFile(t, filepath.Join(path, "config", "data", "nested.yaml")))

	for _, excluded := range []string{"crowdsec.db", "crowdsec.db-wal", "crowdsec.db-shm", filepath.Join("config", "crowdsec.db"), "data", "hub_cache"} {
		_, statErr := os.Lstat(filepath.Join(path, excluded))
		require.True(t, os.IsNotExist(statErr), "%s must not be in the snapshot", excluded)
	}

	target, err := os.Readlink(filepath.Join(path, "config", "link.yaml"))
	require.NoError(t, err)
	require.Equal(t, "../hub/x.yaml", target)

	// The live tree is unchanged.
	require.Equal(t, "live-db", readFile(t, filepath.Join(dir, "crowdsec.db")))
}

func TestSnapshotSizeIgnoresLargeEngineOwnedFiles(t *testing.T) {
	t.Parallel()
	dir := seedPersistentTree(t)
	big := bytes.Repeat([]byte("x"), 4<<20)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data", "big.bin"), big, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hub_cache", "big.bin"), big, 0o600))
	svc := NewHubService(nil, nil, dir)

	path, err := svc.snapshot()
	require.NoError(t, err)
	require.Less(t, dirSize(t, path), int64(1<<10), "snapshot must only hold the small config files")
}

func TestSnapshotMissingDataDirYieldsEmptySnapshot(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "crowdsec")
	svc := NewHubService(nil, nil, dir)

	path, err := svc.snapshot()
	require.NoError(t, err)
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	require.Empty(t, entries)
	require.NoDirExists(t, dir, "snapshot must not create DataDir")
}

func TestSnapshotErrors(t *testing.T) {
	t.Parallel()
	t.Run("backup directory cannot be created", func(t *testing.T) {
		t.Parallel()
		svc := NewHubService(nil, nil, filepath.Join(t.TempDir(), "missing-parent", "crowdsec"))
		_, err := svc.snapshot()
		require.ErrorContains(t, err, "mkdir backup")
	})
	t.Run("data dir is not a directory", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join(t.TempDir(), "crowdsec")
		writeFile(t, dir, "x")
		svc := NewHubService(nil, nil, dir)
		_, err := svc.snapshot()
		require.ErrorContains(t, err, "copy backup")
		matches, globErr := filepath.Glob(dir + ".backup.*")
		require.NoError(t, globErr)
		require.Empty(t, matches, "partial snapshot removed")
	})
	t.Run("data dir cannot be inspected", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join(t.TempDir(), "crowdsec")
		writeFile(t, dir, "x")
		// A path through a regular file yields ENOTDIR, which is not "does not exist".
		svc := NewHubService(nil, nil, filepath.Join(dir, "child"))
		_, err := svc.snapshot()
		require.Error(t, err)
	})
}

func TestSnapshotNamesAreUnique(t *testing.T) {
	t.Parallel()
	dir := seedPersistentTree(t)
	svc := NewHubService(nil, nil, dir)
	seen := map[string]bool{}
	for range 5 {
		path, err := svc.snapshot()
		require.NoError(t, err)
		require.False(t, seen[path], "duplicate snapshot dir %s", path)
		seen[path] = true
	}
}

func TestRestoreReplacesConfigAndLeavesEngineOwnedStateUntouched(t *testing.T) {
	t.Parallel()
	dir := seedPersistentTree(t)
	svc := NewHubService(nil, nil, dir)
	before, err := os.Stat(dir)
	require.NoError(t, err)

	path, err := svc.snapshot()
	require.NoError(t, err)

	// Simulate a failed apply: config changed, files added, link replaced, engine state moved on.
	writeFile(t, filepath.Join(dir, "config", "config.yaml"), "original: false\n")
	writeFile(t, filepath.Join(dir, "config", "added.yaml"), "new")
	writeFile(t, filepath.Join(dir, "added-top.yaml"), "new")
	require.NoError(t, os.Remove(filepath.Join(dir, "config", "link.yaml")))
	writeFile(t, filepath.Join(dir, "config", "link.yaml"), "dereferenced")
	writeFile(t, filepath.Join(dir, "crowdsec.db"), "newer-live-db")
	writeFile(t, filepath.Join(dir, "data", "crowdsec.db"), "newer-lapi-db")
	writeFile(t, filepath.Join(dir, "hub_cache", "test", "bundle.tgz"), "newer-archive")

	require.NoError(t, svc.restore(path))

	require.Equal(t, "original: true\n", readFile(t, filepath.Join(dir, "config", "config.yaml")))
	require.Equal(t, "nested: true\n", readFile(t, filepath.Join(dir, "config", "data", "nested.yaml")))
	require.Equal(t, "secret", readFile(t, filepath.Join(dir, "bouncer_key")))
	for _, gone := range []string{filepath.Join("config", "added.yaml"), "added-top.yaml"} {
		_, statErr := os.Lstat(filepath.Join(dir, gone))
		require.True(t, os.IsNotExist(statErr), gone)
	}
	target, err := os.Readlink(filepath.Join(dir, "config", "link.yaml"))
	require.NoError(t, err)
	require.Equal(t, "../hub/x.yaml", target)

	require.Equal(t, "newer-live-db", readFile(t, filepath.Join(dir, "crowdsec.db")))
	require.Equal(t, "wal", readFile(t, filepath.Join(dir, "crowdsec.db-wal")))
	require.Equal(t, "nested-db", readFile(t, filepath.Join(dir, "config", "crowdsec.db")))
	require.Equal(t, "newer-lapi-db", readFile(t, filepath.Join(dir, "data", "crowdsec.db")))
	require.Equal(t, "mmdb", readFile(t, filepath.Join(dir, "data", "GeoLite2-City.mmdb")))
	require.Equal(t, "newer-archive", readFile(t, filepath.Join(dir, "hub_cache", "test", "bundle.tgz")))

	after, err := os.Stat(dir)
	require.NoError(t, err)
	require.True(t, os.SameFile(before, after), "DataDir must stay in place")
}

func TestRestoreRecreatesMissingDataDir(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "crowdsec")
	svc := NewHubService(nil, nil, dir)
	path, err := svc.snapshot()
	require.NoError(t, err)
	writeFile(t, filepath.Join(dir, "partial.yaml"), "x")

	require.NoError(t, svc.restore(path))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestRestoreErrors(t *testing.T) {
	t.Parallel()
	t.Run("data dir cannot be created", func(t *testing.T) {
		t.Parallel()
		blocker := filepath.Join(t.TempDir(), "blocker")
		writeFile(t, blocker, "x")
		err := Restore(t.TempDir(), filepath.Join(blocker, "crowdsec"))
		require.ErrorContains(t, err, "mkdir data dir")
	})
	t.Run("snapshot is missing", func(t *testing.T) {
		t.Parallel()
		dir := seedPersistentTree(t)
		err := Restore(filepath.Join(t.TempDir(), "gone"), dir)
		require.ErrorContains(t, err, "restore backup")
	})
	t.Run("data dir is a file", func(t *testing.T) {
		t.Parallel()
		file := filepath.Join(t.TempDir(), "file")
		writeFile(t, file, "x")
		require.Error(t, Restore(t.TempDir(), file))
	})
}

func TestEmptyDirExceptKeepsMatchesByRelativePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "keep", "a.txt"), "a")
	writeFile(t, filepath.Join(dir, "drop", "b.txt"), "b")
	writeFile(t, filepath.Join(dir, "mixed", "keep.txt"), "k")
	writeFile(t, filepath.Join(dir, "mixed", "drop.txt"), "d")

	keep := func(rel string) bool { return rel == "keep" || rel == filepath.Join("mixed", "keep.txt") }
	require.NoError(t, emptyDirExcept(dir, keep))

	require.FileExists(t, filepath.Join(dir, "keep", "a.txt"))
	require.FileExists(t, filepath.Join(dir, "mixed", "keep.txt"))
	require.NoFileExists(t, filepath.Join(dir, "mixed", "drop.txt"))
	require.NoDirExists(t, filepath.Join(dir, "drop"))
}

func TestCopyTreeSkipUsesRelativePath(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, filepath.Join(src, "skipme", "a"), "a")
	writeFile(t, filepath.Join(src, "sub", "skipme", "b"), "b")

	require.NoError(t, copyTree(src, dst, func(rel string) bool { return rel == "skipme" }))
	require.NoDirExists(t, filepath.Join(dst, "skipme"))
	require.FileExists(t, filepath.Join(dst, "sub", "skipme", "b"))
}

// mkBackupDir creates a sibling backup directory named after dataDir/kind/suffix.
func mkBackupDir(t *testing.T, dataDir, kind, suffix string) string {
	t.Helper()
	path := fmt.Sprintf("%s.%s.%s", dataDir, kind, suffix)
	require.NoError(t, os.Mkdir(path, 0o700))
	return path
}

func TestPruneBackupsOrdersByNameTimestampNotMtime(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	require.NoError(t, os.Mkdir(dataDir, 0o700))
	oldest := mkBackupDir(t, dataDir, "backup", "20250101-000000.000001")
	mid := mkBackupDir(t, dataDir, "backup", "20250102-000000.000000")
	newest := mkBackupDir(t, dataDir, "backup", "20250103-000000.000000")

	// Touching the oldest directory must not save it.
	future := time.Now().Add(time.Hour)
	require.NoError(t, os.Chtimes(oldest, future, future))

	removed, err := PruneBackups(dataDir, BackupKindSnapshot, 2)
	require.NoError(t, err)
	require.Equal(t, []string{oldest}, removed)
	require.NoDirExists(t, oldest)
	require.DirExists(t, mid)
	require.DirExists(t, newest)
	require.DirExists(t, dataDir)
}

func TestPruneBackupsHandlesBothNameLayouts(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	legacyOld := mkBackupDir(t, dataDir, "backup", "20250101-120000")
	legacyNew := mkBackupDir(t, dataDir, "backup", "20250101-120005")
	micro := mkBackupDir(t, dataDir, "backup", "20250101-120005.500000")

	removed, err := PruneBackups(dataDir, BackupKindSnapshot, 2)
	require.NoError(t, err)
	require.Equal(t, []string{legacyOld}, removed)
	require.DirExists(t, legacyNew)
	require.DirExists(t, micro)
}

func TestPruneBackupsIgnoresUnparsableAndNonMatchingEntries(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	dataDir := filepath.Join(parent, "crowdsec")
	require.NoError(t, os.Mkdir(dataDir, 0o700))
	newest := mkBackupDir(t, dataDir, "backup", "20250103-000000.000000")
	old := mkBackupDir(t, dataDir, "backup", "20250101-000000.000000")

	unparsable := mkBackupDir(t, dataDir, "backup", "manual-copy")
	otherBase := filepath.Join(parent, "other.backup.20240101-000000.000000")
	require.NoError(t, os.Mkdir(otherBase, 0o700))
	plainFile := dataDir + ".backup.20240102-000000.000000"
	require.NoError(t, os.WriteFile(plainFile, []byte("x"), 0o600))

	linkTarget := t.TempDir()
	symlink := dataDir + ".backup.20240103-000000.000000"
	require.NoError(t, os.Symlink(linkTarget, symlink))

	removed, err := PruneBackups(dataDir, BackupKindSnapshot, 1)
	require.NoError(t, err)
	require.Equal(t, []string{old}, removed)
	require.DirExists(t, newest)
	for _, kept := range []string{unparsable, otherBase, plainFile, symlink, linkTarget, dataDir} {
		_, statErr := os.Lstat(kept)
		require.NoError(t, statErr, kept)
	}
}

func TestPruneBackupsClampsKeepToOne(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	old := mkBackupDir(t, dataDir, "backup", "20250101-000000.000000")
	newest := mkBackupDir(t, dataDir, "backup", "20250102-000000.000000")

	for _, keep := range []int{0, -3} {
		removed, err := PruneBackups(dataDir, BackupKindSnapshot, keep)
		require.NoError(t, err)
		if keep == 0 {
			require.Equal(t, []string{old}, removed)
		} else {
			require.Empty(t, removed)
		}
		require.DirExists(t, newest, "the newest backup is always kept")
	}
}

func TestPruneBackupsEqualTimestampsTieBreakDeterministically(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	// The legacy and microsecond layouts can name the same instant.
	a := mkBackupDir(t, dataDir, "backup", "20250101-000000")
	b := mkBackupDir(t, dataDir, "backup", "20250101-000000.000000")

	removed, err := PruneBackups(dataDir, BackupKindSnapshot, 1)
	require.NoError(t, err)
	require.Equal(t, []string{a}, removed)
	require.DirExists(t, b)
}

func TestPruneBackupsMissingParentAndNothingToPrune(t *testing.T) {
	t.Parallel()
	removed, err := PruneBackups(filepath.Join(t.TempDir(), "gone", "crowdsec"), BackupKindSnapshot, 5)
	require.NoError(t, err)
	require.Empty(t, removed)

	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	mkBackupDir(t, dataDir, "backup", "20250101-000000.000000")
	removed, err = PruneBackups(dataDir, BackupKindSnapshot, 5)
	require.NoError(t, err)
	require.Empty(t, removed)
}

func TestPruneBackupsParentIsNotADirectory(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "file")
	writeFile(t, file, "x")
	_, err := PruneBackups(filepath.Join(file, "crowdsec"), BackupKindSnapshot, 5)
	require.ErrorContains(t, err, "list backups")
}

func TestPruneBackupsRelativeDataDir(t *testing.T) {
	// Not parallel: changes the working directory.
	tmp := t.TempDir()
	t.Chdir(tmp)
	require.NoError(t, os.Mkdir("crowdsec.backup.20250101-000000.000000", 0o700))
	require.NoError(t, os.Mkdir("crowdsec.backup.20250102-000000.000000", 0o700))

	removed, err := PruneBackups("crowdsec", BackupKindSnapshot, 1)
	require.NoError(t, err)
	require.Len(t, removed, 1)
	require.NoDirExists(t, "crowdsec.backup.20250101-000000.000000")
}

func TestPruneBackupsNamespacesNeverEvictEachOther(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	var snaps, files []string
	for i := 1; i <= 7; i++ {
		suffix := fmt.Sprintf("2025010%d-000000.000000", i)
		snaps = append(snaps, mkBackupDir(t, dataDir, "backup", suffix))
		files = append(files, mkBackupDir(t, dataDir, "filebackup", suffix))
	}

	removed, err := PruneBackups(dataDir, BackupKindSnapshot, DefaultBackupRetention)
	require.NoError(t, err)
	require.ElementsMatch(t, snaps[:2], removed)
	for _, f := range files {
		require.DirExists(t, f, "file backups are not touched by snapshot pruning")
	}

	removed, err = PruneBackups(dataDir, BackupKindFile, 3)
	require.NoError(t, err)
	require.ElementsMatch(t, files[:4], removed)
	for _, s := range snaps[2:] {
		require.DirExists(t, s, "snapshots are not touched by file backup pruning")
	}
}

func TestPruneBackupsReportsRemovalErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission-based failure cannot be provoked as root")
	}
	parent := t.TempDir()
	dataDir := filepath.Join(parent, "crowdsec")
	old := mkBackupDir(t, dataDir, "backup", "20250101-000000.000000")
	mkBackupDir(t, dataDir, "backup", "20250102-000000.000000")
	// A read-only parent makes the removal fail.
	require.NoError(t, os.Chmod(parent, 0o500)) //nolint:gosec // G302: test needs a read-only dir
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })

	removed, err := PruneBackups(dataDir, BackupKindSnapshot, 1)
	require.Error(t, err)
	require.Empty(t, removed)
	_ = old
}

// ---- HubService.Apply flows ----

type funcExec func(ctx context.Context, cmd string) ([]byte, error)

func (f funcExec) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	return f(ctx, strings.Join(append([]string{name}, args...), " "))
}

// applyFixture builds a persistent-tree HubService whose cache lives inside DataDir/hub_cache,
// as in production, with one cached preset archive.
func applyFixture(t *testing.T, exec CommandExecutor, archive []byte) (svc *HubService, dir string) {
	t.Helper()
	dir = seedPersistentTree(t)
	cache, err := NewHubCache(filepath.Join(dir, "hub_cache"), time.Hour)
	require.NoError(t, err)
	if archive != nil {
		_, err = cache.Store(context.Background(), "test/preset", "etag", "hub", "preview", archive)
		require.NoError(t, err)
	}
	svc = NewHubService(exec, cache, dir)
	svc.ApplyTimeout = 5 * time.Second
	return svc, dir
}

func countBackups(t *testing.T, dir string) int {
	t.Helper()
	m, err := filepath.Glob(dir + ".backup.*")
	require.NoError(t, err)
	return len(m)
}

func TestApplyCSCLISuccessKeepsDataDirAndPrunes(t *testing.T) {
	t.Parallel()
	exec := funcExec(func(_ context.Context, _ string) ([]byte, error) { return []byte("ok"), nil })
	svc, dir := applyFixture(t, exec, nil)
	reloads := 0
	svc.Reload = func(context.Context) error { reloads++; return nil }
	before, err := os.Stat(dir)
	require.NoError(t, err)

	for range DefaultBackupRetention + 2 {
		res, applyErr := svc.Apply(context.Background(), "test/preset")
		require.NoError(t, applyErr)
		require.Equal(t, "applied", res.Status)
		require.True(t, res.UsedCSCLI)
		require.False(t, res.ReloadHint, "reload succeeded")
		require.DirExists(t, res.BackupPath)
	}

	after, err := os.Stat(dir)
	require.NoError(t, err)
	require.True(t, os.SameFile(before, after), "DataDir must never be renamed")
	require.Equal(t, DefaultBackupRetention, countBackups(t, dir))
	require.Equal(t, DefaultBackupRetention+2, reloads)
	require.Equal(t, "live-db", readFile(t, filepath.Join(dir, "crowdsec.db")))
}

func TestApplyCSCLIFailureRestoresThenExtractsCachedArchive(t *testing.T) {
	t.Parallel()
	var svc *HubService
	var dir string
	exec := funcExec(func(_ context.Context, cmd string) ([]byte, error) {
		switch {
		case strings.Contains(cmd, "hub install"):
			// cscli half-installs, then fails.
			writeFile(t, filepath.Join(dir, "config", "cscli-leftover.yaml"), "partial")
			writeFile(t, filepath.Join(dir, "config", "config.yaml"), "original: false\n")
			return nil, errors.New("install failed")
		default:
			return []byte("ok"), nil
		}
	})
	archive := makeTarGz(t, map[string]string{
		"config/from-archive.yaml": "archive: true",
		"crowdsec.db":              "evil-db",
		"data/evil":                "evil",
		"hub_cache/evil":           "evil",
	})
	svc, dir = applyFixture(t, exec, archive)

	res, err := svc.Apply(context.Background(), "test/preset")
	require.NoError(t, err)
	require.Equal(t, "applied", res.Status)
	require.False(t, res.UsedCSCLI)
	require.True(t, res.ReloadHint, "no reloader configured")

	require.Equal(t, "archive: true", readFile(t, filepath.Join(dir, "config", "from-archive.yaml")))
	require.Equal(t, "original: true\n", readFile(t, filepath.Join(dir, "config", "config.yaml")), "snapshot restored before extract")
	require.NoFileExists(t, filepath.Join(dir, "config", "cscli-leftover.yaml"))
	require.Equal(t, "live-db", readFile(t, filepath.Join(dir, "crowdsec.db")), "archive cannot overwrite engine state")
	require.NoFileExists(t, filepath.Join(dir, "data", "evil"))
	require.NoFileExists(t, filepath.Join(dir, "hub_cache", "evil"))
	require.FileExists(t, filepath.Join(dir, "hub_cache", "test", "preset", "bundle.tgz"), "cached archive survives the restore")
}

func TestApplyCSCLIAndExtractFailureRestoresSnapshot(t *testing.T) {
	t.Parallel()
	var dir string
	exec := funcExec(func(_ context.Context, cmd string) ([]byte, error) {
		if strings.Contains(cmd, "hub install") {
			writeFile(t, filepath.Join(dir, "config", "cscli-leftover.yaml"), "partial")
			return nil, errors.New("install failed")
		}
		return []byte("ok"), nil
	})
	// A symlink entry makes extraction fail after files were already written.
	svc, d := applyFixture(t, exec, makeTarGzWithSymlink(t))
	dir = d

	res, err := svc.Apply(context.Background(), "test/preset")
	require.ErrorContains(t, err, "extract")
	require.Equal(t, "failed", res.Status)
	require.NotEmpty(t, res.BackupPath)

	require.Equal(t, "original: true\n", readFile(t, filepath.Join(dir, "config", "config.yaml")))
	require.NoFileExists(t, filepath.Join(dir, "config", "cscli-leftover.yaml"))
	require.NoFileExists(t, filepath.Join(dir, "partial-before-symlink.yaml"))
	require.DirExists(t, res.BackupPath, "failed apply keeps its snapshot")
}

func TestApplyCacheMissAndRefreshFailureRestoresSnapshot(t *testing.T) {
	t.Parallel()
	var dir string
	exec := funcExec(func(_ context.Context, cmd string) ([]byte, error) {
		if strings.Contains(cmd, "hub install") {
			writeFile(t, filepath.Join(dir, "config", "cscli-leftover.yaml"), "partial")
			return nil, errors.New("install failed")
		}
		return []byte("ok"), nil
	})
	svc, d := applyFixture(t, exec, nil) // nothing cached
	dir = d
	svc.HubBaseURL = "http://127.0.0.1:1"
	svc.MirrorBaseURL = "http://127.0.0.1:1"

	res, err := svc.Apply(context.Background(), "test/preset")
	require.ErrorContains(t, err, "load cache")
	require.Equal(t, "failed", res.Status)
	require.NotEmpty(t, res.BackupPath)
	require.NotEmpty(t, res.ErrorMessage)
	require.NoFileExists(t, filepath.Join(dir, "config", "cscli-leftover.yaml"))
	require.Equal(t, "original: true\n", readFile(t, filepath.Join(dir, "config", "config.yaml")))
}

func TestApplyPrunesAfterFailureAndKeepsTheFailedSnapshot(t *testing.T) {
	t.Parallel()
	svc, dir := applyFixture(t, nil, nil) // no cscli, nothing cached: every apply fails
	svc.HubBaseURL = "http://127.0.0.1:1"
	svc.MirrorBaseURL = "http://127.0.0.1:1"
	for i := 1; i <= DefaultBackupRetention+3; i++ {
		mkBackupDir(t, dir, "backup", fmt.Sprintf("2025010%d-000000.000000", i))
	}

	res, err := svc.Apply(context.Background(), "test/preset")
	require.Error(t, err)
	require.DirExists(t, res.BackupPath, "the failed apply's snapshot is the newest and always kept")
	require.Equal(t, DefaultBackupRetention, countBackups(t, dir))
}

func TestApplyRestoreFailureIsReported(t *testing.T) {
	t.Parallel()
	var dir string
	var svc *HubService
	exec := funcExec(func(_ context.Context, cmd string) ([]byte, error) {
		if strings.Contains(cmd, "hub install") {
			// Make the restore itself fail: DataDir is replaced by a regular file mid-apply.
			require.NoError(t, os.RemoveAll(dir))
			writeFile(t, dir, "not a dir")
			return nil, errors.New("install failed")
		}
		return []byte("ok"), nil
	})
	svc, dir = applyFixture(t, exec, nil)

	res, err := svc.Apply(context.Background(), "test/preset")
	require.ErrorContains(t, err, "rollback failed")
	require.ErrorContains(t, err, "backup retained at "+res.BackupPath)
}

func TestApplySnapshotFailureChangesNothing(t *testing.T) {
	t.Parallel()
	called := false
	exec := funcExec(func(_ context.Context, _ string) ([]byte, error) { called = true; return []byte("ok"), nil })
	svc := NewHubService(exec, nil, filepath.Join(t.TempDir(), "missing-parent", "crowdsec"))

	res, err := svc.Apply(context.Background(), "test/preset")
	require.ErrorContains(t, err, "backup")
	require.Empty(t, res.BackupPath)
	require.False(t, called, "cscli must not run when no snapshot exists")
}

func TestApplyUnreadableCachedArchiveTriggersRefreshFailure(t *testing.T) {
	t.Parallel()
	svc, dir := applyFixture(t, nil, makeTarGz(t, map[string]string{"x.yaml": "x"}))
	svc.HubBaseURL = "http://127.0.0.1:1"
	svc.MirrorBaseURL = "http://127.0.0.1:1"
	require.NoError(t, os.Remove(filepath.Join(dir, "hub_cache", "test", "preset", "bundle.tgz")))

	_, err := svc.Apply(context.Background(), "test/preset")
	require.ErrorContains(t, err, "load cache")
}

func TestExtractTarGzOverlaysWithoutWipingAndSkipsEngineOwnedEntries(t *testing.T) {
	t.Parallel()
	dir := seedPersistentTree(t)
	svc := NewHubService(nil, nil, dir)
	archive := makeTarGz(t, map[string]string{
		"config/new.yaml": "new",
		"crowdsec.db":     "evil",
		"data/x":          "evil",
		"hub_cache/y":     "evil",
	})

	require.NoError(t, svc.extractTarGz(context.Background(), archive, dir))
	require.Equal(t, "new", readFile(t, filepath.Join(dir, "config", "new.yaml")))
	require.Equal(t, "original: true\n", readFile(t, filepath.Join(dir, "config", "config.yaml")), "existing files are not wiped")
	require.Equal(t, "live-db", readFile(t, filepath.Join(dir, "crowdsec.db")))
	require.NoFileExists(t, filepath.Join(dir, "data", "x"))
	require.NoFileExists(t, filepath.Join(dir, "hub_cache", "y"))
}

func TestRollbackFailureWrapsCause(t *testing.T) {
	t.Parallel()
	cause := errors.New("boom")
	err := rollbackFailure(cause, errors.New("restore broke"), "/b")
	require.ErrorIs(t, err, cause)
	require.ErrorContains(t, err, "restore broke")
	require.ErrorContains(t, err, "/b")
}

// makeTarGzWithSymlink returns an archive that writes one file and then hits a symlink entry.
func makeTarGzWithSymlink(t *testing.T) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)
	body := "partial"
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "partial-before-symlink.yaml", Mode: 0o644, Size: int64(len(body))}))
	_, err := tw.Write([]byte(body))
	require.NoError(t, err)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd", Mode: 0o777}))
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func TestBackupFileCopiesOnlyTheTargetAndPrunesIndependently(t *testing.T) {
	t.Parallel()
	dir := seedPersistentTree(t)
	snap := mkBackupDir(t, dir, "backup", "20240101-000000.000000")
	for i := 1; i <= DefaultFileBackupRetention; i++ {
		mkBackupDir(t, dir, "filebackup", fmt.Sprintf("202401%02d-000000.000000", i))
	}

	backup, err := BackupFile(dir, filepath.Join("config", "config.yaml"))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(backup, dir+".filebackup."), backup)
	require.Equal(t, "original: true\n", readFile(t, filepath.Join(backup, "config", "config.yaml")))
	require.Less(t, dirSize(t, backup), int64(100), "only the target file is copied")

	matches, err := filepath.Glob(dir + ".filebackup.*")
	require.NoError(t, err)
	require.Len(t, matches, DefaultFileBackupRetention)
	require.DirExists(t, snap, "full snapshots are not evicted by file backups")
	require.DirExists(t, dir)
}

func TestBackupFileEdgeCases(t *testing.T) {
	t.Parallel()
	dir := seedPersistentTree(t)

	backup, err := BackupFile(dir, "config/new-file.yaml")
	require.NoError(t, err)
	require.Empty(t, backup, "nothing to back up for a new file")

	_, err = BackupFile(dir, "config")
	require.ErrorContains(t, err, "not a regular file")

	_, err = BackupFile(dir, filepath.Join("bouncer_key", "x"))
	require.ErrorContains(t, err, "stat file")
}

func TestBackupFileMkdirFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission-based failure cannot be provoked as root")
	}
	parent := t.TempDir()
	dir := filepath.Join(parent, "crowdsec")
	writeFile(t, filepath.Join(dir, "f.yaml"), "x")
	require.NoError(t, os.Chmod(parent, 0o500)) //nolint:gosec // G302: test needs a read-only dir
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })

	_, err := BackupFile(dir, "f.yaml")
	require.ErrorContains(t, err, "mkdir file backup")
}
