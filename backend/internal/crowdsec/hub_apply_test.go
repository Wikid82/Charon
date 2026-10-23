package crowdsec

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const applyTestSlug = "test/preset"

var applyTestItem = []byte("name: test/preset\ncollections:\n  - test/dep\nparsers:\n  - test/parser\n")

const (
	applyInstall = "cscli collections install test/preset"
	applyInspect = "cscli collections inspect test/preset -o json"
	applyUpgrade = "cscli collections upgrade test/preset"
	applyUpdate  = "cscli hub update"
	applyList    = "cscli hub list -o json"
	applyVersion = "cscli version"
)

// cscliReply is one scripted cscli answer.
type cscliReply struct {
	out []byte
	err error
}

// applyEnv wires a HubService to a scripted cscli whose defaults are a healthy typed install.
type applyEnv struct {
	t       *testing.T
	svc     *HubService
	cache   *HubCache
	dir     string
	meta    CachedPreset
	reloads int

	mu     sync.Mutex
	calls  []string
	counts map[string]int
	// script may answer a command (n counts earlier runs of the same command); nil means the healthy default.
	script func(ctx context.Context, cmd string, n int) *cscliReply
	// onInstall runs when the install command is received.
	onInstall func()
}

func (e *applyEnv) healthyInspect() []byte {
	// The shape cscli reports: `path` is the item's file below the hub directory, `local_path` the installed link.
	link := filepath.Join(e.dir, "config", "collections", "preset.yaml")
	return fmt.Appendf(nil, `{"installed":true,"tainted":false,"up_to_date":true,"local_hash":%q,"path":"collections/test/preset.yaml","local_path":%q}`, e.meta.ItemSHA256, link)
}

func inspectReply(installed, tainted, upToDate bool, hash string) *cscliReply {
	return &cscliReply{out: fmt.Appendf(nil, `{"installed":%t,"tainted":%t,"up_to_date":%t,"local_hash":%q}`, installed, tainted, upToDate, hash)}
}

// withMembers is a tainted inspect answer for the preset that also lists member items (extra is raw JSON fields).
func withMembers(extra string) *cscliReply {
	return &cscliReply{out: fmt.Appendf(nil, `{"installed":true,"tainted":true,"up_to_date":false,"local_hash":%q,%s}`, strings.Repeat("1", 64), extra)}
}

const healthyHubList = `{"collections":[{"name":"test/preset","status":"enabled"},{"name":"test/dep","status":"enabled"}],"parsers":[{"name":"test/parser","status":"enabled"}]}`

func (e *applyEnv) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := strings.Join(append([]string{name}, stripConfigFlag(args)...), " ")
	e.mu.Lock()
	e.calls = append(e.calls, cmd)
	n := e.counts[cmd]
	e.counts[cmd]++
	script := e.script
	e.mu.Unlock()
	if script != nil {
		if r := script(ctx, cmd, n); r != nil {
			return r.out, r.err
		}
	}
	switch cmd {
	case applyVersion:
		return []byte("version: v1.8.1"), nil
	case applyInstall:
		if e.onInstall != nil {
			e.onInstall()
		}
		return []byte("installed"), nil
	case applyInspect:
		return e.healthyInspect(), nil
	case applyList:
		return []byte(healthyHubList), nil
	default:
		return []byte("ok"), nil
	}
}

func (e *applyEnv) commands() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.calls...)
}

func newApplyEnv(t *testing.T) *applyEnv {
	t.Helper()
	dir := seedPersistentTree(t)
	cache, err := NewHubCache(filepath.Join(dir, "hub_cache"), time.Hour)
	require.NoError(t, err)
	meta, err := cache.Store(context.Background(), applyTestSlug, "collections", "1.0", "hub", applyTestItem)
	require.NoError(t, err)

	e := &applyEnv{t: t, cache: cache, dir: dir, meta: meta, counts: map[string]int{}}
	e.svc = NewHubService(e, cache, dir)
	e.svc.ApplyTimeout = 5 * time.Second
	e.svc.Reload = func(context.Context) error { e.reloads++; return nil }
	return e
}

func (e *applyEnv) apply() (ApplyResult, error) {
	e.t.Helper()
	return e.svc.Apply(context.Background(), applyTestSlug, "collections", e.meta.CacheKey)
}

func (e *applyEnv) requireRolledBack() {
	e.t.Helper()
	require.Equal(e.t, "original: true\n", readFile(e.t, filepath.Join(e.dir, "config", "config.yaml")))
	require.NoFileExists(e.t, filepath.Join(e.dir, "config", "cscli-leftover.yaml"))
	require.Zero(e.t, e.reloads, "no reload before the install is verified")
}

func (e *applyEnv) leaveFileOnInstall() {
	e.onInstall = func() { writeFile(e.t, filepath.Join(e.dir, "config", "cscli-leftover.yaml"), "partial") }
}

func TestApplyInstallsAndVerifies(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)

	res, err := e.apply()
	require.NoError(t, err)
	require.Equal(t, "applied", res.Status)
	require.True(t, res.UsedCSCLI)
	require.False(t, res.ReloadHint, "the reload succeeded")
	require.Equal(t, applyTestSlug, res.AppliedPreset)
	require.Equal(t, e.meta.CacheKey, res.CacheKey)
	require.Empty(t, res.Warnings)
	require.DirExists(t, res.BackupPath)
	require.Equal(t, []string{applyVersion, applyUpdate, applyInspect, applyInstall, applyInspect, applyList}, e.commands())
	require.Equal(t, 1, e.reloads)
	require.FileExists(t, filepath.Join(e.dir, "hub_cache", "collections", "test", "preset", "item.yaml"), "a successful apply keeps the cache entry")
}

func TestApplyKeepsDataDirAndPrunesBackups(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	before, err := os.Stat(e.dir)
	require.NoError(t, err)

	for range DefaultBackupRetention + 2 {
		res, applyErr := e.apply()
		require.NoError(t, applyErr)
		require.Equal(t, "applied", res.Status)
	}

	after, err := os.Stat(e.dir)
	require.NoError(t, err)
	require.True(t, os.SameFile(before, after), "DataDir must never be renamed")
	require.Equal(t, DefaultBackupRetention, countBackups(t, e.dir))
	require.Equal(t, "live-db", readFile(t, filepath.Join(e.dir, "crowdsec.db")))
	require.FileExists(t, filepath.Join(e.dir, "hub_cache", "collections", "test", "preset", "item.yaml"))
}

func TestApplyReportsReloadHintWithoutReloader(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.svc.Reload = nil

	res, err := e.apply()
	require.NoError(t, err)
	require.True(t, res.ReloadHint)
}

func TestApplyRefusesBadInput(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	ctx := context.Background()

	_, err := e.svc.Apply(ctx, "../bad", "collections", "key")
	require.ErrorIs(t, err, ErrInvalidSlug)
	_, err = e.svc.Apply(ctx, applyTestSlug, "collections", "")
	require.ErrorIs(t, err, ErrPresetCacheKeyRequired)
	require.Empty(t, e.commands())

	e.svc.Cache = nil
	_, err = e.svc.Apply(ctx, applyTestSlug, "collections", "key")
	require.ErrorContains(t, err, "cache unavailable")
	require.Empty(t, e.commands())
}

func TestApplyReportsOutdatedCacheEntry(t *testing.T) {
	t.Parallel()
	cases := map[string]func(e *applyEnv) string{
		"different key": func(e *applyEnv) string { return applyTestSlug + "-1-" + strings.Repeat("0", 64) },
		"no entry": func(e *applyEnv) string {
			require.NoError(e.t, e.cache.Evict(context.Background(), "collections", applyTestSlug))
			return e.meta.CacheKey
		},
		"expired entry": func(e *applyEnv) string {
			e.cache.nowFn = func() time.Time { return time.Now().Add(2 * time.Hour) }
			return e.meta.CacheKey
		},
		"entry without a digest": func(e *applyEnv) string {
			writeFile(e.t, filepath.Join(e.dir, "hub_cache", "collections", "test", "preset", "metadata.json"), `{"slug":"test/preset"}`)
			return e.meta.CacheKey
		},
		"content differs from the digest": func(e *applyEnv) string {
			writeFile(e.t, filepath.Join(e.dir, "hub_cache", "collections", "test", "preset", "item.yaml"), "name: other\n")
			return e.meta.CacheKey
		},
	}
	for name, prepare := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newApplyEnv(t)
			key := prepare(e)

			res, err := e.svc.Apply(context.Background(), applyTestSlug, "collections", key)
			require.ErrorIs(t, err, ErrPresetCacheStale)
			require.Empty(t, e.commands(), "no cscli command runs for an outdated entry")
			require.Empty(t, res.BackupPath)
			e.requireRolledBack()
		})
	}
}

func TestApplyNeverContactsTheHub(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	require.NoError(t, e.cache.Evict(context.Background(), "collections", applyTestSlug))
	e.svc.HubBaseURL = "http://127.0.0.1:1"
	e.svc.MirrorBaseURL = "http://127.0.0.1:1"
	e.svc.HTTPClient = nil

	_, err := e.apply()
	require.ErrorIs(t, err, ErrPresetCacheStale)
}

func TestApplyReturnsCacheReadErrors(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := e.svc.Apply(ctx, applyTestSlug, "collections", e.meta.CacheKey)
	require.ErrorIs(t, err, context.Canceled)
	require.NotErrorIs(t, err, ErrPresetCacheStale)
}

func TestApplyReportsUnavailableCSCLI(t *testing.T) {
	t.Parallel()
	t.Run("no executor", func(t *testing.T) {
		t.Parallel()
		e := newApplyEnv(t)
		e.svc.Exec = nil
		res, err := e.apply()
		require.ErrorIs(t, err, ErrCSCLIUnavailable)
		require.EqualError(t, err, "cscli unavailable or not responding")
		require.Empty(t, res.BackupPath)
	})
	t.Run("version probe fails", func(t *testing.T) {
		t.Parallel()
		e := newApplyEnv(t)
		e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
			if cmd == applyVersion {
				return &cscliReply{err: errors.New("cscli: not found")}
			}
			return nil
		}
		_, err := e.apply()
		require.ErrorIs(t, err, ErrCSCLIUnavailable)
		require.Equal(t, []string{applyVersion}, e.commands())
	})
}

func TestApplySnapshotFailureChangesNothing(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	cache, err := NewHubCache(filepath.Join(t.TempDir(), "hub_cache"), time.Hour)
	require.NoError(t, err)
	meta, err := cache.Store(context.Background(), applyTestSlug, "collections", "1.0", "hub", applyTestItem)
	require.NoError(t, err)
	svc := NewHubService(e, cache, filepath.Join(t.TempDir(), "missing-parent", "crowdsec"))

	res, err := svc.Apply(context.Background(), applyTestSlug, "collections", meta.CacheKey)
	require.ErrorContains(t, err, "backup")
	require.Empty(t, res.BackupPath)
	require.Equal(t, []string{applyVersion, applyUpdate, applyInspect}, e.commands(), "nothing is installed when no snapshot exists")
}

func TestApplyInstallFailureRestoresSnapshot(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.leaveFileOnInstall()
	e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
		if cmd == applyInstall {
			writeFile(e.t, filepath.Join(e.dir, "config", "config.yaml"), "original: false\n")
			return &cscliReply{err: errors.New("exit status 1")}
		}
		return nil
	}

	res, err := e.apply()
	require.ErrorContains(t, err, "install collections test/preset")
	require.NotErrorIs(t, err, ErrPresetCacheStale)
	require.Equal(t, "failed", res.Status)
	require.DirExists(t, res.BackupPath, "a failed apply keeps its snapshot")
	e.requireRolledBack()
	require.FileExists(t, filepath.Join(e.dir, "hub_cache", "collections", "test", "preset", "item.yaml"), "the cache entry survives the restore")
}

func TestApplyReportsItemUnknownToCSCLIAsOutdated(t *testing.T) {
	t.Parallel()
	for name, out := range map[string]string{
		"unknown item":        "Error: cscli collections install: can't find 'test/preset' in collections",
		"missing local index": "Error: cscli collections install: invalid hub index: unable to read index file",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newApplyEnv(t)
			e.leaveFileOnInstall()
			e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
				if cmd == applyInstall {
					return &cscliReply{out: []byte(out), err: errors.New("exit status 1")}
				}
				return nil
			}
			_, err := e.apply()
			require.ErrorIs(t, err, ErrPresetCacheStale)
			e.requireRolledBack()
		})
	}
}

func TestApplyRejectsUnusableInspectOutput(t *testing.T) {
	t.Parallel()
	cases := map[string]*cscliReply{
		"command fails":  {err: errors.New("exit status 1")},
		"not JSON":       {out: []byte("not json")},
		"no installed":   {out: []byte(`{"tainted":false}`)},
		"empty document": {out: []byte(`{}`)},
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newApplyEnv(t)
			e.leaveFileOnInstall()
			e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
				if cmd == applyInspect {
					return reply
				}
				return nil
			}
			_, err := e.apply()
			require.Error(t, err)
			require.NotErrorIs(t, err, ErrPresetCacheStale)
			e.requireRolledBack()
		})
	}
}

func TestApplyReportsItemThatIsNotInstalled(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.leaveFileOnInstall()
	e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
		if cmd == applyInspect {
			return inspectReply(false, false, true, "")
		}
		return nil
	}
	_, err := e.apply()
	require.ErrorIs(t, err, ErrPresetCacheStale)
	e.requireRolledBack()
}

func TestApplyReportsLocallyEditedItem(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.leaveFileOnInstall()
	e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
		if cmd == applyInspect {
			return inspectReply(true, true, false, strings.Repeat("c", 64))
		}
		return nil
	}

	_, err := e.apply()
	require.ErrorIs(t, err, ErrPresetLocalModified)
	e.requireRolledBack()
	require.FileExists(t, filepath.Join(e.dir, "hub_cache", "collections", "test", "preset", "item.yaml"), "the entry stays")
	for _, c := range e.commands() {
		require.NotContains(t, c, "--force")
	}
}

func TestApplyUpgradesOlderInstalledItem(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.script = func(_ context.Context, cmd string, n int) *cscliReply {
		if cmd == applyInspect && n == 1 { // the first inspect is the pre-flight check
			return inspectReply(true, false, false, strings.Repeat("1", 64))
		}
		return nil
	}

	res, err := e.apply()
	require.NoError(t, err)
	require.Equal(t, "applied", res.Status)
	require.Equal(t, []string{applyVersion, applyUpdate, applyInspect, applyInstall, applyInspect, applyUpgrade, applyInspect, applyList}, e.commands())
}

func TestApplyUpgradeFailures(t *testing.T) {
	t.Parallel()
	older := inspectReply(true, false, false, strings.Repeat("1", 64))
	cases := map[string]struct {
		script func(cmd string, n int) *cscliReply
		check  func(t *testing.T, err error)
	}{
		"upgrade fails": {
			script: func(cmd string, n int) *cscliReply {
				switch cmd {
				case applyInspect:
					return older
				case applyUpgrade:
					return &cscliReply{err: errors.New("exit status 1")}
				}
				return nil
			},
			check: func(t *testing.T, err error) { require.ErrorContains(t, err, "upgrade collections test/preset") },
		},
		"second inspect fails": {
			script: func(cmd string, n int) *cscliReply {
				if cmd == applyInspect && n == 0 {
					return older
				}
				if cmd == applyInspect {
					return &cscliReply{err: errors.New("exit status 1")}
				}
				return nil
			},
			check: func(t *testing.T, err error) { require.ErrorContains(t, err, "inspect collections test/preset") },
		},
		"edited after the upgrade": {
			script: func(cmd string, n int) *cscliReply {
				if cmd == applyInspect && n == 0 {
					return older
				}
				if cmd == applyInspect {
					return inspectReply(true, true, false, strings.Repeat("2", 64))
				}
				return nil
			},
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, ErrPresetLocalModified) },
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newApplyEnv(t)
			e.leaveFileOnInstall()
			e.script = func(_ context.Context, cmd string, n int) *cscliReply { return tc.script(cmd, n) }

			_, err := e.apply()
			tc.check(t, err)
			e.requireRolledBack()
			require.FileExists(t, filepath.Join(e.dir, "hub_cache", "collections", "test", "preset", "item.yaml"))
		})
	}
}

func TestApplyHashMismatchEvictsEntry(t *testing.T) {
	t.Parallel()
	cases := map[string]func(cmd string) *cscliReply{
		"up to date but different": func(cmd string) *cscliReply {
			if cmd == applyInspect {
				return inspectReply(true, false, true, strings.Repeat("1", 64))
			}
			return nil
		},
		"still different after the upgrade": func(cmd string) *cscliReply {
			if cmd == applyInspect {
				return inspectReply(true, false, false, strings.Repeat("1", 64))
			}
			return nil
		},
	}
	for name, script := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newApplyEnv(t)
			e.leaveFileOnInstall()
			e.script = func(_ context.Context, cmd string, _ int) *cscliReply { return script(cmd) }

			_, err := e.apply()
			require.ErrorIs(t, err, ErrPresetHubUpdating)
			e.requireRolledBack()
			require.NoDirExists(t, filepath.Join(e.dir, "hub_cache", "collections", "test", "preset"), "the mismatching entry is evicted")
		})
	}
}

func TestApplyKeepsNewerEntryOnMismatch(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	var newer CachedPreset
	e.onInstall = func() {
		// A pull replaces the entry while Apply is inside cscli.
		var err error
		newer, err = e.cache.Store(context.Background(), applyTestSlug, "collections", "2.0", "hub", []byte("name: test/preset\n"))
		require.NoError(e.t, err)
	}
	e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
		if cmd == applyInspect {
			return inspectReply(true, false, true, strings.Repeat("1", 64))
		}
		return nil
	}

	_, err := e.apply()
	require.ErrorIs(t, err, ErrPresetHubUpdating)
	loaded, err := e.cache.Load(context.Background(), "collections", applyTestSlug)
	require.NoError(t, err, "the newer entry survives")
	require.Equal(t, newer.CacheKey, loaded.CacheKey)
}

func TestApplyChecksDependencies(t *testing.T) {
	t.Parallel()
	list := func(body string) func(string) *cscliReply {
		return func(cmd string) *cscliReply {
			if cmd == applyList {
				return &cscliReply{out: []byte(body)}
			}
			return nil
		}
	}
	t.Run("missing dependency", func(t *testing.T) {
		t.Parallel()
		e := newApplyEnv(t)
		e.leaveFileOnInstall()
		script := list(`{"collections":[{"name":"test/preset","status":"enabled"},{"name":"test/dep","status":"enabled"}],"parsers":[]}`)
		e.script = func(_ context.Context, cmd string, _ int) *cscliReply { return script(cmd) }
		_, err := e.apply()
		require.ErrorIs(t, err, ErrPresetCacheStale)
		e.requireRolledBack()
	})
	t.Run("item missing from the listing", func(t *testing.T) {
		t.Parallel()
		e := newApplyEnv(t)
		e.leaveFileOnInstall()
		script := list(`{"collections":[]}`)
		e.script = func(_ context.Context, cmd string, _ int) *cscliReply { return script(cmd) }
		_, err := e.apply()
		require.ErrorIs(t, err, ErrPresetCacheStale)
		e.requireRolledBack()
	})
	t.Run("edited dependency only warns", func(t *testing.T) {
		t.Parallel()
		e := newApplyEnv(t)
		script := list(`{"collections":[{"name":"test/preset","status":"enabled"},{"name":"test/dep","status":"enabled,tainted"}],"parsers":[{"name":"test/parser","status":"enabled"}]}`)
		e.script = func(_ context.Context, cmd string, _ int) *cscliReply { return script(cmd) }
		res, err := e.apply()
		require.NoError(t, err)
		require.Equal(t, []string{"dependency test/dep was edited locally"}, res.Warnings)
		require.Equal(t, 1, e.reloads)
	})
	t.Run("listing fails", func(t *testing.T) {
		t.Parallel()
		e := newApplyEnv(t)
		e.leaveFileOnInstall()
		e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
			if cmd == applyList {
				return &cscliReply{err: errors.New("exit status 1")}
			}
			return nil
		}
		_, err := e.apply()
		require.ErrorContains(t, err, "list installed hub items")
		e.requireRolledBack()
	})
	t.Run("listing is not JSON", func(t *testing.T) {
		t.Parallel()
		e := newApplyEnv(t)
		e.leaveFileOnInstall()
		script := list("not json")
		e.script = func(_ context.Context, cmd string, _ int) *cscliReply { return script(cmd) }
		_, err := e.apply()
		require.ErrorContains(t, err, "parse hub list")
		e.requireRolledBack()
	})
}

func TestItemDependencies(t *testing.T) {
	t.Parallel()
	deps, err := itemDependencies([]byte("name: x\nparsers:\n  - a/p\nscenarios: [a/s1, a/s2]\ncontexts:\n  - 5\n  - a/c\nlabels: {l: v}\n"))
	require.NoError(t, err)
	require.Equal(t, []itemDependency{{"parsers", "a/p"}, {"scenarios", "a/s1"}, {"scenarios", "a/s2"}, {"contexts", "a/c"}}, deps)

	deps, err = itemDependencies([]byte("name: x\n"))
	require.NoError(t, err)
	require.Empty(t, deps)

	_, err = itemDependencies([]byte("a: [unclosed"))
	require.ErrorIs(t, err, ErrPresetCacheStale)
	_, err = itemDependencies([]byte("- just\n- a list\n"))
	require.ErrorIs(t, err, ErrPresetCacheStale)

	var many strings.Builder
	many.WriteString("parsers:\n")
	for i := range maxDependencies + 1 {
		fmt.Fprintf(&many, "  - a/p%d\n", i)
	}
	_, err = itemDependencies([]byte(many.String()))
	require.ErrorIs(t, err, ErrPresetCacheStale)
}

func TestApplyTimeoutRestoresSnapshotAndKeepsEntry(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.svc.ApplyTimeout = 200 * time.Millisecond
	e.leaveFileOnInstall()
	e.script = func(ctx context.Context, cmd string, _ int) *cscliReply {
		if cmd == applyInstall {
			<-ctx.Done()
			return &cscliReply{err: ctx.Err()}
		}
		return nil
	}

	res, err := e.apply()
	require.ErrorIs(t, err, ErrPresetApplyTimeout)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, "failed", res.Status)
	e.requireRolledBack()
	_, loadErr := e.cache.Load(context.Background(), "collections", applyTestSlug)
	require.NoError(t, loadErr, "a slow server does not evict a good entry")
}

func TestApplyTimeoutDuringVerificationDoesNotEvict(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.svc.ApplyTimeout = 200 * time.Millisecond
	e.script = func(ctx context.Context, cmd string, _ int) *cscliReply {
		if cmd == applyInspect {
			<-ctx.Done()
			return inspectReply(true, false, true, strings.Repeat("1", 64))
		}
		return nil
	}

	_, err := e.apply()
	require.ErrorIs(t, err, ErrPresetApplyTimeout)
	_, loadErr := e.cache.Load(context.Background(), "collections", applyTestSlug)
	require.NoError(t, loadErr)
	require.Zero(t, e.reloads)
}

func TestApplyHubUpdateHasItsOwnBudget(t *testing.T) {
	t.Parallel()
	for name, budget := range map[string]time.Duration{"short budget": 50 * time.Millisecond, "default budget": 0} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newApplyEnv(t)
			e.svc.HubUpdateTimeout = budget
			e.svc.ApplyTimeout = 2 * time.Second
			e.script = func(ctx context.Context, cmd string, _ int) *cscliReply {
				if cmd == applyUpdate {
					if budget == 0 {
						return &cscliReply{err: errors.New("hub unreachable")}
					}
					<-ctx.Done() // the sub-budget, not the apply deadline, ends the update
					return &cscliReply{err: ctx.Err()}
				}
				return nil
			}

			start := time.Now()
			res, err := e.apply()
			require.NoError(t, err)
			require.Equal(t, "applied", res.Status)
			require.Less(t, time.Since(start), 1500*time.Millisecond)
			require.Contains(t, e.commands(), applyInstall)
		})
	}
}

func TestApplyRestoreFailureIsReported(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
		if cmd == applyInstall {
			// Make the restore itself fail: DataDir is replaced by a regular file mid-apply.
			require.NoError(e.t, os.RemoveAll(e.dir))
			writeFile(e.t, e.dir, "not a dir")
			return &cscliReply{err: errors.New("exit status 1")}
		}
		return nil
	}

	res, err := e.apply()
	require.ErrorContains(t, err, "rollback failed")
	require.ErrorContains(t, err, "backup retained, see server logs")
	require.NotContains(t, err.Error(), res.BackupPath, "the backup location stays in server logs")
	require.NotContains(t, err.Error(), e.dir)
	require.FileExists(t, res.BackupPath+retainMarkerSuffix, "the snapshot a failed restore left behind is kept beyond automatic cleanup")
}

func TestApplyPrunesAfterFailureAndKeepsTheFailedSnapshot(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
		if cmd == applyInstall {
			return &cscliReply{err: errors.New("exit status 1")}
		}
		return nil
	}
	for i := 1; i <= DefaultBackupRetention+3; i++ {
		mkBackupDir(t, e.dir, "backup", fmt.Sprintf("2025010%d-000000.000000", i))
	}

	res, err := e.apply()
	require.Error(t, err)
	require.DirExists(t, res.BackupPath, "the failed apply's snapshot is the newest and always kept")
	require.Equal(t, DefaultBackupRetention, countBackups(t, e.dir))
}

// An open file handle in the data dir does not prevent the copy-based snapshot, DataDir is never renamed
// and the engine-owned hub_cache stays out of the snapshot.
func TestApplyWithOpenFileHandles(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	cacheFile := filepath.Join(e.dir, "hub_cache", "collections", "test", "preset", "item.yaml")
	f, err := os.Open(cacheFile) // #nosec G304 -- test path
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	before, err := os.Stat(e.dir)
	require.NoError(t, err)
	e.onInstall = func() { writeFile(e.t, filepath.Join(e.dir, "installed.marker"), "x") }

	res, err := e.apply()
	require.NoError(t, err)
	after, err := os.Stat(e.dir)
	require.NoError(t, err)
	require.True(t, os.SameFile(before, after), "DataDir must never be renamed or replaced")
	require.Equal(t, "original: true\n", readFile(t, filepath.Join(res.BackupPath, "config", "config.yaml")))
	require.NoDirExists(t, filepath.Join(res.BackupPath, "hub_cache"))
	require.FileExists(t, filepath.Join(e.dir, "installed.marker"))
	require.FileExists(t, cacheFile)
}

// Pull and Apply share one HubCache; run together under -race they never see a torn entry.
func TestPullAndApplyShareOneCache(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.svc.HubBaseURL = "https://hub.crowdsec.net"
	e.svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://hub.crowdsec.net" + defaultHubIndexPath:
			return newResponse(http.StatusOK, `{"items":[{"name":"test/preset","type":"collections","etag":"1","download_url":"https://hub.crowdsec.net/preset.yaml"}]}`), nil
		case "https://hub.crowdsec.net/preset.yaml":
			return newResponse(http.StatusOK, string(applyTestItem)), nil
		}
		return newResponse(http.StatusNotFound, ""), nil
	})}
	// Entries expire between pulls so that every Pull stores again.
	e.cache.ttl = time.Nanosecond

	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 5 {
				if _, err := e.svc.Pull(context.Background(), applyTestSlug, ""); err != nil {
					errs <- err
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 5 {
			_, err := e.svc.Apply(context.Background(), applyTestSlug, "collections", e.meta.CacheKey)
			if err != nil && !errors.Is(err, ErrPresetCacheStale) {
				errs <- err
			}
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}

func TestApplyTaintedDependencyOfCollectionNamesEditedItems(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.leaveFileOnInstall()
	taintedList := []byte(readCSCLIFixture(t, "hub_list_tainted_dependency_with_progress.txt"))
	e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
		switch cmd {
		case applyInspect:
			return withMembers(`"collections":["crowdsecurity/nginx"],"parsers":["crowdsecurity/nginx-logs"]`)
		case applyList:
			return &cscliReply{out: taintedList}
		}
		return nil
	}

	_, err := e.apply()
	require.ErrorIs(t, err, ErrPresetLocalModified)
	var modified *LocalModifiedError
	require.ErrorAs(t, err, &modified)
	require.Equal(t, []string{"collections/crowdsecurity/nginx", "parsers/crowdsecurity/nginx-logs"}, modified.Items)
	e.requireRolledBack()
	for _, c := range e.commands() {
		require.NotContains(t, c, "--force")
	}
}

func TestApplyTaintedItemFallsBackToTopLevelName(t *testing.T) {
	t.Parallel()
	cases := map[string]*cscliReply{
		"hub list fails":         {err: errors.New("exit status 1")},
		"hub list names no item": {out: []byte(healthyHubList)},
		"hub list is not a JSON": {out: []byte("Loaded: 1 parsers\n")},
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newApplyEnv(t)
			e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
				switch cmd {
				case applyInspect:
					return inspectReply(true, true, false, strings.Repeat("1", 64))
				case applyList:
					return reply
				}
				return nil
			}

			_, err := e.apply()
			var modified *LocalModifiedError
			require.ErrorAs(t, err, &modified)
			require.Equal(t, []string{"collections/" + applyTestSlug}, modified.Items)
			e.requireRolledBack()
		})
	}
}

func TestApplyTaintedDependencyOfUntaintedItemStaysWarning(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
		if cmd == applyList {
			return &cscliReply{out: []byte(progressLine + `{"collections":[{"name":"test/preset","status":"enabled"},{"name":"test/dep","status":"enabled"}],"parsers":[{"name":"test/parser","status":"enabled,tainted"}]}`)}
		}
		return nil
	}

	res, err := e.apply()
	require.NoError(t, err)
	require.Equal(t, []string{"dependency test/parser was edited locally"}, res.Warnings)
}

func TestApplyTaintedListsOnlyItemsOfThePreset(t *testing.T) {
	t.Parallel()
	const subInspect = "cscli collections inspect test/sub -o json"
	list := `{"collections":[{"name":"test/preset","status":"enabled,tainted"},{"name":"test/sub","status":"enabled"},{"name":"test/other","status":"enabled,tainted"}],` +
		`"parsers":[{"name":"test/parser","status":"enabled,tainted"},{"name":"test/deep","status":"enabled,tainted"},{"name":"test/unrelated","status":"enabled,tainted"}]}`
	cases := map[string]struct {
		inspect func(cmd string) *cscliReply
		want    []string
	}{
		"unrelated edited item is not listed": {
			inspect: func(cmd string) *cscliReply {
				if cmd == applyInspect {
					return withMembers(`"parsers":["test/parser"],"collections":["test/sub"]`)
				}
				return &cscliReply{err: errors.New("exit status 1")}
			},
			want: []string{"collections/test/preset", "parsers/test/parser"},
		},
		"member collections are followed": {
			inspect: func(cmd string) *cscliReply {
				switch cmd {
				case applyInspect:
					return withMembers(`"parsers":["test/parser"],"collections":["test/sub"]`)
				case subInspect:
					return &cscliReply{out: []byte(`{"parsers":["test/deep","bad name"],"collections":["test/preset"]}`)}
				}
				return nil
			},
			want: []string{"collections/test/preset", "parsers/test/deep", "parsers/test/parser"},
		},
		"unknown member set falls back to the preset": {
			inspect: func(cmd string) *cscliReply {
				if cmd == applyInspect {
					return &cscliReply{out: []byte(progressLine + `{"installed":true,"tainted":true,"up_to_date":false,"local_hash":"` + strings.Repeat("1", 64) + `"}`)}
				}
				return nil
			},
			want: []string{"collections/test/preset"},
		},
		"member inspection that is not JSON is ignored": {
			inspect: func(cmd string) *cscliReply {
				if cmd == applyInspect {
					return withMembers(`"collections":["test/sub"]`)
				}
				return &cscliReply{out: []byte("not json")}
			},
			want: []string{"collections/test/preset"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newApplyEnv(t)
			e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
				if cmd == applyList {
					return &cscliReply{out: []byte(list)}
				}
				if strings.HasPrefix(cmd, "cscli collections inspect") {
					return tc.inspect(cmd)
				}
				return nil
			}

			_, err := e.apply()
			var modified *LocalModifiedError
			require.ErrorAs(t, err, &modified)
			require.Equal(t, tc.want, modified.Items)
			e.requireRolledBack()
			for _, c := range e.commands() {
				require.NotContains(t, c, "--force")
			}
		})
	}
}

func TestApplyClientCancelIsNotReportedAsTimeout(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.leaveFileOnInstall()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
		if cmd == applyInspect {
			cancel()
			return inspectReply(true, false, true, strings.Repeat("1", 64))
		}
		return nil
	}

	_, err := e.svc.Apply(ctx, applyTestSlug, "collections", e.meta.CacheKey)
	require.ErrorIs(t, err, context.Canceled)
	require.NotErrorIs(t, err, ErrPresetApplyTimeout)
	e.requireRolledBack()
	_, loadErr := e.cache.Load(context.Background(), "collections", applyTestSlug)
	require.NoError(t, loadErr, "a cancelled request does not evict the entry")
}

func TestApplyDeadlineWhileLoadingTheEntryIsATimeout(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.svc.ApplyTimeout = time.Nanosecond

	_, err := e.apply()
	require.ErrorIs(t, err, ErrPresetApplyTimeout)
	require.Empty(t, e.commands(), "nothing was changed")
}

func TestApplyOmitsMalformedDependencyNamesFromWarnings(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	item := []byte("name: test/preset\ncollections:\n  - test/dep\n  - bad dependency name\n")
	meta, err := e.cache.Store(context.Background(), applyTestSlug, "collections", "1.0", "hub", item)
	require.NoError(t, err)
	e.meta = meta
	e.script = func(_ context.Context, cmd string, _ int) *cscliReply {
		if cmd == applyList {
			return &cscliReply{out: []byte(`{"collections":[{"name":"test/preset","status":"enabled"},{"name":"test/dep","status":"enabled,tainted"},{"name":"bad dependency name","status":"enabled,tainted"}]}`)}
		}
		return nil
	}

	res, err := e.apply()
	require.NoError(t, err)
	require.Equal(t, []string{"dependency test/dep was edited locally"}, res.Warnings)
}

func TestRollBackMismatchReportsDeadlineAsTimeout(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	e.leaveFileOnInstall()
	backup, err := e.svc.snapshot()
	require.NoError(t, err)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, err = e.svc.rollBackMismatch(ctx, ApplyResult{}, backup, e.meta)
	require.ErrorIs(t, err, ErrPresetApplyTimeout)
	_, loadErr := e.cache.Load(context.Background(), "collections", applyTestSlug)
	require.NoError(t, loadErr, "a deadline does not evict the entry")
}
