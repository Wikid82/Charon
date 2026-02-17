//go:build ignore
// +build ignore

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSeedMain_CreatesDatabaseFile(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}

	main()

	dbPath := filepath.Join("data", "charon.db")
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("expected db file to exist at %s: %v", dbPath, err)
	}
	if info.Size() == 0 {
		t.Fatalf("expected db file to be non-empty")
	}
}
