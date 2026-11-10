package testutil

import (
	"fmt"
	"math"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oschwald/geoip2-golang/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingTB embeds a real test and records the message of a Fatalf call instead of stopping the test.
type failingTB struct {
	testing.TB
	message string
}

func (f *failingTB) Fatalf(format string, args ...any) {
	f.message = fmt.Sprintf(format, args...)
}

func TestWriteGeoIPCountryDBOpensWithBuildEpoch(t *testing.T) {
	cases := []struct {
		name  string
		epoch uint32
	}{
		{name: "zero epoch", epoch: 0},
		{name: "single byte epoch", epoch: 1},
		{name: "epoch fits one byte", epoch: 255},
		{name: "typical epoch", epoch: 1_700_000_000},
		{name: "largest epoch", epoch: math.MaxUint32},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
			WriteGeoIPCountryDB(t, path, tc.epoch)

			db, err := geoip2.Open(path)
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			meta := db.Metadata()
			assert.Equal(t, uint(tc.epoch), meta.BuildEpoch)
			assert.Equal(t, "GeoIP2-Country", meta.DatabaseType)
			assert.Equal(t, uint(1), meta.NodeCount)
			assert.Equal(t, uint(24), meta.RecordSize)
		})
	}
}

func TestWriteGeoIPCountryDBReportsWriteFailure(t *testing.T) {
	missingDir := filepath.Join(t.TempDir(), "absent", "GeoLite2-Country.mmdb")
	fake := &failingTB{TB: t}

	WriteGeoIPCountryDB(fake, missingDir, 1)

	assert.True(t, strings.HasPrefix(fake.message, "write geoip test database: "), "unexpected message: %q", fake.message)
}

func TestAppendMMDBStringEncodes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []byte
	}{
		{name: "empty", in: "", want: []byte{0x40}},
		{name: "short", in: "ab", want: []byte{0x42, 'a', 'b'}},
		{name: "longest accepted", in: strings.Repeat("x", 28), want: append([]byte{0x40 | 28}, strings.Repeat("x", 28)...)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, appendMMDBString(nil, tc.in))
		})
	}
}

func TestAppendMMDBStringRejectsLongString(t *testing.T) {
	assert.Panics(t, func() { appendMMDBString(nil, strings.Repeat("x", 29)) })
}

func TestMMDBUintEncodesFewestBytes(t *testing.T) {
	cases := []struct {
		name     string
		typeCode byte
		value    uint64
		want     []byte
	}{
		{name: "uint16 zero", typeCode: 5, value: 0, want: []byte{0xA0}},
		{name: "uint16 one byte", typeCode: 5, value: 1, want: []byte{0xA1, 0x01}},
		{name: "uint16 two bytes", typeCode: 5, value: 256, want: []byte{0xA2, 0x01, 0x00}},
		{name: "uint32 zero", typeCode: 6, value: 0, want: []byte{0xC0}},
		{name: "uint32 four bytes", typeCode: 6, value: math.MaxUint32, want: []byte{0xC4, 0xFF, 0xFF, 0xFF, 0xFF}},
		{name: "uint32 three bytes", typeCode: 6, value: 0x010000, want: []byte{0xC3, 0x01, 0x00, 0x00}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, mmdbUint(tc.typeCode, tc.value))
		})
	}
}

func TestWriteGeoIPCountryDBDatabaseTypeOption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-ASN.mmdb")
	WriteGeoIPCountryDB(t, path, 1, WithGeoIPDatabaseType("GeoLite2-ASN"))

	db, err := geoip2.Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	assert.Equal(t, "GeoLite2-ASN", db.Metadata().DatabaseType)
}

func TestWriteGeoIPCountryDBRecordOptionResolvesCountry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	WriteGeoIPCountryDB(t, path, 1, WithGeoIPRecordISOCode("DE"))

	db, err := geoip2.Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	record, err := db.Country(netip.MustParseAddr("192.0.2.1"))
	require.NoError(t, err)
	assert.Equal(t, "DE", record.Country.ISOCode)
}

func TestWriteGeoIPCountryDB_RefusesRecordsTheEncoderCannotWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	tooLarge := make(map[string]any, 29)
	for i := range 29 {
		tooLarge[fmt.Sprintf("k%02d", i)] = "v"
	}

	assert.PanicsWithValue(t, "testutil: MMDB map too large for the fixture encoder", func() {
		WriteGeoIPCountryDB(t, path, 1_700_000_000, WithGeoIPRecord(tooLarge))
	})
	assert.PanicsWithValue(t, "testutil: unsupported MMDB value for the fixture encoder", func() {
		WriteGeoIPCountryDB(t, path, 1_700_000_000, WithGeoIPRecord(map[string]any{"count": 1}))
	})
}
