package services

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errInjected is the failure the file-operation fakes return.
var errInjected = errors.New("injected file failure")

// faultyUploadFile is a real temporary file whose Write, Sync or Close fails on request. The real file is still closed,
// so the store's cleanup can remove it.
type faultyUploadFile struct {
	*os.File
	failWrite, failSync, failClose bool
}

func (f *faultyUploadFile) Write(p []byte) (int, error) {
	if f.failWrite {
		return 0, errInjected
	}
	return f.File.Write(p)
}

func (f *faultyUploadFile) Sync() error {
	if f.failSync {
		return errInjected
	}
	return f.File.Sync()
}

func (f *faultyUploadFile) Close() error {
	err := f.File.Close()
	if f.failClose {
		return errInjected
	}
	return err
}

// withFaultyTemp makes the store stream uploads into a faultyUploadFile configured by configure.
func withFaultyTemp(store *GeoIPDatabaseStore, configure func(*faultyUploadFile)) {
	store.files.createTemp = func(dir, pattern string) (uploadFile, error) {
		real, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		f := &faultyUploadFile{File: real}
		configure(f)
		return f, nil
	}
}

func TestGeoIPDatabaseStoreReplace_TempFileFailuresAreWriteFailures(t *testing.T) {
	cases := map[string]func(*faultyUploadFile){
		"write": func(f *faultyUploadFile) { f.failWrite = true },
		"sync":  func(f *faultyUploadFile) { f.failSync = true },
		"close": func(f *faultyUploadFile) { f.failClose = true },
	}
	for name, configure := range cases {
		t.Run(name, func(t *testing.T) {
			f := newUploadFixture(t, false)
			withFaultyTemp(f.store, configure)

			res, err := f.store.Replace(context.Background(), bytes.NewReader(uploadBytes(t, 1_750_000_000)))

			require.ErrorIs(t, err, ErrGeoIPWriteFailed)
			assert.ErrorIs(t, err, errInjected)
			assert.NotErrorIs(t, err, ErrGeoIPUploadRead)
			assert.Empty(t, res.SHA256, "a digest is only reported for an upload written in full")
			assert.NoFileExists(t, f.dataPath)
			assert.Empty(t, leftoverUploads(t, f.dataDir))
			assert.Equal(t, 0, f.applyCalls)
		})
	}
}

func TestGeoIPDatabaseStoreReplace_InstallRenameFailureRestoresPrevious(t *testing.T) {
	f := newUploadFixture(t, false)
	writeCountryDB(t, f.dataPath, 1_700_000_000)
	require.NoError(t, f.store.Reload(context.Background()))
	before, err := os.ReadFile(f.dataPath)
	require.NoError(t, err)
	applied := f.applyCalls
	f.store.files.rename = func(oldpath, newpath string) error {
		if strings.HasPrefix(filepath.Base(oldpath), uploadTempPrefix) {
			return errInjected
		}
		return os.Rename(oldpath, newpath)
	}

	_, err = f.store.Replace(context.Background(), bytes.NewReader(uploadBytes(t, 1_750_000_000)))

	require.ErrorIs(t, err, ErrGeoIPWriteFailed)
	after, err := os.ReadFile(f.dataPath)
	require.NoError(t, err)
	assert.Equal(t, before, after, "the previous copy is moved back")
	assert.NoFileExists(t, f.dataPath+prevSuffix)
	assert.Empty(t, leftoverUploads(t, f.dataDir))
	assert.Equal(t, applied, f.applyCalls, "nothing is applied when the files could not be changed")
}

func TestGeoIPDatabaseStoreReplace_FolderSyncFailureRemovesNewCopy(t *testing.T) {
	f := newUploadFixture(t, false)
	f.store.files.syncDir = func(string) error { return errInjected }

	_, err := f.store.Replace(context.Background(), bytes.NewReader(uploadBytes(t, 1_750_000_000)))

	require.ErrorIs(t, err, ErrGeoIPWriteFailed)
	assert.NoFileExists(t, f.dataPath, "without a previous copy the new file is removed")
	assert.Empty(t, leftoverUploads(t, f.dataDir))
	assert.False(t, f.svc.Info().Loaded)
	assert.Equal(t, 0, f.applyCalls)
}

func TestGeoIPDatabaseStoreRemove_FolderSyncFailureRestoresCopy(t *testing.T) {
	f := newUploadFixture(t, false)
	writeCountryDB(t, f.dataPath, 1_750_000_000)
	f.store.files.syncDir = func(string) error { return errInjected }

	err := f.store.RemoveDataCopy(context.Background())

	require.ErrorIs(t, err, ErrGeoIPWriteFailed)
	assert.FileExists(t, f.dataPath)
	assert.NoFileExists(t, f.dataPath+prevSuffix)
	assert.Equal(t, 0, f.applyCalls)
}

func TestSyncDir_MissingFolderIsAnError(t *testing.T) {
	require.ErrorIs(t, syncDir(filepath.Join(t.TempDir(), "missing")), os.ErrNotExist)
	require.NoError(t, syncDir(t.TempDir()))
}

func TestOSGeoIPFileOps_CreateTempFailureReturnsNoFile(t *testing.T) {
	f, err := osGeoIPFileOps().createTemp(filepath.Join(t.TempDir(), "missing"), uploadTempPattern)

	require.Error(t, err)
	assert.Nil(t, f, "a failed create must not hand back a typed nil file")
}

func TestFormatTimeUTC_ZeroTimeIsUnknown(t *testing.T) {
	assert.Nil(t, formatTimeUTC(time.Time{}))
	got := formatTimeUTC(time.Date(2026, 11, 10, 5, 0, 1, 0, time.FixedZone("x", 3600)))
	require.NotNil(t, got)
	assert.Equal(t, "2026-11-10T04:00:01Z", *got)
}

func TestGeoIPDatabaseStoreCheckBuildEpoch_RefusesEpochsPastTheTimeRange(t *testing.T) {
	f := newUploadFixture(t, false)

	assert.ErrorIs(t, f.store.checkBuildEpoch(math.MaxUint), ErrGeoIPBuildDateInFuture)
	assert.ErrorIs(t, f.store.checkBuildEpoch(uint(math.MaxInt64)+1), ErrGeoIPBuildDateInFuture)
}
