package dbmaint

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Querier is the subset of *sql.DB and *sql.Conn the package needs. Passing a
// pinned *sql.Conn keeps every statement on one pool connection.
type Querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Stats is a point-in-time view of the database file.
type Stats struct {
	PageSize      int64
	PageCount     int64
	FreelistCount int64
	AutoVacuum    int
	MainBytes     int64
	WALBytes      int64
}

// ReclaimableBytes is the space held by free pages.
func (s Stats) ReclaimableBytes() int64 { return s.FreelistCount * s.PageSize }

// LiveBytes is the space held by pages in use.
func (s Stats) LiveBytes() int64 { return (s.PageCount - s.FreelistCount) * s.PageSize }

// FreeRatio is the fraction of pages that are free (0 for an empty database).
func (s Stats) FreeRatio() float64 {
	if s.PageCount <= 0 {
		return 0
	}
	return float64(s.FreelistCount) / float64(s.PageCount)
}

// IsIncremental reports whether the file is in incremental auto-vacuum mode.
func (s Stats) IsIncremental() bool { return s.AutoVacuum == AutoVacuumIncremental }

// Inspect reads the page statistics and the main and WAL file sizes. A missing
// WAL file counts as zero bytes.
func Inspect(ctx context.Context, q Querier, dbPath string) (Stats, error) {
	var s Stats
	for _, p := range []struct {
		pragma string
		dst    any
	}{
		{"page_size", &s.PageSize},
		{"page_count", &s.PageCount},
		{"freelist_count", &s.FreelistCount},
		{"auto_vacuum", &s.AutoVacuum},
	} {
		if err := q.QueryRowContext(ctx, "PRAGMA "+p.pragma).Scan(p.dst); err != nil {
			return Stats{}, fmt.Errorf("read %s: %w", p.pragma, err)
		}
	}

	clean := filepath.Clean(dbPath)
	info, err := os.Stat(clean)
	if err != nil {
		return Stats{}, fmt.Errorf("stat database file: %w", err)
	}
	s.MainBytes = info.Size()

	walInfo, err := os.Stat(clean + "-wal")
	switch {
	case err == nil:
		s.WALBytes = walInfo.Size()
	case !errors.Is(err, os.ErrNotExist):
		return Stats{}, fmt.Errorf("stat wal file: %w", err)
	}
	return s, nil
}
