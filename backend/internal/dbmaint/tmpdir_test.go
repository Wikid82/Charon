package dbmaint

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

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
	require.NoError(t, os.Mkdir(dir, 0o755))

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
