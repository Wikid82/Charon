package util

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"
)

func TestMapSaveErrorCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
		wantOK   bool
	}{
		{
			name:     "sqlite readonly",
			err:      errors.New("attempt to write a readonly database"),
			wantCode: "permissions_db_readonly",
			wantOK:   true,
		},
		{
			name:     "sqlite locked",
			err:      errors.New("database is locked"),
			wantCode: "permissions_db_locked",
			wantOK:   true,
		},
		{
			name:     "permission denied",
			err:      fmt.Errorf("write failed: %w", syscall.EACCES),
			wantCode: "permissions_write_denied",
			wantOK:   true,
		},
		{
			name:     "not a permission error",
			err:      errors.New("other error"),
			wantCode: "",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, ok := MapSaveErrorCode(tt.err)
			if code != tt.wantCode || ok != tt.wantOK {
				t.Fatalf("MapSaveErrorCode() = (%q, %v), want (%q, %v)", code, ok, tt.wantCode, tt.wantOK)
			}
		})
	}
}

func TestIsSQLiteReadOnlyError(t *testing.T) {
	if !IsSQLiteReadOnlyError(errors.New("SQLITE_READONLY")) {
		t.Fatalf("expected SQLITE_READONLY to be detected")
	}
}

func TestMapDiagnosticErrorCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil", err: nil, want: ""},
		{name: "not found", err: os.ErrNotExist, want: "permissions_missing_path"},
		{name: "readonly", err: syscall.EROFS, want: "permissions_readonly"},
		{name: "permission denied", err: syscall.EACCES, want: "permissions_write_denied"},
		{name: "other", err: errors.New("boom"), want: "permissions_write_failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MapDiagnosticErrorCode(tt.err); got != tt.want {
				t.Fatalf("MapDiagnosticErrorCode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckPathPermissions(t *testing.T) {
	t.Run("missing path", func(t *testing.T) {
		result := CheckPathPermissions("/definitely/missing/path", "rw")
		if result.Exists {
			t.Fatalf("expected missing path to not exist")
		}
		if result.ErrorCode != "permissions_missing_path" {
			t.Fatalf("expected permissions_missing_path, got %q", result.ErrorCode)
		}
	})

	t.Run("writable file", func(t *testing.T) {
		tempFile, err := os.CreateTemp(t.TempDir(), "perm-file-*.txt")
		if err != nil {
			t.Fatalf("create temp file: %v", err)
		}
		if closeErr := tempFile.Close(); closeErr != nil {
			t.Fatalf("close temp file: %v", closeErr)
		}

		result := CheckPathPermissions(tempFile.Name(), "rw")
		if !result.Exists {
			t.Fatalf("expected file to exist")
		}
		if !result.Writable {
			t.Fatalf("expected file to be writable, got error: %s", result.Error)
		}
	})

	t.Run("writable directory", func(t *testing.T) {
		dir := t.TempDir()
		result := CheckPathPermissions(dir, "rwx")
		if !result.Exists {
			t.Fatalf("expected directory to exist")
		}
		if !result.Writable {
			t.Fatalf("expected directory to be writable, got error: %s", result.Error)
		}
	})

	t.Run("no write required", func(t *testing.T) {
		tempFile, err := os.CreateTemp(t.TempDir(), "perm-read-*.txt")
		if err != nil {
			t.Fatalf("create temp file: %v", err)
		}
		if closeErr := tempFile.Close(); closeErr != nil {
			t.Fatalf("close temp file: %v", closeErr)
		}

		result := CheckPathPermissions(tempFile.Name(), "r")
		if result.Writable {
			t.Fatalf("expected writable=false when write permission is not required")
		}
	})
}
