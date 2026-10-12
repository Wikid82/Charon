// Package hubcachetest writes hub cache entries to disk for tests. It lays files out exactly as the
// cache stores them but shares no code with the cache, so a test can also write entries the cache
// itself would never produce.
package hubcachetest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Entry describes one cache entry. Zero values select a valid entry.
type Entry struct {
	Slug        string
	Type        string         // defaults to "collections"
	Item        []byte         // raw item bytes stored as item.yaml
	RetrievedAt time.Time      // defaults to now
	Extra       map[string]any // merged over the metadata (set a key to nil to write JSON null)
	Omit        []string       // metadata keys left out
}

// SHA256Hex returns the lowercase hex sha256 of data.
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Write stores the entry below baseDir and returns its cache key.
func Write(t testing.TB, baseDir string, e Entry) string {
	t.Helper()
	if e.Type == "" {
		e.Type = "collections"
	}
	if e.RetrievedAt.IsZero() {
		e.RetrievedAt = time.Now().UTC()
	}
	digest := SHA256Hex(e.Item)
	key := fmt.Sprintf("%s-%d-%s", e.Slug, e.RetrievedAt.Unix(), digest)
	meta := map[string]any{
		"slug":         e.Slug,
		"cache_key":    key,
		"etag":         "1.0",
		"source":       "hub",
		"retrieved_at": e.RetrievedAt,
		"type":         e.Type,
		"name":         e.Slug,
		"item_sha256":  digest,
		"size_bytes":   len(e.Item),
	}
	for k, v := range e.Extra {
		meta[k] = v
	}
	for _, k := range e.Omit {
		delete(meta, k)
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal cache metadata: %v", err)
	}
	dir := filepath.Join(baseDir, filepath.FromSlash(e.Slug))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create cache entry dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "item.yaml"), e.Item, 0o600); err != nil {
		t.Fatalf("write cache item: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), raw, 0o600); err != nil {
		t.Fatalf("write cache metadata: %v", err)
	}
	return key
}
