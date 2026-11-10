package testutil

import (
	"bytes"
	"os"
	"sort"
	"testing"
)

// mmdbMetadataMarker opens the metadata section of a MaxMind DB file.
var mmdbMetadataMarker = []byte("\xAB\xCD\xEFMaxMind.com")

// GeoIPCountryOption adjusts the database that WriteGeoIPCountryDB writes.
type GeoIPCountryOption func(*geoIPCountryConfig)

type geoIPCountryConfig struct {
	databaseType string
	// record is the data record every address resolves to. Nil means an empty search tree.
	record map[string]any
}

// WithGeoIPDatabaseType sets the database_type metadata. The default is GeoIP2-Country.
func WithGeoIPDatabaseType(databaseType string) GeoIPCountryOption {
	return func(c *geoIPCountryConfig) { c.databaseType = databaseType }
}

// WithGeoIPRecord sets the data record that every address resolves to. The record is a tree of maps whose leaves are
// strings, for example {"country": {"iso_code": "US"}}. Use it to write records of any shape, including records that
// lack country.iso_code.
func WithGeoIPRecord(record map[string]any) GeoIPCountryOption {
	return func(c *geoIPCountryConfig) { c.record = record }
}

// WithGeoIPRecordISOCode sets the data record to {"country": {"iso_code": code}}, so every address resolves to that
// country. Without a record option the search tree is empty and no address resolves to a country.
func WithGeoIPRecordISOCode(code string) GeoIPCountryOption {
	return WithGeoIPRecord(map[string]any{"country": map[string]any{"iso_code": code}})
}

// WriteGeoIPCountryDB writes a minimal, valid MaxMind DB file to path. Its metadata carries buildEpoch. By default the
// search tree is empty, so it opens and reports metadata but resolves no addresses to a country; see
// WithGeoIPRecordISOCode for a database that resolves every address.
func WriteGeoIPCountryDB(t testing.TB, path string, buildEpoch uint32, opts ...GeoIPCountryOption) {
	t.Helper()

	cfg := geoIPCountryConfig{databaseType: "GeoIP2-Country"}
	for _, opt := range opts {
		opt(&cfg)
	}

	meta := []byte{0xE0 | 9} // map with nine entries
	meta = appendMMDBString(meta, "binary_format_major_version")
	meta = append(meta, mmdbUint(5, 2)...) // uint16
	meta = appendMMDBString(meta, "binary_format_minor_version")
	meta = append(meta, mmdbUint(5, 0)...)
	meta = appendMMDBString(meta, "build_epoch")
	meta = append(meta, mmdbUint(6, uint64(buildEpoch))...) // uint32
	meta = appendMMDBString(meta, "database_type")
	meta = appendMMDBString(meta, cfg.databaseType)
	meta = appendMMDBString(meta, "description")
	meta = appendMMDBValue(meta, map[string]any{"en": "Charon test fixture"})
	meta = appendMMDBString(meta, "ip_version")
	meta = append(meta, mmdbUint(5, 6)...)
	meta = appendMMDBString(meta, "languages")
	meta = append(meta, 0x00, 0x04) // empty array
	meta = appendMMDBString(meta, "node_count")
	meta = append(meta, mmdbUint(6, 1)...)
	meta = appendMMDBString(meta, "record_size")
	meta = append(meta, mmdbUint(5, 24)...)

	// One node with 24-bit records is six bytes; the 16-byte zero separator follows it. With a record option both
	// records point at data offset zero, which is the node count (1) plus the separator length (16).
	var buf bytes.Buffer
	if cfg.record == nil {
		buf.Write(make([]byte, 6))
	} else {
		buf.Write([]byte{0x00, 0x00, 0x11, 0x00, 0x00, 0x11})
	}
	buf.Write(make([]byte, 16))
	if cfg.record != nil {
		buf.Write(appendMMDBValue(nil, cfg.record))
	}
	buf.Write(mmdbMetadataMarker)
	buf.Write(meta)

	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write geoip test database: %v", err)
	}
}

// appendMMDBValue encodes a tree of maps whose leaves are strings as an MMDB value. Keys are written in sorted order so
// the output is deterministic. Any other value type is a mistake in the test and panics.
func appendMMDBValue(dst []byte, v any) []byte {
	switch value := v.(type) {
	case string:
		return appendMMDBString(dst, value)
	case map[string]any:
		if len(value) > 28 {
			panic("testutil: MMDB map too large for the fixture encoder")
		}
		keys := make([]string, 0, len(value))
		for k := range value {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		dst = append(dst, 0xE0|byte(len(value))) //nolint:gosec // G115: length bounded to 28 above
		for _, k := range keys {
			dst = appendMMDBString(dst, k)
			dst = appendMMDBValue(dst, value[k])
		}
		return dst
	default:
		panic("testutil: unsupported MMDB value for the fixture encoder")
	}
}

// appendMMDBString appends s as an MMDB UTF-8 string. The size must fit the control byte, so strings of 28 bytes or
// more are rejected; every string the fixture writes is far shorter.
func appendMMDBString(dst []byte, s string) []byte {
	if len(s) > 28 {
		panic("testutil: MMDB string too long for the fixture encoder")
	}
	dst = append(dst, 0x40|byte(len(s))) //nolint:gosec // G115: length bounded to 28 above
	return append(dst, s...)
}

// mmdbUint encodes v as an MMDB unsigned integer of the given type code (5 for uint16, 6 for uint32), using the
// fewest bytes that hold v. A uint64 never needs more than eight bytes, which fits the size bits.
func mmdbUint(typeCode byte, v uint64) []byte {
	var raw []byte
	for x := v; x > 0; x >>= 8 {
		raw = append([]byte{byte(x)}, raw...)
	}
	out := []byte{typeCode<<5 | byte(len(raw))} //nolint:gosec // G115: at most eight bytes for a uint64
	return append(out, raw...)
}
