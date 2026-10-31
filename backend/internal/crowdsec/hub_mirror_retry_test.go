package crowdsec

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// dialFailureText is the output cscli prints when the hub address refuses the connection.
const dialFailureText = "failed http request for https://cdn-hub.crowdsec.net/crowdsecurity/hub/.index.json: dial tcp 1.2.3.4:443: connection refused"

var (
	errLiveCSCLI   = errors.New("live cscli failed")
	errMirrorCSCLI = errors.New("mirror cscli failed")
)

// scriptedCSCLI answers each cscli call from respond and records the arguments of every call.
type scriptedCSCLI struct {
	mu      sync.Mutex
	calls   [][]string
	respond func(ctx context.Context, n int, args []string) ([]byte, error)
}

func (e *scriptedCSCLI) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	if name != "cscli" {
		return nil, errors.New("unexpected executable: " + name)
	}
	e.mu.Lock()
	n := len(e.calls)
	e.calls = append(e.calls, append([]string(nil), args...))
	e.mu.Unlock()
	if e.respond == nil {
		return nil, nil
	}
	return e.respond(ctx, n, args)
}

// recorded returns a copy of the calls made so far.
func (e *scriptedCSCLI) recorded() [][]string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([][]string(nil), e.calls...)
}

// configFlag returns the path after the -c flag of a cscli call, or "" when there is none.
func configFlag(args []string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-c" {
			return args[i+1]
		}
	}
	return ""
}

// isDerivedConfig reports whether path names a derived mirror configuration.
func isDerivedConfig(path string) bool {
	return strings.HasPrefix(filepath.Base(path), mirrorConfigPrefix)
}

// assertDerivedConfig checks a derived mirror configuration while it exists: owner-only mode, the mirror template in
// the cscli section, and the local override copy next to it with the live content.
func assertDerivedConfig(t *testing.T, path string) {
	t.Helper()
	require.True(t, isDerivedConfig(path), path)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc struct {
		CSCLI map[string]any `yaml:"cscli"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	require.Equal(t, testMirrorTemplate, doc.CSCLI[mirrorTemplateKey])
	require.Equal(t, "master", doc.CSCLI[mirrorBranchKey])

	local, err := os.ReadFile(path + ".local")
	require.NoError(t, err)
	require.Contains(t, string(local), "/local/hub")
	localInfo, err := os.Stat(path + ".local")
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), localInfo.Mode().Perm())
}

// mirrorTestSetup creates a data directory with a live config.yaml and a service without an executor; tests set Exec.
func mirrorTestSetup(t *testing.T) (svc *HubService, live string) {
	t.Helper()
	dir := t.TempDir()
	live = writeCSCLIConfig(t, dir, "cscli:\n  hub_branch: master\n  hub_dir: /etc/crowdsec/hub\n")
	svc = NewHubService(nil, nil, dir)
	return svc, live
}

func mirrorTestOpts(live string, mem *MirrorMemory) HubCSCLIOptions {
	return HubCSCLIOptions{ConfigPath: live, Template: testMirrorTemplate, Memory: mem}
}

func TestRunHubCSCLIPlainSuccessIsOneCall(t *testing.T) {
	t.Parallel()
	svc, live := mirrorTestSetup(t)
	exec := &scriptedCSCLI{respond: func(_ context.Context, _ int, _ []string) ([]byte, error) {
		return []byte("ok"), nil
	}}
	out, err := RunHubCSCLI(context.Background(), exec, mirrorTestOpts(live, NewMirrorMemory(time.Minute, nil)), "collections", "install", "demo")
	require.NoError(t, err)
	require.Equal(t, "ok", string(out))
	calls := exec.recorded()
	require.Len(t, calls, 1)
	require.Equal(t, live, configFlag(calls[0]))
	require.Equal(t, []string{"collections", "install", "demo"}, subcommand(calls[0]))
	require.NotNil(t, svc)
}

func TestRunHubCSCLIInstallNetworkFailureRetriesUnderDerivedConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	live := writeCSCLIConfig(t, dir, "cscli:\n  hub_branch: master\n")
	require.NoError(t, os.WriteFile(live+".local", []byte("cscli:\n  hub_dir: /local/hub\n"), 0o600))
	mem := NewMirrorMemory(time.Minute, nil)
	exec := &scriptedCSCLI{respond: func(_ context.Context, n int, args []string) ([]byte, error) {
		switch n {
		case 0:
			require.Equal(t, live, configFlag(args))
			return []byte(dialFailureText), errLiveCSCLI
		case 1:
			// The retried install refreshes the index under the same derived configuration first.
			require.Equal(t, []string{"hub", "update"}, subcommand(args))
			assertDerivedConfig(t, configFlag(args))
			return nil, nil
		case 2:
			require.Equal(t, []string{"collections", "install", "demo"}, subcommand(args))
			assertDerivedConfig(t, configFlag(args))
			return []byte("installed"), nil
		}
		t.Fatalf("unexpected call %d: %v", n, args)
		return nil, nil
	}}

	out, err := RunHubCSCLI(context.Background(), exec, mirrorTestOpts(live, mem), "collections", "install", "demo")
	require.NoError(t, err)
	require.Equal(t, "installed", string(out))
	require.Len(t, exec.recorded(), 3)
	require.True(t, mem.Active(), "a mirror win opens the mirror-first window")
	require.Equal(t, []string{"config.yaml", "config.yaml.local"}, dirEntryNames(t, dir), "derived copies are removed after the retry")
}

func TestRunHubCSCLIHubUpdateRetriesOnMirrorAfterAttemptTimeout(t *testing.T) {
	t.Parallel()
	_, live := mirrorTestSetup(t)
	exec := &scriptedCSCLI{respond: func(ctx context.Context, n int, args []string) ([]byte, error) {
		if n == 0 {
			// The live attempt hangs until its share of the time is spent.
			<-ctx.Done()
			return nil, ctx.Err()
		}
		require.True(t, isDerivedConfig(configFlag(args)))
		require.Equal(t, []string{"hub", "update"}, subcommand(args))
		return nil, nil
	}}
	parent, cancel := context.WithTimeout(context.Background(), 9*time.Second)
	defer cancel()

	_, err := RunHubCSCLI(parent, exec, mirrorTestOpts(live, nil), "hub", "update")
	require.NoError(t, err)
	require.Len(t, exec.recorded(), 2)
}

func TestRunHubCSCLIInstallDoesNotRetryOnBareDeadlineText(t *testing.T) {
	t.Parallel()
	_, live := mirrorTestSetup(t)
	exec := &scriptedCSCLI{respond: func(_ context.Context, _ int, _ []string) ([]byte, error) {
		return []byte("context deadline exceeded"), errLiveCSCLI
	}}
	_, err := RunHubCSCLI(context.Background(), exec, mirrorTestOpts(live, nil), "collections", "install", "demo")
	require.ErrorIs(t, err, errLiveCSCLI)
	require.Len(t, exec.recorded(), 1, "a healthy slow install is not killed by a retry")
}

func TestRunHubCSCLIItemErrorIsNotRetried(t *testing.T) {
	t.Parallel()
	_, live := mirrorTestSetup(t)
	exec := &scriptedCSCLI{respond: func(_ context.Context, _ int, _ []string) ([]byte, error) {
		return []byte("can't find collections/nope"), errLiveCSCLI
	}}
	_, err := RunHubCSCLI(context.Background(), exec, mirrorTestOpts(live, nil), "collections", "install", "nope")
	require.ErrorIs(t, err, errLiveCSCLI)
	require.Len(t, exec.recorded(), 1)
}

func TestRunHubCSCLIDeadParentIsNotRetried(t *testing.T) {
	t.Parallel()
	_, live := mirrorTestSetup(t)
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	exec := &scriptedCSCLI{respond: func(_ context.Context, _ int, _ []string) ([]byte, error) {
		cancel()
		return []byte(dialFailureText), errLiveCSCLI
	}}
	_, err := RunHubCSCLI(parent, exec, mirrorTestOpts(live, nil), "collections", "install", "demo")
	require.ErrorIs(t, err, errLiveCSCLI)
	require.Len(t, exec.recorded(), 1)
}

func TestRunHubCSCLIWithoutMirrorConfigIsNotRetried(t *testing.T) {
	t.Parallel()
	_, live := mirrorTestSetup(t)
	cases := map[string]HubCSCLIOptions{
		"empty template":    {ConfigPath: live},
		"empty config path": {Template: testMirrorTemplate},
	}
	for name, opts := range cases {
		exec := &scriptedCSCLI{respond: func(_ context.Context, _ int, _ []string) ([]byte, error) {
			return []byte(dialFailureText), errLiveCSCLI
		}}
		_, err := RunHubCSCLI(context.Background(), exec, opts, "collections", "install", "demo")
		require.ErrorIs(t, err, errLiveCSCLI, name)
		require.Len(t, exec.recorded(), 1, name)
	}
}

func TestRunHubCSCLIOperatorTemplateFailsClosed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const operatorConfig = "cscli:\n  hub_branch: master\n  __hub_url_template__: \"https://hub.example.org/%s/%s\"\n"
	live := writeCSCLIConfig(t, dir, operatorConfig)
	mem := NewMirrorMemory(time.Minute, nil)
	exec := &scriptedCSCLI{respond: func(_ context.Context, _ int, _ []string) ([]byte, error) {
		return []byte(dialFailureText), errLiveCSCLI
	}}

	_, err := RunHubCSCLI(context.Background(), exec, mirrorTestOpts(live, mem), "collections", "install", "demo")
	require.ErrorIs(t, err, errLiveCSCLI)
	require.Len(t, exec.recorded(), 1, "the operator's own hub is never replaced by the mirror")
	require.False(t, mem.Active())
	raw, readErr := os.ReadFile(live)
	require.NoError(t, readErr)
	require.Equal(t, operatorConfig, string(raw))
}

func TestRunHubCSCLILocalOperatorTemplateFailsClosed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	live := writeCSCLIConfig(t, dir, "cscli:\n  hub_branch: master\n")
	const localConfig = "cscli:\n  __hub_url_template__: \"https://hub.example.org/%s/%s\"\n"
	require.NoError(t, os.WriteFile(live+".local", []byte(localConfig), 0o600))
	mem := NewMirrorMemory(time.Minute, nil)
	exec := &scriptedCSCLI{respond: func(_ context.Context, _ int, _ []string) ([]byte, error) {
		return []byte(dialFailureText), errLiveCSCLI
	}}

	_, err := RunHubCSCLI(context.Background(), exec, mirrorTestOpts(live, mem), "collections", "install", "demo")
	require.ErrorIs(t, err, errLiveCSCLI)
	require.Len(t, exec.recorded(), 1, "a .local template that the mirror would override is never retried")
	require.False(t, mem.Active())
	require.Equal(t, []string{"config.yaml", "config.yaml.local"}, dirEntryNames(t, dir), "no derived copy is left behind")
}

func TestHubServiceMirrorTemplateLogsUnusableBaseOnce(t *testing.T) {
	// Not parallel: the test swaps the global logger output.
	var logs bytes.Buffer
	logger.Init(false, &logs)
	t.Cleanup(func() { logger.Init(false, os.Stdout) })

	svc, _ := mirrorTestSetup(t)
	svc.MirrorBaseURL = "https://example.com/hub/master"
	for range 3 {
		opts := svc.MirrorOptions()
		require.Equal(t, builtinMirrorTemplate, opts.Template)
	}
	require.Equal(t, 1, strings.Count(logs.String(), "hub mirror base not usable"), "a repeated call does not repeat the warning")

	svc.MirrorBaseURL = "https://raw.githubusercontent.com/example/hub/master"
	require.Equal(t, "https://raw.githubusercontent.com/example/hub/%s/%s", svc.MirrorOptions().Template)
	require.Equal(t, 1, strings.Count(logs.String(), "hub mirror base not usable"), "a usable base logs nothing")

	svc.MirrorBaseURL = "https://example.com/other/master"
	require.Equal(t, builtinMirrorTemplate, svc.MirrorOptions().Template)
	require.Equal(t, 2, strings.Count(logs.String(), "hub mirror base not usable"), "a new unusable base is named once")
}

func TestRunHubCSCLIMirrorFirstSkipsLiveWhileWindowIsOpen(t *testing.T) {
	t.Parallel()
	_, live := mirrorTestSetup(t)
	mem := NewMirrorMemory(time.Minute, nil)
	mem.Remember()
	exec := &scriptedCSCLI{respond: func(_ context.Context, n int, args []string) ([]byte, error) {
		require.Equal(t, 0, n, "the live attempt is skipped")
		require.True(t, isDerivedConfig(configFlag(args)))
		return []byte("installed"), nil
	}}

	out, err := RunHubCSCLI(context.Background(), exec, mirrorTestOpts(live, mem), "collections", "install", "demo")
	require.NoError(t, err)
	require.Equal(t, "installed", string(out))
	require.Len(t, exec.recorded(), 1)
	require.True(t, mem.Active())
}

func TestRunHubCSCLIMirrorFailureRescuedByLiveClearsWindow(t *testing.T) {
	t.Parallel()
	_, live := mirrorTestSetup(t)
	mem := NewMirrorMemory(time.Minute, nil)
	mem.Remember()
	exec := &scriptedCSCLI{respond: func(_ context.Context, n int, args []string) ([]byte, error) {
		switch n {
		case 0:
			require.True(t, isDerivedConfig(configFlag(args)))
			return []byte(dialFailureText), errMirrorCSCLI
		case 1:
			require.Equal(t, live, configFlag(args))
			require.Equal(t, []string{"hub", "update"}, subcommand(args))
			return nil, nil
		case 2:
			require.Equal(t, live, configFlag(args))
			return []byte("installed"), nil
		}
		t.Fatalf("unexpected call %d: %v", n, args)
		return nil, nil
	}}

	out, err := RunHubCSCLI(context.Background(), exec, mirrorTestOpts(live, mem), "collections", "install", "demo")
	require.NoError(t, err)
	require.Equal(t, "installed", string(out))
	require.Len(t, exec.recorded(), 3)
	require.False(t, mem.Active(), "the live config answering again closes the window")
}

func TestRunHubCSCLIMirrorAndLiveFailureKeepsWindow(t *testing.T) {
	t.Parallel()
	_, live := mirrorTestSetup(t)
	mem := NewMirrorMemory(time.Minute, nil)
	mem.Remember()
	exec := &scriptedCSCLI{respond: func(_ context.Context, n int, _ []string) ([]byte, error) {
		if n == 0 {
			return []byte(dialFailureText), errMirrorCSCLI
		}
		return []byte(dialFailureText), errLiveCSCLI
	}}

	_, err := RunHubCSCLI(context.Background(), exec, mirrorTestOpts(live, mem), "hub", "update")
	require.ErrorIs(t, err, errMirrorCSCLI)
	require.ErrorIs(t, err, errLiveCSCLI)
	require.Len(t, exec.recorded(), 2)
	require.True(t, mem.Active())
}

func TestRunHubCSCLIFailedMirrorUpdateEndsRetry(t *testing.T) {
	t.Parallel()
	_, live := mirrorTestSetup(t)
	mem := NewMirrorMemory(time.Minute, nil)
	exec := &scriptedCSCLI{respond: func(_ context.Context, n int, _ []string) ([]byte, error) {
		if n == 0 {
			return []byte(dialFailureText), errLiveCSCLI
		}
		return []byte(dialFailureText), errMirrorCSCLI
	}}

	_, err := RunHubCSCLI(context.Background(), exec, mirrorTestOpts(live, mem), "collections", "install", "demo")
	require.ErrorIs(t, err, errLiveCSCLI)
	require.ErrorIs(t, err, errMirrorCSCLI)
	calls := exec.recorded()
	require.Len(t, calls, 2, "the install does not run when the mirror index refresh fails")
	require.Equal(t, []string{"hub", "update"}, subcommand(calls[1]))
	require.False(t, mem.Active())
}

func TestHubServiceMirrorOptionsUseBuiltInMirrorAndSharedMemory(t *testing.T) {
	t.Parallel()
	svc, live := mirrorTestSetup(t)
	opts := svc.MirrorOptions()
	require.Equal(t, live, opts.ConfigPath)
	require.Equal(t, testMirrorTemplate, opts.Template)
	require.NotNil(t, opts.Memory)
	require.Same(t, opts.Memory, svc.MirrorOptions().Memory)
}

func TestNewHubServiceSweepsMirrorLeftovers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "config"), 0o750))
	for _, rel := range []string{
		".charon-hub-mirror-a.yaml",
		".charon-hub-mirror-a.yaml.local",
		filepath.Join("config", ".charon-hub-mirror-b.yaml"),
		filepath.Join("config", ".CHARON-HUB-MIRROR-c.YAML.local"),
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, rel), []byte("x"), 0o600))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("cscli: {}\n"), 0o600))

	NewHubService(nil, nil, dir)

	require.Equal(t, []string{"config", "config.yaml"}, dirEntryNames(t, dir))
	require.Empty(t, dirEntryNames(t, filepath.Join(dir, "config")))
}
