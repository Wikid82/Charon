package geoip

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	validBytes, err := os.ReadFile(valid) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)

	truncated := filepath.Join(dir, "truncated.mmdb")
	require.NoError(t, os.WriteFile(truncated, validBytes[:len(validBytes)/2], 0o600)) //nolint:gosec // G703: t.TempDir-derived path

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

func TestBuildTime_ClampsEpochsPastTheTimeRange(t *testing.T) {
	assert.Equal(t, time.Unix(1_750_000_000, 0), BuildTime(1_750_000_000))
	last := time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
	assert.True(t, last.Equal(BuildTime(maxBuildEpoch)))
	assert.True(t, last.Equal(BuildTime(uint(math.MaxInt64)+1)))
	huge := BuildTime(math.MaxUint)
	assert.True(t, last.Equal(huge), "an epoch past the int64 range must not wrap into the past")
	assert.True(t, huge.After(time.Now()))
}
