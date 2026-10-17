package crowdsec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/hubcachetest"
)

const (
	cacheTestSlug  = "crowdsecurity/base-http-scenarios"
	otherCacheSlug = "other/entry"
)

var cacheTestItem = []byte("name: crowdsecurity/base-http-scenarios\nscenarios:\n  - crowdsecurity/http-probing\n")

func newValidationCache(t *testing.T) (cache *HubCache, base string) {
	t.Helper()
	base = filepath.Join(t.TempDir(), "cache")
	cache, err := NewHubCache(base, time.Hour)
	require.NoError(t, err)
	return cache, base
}

func testEntryDir(base, slug string) string {
	return filepath.Join(base, "collections", filepath.FromSlash(slug))
}

// writeOtherFile creates a file outside the cache directory whose content is not part of any entry.
func writeOtherFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "other.txt")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func TestLoadAcceptsValidCacheEntry(t *testing.T) {
	cache, base := newValidationCache(t)
	key := hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem})

	meta, err := cache.Load(context.Background(), "collections", cacheTestSlug)
	require.NoError(t, err)
	require.Equal(t, key, meta.CacheKey)
	preview, err := cache.LoadPreview(context.Background(), "collections", cacheTestSlug)
	require.NoError(t, err)
	require.Equal(t, string(cacheTestItem), preview)
}

func TestLoadRejectsInvalidCacheEntry(t *testing.T) {
	big := strings.Repeat("a", 70<<10)
	cases := map[string]func(t *testing.T, base string){
		"metadata names another slug": func(t *testing.T, base string) {
			hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem, Extra: map[string]any{"slug": otherCacheSlug}})
		},
		"entry directory links to another entry": func(t *testing.T, base string) {
			hubcachetest.Write(t, base, hubcachetest.Entry{Slug: otherCacheSlug, Item: cacheTestItem})
			require.NoError(t, os.MkdirAll(filepath.Dir(testEntryDir(base, cacheTestSlug)), 0o700))
			require.NoError(t, os.Symlink(testEntryDir(base, otherCacheSlug), testEntryDir(base, cacheTestSlug)))
		},
		"cache key with a non-hex suffix": func(t *testing.T, base string) {
			hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem, Extra: map[string]any{"cache_key": cacheTestSlug + "-1-" + strings.Repeat("z", 64)}})
		},
		"retrieval time far in the future": func(t *testing.T, base string) {
			hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem, RetrievedAt: time.Now().Add(48 * time.Hour)})
		},
		"oversize metadata": func(t *testing.T, base string) {
			hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem, Extra: map[string]any{"padding": big}})
		},
		"oversize item": func(t *testing.T, base string) {
			hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: []byte("a: " + strings.Repeat("b", 1<<20) + "\n")})
		},
		"item is a link to another entry": func(t *testing.T, base string) {
			hubcachetest.Write(t, base, hubcachetest.Entry{Slug: otherCacheSlug, Item: cacheTestItem})
			hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem})
			item := filepath.Join(testEntryDir(base, cacheTestSlug), "item.yaml")
			require.NoError(t, os.Remove(item))
			require.NoError(t, os.Symlink(filepath.Join(testEntryDir(base, otherCacheSlug), "item.yaml"), item))
		},
		"item is a link": func(t *testing.T, base string) {
			hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem})
			item := filepath.Join(testEntryDir(base, cacheTestSlug), "item.yaml")
			require.NoError(t, os.Remove(item))
			require.NoError(t, os.Symlink(writeOtherFile(t, string(cacheTestItem)), item))
		},
		"item is a directory": func(t *testing.T, base string) {
			hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem})
			item := filepath.Join(testEntryDir(base, cacheTestSlug), "item.yaml")
			require.NoError(t, os.Remove(item))
			require.NoError(t, os.Mkdir(item, 0o700))
		},
		"content differs from the recorded digest": func(t *testing.T, base string) {
			hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem})
			require.NoError(t, os.WriteFile(filepath.Join(testEntryDir(base, cacheTestSlug), "item.yaml"), []byte("name: other\n"), 0o600))
		},
		"unsupported item type": func(t *testing.T, base string) {
			hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem, Type: "links"})
		},
		"metadata is not JSON": func(t *testing.T, base string) {
			hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem})
			require.NoError(t, os.WriteFile(filepath.Join(testEntryDir(base, cacheTestSlug), "metadata.json"), []byte("{"), 0o600))
		},
	}
	for name, prepare := range cases {
		t.Run(name, func(t *testing.T) {
			cache, base := newValidationCache(t)
			prepare(t, base)

			_, err := cache.Load(context.Background(), "collections", cacheTestSlug)
			require.Error(t, err)
			require.NoDirExists(t, testEntryDir(base, cacheTestSlug), "an invalid entry is evicted")
			if name == "entry directory links to another entry" {
				require.DirExists(t, testEntryDir(base, otherCacheSlug), "the entry behind the link survives")
			}
		})
	}
}

func TestLoadTreatsEntryWithoutDigestAsMiss(t *testing.T) {
	cache, base := newValidationCache(t)
	hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem, Omit: []string{"item_sha256"}})

	_, err := cache.Load(context.Background(), "collections", cacheTestSlug)
	require.ErrorIs(t, err, ErrCacheMiss)
	require.NoDirExists(t, testEntryDir(base, cacheTestSlug))
}

func TestLoadPreviewIgnoresUnknownMetadataFields(t *testing.T) {
	cache, base := newValidationCache(t)
	hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem, Extra: map[string]any{"unused_field": "ignored"}})

	preview, err := cache.LoadPreview(context.Background(), "collections", cacheTestSlug)
	require.NoError(t, err)
	require.Equal(t, string(cacheTestItem), preview)
}

func TestLoadPreviewRejectsInvalidCacheEntry(t *testing.T) {
	cache, base := newValidationCache(t)
	otherPath := writeOtherFile(t, string(cacheTestItem))
	hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem})
	item := filepath.Join(testEntryDir(base, cacheTestSlug), "item.yaml")
	require.NoError(t, os.Remove(item))
	require.NoError(t, os.Symlink(otherPath, item))

	_, err := cache.LoadPreview(context.Background(), "collections", cacheTestSlug)
	require.Error(t, err)
}

func TestPullCacheHitServesTheEntryItem(t *testing.T) {
	cache, base := newValidationCache(t)
	hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem, Extra: map[string]any{"unused_field": "ignored"}})
	svc := NewHubService(nil, cache, t.TempDir())
	svc.HTTPClient = nil // a cache hit needs no network

	res, err := svc.Pull(context.Background(), cacheTestSlug, "")
	require.NoError(t, err)
	require.Equal(t, string(cacheTestItem), res.Preview)
}

func TestListSkipsInvalidCacheEntries(t *testing.T) {
	cache, base := newValidationCache(t)
	hubcachetest.Write(t, base, hubcachetest.Entry{Slug: cacheTestSlug, Item: cacheTestItem})
	hubcachetest.Write(t, base, hubcachetest.Entry{Slug: otherCacheSlug, Item: cacheTestItem, Extra: map[string]any{"slug": "unrelated/entry"}})

	entries, err := cache.List(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, cacheTestSlug, entries[0].Slug)
}

func TestHubCacheRejectsMalformedSlugs(t *testing.T) {
	slugs := []string{".", "./", "a/..", "a//b", "-x/y", "ab", "a/b/c", "a/metadata.json", "a/item.yaml", "../x/y", "/abs/x", "a/b c", "a\\b"}
	for _, slug := range slugs {
		t.Run(slug, func(t *testing.T) {
			cache, base := newValidationCache(t)
			hubcachetest.Write(t, base, hubcachetest.Entry{Slug: "ab/cd", Item: cacheTestItem})

			_, loadErr := cache.Load(context.Background(), "collections", slug)
			require.Error(t, loadErr)
			require.Error(t, cache.Evict(context.Background(), "collections", slug))
			require.DirExists(t, base, "the cache root stays intact")
			require.DirExists(t, testEntryDir(base, "ab/cd"), "other entries stay intact")
		})
	}
}

func TestHubCacheAcceptsMixedCaseSlug(t *testing.T) {
	cache, base := newValidationCache(t)
	hubcachetest.Write(t, base, hubcachetest.Entry{Slug: "Azlaroc/sftpgo", Item: []byte("name: Azlaroc/sftpgo\n")})

	meta, err := cache.Load(context.Background(), "collections", "Azlaroc/sftpgo")
	require.NoError(t, err)
	require.Equal(t, "Azlaroc/sftpgo", meta.Slug)
}
