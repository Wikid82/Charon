package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain_CaddyAdminCommandRunsBeforeBoot_Subprocess runs the binary as `charon caddy-admin listen`. The
// entrypoint relies on this path: it must print only the admin.listen value and exit 0 without starting the server.
func TestMain_CaddyAdminCommandRunsBeforeBoot_Subprocess(t *testing.T) {
	if os.Getenv("CHARON_TEST_RUN_CADDY_ADMIN") == "1" {
		os.Args = []string{"charon", "caddy-admin", "listen"}
		main()
		return
	}

	tmp := t.TempDir()
	socketPath := filepath.Join(tmp, "run", "admin.sock")
	ctx, cancel := context.WithTimeout(context.Background(), bootSubprocessTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMain_CaddyAdminCommandRunsBeforeBoot_Subprocess$") //nolint:gosec // G204: re-executes this test binary
	cmd.Dir = tmp
	cmd.Env = append(os.Environ(),
		"CHARON_TEST_RUN_CADDY_ADMIN=1",
		"CHARON_CADDY_ADMIN_API=unix://"+socketPath,
		"CPM_CADDY_ADMIN_API=",
		"CHARON_DB_PATH="+filepath.Join(tmp, "data", "test.db"),
		"CHARON_CADDY_CONFIG_DIR="+filepath.Join(tmp, "caddy"),
		"CHARON_IMPORT_DIR="+filepath.Join(tmp, "imports"),
	)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("caddy-admin listen failed: %v; output=%s", err, out)
	}
	if got, want := strings.TrimSpace(string(out)), "unix/"+socketPath+"|0600"; got != want {
		t.Fatalf("caddy-admin listen printed %q, want %q", got, want)
	}
}
