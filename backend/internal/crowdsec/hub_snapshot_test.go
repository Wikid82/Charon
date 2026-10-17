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

func TestHubFileRel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	item := filepath.Join(dir, "config", "hub", "collections", "a.yaml")
	writeFile(t, item, "x")
	writeFile(t, filepath.Join(dir, "config", "config.yaml"), "x")
	require.NoError(t, os.Symlink(item, filepath.Join(dir, "config", "hub", "link.yaml")))

	rel, ok := hubFileRel(dir, item)
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
		_, ok := hubFileRel(dir, file)
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

	snap, err := snapshotWithoutHub(dir, foreign)
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
	snap, err := snapshotWithoutHub(dir, "")
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
	snap, err := snapshotWithoutHub(dir, item)
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
	snap, err := snapshotWithoutHub(dir, "")
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

	snap, err := snapshotWithoutHub(dir, "")

	require.NoError(t, err)
	require.DirExists(t, snap)
}
