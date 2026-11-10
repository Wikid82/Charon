package geoip

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/oschwald/maxminddb-golang/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/testutil"
)

// writeFixture writes a database fixture into a fresh temp folder and returns its path.
func writeFixture(t *testing.T, epoch uint32, opts ...testutil.GeoIPCountryOption) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), DatabaseFileName)
	testutil.WriteGeoIPCountryDB(t, path, epoch, opts...)
	return path
}

func TestValidateCountryDatabase_AcceptsCountryDatabase(t *testing.T) {
	for _, typ := range []string{"GeoLite2-Country", "GeoIP2-Country", "DBIP-Country-Lite"} {
		t.Run(typ, func(t *testing.T) {
			path := writeFixture(t, 1_700_000_000,
				testutil.WithGeoIPDatabaseType(typ), testutil.WithGeoIPRecordISOCode("US"))

			info, err := ValidateCountryDatabase(path)

			require.NoError(t, err)
			assert.Equal(t, typ, info.DatabaseType)
			assert.Equal(t, uint(1_700_000_000), info.BuildEpoch)
		})
	}
}

func TestValidateCountryDatabase_RejectsNonMMDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), DatabaseFileName)
	require.NoError(t, os.WriteFile(path, []byte("this is plain text, not a database"), 0o600))

	_, err := ValidateCountryDatabase(path)

	assert.ErrorIs(t, err, ErrNotMMDB)
}

func TestValidateCountryDatabase_RejectsMissingFile(t *testing.T) {
	_, err := ValidateCountryDatabase(filepath.Join(t.TempDir(), "absent.mmdb"))

	assert.ErrorIs(t, err, ErrNotMMDB)
}

func TestValidateCountryDatabase_RejectsTruncatedFile(t *testing.T) {
	path := writeFixture(t, 1_700_000_000, testutil.WithGeoIPRecordISOCode("US"))
	data, err := os.ReadFile(path) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	// Cutting the file inside the metadata section removes the marker the reader searches for.
	require.NoError(t, os.WriteFile(path, data[:len(data)-8], 0o600)) //nolint:gosec // G703: t.TempDir-derived path

	_, err = ValidateCountryDatabase(path)

	assert.ErrorIs(t, err, ErrNotMMDB)
}

func TestValidateCountryDatabase_RejectsUnsupportedFormatVersion(t *testing.T) {
	path := writeFixture(t, 1_700_000_000, testutil.WithGeoIPRecordISOCode("US"))
	data, err := os.ReadFile(path) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	// The metadata value after the key is the uint16 major version 2; change it to 3.
	key := []byte("binary_format_major_version")
	at := bytes.Index(data, key) + len(key)
	require.Equal(t, byte(0xA1), data[at], "fixture layout changed")
	data[at+1] = 3
	require.NoError(t, os.WriteFile(path, data, 0o600)) //nolint:gosec // G703: t.TempDir-derived path

	_, err = ValidateCountryDatabase(path)

	assert.ErrorIs(t, err, ErrNotMMDB)
}

func TestValidateCountryDatabase_RejectsCorruptTree(t *testing.T) {
	// The fixture without a record has a search tree whose only node points back at itself, which the structural
	// verification rejects while the metadata still opens.
	path := writeFixture(t, 1_700_000_000)

	_, err := ValidateCountryDatabase(path)

	assert.ErrorIs(t, err, ErrCorruptDatabase)
}

func TestValidateCountryDatabase_RejectsNonCountryType(t *testing.T) {
	for _, typ := range []string{"GeoLite2-City", "GeoLite2-ASN"} {
		t.Run(typ, func(t *testing.T) {
			path := writeFixture(t, 1_700_000_000,
				testutil.WithGeoIPDatabaseType(typ), testutil.WithGeoIPRecordISOCode("US"))

			_, err := ValidateCountryDatabase(path)

			assert.ErrorIs(t, err, ErrWrongDatabaseType)
		})
	}
}

func TestValidateCountryDatabase_RejectsZeroBuildEpoch(t *testing.T) {
	path := writeFixture(t, 0, testutil.WithGeoIPRecordISOCode("US"))

	_, err := ValidateCountryDatabase(path)

	assert.ErrorIs(t, err, ErrNoBuildDate)
}

func TestValidateCountryDatabase_RejectsRecordsWithoutISOCode(t *testing.T) {
	cases := []struct {
		name   string
		record map[string]any
	}{
		{name: "no country field", record: map[string]any{"continent": map[string]any{"code": "EU"}}},
		{name: "country without iso_code", record: map[string]any{"country": map[string]any{"names": "x"}}},
		{name: "iso_code of three letters", record: map[string]any{"country": map[string]any{"iso_code": "USA"}}},
		{name: "iso_code with a digit", record: map[string]any{"country": map[string]any{"iso_code": "U1"}}},
		{name: "empty iso_code", record: map[string]any{"country": map[string]any{"iso_code": ""}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFixture(t, 1_700_000_000, testutil.WithGeoIPRecord(tc.record))

			_, err := ValidateCountryDatabase(path)

			assert.ErrorIs(t, err, ErrWrongDatabaseType)
		})
	}
}

func TestValidateCountryDatabase_RejectsRecordsOfUnreadableShape(t *testing.T) {
	// A record whose country field is a string rather than a map decodes to nothing, so no country code is found.
	path := writeFixture(t, 1_700_000_000, testutil.WithGeoIPRecord(map[string]any{"country": "US"}))

	_, err := ValidateCountryDatabase(path)

	assert.ErrorIs(t, err, ErrWrongDatabaseType)
}

func TestHasCountryRecord_StopsAfterBoundedRecords(t *testing.T) {
	// Results that cannot be decoded are skipped. The scan stops after maxRecordsChecked results, so a tree of records
	// without a country code costs bounded work and reports no country record.
	yielded := 0
	results := func(yield func(maxminddb.Result) bool) {
		for range maxRecordsChecked + 10 {
			yielded++
			if !yield(maxminddb.Result{}) {
				return
			}
		}
	}

	assert.False(t, hasCountryRecord(results))
	assert.LessOrEqual(t, yielded, maxRecordsChecked+1, "the scan must stop at the bound")
}

func TestHasCountryRecord_EmptyTreeHasNoCountryRecord(t *testing.T) {
	none := func(func(maxminddb.Result) bool) {}

	assert.False(t, hasCountryRecord(none))
}
