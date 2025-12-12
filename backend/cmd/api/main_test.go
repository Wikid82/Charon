package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/database"
	"github.com/Wikid82/charon/backend/internal/models"
)

func TestResetPasswordCommand_Succeeds(t *testing.T) {
	if os.Getenv("CHARON_TEST_RUN_MAIN") == "1" {
		// Child process: emulate CLI args and run main().
		email := os.Getenv("CHARON_TEST_EMAIL")
		newPassword := os.Getenv("CHARON_TEST_NEW_PASSWORD")
		os.Args = []string{"charon", "reset-password", email, newPassword}
		main()
		return
	}

	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data", "test.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}

	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	email := "user@example.com"
	user := models.User{UUID: "u-1", Email: email, Name: "User", Role: "admin", Enabled: true}
	user.PasswordHash = "$2a$10$example_hashed_password"
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestResetPasswordCommand_Succeeds")
	cmd.Dir = tmp
	cmd.Env = append(os.Environ(),
		"CHARON_TEST_RUN_MAIN=1",
		"CHARON_TEST_EMAIL="+email,
		"CHARON_TEST_NEW_PASSWORD=new-password",
		"CHARON_DB_PATH="+dbPath,
		"CHARON_CADDY_CONFIG_DIR="+filepath.Join(tmp, "caddy"),
		"CHARON_IMPORT_DIR="+filepath.Join(tmp, "imports"),
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0; err=%v; output=%s", err, string(out))
	}
}
