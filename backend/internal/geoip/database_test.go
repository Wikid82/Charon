package geoip

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseAvailable(t *testing.T) {
	dir := t.TempDir()
	mmdb := filepath.Join(dir, "GeoLite2-Country.mmdb")
	require.NoError(t, os.WriteFile(mmdb, []byte("db"), 0o600))
	placeholder := filepath.Join(dir, "GeoLite2-Country.mmdb.placeholder")
	require.NoError(t, os.WriteFile(placeholder, nil, 0o600))
	dirNamedLikeDB := filepath.Join(dir, "folder.mmdb")
	require.NoError(t, os.Mkdir(dirNamedLikeDB, 0o700))

	cases := []struct {
		name   string
		path   string
		loaded bool
		want   bool
	}{
		{name: "loaded file with mmdb name", path: mmdb, loaded: true, want: true},
		{name: "file present but service not loaded", path: mmdb, loaded: false, want: false},
		{name: "file missing", path: filepath.Join(dir, "absent.mmdb"), loaded: true, want: false},
		{name: "placeholder file", path: placeholder, loaded: true, want: false},
		{name: "empty path", path: "", loaded: true, want: false},
		{name: "directory with mmdb name", path: dirNamedLikeDB, loaded: true, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, DatabaseAvailable(tc.path, tc.loaded))
		})
	}
}

func TestMissingMarkerPath(t *testing.T) {
	assert.Equal(t, "/app/data/geoip/.missing", MissingMarkerPath("/app/data/geoip/GeoLite2-Country.mmdb"))
	assert.Equal(t, "/app/geoip/.missing", MissingMarkerPath(filepath.Join(BundledDatabaseDir, DatabaseFileName)))
}

func TestReportResolutionNotes(t *testing.T) {
	cases := []struct {
		name       string
		resolution resolution
	}{
		{name: "no notes", resolution: resolution{path: DataDatabasePath()}},
		{name: "former default ignored", resolution: resolution{path: BundledDatabasePath(), ignoredFormerDefault: true}},
		{name: "data copy unreadable", resolution: resolution{path: BundledDatabasePath(), skippedUnreadable: DataDatabasePath()}},
		{name: "both notes", resolution: resolution{path: BundledDatabasePath(), ignoredFormerDefault: true, skippedUnreadable: DataDatabasePath()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotPanics(t, func() { reportResolution(tc.resolution) })
		})
	}
}

func TestEnvPathInEffect(t *testing.T) {
	cases := []struct {
		name    string
		env     string
		present []string
		want    bool
	}{
		{name: "unset", env: "", want: false},
		{name: "custom path", env: testCustom, want: true},
		{name: "former default with no file", env: FormerImageDefaultDBPath, want: false},
		{name: "former default with a file", env: FormerImageDefaultDBPath, present: []string{FormerImageDefaultDBPath}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, envPathInEffect(tc.env, fakeFiles(tc.present...)))
		})
	}
}

func TestSourceOf(t *testing.T) {
	cases := []struct {
		name string
		path string
		env  bool
		want Source
	}{
		{name: "env in effect is custom", path: testCustom, env: true, want: SourceCustom},
		{name: "env in effect wins over the data path", path: DataDatabasePath(), env: true, want: SourceCustom},
		{name: "data-folder copy", path: DataDatabasePath(), want: SourceData},
		{name: "bundled copy", path: BundledDatabasePath(), want: SourceBundled},
		{name: "empty path", path: "", want: SourceNone},
		{name: "unknown location without env", path: "/srv/geo/other.mmdb", want: SourceNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SourceOf(tc.path, tc.env))
		})
	}
}

func TestEnvPathInEffect_ReadsTheEnvironment(t *testing.T) {
	t.Setenv(EnvDBPath, "")
	assert.False(t, EnvPathInEffect())

	t.Setenv(EnvDBPath, testCustom)
	assert.True(t, EnvPathInEffect())
}
