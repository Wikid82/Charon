package geoip

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/testutil"
)

func TestInspectReadsMetadata(t *testing.T) {
	cases := []struct {
		name  string
		epoch uint32
		typ   string
	}{
		{name: "country database", epoch: 1_700_000_000, typ: "GeoLite2-Country"},
		{name: "zero epoch", epoch: 0, typ: "GeoIP2-Country"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), DatabaseFileName)
			testutil.WriteGeoIPCountryDB(t, path, tc.epoch, testutil.WithGeoIPDatabaseType(tc.typ))

			info, err := Inspect(path)
			require.NoError(t, err)
			assert.Equal(t, tc.typ, info.DatabaseType)
			assert.Equal(t, uint(tc.epoch), info.BuildEpoch)
			assert.Equal(t, uint(1), info.NodeCount)
			assert.Equal(t, uint(6), info.IPVersion)
		})
	}
}

func TestInspectRejectsUnreadableFiles(t *testing.T) {
	dir := t.TempDir()

	valid := filepath.Join(dir, "valid.mmdb")
	testutil.WriteGeoIPCountryDB(t, valid, 1, testutil.WithGeoIPDatabaseType("GeoIP2-Country"))
	validBytes, err := os.ReadFile(valid)
	require.NoError(t, err)

	truncated := filepath.Join(dir, "truncated.mmdb")
	require.NoError(t, os.WriteFile(truncated, validBytes[:len(validBytes)/2], 0o600))

	random := filepath.Join(dir, "random.mmdb")
	require.NoError(t, os.WriteFile(random, []byte("not a maxmind database at all, just text"), 0o600))

	cases := []struct {
		name string
		path string
	}{
		{name: "missing file", path: filepath.Join(dir, "absent.mmdb")},
		{name: "truncated file", path: truncated},
		{name: "random bytes", path: random},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Inspect(tc.path)
			assert.Error(t, err)
		})
	}
}
