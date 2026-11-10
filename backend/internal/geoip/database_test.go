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
