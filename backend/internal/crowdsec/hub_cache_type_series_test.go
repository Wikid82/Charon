package crowdsec

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/hubcachetest"
)

// A hub name can exist in several sections (crowdsecurity/crs is both an appsec-rules and an appsec-configs
// item), so the cache keeps one entry per type and name: hub_cache/<type>/<owner>/<name>/.
func TestHubCacheKeepsOneEntryPerTypeAndName(t *testing.T) {
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

func TestHubCacheListSlugReturnsOneValidEntryPerSection(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	ctx := context.Background()
	_, err = cache.Store(ctx, "crowdsecurity/crs", "appsec-rules", "e", "hub", []byte("a: 1\n"))
	require.NoError(t, err)
	_, err = cache.Store(ctx, "crowdsecurity/crs", "appsec-configs", "e", "hub", []byte("a: 2\n"))
	require.NoError(t, err)
	_, err = cache.Store(ctx, "crowdsecurity/other", "collections", "e", "hub", []byte("a: 3\n"))
	require.NoError(t, err)

	found, err := cache.ListSlug(ctx, "crowdsecurity/crs")
	require.NoError(t, err)
	var types []string
	for _, entry := range found {
		types = append(types, entry.Type)
	}
	require.ElementsMatch(t, []string{"appsec-rules", "appsec-configs"}, types)

	none, err := cache.ListSlug(ctx, "crowdsecurity/missing")
	require.NoError(t, err)
	require.Empty(t, none)

	_, err = cache.ListSlug(ctx, "../x")
	require.ErrorIs(t, err, ErrInvalidSlug)

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = cache.ListSlug(canceled, "crowdsecurity/crs")
	require.ErrorIs(t, err, context.Canceled)

	cache.nowFn = func() time.Time { return time.Now().Add(2 * time.Hour) }
	expired, err := cache.ListSlug(ctx, "crowdsecurity/crs")
	require.NoError(t, err)
	require.Empty(t, expired, "expired entries are not listed")
}

func TestHubCacheRemoveLegacyEntriesKeepsTypeDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cache, err := NewHubCache(dir, time.Hour)
	require.NoError(t, err)
	ctx := context.Background()
	_, err = cache.Store(ctx, "crowdsecurity/demo", "collections", "e", "hub", []byte("a: 1\n"))
	require.NoError(t, err)
	writeFile(t, filepath.Join(dir, "legacy", "old", "item.yaml"), "x")
	writeFile(t, filepath.Join(dir, "stray.txt"), "x")
	// A file named like a type is not a type directory.
	writeFile(t, filepath.Join(dir, "parsers"), "x")

	require.NoError(t, cache.RemoveLegacyEntries(ctx))

	require.NoDirExists(t, filepath.Join(dir, "legacy"))
	require.NoFileExists(t, filepath.Join(dir, "stray.txt"))
	require.NoFileExists(t, filepath.Join(dir, "parsers"))
	require.FileExists(t, filepath.Join(dir, "collections", "crowdsecurity", "demo", "item.yaml"))

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorIs(t, cache.RemoveLegacyEntries(canceled), context.Canceled)
}

func TestHubCacheRemoveLegacyEntriesReportsMissingCacheRoot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cache, err := NewHubCache(dir, time.Hour)
	require.NoError(t, err)
	require.NoError(t, os.RemoveAll(dir))

	err = cache.RemoveLegacyEntries(context.Background())

	require.Error(t, err)
	require.NotContains(t, err.Error(), dir, "no server paths in the error")
}

func TestHubServiceOnlyCachedType(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	svc := NewHubService(nil, cache, t.TempDir())
	ctx := context.Background()
	require.Empty(t, svc.onlyCachedType(ctx, "crowdsecurity/crs"), "no entry")

	_, err = cache.Store(ctx, "crowdsecurity/crs", "appsec-rules", "e", "hub", []byte("a: 1\n"))
	require.NoError(t, err)
	require.Equal(t, "appsec-rules", svc.onlyCachedType(ctx, "crowdsecurity/crs"))

	_, err = cache.Store(ctx, "crowdsecurity/crs", "appsec-configs", "e", "hub", []byte("a: 2\n"))
	require.NoError(t, err)
	require.Empty(t, svc.onlyCachedType(ctx, "crowdsecurity/crs"), "several sections need a choice")
	require.Empty(t, svc.onlyCachedType(ctx, "../x"), "an invalid slug has no entry")
}

func TestApplyTakesTheTypeFromTheCacheKey(t *testing.T) {
	t.Parallel()
	e := newApplyEnv(t)
	ctx := context.Background()

	require.Equal(t, "collections", e.svc.cachedTypeOfKey(ctx, applyTestSlug, e.meta.CacheKey))
	require.Empty(t, e.svc.cachedTypeOfKey(ctx, applyTestSlug, applyTestSlug+"-1-"+strings.Repeat("0", 64)))
	require.Empty(t, e.svc.cachedTypeOfKey(ctx, "../x", e.meta.CacheKey))

	_, err := e.svc.Apply(ctx, applyTestSlug, "", applyTestSlug+"-1-"+strings.Repeat("0", 64))
	require.ErrorIs(t, err, ErrPresetCacheStale, "no entry carries the key")
	_, err = e.svc.Apply(ctx, applyTestSlug, "links", e.meta.CacheKey)
	require.ErrorIs(t, err, ErrInvalidItemType)

	res, err := e.svc.Apply(ctx, applyTestSlug, "", e.meta.CacheKey)
	require.NoError(t, err)
	require.Equal(t, "collections", res.Type)
}

func TestResolveItemType(t *testing.T) {
	t.Parallel()
	idx := HubIndex{Items: []HubIndexEntry{
		{Name: "a/one", Type: "collections"},
		{Name: "a/two", Type: "appsec-rules"},
		{Name: "a/two", Type: "appsec-configs"},
		{Name: "a/odd", Type: "bogus"},
	}}
	got, err := resolveItemType(idx, "a/one")
	require.NoError(t, err)
	require.Equal(t, "collections", got)

	_, err = resolveItemType(idx, "a/two")
	var ambiguous *AmbiguousTypeError
	require.ErrorAs(t, err, &ambiguous)
	require.Equal(t, []string{"appsec-configs", "appsec-rules"}, ambiguous.Types)
	require.ErrorIs(t, err, ErrPresetTypeAmbiguous)
	require.Contains(t, err.Error(), "appsec-configs, appsec-rules")

	_, err = resolveItemType(idx, "a/odd")
	require.ErrorContains(t, err, "unsupported hub item type")
	_, err = resolveItemType(idx, "a/none")
	require.ErrorContains(t, err, "not found")
}

func TestPullRejectsAnUnknownItemType(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	_, err = NewHubService(nil, cache, t.TempDir()).Pull(context.Background(), "a/b", "links")
	require.ErrorIs(t, err, ErrInvalidItemType)
}
