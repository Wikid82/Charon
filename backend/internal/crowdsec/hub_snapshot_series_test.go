package crowdsec

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// hubTreeFiles lays out an installed hub below the data folder the way cscli keeps it: the downloaded item
// files below config/hub and links from config/<type>/ into them.
func hubTreeFiles(t *testing.T, dir string, extra int) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "config", "hub", ".index.json"), `{"collections":{}}`)
	writeFile(t, filepath.Join(dir, "config", "hub", "collections", "test", "preset.yaml"), "name: test/preset\nversion: old\n")
	writeFile(t, filepath.Join(dir, "config", "hub", "parsers", "s01-parse", "test", "parser.yaml"), "name: test/parser\n")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "config", "collections"), 0o750))
	require.NoError(t, os.Symlink(filepath.Join(dir, "config", "hub", "collections", "test", "preset.yaml"), filepath.Join(dir, "config", "collections", "preset.yaml")))
	for i := range extra {
		writeFile(t, filepath.Join(dir, "config", "hub", "scenarios", "bulk", fmt.Sprintf("s%05d.yaml", i)), "name: bulk\n")
	}
}

func countEntries(t *testing.T, root string) int {
	t.Helper()
	n := 0
	require.NoError(t, filepath.WalkDir(root, func(string, fs.DirEntry, error) error { n++; return nil }))
	return n
}

// treeDigest describes every entry below root (kind, mode, content, link target).
func treeDigest(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	require.NoError(t, filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		require.NoError(t, err)
		rel, relErr := filepath.Rel(root, p)
		require.NoError(t, relErr)
		if top, _, _ := strings.Cut(filepath.ToSlash(rel), "/"); top == "hub_cache" {
			return nil // the preset cache is evicted on purpose by some failures
		}
		info, infoErr := d.Info()
		require.NoError(t, infoErr)
		entry := info.Mode().String()
		switch {
		case info.IsDir():
			entry = "dir" // directory modes are not part of the contract
		case info.Mode()&os.ModeSymlink != 0:
			target, linkErr := os.Readlink(p)
			require.NoError(t, linkErr)
			entry += " -> " + target
		case info.Mode().IsRegular():
			entry += " " + readFile(t, p)
		}
		out[rel] = entry
		return nil
	}))
	return out
}

func snapshotsOf(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(dir + ".backup.*")
	require.NoError(t, err)
	return matches
}

// The snapshot of a preset apply leaves out the hub tree: it is a cache that cscli can download again.
func TestApplySnapshotLeavesOutTheHubTree(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	hubTreeFiles(t, e.dir, 5)

	res, err := e.apply()

	require.NoError(t, err)
	require.DirExists(t, res.BackupPath)
	require.NoError(t, filepath.WalkDir(res.BackupPath, func(p string, _ fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(res.BackupPath, p)
		require.NoError(t, relErr)
		require.False(t, IsHubPath(rel), "the snapshot holds hub file %s", rel)
		return nil
	}))
	require.FileExists(t, filepath.Join(res.BackupPath, "config", "config.yaml"), "the configuration is still copied")
	require.DirExists(t, filepath.Join(e.dir, "config", "hub"), "the live hub tree stays")
}

// What a snapshot costs does not grow with the hub: the same entries are copied whatever the hub holds.
func TestApplySnapshotSizeDoesNotDependOnTheHub(t *testing.T) {
	t.Parallel()
	small, large := newApplyEnv(t), newApplyEnv(t)
	hubTreeFiles(t, small.dir, 0)
	hubTreeFiles(t, large.dir, 4000)

	smallRes, err := small.apply()
	require.NoError(t, err)
	largeRes, err := large.apply()
	require.NoError(t, err)

	require.Equal(t, countEntries(t, smallRes.BackupPath), countEntries(t, largeRes.BackupPath),
		"a hub of 4000 more files adds nothing to the snapshot")
}

// Refusals that depend only on the state before the install cost no snapshot at all.
func TestApplyRefusesBeforeTheSnapshotWhenTheInspectShowsAProblem(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		reply *cscliReply
		want  error
	}{
		"edited item": {inspectReply(true, true, false, strings.Repeat("c", 64)), ErrPresetLocalModified},
		"unknown to the local index": {&cscliReply{
			out: []byte("Error: cscli collections inspect: can't find 'test/preset' in collections"),
			err: errors.New("exit status 1"),
		}, ErrPresetCacheStale},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newApplyEnv(t)
			hubTreeFiles(t, e.dir, 3)
			e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
				if cmd == applyInspect {
					return tc.reply
				}
				return nil
			}
			before := treeDigest(t, e.dir)

			_, err := e.apply()

			require.ErrorIs(t, err, tc.want)
			require.Empty(t, snapshotsOf(t, e.dir), "no snapshot is taken")
			require.NotContains(t, e.commands(), applyInstall, "nothing is installed")
			require.Equal(t, before, treeDigest(t, e.dir))
			e.requireRolledBack()
		})
	}
}

// The inspect that decides runs after the hub index update and before the install.
func TestApplyInspectsAfterTheHubUpdateAndBeforeTheInstall(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)

	_, err := e.apply()

	require.NoError(t, err)
	cmds := e.commands()
	update, inspect, install := slices.Index(cmds, applyUpdate), slices.Index(cmds, applyInspect), slices.Index(cmds, applyInstall)
	require.GreaterOrEqual(t, update, 0)
	require.Less(t, update, inspect)
	require.Less(t, inspect, install, "the first inspect is the pre-flight check")
}

// Whatever phase fails, the data folder ends up as it was before the apply: configuration, links, the item's own
// hub file and no hub file the install added.
func TestApplyRollbackRestoresThePreApplyState(t *testing.T) {
	t.Parallel()
	hash := strings.Repeat("a", 64)
	phases := map[string]func(e *applyEnv, mutate func()) func(ctx context.Context, cmd string, n int) *cscliReply{
		"install fails": func(_ *applyEnv, mutate func()) func(context.Context, string, int) *cscliReply {
			return func(_ context.Context, cmd string, _ int) *cscliReply {
				if cmd == applyInstall {
					mutate()
					return &cscliReply{err: errors.New("exit status 1")}
				}
				return nil
			}
		},
		"inspect after the install fails": afterInstall(func(cmd string, mutate func()) *cscliReply {
			if cmd == applyInspect {
				return &cscliReply{out: []byte("not json")}
			}
			return nil
		}),
		"item edited": afterInstall(func(cmd string, _ func()) *cscliReply {
			if cmd == applyInspect {
				return inspectReply(true, true, false, hash)
			}
			return nil
		}),
		"hash differs": afterInstall(func(cmd string, _ func()) *cscliReply {
			if cmd == applyInspect {
				return inspectReply(true, false, true, hash)
			}
			return nil
		}),
		"upgrade fails": afterInstall(func(cmd string, _ func()) *cscliReply {
			switch cmd {
			case applyInspect:
				return inspectReply(true, false, false, hash)
			case applyUpgrade:
				return &cscliReply{err: errors.New("exit status 1")}
			}
			return nil
		}),
		"timeout": func(_ *applyEnv, mutate func()) func(context.Context, string, int) *cscliReply {
			return func(ctx context.Context, cmd string, _ int) *cscliReply {
				if cmd == applyInstall {
					mutate()
					<-ctx.Done()
					return &cscliReply{err: ctx.Err()}
				}
				return nil
			}
		},
	}
	for name, script := range phases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newApplyEnv(t)
			e.svc.ApplyTimeout = 300 * time.Millisecond
			hubTreeFiles(t, e.dir, 3)
			before := treeDigest(t, e.dir)
			mutate := func() {
				// What a failing install can leave behind: configuration edits, a new hub file with its link,
				// and a changed hub file of the item itself.
				writeFile(t, filepath.Join(e.dir, "config", "config.yaml"), "original: false\n")
				writeFile(t, filepath.Join(e.dir, "config", "hub", "parsers", "s01-parse", "test", "orphan.yaml"), "name: test/orphan\n")
				require.NoError(t, os.Symlink(filepath.Join(e.dir, "config", "hub", "parsers", "s01-parse", "test", "orphan.yaml"), filepath.Join(e.dir, "config", "orphan-link.yaml")))
				writeFile(t, filepath.Join(e.dir, "config", "hub", "collections", "test", "preset.yaml"), "name: test/preset\nversion: new\n")
			}
			e.script = script(e, mutate)

			_, err := e.apply()

			require.Error(t, err)
			require.Equal(t, before, treeDigest(t, e.dir), "the data folder is what it was before the apply")
			require.NoFileExists(t, filepath.Join(e.dir, "config", "hub", "parsers", "s01-parse", "test", "orphan.yaml"))
			require.Equal(t, "name: test/preset\nversion: old\n", readFile(t, filepath.Join(e.dir, "config", "hub", "collections", "test", "preset.yaml")))
		})
	}
}

// afterInstall wraps a per-command answer: the install command runs mutate and succeeds; the rest is answered by
// answer once the install ran (before it, the healthy default applies).
func afterInstall(answer func(cmd string, mutate func()) *cscliReply) func(e *applyEnv, mutate func()) func(context.Context, string, int) *cscliReply {
	return func(_ *applyEnv, mutate func()) func(context.Context, string, int) *cscliReply {
		installed := false
		return func(_ context.Context, cmd string, _ int) *cscliReply {
			if cmd == applyInstall {
				installed = true
				mutate()
				return nil
			}
			if !installed {
				return nil
			}
			return answer(cmd, mutate)
		}
	}
}

// A successful apply keeps the hub tree in place and the item's own file is the new one.
func TestApplySuccessKeepsTheHubTree(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	hubTreeFiles(t, e.dir, 3)
	e.onInstall = func() {
		writeFile(t, filepath.Join(e.dir, "config", "hub", "collections", "test", "preset.yaml"), "name: test/preset\nversion: new\n")
	}

	res, err := e.apply()

	require.NoError(t, err)
	require.Equal(t, "applied", res.Status)
	require.Equal(t, "name: test/preset\nversion: new\n", readFile(t, filepath.Join(e.dir, "config", "hub", "collections", "test", "preset.yaml")))
	require.FileExists(t, filepath.Join(e.dir, "config", "hub", "scenarios", "bulk", "s00000.yaml"))
}
