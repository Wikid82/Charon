package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// bootSubprocessTimeout bounds a boot run. A regression that keeps the server running must fail the test, not hang it.
const bootSubprocessTimeout = 60 * time.Second

// newBootDatabasePath returns a fresh temporary directory and the database path inside its data folder. The data
// folder exists, so a seeded database can be written before the boot runs.
func newBootDatabasePath(t *testing.T) (tmp, dbPath string) {
	t.Helper()
	tmp = t.TempDir()
	dbPath = filepath.Join(tmp, "data", "test.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}
	return tmp, dbPath
}

// runBootSubprocess re-runs this test binary with only testName selected and childFlag set, so the named test calls
// main and the process exits the way a production boot does. It returns the combined output and the run error.
func runBootSubprocess(t *testing.T, testName, childFlag, tmp, dbPath string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), bootSubprocessTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+testName+"$") //nolint:gosec // G204: Test subprocess pattern using os.Args[0] is safe
	cmd.Dir = tmp
	cmd.Env = append(os.Environ(),
		childFlag+"=1",
		"CHARON_DB_PATH="+dbPath,
		"CHARON_HTTP_PORT=0",
		"CHARON_EMERGENCY_SERVER_ENABLED=false",
		"CHARON_CADDY_CONFIG_DIR="+filepath.Join(tmp, "caddy"),
		"CHARON_IMPORT_DIR="+filepath.Join(tmp, "imports"),
		"CHARON_IMPORT_CADDYFILE="+filepath.Join(tmp, "imports", "does-not-exist", "Caddyfile"),
		"CHARON_FRONTEND_DIR="+filepath.Join(tmp, "frontend", "dist"),
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// requireBootExitedNonZero fails the test unless the boot subprocess ended with a non-zero exit status.
func requireBootExitedNonZero(t *testing.T, out string, err error) {
	t.Helper()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() == 0 {
		t.Fatalf("expected the boot to exit non-zero; err=%v; output=%s", err, out)
	}
}
