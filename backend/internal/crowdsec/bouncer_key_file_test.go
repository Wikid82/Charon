package crowdsec

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// The tests in this file swap the package seams (see atomicwrite_test.go). They must not call t.Parallel.

const bouncerKeyTestKey = "bouncer_test_value_0123456789abcdefghijklmnop"

// bouncerKeyFixture returns the key path inside a fresh directory that does not exist yet.
func bouncerKeyFixture(t *testing.T) (path, dir string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "keys")
	return filepath.Join(dir, "bouncer_key"), dir
}

// writeLiveBouncerKey creates an existing key file holding "old-key" with the given mode.
func writeLiveBouncerKey(t *testing.T, path string, mode fs.FileMode) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte("old-key\n"), mode))
	require.NoError(t, os.Chmod(path, mode))
}

// keyLogs captures the package logger for one test and returns the buffer.
func keyLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	logger.Init(false, &buf)
	t.Cleanup(func() { logger.Init(false, io.Discard) })
	return &buf
}

func TestSaveBouncerKeyRefusesAnEmptyOrBlankKey(t *testing.T) {
	for _, key := range []string{"", " ", "\n\t "} {
		path, dir := bouncerKeyFixture(t)

		err := SaveBouncerKey(path, key)
		require.Error(t, err, "key %q", key)
		require.Contains(t, err.Error(), "cannot save empty key")
		require.NoDirExists(t, dir, "nothing is created for a refused key")
	}
}

func TestSaveBouncerKeyWritesANewKeyWithSecureModeAndDirectory(t *testing.T) {
	path, dir := bouncerKeyFixture(t)

	require.NoError(t, SaveBouncerKey(path, bouncerKeyTestKey))

	got, err := os.ReadFile(path) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Equal(t, bouncerKeyTestKey+"\n", string(got))
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, fs.FileMode(0o600), info.Mode().Perm())
	dirInfo, err := os.Stat(dir)
	require.NoError(t, err)
	require.Equal(t, fs.FileMode(0o750), dirInfo.Mode().Perm(), "a new key directory is 0750")
	requireAtomicEntries(t, dir, "bouncer_key")
}

func TestSaveBouncerKeyLeavesAnExistingDirectoryMode(t *testing.T) {
	path, dir := bouncerKeyFixture(t)
	require.NoError(t, os.MkdirAll(dir, 0o700))

	require.NoError(t, SaveBouncerKey(path, bouncerKeyTestKey))

	dirInfo, err := os.Stat(dir)
	require.NoError(t, err)
	require.Equal(t, fs.FileMode(0o700), dirInfo.Mode().Perm(), "an existing directory keeps its mode")
}

func TestSaveBouncerKeyRenamesOverAnExistingKeyByDefault(t *testing.T) {
	path, _ := bouncerKeyFixture(t)
	writeLiveBouncerKey(t, path, 0o644)
	before, err := os.Stat(path)
	require.NoError(t, err)
	inPlaceOpens := countInPlaceOpens(t)

	require.NoError(t, SaveBouncerKey(path, bouncerKeyTestKey))

	after, err := os.Stat(path)
	require.NoError(t, err)
	require.False(t, os.SameFile(before, after), "the normal save replaces the inode by rename")
	require.Equal(t, fs.FileMode(0o600), after.Mode().Perm(), "the mode is forced to 0600 on the rename path")
	require.Zero(t, *inPlaceOpens, "no in-place write happens when the rename works")
	requireAtomicContent(t, path, bouncerKeyTestKey+"\n")
}

func TestSaveBouncerKeyWritesInPlaceOnlyWhenTheRenameIsRefused(t *testing.T) {
	for _, errno := range []syscall.Errno{syscall.EBUSY, syscall.EXDEV, syscall.EROFS} {
		t.Run(errno.Error(), func(t *testing.T) {
			path, dir := bouncerKeyFixture(t)
			writeLiveBouncerKey(t, path, 0o644)
			before, err := os.Stat(path)
			require.NoError(t, err)
			stubAtomicRename(t, &os.LinkError{Op: "rename", Err: errno})
			inPlaceOpens := countInPlaceOpens(t)

			require.NoError(t, SaveBouncerKey(path, bouncerKeyTestKey))

			require.Equal(t, 1, *inPlaceOpens)
			after, err := os.Stat(path)
			require.NoError(t, err)
			require.True(t, os.SameFile(before, after), "a bind-mounted file keeps its inode")
			require.Equal(t, fs.FileMode(0o600), after.Mode().Perm(), "ForceMode sets 0600 on the in-place path")
			requireAtomicContent(t, path, bouncerKeyTestKey+"\n")
			requireAtomicEntries(t, dir, "bouncer_key")
		})
	}
}

func TestSaveBouncerKeyDoesNotWriteInPlaceForOtherRenameFailures(t *testing.T) {
	path, dir := bouncerKeyFixture(t)
	writeLiveBouncerKey(t, path, 0o600)
	stubAtomicRename(t, &os.LinkError{Op: "rename", Err: syscall.EIO})
	inPlaceOpens := countInPlaceOpens(t)

	err := SaveBouncerKey(path, bouncerKeyTestKey)

	require.Error(t, err)
	require.Zero(t, *inPlaceOpens, "an I/O error on rename never falls back to writing the live key")
	requireAtomicContent(t, path, "old-key\n")
	requireAtomicEntries(t, dir, "bouncer_key")
}

func TestSaveBouncerKeyNeverChangesTheOwner(t *testing.T) {
	path, _ := bouncerKeyFixture(t)
	writeLiveBouncerKey(t, path, 0o600)
	chownCalls := stubAtomicOwner(t, 2000, 2000, nil)

	require.NoError(t, SaveBouncerKey(path, bouncerKeyTestKey))

	require.Empty(t, *chownCalls, "the key is Charon's own file, so no Chown is attempted")
	requireAtomicContent(t, path, bouncerKeyTestKey+"\n")
}

func TestSaveBouncerKeyReplacesASymlinkedKeyWithARegularFile(t *testing.T) {
	path, dir := bouncerKeyFixture(t)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	target := filepath.Join(t.TempDir(), "elsewhere")
	require.NoError(t, os.WriteFile(target, []byte("other-key\n"), 0o600))
	require.NoError(t, os.Symlink(target, path))

	require.NoError(t, SaveBouncerKey(path, bouncerKeyTestKey))

	info, err := os.Lstat(path)
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular(), "the link is replaced by a regular file")
	require.Equal(t, fs.FileMode(0o600), info.Mode().Perm())
	requireAtomicContent(t, path, bouncerKeyTestKey+"\n")
	requireAtomicContent(t, target, "other-key\n")
}

func TestSaveBouncerKeyRemovesALegacyTempFileOnly(t *testing.T) {
	path, dir := bouncerKeyFixture(t)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	legacy := path + ".tmp"
	require.NoError(t, os.WriteFile(legacy, []byte("leftover\n"), 0o600))

	require.NoError(t, SaveBouncerKey(path, bouncerKeyTestKey))

	requireAtomicEntries(t, dir, "bouncer_key")
}

func TestSaveBouncerKeyLeavesALegacyTempDirectoryAlone(t *testing.T) {
	path, dir := bouncerKeyFixture(t)
	legacyDir := path + ".tmp"
	require.NoError(t, os.MkdirAll(legacyDir, 0o750))

	require.NoError(t, SaveBouncerKey(path, bouncerKeyTestKey))

	info, err := os.Stat(legacyDir)
	require.NoError(t, err)
	require.True(t, info.IsDir(), "a directory is never removed as a temp file")
	requireAtomicEntries(t, dir, "bouncer_key", "bouncer_key.tmp")
}

func TestSaveBouncerKeyCrashBeforeRenameKeepsTheOldKey(t *testing.T) {
	path, dir := bouncerKeyFixture(t)
	writeLiveBouncerKey(t, path, 0o600)
	stubAtomicRename(t, &os.LinkError{Op: "rename", Err: syscall.EIO})

	require.Error(t, SaveBouncerKey(path, bouncerKeyTestKey))

	requireAtomicContent(t, path, "old-key\n")
	requireAtomicEntries(t, dir, "bouncer_key")
}

func TestSaveBouncerKeySweepsOnlyStaleTempFiles(t *testing.T) {
	path, dir := bouncerKeyFixture(t)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	stale := path + ".tmp-111"
	fresh := path + ".tmp-222"
	require.NoError(t, os.WriteFile(stale, []byte("x"), 0o600))
	old := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(stale, old, old))
	require.NoError(t, os.WriteFile(fresh, []byte("y"), 0o600))

	require.NoError(t, SaveBouncerKey(path, bouncerKeyTestKey))

	requireAtomicEntries(t, dir, "bouncer_key", filepath.Base(fresh))
}

func TestSaveBouncerKeyConcurrentWritersLeaveOneWholeKey(t *testing.T) {
	path, dir := bouncerKeyFixture(t)
	keys := []string{bouncerKeyTestKey + "-a", bouncerKeyTestKey + "-b"}

	errs := make(chan error, 20)
	var wg sync.WaitGroup
	for _, key := range keys {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			for range 10 {
				errs <- SaveBouncerKey(path, key)
			}
		}(key)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	got, err := os.ReadFile(path) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Contains(t, []string{keys[0] + "\n", keys[1] + "\n"}, string(got))
	requireAtomicEntries(t, dir, "bouncer_key")
}

func TestSaveBouncerKeyLogsNeverContainTheKey(t *testing.T) {
	buf := keyLogs(t)
	path, _ := bouncerKeyFixture(t)
	require.NoError(t, SaveBouncerKey(path, bouncerKeyTestKey))

	stubAtomicRename(t, &os.LinkError{Op: "rename", Err: syscall.EIO})
	require.Error(t, SaveBouncerKey(path, "failed_test_value_0123456789abcdefghijklmnop"))

	require.False(t, strings.Contains(buf.String(), bouncerKeyTestKey), "the key must never reach the logs")
}
