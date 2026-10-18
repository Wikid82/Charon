package crowdsec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteSnapshotSidecarFailsWithoutASnapshotDirectory(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "crowdsec")

	err := writeSnapshotSidecar(dir, filepath.Join(t.TempDir(), "absent"), itemState{})

	require.ErrorContains(t, err, "mkdir snapshot sidecar")
}

func TestSnapshotWithoutHubFailsWhenTheHubTreeCannotBeListed(t *testing.T) {
	t.Parallel()
	skipIfRoot(t)
	dir := filepath.Join(t.TempDir(), "crowdsec")
	writeFile(t, filepath.Join(dir, "config", "config.yaml"), "x")
	blocked := filepath.Join(dir, "config", "hub", "collections")
	writeFile(t, filepath.Join(blocked, "c.yaml"), "c")
	lockDir(t, blocked, 0o000)

	_, err := snapshotWithoutHub(dir, itemState{})

	require.ErrorContains(t, err, "walk hub tree")
}

func TestSnapshotWithoutHubFailsWhenTheItemFileCannotBeRead(t *testing.T) {
	t.Parallel()
	skipIfRoot(t)
	dir := filepath.Join(t.TempDir(), "crowdsec")
	item := filepath.Join(dir, "config", "hub", "collections", "c.yaml")
	writeFile(t, item, "old")
	require.NoError(t, os.Chmod(item, 0o000)) //nolint:gosec // G302: test needs an unreadable file

	_, err := snapshotWithoutHub(dir, itemState{LocalPath: item})

	require.Error(t, err)
}

func TestLightRestoreFailsWhenTheDataDirIsNotADirectory(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "crowdsec")
	writeFile(t, filepath.Join(dir, "config", "config.yaml"), "x")
	snap, err := snapshotWithoutHub(dir, itemState{})
	require.NoError(t, err)
	target := filepath.Join(t.TempDir(), "file")
	writeFile(t, target, "not a directory")

	require.ErrorContains(t, restoreWithoutHub(snap, target), "mkdir data dir")
}

func TestLightRestoreFailsWhenTheHubTreeCannotBePruned(t *testing.T) {
	t.Parallel()
	skipIfRoot(t)
	dir := filepath.Join(t.TempDir(), "crowdsec")
	writeFile(t, filepath.Join(dir, "config", "config.yaml"), "x")
	snap, err := snapshotWithoutHub(dir, itemState{})
	require.NoError(t, err)
	orphans := filepath.Join(dir, "config", "hub", "collections")
	writeFile(t, filepath.Join(orphans, "c.yaml"), "c")
	lockDir(t, orphans, 0o500)

	require.Error(t, restoreWithoutHub(snap, dir))
}

func TestPruneHubToManifestFailsWhenAnOrphanCannotBeRemoved(t *testing.T) {
	t.Parallel()
	skipIfRoot(t)
	dir := filepath.Join(t.TempDir(), "crowdsec")
	orphans := filepath.Join(dir, "hub", "collections")
	writeFile(t, filepath.Join(orphans, "c.yaml"), "c")
	lockDir(t, orphans, 0o500)

	err := pruneHubToManifest(dir, nil)

	require.ErrorContains(t, err, "walk hub tree")
	require.FileExists(t, filepath.Join(orphans, "c.yaml"))
}

// A listed file whose size changed is only logged: the prune keeps it and reports no error.
func TestPruneHubToManifestKeepsListedFilesWhoseSizeChanged(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "crowdsec")
	writeFile(t, filepath.Join(dir, "hub", "kept.yaml"), "grown")
	writeFile(t, filepath.Join(dir, "hub", "orphan.yaml"), "o")
	manifest := []hubManifestEntry{
		{Path: "hub", Dir: true},
		{Path: filepath.Join("hub", "kept.yaml"), Size: 1},
	}

	require.NoError(t, pruneHubToManifest(dir, manifest))

	require.FileExists(t, filepath.Join(dir, "hub", "kept.yaml"))
	require.NoFileExists(t, filepath.Join(dir, "hub", "orphan.yaml"))
}

func TestRestoreSavedItemWithoutASavedItemIsANoop(t *testing.T) {
	t.Parallel()
	snap := t.TempDir()

	require.NoError(t, restoreSavedItem(snap, t.TempDir()))
}

// savedItemSnapshot returns a snapshot directory holding one saved item file below config/hub.
func savedItemSnapshot(t *testing.T) (snap, saved string) {
	t.Helper()
	snap = t.TempDir()
	saved = filepath.Join(snap, snapshotSidecarDir, sidecarItemDir, "config", "hub", "collections", "c.yaml")
	writeFile(t, saved, "saved")
	return snap, saved
}

func TestRestoreSavedItemFailsWhenTheSavedItemCannotBeRead(t *testing.T) {
	t.Parallel()
	skipIfRoot(t)
	snap, saved := savedItemSnapshot(t)
	lockDir(t, filepath.Dir(saved), 0o000)

	require.ErrorContains(t, restoreSavedItem(snap, t.TempDir()), "read saved item")
}

func TestRestoreSavedItemFailsWhenTheTargetFolderCannotBeCreated(t *testing.T) {
	t.Parallel()
	snap, _ := savedItemSnapshot(t)
	dataDir := t.TempDir()
	writeFile(t, filepath.Join(dataDir, "config"), "a file where a folder belongs")

	require.ErrorContains(t, restoreSavedItem(snap, dataDir), "mkdir saved item path")
}

func TestRestoreSavedItemFailsWhenTheExistingFileCannotBeReplaced(t *testing.T) {
	t.Parallel()
	skipIfRoot(t)
	snap, _ := savedItemSnapshot(t)
	dataDir := t.TempDir()
	target := filepath.Join(dataDir, "config", "hub", "collections")
	writeFile(t, filepath.Join(target, "c.yaml"), "applied")
	lockDir(t, target, 0o500)

	require.ErrorContains(t, restoreSavedItem(snap, dataDir), "replace saved item")
}

func TestRestoreSavedItemFailsWhenTheSavedFileCannotBeCopied(t *testing.T) {
	t.Parallel()
	skipIfRoot(t)
	snap, saved := savedItemSnapshot(t)
	require.NoError(t, os.Chmod(saved, 0o000)) //nolint:gosec // G302: test needs an unreadable file

	require.ErrorContains(t, restoreSavedItem(snap, t.TempDir()), "restore saved item")
}
