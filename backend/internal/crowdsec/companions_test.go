package crowdsec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	companionProfiles = "name: p\nfilters:\n - Alert.Remediation == true\ndecisions:\n - type: ban\n   duration: 4h\nnotifications:\n - %s\n"
)

func putFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	}
}

// companionTrees returns a snapshot and a live tree (both nested) holding the given files.
func companionTrees(t *testing.T, snap, live map[string]string) (snapDir, liveDir string) {
	t.Helper()
	snapDir, liveDir = t.TempDir(), t.TempDir()
	putFiles(t, snapDir, snap)
	putFiles(t, liveDir, live)
	return snapDir, liveDir
}

func TestRestoreMissingCompanionsKeepsWhatTheArchiveLacks(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t,
		map[string]string{
			"config/acquis.yaml":       "snap-acquis",
			"config/profiles.yaml":     "snap-profiles",
			"config/simulation.yaml":   "snap-sim",
			"config/patterns/a":        "pa",
			"config/patterns/sub/b":    "pb",
			"config/unrelated.yaml":    "x",
			"config/notifications/n.y": "ignored",
		},
		map[string]string{"config/config.yaml": plainConfig})

	kept, err := RestoreMissingCompanions(snapDir, liveDir, "config")

	require.NoError(t, err)
	require.Equal(t, []string{"acquis.yaml", "patterns", "profiles.yaml", "simulation.yaml"}, kept)
	require.Equal(t, "snap-acquis", readFile(t, filepath.Join(liveDir, "config", "acquis.yaml")))
	require.Equal(t, "snap-profiles", readFile(t, filepath.Join(liveDir, "config", "profiles.yaml")))
	require.Equal(t, "snap-sim", readFile(t, filepath.Join(liveDir, "config", "simulation.yaml")))
	require.Equal(t, "pa", readFile(t, filepath.Join(liveDir, "config", "patterns", "a")))
	require.Equal(t, "pb", readFile(t, filepath.Join(liveDir, "config", "patterns", "sub", "b")))
	require.NoFileExists(t, filepath.Join(liveDir, "config", "unrelated.yaml"))
}

func TestRestoreMissingCompanionsArchiveFilesWin(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t,
		map[string]string{"config/acquis.yaml": "snap", "config/profiles.yaml": "snap", "config/patterns/a": "snap"},
		map[string]string{"config/acquis.yaml": "archive", "config/patterns/a": "archive", "config/patterns/extra": "archive"})

	kept, err := RestoreMissingCompanions(snapDir, liveDir, "config")

	require.NoError(t, err)
	require.Equal(t, []string{"profiles.yaml"}, kept)
	require.Equal(t, "archive", readFile(t, filepath.Join(liveDir, "config", "acquis.yaml")))
	require.Equal(t, "archive", readFile(t, filepath.Join(liveDir, "config", "patterns", "a")))
	require.FileExists(t, filepath.Join(liveDir, "config", "patterns", "extra"))
}

func TestRestoreMissingCompanionsAbsentInSnapshot(t *testing.T) {
	t.Parallel()
	for name, snap := range map[string]map[string]string{
		"nothing in the snapshot": {"config/config.yaml": plainConfig},
		"no config folder":        {"other.yaml": "x"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			snapDir, liveDir := companionTrees(t, snap, map[string]string{"config/config.yaml": plainConfig})
			kept, err := RestoreMissingCompanions(snapDir, liveDir, "config")
			require.NoError(t, err)
			require.Empty(t, kept)
		})
	}
}

func TestRestoreMissingCompanionsFlatPrefix(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t,
		map[string]string{"acquis.yaml": "snap", "config/acquis.yaml": "nested snap"},
		map[string]string{"config/config.yaml": plainConfig})

	kept, err := RestoreMissingCompanions(snapDir, liveDir, "")

	require.NoError(t, err)
	require.Equal(t, []string{"acquis.yaml"}, kept)
	require.Equal(t, "snap", readFile(t, filepath.Join(liveDir, "acquis.yaml")))
	require.NoFileExists(t, filepath.Join(liveDir, "config", "acquis.yaml"))
}

func TestRestoreMissingCompanionsLinkedCompanionsCountAsPresent(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t,
		map[string]string{"config/profiles.yaml": "snap", "config/patterns/a": "snap", "config/acquis.yaml": "snap"},
		map[string]string{"config/config.yaml": plainConfig})
	outside := t.TempDir()
	putFiles(t, outside, map[string]string{"profiles.yaml": "outside", "patterns/a": "outside"})
	cfg := filepath.Join(liveDir, "config")
	require.NoError(t, os.Symlink(filepath.Join(outside, "patterns"), filepath.Join(cfg, "patterns")))
	require.NoError(t, os.Symlink(filepath.Join(outside, "profiles.yaml"), filepath.Join(cfg, "profiles.yaml")))
	require.NoError(t, os.Symlink(filepath.Join(outside, "missing-target"), filepath.Join(cfg, "acquis.yaml"))) // dangling

	kept, err := RestoreMissingCompanions(snapDir, liveDir, "config")

	require.NoError(t, err)
	require.Empty(t, kept)
	require.Equal(t, "outside", readFile(t, filepath.Join(outside, "profiles.yaml")))
	require.Equal(t, "outside", readFile(t, filepath.Join(outside, "patterns", "a")))
	require.NoFileExists(t, filepath.Join(outside, "missing-target"))
	for _, name := range []string{"patterns", "profiles.yaml", "acquis.yaml"} {
		st, lerr := os.Lstat(filepath.Join(cfg, name))
		require.NoError(t, lerr)
		require.NotZero(t, st.Mode()&os.ModeSymlink, name)
	}
}

func TestRestoreMissingCompanionsSkipsLinkedOrMistypedSnapshotEntries(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t,
		map[string]string{"config/simulation.yaml/inside": "dir where a file belongs", "config/patterns": "file where a dir belongs"},
		map[string]string{"config/config.yaml": plainConfig})
	require.NoError(t, os.Symlink("/etc/hostname", filepath.Join(snapDir, "config", "acquis.yaml")))

	kept, err := RestoreMissingCompanions(snapDir, liveDir, "config")

	require.NoError(t, err)
	require.Empty(t, kept)
	for _, name := range []string{"acquis.yaml", "simulation.yaml", "patterns"} {
		_, lerr := os.Lstat(filepath.Join(liveDir, "config", name))
		require.ErrorIs(t, lerr, os.ErrNotExist, name)
	}
}

func TestRestoreMissingCompanionsReportsUnreadableTree(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits do not restrict root")
	}
	t.Parallel()
	snapDir, liveDir := companionTrees(t, map[string]string{"config/acquis.yaml": "x"}, map[string]string{"config/config.yaml": plainConfig})
	require.NoError(t, os.Chmod(filepath.Join(liveDir, "config"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(liveDir, "config"), 0o750) }) //nolint:gosec // G302: the test tree must be removable

	_, err := RestoreMissingCompanions(snapDir, liveDir, "config")

	require.Error(t, err)
	require.NotContains(t, err.Error(), liveDir)
}

func TestRestoreMissingCompanionsCopyFailureIsReported(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t, map[string]string{"config/patterns/a": "x", "config/profiles.yaml": "x"}, nil)
	// The live folder is missing, so creating the companions below it fails.
	_, err := RestoreMissingCompanions(snapDir, liveDir, "config")
	require.Error(t, err)
}

// keptProfileTrees is a nested snapshot with a profile naming "alpha", defined by a notification file.
func keptProfileTrees(t *testing.T, archive map[string]string) (snapDir, liveDir string) {
	t.Helper()
	snap := map[string]string{
		"config/config.yaml":              plainConfig,
		"config/profiles.yaml":            strings.Replace(companionProfiles, "%s", "alpha", 1),
		"config/notifications/alpha.yaml": notif("alpha"),
		"config/notifications/beta.yml":   notif("beta"),
	}
	live := map[string]string{"config/config.yaml": plainConfig}
	for k, v := range archive {
		live[k] = v
	}
	return companionTrees(t, snap, live)
}

func TestRestoreMissingCompanionsRestoresNeededNotificationFiles(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := keptProfileTrees(t, nil)

	kept, err := RestoreMissingCompanions(snapDir, liveDir, "config")

	require.NoError(t, err)
	require.Equal(t, []string{"notifications/alpha.yaml", "profiles.yaml"}, kept)
	require.Equal(t, notif("alpha"), readFile(t, filepath.Join(liveDir, "config", "notifications", "alpha.yaml")))
	require.NoFileExists(t, filepath.Join(liveDir, "config", "notifications", "beta.yml"), "files the profile does not need stay out")
	require.NoError(t, VerifyKeptProfile(liveDir, kept))
}

func TestRestoreMissingCompanionsNeededNotificationAlreadyDefined(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := keptProfileTrees(t, map[string]string{"config/notifications/mine.yaml": notif("alpha")})

	kept, err := RestoreMissingCompanions(snapDir, liveDir, "config")

	require.NoError(t, err)
	require.Equal(t, []string{"profiles.yaml"}, kept)
	require.NoError(t, VerifyKeptProfile(liveDir, kept))
}

func TestRestoreMissingCompanionsNotificationFlatLayout(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t,
		map[string]string{"profiles.yaml": strings.Replace(companionProfiles, "%s", "alpha", 1), "notifications/alpha.yaml": notif("alpha")},
		map[string]string{"config.yaml": plainConfig})

	kept, err := RestoreMissingCompanions(snapDir, liveDir, "")

	require.NoError(t, err)
	require.Equal(t, []string{"notifications/alpha.yaml", "profiles.yaml"}, kept)
	require.NoError(t, VerifyKeptProfile(liveDir, kept))
}

func TestRestoreMissingCompanionsNotificationFolderIsLinked(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := keptProfileTrees(t, nil)
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(liveDir, "config", "notifications")))

	kept, err := RestoreMissingCompanions(snapDir, liveDir, "config")

	require.NoError(t, err)
	require.Equal(t, []string{"profiles.yaml"}, kept)
	entries, rerr := os.ReadDir(outside)
	require.NoError(t, rerr)
	require.Empty(t, entries, "nothing may be written through the link")
	var undefined *UndefinedNotificationsError
	require.ErrorAs(t, VerifyKeptProfile(liveDir, kept), &undefined)
	require.Equal(t, []string{"alpha"}, undefined.Names)
}

func TestRestoreMissingCompanionsNameCollisionKeepsArchiveFile(t *testing.T) {
	t.Parallel()
	// The archive's own alpha.yaml defines another name, so the needed file cannot be restored under it.
	snapDir, liveDir := keptProfileTrees(t, map[string]string{"config/notifications/alpha.yaml": notif("other")})

	kept, err := RestoreMissingCompanions(snapDir, liveDir, "config")

	require.NoError(t, err)
	require.Equal(t, []string{"profiles.yaml"}, kept)
	require.Equal(t, notif("other"), readFile(t, filepath.Join(liveDir, "config", "notifications", "alpha.yaml")))
	var undefined *UndefinedNotificationsError
	require.ErrorAs(t, VerifyKeptProfile(liveDir, kept), &undefined)
	require.Equal(t, []string{"alpha"}, undefined.Names)
}

func TestRestoreMissingCompanionsDestinationLinkIsNeverReplaced(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := keptProfileTrees(t, nil)
	outside := filepath.Join(t.TempDir(), "target.yaml")
	require.NoError(t, os.WriteFile(outside, []byte("outside"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(liveDir, "config", "notifications"), 0o750))
	require.NoError(t, os.Symlink(outside, filepath.Join(liveDir, "config", "notifications", "alpha.yaml")))

	kept, err := RestoreMissingCompanions(snapDir, liveDir, "config")

	require.NoError(t, err)
	require.Equal(t, []string{"profiles.yaml"}, kept)
	require.Equal(t, "outside", readFile(t, outside))
	var undefined *UndefinedNotificationsError
	require.ErrorAs(t, VerifyKeptProfile(liveDir, kept), &undefined)
}

func TestRestoreMissingCompanionsSkipsLinkedSnapshotNotificationFile(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := keptProfileTrees(t, nil)
	require.NoError(t, os.Remove(filepath.Join(snapDir, "config", "notifications", "alpha.yaml")))
	outside := filepath.Join(t.TempDir(), "alpha.yaml")
	require.NoError(t, os.WriteFile(outside, []byte(notif("alpha")), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(snapDir, "config", "notifications", "alpha.yaml")))

	kept, err := RestoreMissingCompanions(snapDir, liveDir, "config")

	require.NoError(t, err)
	require.Equal(t, []string{"profiles.yaml"}, kept)
	require.NoDirExists(t, filepath.Join(liveDir, "config", "notifications"))
}

func TestRestoreMissingCompanionsDuplicateAfterRestore(t *testing.T) {
	t.Parallel()
	// The snapshot file that defines alpha also defines beta, which the archive already defines.
	snapDir, liveDir := keptProfileTrees(t, map[string]string{"config/notifications/beta-live.yaml": notif("beta")})
	putFiles(t, snapDir, map[string]string{"config/notifications/alpha.yaml": notif("alpha") + "---\n" + notif("beta")})

	kept, err := RestoreMissingCompanions(snapDir, liveDir, "config")

	require.NoError(t, err)
	require.Equal(t, []string{"notifications/alpha.yaml", "profiles.yaml"}, kept)
	require.ErrorIs(t, VerifyKeptProfile(liveDir, kept), ErrDuplicateNotifications)
}

func TestRestoreMissingCompanionsNoSnapshotFileDefinesTheName(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := companionTrees(t,
		map[string]string{"config/profiles.yaml": strings.Replace(companionProfiles, "%s", "ghost", 1)},
		map[string]string{"config/config.yaml": plainConfig})

	kept, err := RestoreMissingCompanions(snapDir, liveDir, "config")

	require.NoError(t, err)
	require.Equal(t, []string{"profiles.yaml"}, kept)
	var undefined *UndefinedNotificationsError
	require.ErrorAs(t, VerifyKeptProfile(liveDir, kept), &undefined)
	require.Equal(t, []string{"ghost"}, undefined.Names)
}

func TestRestoreMissingCompanionsUnparsableSnapshotNotificationIsSkipped(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := keptProfileTrees(t, nil)
	putFiles(t, snapDir, map[string]string{"config/notifications/broken.yaml": "name: [unclosed\n"})

	kept, err := RestoreMissingCompanions(snapDir, liveDir, "config")

	require.NoError(t, err)
	require.Equal(t, []string{"profiles.yaml"}, kept)
	require.NoDirExists(t, filepath.Join(liveDir, "config", "notifications"))
}

func TestVerifyKeptProfileOnlyChecksAKeptProfile(t *testing.T) {
	t.Parallel()
	_, liveDir := companionTrees(t, nil, map[string]string{
		"config/config.yaml":   plainConfig,
		"config/profiles.yaml": strings.Replace(companionProfiles, "%s", "ghost", 1),
	})
	require.NoError(t, VerifyKeptProfile(liveDir, nil))
	require.NoError(t, VerifyKeptProfile(liveDir, []string{"acquis.yaml"}))
	require.Error(t, VerifyKeptProfile(liveDir, []string{"profiles.yaml"}))
}

func TestVerifyKeptProfileFailsOpenWhenTheTreeCannotBeRead(t *testing.T) {
	t.Parallel()
	_, liveDir := companionTrees(t, nil, map[string]string{"config/config.yaml": plainConfig})
	require.NoError(t, os.WriteFile(filepath.Join(liveDir, "config", "profiles.yaml"), []byte(strings.Repeat("#", int(maxInspectEntryBytes)+1)), 0o600))
	require.NoError(t, VerifyKeptProfile(liveDir, []string{"profiles.yaml"}), "an oversized entry makes the inspection incomplete")
	if os.Geteuid() != 0 {
		require.NoError(t, os.Chmod(filepath.Join(liveDir, "config", "profiles.yaml"), 0o000))
		require.NoError(t, VerifyKeptProfile(liveDir, []string{"profiles.yaml"}), "an unreadable entry fails open")
	}
}

func TestHasDuplicateNotificationIgnoresUnparsableFiles(t *testing.T) {
	t.Parallel()
	require.False(t, hasDuplicateNotification(ArchiveInfo{Files: map[string][]byte{"notifications/a.yaml": []byte("name: [x\n")}}))
}

func TestInspectTree(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	putFiles(t, dir, map[string]string{
		"config/config.yaml":                  plainConfig,
		"config/profiles.yaml":                "p",
		"config/notifications/a.yaml":         notif("a"),
		"config/notifications/b.yml":          notif("b"),
		"config/notifications/ignored.txt":    "x",
		"config/notifications/sub/deeper.yml": "x",
		"config/acquis.yaml":                  "not wanted",
		"config.yaml":                         plainConfig,
		"notifications/top.yaml":              notif("top"),
	})

	info, err := InspectTree(dir)

	require.NoError(t, err)
	require.False(t, info.Incomplete)
	names := make([]string, 0, len(info.Files))
	for n := range info.Files {
		names = append(names, n)
	}
	require.ElementsMatch(t, []string{"config.yaml", "notifications/top.yaml", "config/config.yaml", "config/profiles.yaml", "config/notifications/a.yaml", "config/notifications/b.yml"}, names)
}

func TestInspectTreeIncompleteCases(t *testing.T) {
	t.Parallel()
	huge := strings.Repeat("#", int(maxInspectEntryBytes)+1)
	cases := map[string]func(t *testing.T, dir string){
		"oversized profile": func(t *testing.T, dir string) { putFiles(t, dir, map[string]string{"config/profiles.yaml": huge}) },
		"directory named like the profile": func(t *testing.T, dir string) {
			require.NoError(t, os.MkdirAll(filepath.Join(dir, "config", "profiles.yaml"), 0o750))
		},
		"notifications is a file": func(t *testing.T, dir string) { putFiles(t, dir, map[string]string{"config/notifications": "x"}) },
		"too many notification files": func(t *testing.T, dir string) {
			for i := 0; i <= maxInspectEntries; i++ {
				putFiles(t, dir, map[string]string{"config/notifications/" + strings.Repeat("n", 1) + string(rune('a'+i%26)) + strings.Repeat("x", i/26) + ".yaml": "name: x\n"})
			}
		},
		"total size": func(t *testing.T, dir string) {
			big := strings.Repeat("#", int(maxInspectEntryBytes))
			for i := 0; i < 9; i++ {
				putFiles(t, dir, map[string]string{"config/notifications/big" + string(rune('a'+i)) + ".yaml": big})
			}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			setup(t, dir)
			info, err := InspectTree(dir)
			require.NoError(t, err)
			require.True(t, info.Incomplete)
		})
	}
}

func TestInspectTreeErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits do not restrict root")
	}
	t.Parallel()
	for name, mode := range map[string]os.FileMode{"unreadable profile": 0o000} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			putFiles(t, dir, map[string]string{"profiles.yaml": "p"})
			require.NoError(t, os.Chmod(filepath.Join(dir, "profiles.yaml"), mode))
			_, err := InspectTree(dir)
			require.Error(t, err)
			require.NotContains(t, err.Error(), dir)
		})
	}
	t.Run("unreadable notifications folder", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		putFiles(t, dir, map[string]string{"notifications/a.yaml": "x"})
		require.NoError(t, os.Chmod(filepath.Join(dir, "notifications"), 0o000))
		t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "notifications"), 0o750) }) //nolint:gosec // G302: the test tree must be removable
		_, err := InspectTree(dir)
		require.Error(t, err)
	})
	t.Run("unreadable config folder", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		putFiles(t, dir, map[string]string{"config/profiles.yaml": "p"})
		require.NoError(t, os.Chmod(filepath.Join(dir, "config"), 0o000))
		t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "config"), 0o750) }) //nolint:gosec // G302: the test tree must be removable
		_, err := InspectTree(dir)
		require.Error(t, err)
	})
}

func TestUndefinedNotificationsErrorNameList(t *testing.T) {
	t.Parallel()
	e := &UndefinedNotificationsError{Names: []string{"a", "b"}}
	require.Equal(t, "a, b", e.NameList())
	require.Equal(t, "profiles.yaml names notifications that have no matching file: a, b", e.Error())
}

func TestRestoreMissingCompanionsNotificationsPathIsAFile(t *testing.T) {
	t.Parallel()
	snapDir, liveDir := keptProfileTrees(t, map[string]string{"config/notifications": "a file"})

	kept, err := RestoreMissingCompanions(snapDir, liveDir, "config")

	require.NoError(t, err)
	require.Equal(t, []string{"profiles.yaml"}, kept)
	require.Equal(t, "a file", readFile(t, filepath.Join(liveDir, "config", "notifications")))
}

func TestRestoreMissingCompanionsCannotCreateNotificationsFolder(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits do not restrict root")
	}
	t.Parallel()
	snapDir, liveDir := keptProfileTrees(t, nil)
	cfg := filepath.Join(liveDir, "config")
	require.NoError(t, os.WriteFile(filepath.Join(cfg, "acquis.yaml"), []byte("a"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(cfg, "simulation.yaml"), []byte("s"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(cfg, "patterns"), 0o750))
	require.NoError(t, os.Chmod(cfg, 0o500))       //nolint:gosec // G302: read-only folder under test
	t.Cleanup(func() { _ = os.Chmod(cfg, 0o750) }) //nolint:gosec // G302: the test tree must be removable

	_, err := RestoreMissingCompanions(snapDir, liveDir, "config")

	require.Error(t, err)
}

func TestInspectTreeFollowsLinksLikeCrowdSecDoes(t *testing.T) {
	t.Parallel()
	dir, outside := t.TempDir(), t.TempDir()
	putFiles(t, outside, map[string]string{"notifications/a.yaml": notif("a"), "profiles.yaml": "linked profiles"})
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "config"), 0o750))
	require.NoError(t, os.Symlink(filepath.Join(outside, "notifications"), filepath.Join(dir, "config", "notifications")))
	require.NoError(t, os.Symlink(filepath.Join(outside, "profiles.yaml"), filepath.Join(dir, "config", "profiles.yaml")))
	require.NoError(t, os.Symlink(filepath.Join(outside, "gone.yaml"), filepath.Join(dir, "config", "config.yaml"))) // dangling

	info, err := InspectTree(dir)

	require.NoError(t, err)
	require.False(t, info.Incomplete)
	require.Equal(t, notif("a"), string(info.Files["config/notifications/a.yaml"]))
	require.Equal(t, "linked profiles", string(info.Files["config/profiles.yaml"]))
	require.NotContains(t, info.Files, "config/config.yaml")
}

func TestRestoreMissingCompanionsUnreadableNotificationFoldersFailOpen(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits do not restrict root")
	}
	t.Parallel()
	for name, where := range map[string]string{"live": "live", "snapshot": "snapshot"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			snapDir, liveDir := keptProfileTrees(t, map[string]string{"config/notifications/mine.yaml": notif("mine")})
			locked := filepath.Join(liveDir, "config", "notifications")
			if where == "snapshot" {
				locked = filepath.Join(snapDir, "config", "notifications")
				require.NoError(t, os.Remove(filepath.Join(liveDir, "config", "notifications", "mine.yaml")))
				require.NoError(t, os.Remove(filepath.Join(liveDir, "config", "notifications")))
			}
			require.NoError(t, os.Chmod(locked, 0o000))
			t.Cleanup(func() { _ = os.Chmod(locked, 0o750) }) //nolint:gosec // G302: the test tree must be removable

			kept, err := RestoreMissingCompanions(snapDir, liveDir, "config")

			require.NoError(t, err)
			require.Equal(t, []string{"profiles.yaml"}, kept)
		})
	}
}
