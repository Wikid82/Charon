package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSeedMain_Smoke(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	// #nosec G301 -- Test data directory, 0o755 acceptable for test environment
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}

	main()

	p := filepath.Join("data", "charon.db")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("expected db file to exist: %v", err)
	}
}
