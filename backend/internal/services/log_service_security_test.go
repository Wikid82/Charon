package services

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// securityFixtureService creates a log dir containing real.log and returns
// the service plus the log dir path.
func securityFixtureService(t *testing.T) (*LogService, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	logsDir := filepath.Join(dataDir, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "real.log"), []byte("line\n"), 0o600)) // #nosec G306 -- test fixture

	service := NewLogService(&config.Config{DatabasePath: filepath.Join(dataDir, "charon.db")})
	return service, logsDir
}

func TestGetLogPath_Traversal(t *testing.T) {
	service, _ := securityFixtureService(t)

	traversalShaped := []string{
		"../secret",
		"..%2Fsecret", // literal percent-encoded separator
		"/etc/passwd",
		"a/../b.log",
		".",
		"..",
		"",
	}
	for _, name := range traversalShaped {
		t.Run("shape_"+name, func(t *testing.T) {
			_, err := service.GetLogPath(name)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInvalidFilename),
				"expected ErrInvalidFilename for %q, got: %v", name, err)
		})
	}

	// Name absent from the raw directory listing -> os.ErrNotExist without
	// opening any file.
	_, err := service.GetLogPath("unknown.log")
	require.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist))

	// A well-shaped name that is not a log file (extension rules) is not
	// servable either.
	_, err = service.GetLogPath("notes.txt")
	require.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist))
}

func TestGetLogPath_SymlinkEscapeRejected(t *testing.T) {
	service, logsDir := securityFixtureService(t)

	// Plant a symlink inside the log dir pointing OUTSIDE all log dirs.
	outsideDir := t.TempDir()
	secret := filepath.Join(outsideDir, "secret")
	require.NoError(t, os.WriteFile(secret, []byte("top secret"), 0o600)) // #nosec G306 -- test fixture
	require.NoError(t, os.Symlink(secret, filepath.Join(logsDir, "evil.log")))

	_, err := service.GetLogPath("evil.log")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidFilename),
		"symlink escape must be refused, got: %v", err)
}

func TestGetLogPath_SymlinkedLogDirStillAllowed(t *testing.T) {
	// A log dir that is ITSELF a symlink must not break containment for
	// legitimately contained files (both-sides resolution).
	tmpDir := t.TempDir()
	realDir := filepath.Join(tmpDir, "real-logs")
	require.NoError(t, os.MkdirAll(realDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(realDir, "contained.log"), []byte("line\n"), 0o600)) // #nosec G306 -- test fixture

	linkDir := filepath.Join(tmpDir, "link-logs")
	require.NoError(t, os.Symlink(realDir, linkDir))

	service := &LogService{LogDir: linkDir}
	path, err := service.GetLogPath("contained.log")
	require.NoError(t, err)

	resolvedReal, err := filepath.EvalSymlinks(filepath.Join(realDir, "contained.log"))
	require.NoError(t, err)
	assert.Equal(t, resolvedReal, path)
}

func TestGetLogPath_SymlinkAliasesBothServable(t *testing.T) {
	// Guards the ListLogs-dedup finding: BOTH names of a legitimate same-dir
	// alias pair (charon.log / cpmp.log) must stay servable, because the
	// allowlist is built from RAW directory entries, not ListLogs output.
	service, logsDir := securityFixtureService(t)
	require.NoError(t, os.Symlink(filepath.Join(logsDir, "real.log"), filepath.Join(logsDir, "alias.log")))

	realResolved, err := filepath.EvalSymlinks(filepath.Join(logsDir, "real.log"))
	require.NoError(t, err)

	pathReal, err := service.GetLogPath("real.log")
	require.NoError(t, err)
	assert.Equal(t, realResolved, pathReal)

	pathAlias, err := service.GetLogPath("alias.log")
	require.NoError(t, err)
	assert.Equal(t, realResolved, pathAlias)
}

func TestGetLogPath_ReturnsResolvedPath(t *testing.T) {
	// TOCTOU guard: for a symlinked name the returned path is the
	// EvalSymlinks target (callers open exactly this path).
	service, logsDir := securityFixtureService(t)
	require.NoError(t, os.Symlink(filepath.Join(logsDir, "real.log"), filepath.Join(logsDir, "alias.log")))

	path, err := service.GetLogPath("alias.log")
	require.NoError(t, err)

	target, err := filepath.EvalSymlinks(filepath.Join(logsDir, "real.log"))
	require.NoError(t, err)
	assert.Equal(t, target, path, "returned path must be the resolved target, not the symlink")
	assert.NotEqual(t, filepath.Join(logsDir, "alias.log"), path)
}

func TestGetLogPath_ListServableNamesError(t *testing.T) {
	// Log dir is a file, so os.ReadDir fails with a non-NotExist error which
	// must be wrapped and surfaced.
	tmpDir := t.TempDir()
	notDir := filepath.Join(tmpDir, "not-a-dir")
	require.NoError(t, os.WriteFile(notDir, []byte("x"), 0o600)) // #nosec G306 -- test fixture

	service := &LogService{LogDir: notDir}
	_, err := service.GetLogPath("real.log")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list servable log names")
}

func TestGetLogPath_DanglingSymlinkIsNotFound(t *testing.T) {
	service, logsDir := securityFixtureService(t)
	require.NoError(t, os.Symlink(filepath.Join(logsDir, "gone.log"), filepath.Join(logsDir, "dangling.log")))

	_, err := service.GetLogPath("dangling.log")
	require.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist))
}

func TestGetLogPath_SymlinkToDirectoryRejected(t *testing.T) {
	// A symlink whose target is a directory must never be served.
	service, logsDir := securityFixtureService(t)
	targetDir := t.TempDir()
	require.NoError(t, os.Symlink(targetDir, filepath.Join(logsDir, "dir.log")))

	_, err := service.GetLogPath("dir.log")
	require.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist), "non-regular resolved target must not be servable, got: %v", err)
}

func TestGetLogPath_SkipsUnresolvableLogDir(t *testing.T) {
	// One configured log dir does not exist; containment must still succeed
	// against the resolvable dir (resolvedInLogDirs skips the bad one).
	tmpDir := t.TempDir()
	caddyDir := filepath.Join(tmpDir, "caddy-logs")
	require.NoError(t, os.MkdirAll(caddyDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(caddyDir, "caddy.log"), []byte("line\n"), 0o600)) // #nosec G306 -- test fixture

	service := &LogService{
		LogDir:      filepath.Join(tmpDir, "missing-logs"), // does not exist
		CaddyLogDir: caddyDir,
	}

	path, err := service.GetLogPath("caddy.log")
	require.NoError(t, err)
	resolved, err := filepath.EvalSymlinks(filepath.Join(caddyDir, "caddy.log"))
	require.NoError(t, err)
	assert.Equal(t, resolved, path)
}

func TestQueryLogs_InvalidFilenamePropagatesSentinel(t *testing.T) {
	service, _ := securityFixtureService(t)
	_, _, _, err := service.QueryLogs("../real.log", models.LogFilter{Limit: 10})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidFilename))
}

func TestListServableNames_RawEntriesPreDedup(t *testing.T) {
	service, logsDir := securityFixtureService(t)
	require.NoError(t, os.Symlink(filepath.Join(logsDir, "real.log"), filepath.Join(logsDir, "alias.log")))
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "rotated.log.1"), []byte("x\n"), 0o600)) // #nosec G306 -- test fixture
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "notes.txt"), []byte("x\n"), 0o600))     // #nosec G306 -- test fixture
	require.NoError(t, os.MkdirAll(filepath.Join(logsDir, "sub.log"), 0o750))                       // directory with log-ish name

	names, err := service.listServableNames()
	require.NoError(t, err)
	assert.True(t, names["real.log"])
	assert.True(t, names["alias.log"], "both symlink alias names must be servable (pre-dedup)")
	assert.True(t, names["rotated.log.1"])
	assert.False(t, names["notes.txt"])
	assert.False(t, names["sub.log"], "directories are never servable")
}
