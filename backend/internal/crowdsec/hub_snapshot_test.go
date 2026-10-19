package crowdsec

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A pre-flight inspect that fails for another reason, or answers something unreadable, does not stop the apply:
// the checks after the install decide.
func TestApplyContinuesWhenThePreflightInspectIsInconclusive(t *testing.T) {
	t.Parallel()
	replies := map[string]*cscliReply{
		"command fails": {err: errors.New("exit status 1")},
		"not json":      {out: []byte("garbage")},
	}
	for name, reply := range replies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newApplyEnv(t)
			e.script = func(_ context.Context, cmd string, n int) *cscliReply {
				if cmd == applyInspect && n == 0 {
					return reply
				}
				return nil
			}

			res, err := e.apply()

			require.NoError(t, err)
			require.Equal(t, "applied", res.Status)
		})
	}
}

// A deadline that passed during the pre-flight stops the apply before a snapshot is taken.
func TestApplyTimesOutBeforeTheSnapshot(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.svc.ApplyTimeout = 200 * time.Millisecond
	e.script = func(ctx context.Context, cmd string, _ int) *cscliReply {
		if cmd == applyInspect {
			<-ctx.Done()
			return &cscliReply{err: ctx.Err()}
		}
		return nil
	}

	_, err := e.apply()

	require.ErrorIs(t, err, ErrPresetApplyTimeout)
	require.Empty(t, snapshotsOf(t, e.dir))
}

func TestHubFileRelFromLocalPathFallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	item := filepath.Join(dir, "config", "hub", "collections", "a.yaml")
	writeFile(t, item, "x")
	writeFile(t, filepath.Join(dir, "config", "config.yaml"), "x")
	require.NoError(t, os.Symlink(item, filepath.Join(dir, "config", "hub", "link.yaml")))

	rel, ok := hubFileRel(dir, itemState{LocalPath: item})
	require.True(t, ok)
	require.Equal(t, filepath.Join("config", "hub", "collections", "a.yaml"), rel)

	for name, file := range map[string]string{
		"empty":           "",
		"outside":         filepath.Join(os.TempDir(), "elsewhere.yaml"),
		"not a hub file":  filepath.Join(dir, "config", "config.yaml"),
		"missing":         filepath.Join(dir, "config", "hub", "collections", "missing.yaml"),
		"symlink":         filepath.Join(dir, "config", "hub", "link.yaml"),
		"directory":       filepath.Join(dir, "config", "hub", "collections"),
		"escapes the dir": filepath.Join(dir, "config", "hub", "..", "..", "..", "x"),
	} {
		_, ok := hubFileRel(dir, itemState{LocalPath: file})
		require.False(t, ok, name)
	}
}

// A top-level hub folder is covered like config/hub, and an item file outside the data folder is not saved.
func TestLightSnapshotCoversTopLevelHubAndIgnoresForeignItemFile(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "crowdsec")
	writeFile(t, filepath.Join(dir, "config", "config.yaml"), "keep: 1\n")
	writeFile(t, filepath.Join(dir, "hub", "parsers", "p.yaml"), "p")
	foreign := filepath.Join(t.TempDir(), "elsewhere.yaml")
	writeFile(t, foreign, "x")

	snap, err := snapshotWithoutHub(dir, itemState{LocalPath: foreign})
	require.NoError(t, err)
	require.NoDirExists(t, filepath.Join(snap, "hub"))
	require.NoDirExists(t, filepath.Join(snap, snapshotSidecarDir, sidecarItemDir))

	writeFile(t, filepath.Join(dir, "config", "config.yaml"), "keep: 2\n")
	writeFile(t, filepath.Join(dir, "hub", "parsers", "orphan.yaml"), "o")
	writeFile(t, filepath.Join(dir, "hub", "scenarios", "new", "s.yaml"), "s")
	writeFile(t, filepath.Join(dir, "hub", "parsers", "p.yaml"), "changed")

	require.NoError(t, Restore(snap, dir))

	require.Equal(t, "keep: 1\n", readFile(t, filepath.Join(dir, "config", "config.yaml")))
	require.NoFileExists(t, filepath.Join(dir, "hub", "parsers", "orphan.yaml"))
	require.NoDirExists(t, filepath.Join(dir, "hub", "scenarios"))
	require.FileExists(t, filepath.Join(dir, "hub", "parsers", "p.yaml"))
	require.NoDirExists(t, filepath.Join(dir, snapshotSidecarDir), "the sidecar never reaches the data folder")
	require.NoFileExists(t, filepath.Join(dir, hubManifestName))
}

// A hub folder created during the apply is removed whole.
func TestLightRestoreRemovesAHubFolderCreatedByTheApply(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "crowdsec")
	writeFile(t, filepath.Join(dir, "config", "config.yaml"), "x")
	snap, err := snapshotWithoutHub(dir, itemState{})
	require.NoError(t, err)
	writeFile(t, filepath.Join(dir, "config", "hub", "collections", "c.yaml"), "c")

	require.NoError(t, Restore(snap, dir))

	require.NoDirExists(t, filepath.Join(dir, "config", "hub"))
	require.FileExists(t, filepath.Join(dir, "config", "config.yaml"))
}

// A saved item file replaces a link left at its path.
func TestLightRestorePutsTheItemFileBackOverALink(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "crowdsec")
	item := filepath.Join(dir, "config", "hub", "collections", "c.yaml")
	writeFile(t, item, "old")
	snap, err := snapshotWithoutHub(dir, itemState{LocalPath: item})
	require.NoError(t, err)
	require.NoError(t, os.Remove(item))
	outside := filepath.Join(t.TempDir(), "outside.yaml")
	writeFile(t, outside, "outside")
	require.NoError(t, os.Symlink(outside, item))

	require.NoError(t, Restore(snap, dir))

	info, err := os.Lstat(item)
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular())
	require.Equal(t, "old", readFile(t, item))
	require.Equal(t, "outside", readFile(t, outside), "nothing is written through the link")
}

func TestLightRestoreFailsOnAnUnreadableManifest(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "crowdsec")
	writeFile(t, filepath.Join(dir, "config", "config.yaml"), "x")
	snap, err := snapshotWithoutHub(dir, itemState{})
	require.NoError(t, err)
	manifest := filepath.Join(snap, snapshotSidecarDir, hubManifestName)

	writeFile(t, manifest, "{not json")
	require.ErrorContains(t, Restore(snap, dir), "parse hub manifest")

	require.NoError(t, os.Remove(manifest))
	require.ErrorContains(t, Restore(snap, dir), "read hub manifest")
}

func TestSnapshotWithoutHubOfAMissingDataDir(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "absent")

	snap, err := snapshotWithoutHub(dir, itemState{})

	require.NoError(t, err)
	require.DirExists(t, snap)
}

// cscli reports the item's own hub file as `path` (relative to the hub directory) and the installed link as
// `local_path`; the hub file is found from `path`, whichever hub root holds it.
func TestHubFileRelUsesThePathField(t *testing.T) {
	t.Parallel()
	for _, root := range hubRoots {
		t.Run(root, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, root, "collections", "acme", "nginx.yaml"), "x")
			link := filepath.Join(dir, "config", "collections", "nginx.yaml")
			require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o700))
			require.NoError(t, os.Symlink(filepath.Join(dir, root, "collections", "acme", "nginx.yaml"), link))

			rel, ok := hubFileRel(dir, itemState{Path: "collections/acme/nginx.yaml", LocalPath: link})

			require.True(t, ok)
			require.Equal(t, filepath.Join(root, "collections", "acme", "nginx.yaml"), rel)
		})
	}
}

func TestHubFileRelRejectsUnusablePaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config", "hub", "collections", "real.yaml"), "x")
	writeFile(t, filepath.Join(dir, "config", "outside.yaml"), "x")
	require.NoError(t, os.Symlink(filepath.Join(dir, "config", "outside.yaml"), filepath.Join(dir, "config", "hub", "collections", "link.yaml")))

	for name, item := range map[string]itemState{
		"escapes the hub":      {Path: "../../outside.yaml"},
		"absolute":             {Path: filepath.Join(dir, "config", "hub", "collections", "real.yaml")},
		"a link":               {Path: "collections/link.yaml"},
		"missing":              {Path: "collections/missing.yaml"},
		"a directory":          {Path: "collections"},
		"link local_path only": {LocalPath: filepath.Join(dir, "config", "collections", "real.yaml")},
	} {
		_, ok := hubFileRel(dir, item)
		require.False(t, ok, name)
	}
}

// A configuration holding a sidecar-named entry never turns a full snapshot into a light one, and a light
// snapshot still writes its own sidecar.
func TestSnapshotsIgnoreASidecarNamedEntryOfTheConfiguration(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "crowdsec")
	writeFile(t, filepath.Join(dir, "config", "config.yaml"), "x")
	writeFile(t, filepath.Join(dir, snapshotSidecarDir, hubManifestName), "[]")

	full, err := Snapshot(dir)
	require.NoError(t, err)
	require.False(t, isLightSnapshot(full))
	require.NoDirExists(t, filepath.Join(full, snapshotSidecarDir))

	light, err := snapshotWithoutHub(dir, itemState{})
	require.NoError(t, err)
	require.True(t, isLightSnapshot(light))
	require.FileExists(t, filepath.Join(light, snapshotSidecarDir, hubManifestName))
}

func TestImportKeeperKeepsASidecarNamedEntry(t *testing.T) {
	t.Parallel()
	require.True(t, ImportKeeper(t.TempDir())(filepath.Join(snapshotSidecarDir, hubManifestName)))
}

// A file or link at the target makes an exclusive copy fail instead of being written through.
func TestCopyFileExclusiveRefusesAnExistingTarget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	writeFile(t, src, "new")
	existing := filepath.Join(dir, "existing")
	writeFile(t, existing, "old")
	outside := filepath.Join(dir, "outside")
	writeFile(t, outside, "outside")
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(outside, link))

	require.Error(t, copyFileExclusive(src, existing))
	require.Error(t, copyFileExclusive(src, link))
	require.Equal(t, "old", readFile(t, existing))
	require.Equal(t, "outside", readFile(t, outside))

	fresh := filepath.Join(dir, "fresh")
	require.NoError(t, copyFileExclusive(src, fresh))
	require.Equal(t, "new", readFile(t, fresh))
}
