package dbmaint

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Wikid82/charon/backend/internal/logger"
)

const (
	// tempEnvVar is the variable SQLite reads for its temporary-file directory.
	// It only takes effect when set before the driver's first sql.Open; set
	// later it is ignored (verified by TestDriver_SQLiteTmpDirOnlyHonouredBeforeFirstOpen).
	tempEnvVar = "SQLITE_TMPDIR"
	// tempDirName is the directory created next to the database file.
	tempDirName = ".tmp"
	// tempDirMode keeps the directory private to the charon user.
	tempDirMode os.FileMode = 0o700
)

// defaultTempDirs is SQLite's own search order when SQLITE_TMPDIR is unset.
var defaultTempDirs = []string{"/var/tmp", "/usr/tmp", "/tmp"}

// ErrTempDirUnsafe is returned by PrepareTempDir for a symlink, a non-directory
// or a directory owned by another user.
var ErrTempDirUnsafe = errors.New("temp directory is not safe to use")

// PrepareTempDir creates (or validates) <dataDir>/.tmp, the directory SQLite
// uses for the temporary file of a VACUUM, so a large rebuild lands on the data
// volume instead of a small RAM-backed /tmp. A symlink, a non-directory or a
// directory owned by another user is refused; the directory is 0700.
func PrepareTempDir(dataDir string) (string, error) {
	dir := filepath.Join(filepath.Clean(dataDir), tempDirName)

	info, err := os.Lstat(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if mkErr := os.Mkdir(dir, tempDirMode); mkErr != nil {
			return "", fmt.Errorf("create temp directory: %w", mkErr)
		}
		info, err = os.Lstat(dir)
		if err != nil {
			return "", fmt.Errorf("stat temp directory: %w", err)
		}
	case err != nil:
		return "", fmt.Errorf("stat temp directory: %w", err)
	}

	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("%w: %s is a symlink or not a directory", ErrTempDirUnsafe, dir)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int64(st.Uid) != int64(os.Geteuid()) {
		return "", fmt.Errorf("%w: %s is not owned by the current user", ErrTempDirUnsafe, dir)
	}
	if info.Mode().Perm() != tempDirMode {
		if err := os.Chmod(dir, tempDirMode); err != nil {
			return "", fmt.Errorf("restrict temp directory: %w", err)
		}
	}
	return dir, nil
}

// ApplyTempDir points SQLite at <dataDir>/.tmp by setting SQLITE_TMPDIR. It
// MUST run before the first sql.Open of the process. An operator-set
// SQLITE_TMPDIR is honoured and left untouched. On any failure the variable is
// not set, SQLite keeps its default search order and a warning is logged.
func ApplyTempDir(dataDir string) {
	if os.Getenv(tempEnvVar) != "" {
		return
	}
	dir, err := PrepareTempDir(dataDir)
	if err != nil {
		logger.Log().WithError(err).Warn("database temp directory not prepared; SQLite will use its default temp location")
		return
	}
	if err := os.Setenv(tempEnvVar, dir); err != nil {
		logger.Log().WithError(err).Warn("could not set SQLITE_TMPDIR; SQLite will use its default temp location")
	}
}

// EffectiveTempDir returns the directory SQLite will actually use for
// temporary files: SQLITE_TMPDIR when set (validated as a writable directory),
// else the first writable directory of SQLite's default search order.
func EffectiveTempDir() (string, error) {
	if dir := os.Getenv(tempEnvVar); dir != "" {
		dir = filepath.Clean(dir)
		if !writableDir(dir) {
			return "", fmt.Errorf("%s=%s is not a writable directory", tempEnvVar, dir)
		}
		return dir, nil
	}
	for _, dir := range defaultTempDirs {
		if writableDir(dir) {
			return dir, nil
		}
	}
	return "", errors.New("no writable temp directory found")
}

func writableDir(dir string) bool {
	info, err := os.Stat(dir) //nolint:gosec // operator-configured directory; only stat'ed and access-checked
	if err != nil || !info.IsDir() {
		return false
	}
	return syscall.Access(dir, 2) == nil // 2 == W_OK
}
