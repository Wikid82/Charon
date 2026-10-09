package crowdsec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	inspectInstalledJSON    = `{"name":"crowdsecurity/sshd","type":"collections","installed":true,"up_to_date":true,"tainted":false,"local":false,"version":"0.2"}`
	inspectNotInstalledJSON = `{"name":"crowdsecurity/sshd","type":"collections","installed":false}`
)

// curatedExec is a scriptable CommandExecutor. hook runs first and may fully handle a command.
type curatedExec struct {
	mu    sync.Mutex
	calls []string
	hook  func(ctx context.Context, cmd string) (out []byte, handled bool, err error)
}

func (e *curatedExec) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := strings.Join(append([]string{name}, args...), " ")
	e.mu.Lock()
	e.calls = append(e.calls, cmd)
	hook := e.hook
	e.mu.Unlock()
	if hook != nil {
		if out, handled, err := hook(ctx, cmd); handled {
			return out, err
		}
	}
	if strings.Contains(cmd, " inspect ") {
		return []byte(inspectInstalledJSON), nil
	}
	return []byte("ok"), nil
}

func (e *curatedExec) snapshot() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.calls...)
}

func (e *curatedExec) count(substr string) int {
	n := 0
	for _, c := range e.snapshot() {
		if strings.Contains(c, substr) {
			n++
		}
	}
	return n
}

func twoItemPreset() Preset {
	return Preset{
		Slug: "test-preset",
		Items: []PresetItem{
			{Type: "collections", Name: "crowdsecurity/sshd"},
			{Type: "parsers", Name: "crowdsecurity/whitelists"},
		},
	}
}

// seedDataDir builds a Charon-like layout including relative, directory and dangling symlinks.
func seedDataDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "crowdsec")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "hub", "collections", "crowdsecurity"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "collections"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("original: true\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hub", "collections", "crowdsecurity", "sshd.yaml"), []byte("name: sshd\n"), 0o600))
	require.NoError(t, os.Symlink("../hub/collections/crowdsecurity/sshd.yaml", filepath.Join(dir, "collections", "sshd.yaml")))
	require.NoError(t, os.Symlink("../hub/missing.yaml", filepath.Join(dir, "collections", "dangling.yaml")))
	require.NoError(t, os.Symlink("hub/collections", filepath.Join(dir, "dirlink")))
	return dir
}

func newCuratedHub(t *testing.T, exec CommandExecutor) (hub *HubService, dataDir string) {
	t.Helper()
	dataDir = seedDataDir(t)
	hub = NewHubService(exec, nil, dataDir)
	hub.ApplyTimeout = 5 * time.Second
	return hub, dataDir
}

func backupsOf(t *testing.T, dataDir string) []string {
	t.Helper()
	m, err := filepath.Glob(dataDir + ".backup.*")
	require.NoError(t, err)
	return m
}

// assertSeedIntact checks the seeded layout byte-for-byte and symlink-for-symlink.
func assertSeedIntact(t *testing.T, dir string) {
	t.Helper()
	cfg, err := os.ReadFile(filepath.Join(dir, "config.yaml")) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, err)
	require.Equal(t, "original: true\n", string(cfg))

	yml := filepath.Join(dir, "hub", "collections", "crowdsecurity", "sshd.yaml")
	data, err := os.ReadFile(yml) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, err)
	require.Equal(t, "name: sshd\n", string(data))
	info, err := os.Lstat(yml)
	require.NoError(t, err)
	require.Zero(t, info.Mode()&os.ModeSymlink)

	for link, target := range map[string]string{
		"collections/sshd.yaml":     "../hub/collections/crowdsecurity/sshd.yaml",
		"collections/dangling.yaml": "../hub/missing.yaml",
		"dirlink":                   "hub/collections",
	} {
		li, lerr := os.Lstat(filepath.Join(dir, link))
		require.NoError(t, lerr, link)
		require.NotZero(t, li.Mode()&os.ModeSymlink, "%s must be a symlink after rollback", link)
		got, rerr := os.Readlink(filepath.Join(dir, link))
		require.NoError(t, rerr)
		require.Equal(t, target, got)
	}
	_, err = os.Stat(filepath.Join(dir, "added-by-install.yaml"))
	require.True(t, os.IsNotExist(err), "files added by a failed install must be removed")
}

// mutateDataDir simulates cscli writing into DataDir mid-install.
func mutateDataDir(dir string) {
	_ = os.WriteFile(filepath.Join(dir, "added-by-install.yaml"), []byte("new"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("original: false\n"), 0o600)
	_ = os.Remove(filepath.Join(dir, "collections", "sshd.yaml"))
	_ = os.Remove(filepath.Join(dir, "collections", "dangling.yaml"))
	_ = os.WriteFile(filepath.Join(dir, "collections", "sshd.yaml"), []byte("dereferenced"), 0o600)
}

func TestApplyCuratedSuccess(t *testing.T) {
	exec := &curatedExec{}
	hub, dir := newCuratedHub(t, exec)
	reloads := 0
	hub.Reload = func(context.Context) error { reloads++; return nil }

	res, err := hub.ApplyCurated(context.Background(), twoItemPreset())
	require.NoError(t, err)
	require.Equal(t, "applied", res.Status)
	require.True(t, res.UsedCSCLI)
	require.False(t, res.ReloadHint, "reload succeeded")
	require.Equal(t, 1, reloads)
	require.Equal(t, "test-preset", res.AppliedPreset)
	require.Equal(t, "curated-test-preset", res.CacheKey)
	require.NotEmpty(t, res.BackupPath)
	require.DirExists(t, res.BackupPath)

	require.Equal(t, []string{
		"cscli version",
		"cscli hub update",
		"cscli collections install crowdsecurity/sshd",
		"cscli collections inspect crowdsecurity/sshd -o json",
		"cscli parsers install crowdsecurity/whitelists",
		"cscli parsers inspect crowdsecurity/whitelists -o json",
	}, exec.snapshot())
	assertSeedIntact(t, dir)
	// the backup is a faithful copy including symlinks
	assertSeedIntact(t, res.BackupPath)
}

func TestApplyCuratedHubUpdateFailureIsNonFatal(t *testing.T) {
	exec := &curatedExec{hook: func(_ context.Context, cmd string) ([]byte, bool, error) {
		if cmd == "cscli hub update" {
			return nil, true, errors.New("hub unreachable")
		}
		return nil, false, nil
	}}
	hub, _ := newCuratedHub(t, exec)

	res, err := hub.ApplyCurated(context.Background(), twoItemPreset())
	require.NoError(t, err)
	require.Equal(t, "applied", res.Status)
}

func TestApplyCuratedReloadOutcomes(t *testing.T) {
	cases := map[string]struct {
		reload   ReloadFunc
		wantHint bool
	}{
		"success":     {func(context.Context) error { return nil }, false},
		"failure":     {func(context.Context) error { return errors.New("signal failed") }, true},
		"not running": {func(context.Context) error { return fmt.Errorf("status: %w", ErrCrowdSecNotRunning) }, true},
		"no reloader": {nil, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			exec := &curatedExec{}
			hub, _ := newCuratedHub(t, exec)
			hub.Reload = tc.reload

			res, err := hub.ApplyCurated(context.Background(), twoItemPreset())
			require.NoError(t, err)
			require.Equal(t, "applied", res.Status)
			require.Equal(t, tc.wantHint, res.ReloadHint)
			require.Equal(t, 0, exec.count("hub reload"))
		})
	}
}

func TestApplyCuratedCSCLIUnavailable(t *testing.T) {
	exec := &curatedExec{hook: func(_ context.Context, cmd string) ([]byte, bool, error) {
		if cmd == "cscli version" {
			return nil, true, errors.New("not found")
		}
		return nil, false, nil
	}}
	hub, dir := newCuratedHub(t, exec)

	res, err := hub.ApplyCurated(context.Background(), twoItemPreset())
	require.ErrorIs(t, err, ErrCSCLIUnavailable)
	require.Equal(t, "failed", res.Status)
	require.NotEmpty(t, res.ErrorMessage)
	require.Empty(t, res.BackupPath)
	require.Empty(t, backupsOf(t, dir))
	require.Equal(t, []string{"cscli version"}, exec.snapshot())
	assertSeedIntact(t, dir)
}

func TestApplyCuratedNilExecutor(t *testing.T) {
	hub, _ := newCuratedHub(t, nil)
	_, err := hub.ApplyCurated(context.Background(), twoItemPreset())
	require.ErrorIs(t, err, ErrCSCLIUnavailable)
}

func TestApplyCuratedInvalidDefinitionTouchesNothing(t *testing.T) {
	cases := map[string]Preset{
		"empty":          {Slug: "x"},
		"bad type":       {Slug: "x", Items: []PresetItem{{Type: "../x", Name: "crowdsecurity/sshd"}}},
		"bad name":       {Slug: "x", Items: []PresetItem{{Type: "collections", Name: "a b"}}},
		"shell meta":     {Slug: "x", Items: []PresetItem{{Type: "collections", Name: "a/b;rm -rf"}}},
		"second invalid": {Slug: "x", Items: []PresetItem{{Type: "collections", Name: "crowdsecurity/sshd"}, {Type: "bogus", Name: "crowdsecurity/x"}}},
	}
	for name, preset := range cases {
		preset := preset
		t.Run(name, func(t *testing.T) {
			exec := &curatedExec{}
			hub, dir := newCuratedHub(t, exec)
			res, err := hub.ApplyCurated(context.Background(), preset)
			require.ErrorIs(t, err, ErrInvalidPresetDefinition)
			require.Equal(t, "failed", res.Status)
			require.Empty(t, exec.snapshot(), "no exec calls for an invalid definition")
			require.Empty(t, backupsOf(t, dir))
			assertSeedIntact(t, dir)
		})
	}
}

func TestApplyCuratedInstallFailureRollsBackInPlace(t *testing.T) {
	var dir string
	var installs int32
	exec := &curatedExec{}
	exec.hook = func(_ context.Context, cmd string) ([]byte, bool, error) {
		if !strings.Contains(cmd, " install ") {
			return nil, false, nil
		}
		// DataDir must exist and be intact while cscli runs (never renamed away).
		cfg, err := os.ReadFile(filepath.Join(dir, "config.yaml")) //nolint:gosec // G304: Test file in temp directory
		if err != nil {
			return nil, true, fmt.Errorf("data dir not in place during install: %w", err)
		}
		if atomic.AddInt32(&installs, 1) == 1 {
			require.Equal(t, "original: true\n", string(cfg))
			mutateDataDir(dir)
			return []byte("ok"), true, nil
		}
		return nil, true, errors.New("install exploded")
	}
	hub, d := newCuratedHub(t, exec)
	dir = d
	reloads := 0
	hub.Reload = func(context.Context) error { reloads++; return nil }

	res, err := hub.ApplyCurated(context.Background(), twoItemPreset())
	require.Error(t, err)
	require.Contains(t, err.Error(), "install parsers crowdsecurity/whitelists")
	require.Contains(t, err.Error(), "install exploded")
	require.Equal(t, "failed", res.Status)
	require.NotEmpty(t, res.ErrorMessage)
	require.NotEmpty(t, res.BackupPath)
	require.DirExists(t, res.BackupPath, "backup copy retained after rollback")
	assertSeedIntact(t, dir)
	require.Equal(t, 0, reloads, "no reload after failure")
}

func TestApplyCuratedVerificationFailures(t *testing.T) {
	cases := map[string]func() ([]byte, error){
		"not installed": func() ([]byte, error) { return []byte(inspectNotInstalledJSON), nil },
		"malformed":     func() ([]byte, error) { return []byte("not json"), nil },
		"nonzero exit":  func() ([]byte, error) { return nil, errors.New("exit status 1") },
		"missing field": func() ([]byte, error) { return []byte(`{"name":"crowdsecurity/sshd"}`), nil },
		"tainted":       func() ([]byte, error) { return []byte(`{"installed":true,"tainted":true}`), nil },
	}
	for name, inspect := range cases {
		inspect := inspect
		t.Run(name, func(t *testing.T) {
			var dir string
			exec := &curatedExec{}
			exec.hook = func(_ context.Context, cmd string) ([]byte, bool, error) {
				if strings.Contains(cmd, " install ") {
					mutateDataDir(dir)
					return []byte("ok"), true, nil
				}
				if strings.Contains(cmd, " inspect ") {
					out, err := inspect()
					return out, true, err
				}
				return nil, false, nil
			}
			hub, d := newCuratedHub(t, exec)
			dir = d

			res, err := hub.ApplyCurated(context.Background(), twoItemPreset())
			require.Error(t, err)
			require.Contains(t, err.Error(), "verify collections crowdsecurity/sshd")
			require.Equal(t, "failed", res.Status)
			require.Equal(t, 1, exec.count(" install "), "stops at first failing item")
			require.Contains(t, exec.snapshot(), "cscli collections inspect crowdsecurity/sshd -o json")
			assertSeedIntact(t, dir)
		})
	}
}

func TestApplyCuratedTimeoutRollsBack(t *testing.T) {
	var dir string
	exec := &curatedExec{}
	exec.hook = func(ctx context.Context, cmd string) ([]byte, bool, error) {
		if strings.Contains(cmd, " install ") {
			mutateDataDir(dir)
			<-ctx.Done()
			// DataDir stays in place even while the command is hanging past the deadline.
			if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil {
				return nil, true, err
			}
			return nil, true, ctx.Err()
		}
		return nil, false, nil
	}
	hub, d := newCuratedHub(t, exec)
	dir = d
	hub.ApplyTimeout = 50 * time.Millisecond

	res, err := hub.ApplyCurated(context.Background(), twoItemPreset())
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, "failed", res.Status)
	assertSeedIntact(t, dir)
}

func TestApplyCuratedCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var dir string
	exec := &curatedExec{}
	exec.hook = func(c context.Context, cmd string) ([]byte, bool, error) {
		if strings.Contains(cmd, " install ") {
			mutateDataDir(dir)
			cancel()
			return nil, true, c.Err()
		}
		return nil, false, nil
	}
	hub, d := newCuratedHub(t, exec)
	dir = d

	_, err := hub.ApplyCurated(ctx, twoItemPreset())
	require.ErrorIs(t, err, context.Canceled)
	assertSeedIntact(t, dir)
}

func TestApplyCuratedContextExpiresBetweenItems(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	exec := &curatedExec{}
	exec.hook = func(_ context.Context, cmd string) ([]byte, bool, error) {
		if strings.Contains(cmd, " inspect ") {
			cancel() // install succeeded, then the context dies before the next item
			return []byte(inspectInstalledJSON), true, nil
		}
		return nil, false, nil
	}
	hub, dir := newCuratedHub(t, exec)

	_, err := hub.ApplyCurated(ctx, twoItemPreset())
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, exec.count(" install "), "no install after the context expired")
	assertSeedIntact(t, dir)
}

func TestApplyCuratedBackupFailureInstallsNothing(t *testing.T) {
	exec := &curatedExec{}
	hub, _ := newCuratedHub(t, exec)
	hub.DataDir = filepath.Join(t.TempDir(), "does-not-exist")

	res, err := hub.ApplyCurated(context.Background(), twoItemPreset())
	require.Error(t, err)
	require.Contains(t, err.Error(), "backup")
	require.Equal(t, "failed", res.Status)
	require.Empty(t, res.BackupPath)
	require.Equal(t, 0, exec.count(" install "))
	require.Empty(t, backupsOf(t, hub.DataDir), "partial backup removed")
}

func TestApplyCuratedBackupMkdirFailure(t *testing.T) {
	exec := &curatedExec{}
	hub, dir := newCuratedHub(t, exec)
	// Parent of the backup path does not exist, so creating the backup dir fails.
	hub.DataDir = filepath.Join(filepath.Dir(dir), "nested", "..", "crowdsec", "..", "missing-parent", "x")

	_, err := hub.ApplyCurated(context.Background(), twoItemPreset())
	require.Error(t, err)
	require.Equal(t, 0, exec.count(" install "))
}

func TestApplyCuratedRollbackFailureIsReported(t *testing.T) {
	var dir string
	exec := &curatedExec{}
	exec.hook = func(_ context.Context, cmd string) ([]byte, bool, error) {
		if strings.Contains(cmd, " install ") {
			// Destroy the backup so the restore step cannot succeed.
			matches, _ := filepath.Glob(dir + ".backup.*")
			for _, m := range matches {
				_ = os.RemoveAll(m)
			}
			return nil, true, errors.New("install exploded")
		}
		return nil, false, nil
	}
	hub, d := newCuratedHub(t, exec)
	dir = d

	res, err := hub.ApplyCurated(context.Background(), twoItemPreset())
	require.Error(t, err)
	require.Contains(t, err.Error(), "install exploded")
	require.Contains(t, err.Error(), "rollback failed")
	require.Contains(t, err.Error(), res.BackupPath)
	require.Equal(t, "failed", res.Status)
	require.DirExists(t, dir, "DataDir itself is never removed")
}

func TestApplyCuratedSerializesConcurrentCalls(t *testing.T) {
	var inFlight, maxInFlight int32
	exec := &curatedExec{}
	exec.hook = func(_ context.Context, cmd string) ([]byte, bool, error) {
		if strings.Contains(cmd, " install ") {
			cur := atomic.AddInt32(&inFlight, 1)
			for {
				prev := atomic.LoadInt32(&maxInFlight)
				if cur <= prev || atomic.CompareAndSwapInt32(&maxInFlight, prev, cur) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt32(&inFlight, -1)
			return []byte("ok"), true, nil
		}
		return nil, false, nil
	}
	hub, dir := newCuratedHub(t, exec)

	var wg sync.WaitGroup
	results := make([]error, 4)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = hub.ApplyCurated(context.Background(), twoItemPreset())
		}(i)
	}
	wg.Wait()

	for _, err := range results {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), atomic.LoadInt32(&maxInFlight), "applies must not interleave")
	require.Len(t, backupsOf(t, dir), 4, "each apply gets its own backup")
}

func TestApplyCuratedSerializesWithApply(t *testing.T) {
	// Apply holds the same mutex: with the lock held ApplyCurated must wait.
	exec := &curatedExec{}
	hub, _ := newCuratedHub(t, exec)
	hub.mu.Lock()
	done := make(chan struct{})
	go func() {
		_, _ = hub.ApplyCurated(context.Background(), twoItemPreset())
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("ApplyCurated ran while the hub lock was held")
	case <-time.After(100 * time.Millisecond):
	}
	hub.mu.Unlock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("ApplyCurated never completed after the lock was released")
	}
}

func TestVerifyItemNilContextError(t *testing.T) {
	hub, _ := newCuratedHub(t, &curatedExec{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, hub.verifyItem(ctx, PresetItem{Type: "collections", Name: "crowdsecurity/sshd"}), context.Canceled)
}

func TestApplyCuratedRollbackLeavesLiveDatabaseUntouched(t *testing.T) {
	var dir string
	exec := &curatedExec{}
	exec.hook = func(_ context.Context, cmd string) ([]byte, bool, error) {
		if strings.Contains(cmd, " install ") {
			// The running engine keeps writing to its database while the install is in flight.
			for _, f := range []string{"crowdsec.db", "crowdsec.db-wal", "crowdsec.db-shm"} {
				_ = os.WriteFile(filepath.Join(dir, "data", f), []byte("live-"+f), 0o600)
			}
			mutateDataDir(dir)
			return nil, true, errors.New("install exploded")
		}
		return nil, false, nil
	}
	hub, d := newCuratedHub(t, exec)
	dir = d
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "data"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data", "crowdsec.db"), []byte("before"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data", "keep.txt"), []byte("kept"), 0o600))

	res, err := hub.ApplyCurated(context.Background(), twoItemPreset())
	require.Error(t, err)

	// Backup never contains the database or its sidecars.
	for _, f := range []string{"crowdsec.db", "crowdsec.db-wal", "crowdsec.db-shm"} {
		_, statErr := os.Stat(filepath.Join(res.BackupPath, "data", f))
		require.True(t, os.IsNotExist(statErr), "%s must not be backed up", f)
		// Rollback neither deleted nor rewound the live files.
		got, readErr := os.ReadFile(filepath.Join(dir, "data", f)) //nolint:gosec // G304: Test file in temp directory
		require.NoError(t, readErr, f)
		require.Equal(t, "live-"+f, string(got))
	}
	got, err := os.ReadFile(filepath.Join(dir, "data", "keep.txt")) //nolint:gosec // G304: Test file in temp directory
	require.NoError(t, err)
	require.Equal(t, "kept", string(got))
	assertSeedIntact(t, dir)
}

func TestEmptyDirExcept(t *testing.T) {
	require.NoError(t, emptyDirExcept(filepath.Join(t.TempDir(), "missing"), isLiveDBFile))

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "a", "b"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "gone", "x"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a", "crowdsec.db"), []byte("db"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a", "b", "f"), []byte("f"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "top"), []byte("t"), 0o600))

	require.NoError(t, emptyDirExcept(dir, isLiveDBFile))
	require.FileExists(t, filepath.Join(dir, "a", "crowdsec.db"))
	require.NoFileExists(t, filepath.Join(dir, "top"))
	require.NoDirExists(t, filepath.Join(dir, "a", "b"))
	require.NoDirExists(t, filepath.Join(dir, "gone"))

	// a regular file instead of a directory cannot be listed
	require.Error(t, emptyDirExcept(filepath.Join(dir, "a", "crowdsec.db"), isLiveDBFile))
}
