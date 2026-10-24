package handlers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
)

func TestSaveAndReadKeyFromFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "crowdsec-bouncer-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	keyFile := filepath.Join(tmpDir, "subdir", "bouncer_key")
	testKey := "test-api-key-789"

	// Test saveKeyToFile creates directories and saves key
	if saveErr := saveKeyToFile(keyFile, testKey); saveErr != nil {
		t.Fatalf("saveKeyToFile() error = %v", saveErr)
	}

	// Verify file was created
	info, err := os.Stat(keyFile)
	if err != nil {
		t.Fatalf("key file not created: %v", err)
	}

	// Verify permissions (0600)
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("saveKeyToFile() file permissions = %o, want 0600", perm)
	}

	// Test bouncerkey.ReadFile
	readKey := bouncerkey.ReadFile(keyFile)
	if readKey != testKey {
		t.Errorf("bouncerkey.ReadFile() = %q, want %q", readKey, testKey)
	}
}

func TestSaveKeyToFile_EmptyKey(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "crowdsec-bouncer-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	keyFile := filepath.Join(tmpDir, "bouncer_key")

	// Should return error for empty key
	if err := saveKeyToFile(keyFile, ""); err == nil {
		t.Error("saveKeyToFile() expected error for empty key")
	}
}
