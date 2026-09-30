package dbmaint

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"syscall"
)

// AvailableBytes returns the disk space in bytes available to an unprivileged
// process on the filesystem holding path (Bavail, not Bfree).
func AvailableBytes(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(filepath.Clean(path), &stat); err != nil {
		return 0, fmt.Errorf("failed to get disk space: %w", err)
	}

	// Safe conversion with overflow protection (gosec G115)
	bsize := stat.Bsize
	bavail := stat.Bavail

	// Check for invalid filesystem (negative block size)
	if bsize < 0 {
		return 0, fmt.Errorf("invalid block size: %d", bsize)
	}

	// Check if bavail exceeds max int64 before conversion
	if bavail > uint64(math.MaxInt64) {
		return math.MaxInt64, nil
	}

	availBlocks := int64(bavail)
	blockSize := int64(bsize)

	// Check for multiplication overflow
	if availBlocks > 0 && blockSize > math.MaxInt64/availBlocks {
		return math.MaxInt64, nil
	}

	return availBlocks * blockSize, nil
}

// SameFilesystem reports whether two existing paths live on the same device.
func SameFilesystem(a, b string) (bool, error) {
	devA, err := deviceOf(a)
	if err != nil {
		return false, err
	}
	devB, err := deviceOf(b)
	if err != nil {
		return false, err
	}
	return devA == devB, nil
}

func deviceOf(path string) (uint64, error) {
	info, err := os.Stat(filepath.Clean(path))
	if err != nil {
		return 0, fmt.Errorf("stat %s: %w", path, err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("stat %s: unsupported platform", path)
	}
	return uint64(st.Dev), nil //nolint:gosec // Dev is an opaque identifier, never used arithmetically
}
