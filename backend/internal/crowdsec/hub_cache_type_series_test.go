package crowdsec

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/hubcachetest"
)

// A hub name can exist in several sections (crowdsecurity/crs is both an appsec-rules and an appsec-configs
// item), so the cache keeps one entry per type and name: hub_cache/<type>/<owner>/<name>/.
func TestHubCacheKeepsOneEntryPerTypeAndName(t *testing.T) {
	skipUntilLaterCommit(t)
	t.Parallel()
	dir := t.TempDir()
	cache, err := NewHubCache(dir, time.Hour)
	require.NoError(t, err)
	ctx := context.Background()

	rules, err := cache.Store(ctx, "crowdsecurity/crs", "appsec-rules", "0.6", "hub", []byte("name: crowdsecurity/crs\nrules: []\n"))
	require.NoError(t, err)
	configs, err := cache.Store(ctx, "crowdsecurity/crs", "appsec-configs", "0.3", "hub", []byte("name: crowdsecurity/crs\ndefault_remediation: ban\n"))
	require.NoError(t, err)

	require.Equal(t, "appsec-rules", rules.Type)
	require.Equal(t, "appsec-configs", configs.Type)
	require.NotEqual(t, rules.CacheKey, configs.CacheKey)
	require.FileExists(t, filepath.Join(dir, "appsec-rules", "crowdsecurity", "crs", "item.yaml"))
	require.FileExists(t, filepath.Join(dir, "appsec-configs", "crowdsecurity", "crs", "item.yaml"))
	require.NoDirExists(t, filepath.Join(dir, "crowdsecurity"), "the entry no longer lives at the owner/name layout")

	listed, err := cache.List(ctx)
	require.NoError(t, err)
	types := map[string]string{}
	for _, entry := range listed {
		require.Equal(t, "crowdsecurity/crs", entry.Slug)
		types[entry.Type] = entry.CacheKey
	}
	require.Equal(t, map[string]string{"appsec-rules": rules.CacheKey, "appsec-configs": configs.CacheKey}, types)
}

// An entry at the old owner/name layout (no type folder) is not served: it is a miss, and a pull fetches it again.
func TestHubCacheDoesNotListEntriesOfTheOldLayout(t *testing.T) {
	skipUntilLaterCommit(t)
	t.Parallel()
	dir := t.TempDir()
	cache, err := NewHubCache(dir, time.Hour)
	require.NoError(t, err)
	item := []byte("name: legacy/old\n")
	digest := hubcachetest.SHA256Hex(item)
	now := time.Now().UTC()
	meta, err := json.Marshal(CachedPreset{
		Slug: "legacy/old", CacheKey: fmt.Sprintf("legacy/old-%d-%s", now.Unix(), digest), Etag: "e", Source: "hub",
		RetrievedAt: now, Type: "collections", Name: "legacy/old", ItemSHA256: digest, SizeBytes: int64(len(item)),
	})
	require.NoError(t, err)
	writeFile(t, filepath.Join(dir, "legacy", "old", "item.yaml"), string(item))
	writeFile(t, filepath.Join(dir, "legacy", "old", "metadata.json"), string(meta))

	listed, err := cache.List(context.Background())

	require.NoError(t, err)
	require.Empty(t, listed)
}
