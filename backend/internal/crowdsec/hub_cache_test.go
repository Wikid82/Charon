package crowdsec

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/hubcachetest"
)

func storeItem(t *testing.T, cache *HubCache, slug string, item []byte) CachedPreset {
	t.Helper()
	meta, err := cache.Store(context.Background(), slug, "collections", "etag1", "hub", item)
	require.NoError(t, err)
	return meta
}

func TestHubCacheStoreLoadAndExpire(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Minute)
	require.NoError(t, err)
	ctx := context.Background()

	meta := storeItem(t, cache, "crowdsecurity/demo", []byte("name: demo\n"))
	require.Equal(t, "crowdsecurity/demo", meta.Slug)
	require.Equal(t, "collections", meta.Type)
	require.Equal(t, hubcachetest.SHA256Hex([]byte("name: demo\n")), meta.ItemSHA256)
	require.Contains(t, meta.CacheKey, "crowdsecurity/demo-")
	require.True(t, strings.HasSuffix(meta.CacheKey, meta.ItemSHA256))

	loaded, err := cache.Load(ctx, "crowdsecurity/demo")
	require.NoError(t, err)
	require.Equal(t, meta.CacheKey, loaded.CacheKey)
	preview, err := cache.LoadPreview(ctx, "crowdsecurity/demo")
	require.NoError(t, err)
	require.Equal(t, "name: demo\n", preview)

	cache.nowFn = func() time.Time { return time.Now().Add(2 * time.Minute) }
	_, err = cache.Load(ctx, "crowdsecurity/demo")
	require.ErrorIs(t, err, ErrCacheExpired)
	_, err = cache.LoadPreview(ctx, "crowdsecurity/demo")
	require.ErrorIs(t, err, ErrCacheExpired)
}

func TestHubCacheMissingEntryIsAMiss(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	_, err = cache.Load(context.Background(), "crowdsecurity/none")
	require.ErrorIs(t, err, ErrCacheMiss)
	_, err = cache.LoadPreview(context.Background(), "crowdsecurity/none")
	require.ErrorIs(t, err, ErrCacheMiss)
}

func TestHubCacheStoreRejectsBadInput(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	ctx := context.Background()

	for _, slug := range []string{"../bad/x", "..\\bad", "demo", "a/b/c", "", "a/item.yaml"} {
		_, err = cache.Store(ctx, slug, "collections", "e", "hub", []byte("a: 1\n"))
		require.ErrorIs(t, err, ErrInvalidSlug, slug)
	}
	_, err = cache.Store(ctx, "a/b", "links", "e", "hub", []byte("a: 1\n"))
	require.Error(t, err)
	_, err = cache.Store(ctx, "a/b", "collections", "e", "hub", nil)
	require.Error(t, err)
	_, err = cache.Store(ctx, "a/b", "collections", "e", "hub", make([]byte, maxCacheItemBytes+1))
	require.Error(t, err)

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = cache.Store(canceled, "a/b", "collections", "e", "hub", []byte("a: 1\n"))
	require.ErrorIs(t, err, context.Canceled)
	_, err = cache.Load(canceled, "a/b")
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, cache.Evict(canceled, "a/b"), context.Canceled)
	require.ErrorIs(t, cache.EvictIfKey(canceled, "a/b", "k"), context.Canceled)
	_, err = cache.List(canceled)
	require.ErrorIs(t, err, context.Canceled)
}

func TestNewHubCacheRequiresBaseDir(t *testing.T) {
	t.Parallel()
	_, err := NewHubCache("", time.Hour)
	require.Error(t, err)
	file := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	_, err = NewHubCache(filepath.Join(file, "sub"), time.Hour)
	require.Error(t, err)
}

func TestHubCacheListAndEvict(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	ctx := context.Background()
	storeItem(t, cache, "crowdsecurity/demo", []byte("a: 1\n"))
	storeItem(t, cache, "crowdsecurity/other", []byte("a: 2\n"))

	entries, err := cache.List(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	require.NoError(t, cache.Evict(ctx, "crowdsecurity/demo"))
	entries, err = cache.List(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "crowdsecurity/other", entries[0].Slug)

	require.NoError(t, cache.Evict(ctx, "crowdsecurity/none"), "evicting an absent entry is not an error")
	require.NoError(t, cache.Evict(ctx, "crowdsecurity/other"))
	require.NoDirExists(t, filepath.Join(cache.baseDir, "crowdsecurity"), "an empty owner directory is removed")
}

func TestHubCacheListSkipsExpiredAndUnrelatedFiles(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Second)
	require.NoError(t, err)
	storeItem(t, cache, "crowdsecurity/demo", []byte("a: 1\n"))
	require.NoError(t, os.WriteFile(filepath.Join(cache.baseDir, "stray.txt"), []byte("x"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(cache.baseDir, "single"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(cache.baseDir, "owner", "bad name"), 0o700))
	cache.nowFn = func() time.Time { return time.Now().Add(time.Minute) }

	entries, err := cache.List(context.Background())
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestHubCacheListWithRemovedBaseDir(t *testing.T) {
	t.Parallel()
	base := filepath.Join(t.TempDir(), "cache")
	cache, err := NewHubCache(base, time.Hour)
	require.NoError(t, err)
	require.NoError(t, os.RemoveAll(base))

	entries, err := cache.List(context.Background())
	require.NoError(t, err)
	require.Empty(t, entries)
	_, err = cache.Load(context.Background(), "a/b")
	require.ErrorIs(t, err, ErrCacheMiss)
	require.NoError(t, cache.Evict(context.Background(), "a/b"))
	_, err = cache.Store(context.Background(), "a/b", "collections", "e", "hub", []byte("a: 1\n"))
	require.Error(t, err)
}

func TestHubCacheTTL(t *testing.T) {
	t.Parallel()
	for _, ttl := range []time.Duration{2 * time.Hour, time.Minute, 0} {
		cache, err := NewHubCache(t.TempDir(), ttl)
		require.NoError(t, err)
		require.Equal(t, ttl, cache.TTL())
	}
	cache, err := NewHubCache(t.TempDir(), 0)
	require.NoError(t, err)
	storeItem(t, cache, "a/b", []byte("a: 1\n"))
	cache.nowFn = func() time.Time { return time.Now().Add(1000 * time.Hour) }
	_, err = cache.Load(context.Background(), "a/b")
	require.NoError(t, err, "a zero TTL never expires")
}

func TestValidCacheSlug(t *testing.T) {
	t.Parallel()
	valid := []string{"crowdsecurity/base-http-scenarios", "Azlaroc/sftpgo", "a/b", "a.b/c_d", "9x/1y", "crowdsecurity/CVE-2022-26134"}
	invalid := []string{"", "a", "a/", "/a", "a/b/c", "a//b", ".", "..", "a/..", "../a", "-a/b", "a/-b", ".a/b", "a/.b", "a b/c", "a/b c", "a\\b", "a/b+c", "a/metadata.json", "metadata.json/a", "a/item.yaml", "é/x", strings.Repeat("a", 200) + "/" + strings.Repeat("b", 100)}
	for _, slug := range valid {
		require.True(t, ValidCacheSlug(slug), slug)
	}
	for _, slug := range invalid {
		require.False(t, ValidCacheSlug(slug), slug)
	}
}

func TestIsCacheItemType(t *testing.T) {
	t.Parallel()
	for _, typ := range []string{"collections", "parsers", "scenarios", "postoverflows", "contexts", "appsec-configs", "appsec-rules"} {
		require.True(t, IsCacheItemType(typ), typ)
	}
	for _, typ := range []string{"", "links", "collection", "Collections"} {
		require.False(t, IsCacheItemType(typ), typ)
	}
}

func TestHubCacheStoreCleansUpAfterFailure(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	first := storeItem(t, cache, "crowdsecurity/demo", []byte("a: 1\n"))
	dir := filepath.Join(cache.baseDir, "crowdsecurity", "demo")

	// The metadata cannot be replaced while a directory occupies its name; the item rename already happened.
	require.NoError(t, os.Remove(filepath.Join(dir, "metadata.json")))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "metadata.json"), 0o700))
	_, err = cache.Store(context.Background(), "crowdsecurity/demo", "collections", "e", "hub", []byte("a: 2\n"))
	require.Error(t, err)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		require.False(t, strings.HasPrefix(e.Name(), cacheTempPrefix), "temporary file %s was left behind", e.Name())
	}
	require.NotEmpty(t, first.CacheKey)
}

func TestHubCacheStoreFailsWhenItemCannotBeReplaced(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	dir := filepath.Join(cache.baseDir, "crowdsecurity", "demo")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "item.yaml", "child"), 0o700))

	_, err = cache.Store(context.Background(), "crowdsecurity/demo", "collections", "e", "hub", []byte("a: 1\n"))
	require.Error(t, err)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "only the original directory remains")
}

func TestHubCacheRemovesTemporaryFilesFromACrashedStore(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	dir := filepath.Join(cache.baseDir, "crowdsecurity", "demo")

	t.Run("Load of a finished entry", func(t *testing.T) {
		storeItem(t, cache, "crowdsecurity/demo", []byte("a: 1\n"))
		leftover := filepath.Join(dir, cacheTempPrefix+"1234")
		require.NoError(t, os.WriteFile(leftover, []byte("partial"), 0o600))

		_, err := cache.Load(context.Background(), "crowdsecurity/demo")
		require.NoError(t, err)
		require.NoFileExists(t, leftover)
	})
	t.Run("Load of an entry that never finished", func(t *testing.T) {
		require.NoError(t, os.RemoveAll(dir))
		require.NoError(t, os.MkdirAll(dir, 0o700))
		leftover := filepath.Join(dir, cacheTempPrefix+"abcd")
		require.NoError(t, os.WriteFile(leftover, []byte("partial"), 0o600))

		_, err := cache.Load(context.Background(), "crowdsecurity/demo")
		require.ErrorIs(t, err, ErrCacheMiss)
		require.NoFileExists(t, leftover)
	})
	t.Run("Store", func(t *testing.T) {
		leftover := filepath.Join(dir, cacheTempPrefix+"ffff")
		require.NoError(t, os.WriteFile(leftover, []byte("partial"), 0o600))

		storeItem(t, cache, "crowdsecurity/demo", []byte("a: 3\n"))
		require.NoFileExists(t, leftover)
	})
}

func TestHubCacheCrashLeftoverIsInvalid(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	storeItem(t, cache, "crowdsecurity/demo", []byte("a: 1\n"))
	// A crash between the two renames leaves a new item next to the old metadata.
	item := filepath.Join(cache.baseDir, "crowdsecurity", "demo", "item.yaml")
	require.NoError(t, os.WriteFile(item, []byte("a: 2\n"), 0o600))

	_, err = cache.Load(context.Background(), "crowdsecurity/demo")
	require.ErrorIs(t, err, ErrCacheInvalid)
	require.NoDirExists(t, filepath.Join(cache.baseDir, "crowdsecurity", "demo"))
}

func TestHubCacheEvictInvalidKeepsEntryRewrittenMeanwhile(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	// The entry was invalid when it was read, but a writer replaced it before the write lock was taken.
	meta := storeItem(t, cache, "crowdsecurity/demo", []byte("a: 1\n"))

	got, item, err := cache.evictInvalid("crowdsecurity/demo")
	require.NoError(t, err)
	require.Equal(t, meta.CacheKey, got.CacheKey)
	require.Equal(t, "a: 1\n", string(item))
	require.DirExists(t, filepath.Join(cache.baseDir, "crowdsecurity", "demo"))
}

func TestHubCacheEvictIfKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)

	t.Run("matching key removes the entry", func(t *testing.T) {
		meta := storeItem(t, cache, "crowdsecurity/demo", []byte("a: 1\n"))
		require.NoError(t, cache.EvictIfKey(ctx, "crowdsecurity/demo", meta.CacheKey))
		require.NoDirExists(t, filepath.Join(cache.baseDir, "crowdsecurity", "demo"))
	})
	t.Run("a newer entry stays", func(t *testing.T) {
		old := storeItem(t, cache, "crowdsecurity/demo", []byte("a: 1\n"))
		newer := storeItem(t, cache, "crowdsecurity/demo", []byte("a: 2\n"))
		require.NotEqual(t, old.CacheKey, newer.CacheKey, "different content gives a different key within the same second")

		require.NoError(t, cache.EvictIfKey(ctx, "crowdsecurity/demo", old.CacheKey))
		loaded, err := cache.Load(ctx, "crowdsecurity/demo")
		require.NoError(t, err)
		require.Equal(t, newer.CacheKey, loaded.CacheKey)
	})
	t.Run("an absent entry is not an error", func(t *testing.T) {
		require.NoError(t, cache.EvictIfKey(ctx, "crowdsecurity/none", "key"))
	})
	t.Run("an invalid entry is removed", func(t *testing.T) {
		storeItem(t, cache, "crowdsecurity/broken", []byte("a: 1\n"))
		require.NoError(t, os.WriteFile(filepath.Join(cache.baseDir, "crowdsecurity", "broken", "item.yaml"), []byte("x"), 0o600))
		require.NoError(t, cache.EvictIfKey(ctx, "crowdsecurity/broken", "other-key"))
		require.NoDirExists(t, filepath.Join(cache.baseDir, "crowdsecurity", "broken"))
	})
	t.Run("a malformed slug is refused", func(t *testing.T) {
		require.ErrorIs(t, cache.EvictIfKey(ctx, "..", "k"), ErrInvalidSlug)
	})
}

func TestHubCacheEvictRemovesLinksNotTargets(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	storeItem(t, cache, "other/entry", []byte("a: 1\n"))

	t.Run("entry directory link", func(t *testing.T) {
		require.NoError(t, os.MkdirAll(filepath.Join(cache.baseDir, "linked"), 0o700))
		require.NoError(t, os.Symlink(filepath.Join(cache.baseDir, "other", "entry"), filepath.Join(cache.baseDir, "linked", "entry")))
		require.NoError(t, cache.Evict(context.Background(), "linked/entry"))
		require.NoFileExists(t, filepath.Join(cache.baseDir, "linked", "entry"))
		require.DirExists(t, filepath.Join(cache.baseDir, "other", "entry"))
	})
	t.Run("owner directory link", func(t *testing.T) {
		require.NoError(t, os.Symlink(filepath.Join(cache.baseDir, "other"), filepath.Join(cache.baseDir, "alias")))
		require.NoError(t, cache.Evict(context.Background(), "alias/entry"))
		require.DirExists(t, filepath.Join(cache.baseDir, "other", "entry"), "the entry behind a linked owner directory survives")
		_, err := os.Lstat(filepath.Join(cache.baseDir, "alias"))
		require.True(t, os.IsNotExist(err))
	})
}

func TestHubCacheRejectsLinkedOwnerDirectory(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	outside := t.TempDir()
	hubcachetest.Write(t, outside, hubcachetest.Entry{Slug: "x/y", Item: []byte("a: 1\n")})
	require.NoError(t, os.Symlink(filepath.Join(outside, "x"), filepath.Join(cache.baseDir, "x")))

	_, err = cache.Load(context.Background(), "x/y")
	require.ErrorIs(t, err, ErrCacheInvalid)
	require.DirExists(t, filepath.Join(outside, "x", "y"), "the linked directory is left untouched")
}

func TestHubCacheRejectsInvalidMetadataFields(t *testing.T) {
	t.Parallel()
	item := []byte("a: 1\n")
	cases := map[string]hubcachetest.Entry{
		"name differs from slug":   {Slug: "a/b", Item: item, Extra: map[string]any{"name": "a/c"}},
		"digest field malformed":   {Slug: "a/b", Item: item, Extra: map[string]any{"item_sha256": "zz"}},
		"size differs":             {Slug: "a/b", Item: item, Extra: map[string]any{"size_bytes": 3}},
		"etag has control bytes":   {Slug: "a/b", Item: item, Extra: map[string]any{"etag": "a\x01b"}},
		"source is too long":       {Slug: "a/b", Item: item, Extra: map[string]any{"source": strings.Repeat("s", 300)}},
		"retrieval time is zero":   {Slug: "a/b", Item: item, Extra: map[string]any{"retrieved_at": time.Time{}}},
		"cache key digest differs": {Slug: "a/b", Item: item, Extra: map[string]any{"cache_key": "a/b-1-" + strings.Repeat("a", 64)}},
	}
	for name, entry := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cache, err := NewHubCache(t.TempDir(), time.Hour)
			require.NoError(t, err)
			hubcachetest.Write(t, cache.baseDir, entry)

			_, err = cache.Load(context.Background(), "a/b")
			require.ErrorIs(t, err, ErrCacheInvalid)
			require.NoDirExists(t, filepath.Join(cache.baseDir, "a", "b"))
		})
	}
}

func TestHubCacheMissingItemFileIsInvalid(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	storeItem(t, cache, "a/b", []byte("a: 1\n"))
	require.NoError(t, os.Remove(filepath.Join(cache.baseDir, "a", "b", "item.yaml")))

	_, err = cache.Load(context.Background(), "a/b")
	require.ErrorIs(t, err, ErrCacheInvalid)
}

func TestHubCacheStoreNormalizesDisplayText(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	meta, err := cache.Store(context.Background(), "a/b", "collections", "v1\x00\n"+strings.Repeat("é", 300), "hub", []byte("a: 1\n"))
	require.NoError(t, err)
	require.LessOrEqual(t, len(meta.Etag), maxCacheTextBytes)
	require.True(t, printableText(meta.Etag))
	_, err = cache.Load(context.Background(), "a/b")
	require.NoError(t, err)
}

// Store, Load, List and Evict run against one shared HubCache from several goroutines (use -race).
func TestHubCacheConcurrentUseOfOneInstance(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	ctx := context.Background()
	const slug = "crowdsecurity/demo"

	var wg sync.WaitGroup
	errs := make(chan error, 256)
	report := func(err error) {
		select {
		case errs <- err:
		default:
		}
	}
	for w := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 25 {
				item := []byte("n: " + strings.Repeat("x", w*100+i) + "\n")
				if _, err := cache.Store(ctx, slug, "collections", "e", "hub", item); err != nil {
					report(err)
				}
			}
		}()
	}
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				_, item, err := cache.LoadItem(ctx, slug)
				if err != nil && !errors.Is(err, ErrCacheMiss) {
					report(err)
				}
				if err == nil && !strings.HasPrefix(string(item), "n: ") {
					report(errors.New("torn item"))
				}
				if _, err := cache.List(ctx); err != nil {
					report(err)
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 20 {
			if err := cache.Evict(ctx, slug); err != nil {
				report(err)
			}
			if err := cache.EvictIfKey(ctx, slug, "unknown-key"); err != nil {
				report(err)
			}
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}

func TestHubCacheStoreFailsInUnwritableEntryDir(t *testing.T) {
	t.Parallel()
	skipIfRoot(t)
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	storeItem(t, cache, "crowdsecurity/demo", []byte("a: 1\n"))
	dir := filepath.Join(cache.baseDir, "crowdsecurity", "demo")
	require.NoError(t, os.Chmod(dir, 0o500))       //nolint:gosec // G302: test needs a read-only dir
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: restore access so cleanup can remove the dir

	_, err = cache.Store(context.Background(), "crowdsecurity/demo", "collections", "e", "hub", []byte("a: 2\n"))
	require.Error(t, err)
	require.NotContains(t, err.Error(), cache.baseDir, "no server paths in the error")
	loaded, err := cache.Load(context.Background(), "crowdsecurity/demo")
	require.NoError(t, err)
	require.Equal(t, hubcachetest.SHA256Hex([]byte("a: 1\n")), loaded.ItemSHA256, "the previous entry is unchanged")
}

func TestHubCacheEvictReportsUnusableOwnerDirectory(t *testing.T) {
	t.Parallel()
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cache.baseDir, "owner"), []byte("file"), 0o600))

	require.Error(t, cache.Evict(context.Background(), "owner/name"))
	_, err = cache.Load(context.Background(), "owner/name")
	require.Error(t, err)
}

func TestValidCacheKey(t *testing.T) {
	t.Parallel()
	digest := strings.Repeat("a", 64)
	require.True(t, validCacheKey("owner/name-123-"+digest, "owner/name", digest))
	for name, key := range map[string]string{
		"other slug":      "owner/other-123-" + digest,
		"no timestamp":    "owner/name--" + digest,
		"letters in time": "owner/name-12x-" + digest,
		"other digest":    "owner/name-123-" + strings.Repeat("b", 64),
		"no digest":       "owner/name-123",
		"uppercase":       "owner/name-123-" + strings.ToUpper(digest),
	} {
		require.False(t, validCacheKey(key, "owner/name", digest), name)
	}
}
