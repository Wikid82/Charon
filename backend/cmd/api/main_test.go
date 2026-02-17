package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

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
	// #nosec G301 -- Test fixture directory with standard permissions
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}

	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	if err = db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	email := "user@example.com"
	user := models.User{UUID: "u-1", Email: email, Name: "User", Role: "admin", Enabled: true}
	user.PasswordHash = "$2a$10$example_hashed_password"
	if err = db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestResetPasswordCommand_Succeeds") //nolint:gosec // G204: Test subprocess pattern using os.Args[0] is safe
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

func TestMigrateCommand_Succeeds(t *testing.T) {
	if os.Getenv("CHARON_TEST_RUN_MAIN") == "1" {
		// Child process: emulate CLI args and run main().
		os.Args = []string{"charon", "migrate"}
		main()
		return
	}

	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data", "test.db")
	// #nosec G301 -- Test fixture directory with standard permissions
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}

	// Create database without security tables
	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	// Only migrate User table to simulate old database
	if err = db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("automigrate user: %v", err)
	}

	// Verify security tables don't exist
	if db.Migrator().HasTable(&models.SecurityConfig{}) {
		t.Fatal("SecurityConfig table should not exist yet")
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMigrateCommand_Succeeds") //nolint:gosec // G204: Test subprocess pattern using os.Args[0] is safe
	cmd.Dir = tmp
	cmd.Env = append(os.Environ(),
		"CHARON_TEST_RUN_MAIN=1",
		"CHARON_DB_PATH="+dbPath,
		"CHARON_CADDY_CONFIG_DIR="+filepath.Join(tmp, "caddy"),
		"CHARON_IMPORT_DIR="+filepath.Join(tmp, "imports"),
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0; err=%v; output=%s", err, string(out))
	}

	// Reconnect and verify security tables were created
	db2, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("reconnect db: %v", err)
	}

	securityModels := []any{
		&models.SecurityConfig{},
		&models.SecurityDecision{},
		&models.SecurityAudit{},
		&models.SecurityRuleSet{},
		&models.CrowdsecPresetEvent{},
		&models.CrowdsecConsoleEnrollment{},
	}

	for _, model := range securityModels {
		if !db2.Migrator().HasTable(model) {
			t.Errorf("Table for %T was not created by migrate command", model)
		}
	}
}

func TestStartupVerification_MissingTables(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data", "test.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}

	// Create database without security tables
	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	// Only migrate User table to simulate old database
	if err = db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("automigrate user: %v", err)
	}

	// Verify security tables don't exist
	if db.Migrator().HasTable(&models.SecurityConfig{}) {
		t.Fatal("SecurityConfig table should not exist yet")
	}

	// Close and reopen to simulate startup scenario
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	db, err = database.Connect(dbPath)
	if err != nil {
		t.Fatalf("reconnect db: %v", err)
	}

	// Simulate startup verification logic from main.go
	securityModels := []any{
		&models.SecurityConfig{},
		&models.SecurityDecision{},
		&models.SecurityAudit{},
		&models.SecurityRuleSet{},
		&models.CrowdsecPresetEvent{},
		&models.CrowdsecConsoleEnrollment{},
	}

	missingTables := false
	for _, model := range securityModels {
		if !db.Migrator().HasTable(model) {
			missingTables = true
			t.Logf("Missing table for model %T", model)
		}
	}

	if !missingTables {
		t.Fatal("Expected to find missing tables but all were present")
	}

	// Run auto-migration (simulating startup verification logic)
	if err := db.AutoMigrate(securityModels...); err != nil {
		t.Fatalf("failed to migrate security tables: %v", err)
	}

	// Verify all tables now exist
	for _, model := range securityModels {
		if !db.Migrator().HasTable(model) {
			t.Errorf("Table for %T was not created by auto-migration", model)
		}
	}
}

func TestMain_MigrateCommand_InProcess(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data", "test.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}

	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	if err = db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("automigrate user: %v", err)
	}

	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })

	t.Setenv("CHARON_DB_PATH", dbPath)
	t.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(tmp, "caddy"))
	t.Setenv("CHARON_IMPORT_DIR", filepath.Join(tmp, "imports"))
	os.Args = []string{"charon", "migrate"}

	main()

	db2, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("reconnect db: %v", err)
	}

	securityModels := []any{
		&models.SecurityConfig{},
		&models.SecurityDecision{},
		&models.SecurityAudit{},
		&models.SecurityRuleSet{},
		&models.CrowdsecPresetEvent{},
		&models.CrowdsecConsoleEnrollment{},
	}

	for _, model := range securityModels {
		if !db2.Migrator().HasTable(model) {
			t.Errorf("Table for %T was not created by migrate command", model)
		}
	}
}

func TestMain_ResetPasswordCommand_InProcess(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data", "test.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}

	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	if err = db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	email := "user@example.com"
	user := models.User{UUID: "u-1", Email: email, Name: "User", Role: "admin", Enabled: true}
	user.PasswordHash = "$2a$10$example_hashed_password"
	user.FailedLoginAttempts = 3
	if err = db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })

	t.Setenv("CHARON_DB_PATH", dbPath)
	t.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(tmp, "caddy"))
	t.Setenv("CHARON_IMPORT_DIR", filepath.Join(tmp, "imports"))
	os.Args = []string{"charon", "reset-password", email, "new-password"}

	main()

	var updated models.User
	if err := db.Where("email = ?", email).First(&updated).Error; err != nil {
		t.Fatalf("fetch updated user: %v", err)
	}
	if updated.PasswordHash == "$2a$10$example_hashed_password" {
		t.Fatal("expected password hash to be updated")
	}
	if updated.FailedLoginAttempts != 0 {
		t.Fatalf("expected failed login attempts reset to 0, got %d", updated.FailedLoginAttempts)
	}
}

func TestMain_DefaultStartupGracefulShutdown_Subprocess(t *testing.T) {
	if os.Getenv("CHARON_TEST_RUN_MAIN_SERVER") == "1" {
		os.Args = []string{"charon"}
		signalPort := os.Getenv("CHARON_TEST_SIGNAL_PORT")

		go func() {
			if signalPort != "" {
				_ = waitForTCPReady("127.0.0.1:"+signalPort, 10*time.Second)
			}
			process, err := os.FindProcess(os.Getpid())
			if err == nil {
				_ = process.Signal(syscall.SIGTERM)
			}
		}()

		main()
		return
	}

	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data", "test.db")
	httpPort, err := findFreeTCPPort()
	if err != nil {
		t.Fatalf("find free http port: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMain_DefaultStartupGracefulShutdown_Subprocess") //nolint:gosec // G204: Test subprocess pattern using os.Args[0] is safe
	cmd.Dir = tmp
	cmd.Env = append(os.Environ(),
		"CHARON_TEST_RUN_MAIN_SERVER=1",
		"CHARON_DB_PATH="+dbPath,
		"CHARON_HTTP_PORT="+httpPort,
		"CHARON_TEST_SIGNAL_PORT="+httpPort,
		"CHARON_EMERGENCY_SERVER_ENABLED=false",
		"CHARON_CADDY_CONFIG_DIR="+filepath.Join(tmp, "caddy"),
		"CHARON_IMPORT_DIR="+filepath.Join(tmp, "imports"),
		"CHARON_IMPORT_CADDYFILE="+filepath.Join(tmp, "imports", "does-not-exist", "Caddyfile"),
		"CHARON_FRONTEND_DIR="+filepath.Join(tmp, "frontend", "dist"),
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected startup/shutdown to exit 0; err=%v; output=%s", err, string(out))
	}
}

func TestMain_DefaultStartupGracefulShutdown_InProcess(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data", "test.db")
	httpPort, err := findFreeTCPPort()
	if err != nil {
		t.Fatalf("find free http port: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}

	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })

	t.Setenv("CHARON_DB_PATH", dbPath)
	t.Setenv("CHARON_HTTP_PORT", httpPort)
	t.Setenv("CHARON_EMERGENCY_SERVER_ENABLED", "false")
	t.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(tmp, "caddy"))
	t.Setenv("CHARON_IMPORT_DIR", filepath.Join(tmp, "imports"))
	t.Setenv("CHARON_IMPORT_CADDYFILE", filepath.Join(tmp, "imports", "does-not-exist", "Caddyfile"))
	t.Setenv("CHARON_FRONTEND_DIR", filepath.Join(tmp, "frontend", "dist"))
	os.Args = []string{"charon"}

	go func() {
		_ = waitForTCPReady("127.0.0.1:"+httpPort, 10*time.Second)
		process, err := os.FindProcess(os.Getpid())
		if err == nil {
			_ = process.Signal(syscall.SIGTERM)
		}
	}()

	main()
}

func findFreeTCPPort() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("listen free port: %w", err)
	}
	defer func() {
		_ = listener.Close()
	}()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return "", fmt.Errorf("unexpected listener addr type: %T", listener.Addr())
	}

	return fmt.Sprintf("%d", addr.Port), nil
}

func waitForTCPReady(address string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}

		time.Sleep(25 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for TCP readiness at %s", address)
}
