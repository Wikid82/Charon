package database

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// backendModuleRoot resolves the backend module's root directory (the
// directory containing go.mod, i.e. the parent of internal/database) from
// this test file's own source path, so building the sibling
// cmd/pending-restore-harness binary doesn't depend on the test runner's
// working directory.
func backendModuleRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must resolve this file's path")
	// thisFile: <root>/backend/internal/database/pending_restore_process_test.go
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

// harnessGOCOVERDIREnv is set by scripts/go-test-coverage.sh to a durable
// directory when it wants this test's separately-compiled/executed
// subprocess (see buildPendingRestoreHarness) to report its coverage back
// into the aggregate coverage.txt pipeline. Go's default -coverprofile
// instrumentation has no visibility into a child OS process, so without
// this the harness binary's 100%-exercised code (main.go, prep, boot) would
// always show as 0% patch coverage despite being fully driven by
// TestPendingRestoreBootSwap_AcrossRealProcessBoundary and its corrupt-file
// sibling below. See scripts/go-test-coverage.sh for the merge step that
// converts this directory's binary coverage data into text format and
// appends it to backend/coverage.txt.
const harnessGOCOVERDIREnv = "CHARON_HARNESS_GOCOVERDIR"

// harnessCoverDir resolves the directory the compiled harness binary should
// write GOCOVERDIR coverage data into. When CHARON_HARNESS_GOCOVERDIR is set
// (by the coverage pipeline) that durable directory is used so data survives
// past this test's own cleanup; otherwise a throwaway t.TempDir() is used
// (e.g. for a bare `go test ./internal/database/...` run where no one is
// collecting cross-process coverage).
func harnessCoverDir(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv(harnessGOCOVERDIREnv); dir != "" {
		require.NoError(t, os.MkdirAll(dir, 0o755))
		return dir
	}
	return t.TempDir()
}

// buildPendingRestoreHarness compiles cmd/pending-restore-harness (test
// infrastructure only — see that command's doc comment) to a temp binary and
// returns its path. Building fresh per test keeps this independent of
// whatever else may or may not already be on PATH/GOBIN, at the cost of a
// couple of seconds of compile time, which is acceptable for a test that
// exists specifically to cross a real process boundary.
//
// It is always built with -cover -covermode=atomic (matching the atomic
// mode -race forces on the surrounding `go test` run) so that when the
// resulting binary is executed with GOCOVERDIR set (see harnessCoverDir),
// it emits real binary coverage data for its own source instead of the
// otherwise-permanent 0% patch coverage a separately-executed process gets.
// Building with -cover costs a little extra compile time but has no effect
// on the binary's behavior, so it's safe to do unconditionally.
func buildPendingRestoreHarness(t *testing.T) string {
	t.Helper()

	root := backendModuleRoot(t)
	binPath := filepath.Join(t.TempDir(), "pending-restore-harness")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	cmd := exec.Command("go", "build", "-cover", "-covermode=atomic", "-o", binPath, "./cmd/pending-restore-harness") // #nosec G204 -- fixed args, test-controlled
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "building pending-restore-harness failed: %s", string(output))
	require.FileExists(t, binPath)

	return binPath
}

// harnessCmd builds an *exec.Cmd for the compiled harness binary with
// GOCOVERDIR pointed at coverDir, so coverage data for this invocation is
// written there when the process exits normally (including via os.Exit,
// which the harness uses — Go's runtime coverage instrumentation registers
// an atexit-style hook that fires even on explicit os.Exit calls).
func harnessCmd(harness, coverDir string, args ...string) *exec.Cmd {
	cmd := exec.Command(harness, args...) // #nosec G204 -- test-controlled binary and args
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)
	return cmd
}

// createMarkerSQLiteFile creates a tiny valid sqlite database at path
// containing a single distinguishing marker row, so a test can tell which
// physical database file (the pre-restore "old" one vs. the validated
// "new"/replacement one) ended up installed at a given path after the
// harness runs.
func createMarkerSQLiteFile(t *testing.T, path, markerValue string) {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`CREATE TABLE marker (id INTEGER PRIMARY KEY, value TEXT)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO marker (value) VALUES (?)`, markerValue)
	require.NoError(t, err)
}

func readMarkerValue(t *testing.T, path string) string {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var value string
	require.NoError(t, db.QueryRow(`SELECT value FROM marker LIMIT 1`).Scan(&value))
	return value
}

// TestPendingRestoreBootSwap_AcrossRealProcessBoundary is required test #2b
// from the Issue #32 gap-closing QA report: existing tests
// (TestApplyPendingRestore_ValidPendingFile_SwapsIntoPlace etc.) only call
// ApplyPendingRestore directly in-process and comment that this "simulates"
// a restart. This test instead spawns the real compiled
// cmd/pending-restore-harness binary — test infrastructure that replicates
// backend/cmd/api/main.go's exact startup ordering (ApplyPendingRestore
// before database.Connect, main.go:214-218) — across two separate OS
// process invocations, so a future accidental reordering of those two
// calls in main.go would cause this test's "boot" process to stop applying
// the swap and the test would fail, not silently pass.
//
// Invocation 1 ("prep"): simulates a process that validated a replacement
// database during RestoreBackupSafe's F3 fallback, persisted the durable
// .pending-restore marker, and then exited/crashed before its live
// rehydrate could apply it — exactly the gap F3 exists to cover (spec
// §3.5).
//
// Invocation 2 ("boot"): a fresh process start. It must consume the pending
// marker BEFORE the database is ever opened for normal use, exactly as
// main.go does on every real `docker restart`.
func TestPendingRestoreBootSwap_AcrossRealProcessBoundary(t *testing.T) {
	harness := buildPendingRestoreHarness(t)
	coverDir := harnessCoverDir(t)

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "charon.db")
	sourcePath := filepath.Join(dir, "validated-replacement.db")

	// The "old" database currently live on disk before the restart.
	createMarkerSQLiteFile(t, dbPath, "old-database-pre-restart")
	// Stale -wal/-shm siblings must be cleaned up by the swap, exactly as
	// the in-process ApplyPendingRestore tests already prove for a direct
	// call — here we additionally prove it survives a real restart.
	require.NoError(t, os.WriteFile(dbPath+"-wal", []byte("stale wal"), 0o600))
	require.NoError(t, os.WriteFile(dbPath+"-shm", []byte("stale shm"), 0o600))

	// The already-validated replacement database RestoreBackupSafe would
	// have produced.
	createMarkerSQLiteFile(t, sourcePath, "new-database-post-restart")

	// --- Invocation 1: "prep" (the crashed/killed restore process) ---
	prepCmd := harnessCmd(harness, coverDir, "-mode=prep", "-db="+dbPath, "-source="+sourcePath)
	prepOutput, err := prepCmd.CombinedOutput()
	require.NoError(t, err, "prep invocation failed: %s", string(prepOutput))
	require.Contains(t, string(prepOutput), "RESULT: prep-ok")
	require.FileExists(t, dbPath+".pending-restore", "prep invocation must leave a durable pending-restore marker on disk")

	// Between the two invocations, dbPath itself must still be the OLD
	// database — nothing may touch it until the next boot's
	// ApplyPendingRestore call.
	require.Equal(t, "old-database-pre-restart", readMarkerValue(t, dbPath))

	// --- Invocation 2: "boot" (a real fresh process start) ---
	bootCmd := harnessCmd(harness, coverDir, "-mode=boot", "-db="+dbPath)
	bootOutput, err := bootCmd.CombinedOutput()
	require.NoError(t, err, "boot invocation failed: %s", string(bootOutput))
	require.True(t, strings.Contains(string(bootOutput), "RESULT: boot-ok-pending-restore-applied-or-absent"),
		"boot invocation must report a clean pending-restore outcome, got: %s", string(bootOutput))

	// The defining assertion: after a REAL process restart (not an
	// in-process function call), dbPath must now be the validated
	// replacement database, and the durable marker must be gone.
	require.NoFileExists(t, dbPath+".pending-restore")
	require.NoFileExists(t, dbPath+"-wal")
	require.NoFileExists(t, dbPath+"-shm")
	require.Equal(t, "new-database-post-restart", readMarkerValue(t, dbPath),
		"a fresh process boot must have swapped the validated replacement database into place before ever opening dbPath for normal use")
}

// TestPendingRestoreBootSwap_CorruptPendingFile_AcrossRealProcessBoundary
// mirrors the above for the F3 failure path: a pending-restore file that
// fails its independent integrity re-check must be quarantined by the real
// "boot" process, and the prior (old) database must remain the one actually
// serving traffic — proven across the same real process boundary as the
// happy path above, not merely by an in-process call.
func TestPendingRestoreBootSwap_CorruptPendingFile_AcrossRealProcessBoundary(t *testing.T) {
	harness := buildPendingRestoreHarness(t)
	coverDir := harnessCoverDir(t)

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "charon.db")
	corruptSourcePath := filepath.Join(dir, "corrupt-replacement.db")

	createMarkerSQLiteFile(t, dbPath, "old-database-pre-restart")
	require.NoError(t, os.WriteFile(corruptSourcePath, []byte("this is not a valid sqlite database file"), 0o600))

	prepCmd := harnessCmd(harness, coverDir, "-mode=prep", "-db="+dbPath, "-source="+corruptSourcePath)
	prepOutput, err := prepCmd.CombinedOutput()
	require.NoError(t, err, "prep invocation failed: %s", string(prepOutput))
	require.FileExists(t, dbPath+".pending-restore")

	bootCmd := harnessCmd(harness, coverDir, "-mode=boot", "-db="+dbPath)
	bootOutput, err := bootCmd.CombinedOutput()
	require.NoError(t, err, "boot invocation itself must still succeed (a rejected swap is not a boot failure): %s", string(bootOutput))
	require.Contains(t, string(bootOutput), "RESULT: boot-ok-pending-restore-failed")

	require.NoFileExists(t, dbPath+".pending-restore", "a corrupt pending file must be quarantined, not left in place")
	require.FileExists(t, dbPath+".pending-restore.failed")
	require.Equal(t, "old-database-pre-restart", readMarkerValue(t, dbPath),
		"the prior database must remain completely untouched and authoritative after a real process boot rejects a corrupt pending file")
}

// TestPendingRestoreHarness_ErrorPaths drives cmd/pending-restore-harness
// through every one of its own error branches (usage errors, prep's
// open/create/copy failures, boot's usage error and database.Connect
// failure) directly, rather than only the two happy/quarantine paths above.
// These are real, distinct process exit codes and RESULT lines a future
// change to the harness (or to its production-mirroring call sites in
// main.go) could silently break, and — as test infrastructure that is
// itself part of this feature's patch — closing these branches is what
// gets the harness binary counted as genuinely covered rather than merely
// built.
func TestPendingRestoreHarness_ErrorPaths(t *testing.T) {
	harness := buildPendingRestoreHarness(t)
	coverDir := harnessCoverDir(t)

	run := func(args ...string) (output string, exitCode int) {
		cmd := harnessCmd(harness, coverDir, args...)
		out, err := cmd.CombinedOutput()
		output = string(out)
		if err == nil {
			return output, 0
		}
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr, "harness must exit cleanly (possibly non-zero), not fail to even run: %s", output)
		return output, exitErr.ExitCode()
	}

	t.Run("no mode flag is a usage error", func(t *testing.T) {
		output, code := run()
		require.Equal(t, 2, code)
		require.Contains(t, output, "RESULT: usage-error: -mode must be prep or boot")
	})

	t.Run("unrecognized mode is a usage error", func(t *testing.T) {
		output, code := run("-mode=bogus")
		require.Equal(t, 2, code)
		require.Contains(t, output, "RESULT: usage-error: -mode must be prep or boot")
	})

	t.Run("prep missing -db and -source", func(t *testing.T) {
		output, code := run("-mode=prep", "-db="+filepath.Join(t.TempDir(), "charon.db"))
		require.Equal(t, 1, code)
		require.Contains(t, output, "RESULT: prep-failed: -db and -source are both required in prep mode")
	})

	t.Run("prep source does not exist", func(t *testing.T) {
		dir := t.TempDir()
		output, code := run("-mode=prep", "-db="+filepath.Join(dir, "charon.db"), "-source="+filepath.Join(dir, "no-such-source.db"))
		require.Equal(t, 1, code)
		require.Contains(t, output, "RESULT: prep-failed: open source database:")
	})

	t.Run("prep source is unreadable as a file (io.Copy fails)", func(t *testing.T) {
		dir := t.TempDir()
		// A directory opens successfully (os.Open succeeds on directories)
		// but reading from it during io.Copy fails, exercising prep's copy
		// error branch distinctly from its open error branch above.
		sourceDir := filepath.Join(dir, "not-a-file")
		require.NoError(t, os.Mkdir(sourceDir, 0o755))

		output, code := run("-mode=prep", "-db="+filepath.Join(dir, "charon.db"), "-source="+sourceDir)
		require.Equal(t, 1, code)
		require.Contains(t, output, "RESULT: prep-failed: write pending-restore file:")
	})

	t.Run("prep pending-restore file cannot be created", func(t *testing.T) {
		dir := t.TempDir()
		sourcePath := filepath.Join(dir, "source.db")
		require.NoError(t, os.WriteFile(sourcePath, []byte("data"), 0o600))

		// dbPath's parent directory doesn't exist, so dbPath+".pending-restore"
		// can't be created.
		output, code := run("-mode=prep", "-db="+filepath.Join(dir, "no-such-parent", "charon.db"), "-source="+sourcePath)
		require.Equal(t, 1, code)
		require.Contains(t, output, "RESULT: prep-failed: create pending-restore file:")
	})

	t.Run("boot missing -db", func(t *testing.T) {
		output, code := run("-mode=boot")
		require.Equal(t, 2, code)
		require.Contains(t, output, "RESULT: usage-error: -db is required in boot mode")
	})

	t.Run("boot database.Connect fails", func(t *testing.T) {
		dir := t.TempDir()
		// A directory can never be opened as a sqlite database file, so
		// database.Connect fails deterministically without needing to
		// corrupt anything.
		output, code := run("-mode=boot", "-db="+dir)
		require.Equal(t, 1, code)
		require.Contains(t, output, "RESULT: connect-failed: open database:")
	})
}
