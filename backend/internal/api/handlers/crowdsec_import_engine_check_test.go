package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

const (
	engineCheckBinPath     = "/usr/bin/crowdsec"
	importConfigInvalidMsg = "CrowdSec could not load the imported configuration. Check that config.yaml has a config_paths section and that the folders it names exist."
	noConfigPathsConfig    = "api:\n  server:\n    listen_uri: 0.0.0.0:8080\n"
)

// engineCheckRecorder stands in for the handler's CommandExecutor during an import. It records each call and
// the deadline of its context, then answers with run.
type engineCheckRecorder struct {
	calls       [][]string
	deadline    time.Time
	hasDeadline bool
	run         func(ctx context.Context) ([]byte, error)
}

func (r *engineCheckRecorder) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	r.deadline, r.hasDeadline = ctx.Deadline()
	if r.run == nil {
		return nil, nil
	}
	return r.run(ctx)
}

// answer returns a recorder whose check always returns out and err.
func answer(out []byte, err error) *engineCheckRecorder {
	return &engineCheckRecorder{run: func(context.Context) ([]byte, error) { return out, err }}
}

// shellExit runs a real shell that writes msg to stderr and exits with code. The error is the *exec.ExitError
// an engine's own non-zero exit produces.
func shellExit(t *testing.T, code int, msg string) ([]byte, error) {
	t.Helper()
	//nolint:gosec // G204: fixed test script; arguments are passed as positional parameters
	cmd := exec.Command("sh", "-c", `printf '%s\n' "$1" >&2; exit "$2"`, "sh", msg, strconv.Itoa(code))
	return cmd.CombinedOutput()
}

// treeContents maps every regular file under root to its bytes, keyed by relative path.
func treeContents(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	require.NoError(t, filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, readErr := os.ReadFile(p) //nolint:gosec // G304: test fixture under t.TempDir
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		out[rel] = string(b)
		return nil
	}))
	return out
}

// newEngineCheckRouter builds an import router whose engine check runs through cmd against binPath.
func newEngineCheckRouter(t *testing.T, db *gorm.DB, dataDir, binPath string, cmd CommandExecutor) (*CrowdsecHandler, *gin.Engine) {
	t.Helper()
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, binPath, dataDir)
	h.CmdExec = cmd
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	return h, r
}

// importWithTree posts an archive built from files and returns the response recorder.
func importWithTree(t *testing.T, r *gin.Engine, files map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, importRequest(t, files))
	return w
}

func TestImportConfigRefusesConfigWithoutConfigPathsAndKeepsLiveConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedEngineState(t, dir)
	before := treeContents(t, dir)
	rec := answer(nil, nil)
	_, r := newEngineCheckRouter(t, OpenTestDB(t), dir, engineCheckBinPath, rec)

	w := importWithTree(t, r, map[string]string{"config.yaml": noConfigPathsConfig})

	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	require.JSONEq(t, `{"error":"config validation failed: config.yaml has no config_paths section"}`, w.Body.String())
	require.Empty(t, rec.calls, "the engine must not be asked about a configuration that is refused earlier")
	require.Equal(t, before, treeContents(t, dir), "the live configuration must be byte-identical after a refusal")
}

func TestImportConfigEngineRejectionRollsBack(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedEngineState(t, dir)
	before := treeContents(t, dir)
	out, err := shellExit(t, 1, "level=fatal msg=\"parsing /etc/secret/path: bad\"")
	_, r := newEngineCheckRouter(t, OpenTestDB(t), dir, engineCheckBinPath, answer(out, err))

	w := importWithTree(t, r, map[string]string{"config.yaml": importableConfig})

	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "import_config_invalid", body["error_code"])
	require.Equal(t, importConfigInvalidMsg, body["error"])
	require.NotContains(t, w.Body.String(), "secret", "engine output must never be returned")
	require.Equal(t, before, treeContents(t, dir), "a rejected import must restore the live configuration")
}

func TestImportConfigSkipsEngineCheckWhenItCannotRun(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		binPath string
		cmd     func(t *testing.T) CommandExecutor
		timeout time.Duration
	}{
		"missing binary": {
			binPath: filepath.Join(t.TempDir(), "no-such-crowdsec"),
			cmd:     func(*testing.T) CommandExecutor { return &RealCommandExecutor{} },
		},
		"start failure": {
			binPath: engineCheckBinPath,
			cmd: func(*testing.T) CommandExecutor {
				return answer(nil, errors.New("fork/exec: resource temporarily unavailable"))
			},
		},
		"permission denied": {
			binPath: engineCheckBinPath,
			cmd: func(*testing.T) CommandExecutor {
				return answer(nil, &exec.Error{Name: "crowdsec", Err: fs.ErrPermission})
			},
		},
		"exec format error": {
			binPath: engineCheckBinPath,
			cmd: func(*testing.T) CommandExecutor {
				return answer(nil, &exec.Error{Name: "crowdsec", Err: syscall.ENOEXEC})
			},
		},
		"killed by a signal": {
			binPath: engineCheckBinPath,
			cmd: func(t *testing.T) CommandExecutor {
				//nolint:gosec // G204: fixed test script
				out, err := exec.Command("sh", "-c", "kill -KILL $$").CombinedOutput()
				return answer(out, err)
			},
		},
		"deadline exceeded": {
			binPath: engineCheckBinPath,
			timeout: 20 * time.Millisecond,
			cmd: func(*testing.T) CommandExecutor {
				return &engineCheckRecorder{run: func(ctx context.Context) ([]byte, error) {
					<-ctx.Done()
					return nil, ctx.Err()
				}}
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			h, r := newEngineCheckRouter(t, OpenTestDB(t), dir, tc.binPath, tc.cmd(t))
			if tc.timeout > 0 {
				h.engineCheckTimeout = tc.timeout
			}

			w := importWithTree(t, r, map[string]string{"config.yaml": importableConfig})

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), `"status":"imported"`)
		})
	}
}

func TestImportConfigSkipsEngineCheckForEnvironmentalOutput(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"port in use":        "listen tcp 127.0.0.1:8080: bind: address already in use",
		"database locked":    "database is locked",
		"table locked":       "database table is locked: crowdsec_machines",
		"sqlite busy result": "error: SQLITE_BUSY",
	}
	for name, msg := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := shellExit(t, 1, msg)
			dir := t.TempDir()
			_, r := newEngineCheckRouter(t, OpenTestDB(t), dir, engineCheckBinPath, answer(out, err))

			w := importWithTree(t, r, map[string]string{"config.yaml": importableConfig})

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), `"status":"imported"`)
		})
	}
}

func TestImportConfigEngineCheckInvocation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rec := answer([]byte("ok"), nil)
	h, r := newEngineCheckRouter(t, OpenTestDB(t), dir, engineCheckBinPath, rec)

	w := importWithTree(t, r, map[string]string{"config.yaml": importableConfig})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, [][]string{{engineCheckBinPath, "-t", "-c", filepath.Join(dir, "config.yaml")}}, rec.calls)
	require.True(t, rec.hasDeadline, "the engine check must carry a deadline")
	require.Equal(t, 30*time.Second, h.engineCheckTimeout)
	require.WithinDuration(t, time.Now().Add(30*time.Second), rec.deadline, 5*time.Second)
}

func TestImportConfigWhitelistRegenerationFailureRollsBack(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedEngineState(t, dir)
	before := treeContents(t, dir)
	db := OpenTestDB(t)
	_, r := newEngineCheckRouter(t, db, dir, engineCheckBinPath, answer([]byte("ok"), nil))
	// The router migrates the whitelist table; dropping it after that makes the regeneration query fail.
	require.NoError(t, db.Migrator().DropTable(&models.CrowdSecWhitelist{}))

	w := importWithTree(t, r, map[string]string{"config.yaml": importableConfig})

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.JSONEq(t, `{"error":"failed to regenerate the whitelist file"}`, w.Body.String())
	require.Equal(t, before, treeContents(t, dir), "a failed whitelist regeneration must restore the live configuration")
}
