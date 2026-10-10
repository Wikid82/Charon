package crowdsec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsHubPath(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"hub":                  true,
		"hub/index.json":       true,
		"hub/a/b.yaml":         true,
		"config/hub":           true,
		"config/hub/x.yaml":    true,
		"./config/hub/x.yaml":  true,
		"Hub/x":                false,
		"config/Hub/x":         false,
		"config/hubs/x":        false,
		"hubs/x":               false,
		"nested/hub/x":         false,
		"config/sub/hub/x":     false,
		"config":               false,
		"hub.bak":              false,
		"":                     false,
		".":                    false,
		"config/collections/x": false,
	}
	for rel, want := range cases {
		require.Equal(t, want, IsHubPath(rel), rel)
	}
}

func TestIsSecretPathBouncersDir(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"bouncers":              true,
		"bouncers/x.key":        true,
		"config/bouncers":       true,
		"config/bouncers/x.key": true,
		"Config/Bouncers/x":     true,
		"BOUNCERS/x":            true,
		"bouncers.bak":          false,
		"mybouncers/x":          false,
		"a/bouncers/x":          false,
		"config/sub/bouncers/x": false,
		"config/bouncers.bak/x": false,
		"config":                false,
	}
	for rel, want := range cases {
		require.Equal(t, want, IsSecretPath(rel), rel)
	}
}

func TestIsEngineOwnedPathPidFile(t *testing.T) {
	t.Parallel()
	require.True(t, IsEngineOwnedPath("crowdsec.pid"))
	require.True(t, IsEngineOwnedPath("./crowdsec.pid"))
	for _, rel := range []string{"a/crowdsec.pid", "config/crowdsec.pid", "crowdsec.pid.bak", "crowdsec.pidx", "my-crowdsec.pid"} {
		require.False(t, IsEngineOwnedPath(rel), rel)
	}
}

func TestIsExportExcluded(t *testing.T) {
	t.Parallel()
	for _, rel := range []string{"hub/x", "config/hub/x", "crowdsec.pid", "config/bouncers/x.key", "bouncer_key", "data/x", "crowdsec.db"} {
		require.True(t, IsExportExcluded(rel), rel)
	}
	for _, rel := range []string{"config/config.yaml", "caddy.yaml", "nested/hub/x"} {
		require.False(t, IsExportExcluded(rel), rel)
	}
}

func TestImportKeeper(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "f"), []byte("x"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "config", "collections"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config", "config.yaml"), []byte("x"), 0o600))
	require.NoError(t, os.Symlink("/etc/crowdsec/hub/collections/x.yaml", filepath.Join(dir, "config", "collections", "dangling.yaml")))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "config", "outlink")))

	keep := ImportKeeper(dir)
	cases := map[string]bool{
		"config/collections/dangling.yaml": true,
		"config/outlink":                   true,
		"config/hub/x":                     true,
		"hub":                              true,
		"crowdsec.pid":                     true,
		"config/bouncers/x.key":            true,
		"bouncer_key":                      true,
		"data/x":                           true,
		"config/config.yaml":               false,
		"config/collections":               false,
		"config/missing.yaml":              false,
		"../escape":                        false,
		"/abs/path":                        false,
	}
	for rel, want := range cases {
		require.Equal(t, want, keep(rel), rel)
	}
	// A path beneath a link is not itself a link and the link target is never followed.
	require.False(t, isLiveSymlink(dir, "config/outlink/f"))
	require.False(t, isLiveSymlink(dir, "config/bad\x00name"))
	require.False(t, isLiveSymlink(dir, ""))
	require.True(t, KeptOrBeneathKept(keep, "config/outlink/f"))
	require.False(t, KeptOrBeneathKept(keep, "config/collections/new.yaml"))
}

func TestClearAndRestoreKeepingHubLinks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mk := func(rel, content string) {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	}
	mk("config/config.yaml", "v1")
	mk("config/hub/index.json", "live-hub")
	mk("crowdsec.pid", "7")
	mk("config/bouncers/k.key", "live-key")
	link := filepath.Join(dir, "config", "parsers", "s01-parse", "p.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o750))
	require.NoError(t, os.Symlink("/etc/crowdsec/hub/p.yaml", link))
	gone := filepath.Join(dir, "config", "collections", "gone.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(gone), 0o750))
	require.NoError(t, os.Symlink("/etc/crowdsec/hub/gone.yaml", gone))

	snap, err := Snapshot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(snap) })
	// The snapshot carries links and config but never the pid file.
	require.NoFileExists(t, filepath.Join(snap, "crowdsec.pid"))
	_, err = os.Lstat(filepath.Join(snap, "config", "parsers", "s01-parse", "p.yaml"))
	require.NoError(t, err)

	keep := ImportKeeper(dir)
	require.NoError(t, ClearConfigKeeping(dir, keep))
	require.NoFileExists(t, filepath.Join(dir, "config", "config.yaml"))
	for _, p := range []string{link, gone} {
		_, lerr := os.Lstat(p)
		require.NoError(t, lerr, p)
	}
	require.Equal(t, "7", readFile(t, filepath.Join(dir, "crowdsec.pid")))

	// A link missing live is restored from the snapshot; a link present live is left alone.
	require.NoError(t, os.Remove(gone))
	require.NoError(t, RestoreKeeping(snap, dir, keep))
	target, err := os.Readlink(gone)
	require.NoError(t, err)
	require.Equal(t, "/etc/crowdsec/hub/gone.yaml", target)
	target, err = os.Readlink(link)
	require.NoError(t, err)
	require.Equal(t, "/etc/crowdsec/hub/p.yaml", target)
	require.Equal(t, "v1", readFile(t, filepath.Join(dir, "config", "config.yaml")))
	require.Equal(t, "live-hub", readFile(t, filepath.Join(dir, "config", "hub", "index.json")))
	require.Equal(t, "live-key", readFile(t, filepath.Join(dir, "config", "bouncers", "k.key")))
}

func TestClearConfigKeepsPidFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "crowdsec.pid"), []byte("7"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.yaml"), []byte("x"), 0o600))
	require.NoError(t, ClearConfig(dir))
	require.Equal(t, "7", readFile(t, filepath.Join(dir, "crowdsec.pid")))
	require.NoFileExists(t, filepath.Join(dir, "other.yaml"))
}

// TestIsLiveSymlinkFollowsIntermediateLinks documents the invariant on isLiveSymlink: a path beneath a
// live link is answered for the link's target, so callers must check ancestors first. The ancestor-first
// KeptOrBeneathKept still keeps everything under the link, and never reports a target outside the tree.
func TestIsLiveSymlinkFollowsIntermediateLinks(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	require.NoError(t, os.Symlink("x", filepath.Join(outside, "inner")))
	dir := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "link")))

	require.True(t, isLiveSymlink(dir, "link"))
	// On its own, the deeper path answers for the link's target (a link outside the tree)...
	require.True(t, isLiveSymlink(dir, filepath.Join("link", "inner")))
	// ...and a regular file or missing name behind the link is not a link, so the answer is not
	// meaningful without the ancestor check.
	require.NoError(t, os.WriteFile(filepath.Join(outside, "plain"), []byte("p"), 0o600))
	require.False(t, isLiveSymlink(dir, filepath.Join("link", "plain")))

	keep := ImportKeeper(dir)
	for _, rel := range []string{"link", "link/plain", "link/inner", "link/missing/deep"} {
		require.True(t, KeptOrBeneathKept(keep, rel), rel)
	}
	require.False(t, KeptOrBeneathKept(keep, "other/plain"))
}
