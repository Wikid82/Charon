package handlers

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

// errInjectedAcquisitionStep is returned by the fake file wrappers so tests can tell it from real errors.
var errInjectedAcquisitionStep = errors.New("injected acquisition step failure")

// failingAcquisitionFile wraps a real backup file and makes one step fail. Close always closes the real file so
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

// failBackupStep makes the backup file fail at step, and restores the seam on cleanup. Tests that use it must not
// run in parallel.
func failBackupStep(t *testing.T, step string) {
	t.Helper()
	origOpen := openAcquisitionBackupFile
	t.Cleanup(func() { openAcquisitionBackupFile = origOpen })
	openAcquisitionBackupFile = func(name string, mode fs.FileMode) (acquisitionFile, error) {
		f, err := origOpen(name, mode)
		if err != nil {
			return nil, err
		}
		return failingAcquisitionFile{acquisitionFile: f, step: step}, nil
	}
}

func TestReplaceAcquisitionConfigBackupStepFailuresKeepTheLiveFile(t *testing.T) {
	for _, step := range []string{"write", "sync", "close"} {
		t.Run(step, func(t *testing.T) {
			failBackupStep(t, step)
			dir := t.TempDir()
			acquis := filepath.Join(dir, "acquis.yaml")
			require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o600))

			backup, err := replaceAcquisitionConfig(acquis, "new\n", time.Now())
			require.ErrorIs(t, err, errInjectedAcquisitionStep)
			require.Empty(t, backup)

			got, readErr := os.ReadFile(acquis) //nolint:gosec // G304: t.TempDir-derived path
			require.NoError(t, readErr)
			require.Equal(t, "original\n", string(got), "a failed backup never changes the live file")
			requireOnlyEntries(t, dir, "acquis.yaml")
		})
	}
}

func TestOpenBackupFileReturnsANilInterfaceOnError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-dir", "acquis.yaml.backup.x")

	f, err := openAcquisitionBackupFile(missing, 0o600)
	require.Error(t, err)
	require.True(t, f == nil, "the seam must return a nil interface on error")
}

func TestDefaultBackupFileSeamIsExclusive(t *testing.T) {
	dir := t.TempDir()

	backup, err := openAcquisitionBackupFile(filepath.Join(dir, "acquis.yaml.backup.x"), 0o600)
	require.NoError(t, err)
	require.NoError(t, crowdsec.WriteSyncClose(backup, []byte("data")))

	_, err = openAcquisitionBackupFile(filepath.Join(dir, "acquis.yaml.backup.x"), 0o600)
	require.ErrorIs(t, err, fs.ErrExist, "the backup open stays exclusive")
}

func TestReplaceAcquisitionConfigPassesTheAcquisitionOptions(t *testing.T) {
	var gotOpts crowdsec.AtomicWriteOptions
	var gotMode fs.FileMode
	orig := writeFileAtomic
	t.Cleanup(func() { writeFileAtomic = orig })
	writeFileAtomic = func(_ string, _ []byte, mode fs.FileMode, opts crowdsec.AtomicWriteOptions) (crowdsec.AtomicWriteResult, error) {
		gotOpts, gotMode = opts, mode
		return crowdsec.AtomicWriteResult{}, nil
	}
	acquis, _ := acquisitionFixture(t, 0o640)
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	_, err := replaceAcquisitionConfig(acquis, "new\n", now)
	require.NoError(t, err)
	require.Equal(t, fs.FileMode(0o640), gotMode, "the live file's mode is passed on")
	require.Equal(t, crowdsec.AtomicWriteOptions{
		KeepOwner:           true,
		InPlaceFallback:     true,
		SweepTempsOlderThan: 10 * time.Minute,
		Now:                 now,
	}, gotOpts)
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

// acquisitionFixture writes an acquis.yaml with the content original and returns its path and directory.
func acquisitionFixture(t *testing.T, mode fs.FileMode) (acquis, dir string) {
	t.Helper()
	dir = t.TempDir()
	acquis = filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), mode))
	require.NoError(t, os.Chmod(acquis, mode))
	return acquis, dir
}

// stubSyncDir replaces the directory sync seam used by the file editor's save; the returned slice records each
// directory synced.
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
