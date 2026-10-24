package handlers

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

// errInjectedAcquisitionStep is returned by the fake file wrappers so tests can tell it from real errors.
var errInjectedAcquisitionStep = errors.New("injected acquisition step failure")

// failingAcquisitionFile wraps a real file and makes one step fail. Close always closes the real file so
// no descriptor leaks, even when the injected failure is the close itself.
type failingAcquisitionFile struct {
	acquisitionFile
	step string
}

func (f failingAcquisitionFile) Write(b []byte) (int, error) {
	if f.step == "write" {
		return 0, errInjectedAcquisitionStep
	}
	return f.acquisitionFile.Write(b)
}

func (f failingAcquisitionFile) Chmod(mode fs.FileMode) error {
	if f.step == "chmod" {
		return errInjectedAcquisitionStep
	}
	return f.acquisitionFile.Chmod(mode)
}

func (f failingAcquisitionFile) Sync() error {
	if f.step == "sync" {
		return errInjectedAcquisitionStep
	}
	return f.acquisitionFile.Sync()
}

func (f failingAcquisitionFile) Close() error {
	if err := f.acquisitionFile.Close(); err != nil {
		return err
	}
	if f.step == "close" {
		return errInjectedAcquisitionStep
	}
	return nil
}

// failAcquisitionStep makes the temp file ("temp") or the backup file ("backup") fail at step, and restores
// the seams on cleanup. Tests that use it must not run in parallel.
func failAcquisitionStep(t *testing.T, target, step string) {
	t.Helper()
	origCreate, origOpen := createAcquisitionTempFile, openAcquisitionBackupFile
	t.Cleanup(func() {
		createAcquisitionTempFile = origCreate
		openAcquisitionBackupFile = origOpen
	})
	switch target {
	case "temp":
		createAcquisitionTempFile = func(dir, pattern string) (acquisitionFile, error) {
			f, err := origCreate(dir, pattern)
			if err != nil {
				return nil, err
			}
			return failingAcquisitionFile{acquisitionFile: f, step: step}, nil
		}
	case "backup":
		openAcquisitionBackupFile = func(name string, mode fs.FileMode) (acquisitionFile, error) {
			f, err := origOpen(name, mode)
			if err != nil {
				return nil, err
			}
			return failingAcquisitionFile{acquisitionFile: f, step: step}, nil
		}
	default:
		t.Fatalf("unknown acquisition target %q", target)
	}
}

func TestReplaceAcquisitionConfigStepFailuresKeepTheLiveFile(t *testing.T) {
	cases := []struct {
		target string
		step   string
	}{
		{"temp", "write"},
		{"temp", "chmod"},
		{"temp", "sync"},
		{"temp", "close"},
		{"backup", "write"},
		{"backup", "sync"},
		{"backup", "close"},
	}
	for _, tc := range cases {
		t.Run(tc.target+" "+tc.step, func(t *testing.T) {
			failAcquisitionStep(t, tc.target, tc.step)
			dir := t.TempDir()
			acquis := filepath.Join(dir, "acquis.yaml")
			require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o600))

			backup, err := replaceAcquisitionConfig(acquis, "new\n", time.Now())
			require.ErrorIs(t, err, errInjectedAcquisitionStep)
			require.Empty(t, backup)

			got, readErr := os.ReadFile(acquis) //nolint:gosec // G304: t.TempDir-derived path
			require.NoError(t, readErr)
			require.Equal(t, "original\n", string(got), "a failed step never changes the live file")
			requireOnlyEntries(t, dir, "acquis.yaml")
		})
	}
}

func TestCreateTempFileReturnsANilInterfaceOnError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-dir")

	f, err := createAcquisitionTempFile(missing, "acquis.yaml.tmp-*")
	require.Error(t, err)
	// A typed nil *os.File inside the interface would compare unequal to nil and hide the failure.
	require.True(t, f == nil, "the seam must return a nil interface on error")
}

func TestOpenBackupFileReturnsANilInterfaceOnError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-dir", "acquis.yaml.backup.x")

	f, err := openAcquisitionBackupFile(missing, 0o600)
	require.Error(t, err)
	require.True(t, f == nil, "the seam must return a nil interface on error")
}

func TestDefaultAcquisitionFileSeamsCreateUsableFiles(t *testing.T) {
	dir := t.TempDir()

	tmp, err := createAcquisitionTempFile(dir, "acquis.yaml.tmp-*")
	require.NoError(t, err)
	require.Equal(t, dir, filepath.Dir(tmp.Name()))
	require.NoError(t, tmp.Close())

	backup, err := openAcquisitionBackupFile(filepath.Join(dir, "acquis.yaml.backup.x"), 0o600)
	require.NoError(t, err)
	_, err = backup.Write([]byte("data"))
	require.NoError(t, err)
	require.NoError(t, backup.Close())

	_, err = openAcquisitionBackupFile(filepath.Join(dir, "acquis.yaml.backup.x"), 0o600)
	require.ErrorIs(t, err, fs.ErrExist, "the backup open stays exclusive")
}

// stubAcquisitionOwner makes the live file look owned by (liveUID, liveGID) while the process owner is
// (1000, 1000), and records every Chown the temp file receives; chownErr is what each Chown returns.
// Real ownership is never changed, so these tests run the same way as root and as a regular user.
func stubAcquisitionOwner(t *testing.T, liveUID, liveGID int, chownErr error) *[][2]int {
	t.Helper()
	origOwner, origProcess, origCreate := fileOwner, processOwner, createAcquisitionTempFile
	t.Cleanup(func() {
		fileOwner = origOwner
		processOwner = origProcess
		createAcquisitionTempFile = origCreate
	})
	fileOwner = func(fs.FileInfo) (int, int, bool) { return liveUID, liveGID, true }
	processOwner = func() (int, int) { return 1000, 1000 }
	calls := &[][2]int{}
	createAcquisitionTempFile = func(dir, pattern string) (acquisitionFile, error) {
		f, err := origCreate(dir, pattern)
		if err != nil {
			return nil, err
		}
		return chownAcquisitionFile{acquisitionFile: f, calls: calls, err: chownErr}, nil
	}
	return calls
}

// chownAcquisitionFile records Chown calls and returns a configured error instead of changing ownership.
type chownAcquisitionFile struct {
	acquisitionFile
	calls *[][2]int
	err   error
}

func (f chownAcquisitionFile) Chown(uid, gid int) error {
	*f.calls = append(*f.calls, [2]int{uid, gid})
	return f.err
}

// writeFailingInPlace makes the in-place open return a file whose write fails.
func writeFailingInPlace(t *testing.T) {
	t.Helper()
	orig := openInPlaceFile
	t.Cleanup(func() { openInPlaceFile = orig })
	openInPlaceFile = func(name string, mode fs.FileMode) (acquisitionFile, error) {
		f, err := orig(name, mode)
		if err != nil {
			return nil, err
		}
		return failingAcquisitionFile{acquisitionFile: f, step: "write"}, nil
	}
}

// acquisitionFixture writes an acquis.yaml with the content original and returns its path and directory.
func acquisitionFixture(t *testing.T, mode fs.FileMode) (acquis, dir string) {
	t.Helper()
	dir = t.TempDir()
	acquis = filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), mode))
	require.NoError(t, os.Chmod(acquis, mode))
	return acquis, dir
}

func TestReplaceAcquisitionConfigLeavesOwnerAloneWhenItMatchesTheProcess(t *testing.T) {
	calls := stubAcquisitionOwner(t, 1000, 1000, nil)
	acquis, _ := acquisitionFixture(t, 0o600)

	_, err := replaceAcquisitionConfig(acquis, "new\n", time.Now())
	require.NoError(t, err)
	require.Empty(t, *calls, "chown is never needed when the live owner is the process owner")
}

func TestReplaceAcquisitionConfigChangesOwnerByRenameWhenChownSucceeds(t *testing.T) {
	calls := stubAcquisitionOwner(t, 2000, 2000, nil)
	acquis, _ := acquisitionFixture(t, 0o600)
	before, err := os.Stat(acquis)
	require.NoError(t, err)

	_, err = replaceAcquisitionConfig(acquis, "new\n", time.Now())
	require.NoError(t, err)
	require.Equal(t, [][2]int{{2000, 2000}}, *calls, "the temp file takes the owner of the live file")
	after, err := os.Stat(acquis)
	require.NoError(t, err)
	require.False(t, os.SameFile(before, after), "a successful chown keeps the rename path")
	requireContent(t, acquis, "new\n")
}

func TestReplaceAcquisitionConfigWritesInPlaceWhenChownIsNotPermitted(t *testing.T) {
	calls := stubAcquisitionOwner(t, 2000, 2000, syscall.EPERM)
	acquis, dir := acquisitionFixture(t, 0o600)
	before, err := os.Stat(acquis)
	require.NoError(t, err)

	backup, err := replaceAcquisitionConfig(acquis, "new\n", time.Now())
	require.NoError(t, err)
	require.Len(t, *calls, 1)
	after, err := os.Stat(acquis)
	require.NoError(t, err)
	require.True(t, os.SameFile(before, after), "the inode, and with it the owner and any bind mount, is kept")
	require.NotEmpty(t, backup)
	requireContent(t, acquis, "new\n")
	requireOnlyEntries(t, dir, "acquis.yaml", filepath.Base(backup))
}

func TestReplaceAcquisitionConfigRenamesWhenInPlaceOpenIsDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("file permissions do not bind root")
	}
	stubAcquisitionOwner(t, 2000, 2000, syscall.EPERM)
	acquis, dir := acquisitionFixture(t, 0o400)
	before, err := os.Stat(acquis)
	require.NoError(t, err)

	backup, err := replaceAcquisitionConfig(acquis, "new\n", time.Now())
	require.NoError(t, err, "the last resort is today's rename, with a warning")
	after, err := os.Stat(acquis)
	require.NoError(t, err)
	require.False(t, os.SameFile(before, after))
	requireContent(t, acquis, "new\n")
	requireOnlyEntries(t, dir, "acquis.yaml", filepath.Base(backup))
}

// Only a refused chown (EPERM) makes the save write the live file in place, so its owner is kept. Any other chown
// failure saves through the atomic rename, as it did before owner handling existed, and the live file is never
// opened for in-place writing. Every case still succeeds and keeps the backup.
func TestReplaceAcquisitionConfigSavesByChownErrorKind(t *testing.T) {
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
			calls := stubAcquisitionOwner(t, 2000, 2000, tc.chown)
			inPlaceOpens := countInPlaceOpens(t)
			acquis, dir := acquisitionFixture(t, 0o600)
			before, err := os.Stat(acquis)
			require.NoError(t, err)

			backup, err := replaceAcquisitionConfig(acquis, "new\n", time.Now())
			require.NoError(t, err)
			require.Len(t, *calls, 1)
			after, err := os.Stat(acquis)
			require.NoError(t, err)
			require.Equal(t, tc.inPlace, os.SameFile(before, after), "in place keeps the inode (and owner); rename replaces it")
			if tc.inPlace {
				require.Equal(t, 1, *inPlaceOpens, "EPERM writes the live file itself")
			} else {
				require.Zero(t, *inPlaceOpens, "a non-EPERM chown failure must never open the live file for in-place writing")
			}
			require.NotEmpty(t, backup)
			requireContent(t, acquis, "new\n")
			requireOnlyEntries(t, dir, "acquis.yaml", filepath.Base(backup))
		})
	}
}

// countInPlaceOpens counts how many times the live file is opened for an in-place write.
func countInPlaceOpens(t *testing.T) *int {
	t.Helper()
	orig := openInPlaceFile
	t.Cleanup(func() { openInPlaceFile = orig })
	opens := new(int)
	openInPlaceFile = func(name string, mode fs.FileMode) (acquisitionFile, error) {
		*opens++
		return orig(name, mode)
	}
	return opens
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
	tmp := chownAcquisitionFile{acquisitionFile: f, calls: &calls, err: syscall.EIO}

	require.NoError(t, keepOwner(tmp, info), "an owner that cannot be read is not a failure")
	require.Empty(t, calls, "without a known owner there is nothing to chown to")
}

func TestReplaceAcquisitionConfigKeepsBackupWhenInPlaceWriteFails(t *testing.T) {
	stubAcquisitionOwner(t, 2000, 2000, syscall.EPERM)
	writeFailingInPlace(t)
	acquis, dir := acquisitionFixture(t, 0o600)

	backup, err := replaceAcquisitionConfig(acquis, "new\n", time.Now())
	require.ErrorIs(t, err, errInjectedAcquisitionStep)
	require.Empty(t, backup)
	backups, globErr := filepath.Glob(acquis + ".backup.*")
	require.NoError(t, globErr)
	require.Len(t, backups, 1, "a half-written live file leaves the backup as the only intact copy")
	requireOnlyEntries(t, dir, "acquis.yaml", filepath.Base(backups[0]))
}

func TestReplaceAcquisitionConfigNeverChownsANewFile(t *testing.T) {
	calls := stubAcquisitionOwner(t, 2000, 2000, syscall.EPERM)
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")

	backup, err := replaceAcquisitionConfig(acquis, "new\n", time.Now())
	require.NoError(t, err)
	require.Empty(t, backup)
	require.Empty(t, *calls, "a new file is owned by the process already")
	requireContent(t, acquis, "new\n")
}

func requireContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Equal(t, want, string(got))
}

func TestSweepStaleAcquisitionTempsRemovesOnlyOldRegularTemps(t *testing.T) {
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

	sweepStaleAcquisitionTemps(acquis, now)
	requireOnlyEntries(t, dir, "acquis.yaml", filepath.Base(fresh), filepath.Base(link), filepath.Base(subdir), filepath.Base(unrelated))

	// Far enough in the future that the fresh temp is old too: only the IsRegular check now keeps the symlink
	// and the directory, and the old regular temp is already gone.
	sweepStaleAcquisitionTemps(acquis, time.Now().Add(24*time.Hour))
	requireOnlyEntries(t, dir, "acquis.yaml", filepath.Base(link), filepath.Base(subdir), filepath.Base(unrelated))
}

func TestSweepStaleAcquisitionTempsLeavesNearMissNamesAlone(t *testing.T) {
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	old := time.Now().Add(-time.Hour)
	nearMisses := []string{acquis + ".tmpX", filepath.Join(dir, "other.yaml.tmp-1")}
	for _, name := range nearMisses {
		require.NoError(t, os.WriteFile(name, []byte("x"), 0o600))
		require.NoError(t, os.Chtimes(name, old, old))
	}

	sweepStaleAcquisitionTemps(acquis, time.Now().Add(24*time.Hour))
	requireOnlyEntries(t, dir, filepath.Base(nearMisses[0]), filepath.Base(nearMisses[1]))
}

func TestSweepStaleAcquisitionTempsToleratesAMissingDirectory(t *testing.T) {
	require.NotPanics(t, func() {
		sweepStaleAcquisitionTemps(filepath.Join(t.TempDir(), "missing", "acquis.yaml"), time.Now())
	})
}

func TestReplaceAcquisitionConfigRemovesStaleTempsFromEarlierCrashes(t *testing.T) {
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o600))
	stale := acquis + ".tmp-111"
	require.NoError(t, os.WriteFile(stale, []byte("x"), 0o600))
	old := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(stale, old, old))

	backup, err := replaceAcquisitionConfig(acquis, "new\n", time.Now())
	require.NoError(t, err)
	requireOnlyEntries(t, dir, "acquis.yaml", filepath.Base(backup))
}

// stubSyncDir replaces the directory sync seam; the returned slice records each directory synced.
func stubSyncDir(t *testing.T, syncErr error) *[]string {
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

// TestReplaceAcquisitionConfigBestEffortStepsDoNotFailTheSave makes the directory unreadable (mode 0o300: write and
// search only). The stale-temp sweep cannot list it and the directory sync cannot open it; both must only log.
func TestReplaceAcquisitionConfigBestEffortStepsDoNotFailTheSave(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not bind root")
	}
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o600))
	stale := acquis + ".tmp-111"
	require.NoError(t, os.WriteFile(stale, []byte("x"), 0o600))
	old := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(stale, old, old))
	require.NoError(t, os.Chmod(dir, 0o300))       //nolint:gosec // G302: test needs a directory that cannot be read
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: restore owner access so cleanup can remove it

	backup, err := replaceAcquisitionConfig(acquis, "new\n", time.Now())
	require.NoError(t, err, "the save does not depend on listing or syncing the directory")
	require.NotEmpty(t, backup)
	requireContent(t, acquis, "new\n")

	require.NoError(t, os.Chmod(dir, 0o700)) //nolint:gosec // G302: restore owner access to list the directory
	requireOnlyEntries(t, dir, "acquis.yaml", filepath.Base(backup), filepath.Base(stale))
}

func TestReplaceAcquisitionConfigSyncsTheDirectoryOnceAfterRename(t *testing.T) {
	synced := stubSyncDir(t, nil)
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o600))

	_, err := replaceAcquisitionConfig(acquis, "new\n", time.Now())
	require.NoError(t, err)
	require.Equal(t, []string{dir}, *synced)
}

func TestReplaceAcquisitionConfigSkipsDirectorySyncWhenRenameFails(t *testing.T) {
	synced := stubSyncDir(t, nil)
	orig := renameFile
	renameFile = func(string, string) error { return errors.New("boom") }
	t.Cleanup(func() { renameFile = orig })
	acquis, _ := acquisitionFixture(t, 0o600)

	_, err := replaceAcquisitionConfig(acquis, "new\n", time.Now())
	require.Error(t, err)
	require.Empty(t, *synced)
}

func TestReplaceAcquisitionConfigSkipsDirectorySyncOnInPlaceWrite(t *testing.T) {
	synced := stubSyncDir(t, nil)
	orig := renameFile
	renameFile = func(oldpath, newpath string) error {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: syscall.EBUSY}
	}
	t.Cleanup(func() { renameFile = orig })
	acquis, _ := acquisitionFixture(t, 0o600)

	_, err := replaceAcquisitionConfig(acquis, "new\n", time.Now())
	require.NoError(t, err)
	require.Empty(t, *synced, "no rename happened, so there is no new directory entry to sync")
}

func TestReplaceAcquisitionConfigIgnoresADirectorySyncFailure(t *testing.T) {
	synced := stubSyncDir(t, errors.New("sync refused"))
	acquis, _ := acquisitionFixture(t, 0o600)

	_, err := replaceAcquisitionConfig(acquis, "new\n", time.Now())
	require.NoError(t, err, "the new content is already in place, so a sync failure only logs")
	require.Len(t, *synced, 1)
	requireContent(t, acquis, "new\n")
}

func TestSweepStaleAcquisitionTempsSkipsEntriesItCannotInspectOrRemove(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not bind root")
	}
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	stale := acquis + ".tmp-111"
	require.NoError(t, os.WriteFile(stale, []byte("x"), 0o600))
	old := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(stale, old, old))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: restore owner access so cleanup can remove it

	// Readable but not searchable: the names list, but the entry cannot be inspected.
	require.NoError(t, os.Chmod(dir, 0o400)) //nolint:gosec // G302: test needs a directory that cannot be searched
	sweepStaleAcquisitionTemps(acquis, time.Now())
	// Searchable but not writable: the entry is inspected, but cannot be removed.
	require.NoError(t, os.Chmod(dir, 0o500)) //nolint:gosec // G302: test needs a directory that cannot be written
	sweepStaleAcquisitionTemps(acquis, time.Now())

	require.NoError(t, os.Chmod(dir, 0o700)) //nolint:gosec // G302: restore owner access to check the entry
	require.FileExists(t, stale, "entries that cannot be inspected or removed are skipped, never fatal")
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
