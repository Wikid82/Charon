package geoip

import (
	"errors"
	"fmt"
	"iter"
	"path/filepath"
	"strings"

	"github.com/oschwald/maxminddb-golang/v2"
)

// Sentinel errors of ValidateCountryDatabase. The wrapped detail is for the server log only; API responses carry a
// fixed error_code per sentinel.
var (
	// ErrNotMMDB means the file cannot be opened as a MaxMind DB, or its binary format is not major version 2.
	ErrNotMMDB = errors.New("not a MaxMind database")
	// ErrCorruptDatabase means the file opens but fails the structural verification.
	ErrCorruptDatabase = errors.New("corrupt MaxMind database")
	// ErrWrongDatabaseType means the file is not a country database, or its records carry no country ISO code.
	ErrWrongDatabaseType = errors.New("not a country database")
	// ErrNoBuildDate means the metadata carries no build time.
	ErrNoBuildDate = errors.New("database has no build date")
)

// maxRecordsChecked bounds how many records the country check decodes.
const maxRecordsChecked = 64

// ValidateCountryDatabase checks that the file at path is a usable country database and returns its metadata. It
// opens the file, verifies its structure, requires binary format major version 2, a database type naming Country
// (case-insensitive), a non-zero build epoch, and at least one record with a two-letter country.iso_code. The records
// check catches a file whose metadata says Country but whose records do not carry the field the proxy's geoip2 handler
// reads.
func ValidateCountryDatabase(path string) (DatabaseInfo, error) {
	reader, err := maxminddb.Open(filepath.Clean(path)) //nolint:gosec // G703: uploaded database in the data folder, read-only open
	if err != nil {
		return DatabaseInfo{}, fmt.Errorf("%w: %w", ErrNotMMDB, err)
	}
	defer func() { _ = reader.Close() }()

	meta := reader.Metadata
	if meta.BinaryFormatMajorVersion != 2 {
		return DatabaseInfo{}, fmt.Errorf("%w: binary format major version %d", ErrNotMMDB, meta.BinaryFormatMajorVersion)
	}
	if err := reader.Verify(); err != nil {
		return DatabaseInfo{}, fmt.Errorf("%w: %w", ErrCorruptDatabase, err)
	}

	switch {
	case !strings.Contains(strings.ToLower(meta.DatabaseType), "country"):
		return DatabaseInfo{}, fmt.Errorf("%w: database type %q", ErrWrongDatabaseType, meta.DatabaseType)
	case meta.BuildEpoch == 0:
		return DatabaseInfo{}, ErrNoBuildDate
	case !hasCountryRecord(reader.Networks()):
		return DatabaseInfo{}, fmt.Errorf("%w: no record carries a country iso_code", ErrWrongDatabaseType)
	}

	return DatabaseInfo{
		DatabaseType: meta.DatabaseType,
		BuildEpoch:   meta.BuildEpoch,
		NodeCount:    meta.NodeCount,
		IPVersion:    meta.IPVersion,
	}, nil
}

// countryOnly is the part of a country record the check reads.
type countryOnly struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

// hasCountryRecord reports whether one of the first maxRecordsChecked results decodes to a record with a two-letter
// country ISO code. Results that do not decode are skipped.
func hasCountryRecord(results iter.Seq[maxminddb.Result]) bool {
	checked := 0
	for result := range results {
		if checked == maxRecordsChecked {
			return false
		}
		checked++

		var record countryOnly
		if result.Decode(&record) == nil && isISOCountryCode(record.Country.ISOCode) {
			return true
		}
	}
	return false
}

// isISOCountryCode reports whether code is two ASCII letters, the shape of an ISO 3166-1 alpha-2 code.
func isISOCountryCode(code string) bool {
	if len(code) != 2 {
		return false
	}
	for _, r := range code {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			return false
		}
	}
	return true
}
