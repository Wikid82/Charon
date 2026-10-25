package crowdsec

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// capTestNow is a fixed local clock for the cap tests. Snapshot names are built from it in local time.
var capTestNow = time.Date(2026, time.October, 25, 12, 0, 0, 0, time.Local)

const capDay = 24 * time.Hour

// capAgo returns the local time d before capTestNow.
func capAgo(d time.Duration) time.Time { return capTestNow.Add(-d) }

// capSnapshot creates the snapshot directory dataDir.backup.<at> and, when retained, its marker.
func capSnapshot(t *testing.T, dataDir string, at time.Time, retained bool) string {
	t.Helper()
	p := dataDir + "." + BackupKindSnapshot + "." + at.Format(backupTimeFormat)
	require.NoError(t, os.MkdirAll(p, 0o750))
	if retained {
		require.NoError(t, os.WriteFile(p+retainMarkerSuffix, []byte("x"), 0o600))
	}
	return p
}

// capUnretainedNewer creates count unretained snapshots that are 1h, 2h, ... newer than capTestNow.
func capUnretainedNewer(t *testing.T, dataDir string, count int) {
	t.Helper()
	for i := range count {
		capSnapshot(t, dataDir, capTestNow.Add(-time.Duration(i+1)*time.Hour), false)
	}
}

// stubRemoveRetainMarker makes removing a retain marker fail with err. It must not run in parallel.
func stubRemoveRetainMarker(t *testing.T, err error) {
	t.Helper()
	orig := removeRetainMarker
	t.Cleanup(func() { removeRetainMarker = orig })
	removeRetainMarker = func(string) error { return err }
}

// (a) A kept snapshot younger than the floor keeps its marker even when more than the cap are kept.
func TestCapRetainedKeepsEveryYoungSnapshot(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	var snaps []string
	for _, d := range []time.Duration{5, 4, 3, 2, 1} {
		snaps = append(snaps, capSnapshot(t, dataDir, capAgo(d*capDay), true))
	}

	capRetained(snaps[len(snaps)-1], capTestNow)

	for _, p := range snaps {
		require.True(t, isRetained(p), "a snapshot younger than 7 days is never un-marked")
	}
}

// Paths that do not name a snapshot of this kind are ignored, and their siblings are left alone.
func TestCapRetainedIgnoresPathsThatAreNotSnapshots(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	other := capSnapshot(t, dataDir, capAgo(30*capDay), true)
	for _, p := range []string{"plain", filepath.Join(filepath.Dir(dataDir), "x.filebackup.20200101-000000.000000")} {
		capRetained(p, capTestNow)
	}
	require.True(t, isRetained(other), "a call for a path that is not a snapshot leaves the kept snapshots of the stem alone")
}

// (b) Beyond the cap, only the oldest kept snapshots that are past the floor lose their marker.
func TestCapRetainedUnmarksOnlyTheOldestBeyondTheCap(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	p30 := capSnapshot(t, dataDir, capAgo(30*capDay), true)
	p20 := capSnapshot(t, dataDir, capAgo(20*capDay), true)
	p10 := capSnapshot(t, dataDir, capAgo(10*capDay), true)
	p3 := capSnapshot(t, dataDir, capAgo(3*capDay), true)
	p1 := capSnapshot(t, dataDir, capAgo(1*capDay), true)

	capRetained(p1, capTestNow)

	require.False(t, isRetained(p30), "the oldest kept snapshot loses its marker")
	require.False(t, isRetained(p20), "the second oldest loses its marker")
	require.True(t, isRetained(p10), "the cap of 3 is reached, so the rest keep their markers")
	require.True(t, isRetained(p3))
	require.True(t, isRetained(p1))
	require.DirExists(t, p30, "un-marking never deletes the snapshot")
}

// (c) The 7-day floor wins over the cap: a young kept snapshot stops the un-marking.
func TestCapRetainedFloorWinsOverTheCap(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	p30 := capSnapshot(t, dataDir, capAgo(30*capDay), true)
	p6 := capSnapshot(t, dataDir, capAgo(6*capDay), true)
	p5 := capSnapshot(t, dataDir, capAgo(5*capDay), true)
	p4 := capSnapshot(t, dataDir, capAgo(4*capDay), true)
	p1 := capSnapshot(t, dataDir, capAgo(1*capDay), true)

	capRetained(p1, capTestNow)

	require.False(t, isRetained(p30), "the one snapshot past the floor is un-marked")
	for _, p := range []string{p6, p5, p4, p1} {
		require.True(t, isRetained(p), "snapshots younger than 7 days keep their markers, leaving 4 kept")
	}
}

// (d) The just-retained snapshot is protected by identity, even when its timestamp is the oldest.
func TestCapRetainedProtectsJustRetainedByIdentityNotTimestamp(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	justRetained := capSnapshot(t, dataDir, capAgo(30*capDay), true)
	p20 := capSnapshot(t, dataDir, capAgo(20*capDay), true)
	p10 := capSnapshot(t, dataDir, capAgo(10*capDay), true)
	p3 := capSnapshot(t, dataDir, capAgo(3*capDay), true)
	p1 := capSnapshot(t, dataDir, capAgo(1*capDay), true)

	// A path spelled with a redundant element still names the same snapshot once cleaned.
	capRetained(justRetained+string(filepath.Separator)+".", capTestNow)

	require.True(t, isRetained(justRetained), "the just-retained snapshot is never un-marked, however old its name")
	require.False(t, isRetained(p20))
	require.False(t, isRetained(p10))
	require.True(t, isRetained(p3))
	require.True(t, isRetained(p1))
}

// (e) An un-marked snapshot is removed by the next cleanup, not kept.
func TestUnmarkedSnapshotIsRemovedAtTheNextPrune(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	p30 := capSnapshot(t, dataDir, capAgo(30*capDay), true)
	p20 := capSnapshot(t, dataDir, capAgo(20*capDay), true)
	p10 := capSnapshot(t, dataDir, capAgo(10*capDay), true)
	p3 := capSnapshot(t, dataDir, capAgo(3*capDay), true)
	p1 := capSnapshot(t, dataDir, capAgo(1*capDay), true)
	capRetained(p1, capTestNow)
	capUnretainedNewer(t, dataDir, DefaultBackupRetention)

	removed, err := PruneBackups(dataDir, BackupKindSnapshot, DefaultBackupRetention)

	require.NoError(t, err)
	require.ElementsMatch(t, []string{p30, p20}, removed, "the two un-marked snapshots are older than the five newer unretained ones, so the cleanup removes them")
	require.DirExists(t, p10)
	require.DirExists(t, p3)
	require.DirExists(t, p1)
}

// (f) A marker that cannot be removed is logged; the retain itself still succeeds and the marker stays.
func TestRetainSnapshotKeepsTheMarkerWhenItCannotBeRemoved(t *testing.T) {
	stubRemoveRetainMarker(t, errInjectedRetainMarker)
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	now := time.Now()
	old := capSnapshot(t, dataDir, now.Add(-30*capDay), true)
	capSnapshot(t, dataDir, now.Add(-3*capDay), true)
	capSnapshot(t, dataDir, now.Add(-2*capDay), true)
	fresh := capSnapshot(t, dataDir, now, false)

	require.NoError(t, RetainSnapshot(fresh), "a marker that cannot be removed does not fail the retain")
	require.True(t, isRetained(old), "the marker stays when it cannot be removed, the safe direction")
	require.True(t, isRetained(fresh))
}

// The protect list still wins over the cap: a protected snapshot is never deleted, even once un-marked.
func TestPruneProtectListStillWinsOverAnUnmarkedSnapshot(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	p30 := capSnapshot(t, dataDir, capAgo(30*capDay), true)
	p20 := capSnapshot(t, dataDir, capAgo(20*capDay), true)
	p10 := capSnapshot(t, dataDir, capAgo(10*capDay), true)
	p3 := capSnapshot(t, dataDir, capAgo(3*capDay), true)
	p1 := capSnapshot(t, dataDir, capAgo(1*capDay), true)
	capRetained(p1, capTestNow)
	capUnretainedNewer(t, dataDir, DefaultBackupRetention)

	removed, err := PruneBackupsProtecting(dataDir, BackupKindSnapshot, DefaultBackupRetention, []string{p30})

	require.NoError(t, err)
	require.Equal(t, []string{p20}, removed, "the protected, un-marked snapshot survives the cleanup")
	require.DirExists(t, p30)
	require.DirExists(t, p10)
	require.DirExists(t, p3)
	require.DirExists(t, p1)
}

// (g) The 3-marker count case: 3 old kept + 2 young kept + a new failure gives 6 kept; all 3 old lose the marker.
func TestRetainSnapshotUnmarksAllOldSnapshotsWhenSixAreKept(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	now := time.Now()
	old40 := capSnapshot(t, dataDir, now.Add(-40*capDay), true)
	old35 := capSnapshot(t, dataDir, now.Add(-35*capDay), true)
	old30 := capSnapshot(t, dataDir, now.Add(-30*capDay), true)
	young3 := capSnapshot(t, dataDir, now.Add(-3*capDay), true)
	young2 := capSnapshot(t, dataDir, now.Add(-2*capDay), true)
	fresh := capSnapshot(t, dataDir, now, false)

	require.NoError(t, RetainSnapshot(fresh))

	for _, p := range []string{old40, old35, old30} {
		require.False(t, isRetained(p), "every old kept snapshot loses its marker once six are kept")
		require.DirExists(t, p, "un-marking never deletes")
	}
	for _, p := range []string{young3, young2, fresh} {
		require.True(t, isRetained(p), "the young and the just-retained snapshots keep their markers")
	}
}

// (h) The marker tells the admin about the cap and how to release a snapshot.
func TestRetainMarkerTextExplainsTheCap(t *testing.T) {
	t.Parallel()
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	snap := capSnapshot(t, dataDir, time.Now(), false)

	require.NoError(t, RetainSnapshot(snap))

	marker, err := os.ReadFile(snap + retainMarkerSuffix) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Contains(t, string(marker), "7 days")
	require.Contains(t, string(marker), "newest 3")
	require.Contains(t, string(marker), "removed at the next cleanup")
	require.Contains(t, string(marker), "Delete this file")
}

// A snapshot folder that cannot be listed is logged; the marker that was just written stays and nothing is un-marked.
func TestCapRetainedLogsWhenTheSnapshotFolderCannotBeListed(t *testing.T) {
	skipIfRoot(t)
	parent := t.TempDir()
	dataDir := filepath.Join(parent, "crowdsec")
	snap := capSnapshot(t, dataDir, capAgo(30*capDay), true)
	lockDir(t, parent, 0o300)

	capRetained(snap, capTestNow)

	require.True(t, isRetained(snap))
}
