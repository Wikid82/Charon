package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/Wikid82/charon/backend/internal/geoip"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// uploadNow is the store clock for the upload tests; epochs are placed relative to it.
var uploadNow = time.Date(2026, 11, 10, 5, 0, 1, 0, time.UTC)

// newUploadFixture is a store fixture whose clock is uploadNow.
func newUploadFixture(t *testing.T, env bool) *storeFixture {
	t.Helper()
	f := newStoreFixture(t, env)
	f.store.now = fixedClock(uploadNow)
	return f
}

// uploadBytes returns a valid country database with a record that resolves every address to the US, built with buildEpoch
// and any further options.
func uploadBytes(t *testing.T, buildEpoch uint32, opts ...testutil.GeoIPCountryOption) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), geoip.DatabaseFileName)
	all := append([]testutil.GeoIPCountryOption{
		testutil.WithGeoIPDatabaseType("GeoLite2-Country"),
		testutil.WithGeoIPRecordISOCode("US"),
	}, opts...)
	testutil.WriteGeoIPCountryDB(t, path, buildEpoch, all...)
	data, err := os.ReadFile(path) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	return data
}

// placeFile writes data at path, creating the folder when needed.
func placeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, data, 0o600))
}

// leftoverUploads lists the upload temp files in dir.
func leftoverUploads(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, uploadTempPrefix+"*"))
	require.NoError(t, err)
	return matches
}

// failingReader returns err on the first read.
type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestGeoIPDatabaseStoreReplace_InstallsUploadAndLoadsIt(t *testing.T) {
	f := newUploadFixture(t, false)
	data := uploadBytes(t, 1_700_000_000)

	res, err := f.store.Replace(context.Background(), bytes.NewReader(data))

	require.NoError(t, err)
	assert.False(t, res.ReplacedPrevious)
	assert.Equal(t, uint(1_700_000_000), res.Info.BuildEpoch)
	assert.Equal(t, "GeoLite2-Country", res.Info.DatabaseType)
	assert.Equal(t, int64(len(data)), res.SizeBytes)
	sum := sha256.Sum256(data)
	assert.Equal(t, hex.EncodeToString(sum[:]), res.SHA256)

	installed, err := os.ReadFile(f.dataPath)
	require.NoError(t, err)
	assert.Equal(t, data, installed)
	info, err := os.Stat(f.dataPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "the upload is installed with owner-only permissions")

	assert.True(t, f.svc.IsLoaded())
	assert.Equal(t, f.dataPath, f.svc.Info().Path)
	assert.Equal(t, 1, f.applyCalls)
	assert.Empty(t, leftoverUploads(t, f.dataDir))
	assert.NoFileExists(t, f.dataPath+prevSuffix)
}

func TestGeoIPDatabaseStoreReplace_ReplacesExistingCopyAndRemovesPrevious(t *testing.T) {
	f := newUploadFixture(t, false)
	writeCountryDB(t, f.dataPath, 1_700_000_000)
	require.NoError(t, f.store.Reload(context.Background()))
	data := uploadBytes(t, 1_750_000_000)

	res, err := f.store.Replace(context.Background(), bytes.NewReader(data))

	require.NoError(t, err)
	assert.True(t, res.ReplacedPrevious)
	installed, err := os.ReadFile(f.dataPath)
	require.NoError(t, err)
	assert.Equal(t, data, installed)
	assert.NoFileExists(t, f.dataPath+prevSuffix)
	assert.Equal(t, uint(1_750_000_000), f.svc.Info().BuildEpoch)
}

func TestGeoIPDatabaseStoreReplace_CreatesMissingGeoIPFolder(t *testing.T) {
	root := t.TempDir()
	dataPath := filepath.Join(root, "data", "geoip", geoip.DatabaseFileName)
	svc := NewUnloadedGeoIPService(dataPath)
	store := NewGeoIPDatabaseStore(GeoIPDatabaseStoreConfig{
		Service:     svc,
		DataPath:    dataPath,
		BundledPath: filepath.Join(root, "bundled", geoip.DatabaseFileName),
	})
	store.now = fixedClock(uploadNow)
	require.NoDirExists(t, filepath.Dir(dataPath))

	_, err := store.Replace(context.Background(), bytes.NewReader(uploadBytes(t, 1_700_000_000)))

	require.NoError(t, err)
	assert.FileExists(t, dataPath)
}

func TestGeoIPDatabaseStoreReplace_EpochRules(t *testing.T) {
	cases := []struct {
		name    string
		inUse   uint32 // zero means no database in use
		upload  uint32
		wantErr error
	}{
		{name: "equal to the database in use", inUse: 1_750_000_000, upload: 1_750_000_000},
		{name: "23 hours ahead of the clock", upload: uint32(uploadNow.Add(23 * time.Hour).Unix())}, //nolint:gosec // G115: a 2026 Unix time fits in uint32
		{
			name:    "24 hours and a second ahead of the clock",
			upload:  uint32(uploadNow.Add(24*time.Hour + time.Second).Unix()), //nolint:gosec // G115: a 2026 Unix time fits in uint32
			wantErr: ErrGeoIPBuildDateInFuture,
		},
		{name: "older than the database in use", inUse: 1_750_000_000, upload: 1_700_000_000, wantErr: ErrGeoIPOlderThanCurrent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newUploadFixture(t, false)
			if tc.inUse != 0 {
				writeCountryDB(t, f.dataPath, tc.inUse)
				require.NoError(t, f.store.Reload(context.Background()))
			}

			_, err := f.store.Replace(context.Background(), bytes.NewReader(uploadBytes(t, tc.upload)))

			if tc.wantErr == nil {
				require.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestGeoIPDatabaseStoreReplace_OlderThanCurrentReportsBothDates(t *testing.T) {
	f := newUploadFixture(t, false)
	writeCountryDB(t, f.dataPath, 1_750_000_000)
	require.NoError(t, f.store.Reload(context.Background()))
	before, err := os.ReadFile(f.dataPath)
	require.NoError(t, err)

	_, err = f.store.Replace(context.Background(), bytes.NewReader(uploadBytes(t, 1_700_000_000)))

	var older *OlderThanCurrentError
	require.ErrorAs(t, err, &older)
	assert.Equal(t, uint(1_700_000_000), older.Uploaded)
	assert.Equal(t, uint(1_750_000_000), older.Current)
	assert.ErrorIs(t, err, ErrGeoIPOlderThanCurrent)
	assert.Equal(t, "geoip database is older than the current database: uploaded 1700000000, current 1750000000",
		older.Error())
	after, err := os.ReadFile(f.dataPath)
	require.NoError(t, err)
	assert.Equal(t, before, after, "a refused upload leaves the data folder as it was")
}

func TestGeoIPDatabaseStoreReplace_OlderThanBundledCopyIsRefused(t *testing.T) {
	f := newUploadFixture(t, false)
	writeCountryDB(t, f.bundledPath, 1_750_000_000)

	_, err := f.store.Replace(context.Background(), bytes.NewReader(uploadBytes(t, 1_700_000_000)))

	var older *OlderThanCurrentError
	require.ErrorAs(t, err, &older)
	assert.Equal(t, uint(1_750_000_000), older.Current)
}

func TestGeoIPDatabaseStoreReplace_RefusedUploadsLeaveNoTrace(t *testing.T) {
	cases := []struct {
		name      string
		data      []byte
		maxBytes  func(size int64) int64
		wantError error
	}{
		{name: "not a database", data: []byte("plain text"), wantError: geoip.ErrNotMMDB},
		{
			name:      "not a country database",
			data:      uploadBytes(t, 1_700_000_000, testutil.WithGeoIPDatabaseType("GeoLite2-ASN")),
			wantError: geoip.ErrWrongDatabaseType,
		},
		{name: "no build date", data: uploadBytes(t, 0), wantError: geoip.ErrNoBuildDate},
		{
			name:      "over the size limit",
			data:      uploadBytes(t, 1_700_000_000),
			maxBytes:  func(size int64) int64 { return size - 1 },
			wantError: ErrGeoIPTooLarge,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newUploadFixture(t, false)
			if tc.maxBytes != nil {
				f.store.maxBytes = tc.maxBytes(int64(len(tc.data)))
			}

			res, err := f.store.Replace(context.Background(), bytes.NewReader(tc.data))

			assert.ErrorIs(t, err, tc.wantError)
			assert.Equal(t, int64(len(tc.data)), res.SizeBytes)
			assert.NoFileExists(t, f.dataPath)
			assert.Empty(t, leftoverUploads(t, f.dataDir))
			assert.Equal(t, 0, f.applyCalls)
		})
	}
}

func TestGeoIPDatabaseStoreReplace_SizeLimitIsInclusive(t *testing.T) {
	data := uploadBytes(t, 1_700_000_000)

	t.Run("exactly the limit is accepted", func(t *testing.T) {
		f := newUploadFixture(t, false)
		f.store.maxBytes = int64(len(data))

		_, err := f.store.Replace(context.Background(), bytes.NewReader(data))

		require.NoError(t, err)
	})
	t.Run("one byte over the limit is refused", func(t *testing.T) {
		f := newUploadFixture(t, false)
		f.store.maxBytes = int64(len(data)) - 1

		_, err := f.store.Replace(context.Background(), bytes.NewReader(data))

		assert.ErrorIs(t, err, ErrGeoIPTooLarge)
	})
}

func TestGeoIPDatabaseStoreReplace_ClassifiesReadFailures(t *testing.T) {
	t.Run("a failed read is an upload read error", func(t *testing.T) {
		f := newUploadFixture(t, false)

		_, err := f.store.Replace(context.Background(), failingReader{err: errors.New("connection reset")})

		assert.ErrorIs(t, err, ErrGeoIPUploadRead)
		assert.Empty(t, leftoverUploads(t, f.dataDir))
	})
	t.Run("a body over the request limit keeps its type for the handler", func(t *testing.T) {
		f := newUploadFixture(t, false)
		limitErr := fmt.Errorf("read part: %w", &http.MaxBytesError{Limit: 10})

		_, err := f.store.Replace(context.Background(), failingReader{err: limitErr})

		var maxErr *http.MaxBytesError
		assert.ErrorAs(t, err, &maxErr)
		assert.NotErrorIs(t, err, ErrGeoIPUploadRead)
	})
}

func TestGeoIPDatabaseStoreReplace_RefusedWhileCustomPathIsSet(t *testing.T) {
	f := newUploadFixture(t, true)

	_, err := f.store.Replace(context.Background(), bytes.NewReader(uploadBytes(t, 1_700_000_000)))

	assert.ErrorIs(t, err, ErrGeoIPCustomPathSet)
	assert.NoFileExists(t, f.dataPath)
}

func TestGeoIPDatabaseStoreReplace_BusyWhileAnotherChangeRuns(t *testing.T) {
	f := newUploadFixture(t, false)
	f.store.mu.Lock()
	defer f.store.mu.Unlock()

	_, err := f.store.Replace(context.Background(), bytes.NewReader(uploadBytes(t, 1_700_000_000)))

	assert.ErrorIs(t, err, ErrGeoIPBusy)
}

func TestGeoIPDatabaseStoreReplace_LoadFailureRestoresPrevious(t *testing.T) {
	f := newUploadFixture(t, false)
	writeCountryDB(t, f.dataPath, 1_700_000_000)
	require.NoError(t, f.store.Reload(context.Background()))
	before, err := os.ReadFile(f.dataPath)
	require.NoError(t, err)
	f.store.loadFile = func(string) error { return errors.New("load refused") }

	_, err = f.store.Replace(context.Background(), bytes.NewReader(uploadBytes(t, 1_750_000_000)))

	assert.ErrorIs(t, err, ErrGeoIPReloadFailed)
	installed, err := os.ReadFile(f.dataPath)
	require.NoError(t, err)
	assert.Equal(t, before, installed)
	assert.NoFileExists(t, f.dataPath+prevSuffix)
	assert.True(t, f.svc.IsLoaded())
	assert.Equal(t, uint(1_700_000_000), f.svc.Info().BuildEpoch)
	assert.Equal(t, 2, f.applyCalls, "the rollback re-applies the proxy config")
}

func TestGeoIPDatabaseStoreReplace_ApplyFailureRestoresPrevious(t *testing.T) {
	f := newUploadFixture(t, false)
	writeCountryDB(t, f.dataPath, 1_700_000_000)
	require.NoError(t, f.store.Reload(context.Background()))
	before, err := os.ReadFile(f.dataPath)
	require.NoError(t, err)
	calls := 0
	f.store.apply = func(context.Context) error {
		calls++
		if calls == 1 {
			return errors.New("proxy rejected config")
		}
		return nil
	}

	_, err = f.store.Replace(context.Background(), bytes.NewReader(uploadBytes(t, 1_750_000_000)))

	assert.ErrorIs(t, err, ErrGeoIPApplyFailed)
	installed, err := os.ReadFile(f.dataPath)
	require.NoError(t, err)
	assert.Equal(t, before, installed)
	assert.NoFileExists(t, f.dataPath+prevSuffix)
	assert.Equal(t, uint(1_700_000_000), f.svc.Info().BuildEpoch)
	assert.Equal(t, 2, calls)
}

func TestGeoIPDatabaseStoreReplace_ApplyFailureWithoutPreviousUnloads(t *testing.T) {
	f := newUploadFixture(t, false)
	f.store.apply = func(context.Context) error { return errors.New("proxy rejected config") }

	_, err := f.store.Replace(context.Background(), bytes.NewReader(uploadBytes(t, 1_700_000_000)))

	assert.ErrorIs(t, err, ErrGeoIPApplyFailed)
	assert.NoFileExists(t, f.dataPath)
	assert.False(t, f.svc.IsLoaded(), "with nothing to restore, the upload is unloaded again")
}

func TestGeoIPDatabaseStoreReplace_RefusesSymlinkTarget(t *testing.T) {
	f := newUploadFixture(t, false)
	outside := filepath.Join(t.TempDir(), "outside.mmdb")
	require.NoError(t, os.WriteFile(outside, []byte("untouched"), 0o600))
	require.NoError(t, os.Symlink(outside, f.dataPath))

	_, err := f.store.Replace(context.Background(), bytes.NewReader(uploadBytes(t, 1_700_000_000)))

	assert.ErrorIs(t, err, ErrGeoIPWriteFailed)
	content, err := os.ReadFile(outside) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	assert.Equal(t, "untouched", string(content))
}

func TestGeoIPDatabaseStoreReplace_ReadOnlyFolderIsReported(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	t.Run("the GeoIP folder cannot be created", func(t *testing.T) {
		root := t.TempDir()
		dataPath := filepath.Join(root, "data", "geoip", geoip.DatabaseFileName)
		require.NoError(t, os.MkdirAll(filepath.Join(root, "data"), 0o750))
		require.NoError(t, os.Chmod(filepath.Join(root, "data"), 0o500))       //nolint:gosec // G302: directory needs the exec bit; read-only dir fixture
		t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "data"), 0o755) }) //nolint:gosec // G302: the test tree must be removable
		store := NewGeoIPDatabaseStore(GeoIPDatabaseStoreConfig{
			Service: NewUnloadedGeoIPService(dataPath), DataPath: dataPath, BundledPath: filepath.Join(root, "b.mmdb"),
		})
		store.now = fixedClock(uploadNow)

		_, err := store.Replace(context.Background(), bytes.NewReader(uploadBytes(t, 1_700_000_000)))

		assert.ErrorIs(t, err, ErrGeoIPDataFolderReadOnly)
	})
	t.Run("the GeoIP folder exists but refuses new files", func(t *testing.T) {
		f := newUploadFixture(t, false)
		require.NoError(t, os.Chmod(f.dataDir, 0o500))       //nolint:gosec // G302: directory needs the exec bit; read-only dir fixture
		t.Cleanup(func() { _ = os.Chmod(f.dataDir, 0o755) }) //nolint:gosec // G302: the test tree must be removable

		_, err := f.store.Replace(context.Background(), bytes.NewReader(uploadBytes(t, 1_700_000_000)))

		assert.ErrorIs(t, err, ErrGeoIPDataFolderReadOnly)
	})
}

func TestFolderWriteError_ClassifiesReadOnlyFolders(t *testing.T) {
	assert.ErrorIs(t, folderWriteError(unix.EROFS), ErrGeoIPDataFolderReadOnly)
	assert.ErrorIs(t, folderWriteError(os.ErrPermission), ErrGeoIPDataFolderReadOnly)
	assert.ErrorIs(t, folderWriteError(unix.ENOSPC), ErrGeoIPWriteFailed)
}

func TestUploadTempName_IsServerGenerated(t *testing.T) {
	assert.True(t, strings.HasPrefix(uploadTempPrefix, "."), "temp files are hidden")
	assert.NotContains(t, uploadTempPrefix, string(filepath.Separator))
	assert.Equal(t, uploadTempPrefix+"*", uploadTempPattern)
}

func TestGeoIPDatabaseStoreRemove_RestoresBundledCopy(t *testing.T) {
	f := newUploadFixture(t, false)
	writeCountryDB(t, f.bundledPath, 1_700_000_000)
	writeCountryDB(t, f.dataPath, 1_750_000_000)
	require.NoError(t, f.store.Reload(context.Background()))

	require.NoError(t, f.store.RemoveDataCopy(context.Background()))

	assert.NoFileExists(t, f.dataPath)
	assert.NoFileExists(t, f.dataPath+prevSuffix)
	assert.Equal(t, f.bundledPath, f.svc.Info().Path)
	assert.Equal(t, 2, f.applyCalls)
}

func TestGeoIPDatabaseStoreRemove_WithoutBundledCopyUnloads(t *testing.T) {
	f := newUploadFixture(t, false)
	writeCountryDB(t, f.dataPath, 1_750_000_000)
	require.NoError(t, f.store.Reload(context.Background()))

	require.NoError(t, f.store.RemoveDataCopy(context.Background()))

	assert.False(t, f.svc.IsLoaded())
	assert.Equal(t, string(geoip.SourceNone), f.store.Status().Source)
	assert.Equal(t, 2, f.applyCalls)
}

func TestGeoIPDatabaseStoreRemove_RefusedCases(t *testing.T) {
	t.Run("no data-folder copy", func(t *testing.T) {
		f := newUploadFixture(t, false)

		assert.ErrorIs(t, f.store.RemoveDataCopy(context.Background()), ErrGeoIPNoDataCopy)
	})
	t.Run("custom path set", func(t *testing.T) {
		f := newUploadFixture(t, true)
		writeCountryDB(t, f.dataPath, 1_750_000_000)

		assert.ErrorIs(t, f.store.RemoveDataCopy(context.Background()), ErrGeoIPCustomPathSet)
		assert.FileExists(t, f.dataPath)
	})
	t.Run("busy", func(t *testing.T) {
		f := newUploadFixture(t, false)
		writeCountryDB(t, f.dataPath, 1_750_000_000)
		f.store.mu.Lock()
		defer f.store.mu.Unlock()

		assert.ErrorIs(t, f.store.RemoveDataCopy(context.Background()), ErrGeoIPBusy)
	})
	t.Run("symlink target", func(t *testing.T) {
		f := newUploadFixture(t, false)
		outside := filepath.Join(t.TempDir(), "outside.mmdb")
		writeCountryDB(t, outside, 1_750_000_000)
		require.NoError(t, os.Symlink(outside, f.dataPath))

		assert.ErrorIs(t, f.store.RemoveDataCopy(context.Background()), ErrGeoIPWriteFailed)
		assert.FileExists(t, outside)
	})
}

func TestGeoIPDatabaseStoreRemove_FailuresRestoreCopy(t *testing.T) {
	t.Run("unreadable bundled copy", func(t *testing.T) {
		f := newUploadFixture(t, false)
		placeFile(t, f.bundledPath, []byte("not a database"))
		writeCountryDB(t, f.dataPath, 1_750_000_000)
		require.NoError(t, f.store.Reload(context.Background()))
		before, err := os.ReadFile(f.dataPath)
		require.NoError(t, err)

		err = f.store.RemoveDataCopy(context.Background())

		assert.ErrorIs(t, err, ErrGeoIPReloadFailed)
		installed, err := os.ReadFile(f.dataPath)
		require.NoError(t, err)
		assert.Equal(t, before, installed)
		assert.True(t, f.svc.IsLoaded())
		assert.NoFileExists(t, f.dataPath+prevSuffix)
	})
	t.Run("apply failure", func(t *testing.T) {
		f := newUploadFixture(t, false)
		writeCountryDB(t, f.bundledPath, 1_700_000_000)
		writeCountryDB(t, f.dataPath, 1_750_000_000)
		require.NoError(t, f.store.Reload(context.Background()))
		calls := 0
		f.store.apply = func(context.Context) error {
			calls++
			if calls == 1 {
				return errors.New("proxy rejected config")
			}
			return nil
		}

		err := f.store.RemoveDataCopy(context.Background())

		assert.ErrorIs(t, err, ErrGeoIPApplyFailed)
		assert.FileExists(t, f.dataPath)
		assert.Equal(t, uint(1_750_000_000), f.svc.Info().BuildEpoch)
	})
}

func TestSweepGeoIPDataFolder_RemovesUploadTemps(t *testing.T) {
	dir := t.TempDir()
	live := filepath.Join(dir, geoip.DatabaseFileName)
	placeFile(t, live, []byte("live"))
	temp := filepath.Join(dir, uploadTempPrefix+"123")
	placeFile(t, temp, []byte("partial"))

	require.NoError(t, SweepGeoIPDataFolder(live))

	assert.NoFileExists(t, temp)
	assert.FileExists(t, live)
}

func TestSweepGeoIPDataFolder_RestoresPreviousWhenLiveMissing(t *testing.T) {
	dir := t.TempDir()
	live := filepath.Join(dir, geoip.DatabaseFileName)
	placeFile(t, live+prevSuffix, []byte("previous"))

	require.NoError(t, SweepGeoIPDataFolder(live))

	restored, err := os.ReadFile(live) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	assert.Equal(t, "previous", string(restored))
	assert.NoFileExists(t, live+prevSuffix)
}

func TestSweepGeoIPDataFolder_RemovesPreviousWhenLiveExists(t *testing.T) {
	dir := t.TempDir()
	live := filepath.Join(dir, geoip.DatabaseFileName)
	placeFile(t, live, []byte("new"))
	placeFile(t, live+prevSuffix, []byte("previous"))

	require.NoError(t, SweepGeoIPDataFolder(live))

	kept, err := os.ReadFile(live) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	assert.Equal(t, "new", string(kept))
	assert.NoFileExists(t, live+prevSuffix)
}

func TestSweepGeoIPDataFolder_NothingToDo(t *testing.T) {
	t.Run("empty folder", func(t *testing.T) {
		assert.NoError(t, SweepGeoIPDataFolder(filepath.Join(t.TempDir(), geoip.DatabaseFileName)))
	})
	t.Run("folder absent", func(t *testing.T) {
		assert.NoError(t, SweepGeoIPDataFolder(filepath.Join(t.TempDir(), "absent", geoip.DatabaseFileName)))
	})
}

func TestSweepGeoIPDataFolder_UnreadableFolderIsReported(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-a-folder")
	require.NoError(t, os.WriteFile(parent, []byte("file"), 0o600))

	err := SweepGeoIPDataFolder(filepath.Join(parent, geoip.DatabaseFileName))

	assert.Error(t, err)
}

func TestGeoIPDatabaseStoreDataCopyInfo(t *testing.T) {
	t.Run("no data-folder copy", func(t *testing.T) {
		f := newUploadFixture(t, false)

		assert.Equal(t, geoip.DatabaseInfo{}, f.store.DataCopyInfo())
	})
	t.Run("data-folder copy is read", func(t *testing.T) {
		f := newUploadFixture(t, false)
		writeCountryDB(t, f.dataPath, 1_750_000_000)

		info := f.store.DataCopyInfo()

		assert.Equal(t, uint(1_750_000_000), info.BuildEpoch)
		assert.Equal(t, "GeoLite2-Country", info.DatabaseType)
	})
	t.Run("unreadable data-folder copy is zero", func(t *testing.T) {
		f := newUploadFixture(t, false)
		placeFile(t, f.dataPath, []byte("not a database"))

		assert.Equal(t, geoip.DatabaseInfo{}, f.store.DataCopyInfo())
	})
}

func TestClassifyUploadRead(t *testing.T) {
	limited := &http.MaxBytesError{Limit: 1}
	assert.ErrorIs(t, ClassifyUploadRead(limited), limited)
	assert.ErrorIs(t, ClassifyUploadRead(ErrGeoIPUploadRead), ErrGeoIPUploadRead)

	wrapped := ClassifyUploadRead(errors.New("socket closed"))
	assert.ErrorIs(t, wrapped, ErrGeoIPUploadRead)
	assert.NotErrorIs(t, wrapped, ErrGeoIPTooLarge)
}

func TestGeoIPDatabaseStoreReplace_CannotMoveCopyAsideRefusesWithoutTouchingCopy(t *testing.T) {
	f := newUploadFixture(t, false)
	writeCountryDB(t, f.dataPath, 1_700_000_000)
	require.NoError(t, f.store.Reload(context.Background()))
	before, err := os.ReadFile(f.dataPath)
	require.NoError(t, err)
	// A directory where the previous copy would go makes the move aside fail.
	require.NoError(t, os.MkdirAll(filepath.Join(f.dataPath+prevSuffix, "blocker"), 0o750))

	_, err = f.store.Replace(context.Background(), bytes.NewReader(uploadBytes(t, 1_750_000_000)))

	assert.ErrorIs(t, err, ErrGeoIPWriteFailed)
	after, err := os.ReadFile(f.dataPath)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	assert.Empty(t, leftoverUploads(t, f.dataDir))
}

func TestGeoIPDatabaseStoreRemove_ReadOnlyFolderAndUnreachablePath(t *testing.T) {
	t.Run("a path under a regular file cannot be inspected", func(t *testing.T) {
		f := newUploadFixture(t, false)
		parent := filepath.Join(t.TempDir(), "not-a-folder")
		require.NoError(t, os.WriteFile(parent, []byte("file"), 0o600))
		f.store.dataPath = filepath.Join(parent, geoip.DatabaseFileName)

		assert.ErrorIs(t, f.store.RemoveDataCopy(context.Background()), ErrGeoIPWriteFailed)
	})
	t.Run("a read-only folder keeps the copy", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("permission checks do not apply to root")
		}
		f := newUploadFixture(t, false)
		writeCountryDB(t, f.dataPath, 1_750_000_000)
		require.NoError(t, os.Chmod(f.dataDir, 0o500))       //nolint:gosec // G302: directory needs the exec bit; read-only dir fixture
		t.Cleanup(func() { _ = os.Chmod(f.dataDir, 0o755) }) //nolint:gosec // G302: the test tree must be removable

		err := f.store.RemoveDataCopy(context.Background())

		assert.ErrorIs(t, err, ErrGeoIPDataFolderReadOnly)
		assert.FileExists(t, f.dataPath)
		assert.Equal(t, 0, f.applyCalls)
	})
}

func TestSweepGeoIPDataFolder_ReportsFilesThatCannotBeChanged(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	t.Run("an interrupted upload cannot be removed", func(t *testing.T) {
		dir := t.TempDir()
		live := filepath.Join(dir, geoip.DatabaseFileName)
		placeFile(t, filepath.Join(dir, uploadTempPrefix+"1"), []byte("partial"))
		require.NoError(t, os.Chmod(dir, 0o500))       //nolint:gosec // G302: directory needs the exec bit; read-only dir fixture
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) }) //nolint:gosec // G302: the test tree must be removable

		require.Error(t, SweepGeoIPDataFolder(live))
	})
	t.Run("a previous copy cannot be moved back", func(t *testing.T) {
		dir := t.TempDir()
		live := filepath.Join(dir, geoip.DatabaseFileName)
		placeFile(t, live+prevSuffix, []byte("previous"))
		require.NoError(t, os.Chmod(dir, 0o500))       //nolint:gosec // G302: directory needs the exec bit; read-only dir fixture
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) }) //nolint:gosec // G302: the test tree must be removable

		require.Error(t, SweepGeoIPDataFolder(live))
	})
	t.Run("a previous copy beside a live copy cannot be removed", func(t *testing.T) {
		dir := t.TempDir()
		live := filepath.Join(dir, geoip.DatabaseFileName)
		placeFile(t, live, []byte("live"))
		placeFile(t, live+prevSuffix, []byte("previous"))
		require.NoError(t, os.Chmod(dir, 0o500))       //nolint:gosec // G302: directory needs the exec bit; read-only dir fixture
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) }) //nolint:gosec // G302: the test tree must be removable

		require.Error(t, SweepGeoIPDataFolder(live))
	})
}
