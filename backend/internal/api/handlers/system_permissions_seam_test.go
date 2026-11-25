package handlers

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepairPath_OwnershipChangeRefusedIsReported(t *testing.T) {
	h := newRepairTestHandler()
	allowRoot := t.TempDir()
	file := filepath.Join(allowRoot, "refused.txt")
	require.NoError(t, os.WriteFile(file, []byte("ok"), 0o644)) // #nosec G306 -- test fixture: the repair needs these exact modes

	original := changeOwnerOfDescriptor
	changeOwnerOfDescriptor = func(int, int, int) error { return syscall.EPERM }
	t.Cleanup(func() { changeOwnerOfDescriptor = original })

	result := h.repairPath(file, false, []string{allowRoot})

	require.Equal(t, "error", result.Status)
	require.Equal(t, "could not change ownership", result.Message)
	require.Equal(t, os.FileMode(0o644), modeOf(t, file), "a refused ownership change leaves the mode alone")
}
