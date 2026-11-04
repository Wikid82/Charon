package crowdsec

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrArchiveTooLarge reports an uploaded archive whose decompressed content passes the import limits.
var ErrArchiveTooLarge = errors.New("archive is larger than the import limit")

// ErrArchiveTooManyEntries reports an uploaded archive with more tar entries than an import accepts.
var ErrArchiveTooManyEntries = errors.New("archive has more entries than the import limit")

// archiveBounds are the limits one archive read enforces. The import uses the package limits; tests lower them.
type archiveBounds struct {
	entries      int
	uncompressed int64
	ratio        float64
}

// defaultArchiveBounds returns the limits every uploaded archive is read under.
func defaultArchiveBounds() archiveBounds {
	return archiveBounds{entries: MaxImportEntries, uncompressed: MaxImportUncompressedBytes, ratio: MaxImportCompressionRatio}
}

// boundedStream is the decompressed content of one archive. It never hands out bytes past its ceiling, so an
// archive over the limits is refused while it is read rather than after it has been read in full.
type boundedStream struct {
	r          io.Reader
	bounds     archiveBounds
	compressed int64 // size of the archive file
	entries    *int  // entries read so far, shared with the ImportArchive that owns the stream
	read       int64 // decompressed bytes handed out so far
}

// ceiling is the most decompressed bytes the archive may yield now. Both terms allow for tar headers and
// padding: each entry read so far adds MaxImportEntryOverhead, and the archive adds MaxImportOverheadBase. The
// content term is the uncompressed limit; the ratio term is the compression ratio limit applied to the archive
// size. The stream is refused at the smaller of the two, so a highly compressed archive is refused early.
func (s *boundedStream) ceiling() int64 {
	overhead := MaxImportOverheadBase + MaxImportEntryOverhead*int64(*s.entries)
	byContent := s.bounds.uncompressed + overhead
	byRatio := int64(s.bounds.ratio*float64(s.compressed)) + overhead
	return min(byContent, byRatio)
}

func (s *boundedStream) Read(p []byte) (int, error) {
	room := s.ceiling() - s.read
	if room < 0 {
		return 0, ErrArchiveTooLarge
	}
	// One byte past the ceiling is readable, so an overshoot is seen as an error instead of a silent cut.
	if int64(len(p)) > room+1 {
		p = p[:room+1]
	}
	n, err := s.r.Read(p)
	s.read += int64(n)
	if s.read > s.ceiling() {
		return 0, ErrArchiveTooLarge
	}
	return n, err
}

// ImportArchive reads an uploaded tar.gz one entry at a time under the import limits. Every gzip member is
// read, every header counts toward the entry limit (zero-size entries included), and the decompressed content
// is bounded while it is read.
type ImportArchive struct {
	file    *os.File
	gz      *gzip.Reader
	tr      *tar.Reader
	src     *boundedStream
	entries int
	bounds  archiveBounds
}

// OpenImportArchive opens an uploaded tar.gz under the import limits.
func OpenImportArchive(path string) (*ImportArchive, error) {
	return openImportArchive(path, defaultArchiveBounds())
}

func openImportArchive(path string, bounds archiveBounds) (*ImportArchive, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			err = pathErr.Err // the path stays out of the message
		}
		return nil, fmt.Errorf("open archive: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("stat archive: %w", err)
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("create gzip reader: %w", err)
	}
	ar := &ImportArchive{file: f, gz: gz, bounds: bounds}
	ar.src = &boundedStream{r: gz, bounds: bounds, compressed: info.Size(), entries: &ar.entries}
	ar.tr = tar.NewReader(ar.src)
	return ar, nil
}

// Next advances to the next entry and returns its header. io.EOF marks the end of the archive; an archive over
// the entry limit fails with ErrArchiveTooManyEntries on the entry that passes it.
//
// Deliberate deviation: tar.Reader.Next folds pax and global extended headers into the entry that follows
// them, so those records are not counted here. Their bytes still pass through the stream and count against the
// byte ceiling, so an archive cannot use them to exceed the limits.
func (a *ImportArchive) Next() (*tar.Header, error) {
	hdr, err := a.tr.Next()
	if err != nil {
		return nil, err
	}
	a.entries++
	if a.entries > a.bounds.entries {
		return nil, ErrArchiveTooManyEntries
	}
	return hdr, nil
}

// Read reads the body of the current entry, up to its declared size.
func (a *ImportArchive) Read(p []byte) (int, error) {
	return a.tr.Read(p)
}

// Close releases the gzip reader and the archive file.
func (a *ImportArchive) Close() error {
	return errors.Join(a.gz.Close(), a.file.Close())
}
