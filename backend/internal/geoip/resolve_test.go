package geoip

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testDataCopy = filepath.Join(DataDatabaseDir, DatabaseFileName)
	testBundled  = filepath.Join(BundledDatabaseDir, DatabaseFileName)
	testCustom   = "/srv/geo/custom-country.mmdb"
)

// fakeFiles returns an isFile function reporting only the listed paths as regular files.
func fakeFiles(paths ...string) func(string) bool {
	present := make(map[string]bool, len(paths))
	for _, p := range paths {
		present[p] = true
	}
	return func(p string) bool { return present[p] }
}

// fakeEpochs returns a buildEpoch function answering from epochs; a path absent from the map is unreadable.
func fakeEpochs(epochs map[string]uint) func(string) (DatabaseInfo, error) {
	return func(p string) (DatabaseInfo, error) {
		if e, ok := epochs[p]; ok {
			return DatabaseInfo{BuildEpoch: e}, nil
		}
		return DatabaseInfo{}, errors.New("unreadable")
	}
}

func TestGeoIPResolveChoosesLocation(t *testing.T) {
	cases := []struct {
		name        string
		env         string
		present     []string
		epochs      map[string]uint
		want        string
		wantSkipped string
	}{
		{
			name:    "env set uses the path exactly, ignoring files",
			env:     testCustom,
			present: []string{testDataCopy, testBundled},
			want:    testCustom,
		},
		{
			name: "env set with no file present still uses the path",
			env:  testCustom,
			want: testCustom,
		},
		{
			name:    "only the data copy present",
			present: []string{testDataCopy},
			want:    testDataCopy,
		},
		{
			name:    "only the bundled copy present",
			present: []string{testBundled},
			want:    testBundled,
		},
		{
			name: "no copy present falls back to the bundled path",
			want: testBundled,
		},
		{
			name:    "newer bundled copy wins over older data copy",
			present: []string{testDataCopy, testBundled},
			epochs:  map[string]uint{testDataCopy: 100, testBundled: 200},
			want:    testBundled,
		},
		{
			name:    "newer data copy wins over older bundled copy",
			present: []string{testDataCopy, testBundled},
			epochs:  map[string]uint{testDataCopy: 300, testBundled: 200},
			want:    testDataCopy,
		},
		{
			name:    "equal build times prefer the data copy",
			present: []string{testDataCopy, testBundled},
			epochs:  map[string]uint{testDataCopy: 200, testBundled: 200},
			want:    testDataCopy,
		},
		{
			name:        "unreadable data copy is passed over for a readable bundled copy",
			present:     []string{testDataCopy, testBundled},
			epochs:      map[string]uint{testBundled: 200},
			want:        testBundled,
			wantSkipped: testDataCopy,
		},
		{
			name:    "unreadable bundled copy is passed over for a readable data copy",
			present: []string{testDataCopy, testBundled},
			epochs:  map[string]uint{testDataCopy: 100},
			want:    testDataCopy,
		},
		{
			name:    "neither copy readable keeps the data copy",
			present: []string{testDataCopy, testBundled},
			want:    testDataCopy,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveDatabasePath(tc.env, fakeFiles(tc.present...), fakeEpochs(tc.epochs))
			assert.Equal(t, tc.want, got.path)
			assert.Equal(t, tc.wantSkipped, got.skippedUnreadable)
			assert.False(t, got.ignoredFormerDefault)
		})
	}
}

func TestGeoIPResolveFormerDefaultEnv(t *testing.T) {
	t.Run("former default with no file is treated as unset", func(t *testing.T) {
		got := resolveDatabasePath(FormerImageDefaultDBPath, fakeFiles(testBundled), fakeEpochs(nil))
		assert.Equal(t, testBundled, got.path)
		assert.True(t, got.ignoredFormerDefault)
	})

	t.Run("former default that is a regular file is used exactly", func(t *testing.T) {
		got := resolveDatabasePath(FormerImageDefaultDBPath, fakeFiles(FormerImageDefaultDBPath, testBundled), fakeEpochs(nil))
		assert.Equal(t, FormerImageDefaultDBPath, got.path)
		assert.False(t, got.ignoredFormerDefault)
	})

	t.Run("other explicit value that is missing is still used exactly", func(t *testing.T) {
		got := resolveDatabasePath(testCustom, fakeFiles(testBundled), fakeEpochs(nil))
		assert.Equal(t, testCustom, got.path)
		assert.False(t, got.ignoredFormerDefault)
	})
}

func TestGeoIPResolveReadsEnvironment(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	t.Setenv(EnvDBPath, custom)
	assert.Equal(t, custom, ResolveDatabasePath())

	// An empty variable counts as unset, so the resolver falls through to the file locations.
	t.Setenv(EnvDBPath, "")
	assert.Equal(t, filepath.Base(ResolveDatabasePath()), DatabaseFileName)
}
