package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestMain_RetireFailureExitsNonZero_Subprocess boots the server against a database whose retire step cannot delete
// the deprecated mode row. The boot must stop with a non-zero exit that names the step, and the mode row must still
// be there, so the next start can retry the carry-over.
func TestMain_RetireFailureExitsNonZero_Subprocess(t *testing.T) {
	if os.Getenv("CHARON_TEST_RUN_RETIRE_FAIL") == "1" {
		os.Args = []string{"charon"}
		main()
		return
	}

	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data", "test.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}
	seedRetireFailureDatabase(t, dbPath)

	// Bounded: a regression that keeps the server running must fail the test, not hang it.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestMain_RetireFailureExitsNonZero_Subprocess") //nolint:gosec // G204: Test subprocess pattern using os.Args[0] is safe
	cmd.Dir = tmp
	cmd.Env = append(os.Environ(),
		"CHARON_TEST_RUN_RETIRE_FAIL=1",
		"CHARON_DB_PATH="+dbPath,
		"CHARON_HTTP_PORT=0",
		"CHARON_EMERGENCY_SERVER_ENABLED=false",
		"CHARON_CADDY_CONFIG_DIR="+filepath.Join(tmp, "caddy"),
		"CHARON_IMPORT_DIR="+filepath.Join(tmp, "imports"),
		"CHARON_IMPORT_CADDYFILE="+filepath.Join(tmp, "imports", "does-not-exist", "Caddyfile"),
		"CHARON_FRONTEND_DIR="+filepath.Join(tmp, "frontend", "dist"),
	)

	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() == 0 {
		t.Fatalf("expected the boot to exit non-zero; err=%v; output=%s", err, string(out))
	}
	if !strings.Contains(string(out), "retire CrowdSec mode setting") {
		t.Fatalf("expected the failing step to be named; output=%s", string(out))
	}

	db, openErr := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if openErr != nil {
		t.Fatalf("reopen db: %v", openErr)
	}
	var modeRows int64
	if countErr := db.Model(&models.Setting{}).Where("key = ?", models.CrowdSecDeprecatedModeSettingKey).Count(&modeRows).Error; countErr != nil {
		t.Fatalf("count mode rows: %v", countErr)
	}
	if modeRows != 1 {
		t.Fatalf("expected the mode row to survive the failed step, got %d rows", modeRows)
	}
}

// seedRetireFailureDatabase writes a mode row and a trigger that aborts every delete on the settings table, so the
// retire step fails after the schema step succeeds.
func seedRetireFailureDatabase(t *testing.T, dbPath string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open seed db: %v", err)
	}
	if err = db.AutoMigrate(&models.Setting{}); err != nil {
		t.Fatalf("migrate settings: %v", err)
	}
	if err = db.Create(&models.Setting{Key: models.CrowdSecDeprecatedModeSettingKey, Value: "local", Category: "security", Type: "string"}).Error; err != nil {
		t.Fatalf("seed mode row: %v", err)
	}
	if err = db.Exec(`CREATE TRIGGER block_setting_delete BEFORE DELETE ON settings BEGIN SELECT RAISE(ABORT, 'delete blocked'); END;`).Error; err != nil {
		t.Fatalf("create trigger: %v", err)
	}
}
