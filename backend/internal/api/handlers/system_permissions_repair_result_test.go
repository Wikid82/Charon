package handlers

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
)

func TestRepairError_BuildsErrorResult(t *testing.T) {
	result := repairError("/data/file", "permissions_write_denied", "permission denied")
	require.Equal(t, permissionsRepairResult{
		Path:      "/data/file",
		Status:    "error",
		ErrorCode: "permissions_write_denied",
		Message:   "permission denied",
	}, result)
}

func TestRepairFailure_UsesFixedMessageAndCode(t *testing.T) {
	result := repairFailure("/data/file", "permissions_repair_failed", "could not change the mode", errors.New("detail: secret-free"))
	require.Equal(t, "error", result.Status)
	require.Equal(t, "permissions_repair_failed", result.ErrorCode)
	require.Equal(t, "could not change the mode", result.Message)
}

func TestRepairPath_LstatFailureReturnsFixedMessage(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test requires a non-root user so the parent folder is unreadable")
	}

	h := NewSystemPermissionsHandler(config.Config{}, nil, stubPermissionChecker{})
	allowRoot := t.TempDir()
	locked := filepath.Join(allowRoot, "locked")
	require.NoError(t, os.MkdirAll(filepath.Join(locked, "child"), 0o700))
	require.NoError(t, os.Chmod(locked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) }) // #nosec G302 -- test fixture: restores the locked test folder

	result := h.repairPath(filepath.Join(locked, "child", "file"), false, []string{allowRoot})
	require.Equal(t, "error", result.Status)
	require.Equal(t, "permissions_repair_failed", result.ErrorCode)
	require.Equal(t, "could not read the path", result.Message)
}
