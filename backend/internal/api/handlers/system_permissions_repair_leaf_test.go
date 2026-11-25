package handlers

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
)

// repairPathWithTimeout runs repairPath in a goroutine so a regression that
// blocks (for example on a FIFO) fails the test instead of hanging the run.
func repairPathWithTimeout(t *testing.T, h *SystemPermissionsHandler, path string, groupMode bool, allowlist []string) permissionsRepairResult {
	t.Helper()
	done := make(chan permissionsRepairResult, 1)
	go func() {
		done <- h.repairPath(path, groupMode, allowlist)
	}()
	select {
	case result := <-done:
		return result
	case <-time.After(10 * time.Second):
		t.Fatal("repairPath did not return")
		return permissionsRepairResult{}
	}
}

func modeOf(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	require.NoError(t, err)
	return info.Mode().Perm()
}

func newRepairTestHandler() *SystemPermissionsHandler {
	return NewSystemPermissionsHandler(config.Config{}, nil, stubPermissionChecker{})
}

func TestRepairPath_RepairsRegularFileDirectoryAndRoot(t *testing.T) {
	h := newRepairTestHandler()
	allowRoot := t.TempDir()
	file := filepath.Join(allowRoot, "file.txt")
	dir := filepath.Join(allowRoot, "sub")
	require.NoError(t, os.WriteFile(file, []byte("ok"), 0o644)) // #nosec G306 -- test fixture: the repair needs these exact modes
	require.NoError(t, os.Mkdir(dir, 0o755))                    // #nosec G301 -- test fixture: the repair needs these exact modes
	require.NoError(t, os.Chmod(allowRoot, 0o755))              // #nosec G302 -- test fixture: the repair needs these exact modes
	allowlist := []string{allowRoot}

	result := h.repairPath(file, false, allowlist)
	require.Equal(t, "repaired", result.Status)
	require.Equal(t, os.FileMode(0o600), modeOf(t, file))

	result = h.repairPath(dir, false, allowlist)
	require.Equal(t, "repaired", result.Status)
	require.Equal(t, os.FileMode(0o700), modeOf(t, dir))

	result = h.repairPath(allowRoot, false, allowlist)
	require.Equal(t, "repaired", result.Status)
	require.Equal(t, os.FileMode(0o700), modeOf(t, allowRoot))
}

func TestRepairPath_RepairsZeroModeFileAndSearchOnlyDirectoryAsNonRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test covers the non-root case; run as a normal user")
	}
	h := newRepairTestHandler()
	allowRoot := t.TempDir()
	file := filepath.Join(allowRoot, "locked.txt")
	dir := filepath.Join(allowRoot, "searchonly")
	require.NoError(t, os.WriteFile(file, []byte("ok"), 0o600))
	require.NoError(t, os.Chmod(file, 0o000))
	require.NoError(t, os.Mkdir(dir, 0o700))
	require.NoError(t, os.Chmod(dir, 0o300)) // #nosec G302 -- test fixture: the repair needs these exact modes
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o700) // #nosec G302 -- test fixture: the repair needs these exact modes
		_ = os.Chmod(file, 0o600)
	})
	allowlist := []string{allowRoot}

	result := h.repairPath(file, false, allowlist)
	require.Equal(t, "repaired", result.Status)
	require.Equal(t, os.FileMode(0o600), modeOf(t, file))

	result = h.repairPath(dir, false, allowlist)
	require.Equal(t, "repaired", result.Status)
	require.Equal(t, os.FileMode(0o700), modeOf(t, dir))
}

func TestRepairPath_SymlinkLeafIsRejectedAndTargetUntouched(t *testing.T) {
	h := newRepairTestHandler()
	allowRoot := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("keep"), 0o644)) // #nosec G306 -- test fixture: the repair needs these exact modes
	inRootTarget := filepath.Join(allowRoot, "real.txt")
	require.NoError(t, os.WriteFile(inRootTarget, []byte("keep"), 0o644)) // #nosec G306 -- test fixture: the repair needs these exact modes
	inLink := filepath.Join(allowRoot, "in-link")
	outLink := filepath.Join(allowRoot, "out-link")
	require.NoError(t, os.Symlink(inRootTarget, inLink))
	require.NoError(t, os.Symlink(target, outLink))
	allowlist := []string{allowRoot}

	for _, link := range []string{inLink, outLink} {
		result := h.repairPath(link, false, allowlist)
		require.Equal(t, "error", result.Status)
		require.Equal(t, "permissions_symlink_rejected", result.ErrorCode)
	}
	require.Equal(t, os.FileMode(0o644), modeOf(t, target))
	require.Equal(t, os.FileMode(0o644), modeOf(t, inRootTarget))
}

func TestRepairPath_PathChangedDuringRepair(t *testing.T) {
	h := newRepairTestHandler()
	allowRoot := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("keep"), 0o644)) // #nosec G306 -- test fixture: the repair needs these exact modes
	file := filepath.Join(allowRoot, "replaced.txt")
	require.NoError(t, os.WriteFile(file, []byte("ok"), 0o644)) // #nosec G306 -- test fixture: the repair needs these exact modes

	beforeOpenHook = func(path string) {
		if path != file {
			return
		}
		if err := os.Remove(file); err != nil {
			t.Errorf("remove leaf: %v", err)
			return
		}
		if err := os.Symlink(target, file); err != nil {
			t.Errorf("replace leaf with symlink: %v", err)
		}
	}
	t.Cleanup(func() { beforeOpenHook = nil })

	result := repairPathWithTimeout(t, h, file, false, []string{allowRoot})
	require.Equal(t, "error", result.Status)
	require.Equal(t, "permissions_symlink_rejected", result.ErrorCode)
	require.Equal(t, os.FileMode(0o644), modeOf(t, target))
}

func TestRepairPath_FifoIsRejectedWithoutBlocking(t *testing.T) {
	h := newRepairTestHandler()
	allowRoot := t.TempDir()
	fifo := filepath.Join(allowRoot, "pipe")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))

	result := repairPathWithTimeout(t, h, fifo, false, []string{allowRoot})
	require.Equal(t, "error", result.Status)
	require.Equal(t, "permissions_unsupported_type", result.ErrorCode)
}

func TestRepairPath_HardLinkedFileIsRejectedUnchanged(t *testing.T) {
	h := newRepairTestHandler()
	allowRoot := t.TempDir()
	file := filepath.Join(allowRoot, "original.txt")
	other := filepath.Join(allowRoot, "second-name.txt")
	require.NoError(t, os.WriteFile(file, []byte("ok"), 0o644)) // #nosec G306 -- test fixture: the repair needs these exact modes
	require.NoError(t, os.Link(file, other))

	result := h.repairPath(file, false, []string{allowRoot})
	require.Equal(t, "error", result.Status)
	require.Equal(t, "permissions_hardlink_rejected", result.ErrorCode)
	require.Equal(t, os.FileMode(0o644), modeOf(t, file))
}

func TestRepairPath_RefusesWhenProcFDUnavailable(t *testing.T) {
	h := newRepairTestHandler()
	allowRoot := t.TempDir()
	file := filepath.Join(allowRoot, "file.txt")
	require.NoError(t, os.WriteFile(file, []byte("ok"), 0o644)) // #nosec G306 -- test fixture: the repair needs these exact modes

	original := procSelfFDDir
	procSelfFDDir = filepath.Join(t.TempDir(), "missing-fd-dir")
	t.Cleanup(func() { procSelfFDDir = original })

	result := h.repairPath(file, false, []string{allowRoot})
	require.Equal(t, "error", result.Status)
	require.Equal(t, "permissions_repair_unavailable", result.ErrorCode)
	require.Equal(t, os.FileMode(0o644), modeOf(t, file))
}
