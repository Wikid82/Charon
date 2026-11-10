package geoip

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveDatabasePath(t *testing.T) {
	dataCopy := filepath.Join(DataDatabaseDir, DatabaseFileName)
	bundled := filepath.Join(BundledDatabaseDir, DatabaseFileName)
	custom := "/srv/geo/custom-country.mmdb"

	cases := []struct {
		name    string
		env     string
		present map[string]bool
		want    string
	}{
		{
			name:    "env set uses the path exactly, ignoring files",
			env:     custom,
			present: map[string]bool{dataCopy: true, bundled: true},
			want:    custom,
		},
		{
			name: "env set with no file present still uses the path",
			env:  custom,
			want: custom,
		},
		{
			name:    "data copy is preferred over the bundled copy",
			present: map[string]bool{dataCopy: true, bundled: true},
			want:    dataCopy,
		},
		{
			name:    "only the bundled copy present",
			present: map[string]bool{bundled: true},
			want:    bundled,
		},
		{
			name: "no copy present falls back to the bundled path",
			want: bundled,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveDatabasePath(tc.env, func(p string) bool { return tc.present[p] })
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolveDatabasePath_ReadsEnvironment(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	t.Setenv(EnvDBPath, custom)
	assert.Equal(t, custom, ResolveDatabasePath())

	// An empty variable counts as unset, so the resolver falls through to the file locations.
	t.Setenv(EnvDBPath, "")
	assert.Equal(t, filepath.Base(ResolveDatabasePath()), DatabaseFileName)
}
