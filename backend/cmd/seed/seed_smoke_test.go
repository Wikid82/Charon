package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSeedMain_Smoke(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmp := t.TempDir()
	err = os.Chdir(tmp)
	if err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	// #nosec G301 -- Test data directory, 0o755 acceptable for test environment
	err = os.MkdirAll("data", 0o750)
	if err != nil {
		t.Fatalf("mkdir data: %v", err)
	}

	main()

	p := filepath.Join("data", "charon.db")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("expected db file to exist: %v", err)
	}
}

func TestSeedMain_ForceAdminUpdatesExistingUserPassword(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmp := t.TempDir()
	err = os.Chdir(tmp)
	if err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	err = os.MkdirAll("data", 0o750)
	if err != nil {
		t.Fatalf("mkdir data: %v", err)
	}

	dbPath := filepath.Join("data", "charon.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	seeded := models.User{
		UUID:         "existing-user",
		Email:        "admin@localhost",
		Name:         "Old Name",
		Role:         "viewer",
		Enabled:      false,
		PasswordHash: "$2a$10$example_hashed_password",
	}
	if err := db.Create(&seeded).Error; err != nil {
		t.Fatalf("create seeded user: %v", err)
	}

	t.Setenv("CHARON_FORCE_DEFAULT_ADMIN", "1")
	t.Setenv("CHARON_DEFAULT_ADMIN_PASSWORD", "new-password")

	main()

	var updated models.User
	if err := db.Where("email = ?", "admin@localhost").First(&updated).Error; err != nil {
		t.Fatalf("fetch updated user: %v", err)
	}

	if updated.PasswordHash == "$2a$10$example_hashed_password" {
		t.Fatal("expected password hash to be updated for forced admin")
	}
	if updated.Role != "admin" {
		t.Fatalf("expected role admin, got %q", updated.Role)
	}
	if !updated.Enabled {
		t.Fatal("expected forced admin to be enabled")
	}
}

func TestSeedMain_ForceAdminWithoutPasswordUpdatesMetadata(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmp := t.TempDir()
	err = os.Chdir(tmp)
	if err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	err = os.MkdirAll("data", 0o750)
	if err != nil {
		t.Fatalf("mkdir data: %v", err)
	}

	dbPath := filepath.Join("data", "charon.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	seeded := models.User{
		UUID:         "existing-user-no-pass",
		Email:        "admin@localhost",
		Name:         "Old Name",
		Role:         "viewer",
		Enabled:      false,
		PasswordHash: "$2a$10$example_hashed_password",
	}
	if err := db.Create(&seeded).Error; err != nil {
		t.Fatalf("create seeded user: %v", err)
	}

	t.Setenv("CHARON_FORCE_DEFAULT_ADMIN", "1")
	t.Setenv("CHARON_DEFAULT_ADMIN_PASSWORD", "")

	main()

	var updated models.User
	if err := db.Where("email = ?", "admin@localhost").First(&updated).Error; err != nil {
		t.Fatalf("fetch updated user: %v", err)
	}

	if updated.Role != "admin" {
		t.Fatalf("expected role admin, got %q", updated.Role)
	}
	if !updated.Enabled {
		t.Fatal("expected forced admin to be enabled")
	}
	if updated.PasswordHash != "$2a$10$example_hashed_password" {
		t.Fatal("expected password hash to remain unchanged when no password is provided")
	}
}

func TestLogSeedResult_Branches(t *testing.T) {
	entry := logrus.New().WithField("component", "seed-test")

	t.Run("error branch", func(t *testing.T) {
		createdCalled := false
		result := &gorm.DB{Error: errors.New("insert failed")}
		logSeedResult(entry, result, "error", func() {
			createdCalled = true
		}, "exists")
		if createdCalled {
			t.Fatal("created callback should not be called on error")
		}
	})

	t.Run("created branch", func(t *testing.T) {
		createdCalled := false
		result := &gorm.DB{RowsAffected: 1}
		logSeedResult(entry, result, "error", func() {
			createdCalled = true
		}, "exists")
		if !createdCalled {
			t.Fatal("created callback should be called when rows are affected")
		}
	})

	t.Run("exists branch", func(t *testing.T) {
		createdCalled := false
		result := &gorm.DB{RowsAffected: 0}
		logSeedResult(entry, result, "error", func() {
			createdCalled = true
		}, "exists")
		if createdCalled {
			t.Fatal("created callback should not be called when rows are not affected")
		}
	})
}
