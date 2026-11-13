package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startAdminSocketServer serves the given handler on a unix socket inside a fresh
// temporary directory and returns the socket path. The server stops with the test.
func startAdminSocketServer(t *testing.T, handler http.Handler) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "charon-adm-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	socketPath := filepath.Join(dir, "admin.sock")
	ln, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return socketPath
}

func adminConfigHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/config/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apps":{"http":{}}}`))
	})
	mux.HandleFunc("/load", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

func TestClientDialsAdminSocket(t *testing.T) {
	socketPath := startAdminSocketServer(t, adminConfigHandler())
	client := NewClient("unix://" + socketPath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, client.Ping(ctx))

	cfg, err := client.GetConfig(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.NoError(t, client.Load(ctx, &Config{Apps: Apps{}}))
}

func TestClientAdminSocketReportsMissingSocket(t *testing.T) {
	dir, err := os.MkdirTemp("", "charon-adm-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	client := NewClient("unix://" + filepath.Join(dir, "absent.sock"))
	err = client.Ping(context.Background())
	require.Error(t, err)
}

func TestClientAdminListenMatchesEndpoint(t *testing.T) {
	assert.Equal(t, "unix//run/charon/admin.sock|0600", NewClient("unix:///run/charon/admin.sock").AdminListen())
	assert.Equal(t, "127.0.0.1:2019", NewClient("http://localhost:2019").AdminListen())
}

func TestGenerateConfigAdminListenDefaultsToSocket(t *testing.T) {
	cfg, err := GenerateConfig(nil, "/data/caddy/data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, cfg.Admin)
	assert.True(t, strings.HasPrefix(cfg.Admin.Listen, "unix//"), "admin listen = %q", cfg.Admin.Listen)
	assert.True(t, strings.HasSuffix(cfg.Admin.Listen, "/data/run/caddy-admin.sock|0600"), "admin listen = %q", cfg.Admin.Listen)
}

func TestGenerateConfigAdminListenUsesOption(t *testing.T) {
	cfg, err := GenerateConfig(nil, "/data/caddy/data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil,
		WithAdminListen("unix//tmp/x.sock|0600"))
	require.NoError(t, err)
	assert.Equal(t, "unix//tmp/x.sock|0600", cfg.Admin.Listen)
}

func TestPrepareAdminSocketCreatesPrivateDirectory(t *testing.T) {
	dir, err := os.MkdirTemp("", "charon-adm-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	socketDir := filepath.Join(dir, "run")
	require.NoError(t, PrepareAdminSocket(filepath.Join(socketDir, "admin.sock")))

	info, err := os.Stat(socketDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, fs.FileMode(0o700), info.Mode().Perm())
}

func TestPrepareAdminSocketRemovesStaleSocket(t *testing.T) {
	dir, err := os.MkdirTemp("", "charon-adm-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	socketPath := filepath.Join(dir, "admin.sock")
	ln, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	// Leave the file behind, as a process that exited without cleaning up would.
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	require.NoError(t, ln.Close())
	require.FileExists(t, socketPath)

	require.NoError(t, PrepareAdminSocket(socketPath))
	_, err = os.Lstat(socketPath)
	assert.True(t, errors.Is(err, fs.ErrNotExist), "stale socket must be removed, got %v", err)
}

func TestPrepareAdminSocketKeepsLiveSocket(t *testing.T) {
	dir, err := os.MkdirTemp("", "charon-adm-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	socketPath := filepath.Join(dir, "admin.sock")
	ln, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	require.Error(t, PrepareAdminSocket(socketPath))
	require.FileExists(t, socketPath)
}

func TestPrepareAdminSocketRefusesRegularFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "charon-adm-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	path := filepath.Join(dir, "admin.sock")
	require.NoError(t, os.WriteFile(path, []byte("data"), 0o600))

	require.Error(t, PrepareAdminSocket(path))
	require.FileExists(t, path)
}

func TestRunAdminCommandListen(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runAdminCommand("unix:///run/charon/admin.sock", []string{"listen"}, &out, &errOut)
	assert.Equal(t, 0, code)
	assert.Equal(t, "unix//run/charon/admin.sock|0600\n", out.String())
}

func TestRunAdminCommandGetPrintsBody(t *testing.T) {
	socketPath := startAdminSocketServer(t, adminConfigHandler())

	var out, errOut bytes.Buffer
	code := runAdminCommand("unix://"+socketPath, []string{"get", "/config/"}, &out, &errOut)
	assert.Equal(t, 0, code, errOut.String())
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &decoded))
	assert.Contains(t, decoded, "apps")
}

func TestRunAdminCommandGetReportsFailure(t *testing.T) {
	socketPath := startAdminSocketServer(t, adminConfigHandler())

	var out, errOut bytes.Buffer
	code := runAdminCommand("unix://"+socketPath, []string{"get", "/missing/"}, &out, &errOut)
	assert.Equal(t, 1, code)
	assert.NotEmpty(t, errOut.String())
}

func TestRunAdminCommandRejectsBadUsage(t *testing.T) {
	cases := map[string][]string{
		"no command":        {},
		"unknown command":   {"status"},
		"get without path":  {"get"},
		"get relative path": {"get", "config/"},
		"get with extra":    {"get", "/config/", "extra"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			assert.Equal(t, 2, runAdminCommand("unix:///run/x.sock", args, &out, &errOut))
			assert.NotEmpty(t, errOut.String())
		})
	}
}

func TestRunAdminCommandRejectsExtraArguments(t *testing.T) {
	for _, command := range []string{"listen", "prepare"} {
		t.Run(command, func(t *testing.T) {
			var out, errOut bytes.Buffer
			assert.Equal(t, 2, runAdminCommand("unix:///run/x.sock", []string{command, "extra"}, &out, &errOut))
			assert.Empty(t, out.String())
			assert.NotEmpty(t, errOut.String())
		})
	}
}

func TestRunAdminCommandPrepareReportsFailure(t *testing.T) {
	dir := t.TempDir()
	// A regular file where the socket directory should be makes the directory impossible to create.
	blocker := filepath.Join(dir, "run")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))

	var out, errOut bytes.Buffer
	code := runAdminCommand("unix://"+filepath.Join(blocker, "admin.sock"), []string{"prepare"}, &out, &errOut)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "create socket directory")
}

// failingWriter refuses every write, like a closed stdout.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("stdout closed") }

func TestRunAdminCommandGetReportsWriteFailure(t *testing.T) {
	socketPath := startAdminSocketServer(t, adminConfigHandler())

	var errOut bytes.Buffer
	code := runAdminCommand("unix://"+socketPath, []string{"get", "/config/"}, failingWriter{}, &errOut)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "write response")
}

func TestRunAdminCommandReportsInvalidAdminSetting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CHARON_DB_PATH", filepath.Join(dir, "charon.db"))
	t.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(dir, "caddy"))
	t.Setenv("CHARON_IMPORT_DIR", filepath.Join(dir, "imports"))
	t.Setenv("CHARON_CADDY_ADMIN_API", "http://caddy.example.com:2019")
	t.Setenv("CPM_CADDY_ADMIN_API", "")

	var out, errOut bytes.Buffer
	assert.Equal(t, 1, RunAdminCommand([]string{"listen"}, &out, &errOut))
	assert.Empty(t, out.String())
	assert.Contains(t, errOut.String(), "load config")
}

// skipIfRoot skips tests that rely on file permissions, which root bypasses.
func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("file permission checks do not apply to root")
	}
}

func TestPrepareAdminSocketReportsUnreadableDirectory(t *testing.T) {
	skipIfRoot(t)
	dir := t.TempDir()
	socketDir := filepath.Join(dir, "run")
	require.NoError(t, os.Mkdir(socketDir, 0o700))
	require.NoError(t, os.Chmod(socketDir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(socketDir, 0o700) })

	err := PrepareAdminSocket(filepath.Join(socketDir, "admin.sock"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inspect socket path")
}

// leaveStaleSocket creates a socket file at path with no process accepting on it.
func leaveStaleSocket(t *testing.T, path string) {
	t.Helper()
	ln, err := net.Listen("unix", path)
	require.NoError(t, err)
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	require.NoError(t, ln.Close())
}

func TestPrepareAdminSocketReportsUncheckableSocket(t *testing.T) {
	skipIfRoot(t)
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "admin.sock")
	leaveStaleSocket(t, socketPath)
	// Without write permission on the socket file, connecting fails with a
	// permission error rather than "connection refused", so the file is kept.
	require.NoError(t, os.Chmod(socketPath, 0o000))

	err := PrepareAdminSocket(socketPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "check socket")
	_, statErr := os.Lstat(socketPath)
	assert.NoError(t, statErr, "a socket that could not be checked must not be removed")
}

func TestPrepareAdminSocketReportsUnremovableStaleSocket(t *testing.T) {
	skipIfRoot(t)
	dir := t.TempDir()
	socketDir := filepath.Join(dir, "run")
	require.NoError(t, os.Mkdir(socketDir, 0o700))
	socketPath := filepath.Join(socketDir, "admin.sock")
	leaveStaleSocket(t, socketPath)
	// A read-only directory keeps the stale socket from being unlinked.
	require.NoError(t, os.Chmod(socketDir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(socketDir, 0o700) })

	err := PrepareAdminSocket(socketPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remove stale socket")
}

func TestClientFetchReportsUnconfiguredClient(t *testing.T) {
	_, err := (&Client{}).Fetch(context.Background(), "/config/")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base URL is not configured")
}

func TestClientFetchReportsRequestBuildFailure(t *testing.T) {
	socketPath := startAdminSocketServer(t, adminConfigHandler())
	// A nil context makes the request impossible to build.
	var noContext context.Context
	_, err := NewClient("unix://"+socketPath).Fetch(noContext, "/config/")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create request")
}

func TestClientFetchReportsMissingSocket(t *testing.T) {
	_, err := NewClient("unix://"+filepath.Join(t.TempDir(), "absent.sock")).Fetch(context.Background(), "/config/")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "execute request")
}

func TestClientFetchReportsTruncatedBody(t *testing.T) {
	socketPath := startAdminSocketServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Promise more bytes than are sent; the connection closes early.
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte(`{"apps":`))
	}))

	_, err := NewClient("unix://"+socketPath).Fetch(context.Background(), "/config/")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read response")
}

func TestRunAdminCommandPrepareIsNoopForTCP(t *testing.T) {
	var out, errOut bytes.Buffer
	assert.Equal(t, 0, runAdminCommand("http://localhost:2019", []string{"prepare"}, &out, &errOut))
}

func TestRunAdminCommandPrepareCreatesSocketDirectory(t *testing.T) {
	dir, err := os.MkdirTemp("", "charon-adm-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	var out, errOut bytes.Buffer
	code := runAdminCommand("unix://"+filepath.Join(dir, "run", "admin.sock"), []string{"prepare"}, &out, &errOut)
	assert.Equal(t, 0, code, errOut.String())
	assert.DirExists(t, filepath.Join(dir, "run"))
}

func TestRunAdminCommandReadsAdminAPIFromEnvironment(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CHARON_DB_PATH", filepath.Join(dir, "charon.db"))
	t.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(dir, "caddy"))
	t.Setenv("CHARON_IMPORT_DIR", filepath.Join(dir, "imports"))
	t.Setenv("CHARON_CADDY_ADMIN_API", "unix://"+filepath.Join(dir, "admin.sock"))
	t.Setenv("CPM_CADDY_ADMIN_API", "")

	var out, errOut bytes.Buffer
	code := RunAdminCommand([]string{"listen"}, &out, &errOut)
	assert.Equal(t, 0, code, errOut.String())
	assert.Equal(t, "unix/"+filepath.Join(dir, "admin.sock")+"|0600\n", out.String())
}
