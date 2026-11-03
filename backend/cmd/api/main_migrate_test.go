package main

import (
	"os"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestMain_MigrateFailureExitsNonZero_Subprocess boots the server against a database where a view holds the settings
// table name, so the schema step cannot create that table. The boot must stop with a non-zero exit that names the
// migration failure, before the CrowdSec steps run.
func TestMain_MigrateFailureExitsNonZero_Subprocess(t *testing.T) {
	if os.Getenv("CHARON_TEST_RUN_MIGRATE_FAIL") == "1" {
		os.Args = []string{"charon"}
		main()
		return
	}

	tmp, dbPath := newBootDatabasePath(t)
	seedMigrateFailureDatabase(t, dbPath)

	out, err := runBootSubprocess(t, "TestMain_MigrateFailureExitsNonZero_Subprocess", "CHARON_TEST_RUN_MIGRATE_FAIL", tmp, dbPath)
	requireBootExitedNonZero(t, out, err)
	if !strings.Contains(out, "migration failed") {
		t.Fatalf("expected the migration failure to be named; output=%s", out)
	}
}

// seedMigrateFailureDatabase creates a view named like the settings table. The schema step looks for a table of that
// name, finds none, and then cannot create it.
func seedMigrateFailureDatabase(t *testing.T, dbPath string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open seed db: %v", err)
	}
	if err = db.Exec(`CREATE VIEW settings AS SELECT 1 AS key`).Error; err != nil {
		t.Fatalf("create blocking view: %v", err)
	}
}
