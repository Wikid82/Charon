package geoip

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

// DatabaseInfo is the metadata of a MaxMind database that Charon reports and compares.
type DatabaseInfo struct {
	// DatabaseType names the record structure, for example GeoLite2-Country.
	DatabaseType string
	// BuildEpoch is the MaxMind build time in Unix seconds.
	BuildEpoch uint
	// NodeCount is the number of nodes in the search tree.
	NodeCount uint
	// IPVersion is 4 for an IPv4-only database and 6 for IPv4 and IPv6.
	IPVersion uint
}

// Inspect reads the metadata of the MaxMind database at path. Opening the file reads its metadata section; the reader
// is closed again straight away, so no lookup state is kept.
func Inspect(path string) (DatabaseInfo, error) {
	reader, err := maxminddb.Open(filepath.Clean(path)) //nolint:gosec // G703: database path, read-only open
	if err != nil {
		return DatabaseInfo{}, fmt.Errorf("open geoip database %s: %w", path, err)
	}
	defer func() { _ = reader.Close() }()

	meta := reader.Metadata
	return DatabaseInfo{
		DatabaseType: meta.DatabaseType,
		BuildEpoch:   meta.BuildEpoch,
		NodeCount:    meta.NodeCount,
		IPVersion:    meta.IPVersion,
	}, nil
}

// maxBuildEpoch is the last second of the year 9999 UTC, the latest build time Charon represents. Later epochs would
// overflow the conversion to time.Time and wrap into the past.
const maxBuildEpoch = 253402300799

// BuildTime returns a MaxMind build epoch as a time. The epoch comes from the file, so an epoch past maxBuildEpoch is
// clamped to it: it then compares as far in the future instead of wrapping into the past.
func BuildTime(epoch uint) time.Time {
	if epoch > maxBuildEpoch {
		epoch = maxBuildEpoch
	}
	return time.Unix(int64(epoch), 0) //nolint:gosec // G115: bounded by maxBuildEpoch above
}
