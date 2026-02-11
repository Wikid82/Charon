package util

import (
	"errors"
	"fmt"
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
