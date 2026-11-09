package main

import (
	"os"
	"strings"
	"testing"

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

	tmp, dbPath := newBootDatabasePath(t)
	seedRetireFailureDatabase(t, dbPath)

	out, err := runBootSubprocess(t, "TestMain_RetireFailureExitsNonZero_Subprocess", "CHARON_TEST_RUN_RETIRE_FAIL", tmp, dbPath)
	requireBootExitedNonZero(t, out, err)
	if !strings.Contains(out, "retire CrowdSec mode setting") {
		t.Fatalf("expected the failing step to be named; output=%s", out)
	}

	db, openErr := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if openErr != nil {
		t.Fatalf("reopen db: %v", openErr)
	}
	var modeRows int64
	if countErr := db.Model(&models.Setting{}).Where("key = ?", models.CrowdSecRetiredModeSettingKey).Count(&modeRows).Error; countErr != nil {
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
	if err = db.Create(&models.Setting{Key: models.CrowdSecRetiredModeSettingKey, Value: "local", Category: "security", Type: "string"}).Error; err != nil {
		t.Fatalf("seed mode row: %v", err)
	}
	if err = db.Exec(`CREATE TRIGGER block_setting_delete BEFORE DELETE ON settings BEGIN SELECT RAISE(ABORT, 'delete blocked'); END;`).Error; err != nil {
		t.Fatalf("create trigger: %v", err)
	}
}
