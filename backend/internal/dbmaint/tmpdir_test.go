package dbmaint

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareTempDir_CreatesPrivateDirectory(t *testing.T) {
	data := t.TempDir()

	dir, err := PrepareTempDir(data)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(data, ".tmp"), dir)
	info, err := os.Lstat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestPrepareTempDir_ExistingDirIsTightened(t *testing.T) {
	data := t.TempDir()
	dir := filepath.Join(data, ".tmp")
	require.NoError(t, os.Mkdir(dir, 0o750))

	_, err := PrepareTempDir(data)
	require.NoError(t, err)

	info, err := os.Lstat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestPrepareTempDir_RefusesSymlink(t *testing.T) {
	data := t.TempDir()
	target := t.TempDir()
	require.NoError(t, os.Symlink(target, filepath.Join(data, ".tmp")))

	_, err := PrepareTempDir(data)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTempDirUnsafe))
}

func TestPrepareTempDir_RefusesRegularFile(t *testing.T) {
	data := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(data, ".tmp"), []byte("x"), 0o600))

	_, err := PrepareTempDir(data)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTempDirUnsafe))
}

func TestPrepareTempDir_MissingParentFails(t *testing.T) {
	_, err := PrepareTempDir(filepath.Join(t.TempDir(), "nope"))
	require.Error(t, err)
}

func TestApplyTempDir_SetsEnvWhenUnset(t *testing.T) {
	t.Setenv("SQLITE_TMPDIR", "")
	data := t.TempDir()

	ApplyTempDir(data)

	assert.Equal(t, filepath.Join(data, ".tmp"), os.Getenv("SQLITE_TMPDIR"))
}

func TestApplyTempDir_HonoursOperatorValue(t *testing.T) {
	operator := t.TempDir()
	t.Setenv("SQLITE_TMPDIR", operator)
	data := t.TempDir()

	ApplyTempDir(data)

	assert.Equal(t, operator, os.Getenv("SQLITE_TMPDIR"))
	assert.NoDirExists(t, filepath.Join(data, ".tmp"), "no directory is created when the operator chose one")
}

func TestApplyTempDir_UnsafeDirLeavesEnvUnset(t *testing.T) {
	t.Setenv("SQLITE_TMPDIR", "")
	data := t.TempDir()
	require.NoError(t, os.Symlink(t.TempDir(), filepath.Join(data, ".tmp")))

	ApplyTempDir(data)

	assert.Empty(t, os.Getenv("SQLITE_TMPDIR"))
}

func TestEffectiveTempDir_UsesEnvWhenSet(t *testing.T) {
	small := t.TempDir()
	t.Setenv("SQLITE_TMPDIR", small)

	dir, err := EffectiveTempDir()
	require.NoError(t, err)
	assert.Equal(t, small, dir)
}

func TestEffectiveTempDir_EnvMustBeWritableDirectory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "f")
	require.NoError(t, os.WriteFile(file, nil, 0o600))

	for _, bad := range []string{"/definitely/not/there", file} {
		t.Setenv("SQLITE_TMPDIR", bad)
		_, err := EffectiveTempDir()
		require.Error(t, err, bad)
	}
}

func TestEffectiveTempDir_DefaultChain(t *testing.T) {
	t.Setenv("SQLITE_TMPDIR", "")

	dir, err := EffectiveTempDir()
	require.NoError(t, err)
	assert.Contains(t, defaultTempDirs, dir)
}

func TestEffectiveTempDir_NoWritableDefault(t *testing.T) {
	t.Setenv("SQLITE_TMPDIR", "")
	orig := defaultTempDirs
	defaultTempDirs = []string{"/definitely/not/there"}
	t.Cleanup(func() { defaultTempDirs = orig })

	_, err := EffectiveTempDir()
	require.Error(t, err)
}

func TestBuildDiskReport_UsesTheDirectorySQLiteUses(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SQLITE_TMPDIR", tmp)
	dbDir := t.TempDir()

	rep, err := BuildDiskReport(Stats{PageSize: 4096, PageCount: 100}, dbDir)
	require.NoError(t, err)

	avail, err := AvailableBytes(tmp)
	require.NoError(t, err)
	// The free-space figure comes from the configured temp dir; both temp dirs
	// are on the same test filesystem so the report must say so.
	assert.InDelta(t, float64(avail), float64(rep.TmpAvailable), float64(64<<20))
	assert.True(t, rep.SameFS)
}

func TestBuildDiskReport_DifferentFilesystems(t *testing.T) {
	t.Setenv("SQLITE_TMPDIR", "")
	if !writableDir("/dev/shm") {
		t.Skip("no writable /dev/shm")
	}
	t.Setenv("SQLITE_TMPDIR", "/dev/shm")

	rep, err := BuildDiskReport(Stats{PageSize: 4096, PageCount: 100}, "/proc")
	require.NoError(t, err)
	assert.False(t, rep.SameFS)
}

func TestBuildDiskReport_Errors(t *testing.T) {
	t.Setenv("SQLITE_TMPDIR", "/definitely/not/there")
	_, err := BuildDiskReport(Stats{}, t.TempDir())
	require.Error(t, err)

	t.Setenv("SQLITE_TMPDIR", t.TempDir())
	_, err = BuildDiskReport(Stats{}, "/definitely/not/there")
	require.Error(t, err)
}

// TestHelperTmpdirProbe runs in a re-executed test binary (see
// TestDriver_SQLiteTmpDirOnlyHonouredBeforeFirstOpen) because the driver reads
// SQLITE_TMPDIR once, process-wide.
func TestHelperTmpdirProbe(t *testing.T) {
	mode := os.Getenv("CHARON_TMPDIR_PROBE")
	if mode == "" {
		t.Skip("helper process only")
	}
	tmpDir := os.Getenv("CHARON_TMPDIR_PROBE_DIR")
	dbPath := os.Getenv("CHARON_TMPDIR_PROBE_DB")

	switch mode {
	case "before":
		require.NoError(t, os.Setenv("SQLITE_TMPDIR", tmpDir))
	case "after":
		warm, err := sql.Open(sqlite.DriverName, ":memory:")
		require.NoError(t, err)
		require.NoError(t, warm.QueryRow("SELECT 1").Scan(new(int)))
		require.NoError(t, os.Setenv("SQLITE_TMPDIR", tmpDir))
	}

	db := openScratch(t, dbPath, 0)
	fillScratch(t, db, 600, 100000, 3)

	done := make(chan error, 1)
	go func() {
		_, err := db.Exec("VACUUM")
		done <- err
	}()
	seen := map[string]bool{}
	for running := true; running; {
		select {
		case err := <-done:
			require.NoError(t, err)
			running = false
		default:
			for _, link := range openTempFDs(t) {
				seen[link] = true
			}
			time.Sleep(time.Millisecond)
		}
	}
	for link := range seen {
		fmt.Println("CHARON_TMPFILE:" + link)
	}
}

func runTmpdirProbe(t *testing.T, mode string) (tmpDir string, seen []string) {
	t.Helper()
	tmpDir = t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperTmpdirProbe$", "-test.v") //nolint:gosec // re-executes this test binary
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "SQLITE_TMPDIR=") {
			env = append(env, kv)
		}
	}
	env = append(env,
		"CHARON_TMPDIR_PROBE="+mode,
		"CHARON_TMPDIR_PROBE_DIR="+tmpDir,
		"CHARON_TMPDIR_PROBE_DB="+filepath.Join(t.TempDir(), "probe.db"),
	)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	for _, line := range strings.Split(string(out), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "CHARON_TMPFILE:"); ok {
			seen = append(seen, rest)
		}
	}
	return tmpDir, seen
}

func TestDriver_SQLiteTmpDirOnlyHonouredBeforeFirstOpen(t *testing.T) {
	t.Run("set before the first open it is honoured", func(t *testing.T) {
		dir, seen := runTmpdirProbe(t, "before")
		require.NotEmpty(t, seen, "VACUUM must have used a temp file")
		for _, link := range seen {
			assert.True(t, strings.HasPrefix(link, dir+string(filepath.Separator)), "%s should be under %s", link, dir)
		}
	})
	t.Run("set after the first open it is ignored", func(t *testing.T) {
		dir, seen := runTmpdirProbe(t, "after")
		require.NotEmpty(t, seen, "VACUUM must have used a temp file")
		for _, link := range seen {
			assert.False(t, strings.HasPrefix(link, dir+string(filepath.Separator)), "%s must not be under %s", link, dir)
		}
	})
}

func TestRootShouldSkip(t *testing.T) {
	tests := []struct {
		name     string
		euid     int
		ownerUID uint32
		want     bool
	}{
		{"root on a data dir owned by another user", 0, 1000, true},
		{"root on a root-owned data dir", 0, 0, false},
		{"service user on its own data dir", 1000, 1000, false},
		{"service user on another user's data dir", 1000, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, rootShouldSkip(tt.euid, tt.ownerUID))
		})
	}
}

// stubEUID makes the package believe the process runs as euid. The data
// directories of these tests belong to the real (non-root) test user, which is
// what a root-run command against a charon-owned volume looks like.
func stubEUID(t *testing.T, euid int) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("owner of t.TempDir would be root; covered by TestRootShouldSkip")
	}
	prev := currentEUID
	currentEUID = func() int { return euid }
	t.Cleanup(func() { currentEUID = prev })
}

func TestPrepareTempDir_RootLeavesAnotherUsersDataDirAlone(t *testing.T) {
	stubEUID(t, 0)
	data := t.TempDir()

	dir, err := PrepareTempDir(data)

	assert.ErrorIs(t, err, ErrTempDirSkipped)
	assert.Empty(t, dir)
	assert.NoDirExists(t, filepath.Join(data, ".tmp"), "a root process must not create a root-owned directory")
}

func TestPrepareTempDir_RootDoesNotTouchAnExistingTempDir(t *testing.T) {
	stubEUID(t, 0)
	data := t.TempDir()
	tmp := filepath.Join(data, ".tmp")
	require.NoError(t, os.Mkdir(tmp, 0o750))
	before, statErr := os.Lstat(tmp)
	require.NoError(t, statErr)

	_, err := PrepareTempDir(data)

	assert.ErrorIs(t, err, ErrTempDirSkipped, "no unsafe-directory error for the charon-owned directory")
	assert.NotErrorIs(t, err, ErrTempDirUnsafe)
	after, statErr := os.Lstat(tmp)
	require.NoError(t, statErr)
	assert.Equal(t, before.Mode().Perm(), after.Mode().Perm(), "mode untouched, not tightened to 0700")
	assert.NotEqual(t, tempDirMode, after.Mode().Perm())
}

func TestPrepareTempDir_RootWithAnUnreadableDataDirFallsThrough(t *testing.T) {
	stubEUID(t, 0)

	_, err := PrepareTempDir(filepath.Join(t.TempDir(), "nope"))

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrTempDirSkipped, "the existing error path reports it")
}

func TestPrepareTempDir_OwnerCheckUsesTheSeam(t *testing.T) {
	stubEUID(t, os.Geteuid()+1)
	data := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(data, ".tmp"), 0o700))

	_, err := PrepareTempDir(data)

	assert.ErrorIs(t, err, ErrTempDirUnsafe, "a directory owned by another uid than the (stubbed) euid is refused")
}

func TestApplyTempDir_RootLeavesTheEnvUnsetAndDoesNotWarn(t *testing.T) {
	stubEUID(t, 0)
	t.Setenv("SQLITE_TMPDIR", "")
	data := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(data, ".tmp"), 0o700))
	logs := captureLogs(t)

	ApplyTempDir(data)

	assert.Empty(t, os.Getenv("SQLITE_TMPDIR"))
	assert.NotContains(t, logs.String(), `"level":"warning"`)
}
