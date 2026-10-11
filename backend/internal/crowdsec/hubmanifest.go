package crowdsec

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// manifestVersion is the only manifest format this version reads and writes.
const manifestVersion = 1

// maxManifestBytes bounds the manifest an import reads; a full manifest is about 260 KB.
const maxManifestBytes int64 = 1 << 20

// hubManifest is the content of the ManifestEntryName archive entry.
type hubManifest struct {
	Version int       `json:"version"`
	Items   []HubItem `json:"items"`
}

// ArchiveHubItems is the usable hub item manifest of an uploaded archive.
type ArchiveHubItems struct {
	// Items are the valid, de-duplicated manifest items, sorted by type then name.
	Items []HubItem
	// SkippedInvalid counts manifest items that were dropped because they failed validation.
	SkippedInvalid int
}

// HubItemsReport is the hub_items part of an import response.
type HubItemsReport struct {
	InExport       int       `json:"in_export"`
	SkippedInvalid int       `json:"skipped_invalid"`
	Missing        []HubItem `json:"missing"`
}

// IsManifestPath reports whether rel (relative to the archive or data directory root) names the
// reserved manifest entry. Only the exact root path matches: the same name below a directory is an
// ordinary file.
func IsManifestPath(rel string) bool {
	return filepath.ToSlash(filepath.Clean(rel)) == ManifestEntryName
}

// parseHubManifest reads a manifest strictly. Invalid JSON, an unknown version or more than MaxHubItems
// entries yield nil (the whole manifest is ignored); an individually invalid item is dropped and counted.
func parseHubManifest(data []byte) *ArchiveHubItems {
	var raw struct {
		Version int               `json:"version"`
		Items   []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(data, &raw); err != nil || raw.Version != manifestVersion || len(raw.Items) > MaxHubItems {
		return nil
	}
	out := &ArchiveHubItems{Items: make([]HubItem, 0, len(raw.Items))}
	seen := map[HubItem]struct{}{}
	for _, msg := range raw.Items {
		var item HubItem
		if err := json.Unmarshal(msg, &item); err != nil || !item.Valid() {
			out.SkippedInvalid++
			continue
		}
		if _, dup := seen[item]; dup {
			continue
		}
		seen[item] = struct{}{}
		out.Items = append(out.Items, item)
	}
	slices.SortFunc(out.Items, compareHubItems)
	return out
}

// MissingHubItems returns the manifest items that are not in installed, sorted. It is never nil.
func (a *ArchiveHubItems) MissingHubItems(installed []HubItem) []HubItem {
	have := make(map[HubItem]struct{}, len(installed))
	for _, item := range installed {
		have[item] = struct{}{}
	}
	missing := []HubItem{}
	for _, item := range a.Items {
		if _, ok := have[item]; !ok {
			missing = append(missing, item)
		}
	}
	return missing
}

// Report builds the import response field against the items installed on this host.
func (a *ArchiveHubItems) Report(installed []HubItem) HubItemsReport {
	return HubItemsReport{InExport: len(a.Items), SkippedInvalid: a.SkippedInvalid, Missing: a.MissingHubItems(installed)}
}

// writeHubManifest appends the manifest entry for the items installed below dataDir and returns the
// bytes it added. Nothing is written when no item is installed, so exports of such hosts are unchanged.
// A failed scan is logged and the manifest is omitted: it only helps a later import.
func writeHubManifest(tw *tar.Writer, dataDir string, total, maxBytes int64) (int64, error) {
	items, err := InstalledHubItems(dataDir)
	if err != nil {
		logger.Log().WithError(err).Warn("hub item list omitted from the export")
		return 0, nil
	}
	if len(items) == 0 {
		return 0, nil
	}
	body, err := json.Marshal(hubManifest{Version: manifestVersion, Items: items})
	if err != nil {
		return 0, fmt.Errorf("encode hub item manifest: %w", err)
	}
	if total+int64(len(body)) > maxBytes {
		return 0, ErrExportTooLarge
	}
	hdr := &tar.Header{Name: ManifestEntryName, Size: int64(len(body)), Mode: 0o600, ModTime: time.Now()}
	if err = tw.WriteHeader(hdr); err != nil {
		return 0, fmt.Errorf("write tar header: %w", err)
	}
	if _, err = tw.Write(body); err != nil {
		return 0, fmt.Errorf("write hub item manifest: %w", err)
	}
	return int64(len(body)), nil
}
