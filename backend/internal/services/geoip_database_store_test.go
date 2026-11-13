package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/geoip"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// storeFixture is a store over a temporary data folder and bundled copy. The service starts unloaded at the data copy.
type storeFixture struct {
	store       *GeoIPDatabaseStore
	svc         *GeoIPService
	dataDir     string
	dataPath    string
	bundledPath string
	apply       func(context.Context) error
	applyCalls  int
}

func newStoreFixture(t *testing.T, env bool) *storeFixture {
	t.Helper()
	root := t.TempDir()
	dataDir := filepath.Join(root, "data", "geoip")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	f := &storeFixture{
		dataDir:     dataDir,
		dataPath:    filepath.Join(dataDir, geoip.DatabaseFileName),
		bundledPath: filepath.Join(root, "bundled", geoip.DatabaseFileName),
	}
	f.svc = NewUnloadedGeoIPService(f.dataPath)
	f.svc.now = fixedClock(time.Date(2026, 11, 10, 5, 0, 1, 0, time.UTC))
	f.apply = func(context.Context) error { f.applyCalls++; return nil }
	f.store = NewGeoIPDatabaseStore(GeoIPDatabaseStoreConfig{
		Service:     f.svc,
		Apply:       func(ctx context.Context) error { return f.apply(ctx) },
		DataPath:    f.dataPath,
		BundledPath: f.bundledPath,
		EnvInEffect: func() bool { return env },
	})
	return f
}

func writeCountryDB(t *testing.T, path string, epoch uint32) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	testutil.WriteGeoIPCountryDB(t, path, epoch, testutil.WithGeoIPDatabaseType("GeoLite2-Country"))
}

func TestGeoIPDatabaseStoreStatus_NoDatabaseInUse(t *testing.T) {
	f := newStoreFixture(t, false)

	st := f.store.Status()

	assert.Equal(t, string(geoip.SourceNone), st.Source)
	assert.Empty(t, st.DatabaseType)
	assert.Nil(t, st.BuildDate)
	assert.Nil(t, st.SizeBytes)
	assert.Nil(t, st.LoadedAt)
	assert.Nil(t, st.IncludedBuildDate)
	assert.False(t, st.DataCopyPresent)
	assert.True(t, st.CanReplace, "a writable data folder allows a replacement")
	assert.Empty(t, st.ReplaceBlockedReason)
	assert.Zero(t, st.UnenforcedListsTotal)
}

func TestGeoIPDatabaseStoreStatus_ReportsLoadedDatabase(t *testing.T) {
	f := newStoreFixture(t, false)
	writeCountryDB(t, f.dataPath, 1_700_000_000)
	require.NoError(t, f.store.Reload(context.Background()))
	info, err := os.Stat(f.dataPath)
	require.NoError(t, err)

	st := f.store.Status()

	assert.Equal(t, "GeoLite2-Country", st.DatabaseType)
	require.NotNil(t, st.BuildDate)
	assert.Equal(t, "2023-11-14T22:13:20Z", *st.BuildDate)
	require.NotNil(t, st.SizeBytes)
	assert.Equal(t, info.Size(), *st.SizeBytes)
	require.NotNil(t, st.LoadedAt)
	assert.Equal(t, "2026-11-10T05:00:01Z", *st.LoadedAt)
	assert.True(t, st.DataCopyPresent)
	assert.Equal(t, string(geoip.SourceNone), st.Source, "a loaded file outside the known locations has no named source")
}

func TestGeoIPDatabaseStoreStatus_CustomPathBlocksReplacement(t *testing.T) {
	f := newStoreFixture(t, true)
	writeCountryDB(t, f.dataPath, 1_700_000_000)
	require.NoError(t, f.store.Reload(context.Background()))

	st := f.store.Status()

	assert.Equal(t, string(geoip.SourceCustom), st.Source)
	assert.False(t, st.CanReplace)
	assert.Equal(t, "custom_path_set", st.ReplaceBlockedReason)
}

func TestGeoIPDatabaseStoreStatus_EnvPathFromEnvironment(t *testing.T) {
	f := newStoreFixture(t, false)
	custom := filepath.Join(t.TempDir(), "custom.mmdb")
	writeCountryDB(t, custom, 1)
	t.Setenv(geoip.EnvDBPath, custom)

	store := NewGeoIPDatabaseStore(GeoIPDatabaseStoreConfig{
		Service: f.svc, DataPath: f.dataPath, BundledPath: f.bundledPath,
	})

	assert.False(t, store.Status().CanReplace)
	assert.Equal(t, "custom_path_set", store.Status().ReplaceBlockedReason)
}

func TestGeoIPDatabaseStoreStatus_ReadOnlyDataFolderBlocksReplacement(t *testing.T) {
	f := newStoreFixture(t, false)
	f.store.writable = func(string) bool { return false }

	st := f.store.Status()

	assert.False(t, st.CanReplace)
	assert.Equal(t, "data_folder_read_only", st.ReplaceBlockedReason)
}

func TestWritableFolderProbesExistingFolderOrParent(t *testing.T) {
	existing := t.TempDir()
	assert.True(t, writableFolder(existing))
	assert.True(t, writableFolder(filepath.Join(existing, "not-yet-created")), "the parent of a missing folder is probed")
	assert.False(t, writableFolder(filepath.Join(existing, "missing", "parent")), "a probe that cannot run counts as not writable")
}

func TestGeoIPDatabaseStoreStatus_RemovedFileHasNoSize(t *testing.T) {
	f := newStoreFixture(t, false)
	writeCountryDB(t, f.dataPath, 1_700_000_000)
	require.NoError(t, f.store.Reload(context.Background()))
	require.NoError(t, os.Remove(f.dataPath))

	st := f.store.Status()

	assert.Nil(t, st.SizeBytes)
	require.NotNil(t, st.LoadedAt, "the load time is still reported for the database in use")
}

func TestGeoIPDatabaseStoreStatus_DirectoryAsBundledCopyIsUnknown(t *testing.T) {
	f := newStoreFixture(t, false)
	require.NoError(t, os.MkdirAll(f.bundledPath, 0o750))

	assert.Nil(t, f.store.Status().IncludedBuildDate)
}

func TestGeoIPDatabaseStoreStatus_MissingDataFolderProbesParent(t *testing.T) {
	f := newStoreFixture(t, false)
	require.NoError(t, os.RemoveAll(f.dataDir))

	st := f.store.Status()

	assert.True(t, st.CanReplace, "the GeoIP folder is created on the first write, so its parent decides")
}

func TestGeoIPDatabaseStoreStatus_IncludedCopyIsReadAndCached(t *testing.T) {
	f := newStoreFixture(t, false)
	stamp := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	writeCountryDB(t, f.bundledPath, 1_700_000_000)
	require.NoError(t, os.Chtimes(f.bundledPath, stamp, stamp))

	first := f.store.Status()
	require.NotNil(t, first.IncludedBuildDate)
	assert.Equal(t, "2023-11-14T22:13:20Z", *first.IncludedBuildDate)

	// Same size and mtime after a rewrite: the cached epoch is returned without reopening the file.
	writeCountryDB(t, f.bundledPath, 1_800_000_000)
	require.NoError(t, os.Chtimes(f.bundledPath, stamp, stamp))
	cached := f.store.Status()
	require.NotNil(t, cached.IncludedBuildDate)
	assert.Equal(t, "2023-11-14T22:13:20Z", *cached.IncludedBuildDate)

	// A new modification time invalidates the cache.
	later := stamp.Add(time.Hour)
	require.NoError(t, os.Chtimes(f.bundledPath, later, later))
	refreshed := f.store.Status()
	require.NotNil(t, refreshed.IncludedBuildDate)
	assert.Equal(t, "2027-01-15T08:00:00Z", *refreshed.IncludedBuildDate)
}

func TestGeoIPDatabaseStoreStatus_IncludedCopyAbsentOrUnreadable(t *testing.T) {
	f := newStoreFixture(t, false)
	assert.Nil(t, f.store.Status().IncludedBuildDate, "an absent bundled copy is unknown")

	require.NoError(t, os.MkdirAll(filepath.Dir(f.bundledPath), 0o750))
	require.NoError(t, os.WriteFile(f.bundledPath, []byte("not a database"), 0o600))
	assert.Nil(t, f.store.Status().IncludedBuildDate, "an unreadable bundled copy is unknown")
}

func TestGeoIPDatabaseStoreStatus_ZeroEpochIsUnknownBuildDate(t *testing.T) {
	f := newStoreFixture(t, false)
	writeCountryDB(t, f.dataPath, 0)
	require.NoError(t, f.store.Reload(context.Background()))

	assert.Nil(t, f.store.Status().BuildDate)
}

func TestGeoIPDatabaseStoreReload_AppliesAfterLoad(t *testing.T) {
	f := newStoreFixture(t, false)
	writeCountryDB(t, f.dataPath, 1_700_000_000)

	require.NoError(t, f.store.Reload(context.Background()))

	assert.Equal(t, 1, f.applyCalls)
	assert.True(t, f.svc.IsLoaded())
}

func TestGeoIPDatabaseStoreReload_LoadFailureStillAppliesAndReportsReload(t *testing.T) {
	f := newStoreFixture(t, false)

	err := f.store.Reload(context.Background())

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGeoIPReloadFailed)
	assert.NotErrorIs(t, err, ErrGeoIPApplyFailed)
	assert.Equal(t, 1, f.applyCalls, "the proxy converges on the database state even when the load failed")
}

func TestGeoIPDatabaseStoreReload_ApplyFailureIsReported(t *testing.T) {
	f := newStoreFixture(t, false)
	writeCountryDB(t, f.dataPath, 1_700_000_000)
	f.apply = func(context.Context) error { return errors.New("proxy rejected config") }

	err := f.store.Reload(context.Background())

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGeoIPApplyFailed)
	assert.NotErrorIs(t, err, ErrGeoIPReloadFailed)
}

func TestGeoIPDatabaseStoreReload_BothFailuresAreReported(t *testing.T) {
	f := newStoreFixture(t, false)
	f.apply = func(context.Context) error { return errors.New("proxy rejected config") }

	err := f.store.Reload(context.Background())

	assert.ErrorIs(t, err, ErrGeoIPReloadFailed)
	assert.ErrorIs(t, err, ErrGeoIPApplyFailed)
}

func TestGeoIPDatabaseStoreReload_NilApplyIsSkipped(t *testing.T) {
	f := newStoreFixture(t, false)
	writeCountryDB(t, f.dataPath, 1_700_000_000)
	store := NewGeoIPDatabaseStore(GeoIPDatabaseStoreConfig{
		Service: f.svc, DataPath: f.dataPath, BundledPath: f.bundledPath,
	})

	require.NoError(t, store.Reload(context.Background()))
}

func TestGeoIPDatabaseStoreReload_ConcurrentCallIsBusy(t *testing.T) {
	f := newStoreFixture(t, false)
	writeCountryDB(t, f.dataPath, 1_700_000_000)
	started := make(chan struct{})
	release := make(chan struct{})
	f.apply = func(context.Context) error {
		close(started)
		<-release
		return nil
	}

	done := make(chan error, 1)
	go func() { done <- f.store.Reload(context.Background()) }()
	<-started

	err := f.store.Reload(context.Background())
	assert.ErrorIs(t, err, ErrGeoIPBusy)

	close(release)
	require.NoError(t, <-done)
}
