package crowdsec

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPruneBackupsProtectingKeepsProtectedSnapshots(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	base := time.Now()
	var paths []string
	for i := range 4 {
		p := dataDir + "." + BackupKindSnapshot + "." + base.Add(time.Duration(i)*time.Second).Format(backupTimeFormat)
		require.NoError(t, os.MkdirAll(p, 0o750))
		paths = append(paths, p)
	}
	removed, err := PruneBackupsProtecting(dataDir, BackupKindSnapshot, 1, []string{paths[0]})
	require.NoError(t, err)
	require.ElementsMatch(t, paths[1:3], removed)
	require.DirExists(t, paths[0], "the protected, oldest snapshot survives")
	require.DirExists(t, paths[3], "the newest snapshot survives")
}

// retainTestSnapshots creates count snapshot directories for dataDir, oldest first.
func retainTestSnapshots(t *testing.T, dataDir string, count int) []string {
	t.Helper()
	base := time.Now().Add(-time.Hour)
	paths := make([]string, 0, count)
	for i := range count {
		p := dataDir + "." + BackupKindSnapshot + "." + base.Add(time.Duration(i)*time.Second).Format(backupTimeFormat)
		require.NoError(t, os.MkdirAll(p, 0o750))
		paths = append(paths, p)
	}
	return paths
}

func TestPruneBackupsKeepsRetainedSnapshotsOutsideTheKeepCount(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	snaps := retainTestSnapshots(t, dataDir, 7)
	require.NoError(t, RetainSnapshot(snaps[0]))

	removed, err := PruneBackups(dataDir, BackupKindSnapshot, 5)
	require.NoError(t, err)
	require.Equal(t, []string{snaps[1]}, removed, "the retained snapshot does not count against keep, so only the oldest unretained one goes")
	require.DirExists(t, snaps[0], "the retained, oldest snapshot survives")
	for _, p := range snaps[2:] {
		require.DirExists(t, p, "the five newest unretained snapshots survive")
	}
	require.FileExists(t, snaps[0]+retainMarkerSuffix, "the marker itself is never pruned while its snapshot exists")
}

func TestPruneBackupFilesIgnoresRetainMarkersOfOtherStems(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "acquis.yaml")
	backup := BackupFilePath(path, time.Now().Add(-time.Minute))
	require.NoError(t, os.WriteFile(backup, []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(backup+retainMarkerSuffix, []byte("x"), 0o600))

	removed, err := PruneBackupFiles(path, 1)
	require.NoError(t, err)
	require.Empty(t, removed)
	require.FileExists(t, backup)
}

func TestPruneBackupsRemovesOrphanRetainMarkers(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	snaps := retainTestSnapshots(t, dataDir, 1)
	orphan := dataDir + "." + BackupKindSnapshot + "." + time.Now().Add(-2*time.Hour).Format(backupTimeFormat) + retainMarkerSuffix
	require.NoError(t, os.WriteFile(orphan, []byte("x"), 0o600))
	junk := dataDir + "." + BackupKindSnapshot + ".notatimestamp" + retainMarkerSuffix
	require.NoError(t, os.WriteFile(junk, []byte("x"), 0o600))

	_, err := PruneBackups(dataDir, BackupKindSnapshot, 5)
	require.NoError(t, err)
	require.NoFileExists(t, orphan, "a marker whose snapshot was deleted by an admin is removed")
	require.FileExists(t, junk, "a marker whose name does not carry a timestamp is ignored")
	require.DirExists(t, snaps[0])
}

func TestRetainSnapshotWritesAOneTimeMarkerBesideTheSnapshot(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	snap := retainTestSnapshots(t, dataDir, 1)[0]

	require.NoError(t, RetainSnapshot(snap))
	require.NoError(t, RetainSnapshot(snap), "retaining twice is not an error")
	marker, err := os.ReadFile(snap + retainMarkerSuffix) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Contains(t, string(marker), "kept for manual recovery")
	require.NoFileExists(t, filepath.Join(snap, retainMarkerSuffix), "the marker never lives inside the snapshot")
	entries, err := os.ReadDir(snap)
	require.NoError(t, err)
	require.Empty(t, entries, "the snapshot content is untouched")
}

func TestRetainSnapshotSkipsMissingPathsAndSymlinks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	missing := filepath.Join(dir, "crowdsec.backup.20200101-000000.000000")
	require.NoError(t, RetainSnapshot(missing))
	require.NoFileExists(t, missing+retainMarkerSuffix)

	target := filepath.Join(dir, "target")
	require.NoError(t, os.MkdirAll(target, 0o750))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(target, link))
	require.NoError(t, RetainSnapshot(link))
	require.NoFileExists(t, link+retainMarkerSuffix)
}

func TestRetainSnapshotReportsAMarkerThatCannotBeWritten(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	snap := retainTestSnapshots(t, dataDir, 1)[0]
	// A directory where the marker belongs is not a retained marker and must not be mistaken for one.
	require.NoError(t, os.MkdirAll(snap+retainMarkerSuffix, 0o750))

	require.Error(t, RetainSnapshot(snap))
}
