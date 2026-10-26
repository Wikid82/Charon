package crowdsec

import (
	"errors"
	"io"
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
	dir := t.TempDir()
	path := filepath.Join(dir, "acquis.yaml")
	now := time.Now()
	older := BackupFilePath(path, now.Add(-2*time.Minute))
	newer := BackupFilePath(path, now.Add(-time.Minute))
	for _, b := range []string{older, newer} {
		require.NoError(t, os.WriteFile(b, []byte("x"), 0o600))
	}
	// A marker of another stem (other.yaml) must neither protect nor count against acquis.yaml backups.
	otherBackup := BackupFilePath(filepath.Join(dir, "other.yaml"), now.Add(-3*time.Minute))
	require.NoError(t, os.WriteFile(otherBackup, []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(otherBackup+retainMarkerSuffix, []byte("x"), 0o600))

	removed, err := PruneBackupFiles(path, 1)
	require.NoError(t, err)
	require.Equal(t, []string{older}, removed, "only the older acquis.yaml backup goes; the other stem's marker does not protect it")
	require.FileExists(t, newer, "the newest acquis.yaml backup survives")
	require.NoFileExists(t, older)
	require.FileExists(t, otherBackup, "a backup of another stem is never pruned by this stem")
	require.FileExists(t, otherBackup+retainMarkerSuffix, "a marker of another stem is never pruned by this stem")
}

func TestPruneBackupFilesSparesARetainedBackupOfTheSameStem(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "acquis.yaml")
	now := time.Now()
	older := BackupFilePath(path, now.Add(-2*time.Minute))
	newer := BackupFilePath(path, now.Add(-time.Minute))
	for _, b := range []string{older, newer} {
		require.NoError(t, os.WriteFile(b, []byte("x"), 0o600))
	}
	require.NoError(t, os.WriteFile(older+retainMarkerSuffix, []byte("x"), 0o600))

	removed, err := PruneBackupFiles(path, 1)
	require.NoError(t, err)
	require.Empty(t, removed, "the retained older backup is outside the keep count, so the newest one alone is kept and nothing goes")
	require.FileExists(t, older, "a marker on the same stem protects that file backup")
	require.FileExists(t, newer)
}

func TestRollbackFailureReportsTheRollbackWhenTheMarkerCannotBeWritten(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	snap := retainTestSnapshots(t, dataDir, 1)[0]
	// A directory where the marker belongs makes RetainSnapshot fail; the rollback error must still be reported.
	require.NoError(t, os.MkdirAll(snap+retainMarkerSuffix, 0o750))
	cause := errors.New("apply broke")

	err := rollbackFailure(cause, errors.New("restore broke"), snap)
	require.ErrorIs(t, err, cause)
	require.ErrorIs(t, err, ErrRollbackFailed, "a marker that cannot be written does not hide the failed rollback")
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

func TestRetainSnapshotReportsAStatErrorOnTheSnapshot(t *testing.T) {
	skipIfRoot(t)
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	snap := retainTestSnapshots(t, dataDir, 1)[0]
	// Without search permission on the parent the snapshot cannot be inspected at all: that is not "missing".
	lockDir(t, filepath.Dir(snap), 0o600)

	require.Error(t, RetainSnapshot(snap))
}

// errInjectedRetainMarker is returned by the marker wrapper so tests can tell it from real errors.
var errInjectedRetainMarker = errors.New("injected retain marker failure")

// failingRetainMarker wraps the real marker file and makes Write or Close fail. Close always closes the real file.
type failingRetainMarker struct {
	io.WriteCloser
	failWrite bool
	failClose bool
}

func (m failingRetainMarker) Write(p []byte) (int, error) {
	if m.failWrite {
		return 0, errInjectedRetainMarker
	}
	return m.WriteCloser.Write(p)
}

func (m failingRetainMarker) Close() error {
	if err := m.WriteCloser.Close(); err != nil {
		return err
	}
	if m.failClose {
		return errInjectedRetainMarker
	}
	return nil
}

// stubRetainMarker makes the marker write or close fail; it must not run in parallel with other marker tests.
func stubRetainMarker(t *testing.T, failWrite, failClose bool) {
	t.Helper()
	orig := openRetainMarker
	t.Cleanup(func() { openRetainMarker = orig })
	openRetainMarker = func(name string) (io.WriteCloser, error) {
		f, err := orig(name)
		if err != nil {
			return nil, err
		}
		return failingRetainMarker{WriteCloser: f, failWrite: failWrite, failClose: failClose}, nil
	}
}

func TestRetainSnapshotFailsWhenTheMarkerCannotBeWrittenOrClosed(t *testing.T) {
	for _, tc := range []struct {
		name               string
		failWrite, failClo bool
	}{{"write", true, false}, {"close", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			stubRetainMarker(t, tc.failWrite, tc.failClo)
			dataDir := filepath.Join(t.TempDir(), "crowdsec")
			snap := retainTestSnapshots(t, dataDir, 1)[0]

			require.ErrorIs(t, RetainSnapshot(snap), errInjectedRetainMarker)
			require.NoFileExists(t, snap+retainMarkerSuffix, "a marker that failed to be written is removed again")
		})
	}
}

func TestPruneBackupsLogsAnOrphanMarkerItCannotRemove(t *testing.T) {
	skipIfRoot(t)
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	snaps := retainTestSnapshots(t, dataDir, 1)
	orphan := dataDir + "." + BackupKindSnapshot + "." + time.Now().Add(-2*time.Hour).Format(backupTimeFormat) + retainMarkerSuffix
	require.NoError(t, os.WriteFile(orphan, []byte("x"), 0o600))
	lockDir(t, filepath.Dir(dataDir), 0o500)

	_, err := PruneBackups(dataDir, BackupKindSnapshot, 5)
	require.NoError(t, err, "a stray marker that cannot be removed is only logged and retried by the next prune")
	require.FileExists(t, orphan)
	require.DirExists(t, snaps[0])
}
