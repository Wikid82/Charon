package crowdsec

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// ManifestEntryName is the archive entry reserved for the hub item manifest. Exports generate it, an
// import reads it through InspectArchive and never writes it to disk.
const ManifestEntryName = "charon-hub-items.json"

// Bounds on what InspectArchive keeps in memory. The archive is untrusted, and the import validator
// stays the authority on overall size; an entry over a bound only makes the inspection incomplete.
const (
	maxInspectEntryBytes int64 = 1 << 20
	maxInspectTotalBytes int64 = 8 << 20
	maxInspectEntries          = 256
)

// ArchiveInfo holds the few entries of an uploaded archive that the pre-import checks read.
type ArchiveInfo struct {
	// Files maps the normalized, slash-separated entry name to its content.
	Files map[string][]byte
	// Incomplete is set when a wanted entry exceeded a bound, so Files is not the whole picture.
	Incomplete bool
	// HubItems is the usable hub item manifest, nil when the archive has none or it was rejected.
	HubItems *ArchiveHubItems
	// HasNestedConfig and HasFlatConfig report a regular config/config.yaml and a top-level config.yaml
	// entry. They are set whatever the entry size, unlike Files, which skips entries over the bounds.
	HasNestedConfig bool
	HasFlatConfig   bool
}

// ArchiveEntryRel normalizes a tar entry name exactly as the extractor places it below the destination
// directory ("./a", "a/../b" and "b" are the same entry). It reports false for a name that would leave
// the destination or names the destination itself.
func ArchiveEntryRel(name string) (string, bool) {
	root := string(os.PathSeparator) + "root"
	target := filepath.Join(root, name) // #nosec G305 -- containment is checked on the next line
	prefix := root + string(os.PathSeparator)
	if !strings.HasPrefix(target, prefix) {
		return "", false
	}
	return strings.TrimPrefix(target, prefix), true
}

// inspectWanted reports whether rel is an entry the notification check needs, in either layout: the
// config file, the profiles file, or a direct child of notifications/ that CrowdSec loads (lowercase
// .yaml or .yml only, no subdirectories).
func inspectWanted(rel string) bool {
	for _, prefix := range []string{"", "config/"} {
		switch rel {
		case prefix + "config.yaml", prefix + "profiles.yaml":
			return true
		}
		if path.Dir(rel) == prefix+"notifications" {
			ext := path.Ext(rel)
			return (ext == ".yaml" || ext == ".yml") && path.Base(rel) != ext
		}
	}
	return false
}

// InspectArchive reads the entries the pre-import checks need from a tar.gz in one pass. Bodies of
// every other entry are skipped unread, and nothing is written to disk.
func InspectArchive(archivePath string) (ArchiveInfo, error) {
	info := ArchiveInfo{Files: map[string][]byte{}}
	f, err := os.Open(filepath.Clean(archivePath))
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			err = pathErr.Err // the path stays out of the message
		}
		return info, fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = f.Close() }()
	gr, err := gzip.NewReader(f)
	if err != nil {
		return info, fmt.Errorf("create gzip reader: %w", err)
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	var total int64
	manifest := manifestSlot{}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			info.HubItems = manifest.parse()
			return info, nil
		}
		if err != nil {
			return info, fmt.Errorf("read tar header: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		rel, ok := ArchiveEntryRel(hdr.Name)
		if ok && IsManifestPath(rel) {
			if readErr := manifest.read(tr, hdr.Size); readErr != nil {
				return info, readErr
			}
			continue
		}
		if !ok {
			continue
		}
		rel = filepath.ToSlash(rel)
		switch rel {
		case "config/config.yaml":
			info.HasNestedConfig = true
		case "config.yaml":
			info.HasFlatConfig = true
		}
		if !inspectWanted(rel) {
			continue
		}
		if hdr.Size > maxInspectEntryBytes || total+hdr.Size > maxInspectTotalBytes || (len(info.Files) >= maxInspectEntries && info.Files[rel] == nil) {
			info.Incomplete = true
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tr, maxInspectEntryBytes+1))
		if err != nil {
			return info, fmt.Errorf("read archive entry: %w", err)
		}
		if int64(len(data)) > maxInspectEntryBytes {
			info.Incomplete = true
			continue
		}
		total += int64(len(data))
		info.Files[rel] = data
	}
}

// manifestSlot collects the single manifest entry of an archive.
type manifestSlot struct {
	entries int
	data    []byte
	tooBig  bool
}

// read takes the body of a regular manifest entry; its size is bounded by maxManifestBytes.
func (m *manifestSlot) read(r io.Reader, size int64) error {
	m.entries++
	if m.entries > 1 || size > maxManifestBytes {
		m.tooBig = m.tooBig || size > maxManifestBytes
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(r, maxManifestBytes+1))
	if err != nil {
		return fmt.Errorf("read archive entry: %w", err)
	}
	m.data = data
	return nil
}

// parse returns the usable manifest, or nil when there is none: a duplicate entry, an oversized body,
// malformed JSON and an unknown version all ignore the whole manifest.
func (m *manifestSlot) parse() *ArchiveHubItems {
	switch {
	case m.entries == 0:
		return nil
	case m.entries > 1:
		logger.Log().Warn("hub item manifest ignored: the archive holds it more than once")
		return nil
	case m.tooBig || int64(len(m.data)) > maxManifestBytes:
		logger.Log().Warn("hub item manifest ignored: it is too large")
		return nil
	}
	items := parseHubManifest(m.data)
	if items == nil {
		logger.Log().Warn("hub item manifest ignored: it is not valid version 1 content")
	}
	return items
}
