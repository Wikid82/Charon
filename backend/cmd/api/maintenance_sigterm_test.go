package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	glebarez "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/dbmaint"
)

// The stop tests run the real main() in a child process against a legacy
// scratch database that a boot-time conversion takes, send it a signal in the
// middle of the conversion and check what the plan (3.4 step 9, Phase 5)
// promises: whatever the outcome, the database is intact and the following
// boot completes the conversion within MaxConvertAttempts.
//
// Caddy's admin API is fixed to port 2019 by the SSRF policy; a stub listens on
// 127.0.0.2:2019 so the test never touches a Caddy running on the host. The
// stub lives in the child (TestHelperMainWithCaddyStub).

const (
	envMaintMain    = "CHARON_TEST_MAINT_MAIN"
	stubBindFailed  = 42
	lineStarted     = "database optimization started"
	lineFinished    = "database optimization finished"
	childWait       = 3 * time.Minute
	sigtermGraceMax = 30 * time.Second
)

// TestHelperMainWithCaddyStub is the child process: a Caddy admin stub plus the
// real main(). It does nothing in a normal test run.
func TestHelperMainWithCaddyStub(t *testing.T) {
	if os.Getenv(envMaintMain) != "1" {
		t.Skip("helper process for the maintenance stop tests")
	}
	ln, err := net.Listen("tcp", "127.0.0.2:2019")
	if err != nil {
		fmt.Fprintf(os.Stderr, "caddy stub cannot listen: %v\n", err)
		os.Exit(stubBindFailed)
	}
	go func() {
		srv := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/load" {
				w.WriteHeader(http.StatusOK)
				return
			}
			_, _ = w.Write([]byte(`{}`))
		})}
		_ = srv.Serve(ln)
	}()
	os.Args = []string{"charon"}
	main()
}

// charonChild is one run of the helper process.
type charonChild struct {
	t     *testing.T
	cmd   *exec.Cmd
	lines chan string
	done  chan error
}

func startCharon(t *testing.T, dir, dbPath string, httpPort int) *charonChild {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperMainWithCaddyStub$") //nolint:gosec // G204: re-executes this test binary
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		envMaintMain+"=1",
		"CHARON_ENV=development",
		"CHARON_HTTP_PORT="+fmt.Sprint(httpPort),
		"CHARON_DB_PATH="+dbPath,
		"CHARON_CADDY_ADMIN_API=http://127.0.0.2:2019",
		"CHARON_SSRF_INTERNAL_HOST_ALLOWLIST=127.0.0.2",
		"CHARON_CADDY_CONFIG_DIR="+filepath.Join(dir, "caddy"),
		"CHARON_IMPORT_DIR="+filepath.Join(dir, "imports"),
		"CHARON_FRONTEND_DIR="+filepath.Join(dir, "frontend"),
		"CHARON_CADDY_LOG_DIR="+filepath.Join(dir, "caddylogs"),
		"CHARON_CROWDSEC_LOG_DIR="+filepath.Join(dir, "crowdseclogs"),
		"CHARON_JWT_SECRET=maintenance-stop-test-secret",
		"CHARON_EMERGENCY_SERVER_ENABLED=false",
		"CHARON_DB_COMPACT_ON_START=auto",
		"SQLITE_TMPDIR=", // main points it at <data>/.tmp itself
	)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.Stderr = cmd.Stdout
	require.NoError(t, cmd.Start())

	c := &charonChild{t: t, cmd: cmd, lines: make(chan string, 4096), done: make(chan error, 1)}
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			select {
			case c.lines <- sc.Text():
			default: // never block the child on a full buffer
			}
		}
		c.done <- cmd.Wait()
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
	})
	return c
}

// waitLine blocks until a line containing substr appears. It reports false when
// the process exits first.
func (c *charonChild) waitLine(substr string, timeout time.Duration) bool {
	c.t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case line := <-c.lines:
			if strings.Contains(line, substr) {
				return true
			}
		case err := <-c.done:
			c.done <- err // keep the exit status for wait
			return false
		case <-deadline:
			c.t.Fatalf("timed out waiting for %q", substr)
		}
	}
}

// wait returns the exit error of the process.
func (c *charonChild) wait(timeout time.Duration) error {
	c.t.Helper()
	select {
	case err := <-c.done:
		return err
	case <-time.After(timeout):
		_ = c.cmd.Process.Kill()
		c.t.Fatal("the process did not exit in time")
		return nil
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port
}

// seedLegacyDatabase writes a WAL database in auto_vacuum=none mode that a boot
// conversion takes: about 240 MB of which two thirds are free.
func seedLegacyDatabase(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	db, err := sql.Open(glebarez.DriverName, path)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"PRAGMA journal_mode=WAL",
		"CREATE TABLE bulk (id INTEGER PRIMARY KEY, pad BLOB)",
		"WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c WHERE x<2400) " +
			"INSERT INTO bulk(pad) SELECT zeroblob(100000) FROM c",
		"DELETE FROM bulk WHERE id % 3 <> 0",
	} {
		_, err = db.Exec(stmt)
		require.NoError(t, err, stmt)
	}
	var busy, logFrames, checkpointed int64
	require.NoError(t, db.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointed))
	require.NoError(t, db.Close())
}

// dbState is what the test reads from the database file while no process has
// it open.
type dbState struct {
	integrity  string
	autoVacuum int
	size       int64
	marker     bool
	attempts   int
	last       *dbmaint.LastResult
}

func readState(t *testing.T, path string) dbState {
	t.Helper()
	db, err := sql.Open(glebarez.DriverName, path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)

	var st dbState
	require.NoError(t, db.QueryRow("PRAGMA integrity_check").Scan(&st.integrity))
	require.NoError(t, db.QueryRow("PRAGMA auto_vacuum").Scan(&st.autoVacuum))

	var markerValue string
	switch markerErr := db.QueryRow(`SELECT value FROM settings WHERE "key" = 'maintenance.in_progress'`).Scan(&markerValue); {
	case markerErr == nil:
		st.marker = true
	case markerErr != sql.ErrNoRows:
		require.NoError(t, markerErr)
	}

	fileID, err := dbmaint.FileID(path)
	require.NoError(t, err)
	// Peek never consumes the marker, so reading does not change what the next
	// boot will see.
	state, err := dbmaint.NewStore(db).Peek(context.Background(), fileID)
	require.NoError(t, err)
	st.attempts, st.last = state.Attempts, state.LastResult

	_, err = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)
	st.size = info.Size()
	return st
}

type stopCase struct {
	name   string
	signal syscall.Signal
}

func TestMaintenance_StopDuringConversionIsSafeAndTheNextBootConverts(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 240 MB scratch database and runs the server three times")
	}
	for _, tc := range []stopCase{
		{"SIGTERM", syscall.SIGTERM},
		{"SIGKILL", syscall.SIGKILL}, // docker stop -t 1
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			dbPath := filepath.Join(dir, "data", "charon.db")
			seedLegacyDatabase(t, dbPath)
			seeded := fileSizeOf(t, dbPath)

			// Boot 1: stop it as soon as the conversion has started.
			first := startCharon(t, dir, dbPath, freePort(t))
			if !first.waitLine(lineStarted, childWait) {
				if exitCode(first.wait(time.Second)) == stubBindFailed {
					t.Skip("127.0.0.2:2019 is not available for the Caddy stub")
				}
				t.Fatal("the server exited before the conversion started")
			}
			// The VACUUM temp file must be on the data volume (SQLITE_TMPDIR was set
			// before the first open); SQLite unlinks it at once, so only the
			// process's open descriptors show it.
			assert.True(t, waitTempFileIn(first.cmd.Process.Pid, filepath.Join(dir, "data", ".tmp"), 10*time.Second),
				"the conversion's temp file lives under <data>/.tmp")
			require.NoError(t, first.cmd.Process.Signal(tc.signal))
			_ = first.wait(sigtermGraceMax)

			stopped := readState(t, dbPath)
			t.Logf("after %s: auto_vacuum=%d marker=%v attempts=%d last=%+v", tc.name, stopped.autoVacuum, stopped.marker, stopped.attempts, stopped.last)
			require.Equal(t, "ok", stopped.integrity, "the database is intact after a stop mid-conversion")
			assert.Zero(t, stopped.attempts, "the counter is incremented only at the next boot")

			switch {
			case stopped.autoVacuum == dbmaint.AutoVacuumNone && !stopped.marker &&
				stopped.last != nil && stopped.last.Outcome == dbmaint.ResultInterrupted:
				t.Log("outcome (a): mode 0, interrupted, marker cleared")
				assert.Equal(t, syscall.SIGTERM, tc.signal, "only an orderly stop records an interruption")
			case stopped.autoVacuum == dbmaint.AutoVacuumIncremental && !stopped.marker &&
				stopped.last != nil && stopped.last.Outcome.Converted():
				t.Log("outcome (b): mode 2, converted, marker cleared")
				assert.Equal(t, syscall.SIGTERM, tc.signal)
			case stopped.marker:
				t.Log("outcome (c): marker left; the next boot counts one attempt")
			default:
				t.Fatalf("unexpected state after %s: %+v (last %+v)", tc.name, stopped, stopped.last)
			}
			if tc.signal == syscall.SIGKILL {
				assert.True(t, stopped.marker || stopped.autoVacuum == dbmaint.AutoVacuumIncremental,
					"a killed conversion leaves the marker (or had already finished)")
			}

			// Following boots: the conversion completes within MaxConvertAttempts.
			converted := stopped.autoVacuum == dbmaint.AutoVacuumIncremental && !stopped.marker
			for boot := 2; !converted && boot <= dbmaint.MaxConvertAttempts+1; boot++ {
				next := startCharon(t, dir, dbPath, freePort(t))
				require.True(t, next.waitLine(lineFinished, childWait), "boot %d did not finish the conversion", boot)
				require.NoError(t, next.cmd.Process.Signal(syscall.SIGTERM))
				_ = next.wait(sigtermGraceMax)

				st := readState(t, dbPath)
				require.Equal(t, "ok", st.integrity, "boot %d", boot)
				converted = st.autoVacuum == dbmaint.AutoVacuumIncremental && !st.marker
				t.Logf("boot %d: auto_vacuum=%d marker=%v attempts=%d last=%+v", boot, st.autoVacuum, st.marker, st.attempts, st.last)
			}
			require.True(t, converted, "the conversion completes within MaxConvertAttempts boots")

			final := readState(t, dbPath)
			assert.Equal(t, "ok", final.integrity)
			assert.Less(t, final.size, seeded/2, "the file shrank")
			assert.Zero(t, final.attempts, "a successful conversion resets the counter")
			require.NotNil(t, final.last)
			assert.True(t, final.last.Outcome.Converted())
		})
	}
}

// waitTempFileIn polls the process's open descriptors for a SQLite temp file in
// dir. Where /proc is unavailable it reports true (nothing to observe).
func waitTempFileIn(pid int, dir string, timeout time.Duration) bool {
	fdDir := fmt.Sprintf("/proc/%d/fd", pid)
	if _, err := os.ReadDir(fdDir); err != nil {
		return true
	}
	prefix := filepath.Join(dir, "etilqs_")
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(fdDir)
		if err != nil {
			return false
		}
		for _, e := range entries {
			if link, linkErr := os.Readlink(filepath.Join(fdDir, e.Name())); linkErr == nil && strings.HasPrefix(link, prefix) {
				return true
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func fileSizeOf(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Size()
}

func exitCode(err error) int {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return 0
}
