package crowdsec

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The tests in this file swap the package seams (createTempFile, openInPlaceFile, renameFile, syncDir, fileOwner,
// processOwner). They must not call t.Parallel, or they would race with each other.

// staleTempAge is the sweep age the tests use, matching the acquisition writer.
const staleTempAge = 10 * time.Minute

// errAtomicInjected is returned by the fake file wrappers so tests can tell it from real errors.
var errAtomicInjected = errors.New("injected atomic write failure")

// failingTempFile wraps a real file and makes one step fail. Close always closes the real file so no descriptor
// leaks, even when the injected failure is the close itself.
type failingTempFile struct {
	tempFile
	step string
}

func (f failingTempFile) Write(b []byte) (int, error) {
	if f.step == "write" {
		return 0, errAtomicInjected
	}
	return f.tempFile.Write(b)
}

func (f failingTempFile) Chmod(mode fs.FileMode) error {
	if f.step == "chmod" {
		return errAtomicInjected
	}
	return f.tempFile.Chmod(mode)
}

func (f failingTempFile) Sync() error {
	if f.step == "sync" {
		return errAtomicInjected
	}
	return f.tempFile.Sync()
}

func (f failingTempFile) Close() error {
	if err := f.tempFile.Close(); err != nil {
		return err
	}
	if f.step == "close" {
		return errAtomicInjected
	}
	return nil
}

// injectTempStep makes the temporary file fail at step, and restores the seam on cleanup.
func injectTempStep(t *testing.T, step string) {
	t.Helper()
	orig := createTempFile
	t.Cleanup(func() { createTempFile = orig })
	createTempFile = func(dir, pattern string) (tempFile, error) {
		f, err := orig(dir, pattern)
		if err != nil {
			return nil, err
		}
		return failingTempFile{tempFile: f, step: step}, nil
	}
}

// injectInPlaceStep makes the in-place write fail at step, and restores the seam on cleanup.
func injectInPlaceStep(t *testing.T, step string) {
	t.Helper()
	orig := openInPlaceFile
	t.Cleanup(func() { openInPlaceFile = orig })
	openInPlaceFile = func(name string, mode fs.FileMode, noFollow bool) (tempFile, error) {
		f, err := orig(name, mode, noFollow)
		if err != nil {
			return nil, err
		}
		return failingTempFile{tempFile: f, step: step}, nil
	}
}

// countInPlaceOpens counts how many times the live file is opened for an in-place write.
func countInPlaceOpens(t *testing.T) *int {
	t.Helper()
	orig := openInPlaceFile
	t.Cleanup(func() { openInPlaceFile = orig })
	opens := new(int)
	openInPlaceFile = func(name string, mode fs.FileMode, noFollow bool) (tempFile, error) {
		*opens++
		return orig(name, mode, noFollow)
	}
	return opens
}

// recordInPlaceNoFollow records the noFollow flag of every in-place open and returns the slice.
func recordInPlaceNoFollow(t *testing.T) *[]bool {
	t.Helper()
	orig := openInPlaceFile
	t.Cleanup(func() { openInPlaceFile = orig })
	flags := &[]bool{}
	openInPlaceFile = func(name string, mode fs.FileMode, noFollow bool) (tempFile, error) {
		*flags = append(*flags, noFollow)
		return orig(name, mode, noFollow)
	}
	return flags
}

// stubAtomicRename makes the rename that replaces the live file fail with err.
func stubAtomicRename(t *testing.T, err error) {
	t.Helper()
	orig := renameFile
	t.Cleanup(func() { renameFile = orig })
	renameFile = func(string, string) error { return err }
}

// stubAtomicSyncDir replaces the directory sync seam; the returned slice records each directory synced.
func stubAtomicSyncDir(t *testing.T, syncErr error) *[]string {
	t.Helper()
	orig := syncDir
	t.Cleanup(func() { syncDir = orig })
	synced := &[]string{}
	syncDir = func(dir string) error {
		*synced = append(*synced, dir)
		return syncErr
	}
	return synced
}

// stubAtomicOwner makes the live file look owned by (liveUID, liveGID) while the process owner is (1000, 1000), and
// records every Chown the temporary file receives; chownErr is what each Chown returns. Real ownership is never
// changed, so these tests run the same way as root and as a regular user.
func stubAtomicOwner(t *testing.T, liveUID, liveGID int, chownErr error) *[][2]int {
	t.Helper()
	origOwner, origProcess, origCreate := fileOwner, processOwner, createTempFile
	t.Cleanup(func() {
		fileOwner = origOwner
		processOwner = origProcess
		createTempFile = origCreate
	})
	fileOwner = func(fs.FileInfo) (int, int, bool) { return liveUID, liveGID, true }
	processOwner = func() (int, int) { return 1000, 1000 }
	calls := &[][2]int{}
	createTempFile = func(dir, pattern string) (tempFile, error) {
		f, err := origCreate(dir, pattern)
		if err != nil {
			return nil, err
		}
		return chownTempFile{tempFile: f, calls: calls, err: chownErr}, nil
	}
	return calls
}

// chownTempFile records Chown calls and returns a configured error instead of changing ownership.
type chownTempFile struct {
	tempFile
	calls *[][2]int
	err   error
}

func (f chownTempFile) Chown(uid, gid int) error {
	*f.calls = append(*f.calls, [2]int{uid, gid})
	return f.err
}

// atomicFixture writes acquis.yaml with the content original, mode mode, and returns its path and directory.
func atomicFixture(t *testing.T, mode fs.FileMode) (acquis, dir string) {
	t.Helper()
	dir = t.TempDir()
	acquis = filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), mode))
	require.NoError(t, os.Chmod(acquis, mode))
	return acquis, dir
}

// requireAtomicEntries fails when dir does not hold exactly the given names.
func requireAtomicEntries(t *testing.T, dir string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	got := make([]string, 0, len(entries))
	for _, e := range entries {
		got = append(got, e.Name())
	}
	require.ElementsMatch(t, names, got)
}

func requireAtomicContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Equal(t, want, string(got))
}

func TestWriteFileAtomicTempStepFailuresKeepTheLiveFile(t *testing.T) {
	for _, step := range []string{"write", "chmod", "sync", "close"} {
		t.Run(step, func(t *testing.T) {
			injectTempStep(t, step)
			acquis, dir := atomicFixture(t, 0o600)

			res, err := WriteFileAtomic(acquis, []byte("new\n"), 0o600, AtomicWriteOptions{})
			require.ErrorIs(t, err, errAtomicInjected)
			require.Equal(t, AtomicWriteResult{}, res)
			requireAtomicContent(t, acquis, "original\n")
			requireAtomicEntries(t, dir, "acquis.yaml")
		})
	}
}

func TestCreateTempFileReturnsANilInterfaceOnError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-dir")

	f, err := createTempFile(missing, "acquis.yaml.tmp-*")
	require.Error(t, err)
	// A typed nil *os.File inside the interface would compare unequal to nil and hide the failure.
	require.True(t, f == nil, "the seam must return a nil interface on error")
}

func TestWriteFileAtomicReportsAMissingDirectory(t *testing.T) {
	acquis := filepath.Join(t.TempDir(), "missing", "acquis.yaml")

	_, err := WriteFileAtomic(acquis, []byte("new\n"), 0o600, AtomicWriteOptions{})
	require.Error(t, err)
	require.NoFileExists(t, acquis)
}

func TestWriteFileAtomicReportsAStatFailureThatIsNotMissing(t *testing.T) {
	// The parent is a regular file, so the stat fails with ENOTDIR rather than ErrNotExist.
	parent := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(parent, []byte("x"), 0o600))

	_, err := WriteFileAtomic(filepath.Join(parent, "acquis.yaml"), []byte("new\n"), 0o600, AtomicWriteOptions{})
	require.ErrorIs(t, err, syscall.ENOTDIR)
}

func TestWriteFileAtomicCreatesANewFileWithoutOwnerOrBackup(t *testing.T) {
	calls := stubAtomicOwner(t, 2000, 2000, syscall.EPERM)
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")

	res, err := WriteFileAtomic(acquis, []byte("new\n"), 0o644, AtomicWriteOptions{KeepOwner: true, InPlaceFallback: true})
	require.NoError(t, err)
	require.Equal(t, AtomicWriteResult{}, res, "a new file has no owner to keep and is not written in place")
	require.Empty(t, *calls, "a new file is owned by the process already")
	requireAtomicContent(t, acquis, "new\n")
	requireAtomicEntries(t, dir, "acquis.yaml")
}

func TestWriteFileAtomicLeavesOwnerAloneWhenItMatchesTheProcess(t *testing.T) {
	calls := stubAtomicOwner(t, 1000, 1000, nil)
	acquis, _ := atomicFixture(t, 0o600)

	res, err := WriteFileAtomic(acquis, []byte("new\n"), 0o600, AtomicWriteOptions{KeepOwner: true})
	require.NoError(t, err)
	require.Empty(t, *calls, "chown is never needed when the live owner is the process owner")
	require.True(t, res.OwnerKept, "the owner already matches")
}

func TestWriteFileAtomicChangesOwnerByRenameWhenChownSucceeds(t *testing.T) {
	calls := stubAtomicOwner(t, 2000, 2000, nil)
	acquis, _ := atomicFixture(t, 0o600)
	before, err := os.Stat(acquis)
	require.NoError(t, err)

	res, err := WriteFileAtomic(acquis, []byte("new\n"), 0o600, AtomicWriteOptions{KeepOwner: true, InPlaceFallback: true})
	require.NoError(t, err)
	require.Equal(t, AtomicWriteResult{OwnerKept: true}, res)
	require.Equal(t, [][2]int{{2000, 2000}}, *calls, "the temp file takes the owner of the live file")
	after, err := os.Stat(acquis)
	require.NoError(t, err)
	require.False(t, os.SameFile(before, after), "a successful chown keeps the rename path")
	requireAtomicContent(t, acquis, "new\n")
}

func TestWriteFileAtomicWritesInPlaceWhenChownIsNotPermitted(t *testing.T) {
	calls := stubAtomicOwner(t, 2000, 2000, syscall.EPERM)
	acquis, dir := atomicFixture(t, 0o600)
	before, err := os.Stat(acquis)
	require.NoError(t, err)

	res, err := WriteFileAtomic(acquis, []byte("new\n"), 0o600, AtomicWriteOptions{KeepOwner: true, InPlaceFallback: true})
	require.NoError(t, err)
	require.Equal(t, AtomicWriteResult{InPlace: true, OwnerKept: true}, res)
	require.Len(t, *calls, 1)
	after, err := os.Stat(acquis)
	require.NoError(t, err)
	require.True(t, os.SameFile(before, after), "the inode, and with it the owner and any bind mount, is kept")
	requireAtomicContent(t, acquis, "new\n")
	requireAtomicEntries(t, dir, "acquis.yaml")
}

func TestWriteFileAtomicRenamesWhenInPlaceOpenIsDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("file permissions do not bind root")
	}
	stubAtomicOwner(t, 2000, 2000, syscall.EPERM)
	acquis, dir := atomicFixture(t, 0o400)
	before, err := os.Stat(acquis)
	require.NoError(t, err)

	res, err := WriteFileAtomic(acquis, []byte("new\n"), 0o400, AtomicWriteOptions{KeepOwner: true, InPlaceFallback: true})
	require.NoError(t, err, "the last resort is the rename, with a warning")
	require.False(t, res.InPlace)
	after, err := os.Stat(acquis)
	require.NoError(t, err)
	require.False(t, os.SameFile(before, after))
	requireAtomicContent(t, acquis, "new\n")
	requireAtomicEntries(t, dir, "acquis.yaml")
}

func TestWriteFileAtomicRenamesOnEPERMWithoutInPlaceFallback(t *testing.T) {
	calls := stubAtomicOwner(t, 2000, 2000, syscall.EPERM)
	inPlaceOpens := countInPlaceOpens(t)
	acquis, _ := atomicFixture(t, 0o600)
	before, err := os.Stat(acquis)
	require.NoError(t, err)

	res, err := WriteFileAtomic(acquis, []byte("new\n"), 0o600, AtomicWriteOptions{KeepOwner: true})
	require.NoError(t, err)
	require.Len(t, *calls, 1)
	require.Zero(t, *inPlaceOpens, "without the fallback the live file is never opened")
	require.False(t, res.InPlace)
	after, err := os.Stat(acquis)
	require.NoError(t, err)
	require.False(t, os.SameFile(before, after))
	requireAtomicContent(t, acquis, "new\n")
}

// Only a refused chown (EPERM) makes the write touch the live file in place, so its owner is kept. Any other chown
// failure saves through the atomic rename, and the live file is never opened for in-place writing.
func TestWriteFileAtomicSavesByChownErrorKind(t *testing.T) {
	tests := []struct {
		name    string
		chown   error
		inPlace bool
	}{
		{name: "EPERM saves in place", chown: syscall.EPERM, inPlace: true},
		{name: "EINVAL saves by rename", chown: syscall.EINVAL, inPlace: false},
		{name: "ENOTSUP saves by rename", chown: syscall.ENOTSUP, inPlace: false},
		{name: "EROFS saves by rename", chown: syscall.EROFS, inPlace: false},
		{name: "EIO saves by rename", chown: syscall.EIO, inPlace: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls := stubAtomicOwner(t, 2000, 2000, tc.chown)
			inPlaceOpens := countInPlaceOpens(t)
			acquis, dir := atomicFixture(t, 0o600)
			before, err := os.Stat(acquis)
			require.NoError(t, err)

			res, err := WriteFileAtomic(acquis, []byte("new\n"), 0o600, AtomicWriteOptions{KeepOwner: true, InPlaceFallback: true})
			require.NoError(t, err)
			require.Len(t, *calls, 1)
			after, err := os.Stat(acquis)
			require.NoError(t, err)
			require.Equal(t, tc.inPlace, os.SameFile(before, after), "in place keeps the inode (and owner); rename replaces it")
			require.Equal(t, tc.inPlace, res.InPlace)
			if tc.inPlace {
				require.Equal(t, 1, *inPlaceOpens, "EPERM writes the live file itself")
			} else {
				require.Zero(t, *inPlaceOpens, "a non-EPERM chown failure must never open the live file for in-place writing")
			}
			requireAtomicContent(t, acquis, "new\n")
			requireAtomicEntries(t, dir, "acquis.yaml")
		})
	}
}

func TestWriteFileAtomicInPlaceFailureIsPartial(t *testing.T) {
	for _, step := range []string{"write", "sync", "close"} {
		t.Run("EPERM "+step, func(t *testing.T) {
			stubAtomicOwner(t, 2000, 2000, syscall.EPERM)
			injectInPlaceStep(t, step)
			acquis, _ := atomicFixture(t, 0o600)

			_, err := WriteFileAtomic(acquis, []byte("new\n"), 0o600, AtomicWriteOptions{KeepOwner: true, InPlaceFallback: true})
			require.ErrorIs(t, err, ErrPartialInPlaceWrite, "the live file was opened, so it may be half written")
			require.ErrorIs(t, err, errAtomicInjected)
		})
		t.Run("refused rename "+step, func(t *testing.T) {
			stubAtomicRename(t, &os.LinkError{Op: "rename", Err: syscall.EBUSY})
			injectInPlaceStep(t, step)
			acquis, _ := atomicFixture(t, 0o600)

			_, err := WriteFileAtomic(acquis, []byte("new\n"), 0o600, AtomicWriteOptions{InPlaceFallback: true})
			require.ErrorIs(t, err, ErrPartialInPlaceWrite)
			require.ErrorIs(t, err, errAtomicInjected)
		})
	}
}

// A refused in-place open on the rename fallback also reports ErrPartialInPlaceWrite, so the caller keeps its backup
// whatever the cause (this is the save behavior the acquisition writer has always had).
func TestWriteFileAtomicRefusedRenameWithDeniedOpenIsPartial(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("file permissions do not bind root")
	}
	stubAtomicRename(t, &os.LinkError{Op: "rename", Err: syscall.EBUSY})
	acquis, _ := atomicFixture(t, 0o400)

	_, err := WriteFileAtomic(acquis, []byte("new\n"), 0o400, AtomicWriteOptions{InPlaceFallback: true})
	require.ErrorIs(t, err, ErrPartialInPlaceWrite)
	requireAtomicContent(t, acquis, "original\n")
}

func TestWriteFileAtomicFallsBackInPlaceOnRefusedRename(t *testing.T) {
	for _, errno := range []syscall.Errno{syscall.EBUSY, syscall.EXDEV, syscall.EROFS} {
		t.Run(errno.Error(), func(t *testing.T) {
			stubAtomicRename(t, &os.LinkError{Op: "rename", Err: errno})
			inPlaceOpens := countInPlaceOpens(t)
			synced := stubAtomicSyncDir(t, nil)
			acquis, dir := atomicFixture(t, 0o640)

			res, err := WriteFileAtomic(acquis, []byte("new\n"), 0o640, AtomicWriteOptions{InPlaceFallback: true})
			require.NoError(t, err)
			require.Equal(t, AtomicWriteResult{InPlace: true, OwnerKept: true}, res)
			require.Equal(t, 1, *inPlaceOpens)
			require.Empty(t, *synced, "no rename happened, so there is no new directory entry to sync")
			info, err := os.Stat(acquis)
			require.NoError(t, err)
			require.Equal(t, fs.FileMode(0o640), info.Mode().Perm(), "the live mode is kept")
			requireAtomicContent(t, acquis, "new\n")
			requireAtomicEntries(t, dir, "acquis.yaml")
		})
	}
}

func TestWriteFileAtomicRefusedRenameWithoutFallbackFails(t *testing.T) {
	stubAtomicRename(t, &os.LinkError{Op: "rename", Err: syscall.EBUSY})
	inPlaceOpens := countInPlaceOpens(t)
	acquis, dir := atomicFixture(t, 0o600)

	_, err := WriteFileAtomic(acquis, []byte("new\n"), 0o600, AtomicWriteOptions{})
	require.ErrorIs(t, err, syscall.EBUSY)
	require.Zero(t, *inPlaceOpens, "without the fallback the live file is never written in place")
	requireAtomicContent(t, acquis, "original\n")
	requireAtomicEntries(t, dir, "acquis.yaml")
}

// symlinkFixture writes a real file with the content original and a symlink named acquis.yaml that points at it.
// It returns the link path, the target path and the directory.
func symlinkFixture(t *testing.T) (link, target, dir string) {
	t.Helper()
	dir = t.TempDir()
	target = filepath.Join(dir, "real.yaml")
	require.NoError(t, os.WriteFile(target, []byte("original\n"), 0o600))
	link = filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.Symlink(target, link))
	return link, target, dir
}

// With NoFollow the in-place write refuses a symlink at the target before it opens anything, so the link target is
// untouched and the refusal is not a partial write.
func TestWriteFileAtomicNoFollowRefusesASymlinkInPlace(t *testing.T) {
	stubAtomicRename(t, &os.LinkError{Op: "rename", Err: syscall.EBUSY})
	flags := recordInPlaceNoFollow(t)
	link, target, dir := symlinkFixture(t)

	_, err := WriteFileAtomic(link, []byte("new\n"), 0o600, AtomicWriteOptions{InPlaceFallback: true, NoFollow: true})
	require.ErrorIs(t, err, syscall.ELOOP)
	require.NotErrorIs(t, err, ErrPartialInPlaceWrite, "the open was refused, so nothing was touched")
	require.Equal(t, []bool{true}, *flags, "the in-place open carries O_NOFOLLOW")
	requireAtomicContent(t, target, "original\n")
	info, err := os.Lstat(link)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSymlink, "the link is left as it was")
	requireAtomicEntries(t, dir, "acquis.yaml", "real.yaml")
}

// The zero value keeps today's acquisition behavior: the in-place write follows the symlink to its target.
func TestWriteFileAtomicWithoutNoFollowWritesThroughASymlink(t *testing.T) {
	stubAtomicRename(t, &os.LinkError{Op: "rename", Err: syscall.EBUSY})
	flags := recordInPlaceNoFollow(t)
	link, target, _ := symlinkFixture(t)

	res, err := WriteFileAtomic(link, []byte("new\n"), 0o600, AtomicWriteOptions{InPlaceFallback: true})
	require.NoError(t, err)
	require.True(t, res.InPlace)
	require.Equal(t, []bool{false}, *flags)
	requireAtomicContent(t, target, "new\n")
	info, err := os.Lstat(link)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSymlink, "the link itself is kept")
}

func TestWriteFileAtomicSkipsDirectorySyncWhenRenameFails(t *testing.T) {
	synced := stubAtomicSyncDir(t, nil)
	stubAtomicRename(t, errors.New("boom"))
	acquis, _ := atomicFixture(t, 0o600)

	_, err := WriteFileAtomic(acquis, []byte("new\n"), 0o600, AtomicWriteOptions{InPlaceFallback: true})
	require.Error(t, err)
	require.Empty(t, *synced)
}

func TestWriteFileAtomicSyncsTheDirectoryOnceAfterRename(t *testing.T) {
	synced := stubAtomicSyncDir(t, nil)
	acquis, dir := atomicFixture(t, 0o600)

	_, err := WriteFileAtomic(acquis, []byte("new\n"), 0o600, AtomicWriteOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{dir}, *synced)
}

func TestWriteFileAtomicIgnoresADirectorySyncFailure(t *testing.T) {
	synced := stubAtomicSyncDir(t, errors.New("sync refused"))
	acquis, _ := atomicFixture(t, 0o600)

	_, err := WriteFileAtomic(acquis, []byte("new\n"), 0o600, AtomicWriteOptions{})
	require.NoError(t, err, "the new content is already in place, so a sync failure only logs")
	require.Len(t, *synced, 1)
	requireAtomicContent(t, acquis, "new\n")
}

// TestWriteFileAtomicBestEffortStepsDoNotFailTheWrite makes the directory unreadable (mode 0o300: write and search
// only). The stale-temp sweep cannot list it and the directory sync cannot open it; both must only log.
func TestWriteFileAtomicBestEffortStepsDoNotFailTheWrite(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not bind root")
	}
	acquis, dir := atomicFixture(t, 0o600)
	stale := acquis + ".tmp-111"
	require.NoError(t, os.WriteFile(stale, []byte("x"), 0o600))
	old := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(stale, old, old))
	require.NoError(t, os.Chmod(dir, 0o300))       //nolint:gosec // G302: test needs a directory that cannot be read
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: restore owner access so cleanup can remove it

	_, err := WriteFileAtomic(acquis, []byte("new\n"), 0o600, AtomicWriteOptions{SweepTempsOlderThan: staleTempAge})
	require.NoError(t, err, "the write does not depend on listing or syncing the directory")
	requireAtomicContent(t, acquis, "new\n")

	require.NoError(t, os.Chmod(dir, 0o700)) //nolint:gosec // G302: restore owner access to list the directory
	requireAtomicEntries(t, dir, "acquis.yaml", filepath.Base(stale))
}

func TestWriteFileAtomicSweepsStaleTempsOfItsOwnPattern(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "bouncer_key")
	old := time.Now().Add(-time.Hour)
	stale := key + ".tmp-111"
	require.NoError(t, os.WriteFile(stale, []byte("x"), 0o600))
	require.NoError(t, os.Chtimes(stale, old, old))
	nearMiss := key + ".tmpx"
	require.NoError(t, os.WriteFile(nearMiss, []byte("x"), 0o600))
	require.NoError(t, os.Chtimes(nearMiss, old, old))

	_, err := WriteFileAtomic(key, []byte("key\n"), 0o600, AtomicWriteOptions{
		TempPattern:         "bouncer_key.tmp-*",
		SweepTempsOlderThan: staleTempAge,
		Now:                 time.Now(),
	})
	require.NoError(t, err)
	requireAtomicEntries(t, dir, "bouncer_key", filepath.Base(nearMiss))
}

func TestWriteFileAtomicWithZeroOptionsReplacesTheFile(t *testing.T) {
	acquis, dir := atomicFixture(t, 0o600)

	res, err := WriteFileAtomic(acquis, []byte("new\n"), 0o600, AtomicWriteOptions{})
	require.NoError(t, err)
	require.Equal(t, AtomicWriteResult{}, res)
	requireAtomicContent(t, acquis, "new\n")
	requireAtomicEntries(t, dir, "acquis.yaml")
}

func TestSweepStaleTempsRemovesOnlyOldRegularTemps(t *testing.T) {
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("live\n"), 0o600))
	now := time.Now()
	old := now.Add(-time.Hour)

	stale := acquis + ".tmp-111"
	require.NoError(t, os.WriteFile(stale, []byte("x"), 0o600))
	require.NoError(t, os.Chtimes(stale, old, old))
	fresh := acquis + ".tmp-222"
	require.NoError(t, os.WriteFile(fresh, []byte("x"), 0o600))
	link := acquis + ".tmp-333"
	require.NoError(t, os.Symlink(acquis, link))
	require.NoError(t, os.Chtimes(link, old, old))
	subdir := acquis + ".tmp-444"
	require.NoError(t, os.Mkdir(subdir, 0o700))
	require.NoError(t, os.Chtimes(subdir, old, old))
	unrelated := acquis + ".backup.20200101-000000.000000"
	require.NoError(t, os.WriteFile(unrelated, []byte("x"), 0o600))
	require.NoError(t, os.Chtimes(unrelated, old, old))

	sweepStaleTemps(dir, "acquis.yaml.tmp-", staleTempAge, now)
	requireAtomicEntries(t, dir, "acquis.yaml", filepath.Base(fresh), filepath.Base(link), filepath.Base(subdir), filepath.Base(unrelated))

	// Far enough in the future that the fresh temp is old too: only the IsRegular check now keeps the symlink and the
	// directory, and the old regular temp is already gone.
	sweepStaleTemps(dir, "acquis.yaml.tmp-", staleTempAge, time.Now().Add(24*time.Hour))
	requireAtomicEntries(t, dir, "acquis.yaml", filepath.Base(link), filepath.Base(subdir), filepath.Base(unrelated))
}

func TestSweepStaleTempsLeavesNearMissNamesAlone(t *testing.T) {
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	old := time.Now().Add(-time.Hour)
	nearMisses := []string{acquis + ".tmpX", filepath.Join(dir, "other.yaml.tmp-1")}
	for _, name := range nearMisses {
		require.NoError(t, os.WriteFile(name, []byte("x"), 0o600))
		require.NoError(t, os.Chtimes(name, old, old))
	}

	sweepStaleTemps(dir, "acquis.yaml.tmp-", staleTempAge, time.Now().Add(24*time.Hour))
	requireAtomicEntries(t, dir, filepath.Base(nearMisses[0]), filepath.Base(nearMisses[1]))
}

func TestSweepStaleTempsToleratesAMissingDirectory(t *testing.T) {
	require.NotPanics(t, func() {
		sweepStaleTemps(filepath.Join(t.TempDir(), "missing"), "acquis.yaml.tmp-", staleTempAge, time.Now())
	})
}

func TestSweepStaleTempsSkipsEntriesItCannotInspectOrRemove(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not bind root")
	}
	dir := t.TempDir()
	stale := filepath.Join(dir, "acquis.yaml.tmp-111")
	require.NoError(t, os.WriteFile(stale, []byte("x"), 0o600))
	old := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(stale, old, old))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: restore owner access so cleanup can remove it

	// Readable but not searchable: the names list, but the entry cannot be inspected.
	require.NoError(t, os.Chmod(dir, 0o400)) //nolint:gosec // G302: test needs a directory that cannot be searched
	sweepStaleTemps(dir, "acquis.yaml.tmp-", staleTempAge, time.Now())
	// Searchable but not writable: the entry is inspected, but cannot be removed.
	require.NoError(t, os.Chmod(dir, 0o500)) //nolint:gosec // G302: test needs a directory that cannot be written
	sweepStaleTemps(dir, "acquis.yaml.tmp-", staleTempAge, time.Now())

	require.NoError(t, os.Chmod(dir, 0o700)) //nolint:gosec // G302: restore owner access to check the entry
	require.FileExists(t, stale, "entries that cannot be inspected or removed are skipped, never fatal")
}

func TestKeepOwnerLeavesAnUnknownOwnerAlone(t *testing.T) {
	origOwner := fileOwner
	t.Cleanup(func() { fileOwner = origOwner })
	fileOwner = func(fs.FileInfo) (int, int, bool) { return 0, 0, false }
	live := filepath.Join(t.TempDir(), "live")
	require.NoError(t, os.WriteFile(live, []byte("x"), 0o600))
	info, err := os.Stat(live)
	require.NoError(t, err)
	f, err := os.CreateTemp(t.TempDir(), "acquis.yaml.tmp-*")
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()
	var calls [][2]int
	tmp := chownTempFile{tempFile: f, calls: &calls, err: syscall.EIO}

	kept, err := keepOwner(tmp, info)
	require.NoError(t, err, "an owner that cannot be read is not a failure")
	require.False(t, kept)
	require.Empty(t, calls, "without a known owner there is nothing to chown to")
}

// noStatInfo is a FileInfo without a platform stat record, so its owner cannot be read.
type noStatInfo struct{ fs.FileInfo }

func (noStatInfo) Sys() any { return nil }

func TestFileOwnerReadsTheOwnerOfARealFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "acquis.yaml")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
	info, err := os.Stat(path)
	require.NoError(t, err)

	uid, _, ok := fileOwner(info)
	require.True(t, ok)
	require.Equal(t, os.Geteuid(), uid, "the test created the file, so this process owns it")
	_, _, ok = fileOwner(noStatInfo{FileInfo: info})
	require.False(t, ok, "without a stat record the owner is unknown")
}

func TestWriteSyncCloseReportsTheFirstFailure(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "acquis.yaml.tmp-*")
	require.NoError(t, err)

	require.NoError(t, WriteSyncClose(f, []byte("data")), "a healthy file writes, syncs and closes")
	require.Error(t, WriteSyncClose(f, []byte("data")), "a closed file reports the failure")
}
